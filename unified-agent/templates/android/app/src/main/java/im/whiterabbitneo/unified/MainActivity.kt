package im.whiterabbitneo.unified

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewmodel.compose.viewModel
import im.whiterabbitneo.unified.data.GatewayClient
import im.whiterabbitneo.unified.data.MemoryStore
import im.whiterabbitneo.unified.data.TokenVault
import kotlinx.coroutines.launch
import org.json.JSONObject

private val Ink = Color(0xFF07110D)
private val Panel = Color(0xFF10261D)
private val Mint = Color(0xFF9EFFC8)
private val Muted = Color(0xFFB9CCBE)

enum class Destination(val label: String) { Command("Command"), Tools("Tools"), Skills("Skills"), Mcp("MCP"), Memory("Memory") }

class AgentViewModel : ViewModel() {
    var gatewayUrl by mutableStateOf("")
    var pairingCode by mutableStateOf("")
    var paired by mutableStateOf(false)
    var status by mutableStateOf("Gateway not paired")
    var answer by mutableStateOf("")
    var enabled by mutableStateOf(setOf<String>())
    private lateinit var vault: TokenVault
    private lateinit var memoryStore: MemoryStore
    private val client = GatewayClient()
    fun initialize(activity: ComponentActivity) { vault = TokenVault(activity); memoryStore = MemoryStore(activity); paired = vault.load() != null; if (paired) status = "Paired token stored on device" }
    suspend fun pair() { try { val result = client.pair(gatewayUrl, "WhiteRabbit Neo Android", pairingCode); vault.save(result.token); paired = true; status = "Paired as ${result.deviceId.take(8)}…" } catch (e: Exception) { status = e.message ?: "Pairing failed" } }
    suspend fun run(prompt: String) { try { val token = vault.load() ?: error("Pair the gateway first"); status = "DeepSeek R1 routing…"; val result = client.task(gatewayUrl, token, prompt, enabled); status = result.status; answer = result.answer } catch (e: Exception) { status = e.message ?: "Task failed" } }
    suspend fun addRegistry(kind: String, name: String, definition: JSONObject) { try { val token = vault.load() ?: error("Pair the gateway first"); client.createRegistryItem(gatewayUrl, token, kind, name, definition); status = "$kind added to registry" } catch (e: Exception) { status = e.message ?: "Registry update failed" } }
    fun memory(): List<String> = if (::memoryStore.isInitialized) memoryStore.list() else emptyList()
    fun addMemory(note: String) { memoryStore.add(note) }
    fun deleteMemory(note: String) { memoryStore.delete(note) }
}

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) { super.onCreate(savedInstanceState); setContent { WhiteRabbitApp(this) } }
}

@Composable
fun WhiteRabbitApp(activity: ComponentActivity, vm: AgentViewModel = viewModel()) {
    LaunchedEffect(Unit) { vm.initialize(activity) }
    var destination by remember { mutableStateOf(Destination.Command) }
    MaterialTheme(colorScheme = darkColorScheme(primary = Mint, secondary = Mint, surface = Panel, background = Ink, onPrimary = Ink, onBackground = Color.White)) {
        Scaffold(containerColor = Ink, bottomBar = {
            NavigationBar(containerColor = Panel) { Destination.entries.forEach { item -> NavigationBarItem(selected = destination == item, onClick = { destination = item }, icon = { Text(item.label.take(1), color = Mint, fontWeight = FontWeight.Bold) }, label = { Text(item.label) }) } }
        }) { padding -> Column(Modifier.fillMaxSize().padding(padding).padding(16.dp)) {
            Text("WHITE RABBIT NEO", color = Mint, style = MaterialTheme.typography.labelLarge, fontWeight = FontWeight.Black)
            Text("Unified agent HUD · DeepSeek R1 routing", color = Color.White, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(8.dp)); StatusCard(vm.status, vm.paired)
            Spacer(Modifier.height(12.dp))
            when (destination) {
                Destination.Command -> CommandScreen(vm)
                Destination.Tools -> RegistryScreen(vm, "tool", "Tools", "Create permission-scoped web, API, or automation-proposal tools.")
                Destination.Skills -> RegistryScreen(vm, "skill", "Skills", "Save reusable agent instructions with explicit allowed tools.")
                Destination.Mcp -> RegistryScreen(vm, "mcp", "MCP Registry", "Store trusted MCP endpoint metadata. Credentials remain on the authorized gateway.")
                Destination.Memory -> MemoryScreen(vm)
            }
        } }
    }
}

@Composable private fun StatusCard(status: String, paired: Boolean) = Card(colors = CardDefaults.cardColors(containerColor = Panel), shape = RoundedCornerShape(14.dp)) { Row(Modifier.fillMaxWidth().padding(14.dp), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) { Column { Text(if (paired) "SECURE GATEWAY LINK" else "SETUP REQUIRED", color = Mint, style = MaterialTheme.typography.labelMedium); Text(status, color = Muted, style = MaterialTheme.typography.bodySmall) }; AssistChip(onClick = {}, label = { Text(if (paired) "PAIRED" else "OFFLINE") }) } }

@Composable private fun CommandScreen(vm: AgentViewModel) { val scope = rememberCoroutineScope(); var prompt by remember { mutableStateOf("") }; var web by remember { mutableStateOf(false) }; var api by remember { mutableStateOf(false) }; var automation by remember { mutableStateOf(false) }; Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(10.dp)) {
    OutlinedTextField(value = vm.gatewayUrl, onValueChange = { vm.gatewayUrl = it }, label = { Text("HTTPS gateway URL") }, modifier = Modifier.fillMaxWidth(), singleLine = true)
    if (!vm.paired) { OutlinedTextField(value = vm.pairingCode, onValueChange = { vm.pairingCode = it }, label = { Text("One-time pairing code") }, modifier = Modifier.fillMaxWidth(), singleLine = true); Button(onClick = { scope.launch { vm.pair() } }, modifier = Modifier.fillMaxWidth()) { Text("Pair this device") } }
    Text("Capability selection", color = Mint, fontWeight = FontWeight.Bold)
    CapabilityToggle("Web read — public HTTPS only", web) { web = it; vm.enabled = vm.enabled.toMutableSet().apply { if (it) add("web_read") else remove("web_read") } }
    CapabilityToggle("API calls — gateway-authorized only", api) { api = it; vm.enabled = vm.enabled.toMutableSet().apply { if (it) add("api_call") else remove("api_call") } }
    CapabilityToggle("Automation — always proposed, never silent", automation) { automation = it; vm.enabled = vm.enabled.toMutableSet().apply { if (it) add("automation") else remove("automation") } }
    OutlinedTextField(value = prompt, onValueChange = { prompt = it }, label = { Text("Ask the unified agent") }, modifier = Modifier.fillMaxWidth().height(140.dp))
    Button(onClick = { scope.launch { vm.run(prompt) } }, enabled = vm.paired && prompt.isNotBlank(), modifier = Modifier.fillMaxWidth()) { Text("Run with DeepSeek R1") }
    if (vm.answer.isNotBlank()) Card(colors = CardDefaults.cardColors(containerColor = Panel)) { Text(vm.answer, Modifier.padding(14.dp), color = Color.White) }
} }

@Composable private fun CapabilityToggle(label: String, checked: Boolean, changed: (Boolean) -> Unit) = Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) { Switch(checked = checked, onCheckedChange = changed); Spacer(Modifier.width(10.dp)); Text(label, color = Muted, style = MaterialTheme.typography.bodyMedium) }

@Composable private fun RegistryScreen(vm: AgentViewModel, kind: String, title: String, guidance: String) { val scope = rememberCoroutineScope(); var name by remember { mutableStateOf("") }; var detail by remember { mutableStateOf("") }; Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(10.dp)) { Text(title, color = Mint, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold); Text(guidance, color = Muted); OutlinedTextField(value = name, onValueChange = { name = it }, label = { Text("Name") }, modifier = Modifier.fillMaxWidth()); OutlinedTextField(value = detail, onValueChange = { detail = it }, label = { Text(if (kind == "skill") "Instructions" else "HTTPS endpoint or description") }, modifier = Modifier.fillMaxWidth()); Button(onClick = { val definition = when (kind) { "tool" -> JSONObject().put("mode", "web_read").put("url", detail.ifBlank { "https://example.com" }); "skill" -> JSONObject().put("instructions", detail); else -> JSONObject().put("transport", "streamable_http").put("endpoint", detail) }; scope.launch { vm.addRegistry(kind, name.ifBlank { "Untitled $kind" }, definition) } }, enabled = vm.paired, modifier = Modifier.fillMaxWidth()) { Text("Register $kind") }; Text("All registrations remain inert until a user approves a supported gateway invocation.", color = Muted, style = MaterialTheme.typography.bodySmall) } }

@Composable private fun MemoryScreen(vm: AgentViewModel) { var note by remember { mutableStateOf("") }; var notes by remember { mutableStateOf(vm.memory()) }; Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(10.dp)) { Text("Persistent Memory", color = Mint, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold); Text("Notes stay on this device until you delete them.", color = Muted); OutlinedTextField(value = note, onValueChange = { note = it }, label = { Text("Memory note") }, modifier = Modifier.fillMaxWidth()); Button(onClick = { if (note.isNotBlank()) { vm.addMemory(note); notes = vm.memory(); note = "" } }, modifier = Modifier.fillMaxWidth()) { Text("Store locally") }; notes.forEach { item -> Card(colors = CardDefaults.cardColors(containerColor = Panel)) { Row(Modifier.fillMaxWidth().padding(12.dp), horizontalArrangement = Arrangement.SpaceBetween) { Text(item, color = Color.White, modifier = Modifier.weight(1f)); TextButton(onClick = { vm.deleteMemory(item); notes = vm.memory() }) { Text("Delete") } } } } } }

package ai.usbagent.commander

import android.content.Context
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.net.URI

private const val PREFS = "usb_agent_gateway"
private const val ENDPOINT = "endpoint"
private const val TOKEN = "token"

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { MaterialTheme { CommandCenter() } }
    }
}

data class UiState(
    val endpoint: String = "",
    val pairingCode: String = "",
    val prompt: String = "",
    val connection: String = "Not paired",
    val hostSummary: String = "No gateway status loaded.",
    val taskSummary: String = "",
    val integritySummary: String = "Integrity not checked.",
    val skillsSummary: String = "Skills not loaded.",
    val timelineSummary: String = "No task checkpoints.",
    val error: String? = null
)

class GatewayStore(context: Context) {
    private val key = MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build()
    private val prefs = EncryptedSharedPreferences.create(
        context,
        PREFS,
        key,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
    )
    fun endpoint(): String = prefs.getString(ENDPOINT, "") ?: ""
    fun token(): String = prefs.getString(TOKEN, "") ?: ""
    fun save(endpoint: String, token: String) { prefs.edit().putString(ENDPOINT, endpoint).putString(TOKEN, token).apply() }
    fun clear() { prefs.edit().clear().apply() }
}

class GatewayViewModel : ViewModel() {
    private val http = OkHttpClient()
    private var store: GatewayStore? = null
    var ui by mutableStateOf(UiState())
        private set

    fun initialize(context: Context) {
        if (store == null) store = GatewayStore(context.applicationContext)
        val endpoint = store?.endpoint().orEmpty()
        if (endpoint.isNotBlank()) ui = ui.copy(endpoint = endpoint, connection = "Paired")
    }

    fun updateEndpoint(value: String) { ui = ui.copy(endpoint = value, error = null) }
    fun updateCode(value: String) { ui = ui.copy(pairingCode = value, error = null) }
    fun updatePrompt(value: String) { ui = ui.copy(prompt = value, error = null) }

    private fun normalizedEndpoint(): String {
        val endpoint = ui.endpoint.trim().removeSuffix("/")
        val uri = URI(endpoint)
        require(uri.scheme == "https") { "The Android companion accepts HTTPS gateway URLs only." }
        require(!uri.host.isNullOrBlank()) { "Enter a complete HTTPS gateway URL." }
        return endpoint
    }

    fun pair() = CoroutineScope(Dispatchers.IO).launch {
        try {
            val endpoint = normalizedEndpoint()
            val body = JSONObject().put("code", ui.pairingCode).toString().toRequestBody("application/json".toMediaType())
            val request = Request.Builder().url("$endpoint/api/pair").post(body).build()
            http.newCall(request).execute().use { response ->
                if (!response.isSuccessful) throw IllegalStateException("Pairing failed with HTTP ${response.code}")
                val token = JSONObject(response.body?.string() ?: throw IllegalStateException("Gateway returned an empty response body")).getString("access_token")
                store?.save(endpoint, token)
                ui = ui.copy(connection = "Paired securely", pairingCode = "", error = null)
            }
        } catch (error: Exception) { ui = ui.copy(error = error.message ?: "Pairing failed") }
    }

    fun refresh() = CoroutineScope(Dispatchers.IO).launch {
        try {
            val endpoint = store?.endpoint().orEmpty()
            val token = store?.token().orEmpty()
            require(endpoint.isNotBlank() && token.isNotBlank()) { "Pair with a gateway first." }
            val request = Request.Builder().url("$endpoint/api/status").header("Authorization", "Bearer $token").get().build()
            http.newCall(request).execute().use { response ->
                if (!response.isSuccessful) throw IllegalStateException("Status failed with HTTP ${response.code}")
                val root = JSONObject(response.body?.string() ?: throw IllegalStateException("Gateway returned an empty response body"))
                val host = root.getJSONObject("host")
                val profile = host.getJSONObject("profile").getString("name")
                val summary = "Profile $profile · CPU ${host.getDouble("cpu_percent")}% · RAM ${host.getDouble("memory_percent")}% · Free ${host.getLong("disk_free_mb") / 1024} GB"
                val integrity = root.optJSONObject("integrity")?.optString("status", "Unknown") ?: "Unknown"
                val skillCount = root.optJSONArray("skills")?.length() ?: 0
                val tasks = root.optJSONArray("tasks")
                val latest = if (tasks != null && tasks.length() > 0) tasks.getJSONObject(0).optString("state", "Unknown") else "No tasks"
                ui = ui.copy(hostSummary = summary, integritySummary = "Release integrity: $integrity", skillsSummary = "$skillCount registered skills", timelineSummary = "Latest task: $latest", connection = "Gateway online", error = null)
            }
        } catch (error: Exception) { ui = ui.copy(connection = "Gateway unavailable", error = error.message ?: "Status unavailable") }
    }

    fun sendTask() = CoroutineScope(Dispatchers.IO).launch {
        try {
            val endpoint = store?.endpoint().orEmpty()
            val token = store?.token().orEmpty()
            require(ui.prompt.isNotBlank()) { "Enter a task first." }
            val body = JSONObject().put("prompt", ui.prompt).toString().toRequestBody("application/json".toMediaType())
            val request = Request.Builder().url("$endpoint/api/tasks").header("Authorization", "Bearer $token").post(body).build()
            http.newCall(request).execute().use { response ->
                if (!response.isSuccessful) throw IllegalStateException("Task submission failed with HTTP ${response.code}")
                val task = JSONObject(response.body?.string() ?: throw IllegalStateException("Gateway returned an empty response body"))
                val route = task.getJSONObject("route")
                ui = ui.copy(taskSummary = "Queued ${task.getString("id").take(8)} via ${route.getString("reason")}", prompt = "", error = null)
            }
        } catch (error: Exception) { ui = ui.copy(error = error.message ?: "Task submission failed") }
    }

    fun disconnect() { store?.clear(); ui = UiState() }
}

@Composable
fun CommandCenter(viewModel: GatewayViewModel = viewModel()) {
    val context = androidx.compose.ui.platform.LocalContext.current
    viewModel.initialize(context)
    val ui = viewModel.ui
    var showPairing by remember { mutableStateOf(ui.connection != "Paired" && ui.connection != "Paired securely") }

    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp)
    ) {
        Text("USB Agent Commander", style = MaterialTheme.typography.headlineMedium)
        Text("Local-first companion for the portable agent gateway.", color = Color(0xFF8CB1C9))
        Card(Modifier.fillMaxWidth()) {
            Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Gateway", style = MaterialTheme.typography.titleMedium)
                Text(ui.connection)
                Text(ui.hostSummary, style = MaterialTheme.typography.bodySmall)
                Text(ui.integritySummary, style = MaterialTheme.typography.bodySmall)
                Text(ui.skillsSummary, style = MaterialTheme.typography.bodySmall)
                Text(ui.timelineSummary, style = MaterialTheme.typography.bodySmall)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = { showPairing = !showPairing }) { Text(if (showPairing) "Hide pairing" else "Pair gateway") }
                    TextButton(onClick = viewModel::refresh) { Text("Refresh") }
                    TextButton(onClick = viewModel::disconnect) { Text("Disconnect") }
                }
                if (showPairing) {
                    OutlinedTextField(ui.endpoint, viewModel::updateEndpoint, label = { Text("HTTPS gateway URL") }, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(ui.pairingCode, viewModel::updateCode, label = { Text("One-time pairing code") }, modifier = Modifier.fillMaxWidth())
                    Button(onClick = viewModel::pair) { Text("Pair securely") }
                }
            }
        }
        Card(Modifier.fillMaxWidth()) {
            Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Run task", style = MaterialTheme.typography.titleMedium)
                OutlinedTextField(ui.prompt, viewModel::updatePrompt, label = { Text("Task prompt") }, modifier = Modifier.fillMaxWidth(), minLines = 3)
                Button(onClick = viewModel::sendTask) { Text("Send to unified agent") }
                if (ui.taskSummary.isNotBlank()) Text(ui.taskSummary, color = Color(0xFF55D7B5))
            }
        }
        if (ui.error != null) Card(Modifier.fillMaxWidth()) { Text(ui.error, color = Color(0xFFFB7B8B), modifier = Modifier.padding(16.dp)) }
        Card(Modifier.fillMaxWidth()) {
            Text("Device and system actions remain proposals until you approve them locally. This companion does not request accessibility, screen-capture, or administrator-like capabilities by default.", modifier = Modifier.padding(16.dp), style = MaterialTheme.typography.bodySmall)
        }
    }
}

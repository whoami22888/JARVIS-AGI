package im.whiterabbitneo.unified.ui

import android.content.ActivityNotFoundException
import android.content.Intent
import android.net.Uri
import androidx.browser.customtabs.CustomTabsIntent
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.fragment.app.FragmentActivity
import im.whiterabbitneo.unified.connectors.AuthType
import im.whiterabbitneo.unified.connectors.BiometricGate
import im.whiterabbitneo.unified.connectors.ConnectorStatus
import kotlinx.coroutines.launch

private val Mint = Color(0xFF9EFFC8)
private val Muted = Color(0xFFB9CCBE)
private val Panel = Color(0xFF10261D)
private val Amber = Color(0xFFFFD479)

@Composable
fun ConnectorsScreen(vm: ConnectorViewModel) {
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    var addKeyFor by remember { mutableStateOf<ConnectorUiState?>(null) }
    var configureFor by remember { mutableStateOf<ConnectorUiState?>(null) }
    var revealFor by remember { mutableStateOf<ConnectorUiState?>(null) }

    Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Text("Account connectors", color = Mint, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Text(
            "Service connectors store API keys and OAuth2 tokens on this device only. Secrets are encrypted with the Android Keystore and never sent to the gateway without your approval.",
            color = Muted,
            style = MaterialTheme.typography.bodySmall,
        )
        if (vm.message.isNotBlank()) Text(vm.message, color = Amber, style = MaterialTheme.typography.bodySmall)
        OutlinedButton(onClick = { vm.refresh() }, modifier = Modifier.fillMaxWidth()) { Text("Refresh connector status") }
        vm.connectors.forEach { state ->
            ConnectorCard(
                state = state,
                busy = vm.busyConnector == state.definition.id,
                onConnect = { scope.launch { vm.connect(state.definition.id) } },
                onAddKey = { addKeyFor = state },
                onConfigure = { configureFor = state },
                onAuthorize = {
                    scope.launch {
                        try {
                            val url = vm.startAuthorization(state.definition.id)
                            openInCustomTabs(context, url)
                        } catch (e: Exception) {
                            vm.message = e.message ?: "Authorization failed to start"
                        }
                    }
                },
                onDisconnect = { scope.launch { vm.disconnect(state.definition.id) } },
                onReveal = { revealFor = state },
                onToggle = { enabled -> vm.setEnabled(state.definition.id, enabled) },
            )
        }
    }

    addKeyFor?.let { state ->
        AddKeyDialog(
            title = "Connect ${state.name}",
            hint = state.definition.apiKeyHint ?: "API key",
            onDismiss = { addKeyFor = null },
            onConfirm = { key ->
                addKeyFor = null
                scope.launch { vm.connect(state.definition.id, key) }
            },
        )
    }
    configureFor?.let { state ->
        ConfigureOAuthDialog(
            state = state,
            onDismiss = { configureFor = null },
            onConfirm = { clientId, clientSecret, redirectUri ->
                configureFor = null
                scope.launch { vm.saveOAuthConfig(state.definition.id, clientId, clientSecret, redirectUri) }
            },
        )
    }
    revealFor?.let { state ->
        var secret by remember { mutableStateOf<String?>(null) }
        val activity = context as? FragmentActivity
        LaunchedEffect(state.definition.id) {
            if (activity != null && BiometricGate.canAuthenticate(context)) {
                BiometricGate.prompt(
                    activity,
                    "Reveal ${state.name} secret",
                    onSuccess = { vm.revealSecret(state.definition.id) { secret = it } },
                    onError = { message -> secret = "Authentication failed: $message" },
                )
            } else {
                vm.revealSecret(state.definition.id) { secret = it }
            }
        }
        AlertDialog(
            onDismissRequest = { revealFor = null },
            title = { Text(state.name) },
            text = { Text(secret ?: "Waiting for confirmation…", color = Muted) },
            confirmButton = { TextButton(onClick = { revealFor = null }) { Text("Close") } },
        )
    }
}

@Composable
private fun ConnectorCard(
    state: ConnectorUiState,
    busy: Boolean,
    onConnect: () -> Unit,
    onAddKey: () -> Unit,
    onConfigure: () -> Unit,
    onAuthorize: () -> Unit,
    onDisconnect: () -> Unit,
    onReveal: () -> Unit,
    onToggle: (Boolean) -> Unit,
) {
    Card(colors = CardDefaults.cardColors(containerColor = Panel)) {
        Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                Text(state.name, color = Color.White, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
                StatusChip(state.status)
            }
            Text(state.description, color = Muted, style = MaterialTheme.typography.bodySmall)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                SuggestionChip(onClick = {}, label = { Text(state.category.name, color = Mint) })
                SuggestionChip(onClick = {}, label = { Text(state.definition.authType.name, color = Mint) })
            }
            state.accountEmail?.let { Text("Account: $it", color = Muted, style = MaterialTheme.typography.bodySmall) }
            if (busy) {
                LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
            } else {
                when {
                    state.definition.authType == AuthType.NONE -> {
                        if (state.status == ConnectorStatus.NOT_CONNECTED) {
                            Button(onClick = onConnect, modifier = Modifier.fillMaxWidth()) { Text("Connect locally") }
                        }
                    }
                    state.definition.authType == AuthType.API_KEY -> {
                        if (state.status == ConnectorStatus.NOT_CONNECTED || state.status == ConnectorStatus.EXPIRED) {
                            Button(onClick = onAddKey, modifier = Modifier.fillMaxWidth()) { Text("Add API key") }
                        } else {
                            OutlinedButton(onClick = onReveal, modifier = Modifier.fillMaxWidth()) { Text("Show stored key") }
                        }
                    }
                    else -> {
                        if (state.status == ConnectorStatus.CONNECTED) {
                            OutlinedButton(onClick = onReveal, modifier = Modifier.fillMaxWidth()) { Text("Manage connection") }
                        } else {
                            OutlinedButton(onClick = onConfigure, modifier = Modifier.fillMaxWidth()) { Text("Configure OAuth2") }
                            Button(onClick = onAuthorize, modifier = Modifier.fillMaxWidth()) { Text("Authorize in browser") }
                        }
                    }
                }
            }
            if (state.status == ConnectorStatus.CONNECTED) {
                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                    Switch(checked = state.enabled, onCheckedChange = onToggle)
                    Spacer(Modifier.width(10.dp))
                    Text(if (state.enabled) "Enabled for agent tasks" else "Disabled", color = Muted, style = MaterialTheme.typography.bodySmall)
                }
                OutlinedButton(onClick = onDisconnect, modifier = Modifier.fillMaxWidth()) { Text("Disconnect") }
            }
        }
    }
}

@Composable
private fun StatusChip(status: ConnectorStatus) {
    val color = when (status) {
        ConnectorStatus.CONNECTED -> Mint
        ConnectorStatus.EXPIRED -> Amber
        ConnectorStatus.ERROR -> Color(0xFFFF8A80)
        ConnectorStatus.NOT_CONNECTED -> Muted
    }
    SuggestionChip(onClick = {}, label = { Text(status.name, color = color) })
}

@Composable
private fun AddKeyDialog(title: String, hint: String, onDismiss: () -> Unit, onConfirm: (String) -> Unit) {
    var key by remember { mutableStateOf("") }
    var visible by remember { mutableStateOf(false) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = key,
                    onValueChange = { key = it },
                    label = { Text(hint) },
                    singleLine = true,
                    visualTransformation = if (visible) VisualTransformation.None else PasswordVisualTransformation(),
                    modifier = Modifier.fillMaxWidth(),
                )
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(checked = visible, onCheckedChange = { visible = it })
                    Text("Show key", color = Muted, style = MaterialTheme.typography.bodySmall)
                }
                Text("The key is encrypted on this device with the Android Keystore.", color = Muted, style = MaterialTheme.typography.bodySmall)
            }
        },
        confirmButton = {
            TextButton(onClick = { if (key.isNotBlank()) onConfirm(key) }, enabled = key.isNotBlank()) { Text("Connect") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}

@Composable
private fun ConfigureOAuthDialog(state: ConnectorUiState, onDismiss: () -> Unit, onConfirm: (String, String, String) -> Unit) {
    var clientId by remember { mutableStateOf(state.clientId.orEmpty()) }
    var clientSecret by remember { mutableStateOf("") }
    var redirectUri by remember { mutableStateOf(state.redirectUri.orEmpty()) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("${state.name} OAuth2") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(value = clientId, onValueChange = { clientId = it }, label = { Text("OAuth client ID") }, modifier = Modifier.fillMaxWidth())
                OutlinedTextField(value = clientSecret, onValueChange = { clientSecret = it }, label = { Text("OAuth client secret (optional)") }, modifier = Modifier.fillMaxWidth())
                OutlinedTextField(
                    value = redirectUri,
                    onValueChange = { redirectUri = it },
                    label = { Text("Redirect URI (blank = loopback)") },
                    placeholder = { Text("http://127.0.0.1:48731/oauth") },
                    modifier = Modifier.fillMaxWidth(),
                )
                Text(
                    "Register the redirect URI above in the provider console. Blank uses a local loopback server on this device.",
                    color = Muted,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        },
        confirmButton = {
            TextButton(
                onClick = { if (clientId.isNotBlank()) onConfirm(clientId, clientSecret, redirectUri) },
                enabled = clientId.isNotBlank(),
            ) { Text("Save configuration") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}

private fun openInCustomTabs(context: android.content.Context, url: String) {
    val uri = Uri.parse(url)
    try {
        CustomTabsIntent.Builder().setShowTitle(true).build().launchUrl(context, uri)
    } catch (_: ActivityNotFoundException) {
        context.startActivity(Intent(Intent.ACTION_VIEW, uri).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
    }
}
package im.whiterabbitneo.unified.ui

import android.content.Context
import android.net.Uri
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import im.whiterabbitneo.unified.connectors.AuthToken
import im.whiterabbitneo.unified.connectors.ConnectorCatalog
import im.whiterabbitneo.unified.connectors.ConnectorStatus
import im.whiterabbitneo.unified.connectors.ConnectorTokenVault
import im.whiterabbitneo.unified.connectors.OAuthRedirectBus
import im.whiterabbitneo.unified.connectors.SecretBox
import im.whiterabbitneo.unified.connectors.ServiceConnector
import im.whiterabbitneo.unified.connectors.impl.ApiKeyConnector
import im.whiterabbitneo.unified.connectors.impl.LocalOllamaConnector
import im.whiterabbitneo.unified.connectors.impl.LoopbackOAuthServer
import im.whiterabbitneo.unified.connectors.impl.OAuth2Connector
import im.whiterabbitneo.unified.db.AccountEntity
import im.whiterabbitneo.unified.db.ConnectorDatabase
import im.whiterabbitneo.unified.db.ConnectorEntity
import im.whiterabbitneo.unified.db.TokenEntity
import kotlinx.coroutines.launch
import org.json.JSONObject

class ConnectorViewModel : ViewModel() {
    var connectors by mutableStateOf<List<ConnectorUiState>>(emptyList())
        private set
    var message by mutableStateOf("")
    var busyConnector by mutableStateOf<String?>(null)

    private lateinit var appContext: Context
    private lateinit var dao: im.whiterabbitneo.unified.db.ConnectorDao
    private lateinit var vault: ConnectorTokenVault
    private val instances = mutableMapOf<String, ServiceConnector>()
    private val oauthServers = mutableMapOf<String, LoopbackOAuthServer>()

    fun initialize(context: Context) {
        if (::appContext.isInitialized) return
        appContext = context.applicationContext
        dao = ConnectorDatabase.build(context).connectorDao()
        vault = ConnectorTokenVault(context)
        refresh()
        viewModelScope.launch {
            OAuthRedirectBus.observe().collect { uri ->
                handleOAuthRedirect(uri)
            }
        }
    }

    fun refresh() {
        viewModelScope.launch {
            val entities = ConnectorCatalog.connectors.mapNotNull { dao.getConnector(it.id) }.associateBy { it.id }
            connectors = ConnectorCatalog.connectors.map { definition ->
                val entity = entities[definition.id]
                val token = vault.load(definition.id)
                val status = when {
                    token == null -> ConnectorStatus.NOT_CONNECTED
                    token.isExpired() -> ConnectorStatus.EXPIRED
                    else -> ConnectorStatus.CONNECTED
                }
                ConnectorUiState(
                    definition = definition,
                    status = status,
                    accountEmail = entity?.accountEmail,
                    enabled = entity?.isEnabled ?: false,
                    clientId = entity?.clientId,
                    redirectUri = entity?.redirectUri,
                )
            }
        }
    }

    fun connectorFor(id: String): ServiceConnector =
        instances.getOrPut(id) { buildConnector(id) }

    private fun buildConnector(id: String): ServiceConnector {
        val definition = ConnectorCatalog.byId(id) ?: error("Unknown connector: $id")
        return when (definition.authType) {
            im.whiterabbitneo.unified.connectors.AuthType.API_KEY -> ApiKeyConnector(definition)
            im.whiterabbitneo.unified.connectors.AuthType.OAUTH2 -> OAuth2Connector(definition)
            else -> LocalOllamaConnector(definition)
        }
    }

    suspend fun connect(id: String, secret: String? = null) {
        busyConnector = id
        try {
            val connector = connectorFor(id)
            val token = connector.authenticate(appContext, secret).getOrElse { error("${connector.name}: ${it.message}") }
            persistToken(id, token)
            val info = connector.getAccountInfo(token)
            dao.upsertAccount(
                AccountEntity(
                    connectorId = id,
                    email = info.email,
                    displayName = info.displayName,
                    detailsJson = JSONObject(info.details).toString(),
                    lastSync = System.currentTimeMillis(),
                )
            )
            ensureEntity(id)
            message = "${connector.name} connected"
        } catch (e: Exception) {
            message = e.message ?: "Connection failed"
        }
        busyConnector = null
        refresh()
    }

    fun revealSecret(id: String, onResult: (String?) -> Unit) {
        onResult(vault.load(id)?.accessToken)
    }

    suspend fun saveOAuthConfig(id: String, clientId: String, clientSecret: String, redirectUri: String) {
        val box = SecretBox("wrn_connector_$id")
        dao.updateOAuthConfig(
            id = id,
            clientId = clientId.trim().takeIf { it.isNotBlank() },
            secretCipher = clientSecret.trim().takeIf { it.isNotBlank() }?.let { box.encrypt(it) },
            redirectUri = redirectUri.trim().takeIf { it.isNotBlank() },
            scopesJson = ConnectorCatalog.byId(id)?.scopes?.let { scopes -> org.json.JSONArray(scopes).toString() } ?: "[]",
            lastSync = System.currentTimeMillis(),
        )
        message = "OAuth2 configuration saved for ${ConnectorCatalog.byId(id)?.name ?: id}"
        refresh()
    }

    suspend fun startAuthorization(id: String): String {
        val connector = connectorFor(id) as? OAuth2Connector
            ?: error("${ConnectorCatalog.byId(id)?.name ?: id} does not use OAuth2")
        val entity = dao.getConnector(id)
        val clientId = entity?.clientId?.takeIf { it.isNotBlank() }
            ?: error("Save the OAuth2 client ID before authorizing")
        val redirectUri = entity.redirectUri?.takeIf { it.isNotBlank() } ?: run {
            val server = oauthServers.getOrPut(id) { LoopbackOAuthServer() }
            server.start("/oauth") { code, state ->
                viewModelScope.launch { completeExchange(id, serverRedirectUrl(id, code, state), code) }
            }
            server.redirectUri
        }
        return connector.authorizationUrl(clientId, redirectUri)
    }

    fun handleOAuthRedirect(uri: Uri) {
        val connectorId = uri.pathSegments.lastOrNull()?.takeIf { ConnectorCatalog.byId(it) != null } ?: return
        val code = uri.getQueryParameter("code") ?: return
        viewModelScope.launch { completeExchange(connectorId, uri.toString(), code) }
    }

    private suspend fun completeExchange(connectorId: String, redirectUrl: String, code: String) {
        val connector = connectorFor(connectorId) as? OAuth2Connector ?: return
        val entity = dao.getConnector(connectorId) ?: error("Connector is not configured")
        val clientId = entity.clientId ?: error("Client ID is missing")
        val clientSecret = entity.clientSecretCipher?.let { SecretBox("wrn_connector_$connectorId").decrypt(it) }
        try {
            val token = connector.completeAuthorization(appContext, redirectUrl, code, clientId, clientSecret)
                .getOrElse { error("${connector.name}: ${it.message}") }
            persistToken(connectorId, token)
            val info = connector.getAccountInfo(token)
            dao.upsertAccount(
                AccountEntity(
                    connectorId = connectorId,
                    email = info.email,
                    displayName = info.displayName,
                    detailsJson = JSONObject(info.details).toString(),
                    lastSync = System.currentTimeMillis(),
                )
            )
            message = "${connector.name} authorized"
        } catch (e: Exception) {
            message = e.message ?: "Authorization failed"
        }
        oauthServers.remove(connectorId)?.stop()
        refresh()
    }

    suspend fun disconnect(id: String) {
        val name = ConnectorCatalog.byId(id)?.name ?: id
        try {
            val token = vault.load(id)
            if (token != null) connectorFor(id).revokeAccess(token)
        } catch (_: Exception) {
        }
        vault.clear(id)
        dao.deleteToken(id)
        dao.deleteAccount(id)
        oauthServers.remove(id)?.stop()
        message = "$name disconnected"
        refresh()
    }

    fun setEnabled(id: String, enabled: Boolean) {
        viewModelScope.launch {
            ensureEntity(id)
            dao.setEnabled(id, enabled, System.currentTimeMillis())
            refresh()
        }
    }

    private suspend fun persistToken(id: String, token: AuthToken) {
        vault.save(token)
        val box = SecretBox("wrn_connector_$id")
        dao.upsertToken(
            TokenEntity(
                connectorId = id,
                accessTokenCipher = box.encrypt(token.accessToken),
                refreshTokenCipher = token.refreshToken?.let { box.encrypt(it) },
                tokenType = token.tokenType,
                expiresAtEpochSeconds = token.expiresAtEpochSeconds,
                scopesJson = org.json.JSONArray(token.scopes).toString(),
                updatedAt = System.currentTimeMillis(),
            )
        )
    }

    private suspend fun ensureEntity(id: String) {
        if (dao.getConnector(id) == null) {
            val definition = ConnectorCatalog.byId(id) ?: return
            dao.upsertConnector(
                ConnectorEntity(
                    id = definition.id,
                    name = definition.name,
                    category = definition.category.name,
                    authType = definition.authType.name,
                    clientId = null,
                    clientSecretCipher = null,
                    redirectUri = null,
                    scopesJson = org.json.JSONArray(definition.scopes).toString(),
                    isEnabled = false,
                    lastSync = System.currentTimeMillis(),
                    accountEmail = null,
                )
            )
        }
    }

    private fun serverRedirectUrl(id: String, code: String, state: String?): String {
        val server = oauthServers[id]
        val base = server?.redirectUri ?: "http://127.0.0.1:48731/oauth"
        return "$base?code=$code&state=${state.orEmpty()}"
    }
}
package im.whiterabbitneo.unified.connectors.impl

import android.content.Context
import im.whiterabbitneo.unified.connectors.AccountInfo
import im.whiterabbitneo.unified.connectors.AuthToken
import im.whiterabbitneo.unified.connectors.AuthType
import im.whiterabbitneo.unified.connectors.Category
import im.whiterabbitneo.unified.connectors.ConnectorDefinition
import im.whiterabbitneo.unified.connectors.ServiceConnector

class LocalOllamaConnector(private val definition: ConnectorDefinition) : ServiceConnector {
    override val id: String = definition.id
    override val name: String = definition.name
    override val category: Category = definition.category
    override val authType: AuthType = AuthType.NONE
    override val requiredScopes: List<String> = definition.scopes

    override suspend fun authenticate(context: Context, secret: String?): Result<AuthToken> = runCatching {
        AuthToken(
            connectorId = id,
            accessToken = "local-device-access",
            refreshToken = null,
            tokenType = "None",
            expiresAtEpochSeconds = null,
            scopes = emptyList(),
        )
    }

    override suspend fun getAccountInfo(token: AuthToken): AccountInfo =
        AccountInfo(connectorId = id, email = null, displayName = name, details = mapOf("mode" to "local"))
}
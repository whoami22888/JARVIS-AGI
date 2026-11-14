package im.whiterabbitneo.unified.connectors.impl

import android.content.Context
import im.whiterabbitneo.unified.connectors.AccountInfo
import im.whiterabbitneo.unified.connectors.AuthToken
import im.whiterabbitneo.unified.connectors.Category
import im.whiterabbitneo.unified.connectors.AuthType
import im.whiterabbitneo.unified.connectors.ConnectorDefinition
import im.whiterabbitneo.unified.connectors.HttpJson
import im.whiterabbitneo.unified.connectors.ServiceConnector

class ApiKeyConnector(private val definition: ConnectorDefinition) : ServiceConnector {
    override val id: String = definition.id
    override val name: String = definition.name
    override val category: Category = definition.category
    override val authType: AuthType = definition.authType
    override val requiredScopes: List<String> = definition.scopes

    override suspend fun authenticate(context: Context, secret: String?): Result<AuthToken> = runCatching {
        val key = secret?.trim().orEmpty()
        require(key.isNotBlank()) { "$name requires an API key" }
        AuthToken(
            connectorId = id,
            accessToken = key,
            refreshToken = null,
            tokenType = "Bearer",
            expiresAtEpochSeconds = null,
            scopes = definition.scopes,
        )
    }

    override suspend fun getAccountInfo(token: AuthToken): AccountInfo {
        val url = definition.accountInfoUrl ?: return AccountInfo(id, null, null)
        return runCatching {
            val json = HttpJson.getJson(url, token.accessToken)
            val data = json.optJSONObject("data") ?: json
            AccountInfo(
                connectorId = id,
                email = data.optString("email").takeIf { it.isNotBlank() },
                displayName = data.optString("name").takeIf { it.isNotBlank() }
                    ?: data.optString("label").takeIf { it.isNotBlank() },
            )
        }.getOrElse { AccountInfo(id, null, null) }
    }
}
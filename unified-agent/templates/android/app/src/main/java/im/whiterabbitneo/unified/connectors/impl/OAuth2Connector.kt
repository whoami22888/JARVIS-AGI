package im.whiterabbitneo.unified.connectors.impl

import android.content.Context
import im.whiterabbitneo.unified.connectors.AccountInfo
import im.whiterabbitneo.unified.connectors.AuthToken
import im.whiterabbitneo.unified.connectors.AuthType
import im.whiterabbitneo.unified.connectors.Category
import im.whiterabbitneo.unified.connectors.ConnectorDefinition
import im.whiterabbitneo.unified.connectors.HttpJson
import im.whiterabbitneo.unified.connectors.Oauth2Math
import im.whiterabbitneo.unified.connectors.ServiceConnector

class OAuth2Connector(private val definition: ConnectorDefinition) : ServiceConnector {
    private class PendingAuthorization(val state: String, val verifier: String)

    private var pending: PendingAuthorization? = null

    override val id: String = definition.id
    override val name: String = definition.name
    override val category: Category = definition.category
    override val authType: AuthType = AuthType.OAUTH2
    override val requiredScopes: List<String> = definition.scopes

    override fun authorizationUrl(clientId: String, redirectUri: String): String {
        val profile = definition.oauth ?: error("$name has no OAuth2 profile")
        val state = Oauth2Math.randomUrlSafe(16)
        val verifier = Oauth2Math.randomUrlSafe(64)
        pending = PendingAuthorization(state, verifier)
        return Oauth2Math.buildAuthorizeUrl(profile, clientId, redirectUri, state, Oauth2Math.codeChallenge(verifier))
    }

    override suspend fun completeAuthorization(
        context: Context,
        redirectUri: String,
        code: String,
        clientId: String,
        clientSecret: String?,
    ): Result<AuthToken> = runCatching {
        val profile = definition.oauth ?: error("$name has no OAuth2 profile")
        val inFlight = pending ?: error("No authorization in progress for $name")
        val state = redirectUri.substringAfter("state=", "").substringBefore("&").substringBefore("#")
        require(state == inFlight.state) { "OAuth2 state mismatch for $name" }
        val form = buildMap<String, String> {
            put("grant_type", "authorization_code")
            put("code", code)
            put("redirect_uri", redirectUri.substringBefore("?").substringBefore("#"))
            put("client_id", clientId)
            put("code_verifier", inFlight.verifier)
            if (!clientSecret.isNullOrBlank()) put("client_secret", clientSecret)
            if (!profile.tokenAuthInBody && !clientSecret.isNullOrBlank()) {
                put("client_id", clientId)
                put("client_secret", clientSecret)
            }
        }
        val json = HttpJson.postForm(profile.tokenUrl, form, accessToken = null)
        HttpJson.parseTokenResponse(json, id)
    }

    override suspend fun refreshToken(token: AuthToken, clientId: String?, clientSecret: String?): Result<AuthToken> = runCatching {
        val profile = definition.oauth ?: error("$name has no OAuth2 profile")
        val refresh = token.refreshToken ?: error("$name token cannot be refreshed")
        val form = buildMap<String, String> {
            put("grant_type", "refresh_token")
            put("refresh_token", refresh)
            if (!clientId.isNullOrBlank()) put("client_id", clientId)
            if (!clientSecret.isNullOrBlank()) put("client_secret", clientSecret)
        }
        val json = HttpJson.postForm(profile.tokenUrl, form, accessToken = null)
        val refreshed = HttpJson.parseTokenResponse(json, id)
        refreshed.copy(refreshToken = refreshed.refreshToken ?: refresh)
    }

    override suspend fun revokeAccess(token: AuthToken): Result<Unit> = runCatching {
        val profile = definition.oauth ?: error("$name has no OAuth2 profile")
        val revokeUrl = profile.revokeUrl ?: error("$name does not support revocation")
        HttpJson.postForm(revokeUrl, mapOf("token" to token.accessToken), accessToken = null)
        Unit
    }

    override suspend fun getAccountInfo(token: AuthToken): AccountInfo {
        val profile = definition.oauth ?: return AccountInfo(id, null, null)
        val url = profile.userInfoUrl ?: return AccountInfo(id, null, null)
        return runCatching {
            val json = HttpJson.getJson(url, token.accessToken)
            AccountInfo(
                connectorId = id,
                email = json.optString("email").takeIf { it.isNotBlank() },
                displayName = json.optString("name").takeIf { it.isNotBlank() }
                    ?: json.optString("display_name").takeIf { it.isNotBlank() }
                    ?: json.optString("preferred_username").takeIf { it.isNotBlank() }
                    ?: json.optString("login").takeIf { it.isNotBlank() },
                details = mapOf("user_id" to json.optString("id", "")),
            )
        }.getOrElse { AccountInfo(id, null, null) }
    }
}
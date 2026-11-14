package im.whiterabbitneo.unified.connectors

import android.content.Context

enum class Category { AI, CLOUD_STORAGE, EMAIL, CALENDAR, SOCIAL, PRODUCTIVITY, DEVICE_CONTROL, BROWSER }

enum class AuthType { OAUTH2, API_KEY, SESSION_COOKIE, DEVICE_CODE, BIOMETRIC, NONE }

enum class ConnectorStatus { NOT_CONNECTED, CONNECTED, EXPIRED, ERROR }

data class AuthToken(
    val connectorId: String,
    val accessToken: String,
    val refreshToken: String?,
    val tokenType: String,
    val expiresAtEpochSeconds: Long?,
    val scopes: List<String>,
) {
    fun isExpired(nowEpochSeconds: Long = System.currentTimeMillis() / 1000): Boolean =
        expiresAtEpochSeconds != null && nowEpochSeconds >= expiresAtEpochSeconds - 60
}

data class AccountInfo(
    val connectorId: String,
    val email: String?,
    val displayName: String?,
    val details: Map<String, String> = emptyMap(),
)

interface ServiceConnector {
    val id: String
    val name: String
    val category: Category
    val authType: AuthType
    val requiredScopes: List<String>

    fun authorizationUrl(clientId: String, redirectUri: String): String? = null

    suspend fun completeAuthorization(
        context: Context,
        redirectUri: String,
        code: String,
        clientId: String,
        clientSecret: String?,
    ): Result<AuthToken> = Result.failure(UnsupportedOperationException("OAuth2 flow not supported"))

    suspend fun authenticate(context: Context, secret: String? = null): Result<AuthToken>

    suspend fun refreshToken(token: AuthToken, clientId: String?, clientSecret: String?): Result<AuthToken> =
        Result.failure(UnsupportedOperationException("Token refresh not supported"))

    suspend fun revokeAccess(token: AuthToken): Result<Unit> =
        Result.failure(UnsupportedOperationException("Revocation not supported"))

    suspend fun getAccountInfo(token: AuthToken): AccountInfo
}
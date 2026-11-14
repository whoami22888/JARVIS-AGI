package im.whiterabbitneo.unified.connectors

import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64

object Oauth2Math {
    fun randomUrlSafe(byteCount: Int = 64): String {
        val bytes = ByteArray(byteCount)
        SecureRandom().nextBytes(bytes)
        return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes)
    }

    fun codeChallenge(verifier: String): String {
        val digest = MessageDigest.getInstance("SHA-256").digest(verifier.toByteArray())
        return Base64.getUrlEncoder().withoutPadding().encodeToString(digest)
    }

    fun buildAuthorizeUrl(profile: OAuth2Profile, clientId: String, redirectUri: String, state: String, challenge: String): String {
        val scopes = profile.defaultScopes.joinToString(" ")
        return profile.authorizeUrl +
            "?response_type=code" +
            "&client_id=" + enc(clientId) +
            "&redirect_uri=" + enc(redirectUri) +
            "&scope=" + enc(scopes) +
            "&state=" + enc(state) +
            "&code_challenge=" + enc(challenge) +
            "&code_challenge_method=S256"
    }

    private fun enc(value: String): String = java.net.URLEncoder.encode(value, "UTF-8")
}
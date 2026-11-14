package im.whiterabbitneo.unified.connectors

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class Oauth2MathTest {
    @Test
    fun codeChallengeIsDeterministicForSameVerifier() {
        val first = Oauth2Math.codeChallenge("test-verifier-value")
        val second = Oauth2Math.codeChallenge("test-verifier-value")
        assertEquals(first, second)
    }

    @Test
    fun codeChallengeDiffersAcrossVerifiers() {
        assertFalse(Oauth2Math.codeChallenge("verifier-a") == Oauth2Math.codeChallenge("verifier-b"))
    }

    @Test
    fun randomValuesAreUnique() {
        val values = (1..64).map { Oauth2Math.randomUrlSafe() }
        assertEquals(values.size, values.toSet().size)
        values.forEach { assertTrue(it.isNotBlank()) }
    }

    @Test
    fun authorizeUrlContainsPkceParameters() {
        val profile = OAuth2Profile(
            authorizeUrl = "https://example.com/authorize",
            tokenUrl = "https://example.com/token",
            revokeUrl = null,
            userInfoUrl = null,
            defaultScopes = listOf("openid", "email"),
        )
        val url = Oauth2Math.buildAuthorizeUrl(profile, "client-123", "http://127.0.0.1:48731/oauth", "state-abc", "challenge-xyz")
        assertTrue(url.startsWith("https://example.com/authorize?"))
        assertTrue(url.contains("code_challenge_method=S256"))
        assertTrue(url.contains("response_type=code"))
        assertTrue(url.contains("client_id=client-123"))
        assertTrue(url.contains("state=state-abc"))
        assertTrue(url.contains("scope="))
    }
}
package im.whiterabbitneo.unified.connectors

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ConnectorCatalogTest {
    @Test
    fun catalogContainsPlannedConnectors() {
        val ids = ConnectorCatalog.connectors.map { it.id }.toSet()
        listOf(
            "openrouter", "deepinfra", "huggingface", "gemini", "anthropic", "openai", "groq", "ollama-local",
            "google-drive", "google-calendar", "gmail", "microsoft-graph", "github", "notion", "dropbox",
            "telegram", "whatsapp-business", "discord", "slack", "spotify", "home-assistant",
        ).forEach { assertTrue("Missing connector: $it", it in ids) }
    }

    @Test
    fun connectorIdsAreUnique() {
        val ids = ConnectorCatalog.connectors.map { it.id }
        assertEquals(ids.size, ids.toSet().size)
    }

    @Test
    fun oauthConnectorsDefineProfiles() {
        ConnectorCatalog.connectors.filter { it.authType == AuthType.OAUTH2 }.forEach { definition ->
            assertNotNull("OAuth2 connector ${definition.id} missing profile", definition.oauth)
            assertTrue("OAuth2 connector ${definition.id} missing authorize URL", definition.oauth!!.authorizeUrl.startsWith("https://"))
            assertTrue("OAuth2 connector ${definition.id} missing token URL", definition.oauth!!.tokenUrl.startsWith("https://"))
        }
    }

    @Test
    fun apiKeyConnectorsProvideHints() {
        ConnectorCatalog.connectors.filter { it.authType == AuthType.API_KEY }.forEach { definition ->
            assertNotNull("API key connector ${definition.id} missing hint", definition.apiKeyHint)
        }
    }

    @Test
    fun categoriesCoverPlan() {
        val categories = ConnectorCatalog.connectors.map { it.category }.toSet()
        assertTrue(Category.AI in categories)
        assertTrue(Category.CLOUD_STORAGE in categories)
        assertTrue(Category.SOCIAL in categories)
    }

    @Test
    fun tokenExpiryLogic() {
        val expired = AuthToken("x", "a", null, "Bearer", 1L, emptyList())
        val active = AuthToken("x", "a", null, "Bearer", Long.MAX_VALUE, emptyList())
        val never = AuthToken("x", "a", null, "Bearer", null, emptyList())
        assertTrue(expired.isExpired())
        assertFalse(active.isExpired())
        assertFalse(never.isExpired())
    }

    @Test
    fun byIdResolvesKnownConnector() {
        val github = ConnectorCatalog.byId("github")
        assertNotNull(github)
        assertEquals("GitHub", github!!.name)
        assertNull(ConnectorCatalog.byId("does-not-exist"))
    }
}
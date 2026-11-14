package im.whiterabbitneo.unified.connectors

import android.content.Context
import org.json.JSONObject

class ConnectorTokenVault(context: Context) {
    private val prefs = context.getSharedPreferences("wrn_connector_tokens", Context.MODE_PRIVATE)

    fun save(token: AuthToken) {
        val box = SecretBox("wrn_connector_${token.connectorId}")
        val json = JSONObject()
            .put("accessToken", box.encrypt(token.accessToken))
            .put("refreshToken", token.refreshToken?.let { box.encrypt(it) })
            .put("tokenType", token.tokenType)
            .put("expiresAt", token.expiresAtEpochSeconds ?: JSONObject.NULL)
            .put("scopes", org.json.JSONArray(token.scopes))
        prefs.edit().putString(token.connectorId, json.toString()).apply()
    }

    fun load(connectorId: String): AuthToken? {
        val raw = prefs.getString(connectorId, null) ?: return null
        return try {
            val json = JSONObject(raw)
            val box = SecretBox("wrn_connector_$connectorId")
            val refresh = if (json.isNull("refreshToken")) null else box.decrypt(json.getString("refreshToken"))
            AuthToken(
                connectorId = connectorId,
                accessToken = box.decrypt(json.getString("accessToken")) ?: return null,
                refreshToken = refresh,
                tokenType = json.optString("tokenType", "Bearer"),
                expiresAtEpochSeconds = if (json.isNull("expiresAt")) null else json.getLong("expiresAt"),
                scopes = json.optJSONArray("scopes")?.let { arr -> List(arr.length()) { arr.getString(it) } } ?: emptyList(),
            )
        } catch (_: Exception) {
            null
        }
    }

    fun clear(connectorId: String) {
        prefs.edit().remove(connectorId).apply()
    }

    fun clearAll() {
        prefs.edit().clear().apply()
    }

    fun connectedIds(): Set<String> = prefs.all.keys
}
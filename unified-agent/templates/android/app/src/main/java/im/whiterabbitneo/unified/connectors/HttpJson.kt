package im.whiterabbitneo.unified.connectors

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

object HttpJson {
    suspend fun getJson(url: String, accessToken: String?, accept: String = "application/json"): JSONObject =
        withContext(Dispatchers.IO) { execute(url, "GET", null, accessToken, accept, emptyMap()) }

    suspend fun postForm(url: String, form: Map<String, String>, accessToken: String?, accept: String = "application/json"): JSONObject =
        withContext(Dispatchers.IO) { execute(url, "POST", form, accessToken, accept, emptyMap()) }

    suspend fun postJson(url: String, body: JSONObject, accessToken: String?, accept: String = "application/json"): JSONObject =
        withContext(Dispatchers.IO) { execute(url, "POST", null, accessToken, accept, mapOf("Content-Type" to "application/json"), body) }

    suspend fun delete(url: String, accessToken: String?, accept: String = "application/json"): JSONObject =
        withContext(Dispatchers.IO) { execute(url, "DELETE", null, accessToken, accept, emptyMap()) }

    private fun execute(
        url: String,
        method: String,
        form: Map<String, String>?,
        accessToken: String?,
        accept: String,
        extraHeaders: Map<String, String>,
        jsonBody: JSONObject? = null,
    ): JSONObject {
        require(url.startsWith("https://")) { "Connector endpoint must use HTTPS: $url" }
        val connection = (URL(url).openConnection() as HttpURLConnection).apply {
            requestMethod = method
            connectTimeout = 15000
            readTimeout = 60000
            setRequestProperty("Accept", accept)
            accessToken?.let { setRequestProperty("Authorization", "Bearer $it") }
            extraHeaders.forEach { (key, value) -> setRequestProperty(key, value) }
            if (form != null) {
                setRequestProperty("Content-Type", "application/x-www-form-urlencoded")
                doOutput = true
            } else if (jsonBody != null) {
                doOutput = true
            }
        }
        if (form != null) {
            val body = form.entries.joinToString("&") { (k, v) -> enc(k) + "=" + enc(v) }
            connection.outputStream.use { it.write(body.toByteArray()) }
        } else if (jsonBody != null) {
            connection.outputStream.use { it.write(jsonBody.toString().toByteArray()) }
        }
        val stream = if (connection.responseCode in 200..299) connection.inputStream else connection.errorStream
        val text = stream.bufferedReader().use { it.readText() }
        if (connection.responseCode !in 200..299) {
            error("HTTP ${connection.responseCode}: ${text.take(400)}")
        }
        return if (text.isBlank()) JSONObject() else JSONObject(text)
    }

    fun parseTokenResponse(json: JSONObject, connectorId: String): AuthToken {
        val accessToken = json.optString("access_token").ifBlank { json.optString("token") }
        require(accessToken.isNotBlank()) { "Token response missing access token" }
        val scopes = json.optString("scope").takeIf { it.isNotBlank() }?.split(" ") ?: emptyList()
        return AuthToken(
            connectorId = connectorId,
            accessToken = accessToken,
            refreshToken = json.optString("refresh_token").takeIf { it.isNotBlank() },
            tokenType = json.optString("token_type", "Bearer"),
            expiresAtEpochSeconds = if (json.has("expires_in")) System.currentTimeMillis() / 1000 + json.getLong("expires_in") else null,
            scopes = scopes,
        )
    }

    private fun enc(value: String): String = java.net.URLEncoder.encode(value, "UTF-8")
}
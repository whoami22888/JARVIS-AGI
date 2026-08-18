package im.whiterabbitneo.unified.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

data class Pairing(val deviceId: String, val token: String)
data class TaskResult(val status: String, val answer: String, val route: String?)

class GatewayClient {
    private suspend fun post(baseUrl: String, path: String, body: JSONObject, token: String? = null): JSONObject = withContext(Dispatchers.IO) {
        require(baseUrl.startsWith("https://")) { "Gateway URL must use HTTPS." }
        val connection = (URL(baseUrl.trimEnd('/') + path).openConnection() as HttpURLConnection).apply {
            requestMethod = "POST"; connectTimeout = 15000; readTimeout = 60000
            setRequestProperty("Content-Type", "application/json")
            token?.let { setRequestProperty("Authorization", "Bearer $it") }
            doOutput = true
        }
        connection.outputStream.use { it.write(body.toString().toByteArray()) }
        val stream = if (connection.responseCode in 200..299) connection.inputStream else connection.errorStream
        val result = stream.bufferedReader().use { it.readText() }
        if (connection.responseCode !in 200..299) error(JSONObject(result).optString("detail", "Gateway request failed"))
        JSONObject(result)
    }
    suspend fun pair(url: String, deviceName: String, code: String): Pairing {
        val response = post(url, "/v1/pair", JSONObject().put("device_name", deviceName).put("pairing_code", code))
        return Pairing(response.getString("device_id"), response.getString("token"))
    }
    suspend fun task(url: String, token: String, prompt: String, enabled: Set<String>): TaskResult {
        val response = post(url, "/v1/tasks", JSONObject().put("prompt", prompt).put("model", "deepseek-reasoner").put("enabled_capabilities", JSONArray(enabled.toList())), token)
        return TaskResult(response.getString("status"), response.optString("answer", ""), response.optString("route").takeIf { it.isNotBlank() })
    }
    suspend fun createRegistryItem(url: String, token: String, kind: String, name: String, definition: JSONObject) {
        post(url, "/v1/registry", JSONObject().put("kind", kind).put("name", name).put("definition", definition), token)
    }
}

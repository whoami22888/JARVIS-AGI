package im.whiterabbitneo.unified.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

data class Pairing(val deviceId: String, val token: String)
data class TaskResult(val status: String, val answer: String, val route: String?)
data class GatewayHealth(
    val status: String,
    val localOllamaEnabled: Boolean,
    val localOllamaModel: String,
    val authorizedCloudFallbackEnabled: Boolean,
    val cloudModel: String,
)

class GatewayClient {
    private suspend fun request(baseUrl: String, path: String, method: String, body: JSONObject? = null, token: String? = null): JSONObject = withContext(Dispatchers.IO) {
        require(baseUrl.startsWith("https://")) { "Gateway URL must use HTTPS." }
        val connection = (URL(baseUrl.trimEnd('/') + path).openConnection() as HttpURLConnection).apply {
            requestMethod = method
            connectTimeout = 15000
            readTimeout = 60000
            setRequestProperty("Accept", "application/json")
            if (body != null) {
                setRequestProperty("Content-Type", "application/json")
                doOutput = true
            }
            token?.let { setRequestProperty("Authorization", "Bearer $it") }
        }
        body?.let { payload -> connection.outputStream.use { it.write(payload.toString().toByteArray()) } }
        val stream = if (connection.responseCode in 200..299) connection.inputStream else connection.errorStream
        val result = stream.bufferedReader().use { it.readText() }
        if (connection.responseCode !in 200..299) error(JSONObject(result).optString("detail", "Gateway request failed"))
        JSONObject(result)
    }

    suspend fun health(url: String): GatewayHealth {
        val response = request(url, "/health", "GET")
        return GatewayHealth(
            status = response.optString("status", "unavailable"),
            localOllamaEnabled = response.optBoolean("local_ollama_enabled", false),
            localOllamaModel = response.optString("local_ollama_model", ""),
            authorizedCloudFallbackEnabled = response.optBoolean("authorized_cloud_fallback_enabled", false),
            cloudModel = response.optString("cloud_model", ""),
        )
    }

    suspend fun pair(url: String, deviceName: String, code: String): Pairing {
        val response = request(url, "/v1/pair", "POST", JSONObject().put("device_name", deviceName).put("pairing_code", code))
        return Pairing(response.getString("device_id"), response.getString("token"))
    }

    suspend fun task(url: String, token: String, prompt: String, enabled: Set<String>): TaskResult {
        val response = request(url, "/v1/tasks", "POST", JSONObject().put("prompt", prompt).put("model", "deepseek-reasoner").put("enabled_capabilities", JSONArray(enabled.toList())), token)
        return TaskResult(response.getString("status"), response.optString("answer", ""), response.optString("route").takeIf { it.isNotBlank() })
    }

    suspend fun createRegistryItem(url: String, token: String, kind: String, name: String, definition: JSONObject) {
        request(url, "/v1/registry", "POST", JSONObject().put("kind", kind).put("name", name).put("definition", definition), token)
    }

    fun taskEvents(url: String, token: String, prompt: String, enabled: Set<String>): Flow<String> = callbackFlow {
        require(url.startsWith("https://")) { "Gateway URL must use HTTPS." }
        val connection = (URL(url.trimEnd('/') + "/v1/tasks").openConnection() as HttpURLConnection).apply {
            requestMethod = "POST"
            connectTimeout = 15000
            readTimeout = 0
            setRequestProperty("Accept", "text/event-stream")
            setRequestProperty("Content-Type", "application/json")
            setRequestProperty("Authorization", "Bearer $token")
            doOutput = true
        }
        val payload = JSONObject()
            .put("prompt", prompt)
            .put("model", "deepseek-reasoner")
            .put("stream", true)
            .put("enabled_capabilities", JSONArray(enabled.toList()))
        try {
            connection.outputStream.use { it.write(payload.toString().toByteArray()) }
            if (connection.responseCode !in 200..299) {
                val detail = connection.errorStream.bufferedReader().use { it.readText() }
                close(IllegalStateException(JSONObject(detail).optString("detail", "Gateway stream failed")))
            } else {
                connection.inputStream.bufferedReader().forEachLine { line ->
                    if (line.startsWith("data:")) {
                        val data = line.removePrefix("data:").trim()
                        if (data.isNotBlank() && data != "[DONE]") trySend(data)
                    }
                }
            }
            close()
        } catch (e: Exception) {
            close(e)
        }
        awaitClose { connection.disconnect() }
    }.flowOn(Dispatchers.IO)
}

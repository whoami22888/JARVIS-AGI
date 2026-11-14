package im.whiterabbitneo.unified.connectors.impl

import java.io.BufferedReader
import java.net.ServerSocket
import java.util.concurrent.atomic.AtomicBoolean

class LoopbackOAuthServer(private val port: Int = 48731) {
    private val running = AtomicBoolean(false)
    private var server: ServerSocket? = null
    private var callback: ((code: String, state: String?) -> Unit)? = null
    private var thread: Thread? = null

    val redirectUri: String
        get() = "http://127.0.0.1:$port/oauth"

    fun start(redirectPath: String, onCode: (code: String, state: String?) -> Unit): String {
        stop()
        callback = onCode
        running.set(true)
        server = ServerSocket(port).apply { reuseAddress = true }
        thread = Thread {
            try {
                server?.accept()?.use { socket ->
                    val reader = BufferedReader(socket.getInputStream().read(charset = Charsets.UTF_8))
                    val requestLine = reader.readLine() ?: return@Thread
                    if (!requestLine.startsWith("GET")) {
                        respond(socket, 405, "Method Not Allowed")
                        return@Thread
                    }
                    val target = requestLine.split(" ").getOrNull(1) ?: ""
                    val query = target.substringAfter('?', "")
                    val params = parseQuery(query)
                    val code = params["code"]
                    val state = params["state"]
                    val error = params["error"]
                    if (error != null || code == null) {
                        respond(socket, 400, if (error != null) "Authorization failed: $error" else "Missing authorization code")
                    } else {
                        respond(socket, 200, "Authorized. You can close this tab and return to WhiteRabbit Neo.")
                        callback?.invoke(code, state)
                    }
                }
            } catch (_: Exception) {
            } finally {
                running.set(false)
            }
        }.apply { isDaemon = true; start() }
        return redirectUri
    }

    fun stop() {
        running.set(false)
        try {
            server?.close()
        } catch (_: Exception) {
        }
        server = null
        thread?.interrupt()
        thread = null
    }

    private fun respond(socket: java.net.Socket, status: Int, message: String) {
        val body = "<html><body style='font-family:sans-serif;background:#07110D;color:#9EFFC8;padding:48px'>$message</body></html>"
        val headers = "HTTP/1.1 $status OK\r\nContent-Type: text/html\r\nContent-Length: ${body.toByteArray().size}\r\nConnection: close\r\n\r\n"
        socket.getOutputStream().write((headers + body).toByteArray())
    }

    private fun parseQuery(query: String): Map<String, String> =
        query.split("&").mapNotNull { pair ->
            val parts = pair.split("=", limit = 2)
            if (parts.size == 2) parts[0] to java.net.URLDecoder.decode(parts[1], "UTF-8") else null
        }.toMap()
}
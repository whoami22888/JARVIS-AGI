package im.whiterabbitneo.unified.data

import java.net.URI
import java.net.URISyntaxException
import java.util.Locale

/** Permit HTTPS, or strictly the handset's own loopback with an explicit port. */
object GatewayUrlPolicy {
    fun normalize(candidate: String): String? {
        val raw = candidate.trim()
        if (raw.isEmpty() || raw.any { it.isWhitespace() }) return null
        val uri = try { URI(raw) } catch (_: URISyntaxException) { return null }
        val scheme = uri.scheme?.lowercase(Locale.ROOT) ?: return null
        val host = uri.host?.lowercase(Locale.ROOT) ?: return null
        if (uri.rawUserInfo != null || uri.rawQuery != null || uri.rawFragment != null) return null
        if (!uri.rawPath.isNullOrEmpty() && uri.rawPath != "/") return null
        if (uri.port == 0 || uri.port > 65535 || uri.port < -1) return null
        if (scheme != "https" && scheme != "http") return null
        if (scheme == "http" && (host != "127.0.0.1" || uri.port == -1)) return null
        return if (uri.port == -1) "$scheme://$host" else "$scheme://$host:${uri.port}"
    }
}

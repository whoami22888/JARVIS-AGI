package im.whiterabbitneo.unified.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class GatewayUrlPolicyTest {
    @Test fun secureRemoteGatewayAccepted() {
        assertEquals("https://example.com:8443", GatewayUrlPolicy.normalize("HTTPS://EXAMPLE.COM:8443/"))
    }
    @Test fun phoneLoopbackAccepted() {
        assertEquals("http://127.0.0.1:8080", GatewayUrlPolicy.normalize("http://127.0.0.1:8080"))
    }
    @Test fun unsafeUrlsRejected() {
        listOf("http://example.com:8080", "http://192.168.0.1:8080",
            "http://localhost:8080", "http://127.0.0.1", "http://127.0.0.1:0",
            "http://127.0.0.1:99999", "https://user:pass@example.com",
            "https://example.com/path", "https://example.com?token=x",
            "https://example.com/#frag", "file:///etc/passwd", "").forEach {
                assertNull(it, GatewayUrlPolicy.normalize(it))
            }
    }
}

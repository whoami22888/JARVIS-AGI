package im.whiterabbitneo.unified.connectors

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow

object OAuthRedirectBus {
    private val events = MutableSharedFlow<android.net.Uri>(extraBufferCapacity = 8)

    fun deliver(uri: android.net.Uri) {
        events.tryEmit(uri)
    }

    fun observe(): SharedFlow<android.net.Uri> = events.asSharedFlow()
}
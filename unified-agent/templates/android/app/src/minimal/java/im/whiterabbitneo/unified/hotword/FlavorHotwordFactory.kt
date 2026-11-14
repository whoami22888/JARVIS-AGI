package im.whiterabbitneo.unified.hotword

import android.content.Context

class NoopHotwordEngine(override val unavailableReason: String? = "Hotword detection is only available in the full flavor") : HotwordEngine {
    override val keyword: String = "jarvis"

    override fun start(onDetection: () -> Unit): Boolean = false
    override fun stop() = Unit
    override fun release() = Unit
}

fun flavorHotwordEngine(context: Context): HotwordEngine = NoopHotwordEngine()
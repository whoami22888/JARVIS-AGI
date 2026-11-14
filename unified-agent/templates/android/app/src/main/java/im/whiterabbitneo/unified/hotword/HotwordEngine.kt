package im.whiterabbitneo.unified.hotword

import android.content.Context

interface HotwordEngine {
    val keyword: String
    val unavailableReason: String?
        get() = null

    fun start(onDetection: () -> Unit): Boolean
    fun stop()
    fun release()
}

fun createHotwordEngine(context: Context): HotwordEngine = flavorHotwordEngine(context)
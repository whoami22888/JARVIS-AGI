package im.whiterabbitneo.unified.hotword

import android.content.Context

fun flavorHotwordEngine(context: Context): HotwordEngine = PorcupineHotwordEngine(context)
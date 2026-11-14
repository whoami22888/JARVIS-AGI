package im.whiterabbitneo.unified.device

import android.content.Context

object ApprovalGate {
    private const val PREFS = "wrn_approval"
    private const val KEY_GRANTED = "granted"

    @Volatile
    var isGranted: Boolean = false
        private set

    fun load(context: Context) {
        isGranted = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getBoolean(KEY_GRANTED, false)
    }

    fun set(context: Context, granted: Boolean) {
        isGranted = granted
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().putBoolean(KEY_GRANTED, granted).apply()
    }
}
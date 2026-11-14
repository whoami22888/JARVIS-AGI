package im.whiterabbitneo.unified.device

import android.accessibilityservice.AccessibilityService
import android.view.accessibility.AccessibilityEvent

class WhiteRabbitAccessibilityService : AccessibilityService() {
    var lastPackage: String? = null
        private set

    val isApprovalGranted: Boolean
        get() = ApprovalGate.isGranted

    override fun onServiceConnected() {
        super.onServiceConnected()
        ApprovalGate.load(this)
        instance = this
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        if (event?.eventType == AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED) {
            lastPackage = event.packageName?.toString()
        }
    }

    override fun onInterrupt() = Unit

    override fun onDestroy() {
        instance = null
        super.onDestroy()
    }

    companion object {
        @Volatile
        var instance: WhiteRabbitAccessibilityService? = null
            private set
    }
}
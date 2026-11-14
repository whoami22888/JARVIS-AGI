package im.whiterabbitneo.unified.browser

import android.accessibilityservice.AccessibilityService
import android.view.accessibility.AccessibilityNodeInfo
import im.whiterabbitneo.unified.device.WhiteRabbitAccessibilityService

class BrowserAutomationController(private val service: AccessibilityService) {
    private val approved: Boolean
        get() = service is WhiteRabbitAccessibilityService && service.isApprovalGranted

    val currentPackage: String?
        get() = service.rootInActiveWindow?.packageName?.toString()

    fun currentUrl(): String? = searchNodes { node ->
        val viewId = node.viewIdResourceName.orEmpty()
        val text = node.text?.toString().orEmpty()
        viewId.contains("url_bar") || viewId.contains("address_bar") ||
            viewId.contains("location_bar") || text.startsWith("http://") || text.startsWith("https://")
    }?.text?.toString()

    fun visibleText(): List<String> {
        val texts = mutableListOf<String>()
        searchAll { node ->
            node.text?.toString()?.takeIf { it.isNotBlank() }?.let { texts += it }
            node.contentDescription?.toString()?.takeIf { it.isNotBlank() }?.let { texts += it }
            false
        }
        return texts
    }

    fun clickByText(text: String): Boolean = requireApproval { findNodesByText(text).firstOrNull()?.performAction(AccessibilityNodeInfo.ACTION_CLICK) == true }

    fun clickByViewId(viewId: String): Boolean = requireApproval {
        service.rootInActiveWindow?.findAccessibilityNodeInfosByViewId(viewId)?.firstOrNull()
            ?.performAction(AccessibilityNodeInfo.ACTION_CLICK) == true
    }

    fun clickByDescription(description: String): Boolean = requireApproval {
        searchNodes { node ->
            node.contentDescription?.toString().orEmpty().contains(description, ignoreCase = true)
        }?.performAction(AccessibilityNodeInfo.ACTION_CLICK) == true
    }

    fun setText(viewId: String, value: String): Boolean = requireApproval {
        val node = service.rootInActiveWindow?.findAccessibilityNodeInfosByViewId(viewId)?.firstOrNull()
            ?: return@requireApproval false
        val args = android.os.Bundle().apply {
            putCharSequence(AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE, value)
        }
        node.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, args)
    }

    fun scroll(forward: Boolean): Boolean = requireApproval {
        service.performGlobalAction(
            if (forward) AccessibilityService.GLOBAL_ACTION_SCROLL_FORWARD else AccessibilityService.GLOBAL_ACTION_SCROLL_BACKWARD
        )
    }

    fun goBack(): Boolean = requireApproval { service.performGlobalAction(android.accessibilityservice.AccessibilityService.GLOBAL_ACTION_BACK) }

    fun goHome(): Boolean = requireApproval { service.performGlobalAction(android.accessibilityservice.AccessibilityService.GLOBAL_ACTION_HOME) }

    fun recentApps(): Boolean = requireApproval { service.performGlobalAction(android.accessibilityservice.AccessibilityService.GLOBAL_ACTION_RECENTS) }

    private inline fun requireApproval(action: () -> Boolean): Boolean = if (approved) action() else false

    private inline fun findNodesByText(text: String): List<AccessibilityNodeInfo> {
        val root = service.rootInActiveWindow ?: return emptyList()
        return root.findAccessibilityNodeInfosByText(text)
    }

    private inline fun searchNodes(predicate: (AccessibilityNodeInfo) -> Boolean): AccessibilityNodeInfo? {
        val root = service.rootInActiveWindow ?: return null
        val queue = ArrayDeque<AccessibilityNodeInfo>()
        queue.addLast(root)
        val visited = HashSet<AccessibilityNodeInfo>()
        while (queue.isNotEmpty()) {
            val node = queue.removeFirst()
            if (!visited.add(node)) continue
            if (predicate(node)) return node
            for (index in 0 until node.childCount) {
                node.getChild(index)?.let { queue.addLast(it) }
            }
        }
        return null
    }

    private inline fun searchAll(predicate: (AccessibilityNodeInfo) -> Boolean): Boolean {
        val root = service.rootInActiveWindow ?: return false
        val queue = ArrayDeque<AccessibilityNodeInfo>()
        queue.addLast(root)
        val visited = HashSet<AccessibilityNodeInfo>()
        while (queue.isNotEmpty()) {
            val node = queue.removeFirst()
            if (!visited.add(node)) continue
            if (predicate(node)) return true
            for (index in 0 until node.childCount) {
                node.getChild(index)?.let { queue.addLast(it) }
            }
        }
        return false
    }
}
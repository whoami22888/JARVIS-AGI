package im.whiterabbitneo.unified.admin

import android.app.admin.DeviceAdminReceiver
import android.app.admin.DevicePolicyManager
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.widget.Toast
import im.whiterabbitneo.unified.R

class DeviceAdminReceiver : DeviceAdminReceiver() {
    override fun onEnabled(context: Context, intent: Intent) {
        Toast.makeText(context, "WhiteRabbit Neo device admin enabled", Toast.LENGTH_SHORT).show()
    }

    override fun onDisabled(context: Context, intent: Intent) {
        Toast.makeText(context, "WhiteRabbit Neo device admin disabled", Toast.LENGTH_SHORT).show()
    }
}

class DeviceAdminManager(context: Context) {
    private val policy = context.getSystemService(Context.DEVICE_POLICY_SERVICE) as DevicePolicyManager
    private val component = ComponentName(context, DeviceAdminReceiver::class.java)

    fun isAdminActive(): Boolean = policy.isAdminActive(component)

    fun requestAdmin(context: Context) {
        val intent = Intent(DevicePolicyManager.ACTION_ADD_DEVICE_ADMIN).apply {
            putExtra(DevicePolicyManager.EXTRA_DEVICE_ADMIN, component)
            putExtra(DevicePolicyManager.EXTRA_ADD_EXPLANATION, context.getString(R.string.device_admin_description))
        }
        context.startActivity(intent)
    }

    fun lockScreen() {
        if (isAdminActive()) policy.lockNow()
    }

    fun setCameraDisabled(disabled: Boolean) {
        if (isAdminActive()) policy.setCameraDisabled(component, disabled)
    }

    fun wipeData() {
        if (isAdminActive()) policy.wipeData(0)
    }
}
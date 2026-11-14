package im.whiterabbitneo.unified.hotword

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import androidx.core.content.ContextCompat
import im.whiterabbitneo.unified.MainActivity
import im.whiterabbitneo.unified.R

class HotwordService : Service() {
    private var engine: HotwordEngine? = null

    override fun onCreate() {
        super.onCreate()
        createChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val wakeWord = createHotwordEngine(this)
        engine = wakeWord
        val notification = buildNotification()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_MICROPHONE)
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
        if (wakeWord.unavailableReason != null) {
            sendStatus(false, wakeWord.unavailableReason)
            stopSelf()
            return START_NOT_STICKY
        }
        val started = wakeWord.start {
            sendBroadcast(Intent(ACTION_HOTWORD_DETECTED).setPackage(packageName))
        }
        if (!started) {
            sendStatus(false, wakeWord.unavailableReason ?: "Failed to start hotword detection")
            stopSelf()
        }
        return START_STICKY
    }

    override fun onDestroy() {
        engine?.release()
        engine = null
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun createChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val channel = NotificationChannel(CHANNEL_ID, getString(R.string.hotword_channel_name), NotificationManager.IMPORTANCE_LOW)
        getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    private fun buildNotification(): Notification {
        val contentIntent = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            @Suppress("DEPRECATION")
            Notification.Builder(this)
        }
        return builder
            .setContentTitle(getString(R.string.hotword_running))
            .setSmallIcon(android.R.drawable.ic_btn_speak_now)
            .setContentIntent(contentIntent)
            .setOngoing(true)
            .build()
    }

    private fun sendStatus(listening: Boolean, detail: String?) {
        val intent = Intent(ACTION_HOTWORD_STATUS)
            .setPackage(packageName)
            .putExtra(EXTRA_LISTENING, listening)
            .putExtra(EXTRA_DETAIL, detail)
        sendBroadcast(intent)
    }

    companion object {
        const val ACTION_HOTWORD_STATUS = "im.whiterabbitneo.unified.HOTWORD_STATUS"
        const val ACTION_HOTWORD_DETECTED = "im.whiterabbitneo.unified.HOTWORD_DETECTED"
        const val EXTRA_LISTENING = "listening"
        const val EXTRA_DETAIL = "detail"
        private const val CHANNEL_ID = "hotword"
        private const val NOTIFICATION_ID = 4501

        fun start(context: Context) {
            val intent = Intent(context, HotwordService::class.java)
            ContextCompat.startForegroundService(context, intent)
        }

        fun stop(context: Context) {
            context.stopService(Intent(context, HotwordService::class.java))
        }
    }
}
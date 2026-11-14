package im.whiterabbitneo.unified.hotword

import android.content.Context
import android.content.RestrictionsManager
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.MediaRecorder
import com.picovoice.porcupine.Porcupine
import im.whiterabbitneo.unified.BuildConfig
import java.util.concurrent.atomic.AtomicBoolean

class PorcupineHotwordEngine(context: Context) : HotwordEngine {
    private val accessKey: String = (context.getSystemService(Context.RESTRICTIONS_SERVICE) as RestrictionsManager)
        .applicationRestrictions.getString("porcupine_access_key", "")
    private val running = AtomicBoolean(false)
    private var porcupine: Porcupine? = null
    private var record: AudioRecord? = null
    private var thread: Thread? = null

    override val keyword: String = "jarvis"
    override val unavailableReason: String?
        get() = if (accessKey.isBlank()) "Porcupine access key is not configured (set the porcupine_access_key restriction)" else null

    override fun start(onDetection: () -> Unit): Boolean {
        if (!BuildConfig.ON_DEVICE_HOTWORD) return false
        if (unavailableReason != null) return false
        stop()
        val engine = Porcupine.Builder().setAccessKey(accessKey).setKeyword(keyword).build()
        porcupine = engine
        val sampleRate = 16000
        val frameLength = engine.frameLength
        val minBuffer = AudioRecord.getMinBufferSize(
            sampleRate,
            AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
        )
        val audio = AudioRecord(
            MediaRecorder.AudioSource.VOICE_RECOGNITION,
            sampleRate,
            AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
            maxOf(minBuffer, frameLength * 2),
        )
        record = audio
        if (audio.state != AudioRecord.STATE_INITIALIZED) {
            release()
            return false
        }
        audio.startRecording()
        running.set(true)
        thread = Thread {
            val buffer = ShortArray(frameLength)
            while (running.get()) {
                val read = audio.read(buffer, 0, frameLength)
                if (read == frameLength && engine.process(buffer) >= 0) {
                    onDetection()
                }
            }
        }.apply { isDaemon = true; start() }
        return true
    }

    override fun stop() {
        running.set(false)
        thread?.interrupt()
        thread = null
        try {
            record?.stop()
        } catch (_: Exception) {
        }
        record?.release()
        record = null
        porcupine?.delete()
        porcupine = null
    }

    override fun release() {
        stop()
    }
}
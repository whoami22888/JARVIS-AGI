package im.whiterabbitneo.unified.voice

import android.content.Context
import android.media.AudioAttributes
import android.speech.tts.TextToSpeech
import android.speech.tts.UtteranceProgressListener
import java.util.Locale
import java.util.concurrent.atomic.AtomicBoolean

class TextToSpeechEngine(context: Context) {
    private val ready = AtomicBoolean(false)
    private var tts: TextToSpeech? = null

    fun initialize(onReady: () -> Unit = {}) {
        if (tts != null) return
        tts = TextToSpeech(context.applicationContext) { status ->
            if (status == TextToSpeech.SUCCESS) {
                ready.set(true)
                onReady()
            }
        }?.apply {
            setAudioAttributes(
                AudioAttributes.Builder()
                    .setUsage(AudioAttributes.USAGE_MEDIA)
                    .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
                    .build()
            )
            setOnUtteranceProgressListener(object : UtteranceProgressListener() {
                override fun onStart(utteranceId: String?) = Unit
                override fun onDone(utteranceId: String?) = Unit
                @Deprecated("Deprecated in Java")
                override fun onError(utteranceId: String?) = Unit
            })
        }
    }

    fun speak(text: String, onDone: (() -> Unit)? = null) {
        if (!ready.get()) return
        tts?.speak(text, TextToSpeech.QUEUE_FLUSH, null, "wrn-utterance-${System.currentTimeMillis()}")
    }

    fun stop() {
        tts?.stop()
    }

    fun shutdown() {
        tts?.shutdown()
        tts = null
        ready.set(false)
    }
}
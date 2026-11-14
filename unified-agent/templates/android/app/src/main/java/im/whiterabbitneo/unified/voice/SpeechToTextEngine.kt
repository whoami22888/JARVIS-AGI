package im.whiterabbitneo.unified.voice

import android.content.Context
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.withContext
import org.json.JSONObject
import org.vosk.Model
import org.vosk.Recognizer
import org.vosk.android.RecognitionListener
import org.vosk.android.SpeechStreamService
import java.io.File
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.util.zip.ZipInputStream

object VoskModelManager {
    private const val MODEL_DIR = "models"
    private const val MODEL_NAME = "vosk-model-small-en-us-0.15"
    private const val MODEL_URL = "https://alphacephei.com/vosk/models/vosk-model-small-en-us-0.15.zip"

    fun isModelPresent(context: Context): Boolean = File(context.filesDir, "$MODEL_DIR/$MODEL_NAME").isDirectory

    fun modelDirectory(context: Context): File = File(context.filesDir, "$MODEL_DIR/$MODEL_NAME")

    suspend fun ensureModel(context: Context, onProgress: (Float) -> Unit): String {
        val target = modelDirectory(context)
        if (target.isDirectory) return target.absolutePath
        val zipFile = File(context.filesDir, "$MODEL_DIR/model.zip")
        zipFile.parentFile?.mkdirs()
        download(MODEL_URL, zipFile, onProgress)
        unzip(zipFile, File(context.filesDir, MODEL_DIR))
        zipFile.delete()
        return target.absolutePath
    }

    private suspend fun download(url: String, target: File, onProgress: (Float) -> Unit) {
        withContext(Dispatchers.IO) {
            val connection = (URL(url).openConnection() as HttpURLConnection).apply {
                instanceFollowRedirects = true
                connectTimeout = 30000
                readTimeout = 120000
            }
            connection.inputStream.use { input ->
                val total = connection.contentLengthLong.takeIf { it > 0 } ?: -1L
                var written = 0L
                FileOutputStream(target).use { output ->
                    val buffer = ByteArray(1 shl 16)
                    while (true) {
                        val read = input.read(buffer)
                        if (read < 0) break
                        output.write(buffer, 0, read)
                        written += read
                        if (total > 0) onProgress(written.toFloat() / total)
                    }
                }
            }
        }
    }

    private fun unzip(zipFile: File, targetDir: File) {
        targetDir.mkdirs()
        ZipInputStream(zipFile.inputStream().buffered()).use { zip ->
            var entry = zip.nextEntry
            while (entry != null) {
                val outFile = File(targetDir, entry.name)
                if (entry.isDirectory) {
                    outFile.mkdirs()
                } else {
                    outFile.parentFile?.mkdirs()
                    FileOutputStream(outFile).use { zip.copyTo(it) }
                }
                zip.closeEntry()
                entry = zip.nextEntry
            }
        }
    }
}

class SpeechToTextEngine(context: Context) {
    private val sampleRate = 16000.0f
    private var recognizer: Recognizer? = null
    private var speechService: SpeechStreamService? = null

    fun start(modelPath: String, onPartial: (String) -> Unit, onFinal: (String) -> Unit, onError: (String) -> Unit) {
        stop()
        val model = Model(modelPath)
        val rec = Recognizer(model, sampleRate)
        recognizer = rec
        val service = SpeechStreamService(rec, sampleRate)
        speechService = service
        service.startListening(object : RecognitionListener {
            override fun onPartialResult(hypothesis: String?) {
                onPartial(hypothesis?.let { parseText(it) }.orEmpty())
            }

            override fun onResult(hypothesis: String?) {
                onFinal(hypothesis?.let { parseText(it) }.orEmpty())
            }

            override fun onFinalResult(hypothesis: String?) {
                onFinal(hypothesis?.let { parseText(it) }.orEmpty())
            }

            override fun onError(error: Exception?) {
                onError(error?.message ?: "Recognition failed")
            }

            override fun onTimeout() {
                onError("Recognition timed out")
            }
        })
    }

    fun stop() {
        speechService?.stop()
        speechService?.shutdown()
        speechService = null
        recognizer?.close()
        recognizer = null
    }

    private fun parseText(hypothesis: String): String =
        runCatching { JSONObject(hypothesis).optString("text") }.getOrDefault("")
}

fun speechTranscriptionFlow(engine: SpeechToTextEngine, modelPath: String): Flow<String> = callbackFlow {
    engine.start(
        modelPath = modelPath,
        onPartial = { trySend(it) },
        onFinal = { trySend(it) },
        onError = { close(IllegalStateException(it)) },
    )
    awaitClose { engine.stop() }
}.flowOn(Dispatchers.Default)
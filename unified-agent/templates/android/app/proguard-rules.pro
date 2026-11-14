# Vosk STT (JNI)
-keep class org.vosk.** { *; }
-keep class org.kaldi.** { *; }

# Porcupine hotword (full flavor)
-keep class com.picovoice.porcupine.** { *; }

# ML Kit vision
-keep class com.google.mlkit.** { *; }
-dontwarn com.google.mlkit.**

# Room (annotation processing verified at compile time; keep entities for reflection-free DAOs)
-keep class im.whiterabbitneo.unified.db.** { *; }

# Coroutines
-dontwarn kotlinx.coroutines.**

# Kotlin metadata
-keep class kotlin.Metadata { *; }
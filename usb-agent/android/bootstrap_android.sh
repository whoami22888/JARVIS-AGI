#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GRADLE_VERSION="8.9"
CACHE="${ROOT}/.bootstrap-cache"
mkdir -p "${CACHE}"

if [[ ! -x "${ROOT}/gradlew" ]]; then
  ARCHIVE="${CACHE}/gradle-${GRADLE_VERSION}-bin.zip"
  if [[ ! -f "${ARCHIVE}" ]]; then
    curl --fail --location --retry 3 "https://services.gradle.org/distributions/gradle-${GRADLE_VERSION}-bin.zip" -o "${ARCHIVE}"
  fi
  unzip -q -o "${ARCHIVE}" -d "${CACHE}"
  "${CACHE}/gradle-${GRADLE_VERSION}/bin/gradle" -p "${ROOT}" wrapper --gradle-version "${GRADLE_VERSION}"
fi

cat <<'EOF'
Gradle wrapper ready.

To build a debug APK, install Android SDK Platform 36 and Build-Tools 36.0.0, set ANDROID_HOME,
and review/accept the Android SDK licenses yourself, then run:
  ./gradlew :app:assembleDebug
EOF

#!/usr/bin/env bash
# Builds a POEM Android APK: the Go application (c-shared) + this C++ GLES
# presenter, packaged without Gradle (aapt2 + zipalign + apksigner).
#
# Usage:
#   build_apk.sh <app-go-package-dir> <package-id> <app-label> <out.apk> [abi]
# Example:
#   build_apk.sh ../examples/counter/android com.trellis.counter Counter counter.apk x86_64
#
# Requires: Android SDK (build-tools 36.1.0, platform android-36.1, NDK
# 30.0.15729638) at $ANDROID_SDK or %LOCALAPPDATA%/Android/Sdk, Go 1.26+, and
# a debug keystore at ~/.android/debug.keystore. JAVA_HOME defaults to
# Android Studio's JBR.
set -euo pipefail

APP_DIR="$1"
PACKAGE_ID="$2"
APP_LABEL="$3"
OUT_APK="$4"
ABI="${5:-x86_64}"

SDK="${ANDROID_SDK:-$LOCALAPPDATA/Android/Sdk}"
NDK="$SDK/ndk/30.0.15729638"
BIN="$NDK/toolchains/llvm/prebuilt/windows-x86_64/bin"
BT="$SDK/build-tools/36.1.0"
GLUE="$NDK/sources/android/native_app_glue"
export JAVA_HOME="${JAVA_HOME:-C:/Program Files/Android/Android Studio/jbr}"

case "$ABI" in
  x86_64) GOARCH=amd64; CC="$BIN/x86_64-linux-android26-clang.cmd"; CXX="$BIN/x86_64-linux-android26-clang++.cmd" ;;
  arm64-v8a) GOARCH=arm64; CC="$BIN/aarch64-linux-android26-clang.cmd"; CXX="$BIN/aarch64-linux-android26-clang++.cmd" ;;
  *) echo "unsupported abi $ABI"; exit 1 ;;
esac

ENGINE_DIR="$(cd "$(dirname "$0")" && pwd)"
POEM_DIR="$(cd "$ENGINE_DIR/.." && pwd)"
WORK="$ENGINE_DIR/build/$PACKAGE_ID/$ABI"
LIBDIR="$WORK/apkroot/lib/$ABI"
mkdir -p "$LIBDIR"

echo "==> Go application ($GOARCH)"
(cd "$APP_DIR" && CGO_ENABLED=1 GOOS=android GOARCH=$GOARCH CC="$CC" \
  go build -buildmode=c-shared -o "$LIBDIR/libpoemapp.so" .)

echo "==> C++ presenter"
"$CC" -c -fPIC -o "$WORK/glue.o" "$GLUE/android_native_app_glue.c" -I"$GLUE"
"$CXX" -shared -fPIC -std=c++17 -fexceptions -static-libstdc++ -o "$LIBDIR/libpoemhost.so" \
  "$ENGINE_DIR/src/main.cpp" "$ENGINE_DIR/src/renderer_gles.cpp" "$ENGINE_DIR/src/ime_jni.cpp" "$ENGINE_DIR/src/audio_aaudio.cpp" "$POEM_DIR/shared/poem/protocol.cpp" "$POEM_DIR/shared/poem/perf.cpp" \
  "$WORK/glue.o" -I"$ENGINE_DIR/src" -I"$POEM_DIR/shared" -I"$GLUE" \
  -L"$LIBDIR" -lpoemapp -lEGL -lGLESv2 -laaudio -landroid -llog -u ANativeActivity_onCreate

# Optional extra native libraries (space-separated paths) to bundle, e.g. an
# app-specific engine libpoemapp.so links against. They are copied into the ABI
# lib dir and packaged alongside the host/app libs.
for extra in ${EXTRA_LIBS:-}; do
  cp "$extra" "$LIBDIR/"
done

echo "==> Manifest + package"
# DEBUGGABLE=1 marks the app debuggable so `adb run-as` can reach its sandbox
# (used to stage test data). Off by default; never set it for a release build.
debug_attr=""
[ -n "${DEBUGGABLE:-}" ] && debug_attr=' android:debuggable="true"'
sed -e "s/__PACKAGE__/$PACKAGE_ID/" -e "s/__LABEL__/$APP_LABEL/" \
  -e "s#<application #<application${debug_attr} #" \
  "$ENGINE_DIR/AndroidManifest.template.xml" > "$WORK/AndroidManifest.xml"
"$BT/aapt2.exe" link -o "$WORK/unaligned.apk" --manifest "$WORK/AndroidManifest.xml" \
  -I "$SDK/platforms/android-36.1/android.jar"
(cd "$WORK/apkroot" && "$JAVA_HOME/bin/jar.exe" uf ../unaligned.apk lib/"$ABI"/*.so)
"$BT/zipalign.exe" -f -p 4 "$WORK/unaligned.apk" "$OUT_APK"
"$BT/apksigner.bat" sign --ks "$USERPROFILE/.android/debug.keystore" \
  --ks-pass pass:android --ks-key-alias androiddebugkey "$OUT_APK"
echo "==> Built $OUT_APK"

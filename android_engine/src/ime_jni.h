#pragma once

#include <string>

// JNI bridges for text input on the Android presenter. Zero Java ships in the
// APK, but a NativeActivity still carries a JavaVM and an Activity instance,
// which is enough to drive InputMethodManager (the reliable way to summon the
// soft keyboard — ANativeActivity_showSoftInput is notoriously ignored on
// modern Android) and KeyEvent.getUnicodeChar (keycode+meta → codepoint,
// covering the layouts a hardcoded table would get wrong).

struct ANativeActivity;

namespace poem {

// SetSoftKeyboardVisible shows or hides the IME. Safe to call from any
// thread; attaches to the JVM as needed.
void SetSoftKeyboardVisible(ANativeActivity* activity, bool visible);

// UnicodeCharForKey resolves an Android keycode + meta state to the Unicode
// codepoint KeyEvent would produce, or 0 for non-printing keys.
int UnicodeCharForKey(ANativeActivity* activity, int keyCode, int metaState);

} // namespace poem

namespace poem {
// SystemInsets reads the window's system-bar/cutout insets (physical px)
// via View.getRootWindowInsets — the reliable source on edge-to-edge
// Android, where the glue's contentRect stays empty. Returns false if the
// insets are unavailable (pre-attach); out = {left, top, right, bottom}.
bool GetSystemInsets(ANativeActivity* activity, int out[4]);
} // namespace poem

namespace poem {
// GetImeInset returns the soft keyboard's current bottom inset in physical
// px (0 when hidden, or on pre-API-30 devices where adjustResize resizes the
// surface instead).
int GetImeInset(ANativeActivity* activity);
} // namespace poem

namespace poem {
// AppFilesDir returns the app's private internal files directory
// (Context.getFilesDir().getAbsolutePath()) via JNI. This is the reliable
// source for the writable data directory: ANativeActivity::internalDataPath is
// null on some devices/emulators. Returns "" if JNI is unavailable.
std::string AppFilesDir(ANativeActivity* activity);
} // namespace poem

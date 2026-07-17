# POEM Tasks

## Android presenter — Phase 4 (post counter-milestone follow-ups)

The counter milestone (2026-07-16) proved the chain: Trellis Go `App[S]` →
`pkg/mobile` in-process transport → `android_engine` C++/GLES presenter, tap
verified on the API 36 emulator. These are the known, deliberate gaps left
open at that gate:

- [x] **IME / soft keyboard** (2026-07-16): `SetImeVisible` (protocol message
      6) fires on text-entry focus change (hosted mode only); the presenter
      summons/dismisses the keyboard via JNI InputMethodManager and translates
      key events (Win32 VK vocabulary + `KeyChar` via `KeyEvent.getUnicodeChar`).
      Verified on emulator: focus→keyboard, typing with spaces, backspace
      editing, unfocus→dismiss.
- [x] **Audio** (2026-07-17): PlaySound plays through AAudio - the presenter
      synthesizes the same hover/click/success PCM as the Windows sidecar
      (generators mirrored; cpp_sidecar untouched to preserve the provenance
      manifest) and fires a short-lived output stream per sound, capped at 4
      concurrent. Opt-in per app via AppConfig.Effects.Audio (the preferences
      Android example enables it). Verified on emulator: AAUDIO_OK streams
      registered by the audio service on tap.
- [ ] **Accessibility bridge**: the presenter drops `SemanticTree` messages;
      map them to Android accessibility nodes (the cpp_sidecar UIA bridge is
      the reference implementation).
- [x] **Activity lifecycle hardening** (2026-07-17): rotation detected by
      per-frame surface-size comparison (which app-cmd announces it varies),
      re-sending logical WindowSize for relayout; APP_CMD_PAUSE/RESUME/STOP
      stop/resume presentation while the engine keeps running; destroy no
      longer kills the engine or transport (Android can relaunch the activity
      into the same process - re-entry is INIT_WINDOW + atlas restore);
      eglSwapBuffers failure drops the surface gracefully. Verified on
      emulator: rotate to landscape (relayout 914x411 logical), tap in
      landscape (input mapping correct), rotate back, home, relaunch - all
      state intact in the same engine process.
- [ ] **arm64 on-device verification**: the complete preferences APK now
      builds for arm64-v8a (engine + presenter with GLES/IME/AAudio, 6MB) -
      build path fully proven; EXECUTION still needs a physical device
      (arm64 AVDs are unsupported on x86_64 hosts). Plug in a phone with USB
      debugging and install android_engine/build/preferences-arm64.apk.
- [x] **All-16-kinds sweep** (2026-07-16): every Node kind exercised by touch
      on the emulator via the preferences and settings examples - checkbox,
      switch, radio, slider drag, Select popup, TextArea+IME, accordion
      expand with nested dispatch, App[S]-owned tab switch, modal open/
      nested-dispatch/back-dismiss. Found and fixed a latent engine bug
      (Slider.OnMouseUp consumed every release and wiped ActiveID, killing
      all up-driven controls in slider-bearing trees; regression + invariant
      tests in pkg/app/desktop). Back maps to Escape (overlay dismiss);
      manifest opts out of predictive back so the key event is delivered.
- [x] **Parity matrix Android column** (2026-07-16): extend
      `okf/concepts/app/component-parity.md` with per-kind Android
      status once the sweep runs.
- [ ] **CI story**: decide how the Android presenter builds in CI (NDK
      toolchain on a Windows or Linux runner; APK artifact for the release
      workflow).
- [ ] **Atlas re-rasterization at density**: glyphs are rasterized at logical
      size and scaled ×density by the presenter, so text is slightly soft on
      high-density screens; re-rasterize the atlas at physical size (the
      `FontAtlas` message type exists for exactly this).
- [ ] **OKF bundle validation test**: port the markdown frontmatter/link validator (previously Trellis/GopherWeb ./okf Go tests) to validate POEM's okf bundle in CI.
- [ ] **Accordion expansion overlap**: expanding a section pushes content down
      but later siblings (About content / Save) can overlap after relayout on
      Android — reproduce on desktop and fix in FlexBox/Accordion measure.

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
- [x] **arm64 on-device verification** (2026-07-17): preferences ran on a
      physical Xiaomi (2407FPN8EG, arm64-v8a, API 36, 520dpi): rendering,
      touch, IME typing, AAudio click, and rotation all confirmed by hand.
      System-bar insets now come from getRootWindowInsets over JNI (the glue
      contentRect is empty on edge-to-edge Android); drawing/clipping shift
      by the content origin and touch translates back. Remaining device
      findings recorded below (touch scrolling, adjustResize, MIUI inset
      residual).
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
- [x] **Touch scrolling** (2026-07-17, tuned on-device): the pkg/app driver
      wraps every page root in a ScrollView (neutral when content fits, all
      targets), ScrollView wheel handling scales with delta magnitude
      (Windows ±120 keeps its 100px notch; pixel sources track 1:1), and the
      presenter classifies touches - tap (with wobble slop 14dp, suppressed
      when catching a fling), vertical drag = pixel-accurate scroll streamed
      as wheel events, horizontal drag or 220ms hold-to-grab = real drag for
      sliders, and fast release = fling (velocity ×1.4 capped 10k px/s,
      exponential decay τ=0.3s, tap-to-stop). Feel approved on the physical
      device.
- [ ] **Keyboard obscures focused field (device finding, 2026-07-17)**: set
      android:windowSoftInputMode="adjustResize" so the IME shrinks the
      surface (the per-frame geometry check will relayout), and scroll the
      focused text field into view — depends on touch scrolling above.
- [ ] **Bottom inset residual (device finding, 2026-07-17)**: with 3-button
      nav some content still renders behind the buttons — verify whether the
      deprecated systemWindowInset accessors under-report on MIUI/HyperOS and
      switch to the Type-based getInsets(systemBars()|displayCutout) API 30
      path if so.

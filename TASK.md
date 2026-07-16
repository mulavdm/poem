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
- [ ] **Audio**: `PlaySound` messages are currently ignored; wire them to
      AAudio/Oboe or keep a documented silent stub.
- [ ] **Accessibility bridge**: the presenter drops `SemanticTree` messages;
      map them to Android accessibility nodes (the cpp_sidecar UIA bridge is
      the reference implementation).
- [ ] **Activity lifecycle hardening**: pause/resume, surface recreation, and
      rotation (rotation should re-send `WindowSize` and relayout; atlas
      re-upload already handled).
- [ ] **arm64 on-device verification**: the arm64 build compiles but only the
      x86_64 emulator has been exercised; run the counter on a physical
      device.
- [x] **All-16-kinds sweep** (2026-07-16): every Node kind exercised by touch
      on the emulator via the preferences and settings examples - checkbox,
      switch, radio, slider drag, Select popup, TextArea+IME, accordion
      expand with nested dispatch, App[S]-owned tab switch, modal open/
      nested-dispatch/back-dismiss. Found and fixed a latent engine bug
      (Slider.OnMouseUp consumed every release and wiped ActiveID, killing
      all up-driven controls in slider-bearing trees; regression + invariant
      tests in pkg/app/desktop). Back maps to Escape (overlay dismiss);
      manifest opts out of predictive back so the key event is delivered.
- [ ] **Parity matrix Android column**: extend
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

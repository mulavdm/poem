# POEM Tasks

## Android presenter — Phase 4 (post counter-milestone follow-ups)

The counter milestone (2026-07-16) proved the chain: Trellis Go `App[S]` →
`pkg/mobile` in-process transport → `android_engine` C++/GLES presenter, tap
verified on the API 36 emulator. These are the known, deliberate gaps left
open at that gate:

- [ ] **IME / soft keyboard**: summon and dismiss the keyboard when the engine
      focuses a text field, and forward committed characters as `KeyChar`
      events. Needs a small engine→presenter protocol addition (a
      show/hide-keyboard command) — the first schema change of the port.
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
- [ ] **All-16-kinds sweep**: build the Trellis preferences example for
      Android and exercise every Node kind by touch (Select popups, Slider
      drag, TextArea + IME, Modal, Accordion, Tabs).
- [ ] **Parity matrix Android column**: extend
      `Trellis/okf/architecture/component-parity.md` with per-kind Android
      status once the sweep runs.
- [ ] **CI story**: decide how the Android presenter builds in CI (NDK
      toolchain on a Windows or Linux runner; APK artifact for the release
      workflow).
- [ ] **Atlas re-rasterization at density**: glyphs are rasterized at logical
      size and scaled ×density by the presenter, so text is slightly soft on
      high-density screens; re-rasterize the atlas at physical size (the
      `FontAtlas` message type exists for exactly this).
- [ ] **OKF bundle validation test**: port the markdown frontmatter/link validator (previously Trellis/GopherWeb ./okf Go tests) to validate POEM's okf bundle in CI.

# POEM

## Vector cartography foundation (M9, in development)

`MapViewportNode` is POEM's controlled semantic map canvas: applications provide a credential-free `MapSource`, typed style intent, stable map features, camera limits, quality/cache policy, and fallback image. State-derived `App.MapResources` providers keep endpoints and tokens in Go; web sessions access them only through the validated same-origin broker. `App.Subscriptions` adds keyed multi-message sources, and host capabilities are available only through command/subscription contexts via `PlatformServices`.

`pkg/cartography` is the shared Go core for bounded MVT decoding, Web-Mercator coverage, typed-style evaluation, deterministic scene construction, bounded concave polygon tessellation with holes and multipolygon parts, stable application route/marker/POI/traffic overlays, hash-keyed retained buffers, and bounded OpenType shaping through the pinned pure-Go `go-text/typesetting` library. Its GPU-independent label stage performs deterministic priority ordering, camera-aware screen placement, bounded collision, culling, and cancellation and compiles unchanged to WASM. Accepted native labels are rasterized from exact shaped glyph IDs into deterministic bounded alpha atlases and submitted as hash-keyed textured ranges; D3D11 and GLES3 upload each atlas once and retain it until release. Native resource loading is cancellation-aware, limited to eight concurrent fetches and quality-bounded tile coverage, and backed by a snapshot-isolated decoded-tile LRU. Scene submission references unchanged hashes without resending their bytes and explicitly releases resources that leave a viewport. Protocol v4 carries generation-fenced retained map scenes and an explicit viewport-placement draw command. The Windows D3D11 and Android GLES3 presenters cache projected scene geometry in dedicated immutable/static GPU buffers, interleave it with UI draw ranges, preserve local pan/zoom preview and raster fallback ordering, and rebuild only for a new scene or placement. Hash-resource-level vertex/index GPU buffers and the WebGL2/Go-WASM worker remain active M9 gates.

## Map-first adaptive workspaces (M8)

`App.Start` can publish startup state and launch one keyed command exactly once per native engine or new web session. `WorkspaceNode` now keeps a primary canvas persistent while adapting its tools: docked on expanded windows, compact overlay on medium layouts, and a draggable bottom sheet on compact touch layouts. `SectionNode`, semantic action importance/placement, interactive collections, product color overrides, and viewport size commits let applications build rich workspaces without application-owned visual metrics.

The web driver serves framework, design, and component CSS from same-origin endpoints under `style-src 'self'`; no required inline style or script is used. See [MIGRATION_M8.md](MIGRATION_M8.md).

## Single-process native Windows host (M7)

Windows applications are now shipped as an application-specific Go `poem_app.dll` beside the generic `poem_windows_host.exe` (renamed for the product). The host loads the DLL with restricted adjacent-directory search, validates the versioned C ABI, and carries the existing POEM protocol over bounded in-memory pipes. The application and reducer remain compiled Go machine code with an embedded Go runtime; Win32, D3D11, UI Automation, IME, audio, and window lifecycle remain native C++. Both live in one OS process. There is no presenter child process, extracted payload, renderer named pipe, or sidecar override.

## Adaptive native design (M6)

`pkg/design` resolves one semantic application tree into Windows, Android, web, or neutral Adaptive House conventions. It uses the four standard window classes, input capability and density independently of width, text scale, reduced motion, high contrast, and a contrast-safe host accent. The web renderer generates CSS variables from the same resolved tokens used by native components.

Application actions come from the state-derived `App.Commands` registry and are referenced by semantic nodes with stable IDs. Messages and asynchronous commands retain the transport-safe reducer contract. See [MIGRATION_M6.md](MIGRATION_M6.md) for the breaking migration.

POEM is a Go UI framework with **one application API and three targets**: an application is
written once as a `pkg/app` `App[S]` — serializable state, a pure `View` producing a `Node`
tree, a pure `Update` reducer — and runs as a native Windows desktop app, a server-rendered
web app, or an Android app from the same definition.

```go
type State struct{ Count int }

var App = app.App[State]{
	Init: State{},
	View: func(s State) app.Node {
		return app.Container(app.Vertical, 12,
			app.Text("Count: "+strconv.Itoa(s.Count)),
			app.Button("Increment", app.Msg{Name: "increment"}),
		)
	},
	Update: func(s State, m app.Msg) (State, app.Cmd) {
		if m.Name == "increment" {
			s.Count++
		}
		return s, app.Cmd{}
	},
}
```

Host it (see `examples/counter/` for all three targets):

```go
config := desktop.Configure(App, render.AppConfig{Title: "Counter", Width: 480, Height: 320})
poemwindows.MustRegister(config, poemwindows.Metadata{Identity: "POEM.Counter", Title: config.Title, Width: config.Width, Height: config.Height})
web.Run(App, "127.0.0.1:8090", "Counter")              // stateful web server, no JS required
// Android: examples/counter/android + android_engine/build_apk.sh → installable APK
```

As of 2026-07-16 the former sibling projects **Trellis** (the app layer) and **GopherWeb**
(the web component library) are merged into this repository — see
[okf/concepts/decisions/consolidation.md](okf/concepts/decisions/consolidation.md).

## Layers

- **`pkg/app`** — the public application API: `App[S]`, named `Msg`s, keyed asynchronous
  `Cmd`s, and a closed set of `Node` kinds (19 today) that every target renders. Annotated
  `ImageViewportNode`s provide controlled pan/zoom, point activation, and accessible markers;
  `ResponsiveNode` selects compact or wide content from available width. Drivers: `pkg/app/desktop`,
  `pkg/app/web`; Android launches through `pkg/mobile`.
- **`pkg/render`** — the engine layer: the component/theme/layout/semantics runtime, the
  binary presenter protocol (`pkg/render/protocol`) and `render.RunHosted`.
  Public and usable directly for advanced native apps (see `cmd/gallery`), but its event
  handlers are Go closures — they cannot cross the web target's HTTP boundary, which is why
  `pkg/app` is the one application API (see the consolidation decision).
- **`pkg/web`** — the server-rendered HTML component library and HTTP middleware the web
  driver renders through (formerly GopherWeb; `poem-*` CSS classes, no-JS baseline).
- **Presenters** — native hosts that draw the engine's frames and feed input back over the
  protocol: the `poem_windows_host` target in `cpp_sidecar/` (Win32, D3D11, UI Automation),
  `android_engine/` (Android: C++/EGL/GLES2 in a zero-Java NativeActivity APK), and
  `rust_engine/` (legacy Windows/OpenGL reference, no longer the active runtime path).
  These live at the top level because they are foreign-toolchain native code; the web
  target has no presenter directory by design — it never sees draw commands, so its whole
  engine is the pure-Go `pkg/web` + `pkg/app/web` pair.

## Project Layout

```text
.
|-- pkg/app/               # Public application API (App[S], Msg, Node) + desktop/web drivers
|-- pkg/render/            # Engine layer: components, layout, protocol, state, engine loop
|-- pkg/cartography/       # Shared bounded vector decode/style/scene core (Go + future WASM build)
|-- pkg/web/               # Server-rendered HTML components + HTTP middleware
|-- pkg/hosted/            # Shared bounded in-process engine transport
|-- pkg/mobile/            # Android-specific c-shared bridge and logcat integration
|-- pkg/windows/           # Versioned Windows c-shared ABI and application metadata
|-- cpp_sidecar/           # Generic poem_windows_host C++ source (Win32 + D3D11 + UIA)
|-- windows_host/          # Portable ZIP/MSIX packaging and development signing commands
|-- android_engine/        # Android presenter (EGL/GLES2) + no-Gradle APK build script
|-- rust_engine/           # Legacy Windows presenter (OpenGL), reference only
|-- examples/              # counter, preferences, settings — one App[S], three targets each
|-- cmd/engine, cmd/gallery# Engine-layer demos (component API directly, desktop only)
|-- okf/                   # Living documentation bundle (start at okf/index.md)
|-- schema/                # Legacy FlatBuffers schema (superseded by pkg/render/protocol)
`-- TASK.md                # Tracked follow-up work (Android Phase 4, CI, docs)
```

## Automation

The engine layer includes a shared automation surface for native apps, exposing component
tree snapshots, click/focus/text/key commands, direct PNG frame capture
(`self-frame`/`window-frame`/`desktop-frame`, plus a shared `inspect-frame` endpoint that
chooses the safest inspection source automatically), performance snapshots and action
measurement helpers, and native window state.

Minimal example:

```go
config := render.AppConfig{
	Title:        "POEM App",
	Width:        1024,
	Height:       768,
	BuildPagesFn: buildPages,
	Automation: &render.AutomationConfig{
		Enabled:    true,
		Mode:       "http",
		Host:       "127.0.0.1",
		Port:       47831,
		CaptureDir: "output/automation",
	},
}
poemwindows.MustRegister(config, poemwindows.Metadata{
	Identity: "Example.App", Title: config.Title, Width: config.Width, Height: config.Height,
})
```

The source-of-truth automation reference is [okf/concepts/automation/index.md](./okf/concepts/automation/index.md).
Commands may use stable component IDs or unique semantic role/name/state
selectors; successful selector commands report the resolved stable target ID.

Windows accessibility integration is exercised by `cpp_sidecar/test_uia.ps1`.
It reads the platform-neutral semantic tree through UI Automation and verifies
that standard control-pattern actions reach the Go component runtime and that
property/text-change notifications reach a native accessibility client. The
probe also covers emoji/CJK TextPattern ranges and password redaction.
Collection coverage includes tab/table Selection and data-grid Grid/Table item
relationships. Scrollable containers publish live ScrollPattern percentages and
retain their semantic descendants. Stable-ID semantic relationships map to UIA
LabeledBy, DescribedBy, ControllerFor, and FlowsTo properties.

## Layout Measurement

POEM's layout model is intentionally lighter than browser-grade CSS flexbox.

Today:

- parent layouts still place children explicitly
- leaf components can now opt into a native `Measure(...)` contract
- `FlexBox` prefers measured sizes when available and falls back to legacy `Bounds()` sizing otherwise
- containers can optionally expose `ContentSize(...)` when their scrollable content is larger than their assigned draw bounds

This means downstream apps should not assume "declare children and forget it" browser behavior yet. For stable layouts:

- use explicit pane rects for major dashboard regions
- let leaf controls report preferred size through `Measure(...)`
- wrap overflow regions in `ScrollView`; it uses `ContentSize(...)` where available so fixed viewport bounds do not erase scrollable extent
- avoid using oversized placeholder `Bounds()` as a proxy for wrapped text size
- cap long status text or labels explicitly when the UI has tight horizontal budgets
- **Systematic Layout Rule**: Never use hardcoded visual offsets or ad-hoc coordinate bypasses to fix visual text or container clipping. Position and baseline issues must be resolved systematically within the rendering engine's layout components (like FlexBox or Grid) or component-level metric calculations, and parent container dimensions must be properly sized to fit their contents.

See [okf/concepts/architecture/layout-measurement.md](./okf/concepts/architecture/layout-measurement.md) for the downstream migration pattern.

## Windows hosting and packaging

The generic host loads only an absolute adjacent `poem_app.dll`. It validates ABI version 1 and all required exports before starting the Go engine, never unloads the Go runtime, and stops it cooperatively when the window closes. Application packages export:

- `PoemWindowsABIVersion`, `PoemWindowsMetadata`, and `PoemWindowsStart`
- `PoemHostRead` and `PoemHostWrite`
- `PoemWindowsStop`

Build a portable folder/ZIP and unsigned MSIX with `windows_host/build.ps1`. Signing is optional; secrets are read from CI environment configuration. `New-DevelopmentCertificate.ps1` creates or reuses an explicitly named per-user signing certificate, while trusting its exported certificate requires the separate opt-in `Trust-DevelopmentCertificate.ps1` command.

## Build

Windows host and a complete gallery package:

```powershell
cmake -S cpp_sidecar -B cpp_sidecar\build
cmake --build cpp_sidecar\build --config Release
windows_host\build.ps1 -ProjectDirectory . -GoPackage ./cmd/gallery `
  -Identity POEM.Gallery -DisplayName "POEM Gallery" -Version 0.7.0.0 `
  -OutputDirectory .\dist
```

Android APK (NDK clang + aapt2 + apksigner, no Gradle):

```bash
android_engine/build_apk.sh examples/counter/android com.trellis.counter Counter counter.apk x86_64
```

## Current Runtime Status

The single-process Windows host supports cooperative start/shutdown, bootstrap atlas
upload, in-memory protocol frames, mouse/wheel/keyboard/resize/DPI events, portable
shortcuts and UIA-visible mnemonics, DPI-aware startup sizing, cursor switching, core draw
commands with text atlas and clipping, sound triggers, HTTP automation transport, native
foreground activation and window state, and the self/window/desktop capture modes.

The Android presenter covers surface bring-up, density-scaled rendering of the core draw
commands, two-pointer pan/pinch gestures, touch input, and the in-process engine transport. Known follow-up work (tracked in
`TASK.md`): IME/soft keyboard, audio, an accessibility bridge, activity-lifecycle hardening,
and atlas re-rasterization at density. On Windows, raycaster/billboard special paths are not
fully ported from the legacy renderer, and glass/blur are functional approximations.

## Docs

- [okf/index.md](./okf/index.md) — living documentation bundle (architecture, app layer, web
  engine, protocol, automation, decisions)
- [docs/POEM_2_FOUNDATION.md](./docs/POEM_2_FOUNDATION.md) — the POEM 2.0 foundation guide
- [AGENTS.md](./AGENTS.md)
- [RELEASING.md](./RELEASING.md)
- [TASK.md](./TASK.md)

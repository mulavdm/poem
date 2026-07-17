# POEM

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
	Update: func(s State, m app.Msg) State {
		if m.Name == "increment" {
			s.Count++
		}
		return s
	},
}
```

Run it (see `examples/counter/` for all three mains):

```go
desktop.Run(App, render.AppConfig{Title: "Counter"})   // native Windows window
web.Run(App, "127.0.0.1:8090", "Counter")              // stateful web server, no JS required
// Android: examples/counter/android + android_engine/build_apk.sh → installable APK
```

As of 2026-07-16 the former sibling projects **Trellis** (the app layer) and **GopherWeb**
(the web component library) are merged into this repository — see
[okf/concepts/decisions/consolidation.md](okf/concepts/decisions/consolidation.md).

## Layers

- **`pkg/app`** — the public application API: `App[S]`, serializable `Msg`s, and a closed set
  of `Node` kinds (16 today) that every target renders. Drivers: `pkg/app/desktop`,
  `pkg/app/web`; Android launches through `pkg/mobile`.
- **`pkg/render`** — the engine layer: the component/theme/layout/semantics runtime, the
  binary presenter protocol (`pkg/render/protocol`), and `render.Run`/`render.RunHosted`.
  Public and usable directly for advanced native apps (see `cmd/gallery`), but its event
  handlers are Go closures — they cannot cross the web target's HTTP boundary, which is why
  `pkg/app` is the one application API (see the consolidation decision).
- **`pkg/web`** — the server-rendered HTML component library and HTTP middleware the web
  driver renders through (formerly GopherWeb; `poem-*` CSS classes, no-JS baseline).
- **Presenters** — native hosts that draw the engine's frames and feed input back over the
  protocol: `cpp_sidecar/` (the active Windows presenter: Win32, D3D11, UI Automation),
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
|-- pkg/web/               # Server-rendered HTML components + HTTP middleware
|-- pkg/mobile/            # Android in-process engine<->presenter transport (c-shared exports)
|-- cpp_sidecar/           # Active Windows presenter (Win32 + D3D11 + UIA)
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
render.Run(render.AppConfig{
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

## Sidecar Resolution (Windows)

At runtime, POEM resolves `poem_cpp_sidecar.exe` in this order:

1. `POEM_SIDECAR_PATH`
2. the same directory as the host executable
3. development build outputs under `cpp_sidecar/build`
4. an embedded Windows sidecar payload extracted automatically by POEM

This keeps downstream apps simple: a consumer can import the Go API and run without
project-specific sidecar path setup. The embedded payload's provenance is verified in CI
against the committed sidecar sources (see [RELEASING.md](./RELEASING.md)).

## Build

Windows sidecar and demos:

```powershell
cmake -S cpp_sidecar -B cpp_sidecar\build
cmake --build cpp_sidecar\build --config Release
go build ./cmd/gallery    # component-gallery acceptance surface
go build ./cmd/engine     # legacy runtime smoke demo
```

For a Windows GUI binary without a console window:

```powershell
go build -ldflags="-s -w -H=windowsgui" -o POEM.exe ./cmd/engine
```

Android APK (NDK clang + aapt2 + apksigner, no Gradle):

```bash
android_engine/build_apk.sh examples/counter/android com.trellis.counter Counter counter.apk x86_64
```

## Current Runtime Status

The active Windows sidecar supports the current core path: spawn/shutdown, bootstrap atlas
upload, render frames over the protocol, mouse/wheel/keyboard/resize/DPI events, portable
shortcuts and UIA-visible mnemonics, DPI-aware startup sizing, cursor switching, core draw
commands with text atlas and clipping, sound triggers, HTTP automation transport, native
foreground activation and window state, and the self/window/desktop capture modes.

The Android presenter covers surface bring-up, density-scaled rendering of the core draw
commands, touch input, and the in-process engine transport. Known follow-up work (tracked in
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

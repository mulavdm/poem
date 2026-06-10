# POEM

POEM is a frameworkless desktop UI engine for Go applications. The public API lives in `pkg/render`, where consumers call `render.Run(render.AppConfig{...})` and build pages declaratively from Go. Native presentation is handled by a separate sidecar process so layout, state, focus, and input semantics stay in Go while the window, GPU rendering, and local audio stay native.

The active runtime today is:

- Go orchestrator for state, layout, page rebuilds, hit-testing, focus, automation semantics, and draw-command generation
- Windows C++ sidecar for Win32 windowing, D3D11 rendering, cursor updates, DPI handling, and native capture hooks
- local IPC over two Windows named pipes
- repo-owned binary protocol in `pkg/render/protocol`

The public Go API remains stable while the native presentation layer evolves underneath it.

## Project Layout

```text
.
|-- cmd/engine/            # Demo entrypoint that exercises the library
|-- cpp_sidecar/           # Windows-first native presentation sidecar (Win32 + D3D11)
|-- docs/                  # Focused reference docs such as automation
|-- internal/win32/        # Private Win32 syscall wrappers used by the Go side
|-- pkg/render/            # Public Go UI library
|   |-- components/        # Declarative UI primitives
|   |-- layout/            # Layout helpers
|   |-- protocol/          # Custom binary wire protocol
|   |-- types/             # Shared state, interfaces, and contracts
|   |-- painter.go         # Draw-command capture and frame serialization
|   `-- run.go             # Sidecar launch, IPC, and event/render orchestration
|-- AGENTS.md              # Project-specific agent guidance
|-- ARCHITECTURE.md        # Design notes and deeper implementation context
|-- GEMINI.md              # Mirrored project-specific agent guidance
`-- GUIDE.md               # UI authoring quickstart
```

`rust_engine/` is still present as legacy reference material during the port, but it is no longer the active runtime path.

## Using POEM

```go
package main

import "go_native_gpu_gui/pkg/render"

func main() {
	render.Run(render.AppConfig{
		Title:  "POEM App",
		Width:  1024,
		Height: 768,
		BuildPagesFn: func(state *render.ApplicationState) {
			// Build or rebuild your page tree here.
		},
	})
}
```

Consumers keep importing only `go_native_gpu_gui/pkg/render`. They do not need to know whether the native runtime is implemented in C++, Rust, or another sidecar later.

## Automation

POEM includes a shared automation layer for downstream native apps. When enabled through `render.AutomationConfig`, it can expose:

- component tree snapshots
- click/focus/text/key commands
- direct PNG frame capture
- shared performance snapshots and action measurement helpers
- native window-action measurement for latency debugging
- native window state
- dedicated inspection captures:
  - `self-frame`
  - `window-frame`
  - `desktop-frame`
- a shared `inspect-frame` endpoint that chooses the safest inspection source automatically

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

The source-of-truth automation reference is [docs/AUTOMATION.md](./docs/AUTOMATION.md).

## Sidecar Resolution

At runtime, POEM resolves `poem_cpp_sidecar.exe` in this order:

1. `POEM_SIDECAR_PATH`
2. the same directory as the host executable
3. development build outputs under `cpp_sidecar/build`
4. an embedded Windows sidecar payload extracted automatically by POEM

This keeps downstream apps simple: a consumer can import `go_native_gpu_gui/pkg/render` and call the Go API without adding project-specific sidecar path setup.

## Build

Build the sidecar:

```powershell
cmake -S cpp_sidecar -B cpp_sidecar\build
cmake --build cpp_sidecar\build --config Release
```

Build the Go demo:

```powershell
go build ./cmd/engine
```

Run the demo:

```powershell
.\engine.exe
```

For a Windows GUI binary without a console window:

```powershell
go build -ldflags="-s -w -H=windowsgui" -o POEM.exe ./cmd/engine
```

For custom packaging, you can still place `poem_cpp_sidecar.exe` next to the built Go executable or override it with `POEM_SIDECAR_PATH`, but POEM now also carries an embedded Windows fallback for downstream consumers.

## Current Runtime Status

The active C++ sidecar supports the current core path:

- sidecar spawn and shutdown
- bootstrap atlas upload
- render frames over the custom protocol
- mouse, wheel, keyboard, resize, and DPI events
- startup sizing that uses the target monitor DPI and launches within a conservative work-area fraction
- cursor switching
- core draw commands, text atlas rendering, clipping, and sound triggers
- HTTP automation transport
- stronger native foreground activation for automation and inspection flows
- native window state reporting
- self/window/desktop capture modes
- shared inspection fallback flow for agents and tests

Known follow-up work:

- Rust-specific special rendering paths such as raycaster or billboard sentinel handling are not fully ported yet
- glass and blur are currently functional approximations, not full parity with the old renderer
- cross-platform sidecars are a later step; the current native runtime is Windows-first

## Docs

- [GUIDE.md](./GUIDE.md)
- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [docs/AUTOMATION.md](./docs/AUTOMATION.md)
- [AGENTS.md](./AGENTS.md)
- [GEMINI.md](./GEMINI.md)

# POEM: Hybrid Process-Isolated GUI Engine in Go & Rust

A lightweight, highly modular, zero-framework desktop window engine. This project features a state-of-the-art **process-isolated dual-runtime architecture**: an orchestrating Go backend driving a hardware-accelerated **wgpu/winit Rust sidecar** over high-performance FlatBuffers and Windows Named Pipes.

Originally a prototype engine, POEM is now fully packaged as an **importable standalone library (`pkg/render`)** so that any external Go application (like the Book Manager) can easily construct stunning, premium user interfaces.

POEM also includes automated **High-DPI / 4K Multi-Resolution Display Scaling**, and a low-latency **Keyboard Focus Engine** supporting active-page cycling (`Tab`/`Shift+Tab`), glowing neon outline focus indicators, automatic scroll view centering, modular input bindings, and declarative global hotkeys (like `Ctrl+S`).

---

## 🏗️ Project Architecture

The engine is decoupled into modular packages to isolate concerns and enforce a strict acyclic dependency flow:

```text
.
├── cmd/engine/         # Main demo entry point dogfooding the library
├── pkg/
│   └── render/         # Public Standalone Library (Go Orchestrator)
│       ├── render.go   # Type Aliases & Constants (Single Import Interface)
│       ├── run.go      # Named Pipe Server & Loop Orchestration
│       ├── types/      # Component Contracts, ApplicationState & Telemetry
│       ├── components/ # Declarative Primitives (Panel, Button, TextInput, Slider, etc.)
│       └── layout/     # Axis-Aligned FlexBox calculations
├── internal/
│   └── win32/          # Private Syscalls & Named Pipe DLL Bindings
├── rust_engine/        # Hardware-Accelerated wgpu/winit Rust Sidecar
│   ├── src/main.rs     # Event loop & Named Pipe connection logic
│   └── src/renderer.rs # Batch-renderer & Gaussian frosted-glass shaders
├── ARCHITECTURE.md     # Technical Deep-Dive & Engineering Log
└── GEMINI.md           # Developer Guide for AI Assistants
```

---

## 🚀 How to Consume the POEM Library

A client Go application only requires a **single import statement** to gain access to the complete component registry, layout tools, and window runner:

```go
package main

import (
	"go_native_gpu_gui/pkg/render"
)

func main() {
	render.Run(render.AppConfig{
		Title:  "Book Manager // Realized",
		Width:  1024,
		Height: 768,
		BuildPagesFn: func(state *render.ApplicationState) {
			// Populate page registry components reactively here!
		},
	})
}
```

### 🔗 Linking the Dependency in Your Project

Depending on your distribution and team environment, you can reference the POEM library in one of three clean Go-standard ways:

#### Strategy A: Remote Git Repository (Production Release)
If you publish the POEM repository to a hosting platform like GitHub:
1. Change the root `go.mod` module path to match your repo (e.g., `module github.com/username/poem`).
2. Downstream developers simply import it in their source file:
   ```go
   import "github.com/username/poem/pkg/render"
   ```
3. Run standard package fetching:
   ```bash
   go get github.com/username/poem@latest
   ```

#### Strategy B: Local Module Override (Isolated Local Dev)
If downstream developers want to consume a local folder copy without online hosting:
1. In the downstream project's `go.mod`, declare the dependency and add a **local override path**:
   ```go
   module my_app

   go 1.26.3

   require go_native_gpu_gui v0.0.0-00010101000000-000000000000

   // Direct the compiler to search a local absolute or relative folder
   replace go_native_gpu_gui => ../POEM
   ```
2. The Go compiler will seamlessly map `go_native_gpu_gui/pkg/render` to the local target directory.

#### Strategy C: Go Multi-Module Workspaces (Clean Monorepos)
If you are actively developing both the library and consumer application concurrently, you can establish a clean, hardcode-free development workspace:
1. Create a `go.work` file in the shared parent directory:
   ```work
   go 1.26.3

   use (
       ./POEM
       ./Library
   )
   ```
2. This completely eliminates the need for `replace` lines in individual `go.mod` files, resolving modules locally across package boundaries automatically!

---

## 🛠️ Build & Run Instructions

To compile and run the process-isolated dual-runtime POEM GUI framework, you need to compile both the Rust presentation sidecar and the Go orchestrator.

### 1. Compile the Rust Sidecar Core (Release Mode)
This generates the hardware-accelerated wgpu binary which is dynamically spawned by Go.

```bash
cargo build --release --manifest-path rust_engine/Cargo.toml
```

### 2. Compile the Go Orchestrator
This compiles the orchestrator containing all layout formulas, hotkeys, state telemetry, and named pipe logic.

```bash
go build ./cmd/engine
```

### 3. Run the Dual-Runtime Application
Simply launch the Go orchestrator. It will automatically detect, launch, and establish named pipe communication loops with the Rust wgpu sidecar:

```bash
go run ./cmd/engine
```

---

## 📦 Production Release Optimization
To package a clean distribution without intermediate debug binaries or visible command prompt consoles on launch:
1. Compile the Rust sidecar in release mode (creates `poem_rust_engine.exe` inside `rust_engine/target/release/`).
2. Compile the Go engine with headless GUI flags:
   ```bash
   go build -ldflags="-s -w -H=windowsgui" -o POEM.exe ./cmd/engine
   ```

---

## 📂 Developer Guide: Extending the Engine

### Adding Application State variables
Modify **[types.go](file:///d:/Programming/GUIProject/POEM/pkg/render/types/types.go)**:
```go
type ApplicationState struct {
    ClickCount int
    StatusText string
    // Add your custom variables here!
}
```

### Adding New UI Primitives
1. Create your component struct in **[components.go](file:///d:/Programming/GUIProject/POEM/pkg/render/components/components.go)** implementing the `types.Component` interface.
2. Re-export the component inside **[render.go](file:///d:/Programming/GUIProject/POEM/pkg/render/render.go)** using type aliasing:
   ```go
   type MyNewComponent = components.MyNewComponent
   ```

### Modifying Painting & Rendering Logic
* **Updating Painters**: Adjust the drawing command serialization in `pkg/render/painter.go` and schema definitions.
* **Updating GPU Shaders**: Adjust the signed distance fields or blending calculations in **[main.wgsl](file:///d:/Programming/GUIProject/POEM/rust_engine/src/shaders/main.wgsl)** and **[blur.wgsl](file:///d:/Programming/GUIProject/POEM/rust_engine/src/shaders/blur.wgsl)**.
* **Updating GPU Batcher**: Adjust the vertex buffer allocation, batch ranges, or clipping rules inside the wgpu pipeline in **[renderer.rs](file:///d:/Programming/GUIProject/POEM/rust_engine/src/renderer.rs)**.

---

## 📜 Documentation
- **[GUIDE.md](./GUIDE.md)**: A comprehensive quickstart guide to building declarative UIs, laying out widgets with FlexBox, and binding reactive state events.
- **[ARCHITECTURE.md](./ARCHITECTURE.md)**: Deep-dive analysis of memory management, thread locking (`LockOSThread`), and Win32 syscall stabilization strategies.
- **[GEMINI.md](./GEMINI.md)**: Project-specific guardrails and instructions for AI-assisted development.
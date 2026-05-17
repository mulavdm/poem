# PolyEngine: Hybrid Frameworkless GUI Engine in Go

A lightweight, highly modular, zero-framework desktop window engine implemented in Go. This project features a polymorphic architecture that switches between a **Pure Go CPU Software Renderer** (zero external dependencies) and a **Hardware-Accelerated GPU Pipeline** (OpenGL Core Profile) using Go build tags.

Originally a prototype engine, PolyEngine is now fully packaged as an **importable standalone library (`pkg/render`)** so that any external Go application (like the Book Manager) can easily construct stunning, premium user interfaces.

---

## 🏗️ Project Architecture

The engine is decoupled into modular packages to isolate concerns and enforce a strict acyclic dependency flow:

```text
.
├── cmd/engine/         # Main demo entry point dogfooding the library
├── pkg/
│   └── render/         # Public Standalone Library
│       ├── render.go   # Type Aliases & Constants (Single Import Interface)
│       ├── run.go      # Native Win32 Event Pump & Application Runner
│       ├── types/      # Painter, UIRenderer, Component Contracts & ApplicationState
│       ├── components/ # Declarative Primitives (Panel, Button, TextInput, Slider, etc.)
│       ├── layout/     # Axis-Aligned FlexBox calculations
│       └── backend/    # CPU and GPU Painting Implementations (conditional tags)
├── internal/
│   └── win32/          # Private Syscalls & Native Windows Structs
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

---

## 🛠️ Build & Run Instructions

### 1. Pure CPU Mode (Default)
The CPU mode is **zero-dependency** and requires only the Go standard library (plus `golang.org/x/image` for fonts). It uses GDI `StretchDIBits` to blit a software-calculated pixel buffer directly to the window.

**Build Command:**
```bash
go build -o poem_cpu.exe ./cmd/engine
```

**Run:**
```bash
./poem_cpu.exe
```

### 2. Hardware GPU Mode (OpenGL)
The GPU mode pipelines rendering directly to the graphics card. It requires a C compiler (GCC) and the `gpu` build tag.

**Build Command:**
```bash
go build -tags gpu -o poem_gpu.exe ./cmd/engine
```

**Run:**
```bash
./poem_gpu.exe
```

---

## 📦 Production Release Optimization
To create a compact, professional binary without a terminal window:

```bash
go build -ldflags="-s -w -H=windowsgui" -o PolyEngine.exe ./cmd/engine
```
- `-s -w`: Strips debug symbols (reduces size).
- `-H=windowsgui`: Hides the console window on launch.

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

### Modifying Painting Logic
- **For CPU Rasterization**: Adjust the `CPUEngine` methods in **[cpu.go](file:///d:/Programming/GUIProject/POEM/pkg/render/backend/cpu.go)**.
- **For GPU Graphics**: Adjust the `GPUEngine` shader pipeline in **[gpu.go](file:///d:/Programming/GUIProject/POEM/pkg/render/backend/gpu.go)**.

---

## 📜 Documentation
- **[ARCHITECTURE.md](./ARCHITECTURE.md)**: Deep-dive analysis of memory management, thread locking (`LockOSThread`), and Win32 syscall stabilization strategies.
- **[GEMINI.md](./GEMINI.md)**: Project-specific guardrails and instructions for AI-assisted development.
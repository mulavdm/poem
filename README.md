# PolyEngine: Hybrid Frameworkless GUI Engine in Go

A lightweight, modular, zero-framework desktop window engine implemented in Go. This project features a polymorphic architecture that switches between a **Pure Go CPU Software Renderer** (zero external dependencies) and a **Hardware-Accelerated GPU Pipeline** (OpenGL Core Profile) using Go build tags.

---

## 🏗️ Project Architecture

The engine is decoupled into modular packages to ensure maintainability and idiomatic Go structure.

```text
.
├── cmd/engine/         # Main entry point & Win32 Event Loop
├── internal/
│   ├── render/         # Rendering Abstractions & Strategies
│   │   ├── renderer.go # Shared Interfaces & App State
│   │   ├── cpu.go      # Software Rasterizer (!gpu tag)
│   │   └── gpu.go      # OpenGL Pipeline (gpu tag)
│   └── win32/          # Native Syscalls & Windows Structs
├── ARCHITECTURE.md     # Technical Deep-Dive & Engineering Log
└── GEMINI.md           # Developer Guide for AI Assistants
```

---

## 🛠️ Build & Run Instructions

### 1. Pure CPU Mode (Default)
The CPU mode is **zero-dependency** and requires only the Go standard library (plus `golang.org/x/image` for fonts). It uses GDI `StretchDIBits` to blit a software-calculated pixel buffer directly to the window.

**Build Command:**
```bash
go build -o engine.exe ./cmd/engine
```

**Run:**
```bash
./engine.exe
```

### 2. Hardware GPU Mode (OpenGL)
The GPU mode pipelines rendering directly to the graphics card. It requires a C compiler (GCC) and the `gpu` build tag.

**Build Command:**
```bash
go build -tags gpu -o engine_gpu.exe ./cmd/engine
```

**Run:**
```bash
./engine_gpu.exe
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

### Adding Application State
Modify `internal/render/renderer.go`:
```go
type ApplicationState struct {
    ClickCount int
    StatusText string
    // Add your custom variables here
}
```

### Custom CPU Components
Modify `internal/render/cpu.go`. Use the `CPUEngine.Paint` method to draw primitives using Go's `image/draw` package or the custom `drawRoundedRect` and `drawText` helpers.

### Native Window Messages
Modify `cmd/engine/main.go` in the `wndProc` function to handle more Win32 events (e.g., `WM_KEYDOWN`, `WM_SIZE`).

---

## 📜 Documentation
- **[ARCHITECTURE.md](./ARCHITECTURE.md)**: Deep-dive analysis of memory management, thread locking (`LockOSThread`), and Win32 syscall stabilization strategies.
- **[GEMINI.md](./GEMINI.md)**: Project-specific guardrails and instructions for AI-assisted development.
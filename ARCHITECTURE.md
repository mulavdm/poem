# PolyEngine Architectural Specification

This document details the core architectural challenges, thread mechanics, and memory trade-offs involved in building low-level, frameworkless user interfaces in Go.

---

## 🧠 Core Engineering Challenge: The Go Runtime vs. The OS Thread

Go is designed for massive backend concurrency using lightweight `goroutines` multiplexed across a dynamic pool of operating system threads (the `M:N` scheduler model). While highly efficient for networking, this creates a major conflict with graphical subsystems.

### ⚡ The Conundrum
Every major operating system (Windows Win32, macOS Cocoa, Linux X11/Wayland) dictates that **any window modification, device input handling, or graphics resource allocation must happen strictly on the application's main thread**. 

If Go's runtime shifts your execution context to a different OS thread mid-flight while processing user clicks or rendering frames, the operating system kernel will immediately trigger an access violation crash (`SIGSEGV`).

### 🔧 The Solution: Thread Locking
```go
func init() {
    runtime.LockOSThread()
}
```
Invoking `runtime.LockOSThread()` inside the initialization stage binds the calling goroutine exclusively to its current physical OS thread for its entire lifecycle. This stabilizes our native window event loops and protects our active graphics contexts from runtime scheduling shifts.

---

## 🛡️ Win32 Syscall Stabilization (The "Success" Trap)

A critical hurdle in building native Win32 wrappers in Go is handling the `syscall.Proc.Call` return values. In Go, these calls always return a non-nil `error` which defaults to `"The operation completed successfully"` (Errno 0).

### ⚡ The Bug
Standard Go error checking (`if err != nil`) causes panics on success if not handled carefully.

### 🔧 The Solution: Deterministic Error Wrappers
We refactored the `internal/win32` layer to respect the Win32 API contract:
```go
func CreateWindow(...) (uintptr, error) {
    ret, _, err := procCreateWindow.Call(...)
    if ret == 0 { // In Win32, 0 usually indicates failure
        return 0, err
    }
    return ret, nil // Explicitly return nil on success
}
```

---

## 🎨 Architectural Evolution: Modular Polylith

We have moved beyond a monolithic script to a professional, modular architecture using Go build tags.

### 🏗️ Component Topology
- `cmd/engine/`: The application entry point and main message loop.
- `internal/win32/`: Encapsulation of unsafe syscalls and native Windows structs.
- `internal/render/`: The rendering abstraction layer.
    - `renderer.go`: Defines the `UIRenderer` interface and `ApplicationState`.
    - `cpu.go`: Software rasterizer with advanced text support (Build Tag: `!gpu`).
    - `gpu.go`: OpenGL hardware acceleration pipeline (Build Tag: `gpu`).

### 💾 1. Advanced CPU Rasterization
In CPU mode, we implement a custom high-fidelity dashboard system:
- **Text Rendering**: Using `golang.org/x/image/font` to draw bitmap typography directly to memory.
- **Glassmorphism**: Manual alpha-blending of UI layers to create translucent "glass" panels.
- **Reactive States**: Mouse tracking (`WM_MOUSEMOVE`) allows for real-time hover states and interactive buttons.

### 🚀 2. GPU Hardware Acceleration
When compiled with `-tags gpu`, the engine shifts to the graphics card:
- **Zero CPU Blit**: Bypasses GDI entirely.
- **Parallel Pipeline**: Uses VRAM buffers and matrix transformations for fluid 60FPS animations.

🔬 **Rigorous Architectural Comparison Matrix**

| Technical Vector | CPU Strategy (`!gpu`) | GPU Strategy (`gpu`) |
| :--- | :--- | :--- |
| **External Dependencies** | Absolute Zero | `go-gl` (Hardware Drivers) |
| **Build Command** | `go build ./cmd/engine` | `go build -tags gpu ./cmd/engine` |
| **CGO Required** | No | Yes (Requires GCC) |
| **Rendering Method** | `image/draw` + `StretchDIBits` | `gl.DrawArrays` + `SwapBuffers` |
| **Primary Focus** | Portable, text-heavy tools | Animation, complex 2.5D UI |

---

## 📊 Performance Observability
The engine integrates native Go diagnostics to ensure 60FPS stability.

### PPROF Profiling
- **Server**: `http://127.0.0.1:6060/debug/pprof/`
- **CPU Profiling**: `go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=10`

### Benchmarking
- **Location**: `internal/render/cpu_test.go`
- **Command**: `go test -bench=. ./internal/render`
- **Current Baseline**: ~5.9ms per frame (CPU mode).

---

## 💎 Design for External Integration (The "Standalone Library" Goal)

PolyEngine is architected not just as a demo, but as a self-contained, library-grade GUI package. This openness ensures that any external Go program can consume it as a dependency to gain native windowing and high-performance rendering.

### 📦 Package Encapsulation Strategy
- **Public API Surface**: The `internal/render` package (intended to be moved to a public `render` path in future iterations) acts as the primary entry point. External users only interact with `UIRenderer` and `Component` interfaces.
- **Backend Independence**: A host application can initialize multiple renderers (e.g., a CPU-based log viewer and a GPU-based viewport) within the same process, switching strategies based on hardware availability or performance requirements.
- **Self-Contained Logic**: All window management (`internal/win32`) and rasterization logic are bundled within the package, requiring zero boilerplate from the host program beyond satisfying the main event loop.

## 💎 Conclusion
By isolating rendering logic behind a polymorphic interface and stabilizing the Win32 syscall layer, we've created an engine that is both safe and scalable. We can build lightweight internal utilities with zero overhead or pivot to high-performance graphics with a single build flag.

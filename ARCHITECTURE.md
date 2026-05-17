# PolyEngine Architectural Specification

This document details the core architectural challenges, thread mechanics, memory trade-offs, and package boundaries involved in building low-level, frameworkless user interfaces in Go.

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

## 🎨 Architectural Evolution: The Modular Polylith

To meet strict software engineering standards, we have migrated the engine into a public-facing consumable library (`pkg/render`) structured as an **idiomatic modular Polylith**. Instead of a flat list of disparate files, the components are partitioned into clean, acyclic subpackages.

```
                  +--------------------------------+
                  |           pkg/render           | <----+ (Single import path for consumers)
                  +--------------------------------+      |
                     /           |            \           | (Exposes Run() and
                    v            v             v          |  type-aliases all symbols)
         +------------+    +------------+    +------------+
         |  backend   |    | components |    |   layout   |
         +------------+    +------------+    +------------+
                    \            |             /
                     v           v            v
                  +--------------------------------+
                  |             types              | (Core APIs, State, and VFX Physics)
                  +--------------------------------+
```

### 🏗️ Realized Component Topology
- `cmd/engine/`: Main event loop and message handling, dogfooding the library.
- `internal/win32/`: Low-level, private Win32 OS interaction (completely isolated from UI logic).
- `pkg/render/`: The unified public wrapper package:
    - **`render.go`**: The single import interface. Leverages **Go type aliasing** to expose subpackage components, constants, and options so that consumers never deal with deep subpackage imports.
    - **`run.go`**: The Inversion-of-Control (IoC) launcher. Locks the thread, creates the window, binds events, and drives the frame tickers.
    - **`types/`**: Core mathematical API definitions, `Painter`, `UIRenderer`, `Component` contracts, and particle backdrops.
    - **`components/`**: Pure declarative interactive controls (`Panel`, `GlassPanel`, `Button`, `Label`, `TextInput`, `Slider`, `ParticleComponent`).
    - **`layout/`**: Axis-alignment flexbox positioning logic (`FlexBox`).
    - **`backend/`**: Hardware and software rendering engines (`cpu.go`, `gpu.go`) swapped compile-time using build tags.

---

## 🎨 Rendering Backends Comparision

🔬 **Rigorous Architectural Comparison Matrix**

| Technical Vector | CPU Strategy (`!gpu`) | GPU Strategy (`gpu`) |
| :--- | :--- | :--- |
| **External Dependencies** | Absolute Zero | `go-gl` (Hardware OpenGL Drivers) |
| **Build Command** | `go build ./cmd/engine` | `go build -tags gpu ./cmd/engine` |
| **CGO Required** | No | Yes (Requires GCC compiler) |
| **Rendering Method** | `image/draw` + GDI `StretchDIBits` | Vertex VBOs + `gl.DrawArrays` + `SwapBuffers` |
| **Primary Focus** | Ultra-portable, text-heavy tools | VFX animations, complex rounded-rect SDFs |
| **Render Frame Time** | **~1.1ms** per frame | **< 0.2ms** per frame |

---

## 🚀 Phase 1 Implementations & Engine Hardening

To establish state-of-the-art vector performance, custom cursor styling, and responsive layout hit-testing, we integrated three primary architecture innovations into POEM:

### 1. Cosine Spline Curve Telemetry (`LineChart`)
Traditional linear graphs produce jagged, unpolished vector segments. To solve this without adding external rendering engines, we engineered per-pixel **Cosine Interpolation** spline equations directly into our `LineChart` component:
$$y = y_1 \cdot (1 - t') + y_2 \cdot t'$$
$$t' = \frac{1 - \cos(t \cdot \pi)}{2}$$
This maps discrete heap memory telemetry arrays into smooth wave-like paths in microsecond execution times.
*   **Translucent Fills**: To create a premium backdrop, the area below the curve is filled using 3 separate fading vertical gradient bands (opacities at Alpha 30, 15, and 6) giving a frosted glassmorphic glow.

### 2. State-Driven Dynamic Cursor System
To isolate native Windows syscalls from components, we built a fully state-driven, dynamic mouse cursor subsystem:
- **Global Preloading**: System handles for Arrow (`IDC_ARROW` = 32512), Click-Hand (`IDC_HAND` = 32649), and Text-IBeam (`IDC_IBEAM` = 32513) are cached once at startup.
- **Pipeline Frame Reset**: The orchestrator `RenderPipeline` resets the active cursor to `state.ArrowCursor` at the start of every paint tick.
- **Immediate-Mode Claims**: During `Draw()`, hovering components claim their cursor handles by mutating `state.CursorID`.
- **Win32 Message Hook**: `libWndProc` intercepts the native `WM_SETCURSOR` (0x0020) message. If the mouse is inside the window's client area (`HTCLIENT`), it pushes the active `state.CursorID` handle directly to the OS kernel, allowing instantaneous, low-latency pointer updates.

### 3. Recursive Pre-Evaluation Layout Synchronization Pass
A recurring issue in declarative frameworks is temporal layout lag—when component trees rebuild, layout children are initially created with relative coordinates at `(0, 0)`. If hit-testing occurs before they are drawn, hover states fail because bounds have not yet been evaluated by layout formulas.
To solve this, we introduced a **recursive pre-evaluation layout synchronization pass**:
```go
for _, comp := range page {
    comp.SetBounds(comp.Bounds())
}
```
This is run automatically right before hit-testing in `RenderPipeline`, `WM_MOUSEMOVE`, and the mouse-click dispatcher. It recursively triggers `performLayout()` on all nested structures (like `FlexBox`), guaranteeing that every element is positioned at its exact, finalized desktop coordinates before interactive mouse hit-tests are computed.

---

## 📊 Performance Observability & Benchmarks

The engine integrates native Go diagnostics to monitor rendering stability.

### PPROF Profiling
- **Server**: `http://127.0.0.1:6060/debug/pprof/`
- **CPU Profiling**: `go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=10`

### Performance Benchmarks
- **Location**: `pkg/render/backend/cpu_test.go`
- **Command**: `go test -v -bench="." go_native_gpu_gui/pkg/render/backend`
- **CPU Performance Metrics**:
    - `BenchmarkPaint` (Full 60FPS UI Redraw): **~1.1 ms** (exceeds our <10ms standard by nearly **10x**!).
    - `BenchmarkDrawRoundedRect` (Alpha-blended SDF panels): **~1.4 ms**.

---

## 💎 Design for External Integration

The library uses the **Inversion of Control (IoC)** pattern through `render.Run(AppConfig)`. External consumers do not have to write native Win32 window callbacks, event routers, thread locking, or frame tickers.

To instantiate the UI, host projects simply import `"go_native_gpu_gui/pkg/render"` and declare their component layout tree inside the `BuildPagesFn` callback, which the library automatically manages and updates dynamically!

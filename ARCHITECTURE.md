# POEM Architectural Specification

## POEM 2.0 portability boundary

The shipping runtime remains Win32/D3D11. Platform-neutral packages under
`pkg/render` own themes, drawing values, events, semantic accessibility, layout,
and optional platform-service interfaces. Native presentation, text services,
window management, and the Windows UI Automation provider stay behind the
sidecar/platform boundary. Protocol v2 carries semantic snapshots without
embedding Windows accessibility concepts in Go components.

This document details the core architectural challenges, thread mechanics, memory trade-offs, and package boundaries involved in building low-level, frameworkless user interfaces in Go.

Current runtime note: historical sections below describe earlier single-process and Rust-sidecar experiments. The active runtime is now a Go orchestrator plus a Windows-first C++ sidecar using Win32, D3D11, named pipes, and a repo-owned binary protocol.

Current automation note: the active runtime also includes a shared automation layer in `pkg/render/automation.go`. HTTP is the recommended transport. That layer exposes component snapshots, interaction commands, native window state, self/window/desktop capture modes, and a high-level inspection flow (`/inspect-frame`) that can distinguish app-render truth from desktop-visible truth. The source-of-truth API reference for this layer is `docs/AUTOMATION.md`.

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
                  |             types              | (Core APIs, State, Semantics)
                  +--------------------------------+
```

### 🏗️ Realized Component Topology
- `cmd/engine/`: Main event loop and message handling, dogfooding the library.
- `internal/win32/`: Low-level, private Win32 OS interaction (completely isolated from UI logic).
- `pkg/render/`: The unified public wrapper package:
    - **`render.go`**: The single import interface. Leverages **Go type aliasing** to expose subpackage components, constants, and options so that consumers never deal with deep subpackage imports.
    - **`run.go`**: The Inversion-of-Control (IoC) launcher. Locks the thread, creates the window, binds events, and drives the frame tickers.
    - **`types/`**: Core API definitions, `Painter`, `UIRenderer`, `Component` contracts, platform-neutral render context, overlay state, and semantic metadata.
    - **`components/`**: Pure declarative interactive controls (`Panel`, `Button`, `Label`, `TextInput`, `TextArea`, `Slider`, selection controls, menus, tables, dialogs, and opt-in effect components). `GlassPanel` remains compatibility/effect surface, not the default app chrome.
    - **`layout/`**: Axis-alignment flexbox positioning logic (`FlexBox`).
    - **`backend/`**: Hardware and software rendering engines (`cpu.go`, `gpu.go`) swapped compile-time using build tags.

---

## 🎨 Architectural Comparison: Single-Process GDI/OpenGL vs. Process-Isolated Native Presentation

🔬 **Rigorous Architectural Comparison Matrix (Porting Metrics)**

| Architectural Vector | Original Single-Process GDI/OpenGL | Active Go + C++ D3D11 Sidecar |
| :--- | :--- | :--- |
| **Runtime Isolation** | None (Any native crash terminates Go main program) | **Process Isolation** (native presentation sidecar runs separately) |
| **Graphics API** | GDI / OpenGL Core Profile 3.3 | **Direct3D 11** in the active Windows sidecar |
| **Windowing & Input** | Custom Win32 Syscalls (Go thread-locked) | **Win32** in the active Windows sidecar |
| **IPC Strategy** | Direct heap pointer sharing (same process thread) | **Win32 Named Pipes** with a repo-owned custom binary protocol |
| **Acoustic Audio** | Win32 DSP (winmm.dll) in Go | **Native sidecar playback** via Windows multimedia APIs |
| **CGO Required** | Yes (when building `-tags gpu` with `go-gl`) | **No CGO Required** for the active Go + C++ sidecar runtime |
| **High-DPI / 4K Scaling** | None (Microscopic elements, layout coordinate clash) | **Automated Coordinate Translation Subsystem** (Logical vs. Physical) |
| **Frame Telemetry** | CPU bound (~1.1ms), thrashes Go GC on VBO arrays | **Batched protocol frames** with low-allocation Go serialization and native D3D11 presentation |

---

## 🚀 Phase 1 Implementations & Engine Hardening

To establish state-of-the-art vector performance, custom cursor styling, and responsive layout hit-testing, we integrated three primary architecture innovations into POEM:

### 1. Cosine Spline Curve Telemetry (`LineChart`)
Traditional linear graphs produce jagged, unpolished vector segments. To solve this without adding external rendering engines, we engineered per-pixel **Cosine Interpolation** spline equations directly into our `LineChart` component:
$$y = y_1 \cdot (1 - t') + y_2 \cdot t'$$
$$t' = \frac{1 - \cos(t \cdot \pi)}{2}$$
This maps discrete heap memory telemetry arrays into smooth wave-like paths in microsecond execution times.
*   **Translucent Fills**: Data visualizations may use restrained product-specific fills where they communicate information. Ordinary controls and app chrome use semantic theme tokens instead of glow or glass by default.

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

## 📜 Phase 2 Scroll Viewports & Coordinate Translation Engine

To support large lists, telemetry streams, and logging consoles, we integrated vertical scroll viewports (`render.ScrollView`) and dual-backend clipping masks.

### 1. Viewport Bounded Clipping (D3D11 Scissor Tests)
To render items inside a scroll container without them bleeding onto stationary UI elements, we introduced a native viewport clipping bounding box to the drawing tree:
* **Native GPU Scissor Tests**: The active Windows sidecar executes hardware scissor tests in D3D11 render passes.
* **Aspect Scaling Conversion**: Because the native renderer operates on physical pixels, it converts the logical scissor bounds requested by Go into physical pixels using the current display DPI scale factor. The calculated bounds are safely clamped to avoid exceeding swapchain sizes, providing zero-overhead, anti-aliased sub-frame viewport clipping.

### 2. Relative Coordinate Translation
Hit-testing and event dispatching (mouse clicks, dragging, hovers) normally operate on absolute screen coordinates. If a child component is scrolled up by `ScrollY`, it is drawn at screen coordinate `Y - ScrollY`.
To handle this transparently, the `ScrollView` recursively translates the input coordinates for all hit-test and mouse event dispatching loops:
$$\text{scrolledPt} = \text{screenPt} + (0, \text{ScrollY})$$
This translates coordinates perfectly before forwarding events, allowing standard buttons, inputs, sliders, and hovers to remain fully operational when scrolled.

### 3. Captured Drag Capturing
To prevent scrollbar dragging from stuttering when the user moves the mouse rapidly outside the scroll track, POEM locks the scroll captures using Win32 capture handles (`SetCapture`, `ReleaseCapture`).
When the scrollbar thumb is clicked, the `ScrollView` ID becomes the `ActiveID`. Subsequent mouse movements, even those moving outside the client window, continue to route raw offset deltas directly to the active `ScrollView`, ensuring a silky-smooth, glitch-free dragging feedback loop.

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

## ⚡ Phase 3 Performance Breakthroughs & Zero-GC Rendering

In Phase 3, we pushed the engine to state-of-the-art heights by identifying and eliminating low-level threading and memory bottlenecks, unlocking rock-solid 60Hz+ performance on both CPU and GPU backends.

### 1. Asynchronous Thread repaints (`PostMessage` Dummy Wake-up Loop)
To drive real-time background animations, a background thread runs an update ticker at 60Hz. Calling `win32.UpdateWindow(hwnd)` from this background thread forced a synchronous inter-thread `SendMessage` to the main UI thread, blocking the background ticker until the main thread finished rendering and dropping the tick rate to 40Hz.
* **The Solution**: We decoupled the threads by updating `triggerRepaint` to perform an asynchronous `InvalidateRect` (marking the window dirty) followed by a native **`PostMessage(hwnd, WM_NULL, 0, 0)`** call.
* **How it works**: `PostMessage` places the dummy `WM_NULL` message in the main thread's queue asynchronously and returns instantly. The main thread's `GetMessage` loop wakes up immediately, processes and ignores the `WM_NULL` message, and then—seeing the dirty update region—natively generates a high-priority `WM_PAINT` cycle! This yields perfect, unblocked asynchronous execution.

### 2. Zero-GC Allocation Draw Command Buffer
Building declarative UI lists and real-time telemetry graphs requires clearing and rebuilding hundreds of components and draw calls every frame. Allocating fresh slices for paint instructions on every frame would thrash Go's Garbage Collector.
* **The Solution**: We engineered a slice-retaining strategy inside `ProtocolPainter`:
  ```go
  func (p *ProtocolPainter) Reset() {
      p.commands = p.commands[:0]
      ...
  }
  ```
  Calling `commands[:0]` resets the length of the command slice to zero while
  preserving the underlying allocated backing array capacity. Subsequent appends
  copy drawing commands into retained memory, keeping redraw ticks low-allocation
  and predictable.

### 3. High-Precision Floating-Point Delta Physics
Unlocking unthrottled 60Hz+ performance dropped the frame delta `dt` to exactly `0.016` seconds. Because particle coordinates were stored as integers, calculating steps like `int(velocity * dt)` (e.g. `int(40 * 0.016) = int(0.64) = 0`) truncated the step size to exactly `0` every frame, causing the background particles to freeze solid as soon as performance became perfect!
* **The Solution**: We refactored `particles.go` to store and accumulate all coordinates, velocities, and physics steps using high-precision `float64` variables. The values are only cast to integers at the final drawing phase, ensuring gorgeous, fluid animations at any framerate.

---
## 💎 Design for External Integration

The library uses the **Inversion of Control (IoC)** pattern through `render.Run(AppConfig)`. External consumers do not have to write native Win32 window callbacks, event routers, thread locking, or frame tickers.

To instantiate the UI, host projects simply import `"go_native_gpu_gui/pkg/render"` and declare their component layout tree inside the `BuildPagesFn` callback, which the library automatically manages and updates dynamically!

---

## ⌨️ Keyboard Focus & Global Hotkey Engine Architecture

The framework features a dedicated, low-latency, and highly decoupled keyboard controller integrated directly into the native Win32 message procedure (`libWndProc`).

### 1. Sequential Focus Cycling Pass (`CycleFocus`)
Focus navigation is managed sequentially via `Tab` and `Shift+Tab`. To prevent legacy state corruption, `CycleFocus`:
1. Traverses components declared inside the active page registry (`s.Pages[s.CurrentPage]`) instead of a static global registry.
2. Recursively walks the component hierarchy to collect all interactive widgets returning `Focusable() bool { return true }` (e.g. `Button`, `TextInput`, `Slider`).
3. Determines the index of the currently focused widget ID, shifts the focus pointer forward or backward (clamped to slice boundaries), and triggers a repaint.

### 2. Polymorphic Scroll Centering (`ScrollContainer`)
To automatically glide focused items into view when navigating massive lists, we established the `types.ScrollContainer` interface:
```go
type ScrollContainer interface {
    Component
    ScrollToChild(childID string, childBounds image.Rectangle, state *ApplicationState) bool
}
```
* **Circular Import Avoidance**: By defining this interface inside `pkg/render/types` instead of `pkg/render/components`, we keep our package layout clean and fully compliant with Go's package tree dependencies.
* **Centered Centering Math with Boundary Spacing**: `ScrollView` implements `ScrollToChild`. It dynamically queries if the target child is a descendant, computes its Y bounds relative to the viewport top coordinate (independent of LERP visual scroll offsets), and shifts `ScrollY` up or down to center the element inside the visible viewport. Boundary padding prevents focused elements and their theme-defined focus outlines from clipping against viewport edges.

### 3. Semantic Focus Painter Integration
Focused interactive elements use the current theme's focus token. Inside `Button.Draw`, `TextInput.Draw`, and `Slider.Draw`:
- We query `state.FocusedID == CompID`.
- Core controls draw a restrained outline outside the control boundary using `Theme.Colors.Focus`.
- Glow remains available to application-specific effects but is not mandatory control styling.

### 4. Declarative Keyboard Adjustments & Event Submissions
Interactive components capture specialized keystroke operations:
- **`Button.OnKey`**: Intercepts virtual key `Enter` (`VK_RETURN` = 13) and `Space` (`VK_SPACE` = 32) only during `WM_KEYDOWN` key events, running the button's `OnClick` closure. This filters out the duplicate `WM_CHAR` character translations (such as character `\r`) that would otherwise double-trigger the buttons and instantly negate toggles.
- **`Slider.OnKey`**: Captures `Left Arrow` (`VK_LEFT`) and `Right Arrow` (`VK_RIGHT`) keys to mathematically increment or decrement the slider value. It snaps values to the nearest 5% increment (`math.Round((Value ± step) / step) * step`) to clean up any precise decimal offsets left behind from custom mouse dragging.
- **`TextInput.OnKey`**: Supports submission, rune-index selection, word navigation, replacement editing, and clipboard shortcuts. Windows IME pre-edit strings arrive through explicit composition events rather than duplicate `WM_CHAR` insertion.

### 5. Win32 Key Dispatching Loop
Inside the native `libWndProc` under `WM_KEYDOWN` (0x0100):
- **Escape (`VK_ESCAPE`)**: Resets `globalState.FocusedID = ""` to clear keyboard capture instantly.
- **Tab Cycling**: Checks modifier states via `win32.GetKeyState(VK_SHIFT) < 0` to trigger forward/reverse cycling.
- **Modifier Shortcuts (Ctrl+S)**: Detects Left, Right, and Generic Control modifier pressed states simultaneously. Queries registered global listeners from the new `state.Hotkeys` map and fires handlers.
- **Engine Keylogger**: Logs keystroke virtual key codes and active Control modifiers to standard output in real-time, providing immediate visibility during debugging.
- **Event Propagation**: Dispatches keys directly to the focused component's `OnKey(...)` method, facilitating modular key handling.

---

## IX. Acoustic Native Sound Engine (Phase 4)

To deliver a premium, multi-sensory GUI experience, POEM integrates a native **Acoustic Native Sound Engine** that generates and plays real-time synthesized waveforms asynchronously without external dependencies or blocking the main drawing/rendering thread.

```
+------------------------------------+
|       internal/win32/audio.go      | <--- Pure Go 16-bit Mono PCM WAV Synthesizers
+------------------------------------+
                  | (WAV Bytes)
                  v
+------------------------------------+
|       pkg/render/types/audio.go    | <--- Pre-allocated buffer memory on ApplicationState
+------------------------------------+      (PlayHover / PlayClick / PlaySuccess API)
                  | (unsafe Pointer)
                  v
+------------------------------------+
|       internal/win32/win32.go      | <--- PlaySoundW Win32 API binding (winmm.dll)
+------------------------------------+
```

### 1. Zero-Dependency Waveform DSP Synthesis (`internal/win32/audio.go`)
We constructed a pure mathematical oscillator in Go that generates correct little-endian 16-bit Mono PCM WAV streams entirely in RAM:
* **Binary WAV Header Builder**: Dynamically builds the standard 44-byte RIFF/WAVE header (specifying sample rate, block alignment, audio format, and subchunk sizes).
* **Acoustic Waveforms**:
  - **Hover Tick**: A clean sine wave at `1200.0` Hz, lasting `15` milliseconds. Applies an extremely rapid decay envelope (`math.Exp(-t * 220.0)`) for a soft, ultra-responsive interactive cursor click.
  - **Click Chime**: A rich dual-frequency chord combining a `900.0` Hz fundamental with a `1800.0` Hz octave harmonic. Blended and shaped over a `120` millisecond decay envelope (`math.Exp(-t * 28.0)`) for crisp tactile feedback.
  - **Success Melody**: An ascending sci-fi arpeggio sequence spanning `E5` (659.25 Hz), `A5` (880.0 Hz), `C#6` (1109.73 Hz), and `E6` (1318.51 Hz) over a `400` millisecond span.

### 2. Low-Level winmm.dll Bindings (`internal/win32/win32.go`)
We load Windows' low-level multimedia API (`winmm.dll`) and bind its `PlaySoundW` procedure:
* **Memory Playback**: Plays sound files directly from RAM buffers using standard flag combinations:
  - `SND_MEMORY` (`0x0004`): Tells Windows that `pszSound` points to a WAV image loaded in RAM.
  - `SND_ASYNC` (`0x0001`): Starts playback asynchronously and returns immediately.
  - `SND_NODEFAULT` (`0x0002`): Prevents playing default system beep errors in case of failures.
* **Garbage Collection Safety**: The synthesized WAV slices are stored permanently inside `globalState` during application boot. This guarantees that Go's garbage collector never relocates or collects the memory addresses while active Win32 threads are playing them.

### 3. Decoupled Interface Contract (`pkg/render/types/audio.go`)
To strictly follow our **Polylith Architectural Philosophy**, low-level `unsafe` pointers and Win32 interop are hidden behind the `ApplicationState` abstraction:
* Exposes clean public methods `PlayHover()`, `PlayClick()`, and `PlaySuccess()` on `ApplicationState`.
* Allows application layouts (`ui.go`) or components to trigger audio feedback declaratively without importing `internal/win32` or utilizing pointer arithmetic.

### 4. Interactive Mute, Rate-Limiting & Hover Filtering Hooks
Sound triggers are carefully routed inside the core window message loop to optimize user experience:
* **Interactive Hover Filtering**: Hooked in `WM_MOUSEMOVE` in `run.go`. Triggers a hover tick ONLY when the cursor enters a *new* component bounding box (`newHover != "" && newHover != globalState.HoveredID`) AND queries that the target component is focusable (`comp.Focusable() == true`). This guarantees that static text, panels, or layout headers remain silent, focusing ticks exclusively on interactive button, slider, and input fields.
* **Auto-Repeat Debouncing Cooldowns**: Exposed dynamic tracking timestamps (`LastHoverTime`, `LastClickTime`, `LastSuccessTime`) to rate-limit playback:
  - **Hovers**: 50ms cooldown gate.
  - **Clicks**: 150ms cooldown gate.
  - **Success Chimes**: 500ms cooldown gate.
  This completely filters Win32 keyboard auto-repeat message floods (e.g. when holding `Enter` or `Ctrl+S`), ensuring single chimes play cleanly without overlapping audio stutter.
* **Button Clicks**: Hooked inside `WM_LBUTTONDOWN`, playing a click chime when clicking any active focusable component boundary.
* **Global Save & Input Submit**: Plays the success arpeggio on global `"Ctrl+S"` save signals and console command line submissions.
* **Opt-in effects**: Audio feedback is now controlled through `AppConfig.Effects` and remains an application choice rather than a core-control default.

---

## 🖥️ X. Process Isolation & High-DPI Coordinate Translation Subsystem

POEM implements a modern **process-isolated dual-runtime architecture**. The Go orchestrator manages business logic, layout generation, and state metrics, while driving a native presentation sidecar over local IPC. The current Windows runtime uses a C++ sidecar built on Win32 and D3D11 with a repo-owned binary protocol over dual named pipes. To ensure pixel-perfect rendering across varied screen resolutions, POEM incorporates a fully automated **High-DPI Coordinate Translation Subsystem**.

### 1. The High-DPI Engineering Challenge
On high-DPI displays (e.g., 4K monitors with 150%–250% system scaling), rendering layouts at a 1:1 pixel ratio causes the entire UI to appear microscopic. Standard operating system window managers automatically scale windows, but low-level hardware-accelerated drawing surfaces must manually adjust their projection and scissor boundaries.

However, forcing the layout engine to compute high-DPI coordinates causes dynamic layout code to become highly complex and prone to visual alignment bugs.

### 2. The Solution: Dual Coordinate Spaces
POEM segregates the framework into two distinct coordinate systems:
1. **Logical Coordinate Space (Go Orchestrator)**: The layout matrix, event boundaries, padding, rounding, and vector coordinates run strictly in a virtual **Logical Space** (e.g. standard 1024x768 units).
2. **Physical Coordinate Space (Windows D3D11 Sidecar)**: Native presentation resources, render targets, and swapchains run in the device's **Physical Pixel Space** (e.g. 2048x1536 pixels on 200% scaling).

### 3. Dynamic Orthographic Projection Translation
To bridge these coordinate spaces, the sidecar scales its render projection using the current physical size and DPI factor:
```rust
let left = 0.0;
let right = physical_width as f32 / scale_factor;
let bottom = physical_height as f32 / scale_factor;
let top = 0.0;
```
This maps logical layout coordinates into the sidecar's native render space while preserving correct physical sizing.

```
       Go Orchestrator                     Windows D3D11 Sidecar
+----------------------------+      +------------------------------+
| Logical Space (1024x768)   |      | Physical Pixel Space (4K)   |
|                            |      |                              |
| - Layout flex calculations |      | - D3D11 swapchain            |
| - Mouse hit-test loops     |      | - Native render targets      |
| - Padding & SDF bounds     |      | - Scissor clipping rects     |
+----------------------------+      +------------------------------+
              |                                     ^
              | POEM Protocol Frames                | Dynamic Scale
              v                                     | (scale_factor)
      [ DrawCommand(x,y) ] -------------------------+
```

### 4. Physical Presentation Alignment
The Windows sidecar receives logical draw commands from Go and applies the
current DPI scale when configuring projection, scissor, and presentation bounds.
Using physical swapchain dimensions for native presentation prevents visual
stretching, offset shifts, and coordinate mismatches on scaled displays.

### 5. Physical Scissor Clipping Boundaries
Scissor tests (`set_scissor_rect`) operate on physical hardware-level boundaries. If Go requests clipping in a ScrollView at logical `x, y, w, h`, these coordinates are scaled up by `scale_factor` in the renderer before applying the GPU scissor:
$$\text{clip}_{\text{physical}} = \text{clip}_{\text{logical}} \cdot \text{scale\_factor}$$
This ensures that the Settings scrollview is cropped perfectly down to the physical pixel boundary, completely resolving scrolling clip-box bugs on high-density displays.

### 6. Bidirectional Event Scaling
To keep the Go orchestrator decoupled from scaling metrics:
* **Cursor Movements**: Raw native cursor positions are divided by the active DPI scale before being sent back to Go for precise hit-testing.
* **Scroll Deltas**: Pixel-based wheel scroll offsets from `MouseWheel` are divided by `scale_factor` to maintain uniform scrolling sensitivity across all screens.
* **Window Resizes**: Physical window resizes are divided by the scale factor, keeping Go aware of the true logical grid bounds.

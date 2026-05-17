# P.O.E.M. Framework Expansion Roadmap

This document outlines the strategic progression milestones for expanding the POEM Operational Engine Matrix library into a state-of-the-art, multi-sensory GUI development kit.

---

## 🗺️ Strategic Milestones Matrix

| Phase | Feature Vector | Principal Package Bounds | Status | Architectural Impact |
| :--- | :--- | :--- | :--- | :--- |
| **Phase 1** | **📉 Real-Time Performance Charting** | `pkg/render/components/chart.go`, `pkg/render/types/types.go` | **[x] COMPLETED** | **Sleek Data Observability:** Provides a native `render.LineChart` component with Cosine spline smoothing, glowing alpha gradients, dynamic OS cursors, and layout synchronization. |
| **Phase 2** | **📜 Dynamic Scroll Viewports** | `pkg/render/layout/scroll.go`, `pkg/render/components/scroll.go` | `[ ] PLANNED` | **Massive Content Rendering:** Allows layouts to scroll vertically with relative offset coordinates, catching mouse wheel actions (`WM_MOUSEWHEEL`), and drawing a glassmorphic sliding scrollbar. |
| **Phase 3** | **⌨️ Accessibility Focus & Hotkeys** | `pkg/render/types/focus.go`, `pkg/render/run.go` | `[ ] PLANNED` | **Power-User Productivity:** Implements a global Keyboard Controller managing standard submission hotkeys (`Ctrl+S`, `Enter`, `Esc`), sequential tab navigation, and focus capture. |
| **Phase 4** | **🎛️ Acoustic Native Sound Engine** | `internal/win32/audio.go`, `pkg/render/types/audio.go` | `[ ] PLANNED` | **Sleek Audio Feedback:** Binds low-level wave synthesis (`waveOut`) to Win32 threads to output lightweight click and hover chime feedback without blocking the main event loops. |

---

## 🛠️ Phase 1 Technical Specification & Verification Summary: `render.LineChart`

The first expansion phase replaced static load chart placeholders in POEM and downstream projects with a high-fidelity vector curve plotting component, alongside a dynamic cursor subsystem.

### 📐 Component Design
The component is fully declarative, reading telemetry data feeds directly from the application state:
```go
type LineChart struct {
    CompID    string
    Rect      image.Rectangle
    BGColor   color.RGBA
    LineColor color.RGBA
    Data      []float32
    Title     string
    Rounding  int
}
```

### 🧠 Real-Time Telemetry Feeds
We expanded the universal `ApplicationState` in `pkg/render/types/types.go` to capture telemetry historical circular buffers directly from Go's `runtime.ReadMemStats` and frame duration counters:
```go
type ApplicationState struct {
    ...
    FPSHistory  []float32 // Track frame rates
    HeapHistory []float32 // Track memory usage in MB
}
```

### 🎨 Completed Rendering & Interaction Deliverables
1. **Cosine Splines Spline Curve Interpolation**: Replaced jagged linear segments with beautiful, wave-like curves calculated using per-pixel $y = y_1(1-t') + y_2(t')$ trigonometric splines.
2. **Glowing Background Fills**: Added a 3-layer translucent glow gradient under the curve (descending opacities at Alpha 30, 15, and 6) giving a gorgeous glassmorphic look.
3. **Dynamic Windows Cursors**: preloaded system handles once at boot, resetting the handle every frame and reactively updating to a link-hand (`IDC_HAND`) or text-IBeam (`IDC_IBEAM`) based on element hovers via native Win32 `WM_SETCURSOR` intercepts.
4. **Layout Synchronization Pass**: Resolved layout latency by recursively executing a pre-evaluation layout synchronization pass (`comp.SetBounds(comp.Bounds())`) immediately before executing hit-testing checks, ensuring child elements nested inside complex containers (like `FlexBox` headers and sidebar nav columns) react dynamically to hovers and clicks.

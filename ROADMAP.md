# P.O.E.M. Framework Expansion Roadmap

This document outlines the strategic progression milestones for expanding the POEM Operational Engine Matrix library into a state-of-the-art, multi-sensory GUI development kit.

---

## 🗺️ Strategic Milestones Matrix

| Phase | Feature Vector | Principal Package Bounds | Architectural Impact |
| :--- | :--- | :--- | :--- |
| **Phase 1** | **📉 Real-Time Performance Charting** | `pkg/render/components/chart.go`, `pkg/render/types/types.go` | **Sleek Data Observability:** Provides a native `render.LineChart` component utilizing high-fidelity vector lines to display real-time analytics (FPS, Memory heap) without external chart engines. |
| **Phase 2** | **📜 Dynamic Scroll Viewports** | `pkg/render/layout/scroll.go`, `pkg/render/components/scroll.go` | **Massive Content Rendering:** Allows layouts to scroll vertically with relative offset coordinates, catching mouse wheel actions (`WM_MOUSEWHEEL`), and drawing a glassmorphic sliding scrollbar. |
| **Phase 3** | **⌨️ Accessibility Focus & Hotkeys** | `pkg/render/types/focus.go`, `pkg/render/run.go` | **Power-User Productivity:** Implements a global Keyboard Controller managing standard submission hotkeys (`Ctrl+S`, `Enter`, `Esc`), sequential tab navigation, and focus capture. |
| **Phase 4** | **🎛️ Acoustic Native Sound Engine** | `internal/win32/audio.go`, `pkg/render/types/audio.go` | **Sleek Audio Feedback:** Binds low-level wave synthesis (`waveOut`) to Win32 threads to output lightweight click and hover chime feedback without blocking the main event loops. |

---

## 🛠️ Phase 1 Technical Specification: `render.LineChart`

The first expansion focus is to replace static load chart placeholders in POEM and downstream projects with a high-fidelity vector curve plotting component.

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
We will expand the universal `ApplicationState` in `pkg/render/types/types.go` to capture telemetry historical circular buffers (capped at 100 nodes) directly from Go's `runtime.ReadMemStats` and frame duration counters:
```go
type ApplicationState struct {
    ...
    FPSHistory  []float32 // Track frame rates
    HeapHistory []float32 // Track memory usage in MB
}
```

### 🎨 Rendering Algorithm
1. **Backdrop Card**: Renders a rounded background card (using `p.DrawRoundedRect` or `p.FillRect`).
2. **Title Heading**: Places a diagnostic title heading and scale boundaries (`p.DrawText`).
3. **Vector Curve Calculation**:
   - Spreads the $X$ coordinates evenly across the card width.
   - Maps the $Y$ values relative to the maximum and minimum points in the telemetry slice.
   - Iterates through the calculated coordinates, drawing anti-aliased segment bridges using `p.DrawLine(x1, y1, x2, y2, col)`.

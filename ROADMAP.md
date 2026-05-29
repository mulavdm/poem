# P.O.E.M. Framework Expansion Roadmap

This document outlines the strategic progression milestones for expanding the POEM Operational Engine Matrix library into a state-of-the-art, multi-sensory GUI development kit.

---

## 🗺️ Strategic Milestones Matrix

| Phase | Feature Vector | Principal Package Bounds | Status | Architectural Impact |
| :--- | :--- | :--- | :--- | :--- |
| **Phase 1** | **📉 Real-Time Performance Charting** | `pkg/render/components/chart.go` | **[x] COMPLETED** | **Sleek Data Observability:** Provides a native `render.LineChart` component with Cosine spline smoothing, glowing alpha gradients, dynamic OS cursors, and layout synchronization. |
| **Phase 2** | **📜 Dynamic Scroll Viewports** | `pkg/render/components/scroll.go` | **[x] COMPLETED** | **Massive Content Rendering:** Allows layouts to scroll vertically with relative offset coordinates, catching mouse wheel actions (`WM_MOUSEWHEEL`), and drawing a glassmorphic sliding scrollbar. |
| **Phase 3** | **⌨️ Accessibility Focus & Hotkeys** | `pkg/render/types/types.go`, `pkg/render/run.go` | **[x] COMPLETED** | **Power-User Productivity:** Implements a global Keyboard Controller managing standard submission hotkeys (`Ctrl+S`, `Enter`, `Esc`), sequential tab/shift-tab focus cycling, and focus outline glow rings. |
| **Phase 4** | **🎛️ Acoustic Native Sound Engine** | `internal/win32/audio.go`, `pkg/render/types/audio.go` | **[x] COMPLETED** | **Sleek Audio Feedback:** Binds low-level wave synthesis (`winmm.dll`) to play real-time synthesized memory PCM waveforms (hover ticks, click chimes, success arpeggios) asynchronously with debouncing. |
| **Phase 5** | **🎨 Fluid Page Transitions & Spline LERP Animations** | `pkg/render/run.go`, `pkg/render/types/types.go` | **[ ] PLANNED** | **Cinema-Grade Motion Graphics:** Implements dynamic Ease-InOut cubic spline LERP interpolations for page transitions. Pages slide and fade smoothly instead of switching instantly. |
| **Phase 6** | **🛠️ Reactive JSON Configuration State Persistence** | `pkg/render/types/persistence.go` | **[ ] PLANNED** | **Persistent Telemetry Profiles:** Automatically saves active preferences (Blur, Mute, Volume, Custom Input Fields) into `poem_profile.json` upon global save (`Ctrl+S`) or page changes, and re-hydrates state at boot. |
| **Phase 7** | **📊 Advanced Glowing Bar & Radial Charts** | `pkg/render/components/radial.go`, `pkg/render/components/bar.go` | **[ ] PLANNED** | **Premium Infographic Visuals:** Expands POEM's data visualization kit by adding glowing radial progress meters (circular arcs with neon styling) and layered glassmorphic vertical bar charts. |

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

### 🎨 Rendering & Interaction Deliverables
1. **Cosine Splines Curve Interpolation**: Replaced jagged linear segments with beautiful, wave-like curves calculated using per-pixel $y = y_1(1-t') + y_2(t')$ trigonometric splines.
2. **Glowing Background Fills**: Added a 3-layer translucent glow gradient under the curve (descending opacities at Alpha 30, 15, and 6) giving a gorgeous glassmorphic look.
3. **Dynamic Windows Cursors**: Preloaded system handles once at boot, resetting the handle every frame and reactively updating to a link-hand (`IDC_HAND`) or text-IBeam (`IDC_IBEAM`) based on element hovers via native Win32 `WM_SETCURSOR` intercepts.
4. **Layout Synchronization Pass**: Resolved layout latency by recursively executing a pre-evaluation layout synchronization pass (`comp.SetBounds(comp.Bounds())`) immediately before executing hit-testing checks, ensuring child elements nested inside complex containers (like `FlexBox` headers and sidebar nav columns) react dynamically to hovers and clicks.

---

## 🛠️ Phase 2 Technical Specification & Verification Summary: `render.ScrollView`

The second expansion phase added high-performance vertical scroll containers (`render.ScrollView`), bounding box clippers, and native event propagation.

### 📐 Component Design
The `ScrollView` wraps any complex child layout structure (typically a single vertical `FlexBox` containing numerous sub-components):
```go
type ScrollView struct {
    CompID     string
    Rect       image.Rectangle
    Children   []types.Component
    ScrollY    int
    CurrentScrollY int // Smoothly animated/interpolated scroll offset
    ContentH   int
    ScrollbarW int
}
```

### 🎨 Rendering & Clipping Deliverables
1. **Painter-Level Scissor/Clipping**: Added `SetClip(image.Rectangle)` to the core `Painter` interface. On GDI (`CPUEngine`), pixel drawing is mathematically restricted within bounds. On OpenGL (`GPUEngine`), native Scissor Tests (`gl.Enable(gl.SCISSOR_TEST)`, `gl.Scissor`) isolate the rendering layout without bleeding into stationary sections.
2. **GPU Shape Projection Bug Fix**: Offsets the coordinate bounding `params` in GPU `drawQuad` to ensure SDF rounded corner formulas align perfectly at scrolled coordinates.
3. **Glassmorphic Scrollbars**: Renders an elegant semi-translucent track (`color.RGBA{255, 255, 255, 10}`) and a draggable thumb that glows neon green (`color.RGBA{0, 255, 150, 150}`) on hover or drag.
4. **Native Win32 Mouse Wheel Capture**: Registered `WM_MOUSEWHEEL` (0x020A) inside the main `libWndProc` message procedure to capture scrolling direction and scroll offset increments dynamically.
5. **Coordinate Translation**: Translates screen coordinates to scrolled layout coordinates recursively, ensuring hovered states, clicked buttons, sliders, and inputs inside the scrolling panel remain fully reactive.

---

## 🛠️ Phase 3 Technical Specification & Verification Summary: Focus & Hotkeys

The third expansion phase added keyboard accessibility, sequential focus navigation, and custom hotkey captures.

### ⌨️ Features & Controls
1. **Focus Cycling Math**: Recursive Walk collects all interactive elements satisfying `comp.Focusable() == true`. Loops forward (Tab) and backward (Shift+Tab) clamp index pointers and trigger animated centering transitions.
2. **Glow Focus Outline rings**: Focusable components (`Button`, `TextInput`, `Slider`) paint a 2px offset border in neon color with custom `p.SetGlow(6.0)` styling when active.
3. **Key Event Propagation**: Keyboard actions are routed inside `run.go`. Standard keys propagate directly to focused widgets (enabling sliders to snap in 5% snapping increments, buttons to click, and text inputs to submit).
4. **Single-Trigger Key Filters**: Resolved duplicate virtual-key/character triggers (`key == 13` vs `char == '\r'`) by routing button presses strictly on `WM_KEYDOWN` virtual key inputs.

---

## 🛠️ Phase 4 Technical Specification & Verification Summary: Acoustic Sound Engine

The fourth expansion phase designed and implemented the zero-dependency Native Acoustic Sound Engine.

### 🎛️ Synthesis & Playback Deliverables
1. **Low-Level winmm.dll bindings**: Loaded Windows' standard multimedia player `winmm.dll` and exposed the `PlaySoundW` procedure along with memory-resident asynchronous flags (`SND_ASYNC`, `SND_MEMORY`, `SND_NODEFAULT`).
2. **Mathematical Audio DSP Synthesizer**: Structures complete 16-bit Mono 44100Hz PCM WAV arrays directly in RAM, complete with binary 44-byte WAV headers.
3. **Sound Profiles**:
   - **Hover Tick**: A short `1200Hz` tick with rapid exponential decay (`15ms` duration).
   - **Click Chime**: A dual-frequency `900Hz`/`1800Hz` tactile chord with smooth decay (`120ms` duration).
   - **Success Melody**: An ascending sci-fi arpeggio sequence spanning `E5`, `A5`, `C#6`, and `E6` (`400ms` duration).
4. **Auto-Repeat Rate Limiting**: Added debouncing timestamps (`LastHoverTime`, `LastClickTime`, `LastSuccessTime`) to `ApplicationState` to limit playing rates, completely filtering Win32 keyboard repeat repeat noise.
5. **Hover Noise Filtering**: Restressed the hover trigger to play only when cursor moves over a focusable (interactive) element, silencing backgrounds and static text.

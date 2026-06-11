# POEM UI Development Guide (Quickstart)

Welcome to the **P.O.E.M. Operational Engine Matrix (POEM)** UI framework! This guide will walk you through the declarative UI architecture, component catalog, layout system, reactive state loops, and shared automation hooks so you can start building premium, high-performance, glassmorphic interfaces.

## Automation Quickstart

POEM can optionally expose a shared localhost automation server for downstream apps.

Enable it through `render.AppConfig.Automation`:

```go
render.Run(render.AppConfig{
    Title:        "My Downstream POEM App",
    Width:        800,
    Height:       600,
    BuildPagesFn: BuildAllPages,
    Automation: &render.AutomationConfig{
        Enabled:    true,
        Mode:       "http",
        Host:       "127.0.0.1",
        Port:       47831,
        CaptureDir: "output/automation",
    },
})
```

Recommended endpoints:

- `GET /state`
- `GET /components`
- `POST /click`
- `POST /set-text`
- `POST /prepare-window`
- `POST /inspect-frame`

Use `POST /inspect-frame` when you want the safest "show me the app now" PNG for debugging or agent inspection. It prefers a desktop-visible capture only when the window is actually foregrounded, and otherwise falls back to the app's own self-frame.

Full details live in [docs/AUTOMATION.md](./docs/AUTOMATION.md).

---

## Layout Measurement Quickstart

POEM `FlexBox` is not browser-grade intrinsic flexbox. It is a native layout helper where children still contribute preferred sizing information and the parent places them.

The preferred path now is:

1. use explicit rects for major panes or dashboard regions
2. let leaf controls provide preferred size through `Measure(avail, state)`
3. let `FlexBox` consume measured size before falling back to raw `Bounds()`
4. let scrollable containers use `ContentSize(avail, state)` when assigned bounds are smaller than laid-out content

### Native measurement contract

Components can optionally implement:

```go
Measure(avail image.Point, state *render.ApplicationState) render.MeasureResult
```

Where `MeasureResult` provides:

- `Preferred image.Point`
- `Min image.Point`

This is a POEM-native sizing contract, not a CSS model. It does not currently introduce concepts like percentages, flex-basis, or browser intrinsic content negotiation.

Scrollable containers can also consume:

```go
ContentSize(avail image.Point, state *render.ApplicationState) image.Point
```

Use this when a component's assigned `Bounds()` describes the visible box, but its laid-out content is taller or wider. `render.ScrollView` uses this distinction so fixed viewport bounds do not accidentally erase scrollable content.

### Downstream migration checklist

- prefer explicit container rects for major panes
- rely on `Measure(...)` for buttons, labels, inputs, text blocks, image views, and scroll viewports
- rely on `ScrollView` for intentional overflow instead of allowing children to draw through sibling regions
- avoid stale oversized `Bounds()` values as a stand-in for wrapped or clipped content
- cap long status text and chips explicitly when sharing a header row
- preserve explicit component rects when the layout is intentionally fixed

---

## 🏗️ 1. Core Architecture & Reactive Loop

POEM uses an elegant, process-isolated **re-evaluation loop** driven over local IPC. Instead of manually updating widgets, you write a **Page Builder function**.

The lifecycle works as follows:
1. The native **presentation sidecar** captures low-level window interactions and transmits them back to Go.
2. The **Go orchestrator** processes these events, mutates the global `ApplicationState`, and invokes your **Page Builder function** to rebuild the component hierarchy from scratch.
3. Go serializes the new drawing commands using the repo-owned binary protocol in `pkg/render/protocol`.
4. The sidecar processes the frame asynchronously and flushes draw calls to the GPU.

```
+---------------------+   Input Event Batch        +-------------------+
| Native Sidecar Core | -------------------------> |  Go Orchestrator  |
| (Win32 / D3D11 now) | <------------------------- |    (Game Loop)    |
+---------------------+   Render / Sound Commands  +-------------------+
                                                             |
                                                             v Triggers
                                                   +-------------------+
                                                   |  BuildPagesFn()   |
                                                   | (Rebuilds UI tree)|
                                                   +-------------------+
```

---

## 🎨 2. Declarative Component Catalog

All elements are exposed directly under the `render` package namespace.

### A. Static & Glassmorphic Panels
Panels form the background containers and cards of your application.

```go
// 1. Solid Panel
&render.Panel{
    CompID:   "solid_card",
    Rect:     image.Rect(100, 100, 300, 400),
    BGColor:  color.RGBA{20, 25, 40, 255},
    Rounding: 12, // Curvature radius
}

// 2. Glassmorphic Translucent Panel
&render.GlassPanel{
    Panel: render.Panel{
        CompID:   "glass_card",
        Rect:     image.Rect(320, 100, 520, 400),
        BGColor:  color.RGBA{30, 35, 55, 255},
        Rounding: 15,
    },
    Opacity: 160, // 0 (invisible) to 255 (opaque)
}
```

### B. Typography & Dynamic Labels
Labels display text. Dynamic labels automatically query `ApplicationState` to update their strings in real-time.

```go
// 1. Static Label
&render.Label{
    CompID: "static_header",
    Pos:    image.Point{120, 140}, // Top-Left position
    Text:   "DASHBOARD",
    Color:  color.RGBA{255, 255, 255, 255},
}

// 2. Dynamic Label (Updates text on every paint frame)
&render.DynamicLabel{
    CompID: "telemetry_fps",
    Pos:    image.Point{120, 180},
    Color:  color.RGBA{0, 255, 150, 255},
    GetText: func(state *render.ApplicationState) string {
        return fmt.Sprintf("FPS: %.1f // Latency: %.2fms", state.CurrentFPS, float64(state.FrameTime.Microseconds())/1000.0)
    },
}
```

### C. Buttons & Interactions
Buttons capture click actions via a callback hook.

```go
&render.Button{
    CompID:     "btn_reboot",
    Rect:       image.Rect(120, 220, 380, 270),
    Label:      "REBOOT CORE",
    BaseColor:  color.RGBA{180, 60, 60, 255},
    HoverColor: color.RGBA{220, 80, 80, 255}, // Auto-applied on mouseover
    Rounding:   8,
    OnClick: func(state *render.ApplicationState) {
        state.ClickCount++
        state.StatusText = "Engine reboot initiated!"
    },
}
```

### D. Real-Time Range Sliders
Sliders are perfect for adjustments (volume, frequency, thresholds). Dragging the slider automatically updates its percentage.

```go
&render.Slider{
    CompID:     "volume_slider",
    Rect:       image.Rect(120, 300, 380, 325),
    Min:        0,
    Max:        100,
    Value:      state.Volume, // Binds current state
    TrackColor: color.RGBA{10, 10, 20, 255},
    ThumbColor: color.RGBA{0, 150, 255, 255},
}
```

### E. User Text Inputs
Fully functional text-input controls capturing focused keyboard strokes.

```go
&render.TextInput{
    CompID:      "input_node_name",
    Rect:        image.Rect(120, 350, 380, 380),
    Placeholder: "Enter node identity...",
    BGColor:     color.RGBA{10, 10, 20, 255},
    TextColor:   color.RGBA{255, 255, 255, 255},
    Rounding:    5,
}
```

### F. Real-Time Telemetry Line Charts
A high-fidelity vector component that plots historical numerical datasets in real-time. Features automated per-pixel **Cosine Interpolation** spline-smoothing and a **3-layer translucent glowing background area fill**.

```go
&render.LineChart{
    CompID:    "mem_heap_chart",
    Rect:      image.Rect(120, 400, 700, 650),
    BGColor:   color.RGBA{10, 10, 20, 255},
    LineColor: color.RGBA{0, 255, 150, 255}, // Neon green glow
    Data:      state.HeapHistory,             // Slice of float32 telemetry
    Title:     "REALTIME HEAP MONITOR (MB)",
    Rounding:  10,
}
```

---

## 🔲 3. Layout Control using FlexBox

POEM features a fully responsive multi-axis grid positioning system mirroring FlexBox. Instead of hardcoding absolute pixel offsets, wrap your elements in a `render.FlexBox` container:

```go
&render.FlexBox{
    CompID:         "horizontal_button_bar",
    Rect:           image.Rect(100, 420, 600, 480),
    Direction:      render.Horizontal,          // render.Vertical or render.Horizontal
    AlignItems:     render.AlignCenter,         // AlignStart, Center, End, Stretch
    JustifyContent: render.JustifySpaceBetween, // JustifyStart, Center, End, SpaceBetween
    Padding:        10,
    Gap:            15, // Space between elements
    Children: []render.Component{
        &render.Button{CompID: "btn_a", Rect: image.Rect(0,0,100,40), Label: "Button A"},
        &render.Button{CompID: "btn_b", Rect: image.Rect(0,0,100,40), Label: "Button B"},
    },
}
```

---

## 🚀 4. Complete Downstream App Example

Here is a complete, copy-pasteable dashboard example showing how to initialize, lay out, and run a POEM application.

Save this code in your `main.go` and run it:

```go
package main

import (
	"fmt"
	"image"
	"image/color"

	"go_native_gpu_gui/pkg/render"
)

func main() {
	// 1. Run the application window loop
	render.Run(render.AppConfig{
		Title:  "My Downstream POEM App",
		Width:  800,
		Height: 600,
		BuildPagesFn: BuildAllPages,
	})
}

// 2. The Page builder function that is re-evaluated on state changes
func BuildAllPages(state *render.ApplicationState) {
	if state.Pages == nil {
		state.Pages = make(map[string][]render.Component)
	}

	// Define our dashboard page components
	state.Pages[render.PageDashboard] = []render.Component{
		// Backdrop card
		&render.Panel{
			CompID:   "main_bg",
			Rect:     image.Rect(0, 0, render.Width, render.Height),
			BGColor:  color.RGBA{10, 10, 15, 255},
		},

		// Centered Glass Telemetry Panel
		&render.GlassPanel{
			Panel: render.Panel{
				CompID:   "telemetry_panel",
				Rect:     image.Rect(150, 100, 650, 500),
				BGColor:  color.RGBA{30, 35, 55, 255},
				Rounding: 15,
			},
			Opacity: 170,
		},

		// Header Label
		&render.Label{
			CompID: "head_lbl",
			Pos:    image.Point{180, 140},
			Text:   "REALTIME OPERATIONAL CONTROL",
			Color:  color.RGBA{255, 255, 255, 255},
		},

		// Reactive Counter Telemetry
		&render.DynamicLabel{
			CompID: "interactions_telemetry",
			Pos:    image.Point{180, 190},
			Color:  color.RGBA{0, 255, 150, 255},
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("> Interactions Logged: %d clicks", s.ClickCount)
			},
		},

		// Reactive Slider Telemetry
		&render.DynamicLabel{
			CompID: "slider_telemetry",
			Pos:    image.Point{180, 230},
			Color:  color.RGBA{0, 150, 255, 255},
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("> System Parameter Output: %.1f%%", s.Volume)
			},
		},

		// A clickable trigger
		&render.Button{
			CompID:     "action_btn",
			Rect:       image.Rect(180, 280, 620, 330),
			Label:      "TRIGGER INTERACTION",
			BaseColor:  color.RGBA{60, 80, 120, 255},
			HoverColor: color.RGBA{80, 110, 180, 255},
			Rounding:   8,
			OnClick: func(s *render.ApplicationState) {
				s.ClickCount++
				s.StatusText = "Action trigger successfully clicked!"
			},
		},

		// Master Slider
		&render.Slider{
			CompID:     "parameter_slider",
			Rect:       image.Rect(180, 360, 620, 385),
			Min:        0,
			Max:        100,
			Value:      state.Volume,
			TrackColor: color.RGBA{15, 20, 30, 255},
			ThumbColor: color.RGBA{0, 150, 255, 255},
		},

		// Status Footer
		&render.DynamicLabel{
			CompID: "footer_lbl",
			Pos:    image.Point{180, 440},
			Color:  color.RGBA{150, 160, 180, 255},
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("TELEMETRY: %s", s.StatusText)
			},
		},
	}

	// Always default the current page if empty
	if state.CurrentPage == "" {
		state.CurrentPage = render.PageDashboard
	}
}
```

---

## 🖱️ 5. Dynamic Cursors & Layout Synchronization

POEM features a fully state-driven, dynamic mouse cursor and recursive layout-synchronization subsystem.

### A. Automatic Mouse Pointer Transformations
When building custom components or wrapping interactions, you can dynamically control the system mouse cursor by updating `state.CursorID` inside your component's `Draw` or event method.

The core pipeline automatically resets `state.CursorID` to `state.ArrowCursor` at the start of every frame, allowing components to claim cursor states reactively:

- **Standard Pointers**: Default reset inside the coordinate pipelines.
- **Interactive Clicking Hand (`state.HandCursor`)**: Set automatically when hovering over `Button` or `Slider` components.
- **Text I-Beam (`state.IBeamCursor`)**: Set automatically when hovering over `TextInput` controls.

```go
func (b *MyComponent) Draw(pnt types.Painter, state *types.ApplicationState) {
    if state.HoveredID == b.CompID {
        // Shift mouse pointer to the OS interaction hand!
        state.CursorID = state.HandCursor
    }
}
```

### B. Recursive Layout Synchronization Pass
Because layout components (such as `FlexBox` nested rows/columns) place items dynamically, coordinate positions of children might mismatch hit-testing if computed mid-frame. 

To prevent this, POEM runs an automated **recursive layout synchronization pass** right before evaluating hit-tests or mouse interactions:
```go
// Pre-align and calculate nested child bounds instantly before click/hover evaluation
for _, comp := range page {
    comp.SetBounds(comp.Bounds())
}
```
Downstream developers never need to manually align components—nested children bounds are fully updated, responsive, and ready for hover interactions automatically out-of-the-box!

---

## 📜 6. Composable Scroll Viewports

POEM features a high-performance, fully composable vertical scrolling container (`render.ScrollView`) equipped with boundary clipping, scrollbar dragging, and coordinate translation.

### A. Viewport Bounded Clipping
When rendering massive, high-volume lists or logs, downstream elements must be clipped to prevent them from bleeding onto stationary sections (like headers, sidebars, and background panels). 

POEM implements this by adding a native viewport clipping bounding box to the draw-command tree. Elements outside the ScrollView `Rect` are automatically clipped at the GPU level inside the native sidecar:
- **GPU Scissor Test**: The Windows C++ sidecar currently applies hardware scissor clipping in D3D11, translating logical boundaries into physical screen pixels based on the active DPI scale factor.

### B. Nested Layout Composition Example
To build a scrollable view, simply wrap a vertical `render.FlexBox` container inside a `render.ScrollView` and place as many interactive components (labels, buttons, text fields) as you want inside:

```go
&render.ScrollView{
    CompID: "my_diagnostics_scroll",
    Rect:   image.Rect(100, 150, 500, 650), // Fixed viewport bounds (400x500)
    Children: []render.Component{
        &render.FlexBox{
            CompID:    "inner_scroll_content",
            Direction: render.Vertical,
            Padding:   15,
            Gap:       15,
            Children: []render.Component{
                &render.Label{CompID: "lbl_header", Text: "MASSIVE LIST CONTENT"},
                &render.Button{CompID: "scroll_btn_1", Rect: image.Rect(0, 0, 150, 40), Label: "CLICK ME"},
                &render.TextInput{CompID: "scroll_txt_1", Rect: image.Rect(0, 0, 150, 45), Placeholder: "Type here..."},
                // Add as many children as needed...
            },
        },
    },
}
```

### C. Automated Coordinate Space Translation
A major challenge with scrolled viewports is ensuring that mouse clicks and hover coordinates correctly target the scrolled elements. 

POEM **completely abstracts this coordinate translation**! The `ScrollView` recursively intercepts hit-testing and pointer inputs:
- Translates screen coordinates to scrolled layout space: `scrolledPt = pt.Add(image.Point{0, s.ScrollY})`
- Evaluates hover states and dispatches focus clicks perfectly at their scrolling offsets.
- Downstream developers get 100% functional, hover-reactive, and click-sensitive components out-of-the-box inside scrolling panels!

---

## ⌨️ 7. Keyboard Focus & Hotkeys

POEM features a fully integrated keyboard focus and accessibility engine, offering sequential focus cycling, high-contrast neon glowing outlines, automatic scroll centering, and customizable global hotkey listeners.

### A. Focus Cycling and Glowing Outline Rings
Interactive widgets (`Button`, `TextInput`, `Slider`) return `Focusable() bool { return true }`.
- **Focus Navigation**: Tapping `Tab` or `Shift+Tab` cycles keyboard focus sequentially across focusable elements on the active page.
- **Neon Outline Styling**: The focused component automatically draws a high-contrast glowing neon focus ring (`color.RGBA{0, 150, 255, 200}` with 2px offset) to guide the keyboard navigator.
- **Escape Clearing**: Tapping `Esc` clears active focus at any time.

### B. Polymorphic Autoscrolling Centering
When focus cycles to off-screen elements inside a scrolling container, the `ScrollView` automatically glides to center the focused element into view. The parent container handles this polymorphically using the type-agnostic `ScrollContainer` interface:
```go
type ScrollContainer interface {
    Component
    ScrollToChild(childID string, childBounds image.Rectangle, state *ApplicationState) bool
}
```

### C. Standard Key Triggers & Input Submissions
Focused components handle key triggers natively:
- **Buttons**: Pressing `Enter` on a focused button executes its `OnClick` action instantly.
- **Sliders**: Pressing the `Left Arrow` or `Right Arrow` keys increments or decrements the slider value by exactly 5% steps.
- **Text Inputs**: Pressing `Enter` inside a text field fires the optional `OnSubmit` callback:
  ```go
  &render.TextInput{
      CompID: "console_cmd",
      Rect:   image.Rect(0, 0, 150, 40),
      Placeholder: "COMMAND...",
      OnSubmit: func(text string, state *render.ApplicationState) {
          state.StatusText = "Executed: " + text
      },
  }
  ```

### D. Global Shortcuts Registration (Ctrl+S)
You can declare custom keyboard listeners on the `ApplicationState` that run asynchronously whenever modifier shortcuts are triggered:
```go
func BuildAllPages(state *render.ApplicationState) {
    // Register global hotkey
    state.RegisterHotkey("Ctrl+S", func(s *render.ApplicationState) {
        s.StatusText = "Configuration Saved Successfully!"
    })
}
```
Whenever the user hits `Ctrl+S`, the registered handler fires, updates state, and repaints the screen immediately!

---

## 🖥️ 8. High-DPI & Multi-Resolution Display Scaling

POEM automatically scales your layout on high-density displays (such as 4K screens or displays with custom Windows scaling levels). 

As a downstream developer, **you do not need to perform any scaling math, factor multiplication, or resolution adjustments!**

### Transparent Scaling Mechanics
* **Pure Logical Layouts**: When declaring widgets in `BuildPagesFn` (e.g. `image.Rect(100, 100, 300, 400)`), you define them strictly in **logical units**. POEM's layout flexboxes and margins run in this logical space.
* **Auto-Adjusted Views**: Behind the scenes, the presentation engine scales elements to match the screen's system density, preventing them from appearing tiny on high-resolution screens.
* **Integrated Clipping & Interaction**: Scissor clips inside `ScrollView` and mouse coordinates (`MouseMove`, `MouseDown`, `MouseWheel`) are automatically converted between physical screen pixels and your logical layout space. Custom hover cues, clicks, and scrolling operate cleanly out-of-the-box on any screen size.

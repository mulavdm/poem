# POEM UI Development Guide (Quickstart)

Welcome to the **P.O.E.M. Operational Engine Matrix (POEM)** UI framework! This guide will walk you through the declarative UI architecture, component catalog, layout system, and reactive state loops so you can start building premium, high-performance, glassmorphic interfaces.

---

## 🏗️ 1. Core Architecture & Reactive Loop

POEM uses a **re-evaluation model** to drive interactions. Instead of manually updating widgets, you write a **Page Builder function**. Whenever the user clicks, types, drags a slider, or resizes the window, POEM:
1. Mutates variables inside the global `ApplicationState`.
2. Triggers your Page Builder function to rebuild the component hierarchy from scratch.
3. Paints the new layout to the screen at a lockstep 60FPS.

```
+-------------+      User Clicks       +------------------+
| Win32 Loop  | ---------------------> | ApplicationState |
+-------------+                        +------------------+
       ^                                        |
       | Repaints at 60FPS                      v Triggers
+-------------+                        +------------------+
| CPUEngine / | <--------------------- |  BuildPagesFn()  | (Reconstructs component tree)
| GPUEngine   |                        +------------------+
+-------------+
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

POEM implements this by adding a native viewport clipping bounding box to the `Painter` engine. Elements outside the ScrollView `Rect` are automatically clipped:
- **Software Path (GDI)**: Pixels outside the active clipping rectangle are skipped during pixel-rasterization loops.
- **Hardware Path (OpenGL)**: Employs GPU-level Scissor tests (`gl.Enable(gl.SCISSOR_TEST)`, `gl.Scissor`) to clip the viewport with sub-millisecond drawing performance.

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


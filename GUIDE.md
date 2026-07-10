# POEM UI Development Guide (Quickstart)

## POEM 2.0 authoring path

New UI should use the typed constructors and semantic theme variants documented
in [the POEM 2.0 foundation guide](docs/POEM_2_FOUNDATION.md). Legacy raw-color
struct literals still compile during the staged downstream migration, but are no
longer the preferred authoring model.

When application-owned state changes from a goroutine after IO, model work, or a
service callback, call `render.RequestRepaint()` after publishing the new state.
This marks the current frame dirty without enabling continuous animation and
without coupling the application to the Windows backend.

Welcome to the **P.O.E.M. Operational Engine Matrix (POEM)** UI framework! This guide walks through the declarative UI architecture, component catalog, layout system, reactive state loops, and shared automation hooks used to build professional native desktop interfaces. Decorative glass, glow, particles, and sound are opt-in product effects rather than the default authoring model for ordinary controls.

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

### A. Theme-native panels
Panels form the background containers and cards of your application. New code
should start with `render.NewPanel`; it uses the active theme's semantic surface
tokens and remains platform-neutral.

```go
card := render.NewPanel("settings_card")
card.Rect = image.Rect(100, 100, 460, 340)
card.Raised = true
```

`GlassPanel` remains available for deliberate product effects, but ordinary app
chrome should use themed panels instead of glass by default.

### B. Typography & Dynamic Labels
Labels display text. Dynamic labels automatically query `ApplicationState` to update their strings in real-time.

```go
heading := render.NewLabel("static_header", "Dashboard")
heading.Pos = image.Point{120, 140}
heading.Typography = render.TypographyHeading

// 2. Dynamic Label (Updates text on every paint frame)
fps := &render.DynamicLabel{
    CompID: "telemetry_fps",
    Pos:    image.Point{120, 180},
    Role:   render.TextMuted,
    GetText: func(state *render.ApplicationState) string {
        return fmt.Sprintf("FPS: %.1f // Latency: %.2fms", state.CurrentFPS, float64(state.FrameTime.Microseconds())/1000.0)
    },
}
```

### C. Buttons & Interactions
Buttons capture click actions via a callback hook.

```go
button := render.NewButton("btn_reboot", "Reboot Core", func(state *render.ApplicationState) {
        state.ClickCount++
        state.StatusText = "Engine reboot initiated!"
})
button.Rect = image.Rect(120, 220, 380, 270)
button.Variant = render.VariantDestructive
```

### D. Real-Time Range Sliders
Sliders are perfect for adjustments (volume, frequency, thresholds). Dragging the slider automatically updates its percentage.

```go
slider := render.NewSlider("volume_slider", 0, 100, state.Volume, func(value float32, state *render.ApplicationState) {
    state.Volume = value
})
slider.Rect = image.Rect(120, 300, 380, 325)
```

### E. User Text Inputs
Fully functional text-input controls capturing focused keyboard strokes.

```go
input := render.NewTextInput("input_node_name", "Enter node identity...")
input.Rect = image.Rect(120, 350, 380, 386)
```

### F. Real-Time Telemetry Line Charts
A vector component that plots historical numerical datasets in real time. Charts
are content visualizations, so product-specific colors are acceptable when they
communicate data meaning. Avoid neon or glow as the default app chrome.

```go
&render.LineChart{
    CompID:    "mem_heap_chart",
    Rect:      image.Rect(120, 400, 700, 650),
    BGColor:   color.RGBA{21, 24, 29, 255},
    LineColor: color.RGBA{91, 141, 239, 255},
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

	bg := render.NewPanel("main_bg")
	bg.Rect = image.Rect(0, 0, render.Width, render.Height)

	card := render.NewPanel("telemetry_panel")
	card.Rect = image.Rect(150, 100, 650, 500)
	card.Raised = true

	header := render.NewLabel("head_lbl", "Operational Control")
	header.Pos = image.Point{180, 140}
	header.Typography = render.TypographyTitle

	action := render.NewButton("action_btn", "Trigger Interaction", func(s *render.ApplicationState) {
		s.ClickCount++
		s.StatusText = "Action trigger successfully clicked!"
	})
	action.Rect = image.Rect(180, 280, 620, 324)
	action.Variant = render.VariantPrimary

	slider := render.NewSlider("parameter_slider", 0, 100, state.Volume, func(value float32, s *render.ApplicationState) {
		s.Volume = value
	})
	slider.Rect = image.Rect(180, 360, 620, 386)

	// Define our dashboard page components
	state.Pages[render.PageDashboard] = []render.Component{
		bg,
		card,
		header,

		// Reactive Counter Telemetry
		&render.DynamicLabel{
			CompID: "interactions_telemetry",
			Pos:    image.Point{180, 190},
			Role:   render.TextMuted,
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("Interactions logged: %d clicks", s.ClickCount)
			},
		},

		// Reactive Slider Telemetry
		&render.DynamicLabel{
			CompID: "slider_telemetry",
			Pos:    image.Point{180, 230},
			Role:   render.TextAccent,
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("System parameter output: %.1f%%", s.Volume)
			},
		},

		action,
		slider,

		// Status Footer
		&render.DynamicLabel{
			CompID: "footer_lbl",
			Pos:    image.Point{180, 440},
			Role:   render.TextMuted,
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("Status: %s", s.StatusText)
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

POEM provides keyboard focus and platform-neutral accessibility semantics with sequential focus cycling, theme-defined focus indicators, automatic scroll centering, and customizable global hotkey listeners. On Windows, the native sidecar exposes the semantic tree through UI Automation.

Visible form labels should use `LabeledBox`; it emits a semantic text label and
connects the child through the portable `LabeledBy` relationship. Custom semantic
components can populate `semantics.Relationships` (`LabeledBy`, `DescribedBy`,
`Controls`, and `FlowsTo`). Standard buttons expose the same optional value as
`Button.Relations`. POEM validates every relationship target before publication.

For content-owned colors, keep the application theme active and set a narrow
`Panel.Style` override. This is appropriate for a book cover or generated color
swatch; ordinary surfaces and controls should continue to use theme tokens and
semantic variants.

### A. Focus Cycling and Focus Rings
Interactive widgets (`Button`, `TextInput`, `Slider`) return `Focusable() bool { return true }`.
- **Focus Navigation**: Tapping `Tab` or `Shift+Tab` cycles keyboard focus sequentially across focusable elements on the active page.
- **Theme-defined focus styling**: Focused components use the active theme's accessible focus-ring token. Product-specific glow remains an optional application effect rather than a core-control requirement.
- **Escape behavior**: Tapping `Esc` dismisses the top eligible overlay and restores its launcher; with no overlay open it clears active focus.

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
- **Tab lists**: Arrow keys wrap across enabled tabs; `Home` and `End` select the first and last enabled tab. The strip occupies one sequential tab stop.
- **Menus**: Opening a focused menu moves keyboard focus to its first enabled item. Arrows wrap, `Home`/`End` jump, character keys search by prefix, and dismissal restores the launcher’s focus.
- **Selects**: Options use the overlay layer. Closed arrows/typeahead change the controlled value; while expanded, arrows, `Home`, `End`, and typeahead move the active option, `Enter`/`Space` commit, and `Escape` cancels.
- **Date pickers**: Arrow keys move by day or week, `Home`/`End` move to week boundaries, `Page Up`/`Page Down` change month, and range limits are enforced before `Enter`/`Space` commits.
- **Accordions**: Up/Down wrap across enabled headers, `Home`/`End` jump to enabled boundaries, and `Enter`/`Space` toggles only the active disclosure.
- **Trees**: Up/Down and `Home`/`End` move the active selection, character keys search visible labels, and Left/Right navigate the preserved parent/child hierarchy.
- **Pagination**: Left/Right advance the retained active page and `Home`/`End` jump to the first/last page without requiring uncontrolled application state.
- **Anchored overlays**: Tabbing or clicking elsewhere dismisses menus, select lists, autocomplete suggestions, and calendars before advancing focus. Owner clicks still toggle correctly, nested popup ancestors remain open, and non-interactive toasts remain visible.
- **Dialogs**: Modal dialogs trap sequential focus, restore their launcher when closed, wrap message text with active-theme typography, and size actions from their labels.
- **Data tables**: `Up`, `Down`, `Home`, `End`, `Page Up`, and `Page Down` move the active row and its controlled selection; `Enter` and `Space` activate it. Navigation scrolls the row fully into view.
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

### D. Global shortcuts and mnemonics

Register normalized portable chords with `RegisterShortcut`. Supported modifiers
are Control, Alt, Shift, and Meta; keys include letters, digits, navigation keys,
Delete, and F1-F24. Invalid or ambiguous chords return an error.

```go
func BuildAllPages(state *render.ApplicationState) {
    if err := state.RegisterShortcut("Ctrl+Shift+S", func(s *render.ApplicationState) {
        s.StatusText = "Configuration Saved Successfully!"
    }); err != nil {
        panic(err)
    }
}
```

`RegisterHotkey` remains as a source-compatible wrapper. Buttons may also set an
explicit `Mnemonic` rune. POEM routes `Alt+<key>` within the active modal/page,
focuses and invokes the enabled button, and publishes its chord through the
portable semantic `AccessKey` field and Windows UIA `AccessKey` property.

Sliders created with `NewSlider(..., onChange)` are controlled: update the
application value in `onChange` and pass it back on the next build. POEM retains
only the interaction state; it does not overwrite that value from the legacy
slider map.

---

## 🖥️ 8. High-DPI & Multi-Resolution Display Scaling

POEM automatically scales your layout on high-density displays (such as 4K screens or displays with custom Windows scaling levels). 

As a downstream developer, **you do not need to perform any scaling math, factor multiplication, or resolution adjustments!**

### Transparent Scaling Mechanics
* **Pure Logical Layouts**: When declaring widgets in `BuildPagesFn` (e.g. `image.Rect(100, 100, 300, 400)`), you define them strictly in **logical units**. POEM's layout flexboxes and margins run in this logical space.
* **Auto-Adjusted Views**: Behind the scenes, the presentation engine scales elements to match the screen's system density, preventing them from appearing tiny on high-resolution screens.
* **Integrated Clipping & Interaction**: Scissor clips inside `ScrollView` and mouse coordinates (`MouseMove`, `MouseDown`, `MouseWheel`) are automatically converted between physical screen pixels and your logical layout space. Custom hover cues, clicks, and scrolling operate cleanly out-of-the-box on any screen size.

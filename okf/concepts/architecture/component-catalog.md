---
type: Concept
title: Declarative Component Catalog
description: The core render package components — panels, labels, buttons, sliders, text inputs, and line charts.
tags: [components, api, usage]
timestamp: 2026-07-10T00:00:00Z
---
# Declarative Component Catalog

All elements are exposed directly under the `render` package namespace.

## Theme-native panels

Panels form the background containers and cards of an application. New code should start with `render.NewPanel`; it uses the active theme's semantic surface tokens and remains platform-neutral.

```go
card := render.NewPanel("settings_card")
card.Rect = image.Rect(100, 100, 460, 340)
card.Raised = true
```

`GlassPanel` remains available for deliberate product effects, but ordinary app chrome should use themed panels instead of glass by default.

## Typography & dynamic labels

Labels display text. Dynamic labels automatically query `ApplicationState` to update their strings in real-time.

```go
heading := render.NewLabel("static_header", "Dashboard")
heading.Pos = image.Point{120, 140}
heading.Typography = render.TypographyHeading

// Dynamic Label (Updates text on every paint frame)
fps := &render.DynamicLabel{
    CompID: "telemetry_fps",
    Pos:    image.Point{120, 180},
    Role:   render.TextMuted,
    GetText: func(state *render.ApplicationState) string {
        return fmt.Sprintf("FPS: %.1f // Latency: %.2fms", state.CurrentFPS, float64(state.FrameTime.Microseconds())/1000.0)
    },
}
```

## Buttons & interactions

Buttons capture click actions via a callback hook.

```go
button := render.NewButton("btn_reboot", "Reboot Core", func(state *render.ApplicationState) {
        state.ClickCount++
        state.StatusText = "Engine reboot initiated!"
})
button.Rect = image.Rect(120, 220, 380, 270)
button.Variant = render.VariantDestructive
```

## Real-time range sliders

Sliders are perfect for adjustments (volume, frequency, thresholds). Dragging the slider automatically updates its percentage.

```go
slider := render.NewSlider("volume_slider", 0, 100, state.Volume, func(value float32, state *render.ApplicationState) {
    state.Volume = value
})
slider.Rect = image.Rect(120, 300, 380, 325)
```

## User text inputs

Fully functional text-input controls capturing focused keyboard strokes.

```go
input := render.NewTextInput("input_node_name", "Enter node identity...")
input.Rect = image.Rect(120, 350, 380, 386)
```

## Real-time telemetry line charts

See [Cosine Spline Charts](/concepts/architecture/cosine-spline-charts.md) for the `LineChart` rendering math and usage example.

## See also
- [FlexBox Layout](/concepts/architecture/flexbox-layout.md)
- [Downstream App Example](/concepts/architecture/downstream-example.md)
- [Keyboard Focus Engine](/concepts/architecture/keyboard-focus-engine.md)

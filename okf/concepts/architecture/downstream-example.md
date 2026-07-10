---
type: Concept
title: Complete Downstream App Example
description: A copy-pasteable dashboard example showing how to initialize, lay out, and run a POEM application.
tags: [usage, example, downstream]
timestamp: 2026-07-10T00:00:00Z
---
# Complete Downstream App Example

Save this code in `main.go` and run it:

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

## See also
- [Reactive Loop & IoC](/concepts/architecture/reactive-loop-and-ioc.md)
- [Component Catalog](/concepts/architecture/component-catalog.md)
- [Public API Surface](/concepts/public-api.md)

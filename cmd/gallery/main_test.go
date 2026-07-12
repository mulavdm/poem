package main

import (
	"testing"

	"github.com/mulavdm/poem/pkg/render"
	"github.com/mulavdm/poem/pkg/render/theme"
)

func TestGalleryBuildsProfessionalComponentTrees(t *testing.T) {
	t.Cleanup(func() {
		activeTab = "controls"
	})

	for _, tab := range []string{"controls", "inputs", "navigation", "feedback"} {
		t.Run(tab, func(t *testing.T) {
			activeTab = tab
			state := &render.ApplicationState{
				WindowWidth:     1180,
				WindowHeight:    780,
				ThemeManager:    theme.NewManager(theme.ModernDark()),
				TextInputValues: make(map[string]string),
				ScrollPositions: make(map[string]int),
				ScrollCurrent:   make(map[string]float64),
			}

			buildGallery(state)

			page := state.Pages["gallery"]
			if len(page) == 0 {
				t.Fatalf("gallery page was not populated for tab %q", tab)
			}
			for _, component := range page {
				component.Walk(func(child render.Component) {
					switch c := child.(type) {
					case *render.GlassPanel:
						t.Fatalf("gallery tab %q uses GlassPanel %q; gallery should demonstrate theme-native panels", tab, c.ID())
					case *render.ParticleComponent:
						t.Fatalf("gallery tab %q uses ParticleComponent %q; gallery should keep effects opt-in", tab, c.ID())
					case *render.Panel:
						if !c.UseTheme {
							t.Fatalf("gallery tab %q has non-themed panel %q", tab, c.ID())
						}
					case *render.Button:
						if !c.UseTheme {
							t.Fatalf("gallery tab %q has non-themed button %q", tab, c.ID())
						}
					case *render.TextInput:
						if !c.UseTheme {
							t.Fatalf("gallery tab %q has non-themed text input %q", tab, c.ID())
						}
					case *render.TextArea:
						if !c.UseTheme {
							t.Fatalf("gallery tab %q has non-themed text area %q", tab, c.ID())
						}
					case *render.Slider:
						if !c.UseTheme {
							t.Fatalf("gallery tab %q has non-themed slider %q", tab, c.ID())
						}
					case *render.ProgressBar:
						if !c.UseTheme {
							t.Fatalf("gallery tab %q has non-themed progress bar %q", tab, c.ID())
						}
					case *render.ScrollView:
						if !c.UseTheme {
							t.Fatalf("gallery tab %q has non-themed scroll view %q", tab, c.ID())
						}
					}
				})
			}
		})
	}
}

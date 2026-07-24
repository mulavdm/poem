package main

import (
	"fmt"
	"testing"

	"github.com/mulavdm/poem/pkg/render"
	"github.com/mulavdm/poem/pkg/render/theme"
)

func TestGalleryBuildsProfessionalComponentTrees(t *testing.T) {
	t.Cleanup(func() {
		activeTab = "controls"
	})

	viewports := []struct {
		name string
		w, h int
	}{
		{"desktop", 1180, 780},
		{"tablet", 768, 1024},
		{"compact_mobile", 360, 740},
	}

	tabs := []string{"controls", "inputs", "navigation", "feedback", "viewports"}

	for _, vp := range viewports {
		for _, tab := range tabs {
			testName := fmt.Sprintf("%s_%s", vp.name, tab)
			t.Run(testName, func(t *testing.T) {
				activeTab = tab
				state := &render.ApplicationState{
					WindowWidth:     vp.w,
					WindowHeight:    vp.h,
					ThemeManager:    theme.NewManager(theme.ModernDark()),
					TextInputValues: make(map[string]string),
					ScrollPositions: make(map[string]int),
					ScrollCurrent:   make(map[string]float64),
				}

				buildGallery(state)

				page := state.Pages["gallery"]
				if len(page) == 0 {
					t.Fatalf("gallery page was not populated for %s", testName)
				}
				for _, component := range page {
					component.Walk(func(child render.Component) {
						switch c := child.(type) {
						case *render.GlassPanel:
							t.Fatalf("gallery %s uses GlassPanel %q; gallery should demonstrate theme-native panels", testName, c.ID())
						case *render.ParticleComponent:
							t.Fatalf("gallery %s uses ParticleComponent %q; gallery should keep effects opt-in", testName, c.ID())
						case *render.Panel:
							if !c.UseTheme {
								t.Fatalf("gallery %s has non-themed panel %q", testName, c.ID())
							}
						case *render.Button:
							if !c.UseTheme {
								t.Fatalf("gallery %s has non-themed button %q", testName, c.ID())
							}
						case *render.TextInput:
							if !c.UseTheme {
								t.Fatalf("gallery %s has non-themed text input %q", testName, c.ID())
							}
						case *render.TextArea:
							if !c.UseTheme {
								t.Fatalf("gallery %s has non-themed text area %q", testName, c.ID())
							}
						case *render.Slider:
							if !c.UseTheme {
								t.Fatalf("gallery %s has non-themed slider %q", testName, c.ID())
							}
						case *render.ProgressBar:
							if !c.UseTheme {
								t.Fatalf("gallery %s has non-themed progress bar %q", testName, c.ID())
							}
						case *render.ScrollView:
							if !c.UseTheme {
								t.Fatalf("gallery %s has non-themed scroll view %q", testName, c.ID())
							}
						}
					})
				}
			})
		}
	}
}

func TestCompactGalleryKeepsNavigationFixedAndTabStateIndependent(t *testing.T) {
	t.Cleanup(func() { activeTab = "controls" })

	scrollIDs := make(map[string]bool)
	for _, tab := range []string{"controls", "inputs", "navigation", "feedback", "viewports"} {
		activeTab = tab
		state := &render.ApplicationState{
			WindowWidth:     360,
			WindowHeight:    740,
			ThemeManager:    theme.NewManager(theme.ModernLight()),
			TextInputValues: make(map[string]string),
			ScrollPositions: make(map[string]int),
			ScrollCurrent:   make(map[string]float64),
		}
		buildGallery(state)

		var tabs *render.Tabs
		var scroll *render.ScrollView
		for _, component := range state.Pages["gallery"] {
			switch c := component.(type) {
			case *render.Tabs:
				tabs = c
			case *render.ScrollView:
				scroll = c
			}
		}
		if tabs == nil || scroll == nil {
			t.Fatalf("%s compact page needs fixed tabs and a content scroll view", tab)
		}
		if tabs.Bounds().Max.Y > scroll.Bounds().Min.Y {
			t.Fatalf("%s tabs overlap scrolling content: tabs=%v scroll=%v", tab, tabs.Bounds(), scroll.Bounds())
		}
		if scroll.ID() != "gallery.scroll."+tab {
			t.Fatalf("%s uses shared scroll identity %q", tab, scroll.ID())
		}
		if scrollIDs[scroll.ID()] {
			t.Fatalf("duplicate compact scroll identity %q", scroll.ID())
		}
		scrollIDs[scroll.ID()] = true
	}
}

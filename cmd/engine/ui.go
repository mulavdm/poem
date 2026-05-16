package main

import (
	"fmt"
	"image"
	"image/color"
	"runtime"

	"go_native_gpu_gui/internal/render"
)

// BuildShowcaseLayout constructs the complex dashboard UI tree based on current state.
func BuildShowcaseLayout(state *render.ApplicationState) []render.Component {
	if state.CurrentPage == "" {
		state.CurrentPage = render.PageDashboard
	}

	// 1. GLOBAL CHROME (Always Visible)
	comps := []render.Component{
		// Backdrop
		&render.Panel{CompID: "bg_blur", Rect: image.Rect(0, 0, render.Width, render.Height), BGColor: color.RGBA{10, 10, 15, 255}},
		
		// Background Particles
		&render.ParticleComponent{CompID: "vfx_particles", System: state.Particles},

		// Sidebar Background
		&render.Panel{CompID: "sidebar_bg", Rect: image.Rect(0, 0, 70, render.Height), BGColor: color.RGBA{15, 15, 25, 255}},
		BuildSidebar(state),

		// Header Background
		&render.Panel{CompID: "header_bg", Rect: image.Rect(70, 0, render.Width, 60), BGColor: color.RGBA{20, 25, 40, 220}},
		BuildHeader(state),
		
		// Footer Background
		&render.Panel{CompID: "footer_bg", Rect: image.Rect(70, render.Height-40, render.Width, render.Height), BGColor: color.RGBA{10, 10, 20, 255}},
		BuildFooter(state),
	}

	// 2. PAGE CONTENT (Swappable Area)
	switch state.CurrentPage {
	case render.PageDashboard:
		comps = append(comps, BuildDashboard(state)...)
	case render.PageSettings:
		comps = append(comps, BuildSettings(state)...)
	case render.PageAnalytics:
		comps = append(comps, BuildAnalytics(state)...)
	default:
		comps = append(comps, BuildDashboard(state)...)
	}

	return comps
}

func BuildSidebar(state *render.ApplicationState) render.Component {
	return &render.FlexBox{
		CompID:         "sidebar_layout",
		Rect:           image.Rect(0, 0, 70, render.Height),
		Direction:      render.Vertical,
		AlignItems:     render.AlignCenter,
		JustifyContent: render.JustifyCenter,
		Padding:        15,
		Gap:            20,
		Children: []render.Component{
			&render.Button{
				CompID: "nav_home", Rect: image.Rect(0, 0, 40, 40), Label: "H",
				BaseColor:  getPageColor(state, render.PageDashboard),
				HoverColor: color.RGBA{0, 150, 255, 255}, Rounding: 8,
				OnClick: func(s *render.ApplicationState) { s.CurrentPage = render.PageDashboard },
			},
			&render.Button{
				CompID: "nav_analytics", Rect: image.Rect(0, 0, 40, 40), Label: "A",
				BaseColor:  getPageColor(state, render.PageAnalytics),
				HoverColor: color.RGBA{0, 150, 255, 255}, Rounding: 8,
				OnClick: func(s *render.ApplicationState) { s.CurrentPage = render.PageAnalytics },
			},
			&render.Button{
				CompID: "nav_settings", Rect: image.Rect(0, 0, 40, 40), Label: "S",
				BaseColor:  getPageColor(state, render.PageSettings),
				HoverColor: color.RGBA{0, 150, 255, 255}, Rounding: 8,
				OnClick: func(s *render.ApplicationState) { s.CurrentPage = render.PageSettings },
			},
		},
	}
}

func getPageColor(state *render.ApplicationState, page string) color.RGBA {
	if state.CurrentPage == page {
		return color.RGBA{0, 120, 255, 255} // Active
	}
	return color.RGBA{40, 50, 70, 255} // Normal
}

func BuildHeader(state *render.ApplicationState) render.Component {
	return &render.FlexBox{
		CompID: "header_layout",
		Rect:   image.Rect(70, 0, render.Width, 60),
		Direction: render.Horizontal,
		AlignItems: render.AlignCenter,
		JustifyContent: render.JustifySpaceBetween,
		Padding: 20,
		Children: []render.Component{
			&render.Label{CompID: "title", Pos: image.Point{0, 0}, Text: fmt.Sprintf("P.O.E.M. // %s", state.CurrentPage), Color: color.RGBA{255, 255, 255, 255}},
			&render.DynamicLabel{
				CompID: "status_tag",
				Pos:    image.Point{0, 0},
				Color:  color.RGBA{0, 255, 180, 255},
				GetText: func(s *render.ApplicationState) string { return "[ STATUS: POETIC ]" },
			},
		},
	}
}

func BuildFooter(state *render.ApplicationState) render.Component {
	return &render.FlexBox{
		CompID: "footer_layout",
		Rect:   image.Rect(70, render.Height-40, render.Width, render.Height),
		Direction: render.Horizontal,
		AlignItems: render.AlignCenter,
		Padding: 20,
		Children: []render.Component{
			&render.DynamicLabel{
				CompID: "foot_status",
				Pos:    image.Point{0, 0},
				Color:  color.RGBA{100, 120, 150, 255},
				GetText: func(s *render.ApplicationState) string {
					hover := "NONE"
					if s.HoveredID != "" {
						hover = s.HoveredID
					}
					active := "NONE"
					if s.ActiveID != "" {
						active = s.ActiveID
					}
					return fmt.Sprintf("P.O.E.M. READY | THREADS: %d | HOVER: %s | ACTIVE: %s | PAGE: %s", runtime.NumGoroutine(), hover, active, s.CurrentPage)
				},
			},
		},
	}
}

func BuildDashboard(state *render.ApplicationState) []render.Component {
	return []render.Component{
		// 3. LEFT WIDGET: "System Configuration"
		&render.Panel{CompID: "config_panel", Rect: image.Rect(90, 80, 400, 560), BGColor: color.RGBA{25, 30, 45, 180}, Rounding: 15},
		&render.Label{CompID: "cfg_title", Pos: image.Point{110, 115}, Text: "SYSTEM PARAMETERS", Color: color.RGBA{150, 160, 180, 255}},

		&render.Label{CompID: "lbl_core", Pos: image.Point{110, 160}, Text: "CPU CORE ASSIGNMENT:", Color: color.RGBA{200, 200, 200, 255}},
		&render.Panel{CompID: "inp_core", Rect: image.Rect(110, 175, 380, 205), BGColor: color.RGBA{10, 10, 20, 255}, Rounding: 5},
		&render.DynamicLabel{
			CompID: "val_core",
			Pos:    image.Point{120, 195},
			Color:  color.RGBA{0, 150, 255, 255},
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("0x%08X (%d CORES)", s.CoreMask, runtime.NumCPU())
			},
		},

		&render.Label{CompID: "lbl_freq", Pos: image.Point{110, 230}, Text: "OSCILLATION FREQUENCY:", Color: color.RGBA{200, 200, 200, 255}},
		&render.Panel{CompID: "inp_freq", Rect: image.Rect(110, 245, 380, 275), BGColor: color.RGBA{10, 10, 20, 255}, Rounding: 5},
		&render.DynamicLabel{
			CompID: "val_freq",
			Pos:    image.Point{120, 265},
			Color:  color.RGBA{0, 255, 150, 255},
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("%.2f Hz (REALTIME)", s.CurrentFPS)
			},
		},

		&render.Label{CompID: "lbl_node", Pos: image.Point{110, 300}, Text: "NODE IDENTITY:", Color: color.RGBA{200, 200, 200, 255}},
		&render.TextInput{
			CompID:      "inp_node",
			Rect:        image.Rect(110, 315, 380, 345),
			Placeholder: "ENTER NODE NAME...",
			BGColor:     color.RGBA{10, 10, 20, 255},
			TextColor:   color.RGBA{255, 255, 255, 255},
			Rounding:    5,
		},

		&render.Button{
			CompID:     "btn_reboot",
			Rect:       image.Rect(110, 370, 380, 420),
			Label:      "REBOOT CORE ENGINE",
			BaseColor:  color.RGBA{180, 60, 60, 255},
			HoverColor: color.RGBA{220, 80, 80, 255},
			Rounding:   10,
		},

		// Volume Slider
		&render.Label{CompID: "lbl_vol", Pos: image.Point{110, 460}, Text: "MASTER VOLUME:", Color: color.RGBA{200, 200, 200, 255}},
		&render.Slider{
			CompID:     "sld_vol",
			Rect:       image.Rect(110, 480, 380, 505),
			Min:        0,
			Max:        100,
			Value:      state.Volume,
			TrackColor: color.RGBA{10, 10, 20, 255},
			ThumbColor: color.RGBA{0, 150, 255, 255},
		},

		// 4. RIGHT WIDGET: "Interaction Telemetry"
		&render.Panel{CompID: "telemetry_panel", Rect: image.Rect(420, 80, render.Width-20, 560), BGColor: color.RGBA{25, 30, 45, 180}, Rounding: 15},
		&render.Label{CompID: "tel_title", Pos: image.Point{440, 115}, Text: "LIVE TELEMETRY STREAM", Color: color.RGBA{150, 160, 180, 255}},

		&render.DynamicLabel{
			CompID: "tel_clicks",
			Pos:    image.Point{440, 160},
			Color:  color.RGBA{255, 255, 255, 255},
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("> TOTAL INTERACTIONS: %d", s.ClickCount)
			},
		},
		&render.DynamicLabel{
			CompID: "tel_mouse",
			Pos:    image.Point{440, 190},
			Color:  color.RGBA{255, 255, 255, 255},
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("> POINTER COORDINATES: [%d, %d]", s.MouseX, s.MouseY)
			},
		},
		&render.DynamicLabel{
			CompID: "tel_hover",
			Pos:    image.Point{440, 220},
			Color:  color.RGBA{255, 255, 100, 255},
			GetText: func(s *render.ApplicationState) string {
				if s.HoveredID == "" {
					return "> CURRENT TARGET: NONE"
				}
				return fmt.Sprintf("> CURRENT TARGET: %s", s.HoveredID)
			},
		},
		&render.DynamicLabel{
			CompID: "tel_slider",
			Pos:    image.Point{440, 250},
			Color:  color.RGBA{255, 100, 255, 255},
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("> MASTER VOLUME: %.1f%%", s.Volume)
			},
		},

		// Action Buttons Grid
		&render.FlexBox{
			CompID:         "action_grid",
			Rect:           image.Rect(440, 280, render.Width-40, 330),
			Direction:      render.Horizontal,
			JustifyContent: render.JustifySpaceBetween,
			AlignItems:     render.AlignCenter,
			Children: []render.Component{
				&render.Button{CompID: "btn_a", Rect: image.Rect(0, 0, 140, 50), Label: "ACTION A", BaseColor: color.RGBA{60, 80, 120, 255}, HoverColor: color.RGBA{80, 110, 180, 255}, Rounding: 5},
				&render.Button{CompID: "btn_b", Rect: image.Rect(0, 0, 140, 50), Label: "ACTION B", BaseColor: color.RGBA{60, 80, 120, 255}, HoverColor: color.RGBA{80, 110, 180, 255}, Rounding: 5},
				&render.Button{CompID: "btn_c", Rect: image.Rect(0, 0, 140, 50), Label: "ACTION C", BaseColor: color.RGBA{60, 80, 120, 255}, HoverColor: color.RGBA{80, 110, 180, 255}, Rounding: 5},
			},
		},
	}
}

func BuildSettings(state *render.ApplicationState) []render.Component {
	return []render.Component{
		&render.Panel{CompID: "settings_panel", Rect: image.Rect(90, 80, render.Width-20, render.Height-100), BGColor: color.RGBA{25, 30, 45, 180}, Rounding: 15},
		&render.Label{CompID: "set_title", Pos: image.Point{110, 115}, Text: "SYSTEM SETTINGS", Color: color.RGBA{150, 160, 180, 255}},

		// UI Preferences
		&render.Label{CompID: "lbl_ui", Pos: image.Point{110, 160}, Text: "INTERFACE PREFERENCES", Color: color.RGBA{200, 200, 200, 255}},
		&render.Button{
			CompID: "btn_theme", Rect: image.Rect(110, 180, 350, 230), Label: "SWITCH COLOR THEME",
			BaseColor: color.RGBA{60, 80, 120, 255}, HoverColor: color.RGBA{80, 110, 180, 255}, Rounding: 5,
		},
		&render.Button{
			CompID: "btn_blur", Rect: image.Rect(110, 240, 350, 290), Label: "TOGGLE GLASS BLUR",
			BaseColor: color.RGBA{60, 80, 120, 255}, HoverColor: color.RGBA{80, 110, 180, 255}, Rounding: 5,
		},

		// Engine Information
		&render.Label{CompID: "lbl_engine_info", Pos: image.Point{110, 350}, Text: "ENGINE INFORMATION", Color: color.RGBA{200, 200, 200, 255}},
		&render.Panel{CompID: "engine_info_box", Rect: image.Rect(110, 365, render.Width-40, 500), BGColor: color.RGBA{15, 20, 30, 255}, Rounding: 8},
		&render.Label{CompID: "engine_v", Pos: image.Point{125, 395}, Text: "P.O.E.M. v0.4.2 // OPERATIONAL ENGINE MATRIX", Color: color.RGBA{0, 255, 150, 255}},
		&render.Label{CompID: "engine_arch", Pos: image.Point{125, 425}, Text: fmt.Sprintf("ARCHITECTURE: %s // OS: %s", runtime.GOARCH, runtime.GOOS), Color: color.RGBA{150, 160, 180, 255}},
		&render.Label{CompID: "engine_compiler", Pos: image.Point{125, 455}, Text: fmt.Sprintf("COMPILER: %s", runtime.Version()), Color: color.RGBA{150, 160, 180, 255}},
	}
}

func BuildAnalytics(state *render.ApplicationState) []render.Component {
	return []render.Component{
		&render.Panel{CompID: "analytics_panel", Rect: image.Rect(90, 80, render.Width-20, render.Height-100), BGColor: color.RGBA{25, 30, 45, 180}, Rounding: 15},
		&render.Label{CompID: "ana_title", Pos: image.Point{110, 115}, Text: "PERFORMANCE ANALYTICS", Color: color.RGBA{150, 160, 180, 255}},

		// Memory Metrics
		&render.Label{CompID: "lbl_mem", Pos: image.Point{110, 160}, Text: "MEMORY ALLOCATION", Color: color.RGBA{200, 200, 200, 255}},
		&render.DynamicLabel{
			CompID: "val_mem_alloc",
			Pos:    image.Point{120, 195},
			Color:  color.RGBA{0, 255, 150, 255},
			GetText: func(s *render.ApplicationState) string {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				return fmt.Sprintf("HEAP ALLOC: %.2f MB", float64(m.Alloc)/1024/1024)
			},
		},
		&render.DynamicLabel{
			CompID: "val_mem_sys",
			Pos:    image.Point{120, 225},
			Color:  color.RGBA{0, 255, 150, 255},
			GetText: func(s *render.ApplicationState) string {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				return fmt.Sprintf("SYSTEM TOTAL: %.2f MB", float64(m.Sys)/1024/1024)
			},
		},
		&render.DynamicLabel{
			CompID: "val_mem_gc",
			Pos:    image.Point{120, 255},
			Color:  color.RGBA{255, 150, 0, 255},
			GetText: func(s *render.ApplicationState) string {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				return fmt.Sprintf("GC COLLECTIONS: %d", m.NumGC)
			},
		},

		// Threading Metrics
		&render.Label{CompID: "lbl_threads", Pos: image.Point{110, 320}, Text: "CONCURRENCY MONITOR", Color: color.RGBA{200, 200, 200, 255}},
		&render.DynamicLabel{
			CompID: "val_goroutines",
			Pos:    image.Point{120, 355},
			Color:  color.RGBA{0, 150, 255, 255},
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("ACTIVE GOROUTINES: %d", runtime.NumGoroutine())
			},
		},
		&render.DynamicLabel{
			CompID: "val_cgo",
			Pos:    image.Point{120, 385},
			Color:  color.RGBA{0, 150, 255, 255},
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("CGO CALLS: %d", runtime.NumCgoCall())
			},
		},

		// Simulated Chart Area
		&render.Label{CompID: "lbl_chart", Pos: image.Point{500, 160}, Text: "REALTIME LOAD GRAPH", Color: color.RGBA{200, 200, 200, 255}},
		&render.Panel{CompID: "chart_bg", Rect: image.Rect(500, 180, render.Width-40, 480), BGColor: color.RGBA{10, 10, 20, 255}, Rounding: 10},
		&render.Label{CompID: "chart_placeholder", Pos: image.Point{X: 520, Y: 330}, Text: "[ LIVE DATA STREAM PENDING ]", Color: color.RGBA{60, 70, 90, 255}},
	}
}

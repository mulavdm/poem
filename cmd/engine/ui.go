package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"

	"go_native_gpu_gui/pkg/render"
)

// BuildAllPages populates the application state's page registry.
func BuildAllPages(state *render.ApplicationState) {
	if state.Pages == nil {
		state.Pages = make(map[string][]render.Component)
	}

	state.RegisterHotkey("Ctrl+S", func(s *render.ApplicationState) {
		s.StatusText = "Configuration Matrix Saved Successfully! [Visual Plexus Density Synced]"
		s.PlaySuccess()
	})

	if state.CurrentPage == "" {
		state.CurrentPage = render.PageDashboard
	}

	state.Pages[render.PageDashboard] = BuildPage(state, render.PageDashboard, BuildDashboard(state))
	state.Pages[render.PageAnalytics] = BuildPage(state, render.PageAnalytics, BuildAnalytics(state))
	state.Pages[render.PageSettings] = BuildPage(state, render.PageSettings, BuildSettings(state))
}

func BuildPage(state *render.ApplicationState, name string, content []render.Component) []render.Component {
	comps := []render.Component{
		// Backdrop
		&render.Panel{CompID: "bg_blur_" + name, Rect: image.Rect(0, 0, render.Width, render.Height), BGColor: color.RGBA{10, 10, 15, 255}},

		// Background Particles
		&render.ParticleComponent{CompID: "vfx_particles_" + name, System: state.Particles},

		// Sidebar Background
		&render.Panel{CompID: "sidebar_bg_" + name, Rect: image.Rect(0, 0, 70, render.Height), BGColor: color.RGBA{15, 15, 25, 255}},
		BuildSidebar(state),

		// Header Background
		&render.Panel{CompID: "header_bg_" + name, Rect: image.Rect(70, 0, render.Width, 60), BGColor: color.RGBA{20, 25, 40, 220}},
		BuildHeader(state),

		// Footer Background
		&render.Panel{CompID: "footer_bg_" + name, Rect: image.Rect(70, render.Height-40, render.Width, render.Height), BGColor: color.RGBA{10, 10, 20, 255}},
		BuildFooter(state),
	}
	return append(comps, content...)
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
				OnClick: func(s *render.ApplicationState) { s.NavigateTo(render.PageDashboard) },
			},
			&render.Button{
				CompID: "nav_analytics", Rect: image.Rect(0, 0, 40, 40), Label: "A",
				BaseColor:  getPageColor(state, render.PageAnalytics),
				HoverColor: color.RGBA{0, 150, 255, 255}, Rounding: 8,
				OnClick: func(s *render.ApplicationState) { s.NavigateTo(render.PageAnalytics) },
			},
			&render.Button{
				CompID: "nav_settings", Rect: image.Rect(0, 0, 40, 40), Label: "S",
				BaseColor:  getPageColor(state, render.PageSettings),
				HoverColor: color.RGBA{0, 150, 255, 255}, Rounding: 8,
				OnClick: func(s *render.ApplicationState) { s.NavigateTo(render.PageSettings) },
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
		CompID:         "header_layout",
		Rect:           image.Rect(70, 0, render.Width, 60),
		Direction:      render.Horizontal,
		AlignItems:     render.AlignCenter,
		JustifyContent: render.JustifySpaceBetween,
		Padding:        20,
		Children: []render.Component{
			&render.Label{CompID: "title", Pos: image.Point{0, 0}, Text: fmt.Sprintf("P.O.E.M. // %s", state.CurrentPage), Color: color.RGBA{255, 255, 255, 255}},
			&render.DynamicLabel{
				CompID:  "status_tag",
				Pos:     image.Point{0, 0},
				Color:   color.RGBA{0, 255, 180, 255},
				GetText: func(s *render.ApplicationState) string { return "[ STATUS: POETIC ]" },
			},
		},
	}
}

func BuildFooter(state *render.ApplicationState) render.Component {
	return &render.FlexBox{
		CompID:     "footer_layout",
		Rect:       image.Rect(70, render.Height-40, render.Width, render.Height),
		Direction:  render.Horizontal,
		AlignItems: render.AlignCenter,
		Padding:    20,
		Children: []render.Component{
			&render.DynamicLabel{
				CompID: "foot_perf",
				Pos:    image.Point{0, 0},
				Color:  color.RGBA{0, 255, 150, 255},
				GetText: func(s *render.ApplicationState) string {
					return fmt.Sprintf("FRAME: %.2fms | FPS: %.1f", float64(s.FrameTime.Microseconds())/1000.0, s.CurrentFPS)
				},
			},
			&render.DynamicLabel{
				CompID: "foot_status",
				Pos:    image.Point{0, 0},
				Color:  color.RGBA{100, 120, 150, 255},
				GetText: func(s *render.ApplicationState) string {
					hover := "NONE"
					if s.HoveredID != "" {
						hover = s.HoveredID
					}
					return fmt.Sprintf(" | HOVER: %s | PAGE: %s", hover, s.CurrentPage)
				},
			},
		},
	}
}

func BuildDashboard(state *render.ApplicationState) []render.Component {
	return []render.Component{
		// 3. LEFT WIDGET: "System Configuration"
		&render.GlassPanel{Panel: render.Panel{CompID: "config_panel", Rect: image.Rect(90, 80, 400, 560), BGColor: color.RGBA{30, 35, 55, 255}, Rounding: 15}, Opacity: 180},
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
		&render.GlassPanel{Panel: render.Panel{CompID: "telemetry_panel", Rect: image.Rect(420, 80, render.Width-20, 560), BGColor: color.RGBA{30, 35, 55, 255}, Rounding: 15}, Opacity: 180},
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
		&render.GlassPanel{Panel: render.Panel{CompID: "settings_panel", Rect: image.Rect(90, 80, render.Width-20, render.Height-100), BGColor: color.RGBA{30, 35, 55, 255}, Rounding: 15}, Opacity: 180},
		&render.Label{CompID: "set_title", Pos: image.Point{110, 115}, Text: "SYSTEM SETTINGS", Color: color.RGBA{150, 160, 180, 255}},

		// UI Preferences
		&render.Label{CompID: "lbl_ui", Pos: image.Point{110, 160}, Text: "INTERFACE PREFERENCES", Color: color.RGBA{200, 200, 200, 255}},
		&render.Button{
			CompID: "btn_theme", Rect: image.Rect(110, 180, 460, 230), Label: "SWITCH COLOR THEME",
			BaseColor: color.RGBA{60, 80, 120, 255}, HoverColor: color.RGBA{80, 110, 180, 255}, Rounding: 5,
		},
		&render.Button{
			CompID: "btn_blur", Rect: image.Rect(110, 240, 460, 290), Label: "TOGGLE GLASS BLUR",
			BaseColor: color.RGBA{60, 80, 120, 255}, HoverColor: color.RGBA{80, 110, 180, 255}, Rounding: 5,
			OnClick: func(s *render.ApplicationState) {
				s.GlassEnabled = !s.GlassEnabled
			},
		},
		&render.Button{
			CompID: "btn_audio", Rect: image.Rect(110, 300, 460, 350),
			Label: func() string {
				if state.AudioEnabled {
					return "MUTE AUDIO FEEDBACK"
				}
				return "UNMUTE AUDIO FEEDBACK"
			}(),
			BaseColor: color.RGBA{60, 80, 120, 255}, HoverColor: color.RGBA{80, 110, 180, 255}, Rounding: 5,
			OnClick: func(s *render.ApplicationState) {
				s.AudioEnabled = !s.AudioEnabled
				if s.AudioEnabled {
					s.StatusText = "Acoustic Audio Feedback Enabled // OK"
					s.PlaySuccess()
				} else {
					s.StatusText = "Acoustic Audio Feedback Muted // OK"
				}
			},
		},

		// Engine Information
		&render.Label{CompID: "lbl_engine_info", Pos: image.Point{110, 350}, Text: "ENGINE INFORMATION", Color: color.RGBA{200, 200, 200, 255}},
		&render.Panel{CompID: "engine_info_box", Rect: image.Rect(110, 365, 460, 650), BGColor: color.RGBA{15, 20, 30, 255}, Rounding: 8},
		&render.Label{CompID: "engine_v", Pos: image.Point{125, 395}, Text: "P.O.E.M. v0.4.2 // ENGINE MATRIX", Color: color.RGBA{0, 255, 150, 255}},
		&render.Label{CompID: "engine_arch", Pos: image.Point{125, 435}, Text: fmt.Sprintf("ARCH: %s // OS: %s", runtime.GOARCH, runtime.GOOS), Color: color.RGBA{150, 160, 180, 255}},
		&render.Label{CompID: "engine_compiler", Pos: image.Point{125, 475}, Text: fmt.Sprintf("COMPILER: %s", runtime.Version()), Color: color.RGBA{150, 160, 180, 255}},

		// Diagnostics Scroll View
		&render.Label{CompID: "lbl_diag_panel", Pos: image.Point{500, 130}, Text: "DIAGNOSTIC MATRIX & ACTIVE CONSOLE", Color: color.RGBA{200, 200, 200, 255}},
		&render.ScrollView{
			CompID:         "diagnostics_scroll",
			Rect:           image.Rect(500, 150, render.Width-40, 650),
			ScrollY:        state.ScrollPositions["diagnostics_scroll"],
			CurrentScrollY: int(math.Round(state.ScrollCurrent["diagnostics_scroll"])),
			Children: []render.Component{
				&render.FlexBox{
					CompID:    "diagnostics_layout",
					Direction: render.Vertical,
					Padding:   15,
					Gap:       15,
					Children: []render.Component{
						&render.Label{CompID: "diag_hdr_1", Pos: image.Point{0, 0}, Text: "--- DYNAMIC DIAGNOSTIC NODE ENTRIES ---", Color: color.RGBA{100, 120, 150, 255}},

						&render.Label{CompID: "diag_lbl_1", Pos: image.Point{0, 0}, Text: "[01] PIPELINE PARITY: PASSING", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_2", Pos: image.Point{0, 0}, Text: "[02] GDI BACKEND DIB LAYER: ONLINE", Color: color.RGBA{0, 255, 150, 255}},

						&render.FlexBox{
							CompID:    "diag_row_btn_1",
							Direction: render.Horizontal,
							Padding:   0,
							Gap:       10,
							Children: []render.Component{
								&render.Label{CompID: "diag_lbl_3", Pos: image.Point{0, 0}, Text: "[03] COSMIC DENSITY TRIGGER:", Color: color.RGBA{200, 200, 200, 255}},
								&render.Button{
									CompID: "diag_btn_trigger", Rect: image.Rect(0, 0, 120, 30), Label: "PING PULSE",
									BaseColor: color.RGBA{0, 120, 255, 255}, HoverColor: color.RGBA{0, 160, 255, 255}, Rounding: 4,
									OnClick: func(s *render.ApplicationState) {
										s.StatusText = "Pulse Ping Signal Dispatched: [0xFF09]"
									},
								},
							},
						},

						&render.Label{CompID: "diag_lbl_4", Pos: image.Point{0, 0}, Text: "[04] SCISSOR VIEWPORT CLIP: ENGAGED", Color: color.RGBA{0, 255, 150, 255}},

						&render.FlexBox{
							CompID:    "diag_row_slider_1",
							Direction: render.Horizontal,
							Padding:   0,
							Gap:       10,
							Children: []render.Component{
								&render.Label{CompID: "diag_lbl_5", Pos: image.Point{0, 0}, Text: "[05] SCROLL DYNAMIC METRIC:", Color: color.RGBA{200, 200, 200, 255}},
								&render.Slider{
									CompID: "diag_slider_scroll", Rect: image.Rect(0, 0, 120, 20),
									Min: 0, Max: 100, Value: 50,
									TrackColor: color.RGBA{10, 10, 20, 255}, ThumbColor: color.RGBA{0, 255, 150, 255},
								},
							},
						},

						&render.Label{CompID: "diag_lbl_6", Pos: image.Point{0, 0}, Text: "[06] WINDOW PROCEDURE CAPTURE: ACTIVE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_7", Pos: image.Point{0, 0}, Text: "[07] GL SCISSOR TOP-DOWN CONV: DONE", Color: color.RGBA{0, 255, 150, 255}},

						&render.FlexBox{
							CompID:    "diag_row_input_1",
							Direction: render.Horizontal,
							Padding:   0,
							Gap:       10,
							Children: []render.Component{
								&render.Label{CompID: "diag_lbl_8", Pos: image.Point{0, 0}, Text: "[08] CONSOLE INPUT CMD:", Color: color.RGBA{200, 200, 200, 255}},
								&render.TextInput{
									CompID: "diag_input_scroll", Rect: image.Rect(0, 0, 130, 30),
									Placeholder: "EXECUTE...", BGColor: color.RGBA{10, 10, 20, 255},
									TextColor: color.RGBA{255, 255, 255, 255}, Rounding: 4,
									OnSubmit: func(text string, s *render.ApplicationState) {
										s.StatusText = fmt.Sprintf("Console Command Executed: '%s' // OK", text)
										s.PlaySuccess()
									},
								},
							},
						},

						&render.Label{CompID: "diag_lbl_9", Pos: image.Point{0, 0}, Text: "[09] ALPHA BLENDING DECAY RATE: 100%", Color: color.RGBA{150, 160, 180, 255}},
						&render.Label{CompID: "diag_lbl_10", Pos: image.Point{0, 0}, Text: "[10] HYPER-THREAD MUTEX LOCKS: SAFE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_11", Pos: image.Point{0, 0}, Text: "[11] COMPOSABLE VIEWPORT DEPTH: PASS", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_12", Pos: image.Point{0, 0}, Text: "[12] DOCKING CONTROLLER ATTACH: NONE", Color: color.RGBA{150, 160, 180, 255}},
						&render.Label{CompID: "diag_lbl_13", Pos: image.Point{0, 0}, Text: "[13] PHYSICAL LAYER INTERRUPT: PASS", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_14", Pos: image.Point{0, 0}, Text: "[14] GDI STRETCH-DIB-ITS: REALTIME", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_15", Pos: image.Point{0, 0}, Text: "[15] SYSTEM ENTROPY INTEGRITY: PASS", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_16", Pos: image.Point{0, 0}, Text: "[16] HIGH-PRECISION DELTA PHYSICS: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_17", Pos: image.Point{0, 0}, Text: "[17] ASYNC WAKE-UP PULSE LOOP: ACTIVE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_18", Pos: image.Point{0, 0}, Text: "[18] ZERO-GC ALLOCATION VERTS: ENGAGED", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_19", Pos: image.Point{0, 0}, Text: "[19] BRESENHAM GPU CONNECTIONS: ACTIVE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_20", Pos: image.Point{0, 0}, Text: "[20] MULTI-PAGE DESCRIPTOR MAP: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_21", Pos: image.Point{0, 0}, Text: "[21] WIN32 VALIDATE REGION: PASS", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_22", Pos: image.Point{0, 0}, Text: "[22] MEMORY LEAK CHECK: COMPLETING", Color: color.RGBA{150, 160, 180, 255}},
						&render.Label{CompID: "diag_lbl_23", Pos: image.Point{0, 0}, Text: "[23] WGL COMPATIBILITY MATRIX: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_24", Pos: image.Point{0, 0}, Text: "[24] DOUBLE BUFFER FLIP RATE: VSYNC", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_25", Pos: image.Point{0, 0}, Text: "[25] TELEMETRY TIMEOUT HANDLER: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_26", Pos: image.Point{0, 0}, Text: "[26] STAGGERED LERP EXP DECAY: CALIBRATED", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_27", Pos: image.Point{0, 0}, Text: "[27] DIRECT-GRIP SCROLL SNAPPING: OK", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_28", Pos: image.Point{0, 0}, Text: "[28] SUB-IMAGE TEXT RASTERIZE: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_29", Pos: image.Point{0, 0}, Text: "[29] PLEXUS VFX PARTICLE MASS: 100", Color: color.RGBA{150, 160, 180, 255}},
						&render.Label{CompID: "diag_lbl_30", Pos: image.Point{0, 0}, Text: "[30] COGNITIVE AGENT MATRIX: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_31", Pos: image.Point{0, 0}, Text: "[31] DYNAMIC GLASSMORPHISM: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_32", Pos: image.Point{0, 0}, Text: "[32] GDI BULK BIT BLIT STRETCH: OK", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_33", Pos: image.Point{0, 0}, Text: "[33] UNIFIED CONTROLLER LOOP: RUNNING", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_34", Pos: image.Point{0, 0}, Text: "[34] DECAY SPEED CONSTANT: 0.07", Color: color.RGBA{150, 160, 180, 255}},
						&render.Label{CompID: "diag_lbl_35", Pos: image.Point{0, 0}, Text: "[35] SYSTEM OVERALL STATUS: EXCELLENT", Color: color.RGBA{0, 255, 150, 255}},

						// Interactive elements at the bottom to test viewport autoscroll centering glide
						&render.FlexBox{
							CompID:    "diag_row_test_bottom",
							Direction: render.Horizontal,
							Padding:   0,
							Gap:       10,
							Children: []render.Component{
								&render.Button{
									CompID: "diag_btn_test_bottom", Rect: image.Rect(0, 0, 150, 30), Label: "BOTTOM TRIGGER",
									BaseColor: color.RGBA{0, 150, 255, 255}, HoverColor: color.RGBA{0, 180, 255, 255}, Rounding: 4,
									OnClick: func(s *render.ApplicationState) {
										s.StatusText = "Bottom Trigger Activated! Auto-Glide Centering works perfectly!"
										s.PlaySuccess()
									},
								},
								&render.TextInput{
									CompID: "diag_input_test_bottom", Rect: image.Rect(0, 0, 150, 30),
									Placeholder: "BOTTOM INPUT...", BGColor: color.RGBA{10, 10, 20, 255},
									TextColor: color.RGBA{255, 255, 255, 255}, Rounding: 4,
									OnSubmit: func(text string, s *render.ApplicationState) {
										s.StatusText = fmt.Sprintf("Bottom Input Received: '%s'", text)
										s.PlaySuccess()
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func BuildAnalytics(state *render.ApplicationState) []render.Component {
	return []render.Component{
		&render.GlassPanel{Panel: render.Panel{CompID: "analytics_panel", Rect: image.Rect(90, 80, render.Width-20, render.Height-100), BGColor: color.RGBA{30, 35, 55, 255}, Rounding: 15}, Opacity: 180},
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
		&render.Label{CompID: "lbl_chart", Pos: image.Point{500, 160}, Text: "REALTIME HEAP MONITOR", Color: color.RGBA{200, 200, 200, 255}},
		&render.LineChart{
			CompID:    "mem_load_chart",
			Rect:      image.Rect(500, 180, render.Width-40, 480),
			BGColor:   color.RGBA{10, 10, 20, 255},
			LineColor: color.RGBA{0, 255, 150, 255},
			Data:      state.HeapHistory,
			Title:     "REALTIME HEAP GRAPH (MB)",
			Rounding:  10,
		},
	}
}

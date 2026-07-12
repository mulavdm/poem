package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"
	"time"

	"go_native_gpu_gui/pkg/render"
)

type RaycasterDemo struct {
	CompID string
	Rect   image.Rectangle
}

var raycasterDemoMap = buildRaycasterDemoMap()

func buildRaycasterDemoMap() string {
	cells := make([]byte, 64*64)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			v := byte('0')
			if x == 0 || y == 0 || x == 63 || y == 63 {
				v = '1'
			}
			if x == 8 || x == 55 || y == 8 || y == 55 {
				v = '1'
			}
			if x > 16 && x < 22 && y > 12 && y < 44 {
				v = '1'
			}
			if x > 28 && x < 35 && y > 20 && y < 26 {
				v = '1'
			}
			if x > 40 && x < 46 && y > 14 && y < 48 && y%7 != 0 {
				v = '1'
			}
			if y > 30 && y < 36 && x > 22 && x < 52 {
				v = '1'
			}
			if (x == 21 && y > 22 && y < 28) || (y == 35 && x > 31 && x < 37) {
				v = '0'
			}
			if (x > 10 && x < 14 && y > 10 && y < 14) || (x > 48 && x < 53 && y > 43 && y < 48) {
				v = '2'
			}
			cells[y*64+x] = v
		}
	}
	return string(cells)
}

func (r *RaycasterDemo) ID() string                     { return r.CompID }
func (r *RaycasterDemo) GetID() string                  { return r.CompID }
func (r *RaycasterDemo) Bounds() image.Rectangle        { return r.Rect }
func (r *RaycasterDemo) SetBounds(rect image.Rectangle) { r.Rect = rect }
func (r *RaycasterDemo) Focusable() bool                { return false }
func (r *RaycasterDemo) Walk(fn func(render.Component)) { fn(r) }
func (r *RaycasterDemo) HitTest(pt image.Point) string {
	if pt.In(r.Rect) {
		return r.CompID
	}
	return ""
}
func (r *RaycasterDemo) OnKey(key uint32, char rune, state *render.ApplicationState) bool {
	return false
}
func (r *RaycasterDemo) OnMouseDown(pt image.Point, state *render.ApplicationState) bool {
	return false
}
func (r *RaycasterDemo) OnMouseUp(pt image.Point, state *render.ApplicationState) bool { return false }
func (r *RaycasterDemo) OnMouseMove(pt image.Point, state *render.ApplicationState) bool {
	return false
}

func (r *RaycasterDemo) Draw(p render.Painter, state *render.ApplicationState) {
	elapsed := float32(time.Since(state.StartTime).Seconds())
	playerX := float32(12.5 + math.Cos(float64(elapsed*0.33))*2.2)
	playerY := float32(14.0 + math.Sin(float64(elapsed*0.27))*1.8)
	playerAngle := elapsed * 0.9

	p.SetGlass(state.GlassEnabled)
	p.SetShadow(0, 6, 18)
	p.DrawRoundedRect(r.Rect, 14, color.RGBA{22, 26, 38, 220})
	p.SetShadow(0, 0, 0)
	p.SetGlass(false)

	viewport := image.Rect(r.Rect.Min.X+12, r.Rect.Min.Y+12, r.Rect.Max.X-12, r.Rect.Max.Y-12)
	p.DrawRaycasterMapStyled(viewport, playerX, playerY, playerAngle, color.RGBA{255, 155, 70, 255}, raycasterDemoMap)
	p.DrawBillboard3D(viewport, 13.5, 12.0, 1, color.RGBA{255, 215, 30, 255})
	p.DrawBillboard3D(viewport, 18.0, 18.0, 2, color.RGBA{240, 90, 95, 255})
	p.DrawBillboard3D(viewport, 26.0, 24.0, 3, color.RGBA{255, 190, 40, 255})
	p.DrawBillboard3D(viewport, 34.0, 28.5, 4, color.RGBA{255, 245, 110, 255})
	p.DrawBillboard3D(viewport, 43.0, 23.0, 5, color.RGBA{255, 170, 70, 255})
	p.DrawBillboard3D(viewport, 47.5, 42.0, 6, color.RGBA{210, 170, 95, 255})
	p.DrawBillboard3D(viewport, 29.5, 45.0, 7, color.RGBA{70, 205, 255, 255})
	p.DrawBillboard3D(viewport, 52.0, 33.0, 8, color.RGBA{255, 110, 120, 255})
}

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
	bg := render.NewPanel("bg_" + name)
	bg.Rect = image.Rect(0, 0, render.Width, render.Height)

	sidebar := render.NewPanel("sidebar_bg_" + name)
	sidebar.Rect = image.Rect(0, 0, 70, render.Height)
	sidebar.Raised = true

	header := render.NewPanel("header_bg_" + name)
	header.Rect = image.Rect(70, 0, render.Width, 60)
	header.Raised = true

	footer := render.NewPanel("footer_bg_" + name)
	footer.Rect = image.Rect(70, render.Height-40, render.Width, render.Height)

	comps := []render.Component{
		bg,
		sidebar,
		BuildSidebar(state),
		header,
		BuildHeader(state),
		footer,
		BuildFooter(state),
	}
	return append(comps, content...)
}

func BuildSidebar(state *render.ApplicationState) render.Component {
	home := navButton(state, "nav_home", "H", render.PageDashboard)
	analytics := navButton(state, "nav_analytics", "A", render.PageAnalytics)
	settings := navButton(state, "nav_settings", "S", render.PageSettings)
	return &render.FlexBox{
		CompID:         "sidebar_layout",
		Rect:           image.Rect(0, 0, 70, render.Height),
		Direction:      render.Vertical,
		AlignItems:     render.AlignCenter,
		JustifyContent: render.JustifyCenter,
		Padding:        15,
		Gap:            20,
		Children: []render.Component{
			home,
			analytics,
			settings,
		},
	}
}

func navButton(state *render.ApplicationState, id, label, page string) *render.Button {
	button := render.NewButton(id, label, func(s *render.ApplicationState) { s.NavigateTo(page) })
	button.Rect = image.Rect(0, 0, 40, 40)
	button.Variant = render.VariantSubtle
	button.Selected = state.CurrentPage == page
	button.AccessibleName = page
	return button
}

func themedPanel(id string, rect image.Rectangle, raised bool) *render.Panel {
	panel := render.NewPanel(id)
	panel.Rect = rect
	panel.Raised = raised
	return panel
}

func themedLabel(id string, pos image.Point, text string, role render.TextRole, typography render.TypographyRole) *render.Label {
	label := render.NewLabel(id, text)
	label.Pos = pos
	label.Role = role
	label.Typography = typography
	return label
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
			themedLabel("title", image.Point{0, 0}, fmt.Sprintf("POEM // %s", state.CurrentPage), render.TextDefault, render.TypographyTitle),
			&render.DynamicLabel{
				CompID:  "status_tag",
				Pos:     image.Point{0, 0},
				Role:    render.TextAccent,
				GetText: func(s *render.ApplicationState) string { return "Status: Ready" },
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
				Role:   render.TextMuted,
				GetText: func(s *render.ApplicationState) string {
					return fmt.Sprintf("FRAME: %.2fms | FPS: %.1f", float64(s.FrameTime.Microseconds())/1000.0, s.CurrentFPS)
				},
			},
			&render.DynamicLabel{
				CompID: "foot_status",
				Pos:    image.Point{0, 0},
				Role:   render.TextMuted,
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
	configPanel := themedPanel("config_panel", image.Rect(90, 80, 400, 560), true)
	telemetryPanel := themedPanel("telemetry_panel", image.Rect(420, 80, render.Width-20, 560), true)
	nodeInput := render.NewTextInput("inp_node", "Enter node name...")
	nodeInput.Rect = image.Rect(110, 315, 380, 351)
	reboot := render.NewButton("btn_reboot", "Reboot Core Engine", func(s *render.ApplicationState) {
		s.StatusText = "Core engine reboot requested."
	})
	reboot.Rect = image.Rect(110, 370, 380, 414)
	reboot.Variant = render.VariantDanger
	volume := render.NewSlider("sld_vol", 0, 100, state.Volume, func(value float32, s *render.ApplicationState) {
		s.Volume = value
	})
	volume.Rect = image.Rect(110, 480, 380, 506)
	actionA := render.NewButton("btn_a", "Action A", func(s *render.ApplicationState) { s.StatusText = "Action A invoked." })
	actionA.Rect = image.Rect(0, 0, 140, 44)
	actionA.Variant = render.VariantSecondary
	actionB := render.NewButton("btn_b", "Action B", func(s *render.ApplicationState) { s.StatusText = "Action B invoked." })
	actionB.Rect = image.Rect(0, 0, 140, 44)
	actionB.Variant = render.VariantSecondary
	actionC := render.NewButton("btn_c", "Action C", func(s *render.ApplicationState) { s.StatusText = "Action C invoked." })
	actionC.Rect = image.Rect(0, 0, 140, 44)
	actionC.Variant = render.VariantSecondary
	return []render.Component{
		configPanel,
		themedLabel("cfg_title", image.Point{110, 115}, "System Parameters", render.TextMuted, render.TypographyLabel),
		themedLabel("lbl_core", image.Point{110, 160}, "CPU Core Assignment", render.TextDefault, render.TypographyLabel),
		themedPanel("inp_core", image.Rect(110, 175, 380, 207), false),
		&render.DynamicLabel{
			CompID: "val_core",
			Pos:    image.Point{120, 195},
			Role:   render.TextAccent,
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("0x%08X (%d CORES)", s.CoreMask, runtime.NumCPU())
			},
		},

		themedLabel("lbl_freq", image.Point{110, 230}, "Frame Frequency", render.TextDefault, render.TypographyLabel),
		themedPanel("inp_freq", image.Rect(110, 245, 380, 277), false),
		&render.DynamicLabel{
			CompID: "val_freq",
			Pos:    image.Point{120, 265},
			Role:   render.TextSuccess,
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("%.2f FPS", s.CurrentFPS)
			},
		},

		themedLabel("lbl_node", image.Point{110, 300}, "Node Identity", render.TextDefault, render.TypographyLabel),
		nodeInput,
		reboot,

		// Volume Slider
		themedLabel("lbl_vol", image.Point{110, 460}, "Master Volume", render.TextDefault, render.TypographyLabel),
		volume,

		// 4. RIGHT WIDGET: "Interaction Telemetry"
		telemetryPanel,
		themedLabel("tel_title", image.Point{440, 115}, "Live Telemetry", render.TextMuted, render.TypographyLabel),

		&render.DynamicLabel{
			CompID: "tel_clicks",
			Pos:    image.Point{440, 160},
			Role:   render.TextDefault,
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("Total interactions: %d", s.ClickCount)
			},
		},
		&render.DynamicLabel{
			CompID: "tel_mouse",
			Pos:    image.Point{440, 190},
			Role:   render.TextDefault,
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("Pointer coordinates: [%d, %d]", s.MouseX, s.MouseY)
			},
		},
		&render.DynamicLabel{
			CompID: "tel_hover",
			Pos:    image.Point{440, 220},
			Role:   render.TextWarning,
			GetText: func(s *render.ApplicationState) string {
				if s.HoveredID == "" {
					return "Current target: none"
				}
				return fmt.Sprintf("Current target: %s", s.HoveredID)
			},
		},
		&render.DynamicLabel{
			CompID: "tel_slider",
			Pos:    image.Point{440, 250},
			Role:   render.TextAccent,
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("Master volume: %.1f%%", s.Volume)
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
				actionA,
				actionB,
				actionC,
			},
		},
	}
}

func BuildSettings(state *render.ApplicationState) []render.Component {
	settingsPanel := themedPanel("settings_panel", image.Rect(90, 80, render.Width-20, render.Height-100), true)
	themeButton := render.NewButton("btn_theme", "Switch Color Theme", func(s *render.ApplicationState) {
		s.StatusText = "Theme switching is demonstrated in the POEM gallery."
	})
	themeButton.Rect = image.Rect(110, 180, 460, 224)
	themeButton.Variant = render.VariantSecondary
	effectsButton := render.NewButton("btn_blur", "Toggle Optional Glass Effect", func(s *render.ApplicationState) {
		s.GlassEnabled = !s.GlassEnabled
	})
	effectsButton.Rect = image.Rect(110, 240, 460, 284)
	effectsButton.Variant = render.VariantSubtle
	audioButton := render.NewButton("btn_audio", func() string {
		if state.AudioEnabled {
			return "Mute Optional Audio"
		}
		return "Enable Optional Audio"
	}(), func(s *render.ApplicationState) {
		s.AudioEnabled = !s.AudioEnabled
		if s.AudioEnabled {
			s.StatusText = "Optional audio feedback enabled."
			s.PlaySuccess()
		} else {
			s.StatusText = "Optional audio feedback muted."
		}
	})
	audioButton.Rect = image.Rect(110, 300, 460, 344)
	audioButton.Variant = render.VariantSubtle
	return []render.Component{
		settingsPanel,
		themedLabel("set_title", image.Point{110, 115}, "System Settings", render.TextMuted, render.TypographyLabel),

		// UI Preferences
		themedLabel("lbl_ui", image.Point{110, 160}, "Interface Preferences", render.TextDefault, render.TypographyLabel),
		themeButton,
		effectsButton,
		audioButton,

		// Engine Information
		themedLabel("lbl_engine_info", image.Point{110, 350}, "Engine Information", render.TextDefault, render.TypographyLabel),
		themedPanel("engine_info_box", image.Rect(110, 365, 460, 650), false),
		themedLabel("engine_v", image.Point{125, 395}, "POEM 2.0 Native Framework", render.TextSuccess, render.TypographyBody),
		themedLabel("engine_arch", image.Point{125, 435}, fmt.Sprintf("ARCH: %s // OS: %s", runtime.GOARCH, runtime.GOOS), render.TextMuted, render.TypographyBody),
		themedLabel("engine_compiler", image.Point{125, 475}, fmt.Sprintf("COMPILER: %s", runtime.Version()), render.TextMuted, render.TypographyBody),

		// Diagnostics Scroll View
		themedLabel("lbl_diag_panel", image.Point{500, 130}, "Diagnostics & Active Console", render.TextDefault, render.TypographyLabel),
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
						&render.Label{CompID: "diag_lbl_2", Pos: image.Point{0, 0}, Text: "[02] D3D11 SIDECAR IPC BRIDGE: ONLINE", Color: color.RGBA{0, 255, 150, 255}},

						&render.FlexBox{
							CompID:    "diag_row_btn_1",
							Direction: render.Horizontal,
							Padding:   0,
							Gap:       10,
							Children: []render.Component{
								&render.Label{CompID: "diag_lbl_3", Pos: image.Point{0, 0}, Text: "[03] COSMIC DENSITY TRIGGER:", Color: color.RGBA{200, 200, 200, 255}},
								func() render.Component {
									button := render.NewButton("diag_btn_trigger", "Ping Pulse", func(s *render.ApplicationState) {
										s.StatusText = "Pulse Ping Signal Dispatched: [0xFF09]"
									})
									button.Rect = image.Rect(0, 0, 120, 30)
									button.Variant = render.VariantSecondary
									return button
								}(),
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
								func() render.Component {
									slider := render.NewSlider("diag_slider_scroll", 0, 100, 50, nil)
									slider.Rect = image.Rect(0, 0, 120, 24)
									return slider
								}(),
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
								func() render.Component {
									input := render.NewTextInput("diag_input_scroll", "Execute...")
									input.Rect = image.Rect(0, 0, 130, 30)
									input.OnSubmit = func(text string, s *render.ApplicationState) {
										s.StatusText = fmt.Sprintf("Console Command Executed: '%s' // OK", text)
										s.PlaySuccess()
									}
									return input
								}(),
							},
						},

						&render.Label{CompID: "diag_lbl_9", Pos: image.Point{0, 0}, Text: "[09] ALPHA BLENDING DECAY RATE: 100%", Color: color.RGBA{150, 160, 180, 255}},
						&render.Label{CompID: "diag_lbl_10", Pos: image.Point{0, 0}, Text: "[10] HYPER-THREAD MUTEX LOCKS: SAFE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_11", Pos: image.Point{0, 0}, Text: "[11] COMPOSABLE VIEWPORT DEPTH: PASS", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_12", Pos: image.Point{0, 0}, Text: "[12] DOCKING CONTROLLER ATTACH: NONE", Color: color.RGBA{150, 160, 180, 255}},
						&render.Label{CompID: "diag_lbl_13", Pos: image.Point{0, 0}, Text: "[13] PHYSICAL LAYER INTERRUPT: PASS", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_14", Pos: image.Point{0, 0}, Text: "[14] IPC NAMED PIPES TRANSFER: REALTIME", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_15", Pos: image.Point{0, 0}, Text: "[15] SYSTEM ENTROPY INTEGRITY: PASS", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_16", Pos: image.Point{0, 0}, Text: "[16] HIGH-PRECISION DELTA PHYSICS: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_17", Pos: image.Point{0, 0}, Text: "[17] ASYNC WAKE-UP PULSE LOOP: ACTIVE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_18", Pos: image.Point{0, 0}, Text: "[18] ZERO-GC ALLOCATION VERTS: ENGAGED", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_19", Pos: image.Point{0, 0}, Text: "[19] BRESENHAM GPU CONNECTIONS: ACTIVE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_20", Pos: image.Point{0, 0}, Text: "[20] MULTI-PAGE DESCRIPTOR MAP: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_21", Pos: image.Point{0, 0}, Text: "[21] IPC DYNAMIC REPAINT TICK: PASS", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_22", Pos: image.Point{0, 0}, Text: "[22] MEMORY LEAK CHECK: COMPLETING", Color: color.RGBA{150, 160, 180, 255}},
						&render.Label{CompID: "diag_lbl_23", Pos: image.Point{0, 0}, Text: "[23] D3D11 PRESENTATION STATE: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_24", Pos: image.Point{0, 0}, Text: "[24] DOUBLE BUFFER FLIP RATE: VSYNC", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_25", Pos: image.Point{0, 0}, Text: "[25] TELEMETRY TIMEOUT HANDLER: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_26", Pos: image.Point{0, 0}, Text: "[26] STAGGERED LERP EXP DECAY: CALIBRATED", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_27", Pos: image.Point{0, 0}, Text: "[27] DIRECT-GRIP SCROLL SNAPPING: OK", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_28", Pos: image.Point{0, 0}, Text: "[28] SUB-IMAGE TEXT RASTERIZE: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_29", Pos: image.Point{0, 0}, Text: "[29] OPTIONAL EFFECTS: AVAILABLE", Color: color.RGBA{150, 160, 180, 255}},
						&render.Label{CompID: "diag_lbl_30", Pos: image.Point{0, 0}, Text: "[30] COGNITIVE AGENT MATRIX: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_31", Pos: image.Point{0, 0}, Text: "[31] THEME TOKEN RESOLUTION: ONLINE", Color: color.RGBA{0, 255, 150, 255}},
						&render.Label{CompID: "diag_lbl_32", Pos: image.Point{0, 0}, Text: "[32] D3D11 HARDWARE COMPOSITING: ACTIVE", Color: color.RGBA{0, 255, 150, 255}},
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
								func() render.Component {
									button := render.NewButton("diag_btn_test_bottom", "Bottom Trigger", func(s *render.ApplicationState) {
										s.StatusText = "Bottom Trigger Activated! Auto-Glide Centering works perfectly!"
										s.PlaySuccess()
									})
									button.Rect = image.Rect(0, 0, 150, 30)
									button.Variant = render.VariantPrimary
									return button
								}(),
								func() render.Component {
									input := render.NewTextInput("diag_input_test_bottom", "Bottom input...")
									input.Rect = image.Rect(0, 0, 150, 30)
									input.OnSubmit = func(text string, s *render.ApplicationState) {
										s.StatusText = fmt.Sprintf("Bottom Input Received: '%s'", text)
										s.PlaySuccess()
									}
									return input
								}(),
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
		themedPanel("analytics_panel", image.Rect(90, 80, render.Width-20, render.Height-100), true),
		themedLabel("ana_title", image.Point{110, 115}, "Performance Analytics", render.TextMuted, render.TypographyLabel),

		// Memory Metrics
		themedLabel("lbl_mem", image.Point{110, 160}, "Memory Allocation", render.TextDefault, render.TypographyLabel),
		&render.DynamicLabel{
			CompID: "val_mem_alloc",
			Pos:    image.Point{120, 195},
			Role:   render.TextSuccess,
			GetText: func(s *render.ApplicationState) string {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				return fmt.Sprintf("Heap alloc: %.2f MB", float64(m.Alloc)/1024/1024)
			},
		},
		&render.DynamicLabel{
			CompID: "val_mem_sys",
			Pos:    image.Point{120, 225},
			Role:   render.TextSuccess,
			GetText: func(s *render.ApplicationState) string {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				return fmt.Sprintf("System total: %.2f MB", float64(m.Sys)/1024/1024)
			},
		},
		&render.DynamicLabel{
			CompID: "val_mem_gc",
			Pos:    image.Point{120, 255},
			Role:   render.TextWarning,
			GetText: func(s *render.ApplicationState) string {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				return fmt.Sprintf("GC collections: %d", m.NumGC)
			},
		},

		// Threading Metrics
		themedLabel("lbl_threads", image.Point{110, 320}, "Concurrency Monitor", render.TextDefault, render.TypographyLabel),
		&render.DynamicLabel{
			CompID: "val_goroutines",
			Pos:    image.Point{120, 355},
			Role:   render.TextAccent,
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("Active goroutines: %d", runtime.NumGoroutine())
			},
		},
		&render.DynamicLabel{
			CompID: "val_cgo",
			Pos:    image.Point{120, 385},
			Role:   render.TextAccent,
			GetText: func(s *render.ApplicationState) string {
				return fmt.Sprintf("CGO calls: %d", runtime.NumCgoCall())
			},
		},

		// Simulated Chart Area
		themedLabel("lbl_chart", image.Point{500, 160}, "Realtime Heap Monitor", render.TextDefault, render.TypographyLabel),
		&render.LineChart{
			CompID:    "mem_load_chart",
			Rect:      image.Rect(500, 180, render.Width-40, 410),
			BGColor:   color.RGBA{21, 24, 29, 255},
			LineColor: color.RGBA{91, 141, 239, 255},
			Data:      state.HeapHistory,
			Title:     "Realtime Heap Graph (MB)",
			Rounding:  10,
		},
		themedLabel("lbl_raycaster_demo", image.Point{500, 440}, "GPU Special Path Diagnostic", render.TextDefault, render.TypographyLabel),
		&RaycasterDemo{
			CompID: "raycaster_demo",
			Rect:   image.Rect(500, 460, render.Width-40, 650),
		},
	}
}

package main

import (
	"fmt"
	"image"
	"image/color"

	"github.com/mulavdm/poem/pkg/render"
)

var (
	themes               = render.NewThemeManager(render.ModernDarkTheme())
	notificationsEnabled = true
	autosaveEnabled      = false
	quality              = float32(64)
	format               = "png"
	formatOpen           = false
	activeTab            = "controls"
	selectedTreeItem     = "components"
	expandedTreeItems    = map[string]bool{"framework": true}
	autocompleteQuery    = ""
	galleryPage          = 3
	dueDate              = render.NewDate(2026, 6, 21)
	expandedAccordion    = map[string]bool{"appearance": true}
	selectedTableRow     = "alpha"
	viewportTransform    = render.ImageTransform{Scale: 1.0}
	mapViewportTransform = render.ImageTransform{Scale: 1.0}
	sampleImageBytes     []byte
	sampleMapBytes       []byte
)

func init() {
	sampleImageBytes = createSampleImageBytes(200, 120)
	sampleMapBytes = createMapPreviewBytes(200, 120)
}

func createSampleImageBytes(w, h int) []byte {
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := (y*w + x) * 4
			pixels[idx] = byte(30 + (x * 180 / w))   // R
			pixels[idx+1] = byte(60 + (y * 160 / h)) // G
			pixels[idx+2] = byte(180)                // B
			pixels[idx+3] = 255                      // A
		}
	}
	return pixels
}

func createMapPreviewBytes(w, h int) []byte {
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b := byte(224), byte(229), byte(220)

			// A calm shoreline, park, and street network make the fallback
			// legible as a map without pretending that a retained scene loaded.
			if x < 38+y/4 {
				r, g, b = 158, 198, 216
			}
			dx, dy := x-145, y-33
			if dx*dx+dy*dy < 25*25 {
				r, g, b = 181, 207, 174
			}
			if x > 65 && (x/16+y/12)%2 == 0 {
				r, g, b = 211, 216, 207
			}
			diagonalRoad := y - (98 - x/4)
			verticalRoad := x - (104 + y/7)
			if (diagonalRoad >= -3 && diagonalRoad <= 3) || (verticalRoad >= -3 && verticalRoad <= 3) {
				r, g, b = 248, 246, 238
			}

			idx := (y*w + x) * 4
			pixels[idx], pixels[idx+1], pixels[idx+2], pixels[idx+3] = r, g, b, 255
		}
	}
	return pixels
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func galleryHeading(id, text string) *render.Label {
	label := render.NewLabel(id, text)
	label.Typography = render.TypographyTitle
	return label
}

func gallerySupportingText(id, text string) *render.Label {
	label := render.NewLabel(id, text)
	label.Role = render.TextMuted
	return label
}

func galleryScheme(state *render.ApplicationState) string {
	if state != nil && state.DesignSystem != nil {
		if state.DesignEnvironment.HighContrast {
			return "contrast"
		}
		if state.DesignEnvironment.Dark {
			return "dark"
		}
		return "light"
	}
	current, _ := state.CurrentTheme()
	switch current.Mode {
	case render.ThemeModeHighContrast:
		return "contrast"
	case render.ThemeModeDark:
		return "dark"
	default:
		return "light"
	}
}

func applyGalleryScheme(state *render.ApplicationState, scheme string, width, height int) {
	if state == nil {
		return
	}
	if state.DesignSystem != nil {
		state.DesignEnvironment.HighContrast = scheme == "contrast"
		state.DesignEnvironment.Dark = scheme == "dark"
		_ = state.ResolveDesign(width, height)
		return
	}
	switch scheme {
	case "contrast":
		_ = state.SetTheme(render.HighContrastTheme())
	case "dark":
		_ = state.SetTheme(render.ModernDarkTheme())
	default:
		_ = state.SetTheme(render.ModernLightTheme())
	}
}

func buildGallery(state *render.ApplicationState) {
	w, h := state.GetWindowSize()
	if w <= 0 {
		w = 1180
	}
	if h <= 0 {
		h = 780
	}
	state.CurrentPage = "gallery"
	_ = state.RegisterShortcut("Ctrl+Shift+Q", func(*render.ApplicationState) { quality = 77 })

	compact := w < 600
	squareRadius := 0

	background := render.NewPanel("gallery.background")
	background.Rect = image.Rect(0, 0, w, h)
	background.Style = &render.StyleOverride{Radius: &squareRadius}

	header := render.NewPanel("gallery.header")
	header.Style = &render.StyleOverride{Radius: &squareRadius}
	var title *render.Label
	var status *render.Badge

	if compact {
		title = render.NewLabel("gallery.title", "POEM Gallery")
		status = render.NewBadge("gallery.status", "2.0")
	} else {
		title = render.NewLabel("gallery.title", "POEM Component Gallery")
		status = render.NewBadge("gallery.status", "FOUNDATION")
	}
	title.Typography = render.TypographyTitle
	status.Variant = render.VariantNeutral

	dark := render.NewButton("theme.dark", "Modern dark", func(*render.ApplicationState) { _ = themes.Set(render.ModernDarkTheme()) })
	light := render.NewButton("theme.light", "Modern light", func(*render.ApplicationState) { _ = themes.Set(render.ModernLightTheme()) })
	windows := render.NewButton("theme.windows", "Windows", func(*render.ApplicationState) { _ = themes.Set(render.WindowsTheme()) })
	editorial := render.NewButton("theme.editorial", "Editorial", func(*render.ApplicationState) { _ = themes.Set(render.EditorialTheme()) })
	highContrast := render.NewButton("theme.contrast", "High contrast", func(*render.ApplicationState) { _ = themes.Set(render.HighContrastTheme()) })

	var sidebar *render.Panel
	var sidebarContent *render.FlexBox
	var contentPanel *render.Panel
	var scroll *render.ScrollView

	if compact {
		header.Rect = image.Rect(0, 0, w, 56)
		header.Raised = false
		title.SetBounds(image.Rect(16, 14, w-75, 46))
		status.SetBounds(image.Rect(w-70, 14, w-16, 46))

		contentPanel = render.NewPanel("gallery.content")
		contentPanel.Rect = image.Rect(0, 56, w, h)
		contentPanel.Raised = false
		contentPanel.Style = &render.StyleOverride{Radius: &squareRadius}
	} else {
		header.Rect = image.Rect(16, 16, w-16, 68)
		header.Raised = true
		title.SetBounds(image.Rect(32, 24, 420, 60))
		status.SetBounds(image.Rect(w-150, 26, w-32, 56))

		sidebar = render.NewPanel("gallery.sidebar")
		sidebar.Rect = image.Rect(16, 80, 220, h-16)
		sidebar.Raised = true

		sidebarContent = &render.FlexBox{
			CompID:     "gallery.theme-list",
			Rect:       image.Rect(28, 96, 208, h-32),
			Direction:  render.Vertical,
			AlignItems: render.AlignStretch,
			Gap:        10,
			Children: []render.Component{
				render.NewLabel("gallery.theme-heading", "THEMES"),
				dark, light, windows, editorial, highContrast,
			},
		}

		contentPanel = render.NewPanel("gallery.content")
		contentPanel.Rect = image.Rect(232, 80, w-16, h-16)
		contentPanel.Raised = true
	}

	var tabItems []render.TabItem
	if compact {
		tabItems = []render.TabItem{
			{ID: "controls", Label: "Controls"},
			{ID: "inputs", Label: "Inputs"},
			{ID: "navigation", Label: "Nav"},
			{ID: "feedback", Label: "Alerts"},
			{ID: "viewports", Label: "Maps"},
		}
	} else {
		tabItems = []render.TabItem{
			{ID: "controls", Label: "Controls"},
			{ID: "inputs", Label: "Inputs"},
			{ID: "navigation", Label: "Navigation"},
			{ID: "feedback", Label: "Feedback"},
			{ID: "viewports", Label: "Viewports & Maps"},
		}
	}

	tabs := render.NewTabs("gallery.tabs", tabItems, activeTab, func(next string, _ *render.ApplicationState) {
		activeTab = next
	})
	if compact {
		tabs.SetBounds(image.Rect(16, 64, w-16, 112))
	}

	name := render.NewTextInput("gallery.name", "Project name")
	name.AccessibleName = "Project name"
	nameField := &render.LabeledBox{CompID: "gallery.name-field", Title: "Project Name", Child: name}

	password := render.NewTextInput("gallery.password", "Password")
	password.Value = "gallery secret"
	password.Masked = true
	password.AccessibleName = "Password"
	passwordField := &render.LabeledBox{CompID: "gallery.password-field", Title: "Password", Child: password}

	check := render.NewCheckbox("gallery.notifications", "Show notifications", notificationsEnabled, func(next bool, _ *render.ApplicationState) { notificationsEnabled = next })
	toggle := render.NewSwitch("gallery.autosave", "Autosave changes", autosaveEnabled, func(next bool, _ *render.ApplicationState) { autosaveEnabled = next })
	slider := render.NewSlider("gallery.quality", 0, 100, quality, func(next float32, _ *render.ApplicationState) { quality = next })
	sliderField := &render.LabeledBox{CompID: "gallery.slider-field", Title: "Quality Scale", Child: slider}

	selectFormat := render.NewSelect("gallery.format", []render.SelectOption{
		{Value: "png", Label: "PNG image"},
		{Value: "jpg", Label: "JPEG image"},
		{Value: "webp", Label: "WebP image"},
	}, format, func(next string, _ *render.ApplicationState) { format = next })
	selectFormat.Open = formatOpen
	selectFormat.OnOpenChange = func(next bool, _ *render.ApplicationState) { formatOpen = next }
	formatField := &render.LabeledBox{CompID: "gallery.format-field", Title: "Export Format", Child: selectFormat}

	progress := render.NewProgressBar("gallery.progress")
	progress.Value, progress.Max = quality, 100

	primary := render.NewButton("gallery.primary", "Primary", func(*render.ApplicationState) { quality = 80 })
	primary.Variant = render.VariantPrimary
	primary.Mnemonic = 'P'
	primary.Relations = render.SemanticRelationships{
		DescribedBy: []string{"gallery.action-help"},
		Controls:    []string{"gallery.progress"},
		FlowsTo:     []string{"gallery.secondary"},
	}
	secondary := render.NewButton("gallery.secondary", "Secondary", func(*render.ApplicationState) {})
	secondary.Variant = render.VariantSecondary
	danger := render.NewButton("gallery.danger", "Delete", func(*render.ApplicationState) {})
	danger.Variant = render.VariantDanger

	dialogButton := render.NewButton("gallery.dialog", "Open dialog", func(s *render.ApplicationState) {
		dialog := render.NewAlertDialog("gallery.alert", "Professional defaults", "POEM controls now share theme, focus, input, and accessibility semantics.", func(inner *render.ApplicationState) { inner.CloseOverlay("gallery.alert") })
		dialog.SetBounds(image.Rect(0, 0, w, h))
		s.OpenModal("gallery.alert", dialog, nil)
	})

	menuButton := render.NewButton("gallery.menu-button", "Open menu", func(s *render.ApplicationState) {
		menu := render.NewMenu("gallery.menu", []render.MenuItem{
			{ID: "new", Label: "New document", Shortcut: "Ctrl+N"},
			{ID: "open", Label: "Open...", Shortcut: "Ctrl+O"},
			{Separator: true},
			{ID: "autosave", Label: "Autosave", Checked: autosaveEnabled, OnInvoke: func(*render.ApplicationState) { autosaveEnabled = !autosaveEnabled }},
		})
		menu.OnDismiss = func(inner *render.ApplicationState) { inner.CloseOverlay("gallery.menu") }
		minX := maxInt(16, w-260)
		menu.SetBounds(image.Rect(minX, 120, w-16, 120))
		s.OpenFocusedOverlay("gallery.menu", menu, true)
	})

	toastButton := render.NewButton("gallery.toast-button", "Show toast", func(s *render.ApplicationState) {
		toast := render.NewToast("gallery.toast", "Export complete", "The document was written successfully.")
		toast.Variant = render.VariantSuccess
		minX := maxInt(16, w-320)
		toast.SetBounds(image.Rect(minX, h-120, w-16, h-40))
		toast.OnDismiss = func(inner *render.ApplicationState) { inner.CloseOverlay("gallery.toast") }
		s.OpenOverlay("gallery.toast", toast, false, true)
	})

	undo := render.NewButton("toolbar.undo", "Undo", func(*render.ApplicationState) {})
	redo := render.NewButton("toolbar.redo", "Redo", func(*render.ApplicationState) {})
	save := render.NewButton("toolbar.save", "Save", func(*render.ApplicationState) {})
	if compact {
		toolbarButtonWidth := maxInt(72, (w-96)/3)
		for _, button := range []*render.Button{undo, redo, save} {
			button.FixedWidth = toolbarButtonWidth
			button.Size = render.ControlLarge
		}
	}
	toolbar := render.NewToolbar("gallery.toolbar", undo, redo, save)

	spinner := render.NewSpinner("gallery.spinner", "Loading recent documents")
	skeleton := render.NewSkeleton("gallery.skeleton")
	tooltip := render.NewTooltip("gallery.tooltip", "Keyboard accessible actions")
	tooltip.Visible = true

	textarea := render.NewTextArea("gallery.notes", "Write a longer note...")
	textareaField := &render.LabeledBox{CompID: "gallery.notes-field", Title: "Project Notes", Child: textarea}

	autocomplete := render.NewAutocomplete("gallery.language", "Search languages", autocompleteQuery, []render.AutocompleteOption{
		{Value: "go", Label: "Go", Description: "Fast, explicit systems programming"},
		{Value: "rust", Label: "Rust"},
		{Value: "typescript", Label: "TypeScript"},
		{Value: "python", Label: "Python"},
	}, func(next string, _ *render.ApplicationState) { autocompleteQuery = next }, func(value string, _ *render.ApplicationState) {
		autocompleteQuery = value
	})
	autocomplete.AccessibleName = "Programming language"
	autocompleteField := &render.LabeledBox{CompID: "gallery.language-field", Title: "Programming Language", Child: autocomplete}

	datePicker := render.NewDatePicker("gallery.due-date", dueDate, func(next render.Date, _ *render.ApplicationState) { dueDate = next })
	datePicker.AccessibleName = "Due date"
	dateField := &render.LabeledBox{CompID: "gallery.date-field", Title: "Due Date", Child: datePicker}

	radioA := render.NewRadio("gallery.radio-a", "Balanced", "balanced", true, nil)
	radioB := render.NewRadio("gallery.radio-b", "Fast", "fast", false, nil)
	radioBox := &render.LabeledBox{
		CompID: "gallery.radio-group",
		Title:  "Performance Mode",
		Child: &render.FlexBox{
			CompID:     "gallery.radio-flex",
			Direction:  render.Horizontal,
			AlignItems: render.AlignCenter,
			Gap:        16,
			Children:   []render.Component{radioA, radioB},
		},
	}

	var children []render.Component
	if !compact {
		children = append(children, tabs)
	}
	switch activeTab {
	case "inputs":
		children = append(children,
			galleryHeading("gallery.form-heading", "Input controls"),
			nameField, passwordField, autocompleteField, dateField, textareaField, check, toggle, radioBox, sliderField, formatField,
		)
	case "feedback":
		children = append(children,
			galleryHeading("gallery.form-heading", "Feedback and overlays"),
			gallerySupportingText("gallery.feedback-help", "Loading, progress, and temporary surfaces keep system status visible."),
			toolbar, spinner, skeleton, progress,
			&render.FlexBox{
				CompID:     "gallery.overlay-actions",
				Direction:  render.Horizontal,
				Wrap:       true,
				AlignItems: render.AlignCenter,
				Gap:        10,
				LineGap:    8,
				Children:   []render.Component{menuButton, toastButton, dialogButton},
			},
		)
		if !compact {
			children = append(children, &render.FlexBox{
				CompID:     "gallery.tooltip-row",
				Direction:  render.Horizontal,
				AlignItems: render.AlignStart,
				Children:   []render.Component{tooltip},
			})
		}
	case "navigation":
		breadcrumbs := render.NewBreadcrumbs("gallery.breadcrumbs", []render.BreadcrumbItem{
			{ID: "home", Label: "POEM"},
			{ID: "gallery", Label: "Gallery"},
			{ID: "navigation", Label: "Navigation"},
		})
		tree := render.NewTree("gallery.tree", []render.TreeNode{
			{ID: "framework", Label: "Framework", Children: []render.TreeNode{{ID: "components", Label: "Components"}, {ID: "themes", Label: "Themes"}, {ID: "semantics", Label: "Semantics"}}},
			{ID: "examples", Label: "Examples", Children: []render.TreeNode{{ID: "desktop", Label: "Desktop app"}, {ID: "editor", Label: "Editor"}}},
		}, selectedTreeItem, expandedTreeItems, func(next string, _ *render.ApplicationState) { selectedTreeItem = next })
		tree.OnToggle = func(id string, expanded bool, _ *render.ApplicationState) { expandedTreeItems[id] = expanded }

		pagination := render.NewPagination("gallery.pagination", galleryPage, 12, func(page int, _ *render.ApplicationState) { galleryPage = page })
		if compact {
			pagination.MaxButtons = 3
		}
		accordion := render.NewAccordion("gallery.accordion", []render.AccordionItem{
			{ID: "appearance", Title: "Appearance", Content: render.NewLabel("gallery.accordion.appearance", "Themes and density follow immutable design tokens.")},
			{ID: "accessibility", Title: "Accessibility", Content: render.NewLabel("gallery.accordion.accessibility", "Semantic actions remain platform-neutral.")},
		}, expandedAccordion, func(id string, expanded bool, _ *render.ApplicationState) { expandedAccordion[id] = expanded })

		tableCols := []render.TableColumn{
			{Key: "name", Label: "Component", Width: 220},
			{Key: "category", Label: "Category", Width: 180},
			{Key: "status", Label: "Status"},
		}
		if compact {
			tableCols = []render.TableColumn{
				{Key: "name", Label: "Component", Width: 104},
				{Key: "category", Label: "Category", Width: 88},
				{Key: "status", Label: "Status"},
			}
		}

		table := render.NewDataTable("gallery.table", tableCols, []render.TableRow{
			{ID: "alpha", Values: map[string]string{"name": "Button", "category": "Input", "status": "Stable"}},
			{ID: "beta", Values: map[string]string{"name": "Data table", "category": "Data", "status": "New"}},
			{ID: "gamma", Values: map[string]string{"name": "Text area", "category": "Input", "status": "Stable"}},
			{ID: "delta", Values: map[string]string{"name": "Tree", "category": "Nav", "status": "Stable"}},
		})
		table.SelectedID = selectedTableRow
		table.OnSelect = func(id string, _ *render.ApplicationState) { selectedTableRow = id }
		table.Rect = image.Rect(0, 0, 0, 180)
		if compact {
			table.RowHeight = 48
			table.HeaderHeight = 48
			table.Rect = image.Rect(0, 0, 0, 240)
		}

		children = append(children,
			galleryHeading("gallery.navigation-heading", "Navigation controls"),
			breadcrumbs, tree, pagination, accordion,
			galleryHeading("gallery.table-heading", "Accessible data grid"), table,
		)
	case "viewports":
		imgViewport := &render.ImageViewport{
			CompID:      "gallery.image-viewport",
			ImageWidth:  200,
			ImageHeight: 120,
			Pixels:      sampleImageBytes,
			BGColor:     color.RGBA{15, 23, 42, 255},
			Transform:   viewportTransform,
			MinScale:    0.5,
			MaxScale:    4.0,
			Alt:         "Interactive gesture image viewport",
			OnChange: func(next render.ImageTransform, _ *render.ApplicationState) {
				viewportTransform = next
			},
			Markers: []render.ImageMarker{
				{ID: "pin1", Label: "Feature Alpha", X: 0.35, Y: 0.4, Variant: render.ImageMarkerPrimary, Selected: true},
				{ID: "pin2", Label: "Feature Beta", X: 0.7, Y: 0.65, Variant: render.ImageMarkerSuccess},
			},
			Rect: image.Rect(0, 0, 0, 180),
		}

		mapViewport := &render.ImageViewport{
			CompID:         "gallery.map-viewport",
			MapViewportID:  "scene-demo-map",
			ImageWidth:     200,
			ImageHeight:    120,
			Pixels:         sampleMapBytes,
			BGColor:        color.RGBA{24, 32, 47, 255},
			Transform:      mapViewportTransform,
			MinScale:       0.5,
			MaxScale:       8,
			MapInteraction: true,
			MinPitch:       0,
			MaxPitch:       70,
			Alt:            "Retained vector map canvas viewport",
			OnChange: func(next render.ImageTransform, _ *render.ApplicationState) {
				mapViewportTransform = next
			},
			Rect: image.Rect(0, 0, 0, 200),
		}
		mapHeading := "Retained vector map viewport"
		if compact {
			// A phone gallery previews the fallback surface rather than showing
			// a large empty retained canvas before a map scene is attached.
			mapViewport.MapViewportID = ""
			mapViewport.Rect = image.Rect(0, 0, 0, 160)
			mapHeading = "Map preview surface"
		}

		children = append(children,
			galleryHeading("gallery.viewport-heading", "Interactive viewports"),
			gallerySupportingText("gallery.viewport-subheading", "One finger or left-drag pans. Pinch or the mouse wheel zooms. Two fingers or right-drag adjusts bearing and tilt."),
			imgViewport,
			galleryHeading("gallery.map-heading", mapHeading),
			mapViewport,
			gallerySupportingText("gallery.map-camera-status",
				fmt.Sprintf("Zoom %.2f×  •  Bearing %.0f°  •  Tilt %.0f°",
					mapViewportTransform.Scale, mapViewportTransform.Bearing, mapViewportTransform.Pitch)),
		)
	default: // "controls"
		var themeWidget render.Component
		if compact {
			selectTheme := render.NewSelect("gallery.theme-select", []render.SelectOption{
				{Value: "light", Label: "System light"},
				{Value: "dark", Label: "System dark"},
				{Value: "contrast", Label: "High contrast"},
			}, galleryScheme(state), func(next string, s *render.ApplicationState) {
				applyGalleryScheme(s, next, w, h)
			})
			themeWidget = &render.LabeledBox{CompID: "gallery.theme-select-box", Title: "Appearance", Child: selectTheme}
		} else {
			themeWidget = &render.FlexBox{
				CompID:     "gallery.compact-themes",
				Direction:  render.Horizontal,
				Wrap:       true,
				AlignItems: render.AlignCenter,
				Gap:        6,
				LineGap:    6,
				Children:   []render.Component{dark, light, windows, editorial, highContrast},
			}
		}

		actionsDirection := render.Horizontal
		if compact {
			actionsDirection = render.Vertical
		}

		controlsGroup := []render.Component{
			galleryHeading("gallery.form-heading", "Core controls"),
			themeWidget,
			check, toggle, sliderField, formatField, progress,
			gallerySupportingText("gallery.action-help", "Actions demonstrate priority, feedback, and destructive intent."),
			&render.FlexBox{
				CompID:     "gallery.actions",
				Direction:  actionsDirection,
				AlignItems: render.AlignStretch,
				Gap:        8,
				Children:   []render.Component{primary, secondary, danger, dialogButton},
			},
		}
		children = append(children, controlsGroup...)
	}

	content := &render.FlexBox{
		CompID:     "gallery.controls",
		Direction:  render.Vertical,
		AlignItems: render.AlignStretch,
		Gap:        12,
		Padding:    8,
		Children:   children,
	}

	scroll = render.NewScrollView("gallery.scroll."+activeTab, content)
	if compact {
		scroll.SetBounds(image.Rect(16, 120, w-16, h-8))
		state.Pages = map[string][]render.Component{"gallery": {background, header, title, status, contentPanel, tabs, scroll}}
	} else {
		scroll.SetBounds(image.Rect(244, 92, w-28, h-28))
		state.Pages = map[string][]render.Component{"gallery": {background, header, title, status, sidebar, sidebarContent, contentPanel, scroll}}
	}
}

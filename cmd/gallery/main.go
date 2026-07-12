package main

import (
	"image"

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
)

func main() {
	render.Run(render.AppConfig{
		Title:        "POEM Component Gallery",
		Width:        1180,
		Height:       780,
		Theme:        themes,
		BuildPagesFn: buildGallery,
		Automation:   &render.AutomationConfig{Enabled: true, Mode: "http", Host: "127.0.0.1", Port: 47831},
	})
}

func buildGallery(state *render.ApplicationState) {
	w, h := state.GetWindowSize()
	state.CurrentPage = "gallery"
	_ = state.RegisterShortcut("Ctrl+Shift+Q", func(*render.ApplicationState) { quality = 77 })

	background := render.NewPanel("gallery.background")
	background.Rect = image.Rect(0, 0, w, h)

	header := render.NewPanel("gallery.header")
	header.Rect = image.Rect(20, 18, w-20, 78)
	header.Raised = true
	title := render.NewLabel("gallery.title", "POEM 2.0 Component Gallery")
	title.Typography = render.TypographyTitle
	title.SetBounds(image.Rect(40, 28, 420, 68))
	status := render.NewBadge("gallery.status", "FOUNDATION")
	status.Variant = render.VariantSuccess
	status.SetBounds(image.Rect(w-160, 34, w-42, 64))

	sidebar := render.NewPanel("gallery.sidebar")
	sidebar.Rect = image.Rect(20, 96, 230, h-20)
	sidebar.Raised = true
	dark := render.NewButton("theme.dark", "Modern dark", func(*render.ApplicationState) { _ = themes.Set(render.ModernDarkTheme()) })
	light := render.NewButton("theme.light", "Modern light", func(*render.ApplicationState) { _ = themes.Set(render.ModernLightTheme()) })
	windows := render.NewButton("theme.windows", "Windows", func(*render.ApplicationState) { _ = themes.Set(render.WindowsTheme()) })
	editorial := render.NewButton("theme.editorial", "Editorial", func(*render.ApplicationState) { _ = themes.Set(render.EditorialTheme()) })
	highContrast := render.NewButton("theme.contrast", "High contrast", func(*render.ApplicationState) { _ = themes.Set(render.HighContrastTheme()) })
	sidebarContent := &render.FlexBox{CompID: "gallery.theme-list", Rect: image.Rect(36, 120, 214, h-40), Direction: render.Vertical, AlignItems: render.AlignStretch, Gap: 10, Children: []render.Component{
		render.NewLabel("gallery.theme-heading", "THEMES"), dark, light, windows, editorial, highContrast,
	}}

	contentPanel := render.NewPanel("gallery.content")
	contentPanel.Rect = image.Rect(250, 96, w-20, h-20)
	contentPanel.Raised = true

	tabs := render.NewTabs("gallery.tabs", []render.TabItem{{ID: "controls", Label: "Controls"}, {ID: "inputs", Label: "Inputs"}, {ID: "navigation", Label: "Navigation"}, {ID: "feedback", Label: "Feedback"}}, activeTab, func(next string, _ *render.ApplicationState) { activeTab = next })
	name := render.NewTextInput("gallery.name", "Project name")
	password := render.NewTextInput("gallery.password", "Password")
	password.Value = "gallery secret"
	password.Masked = true
	password.AccessibleName = "Password"
	name.AccessibleName = "Project name"
	nameField := &render.LabeledBox{CompID: "gallery.name-field", Title: "Project name", Child: name}
	check := render.NewCheckbox("gallery.notifications", "Show notifications", notificationsEnabled, func(next bool, _ *render.ApplicationState) { notificationsEnabled = next })
	toggle := render.NewSwitch("gallery.autosave", "Autosave changes", autosaveEnabled, func(next bool, _ *render.ApplicationState) { autosaveEnabled = next })
	slider := render.NewSlider("gallery.quality", 0, 100, quality, func(next float32, _ *render.ApplicationState) { quality = next })
	selectFormat := render.NewSelect("gallery.format", []render.SelectOption{{Value: "png", Label: "PNG image"}, {Value: "jpg", Label: "JPEG image"}, {Value: "webp", Label: "WebP image"}}, format, func(next string, _ *render.ApplicationState) { format = next })
	selectFormat.Open = formatOpen
	selectFormat.OnOpenChange = func(next bool, _ *render.ApplicationState) { formatOpen = next }
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
		menu.SetBounds(image.Rect(w-390, 190, w-150, 190))
		s.OpenFocusedOverlay("gallery.menu", menu, true)
	})
	toastButton := render.NewButton("gallery.toast-button", "Show toast", func(s *render.ApplicationState) {
		toast := render.NewToast("gallery.toast", "Export complete", "The document was written successfully.")
		toast.Variant = render.VariantSuccess
		toast.SetBounds(image.Rect(w-390, h-135, w-50, h-53))
		toast.OnDismiss = func(inner *render.ApplicationState) { inner.CloseOverlay("gallery.toast") }
		s.OpenOverlay("gallery.toast", toast, false, true)
	})
	toolbar := render.NewToolbar("gallery.toolbar", render.NewButton("toolbar.undo", "Undo", func(*render.ApplicationState) {}), render.NewButton("toolbar.redo", "Redo", func(*render.ApplicationState) {}), render.NewButton("toolbar.save", "Save", func(*render.ApplicationState) {}))
	spinner := render.NewSpinner("gallery.spinner", "Loading recent documents")
	skeleton := render.NewSkeleton("gallery.skeleton")
	tooltip := render.NewTooltip("gallery.tooltip", "Keyboard accessible actions")
	tooltip.Visible = true
	textarea := render.NewTextArea("gallery.notes", "Write a longer note...")
	autocomplete := render.NewAutocomplete("gallery.language", "Search languages", autocompleteQuery, []render.AutocompleteOption{{Value: "go", Label: "Go", Description: "Fast, explicit systems programming"}, {Value: "rust", Label: "Rust"}, {Value: "typescript", Label: "TypeScript"}, {Value: "python", Label: "Python"}}, func(next string, _ *render.ApplicationState) { autocompleteQuery = next }, func(value string, _ *render.ApplicationState) {
		autocompleteQuery = value
	})
	autocomplete.AccessibleName = "Programming language"
	datePicker := render.NewDatePicker("gallery.due-date", dueDate, func(next render.Date, _ *render.ApplicationState) { dueDate = next })
	datePicker.AccessibleName = "Due date"
	radioA := render.NewRadio("gallery.radio-a", "Balanced", "balanced", true, nil)
	radioB := render.NewRadio("gallery.radio-b", "Fast", "fast", false, nil)

	children := []render.Component{tabs}
	switch activeTab {
	case "inputs":
		children = append(children, render.NewLabel("gallery.form-heading", "Input controls"), nameField, password, autocomplete, datePicker, textarea, check, toggle, radioA, radioB, slider, selectFormat)
	case "feedback":
		children = append(children, render.NewLabel("gallery.form-heading", "Feedback and overlays"), toolbar, spinner, skeleton, progress,
			&render.FlexBox{CompID: "gallery.overlay-actions", Direction: render.Horizontal, AlignItems: render.AlignCenter, Gap: 10, Children: []render.Component{menuButton, toastButton, dialogButton}},
			&render.FlexBox{CompID: "gallery.tooltip-row", Direction: render.Horizontal, AlignItems: render.AlignStart, Children: []render.Component{tooltip}})
	case "navigation":
		breadcrumbs := render.NewBreadcrumbs("gallery.breadcrumbs", []render.BreadcrumbItem{{ID: "home", Label: "POEM"}, {ID: "gallery", Label: "Gallery"}, {ID: "navigation", Label: "Navigation"}})
		tree := render.NewTree("gallery.tree", []render.TreeNode{
			{ID: "framework", Label: "Framework", Children: []render.TreeNode{{ID: "components", Label: "Components"}, {ID: "themes", Label: "Themes"}, {ID: "semantics", Label: "Semantics"}}},
			{ID: "examples", Label: "Examples", Children: []render.TreeNode{{ID: "desktop", Label: "Desktop app"}, {ID: "editor", Label: "Editor"}}},
		}, selectedTreeItem, expandedTreeItems, func(next string, _ *render.ApplicationState) { selectedTreeItem = next })
		tree.OnToggle = func(id string, expanded bool, _ *render.ApplicationState) { expandedTreeItems[id] = expanded }
		pagination := render.NewPagination("gallery.pagination", galleryPage, 12, func(page int, _ *render.ApplicationState) { galleryPage = page })
		accordion := render.NewAccordion("gallery.accordion", []render.AccordionItem{
			{ID: "appearance", Title: "Appearance", Content: render.NewLabel("gallery.accordion.appearance", "Themes and density follow immutable design tokens.")},
			{ID: "accessibility", Title: "Accessibility", Content: render.NewLabel("gallery.accordion.accessibility", "Semantic actions remain platform-neutral.")},
		}, expandedAccordion, func(id string, expanded bool, _ *render.ApplicationState) { expandedAccordion[id] = expanded })
		table := render.NewDataTable("gallery.table", []render.TableColumn{
			{Key: "name", Label: "Component", Width: 220},
			{Key: "category", Label: "Category", Width: 180},
			{Key: "status", Label: "Status"},
		}, []render.TableRow{
			{ID: "alpha", Values: map[string]string{"name": "Button", "category": "Input", "status": "Stable"}},
			{ID: "beta", Values: map[string]string{"name": "Data table", "category": "Collection", "status": "New"}},
			{ID: "gamma", Values: map[string]string{"name": "Text area", "category": "Input", "status": "Stable"}},
			{ID: "delta", Values: map[string]string{"name": "Tree", "category": "Navigation", "status": "Stable"}},
		})
		table.SelectedID = selectedTableRow
		table.OnSelect = func(id string, _ *render.ApplicationState) { selectedTableRow = id }
		table.Rect = image.Rect(0, 0, 0, 180)
		children = append(children, render.NewLabel("gallery.navigation-heading", "Navigation controls"), breadcrumbs, tree, pagination, accordion,
			render.NewLabel("gallery.table-heading", "Accessible data grid"), table)
	default:
		children = append(children, render.NewLabel("gallery.form-heading", "Core controls"), check, toggle, slider, selectFormat, progress,
			render.NewLabel("gallery.action-help", "Primary updates quality progress; actions continue to Secondary."),
			&render.FlexBox{CompID: "gallery.actions", Direction: render.Horizontal, AlignItems: render.AlignCenter, Gap: 10, Children: []render.Component{primary, secondary, danger, dialogButton}})
	}
	content := &render.FlexBox{CompID: "gallery.controls", Direction: render.Vertical, AlignItems: render.AlignStretch, Gap: 12, Padding: 4, Children: children}
	scroll := render.NewScrollView("gallery.scroll", content)
	scroll.SetBounds(image.Rect(276, 120, w-46, h-44))

	state.Pages = map[string][]render.Component{"gallery": {background, header, title, status, sidebar, sidebarContent, contentPanel, scroll}}
}

package render

import (
	"github.com/mulavdm/poem/pkg/render/components"
	"github.com/mulavdm/poem/pkg/render/layout"
	"github.com/mulavdm/poem/pkg/render/platform"
	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/theme"
	"github.com/mulavdm/poem/pkg/render/types"
)

func NewThemeManager(initial theme.Theme) *theme.Manager { return theme.NewManager(initial) }
func ModernDarkTheme() theme.Theme                       { return theme.ModernDark() }
func ModernLightTheme() theme.Theme                      { return theme.ModernLight() }
func WindowsTheme() theme.Theme                          { return theme.Windows() }
func EditorialTheme() theme.Theme                        { return theme.Editorial() }
func HighContrastTheme() theme.Theme                     { return theme.HighContrast() }

// RequestRepaint marks the active application frame dirty from application
// goroutines. Use it after asynchronous model, IO, or service callbacks mutate
// application-owned state that BuildPages reads.
func RequestRepaint() {
	globalRepaintRequested.Store(true)
}

func NewPanel(id string) *components.Panel       { return components.NewPanel(id) }
func NewLabel(id, text string) *components.Label { return components.NewLabel(id, text) }
func NewButton(id, label string, onClick func(*types.ApplicationState)) *components.Button {
	return components.NewButton(id, label, onClick)
}
func NewTextInput(id, placeholder string) *components.TextInput {
	return components.NewTextInput(id, placeholder)
}
func NewTextArea(id, placeholder string) *components.TextArea {
	return components.NewTextArea(id, placeholder)
}
func NewSlider(id string, min, max, value float32, onChange func(float32, *types.ApplicationState)) *components.Slider {
	return components.NewSlider(id, min, max, value, onChange)
}
func NewCheckbox(id, label string, checked bool, onChange func(bool, *types.ApplicationState)) *components.Checkbox {
	return components.NewCheckbox(id, label, checked, onChange)
}
func NewSwitch(id, label string, checked bool, onChange func(bool, *types.ApplicationState)) *components.Switch {
	return components.NewSwitch(id, label, checked, onChange)
}
func NewRadio(id, label, value string, selected bool, onSelect func(string, *types.ApplicationState)) *components.Radio {
	return components.NewRadio(id, label, value, selected, onSelect)
}
func NewProgressBar(id string) *components.ProgressBar { return components.NewProgressBar(id) }
func NewScrollView(id string, children ...types.Component) *components.ScrollView {
	return components.NewScrollView(id, children...)
}
func NewSeparator(id string) *components.Separator { return components.NewSeparator(id) }
func NewBadge(id, text string) *components.Badge   { return components.NewBadge(id, text) }
func NewTabs(id string, items []components.TabItem, selected string, onChange func(string, *types.ApplicationState)) *components.Tabs {
	return components.NewTabs(id, items, selected, onChange)
}
func NewSelect(id string, options []components.SelectOption, value string, onChange func(string, *types.ApplicationState)) *components.Select {
	return components.NewSelect(id, options, value, onChange)
}
func NewVirtualList(id string, itemCount, itemHeight int, build func(int) types.Component) *components.VirtualList {
	return components.NewVirtualList(id, itemCount, itemHeight, build)
}
func NewDataTable(id string, columns []components.TableColumn, rows []components.TableRow) *components.DataTable {
	return components.NewDataTable(id, columns, rows)
}
func NewSpinner(id, label string) *components.Spinner { return components.NewSpinner(id, label) }
func NewSkeleton(id string) *components.Skeleton      { return components.NewSkeleton(id) }
func NewTooltip(id, text string) *components.Tooltip  { return components.NewTooltip(id, text) }
func NewToast(id, title, message string) *components.Toast {
	return components.NewToast(id, title, message)
}
func NewToolbar(id string, children ...types.Component) *components.Toolbar {
	return components.NewToolbar(id, children...)
}
func NewMenu(id string, items []components.MenuItem) *components.Menu {
	return components.NewMenu(id, items)
}
func NewPopover(id string, children ...types.Component) *components.Popover {
	return components.NewPopover(id, children...)
}
func NewBreadcrumbs(id string, items []components.BreadcrumbItem) *components.Breadcrumbs {
	return components.NewBreadcrumbs(id, items)
}
func NewTree(id string, nodes []components.TreeNode, selected string, expanded map[string]bool, onSelect func(string, *types.ApplicationState)) *components.Tree {
	return components.NewTree(id, nodes, selected, expanded, onSelect)
}
func NewAutocomplete(id, placeholder, query string, options []components.AutocompleteOption, onQueryChange func(string, *types.ApplicationState), onSelect func(string, *types.ApplicationState)) *components.Autocomplete {
	return components.NewAutocomplete(id, placeholder, query, options, onQueryChange, onSelect)
}
func NewPagination(id string, page, pageCount int, onChange func(int, *types.ApplicationState)) *components.Pagination {
	return components.NewPagination(id, page, pageCount, onChange)
}
func NewDatePicker(id string, value components.Date, onChange func(components.Date, *types.ApplicationState)) *components.DatePicker {
	return components.NewDatePicker(id, value, onChange)
}
func NewAccordion(id string, items []components.AccordionItem, expanded map[string]bool, onToggle func(string, bool, *types.ApplicationState)) *components.Accordion {
	return components.NewAccordion(id, items, expanded, onToggle)
}

// Re-export Core API Types
type ApplicationState = types.ApplicationState
type Component = types.Component
type ContentSizedComponent = types.ContentSizedComponent
type MeasurableComponent = types.MeasurableComponent
type MeasureResult = types.MeasureResult
type Painter = types.Painter
type UIRenderer = types.UIRenderer
type ParticleSystem = types.ParticleSystem
type RenderContext = types.RenderContext
type Theme = theme.Theme
type ThemeManager = theme.Manager
type ThemeMode = theme.Mode
type ThemeVariant = theme.Variant
type PlatformServices = platform.Services
type SemanticNode = semantics.Node
type SemanticTree = semantics.Tree
type SemanticRelationships = semantics.Relationships
type TransientStateStore = renderstate.Store

func NewTransientStateStore() *renderstate.Store { return renderstate.NewStore() }

const (
	ThemeModeDark         = theme.ModeDark
	ThemeModeLight        = theme.ModeLight
	ThemeModeHighContrast = theme.ModeHighContrast
)

// Re-export Components
type Panel = components.Panel
type GlassPanel = components.GlassPanel
type Button = components.Button
type Label = components.Label
type DynamicLabel = components.DynamicLabel
type TextInput = components.TextInput
type Slider = components.Slider
type ParticleComponent = components.ParticleComponent
type LineChart = components.LineChart
type ScrollView = components.ScrollView
type Paragraph = components.Paragraph
type TextArea = components.TextArea
type Modal = components.Modal
type ImageView = components.ImageView
type LabeledBox = components.LabeledBox
type Checkbox = components.Checkbox
type Switch = components.Switch
type Radio = components.Radio
type ControlSize = components.ControlSize
type StyleOverride = components.StyleOverride
type ProgressBar = components.ProgressBar
type TextRole = components.TextRole
type TypographyRole = components.TypographyRole
type Separator = components.Separator
type Badge = components.Badge
type TabItem = components.TabItem
type Tabs = components.Tabs
type SelectOption = components.SelectOption
type Select = components.Select
type VirtualList = components.VirtualList
type TableColumn = components.TableColumn
type TableRow = components.TableRow
type DataTable = components.DataTable
type Spinner = components.Spinner
type Skeleton = components.Skeleton
type Tooltip = components.Tooltip
type Toast = components.Toast
type Toolbar = components.Toolbar
type MenuItem = components.MenuItem
type Menu = components.Menu
type Popover = components.Popover
type BreadcrumbItem = components.BreadcrumbItem
type Breadcrumbs = components.Breadcrumbs
type TreeNode = components.TreeNode
type Tree = components.Tree
type AutocompleteOption = components.AutocompleteOption
type Autocomplete = components.Autocomplete
type Pagination = components.Pagination
type Date = components.Date
type DatePicker = components.DatePicker
type AccordionItem = components.AccordionItem
type Accordion = components.Accordion
type Dialog = components.Dialog

func NewDate(year, month, day int) components.Date    { return components.NewDate(year, month, day) }
func ParseDate(value string) (components.Date, error) { return components.ParseDate(value) }

func NewAlertDialog(id, title, message string, onDismiss func(*types.ApplicationState)) *components.Dialog {
	return components.NewAlertDialog(id, title, message, onDismiss)
}

// Re-export Layouts
type FlexBox = layout.FlexBox
type LayoutDirection = layout.LayoutDirection
type FlexAlign = layout.FlexAlign
type FlexJustify = layout.FlexJustify

// Re-export Constants
const (
	ControlSmall      = components.ControlSmall
	ControlMedium     = components.ControlMedium
	ControlLarge      = components.ControlLarge
	TextDefault       = components.TextDefault
	TextMuted         = components.TextMuted
	TextAccent        = components.TextAccent
	TextDanger        = components.TextDanger
	TextSuccess       = components.TextSuccess
	TextWarning       = components.TextWarning
	TypographyBody    = components.TypographyBody
	TypographyCaption = components.TypographyCaption
	TypographyLabel   = components.TypographyLabel
	TypographyTitle   = components.TypographyTitle
	TypographyHeading = components.TypographyHeading

	VariantNeutral   = theme.VariantNeutral
	VariantPrimary   = theme.VariantPrimary
	VariantSecondary = theme.VariantSecondary
	VariantSubtle    = theme.VariantSubtle
	VariantDanger    = theme.VariantDanger
	VariantSuccess   = theme.VariantSuccess
	VariantWarning   = theme.VariantWarning

	Vertical   = layout.Vertical
	Horizontal = layout.Horizontal

	AlignStart   = layout.AlignStart
	AlignCenter  = layout.AlignCenter
	AlignEnd     = layout.AlignEnd
	AlignStretch = layout.AlignStretch

	JustifyStart        = layout.JustifyStart
	JustifyCenter       = layout.JustifyCenter
	JustifyEnd          = layout.JustifyEnd
	JustifySpaceBetween = layout.JustifySpaceBetween

	PageDashboard = types.PageDashboard
	PageAnalytics = types.PageAnalytics
	PageSettings  = types.PageSettings
)

// Re-export Mutable Viewport Dimensions
var (
	Width  = 1024
	Height = 768
)

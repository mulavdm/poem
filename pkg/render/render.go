package render

import (
	"go_native_gpu_gui/pkg/render/components"
	"go_native_gpu_gui/pkg/render/layout"
	"go_native_gpu_gui/pkg/render/types"
)

// Re-export Core API Types
type ApplicationState = types.ApplicationState
type Component = types.Component
type Painter = types.Painter
type UIRenderer = types.UIRenderer
type ParticleSystem = types.ParticleSystem

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

// Re-export Layouts
type FlexBox = layout.FlexBox
type LayoutDirection = layout.LayoutDirection
type FlexAlign = layout.FlexAlign
type FlexJustify = layout.FlexJustify

// Re-export Constants
const (
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

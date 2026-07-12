package components

import (
	"image"
	"image/color"
	"math"

	"github.com/mulavdm/poem/pkg/render/theme"
	"github.com/mulavdm/poem/pkg/render/types"
)

type ControlSize uint8

const (
	ControlMedium ControlSize = iota
	ControlSmall
	ControlLarge
)

type TextRole uint8

const (
	TextDefault TextRole = iota
	TextMuted
	TextAccent
	TextDanger
	TextSuccess
	TextWarning
)

type TypographyRole uint8

const (
	TypographyBody TypographyRole = iota
	TypographyCaption
	TypographyLabel
	TypographyTitle
	TypographyHeading
)

type styledTextPainter interface {
	DrawStyledText(string, int, int, color.RGBA, float32)
}

func typographyStyle(t theme.Theme, role TypographyRole) theme.TextStyle {
	switch role {
	case TypographyCaption:
		return t.Typography.Caption
	case TypographyLabel:
		return t.Typography.Label
	case TypographyTitle:
		return t.Typography.Title
	case TypographyHeading:
		return t.Typography.Heading
	default:
		return t.Typography.Body
	}
}

func drawTypography(p types.Painter, text string, x, y int, col color.RGBA, t theme.Theme, role TypographyRole) {
	style := typographyStyle(t, role)
	scale := float32(1)
	if t.Typography.Body.Size > 0 {
		scale = style.Size / t.Typography.Body.Size
	}
	if styled, ok := p.(styledTextPainter); ok {
		styled.DrawStyledText(text, x, y, col, scale)
		return
	}
	p.DrawText(text, x, y, col)
}

func textRoleColor(t theme.Theme, role TextRole) color.RGBA {
	switch role {
	case TextMuted:
		return t.Colors.TextMuted
	case TextAccent:
		return t.Colors.Accent
	case TextDanger:
		return t.Colors.Danger
	case TextSuccess:
		return t.Colors.Success
	case TextWarning:
		return t.Colors.Warning
	default:
		return t.Colors.Text
	}
}

// badgeVisual derives a tinted pill background and full-strength foreground
// for Badge from the same semantic colors buttonVisual uses. VariantSecondary
// and VariantSubtle fall back to the muted default alongside VariantNeutral,
// since the theme palette has no dedicated secondary hue (buttonVisual's own
// VariantSecondary case has no color of its own either).
func badgeVisual(t theme.Theme, variant theme.Variant) (bg, fg color.RGBA) {
	base := t.Colors.TextMuted
	switch variant {
	case theme.VariantPrimary:
		base = t.Colors.Accent
	case theme.VariantDanger:
		base = t.Colors.Danger
	case theme.VariantSuccess:
		base = t.Colors.Success
	case theme.VariantWarning:
		base = t.Colors.Warning
	}
	return mix(t.Colors.SurfaceRaised, base, 30), base
}

// StyleOverride is intentionally small and semantic. Applications normally use
// theme variants; overrides are for genuinely product-specific controls.
type StyleOverride struct {
	Background        *color.RGBA
	BackgroundHover   *color.RGBA
	BackgroundPressed *color.RGBA
	Foreground        *color.RGBA
	Border            *color.RGBA
	Focus             *color.RGBA
	Radius            *int
}

type controlVisual struct {
	background color.RGBA
	foreground color.RGBA
	border     color.RGBA
	focus      color.RGBA
	radius     int
}

func activeTheme(state *types.ApplicationState) theme.Theme {
	if state == nil {
		return theme.ModernDark()
	}
	current, _ := state.CurrentTheme()
	if state.TextScale > 0 && state.TextScale != 1 {
		scale := state.TextScale
		styles := []*theme.TextStyle{&current.Typography.Caption, &current.Typography.Body, &current.Typography.Label, &current.Typography.Title, &current.Typography.Heading}
		for _, style := range styles {
			style.Size *= scale
			style.LineHeight *= scale
			style.LetterSpacing *= scale
		}
		scaleMetric := func(value int) int { return int(math.Ceil(float64(value) * float64(scale))) }
		current.Controls.Small = scaleMetric(current.Controls.Small)
		current.Controls.Medium = scaleMetric(current.Controls.Medium)
		current.Controls.Large = scaleMetric(current.Controls.Large)
		current.Controls.Icon = scaleMetric(current.Controls.Icon)
	}
	return current
}

func themed(explicit bool, state *types.ApplicationState) bool {
	return explicit || state == nil || !state.LegacyComponentStyles
}

func prefersReducedMotion(state *types.ApplicationState) bool {
	if state == nil {
		return false
	}
	return state.RenderContext().ReducedMotion
}

func controlHeight(t theme.Theme, size ControlSize) int {
	switch size {
	case ControlSmall:
		return t.Controls.Small
	case ControlLarge:
		return t.Controls.Large
	default:
		return t.Controls.Medium
	}
}

func mix(a, b color.RGBA, amount uint8) color.RGBA {
	inv := uint16(255 - amount)
	blend := func(x, y uint8) uint8 { return uint8((uint16(x)*inv + uint16(y)*uint16(amount)) / 255) }
	return color.RGBA{blend(a.R, b.R), blend(a.G, b.G), blend(a.B, b.B), blend(a.A, b.A)}
}

func buttonVisual(t theme.Theme, variant theme.Variant, hovered, pressed, disabled, selected bool, override *StyleOverride) controlVisual {
	visual := controlVisual{
		background: t.Colors.SurfaceRaised, foreground: t.Colors.Text,
		border: t.Colors.Border, focus: t.Colors.Focus, radius: t.Radii.Medium,
	}
	switch variant {
	case theme.VariantPrimary:
		visual.background, visual.foreground, visual.border = t.Colors.Accent, t.Colors.OnAccent, t.Colors.Accent
	case theme.VariantSecondary:
		visual.background, visual.border = t.Colors.Surface, t.Colors.BorderStrong
	case theme.VariantSubtle:
		visual.background, visual.border = t.Colors.Surface, color.RGBA{}
	case theme.VariantDanger:
		visual.background, visual.foreground, visual.border = t.Colors.Danger, t.Colors.OnDanger, t.Colors.Danger
	case theme.VariantSuccess:
		visual.background, visual.foreground, visual.border = t.Colors.Success, t.Colors.OnSuccess, t.Colors.Success
	case theme.VariantWarning:
		visual.background, visual.foreground, visual.border = t.Colors.Warning, t.Colors.OnWarning, t.Colors.Warning
	}
	if selected {
		visual.background, visual.border = t.Colors.Selection, t.Colors.Accent
	}
	if hovered && !disabled {
		if variant == theme.VariantPrimary {
			visual.background = t.Colors.AccentHover
		} else {
			visual.background = mix(visual.background, t.Colors.Text, 18)
		}
	}
	if pressed && !disabled {
		if variant == theme.VariantPrimary {
			visual.background = t.Colors.AccentPressed
		} else {
			visual.background = mix(visual.background, t.Colors.Text, 28)
		}
	}
	if disabled {
		visual.background, visual.foreground, visual.border = t.Colors.Surface, t.Colors.TextDisabled, t.Colors.Border
	}
	applyStyleOverride(&visual, override, hovered, pressed)
	return visual
}

func applyStyleOverride(visual *controlVisual, override *StyleOverride, hovered, pressed bool) {
	if override == nil {
		return
	}
	if override.Background != nil {
		visual.background = *override.Background
	}
	if hovered && override.BackgroundHover != nil {
		visual.background = *override.BackgroundHover
	}
	if pressed && override.BackgroundPressed != nil {
		visual.background = *override.BackgroundPressed
	}
	if override.Foreground != nil {
		visual.foreground = *override.Foreground
	}
	if override.Border != nil {
		visual.border = *override.Border
	}
	if override.Focus != nil {
		visual.focus = *override.Focus
	}
	if override.Radius != nil {
		visual.radius = *override.Radius
	}
}

func drawControlSurface(p types.Painter, bounds image.Rectangle, visual controlVisual, focused bool) image.Rectangle {
	visual.radius = roundedRadius(bounds, visual.radius)
	if focused {
		focusRect := image.Rect(bounds.Min.X-2, bounds.Min.Y-2, bounds.Max.X+2, bounds.Max.Y+2)
		p.DrawRoundedRect(focusRect, visual.radius+2, visual.focus)
	}
	if visual.border.A > 0 {
		p.DrawRoundedRect(bounds, visual.radius, visual.border)
		inner := image.Rect(bounds.Min.X+1, bounds.Min.Y+1, bounds.Max.X-1, bounds.Max.Y-1)
		if !inner.Empty() {
			p.DrawRoundedRect(inner, maxInt(0, visual.radius-1), visual.background)
		}
	} else {
		p.DrawRoundedRect(bounds, visual.radius, visual.background)
	}
	return image.Rect(bounds.Min.X+1, bounds.Min.Y+1, bounds.Max.X-1, bounds.Max.Y-1)
}

func roundedRadius(bounds image.Rectangle, requested int) int {
	limit := minValueInt(bounds.Dx(), bounds.Dy()) / 2
	if limit < 0 {
		return 0
	}
	if requested < 0 {
		return 0
	}
	if requested > limit {
		return limit
	}
	return requested
}

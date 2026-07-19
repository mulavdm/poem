package design

import (
	"fmt"
	"image/color"
	"strings"
)

// CSSVariables serializes resolved semantic tokens for the web renderer.
func CSSVariables(tokens ResolvedTokens) string {
	t := tokens.Theme
	values := []string{
		cssColor("surface", t.Colors.Surface), cssColor("surface-muted", t.Colors.SurfaceSunken),
		cssColor("surface-raised", t.Colors.SurfaceRaised), cssColor("text", t.Colors.Text),
		cssColor("muted", t.Colors.TextMuted), cssColor("line", t.Colors.Border),
		cssColor("border", t.Colors.Border),
		cssColor("line-strong", t.Colors.BorderStrong), cssColor("primary", t.Colors.Accent),
		cssColor("on-primary", t.Colors.OnAccent), cssColor("secondary", t.Colors.TextMuted),
		cssColor("danger", t.Colors.Danger), cssColor("success", t.Colors.Success),
		cssColor("warning", t.Colors.Warning), cssColor("neutral", t.Colors.TextMuted),
		cssColor("focus", t.Colors.Focus), cssColor("selection", t.Colors.Selection),
		cssColor("overlay", t.Colors.Overlay), cssColor("shadow", t.Elevation.Medium.Color),
		fmt.Sprintf("--poem-font:%s", cssFont(t.Typography.BodyFamily)),
		fmt.Sprintf("--poem-space-xs:%.3grem", float64(t.Spacing.XS)/16),
		fmt.Sprintf("--poem-space-sm:%.3grem", float64(t.Spacing.SM)/16),
		fmt.Sprintf("--poem-space-md:%.3grem", float64(t.Spacing.MD)/16),
		fmt.Sprintf("--poem-space-lg:%.3grem", float64(t.Spacing.LG)/16),
		fmt.Sprintf("--poem-space-xl:%.3grem", float64(t.Spacing.XL)/16),
		fmt.Sprintf("--poem-radius-sm:%.3grem", float64(t.Radii.Small)/16),
		fmt.Sprintf("--poem-radius-md:%.3grem", float64(t.Radii.Medium)/16),
		fmt.Sprintf("--poem-radius-lg:%.3grem", float64(t.Radii.Large)/16),
		fmt.Sprintf("--poem-target:%.3grem", float64(t.Controls.Medium)/16),
		fmt.Sprintf("--poem-motion-fast:%dms", t.Motion.FastMS),
		fmt.Sprintf("--poem-motion-normal:%dms", t.Motion.NormalMS),
	}
	return ":root{" + strings.Join(values, ";") + "}"
}

func cssColor(name string, value color.RGBA) string {
	return fmt.Sprintf("--poem-%s:rgb(%d %d %d / %.3g)", name, value.R, value.G, value.B, float64(value.A)/255)
}

func cssFont(value string) string {
	if value == "system-ui" || value == "ui-monospace" {
		return value
	}
	return fmt.Sprintf("%q,system-ui,sans-serif", strings.ReplaceAll(value, `"`, ""))
}

// CSSForWeb returns light/dark token variables plus accessibility media
// behavior. Component rules remain token-only.
func (s *System) CSSForWeb() string {
	light := s.Resolve(Environment{Platform: PlatformWeb, Width: 840, TextScale: 1})
	darkEnvironment := light.Environment
	darkEnvironment.Dark = true
	dark := s.Resolve(darkEnvironment)
	highEnvironment := light.Environment
	highEnvironment.HighContrast = true
	high := s.Resolve(highEnvironment)
	return CSSVariables(light) + "@media(prefers-color-scheme:dark){" + CSSVariables(dark) +
		"}@media(forced-colors:active){" + CSSVariables(high) +
		"}@media(prefers-reduced-motion:reduce){:root{--poem-motion-fast:0ms;--poem-motion-normal:0ms}}"
}

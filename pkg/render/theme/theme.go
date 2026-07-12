// Package theme defines platform-neutral visual design tokens for POEM.
package theme

import (
	"fmt"
	"image/color"
	"math"
)

// Mode describes the intended surface luminance of a theme.
type Mode uint8

const (
	ModeDark Mode = iota
	ModeLight
	ModeHighContrast
)

func (m Mode) String() string {
	switch m {
	case ModeLight:
		return "light"
	case ModeHighContrast:
		return "high-contrast"
	default:
		return "dark"
	}
}

// Density controls the default size and spacing of interactive controls.
type Density uint8

const (
	DensityComfortable Density = iota
	DensityCompact
)

// Variant gives controls semantic intent without coupling them to raw colors.
type Variant uint8

const (
	VariantNeutral Variant = iota
	VariantPrimary
	VariantSecondary
	VariantSubtle
	VariantDanger
	VariantSuccess
	VariantWarning
)

// FontWeight uses CSS-compatible numeric weights while remaining backend-neutral.
type FontWeight uint16

const (
	WeightRegular  FontWeight = 400
	WeightMedium   FontWeight = 500
	WeightSemibold FontWeight = 600
	WeightBold     FontWeight = 700
)

type TextStyle struct {
	Family        string
	Size          float32
	LineHeight    float32
	Weight        FontWeight
	LetterSpacing float32
}

type Typography struct {
	BodyFamily string
	MonoFamily string
	Caption    TextStyle
	Body       TextStyle
	Label      TextStyle
	Title      TextStyle
	Heading    TextStyle
}

type Colors struct {
	Background      color.RGBA
	Surface         color.RGBA
	SurfaceRaised   color.RGBA
	SurfaceSunken   color.RGBA
	Border          color.RGBA
	BorderStrong    color.RGBA
	Text            color.RGBA
	TextMuted       color.RGBA
	TextDisabled    color.RGBA
	Accent          color.RGBA
	AccentHover     color.RGBA
	AccentPressed   color.RGBA
	OnAccent        color.RGBA
	OnDanger        color.RGBA
	OnWarning       color.RGBA
	OnSuccess       color.RGBA
	Focus           color.RGBA
	Danger          color.RGBA
	Warning         color.RGBA
	Success         color.RGBA
	Selection       color.RGBA
	Overlay         color.RGBA
	ScrollbarTrack  color.RGBA
	ScrollbarThumb  color.RGBA
	ScrollbarActive color.RGBA
}

type Spacing struct{ XXS, XS, SM, MD, LG, XL, XXL int }
type Radii struct{ Small, Medium, Large, Pill int }
type ControlSizes struct{ Small, Medium, Large, Icon, Scrollbar int }

type Shadow struct {
	OffsetX, OffsetY int
	Blur             int
	Color            color.RGBA
}

type Elevation struct{ Low, Medium, High Shadow }

type Motion struct {
	FastMS, NormalMS, SlowMS int
	Reduced                  bool
}

// Theme is a complete value object. Managers clone values on publication so a
// published theme can be treated as immutable by render and application code.
type Theme struct {
	Name       string
	Mode       Mode
	Density    Density
	Colors     Colors
	Typography Typography
	Spacing    Spacing
	Radii      Radii
	Controls   ControlSizes
	Elevation  Elevation
	Motion     Motion
}

func (t Theme) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("theme name is required")
	}
	if t.Typography.BodyFamily == "" || t.Typography.Body.Size <= 0 {
		return fmt.Errorf("theme %q has incomplete body typography", t.Name)
	}
	if t.Controls.Medium <= 0 || t.Spacing.MD <= 0 {
		return fmt.Errorf("theme %q has invalid control or spacing metrics", t.Name)
	}
	textPairs := []struct {
		name                   string
		foreground, background color.RGBA
	}{
		{"text/background", t.Colors.Text, t.Colors.Background},
		{"text/surface", t.Colors.Text, t.Colors.Surface},
		{"text/raised surface", t.Colors.Text, t.Colors.SurfaceRaised},
		{"text/sunken surface", t.Colors.Text, t.Colors.SurfaceSunken},
		{"muted text/background", t.Colors.TextMuted, t.Colors.Background},
		{"accent foreground", t.Colors.OnAccent, t.Colors.Accent},
		{"danger foreground", t.Colors.OnDanger, t.Colors.Danger},
		{"warning foreground", t.Colors.OnWarning, t.Colors.Warning},
		{"success foreground", t.Colors.OnSuccess, t.Colors.Success},
	}
	for _, pair := range textPairs {
		if pair.foreground.A == 0 || pair.background.A == 0 {
			return fmt.Errorf("theme %q %s colors must be opaque", t.Name, pair.name)
		}
		if ratio := ContrastRatio(pair.foreground, pair.background); ratio < 4.5 {
			return fmt.Errorf("theme %q %s contrast %.2f is below 4.5", t.Name, pair.name, ratio)
		}
	}
	for _, surface := range []struct {
		name  string
		color color.RGBA
	}{{"background", t.Colors.Background}, {"surface", t.Colors.Surface}} {
		if t.Colors.Focus.A == 0 || surface.color.A == 0 {
			return fmt.Errorf("theme %q focus/%s colors must be opaque", t.Name, surface.name)
		}
		if ratio := ContrastRatio(t.Colors.Focus, surface.color); ratio < 3.0 {
			return fmt.Errorf("theme %q focus/%s contrast %.2f is below 3.0", t.Name, surface.name, ratio)
		}
	}
	if ratio := ContrastRatio(t.Colors.BorderStrong, t.Colors.Surface); ratio < 3.0 {
		return fmt.Errorf("theme %q strong border/surface contrast %.2f is below 3.0", t.Name, ratio)
	}
	return nil
}

func ContrastRatio(a, b color.RGBA) float64 {
	lighter, darker := relativeLuminance(a), relativeLuminance(b)
	if darker > lighter {
		lighter, darker = darker, lighter
	}
	return (lighter + 0.05) / (darker + 0.05)
}

func relativeLuminance(c color.RGBA) float64 {
	linear := func(value uint8) float64 {
		channel := float64(value) / 255
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

// With returns a derived theme without mutating the source value.
func (t Theme) With(name string, apply func(*Theme)) Theme {
	next := t
	if name != "" {
		next.Name = name
	}
	if apply != nil {
		apply(&next)
	}
	return next
}

func ModernDark() Theme {
	body := "Segoe UI"
	mono := "Cascadia Mono"
	return Theme{
		Name: "POEM Modern Dark", Mode: ModeDark, Density: DensityComfortable,
		Colors: Colors{
			Background: color.RGBA{18, 20, 24, 255}, Surface: color.RGBA{27, 30, 36, 255},
			SurfaceRaised: color.RGBA{35, 39, 46, 255}, SurfaceSunken: color.RGBA{21, 24, 29, 255},
			Border: color.RGBA{55, 61, 70, 255}, BorderStrong: color.RGBA{104, 114, 128, 255},
			Text: color.RGBA{238, 240, 244, 255}, TextMuted: color.RGBA{171, 177, 188, 255},
			TextDisabled: color.RGBA{112, 118, 128, 255}, Accent: color.RGBA{91, 141, 239, 255},
			AccentHover: color.RGBA{111, 157, 245, 255}, AccentPressed: color.RGBA{73, 121, 213, 255},
			OnAccent: color.RGBA{0, 0, 0, 255}, OnDanger: color.RGBA{0, 0, 0, 255},
			OnWarning: color.RGBA{0, 0, 0, 255}, OnSuccess: color.RGBA{0, 0, 0, 255}, Focus: color.RGBA{126, 170, 255, 255},
			Danger: color.RGBA{218, 91, 91, 255}, Warning: color.RGBA{218, 166, 74, 255},
			Success: color.RGBA{79, 174, 121, 255}, Selection: color.RGBA{66, 105, 170, 180},
			Overlay: color.RGBA{0, 0, 0, 150}, ScrollbarTrack: color.RGBA{255, 255, 255, 12},
			ScrollbarThumb: color.RGBA{151, 158, 170, 115}, ScrollbarActive: color.RGBA{184, 191, 204, 180},
		},
		Typography: Typography{
			BodyFamily: body, MonoFamily: mono,
			Caption: TextStyle{Family: body, Size: 12, LineHeight: 16, Weight: WeightRegular},
			Body:    TextStyle{Family: body, Size: 14, LineHeight: 20, Weight: WeightRegular},
			Label:   TextStyle{Family: body, Size: 14, LineHeight: 20, Weight: WeightSemibold},
			Title:   TextStyle{Family: body, Size: 18, LineHeight: 24, Weight: WeightSemibold},
			Heading: TextStyle{Family: body, Size: 24, LineHeight: 32, Weight: WeightSemibold},
		},
		Spacing:  Spacing{XXS: 2, XS: 4, SM: 8, MD: 12, LG: 16, XL: 24, XXL: 32},
		Radii:    Radii{Small: 4, Medium: 7, Large: 12, Pill: 999},
		Controls: ControlSizes{Small: 28, Medium: 36, Large: 44, Icon: 36, Scrollbar: 10},
		Elevation: Elevation{
			Low:    Shadow{OffsetY: 1, Blur: 3, Color: color.RGBA{0, 0, 0, 70}},
			Medium: Shadow{OffsetY: 4, Blur: 12, Color: color.RGBA{0, 0, 0, 90}},
			High:   Shadow{OffsetY: 10, Blur: 30, Color: color.RGBA{0, 0, 0, 120}},
		},
		Motion: Motion{FastMS: 90, NormalMS: 160, SlowMS: 240},
	}
}

func ModernLight() Theme {
	return ModernDark().With("POEM Modern Light", func(t *Theme) {
		t.Mode = ModeLight
		t.Colors.Background = color.RGBA{244, 246, 249, 255}
		t.Colors.Surface = color.RGBA{255, 255, 255, 255}
		t.Colors.SurfaceRaised = color.RGBA{255, 255, 255, 255}
		t.Colors.SurfaceSunken = color.RGBA{235, 238, 243, 255}
		t.Colors.Border = color.RGBA{210, 215, 223, 255}
		t.Colors.BorderStrong = color.RGBA{126, 136, 151, 255}
		t.Colors.Text = color.RGBA{31, 35, 42, 255}
		t.Colors.TextMuted = color.RGBA{91, 99, 112, 255}
		t.Colors.TextDisabled = color.RGBA{145, 151, 161, 255}
		t.Colors.Focus = color.RGBA{54, 95, 173, 255}
		t.Colors.Overlay = color.RGBA{16, 20, 28, 105}
		t.Colors.ScrollbarTrack = color.RGBA{0, 0, 0, 8}
		t.Colors.ScrollbarThumb = color.RGBA{75, 82, 94, 90}
		t.Colors.ScrollbarActive = color.RGBA{52, 59, 70, 150}
	})
}

// Windows is a restrained Windows-oriented derivative. It remains a token set,
// not a dependency on Win32, so other presentation backends can render it.
func Windows() Theme {
	return ModernDark().With("POEM Windows", func(t *Theme) {
		t.Radii = Radii{Small: 3, Medium: 5, Large: 8, Pill: 999}
		t.Controls = ControlSizes{Small: 28, Medium: 34, Large: 42, Icon: 34, Scrollbar: 10}
		t.Colors.Accent = color.RGBA{0, 120, 212, 255}
		t.Colors.AccentHover = color.RGBA{25, 134, 220, 255}
		t.Colors.AccentPressed = color.RGBA{0, 90, 158, 255}
	})
}

func HighContrast() Theme {
	return ModernDark().With("POEM High Contrast", func(t *Theme) {
		t.Mode = ModeHighContrast
		t.Colors.Background = color.RGBA{0, 0, 0, 255}
		t.Colors.Surface = color.RGBA{0, 0, 0, 255}
		t.Colors.SurfaceRaised = color.RGBA{0, 0, 0, 255}
		t.Colors.SurfaceSunken = color.RGBA{0, 0, 0, 255}
		t.Colors.Border = color.RGBA{255, 255, 255, 255}
		t.Colors.BorderStrong = color.RGBA{255, 255, 255, 255}
		t.Colors.Text = color.RGBA{255, 255, 255, 255}
		t.Colors.TextMuted = color.RGBA{255, 255, 255, 255}
		t.Colors.TextDisabled = color.RGBA{170, 170, 170, 255}
		t.Colors.Accent = color.RGBA{255, 215, 0, 255}
		t.Colors.AccentHover = color.RGBA{255, 235, 100, 255}
		t.Colors.AccentPressed = color.RGBA{220, 185, 0, 255}
		t.Colors.OnAccent = color.RGBA{0, 0, 0, 255}
		t.Colors.Focus = color.RGBA{255, 255, 0, 255}
		t.Motion.Reduced = true
	})
}

func Editorial() Theme {
	return ModernDark().With("POEM Editorial", func(t *Theme) {
		t.Typography.BodyFamily = "Georgia"
		t.Typography.Body.Family = "Georgia"
		t.Typography.Title.Family = "Georgia"
		t.Typography.Heading.Family = "Georgia"
		t.Colors.Background = color.RGBA{24, 23, 20, 255}
		t.Colors.Surface = color.RGBA{34, 33, 29, 255}
		t.Colors.SurfaceRaised = color.RGBA{43, 41, 36, 255}
		t.Colors.Text = color.RGBA{238, 233, 221, 255}
		t.Colors.TextMuted = color.RGBA{174, 166, 149, 255}
		t.Colors.Accent = color.RGBA{181, 142, 88, 255}
		t.Colors.AccentHover = color.RGBA{205, 169, 116, 255}
		t.Colors.AccentPressed = color.RGBA{151, 113, 68, 255}
	})
}

// Package design resolves semantic design tokens against a platform and
// accessibility environment. Applications describe intent once; renderers
// consume the immutable ResolvedTokens value.
package design

import (
	"image/color"
	"math"
	"sync"

	"github.com/mulavdm/poem/pkg/render/theme"
)

// Platform identifies a presentation target without exposing its toolkit.
type Platform uint8

const (
	PlatformUnknown Platform = iota
	PlatformWindows
	PlatformAndroid
	PlatformWeb
)

// WindowClass is the adaptive width class in density-independent pixels.
type WindowClass uint8

const (
	WindowCompact WindowClass = iota
	WindowMedium
	WindowExpanded
	WindowUltraWide
)

// ClassifyWidth applies the framework's shared adaptive breakpoints.
func ClassifyWidth(width int) WindowClass {
	switch {
	case width >= 1200:
		return WindowUltraWide
	case width >= 840:
		return WindowExpanded
	case width >= 600:
		return WindowMedium
	default:
		return WindowCompact
	}
}

// PointerPrecision describes the most appropriate pointing target size.
type PointerPrecision uint8

const (
	PointerUnknown PointerPrecision = iota
	PointerCoarse
	PointerFine
)

// Orientation describes the logical viewport orientation.
type Orientation uint8

const (
	OrientationPortrait Orientation = iota
	OrientationLandscape
)

// Density controls information and control density independently of width.
type Density uint8

const (
	DensityComfortable Density = iota
	DensityStandard
	DensityCompact
)

// RenderingMode selects platform conventions or the neutral POEM house style.
type RenderingMode uint8

const (
	// RenderingModeAuto uses a built-in platform profile and falls back to the
	// adaptive house profile when the platform is unknown.
	RenderingModeAuto RenderingMode = iota
	RenderingModePlatformNative
	RenderingModeAdaptiveHouse
)

// InputCapabilities describes available input paths. Hover and gestures are
// never assumed to be the only path to functionality.
type InputCapabilities struct {
	Pointer        PointerPrecision
	HoverAvailable bool
	Keyboard       bool
	Touch          bool
	Trackpad       bool
	Stylus         bool
}

// Environment is the complete adaptation input snapshot.
type Environment struct {
	Platform      Platform
	Width         int
	Height        int
	WindowClass   WindowClass
	Input         InputCapabilities
	Orientation   Orientation
	Density       Density
	ReducedMotion bool
	HighContrast  bool
	Dark          bool
	TextScale     float32
	SystemAccent  *color.RGBA
	// Locale is a BCP-47 language tag. UnitSystem is "metric", "imperial",
	// or empty when the application should derive it from Locale.
	Locale     string
	UnitSystem string
	GPU        GPUCapabilities
	Background BackgroundCapabilities
}

type GPUCapabilities struct {
	RetainedMeshes bool
	WebGL2         bool
	GLES3          bool
	D3D11          bool
	MaxTextureSize int
	MemoryBudget   int64
}

type BackgroundCapabilities struct {
	LocationContinuous bool
	ExtendedExecution  bool
	Notifications      bool
	Speech             bool
	Haptics            bool
}

// Normalized fills derived and zero-value environment fields.
func (e Environment) Normalized() Environment {
	if e.Width > 0 {
		e.WindowClass = ClassifyWidth(e.Width)
	}
	if e.Height > e.Width && e.Width > 0 {
		e.Orientation = OrientationPortrait
	} else {
		e.Orientation = OrientationLandscape
	}
	if e.TextScale <= 0 {
		e.TextScale = 1
	}
	if e.Density == DensityComfortable && e.Input.Pointer == PointerFine && !e.Input.Touch {
		e.Density = DensityCompact
	}
	return e
}

// FoundationTokens are the only layer that contains raw design coordinates.
type FoundationTokens struct {
	Spacing        theme.Spacing
	Radii          theme.Radii
	FallbackAccent color.RGBA
}

// SemanticTokens bind design purpose to light, dark, and contrast values.
type SemanticTokens struct {
	Light        theme.Theme
	Dark         theme.Theme
	HighContrast theme.Theme
}

// PlatformProfile contains platform-resolved typography and metrics.
type PlatformProfile struct {
	Available        bool
	BodyFamily       string
	MonoFamily       string
	CompactControls  theme.ControlSizes
	StandardControls theme.ControlSizes
	TouchControls    theme.ControlSizes
	Radii            theme.Radii
	Motion           theme.Motion
}

// ResolvedTokens is the immutable renderer input for one environment.
type ResolvedTokens struct {
	Theme       theme.Theme
	Environment Environment
	Mode        RenderingMode
	Profile     PlatformProfile
}

// System is a complete design system. Its internal cache is process-local and
// does not change the value semantics of its public token definitions.
type System struct {
	Mode       RenderingMode
	Foundation FoundationTokens
	Semantic   SemanticTokens
	Windows    PlatformProfile
	Android    PlatformProfile
	Web        PlatformProfile
	Overrides  Overrides

	cache *resolutionCache
}

// ColorOverrides selectively replaces semantic color roles. Pointer fields
// distinguish an intentional transparent/black value from no override.
type ColorOverrides struct {
	Canvas  *color.RGBA
	Sunken  *color.RGBA
	Raised  *color.RGBA
	Text    *color.RGBA
	Muted   *color.RGBA
	Border  *color.RGBA
	Accent  *color.RGBA
	Success *color.RGBA
	Warning *color.RGBA
	Danger  *color.RGBA
}

// Overrides contains product-level semantic colors for light and dark
// environments. High-contrast resolution intentionally remains system-owned.
type Overrides struct {
	Light ColorOverrides
	Dark  ColorOverrides
}

// WithOverrides returns an independent design system with the supplied
// product semantics. Foundation/profile values and the receiver are unchanged.
func (s *System) WithOverrides(overrides Overrides) *System {
	s = s.initialized()
	copy := *s
	copy.Overrides = overrides
	copy.cache = &resolutionCache{values: make(map[cacheKey]ResolvedTokens)}
	return &copy
}

type resolutionCache struct {
	mu     sync.RWMutex
	values map[cacheKey]ResolvedTokens
}

type cacheKey struct {
	platform                Platform
	class                   WindowClass
	density                 Density
	dark, contrast, reduced bool
	textScale               uint32
	accent                  color.RGBA
	hasAccent               bool
	mode                    RenderingMode
}

var defaultAccent = color.RGBA{91, 141, 239, 255}
var systemCaches sync.Map

// DefaultSystem returns POEM's built-in native profiles and adaptive fallback.
func DefaultSystem() *System {
	dark, light := theme.ModernDark(), theme.ModernLight()
	foundation := FoundationTokens{Spacing: dark.Spacing, Radii: dark.Radii, FallbackAccent: defaultAccent}
	compact := theme.ControlSizes{Small: 28, Medium: 32, Large: 40, Icon: 32, Scrollbar: 10}
	standard := theme.ControlSizes{Small: 32, Medium: 40, Large: 44, Icon: 40, Scrollbar: 10}
	touch := theme.ControlSizes{Small: 44, Medium: 48, Large: 52, Icon: 48, Scrollbar: 12}
	return &System{
		Foundation: foundation,
		Semantic:   SemanticTokens{Light: light, Dark: dark, HighContrast: theme.HighContrast()},
		Windows: PlatformProfile{Available: true, BodyFamily: "Segoe UI Variable", MonoFamily: "Cascadia Mono", CompactControls: compact,
			StandardControls: theme.ControlSizes{Small: 28, Medium: 36, Large: 44, Icon: 36, Scrollbar: 10}, TouchControls: touch,
			Radii: theme.Radii{Small: 4, Medium: 4, Large: 8, Pill: 999}, Motion: theme.Motion{FastMS: 100, NormalMS: 167, SlowMS: 250}},
		Android: PlatformProfile{Available: true, BodyFamily: "Roboto", MonoFamily: "Roboto Mono", CompactControls: standard,
			StandardControls: touch, TouchControls: touch, Radii: theme.Radii{Small: 12, Medium: 20, Large: 28, Pill: 999},
			Motion: theme.Motion{FastMS: 150, NormalMS: 250, SlowMS: 300}},
		Web: PlatformProfile{Available: true, BodyFamily: "system-ui", MonoFamily: "ui-monospace", CompactControls: compact,
			StandardControls: standard, TouchControls: touch, Radii: theme.Radii{Small: 6, Medium: 8, Large: 12, Pill: 999},
			Motion: theme.Motion{FastMS: 120, NormalMS: 200, SlowMS: 300}},
		cache: &resolutionCache{values: make(map[cacheKey]ResolvedTokens)},
	}
}

func (s *System) initialized() *System {
	if s == nil || s.Semantic.Dark.Name == "" {
		return DefaultSystem()
	}
	if s.cache == nil {
		cached, _ := systemCaches.LoadOrStore(s, &resolutionCache{values: make(map[cacheKey]ResolvedTokens)})
		copy := *s
		copy.cache = cached.(*resolutionCache)
		return &copy
	}
	return s
}

// Resolve returns cached semantic and platform tokens for environment.
func (s *System) Resolve(environment Environment) ResolvedTokens {
	s = s.initialized()
	environment = environment.Normalized()
	key := makeCacheKey(s.Mode, environment)
	s.cache.mu.RLock()
	resolved, ok := s.cache.values[key]
	s.cache.mu.RUnlock()
	if ok {
		return resolved
	}
	resolved = s.resolve(environment)
	s.cache.mu.Lock()
	s.cache.values[key] = resolved
	s.cache.mu.Unlock()
	return resolved
}

func makeCacheKey(mode RenderingMode, e Environment) cacheKey {
	key := cacheKey{platform: e.Platform, class: e.WindowClass, density: e.Density, dark: e.Dark, contrast: e.HighContrast,
		reduced: e.ReducedMotion, textScale: math.Float32bits(e.TextScale), mode: mode}
	if e.SystemAccent != nil {
		key.accent, key.hasAccent = *e.SystemAccent, true
	}
	return key
}

func (s *System) resolve(e Environment) ResolvedTokens {
	base := s.Semantic.Light
	if e.Dark {
		base = s.Semantic.Dark
	}
	if e.HighContrast {
		base = s.Semantic.HighContrast
	} else if e.Dark {
		applyColorOverrides(&base, s.Overrides.Dark)
	} else {
		applyColorOverrides(&base, s.Overrides.Light)
	}
	mode := s.Mode
	if mode == RenderingModeAuto {
		mode = RenderingModePlatformNative
	}
	profile := s.profile(e.Platform)
	if mode == RenderingModeAdaptiveHouse || !profile.Available {
		mode = RenderingModeAdaptiveHouse
		profile = PlatformProfile{Available: true, BodyFamily: base.Typography.BodyFamily, MonoFamily: base.Typography.MonoFamily,
			CompactControls: base.Controls, StandardControls: base.Controls, TouchControls: theme.ControlSizes{Small: 44, Medium: 48, Large: 52, Icon: 48, Scrollbar: 12},
			Radii: s.Foundation.Radii, Motion: base.Motion}
	}
	base.Typography.BodyFamily, base.Typography.MonoFamily = profile.BodyFamily, profile.MonoFamily
	for _, style := range []*theme.TextStyle{&base.Typography.Caption, &base.Typography.Body, &base.Typography.Label, &base.Typography.Title, &base.Typography.Heading} {
		style.Family = profile.BodyFamily
	}
	base.Controls = profile.StandardControls
	if e.Density == DensityCompact {
		base.Controls = profile.CompactControls
	} else if e.Density == DensityComfortable || e.Input.Pointer == PointerCoarse {
		base.Controls = profile.TouchControls
	}
	base.Radii, base.Motion = profile.Radii, profile.Motion
	base.Motion.Reduced = e.ReducedMotion || e.HighContrast
	if !e.HighContrast {
		applyAccent(&base, s.Foundation.FallbackAccent, e.SystemAccent)
		var productAccent *color.RGBA
		if e.Dark {
			productAccent = s.Overrides.Dark.Accent
		} else {
			productAccent = s.Overrides.Light.Accent
		}
		if productAccent != nil {
			applyAccent(&base, *productAccent, nil)
		}
	}
	return ResolvedTokens{Theme: base, Environment: e, Mode: mode, Profile: profile}
}

func applyColorOverrides(target *theme.Theme, values ColorOverrides) {
	assign := func(dst *color.RGBA, src *color.RGBA) {
		if src != nil {
			*dst = *src
		}
	}
	assign(&target.Colors.Background, values.Canvas)
	assign(&target.Colors.Surface, values.Canvas)
	assign(&target.Colors.SurfaceSunken, values.Sunken)
	assign(&target.Colors.SurfaceRaised, values.Raised)
	assign(&target.Colors.Text, values.Text)
	assign(&target.Colors.TextMuted, values.Muted)
	assign(&target.Colors.Border, values.Border)
	assign(&target.Colors.Success, values.Success)
	assign(&target.Colors.Warning, values.Warning)
	assign(&target.Colors.Danger, values.Danger)
	if values.Success != nil {
		target.Colors.OnSuccess = bestForeground(target.Colors.Success)
	}
	if values.Warning != nil {
		target.Colors.OnWarning = bestForeground(target.Colors.Warning)
	}
	if values.Danger != nil {
		target.Colors.OnDanger = bestForeground(target.Colors.Danger)
	}
}

func (s *System) profile(platform Platform) PlatformProfile {
	switch platform {
	case PlatformWindows:
		return s.Windows
	case PlatformAndroid:
		return s.Android
	case PlatformWeb:
		return s.Web
	default:
		return PlatformProfile{}
	}
}

func applyAccent(target *theme.Theme, fallback color.RGBA, candidate *color.RGBA) {
	accent := fallback
	if candidate != nil && candidate.A == 255 {
		accent = *candidate
	}
	target.Colors.Accent = accent
	target.Colors.OnAccent = bestForeground(accent)
	target.Colors.AccentHover = shift(accent, target.Mode == theme.ModeDark, 0.10)
	target.Colors.AccentPressed = shift(accent, target.Mode == theme.ModeDark, 0.18)
	target.Colors.Focus = contrastColor(accent, target.Colors.Background, 3)
}

func bestForeground(background color.RGBA) color.RGBA {
	black, white := color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}
	if theme.ContrastRatio(white, background) >= theme.ContrastRatio(black, background) {
		return white
	}
	return black
}

func contrastColor(candidate, background color.RGBA, minimum float64) color.RGBA {
	if theme.ContrastRatio(candidate, background) >= minimum {
		return candidate
	}
	black, white := color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}
	if theme.ContrastRatio(black, background) >= theme.ContrastRatio(white, background) {
		return black
	}
	return white
}

func shift(value color.RGBA, lighten bool, amount float64) color.RGBA {
	toward := uint8(0)
	if lighten {
		toward = 255
	}
	blend := func(channel uint8) uint8 { return uint8(float64(channel)*(1-amount) + float64(toward)*amount) }
	return color.RGBA{blend(value.R), blend(value.G), blend(value.B), value.A}
}

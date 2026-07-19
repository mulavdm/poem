package types

import (
	"time"

	"github.com/mulavdm/poem/pkg/design"
	"github.com/mulavdm/poem/pkg/render/platform"
	"github.com/mulavdm/poem/pkg/render/theme"
)

// RenderContext is the platform-neutral snapshot used while measuring,
// laying out, painting, and describing a frame.
type RenderContext struct {
	Theme         theme.Theme
	ThemeRevision uint64
	Services      platform.Services
	Locale        string
	DPIScale      float32
	TextScale     float32
	ReducedMotion bool
	Now           time.Time
	Design        design.ResolvedTokens
}

// ResolveDesign updates the active immutable tokens for a logical size.
func (s *ApplicationState) ResolveDesign(width, height int) bool {
	if s == nil || s.DesignSystem == nil {
		return false
	}
	environment := s.DesignEnvironment
	environment.Width, environment.Height = width, height
	environment.TextScale = s.TextScale
	environment.ReducedMotion = s.ReducedMotion
	resolved := s.DesignSystem.Resolve(environment)
	s.DesignEnvironment = resolved.Environment
	current, _ := s.CurrentTheme()
	return current != resolved.Theme && s.SetTheme(resolved.Theme) == nil
}

func (s *ApplicationState) CurrentTheme() (theme.Theme, uint64) {
	if s != nil && s.ThemeManager != nil {
		return s.ThemeManager.Current()
	}
	return theme.ModernDark(), 0
}

func (s *ApplicationState) SetTheme(next theme.Theme) error {
	if s.ThemeManager == nil {
		s.ThemeManager = theme.NewManager(next)
		return nil
	}
	return s.ThemeManager.Set(next)
}

func (s *ApplicationState) RenderContext() RenderContext {
	current, revision := s.CurrentTheme()
	textScale := s.TextScale
	if textScale <= 0 {
		textScale = 1
	}
	dpiScale := s.DPIScale
	if dpiScale <= 0 {
		dpiScale = 1
	}
	var resolved design.ResolvedTokens
	if s.DesignSystem != nil {
		resolved = s.DesignSystem.Resolve(s.DesignEnvironment)
	}
	return RenderContext{
		Theme: current, ThemeRevision: revision, Services: s.Services,
		Locale: s.Locale, DPIScale: dpiScale, TextScale: textScale,
		ReducedMotion: s.ReducedMotion || current.Motion.Reduced, Now: time.Now(), Design: resolved,
	}
}

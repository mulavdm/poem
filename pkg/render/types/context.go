package types

import (
	"time"

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
	return RenderContext{
		Theme: current, ThemeRevision: revision, Services: s.Services,
		Locale: s.Locale, DPIScale: dpiScale, TextScale: textScale,
		ReducedMotion: s.ReducedMotion || current.Motion.Reduced, Now: time.Now(),
	}
}

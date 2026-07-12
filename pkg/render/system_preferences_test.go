package render

import (
	"context"
	"testing"
	"time"

	"github.com/mulavdm/poem/pkg/render/platform"
	"github.com/mulavdm/poem/pkg/render/theme"
	"github.com/mulavdm/poem/pkg/render/types"
)

type preferenceSequence struct {
	snapshots []platform.PreferenceSnapshot
	index     int
}

func (p *preferenceSequence) Current(context.Context) (platform.PreferenceSnapshot, error) {
	if p.index >= len(p.snapshots) {
		return p.snapshots[len(p.snapshots)-1], nil
	}
	next := p.snapshots[p.index]
	p.index++
	return next, nil
}

func TestApplySystemPreferencesUsesProductResolver(t *testing.T) {
	manager := theme.NewManager(theme.ModernDark())
	state := &types.ApplicationState{}
	config := AccessibilityConfig{
		FollowSystemTheme:   true,
		FollowReducedMotion: true,
		SystemThemeResolver: func(mode theme.Mode) theme.Theme {
			base := theme.ModernLight()
			name := "Product Light"
			if mode == theme.ModeHighContrast {
				base = theme.HighContrast()
				name = "Product High Contrast"
			}
			return base.With(name, nil)
		},
	}
	if !applySystemPreferences(state, manager, config, platform.PreferenceSnapshot{ThemeMode: theme.ModeHighContrast, ReducedMotion: true}) {
		t.Fatal("preference update was ignored")
	}
	current, _ := manager.Current()
	if current.Name != "Product High Contrast" || !state.ReducedMotion {
		t.Fatalf("theme=%q reduced=%v", current.Name, state.ReducedMotion)
	}
	if applySystemPreferences(state, manager, config, platform.PreferenceSnapshot{ThemeMode: theme.ModeHighContrast, ReducedMotion: true}) {
		t.Fatal("identical preference update caused a repaint")
	}
}

func TestExplicitReducedMotionCannotBeDisabledBySystem(t *testing.T) {
	manager := theme.NewManager(theme.ModernDark())
	state := &types.ApplicationState{ReducedMotion: true}
	config := AccessibilityConfig{ReducedMotion: true, FollowReducedMotion: true}
	if applySystemPreferences(state, manager, config, platform.PreferenceSnapshot{}) || !state.ReducedMotion {
		t.Fatal("system preference disabled explicit reduced motion")
	}
}

func TestPollSystemPreferencesPublishesOnlyChanges(t *testing.T) {
	service := &preferenceSequence{snapshots: []platform.PreferenceSnapshot{
		{ThemeMode: theme.ModeDark},
		{ThemeMode: theme.ModeDark},
		{ThemeMode: theme.ModeLight, ReducedMotion: true},
	}}
	updates, stop := pollSystemPreferences(service, time.Millisecond)
	defer close(stop)
	first := <-updates
	if first.ThemeMode != theme.ModeDark {
		t.Fatalf("first snapshot=%+v", first)
	}
	select {
	case second := <-updates:
		if second.ThemeMode != theme.ModeLight || !second.ReducedMotion {
			t.Fatalf("changed snapshot=%+v", second)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("changed preferences were not published")
	}
}

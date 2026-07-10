package types

import (
	"testing"

	"go_native_gpu_gui/pkg/render/theme"
)

func TestReducedMotionCompletesTransitionImmediately(t *testing.T) {
	state := &ApplicationState{
		ThemeManager:       theme.NewManager(theme.ModernDark()),
		ReducedMotion:      true,
		IsTransitioning:    true,
		TransitionProgress: 0.25,
	}
	state.UpdateAnimations(0.01)
	if state.IsTransitioning || state.TransitionProgress != 1 {
		t.Fatalf("transition remained animated: active=%v progress=%v", state.IsTransitioning, state.TransitionProgress)
	}
}

func TestThemeCanRequireReducedMotion(t *testing.T) {
	state := &ApplicationState{ThemeManager: theme.NewManager(theme.HighContrast())}
	if !state.RenderContext().ReducedMotion {
		t.Fatal("theme reduced-motion preference was ignored")
	}
}

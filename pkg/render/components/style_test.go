package components

import (
	"image"
	"testing"

	"go_native_gpu_gui/pkg/render/theme"
	"go_native_gpu_gui/pkg/render/types"
)

func TestActiveThemeAppliesTextScaleWithoutMutatingSnapshot(t *testing.T) {
	manager := theme.NewManager(theme.ModernLight())
	state := &types.ApplicationState{ThemeManager: manager, TextScale: 1.5}
	original, _ := manager.Current()
	scaled := activeTheme(state)
	if scaled.Typography.Body.Size != original.Typography.Body.Size*1.5 {
		t.Fatalf("body size=%v want=%v", scaled.Typography.Body.Size, original.Typography.Body.Size*1.5)
	}
	if scaled.Controls.Medium <= original.Controls.Medium {
		t.Fatalf("control height did not scale: %d <= %d", scaled.Controls.Medium, original.Controls.Medium)
	}
	after, _ := manager.Current()
	if after.Typography.Body.Size != original.Typography.Body.Size || after.Controls.Medium != original.Controls.Medium {
		t.Fatal("published theme snapshot was mutated by text scaling")
	}
}

func TestReducedMotionCombinesApplicationAndThemePreferences(t *testing.T) {
	if prefersReducedMotion(&types.ApplicationState{ThemeManager: theme.NewManager(theme.ModernDark())}) {
		t.Fatal("normal theme unexpectedly reduced motion")
	}
	if !prefersReducedMotion(&types.ApplicationState{ThemeManager: theme.NewManager(theme.HighContrast())}) {
		t.Fatal("high-contrast theme reduced-motion token was ignored")
	}
	if !prefersReducedMotion(&types.ApplicationState{ThemeManager: theme.NewManager(theme.ModernDark()), ReducedMotion: true}) {
		t.Fatal("application reduced-motion preference was ignored")
	}
}

func TestRoundedRadiusClampsPillTokenToControlGeometry(t *testing.T) {
	if got := roundedRadius(image.Rect(0, 0, 200, 4), 999); got != 2 {
		t.Fatalf("track radius=%d want=2", got)
	}
}

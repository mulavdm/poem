package components

import (
	"image/color"
	"testing"

	"github.com/mulavdm/poem/pkg/render/theme"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestPanelScopedBackgroundOverrideDoesNotMutateTheme(t *testing.T) {
	manager := theme.NewManager(theme.ModernLight())
	state := &types.ApplicationState{ThemeManager: manager}
	custom := color.RGBA{40, 60, 100, 255}
	panel := NewPanel("cover")
	panel.Style = &StyleOverride{Background: &custom}
	visual := activeTheme(state)
	if *panel.Style.Background != custom || visual.Colors.Surface == custom {
		t.Fatalf("override=%v surface=%v", *panel.Style.Background, visual.Colors.Surface)
	}
}

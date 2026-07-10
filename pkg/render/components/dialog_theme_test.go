package components

import (
	"image"
	"strings"
	"testing"

	"go_native_gpu_gui/pkg/render/types"
)

func TestDialogAppliesInitialRevisionZeroTheme(t *testing.T) {
	state := &types.ApplicationState{}
	dialog := &Dialog{CompID: "pause", Title: "Paused", Message: "Message", UseTheme: true}
	_, revision := state.CurrentTheme()
	if revision != 0 {
		t.Fatalf("test requires initial revision zero, got %d", revision)
	}
	dialog.applyTheme(state)
	if dialog.TextColor.A == 0 || !dialog.themeApplied {
		t.Fatalf("dialog theme not initialized: %+v", dialog)
	}
}

func TestDialogButtonsSizeToLabels(t *testing.T) {
	dialog := &Dialog{CompID: "pause", Buttons: []*Button{{CompID: "restart", Label: "Restart run"}}}
	dialog.SetBounds(image.Rect(0, 0, 800, 600))
	if width := dialog.Buttons[0].Bounds().Dx(); width <= 100 {
		t.Fatalf("descriptive action width=%d", width)
	}
}

func TestDialogMessageWrapsIntoVisibleSemanticLabels(t *testing.T) {
	dialog := &Dialog{CompID: "pause", Message: "Take a breath. Resume this room or restart from the Habitrail map."}
	dialog.SetBounds(image.Rect(0, 0, 800, 600))
	children := dialog.ChildComponents()
	lines := 0
	for _, child := range children {
		if strings.HasPrefix(child.ID(), "pause_msg_") {
			lines++
		}
	}
	if lines != 2 {
		t.Fatalf("message lines=%d children=%d", lines, len(children))
	}
}

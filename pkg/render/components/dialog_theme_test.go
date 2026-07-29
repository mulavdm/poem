package components

import (
	"image"
	"strings"
	"testing"

	"github.com/mulavdm/poem/pkg/render/types"
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
	dialog := &Dialog{CompID: "pause", Buttons: []*Button{{CompID: "restart", Text: "Restart run"}}}
	dialog.SetBounds(image.Rect(0, 0, 800, 600))
	if width := dialog.Buttons[0].Bounds().Dx(); width <= 100 {
		t.Fatalf("descriptive action width=%d", width)
	}
}

// TestDialogManyButtonsWrapWithoutOverlapOrOverflow guards against the
// regression buildModal was fixed for: a hardcoded 400x250 card laid every
// button out in one un-wrapping row starting from the right edge, so past a
// handful of buttons most of them rendered outside the card entirely,
// overlapping or clipped. Buttons must now wrap onto new rows that fit
// inside the (grown-to-fit) card, with no two overlapping.
func TestDialogManyButtonsWrapWithoutOverlapOrOverflow(t *testing.T) {
	buttons := make([]*Button, 0, 12)
	for _, label := range []string{"Save As", "Undo", "Redo", "Refresh", "Move", "Rotate", "Scale", "World", "Snap Off 0.25", "Run Game", "Help", "Close"} {
		buttons = append(buttons, &Button{CompID: "btn_" + label, Text: label})
	}
	dialog := &Dialog{CompID: "overflow", Title: "More editor actions", Message: "File, transform, and game actions", Buttons: buttons}
	dialog.SetBounds(image.Rect(0, 0, 1440, 900))
	card := dialog.ChildComponents()[0].(*Panel)

	for i, a := range buttons {
		if a.Bounds().Empty() {
			t.Fatalf("button %q has an empty rect", a.CompID)
		}
		if !a.Bounds().In(card.Rect) {
			t.Fatalf("button %q at %v falls outside the card %v", a.CompID, a.Bounds(), card.Rect)
		}
		for j, b := range buttons {
			if i != j && a.Bounds().Overlaps(b.Bounds()) {
				t.Fatalf("buttons %q and %q overlap: %v vs %v", a.CompID, b.CompID, a.Bounds(), b.Bounds())
			}
		}
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

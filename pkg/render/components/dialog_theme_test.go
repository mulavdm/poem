package components

import (
	"image"
	"image/color"
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

// TestDialogMessageIsOneLabelTallEnoughToAvoidOverlap guards against the
// regression buildModal was fixed for: the message used to be pre-wrapped
// by an assumed rune-per-line count into one fixed-24px-tall Label per
// line, while each Label independently re-wrapped by its own pixel width at
// Draw time -- when the two disagreed, a label's internal re-wrap drew a
// second sub-line into the next label's 24px slot, overlapping it. The
// message is now one Label spanning a rect sized from the same wrapping
// algorithm the Label itself uses, with no other label positioned inside
// that span to overlap.
func TestDialogMessageIsOneLabelTallEnoughToAvoidOverlap(t *testing.T) {
	dialog := &Dialog{CompID: "pause", Title: "Paused", Message: "Take a breath. Resume this room or restart from the Habitrail map.",
		Buttons: []*Button{{CompID: "resume", Text: "Resume"}, {CompID: "restart", Text: "Restart"}}}
	dialog.SetBounds(image.Rect(0, 0, 800, 600))
	children := dialog.ChildComponents()

	var message *Label
	var buttons []*Button
	for _, child := range children {
		if l, ok := child.(*Label); ok && l.CompID == "pause_msg" {
			message = l
		}
		if b, ok := child.(*Button); ok {
			buttons = append(buttons, b)
		}
	}
	if message == nil {
		t.Fatal("expected a single pause_msg label")
	}
	if message.Bounds().Dy() < 2*24 {
		t.Fatalf("message rect too short for a multi-line message: %v", message.Bounds())
	}
	for _, b := range buttons {
		if message.Bounds().Overlaps(b.Bounds()) {
			t.Fatalf("message %v overlaps button %q %v", message.Bounds(), b.CompID, b.Bounds())
		}
	}
}

// TestDialogDefaultsToVisibleColorsWhenNoneSet guards against the
// regression buildModal was fixed for: BackdropColor/CardColor/TitleColor
// default to Go zero-value (fully transparent) and are only populated from
// the active theme when UseTheme is true -- a bare &Dialog{...} literal
// with UseTheme left false (this app's own convention when it wants manual
// colors, e.g. under LegacyComponentStyles) rendered an invisible card and
// title.
func TestDialogDefaultsToVisibleColorsWhenNoneSet(t *testing.T) {
	dialog := &Dialog{CompID: "bare", Title: "Bare Dialog", Message: "No colors were set on this dialog."}
	dialog.SetBounds(image.Rect(0, 0, 800, 600))
	if dialog.BackdropColor.A == 0 || dialog.CardColor.A == 0 || dialog.TitleColor.A == 0 {
		t.Fatalf("expected non-transparent default colors, got %+v", dialog)
	}
	for _, child := range dialog.ChildComponents() {
		if p, ok := child.(*Panel); ok && p.CompID == "bare_card" && p.BGColor.A == 0 {
			t.Fatal("card panel must render with a non-transparent background")
		}
		if l, ok := child.(*Label); ok && l.CompID == "bare_title" && l.Color.A == 0 {
			t.Fatal("title label must render with a non-transparent color")
		}
	}
}

// TestDialogPreservesExplicitlySetColors guards against the default-color
// fallback overwriting a caller's own choice (or an already-applied theme):
// only fields still at Go zero-value get a default.
func TestDialogPreservesExplicitlySetColors(t *testing.T) {
	custom := color.RGBA{10, 20, 30, 255}
	dialog := &Dialog{CompID: "custom", Title: "Custom", BackdropColor: custom, CardColor: custom, TitleColor: custom}
	dialog.SetBounds(image.Rect(0, 0, 800, 600))
	if dialog.BackdropColor != custom || dialog.CardColor != custom || dialog.TitleColor != custom {
		t.Fatalf("explicit colors must not be overwritten, got %+v", dialog)
	}
}

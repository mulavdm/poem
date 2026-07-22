package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/layout"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestButtonMeasureReflectsLabelAndPadding(t *testing.T) {
	button := &Button{Text: "GENERATE IMAGE"}

	result := button.Measure(image.Pt(400, 80), &types.ApplicationState{FontCharWidth: 8})

	if result.Preferred.X < 140 {
		t.Fatalf("expected measured width to include padding, got %d", result.Preferred.X)
	}
	if result.Preferred.Y != 36 {
		t.Fatalf("expected themed button height 36, got %d", result.Preferred.Y)
	}
}

func TestLabelMeasureDeterministicForLongText(t *testing.T) {
	label := &Label{Text: "this is a long deterministic label"}

	first := label.Measure(image.Point{}, &types.ApplicationState{FontCharWidth: 8})
	second := label.Measure(image.Point{}, &types.ApplicationState{FontCharWidth: 8})

	if first != second {
		t.Fatalf("expected deterministic measurement, got %v and %v", first, second)
	}
}

func TestParagraphMeasureHeightGrowsWithWrapping(t *testing.T) {
	paragraph := &Paragraph{
		Text:       "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu",
		CharWidth:  8,
		LineHeight: 20,
	}

	narrow := paragraph.Measure(image.Pt(120, 0), nil)
	wide := paragraph.Measure(image.Pt(320, 0), nil)

	if narrow.Preferred.Y <= wide.Preferred.Y {
		t.Fatalf("expected narrow paragraph to be taller, got narrow=%v wide=%v", narrow, wide)
	}
}

func TestTextAreaMeasureHeightGrowsWithWrapping(t *testing.T) {
	area := &TextArea{
		Value:      "one two three four five six seven eight nine ten eleven twelve",
		CharWidth:  8,
		LineHeight: 20,
	}

	narrow := area.Measure(image.Pt(160, 0), nil)
	wide := area.Measure(image.Pt(320, 0), nil)

	if narrow.Preferred.Y <= wide.Preferred.Y {
		t.Fatalf("expected narrow text area to be taller, got narrow=%v wide=%v", narrow, wide)
	}
}

func TestImageViewMeasurePreservesAspectIntent(t *testing.T) {
	view := &ImageView{
		ImageWidth:  1024,
		ImageHeight: 512,
	}

	result := view.Measure(image.Pt(300, 300), nil)

	if result.Preferred.X != 300 || result.Preferred.Y != 150 {
		t.Fatalf("expected contain-style measurement 300x150, got %v", result.Preferred)
	}
}

func TestScrollViewSetBoundsUsesChildMeasurement(t *testing.T) {
	scroll := &ScrollView{
		Rect: image.Rect(0, 0, 160, 120),
		Children: []types.Component{
			&Paragraph{
				Text:       "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau upsilon phi chi psi omega alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu",
				CharWidth:  8,
				LineHeight: 20,
			},
		},
	}

	scroll.SetBounds(scroll.Rect)

	if scroll.ContentH <= scroll.Rect.Dy() {
		t.Fatalf("expected measured scroll content to exceed viewport, got content=%d viewport=%d", scroll.ContentH, scroll.Rect.Dy())
	}
	if scroll.Children[0].Bounds().Dx() <= 0 {
		t.Fatalf("expected child width to be assigned from viewport")
	}
}

func TestScrollViewPreservesMeasuredContentWhenChildHasFixedBounds(t *testing.T) {
	children := []types.Component{
		&Button{Text: "one"},
		&Button{Text: "two"},
		&Button{Text: "three"},
		&Button{Text: "four"},
		&Button{Text: "five"},
	}
	stack := &layout.FlexBox{
		Rect:           image.Rect(0, 0, 180, 80),
		Direction:      layout.Vertical,
		Gap:            10,
		Padding:        4,
		JustifyContent: layout.JustifyStart,
		AlignItems:     layout.AlignStretch,
		Children:       children,
	}
	scroll := &ScrollView{
		Rect:     image.Rect(0, 0, 180, 80),
		Children: []types.Component{stack},
	}

	scroll.SetBounds(scroll.Rect)

	if scroll.ContentH <= scroll.Rect.Dy() {
		t.Fatalf("expected fixed-height child content to remain scrollable, got content=%d viewport=%d", scroll.ContentH, scroll.Rect.Dy())
	}
	if stack.Bounds().Dy() != scroll.ContentH {
		t.Fatalf("expected scroll child to receive full content height, got child=%d content=%d", stack.Bounds().Dy(), scroll.ContentH)
	}
}

func TestSliderMeasureProvidesNonZeroControlHeight(t *testing.T) {
	slider := NewSlider("quality", 0, 100, 50, nil)
	result := slider.Measure(image.Pt(420, 200), nil)
	if result.Preferred.X != 420 || result.Preferred.Y <= 0 || result.Min.Y <= 0 {
		t.Fatalf("slider measurement=%+v", result)
	}
}

// A button's reported Min must hold its own label. The regression this guards:
// Min was clamped to 90px while Preferred held the full label, so a flex row
// under pressure shrank the button and fitButtonLabel truncated the text —
// "Calculate route" rendered as "Calculate …" with free space beside it
// (design guide: TOKENS scale independence, lint UI023 / clipped scaled text).
func TestButtonMinWidthHoldsItsLabel(t *testing.T) {
	const charW = 8
	button := &Button{Text: "Calculate route"}
	result := button.Measure(image.Pt(1200, 80), &types.ApplicationState{FontCharWidth: charW})

	labelWidth := len([]rune("Calculate route")) * charW
	if result.Min.X < labelWidth {
		t.Fatalf("Min width %d cannot hold a %d px label; the label would truncate under pressure", result.Min.X, labelWidth)
	}
	// And the label fits without an ellipsis at that minimum width.
	if fitted := fitButtonLabel("Calculate route", result.Min.X, charW); fitted != "Calculate route" {
		t.Fatalf("label truncated at its own minimum width: %q", fitted)
	}
}

// An author-set FixedWidth pins the width, so fixed toolbar slots stay fixed.
func TestFixedWidthButtonKeepsItsWidth(t *testing.T) {
	button := &Button{Text: "A very long label that would otherwise expand", FixedWidth: 120}
	result := button.Measure(image.Pt(1200, 80), &types.ApplicationState{FontCharWidth: 8})
	if result.Preferred.X != 120 || result.Min.X != 120 {
		t.Fatalf("fixed-width button reported %dx (min %d), want 120", result.Preferred.X, result.Min.X)
	}
}

// The regression that shipped as truncated labels after a resize: a button
// remeasured after layout wrote its bounds must size to its content again, not
// freeze at the width it was first laid out at. Rect is layout output, not a
// size request.
func TestButtonRemeasuresToContentAfterLayout(t *testing.T) {
	const charW = 8
	button := &Button{Text: "From · Choose a starting point"}
	state := &types.ApplicationState{FontCharWidth: charW}

	// Simulate a first layout at a cramped width, as happens in a narrow window.
	button.SetBounds(image.Rect(0, 0, 120, 36))

	// Remeasured with room available, it must ask for its full label width.
	result := button.Measure(image.Pt(1200, 80), state)
	labelWidth := len([]rune("From · Choose a starting point")) * charW
	if result.Preferred.X < labelWidth {
		t.Fatalf("button froze at its laid-out width: Preferred %d < label %d", result.Preferred.X, labelWidth)
	}
}

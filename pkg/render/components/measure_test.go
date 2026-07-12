package components

import (
	"image"
	"testing"

	"go_native_gpu_gui/pkg/render/layout"
	"go_native_gpu_gui/pkg/render/types"
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

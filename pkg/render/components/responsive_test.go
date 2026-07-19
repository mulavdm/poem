package components

import (
	"image"
	"testing"
)

func TestResponsiveSelectsBranchFromAvailableWidth(t *testing.T) {
	compact := NewLabel("compact", "compact")
	wide := NewLabel("wide", "a much wider label")
	responsive := &Responsive{CompID: "responsive", Breakpoint: 600, Compact: compact, Wide: wide}

	compactMeasure := responsive.Measure(image.Pt(500, 100), nil)
	wideMeasure := responsive.Measure(image.Pt(900, 100), nil)
	if compactMeasure.Preferred.X >= wideMeasure.Preferred.X {
		t.Fatalf("compact=%v wide=%v", compactMeasure.Preferred, wideMeasure.Preferred)
	}

	responsive.SetBounds(image.Rect(0, 0, 500, 100))
	if children := responsive.ChildComponents(); len(children) != 1 || children[0].ID() != "compact" {
		t.Fatalf("compact children = %#v", children)
	}
	responsive.SetBounds(image.Rect(0, 0, 900, 100))
	if children := responsive.ChildComponents(); len(children) != 1 || children[0].ID() != "wide" {
		t.Fatalf("wide children = %#v", children)
	}
}

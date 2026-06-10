package components

import (
	"image"
	"testing"
)

func TestContainRectPreservesAspect(t *testing.T) {
	rect := containRect(image.Rect(0, 0, 400, 200), 100, 100)
	if rect.Dx() != 200 || rect.Dy() != 200 {
		t.Fatalf("unexpected contain rect size: %v", rect)
	}
	if rect.Min.X != 100 || rect.Min.Y != 0 {
		t.Fatalf("unexpected contain rect placement: %v", rect)
	}
}

func TestImageViewHitTestUsesBounds(t *testing.T) {
	view := &ImageView{
		CompID: "preview",
		Rect:   image.Rect(10, 10, 110, 110),
	}
	if view.HitTest(image.Pt(50, 50)) != "preview" {
		t.Fatalf("expected hit inside image view")
	}
	if view.HitTest(image.Pt(200, 200)) != "" {
		t.Fatalf("expected miss outside image view")
	}
}

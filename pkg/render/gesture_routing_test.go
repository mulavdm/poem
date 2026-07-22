package render

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/components"
	"github.com/mulavdm/poem/pkg/render/layout"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestOverlayHitTargetPreventsCoveredCanvasFromOwningPan(t *testing.T) {
	canvas := &components.ImageViewport{CompID: "workspace/map", Rect: image.Rect(0, 0, 400, 800)}
	action := components.NewButton("workspace/tools/action", "Action", nil)
	action.SetBounds(image.Rect(20, 620, 220, 660))
	tools := components.NewScrollView("workspace/tools-scroll", action)
	tools.SetBounds(image.Rect(0, 580, 400, 800))
	root := &layout.Overlay{
		CompID: "workspace",
		Rect:   image.Rect(0, 0, 400, 800),
		Base:   canvas,
		Layers: []layout.OverlayLayer{{Content: tools}},
	}

	point := image.Pt(100, 640)
	hitID := root.HitTest(point)
	if hitID != action.ID() {
		t.Fatalf("topmost hit = %q, want tools action %q", hitID, action.ID())
	}
	if canvas.ID() == hitID {
		t.Fatal("covered canvas must not own the overlay hit")
	}
	if targets := pannableTargetsAt(root, point); len(targets) != 0 {
		t.Fatalf("covered canvas received %d pannable targets", len(targets))
	}
	if targets := pannableTargetsAt(root, image.Pt(100, 300)); len(targets) != 1 {
		t.Fatalf("uncovered canvas received %d pannable targets, want 1", len(targets))
	}
	if _, ok := any(canvas).(types.PannableComponent); !ok {
		t.Fatal("fixture canvas must be pannable")
	}
}

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
	if route := inputRouteAt(root, point, &types.ApplicationState{}); len(route) == 0 || route[len(route)-1].component.ID() != action.ID() {
		t.Fatalf("covered point route = %+v, want action", route)
	}
	if route := inputRouteAt(root, image.Pt(100, 300), &types.ApplicationState{}); len(route) == 0 || route[len(route)-1].component.ID() != canvas.ID() {
		t.Fatalf("uncovered point route = %+v, want canvas", route)
	}
	if _, ok := any(canvas).(types.PannableComponent); !ok {
		t.Fatal("fixture canvas must be pannable")
	}
}

func TestInputRouteMapsScrolledViewportCoordinatesToCanvas(t *testing.T) {
	canvas := &components.ImageViewport{CompID: "map", Rect: image.Rect(0, 500, 400, 700)}
	scroll := components.NewScrollView("scroll", canvas)
	scroll.Rect = image.Rect(0, 100, 400, 500)
	scroll.ContentH = 800
	scroll.ScrollY, scroll.CurrentScrollY = 400, 400
	state := &types.ApplicationState{ScrollCurrent: map[string]float64{"scroll": 400}}

	route := inputRouteAt(scroll, image.Pt(200, 200), state)
	if len(route) == 0 {
		t.Fatal("scrolled canvas was not routed")
	}
	got := route[len(route)-1]
	if got.component.ID() != "map" {
		t.Fatalf("deepest component = %q, want map", got.component.ID())
	}
	if got.point != image.Pt(200, 600) {
		t.Fatalf("content point = %v, want (200,600)", got.point)
	}
}

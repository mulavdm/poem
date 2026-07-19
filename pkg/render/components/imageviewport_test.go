package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestImageViewportDragAndPinchCommitControlledTransform(t *testing.T) {
	state := &types.ApplicationState{}
	var committed ImageTransform
	commits := 0
	view := &ImageViewport{CompID: "map", Rect: image.Rect(0, 0, 200, 100), ImageWidth: 200, ImageHeight: 100, Pixels: make([]byte, 200*100*4), MinScale: 0.5, MaxScale: 8, OnChange: func(transform ImageTransform, _ *types.ApplicationState) { committed = transform; commits++ }}
	if !view.OnMouseDown(image.Pt(100, 50), state) || !view.OnMouseMove(image.Pt(120, 60), state) || !view.OnMouseUp(image.Pt(120, 60), state) {
		t.Fatal("drag was not consumed")
	}
	if commits != 1 || committed.OffsetX != 0.1 || committed.OffsetY != 0.1 || committed.Scale != 1 {
		t.Fatalf("drag commit = %+v (%d commits)", committed, commits)
	}
	view.OnPinchGesture(image.Pt(100, 50), image.Point{}, 1, types.GestureBegin, state)
	view.OnPinchGesture(image.Pt(100, 50), image.Point{}, 2, types.GestureUpdate, state)
	if commits != 1 {
		t.Fatalf("pinch committed before end: %d", commits)
	}
	view.OnPinchGesture(image.Pt(100, 50), image.Point{}, 1, types.GestureEnd, state)
	if commits != 2 || committed.Scale != 2 {
		t.Fatalf("pinch commit = %+v (%d commits)", committed, commits)
	}
}

func TestImageViewportTapMarkerAndDragAreExclusive(t *testing.T) {
	state := &types.ApplicationState{}
	points, markerID, commits := 0, "", 0
	view := &ImageViewport{CompID: "map", Rect: image.Rect(0, 0, 200, 100), ImageWidth: 200, ImageHeight: 100,
		Pixels: make([]byte, 200*100*4), Markers: []ImageMarker{{ID: "one", Label: "One", X: 0.25, Y: 0.5, Selected: true}},
		OnActivate: func(x, y float64, _ *types.ApplicationState) {
			points++
			if x != 0.75 || y != 0.5 {
				t.Fatalf("point=%v,%v", x, y)
			}
		}, OnMarker: func(id string, _ *types.ApplicationState) { markerID = id },
		OnChange: func(ImageTransform, *types.ApplicationState) { commits++ }}
	view.OnMouseDown(image.Pt(50, 50), state)
	view.OnMouseUp(image.Pt(50, 50), state)
	if markerID != "one" || points != 0 || commits != 0 {
		t.Fatalf("marker=%q points=%d commits=%d", markerID, points, commits)
	}
	view.OnMouseDown(image.Pt(150, 50), state)
	view.OnMouseUp(image.Pt(150, 50), state)
	if points != 1 || commits != 0 {
		t.Fatalf("background points=%d commits=%d", points, commits)
	}
	view.OnMouseDown(image.Pt(150, 50), state)
	view.OnMouseMove(image.Pt(170, 50), state)
	view.OnMouseUp(image.Pt(170, 50), state)
	if points != 1 || commits != 1 {
		t.Fatalf("drag points=%d commits=%d", points, commits)
	}
	node := view.Semantics(state)
	if len(node.Children) != 1 || node.Children[0].Name != "One" || !node.Children[0].State.Selected {
		t.Fatalf("marker semantics=%+v", node.Children)
	}
	if !view.PerformSemanticAction("map/marker/one", semantics.ActionInvoke, "", state) || markerID != "one" {
		t.Fatal("semantic marker invocation failed")
	}
}

func TestImageViewportWheelClampsScale(t *testing.T) {
	state := &types.ApplicationState{}
	view := &ImageViewport{CompID: "map", Rect: image.Rect(0, 0, 100, 100), MinScale: 0.5, MaxScale: 2}
	for range 20 {
		view.OnMouseWheel(image.Pt(50, 50), 120, state)
	}
	if view.Transform.Scale != 2 {
		t.Fatalf("scale = %v, want 2", view.Transform.Scale)
	}
}

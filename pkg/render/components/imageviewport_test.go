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
	if commits != 2 || committed.OffsetX != 0.1 || committed.OffsetY != 0.1 || committed.Scale != 1 {
		t.Fatalf("drag commit = %+v (%d commits)", committed, commits)
	}
	view.OnPinchGesture(image.Pt(100, 50), image.Point{}, 1, types.GestureBegin, state)
	view.OnPinchGesture(image.Pt(100, 50), image.Point{}, 2, types.GestureUpdate, state)
	if commits != 3 {
		t.Fatalf("pinch did not publish its live update: %d", commits)
	}
	view.OnPinchGesture(image.Pt(100, 50), image.Point{}, 1, types.GestureEnd, state)
	if commits != 4 || committed.Scale != 2 {
		t.Fatalf("pinch commit = %+v (%d commits)", committed, commits)
	}
}

func TestImageViewportDragSurvivesComponentRebuild(t *testing.T) {
	state := &types.ApplicationState{MouseButton: 1}
	transform := ImageTransform{Scale: 1}
	onChange := func(next ImageTransform, _ *types.ApplicationState) { transform = next }
	first := &ImageViewport{CompID: "map", Rect: image.Rect(0, 0, 200, 100), Transform: transform, OnChange: onChange}
	first.OnMouseDown(image.Pt(100, 50), state)
	first.OnMouseMove(image.Pt(120, 60), state)

	// BuildPages replaces component instances between native event batches.
	// The second instance must resume the captured drag from transient state.
	second := &ImageViewport{CompID: "map", Rect: image.Rect(0, 0, 200, 100), Transform: transform, OnChange: onChange}
	if !second.OnMouseMove(image.Pt(140, 70), state) || !second.OnMouseUp(image.Pt(140, 70), state) {
		t.Fatal("rebuilt viewport lost pointer capture")
	}
	if transform.OffsetX != 0.2 || transform.OffsetY != 0.2 {
		t.Fatalf("rebuilt drag transform = %+v", transform)
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
	if points != 1 || commits != 2 {
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

func TestMapViewportSecondaryDragAndTwoFingerPose(t *testing.T) {
	state := &types.ApplicationState{MouseButton: 2}
	var committed ImageTransform
	view := &ImageViewport{
		CompID: "map", Rect: image.Rect(0, 0, 200, 100), MapInteraction: true,
		Transform: ImageTransform{Scale: 1, Bearing: 350, Pitch: 10},
		MinScale:  0.5, MaxScale: 8, MinPitch: 0, MaxPitch: 70,
		OnChange: func(transform ImageTransform, _ *types.ApplicationState) { committed = transform },
	}
	view.OnMouseDown(image.Pt(100, 50), state)
	view.OnMouseMove(image.Pt(140, 70), state)
	view.OnMouseUp(image.Pt(140, 70), state)
	if committed.Bearing != 4 || committed.Pitch != 15 {
		t.Fatalf("secondary drag pose = bearing %.1f pitch %.1f", committed.Bearing, committed.Pitch)
	}

	view.OnPinchGesture(image.Pt(100, 50), image.Point{}, 1, types.GestureBegin, state)
	view.OnPinchGesture(image.Pt(110, 70), image.Pt(10, 20), 1.25, types.GestureUpdate, state)
	view.OnPinchGesture(image.Pt(110, 70), image.Point{}, 1, types.GestureEnd, state)
	if committed.Scale != 1.25 || committed.Bearing != 7.5 || committed.Pitch != 20 {
		t.Fatalf("two-finger pose = %+v", committed)
	}
}

func TestImageViewportMapFillsAvailableAndTracksResize(t *testing.T) {
	// A map viewport must re-fill the available space every measure rather than
	// freezing at its first laid-out Rect — otherwise it stops tracking window
	// resizes and carries edge-anchored controls off-screen.
	view := &ImageViewport{CompID: "map", MapViewportID: "map", ImageWidth: 960, ImageHeight: 640, MinScale: 0.5, MaxScale: 8}
	first := view.Measure(image.Pt(800, 600), nil).Preferred
	if first != image.Pt(800, 600) {
		t.Fatalf("map should fill available space, got %v", first)
	}
	// Simulate a layout pass pinning a Rect, then a larger window.
	view.SetBounds(image.Rect(0, 0, 800, 600))
	grown := view.Measure(image.Pt(1400, 900), nil).Preferred
	if grown != image.Pt(1400, 900) {
		t.Fatalf("map must track a larger viewport, got %v (froze at laid-out Rect?)", grown)
	}
}

func TestImageViewportImageContainsWithinMaxCaps(t *testing.T) {
	// A still image keeps its aspect ratio and honours author Max caps, without
	// reading the laid-out Rect.
	view := &ImageViewport{CompID: "img", ImageWidth: 200, ImageHeight: 100, MaxWidth: 300, MaxHeight: 300}
	got := view.Measure(image.Pt(1000, 1000), nil).Preferred
	// Capped to 300 wide, contained to 2:1 aspect -> 300x150.
	if got != image.Pt(300, 150) {
		t.Fatalf("image should cap and keep aspect, got %v", got)
	}
}

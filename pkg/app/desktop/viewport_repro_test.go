package desktop

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/app"
	"github.com/mulavdm/poem/pkg/render"
)

// Exercises the desktop BuildPagesFn with a responsive (fill) map inside an
// overlay across several window sizes — build, layout, and native reconcile —
// asserting the path stays panic-free and produces a non-empty page.
func TestBuildPagesWithMapOverlayDoesNotPanic(t *testing.T) {
	type S struct{ n int }
	view := func(s S) app.Node {
		return app.WorkspaceNode{
			Semantic: app.Semantic{ID: "ws", Name: "ws", Enabled: true},
			Title:    "MAPPS",
			Content: []app.Node{
				app.OverlayFloat(
					app.MapViewportNode{
						Semantic:       app.Semantic{ID: "route-map", Name: "map", Enabled: true},
						Source:         app.MapSource{ID: "s", ProviderID: "p", MinZoom: 5, MaxZoom: 14, Bounds: app.MapBounds{West: 4, South: 52, East: 5, North: 53}},
						Camera:         app.MapCamera{Latitude: 52.3, Longitude: 4.9, Zoom: 11},
						Style:          app.MapStyleSet{House: app.MapStyle{ID: "st", LabelDensity: 1, TerrainScale: 1}},
						MinZoom:        5,
						MaxZoom:        18,
						OnCameraChange: app.Msg{Name: "map.camera"},
					},
					app.OverlayLayer{Anchor: app.OverlayTopRight, Inset: 12, Content: app.Button("Zoom in", app.Msg{Name: "zoom-in"})},
				),
			},
		}
	}
	a := app.App[S]{
		Init: S{},
		View: view,
		Update: func(s S, m app.Msg) (S, app.Cmd) {
			s.n++
			return s, app.Cmd{}
		},
	}
	config := Configure(a, render.AppConfig{Title: "t"})
	for _, wh := range [][2]int{{1125, 875}, {900, 700}, {1400, 900}} {
		rstate := &render.ApplicationState{WindowWidth: wh[0], WindowHeight: wh[1]}
		config.BuildPagesFn(rstate)
		page := rstate.Pages["trellis-root"]
		if len(page) == 0 {
			t.Fatalf("no page produced at %dx%d", wh[0], wh[1])
		}
		// Find the laid-out map viewport and assert it has a real, filled rect —
		// the native draw log showed it collapsing to a zero-size command.
		var mapRect image.Rectangle
		var found bool
		page[0].Walk(func(c render.Component) {
			if iv, ok := c.(*render.ImageViewport); ok && iv.MapViewportID == "route-map" {
				mapRect = iv.Bounds()
				found = true
			}
		})
		if !found {
			t.Fatalf("map viewport not in tree at %dx%d", wh[0], wh[1])
		}
		if mapRect.Dx() <= 100 || mapRect.Dy() <= 100 {
			t.Fatalf("map collapsed at %dx%d: rect=%v", wh[0], wh[1], mapRect)
		}
		t.Logf("%dx%d -> map rect %v", wh[0], wh[1], mapRect)
	}
}

func TestMapViewportPublishesZoomPanBearingAndPitch(t *testing.T) {
	node := app.MapViewportNode{
		Semantic: app.Semantic{ID: "map", Name: "Map", Enabled: true},
		Source:   app.MapSource{ID: "source", ProviderID: "tiles", MinZoom: 0, MaxZoom: 18},
		Camera:   app.MapCamera{Latitude: 52, Longitude: 5, Zoom: 10, Bearing: 20, Pitch: 25},
		MinZoom:  0, MaxZoom: 18, MinPitch: 0, MaxPitch: 70,
		OnCameraChange: app.Msg{Name: "map.camera"},
	}
	var dispatched app.Msg
	component := build(node, "map", func(msg app.Msg) { dispatched = msg })
	viewport, ok := component.(*render.ImageViewport)
	if !ok {
		t.Fatalf("map built as %T", component)
	}
	viewport.OnChange(render.ImageTransform{
		Scale: 2, OffsetX: 0.1, OffsetY: -0.2, Bearing: 55, Pitch: 45,
	}, &render.ApplicationState{})
	camera, ok := dispatched.MapCamera()
	if !ok {
		t.Fatalf("invalid camera payload %q", dispatched.Payload)
	}
	if camera.Zoom != 11 || camera.Bearing != 55 || camera.Pitch != 45 {
		t.Fatalf("camera pose = %+v", camera)
	}
	if camera.Longitude >= node.Camera.Longitude || camera.Latitude >= node.Camera.Latitude {
		t.Fatalf("pan did not update map center: %+v", camera)
	}
}

package cartography

import "testing"

// A camera whose viewport centre sits on the tile in play. Tile z14 x8411 y5479
// covers Amsterdam; Unproject of its centre gives a matching camera.
func declutterCamera() Camera {
	longitude, latitude := Unproject(
		(float64(8411)+0.5)/float64(int(1)<<14),
		(float64(5479)+0.5)/float64(int(1)<<14),
	)
	return Camera{Latitude: latitude, Longitude: longitude, Zoom: 14, Width: 800, Height: 600, Pitch: 60}
}

func TestDeclutterAdmitsNearFadesMidCullsFar(t *testing.T) {
	camera := declutterCamera()
	declutter := newPointDeclutter(camera)

	// At the camera: full strength.
	scale, ok := declutter.admit(float32(declutter.centerX), float32(declutter.centerY))
	if !ok || scale != 1 {
		t.Fatalf("centre dot: scale=%v ok=%v", scale, ok)
	}

	// Inside the fade band: admitted but weakened.
	midOffset := (declutter.fadeStartMeters + declutter.fadeEndMeters) / 2 / declutter.metersPerUnit
	scale, ok = declutter.admit(float32(declutter.centerX+midOffset), float32(declutter.centerY))
	if !ok || scale <= 0 || scale >= 1 {
		t.Fatalf("mid-band dot: scale=%v ok=%v, want 0<scale<1", scale, ok)
	}

	// Past the fade end: culled.
	farOffset := declutter.fadeEndMeters * 1.1 / declutter.metersPerUnit
	if _, ok = declutter.admit(float32(declutter.centerX+farOffset), float32(declutter.centerY)); ok {
		t.Fatal("far dot admitted; want culled")
	}
}

func TestDeclutterThinsPerCellAndCulledDotsClaimNothing(t *testing.T) {
	camera := declutterCamera()
	declutter := newPointDeclutter(camera)

	x, y := float32(declutter.centerX), float32(declutter.centerY)
	if _, ok := declutter.admit(x, y); !ok {
		t.Fatal("first dot rejected")
	}
	// A second dot in the same cell is thinned.
	if _, ok := declutter.admit(x, y); ok {
		t.Fatal("duplicate cell dot admitted")
	}
	// The next cell over is free.
	if _, ok := declutter.admit(x+float32(declutter.cellSize)*1.5, y); !ok {
		t.Fatal("neighbouring cell dot rejected")
	}

	// A culled far dot must not claim its cell (a nearer arrival stays welcome —
	// relevant when overzoomed parents deliver far geometry first).
	fresh := newPointDeclutter(camera)
	farX := float32(fresh.centerX + fresh.fadeEndMeters*2/fresh.metersPerUnit)
	if _, ok := fresh.admit(farX, y); ok {
		t.Fatal("far dot admitted")
	}
	if len(fresh.cells) != 0 {
		t.Fatal("culled dot claimed a cell")
	}
}

func TestBuildSceneDeclutersDotLayer(t *testing.T) {
	camera := declutterCamera()
	style := Style{Layers: []StyleLayer{{ID: "poi", SourceLayer: "poi", Geometry: GeometryPoint, MinZoom: 0, MaxZoom: 24, Color: Color{R: 25, G: 165, B: 120, A: 255}, Width: []ZoomStop{{Zoom: 0, Value: 3}}}}}

	// 100 POIs crammed into one corner of the tile: same thinning cell.
	features := make([]Feature, 0, 100)
	for index := 0; index < 100; index++ {
		features = append(features, Feature{ID: uint64(index + 1), Kind: GeometryPoint, Paths: [][]Point{{{int32(index % 10), int32(index / 10)}}}})
	}
	tiles := []Tile{{ID: TileID{Z: 14, X: 8411, Y: 5479}, Layers: []Layer{{Name: "poi", Extent: 4096, Features: features}}}}
	scene, err := BuildScene("map", 1, camera, style, tiles)
	if err != nil {
		t.Fatal(err)
	}
	drawn := uint32(0)
	for _, draw := range scene.Delta.Draws {
		drawn += draw.Count
	}
	if drawn >= 100 {
		t.Fatalf("dot layer not thinned: %d dots drawn", drawn)
	}
	if drawn == 0 {
		t.Fatal("thinning removed every dot; the first should survive")
	}
}

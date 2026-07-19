package cartography

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/mulavdm/poem/pkg/render/protocol"
	"golang.org/x/image/font/gofont/goregular"
)

func TestProjectionRoundTrip(t *testing.T) {
	for _, input := range [][2]float64{{4.9041, 52.3676}, {-122.4194, 37.7749}, {179.9, -80}} {
		x, y := Project(input[0], input[1])
		longitude, latitude := Unproject(x, y)
		if math.Abs(longitude-input[0]) > 1e-9 || math.Abs(latitude-input[1]) > 1e-9 {
			t.Fatalf("round trip = %v,%v", longitude, latitude)
		}
	}
}

func TestPerspectiveProjectionTiltsGroundAndRaisesElevation(t *testing.T) {
	camera := Camera{Latitude: 0, Longitude: 0, Zoom: 4, Bearing: 0, Pitch: 60, Width: 800, Height: 600}
	centerX, centerY := Project(0, 0)
	center, err := ProjectWorldPoint(camera, centerX, centerY, 0)
	if err != nil || !center.Visible || math.Abs(center.X-400) > 1e-9 || math.Abs(center.Y-300) > 1e-9 {
		t.Fatalf("center=%+v err=%v", center, err)
	}
	ahead, _ := ProjectWorldPoint(camera, centerX, centerY-.01, 0)
	behind, _ := ProjectWorldPoint(camera, centerX, centerY+.01, 0)
	raised, _ := ProjectWorldPoint(camera, centerX, centerY, 100)
	if !ahead.Visible || !behind.Visible || ahead.Y >= center.Y || behind.Y <= center.Y || math.Abs(ahead.Y-center.Y) >= math.Abs(behind.Y-center.Y) || raised.Y >= center.Y {
		t.Fatalf("ahead=%+v center=%+v behind=%+v raised=%+v", ahead, center, behind, raised)
	}
}

func TestCoverIsBoundedAndDeterministic(t *testing.T) {
	camera := Camera{Latitude: 52.37, Longitude: 4.9, Zoom: 12.5, Bearing: 37, Pitch: 45, Width: 1200, Height: 800}
	a, err := Cover(camera, 64)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Cover(camera, 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) == 0 || len(a) > 64 || !equalTiles(a, b) {
		t.Fatalf("unexpected cover %#v", a)
	}
	if _, err := Cover(camera, 1); err == nil {
		t.Fatal("expected tile-limit error")
	}
}

func TestCoverAtZoomOverzoomsNativeTiles(t *testing.T) {
	camera := Camera{Latitude: 52.3789, Longitude: 4.9006, Zoom: 18, Width: 1000, Height: 700}
	cover, err := CoverAtZoom(camera, 14, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(cover) == 0 || len(cover) > 4 {
		t.Fatalf("cover=%+v", cover)
	}
	for _, tile := range cover {
		if tile.Z != 14 {
			t.Fatalf("tile=%+v", tile)
		}
	}
	if _, err := CoverAtZoom(camera, 23, 8); err == nil {
		t.Fatal("accepted unsupported tile zoom")
	}
}

func TestBuildSceneIsDeterministicAndProtocolSafe(t *testing.T) {
	style := Style{Layers: []StyleLayer{{ID: "road", SourceLayer: "transportation", Geometry: GeometryLine, MinZoom: 0, MaxZoom: 24, Color: Color{30, 100, 220, 255}, Width: []ZoomStop{{0, 1}, {20, 8}}, Interactive: true}}}
	tiles := []Tile{{ID: TileID{Z: 1, X: 1, Y: 0}, Layers: []Layer{{Name: "transportation", Extent: 4096, Features: []Feature{{ID: 7, Kind: GeometryLine, Paths: [][]Point{{{0, 0}, {4096, 4096}}}}}}}}}
	camera := Camera{Latitude: 0, Longitude: 0, Zoom: 1, Width: 800, Height: 600}
	a, err := BuildScene("map", 1, camera, style, tiles)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildScene("map", 1, camera, style, tiles)
	if err != nil {
		t.Fatal(err)
	}
	ea, err := protocol.EncodeMapSceneDelta(a.Delta)
	if err != nil {
		t.Fatal(err)
	}
	eb, err := protocol.EncodeMapSceneDelta(b.Delta)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ea, eb) || len(a.Picks) != 1 || a.Picks[0].FeatureID != 7 || len(a.Delta.Draws) != 1 || a.Delta.Draws[0].Primitive != protocol.MapPrimitiveTriangles || a.Delta.Draws[0].Count != 6 {
		t.Fatal("scene is not stable")
	}
}

func TestLineMeshPreservesRequestedScreenWidthAtSettledZoom(t *testing.T) {
	var vertices, indices []byte
	var vertexCount, indexCount uint32
	appendLineSegment(&vertices, &indices, &vertexCount, &indexCount, 0.25, 0.5, 0.75, 0.5, Color{255, 0, 0, 255}, 8, 1, 4)
	if vertexCount != 4 || indexCount != 6 {
		t.Fatalf("vertices=%d indices=%d", vertexCount, indexCount)
	}
	y0 := math.Float32frombits(binary.LittleEndian.Uint32(vertices[4:8]))
	y1 := math.Float32frombits(binary.LittleEndian.Uint32(vertices[vertexStride+4 : vertexStride+8]))
	widthPixels := math.Abs(float64(y0-y1)) * 512 * math.Exp2(4)
	if math.Abs(widthPixels-8) > 0.001 {
		t.Fatalf("width=%f", widthPixels)
	}
}

func TestBuildSceneSkipsMalformedPolygonWithoutBlankingLayer(t *testing.T) {
	style := Style{Layers: []StyleLayer{{ID: "land", SourceLayer: "landuse", Geometry: GeometryPolygon, MinZoom: 0, MaxZoom: 24, Color: Color{40, 90, 60, 255}}}}
	malformed := Feature{ID: 1, Kind: GeometryPolygon, Paths: [][]Point{{{0, 0}, {2000, 2000}, {0, 2000}, {2000, 0}, {0, 0}}}}
	valid := Feature{ID: 2, Kind: GeometryPolygon, Paths: [][]Point{{{0, 0}, {4096, 0}, {4096, 4096}, {0, 4096}, {0, 0}}}}
	tiles := []Tile{{ID: TileID{Z: 1}, Layers: []Layer{{Name: "landuse", Extent: 4096, Features: []Feature{malformed, valid}}}}}
	scene, err := BuildScene("map", 1, Camera{Zoom: 1, Width: 800, Height: 600}, style, tiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(scene.Delta.Draws) != 1 || scene.Delta.Draws[0].Count != 6 {
		t.Fatalf("draws=%+v", scene.Delta.Draws)
	}
}

func TestBuildSceneExtrudesBuildingHeightAndWalls(t *testing.T) {
	style := Style{Layers: []StyleLayer{{ID: "building", SourceLayer: "building", Geometry: GeometryPolygon, MinZoom: 0, MaxZoom: 24, Color: Color{180, 170, 160, 255}, Extrude: true}}}
	building := Feature{ID: 9, Kind: GeometryPolygon, Tags: map[string]string{"render_height": "20", "render_min_height": "3"}, Paths: [][]Point{{{0, 0}, {4096, 0}, {4096, 4096}, {0, 4096}, {0, 0}}}}
	scene, err := BuildScene("map", 1, Camera{Zoom: 14, Width: 800, Height: 600, Pitch: 45}, style, []Tile{{ID: TileID{Z: 14}, Layers: []Layer{{Name: "building", Extent: 4096, Features: []Feature{building}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(scene.Delta.Draws) != 1 || scene.Delta.Draws[0].Count != 30 || !scene.Delta.Draws[0].DepthTest {
		t.Fatalf("draw=%+v", scene.Delta.Draws)
	}
	resource := scene.Delta.Resources[0]
	foundMinimum, foundRoof := false, false
	for offset := 0; offset+vertexStride <= len(resource.Bytes); offset += vertexStride {
		elevation := math.Float32frombits(binary.LittleEndian.Uint32(resource.Bytes[offset+8 : offset+12]))
		foundMinimum = foundMinimum || elevation == 3
		foundRoof = foundRoof || elevation == 20
	}
	if !foundMinimum || !foundRoof {
		t.Fatalf("missing extrusion elevations: min=%t roof=%t", foundMinimum, foundRoof)
	}
	if minimum, height := extrusionHeights(map[string]string{"levels": "4"}); minimum != 0 || height != 12 {
		t.Fatalf("level fallback=%v,%v", minimum, height)
	}
}

func TestDecodeMVTRejectsMalformedInput(t *testing.T) {
	for _, input := range [][]byte{nil, {0}, {0x1a, 0x02, 0x08}} {
		if _, err := DecodeMVT(input); err == nil {
			t.Fatalf("accepted %x", input)
		}
	}
}

func FuzzDecodeMVT(f *testing.F) {
	f.Add([]byte{0x1a, 0x05, 0x0a, 0x01, 'x', 0x28, 0x01})
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = DecodeMVT(data) })
}

func TestTextShaperRejectsUntrustedBounds(t *testing.T) {
	if _, err := NewTextShaper(nil); err == nil {
		t.Fatal("empty font accepted")
	}
	shaper := &TextShaper{}
	if _, err := shaper.Shape("label", "en", 16); err == nil {
		t.Fatal("uninitialized shaper accepted")
	}
}

func TestPlaceLabelsUsesPriorityAndStableCollision(t *testing.T) {
	shaper, err := NewTextShaper(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	camera := Camera{Latitude: 0, Longitude: 0, Zoom: 4, Width: 800, Height: 600}
	centerX, centerY := Project(0, 0)
	candidates := []LabelCandidate{
		{ID: "lower", Text: "Lower priority", Locale: "en", WorldX: centerX, WorldY: centerY, Size: 18, Priority: 1},
		{ID: "higher", Text: "Higher priority", Locale: "en", WorldX: centerX, WorldY: centerY, Size: 18, Priority: 10},
		{ID: "visible", Text: "Visible", Locale: "en", WorldX: centerX + .03, WorldY: centerY, Size: 18, Priority: 1},
	}
	one, err := PlaceLabels(context.Background(), camera, shaper, candidates)
	if err != nil {
		t.Fatal(err)
	}
	two, err := PlaceLabels(context.Background(), camera, shaper, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one, two) {
		t.Fatal("label placement is not deterministic")
	}
	if len(one) != 2 || one[0].Candidate.ID != "higher" || one[1].Candidate.ID != "visible" {
		t.Fatalf("placements=%+v", one)
	}
}

func TestPlaceLabelsAllowsExplicitOverlapAndCancellation(t *testing.T) {
	shaper, err := NewTextShaper(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	camera := Camera{Latitude: 0, Longitude: 0, Zoom: 4, Width: 800, Height: 600}
	x, y := Project(0, 0)
	placements, err := PlaceLabels(context.Background(), camera, shaper, []LabelCandidate{
		{ID: "a", Text: "First", Locale: "en", WorldX: x, WorldY: y, Size: 18, AllowOverlap: true},
		{ID: "b", Text: "Second", Locale: "en", WorldX: x, WorldY: y, Size: 18, AllowOverlap: true},
	})
	if err != nil || len(placements) != 2 {
		t.Fatalf("placements=%d err=%v", len(placements), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PlaceLabels(ctx, camera, shaper, []LabelCandidate{{ID: "a", Text: "First", WorldX: x, WorldY: y, Size: 18}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
}

func TestPlaceLabelsRejectsUntrustedBounds(t *testing.T) {
	shaper, err := NewTextShaper(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	camera := Camera{Latitude: 0, Longitude: 0, Zoom: 4, Width: 800, Height: 600}
	if _, err := PlaceLabels(context.Background(), camera, shaper, []LabelCandidate{{ID: "bad", Text: "Bad", WorldX: math.NaN(), WorldY: .5, Size: 18}}); err == nil {
		t.Fatal("non-finite label accepted")
	}
}

func TestGlyphAtlasAndLabelResourcesAreDeterministic(t *testing.T) {
	shaper, err := NewTextShaper(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	camera := Camera{Latitude: 0, Longitude: 0, Zoom: 4, Width: 800, Height: 600}
	x, y := Project(0, 0)
	placements, err := PlaceLabels(context.Background(), camera, shaper, []LabelCandidate{{ID: "place", Text: "Main St", Locale: "en", WorldX: x, WorldY: y, Size: 18}})
	if err != nil {
		t.Fatal(err)
	}
	one, err := AddLabelPlacements(Scene{Delta: protocol.MapSceneDelta{ViewportID: "map", Generation: 1}}, placements, shaper, Color{20, 30, 40, 255}, 10)
	if err != nil {
		t.Fatal(err)
	}
	two, err := AddLabelPlacements(Scene{Delta: protocol.MapSceneDelta{ViewportID: "map", Generation: 1}}, placements, shaper, Color{20, 30, 40, 255}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one, two) {
		t.Fatal("label resources are not deterministic")
	}
	if len(one.Delta.Resources) != 3 || len(one.Delta.Draws) != 1 || one.Delta.Resources[2].Type != protocol.MapResourceTextureAlpha || one.Delta.Draws[0].TextureHash == ([32]byte{}) {
		t.Fatalf("label scene=%+v", one.Delta)
	}
	if _, err := protocol.EncodeMapSceneDelta(one.Delta); err != nil {
		t.Fatalf("label scene is not protocol-safe: %v", err)
	}
	if !bytes.Contains(one.Delta.Resources[2].Bytes, []byte{255}) {
		t.Fatal("glyph atlas contains no opaque coverage")
	}
}

func TestBuildLabelCandidatesUsesTypedRulesAndStableTileOrder(t *testing.T) {
	camera := Camera{Latitude: 0, Longitude: 0, Zoom: 12, Width: 800, Height: 600}
	rules := []LabelRule{{ID: "places", SourceLayer: "place", Geometry: GeometryPoint, TextKeys: []string{"name:en", "name"}, Locale: "en", MinZoom: 0, MaxZoom: 24, Size: []ZoomStop{{Zoom: 0, Value: 10}, {Zoom: 20, Value: 20}}, Priority: 10}}
	tiles := []Tile{
		{ID: TileID{Z: 1, X: 1, Y: 0}, Layers: []Layer{{Name: "place", Extent: 4096, Features: []Feature{{ID: 2, Kind: GeometryPoint, Tags: map[string]string{"name": "Second"}, Paths: [][]Point{{{100, 100}}}}}}}},
		{ID: TileID{Z: 1, X: 0, Y: 0}, Layers: []Layer{{Name: "place", Extent: 4096, Features: []Feature{{ID: 1, Kind: GeometryPoint, Tags: map[string]string{"name:en": "First", "name": "Eerste"}, Paths: [][]Point{{{100, 100}}}}}}}},
	}
	candidates, err := BuildLabelCandidates(camera, rules, tiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || candidates[0].Text != "First" || candidates[1].Text != "Second" || candidates[0].Size != 16 {
		t.Fatalf("candidates=%+v", candidates)
	}
}

func TestPOILabelCategoriesAndDensityAreAppliedDeterministically(t *testing.T) {
	camera := Camera{Latitude: 0, Longitude: 0, Zoom: 15, Width: 800, Height: 600}
	tile := Tile{ID: TileID{Z: 1}, Layers: []Layer{{Name: "poi", Extent: 4096, Features: []Feature{
		{ID: 1, Kind: GeometryPoint, Tags: map[string]string{"name": "Cafe", "class": "cafe"}, Paths: [][]Point{{{100, 100}}}},
		{ID: 2, Kind: GeometryPoint, Tags: map[string]string{"name": "Hospital", "class": "hospital"}, Paths: [][]Point{{{200, 200}}}},
	}}}}
	candidates, err := BuildLabelCandidates(camera, DefaultLabelRulesFor("en", POIFoodDrink), []Tile{tile})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Text != "Cafe" {
		t.Fatalf("filtered candidates=%+v", candidates)
	}
	input := []LabelCandidate{{ID: "low", Priority: 1}, {ID: "high-b", Priority: 2}, {ID: "high-a", Priority: 2}}
	limited, err := LimitLabelCandidates(input, 4, 2)
	if err != nil || len(limited) != 2 || limited[0].ID != "high-a" || limited[1].ID != "high-b" || input[0].ID != "low" {
		t.Fatalf("limited=%+v input=%+v err=%v", limited, input, err)
	}
}

func TestAddSceneFeaturesBuildsDeterministicOverlayResources(t *testing.T) {
	base := Scene{Delta: protocol.MapSceneDelta{ViewportID: "map", Generation: 1}}
	features := []SceneFeature{
		{ID: "route", Geometry: GeometryLine, Coordinates: [][2]float64{{4.8, 52.3}, {4.9, 52.4}}, Color: Color{20, 80, 240, 255}, Width: 6, Layer: 20, Interactive: true},
		{ID: "destination", Geometry: GeometryPoint, Coordinates: [][2]float64{{4.9, 52.4}}, Color: Color{20, 180, 100, 255}, Width: 12, Layer: 30, Interactive: true},
	}
	one, err := AddSceneFeatures(base, features)
	if err != nil {
		t.Fatal(err)
	}
	two, err := AddSceneFeatures(base, features)
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Delta.Resources) != 4 || len(one.Delta.Draws) != 2 || len(one.Picks) != 2 {
		t.Fatalf("overlay scene = %d resources, %d draws, %d picks", len(one.Delta.Resources), len(one.Delta.Draws), len(one.Picks))
	}
	if !reflect.DeepEqual(one, two) {
		t.Fatal("overlay scene is not deterministic")
	}
}

func TestAddSceneFeaturesRejectsNonFiniteCoordinates(t *testing.T) {
	_, err := AddSceneFeatures(Scene{}, []SceneFeature{{ID: "bad", Geometry: GeometryPoint, Coordinates: [][2]float64{{math.NaN(), 0}}}})
	if err == nil {
		t.Fatal("accepted non-finite coordinate")
	}
}

func TestTriangulateConcaveRingPreservesArea(t *testing.T) {
	path := []Point{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 4}, {X: 0, Y: 4}, {X: 0, Y: 0}}
	var encoded []byte
	count, err := triangulateRing(path, 0, &encoded)
	if err != nil {
		t.Fatal(err)
	}
	if count != 12 || len(encoded) != int(count)*4 {
		t.Fatalf("indices=%d bytes=%d", count, len(encoded))
	}
	var triangleArea int64
	for offset := 0; offset < len(encoded); offset += 12 {
		a := path[binary.LittleEndian.Uint32(encoded[offset:])]
		b := path[binary.LittleEndian.Uint32(encoded[offset+4:])]
		c := path[binary.LittleEndian.Uint32(encoded[offset+8:])]
		area := pointCross(a, b, c)
		if area < 0 {
			area = -area
		}
		triangleArea += area
	}
	vertices := make([]ringVertex, len(path)-1)
	for index := range vertices {
		vertices[index] = ringVertex{point: path[index]}
	}
	polygonArea := ringArea(vertices)
	if polygonArea < 0 {
		polygonArea = -polygonArea
	}
	if triangleArea != polygonArea {
		t.Fatalf("triangle area=%d polygon area=%d", triangleArea, polygonArea)
	}
}

func TestTriangulateRingRejectsMalformedAndUnboundedInput(t *testing.T) {
	bowTie := []Point{{X: 0, Y: 0}, {X: 2, Y: 2}, {X: 0, Y: 2}, {X: 2, Y: 0}, {X: 0, Y: 0}}
	if _, err := triangulateRing(bowTie, 0, new([]byte)); err == nil {
		t.Fatal("self-intersecting ring accepted")
	}
	tooLarge := make([]Point, maxTessellationRingVertices+1)
	if _, err := triangulateRing(tooLarge, 0, new([]byte)); err == nil {
		t.Fatal("unbounded ring accepted")
	}
}

func TestTriangulatePolygonPreservesHoles(t *testing.T) {
	outer := []Point{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}
	hole := []Point{{3, 3}, {3, 7}, {7, 7}, {7, 3}, {3, 3}}
	points := append(append([]Point(nil), outer...), hole...)
	var encoded []byte
	count, err := triangulatePolygon([][]Point{outer, hole}, []uint32{0, uint32(len(outer))}, &encoded)
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 || len(encoded) != int(count)*4 {
		t.Fatalf("indices=%d bytes=%d", count, len(encoded))
	}
	if area := indexedTriangleArea(t, points, encoded); area != 168 {
		t.Fatalf("triangulated twice-area=%d want 168", area)
	}
	for offset := 0; offset < len(encoded); offset += 12 {
		a := points[binary.LittleEndian.Uint32(encoded[offset:])]
		b := points[binary.LittleEndian.Uint32(encoded[offset+4:])]
		c := points[binary.LittleEndian.Uint32(encoded[offset+8:])]
		centroidX := float64(a.X+b.X+c.X) / 3
		centroidY := float64(a.Y+b.Y+c.Y) / 3
		if centroidX > 3 && centroidX < 7 && centroidY > 3 && centroidY < 7 {
			t.Fatalf("triangle centroid (%v,%v) fills polygon hole", centroidX, centroidY)
		}
	}
}

func TestTriangulatePolygonSupportsMultipleExteriors(t *testing.T) {
	outer := []Point{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}
	hole := []Point{{3, 3}, {3, 7}, {7, 7}, {7, 3}, {3, 3}}
	island := []Point{{20, 0}, {24, 0}, {24, 4}, {20, 4}, {20, 0}}
	points := append(append(append([]Point(nil), outer...), hole...), island...)
	starts := []uint32{0, uint32(len(outer)), uint32(len(outer) + len(hole))}
	var encoded []byte
	if _, err := triangulatePolygon([][]Point{outer, hole, island}, starts, &encoded); err != nil {
		t.Fatal(err)
	}
	if area := indexedTriangleArea(t, points, encoded); area != 200 {
		t.Fatalf("multipolygon twice-area=%d want 200", area)
	}
}

func TestTriangulatePolygonRejectsUnjoinableHole(t *testing.T) {
	outer := []Point{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}}
	outsideHole := []Point{{20, 0}, {20, 4}, {24, 4}, {24, 0}, {20, 0}}
	if _, err := triangulatePolygon([][]Point{outer, outsideHole}, []uint32{0, uint32(len(outer))}, new([]byte)); err == nil {
		t.Fatal("hole outside its exterior accepted")
	}
}

func FuzzTriangulatePolygonDoesNotPanic(f *testing.F) {
	f.Add([]byte{0, 0, 10, 0, 10, 10, 0, 10}, []byte{3, 3, 3, 7, 7, 7, 7, 3})
	f.Fuzz(func(t *testing.T, outerData, holeData []byte) {
		decode := func(data []byte) []Point {
			if len(data) > 256 {
				data = data[:256]
			}
			points := make([]Point, 0, len(data)/2)
			for len(data) >= 2 {
				points = append(points, Point{X: int32(int8(data[0])), Y: int32(int8(data[1]))})
				data = data[2:]
			}
			return points
		}
		outer, hole := decode(outerData), decode(holeData)
		_, _ = triangulatePolygon([][]Point{outer, hole}, []uint32{0, uint32(len(outer))}, new([]byte))
	})
}

func indexedTriangleArea(t *testing.T, points []Point, encoded []byte) int64 {
	t.Helper()
	var area int64
	for offset := 0; offset < len(encoded); offset += 12 {
		indices := [3]uint32{
			binary.LittleEndian.Uint32(encoded[offset:]),
			binary.LittleEndian.Uint32(encoded[offset+4:]),
			binary.LittleEndian.Uint32(encoded[offset+8:]),
		}
		for _, index := range indices {
			if int(index) >= len(points) {
				t.Fatalf("index %d exceeds %d points", index, len(points))
			}
		}
		triangle := pointCross(points[indices[0]], points[indices[1]], points[indices[2]])
		if triangle < 0 {
			triangle = -triangle
		}
		area += triangle
	}
	return area
}

func equalTiles(a, b []TileID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

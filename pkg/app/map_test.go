package app

import (
	"math"
	"testing"
)

func TestMapCameraPayloadStrictRoundTrip(t *testing.T) {
	want := MapCamera{Latitude: 52.3676, Longitude: 4.9041, Zoom: 13.5, Bearing: -15, Pitch: 40}
	payload := MapCameraPayload(want)
	got, ok := (Msg{Payload: payload}).MapCamera()
	if !ok {
		t.Fatalf("payload rejected: %q", payload)
	}
	if got.Bearing != 345 || got.Latitude != want.Latitude || got.Longitude != want.Longitude || got.Zoom != want.Zoom || got.Pitch != want.Pitch {
		t.Fatalf("camera = %+v", got)
	}
	for _, payload := range []string{
		`{"latitude":52,"longitude":4,"zoom":10,"bearing":0,"pitch":0,"forged":true}`,
		`{"latitude":52,"longitude":4,"zoom":10,"bearing":0,"pitch":0} trailing`,
		`{"latitude":91,"longitude":4,"zoom":10,"bearing":0,"pitch":0}`,
	} {
		if _, ok := (Msg{Payload: payload}).MapCamera(); ok {
			t.Fatalf("accepted %q", payload)
		}
	}
	if MapCameraPayload(MapCamera{Latitude: math.NaN()}) != "" {
		t.Fatal("encoded non-finite camera")
	}
}

func TestMapResourceAndNodeValidation(t *testing.T) {
	source := MapSource{ID: "openmaptiles", ProviderID: "mapps", Snapshot: "snapshot-1", MinZoom: 5, MaxZoom: 18,
		Bounds: MapBounds{West: 4.4, South: 52.1, East: 5.4, North: 53.2}, Attribution: "OpenStreetMap contributors"}
	if !source.Valid() {
		t.Fatal("valid source rejected")
	}
	request := MapResourceRequest{SourceID: source.ID, Snapshot: source.Snapshot, Kind: MapResourceVectorTile, Z: 12, X: 2100, Y: 1340}
	if !request.Valid() {
		t.Fatal("valid tile request rejected")
	}
	request.X = 1 << request.Z
	if request.Valid() {
		t.Fatal("out-of-range tile accepted")
	}
	node := MapViewportNode{
		Semantic: Semantic{ID: "map", Name: "Route map", Enabled: true}, Source: source,
		Camera:  MapCamera{Latitude: 52.37, Longitude: 4.9, Zoom: 12},
		MinZoom: 5, MaxZoom: 18, MinPitch: 0, MaxPitch: 70, Quality: MapQualityAuto,
		CachePolicy: MapCachePersistentOffline, CacheBytes: 2 << 30,
		Features: []MapFeature{{ID: "start", Name: "Start", Geometry: MapGeometryPoint, Role: MapFeatureMarker,
			Positions: []MapPosition{{Latitude: 52.37, Longitude: 4.9}}}},
	}
	if !node.Valid() {
		t.Fatal("valid map node rejected")
	}
	node.Features[0].Positions[0].Longitude = 181
	if node.Valid() {
		t.Fatal("invalid feature accepted")
	}
}

func TestMapStyleAcceptsSemanticTokensWithoutConfusingThemForHex(t *testing.T) {
	style := MapStyle{ID: "semantic", Colors: MapSemanticColors{Land: "surface", Water: "water", Road: "road", Building: "building", Label: "label", Route: "route", Traffic: "traffic"}, LabelDensity: 1, TerrainScale: 1}
	if !style.Valid() {
		t.Fatal("valid semantic map tokens were rejected")
	}
	style.Colors.Traffic = "#12345z"
	if style.Valid() {
		t.Fatal("malformed raw color was accepted")
	}
}

func TestMapFeatureActivationPayloadIsStrictAndBounded(t *testing.T) {
	activation := MapFeatureActivation{ID: "14-8392-5372-poi-7", Name: "Museum", Category: "leisure", SourceLayer: "poi", Latitude: 52.37, Longitude: 4.9}
	payload := MapFeatureActivationPayload(activation)
	decoded, ok := (Msg{Payload: payload}).MapFeatureActivation()
	if !ok || decoded != activation {
		t.Fatalf("decoded=%+v ok=%v payload=%q", decoded, ok, payload)
	}
	for _, invalid := range []string{`{"id":"x","latitude":52,"longitude":4,"forged":true}`, `{"id":"x","latitude":999,"longitude":4}`, `{"id":"","latitude":52,"longitude":4}`} {
		if _, ok := (Msg{Payload: invalid}).MapFeatureActivation(); ok {
			t.Fatalf("accepted %s", invalid)
		}
	}
}

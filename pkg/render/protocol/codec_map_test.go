package protocol

import (
	"crypto/sha256"
	"testing"
)

func TestMapSceneDeltaRoundTrip(t *testing.T) {
	vertexHash := sha256.Sum256([]byte("vertices"))
	indexHash := sha256.Sum256([]byte("indices"))
	want := MapSceneDelta{
		ViewportID: "map", Generation: 7,
		Resources:  []MapSceneResource{{Operation: MapResourceUpload, Type: MapResourceVertexBuffer, Hash: vertexHash, Stride: 20, Bytes: make([]byte, 20)}},
		Draws:      []MapDrawBatch{{VertexHash: vertexHash, IndexHash: indexHash, Primitive: MapPrimitiveTriangles, Count: 6, Layer: 10, Opacity: .75, DepthTest: true}},
		Camera:     MapCamera{Latitude: 52.37, Longitude: 4.9, Zoom: 13, Bearing: 20, Pitch: 45, ViewportWidth: 800, ViewportHeight: 600},
		SunAzimuth: 180, SunElevation: 30,
	}
	payload, err := EncodeMapSceneDelta(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeMapSceneDelta(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.ViewportID != want.ViewportID || got.Generation != want.Generation || len(got.Resources) != 1 || len(got.Draws) != 1 || got.Resources[0].Hash != vertexHash || got.Draws[0].IndexHash != indexHash || got.Camera != want.Camera {
		t.Fatalf("round trip = %+v", got)
	}
}

func TestMapSceneRejectsReleaseBytesAndTrailingData(t *testing.T) {
	hash := sha256.Sum256([]byte("resource"))
	if _, err := EncodeMapSceneDelta(MapSceneDelta{ViewportID: "map", Resources: []MapSceneResource{{Operation: MapResourceRelease, Hash: hash, Bytes: []byte{1}}}}); err == nil {
		t.Fatal("accepted release bytes")
	}
	payload, err := EncodeMapCamera(MapCamera{Latitude: 1})
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, 1)
	if _, err := DecodeMapCamera(payload); err == nil {
		t.Fatal("accepted trailing envelope data")
	}
}

func TestMapSceneRejectsMalformedTexture(t *testing.T) {
	hash := sha256.Sum256([]byte("texture"))
	_, err := EncodeMapSceneDelta(MapSceneDelta{ViewportID: "map", Generation: 1, Resources: []MapSceneResource{{Operation: MapResourceUpload, Type: MapResourceTextureSDF, Hash: hash, Width: 8, Height: 8, Stride: 1, Bytes: make([]byte, 63)}}})
	if err == nil {
		t.Fatal("malformed map texture accepted")
	}
}

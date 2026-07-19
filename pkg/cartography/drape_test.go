package cartography

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/mulavdm/poem/pkg/render/protocol"
)

func TestDrapeSceneRaisesOrdinaryVerticesAndRekeysResources(t *testing.T) {
	var vertices []byte
	appendVertex(&vertices, .25, .25, 7, Color{R: 1, A: 255}, 1, 1)
	oldHash := [32]byte{1}
	scene := Scene{Delta: protocol.MapSceneDelta{
		Resources: []protocol.MapSceneResource{{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceVertexBuffer, Hash: oldHash, Stride: vertexStride, Bytes: vertices}},
		Draws:     []protocol.MapDrawBatch{{VertexHash: oldHash, Layer: 2}},
	}}
	elevation := ElevationTile{Width: 2, Height: 2, Scale: 1, Samples: []int16{10, 10, 10, 10}}
	draped, err := DrapeSceneToTerrain(scene, []TerrainTile{{ID: TileID{Z: 1, X: 0, Y: 0}, Elevation: elevation}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	z := math.Float32frombits(binary.LittleEndian.Uint32(draped.Delta.Resources[0].Bytes[8:12]))
	if z != 27 || draped.Delta.Resources[0].Hash == oldHash || draped.Delta.Draws[0].VertexHash != draped.Delta.Resources[0].Hash {
		t.Fatalf("z=%v resource=%x draw=%x", z, draped.Delta.Resources[0].Hash, draped.Delta.Draws[0].VertexHash)
	}
	if original := math.Float32frombits(binary.LittleEndian.Uint32(scene.Delta.Resources[0].Bytes[8:12])); original != 7 {
		t.Fatalf("input scene mutated: z=%v", original)
	}
}

func TestDrapeSceneLeavesTerrainMeshAndUncoveredVerticesAlone(t *testing.T) {
	var vertices []byte
	appendVertex(&vertices, .75, .75, 3, Color{A: 255}, 1, 0)
	hash := [32]byte{2}
	scene := Scene{Delta: protocol.MapSceneDelta{
		Resources: []protocol.MapSceneResource{{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceVertexBuffer, Hash: hash, Stride: vertexStride, Bytes: vertices}},
		Draws:     []protocol.MapDrawBatch{{VertexHash: hash, Layer: -1_000_000}},
	}}
	elevation := ElevationTile{Width: 2, Height: 2, Scale: 1, Samples: []int16{10, 10, 10, 10}}
	draped, err := DrapeSceneToTerrain(scene, []TerrainTile{{ID: TileID{Z: 1, X: 0, Y: 0}, Elevation: elevation}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if draped.Delta.Resources[0].Hash != hash || draped.Delta.Draws[0].VertexHash != hash {
		t.Fatal("terrain resource was modified")
	}
}

func TestDrapeSceneRejectsMixedOrDuplicateTerrainTiles(t *testing.T) {
	elevation := ElevationTile{Width: 2, Height: 2, Scale: 1, Samples: []int16{1, 1, 1, 1}}
	for _, tiles := range [][]TerrainTile{
		{{ID: TileID{Z: 1}, Elevation: elevation}, {ID: TileID{Z: 2}, Elevation: elevation}},
		{{ID: TileID{Z: 1}, Elevation: elevation}, {ID: TileID{Z: 1}, Elevation: elevation}},
	} {
		if _, err := DrapeSceneToTerrain(Scene{}, tiles, 1); err == nil {
			t.Fatal("invalid terrain set accepted")
		}
	}
}

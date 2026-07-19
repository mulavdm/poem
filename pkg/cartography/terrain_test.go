package cartography

import (
	"bytes"
	"testing"

	"github.com/mulavdm/poem/pkg/render/protocol"
)

func TestTerrainMeshesAreDeterministicBoundedAndSkirted(t *testing.T) {
	elevation := ElevationTile{Width: 3, Height: 3, Scale: 1, Samples: []int16{0, 1, 2, 3, 4, 5, 6, 7, 8}}
	base := Scene{Delta: protocol.MapSceneDelta{ViewportID: "map", Generation: 1}}
	options := TerrainMeshOptions{MaxDimension: 3, VerticalScale: 2, SkirtDepth: 10, Color: Color{R: 20, G: 30, B: 40, A: 255}}
	one, err := AddTerrainMeshes(base, []TerrainTile{{ID: TileID{Z: 1}, Elevation: elevation}}, options)
	if err != nil {
		t.Fatal(err)
	}
	two, err := AddTerrainMeshes(base, []TerrainTile{{ID: TileID{Z: 1}, Elevation: elevation}}, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Delta.Resources) != 2 || len(one.Delta.Draws) != 1 || !one.Delta.Draws[0].DepthTest || one.Delta.Draws[0].Layer >= 0 || one.Delta.Draws[0].Count <= 24 {
		t.Fatalf("terrain scene=%+v", one.Delta)
	}
	if !bytes.Equal(one.Delta.Resources[0].Bytes, two.Delta.Resources[0].Bytes) || !bytes.Equal(one.Delta.Resources[1].Bytes, two.Delta.Resources[1].Bytes) {
		t.Fatal("terrain mesh is not deterministic")
	}
}

func TestTerrainMeshesRejectInvalidOptionsAndTiles(t *testing.T) {
	base := Scene{Delta: protocol.MapSceneDelta{ViewportID: "map", Generation: 1}}
	if _, err := AddTerrainMeshes(base, nil, TerrainMeshOptions{MaxDimension: 1, VerticalScale: 1}); err == nil {
		t.Fatal("accepted invalid terrain dimension")
	}
	if _, err := AddTerrainMeshes(base, []TerrainTile{{ID: TileID{Z: 23}, Elevation: ElevationTile{Width: 2, Height: 2, Scale: 1, Samples: []int16{0, 0, 0, 0}}}}, TerrainMeshOptions{MaxDimension: 2, VerticalScale: 1}); err == nil {
		t.Fatal("accepted invalid terrain tile")
	}
}

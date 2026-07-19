package cartography

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"

	"github.com/mulavdm/poem/pkg/render/protocol"
)

// DrapeSceneToTerrain raises every ordinary scene vertex by the elevation
// sampled from the matching normalized terrain tile. Existing vertex height is
// retained, so building extrusions and other elevated features remain above
// the terrain surface. Terrain mesh draw batches are never modified.
func DrapeSceneToTerrain(scene Scene, tiles []TerrainTile, verticalScale float32) (Scene, error) {
	if !finite(float64(verticalScale)) || verticalScale < 0 || verticalScale > 8 || len(tiles) > 64 {
		return Scene{}, errors.New("cartography: invalid terrain drape options")
	}
	if verticalScale == 0 || len(tiles) == 0 {
		return scene, nil
	}
	sampler, err := newTerrainSampler(tiles)
	if err != nil {
		return Scene{}, err
	}
	// Scene values are immutable once published. Clone the slice headers before
	// replacing resource descriptors or draw references.
	scene.Delta.Resources = append([]protocol.MapSceneResource(nil), scene.Delta.Resources...)
	scene.Delta.Draws = append([]protocol.MapDrawBatch(nil), scene.Delta.Draws...)
	terrainResources := make(map[[32]byte]struct{})
	for _, draw := range scene.Delta.Draws {
		if draw.Layer == -1_000_000 {
			terrainResources[draw.VertexHash] = struct{}{}
		}
	}
	replacements := make(map[[32]byte][32]byte)
	for index := range scene.Delta.Resources {
		resource := &scene.Delta.Resources[index]
		if resource.Operation != protocol.MapResourceUpload || resource.Type != protocol.MapResourceVertexBuffer || resource.Stride < 12 || resource.Stride > 4096 || len(resource.Bytes)%int(resource.Stride) != 0 {
			continue
		}
		if _, terrain := terrainResources[resource.Hash]; terrain {
			continue
		}
		data := append([]byte(nil), resource.Bytes...)
		changed := false
		for offset := 0; offset < len(data); offset += int(resource.Stride) {
			x := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[offset : offset+4])))
			y := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[offset+4 : offset+8])))
			height, ok := sampler.elevationAt(x, y)
			if !ok {
				continue
			}
			z := math.Float32frombits(binary.LittleEndian.Uint32(data[offset+8 : offset+12]))
			z += height * verticalScale
			if !finite(float64(z)) {
				return Scene{}, errors.New("cartography: draped vertex is non-finite")
			}
			binary.LittleEndian.PutUint32(data[offset+8:offset+12], math.Float32bits(z))
			changed = true
		}
		if !changed {
			continue
		}
		oldHash := resource.Hash
		resource.Bytes = data
		resource.Hash = sha256.Sum256(data)
		replacements[oldHash] = resource.Hash
	}
	for index := range scene.Delta.Draws {
		if replacement, ok := replacements[scene.Delta.Draws[index].VertexHash]; ok {
			scene.Delta.Draws[index].VertexHash = replacement
		}
	}
	return scene, nil
}

type terrainSampler struct {
	zoom  uint8
	tiles map[uint64]ElevationTile
}

func newTerrainSampler(tiles []TerrainTile) (terrainSampler, error) {
	sampler := terrainSampler{tiles: make(map[uint64]ElevationTile, len(tiles))}
	for index, tile := range tiles {
		if !tile.Elevation.Valid() || tile.ID.Z > 22 || tile.ID.X >= 1<<tile.ID.Z || tile.ID.Y >= 1<<tile.ID.Z {
			return terrainSampler{}, errors.New("cartography: invalid terrain tile")
		}
		if index == 0 {
			sampler.zoom = tile.ID.Z
		} else if tile.ID.Z != sampler.zoom {
			return terrainSampler{}, errors.New("cartography: terrain tiles use mixed zoom levels")
		}
		key := uint64(tile.ID.X)<<32 | uint64(tile.ID.Y)
		if _, duplicate := sampler.tiles[key]; duplicate {
			return terrainSampler{}, errors.New("cartography: duplicate terrain tile")
		}
		sampler.tiles[key] = tile.Elevation
	}
	return sampler, nil
}

func (sampler terrainSampler) elevationAt(worldX, worldY float64) (float32, bool) {
	if !finite(worldX) || !finite(worldY) || worldY < 0 || worldY > 1 {
		return 0, false
	}
	n := math.Exp2(float64(sampler.zoom))
	wrappedX := worldX - math.Floor(worldX)
	if worldX == 1 {
		wrappedX = math.Nextafter(1, 0)
	}
	scaledX, scaledY := wrappedX*n, math.Min(math.Nextafter(n, 0), worldY*n)
	x, y := uint32(math.Floor(scaledX)), uint32(math.Floor(scaledY))
	tile, ok := sampler.tiles[uint64(x)<<32|uint64(y)]
	if !ok {
		return 0, false
	}
	return tile.ElevationAt(scaledX-float64(x), scaledY-float64(y)), true
}

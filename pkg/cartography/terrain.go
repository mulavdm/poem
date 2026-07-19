package cartography

import (
	"crypto/sha256"
	"errors"
	"math"
	"sort"

	"github.com/mulavdm/poem/pkg/render/protocol"
)

// TerrainTile pairs one Web-Mercator tile identity with normalized elevation.
type TerrainTile struct {
	// ID identifies the tile footprint.
	ID TileID
	// Elevation contains the immutable normalized height grid.
	Elevation ElevationTile
}

// TerrainMeshOptions bounds terrain detail and vertical presentation.
type TerrainMeshOptions struct {
	// MaxDimension is the largest generated grid edge, from 2 through 257.
	MaxDimension int
	// VerticalScale multiplies elevations and must be within 0 through 8.
	VerticalScale float32
	// SkirtDepth extends tile borders downward in metres to conceal seams.
	SkirtDepth float32
	// Color is the semantic terrain surface color.
	Color Color
}

// AddTerrainMeshes deterministically prepends retained terrain meshes to a
// scene. Raw elevation remains outside the retained GPU protocol.
func AddTerrainMeshes(scene Scene, tiles []TerrainTile, options TerrainMeshOptions) (Scene, error) {
	if options.MaxDimension < 2 || options.MaxDimension > maxElevationGrid || !finite(float64(options.VerticalScale)) || options.VerticalScale < 0 || options.VerticalScale > 8 || !finite(float64(options.SkirtDepth)) || options.SkirtDepth < 0 || options.SkirtDepth > 1000 || len(tiles) > 64 {
		return Scene{}, errors.New("cartography: invalid terrain mesh options")
	}
	if options.VerticalScale == 0 || len(tiles) == 0 {
		return scene, nil
	}
	ordered := append([]TerrainTile(nil), tiles...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].ID.Z != ordered[j].ID.Z {
			return ordered[i].ID.Z < ordered[j].ID.Z
		}
		if ordered[i].ID.Y != ordered[j].ID.Y {
			return ordered[i].ID.Y < ordered[j].ID.Y
		}
		return ordered[i].ID.X < ordered[j].ID.X
	})
	var vertices, indices []byte
	var vertexCount, indexCount uint32
	for _, tile := range ordered {
		if !tile.Elevation.Valid() || tile.ID.Z > 22 || tile.ID.X >= 1<<tile.ID.Z || tile.ID.Y >= 1<<tile.ID.Z {
			return Scene{}, errors.New("cartography: invalid terrain tile")
		}
		width := min(options.MaxDimension, int(tile.Elevation.Width))
		height := min(options.MaxDimension, int(tile.Elevation.Height))
		start := vertexCount
		for y := 0; y < height; y++ {
			v := float64(y) / float64(height-1)
			for x := 0; x < width; x++ {
				u := float64(x) / float64(width-1)
				worldX, worldY := terrainWorldPosition(tile.ID, u, v)
				elevation := tile.Elevation.ElevationAt(u, v) * options.VerticalScale
				appendVertex(&vertices, worldX, worldY, elevation, options.Color, 1, 0)
				vertexCount++
			}
		}
		for y := 0; y < height-1; y++ {
			for x := 0; x < width-1; x++ {
				topLeft := start + uint32(y*width+x)
				topRight, bottomLeft := topLeft+1, topLeft+uint32(width)
				bottomRight := bottomLeft + 1
				appendIndex(&indices, topLeft)
				appendIndex(&indices, bottomLeft)
				appendIndex(&indices, topRight)
				appendIndex(&indices, topRight)
				appendIndex(&indices, bottomLeft)
				appendIndex(&indices, bottomRight)
				indexCount += 6
			}
		}
		if options.SkirtDepth > 0 {
			for _, edge := range terrainEdges(width, height) {
				for index := 1; index < len(edge); index++ {
					for _, gridIndex := range []int{edge[index-1], edge[index]} {
						u := float64(gridIndex%width) / float64(width-1)
						v := float64(gridIndex/width) / float64(height-1)
						worldX, worldY := terrainWorldPosition(tile.ID, u, v)
						top := tile.Elevation.ElevationAt(u, v) * options.VerticalScale
						appendVertex(&vertices, worldX, worldY, top, options.Color, 1, 0)
						appendVertex(&vertices, worldX, worldY, top-options.SkirtDepth, options.Color, 1, 0)
						vertexCount += 2
					}
					base := vertexCount - 4
					appendIndex(&indices, base)
					appendIndex(&indices, base+1)
					appendIndex(&indices, base+2)
					appendIndex(&indices, base+2)
					appendIndex(&indices, base+1)
					appendIndex(&indices, base+3)
					indexCount += 6
				}
			}
		}
		if vertexCount > maxSceneVertices || indexCount > maxSceneIndices {
			return Scene{}, errors.New("cartography: terrain scene exceeds geometry bounds")
		}
	}
	if vertexCount == 0 || indexCount == 0 {
		return scene, nil
	}
	vertexHash, indexHash := sha256.Sum256(vertices), sha256.Sum256(indices)
	scene.Delta.Resources = append(scene.Delta.Resources,
		protocol.MapSceneResource{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceVertexBuffer, Hash: vertexHash, Stride: vertexStride, Bytes: vertices},
		protocol.MapSceneResource{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceIndexBuffer, Hash: indexHash, Stride: 4, Bytes: indices},
	)
	draw := protocol.MapDrawBatch{VertexHash: vertexHash, IndexHash: indexHash, Primitive: protocol.MapPrimitiveTriangles, Count: indexCount, Layer: -1_000_000, Opacity: float32(options.Color.A) / 255, DepthTest: true}
	scene.Delta.Draws = append([]protocol.MapDrawBatch{draw}, scene.Delta.Draws...)
	return scene, nil
}

func terrainWorldPosition(tile TileID, u, v float64) (float32, float32) {
	scale := math.Exp2(float64(tile.Z))
	return float32((float64(tile.X) + u) / scale), float32((float64(tile.Y) + v) / scale)
}

func terrainEdges(width, height int) [][]int {
	top, right, bottom, left := make([]int, width), make([]int, height), make([]int, width), make([]int, height)
	for x := 0; x < width; x++ {
		top[x] = x
		bottom[x] = (height-1)*width + (width - 1 - x)
	}
	for y := 0; y < height; y++ {
		right[y] = y*width + width - 1
		left[y] = (height - 1 - y) * width
	}
	return [][]int{top, right, bottom, left}
}

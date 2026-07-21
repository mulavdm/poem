package cartography

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/mulavdm/poem/pkg/render/protocol"
)

const (
	vertexStride                = 32
	maxSceneVertices            = 4_000_000
	maxSceneIndices             = 12_000_000
	maxTessellationRingVertices = 8192
	maxTessellationCoordinate   = 1 << 22
)

// Tile combines a tile identity with decoded layers.
type Tile struct {
	ID     TileID
	Layers []Layer
}

// PickRecord maps a stable feature ID to a style layer and geographic bounds.
// Presenters can use this bounded index to resolve GPU or CPU feature picking.
type PickRecord struct {
	FeatureID   uint64
	StableID    string
	Name        string
	Description string
	Category    string
	SourceLayer string
	Longitude   float64
	Latitude    float64
	StyleLayer  string
	MinX, MinY  float32
	MaxX, MaxY  float32
}

// Scene is a deterministic retained scene update plus its feature-picking data.
type Scene struct {
	Delta protocol.MapSceneDelta
	Picks []PickRecord
}

// BuildSceneLit is BuildScene with scene lighting. When lighting.DynamicSun is
// set it computes the sun's azimuth and clamped visual elevation for the
// camera's location and injected clock; otherwise it keeps the fixed default.
func BuildSceneLit(viewportID string, generation uint64, camera Camera, style Style, tiles []Tile, lighting Lighting) (Scene, error) {
	scene, err := BuildScene(viewportID, generation, camera, style, tiles)
	if err != nil {
		return Scene{}, err
	}
	azimuth, elevation := lighting.resolve(camera.Latitude, camera.Longitude)
	scene.Delta.SunAzimuth = float32(azimuth)
	scene.Delta.SunElevation = float32(elevation)
	scene.Delta.ShadowCascades = lighting.ShadowCascades
	if lighting.FogDensity > 0 {
		// Callers pass the land tone so the horizon dissolves into the map's own
		// background; the fallback is a neutral haze for callers that do not.
		fog := Color{R: 226, G: 232, B: 240, A: 255}
		if lighting.FogColor != nil {
			fog = *lighting.FogColor
		}
		scene.Delta.FogDensity = lighting.FogDensity
		scene.Delta.FogRed = float32(fog.R) / 255
		scene.Delta.FogGreen = float32(fog.G) / 255
		scene.Delta.FogBlue = float32(fog.B) / 255
	}
	return scene, nil
}

// BuildScene evaluates a typed style over decoded tiles and creates hash-keyed
// immutable buffers. Inputs are copied into the resulting byte resources.
func BuildScene(viewportID string, generation uint64, camera Camera, style Style, tiles []Tile) (Scene, error) {
	if viewportID == "" || len(viewportID) > 1024 || generation == 0 {
		return Scene{}, errors.New("cartography: invalid scene identity")
	}
	if err := camera.Validate(); err != nil {
		return Scene{}, err
	}
	if err := style.Validate(); err != nil {
		return Scene{}, err
	}
	ordered := append([]Tile(nil), tiles...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].ID.Z != ordered[j].ID.Z {
			return ordered[i].ID.Z < ordered[j].ID.Z
		}
		if ordered[i].ID.Y != ordered[j].ID.Y {
			return ordered[i].ID.Y < ordered[j].ID.Y
		}
		return ordered[i].ID.X < ordered[j].ID.X
	})
	delta := protocol.MapSceneDelta{
		ViewportID:   viewportID,
		Generation:   generation,
		Camera:       protocol.MapCamera{Latitude: camera.Latitude, Longitude: camera.Longitude, Zoom: float32(camera.Zoom), Bearing: float32(camera.Bearing), Pitch: float32(camera.Pitch), ViewportWidth: camera.Width, ViewportHeight: camera.Height},
		SunAzimuth:   defaultSunAzimuth,
		SunElevation: defaultSunElevation,
	}
	var picks []PickRecord
	for layerIndex, styleLayer := range style.Layers {
		var vertices, indices []byte
		var vertexCount, indexCount uint32
		// Dot layers get density thinning and a distance fade so a pitched view
		// does not drown in markers; see declutter.go.
		var declutter *pointDeclutter
		if styleLayer.Geometry == GeometryPoint {
			declutter = newPointDeclutter(camera)
		}
		for _, tile := range ordered {
			for _, source := range tile.Layers {
				if source.Name != styleLayer.SourceLayer || source.Extent == 0 {
					continue
				}
				for featureIndex, feature := range source.Features {
					if !styleLayer.matches(feature, camera.Zoom) {
						continue
					}
					featureID := feature.ID
					if featureID == 0 {
						featureID = syntheticFeatureID(tile.ID, source.Name, featureIndex)
					}
					startVertex := vertexCount
					startVertexBytes := len(vertices)
					startIndexBytes := len(indices)
					minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
					maxX, maxY := -minX, -minY
					var polygonPaths [][]Point
					var polygonStarts []uint32
					minimumHeight, height := float32(0), float32(0)
					if styleLayer.Extrude {
						minimumHeight, height = extrusionHeights(feature.Tags)
					}
					for _, path := range feature.Paths {
						for _, point := range path {
							x, y := tilePosition(tile.ID, source.Extent, point)
							minX, minY = min(minX, x), min(minY, y)
							maxX, maxY = max(maxX, x), max(maxY, y)
						}
						switch feature.Kind {
						case GeometryPoint:
							pathStart := vertexCount
							for _, point := range path {
								x, y := tilePosition(tile.ID, source.Extent, point)
								fade := float32(1)
								if declutter != nil {
									scale, ok := declutter.admit(x, y)
									if !ok {
										continue
									}
									fade = scale
								}
								appendVertex(&vertices, x, y, height, fadedColor(styleLayer.Color, fade), styleLayer.width(camera.Zoom), featureID)
								vertexCount++
							}
							count := vertexCount - pathStart
							for i := uint32(0); i < count; i++ {
								appendIndex(&indices, pathStart+i)
								indexCount++
							}
						case GeometryLine:
							if err := appendTileLineMesh(&vertices, &indices, &vertexCount, &indexCount, tile.ID, source.Extent, path, styleLayer.Color, styleLayer.width(camera.Zoom), featureID, camera.Zoom); err != nil {
								return Scene{}, err
							}
						case GeometryPolygon:
							pathStart := vertexCount
							for _, point := range path {
								x, y := tilePosition(tile.ID, source.Extent, point)
								appendVertex(&vertices, x, y, 0, styleLayer.Color, styleLayer.width(camera.Zoom), featureID)
								vertexCount++
							}
							polygonPaths = append(polygonPaths, path)
							polygonStarts = append(polygonStarts, pathStart)
						}
					}
					if feature.Kind == GeometryPolygon {
						added, triangulateErr := triangulatePolygon(polygonPaths, polygonStarts, &indices)
						if triangulateErr != nil {
							// MVT is external input and a single malformed feature must
							// not blank every otherwise valid layer in the viewport. The
							// decoder already bounds paths and coordinates; discard this
							// feature's unreferenced vertices and any partial ear-clipping
							// indices, then continue deterministically.
							vertices = vertices[:startVertexBytes]
							indices = indices[:startIndexBytes]
							vertexCount = startVertex
							continue
						}
						indexCount += added
						if styleLayer.Extrude && height > minimumHeight {
							for _, path := range polygonPaths {
								if err := appendExtrusionWalls(&vertices, &indices, &vertexCount, &indexCount, tile.ID, source.Extent, path, styleLayer.Color, minimumHeight, height, featureID); err != nil {
									return Scene{}, err
								}
							}
						}
					}
					if vertexCount > maxSceneVertices || indexCount > maxSceneIndices {
						return Scene{}, errors.New("cartography: scene exceeds geometry bounds")
					}
					if styleLayer.Interactive && vertexCount > startVertex {
						longitude, latitude := Unproject(float64(minX+maxX)/2, float64(minY+maxY)/2)
						picks = append(picks, PickRecord{FeatureID: featureID, StableID: fmt.Sprintf("%d-%d-%d-%s-%d", tile.ID.Z, tile.ID.X, tile.ID.Y, source.Name, featureID), Name: feature.Tags["name"], Description: feature.Tags["class"], Category: poiCategoryName(classifyPOI(feature.Tags)), SourceLayer: source.Name, Longitude: longitude, Latitude: latitude, StyleLayer: styleLayer.ID, MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY})
					}
				}
			}
		}
		if vertexCount == 0 || indexCount == 0 {
			continue
		}
		vertexHash, indexHash := sha256.Sum256(vertices), sha256.Sum256(indices)
		delta.Resources = append(delta.Resources,
			protocol.MapSceneResource{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceVertexBuffer, Hash: vertexHash, Stride: vertexStride, Bytes: vertices},
			protocol.MapSceneResource{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceIndexBuffer, Hash: indexHash, Stride: 4, Bytes: indices},
		)
		primitive := protocol.MapPrimitiveTriangles
		if styleLayer.Geometry == GeometryPoint {
			primitive = protocol.MapPrimitivePoints
		}
		delta.Draws = append(delta.Draws, protocol.MapDrawBatch{VertexHash: vertexHash, IndexHash: indexHash, Primitive: primitive, Count: indexCount, Layer: int32(layerIndex), Opacity: float32(styleLayer.Color.A) / 255, DepthTest: styleLayer.Extrude})
	}
	sort.Slice(picks, func(i, j int) bool {
		if picks[i].StyleLayer != picks[j].StyleLayer {
			return picks[i].StyleLayer < picks[j].StyleLayer
		}
		return picks[i].StableID < picks[j].StableID
	})
	return Scene{Delta: delta, Picks: picks}, nil
}

func poiCategoryName(category POICategories) string {
	switch category {
	case POITransport:
		return "transport"
	case POIParkingFuel:
		return "parking-fuel"
	case POIFoodDrink:
		return "food-drink"
	case POIHealth:
		return "health"
	case POIShoppingServices:
		return "shopping-services"
	case POILeisureTourism:
		return "leisure-tourism"
	case POICivic:
		return "civic"
	default:
		return ""
	}
}

type ringVertex struct {
	point Point
	index uint32
}

func triangulateRing(path []Point, pathStart uint32, indices *[]byte) (uint32, error) {
	vertices, err := normalizeRing(path, pathStart)
	if err != nil {
		return 0, err
	}
	return triangulateSimpleRing(vertices, indices)
}

func normalizeRing(path []Point, pathStart uint32) ([]ringVertex, error) {
	end := len(path)
	if end > 1 && path[0] == path[end-1] {
		end--
	}
	if end < 3 || end > maxTessellationRingVertices {
		return nil, errors.New("cartography: polygon ring exceeds tessellation bounds")
	}
	vertices := make([]ringVertex, 0, end)
	for index := 0; index < end; index++ {
		if path[index].X < -maxTessellationCoordinate || path[index].X > maxTessellationCoordinate ||
			path[index].Y < -maxTessellationCoordinate || path[index].Y > maxTessellationCoordinate {
			return nil, errors.New("cartography: polygon coordinate exceeds tessellation bounds")
		}
		if len(vertices) > 0 && vertices[len(vertices)-1].point == path[index] {
			continue
		}
		vertices = append(vertices, ringVertex{point: path[index], index: pathStart + uint32(index)})
	}
	if len(vertices) > 2 && vertices[0].point == vertices[len(vertices)-1].point {
		vertices = vertices[:len(vertices)-1]
	}
	if len(vertices) < 3 {
		return nil, errors.New("cartography: degenerate polygon ring")
	}
	for changed := true; changed && len(vertices) > 3; {
		changed = false
		for index := range vertices {
			if pointCross(vertices[(index+len(vertices)-1)%len(vertices)].point, vertices[index].point, vertices[(index+1)%len(vertices)].point) == 0 {
				copy(vertices[index:], vertices[index+1:])
				vertices = vertices[:len(vertices)-1]
				changed = true
				break
			}
		}
	}
	orientation := ringArea(vertices)
	if orientation == 0 {
		return nil, errors.New("cartography: zero-area polygon ring")
	}
	return vertices, nil
}

type polygonRings struct {
	exterior []ringVertex
	holes    [][]ringVertex
}

func triangulatePolygon(paths [][]Point, starts []uint32, indices *[]byte) (uint32, error) {
	if len(paths) == 0 || len(paths) != len(starts) {
		return 0, errors.New("cartography: invalid polygon paths")
	}
	var polygons []polygonRings
	var exteriorOrientation int64
	for index, path := range paths {
		ring, err := normalizeRing(path, starts[index])
		if err != nil {
			return 0, err
		}
		orientation := ringArea(ring)
		if exteriorOrientation == 0 {
			exteriorOrientation = orientation
		}
		if (orientation > 0) == (exteriorOrientation > 0) {
			polygons = append(polygons, polygonRings{exterior: ring})
			continue
		}
		if len(polygons) == 0 {
			return 0, errors.New("cartography: polygon hole has no exterior")
		}
		polygons[len(polygons)-1].holes = append(polygons[len(polygons)-1].holes, ring)
	}
	var added uint32
	for _, polygon := range polygons {
		merged := append([]ringVertex(nil), polygon.exterior...)
		holes := append([][]ringVertex(nil), polygon.holes...)
		sort.SliceStable(holes, func(i, j int) bool {
			a := rightmostRingVertex(holes[i])
			b := rightmostRingVertex(holes[j])
			if holes[i][a].point.X != holes[j][b].point.X {
				return holes[i][a].point.X > holes[j][b].point.X
			}
			if holes[i][a].point.Y != holes[j][b].point.Y {
				return holes[i][a].point.Y < holes[j][b].point.Y
			}
			return holes[i][a].index < holes[j][b].index
		})
		for holeIndex, hole := range holes {
			var err error
			merged, err = bridgeHole(merged, hole, holes[holeIndex+1:])
			if err != nil {
				return 0, err
			}
		}
		count, err := triangulateSimpleRing(merged, indices)
		if err != nil {
			return 0, err
		}
		added += count
	}
	return added, nil
}

func rightmostRingVertex(ring []ringVertex) int {
	best := 0
	for index := 1; index < len(ring); index++ {
		if ring[index].point.X > ring[best].point.X ||
			(ring[index].point.X == ring[best].point.X && ring[index].point.Y < ring[best].point.Y) ||
			(ring[index].point == ring[best].point && ring[index].index < ring[best].index) {
			best = index
		}
	}
	return best
}

func bridgeHole(exterior, hole []ringVertex, unmergedHoles [][]ringVertex) ([]ringVertex, error) {
	holeIndex := rightmostRingVertex(hole)
	holePoint := hole[holeIndex].point
	best := -1
	var bestDistance uint64
	for exteriorIndex, candidate := range exterior {
		if !bridgeVisible(holePoint, candidate.point, exterior, exteriorIndex, hole, holeIndex, unmergedHoles) {
			continue
		}
		dx := int64(candidate.point.X) - int64(holePoint.X)
		dy := int64(candidate.point.Y) - int64(holePoint.Y)
		distance := uint64(dx*dx + dy*dy)
		if best < 0 || distance < bestDistance || (distance == bestDistance && candidate.index < exterior[best].index) {
			best, bestDistance = exteriorIndex, distance
		}
	}
	if best < 0 {
		return nil, errors.New("cartography: polygon hole cannot be joined to exterior")
	}
	merged := make([]ringVertex, 0, len(exterior)+len(hole)+2)
	merged = append(merged, exterior[:best+1]...)
	for offset := 0; offset < len(hole); offset++ {
		merged = append(merged, hole[(holeIndex+offset)%len(hole)])
	}
	merged = append(merged, hole[holeIndex], exterior[best])
	merged = append(merged, exterior[best+1:]...)
	return merged, nil
}

func bridgeVisible(a, b Point, exterior []ringVertex, exteriorIndex int, hole []ringVertex, holeIndex int, otherHoles [][]ringVertex) bool {
	if a == b {
		return false
	}
	if segmentCrossesRing(a, b, exterior, exteriorIndex) || segmentCrossesRing(a, b, hole, holeIndex) {
		return false
	}
	for _, other := range otherHoles {
		if segmentCrossesRing(a, b, other, -1) {
			return false
		}
	}
	midX := (float64(a.X) + float64(b.X)) / 2
	midY := (float64(a.Y) + float64(b.Y)) / 2
	if !pointInRing(midX, midY, exterior) || pointInRing(midX, midY, hole) {
		return false
	}
	for _, other := range otherHoles {
		if pointInRing(midX, midY, other) {
			return false
		}
	}
	return true
}

func segmentCrossesRing(a, b Point, ring []ringVertex, allowedVertex int) bool {
	for index, vertex := range ring {
		nextIndex := (index + 1) % len(ring)
		next := ring[nextIndex]
		if allowedVertex >= 0 && (index == allowedVertex || nextIndex == allowedVertex) {
			continue
		}
		if segmentsIntersect(a, b, vertex.point, next.point) {
			return true
		}
	}
	return false
}

func segmentsIntersect(a, b, c, d Point) bool {
	abc := pointCross(a, b, c)
	abd := pointCross(a, b, d)
	cda := pointCross(c, d, a)
	cdb := pointCross(c, d, b)
	if abc == 0 && pointOnSegment(c, a, b) || abd == 0 && pointOnSegment(d, a, b) ||
		cda == 0 && pointOnSegment(a, c, d) || cdb == 0 && pointOnSegment(b, c, d) {
		return true
	}
	return (abc > 0) != (abd > 0) && (cda > 0) != (cdb > 0)
}

func pointOnSegment(point, a, b Point) bool {
	return point.X >= min(a.X, b.X) && point.X <= max(a.X, b.X) &&
		point.Y >= min(a.Y, b.Y) && point.Y <= max(a.Y, b.Y)
}

func pointInRing(x, y float64, ring []ringVertex) bool {
	inside := false
	for index, vertex := range ring {
		next := ring[(index+1)%len(ring)]
		ay, by := float64(vertex.point.Y), float64(next.point.Y)
		if (ay > y) == (by > y) {
			continue
		}
		intersectionX := float64(next.point.X-vertex.point.X)*(y-ay)/(by-ay) + float64(vertex.point.X)
		if x < intersectionX {
			inside = !inside
		}
	}
	return inside
}

func triangulateSimpleRing(vertices []ringVertex, indices *[]byte) (uint32, error) {
	orientation := ringArea(vertices)
	remaining := append([]ringVertex(nil), vertices...)
	var added uint32
	for len(remaining) > 3 {
		ear := -1
		for index := range remaining {
			previous := remaining[(index+len(remaining)-1)%len(remaining)]
			current := remaining[index]
			next := remaining[(index+1)%len(remaining)]
			cross := pointCross(previous.point, current.point, next.point)
			if cross == 0 || (cross > 0) != (orientation > 0) {
				continue
			}
			contains := false
			for candidateIndex, candidate := range remaining {
				if candidateIndex == index || candidateIndex == (index+len(remaining)-1)%len(remaining) || candidateIndex == (index+1)%len(remaining) {
					continue
				}
				if candidate.point == previous.point || candidate.point == current.point || candidate.point == next.point {
					continue
				}
				if pointInTriangle(candidate.point, previous.point, current.point, next.point, orientation) {
					contains = true
					break
				}
			}
			if !contains {
				ear = index
				appendIndex(indices, previous.index)
				appendIndex(indices, current.index)
				appendIndex(indices, next.index)
				added += 3
				break
			}
		}
		if ear < 0 {
			return 0, errors.New("cartography: polygon ring is self-intersecting or malformed")
		}
		copy(remaining[ear:], remaining[ear+1:])
		remaining = remaining[:len(remaining)-1]
	}
	appendIndex(indices, remaining[0].index)
	appendIndex(indices, remaining[1].index)
	appendIndex(indices, remaining[2].index)
	return added + 3, nil
}

func ringArea(vertices []ringVertex) int64 {
	var twiceArea int64
	for index, vertex := range vertices {
		next := vertices[(index+1)%len(vertices)]
		twiceArea += int64(vertex.point.X)*int64(next.point.Y) - int64(next.point.X)*int64(vertex.point.Y)
	}
	return twiceArea
}

func pointCross(a, b, c Point) int64 {
	return (int64(b.X)-int64(a.X))*(int64(c.Y)-int64(b.Y)) -
		(int64(b.Y)-int64(a.Y))*(int64(c.X)-int64(b.X))
}

func pointInTriangle(point, a, b, c Point, orientation int64) bool {
	ab := pointCross(a, b, point)
	bc := pointCross(b, c, point)
	ca := pointCross(c, a, point)
	if orientation > 0 {
		return ab >= 0 && bc >= 0 && ca >= 0
	}
	return ab <= 0 && bc <= 0 && ca <= 0
}

func tilePosition(id TileID, extent uint32, point Point) (float32, float32) {
	n := math.Exp2(float64(id.Z))
	return float32((float64(id.X) + float64(point.X)/float64(extent)) / n), float32((float64(id.Y) + float64(point.Y)/float64(extent)) / n)
}

func appendVertex(dst *[]byte, x, y, z float32, color Color, width float32, featureID uint64) {
	var data [vertexStride]byte
	binary.LittleEndian.PutUint32(data[0:4], math.Float32bits(x))
	binary.LittleEndian.PutUint32(data[4:8], math.Float32bits(y))
	binary.LittleEndian.PutUint32(data[8:12], math.Float32bits(z))
	data[12], data[13], data[14], data[15] = color.R, color.G, color.B, color.A
	binary.LittleEndian.PutUint32(data[16:20], math.Float32bits(width))
	binary.LittleEndian.PutUint64(data[20:28], featureID)
	*dst = append(*dst, data[:]...)
}

func appendTileLineMesh(vertices, indices *[]byte, vertexCount, indexCount *uint32, id TileID, extent uint32, path []Point, color Color, width float32, featureID uint64, zoom float64) error {
	if len(path) < 2 || width <= 0 {
		return nil
	}
	if uint64(len(path)-1)*4 > uint64(maxSceneVertices-*vertexCount) || uint64(len(path)-1)*6 > uint64(maxSceneIndices-*indexCount) {
		return errors.New("cartography: line mesh exceeds geometry bounds")
	}
	previousX, previousY := tilePosition(id, extent, path[0])
	for _, point := range path[1:] {
		x, y := tilePosition(id, extent, point)
		appendLineSegment(vertices, indices, vertexCount, indexCount, previousX, previousY, x, y, color, width, featureID, zoom)
		previousX, previousY = x, y
	}
	return nil
}

func appendLineSegment(vertices, indices *[]byte, vertexCount, indexCount *uint32, ax, ay, bx, by float32, color Color, width float32, featureID uint64, zoom float64) {
	dx, dy := float64(bx-ax), float64(by-ay)
	length := math.Hypot(dx, dy)
	if length == 0 || width <= 0 {
		return
	}
	halfWorld := float64(width) / (2 * 512 * math.Exp2(zoom))
	nx, ny := float32(-dy/length*halfWorld), float32(dx/length*halfWorld)
	base := *vertexCount
	appendVertex(vertices, ax+nx, ay+ny, 0, color, width, featureID)
	appendVertex(vertices, ax-nx, ay-ny, 0, color, width, featureID)
	appendVertex(vertices, bx+nx, by+ny, 0, color, width, featureID)
	appendVertex(vertices, bx-nx, by-ny, 0, color, width, featureID)
	for _, index := range [...]uint32{base, base + 1, base + 2, base + 2, base + 1, base + 3} {
		appendIndex(indices, index)
	}
	*vertexCount += 4
	*indexCount += 6
}

func appendExtrusionWalls(vertices, indices *[]byte, vertexCount, indexCount *uint32, id TileID, extent uint32, path []Point, color Color, minimumHeight, height float32, featureID uint64) error {
	ring, err := normalizeRing(path, 0)
	if err != nil {
		return err
	}
	if uint64(len(ring))*4 > uint64(maxSceneVertices-*vertexCount) || uint64(len(ring))*6 > uint64(maxSceneIndices-*indexCount) {
		return errors.New("cartography: extrusion exceeds geometry bounds")
	}
	for index, current := range ring {
		next := ring[(index+1)%len(ring)]
		ax, ay := tilePosition(id, extent, current.point)
		bx, by := tilePosition(id, extent, next.point)
		base := *vertexCount
		appendVertex(vertices, ax, ay, minimumHeight, color, 0, featureID)
		appendVertex(vertices, bx, by, minimumHeight, color, 0, featureID)
		appendVertex(vertices, ax, ay, height, color, 0, featureID)
		appendVertex(vertices, bx, by, height, color, 0, featureID)
		for _, vertex := range [...]uint32{base, base + 1, base + 2, base + 2, base + 1, base + 3} {
			appendIndex(indices, vertex)
		}
		*vertexCount += 4
		*indexCount += 6
	}
	return nil
}

func extrusionHeights(tags map[string]string) (minimum, height float32) {
	if tags["hide_3d"] == "true" {
		return 0, 0
	}
	parse := func(keys ...string) (float64, bool) {
		for _, key := range keys {
			if value, ok := tags[key]; ok {
				parsed, err := strconv.ParseFloat(value, 64)
				if err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) {
					return parsed, true
				}
			}
		}
		return 0, false
	}
	value, ok := parse("render_height", "height")
	if !ok {
		if levels, levelsOK := parse("building:levels", "levels"); levelsOK {
			value = levels * 3
		} else {
			value = 8
		}
	}
	minimumValue, _ := parse("render_min_height", "min_height")
	value = math.Max(0, math.Min(1000, value))
	minimumValue = math.Max(0, math.Min(value, minimumValue))
	return float32(minimumValue), float32(value)
}

func appendIndex(dst *[]byte, value uint32) {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], value)
	*dst = append(*dst, data[:]...)
}

func syntheticFeatureID(id TileID, layer string, index int) uint64 {
	hash := sha256.New()
	hash.Write([]byte{byte(id.Z)})
	var data [12]byte
	binary.LittleEndian.PutUint32(data[0:4], id.X)
	binary.LittleEndian.PutUint32(data[4:8], id.Y)
	binary.LittleEndian.PutUint32(data[8:12], uint32(index))
	hash.Write(data[:])
	hash.Write([]byte(layer))
	return binary.LittleEndian.Uint64(hash.Sum(nil)[:8])
}

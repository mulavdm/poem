package cartography

import (
	"crypto/sha256"
	"errors"
	"math"

	"github.com/mulavdm/poem/pkg/render/protocol"
)

// SceneFeature is application-owned geographic geometry appended above base
// cartography. Coordinates are immutable longitude/latitude pairs.
type SceneFeature struct {
	ID          string
	Geometry    GeometryKind
	Coordinates [][2]float64
	Color       Color
	Width       float32
	Layer       int32
	Interactive bool
}

// AddSceneFeatures appends deterministic application overlays to a retained
// scene without rebuilding or copying its base-tile resources.
func AddSceneFeatures(scene Scene, features []SceneFeature) (Scene, error) {
	if len(features) > 1_000_000 {
		return Scene{}, errors.New("cartography: too many scene features")
	}
	for _, primitive := range []GeometryKind{GeometryPolygon, GeometryLine, GeometryPoint} {
		var vertices, indices []byte
		var vertexCount, indexCount uint32
		layer := int32(1_000_000)
		for _, feature := range features {
			if feature.Geometry != primitive {
				continue
			}
			if err := validateSceneFeature(feature); err != nil {
				return Scene{}, err
			}
			start := vertexCount
			featureID := stableSceneFeatureID(feature.ID)
			minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
			maxX, maxY := -minX, -minY
			for _, coordinate := range feature.Coordinates {
				x, y := worldPosition(coordinate[0], coordinate[1])
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
			switch primitive {
			case GeometryPoint:
				x, y := worldPosition(feature.Coordinates[0][0], feature.Coordinates[0][1])
				appendVertex(&vertices, x, y, 0, feature.Color, feature.Width, featureID)
				vertexCount++
				appendIndex(&indices, start)
				indexCount++
			case GeometryLine:
				if uint64(len(feature.Coordinates)-1)*4 > uint64(maxSceneVertices-vertexCount) || uint64(len(feature.Coordinates)-1)*6 > uint64(maxSceneIndices-indexCount) {
					return Scene{}, errors.New("cartography: feature line mesh exceeds geometry bounds")
				}
				ax, ay := worldPosition(feature.Coordinates[0][0], feature.Coordinates[0][1])
				for _, coordinate := range feature.Coordinates[1:] {
					bx, by := worldPosition(coordinate[0], coordinate[1])
					appendLineSegment(&vertices, &indices, &vertexCount, &indexCount, ax, ay, bx, by, feature.Color, feature.Width, featureID, float64(scene.Delta.Camera.Zoom))
					ax, ay = bx, by
				}
			case GeometryPolygon:
				for _, coordinate := range feature.Coordinates {
					x, y := worldPosition(coordinate[0], coordinate[1])
					appendVertex(&vertices, x, y, 0, feature.Color, feature.Width, featureID)
					vertexCount++
				}
				count := vertexCount - start
				for index := uint32(2); index < count; index++ {
					appendIndex(&indices, start)
					appendIndex(&indices, start+index-1)
					appendIndex(&indices, start+index)
					indexCount += 3
				}
			}
			if feature.Interactive {
				scene.Picks = append(scene.Picks, PickRecord{FeatureID: featureID, StableID: feature.ID, StyleLayer: "feature/" + feature.ID, MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY})
			}
			layer = max(layer, feature.Layer)
		}
		if vertexCount == 0 || indexCount == 0 {
			continue
		}
		if vertexCount > maxSceneVertices || indexCount > maxSceneIndices {
			return Scene{}, errors.New("cartography: feature scene exceeds geometry bounds")
		}
		vertexHash, indexHash := sha256.Sum256(vertices), sha256.Sum256(indices)
		scene.Delta.Resources = append(scene.Delta.Resources,
			protocol.MapSceneResource{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceVertexBuffer, Hash: vertexHash, Stride: vertexStride, Bytes: vertices},
			protocol.MapSceneResource{Operation: protocol.MapResourceUpload, Type: protocol.MapResourceIndexBuffer, Hash: indexHash, Stride: 4, Bytes: indices},
		)
		mapPrimitive := protocol.MapPrimitiveTriangles
		if primitive == GeometryPoint {
			mapPrimitive = protocol.MapPrimitivePoints
		}
		scene.Delta.Draws = append(scene.Delta.Draws, protocol.MapDrawBatch{VertexHash: vertexHash, IndexHash: indexHash, Primitive: mapPrimitive, Count: indexCount, Layer: layer, Opacity: 1})
	}
	return scene, nil
}

func validateSceneFeature(feature SceneFeature) error {
	if feature.ID == "" || len(feature.ID) > 1024 || feature.Geometry > GeometryPolygon || len(feature.Coordinates) == 0 || len(feature.Coordinates) > 1_000_000 || math.IsNaN(float64(feature.Width)) || math.IsInf(float64(feature.Width), 0) || feature.Width < 0 || feature.Width > 256 {
		return errors.New("cartography: invalid scene feature")
	}
	if feature.Geometry == GeometryPoint && len(feature.Coordinates) != 1 || feature.Geometry != GeometryPoint && len(feature.Coordinates) < 2 {
		return errors.New("cartography: invalid scene feature geometry")
	}
	for _, coordinate := range feature.Coordinates {
		if math.IsNaN(coordinate[0]) || math.IsInf(coordinate[0], 0) || math.IsNaN(coordinate[1]) || math.IsInf(coordinate[1], 0) || coordinate[0] < -180 || coordinate[0] > 180 || coordinate[1] < -85.05112878 || coordinate[1] > 85.05112878 {
			return errors.New("cartography: invalid scene coordinate")
		}
	}
	return nil
}

func stableSceneFeatureID(id string) uint64 {
	hash := sha256.Sum256([]byte(id))
	return uint64(hash[0]) | uint64(hash[1])<<8 | uint64(hash[2])<<16 | uint64(hash[3])<<24 | uint64(hash[4])<<32 | uint64(hash[5])<<40 | uint64(hash[6])<<48 | uint64(hash[7])<<56
}

func worldPosition(longitude, latitude float64) (float32, float32) {
	x := (longitude + 180) / 360
	sine := math.Sin(latitude * math.Pi / 180)
	y := .5 - math.Log((1+sine)/(1-sine))/(4*math.Pi)
	return float32(x), float32(y)
}

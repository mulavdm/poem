// Package cartography contains POEM's bounded, deterministic vector-map core.
// It has no platform or GPU dependencies and is safe to compile to WebAssembly.
package cartography

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
)

const (
	maxTileBytes   = 32 << 20
	maxLayers      = 128
	maxFeatures    = 200_000
	maxPoints      = 2_000_000
	maxDictionary  = 1 << 16
	maxStringBytes = 1 << 20
)

// GeometryKind is an MVT feature geometry type.
type GeometryKind uint8

const (
	GeometryUnknown GeometryKind = iota
	GeometryPoint
	GeometryLine
	GeometryPolygon
)

// Point is a coordinate in a layer's integer extent.
type Point struct{ X, Y int32 }

// Feature is one decoded immutable MVT feature.
type Feature struct {
	ID    uint64
	Kind  GeometryKind
	Tags  map[string]string
	Paths [][]Point
}

// Layer is one decoded MVT layer.
type Layer struct {
	Name     string
	Extent   uint32
	Features []Feature
}

// DecodeMVT decodes a Mapbox Vector Tile using strict allocation and geometry
// bounds. Unknown protobuf fields are skipped; malformed known fields fail.
func DecodeMVT(data []byte) ([]Layer, error) {
	if len(data) == 0 || len(data) > maxTileBytes {
		return nil, errors.New("mvt: invalid tile size")
	}
	var layers []Layer
	for len(data) > 0 {
		field, wire, rest, err := protobufTag(data)
		if err != nil {
			return nil, err
		}
		data = rest
		if field == 3 && wire == 2 {
			body, tail, err := protobufBytes(data)
			if err != nil {
				return nil, err
			}
			layer, err := decodeLayer(body)
			if err != nil {
				return nil, err
			}
			layers = append(layers, layer)
			if len(layers) > maxLayers {
				return nil, errors.New("mvt: too many layers")
			}
			data = tail
			continue
		}
		data, err = protobufSkip(data, wire)
		if err != nil {
			return nil, err
		}
	}
	return layers, nil
}

func decodeLayer(data []byte) (Layer, error) {
	layer := Layer{Extent: 4096}
	var features [][]byte
	var keys, values []string
	for len(data) > 0 {
		field, wire, rest, err := protobufTag(data)
		if err != nil {
			return layer, err
		}
		data = rest
		switch {
		case field == 1 && wire == 2:
			body, tail, err := protobufBytes(data)
			if err != nil || len(body) == 0 || len(body) > maxStringBytes {
				return layer, errors.New("mvt: invalid layer name")
			}
			layer.Name, data = string(body), tail
		case field == 2 && wire == 2:
			body, tail, err := protobufBytes(data)
			if err != nil {
				return layer, err
			}
			features, data = append(features, body), tail
			if len(features) > maxFeatures {
				return layer, errors.New("mvt: too many features")
			}
		case field == 3 && wire == 2:
			body, tail, err := protobufBytes(data)
			if err != nil || len(body) > maxStringBytes || len(keys) >= maxDictionary {
				return layer, errors.New("mvt: invalid key dictionary")
			}
			keys, data = append(keys, string(body)), tail
		case field == 4 && wire == 2:
			body, tail, err := protobufBytes(data)
			if err != nil || len(values) >= maxDictionary {
				return layer, errors.New("mvt: invalid value dictionary")
			}
			value, err := decodeValue(body)
			if err != nil || len(value) > maxStringBytes {
				return layer, errors.New("mvt: invalid value")
			}
			values, data = append(values, value), tail
		case field == 5 && wire == 0:
			value, tail, err := protobufVarint(data)
			if err != nil || value == 0 || value > 1<<20 {
				return layer, errors.New("mvt: invalid extent")
			}
			layer.Extent, data = uint32(value), tail
		default:
			data, err = protobufSkip(data, wire)
			if err != nil {
				return layer, err
			}
		}
	}
	if layer.Name == "" {
		return layer, errors.New("mvt: missing layer name")
	}
	for _, body := range features {
		feature, err := decodeFeature(body, keys, values)
		if err != nil {
			return layer, err
		}
		layer.Features = append(layer.Features, feature)
	}
	return layer, nil
}

func decodeFeature(data []byte, keys, values []string) (Feature, error) {
	var feature Feature
	var tags, geometry []uint64
	for len(data) > 0 {
		field, wire, rest, err := protobufTag(data)
		if err != nil {
			return feature, err
		}
		data = rest
		switch {
		case field == 1 && wire == 0:
			feature.ID, data, err = protobufVarint(data)
		case field == 2 && wire == 2:
			var body []byte
			body, data, err = protobufBytes(data)
			if err == nil {
				tags, err = packedVarints(body, maxDictionary)
			}
		case field == 3 && wire == 0:
			var kind uint64
			kind, data, err = protobufVarint(data)
			if kind > uint64(GeometryPolygon) {
				err = errors.New("mvt: invalid geometry kind")
			}
			feature.Kind = GeometryKind(kind)
		case field == 4 && wire == 2:
			var body []byte
			body, data, err = protobufBytes(data)
			if err == nil {
				geometry, err = packedVarints(body, maxPoints*2)
			}
		default:
			data, err = protobufSkip(data, wire)
		}
		if err != nil {
			return feature, err
		}
	}
	if feature.Kind == GeometryUnknown || len(tags)%2 != 0 {
		return feature, errors.New("mvt: invalid feature")
	}
	feature.Tags = make(map[string]string, len(tags)/2)
	for i := 0; i < len(tags); i += 2 {
		if tags[i] >= uint64(len(keys)) || tags[i+1] >= uint64(len(values)) {
			return feature, errors.New("mvt: tag index out of range")
		}
		feature.Tags[keys[tags[i]]] = values[tags[i+1]]
	}
	var err error
	feature.Paths, err = decodeGeometry(geometry)
	return feature, err
}

func decodeGeometry(commands []uint64) ([][]Point, error) {
	var paths [][]Point
	var x, y int64
	points := 0
	for index := 0; index < len(commands); {
		command := commands[index]
		index++
		id, count := command&7, int(command>>3)
		if count <= 0 {
			return nil, errors.New("mvt: empty geometry command")
		}
		switch id {
		case 1:
			for range count {
				if index+1 >= len(commands) {
					return nil, errors.New("mvt: truncated moveto")
				}
				x += zigzag(commands[index])
				y += zigzag(commands[index+1])
				index += 2
				if x < math.MinInt32 || x > math.MaxInt32 || y < math.MinInt32 || y > math.MaxInt32 {
					return nil, errors.New("mvt: coordinate overflow")
				}
				paths = append(paths, []Point{{int32(x), int32(y)}})
				points++
			}
		case 2:
			if len(paths) == 0 {
				return nil, errors.New("mvt: lineto before moveto")
			}
			for range count {
				if index+1 >= len(commands) {
					return nil, errors.New("mvt: truncated lineto")
				}
				x += zigzag(commands[index])
				y += zigzag(commands[index+1])
				index += 2
				if x < math.MinInt32 || x > math.MaxInt32 || y < math.MinInt32 || y > math.MaxInt32 {
					return nil, errors.New("mvt: coordinate overflow")
				}
				paths[len(paths)-1] = append(paths[len(paths)-1], Point{int32(x), int32(y)})
				points++
			}
		case 7:
			if len(paths) == 0 || count != 1 {
				return nil, errors.New("mvt: invalid closepath")
			}
			path := paths[len(paths)-1]
			if len(path) > 0 && path[len(path)-1] != path[0] {
				paths[len(paths)-1] = append(path, path[0])
			}
		default:
			return nil, fmt.Errorf("mvt: unknown geometry command %d", id)
		}
		if points > maxPoints {
			return nil, errors.New("mvt: too many geometry points")
		}
	}
	return paths, nil
}

func decodeValue(data []byte) (string, error) {
	for len(data) > 0 {
		field, wire, rest, err := protobufTag(data)
		if err != nil {
			return "", err
		}
		data = rest
		switch {
		case field == 1 && wire == 2:
			body, _, err := protobufBytes(data)
			return string(body), err
		case field == 2 && wire == 5:
			if len(data) < 4 {
				return "", errors.New("mvt: truncated float")
			}
			return strconv.FormatFloat(float64(math.Float32frombits(binary.LittleEndian.Uint32(data))), 'g', -1, 32), nil
		case field == 3 && wire == 1:
			if len(data) < 8 {
				return "", errors.New("mvt: truncated double")
			}
			return strconv.FormatFloat(math.Float64frombits(binary.LittleEndian.Uint64(data)), 'g', -1, 64), nil
		case (field == 4 || field == 5 || field == 6 || field == 7) && wire == 0:
			value, _, err := protobufVarint(data)
			if err != nil {
				return "", err
			}
			if field == 6 {
				return strconv.FormatInt(zigzag(value), 10), nil
			}
			if field == 7 {
				return strconv.FormatBool(value != 0), nil
			}
			return strconv.FormatUint(value, 10), nil
		default:
			data, err = protobufSkip(data, wire)
			if err != nil {
				return "", err
			}
		}
	}
	return "", nil
}

func protobufTag(data []byte) (uint64, byte, []byte, error) {
	tag, rest, err := protobufVarint(data)
	if err != nil {
		return 0, 0, nil, err
	}
	if tag>>3 == 0 {
		return 0, 0, nil, errors.New("mvt: zero protobuf field")
	}
	return tag >> 3, byte(tag & 7), rest, nil
}

func protobufVarint(data []byte) (uint64, []byte, error) {
	var value uint64
	for i := 0; i < 10 && i < len(data); i++ {
		b := data[i]
		if i == 9 && b > 1 {
			return 0, nil, errors.New("mvt: varint overflow")
		}
		value |= uint64(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return value, data[i+1:], nil
		}
	}
	return 0, nil, errors.New("mvt: truncated varint")
}

func protobufBytes(data []byte) ([]byte, []byte, error) {
	length, rest, err := protobufVarint(data)
	if err != nil || length > uint64(len(rest)) {
		return nil, nil, errors.New("mvt: truncated bytes")
	}
	return rest[:int(length)], rest[int(length):], nil
}

func protobufSkip(data []byte, wire byte) ([]byte, error) {
	switch wire {
	case 0:
		_, rest, err := protobufVarint(data)
		return rest, err
	case 1:
		if len(data) < 8 {
			return nil, errors.New("mvt: truncated fixed64")
		}
		return data[8:], nil
	case 2:
		_, rest, err := protobufBytes(data)
		return rest, err
	case 5:
		if len(data) < 4 {
			return nil, errors.New("mvt: truncated fixed32")
		}
		return data[4:], nil
	default:
		return nil, fmt.Errorf("mvt: unsupported wire type %d", wire)
	}
}

func packedVarints(data []byte, limit int) ([]uint64, error) {
	values := make([]uint64, 0)
	for len(data) > 0 {
		value, rest, err := protobufVarint(data)
		if err != nil {
			return nil, err
		}
		values, data = append(values, value), rest
		if len(values) > limit {
			return nil, errors.New("mvt: packed field exceeds limit")
		}
	}
	return values, nil
}

func zigzag(value uint64) int64 { return int64(value>>1) ^ -int64(value&1) }

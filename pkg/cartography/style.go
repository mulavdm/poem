package cartography

import (
	"errors"
	"math"
)

// Color is a linear render color encoded as unpremultiplied RGBA bytes.
type Color struct{ R, G, B, A uint8 }

// FilterOp is a bounded typed feature-filter operation.
type FilterOp uint8

const (
	FilterExists FilterOp = iota
	FilterEqual
	FilterNotEqual
)

// Filter matches one MVT tag without evaluating application-provided code.
type Filter struct {
	Key, Value string
	Operation  FilterOp
}

// ZoomStop gives a scalar value at one zoom. Stops interpolate linearly.
type ZoomStop struct{ Zoom, Value float64 }

// StyleLayer describes one ordered vector style layer.
type StyleLayer struct {
	ID, SourceLayer  string
	Geometry         GeometryKind
	Filters          []Filter
	MinZoom, MaxZoom float64
	Color            Color
	Width            []ZoomStop
	Extrude          bool
	Interactive      bool
	FilterPOI        bool
	POICategories    POICategories
}

// Style is POEM's validated typed map style. It deliberately does not accept
// shaders or MapLibre Style JSON.
type Style struct{ Layers []StyleLayer }

// Validate checks all style bounds before external tile data is evaluated.
func (style Style) Validate() error {
	if len(style.Layers) == 0 || len(style.Layers) > 256 {
		return errors.New("cartography: invalid style layer count")
	}
	ids := make(map[string]struct{}, len(style.Layers))
	for _, layer := range style.Layers {
		if layer.ID == "" || len(layer.ID) > 256 || layer.SourceLayer == "" || len(layer.SourceLayer) > 256 || layer.Geometry < GeometryPoint || layer.Geometry > GeometryPolygon || len(layer.Filters) > 32 || len(layer.Width) > 32 || layer.POICategories&^POIAll != 0 {
			return errors.New("cartography: invalid style layer")
		}
		if _, exists := ids[layer.ID]; exists {
			return errors.New("cartography: duplicate style layer")
		}
		ids[layer.ID] = struct{}{}
		if !finite(layer.MinZoom) || !finite(layer.MaxZoom) || layer.MinZoom < 0 || layer.MaxZoom > 24 || layer.MaxZoom < layer.MinZoom {
			return errors.New("cartography: invalid style zoom range")
		}
		previous := -1.0
		for _, stop := range layer.Width {
			if !finite(stop.Zoom) || !finite(stop.Value) || stop.Zoom < 0 || stop.Zoom > 24 || stop.Zoom <= previous || stop.Value < 0 || stop.Value > 1024 {
				return errors.New("cartography: invalid zoom stop")
			}
			previous = stop.Zoom
		}
		for _, filter := range layer.Filters {
			if filter.Key == "" || len(filter.Key) > 256 || len(filter.Value) > 4096 || filter.Operation > FilterNotEqual {
				return errors.New("cartography: invalid filter")
			}
		}
	}
	return nil
}

func (layer StyleLayer) matches(feature Feature, zoom float64) bool {
	if feature.Kind != layer.Geometry || zoom < layer.MinZoom || zoom > layer.MaxZoom {
		return false
	}
	if layer.FilterPOI && layer.POICategories&classifyPOI(feature.Tags) == 0 {
		return false
	}
	for _, filter := range layer.Filters {
		value, exists := feature.Tags[filter.Key]
		switch filter.Operation {
		case FilterExists:
			if !exists {
				return false
			}
		case FilterEqual:
			if !exists || value != filter.Value {
				return false
			}
		case FilterNotEqual:
			if exists && value == filter.Value {
				return false
			}
		}
	}
	return true
}

func (layer StyleLayer) width(zoom float64) float32 {
	if len(layer.Width) == 0 {
		return 1
	}
	if zoom <= layer.Width[0].Zoom {
		return float32(layer.Width[0].Value)
	}
	for i := 1; i < len(layer.Width); i++ {
		if zoom <= layer.Width[i].Zoom {
			left, right := layer.Width[i-1], layer.Width[i]
			t := (zoom - left.Zoom) / (right.Zoom - left.Zoom)
			return float32(left.Value + t*(right.Value-left.Value))
		}
	}
	return float32(layer.Width[len(layer.Width)-1].Value)
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

package cartography

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// POICategories is a bit set of portable point-of-interest categories.
type POICategories uint32

const (
	// POITransport includes public transport and mobility features.
	POITransport POICategories = 1 << iota
	// POIParkingFuel includes parking, fuel, and charging features.
	POIParkingFuel
	// POIFoodDrink includes food and drink features.
	POIFoodDrink
	// POIHealth includes health and emergency-care features.
	POIHealth
	// POIShoppingServices includes shops and everyday services.
	POIShoppingServices
	// POILeisureTourism includes leisure, lodging, and tourism features.
	POILeisureTourism
	// POICivic includes education, government, and civic features.
	POICivic
	// POIAll includes every portable category.
	POIAll = POITransport | POIParkingFuel | POIFoodDrink | POIHealth | POIShoppingServices | POILeisureTourism | POICivic
)

// LabelWeight is a portable cartographic emphasis tier. It is rendered from
// the same pinned face with deterministic bounded emboldening.
type LabelWeight uint8

const (
	LabelWeightRegular LabelWeight = iota
	LabelWeightMedium
	LabelWeightSemibold
)

// LabelRule is the bounded typed-style subset that converts tile features
// into immutable label candidates. TextKeys are tried in order.
type LabelRule struct {
	ID, SourceLayer string
	Geometry        GeometryKind
	TextKeys        []string
	Locale          string
	MinZoom         float64
	MaxZoom         float64
	Size            []ZoomStop
	Priority        int32
	Weight          LabelWeight
	Filters         []Filter
	AllowOverlap    bool
	FilterPOI       bool
	POICategories   POICategories
}

func BuildLabelCandidates(camera Camera, rules []LabelRule, tiles []Tile) ([]LabelCandidate, error) {
	if err := camera.Validate(); err != nil {
		return nil, err
	}
	if len(rules) == 0 || len(rules) > 256 {
		return nil, errors.New("cartography: invalid label rules")
	}
	for _, rule := range rules {
		if rule.ID == "" || len(rule.ID) > 256 || rule.SourceLayer == "" || len(rule.SourceLayer) > 256 || rule.Geometry < GeometryPoint || rule.Geometry > GeometryPolygon || len(rule.TextKeys) == 0 || len(rule.TextKeys) > 8 || len(rule.Locale) > 128 || !finite(rule.MinZoom) || !finite(rule.MaxZoom) || rule.MinZoom < 0 || rule.MaxZoom > 24 || rule.MaxZoom < rule.MinZoom || len(rule.Size) == 0 || len(rule.Size) > 32 || len(rule.Filters) > 32 || rule.Weight > LabelWeightSemibold || rule.POICategories&^POIAll != 0 {
			return nil, errors.New("cartography: invalid label rule")
		}
		for _, key := range rule.TextKeys {
			if key == "" || len(key) > 256 {
				return nil, errors.New("cartography: invalid label text key")
			}
		}
		for _, filter := range rule.Filters {
			if filter.Key == "" || len(filter.Key) > 256 || len(filter.Value) > 4096 || filter.Operation > FilterNotEqual {
				return nil, errors.New("cartography: invalid label filter")
			}
		}
		if _, err := interpolateStops(rule.Size, camera.Zoom, 6, 256); err != nil {
			return nil, err
		}
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
	candidates := make([]LabelCandidate, 0)
	for _, rule := range rules {
		if camera.Zoom < rule.MinZoom || camera.Zoom > rule.MaxZoom {
			continue
		}
		size, _ := interpolateStops(rule.Size, camera.Zoom, 6, 256)
		for _, tile := range ordered {
			for _, layer := range tile.Layers {
				if layer.Name != rule.SourceLayer || layer.Extent == 0 {
					continue
				}
				for featureIndex, feature := range layer.Features {
					if feature.Kind != rule.Geometry {
						continue
					}
					if !filtersMatch(feature.Tags, rule.Filters) {
						continue
					}
					if rule.FilterPOI && rule.POICategories&classifyPOI(feature.Tags) == 0 {
						continue
					}
					text := ""
					for _, key := range rule.TextKeys {
						if value := feature.Tags[key]; value != "" {
							text = value
							break
						}
					}
					if text == "" {
						continue
					}
					point, ok := labelAnchor(feature)
					if !ok {
						continue
					}
					x, y := tilePosition(tile.ID, layer.Extent, point)
					featureID := feature.ID
					if featureID == 0 {
						featureID = syntheticFeatureID(tile.ID, layer.Name, featureIndex)
					}
					candidates = append(candidates, LabelCandidate{ID: fmt.Sprintf("%s/%d/%d/%d/%d", rule.ID, tile.ID.Z, tile.ID.X, tile.ID.Y, featureID), Text: text, Locale: rule.Locale, WorldX: float64(x), WorldY: float64(y), Size: size, Priority: rule.Priority, Weight: rule.Weight, AllowOverlap: rule.AllowOverlap})
					if len(candidates) > maxLabelCandidates {
						return nil, errors.New("cartography: label candidates exceed bounds")
					}
				}
			}
		}
	}
	return candidates, nil
}

func classifyPOI(tags map[string]string) POICategories {
	values := []string{strings.ToLower(tags["class"]), strings.ToLower(tags["subclass"]), strings.ToLower(tags["amenity"]), strings.ToLower(tags["shop"]), strings.ToLower(tags["tourism"]), strings.ToLower(tags["leisure"])}
	for _, value := range values {
		switch value {
		case "transport", "railway", "rail", "station", "bus", "bus_stop", "tram_stop", "subway_entrance", "airport", "aerodrome", "ferry", "ferry_terminal", "bicycle_rental":
			return POITransport
		case "parking", "parking_entrance", "fuel", "charging_station", "car_rental", "car_sharing":
			return POIParkingFuel
		case "food", "restaurant", "cafe", "fast_food", "bar", "pub", "biergarten", "ice_cream":
			return POIFoodDrink
		case "health", "hospital", "clinic", "doctors", "pharmacy", "dentist", "veterinary":
			return POIHealth
		case "shop", "shopping", "bank", "atm", "post_office", "marketplace", "supermarket", "convenience", "mall":
			return POIShoppingServices
		case "leisure", "tourism", "attraction", "museum", "gallery", "hotel", "hostel", "camp_site", "park", "stadium", "sports_centre", "zoo", "theme_park":
			return POILeisureTourism
		case "civic", "school", "college", "university", "library", "town_hall", "courthouse", "police", "fire_station", "community_centre", "place_of_worship":
			return POICivic
		}
	}
	return 0
}

func interpolateStops(stops []ZoomStop, zoom, minimum, maximum float64) (float64, error) {
	previous := -1.0
	for _, stop := range stops {
		if !finite(stop.Zoom) || !finite(stop.Value) || stop.Zoom < 0 || stop.Zoom > 24 || stop.Zoom <= previous || stop.Value < minimum || stop.Value > maximum {
			return 0, errors.New("cartography: invalid label size stops")
		}
		previous = stop.Zoom
	}
	if zoom <= stops[0].Zoom {
		return stops[0].Value, nil
	}
	for index := 1; index < len(stops); index++ {
		if zoom <= stops[index].Zoom {
			left, right := stops[index-1], stops[index]
			factor := (zoom - left.Zoom) / (right.Zoom - left.Zoom)
			return left.Value + factor*(right.Value-left.Value), nil
		}
	}
	return stops[len(stops)-1].Value, nil
}

func labelAnchor(feature Feature) (Point, bool) {
	if len(feature.Paths) == 0 {
		return Point{}, false
	}
	path := feature.Paths[0]
	if len(path) == 0 {
		return Point{}, false
	}
	if feature.Kind == GeometryPoint {
		return path[0], true
	}
	if feature.Kind == GeometryLine {
		return path[len(path)/2], true
	}
	var x, y int64
	count := len(path)
	if count > 1 && path[0] == path[count-1] {
		count--
	}
	if count == 0 {
		return Point{}, false
	}
	for _, point := range path[:count] {
		x += int64(point.X)
		y += int64(point.Y)
	}
	return Point{X: int32(x / int64(count)), Y: int32(y / int64(count))}, true
}

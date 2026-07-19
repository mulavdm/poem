package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
)

// MapCamera is the controlled camera of a MapViewportNode. Bearing is degrees
// clockwise from north; Pitch is degrees away from a straight-down view.
type MapCamera struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Zoom      float64 `json:"zoom"`
	Bearing   float64 `json:"bearing"`
	Pitch     float64 `json:"pitch"`
}

func (c MapCamera) Normalized() MapCamera {
	if c.Bearing != 0 {
		c.Bearing = math.Mod(c.Bearing, 360)
		if c.Bearing < 0 {
			c.Bearing += 360
		}
	}
	return c
}

func (c MapCamera) Valid() bool {
	c = c.Normalized()
	values := [...]float64{c.Latitude, c.Longitude, c.Zoom, c.Bearing, c.Pitch}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return c.Latitude >= -85.05112878 && c.Latitude <= 85.05112878 &&
		c.Longitude >= -180 && c.Longitude <= 180 && c.Zoom >= 0 && c.Zoom <= 24 &&
		c.Bearing >= 0 && c.Bearing < 360 && c.Pitch >= 0 && c.Pitch <= 85
}

func MapCameraPayload(camera MapCamera) string {
	camera = camera.Normalized()
	if !camera.Valid() {
		return ""
	}
	body, err := json.Marshal(camera)
	if err != nil {
		return ""
	}
	return string(body)
}

func (m Msg) MapCamera() (MapCamera, bool) {
	var camera MapCamera
	decoder := json.NewDecoder(strings.NewReader(m.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&camera); err != nil {
		return MapCamera{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return MapCamera{}, false
	}
	camera = camera.Normalized()
	return camera, camera.Valid()
}

type MapBounds struct {
	West, South, East, North float64
}

func (b MapBounds) Valid() bool {
	values := [...]float64{b.West, b.South, b.East, b.North}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return b.West >= -180 && b.East <= 180 && b.South >= -85.05112878 && b.North <= 85.05112878 && b.West < b.East && b.South < b.North
}

type MapQuality uint8

const (
	MapQualityAuto MapQuality = iota
	MapQualityBatterySaver
	MapQualityBalanced
	MapQualityHigh
)

type MapCachePolicy uint8

const (
	MapCacheMemoryOnly MapCachePolicy = iota
	MapCachePersistentOnlineOnly
	MapCachePersistentOffline
)

type MapResourceKind uint8

const (
	MapResourceVectorTile MapResourceKind = iota
	MapResourceElevationTile
	MapResourceFont
	MapResourceIcon
)

// MapSource names a provider registered by App.MapResources. It deliberately
// contains no URL or authorization header.
type MapSource struct {
	ID          string
	ProviderID  string
	Snapshot    string
	MinZoom     float64
	MaxZoom     float64
	Bounds      MapBounds
	Attribution string
	Elevation   bool
}

func (s MapSource) Valid() bool {
	return validMapIdentifier(s.ID) && validMapIdentifier(s.ProviderID) && len(s.Snapshot) <= 256 &&
		s.MinZoom >= 0 && s.MaxZoom >= s.MinZoom && s.MaxZoom <= 24 &&
		(s.Bounds == (MapBounds{}) || s.Bounds.Valid()) && len(s.Attribution) <= 4096
}

type MapResourceRequest struct {
	SourceID string
	Snapshot string
	Kind     MapResourceKind
	Z, X, Y  int
	Name     string
}

func (r MapResourceRequest) Valid() bool {
	if !validMapIdentifier(r.SourceID) || len(r.Snapshot) > 256 || len(r.Name) > 256 || r.Kind > MapResourceIcon {
		return false
	}
	if r.Kind == MapResourceVectorTile || r.Kind == MapResourceElevationTile {
		if r.Z < 0 || r.Z > 24 {
			return false
		}
		limit := 1 << r.Z
		return r.X >= 0 && r.X < limit && r.Y >= 0 && r.Y < limit
	}
	return r.Name != ""
}

type MapResource struct {
	Bytes       []byte
	ContentType string
	ETag        string
	Immutable   bool
}

const MaxMapResourceBytes = 16 << 20

func (r MapResource) Valid() bool {
	return len(r.Bytes) > 0 && len(r.Bytes) <= MaxMapResourceBytes && len(r.ContentType) <= 128 && len(r.ETag) <= 256
}

type MapResourceProvider struct {
	ID    string
	Fetch func(context.Context, MapResourceRequest) (MapResource, error)
}

func (p MapResourceProvider) Valid() bool { return validMapIdentifier(p.ID) && p.Fetch != nil }

var ErrMapResourceRejected = errors.New("poem: map resource rejected")

type MapGeometry uint8

const (
	MapGeometryPoint MapGeometry = iota
	MapGeometryLine
	MapGeometryPolygon
)

type MapPosition struct {
	Latitude, Longitude, Altitude float64
}

func (p MapPosition) Valid() bool {
	return (MapCamera{Latitude: p.Latitude, Longitude: p.Longitude}).Valid() && !math.IsNaN(p.Altitude) && !math.IsInf(p.Altitude, 0)
}

type MapFeatureRole uint8

const (
	MapFeatureGeneric MapFeatureRole = iota
	MapFeatureRoute
	MapFeatureMarker
	MapFeaturePOI
	MapFeatureTraffic
)

type MapFeature struct {
	ID          string
	Name        string
	Description string
	Geometry    MapGeometry
	Role        MapFeatureRole
	Positions   []MapPosition
	Selected    bool
	Disabled    bool
	OnActivate  Msg
}

// MapFeatureActivation is the bounded browser-safe description emitted when
// a retained vector feature is selected.
type MapFeatureActivation struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Category    string  `json:"category,omitempty"`
	SourceLayer string  `json:"source_layer"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

// Valid reports whether an activation is safe to dispatch to an application.
func (activation MapFeatureActivation) Valid() bool {
	if activation.ID == "" || len(activation.ID) > 512 || !safeMapActivationText(activation.ID) || len(activation.Name) > 512 || !safeMapActivationText(activation.Name) || len(activation.Description) > 4096 || !safeMapActivationText(activation.Description) || (activation.Category != "" && !validMapIdentifier(activation.Category)) || !validMapIdentifier(activation.SourceLayer) {
		return false
	}
	return (MapPosition{Latitude: activation.Latitude, Longitude: activation.Longitude}).Valid()
}

func safeMapActivationText(value string) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

// MapFeatureActivationPayload validates and encodes a vector activation.
func MapFeatureActivationPayload(activation MapFeatureActivation) string {
	if !activation.Valid() {
		return ""
	}
	body, err := json.Marshal(activation)
	if err != nil {
		return ""
	}
	return string(body)
}

// MapFeatureActivation strictly decodes a vector activation payload.
func (m Msg) MapFeatureActivation() (MapFeatureActivation, bool) {
	if len(m.Payload) == 0 || len(m.Payload) > 8<<10 {
		return MapFeatureActivation{}, false
	}
	decoder := json.NewDecoder(strings.NewReader(m.Payload))
	decoder.DisallowUnknownFields()
	var activation MapFeatureActivation
	if err := decoder.Decode(&activation); err != nil {
		return MapFeatureActivation{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF || !activation.Valid() {
		return MapFeatureActivation{}, false
	}
	return activation, true
}

func (f MapFeature) Valid() bool {
	if !validMapIdentifier(f.ID) || f.Geometry > MapGeometryPolygon || f.Role > MapFeatureTraffic || len(f.Name) > 512 || len(f.Description) > 4096 || len(f.Positions) == 0 || len(f.Positions) > 1_000_000 {
		return false
	}
	if f.Geometry == MapGeometryPoint && len(f.Positions) != 1 || f.Geometry != MapGeometryPoint && len(f.Positions) < 2 {
		return false
	}
	for _, position := range f.Positions {
		if !position.Valid() {
			return false
		}
	}
	return true
}

type MapSemanticColors struct {
	Land, Water, Road, Building, Label, Route, Traffic string
}

type MapStyle struct {
	ID              string
	Revision        string
	Colors          MapSemanticColors
	LabelDensity    float64
	TerrainScale    float64
	BuildingHeights bool
	DynamicSun      bool
	Locale          string
	POIFilters      MapPOIFilters
}

func (s MapStyle) Valid() bool {
	if !validMapIdentifier(s.ID) || len(s.Revision) > 128 || !finiteRange(s.LabelDensity, 0, 4) || !finiteRange(s.TerrainScale, 0, 8) || !validLocale(s.Locale) || s.POIFilters&^allPOIFilters != 0 {
		return false
	}
	values := [...]string{s.Colors.Land, s.Colors.Water, s.Colors.Road, s.Colors.Building, s.Colors.Label, s.Colors.Route, s.Colors.Traffic}
	for _, value := range values {
		if !validMapSemanticColor(value) {
			return false
		}
	}
	return true
}

type MapStyleSet struct {
	House, Windows, Android, Web MapStyle
}

func (s MapStyleSet) valid() bool {
	styles := [...]MapStyle{s.House, s.Windows, s.Android, s.Web}
	for _, style := range styles {
		if style == (MapStyle{}) {
			continue
		}
		if !style.Valid() {
			return false
		}
	}
	return true
}

// MapViewportNode is POEM's controlled, semantic vector-map canvas. Fallback
// is used by no-JavaScript web, unsupported GPUs, export, and recovery paths.
type MapViewportNode struct {
	Semantic       Semantic
	Source         MapSource
	Camera         MapCamera
	Style          MapStyleSet
	Features       []MapFeature
	Fallback       ImageNode
	Quality        MapQuality
	CachePolicy    MapCachePolicy
	CacheBytes     int64
	MinZoom        float64
	MaxZoom        float64
	MinPitch       float64
	MaxPitch       float64
	OnCameraChange Msg
	OnFeature      Msg
}

func (MapViewportNode) isNode() {}

func (n MapViewportNode) Valid() bool {
	if n.Semantic.ID == "" || !n.Source.Valid() || !n.Camera.Valid() || !n.Style.valid() || n.Quality > MapQualityHigh || n.CachePolicy > MapCachePersistentOffline || n.CacheBytes < 0 || n.CacheBytes > 64<<30 {
		return false
	}
	if n.MinZoom < 0 || n.MaxZoom < n.MinZoom || n.MaxZoom > 24 || n.MinPitch < 0 || n.MaxPitch < n.MinPitch || n.MaxPitch > 85 {
		return false
	}
	for _, feature := range n.Features {
		if !feature.Valid() {
			return false
		}
	}
	return true
}

func validMapSemanticColor(value string) bool {
	if value == "" {
		return true
	}
	if value[0] == '#' {
		if len(value) != 7 && len(value) != 9 {
			return false
		}
		for _, character := range value[1:] {
			if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f' || character >= 'A' && character <= 'F') {
				return false
			}
		}
		return true
	}
	switch value {
	case "land", "water", "road", "building", "label", "route", "traffic", "marker", "surface", "surface-muted", "sunken", "surface-raised", "raised", "text", "muted", "line", "border", "primary", "accent", "success", "warning", "danger":
		return true
	default:
		return false
	}
}

func validMapIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

func finiteRange(value, min, max float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= min && value <= max
}

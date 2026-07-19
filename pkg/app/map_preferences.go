package app

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// MapPreferencesVersion is the current durable device-preference schema.
const MapPreferencesVersion = 1

// RegionalUnits selects the unit system used for map and guidance measurements.
type RegionalUnits uint8

const (
	// UnitsSystemDefault follows the device locale and regional preferences.
	UnitsSystemDefault RegionalUnits = iota
	// UnitsMetric displays metric distances and speeds.
	UnitsMetric
	// UnitsImperial displays imperial distances and speeds.
	UnitsImperial
)

// MapPOIFilters is a bit set of visible point-of-interest categories.
type MapPOIFilters uint32

const (
	// POITransport includes public transport and mobility places.
	POITransport MapPOIFilters = 1 << iota
	// POIParkingFuel includes parking and fuel places.
	POIParkingFuel
	// POIFoodDrink includes food and drink places.
	POIFoodDrink
	// POIHealth includes health and emergency places.
	POIHealth
	// POIShoppingServices includes shops and services.
	POIShoppingServices
	// POILeisureTourism includes leisure and tourism places.
	POILeisureTourism
	// POICivic includes civic and public-service places.
	POICivic
	allPOIFilters = POITransport | POIParkingFuel | POIFoodDrink | POIHealth | POIShoppingServices | POILeisureTourism | POICivic
)

// GuidancePreferences contains non-secret per-device navigation choices.
type GuidancePreferences struct {
	// Voice enables spoken guidance.
	Voice bool `json:"voice"`
	// Haptics enables supported guidance haptics.
	Haptics bool `json:"haptics"`
	// SpeedWarnings enables non-authoritative speed warnings.
	SpeedWarnings bool `json:"speed_warnings"`
}

// MapPreferences is the versioned, non-secret device map-preference record.
type MapPreferences struct {
	// Version identifies the durable preference schema.
	Version uint16 `json:"version"`
	// Quality requests the map rendering quality tier.
	Quality MapQuality `json:"quality"`
	// CachePolicy controls whether verified raw resources may persist.
	CachePolicy MapCachePolicy `json:"cache_policy"`
	// CacheBytes bounds ordinary map resource storage.
	CacheBytes int64 `json:"cache_bytes"`
	// Locale optionally overrides the system locale using a BCP-47-like tag.
	Locale string `json:"locale"`
	// Units optionally overrides the system regional unit convention.
	Units RegionalUnits `json:"units"`
	// POIFilters selects visible POI categories.
	POIFilters MapPOIFilters `json:"poi_filters"`
	// Guidance stores navigation presentation preferences.
	Guidance GuidancePreferences `json:"guidance"`
	// Camera stores the last settled camera; zero means no saved camera.
	Camera MapCamera `json:"camera"`
}

// DefaultMapPreferences returns safe platform-neutral defaults.
func DefaultMapPreferences() MapPreferences {
	return MapPreferences{Version: MapPreferencesVersion, Quality: MapQualityAuto, CachePolicy: MapCachePersistentOnlineOnly, CacheBytes: 2 << 30, Units: UnitsSystemDefault, POIFilters: allPOIFilters}
}

// Valid reports whether preferences satisfy the current durable schema.
func (preferences MapPreferences) Valid() bool {
	return preferences.Version == MapPreferencesVersion && preferences.Quality <= MapQualityHigh && preferences.CachePolicy <= MapCachePersistentOffline &&
		preferences.CacheBytes >= 0 && preferences.CacheBytes <= 64<<30 && preferences.Units <= UnitsImperial && preferences.POIFilters&^allPOIFilters == 0 &&
		validLocale(preferences.Locale) && (preferences.Camera == (MapCamera{}) || preferences.Camera.Valid())
}

// MarshalMapPreferences validates and encodes a durable preference record.
func MarshalMapPreferences(preferences MapPreferences) ([]byte, error) {
	if !preferences.Valid() {
		return nil, errors.New("poem: invalid map preferences")
	}
	return json.Marshal(preferences)
}

// ParseMapPreferences strictly decodes one bounded durable preference record.
func ParseMapPreferences(data []byte) (MapPreferences, error) {
	if len(data) == 0 || len(data) > 64<<10 {
		return MapPreferences{}, errors.New("poem: invalid map preferences")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var preferences MapPreferences
	if err := decoder.Decode(&preferences); err != nil {
		return MapPreferences{}, errors.New("poem: invalid map preferences")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF || !preferences.Valid() {
		return MapPreferences{}, errors.New("poem: invalid map preferences")
	}
	return preferences, nil
}

func validLocale(locale string) bool {
	if len(locale) > 64 {
		return false
	}
	for _, value := range locale {
		if !(value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '-') {
			return false
		}
	}
	return true
}

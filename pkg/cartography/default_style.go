package cartography

// SemanticPalette is the concrete color input for POEM's bounded semantic
// base style. Applications resolve token IDs before scene construction.
type SemanticPalette struct {
	// Land colors ordinary landcover polygons.
	Land Color
	// Water colors water polygons.
	Water Color
	// Waterway colors linear waterways.
	Waterway Color
	// Road colors the base road network.
	Road Color
	// Building colors building roofs and extrusions.
	Building Color
	// Label colors shaped map text.
	Label Color
	// Route colors application route overlays.
	Route Color
	// Marker colors application markers and POIs.
	Marker Color
	// Traffic colors traffic overlays and incidents.
	Traffic Color
}

// DefaultPalette returns POEM's neutral light cartographic palette.
func DefaultPalette() SemanticPalette {
	return SemanticPalette{
		Land: Color{R: 225, G: 229, B: 220, A: 255}, Water: Color{R: 145, G: 190, B: 220, A: 255}, Waterway: Color{R: 120, G: 175, B: 215, A: 255},
		Road: Color{R: 245, G: 245, B: 242, A: 255}, Building: Color{R: 195, G: 190, B: 184, A: 255}, Label: Color{R: 35, G: 42, B: 48, A: 255},
		Route: Color{R: 35, G: 110, B: 230, A: 255}, Marker: Color{R: 25, G: 165, B: 120, A: 255}, Traffic: Color{R: 210, G: 55, B: 65, A: 255},
	}
}

// DefaultStyle is POEM's neutral vector fallback when an application style
// does not provide a complete platform override.
func DefaultStyle() Style {
	return DefaultStyleWithOptions(DefaultPalette(), StyleOptions{BuildingHeights: true, POICategories: POIAll})
}

// StyleOptions controls bounded optional layers in the semantic base style.
type StyleOptions struct {
	// BuildingHeights enables building roof elevation and wall extrusion.
	BuildingHeights bool
	// POICategories selects visible and interactive POI categories.
	POICategories POICategories
}

// DefaultStyleWithPalette creates POEM's typed base style from resolved
// semantic colors. A zero color retains the corresponding safe default.
func DefaultStyleWithPalette(palette SemanticPalette, buildingHeights bool) Style {
	return DefaultStyleWithOptions(palette, StyleOptions{BuildingHeights: buildingHeights, POICategories: POIAll})
}

// DefaultStyleWithOptions creates the semantic base style with explicit
// bounded layer options.
func DefaultStyleWithOptions(palette SemanticPalette, options StyleOptions) Style {
	palette = normalizedPalette(palette)
	return Style{Layers: []StyleLayer{
		{ID: "landcover", SourceLayer: "landcover", Geometry: GeometryPolygon, MinZoom: 0, MaxZoom: 24, Color: palette.Land},
		{ID: "water", SourceLayer: "water", Geometry: GeometryPolygon, MinZoom: 0, MaxZoom: 24, Color: palette.Water},
		{ID: "waterway", SourceLayer: "waterway", Geometry: GeometryLine, MinZoom: 0, MaxZoom: 24, Color: palette.Waterway, Width: []ZoomStop{{Zoom: 0, Value: 1}, {Zoom: 18, Value: 4}}},
		{ID: "building", SourceLayer: "building", Geometry: GeometryPolygon, MinZoom: 13, MaxZoom: 24, Color: palette.Building, Extrude: options.BuildingHeights},
		{ID: "road", SourceLayer: "transportation", Geometry: GeometryLine, MinZoom: 5, MaxZoom: 24, Color: palette.Road, Width: []ZoomStop{{Zoom: 5, Value: 1}, {Zoom: 18, Value: 8}}, Interactive: true},
		{ID: "poi", SourceLayer: "poi", Geometry: GeometryPoint, MinZoom: 14, MaxZoom: 24, Color: palette.Marker, Width: []ZoomStop{{Zoom: 14, Value: 3}, {Zoom: 20, Value: 6}}, Interactive: true, FilterPOI: true, POICategories: options.POICategories},
	}}
}

func normalizedPalette(palette SemanticPalette) SemanticPalette {
	defaults := DefaultPalette()
	values := []*Color{&palette.Land, &palette.Water, &palette.Waterway, &palette.Road, &palette.Building, &palette.Label, &palette.Route, &palette.Marker, &palette.Traffic}
	fallbacks := []Color{defaults.Land, defaults.Water, defaults.Waterway, defaults.Road, defaults.Building, defaults.Label, defaults.Route, defaults.Marker, defaults.Traffic}
	for index, value := range values {
		if *value == (Color{}) {
			*value = fallbacks[index]
		}
	}
	return palette
}

// DefaultLabelRules are the portable place, road, POI, and water-name rules.
func DefaultLabelRules(locale string) []LabelRule {
	return DefaultLabelRulesFor(locale, POIAll)
}

// DefaultLabelRulesFor returns portable label rules with a bounded POI
// category filter. A zero category set intentionally hides POI labels.
func DefaultLabelRulesFor(locale string, categories POICategories) []LabelRule {
	if locale == "" {
		locale = "und"
	}
	textKeys := []string{"name:" + locale, "name"}
	place := func(id, class string, minZoom, smallZoom, smallSize, largeZoom, largeSize float64, priority int32, weight LabelWeight) LabelRule {
		return LabelRule{ID: id, SourceLayer: "place", Geometry: GeometryPoint, TextKeys: textKeys, Locale: locale, Filters: []Filter{{Key: "class", Value: class, Operation: FilterEqual}}, MinZoom: minZoom, MaxZoom: 24, Size: []ZoomStop{{Zoom: smallZoom, Value: smallSize}, {Zoom: largeZoom, Value: largeSize}}, Priority: priority, Weight: weight}
	}
	rules := []LabelRule{
		place("place-city", "city", 3, 3, 16, 14, 22, 600, LabelWeightSemibold),
		place("place-town", "town", 6, 6, 14, 15, 19, 560, LabelWeightSemibold),
		place("place-village", "village", 8, 8, 13, 16, 17, 520, LabelWeightMedium),
		place("place-district", "suburb", 10, 10, 13, 17, 17, 460, LabelWeightMedium),
		place("place-quarter", "quarter", 11, 11, 12, 18, 16, 430, LabelWeightMedium),
		place("place-neighbourhood", "neighbourhood", 12, 12, 11, 19, 15, 400, LabelWeightRegular),
		{ID: "water-name", SourceLayer: "water_name", Geometry: GeometryPoint, TextKeys: textKeys, Locale: locale, MinZoom: 5, MaxZoom: 24, Size: []ZoomStop{{Zoom: 5, Value: 11}, {Zoom: 18, Value: 16}}, Priority: 350, Weight: LabelWeightMedium},
		{ID: "road-name", SourceLayer: "transportation_name", Geometry: GeometryLine, TextKeys: textKeys, Locale: locale, MinZoom: 11, MaxZoom: 24, Size: []ZoomStop{{Zoom: 11, Value: 10}, {Zoom: 20, Value: 15}}, Priority: 300, Weight: LabelWeightRegular},
		{ID: "poi", SourceLayer: "poi", Geometry: GeometryPoint, TextKeys: textKeys, Locale: locale, MinZoom: 14, MaxZoom: 24, Size: []ZoomStop{{Zoom: 14, Value: 10}, {Zoom: 20, Value: 13}}, Priority: 100, Weight: LabelWeightRegular, FilterPOI: true, POICategories: categories},
	}
	fallbackFilters := make([]Filter, 0, 6)
	for _, class := range []string{"city", "town", "village", "suburb", "quarter", "neighbourhood"} {
		fallbackFilters = append(fallbackFilters, Filter{Key: "class", Value: class, Operation: FilterNotEqual})
	}
	rules = append(rules, LabelRule{ID: "place-other", SourceLayer: "place", Geometry: GeometryPoint, TextKeys: textKeys, Locale: locale, Filters: fallbackFilters, MinZoom: 8, MaxZoom: 24, Size: []ZoomStop{{Zoom: 8, Value: 12}, {Zoom: 18, Value: 16}}, Priority: 380, Weight: LabelWeightRegular})
	return rules
}

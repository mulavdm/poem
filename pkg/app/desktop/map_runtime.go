package desktop

import (
	"bytes"
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mulavdm/poem/pkg/app"
	"github.com/mulavdm/poem/pkg/cartography"
	"github.com/mulavdm/poem/pkg/design"
	"github.com/mulavdm/poem/pkg/render"
	"github.com/mulavdm/poem/pkg/render/protocol"
	"github.com/mulavdm/poem/pkg/render/theme"
	"golang.org/x/image/font/gofont/goregular"
)

var (
	nativeLabelsOnce   sync.Once
	nativeLabelsShaper *cartography.TextShaper
	nativeLabelsErr    error
)

type nativeMapRuntime struct {
	ctx       context.Context
	mu        sync.Mutex
	next      uint64
	active    map[string]nativeMapWork
	cache     *nativeTileCache
	rawCache  *nativeRawTileCache
	resources map[string]map[[32]byte]nativeMapResourceDescriptor
}

type nativeMapWork struct {
	signature  string
	generation uint64
	cancel     context.CancelFunc
}

type nativeMapResourceDescriptor struct {
	Type                  protocol.MapResourceType
	Stride, Width, Height uint32
}

func newNativeMapRuntime(ctx context.Context) *nativeMapRuntime {
	return &nativeMapRuntime{ctx: ctx, active: make(map[string]nativeMapWork), cache: newNativeTileCache(8192, 512<<20), rawCache: newDefaultNativeRawTileCache(), resources: make(map[string]map[[32]byte]nativeMapResourceDescriptor)}
}

func (runtime *nativeMapRuntime) reconcile(root app.Node, providers []app.MapResourceProvider, viewports map[string]image.Point, tokens design.ResolvedTokens, width, height int) {
	nodes := collectMapNodes(root, nil)
	runtime.cache.SetBudget(nativeMapMemoryBudget(nodes))
	runtime.rawCache.SetBudget(nativeMapCacheBudget(nodes))
	available := make(map[string]app.MapResourceProvider, len(providers))
	for _, provider := range providers {
		if provider.Valid() {
			available[provider.ID] = provider
		}
	}
	runtime.mu.Lock()
	var releases []protocol.MapSceneDelta
	wanted := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		id := node.Semantic.ID
		wanted[id] = struct{}{}
		viewport := viewports[id]
		if viewport.X <= 0 || viewport.Y <= 0 {
			viewport = image.Pt(width, height)
		}
		mapStyle, palette, semanticStyle := nativeResolvedMapStyle(node, tokens)
		signature := nativeMapSignature(node, mapStyle, semanticStyle, viewport.X, viewport.Y)
		if current, ok := runtime.active[id]; ok && current.signature == signature {
			continue
		} else if ok {
			current.cancel()
		}
		provider, ok := available[node.Source.ProviderID]
		if !ok {
			delete(runtime.active, id)
			continue
		}
		runtime.next++
		generation := runtime.next
		ctx, cancel := context.WithCancel(runtime.ctx)
		runtime.active[id] = nativeMapWork{signature: signature, generation: generation, cancel: cancel}
		go buildNativeMapScene(ctx, runtime.cache, runtime.rawCache, provider, node, mapStyle, palette, semanticStyle, viewport.X, viewport.Y, generation, runtime.acceptScene)
	}
	for id, work := range runtime.active {
		if _, keep := wanted[id]; !keep {
			work.cancel()
			delete(runtime.active, id)
			runtime.next++
			delta := protocol.MapSceneDelta{ViewportID: id, Generation: runtime.next}
			for _, hash := range sortedMapResourceHashes(runtime.resources[id]) {
				descriptor := runtime.resources[id][hash]
				delta.Resources = append(delta.Resources, protocol.MapSceneResource{Operation: protocol.MapResourceRelease, Type: descriptor.Type, Hash: hash, Stride: descriptor.Stride, Width: descriptor.Width, Height: descriptor.Height})
			}
			delete(runtime.resources, id)
			releases = append(releases, delta)
		}
	}
	runtime.mu.Unlock()
	for _, delta := range releases {
		_ = render.SubmitMapScene(delta)
	}
}

func nativeMapSignature(node app.MapViewportNode, style cartography.Style, semanticStyle app.MapStyle, width, height int) string {
	content, _ := json.Marshal(struct {
		Source   app.MapSource
		Camera   app.MapCamera
		Style    app.MapStyleSet
		Features []app.MapFeature
		Resolved cartography.Style
		Semantic app.MapStyle
	}{node.Source, node.Camera, node.Style, node.Features, style, semanticStyle})
	digest := sha256.Sum256(content)
	return fmt.Sprintf("%x|%dx%d", digest, width, height)
}

func buildNativeMapScene(ctx context.Context, cache *nativeTileCache, rawCache *nativeRawTileCache, provider app.MapResourceProvider, node app.MapViewportNode, style cartography.Style, palette cartography.SemanticPalette, semanticStyle app.MapStyle, width, height int, generation uint64, submit func(protocol.MapSceneDelta)) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	camera := cartography.Camera{Latitude: node.Camera.Latitude, Longitude: node.Camera.Longitude, Zoom: node.Camera.Zoom, Bearing: node.Camera.Bearing, Pitch: node.Camera.Pitch, Width: uint32(max(1, width)), Height: uint32(max(1, height))}
	tileZoom := int(math.Floor(math.Min(camera.Zoom, node.Source.MaxZoom)))
	cover, err := cartography.CoverAtZoom(camera, tileZoom, nativeMapTileLimit(node.Quality))
	if err != nil {
		return
	}
	type result struct {
		tile      cartography.Tile
		elevation *cartography.TerrainTile
		err       error
	}
	jobs := make(chan cartography.TileID)
	results := make(chan result, len(cover))
	workers := min(8, len(cover))
	var group sync.WaitGroup
	var fetchedBytes atomic.Int64
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for tileID := range jobs {
				cacheKey := nativeTileCacheKey{Provider: provider.ID, Source: node.Source.ID, Snapshot: node.Source.Snapshot, Tile: tileID}
				if layers, ok := cache.Get(cacheKey); ok {
					elevation := loadNativeElevation(ctx, rawCache, provider, node, tileID, &fetchedBytes)
					results <- result{tile: cartography.Tile{ID: tileID, Layers: layers}, elevation: elevation}
					continue
				}
				if node.CachePolicy != app.MapCacheMemoryOnly {
					if encoded, ok := rawCache.Get(cacheKey); ok {
						layers, decodeErr := cartography.DecodeMVT(encoded)
						if decodeErr == nil {
							cache.Add(cacheKey, layers, int64(len(encoded)))
							elevation := loadNativeElevation(ctx, rawCache, provider, node, tileID, &fetchedBytes)
							results <- result{tile: cartography.Tile{ID: tileID, Layers: layers}, elevation: elevation}
							continue
						}
						rawCache.Remove(cacheKey)
					}
				}
				resource, fetchErr := provider.Fetch(ctx, app.MapResourceRequest{SourceID: node.Source.ID, Snapshot: node.Source.Snapshot, Kind: app.MapResourceVectorTile, Z: int(tileID.Z), X: int(tileID.X), Y: int(tileID.Y)})
				if fetchErr != nil {
					results <- result{err: fetchErr}
					continue
				}
				if !resource.Valid() {
					results <- result{err: app.ErrMapResourceRejected}
					continue
				}
				if fetchedBytes.Add(int64(len(resource.Bytes))) > 128<<20 {
					cancel()
					results <- result{err: fmt.Errorf("map tile byte budget exceeded")}
					continue
				}
				layers, decodeErr := cartography.DecodeMVT(resource.Bytes)
				if decodeErr == nil {
					cache.Add(cacheKey, layers, int64(len(resource.Bytes)))
					if node.CachePolicy != app.MapCacheMemoryOnly {
						rawCache.Put(cacheKey, resource.Bytes)
					}
				}
				var elevation *cartography.TerrainTile
				if decodeErr == nil {
					elevation = loadNativeElevation(ctx, rawCache, provider, node, tileID, &fetchedBytes)
				}
				results <- result{tile: cartography.Tile{ID: tileID, Layers: layers}, elevation: elevation, err: decodeErr}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, tile := range cover {
			select {
			case jobs <- tile:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { group.Wait(); close(results) }()
	var tiles []cartography.Tile
	var terrain []cartography.TerrainTile
	for result := range results {
		if result.err == nil {
			tiles = append(tiles, result.tile)
			if result.elevation != nil {
				terrain = append(terrain, *result.elevation)
			}
		}
	}
	if ctx.Err() != nil || len(tiles) == 0 {
		return
	}
	// Fog fades distant ground into the map's own land tone, so a pitched view
	// dissolves at the horizon instead of ending in a hard edge. The presenter
	// fades it in with pitch, so a top-down map is unaffected.
	fogColor := palette.Land
	scene, err := cartography.BuildSceneLit(node.Semantic.ID, generation, camera, style, tiles, cartography.Lighting{
		DynamicSun: semanticStyle.DynamicSun, Clock: time.Now(),
		FogDensity: cartography.DefaultFogDensity, FogColor: &fogColor,
		ShadowCascades: nativeShadowCascades(node.Quality),
	})
	if err == nil {
		scene, err = cartography.AddSceneFeatures(scene, nativeSceneFeatures(node, palette))
	}
	if err == nil && ctx.Err() == nil {
		if shaper, shaperErr := nativeMapLabelShaper(); shaperErr == nil {
			if candidates, candidateErr := cartography.BuildLabelCandidates(camera, cartography.DefaultLabelRulesFor(semanticStyle.Locale, nativePOICategories(semanticStyle.POIFilters)), tiles); candidateErr == nil {
				candidates, candidateErr = cartography.LimitLabelCandidates(candidates, semanticStyle.LabelDensity, nativeMapLabelLimit(node.Quality))
				if candidateErr == nil {
					if placements, placementErr := cartography.PlaceLabels(ctx, camera, shaper, candidates); placementErr == nil {
						if labelled, labelErr := cartography.AddLabelPlacements(scene, placements, shaper, palette.Label, 900_000); labelErr == nil {
							scene = labelled
						}
					}
				}
			}
		}
	}
	if err == nil && len(terrain) > 0 {
		scene, err = cartography.DrapeSceneToTerrain(scene, terrain, float32(semanticStyle.TerrainScale))
	}
	if err == nil && len(terrain) > 0 {
		scene, err = cartography.AddTerrainMeshes(scene, terrain, cartography.TerrainMeshOptions{MaxDimension: nativeTerrainDimension(node.Quality), VerticalScale: float32(semanticStyle.TerrainScale), SkirtDepth: 20, Color: palette.Land})
	}
	if err == nil && ctx.Err() == nil {
		submit(scene.Delta)
	}
}

func loadNativeElevation(ctx context.Context, rawCache *nativeRawTileCache, provider app.MapResourceProvider, node app.MapViewportNode, tileID cartography.TileID, fetchedBytes *atomic.Int64) *cartography.TerrainTile {
	if !node.Source.Elevation || ctx.Err() != nil {
		return nil
	}
	key := nativeTileCacheKey{Provider: provider.ID, Source: node.Source.ID, Snapshot: node.Source.Snapshot, Kind: app.MapResourceElevationTile, Tile: tileID}
	var encoded []byte
	if node.CachePolicy != app.MapCacheMemoryOnly {
		encoded, _ = rawCache.Get(key)
	}
	if len(encoded) == 0 {
		resource, err := provider.Fetch(ctx, app.MapResourceRequest{SourceID: node.Source.ID, Snapshot: node.Source.Snapshot, Kind: app.MapResourceElevationTile, Z: int(tileID.Z), X: int(tileID.X), Y: int(tileID.Y)})
		if err != nil || !resource.Valid() {
			return nil
		}
		encoded = resource.Bytes
	}
	if fetchedBytes.Add(int64(len(encoded))) > 128<<20 {
		return nil
	}
	elevation, err := cartography.DecodeElevationTile(encoded)
	if err != nil {
		rawCache.Remove(key)
		return nil
	}
	if node.CachePolicy != app.MapCacheMemoryOnly {
		rawCache.Put(key, encoded)
	}
	return &cartography.TerrainTile{ID: tileID, Elevation: elevation}
}

func nativeMapLabelShaper() (*cartography.TextShaper, error) {
	nativeLabelsOnce.Do(func() { nativeLabelsShaper, nativeLabelsErr = cartography.NewTextShaper(goregular.TTF) })
	return nativeLabelsShaper, nativeLabelsErr
}

func nativeMapTileLimit(quality app.MapQuality) int {
	switch quality {
	case app.MapQualityBatterySaver:
		return 24
	case app.MapQualityHigh:
		return 64
	default:
		return 48
	}
}

func nativeMapLabelLimit(quality app.MapQuality) int {
	switch quality {
	case app.MapQualityBatterySaver:
		return 4096
	case app.MapQualityHigh:
		return 16_384
	default:
		return 12_288
	}
}

// nativeShadowCascades maps the quality tier onto cascade count: Battery Saver
// casts none (the shadow pass is the most expensive part of the frame),
// Balanced one, High two.
func nativeShadowCascades(quality app.MapQuality) uint8 {
	switch quality {
	case app.MapQualityBatterySaver:
		return 0
	case app.MapQualityHigh:
		return 2
	default:
		return 1
	}
}

func nativeTerrainDimension(quality app.MapQuality) int {
	switch quality {
	case app.MapQualityBatterySaver:
		return 33
	case app.MapQualityHigh:
		return 129
	default:
		return 65
	}
}

func nativePOICategories(filters app.MapPOIFilters) cartography.POICategories {
	var categories cartography.POICategories
	if filters&app.POITransport != 0 {
		categories |= cartography.POITransport
	}
	if filters&app.POIParkingFuel != 0 {
		categories |= cartography.POIParkingFuel
	}
	if filters&app.POIFoodDrink != 0 {
		categories |= cartography.POIFoodDrink
	}
	if filters&app.POIHealth != 0 {
		categories |= cartography.POIHealth
	}
	if filters&app.POIShoppingServices != 0 {
		categories |= cartography.POIShoppingServices
	}
	if filters&app.POILeisureTourism != 0 {
		categories |= cartography.POILeisureTourism
	}
	if filters&app.POICivic != 0 {
		categories |= cartography.POICivic
	}
	return categories
}

func (runtime *nativeMapRuntime) acceptScene(delta protocol.MapSceneDelta) {
	runtime.mu.Lock()
	work, ok := runtime.active[delta.ViewportID]
	if !ok || work.generation != delta.Generation {
		runtime.mu.Unlock()
		return
	}
	filtered, current := deduplicateMapResources(runtime.resources[delta.ViewportID], delta)
	runtime.mu.Unlock()
	if err := render.SubmitMapScene(filtered); err == nil {
		runtime.mu.Lock()
		work, ok = runtime.active[delta.ViewportID]
		if ok && work.generation == delta.Generation {
			runtime.resources[delta.ViewportID] = current
		}
		runtime.mu.Unlock()
	}
}

func deduplicateMapResources(previous map[[32]byte]nativeMapResourceDescriptor, delta protocol.MapSceneDelta) (protocol.MapSceneDelta, map[[32]byte]nativeMapResourceDescriptor) {
	current := make(map[[32]byte]nativeMapResourceDescriptor, len(delta.Resources))
	filtered := delta
	filtered.Resources = make([]protocol.MapSceneResource, 0, len(delta.Resources)+len(previous))
	for _, resource := range delta.Resources {
		if resource.Operation != protocol.MapResourceUpload {
			continue
		}
		descriptor := nativeMapResourceDescriptor{Type: resource.Type, Stride: resource.Stride, Width: resource.Width, Height: resource.Height}
		current[resource.Hash] = descriptor
		if prior, ok := previous[resource.Hash]; !ok || prior != descriptor {
			filtered.Resources = append(filtered.Resources, resource)
		}
	}
	for _, hash := range sortedMapResourceHashes(previous) {
		descriptor := previous[hash]
		if _, ok := current[hash]; !ok {
			filtered.Resources = append(filtered.Resources, protocol.MapSceneResource{Operation: protocol.MapResourceRelease, Type: descriptor.Type, Hash: hash, Stride: descriptor.Stride, Width: descriptor.Width, Height: descriptor.Height})
		}
	}
	return filtered, current
}

func sortedMapResourceHashes(resources map[[32]byte]nativeMapResourceDescriptor) [][32]byte {
	hashes := make([][32]byte, 0, len(resources))
	for hash := range resources {
		hashes = append(hashes, hash)
	}
	sort.Slice(hashes, func(i, j int) bool { return bytes.Compare(hashes[i][:], hashes[j][:]) < 0 })
	return hashes
}

type nativeTileCacheKey struct {
	Provider, Source, Snapshot string
	Kind                       app.MapResourceKind
	Tile                       cartography.TileID
}

type nativeTileCacheEntry struct {
	key    nativeTileCacheKey
	layers []cartography.Layer
	bytes  int64
}

type nativeTileCache struct {
	mu         sync.Mutex
	entries    map[nativeTileCacheKey]*list.Element
	recent     *list.List
	maxEntries int
	maxBytes   int64
	bytes      int64
}

func nativeMapCacheBudget(nodes []app.MapViewportNode) int64 {
	const defaultBudget = int64(2 << 30)
	budget := int64(0)
	for _, node := range nodes {
		requested := node.CacheBytes
		if requested == 0 {
			requested = defaultBudget
		}
		budget = max(budget, requested)
	}
	if budget == 0 {
		budget = defaultBudget
	}
	return min(budget, int64(64<<30))
}

func nativeMapMemoryBudget(nodes []app.MapViewportNode) int64 {
	return min(nativeMapCacheBudget(nodes), int64(512<<20))
}

func newNativeTileCache(maxEntries int, maxBytes int64) *nativeTileCache {
	return &nativeTileCache{entries: make(map[nativeTileCacheKey]*list.Element), recent: list.New(), maxEntries: maxEntries, maxBytes: maxBytes}
}

func (cache *nativeTileCache) SetBudget(maxBytes int64) {
	if cache == nil || maxBytes <= 0 {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.maxBytes = maxBytes
	cache.evictLocked()
}

func (cache *nativeTileCache) Get(key nativeTileCacheKey) ([]cartography.Layer, bool) {
	if cache == nil {
		return nil, false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	element := cache.entries[key]
	if element == nil {
		return nil, false
	}
	cache.recent.MoveToFront(element)
	return element.Value.(nativeTileCacheEntry).layers, true
}

func (cache *nativeTileCache) Add(key nativeTileCacheKey, layers []cartography.Layer, bytes int64) {
	if cache == nil || bytes <= 0 || bytes > cache.maxBytes {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if existing := cache.entries[key]; existing != nil {
		cache.recent.MoveToFront(existing)
		return
	}
	entry := nativeTileCacheEntry{key: key, layers: layers, bytes: bytes}
	cache.entries[key] = cache.recent.PushFront(entry)
	cache.bytes += bytes
	cache.evictLocked()
}

func (cache *nativeTileCache) evictLocked() {
	for len(cache.entries) > cache.maxEntries || cache.bytes > cache.maxBytes {
		oldest := cache.recent.Back()
		if oldest == nil {
			break
		}
		value := oldest.Value.(nativeTileCacheEntry)
		delete(cache.entries, value.key)
		cache.bytes -= value.bytes
		cache.recent.Remove(oldest)
	}
}

func nativeSceneFeatures(node app.MapViewportNode, palette cartography.SemanticPalette) []cartography.SceneFeature {
	features := make([]cartography.SceneFeature, 0, len(node.Features))
	for _, feature := range node.Features {
		if !feature.Valid() {
			continue
		}
		geometry := cartography.GeometryPoint
		if feature.Geometry == app.MapGeometryLine {
			geometry = cartography.GeometryLine
		} else if feature.Geometry == app.MapGeometryPolygon {
			geometry = cartography.GeometryPolygon
		}
		color, width, layer := nativeFeatureAppearance(feature, palette)
		coordinates := make([][2]float64, len(feature.Positions))
		for index, position := range feature.Positions {
			coordinates[index] = [2]float64{position.Longitude, position.Latitude}
		}
		features = append(features, cartography.SceneFeature{ID: feature.ID, Geometry: geometry, Coordinates: coordinates, Color: color, Width: width, Layer: layer, Interactive: !feature.Disabled && feature.OnActivate.Name != ""})
	}
	return features
}

func nativeFeatureAppearance(feature app.MapFeature, palette cartography.SemanticPalette) (cartography.Color, float32, int32) {
	color, width, layer := palette.Route, float32(6), int32(1_000_100)
	switch feature.Role {
	case app.MapFeatureMarker, app.MapFeaturePOI:
		color, width, layer = palette.Marker, 12, 1_000_300
	case app.MapFeatureTraffic:
		color, width, layer = palette.Traffic, 8, 1_000_200
	}
	if feature.Selected {
		width *= 1.5
	}
	return color, width, layer
}

func nativeResolvedMapStyle(node app.MapViewportNode, tokens design.ResolvedTokens) (cartography.Style, cartography.SemanticPalette, app.MapStyle) {
	selected := node.Style.House
	switch tokens.Environment.Platform {
	case design.PlatformWindows:
		if node.Style.Windows.Valid() {
			selected = node.Style.Windows
		}
	case design.PlatformAndroid:
		if node.Style.Android.Valid() {
			selected = node.Style.Android
		}
	case design.PlatformWeb:
		if node.Style.Web.Valid() {
			selected = node.Style.Web
		}
	}
	if !selected.Valid() {
		selected = node.Style.House
	}
	if !selected.Valid() {
		selected = app.MapStyle{ID: "poem-default", LabelDensity: 1, TerrainScale: 1, BuildingHeights: true, POIFilters: app.POITransport | app.POIParkingFuel | app.POIFoodDrink | app.POIHealth | app.POIShoppingServices | app.POILeisureTourism | app.POICivic}
	}
	if selected.Locale == "" {
		selected.Locale = tokens.Environment.Locale
		if selected.Locale == "" {
			selected.Locale = "en"
		}
	}
	colors := tokens.Theme.Colors
	palette := cartography.SemanticPalette{
		Land: mapRGBA(colors.SurfaceSunken), Water: mapRGBA(colors.Accent), Waterway: mapRGBA(colors.Accent), Road: mapRGBA(colors.SurfaceRaised),
		Building: mapRGBA(colors.Border), Label: mapRGBA(colors.Text), Route: mapRGBA(colors.Accent), Marker: mapRGBA(colors.Success), Traffic: mapRGBA(colors.Danger),
	}
	if selected.Valid() {
		palette.Land = resolveNativeMapColor(selected.Colors.Land, palette.Land, palette, colors)
		palette.Water = resolveNativeMapColor(selected.Colors.Water, palette.Water, palette, colors)
		palette.Waterway = palette.Water
		palette.Road = resolveNativeMapColor(selected.Colors.Road, palette.Road, palette, colors)
		palette.Building = resolveNativeMapColor(selected.Colors.Building, palette.Building, palette, colors)
		palette.Label = resolveNativeMapColor(selected.Colors.Label, palette.Label, palette, colors)
		palette.Route = resolveNativeMapColor(selected.Colors.Route, palette.Route, palette, colors)
		palette.Traffic = resolveNativeMapColor(selected.Colors.Traffic, palette.Traffic, palette, colors)
	}
	return cartography.DefaultStyleWithOptions(palette, cartography.StyleOptions{BuildingHeights: selected.BuildingHeights, POICategories: nativePOICategories(selected.POIFilters)}), palette, selected
}

func resolveNativeMapColor(value string, fallback cartography.Color, palette cartography.SemanticPalette, colors theme.Colors) cartography.Color {
	if parsed, ok := parseMapColor(value); ok {
		return parsed
	}
	switch strings.TrimSpace(value) {
	case "land":
		return palette.Land
	case "water":
		return palette.Water
	case "road":
		return palette.Road
	case "building":
		return palette.Building
	case "label", "text":
		return mapRGBA(colors.Text)
	case "route", "primary", "accent":
		return mapRGBA(colors.Accent)
	case "traffic", "danger":
		return mapRGBA(colors.Danger)
	case "surface":
		return mapRGBA(colors.Surface)
	case "surface-muted", "sunken":
		return mapRGBA(colors.SurfaceSunken)
	case "surface-raised", "raised":
		return mapRGBA(colors.SurfaceRaised)
	case "line", "border":
		return mapRGBA(colors.Border)
	case "muted":
		return mapRGBA(colors.TextMuted)
	case "success", "marker":
		return mapRGBA(colors.Success)
	case "warning":
		return mapRGBA(colors.Warning)
	default:
		return fallback
	}
}

func mapRGBA(value color.RGBA) cartography.Color {
	return cartography.Color{R: value.R, G: value.G, B: value.B, A: value.A}
}

func parseMapColor(value string) (cartography.Color, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 && len(value) != 8 {
		return cartography.Color{}, false
	}
	parsed, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return cartography.Color{}, false
	}
	if len(value) == 6 {
		return cartography.Color{R: byte(parsed >> 16), G: byte(parsed >> 8), B: byte(parsed), A: 255}, true
	}
	return cartography.Color{R: byte(parsed >> 24), G: byte(parsed >> 16), B: byte(parsed >> 8), A: byte(parsed)}, true
}

func collectMapNodes(node app.Node, output []app.MapViewportNode) []app.MapViewportNode {
	switch value := node.(type) {
	case app.MapViewportNode:
		output = append(output, value)
	case app.IdentityNode:
		output = collectMapNodes(value.Child, output)
	case app.ContainerNode:
		for _, child := range value.Children {
			output = collectMapNodes(child, output)
		}
	case app.ModalNode:
		for _, child := range value.Content {
			output = collectMapNodes(child, output)
		}
	case app.AccordionNode:
		for _, section := range value.Sections {
			for _, child := range section.Content {
				output = collectMapNodes(child, output)
			}
		}
	case app.TabsNode:
		if index := value.ActiveIndex(); index >= 0 {
			for _, child := range value.Tabs[index].Content {
				output = collectMapNodes(child, output)
			}
		}
	case app.ResponsiveNode:
		for _, child := range value.Compact {
			output = collectMapNodes(child, output)
		}
		for _, child := range value.Wide {
			output = collectMapNodes(child, output)
		}
	// The semantic container nodes below survive LowerSemantic (it lowers their
	// children but keeps the node), so they must be traversed here too — a map
	// placed in a WorkspaceNode's content is the normal composition.
	case app.WorkspaceNode:
		output = collectMapNodes(value.Status, output)
		output = collectMapNodes(value.Navigation, output)
		for _, child := range value.Header {
			output = collectMapNodes(child, output)
		}
		for _, child := range value.Content {
			output = collectMapNodes(child, output)
		}
		for _, child := range value.Tools {
			output = collectMapNodes(child, output)
		}
	case app.SectionNode:
		for _, child := range value.Children {
			output = collectMapNodes(child, output)
		}
	case app.AdaptiveNode:
		for _, children := range [][]app.Node{value.Compact, value.Medium, value.Expanded, value.UltraWide} {
			for _, child := range children {
				output = collectMapNodes(child, output)
			}
		}
	}
	return output
}

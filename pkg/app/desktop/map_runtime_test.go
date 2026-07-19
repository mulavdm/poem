package desktop

import (
	"context"
	"crypto/sha256"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mulavdm/poem/pkg/app"
	"github.com/mulavdm/poem/pkg/cartography"
	"github.com/mulavdm/poem/pkg/design"
	"github.com/mulavdm/poem/pkg/render/protocol"
)

func TestNativeMapRuntimeBoundsFetchConcurrency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	camera := cartography.Camera{Latitude: 52.37, Longitude: 4.9, Zoom: 12, Width: 800, Height: 600}
	cover, err := cartography.Cover(camera, nativeMapTileLimit(app.MapQualityAuto))
	if err != nil {
		t.Fatal(err)
	}
	var active, maximum, calls atomic.Int32
	done := make(chan struct{}, 1)
	provider := app.MapResourceProvider{ID: "tiles", Fetch: func(ctx context.Context, request app.MapResourceRequest) (app.MapResource, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		select {
		case <-time.After(time.Millisecond):
		case <-ctx.Done():
			return app.MapResource{}, ctx.Err()
		}
		if calls.Add(1) == int32(len(cover)) {
			done <- struct{}{}
		}
		return app.MapResource{Bytes: []byte{0x1a, 0x05, 0x0a, 0x01, 'x', 0x28, 0x01}, ContentType: "application/vnd.mapbox-vector-tile"}, nil
	}}
	runtime := newNativeMapRuntime(ctx)
	node := app.MapViewportNode{Semantic: app.Semantic{ID: "map", Name: "Map", Enabled: true}, Source: app.MapSource{ID: "source", ProviderID: "tiles", MinZoom: 0, MaxZoom: 18}, Camera: app.MapCamera{Latitude: camera.Latitude, Longitude: camera.Longitude, Zoom: camera.Zoom}, MinZoom: 0, MaxZoom: 18, MinPitch: 0, MaxPitch: 60}
	tokens := design.DefaultSystem().Resolve(design.Environment{Platform: design.PlatformWindows, Width: 800, Height: 600})
	runtime.reconcile(node, []app.MapResourceProvider{provider}, nil, tokens, 800, 600)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("only %d of %d tiles fetched", calls.Load(), len(cover))
	}
	if maximum.Load() > 8 || calls.Load() > 64 {
		t.Fatalf("maximum=%d calls=%d", maximum.Load(), calls.Load())
	}
	// An identical view reuses the active generation and performs no new work.
	runtime.reconcile(node, []app.MapResourceProvider{provider}, nil, tokens, 800, 600)
	time.Sleep(20 * time.Millisecond)
	if calls.Load() != int32(len(cover)) {
		t.Fatalf("unchanged map refetched: %d", calls.Load())
	}
}

func TestCollectMapNodesTraversesIdentity(t *testing.T) {
	node := app.IdentityNode{Semantic: app.Semantic{ID: "outer"}, Child: app.Container(app.Vertical, 0, app.MapViewportNode{Semantic: app.Semantic{ID: "map"}})}
	if maps := collectMapNodes(node, nil); len(maps) != 1 || maps[0].Semantic.ID != "map" {
		t.Fatalf("maps=%#v", maps)
	}
}

func TestNativeMapSignatureAndSceneFeaturesIncludeApplicationOverlays(t *testing.T) {
	node := app.MapViewportNode{
		Semantic: app.Semantic{ID: "map"},
		Source:   app.MapSource{ID: "source", ProviderID: "tiles", MinZoom: 0, MaxZoom: 18},
		Camera:   app.MapCamera{Latitude: 52.3, Longitude: 4.9, Zoom: 12},
		Style:    app.MapStyleSet{House: app.MapStyle{ID: "house", Colors: app.MapSemanticColors{Route: "#123456"}}},
		Features: []app.MapFeature{{ID: "route", Geometry: app.MapGeometryLine, Role: app.MapFeatureRoute, Positions: []app.MapPosition{{Latitude: 52.3, Longitude: 4.8}, {Latitude: 52.4, Longitude: 4.9}}, OnActivate: app.Msg{Name: "route.pick"}}},
	}
	tokens := design.DefaultSystem().Resolve(design.Environment{Platform: design.PlatformWindows, Width: 800, Height: 600})
	style, palette, semanticStyle := nativeResolvedMapStyle(node, tokens)
	before := nativeMapSignature(node, style, semanticStyle, 800, 600)
	features := nativeSceneFeatures(node, palette)
	if len(features) != 1 || features[0].Geometry != cartography.GeometryLine || features[0].Color != (cartography.Color{R: 0x12, G: 0x34, B: 0x56, A: 0xff}) || !features[0].Interactive {
		t.Fatalf("native features = %+v", features)
	}
	node.Features[0].Positions[1].Longitude = 5
	if after := nativeMapSignature(node, style, semanticStyle, 800, 600); after == before {
		t.Fatal("feature geometry did not invalidate native scene signature")
	}
}

func TestNativeTileCacheEvictsLeastRecentAndIsolatesSnapshots(t *testing.T) {
	cache := newNativeTileCache(2, 10)
	one := nativeTileCacheKey{Provider: "tiles", Source: "map", Snapshot: "one", Tile: cartography.TileID{Z: 1}}
	two := nativeTileCacheKey{Provider: "tiles", Source: "map", Snapshot: "one", Tile: cartography.TileID{Z: 1, X: 1}}
	otherSnapshot := nativeTileCacheKey{Provider: "tiles", Source: "map", Snapshot: "two", Tile: cartography.TileID{Z: 1}}
	cache.Add(one, []cartography.Layer{{Name: "one"}}, 4)
	cache.Add(two, []cartography.Layer{{Name: "two"}}, 4)
	if _, ok := cache.Get(one); !ok {
		t.Fatal("recent tile missing")
	}
	cache.Add(otherSnapshot, []cartography.Layer{{Name: "other"}}, 4)
	if _, ok := cache.Get(two); ok {
		t.Fatal("least-recent tile was not evicted")
	}
	if layers, ok := cache.Get(otherSnapshot); !ok || len(layers) != 1 || layers[0].Name != "other" {
		t.Fatalf("snapshot tile = %+v, %v", layers, ok)
	}
}

func TestNativeTileCacheAppliesViewportBudgetImmediately(t *testing.T) {
	cache := newNativeTileCache(8, 100)
	one := nativeTileCacheKey{Provider: "tiles", Source: "map", Tile: cartography.TileID{Z: 1}}
	two := nativeTileCacheKey{Provider: "tiles", Source: "map", Tile: cartography.TileID{Z: 1, X: 1}}
	cache.Add(one, []cartography.Layer{{Name: "one"}}, 40)
	cache.Add(two, []cartography.Layer{{Name: "two"}}, 40)
	cache.SetBudget(50)
	if _, ok := cache.Get(one); ok {
		t.Fatal("least-recent tile survived reduced budget")
	}
	if _, ok := cache.Get(two); !ok {
		t.Fatal("most-recent tile was evicted")
	}
	if budget := nativeMapCacheBudget([]app.MapViewportNode{{CacheBytes: 32 << 20}, {CacheBytes: 64 << 20}}); budget != 64<<20 {
		t.Fatalf("budget=%d", budget)
	}
}

func TestDeduplicateMapResourcesReferencesUnchangedAndReleasesOld(t *testing.T) {
	keepHash := sha256.Sum256([]byte("keep"))
	oldHash := sha256.Sum256([]byte("old"))
	newHash := sha256.Sum256([]byte("new"))
	vertex := nativeMapResourceDescriptor{Type: protocol.MapResourceVertexBuffer, Stride: 32}
	previous := map[[32]byte]nativeMapResourceDescriptor{keepHash: vertex, oldHash: vertex}
	delta := protocol.MapSceneDelta{ViewportID: "map", Generation: 2, Resources: []protocol.MapSceneResource{
		{Operation: protocol.MapResourceUpload, Type: vertex.Type, Hash: keepHash, Stride: vertex.Stride, Bytes: []byte("keep")},
		{Operation: protocol.MapResourceUpload, Type: vertex.Type, Hash: newHash, Stride: vertex.Stride, Bytes: []byte("new")},
	}}
	filtered, current := deduplicateMapResources(previous, delta)
	if len(filtered.Resources) != 2 || len(current) != 2 {
		t.Fatalf("filtered=%+v current=%+v", filtered.Resources, current)
	}
	if filtered.Resources[0].Hash != newHash || filtered.Resources[0].Operation != protocol.MapResourceUpload {
		t.Fatalf("new upload=%+v", filtered.Resources[0])
	}
	if filtered.Resources[1].Hash != oldHash || filtered.Resources[1].Operation != protocol.MapResourceRelease || len(filtered.Resources[1].Bytes) != 0 {
		t.Fatalf("old release=%+v", filtered.Resources[1])
	}
	if _, found := current[keepHash]; !found {
		t.Fatal("unchanged reference was not retained")
	}
}

func TestNativeMapQualityBoundsTileWork(t *testing.T) {
	if nativeMapTileLimit(app.MapQualityBatterySaver) != 24 || nativeMapTileLimit(app.MapQualityAuto) != 48 || nativeMapTileLimit(app.MapQualityBalanced) != 48 || nativeMapTileLimit(app.MapQualityHigh) != 64 {
		t.Fatal("unexpected native quality tile limits")
	}
}

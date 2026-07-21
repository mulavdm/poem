package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mulavdm/poem/pkg/app"
	"github.com/mulavdm/poem/pkg/cartography"
)

func cacheKeyFor(z uint8, x, y uint32) nativeTileCacheKey {
	return nativeTileCacheKey{
		Provider: "mapps-vector", Source: "mapps-local", Snapshot: "2026-07",
		Kind: app.MapResourceVectorTile,
		Tile: cartography.TileID{Z: z, X: x, Y: y},
	}
}

func TestMemoryOnlyPolicyWritesNothingToDisk(t *testing.T) {
	directory := t.TempDir()
	cache := newNativeRawTileCache(filepath.Join(directory, "resources"), 2<<30)
	cache.SetPolicy(app.MapCacheMemoryOnly)

	cache.Put(cacheKeyFor(14, 8412, 5387), []byte("verified-tile-bytes"))

	if _, ok := cache.Get(cacheKeyFor(14, 8412, 5387)); ok {
		t.Fatal("memory-only policy served a resource from disk")
	}
	entries, err := os.ReadDir(filepath.Join(directory, "resources"))
	if err == nil && len(entries) > 0 {
		t.Fatalf("memory-only policy left %d files on disk", len(entries))
	}
}

func TestPersistentPoliciesRoundTripThroughDisk(t *testing.T) {
	for _, policy := range []app.MapCachePolicy{app.MapCachePersistentOnlineOnly, app.MapCachePersistentOffline} {
		cache := newNativeRawTileCache(filepath.Join(t.TempDir(), "resources"), 2<<30)
		cache.SetPolicy(policy)
		cache.Put(cacheKeyFor(14, 1, 2), []byte("verified-tile-bytes"))
		body, ok := cache.Get(cacheKeyFor(14, 1, 2))
		if !ok || string(body) != "verified-tile-bytes" {
			t.Fatalf("policy %v did not persist and return the resource", policy)
		}
	}
}

// Only Persistent Offline may answer once the provider cannot; the online-only
// policy is an accelerator, so an unreachable provider stays an error.
func TestOnlyPersistentOfflineServesOffline(t *testing.T) {
	cases := map[app.MapCachePolicy]bool{
		app.MapCacheMemoryOnly:           false,
		app.MapCachePersistentOnlineOnly: false,
		app.MapCachePersistentOffline:    true,
	}
	for policy, want := range cases {
		cache := newNativeRawTileCache(filepath.Join(t.TempDir(), "resources"), 2<<30)
		cache.SetPolicy(policy)
		if got := cache.servesOffline(); got != want {
			t.Fatalf("policy %v servesOffline = %v, want %v", policy, got, want)
		}
	}
}

// The plan's explicit guarantee: clearing the resource cache must not cost the
// user a region they deliberately downloaded.
func TestClearLeavesDownloadedRegionsIntact(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "cache", "resources")
	packageDir := filepath.Join(root, "data", "MAPPS", "packages", "regions", "noord-holland", "2026-07")
	if err := os.MkdirAll(packageDir, 0o700); err != nil {
		t.Fatal(err)
	}
	packaged := filepath.Join(packageDir, "valhalla_tiles.gph")
	if err := os.WriteFile(packaged, []byte("graph-tile"), 0o600); err != nil {
		t.Fatal(err)
	}

	cache := newNativeRawTileCache(cacheDir, 2<<30)
	cache.SetPolicy(app.MapCachePersistentOffline)
	cache.Put(cacheKeyFor(14, 3, 4), []byte("verified-tile-bytes"))
	if _, ok := cache.Get(cacheKeyFor(14, 3, 4)); !ok {
		t.Fatal("resource was not cached before clearing")
	}

	cache.Clear()

	if _, ok := cache.Get(cacheKeyFor(14, 3, 4)); ok {
		t.Fatal("Clear left a cached resource behind")
	}
	if body, err := os.ReadFile(packaged); err != nil || string(body) != "graph-tile" {
		t.Fatalf("Clear damaged a downloaded region package: %q err=%v", body, err)
	}
}

// Switching to memory-only must also drop what earlier policies persisted.
func TestSwitchingToMemoryOnlyClearsExistingCache(t *testing.T) {
	cache := newNativeRawTileCache(filepath.Join(t.TempDir(), "resources"), 2<<30)
	cache.SetPolicy(app.MapCachePersistentOffline)
	cache.Put(cacheKeyFor(14, 5, 6), []byte("verified-tile-bytes"))
	if _, ok := cache.Get(cacheKeyFor(14, 5, 6)); !ok {
		t.Fatal("resource was not cached under the persistent policy")
	}

	cache.SetPolicy(app.MapCacheMemoryOnly)

	if _, ok := cache.Get(cacheKeyFor(14, 5, 6)); ok {
		t.Fatal("switching to memory-only left the persisted copy on disk")
	}
}

// The shared cache takes the most restrictive viewport's policy.
func TestCachePolicyTakesTheMostRestrictiveViewport(t *testing.T) {
	nodes := []app.MapViewportNode{
		{CachePolicy: app.MapCachePersistentOffline},
		{CachePolicy: app.MapCacheMemoryOnly},
	}
	if got := nativeMapCachePolicy(nodes); got != app.MapCacheMemoryOnly {
		t.Fatalf("policy = %v, want MemoryOnly", got)
	}
	if got := nativeMapCachePolicy([]app.MapViewportNode{{CachePolicy: app.MapCachePersistentOffline}}); got != app.MapCachePersistentOffline {
		t.Fatalf("single offline viewport resolved to %v", got)
	}
}

// Default budgets: 2 GB native, per the plan.
func TestDefaultNativeCacheBudget(t *testing.T) {
	if got := nativeMapCacheBudget([]app.MapViewportNode{{}}); got != int64(2<<30) {
		t.Fatalf("default native budget = %d, want 2 GiB", got)
	}
}

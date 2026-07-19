package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mulavdm/poem/pkg/cartography"
)

func TestNativeRawTileCacheRoundTripAndCorruptionRemoval(t *testing.T) {
	cache := newNativeRawTileCache(t.TempDir(), 1<<20)
	key := nativeTileCacheKey{Provider: "engine", Source: "streets", Snapshot: "s1", Tile: cartography.TileID{Z: 4, X: 8, Y: 5}}
	payload := []byte("verified raw tile")
	cache.Put(key, payload)
	got, ok := cache.Get(key)
	if !ok || !bytes.Equal(got, payload) {
		t.Fatalf("Get() = %q, %v", got, ok)
	}
	got[0] ^= 0xff
	again, ok := cache.Get(key)
	if !ok || !bytes.Equal(again, payload) {
		t.Fatal("cache returned mutable shared bytes")
	}
	path := cache.path(key)
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	encoded[len(encoded)-1] ^= 0xff
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.Get(key); ok {
		t.Fatal("accepted corrupt cache entry")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("corrupt cache entry remains: %v", err)
	}
}

func TestNativeRawTileCacheBudgetEvictsOrdinaryResources(t *testing.T) {
	directory := t.TempDir()
	payload := bytes.Repeat([]byte{7}, 64)
	entryBytes := int64(nativeRawCacheHeaderBytes + len(payload))
	cache := newNativeRawTileCache(directory, entryBytes)
	one := nativeTileCacheKey{Provider: "engine", Source: "streets", Snapshot: "s1", Tile: cartography.TileID{Z: 1}}
	two := nativeTileCacheKey{Provider: "engine", Source: "streets", Snapshot: "s1", Tile: cartography.TileID{Z: 1, X: 1}}
	cache.Put(one, payload)
	cache.Put(two, payload)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var raw int
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".raw" {
			raw++
		}
	}
	if raw != 1 {
		t.Fatalf("raw entries=%d", raw)
	}
	cache.SetBudget(entryBytes - 1)
	entries, err = os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".raw" {
			t.Fatalf("entry %q survived reduced budget", entry.Name())
		}
	}
}

func TestNativeRawCacheEncodingRejectsMalformedRecords(t *testing.T) {
	for _, encoded := range [][]byte{nil, []byte("short"), encodeNativeRawCacheFile([]byte("ok"))[:nativeRawCacheHeaderBytes]} {
		if _, ok := decodeNativeRawCacheFile(encoded); ok {
			t.Fatalf("accepted %x", encoded)
		}
	}
}

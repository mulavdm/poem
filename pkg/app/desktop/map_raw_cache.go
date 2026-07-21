package desktop

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/mulavdm/poem/pkg/app"
)

var nativeRawCacheMagic = [8]byte{'P', 'O', 'E', 'M', 'R', 'A', 'W', 1}

const nativeRawCacheHeaderBytes = len(nativeRawCacheMagic) + sha256.Size + 4

type nativeRawTileCache struct {
	mu        sync.Mutex
	directory string
	maxBytes  int64
	// policy decides whether verified resources reach the disk at all, and
	// whether the disk may answer once the provider cannot. It is the app's
	// MapCachePolicy, not a renderer detail: Memory Only must leave nothing
	// behind on the device.
	policy app.MapCachePolicy
}

type nativeRawCacheFile struct {
	path     string
	size     int64
	modified time.Time
}

func newDefaultNativeRawTileCache() *nativeRawTileCache {
	directory, err := os.UserCacheDir()
	if err != nil || directory == "" {
		return &nativeRawTileCache{}
	}
	return newNativeRawTileCache(filepath.Join(directory, "poem", "maps", "resources-v1"), 2<<30)
}

func newNativeRawTileCache(directory string, maxBytes int64) *nativeRawTileCache {
	// Persist by default. MapCacheMemoryOnly is the zero value of the policy
	// enum, so leaving the field unset would silently disable the disk cache
	// for every caller that never calls SetPolicy.
	return &nativeRawTileCache{directory: directory, maxBytes: maxBytes, policy: app.MapCachePersistentOffline}
}

// SetPolicy applies the viewport's cache policy. Switching to Memory Only also
// clears what earlier policies persisted: leaving a disk copy behind after the
// user asks for memory-only caching would defeat the setting.
func (cache *nativeRawTileCache) SetPolicy(policy app.MapCachePolicy) {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	changed := cache.policy != policy
	cache.policy = policy
	cache.mu.Unlock()
	if changed && policy == app.MapCacheMemoryOnly {
		cache.Clear()
	}
}

// persists reports whether verified resources may be written to disk.
func (cache *nativeRawTileCache) persists() bool {
	if cache == nil {
		return false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.policy != app.MapCacheMemoryOnly
}

// servesOffline reports whether the disk may answer when the provider cannot.
// Persistent Online Only is an accelerator, so a failed fetch is a failure;
// only Persistent Offline is allowed to keep drawing without a provider.
func (cache *nativeRawTileCache) servesOffline() bool {
	if cache == nil {
		return false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.policy == app.MapCachePersistentOffline
}

// Clear removes every cached resource. It only ever touches this cache's own
// directory, so downloaded region packages — which live in the application's
// data directory, not the OS cache directory — survive: clearing a cache must
// never cost the user a deliberate offline download.
func (cache *nativeRawTileCache) Clear() {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.directory == "" {
		return
	}
	entries, err := os.ReadDir(cache.directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(cache.directory, entry.Name()))
	}
}

func (cache *nativeRawTileCache) SetBudget(maxBytes int64) {
	if cache == nil || maxBytes <= 0 {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.maxBytes = maxBytes
	cache.evictLocked()
}

func (cache *nativeRawTileCache) Get(key nativeTileCacheKey) ([]byte, bool) {
	if cache == nil || cache.directory == "" {
		return nil, false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	path := cache.path(key)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= int64(nativeRawCacheHeaderBytes) || info.Size() > int64(nativeRawCacheHeaderBytes+app.MaxMapResourceBytes) {
		if err == nil {
			_ = os.Remove(path)
		}
		return nil, false
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	encoded, err := io.ReadAll(io.LimitReader(file, int64(nativeRawCacheHeaderBytes+app.MaxMapResourceBytes+1)))
	closeErr := file.Close()
	if err != nil || closeErr != nil || len(encoded) != int(info.Size()) {
		_ = os.Remove(path)
		return nil, false
	}
	payload, ok := decodeNativeRawCacheFile(encoded)
	if !ok {
		_ = os.Remove(path)
		return nil, false
	}
	return payload, true
}

func (cache *nativeRawTileCache) Put(key nativeTileCacheKey, payload []byte) {
	if cache == nil || cache.directory == "" || len(payload) == 0 || len(payload) > app.MaxMapResourceBytes {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	// Memory Only keeps resources in the in-memory tile cache alone.
	if cache.policy == app.MapCacheMemoryOnly {
		return
	}
	if int64(len(payload)+nativeRawCacheHeaderBytes) > cache.maxBytes || os.MkdirAll(cache.directory, 0o700) != nil {
		return
	}
	target := cache.path(key)
	if info, err := os.Lstat(target); err == nil && info.Mode().IsRegular() {
		return
	}
	temporary, err := os.CreateTemp(cache.directory, ".poem-map-*")
	if err != nil {
		return
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	encoded := encodeNativeRawCacheFile(payload)
	if _, err = temporary.Write(encoded); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil || os.Rename(temporaryPath, target) != nil {
		return
	}
	cache.evictLocked()
}

func (cache *nativeRawTileCache) Remove(key nativeTileCacheKey) {
	if cache == nil || cache.directory == "" {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	_ = os.Remove(cache.path(key))
}

func (cache *nativeRawTileCache) path(key nativeTileCacheKey) string {
	identity := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d\x00%d\x00%d", key.Provider, key.Source, key.Snapshot, key.Kind, key.Tile.Z, key.Tile.X, key.Tile.Y)
	digest := sha256.Sum256([]byte(identity))
	return filepath.Join(cache.directory, fmt.Sprintf("%x.raw", digest))
}

func (cache *nativeRawTileCache) evictLocked() {
	if cache.directory == "" || cache.maxBytes <= 0 {
		return
	}
	entries, err := os.ReadDir(cache.directory)
	if err != nil {
		return
	}
	files := make([]nativeRawCacheFile, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".raw" {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			continue
		}
		file := nativeRawCacheFile{path: filepath.Join(cache.directory, entry.Name()), size: info.Size(), modified: info.ModTime()}
		files = append(files, file)
		total += file.size
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modified.Equal(files[j].modified) {
			return files[i].path < files[j].path
		}
		return files[i].modified.Before(files[j].modified)
	})
	for _, file := range files {
		if total <= cache.maxBytes {
			break
		}
		if os.Remove(file.path) == nil {
			total -= file.size
		}
	}
}

func encodeNativeRawCacheFile(payload []byte) []byte {
	encoded := make([]byte, nativeRawCacheHeaderBytes+len(payload))
	copy(encoded, nativeRawCacheMagic[:])
	digest := sha256.Sum256(payload)
	copy(encoded[len(nativeRawCacheMagic):], digest[:])
	binary.LittleEndian.PutUint32(encoded[len(nativeRawCacheMagic)+sha256.Size:], uint32(len(payload)))
	copy(encoded[nativeRawCacheHeaderBytes:], payload)
	return encoded
}

func decodeNativeRawCacheFile(encoded []byte) ([]byte, bool) {
	if len(encoded) <= nativeRawCacheHeaderBytes || !bytes.Equal(encoded[:len(nativeRawCacheMagic)], nativeRawCacheMagic[:]) {
		return nil, false
	}
	length := binary.LittleEndian.Uint32(encoded[len(nativeRawCacheMagic)+sha256.Size:])
	if length == 0 || length > app.MaxMapResourceBytes || int(length) != len(encoded)-nativeRawCacheHeaderBytes {
		return nil, false
	}
	payload := encoded[nativeRawCacheHeaderBytes:]
	digest := sha256.Sum256(payload)
	if !bytes.Equal(digest[:], encoded[len(nativeRawCacheMagic):len(nativeRawCacheMagic)+sha256.Size]) {
		return nil, false
	}
	return append([]byte(nil), payload...), true
}

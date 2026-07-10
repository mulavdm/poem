// Package state provides retained transient interaction state keyed by stable
// component IDs. Application/domain values should remain owned by the app.
package state

import "sync"

// Store is safe for concurrent inspection and UI-thread updates. Published
// values are treated as immutable; callers replace a value instead of mutating
// shared data in place.
type Store struct {
	mu       sync.RWMutex
	values   map[string]any
	revision uint64
}

func NewStore() *Store { return &Store{values: make(map[string]any)} }

func (s *Store) Get(key string) (any, bool) {
	if s == nil || key == "" {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[key]
	return value, ok
}

func (s *Store) Set(key string, value any) uint64 {
	if s == nil || key == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		s.values = make(map[string]any)
	}
	s.values[key] = value
	s.revision++
	return s.revision
}

func (s *Store) Delete(key string) bool {
	if s == nil || key == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.values[key]; !ok {
		return false
	}
	delete(s.values, key)
	s.revision++
	return true
}

func (s *Store) DeletePrefix(prefix string) int {
	if s == nil || prefix == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for key := range s.values {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(s.values, key)
			removed++
		}
	}
	if removed > 0 {
		s.revision++
	}
	return removed
}

func (s *Store) Revision() uint64 {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.revision
}

func (s *Store) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.values)
}

// Load performs a type-safe read. A mismatched type is reported as missing.
func Load[T any](s *Store, key string) (T, bool) {
	var zero T
	value, ok := s.Get(key)
	if !ok {
		return zero, false
	}
	typed, ok := value.(T)
	if !ok {
		return zero, false
	}
	return typed, true
}

func StoreValue[T any](s *Store, key string, value T) uint64 { return s.Set(key, value) }

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// StateStore persists one application state value per signed session ID.
// Implementations must make Save atomic and must not trust session IDs as
// filesystem paths. A store may be backed by a database, service, or local
// files; Trellis serializes requests for its built-in stores.
type StateStore[S any] interface {
	Load(context.Context, string) (state S, found bool, err error)
	Save(context.Context, string, S) error
	Delete(context.Context, string) error
}

type memoryStateStore[S any] struct {
	mu     sync.Mutex
	init   S
	states map[string]S
}

func newMemoryStateStore[S any](init S) *memoryStateStore[S] {
	return &memoryStateStore[S]{init: init, states: make(map[string]S)}
}

func (s *memoryStateStore[S]) Load(ctx context.Context, id string) (S, bool, error) {
	if err := ctx.Err(); err != nil {
		return s.init, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.states[id]
	if !ok {
		return s.init, false, nil
	}
	return state, true, nil
}

func (s *memoryStateStore[S]) Save(ctx context.Context, id string, state S) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.states[id] = state
	s.mu.Unlock()
	return nil
}

func (s *memoryStateStore[S]) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.states, id)
	s.mu.Unlock()
	return nil
}

// FileStateStore persists JSON-serializable state values as one file per
// session. Writes use a temporary file and rename, so a process interruption
// cannot leave a partially written state file. It is intended for a single
// server instance; use a transactional database-backed StateStore for
// multi-instance deployments.
type FileStateStore[S any] struct {
	dir  string
	init S
	mu   sync.Mutex
}

type fileStateRecord[S any] struct {
	State    S         `json:"state"`
	LastSeen time.Time `json:"last_seen"`
}

// NewFileStateStore creates a durable local store rooted at dir.
func NewFileStateStore[S any](dir string, init S) (*FileStateStore[S], error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("trellis: file state store directory is empty")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("trellis: create state store directory: %w", err)
	}
	return &FileStateStore[S]{dir: dir, init: init}, nil
}

func (s *FileStateStore[S]) path(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\\`) || id == "." || id == ".." {
		return "", errors.New("trellis: invalid session ID")
	}
	return filepath.Join(s.dir, id+".json"), nil
}

// Load reads a session state. A missing file returns found=false.
func (s *FileStateStore[S]) Load(ctx context.Context, id string) (S, bool, error) {
	var zero S
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	path, err := s.path(id)
	if err != nil {
		return zero, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s.init, false, nil
	}
	if err != nil {
		return zero, false, fmt.Errorf("trellis: read session state: %w", err)
	}
	var record fileStateRecord[S]
	if err := json.Unmarshal(data, &record); err != nil {
		return zero, false, fmt.Errorf("trellis: decode session state: %w", err)
	}
	return record.State, true, nil
}

// Save atomically writes a session state.
func (s *FileStateStore[S]) Save(ctx context.Context, id string, state S) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.path(id)
	if err != nil {
		return err
	}
	data, err := json.Marshal(fileStateRecord[S]{State: state, LastSeen: time.Now().UTC()})
	if err != nil {
		return fmt.Errorf("trellis: encode session state: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp, err := os.CreateTemp(s.dir, ".trellis-state-*")
	if err != nil {
		return fmt.Errorf("trellis: create state temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("trellis: secure state temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("trellis: write session state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("trellis: sync session state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("trellis: close session state: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("trellis: commit session state: %w", err)
	}
	return nil
}

// Delete removes a persisted session state.
func (s *FileStateStore[S]) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.path(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("trellis: delete session state: %w", err)
	}
	return nil
}

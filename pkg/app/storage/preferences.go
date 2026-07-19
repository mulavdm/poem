// Package storage provides bounded standard-library implementations of
// POEM's non-secret platform storage contracts.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FilePreferences is an atomic, bounded non-secret preference store rooted
// in an application-owned directory. It must not be used for credentials.
type FilePreferences struct {
	mu        sync.RWMutex
	directory string
	maxBytes  int64
}

// NewFilePreferences creates a preference store in directory. maxBytes
// bounds each value; zero selects 64 KiB.
func NewFilePreferences(directory string, maxBytes int64) (*FilePreferences, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, errors.New("poem storage: empty preference directory")
	}
	if maxBytes == 0 {
		maxBytes = 64 << 10
	}
	if maxBytes < 1 || maxBytes > 16<<20 {
		return nil, errors.New("poem storage: invalid preference value bound")
	}
	absolute, err := filepath.Abs(directory)
	if err != nil || filepath.Clean(absolute) == filepath.VolumeName(absolute)+string(filepath.Separator) {
		return nil, errors.New("poem storage: invalid preference directory")
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("poem storage: create preference directory: %w", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("poem storage: unsafe preference directory")
	}
	return &FilePreferences{directory: absolute, maxBytes: maxBytes}, nil
}

// Get returns an immutable copy of one preference value.
func (store *FilePreferences) Get(ctx context.Context, key string) ([]byte, error) {
	if err := validPreferenceContext(ctx); err != nil {
		return nil, err
	}
	path, err := store.path(key)
	if err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > store.maxBytes {
		return nil, errors.New("poem storage: unsafe preference value")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	value, readErr := io.ReadAll(io.LimitReader(file, store.maxBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(value)) > store.maxBytes {
		return nil, errors.New("poem storage: preference value exceeds bound")
	}
	return value, nil
}

// Put atomically replaces one bounded preference value.
func (store *FilePreferences) Put(ctx context.Context, key string, value []byte) error {
	if err := validPreferenceContext(ctx); err != nil {
		return err
	}
	path, err := store.path(key)
	if err != nil {
		return err
	}
	if len(value) == 0 || int64(len(value)) > store.maxBytes {
		return errors.New("poem storage: invalid preference value")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	temporary, err := os.CreateTemp(store.directory, ".poem-preference-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(value)
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if info, statErr := os.Lstat(path); statErr == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return errors.New("poem storage: unsafe preference target")
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

// Delete removes one preference value. A missing value is not an error.
func (store *FilePreferences) Delete(ctx context.Context, key string) error {
	if err := validPreferenceContext(ctx); err != nil {
		return err
	}
	path, err := store.path(key)
	if err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("poem storage: unsafe preference target")
	}
	return os.Remove(path)
}

func (store *FilePreferences) path(key string) (string, error) {
	if store == nil || store.directory == "" || len(key) == 0 || len(key) > 128 {
		return "", errors.New("poem storage: invalid preference key")
	}
	for _, character := range key {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '.' || character == '_' || character == '-') {
			return "", errors.New("poem storage: invalid preference key")
		}
	}
	return filepath.Join(store.directory, key+".json"), nil
}

func validPreferenceContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("poem storage: nil context")
	}
	return ctx.Err()
}

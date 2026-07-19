package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFilePreferencesRoundTripReplaceDeleteAndBounds(t *testing.T) {
	store, err := NewFilePreferences(t.TempDir(), 32)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Put(ctx, "map.settings-v1", []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, "map.settings-v1", []byte("two")); err != nil {
		t.Fatal(err)
	}
	value, err := store.Get(ctx, "map.settings-v1")
	if err != nil || string(value) != "two" {
		t.Fatalf("value=%q err=%v", value, err)
	}
	value[0] = 'X'
	again, _ := store.Get(ctx, "map.settings-v1")
	if string(again) != "two" {
		t.Fatal("returned mutable storage bytes")
	}
	if err := store.Put(ctx, "large", bytes.Repeat([]byte{1}, 33)); err == nil {
		t.Fatal("accepted oversized value")
	}
	if err := store.Delete(ctx, "map.settings-v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "map.settings-v1"); !os.IsNotExist(err) {
		t.Fatalf("deleted Get error=%v", err)
	}
}

func TestFilePreferencesRejectTraversalSymlinkAndCancellation(t *testing.T) {
	directory := t.TempDir()
	store, err := NewFilePreferences(directory, 32)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"../secret", "nested/value", "", "bad key"} {
		if err := store.Put(context.Background(), key, []byte("x")); err == nil {
			t.Fatalf("accepted key %q", key)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Put(ctx, "cancelled", []byte("x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
	target := filepath.Join(directory, "target.json")
	if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "linked.json")
	if err := os.Symlink(target, link); err == nil {
		if _, err := store.Get(context.Background(), "linked"); err == nil {
			t.Fatal("read symlink target")
		}
		if err := store.Put(context.Background(), "linked", []byte("replace")); err == nil {
			t.Fatal("replaced symlink target")
		}
	}
}

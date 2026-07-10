//go:build windows && integration

package windows

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClipboardUnicodeRoundTrip(t *testing.T) {
	clipboard := NewClipboard()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	original, err := clipboard.ReadText(ctx)
	if err != nil {
		t.Skipf("clipboard unavailable: %v", err)
	}
	t.Cleanup(func() {
		restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer restoreCancel()
		_ = clipboard.WriteText(restoreCtx, original)
	})
	want := "POEM clipboard ✓ 日本語"
	if err := clipboard.WriteText(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := clipboard.ReadText(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("clipboard text = %q, want %q", got, want)
	}
}

func TestClipboardRejectsOversizedText(t *testing.T) {
	clipboard := NewClipboard()
	err := clipboard.WriteText(context.Background(), strings.Repeat("x", maxClipboardByte/2+1))
	if err == nil {
		t.Fatal("oversized clipboard write was accepted")
	}
}

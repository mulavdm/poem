//go:build windows

package windows

import (
	"context"
	"testing"

	"github.com/mulavdm/poem/pkg/render/platform"
	"github.com/mulavdm/poem/pkg/render/theme"
)

var _ platform.SystemPreferences = (*SystemPreferences)(nil)

func TestSystemPreferencesReturnsSupportedMode(t *testing.T) {
	snapshot, err := NewSystemPreferences().Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ThemeMode != theme.ModeDark && snapshot.ThemeMode != theme.ModeLight && snapshot.ThemeMode != theme.ModeHighContrast {
		t.Fatalf("unsupported theme mode %q", snapshot.ThemeMode)
	}
}

func TestSystemPreferencesHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewSystemPreferences().Current(ctx); err == nil {
		t.Fatal("canceled preference query succeeded")
	}
}

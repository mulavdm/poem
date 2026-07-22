package components

import (
	"image"
	"image/color"
	"testing"

	"github.com/mulavdm/poem/pkg/render/types"

	"github.com/mulavdm/poem/pkg/render/theme"
)

func TestBadgeVisualGivesDistinctColorsForSemanticVariants(t *testing.T) {
	th := theme.ModernDark()
	cases := []struct {
		variant theme.Variant
		want    color.RGBA
	}{
		{theme.VariantPrimary, th.Colors.Accent},
		{theme.VariantDanger, th.Colors.Danger},
		{theme.VariantSuccess, th.Colors.Success},
		{theme.VariantWarning, th.Colors.Warning},
	}
	seen := map[color.RGBA]theme.Variant{}
	for _, test := range cases {
		_, fg := badgeVisual(th, test.variant)
		if fg != test.want {
			t.Fatalf("variant %v foreground=%v, want %v", test.variant, fg, test.want)
		}
		if other, ok := seen[fg]; ok {
			t.Fatalf("variant %v and %v produced the same foreground color %v", test.variant, other, fg)
		}
		seen[fg] = test.variant
	}
}

func TestBadgeVisualFallsBackToMutedForStructuralVariants(t *testing.T) {
	th := theme.ModernDark()
	for _, variant := range []theme.Variant{theme.VariantNeutral, theme.VariantSecondary, theme.VariantSubtle} {
		_, fg := badgeVisual(th, variant)
		if fg != th.Colors.TextMuted {
			t.Fatalf("variant %v foreground=%v, want the muted default theme.Colors.TextMuted (%v)", variant, fg, th.Colors.TextMuted)
		}
	}
}

func TestBadgeVisualBackgroundIsTintedNotFlat(t *testing.T) {
	th := theme.ModernDark()
	bg, fg := badgeVisual(th, theme.VariantDanger)
	if bg == fg {
		t.Fatalf("expected a tinted background distinct from the full-strength foreground, got bg=fg=%v", bg)
	}
	if bg == th.Colors.SurfaceRaised {
		t.Fatalf("expected the background to be tinted toward the variant color, got the untinted surface %v", bg)
	}
}

// A status badge must size to its label and not freeze at its laid-out width;
// "Engine unavailable" shipped as "Engine unava…" because Badge.Measure read
// its stale Rect as an explicit size (design lint UI101).
func TestBadgeSizesToLabelNotStaleBounds(t *testing.T) {
	const charW = 8
	badge := NewBadge("status", "Engine unavailable")
	state := &types.ApplicationState{FontCharWidth: charW}

	// A prior layout at a cramped width must not pin the badge.
	badge.SetBounds(image.Rect(0, 0, 90, 24))

	result := badge.Measure(image.Pt(1200, 80), state)
	labelWidth := len([]rune("Engine unavailable")) * charW
	if result.Preferred.X < labelWidth || result.Min.X < labelWidth {
		t.Fatalf("badge froze narrower than its label: preferred %d, min %d, label %d", result.Preferred.X, result.Min.X, labelWidth)
	}
	if fitted := fitButtonLabel("Engine unavailable", result.Min.X, charW); fitted != "Engine unavailable" {
		t.Fatalf("badge label truncated at its own width: %q", fitted)
	}
}

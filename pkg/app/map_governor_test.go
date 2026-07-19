package app

import (
	"testing"
	"time"
)

func observeN(g *QualityGovernor, frame time.Duration, n int) MapQuality {
	var q MapQuality
	for i := 0; i < n; i++ {
		q = g.Observe(frame)
	}
	return q
}

func TestGovernorExplicitTierHonoredButCapped(t *testing.T) {
	// Explicit High under a Balanced ceiling resolves to Balanced.
	g := NewQualityGovernor(MapQualityHigh, MapQualityBalanced, 60)
	if got := observeN(g, 4*time.Millisecond, 100); got != MapQualityBalanced {
		t.Fatalf("explicit High capped = %v, want Balanced", got)
	}
	// Explicit Battery Saver is honored even when frames are fast.
	g2 := NewQualityGovernor(MapQualityBatterySaver, MapQualityHigh, 60)
	if got := observeN(g2, 2*time.Millisecond, 100); got != MapQualityBatterySaver {
		t.Fatalf("explicit BatterySaver = %v", got)
	}
}

func TestGovernorAutoDowngradesUnderLoad(t *testing.T) {
	g := NewQualityGovernor(MapQualityAuto, MapQualityHigh, 60) // starts Balanced
	// Slow frames (40ms >> 16.7ms budget): should step down to Battery Saver.
	got := observeN(g, 40*time.Millisecond, minGovernorDwell*3)
	if got != MapQualityBatterySaver {
		t.Fatalf("auto under load = %v (ema %.1f), want BatterySaver", got, g.SmoothedFrameMillis())
	}
}

func TestGovernorAutoUpgradesWhenFast(t *testing.T) {
	g := NewQualityGovernor(MapQualityAuto, MapQualityHigh, 60) // starts Balanced
	got := observeN(g, 5*time.Millisecond, minGovernorDwell*3)  // well under budget
	if got != MapQualityHigh {
		t.Fatalf("auto when fast = %v, want High", got)
	}
}

func TestGovernorAutoRespectsCeiling(t *testing.T) {
	g := NewQualityGovernor(MapQualityAuto, MapQualityBalanced, 60)
	if got := observeN(g, 3*time.Millisecond, minGovernorDwell*3); got != MapQualityBalanced {
		t.Fatalf("auto should not exceed ceiling: %v", got)
	}
}

func TestGovernorHysteresisPreventsImmediateFlip(t *testing.T) {
	g := NewQualityGovernor(MapQualityAuto, MapQualityHigh, 60) // Balanced
	// A single slow frame before the dwell window must not change the tier.
	if got := g.Observe(50 * time.Millisecond); got != MapQualityBalanced {
		t.Fatalf("flipped before dwell elapsed: %v", got)
	}
	// Only after the dwell window does it step down (one tier at a time).
	got := observeN(g, 50*time.Millisecond, minGovernorDwell)
	if got != MapQualityBalanced-1 {
		t.Fatalf("expected single step to %v, got %v", MapQualityBalanced-1, got)
	}
}

func TestQualityCeiling(t *testing.T) {
	if QualityCeiling(false, 8192, 0) != MapQualityBatterySaver {
		t.Fatal("no retained meshes must floor to BatterySaver")
	}
	if QualityCeiling(true, 2048, 0) != MapQualityBalanced {
		t.Fatal("small texture size must cap at Balanced")
	}
	if QualityCeiling(true, 8192, 256<<20) != MapQualityBalanced {
		t.Fatal("tight memory must cap at Balanced")
	}
	if QualityCeiling(true, 8192, 2<<30) != MapQualityHigh {
		t.Fatal("capable GPU should allow High")
	}
}

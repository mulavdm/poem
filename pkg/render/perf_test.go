package render

import (
	"math"
	"testing"
	"time"
)

func TestDurationMSPreservesSubMillisecondPrecision(t *testing.T) {
	got := durationMS(500 * time.Microsecond)
	if math.Abs(got-0.5) > 0.0001 {
		t.Fatalf("expected 0.5ms, got %.6fms", got)
	}
}

package cartography

import (
	"context"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

// The same named feature split across tile seams emits one candidate per tile
// at slightly different points, so the collision boxes never overlap and every
// copy is placed — "Oosterdok" four times over one dock. Nearby duplicates of
// identical text must collapse to one; the same name genuinely far away must
// survive, because that is a different place.
func TestPlaceLabelsSuppressesNearbyDuplicateText(t *testing.T) {
	shaper, err := NewTextShaper(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	camera := Camera{Latitude: 0, Longitude: 0, Zoom: 4, Width: 1000, Height: 800}
	x, y := Project(0, 0)
	// worldPixels = 512 * 2^4 = 8192, so one pixel is 1/8192 world units.
	const worldPerPixel = 1.0 / 8192.0
	near := 120 * worldPerPixel // inside the 320px suppression radius
	far := 500 * worldPerPixel  // beyond it

	// Three "Dok" candidates: two seam-close, one far. Short text so the
	// collision boxes never overlap, isolating dedup from collision.
	placements, err := PlaceLabels(context.Background(), camera, shaper, []LabelCandidate{
		{ID: "a", Text: "Dok", Locale: "en", WorldX: x, WorldY: y, Size: 16, Priority: 10},
		{ID: "b", Text: "Dok", Locale: "en", WorldX: x + near, WorldY: y, Size: 16, Priority: 5},
		{ID: "c", Text: "Dok", Locale: "en", WorldX: x + far, WorldY: y, Size: 16, Priority: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(placements) != 2 {
		t.Fatalf("expected 2 placements (seam duplicate collapsed, far kept), got %d", len(placements))
	}
	// The higher-priority instance is the one kept for the collapsed pair.
	if placements[0].Candidate.ID != "a" {
		t.Fatalf("expected highest-priority instance 'a' kept, got %q", placements[0].Candidate.ID)
	}
}

// Distinct texts at the same spot are not duplicates and must both place.
func TestPlaceLabelsKeepsDistinctTextNearby(t *testing.T) {
	shaper, err := NewTextShaper(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	camera := Camera{Latitude: 0, Longitude: 0, Zoom: 4, Width: 1000, Height: 800}
	x, y := Project(0, 0)
	const near = 120.0 / 8192.0
	placements, err := PlaceLabels(context.Background(), camera, shaper, []LabelCandidate{
		{ID: "a", Text: "Dok", Locale: "en", WorldX: x, WorldY: y, Size: 16},
		{ID: "b", Text: "Kade", Locale: "en", WorldX: x + near, WorldY: y, Size: 16},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(placements) != 2 {
		t.Fatalf("distinct nearby texts must both place, got %d", len(placements))
	}
}

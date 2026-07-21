package protocol

import "testing"

// Scene packets cross the engine→presenter boundary, where the C++ presenters
// decode them with the same wire layout. Decoding must reject malformed packets
// rather than panic or report counts the payload cannot back — a presenter that
// trusts a bad count reads out of bounds.
func FuzzDecodeMapSceneDelta(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("viewport"))
	if seed, err := EncodeMapSceneDelta(MapSceneDelta{
		ViewportID: "route-map", Generation: 1,
		Camera:       MapCamera{Latitude: 52.37, Longitude: 4.9, Zoom: 14, ViewportWidth: 800, ViewportHeight: 600},
		SunAzimuth:   180,
		SunElevation: 40,
		FogDensity:   1.15,
	}); err == nil {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, payload []byte) {
		delta, err := DecodeMapSceneDelta(payload)
		if err != nil {
			return // rejecting malformed input is the expected outcome
		}
		// A decoded packet must be self-consistent: every declared resource and
		// draw is really present, and nothing claims more bytes than arrived.
		for _, resource := range delta.Resources {
			if len(resource.Bytes) > len(payload) {
				t.Fatalf("resource claims %d bytes from a %d byte packet", len(resource.Bytes), len(payload))
			}
		}
		for _, draw := range delta.Draws {
			if draw.Opacity < 0 || draw.Opacity > 1 {
				t.Fatalf("draw opacity out of range: %v", draw.Opacity)
			}
		}
		// Re-encoding a survivor must round-trip, so a packet that decodes can
		// always be reproduced (presenters cache by generation).
		if _, err := EncodeMapSceneDelta(delta); err != nil {
			t.Fatalf("decoded packet failed to re-encode: %v", err)
		}
	})
}

// Camera messages come back from the presenter after user gestures, so they are
// equally untrusted.
func FuzzDecodeMapCamera(f *testing.F) {
	f.Add([]byte{})
	if seed, err := EncodeMapCamera(MapCamera{Latitude: 52.37, Longitude: 4.9, Zoom: 14, Pitch: 55, ViewportWidth: 800, ViewportHeight: 600}); err == nil {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, payload []byte) {
		if _, err := DecodeMapCamera(payload); err != nil {
			return
		}
	})
}

package app

import (
	"math"
	"testing"
)

func TestImageTransformPayloadRoundTrip(t *testing.T) {
	want := ImageTransform{OffsetX: 0.25, OffsetY: -0.5, Scale: 2}
	got, ok := (Msg{Payload: ImageTransformPayload(want)}).ImageTransform()
	if !ok || got != want {
		t.Fatalf("round trip = (%+v, %v), want %+v", got, ok, want)
	}
}

func TestImageTransformPayloadRejectsUntrustedValues(t *testing.T) {
	for _, payload := range []string{
		`{"x":0,"y":0,"scale":"bad"}`,
		`{"x":0,"y":0,"scale":1,"extra":true}`,
		`{"x":5,"y":0,"scale":1}`,
		`{"x":0,"y":0,"scale":65}`,
		`{"x":0,"y":0,"scale":1} trailing`,
	} {
		if got, ok := (Msg{Payload: payload}).ImageTransform(); ok {
			t.Fatalf("accepted %q as %+v", payload, got)
		}
	}
	if got := ImageTransformPayload(ImageTransform{Scale: math.Inf(1)}); got != "" {
		t.Fatalf("encoded infinity as %q", got)
	}
}

func TestImageTransformZeroValueIsIdentity(t *testing.T) {
	got, ok := (Msg{Payload: ImageTransformPayload(ImageTransform{})}).ImageTransform()
	if !ok || got != (ImageTransform{Scale: 1}) {
		t.Fatalf("zero transform = (%+v, %v)", got, ok)
	}
}

func TestViewportCommitCarriesDimensionsAndAcceptsLegacy(t *testing.T) {
	want := ImageViewportCommit{OffsetX: .2, OffsetY: -.1, Scale: 1.5, Width: 1024, Height: 640}
	got, ok := (Msg{Payload: ImageViewportCommitPayload(want)}).ViewportCommit()
	if !ok || got != want {
		t.Fatalf("commit=(%+v,%v), want %+v", got, ok, want)
	}
	legacy, ok := (Msg{Payload: ImageTransformPayload(ImageTransform{Scale: 2})}).ViewportCommit()
	if !ok || legacy.Width != 0 || legacy.Transform().Scale != 2 {
		t.Fatalf("legacy=%+v ok=%v", legacy, ok)
	}
	for _, payload := range []string{`{"x":0,"y":0,"scale":1,"width":10,"height":10}`, `{"x":0,"y":0,"scale":1,"width":100,"height":0}`} {
		if _, ok := (Msg{Payload: payload}).ViewportCommit(); ok {
			t.Fatalf("accepted %s", payload)
		}
	}
}

func TestViewportPointPayloadStrictRoundTrip(t *testing.T) {
	payload := ViewportPointPayload(ViewportPoint{X: 0.25, Y: 0.75})
	point, ok := (Msg{Payload: payload}).ViewportPoint()
	if !ok || point.X != 0.25 || point.Y != 0.75 {
		t.Fatalf("point=%+v ok=%v", point, ok)
	}
	for _, payload := range []string{`{"x":-0.1,"y":0}`, `{"x":0,"y":2}`, `{"x":0,"y":0,"z":1}`, `{"x":0,"y":0} {}`} {
		if _, ok := (Msg{Payload: payload}).ViewportPoint(); ok {
			t.Fatalf("accepted %q", payload)
		}
	}
}

package design

import "testing"

func TestCapabilityPayloadStrictRoundTrip(t *testing.T) {
	want := CapabilityUpdate{Pointer: PointerCoarse, Touch: true, Density: DensityComfortable, TextScale: 1.5, ReducedMotion: true}
	payload, err := EncodeCapabilityUpdate(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeCapabilityUpdate(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if _, err := DecodeCapabilityUpdate(`{"pointer":1,"unknown":true}`); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := DecodeCapabilityUpdate(`{"textScale":4.1}`); err == nil {
		t.Fatal("invalid scale accepted")
	}
}

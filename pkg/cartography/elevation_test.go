package cartography

import (
	"math"
	"reflect"
	"testing"
)

func TestElevationTileRoundTripAndBilinearSampling(t *testing.T) {
	tile := ElevationTile{Width: 2, Height: 2, Scale: .5, Offset: 10, Samples: []int16{0, 20, 40, 60}}
	encoded, err := EncodeElevationTile(tile)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeElevationTile(encoded)
	if err != nil || !reflect.DeepEqual(decoded, tile) {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	if got := decoded.ElevationAt(.5, .5); math.Abs(float64(got-25)) > 1e-6 {
		t.Fatalf("center elevation=%v", got)
	}
}

func TestElevationTileRejectsMalformedAndNonFiniteData(t *testing.T) {
	valid, err := EncodeElevationTile(ElevationTile{Width: 2, Height: 2, Scale: 1, Samples: []int16{1, 2, 3, 4}})
	if err != nil {
		t.Fatal(err)
	}
	invalidScale := append([]byte(nil), valid...)
	for index := 12; index < 16; index++ {
		invalidScale[index] = 0xff
	}
	for _, encoded := range [][]byte{nil, valid[:len(valid)-1], append(valid, 0), invalidScale} {
		if _, err := DecodeElevationTile(encoded); err == nil {
			t.Fatalf("accepted malformed elevation bytes=%d", len(encoded))
		}
	}
}

func FuzzDecodeElevationTile(f *testing.F) {
	seed, _ := EncodeElevationTile(ElevationTile{Width: 2, Height: 2, Scale: 1, Samples: []int16{1, 2, 3, 4}})
	f.Add(seed)
	f.Fuzz(func(t *testing.T, encoded []byte) { _, _ = DecodeElevationTile(encoded) })
}

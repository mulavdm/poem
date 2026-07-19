package cartography

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
)

var elevationMagic = [8]byte{'P', 'O', 'E', 'M', 'D', 'E', 'M', 1}

const (
	elevationHeaderBytes = 8 + 2 + 2 + 4 + 4
	maxElevationGrid     = 257
)

// ElevationTile is POEM's normalized, source-independent terrain grid.
// Samples are signed units transformed to metres by Scale and Offset.
type ElevationTile struct {
	// Width is the west-to-east sample count.
	Width uint16
	// Height is the north-to-south sample count.
	Height uint16
	// Scale converts one signed sample unit to metres.
	Scale float32
	// Offset is added after sample scaling, in metres.
	Offset float32
	// Samples are immutable row-major signed elevation units.
	Samples []int16
}

// Valid reports whether the normalized grid satisfies renderer bounds.
func (tile ElevationTile) Valid() bool {
	if tile.Width < 2 || tile.Height < 2 || tile.Width > maxElevationGrid || tile.Height > maxElevationGrid || len(tile.Samples) != int(tile.Width)*int(tile.Height) || !finite(float64(tile.Scale)) || !finite(float64(tile.Offset)) || tile.Scale <= 0 || tile.Scale > 1000 || math.Abs(float64(tile.Offset)) > 20_000 {
		return false
	}
	for _, sample := range tile.Samples {
		metres := float64(sample)*float64(tile.Scale) + float64(tile.Offset)
		if !finite(metres) || metres < -20_000 || metres > 20_000 {
			return false
		}
	}
	return true
}

// ElevationAt bilinearly samples normalized tile coordinates. U and V are
// clamped to [0,1], making shared tile-edge samples deterministic.
func (tile ElevationTile) ElevationAt(u, v float64) float32 {
	if !tile.Valid() || !finite(u) || !finite(v) {
		return 0
	}
	u, v = max(0, min(1, u)), max(0, min(1, v))
	x := u * float64(tile.Width-1)
	y := v * float64(tile.Height-1)
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := min(x0+1, int(tile.Width)-1), min(y0+1, int(tile.Height)-1)
	fx, fy := float32(x-float64(x0)), float32(y-float64(y0))
	sample := func(px, py int) float32 {
		return float32(tile.Samples[py*int(tile.Width)+px])*tile.Scale + tile.Offset
	}
	top := sample(x0, y0)*(1-fx) + sample(x1, y0)*fx
	bottom := sample(x0, y1)*(1-fx) + sample(x1, y1)*fx
	return top*(1-fy) + bottom*fy
}

// EncodeElevationTile validates and serializes one normalized terrain grid.
func EncodeElevationTile(tile ElevationTile) ([]byte, error) {
	if !tile.Valid() {
		return nil, errors.New("cartography: invalid elevation tile")
	}
	encoded := make([]byte, elevationHeaderBytes+len(tile.Samples)*2)
	copy(encoded, elevationMagic[:])
	binary.LittleEndian.PutUint16(encoded[8:], tile.Width)
	binary.LittleEndian.PutUint16(encoded[10:], tile.Height)
	binary.LittleEndian.PutUint32(encoded[12:], math.Float32bits(tile.Scale))
	binary.LittleEndian.PutUint32(encoded[16:], math.Float32bits(tile.Offset))
	for index, sample := range tile.Samples {
		binary.LittleEndian.PutUint16(encoded[elevationHeaderBytes+index*2:], uint16(sample))
	}
	return encoded, nil
}

// DecodeElevationTile strictly decodes one bounded normalized terrain grid.
func DecodeElevationTile(encoded []byte) (ElevationTile, error) {
	if len(encoded) < elevationHeaderBytes || !bytes.Equal(encoded[:8], elevationMagic[:]) {
		return ElevationTile{}, errors.New("cartography: invalid elevation header")
	}
	tile := ElevationTile{
		Width:  binary.LittleEndian.Uint16(encoded[8:]),
		Height: binary.LittleEndian.Uint16(encoded[10:]),
		Scale:  math.Float32frombits(binary.LittleEndian.Uint32(encoded[12:])),
		Offset: math.Float32frombits(binary.LittleEndian.Uint32(encoded[16:])),
	}
	count := int(tile.Width) * int(tile.Height)
	if tile.Width < 2 || tile.Height < 2 || tile.Width > maxElevationGrid || tile.Height > maxElevationGrid || count > maxElevationGrid*maxElevationGrid || len(encoded) != elevationHeaderBytes+count*2 {
		return ElevationTile{}, errors.New("cartography: invalid elevation dimensions")
	}
	tile.Samples = make([]int16, count)
	for index := range tile.Samples {
		tile.Samples[index] = int16(binary.LittleEndian.Uint16(encoded[elevationHeaderBytes+index*2:]))
	}
	if !tile.Valid() {
		return ElevationTile{}, errors.New("cartography: invalid elevation values")
	}
	return tile, nil
}

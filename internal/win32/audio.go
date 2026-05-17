package win32

import (
	"math"
)

// GenerateWav builds a perfect 44-byte RIFF/WAVE header followed by 16-bit Mono PCM data.
func GenerateWav(sampleRate int, samples []int16) []byte {
	subchunk2Size := len(samples) * 2
	chunkSize := 36 + subchunk2Size

	buf := make([]byte, 44+subchunk2Size)

	// RIFF Header
	copy(buf[0:4], "RIFF")
	buf[4] = byte(chunkSize)
	buf[5] = byte(chunkSize >> 8)
	buf[6] = byte(chunkSize >> 16)
	buf[7] = byte(chunkSize >> 24)
	copy(buf[8:12], "WAVE")

	// fmt Subchunk
	copy(buf[12:16], "fmt ")
	buf[16] = 16 // Subchunk1Size (16 for PCM)
	buf[17] = 0
	buf[18] = 0
	buf[19] = 0
	buf[20] = 1 // AudioFormat (1 = PCM)
	buf[21] = 0
	buf[22] = 1 // NumChannels (1 = Mono)
	buf[23] = 0

	// SampleRate
	buf[24] = byte(sampleRate)
	buf[25] = byte(sampleRate >> 8)
	buf[26] = byte(sampleRate >> 16)
	buf[27] = byte(sampleRate >> 24)

	// ByteRate = SampleRate * NumChannels * BitsPerSample/8 = SampleRate * 2
	byteRate := sampleRate * 2
	buf[28] = byte(byteRate)
	buf[29] = byte(byteRate >> 8)
	buf[30] = byte(byteRate >> 16)
	buf[31] = byte(byteRate >> 24)

	// BlockAlign = NumChannels * BitsPerSample/8 = 2
	buf[32] = 2
	buf[33] = 0

	// BitsPerSample = 16
	buf[34] = 16
	buf[35] = 0

	// data Subchunk
	copy(buf[36:40], "data")
	buf[40] = byte(subchunk2Size)
	buf[41] = byte(subchunk2Size >> 8)
	buf[42] = byte(subchunk2Size >> 16)
	buf[43] = byte(subchunk2Size >> 24)

	// Write 16-bit samples as little-endian bytes
	for i, s := range samples {
		idx := 44 + i*2
		buf[idx] = byte(s)
		buf[idx+1] = byte(s >> 8)
	}

	return buf
}

// SynthesizeHover creates a high-frequency, ultra-short, soft tick for mouse hover events.
func SynthesizeHover() []byte {
	sampleRate := 44100
	duration := 0.015 // 15 milliseconds
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]int16, numSamples)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)

		// Instant attack, very rapid exponential decay
		envelope := math.Exp(-t * 220.0)

		// Ultra-clean sine tick at 1200 Hz
		val := math.Sin(2.0 * math.Pi * 1200.0 * t)
		samples[i] = int16(val * 12000.0 * envelope) // Soft volume
	}

	return GenerateWav(sampleRate, samples)
}

// SynthesizeClick creates a premium double-frequency chime with smooth exponential decay.
func SynthesizeClick() []byte {
	sampleRate := 44100
	duration := 0.12 // 120 milliseconds
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]int16, numSamples)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)

		// Premium exponential decay
		envelope := math.Exp(-t * 28.0)

		// Dual blended frequencies: 900 Hz fundamental + 1800 Hz octave harmonic
		val1 := math.Sin(2.0 * math.Pi * 900.0 * t)
		val2 := math.Sin(2.0*math.Pi*1800.0*t) * 0.4
		sampleVal := (val1 + val2) / 1.4

		samples[i] = int16(sampleVal * 28000.0 * envelope)
	}

	return GenerateWav(sampleRate, samples)
}

// SynthesizeSuccess creates an ascending sci-fi arpeggio sequence for saving/submitting.
func SynthesizeSuccess() []byte {
	sampleRate := 44100

	// Notes: E5 (659.25), A5 (880.0), C#6 (1109.73), E6 (1318.51)
	noteFreqs := []float64{659.25, 880.0, 1109.73, 1318.51}
	noteDuration := 0.10 // 100ms per note
	totalDuration := noteDuration * float64(len(noteFreqs))
	numSamples := int(float64(sampleRate) * totalDuration)
	samples := make([]int16, numSamples)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)

		noteIdx := int(t / noteDuration)
		if noteIdx >= len(noteFreqs) {
			noteIdx = len(noteFreqs) - 1
		}

		freq := noteFreqs[noteIdx]
		// Local time within the current note block
		noteT := t - float64(noteIdx)*noteDuration

		// Decaying envelope inside each note segment
		envelope := math.Exp(-noteT * 18.0)

		val := math.Sin(2.0 * math.Pi * freq * t)
		samples[i] = int16(val * 24000.0 * envelope)
	}

	return GenerateWav(sampleRate, samples)
}

//go:build !gpu

package render

import (
	"image"
	"image/color"
	"testing"
)

// BenchmarkPaint measures the time it takes to render a full frame on the CPU
func BenchmarkPaint(b *testing.B) {
	engine := &CPUEngine{}
	engine.Setup(0) // HDC not needed for setup in CPU mode

	state := &ApplicationState{
		StatusText: "Benchmarking",
		Components: []Component{
			&Panel{CompID: "bg", Rect: image.Rect(0, 0, Width, Height), BGColor: color.RGBA{10, 10, 15, 255}},
			&Panel{CompID: "sidebar", Rect: image.Rect(0, 0, 70, Height), BGColor: color.RGBA{15, 15, 25, 255}},
			&Button{CompID: "btn", Rect: image.Rect(100, 100, 300, 150), Label: "Benchmark", BaseColor: color.RGBA{60, 80, 120, 255}},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Pass 0 as HDC because CPU engine only uses it for StretchDIBits at the end, 
		// but we should probably mock StretchDIBits if we want to test the full pipeline.
		// For now, we are testing the rasterization logic.
		engine.Paint(0, state)
	}
}

// BenchmarkDrawRoundedRect specifically targets the most expensive drawing operation
func BenchmarkDrawRoundedRect(b *testing.B) {
	engine := &CPUEngine{}
	engine.Setup(0)
	rect := image.Rect(0, 0, 500, 500)
	col := color.RGBA{255, 0, 0, 128}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.drawRoundedRect(rect, 50, col)
	}
}

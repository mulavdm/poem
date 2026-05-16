//go:build !gpu

package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"time"
	"unsafe"

	"go_native_gpu_gui/internal/win32"
)

type CPUEngine struct {
	canvas     *image.RGBA
	bitmapInfo win32.BITMAPINFO
}

func New(hdc uintptr) (UIRenderer, error) {
	fmt.Println("⚡ Factory: Spawning Zero-Dependency CPU Engine")
	return &CPUEngine{}, nil
}

func (c *CPUEngine) Setup(hdc uintptr) error {
	c.SetSize(Width, Height)
	return nil
}

func (c *CPUEngine) SetSize(w, h int) {
	c.canvas = image.NewRGBA(image.Rect(0, 0, w, h))
	c.bitmapInfo.Header = win32.BITMAPINFOHEADER{
		Size:        int32(unsafe.Sizeof(c.bitmapInfo.Header)),
		Width:       int32(w),
		Height:      int32(-h), // Top-down
		Planes:      1,
		BitCount:    32,
		Compression: 0,
	}
}

// Painter Interface Implementation
func (c *CPUEngine) DrawRoundedRect(r image.Rectangle, radius int, col color.RGBA) {
	c.drawRoundedRect(r, radius, col)
}

func (c *CPUEngine) DrawText(s string, x, y int, col color.RGBA) {
	c.drawText(s, x, y, col)
}

func (c *CPUEngine) FillRect(r image.Rectangle, col color.RGBA) {
	draw.Draw(c.canvas, r, &image.Uniform{col}, image.Point{}, draw.Src)
}

func (c *CPUEngine) Paint(hdc uintptr, state *ApplicationState) {
	// Reset Hover
	state.HoveredID = ""

	// 1. BACKGROUND GRADIENT (Deep Industrial Blue)
	for y := 0; y < Height; y++ {
		ratio := float64(y) / float64(Height)
		r := uint8(10 + ratio*10)
		g := uint8(12 + ratio*12)
		b := uint8(20 + ratio*15)
		line := c.canvas.Pix[y*c.canvas.Stride : (y+1)*c.canvas.Stride]
		for x := 0; x < Width; x++ {
			line[x*4] = b 
			line[x*4+1] = g
			line[x*4+2] = r
			line[x*4+3] = 255
		}
	}

	// 2. COMPONENT PASS (Hit Testing)
	mousePoint := image.Point{state.MouseX, state.MouseY}
	state.HoveredID = "" // Reset
	for i := len(state.Components) - 1; i >= 0; i-- {
		comp := state.Components[i]
		if id := comp.HitTest(mousePoint); id != "" {
			state.HoveredID = id
			break
		}
	}

	// 3. COMPONENT PASS (Drawing)
	for _, comp := range state.Components {
		comp.Draw(c, state)
	}

	// 4. MANUAL OVERLAYS (Performance pulse, etc.)
	elapsed := time.Since(state.StartTime).Seconds()
	pulse := math.Sin(elapsed*3) * 5
	c.drawRoundedRect(image.Rect(650-int(pulse), 220-int(pulse), 750+int(pulse), 320+int(pulse)), 50, color.RGBA{0, 255, 200, 100})

	// 5. BLIT TO MONITOR
	win32.StretchDIBits(
		hdc, 0, 0, int32(Width), int32(Height), 0, 0, int32(Width), int32(Height),
		uintptr(unsafe.Pointer(&c.canvas.Pix[0])),
		&c.bitmapInfo,
	)
}

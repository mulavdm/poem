//go:build !gpu

package backend

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"unsafe"

	"go_native_gpu_gui/internal/win32"
	"go_native_gpu_gui/pkg/render/types"
)

type CPUEngine struct {
	canvas      *image.RGBA
	bitmapInfo  win32.BITMAPINFO
	offsetX     float32
	offsetY     float32
	clipRect    image.Rectangle
	clipEnabled bool
}

func New(hdc uintptr) (types.UIRenderer, error) {
	fmt.Println("⚡ Factory: Spawning Zero-Dependency CPU Engine")
	return &CPUEngine{}, nil
}

func (c *CPUEngine) Setup(hdc uintptr) error {
	c.SetSize(types.Width, types.Height)
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

func (c *CPUEngine) SetGlow(strength float32) {
	// CPU doesn't support glow yet - maintaining zero-dependency performance promise
}

func (c *CPUEngine) SetGlass(enabled bool) {
	// CPU doesn't support real-time glassmorphism yet
}

func (c *CPUEngine) SetShadow(ox, oy, blur float32) {
	// CPU doesn't support SDF shadows yet
}

func (c *CPUEngine) SetOffset(x, y float32) {
	c.offsetX = x
	c.offsetY = y
}

func (c *CPUEngine) SetClip(r image.Rectangle) {
	if r.Empty() {
		c.clipEnabled = false
	} else {
		c.clipEnabled = true
		c.clipRect = r
	}
}

func (c *CPUEngine) Flush() {
	// CPU rendering is immediate, no flushing needed
}

func (c *CPUEngine) DrawText(s string, x, y int, col color.RGBA) {
	c.drawText(s, x, y, col)
}

func (c *CPUEngine) FillRect(r image.Rectangle, col color.RGBA) {
	ox, oy := int(c.offsetX), int(c.offsetY)
	r = r.Add(image.Point{ox, oy})
	if c.clipEnabled {
		r = r.Intersect(c.clipRect)
		if r.Empty() {
			return
		}
	}
	if col.A < 255 {
		draw.Draw(c.canvas, r, &image.Uniform{col}, image.Point{}, draw.Over)
	} else {
		draw.Draw(c.canvas, r, &image.Uniform{col}, image.Point{}, draw.Src)
	}
}

func (c *CPUEngine) Paint(hdc uintptr, state *types.ApplicationState) {
	// 1. BACKGROUND GRADIENT (Deep Industrial Blue)
	for y := 0; y < types.Height; y++ {
		ratio := float64(y) / float64(types.Height)
		r := uint8(10 + ratio*10)
		g := uint8(12 + ratio*12)
		b := uint8(20 + ratio*15)
		line := c.canvas.Pix[y*c.canvas.Stride : (y+1)*c.canvas.Stride]
		for x := 0; x < types.Width; x++ {
			line[x*4] = b
			line[x*4+1] = g
			line[x*4+2] = r
			line[x*4+3] = 255
		}
	}

	// 2. ORCHESTRATE UI (Shared Logic)
	types.RenderPipeline(c, state)

	// 3. BLIT TO MONITOR
	win32.StretchDIBits(
		hdc, 0, 0, int32(types.Width), int32(types.Height), 0, 0, int32(types.Width), int32(types.Height),
		uintptr(unsafe.Pointer(&c.canvas.Pix[0])),
		&c.bitmapInfo,
	)
}

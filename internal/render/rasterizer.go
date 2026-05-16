//go:build !gpu

package render

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// drawText renders basic bitmap text onto the CPU canvas
func (c *CPUEngine) drawText(s string, x, y int, col color.RGBA) {
	d := &font.Drawer{
		Dst:  c.canvas,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.Int26_6(x << 6), Y: fixed.Int26_6(y << 6)},
	}
	d.DrawString(s)
}

// drawRoundedRect implements a high-fidelity rounded rectangle with manual alpha blending
func (c *CPUEngine) drawRoundedRect(r image.Rectangle, radius int, col color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		if y < 0 || y >= Height {
			continue
		}
		for x := r.Min.X; x < r.Max.X; x++ {
			if x < 0 || x >= Width {
				continue
			}

			dx := 0
			if x < r.Min.X+radius {
				dx = (r.Min.X + radius) - x
			} else if x > r.Max.X-radius-1 {
				dx = x - (r.Max.X - radius - 1)
			}

			dy := 0
			if y < r.Min.Y+radius {
				dy = (r.Min.Y + radius) - y
			} else if y > r.Max.Y-radius-1 {
				dy = y - (r.Max.Y - radius - 1)
			}

			if dx > 0 && dy > 0 {
				if math.Sqrt(float64(dx*dx+dy*dy)) > float64(radius) {
					continue
				}
			}

			alpha := float64(col.A) / 255.0
			idx := y*c.canvas.Stride + x*4

			bgB := float64(c.canvas.Pix[idx])
			bgG := float64(c.canvas.Pix[idx+1])
			bgR := float64(c.canvas.Pix[idx+2])

			newB := uint8(float64(col.B)*alpha + bgB*(1-alpha))
			newG := uint8(float64(col.G)*alpha + bgG*(1-alpha))
			newR := uint8(float64(col.R)*alpha + bgR*(1-alpha))

			c.canvas.Pix[idx] = newB
			c.canvas.Pix[idx+1] = newG
			c.canvas.Pix[idx+2] = newR
			c.canvas.Pix[idx+3] = 255
		}
	}
}

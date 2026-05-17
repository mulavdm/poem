//go:build !gpu

package backend

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"go_native_gpu_gui/pkg/render/types"
)

// drawText renders basic bitmap text onto the CPU canvas
func (c *CPUEngine) drawText(s string, x, y int, col color.RGBA) {
	ox, oy := int(c.offsetX), int(c.offsetY)
	var dst draw.Image = c.canvas
	if c.clipEnabled {
		if sub, ok := c.canvas.SubImage(c.clipRect).(draw.Image); ok {
			dst = sub
		}
	}
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.Int26_6((x + ox) << 6), Y: fixed.Int26_6((y + oy) << 6)},
	}
	d.DrawString(s)
}

// drawRoundedRect implements a high-fidelity rounded rectangle with manual alpha blending
func (c *CPUEngine) drawRoundedRect(r image.Rectangle, radius int, col color.RGBA) {
	ox, oy := int(c.offsetX), int(c.offsetY)
	r = r.Add(image.Point{ox, oy})
	for y := r.Min.Y; y < r.Max.Y; y++ {
		if y < 0 || y >= types.Height {
			continue
		}
		for x := r.Min.X; x < r.Max.X; x++ {
			if x < 0 || x >= types.Width {
				continue
			}
			if c.clipEnabled && !image.Pt(x, y).In(c.clipRect) {
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

func (c *CPUEngine) DrawLine(x1, y1, x2, y2 int, col color.RGBA) {
	ox, oy := int(c.offsetX), int(c.offsetY)
	x1 += ox
	x2 += ox
	y1 += oy
	y2 += oy
	dx := int(math.Abs(float64(x2 - x1)))
	dy := int(math.Abs(float64(y2 - y1)))
	sx, sy := 1, 1
	if x1 >= x2 {
		sx = -1
	}
	if y1 >= y2 {
		sy = -1
	}
	err := dx - dy

	for {
		if x1 >= 0 && x1 < types.Width && y1 >= 0 && y1 < types.Height {
			if !c.clipEnabled || image.Pt(x1, y1).In(c.clipRect) {
				alpha := float64(col.A) / 255.0
				idx := y1*c.canvas.Stride + x1*4

				bgB := float64(c.canvas.Pix[idx])
				bgG := float64(c.canvas.Pix[idx+1])
				bgR := float64(c.canvas.Pix[idx+2])

				c.canvas.Pix[idx] = uint8(float64(col.B)*alpha + bgB*(1-alpha))
				c.canvas.Pix[idx+1] = uint8(float64(col.G)*alpha + bgG*(1-alpha))
				c.canvas.Pix[idx+2] = uint8(float64(col.R)*alpha + bgR*(1-alpha))
				c.canvas.Pix[idx+3] = 255
			}
		}

		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

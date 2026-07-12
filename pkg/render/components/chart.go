package components

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/mulavdm/poem/pkg/render/types"
)

// LineChart is a premium vector component that plots historical data curves in real-time
type LineChart struct {
	CompID    string
	Rect      image.Rectangle
	BGColor   color.RGBA
	LineColor color.RGBA
	Data      []float32
	Title     string
	Rounding  int
}

func (c *LineChart) ID() string                  { return c.CompID }
func (c *LineChart) GetID() string               { return c.CompID }
func (c *LineChart) Bounds() image.Rectangle     { return c.Rect }
func (c *LineChart) SetBounds(r image.Rectangle) { c.Rect = r }
func (c *LineChart) HitTest(pt image.Point) string {
	if pt.In(c.Rect) {
		return c.CompID
	}
	return ""
}
func (c *LineChart) Focusable() bool               { return false }
func (c *LineChart) Walk(fn func(types.Component)) { fn(c) }

func (c *LineChart) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }
func (c *LineChart) OnMouseDown(pt image.Point, state *types.ApplicationState) bool  { return false }
func (c *LineChart) OnMouseUp(pt image.Point, state *types.ApplicationState) bool    { return false }
func (c *LineChart) OnMouseMove(pt image.Point, state *types.ApplicationState) bool  { return false }

func (c *LineChart) Draw(pnt types.Painter, state *types.ApplicationState) {
	// 1. Draw the ambient glow and main backing card
	pnt.SetGlow(3.0)
	pnt.DrawRoundedRect(c.Rect, c.Rounding, c.BGColor)
	pnt.SetGlow(0)

	// Draw chart title
	pnt.DrawText(c.Title, c.Rect.Min.X+20, c.Rect.Min.Y+30, color.RGBA{180, 190, 210, 255})

	// 2. Define grid and graph inner bounds (with padding)
	pad := 20
	hdrH := 45
	graphRect := image.Rect(
		c.Rect.Min.X+pad,
		c.Rect.Min.Y+hdrH+pad,
		c.Rect.Max.X-pad-60, // Leave margin for Y axis values
		c.Rect.Max.Y-pad,
	)

	if graphRect.Dx() <= 0 || graphRect.Dy() <= 0 {
		return
	}

	// 3. Draw horizontal background gridlines (faint alpha)
	gridColor := color.RGBA{255, 255, 255, 8}
	for i := 0; i <= 4; i++ {
		y := graphRect.Min.Y + (i * graphRect.Dy() / 4)
		pnt.DrawLine(graphRect.Min.X, y, graphRect.Max.X, y, gridColor)
	}

	// 4. Handle empty/insufficient data edge cases
	numPts := len(c.Data)
	if numPts < 2 {
		pnt.DrawText("[ NO TELEMETRY STREAM DETECTED ]", graphRect.Min.X+40, graphRect.Min.Y+graphRect.Dy()/2, color.RGBA{100, 110, 130, 255})
		return
	}

	// 5. Calculate range boundaries to dynamically scale the Y-axis
	minVal := c.Data[0]
	maxVal := c.Data[0]
	for _, val := range c.Data {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	// If range is zero, add default bounds padding to prevent division by zero
	if maxVal == minVal {
		maxVal += 10.0
		minVal -= 10.0
		if minVal < 0 {
			minVal = 0
		}
	}
	valSpan := maxVal - minVal

	// Draw Y-axis boundary texts
	textCol := color.RGBA{120, 130, 150, 255}
	pnt.DrawText(fmt.Sprintf("%.1f", maxVal), graphRect.Max.X+10, graphRect.Min.Y-5, textCol)
	pnt.DrawText(fmt.Sprintf("%.1f", minVal), graphRect.Max.X+10, graphRect.Max.Y+15, textCol)

	// Draw current latest value
	latestVal := c.Data[numPts-1]
	pnt.DrawText(fmt.Sprintf("CURRENT: %.2f", latestVal), c.Rect.Max.X-160, c.Rect.Min.Y+30, c.LineColor)

	// 6. Draw fading gradient area and curve line using per-pixel Cosine Interpolation
	dx := graphRect.Dx()
	dy := graphRect.Dy()

	// Maintain previous pixel coordinates to connect vector segments
	prevX := graphRect.Min.X
	prevY := 0

	for x := graphRect.Min.X; x <= graphRect.Max.X; x++ {
		// Proportion of horizontal position from 0.0 to 1.0
		xRatio := float32(x-graphRect.Min.X) / float32(dx)

		// Map ratio to decimal index inside the data slice
		pos := xRatio * float32(numPts-1)
		idx := int(pos)
		t := pos - float32(idx)

		// Edge case boundary clamp
		if idx >= numPts-1 {
			idx = numPts - 2
			t = 1.0
		}

		// Perform Cosine Interpolation for smooth, wave-like transitions
		// t2 represents the sine-smoothed ratio: (1 - cos(t * pi)) / 2
		t2 := float32(1.0-math.Cos(float64(t)*math.Pi)) / 2.0
		val := c.Data[idx]*(1.0-t2) + c.Data[idx+1]*t2

		// Map interpolated value to height bounds
		yRatio := (val - minVal) / valSpan
		y := graphRect.Max.Y - int(yRatio*float32(dy))

		// 7. Draw discrete multi-opacity glowing area fill under the curve
		h := graphRect.Max.Y - y
		if h > 0 {
			y1 := y
			y2 := y + h/3
			y3 := y + 2*h/3
			y4 := graphRect.Max.Y

			// Segment A: High density close to the line (Alpha 30)
			fillA := c.LineColor
			fillA.A = 30
			pnt.DrawLine(x, y1, x, y2, fillA)

			// Segment B: Mid density (Alpha 15)
			fillB := c.LineColor
			fillB.A = 15
			pnt.DrawLine(x, y2, x, y3, fillB)

			// Segment C: Low density base (Alpha 6)
			fillC := c.LineColor
			fillC.A = 6
			pnt.DrawLine(x, y3, x, y4, fillC)
		}

		// 8. Connect the curves with thick neon vector line segment
		if x > graphRect.Min.X {
			pnt.SetGlow(5.0) // Set intense neon glow for the border curve
			pnt.DrawLine(prevX, prevY, x, y, c.LineColor)
			// Double draw for standard thick line presence
			pnt.DrawLine(prevX, prevY+1, x, y+1, c.LineColor)
			pnt.SetGlow(0)
		} else {
			// First pixel: initialize Y
			prevY = y
		}

		prevX = x
		prevY = y
	}
}

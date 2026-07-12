package components

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/types"
)

// ProgressBar displays a completion percentage or an indeterminate loading state.
type ProgressBar struct {
	CompID         string
	Rect           image.Rectangle
	Min, Max       float32
	Value          float32
	Indeterminate  bool
	TrackColor     color.RGBA
	FillColor      color.RGBA
	Rounding       int
	UseTheme       bool
	AccessibleName string
}

func NewProgressBar(id string) *ProgressBar {
	return &ProgressBar{CompID: id, Min: 0, Max: 100, UseTheme: true}
}

func (p *ProgressBar) ID() string                  { return p.CompID }
func (p *ProgressBar) GetID() string               { return p.CompID }
func (p *ProgressBar) Bounds() image.Rectangle     { return p.Rect }
func (p *ProgressBar) SetBounds(r image.Rectangle) { p.Rect = r }

func (p *ProgressBar) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	width := avail.X
	if width <= 0 {
		width = 200
	}
	size := applyExplicitSize(explicitSize(p.Rect), image.Pt(width, 10))
	return types.MeasureResult{
		Preferred: size,
		Min:       image.Pt(50, 10),
	}
}

func (p *ProgressBar) HitTest(pt image.Point) string {
	if pt.In(p.Rect) {
		return p.CompID
	}
	return ""
}

func (p *ProgressBar) Draw(pnt types.Painter, state *types.ApplicationState) {
	trackColor, fillColor, rounding := p.TrackColor, p.FillColor, p.Rounding
	if themed(p.UseTheme, state) {
		current := activeTheme(state)
		trackColor, fillColor = current.Colors.SurfaceSunken, current.Colors.Accent
		if rounding == 0 {
			rounding = current.Radii.Pill
		}
	}
	rounding = roundedRadius(p.Rect, rounding)
	// Draw background track
	pnt.DrawRoundedRect(p.Rect, rounding, trackColor)

	if p.Indeterminate {
		// Calculate a sliding chunk based on time
		elapsed := time.Since(state.StartTime).Seconds()
		// Chunk size is 20% of total width
		chunkWidth := float32(p.Rect.Dx()) * 0.2
		if chunkWidth < 20 {
			chunkWidth = 20
		}

		// Cycle over a duration (e.g. 1.5 seconds)
		speed := 1.5
		cycle := float32(elapsed) / float32(speed)
		cycle = cycle - float32(int(cycle)) // fraction 0 to 1

		// X position oscillates or loops
		// Let's make it loop left to right
		startX := float32(p.Rect.Min.X) + cycle*(float32(p.Rect.Dx())+chunkWidth) - chunkWidth
		endX := startX + chunkWidth

		// Clamp to track bounds
		clipRect := p.Rect

		pnt.PushClip(clipRect)
		fillBounds := image.Rect(int(startX), p.Rect.Min.Y, int(endX), p.Rect.Max.Y)
		pnt.DrawRoundedRect(fillBounds, rounding, fillColor)
		pnt.PopClip()
	} else {
		// Determinate mode
		rng := p.Max - p.Min
		if rng <= 0 {
			rng = 1
		}
		val := p.Value
		if val < p.Min {
			val = p.Min
		}
		if val > p.Max {
			val = p.Max
		}
		percent := (val - p.Min) / rng

		fillW := int(percent * float32(p.Rect.Dx()))
		if fillW > 0 {
			fillBounds := image.Rect(p.Rect.Min.X, p.Rect.Min.Y, p.Rect.Min.X+fillW, p.Rect.Max.Y)

			// We push a clip so that if fillW is very small, we still draw a rounded rect correctly cropped,
			// or we just draw it if it's large enough.
			pnt.PushClip(fillBounds)
			pnt.DrawRoundedRect(p.Rect, rounding, fillColor)
			pnt.PopClip()
		}
	}
}

func (p *ProgressBar) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }
func (p *ProgressBar) OnMouseDown(pt image.Point, state *types.ApplicationState) bool  { return false }
func (p *ProgressBar) OnMouseUp(pt image.Point, state *types.ApplicationState) bool    { return false }
func (p *ProgressBar) OnMouseMove(pt image.Point, state *types.ApplicationState) bool  { return false }
func (p *ProgressBar) Focusable() bool                                                 { return false }
func (p *ProgressBar) Walk(fn func(types.Component))                                   { fn(p) }

func (p *ProgressBar) Semantics(state *types.ApplicationState) semantics.Node {
	name := p.AccessibleName
	if name == "" {
		name = p.CompID
	}
	return semantics.Node{ID: p.CompID, Role: semantics.RoleProgressBar, Name: name, Value: fmt.Sprintf("%.0f", p.Value), Bounds: p.Rect}
}

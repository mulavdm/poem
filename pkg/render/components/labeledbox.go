package components

import (
	"image"
	"image/color"

	"go_native_gpu_gui/pkg/render/types"
)

// LabeledBox is a reusable wrapper that stacks a text label above one child
// control, so downstream apps do not need to hand-place label/input pairs.
type LabeledBox struct {
	CompID      string
	Rect        image.Rectangle
	Title       string
	TitleColor  color.RGBA
	Gap         int
	Child       types.Component
	LineHeight  int
	TitleOffset int
}

func (l *LabeledBox) ID() string              { return l.CompID }
func (l *LabeledBox) GetID() string           { return l.CompID }
func (l *LabeledBox) Bounds() image.Rectangle { return l.Rect }

func (l *LabeledBox) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	charW := 8
	if state != nil && state.FontCharWidth > 0 {
		charW = state.FontCharWidth
	}
	lineH := l.LineHeight
	if lineH <= 0 {
		lineH = 20
	}
	gap := l.Gap
	if gap <= 0 {
		gap = 8
	}

	titleWidth := maxInt(charW, len([]rune(l.Title))*charW)
	titleSize := image.Pt(titleWidth, lineH)
	if l.Child == nil {
		size := applyExplicitSize(explicitSize(l.Rect), image.Pt(titleSize.X, titleSize.Y))
		return types.MeasureResult{Preferred: size, Min: size}
	}

	childAvail := avail
	if childAvail.Y > 0 {
		childAvail.Y = maxInt(0, childAvail.Y-lineH-gap)
	}
	childMeasure := types.MeasureComponent(l.Child, childAvail, state)
	preferred := image.Pt(maxInt(titleSize.X, childMeasure.Preferred.X), titleSize.Y+gap+childMeasure.Preferred.Y)
	preferred = applyExplicitSize(explicitSize(l.Rect), preferred)
	return types.MeasureResult{
		Preferred: preferred,
		Min:       image.Pt(maxInt(titleSize.X, childMeasure.Min.X), titleSize.Y+gap+childMeasure.Min.Y),
	}
}

func (l *LabeledBox) SetBounds(r image.Rectangle) {
	l.Rect = r
	if l.Child == nil {
		return
	}
	lineH := l.LineHeight
	if lineH <= 0 {
		lineH = 20
	}
	gap := l.Gap
	if gap <= 0 {
		gap = 8
	}
	childTop := r.Min.Y + lineH + gap
	l.Child.SetBounds(image.Rect(r.Min.X, childTop, r.Max.X, r.Max.Y))
}

func (l *LabeledBox) Draw(p types.Painter, state *types.ApplicationState) {
	col := l.TitleColor
	if col.A == 0 {
		col = color.RGBA{215, 230, 223, 255}
	}
	lineH := l.LineHeight
	if lineH <= 0 {
		lineH = 20
	}
	offset := l.TitleOffset
	if offset == 0 {
		offset = (lineH-defaultFontHeight)/2 + defaultFontBaseline
		if offset < 0 {
			offset = defaultFontBaseline
		}
	}
	p.DrawText(l.Title, l.Rect.Min.X, l.Rect.Min.Y+offset, col)
	if l.Child != nil {
		l.Child.Draw(p, state)
	}
}

func (l *LabeledBox) HitTest(pt image.Point) string {
	if l.Child != nil {
		if id := l.Child.HitTest(pt); id != "" {
			return id
		}
	}
	if pt.In(l.Rect) {
		return l.CompID
	}
	return ""
}

func (l *LabeledBox) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if l.Child != nil {
		return l.Child.OnKey(key, char, state)
	}
	return false
}

func (l *LabeledBox) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if l.Child != nil {
		return l.Child.OnMouseDown(pt, state)
	}
	return false
}

func (l *LabeledBox) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if l.Child != nil {
		return l.Child.OnMouseUp(pt, state)
	}
	return false
}

func (l *LabeledBox) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if l.Child != nil {
		return l.Child.OnMouseMove(pt, state)
	}
	return false
}

func (l *LabeledBox) Focusable() bool { return false }

func (l *LabeledBox) Walk(fn func(types.Component)) {
	fn(l)
	if l.Child != nil {
		l.Child.Walk(fn)
	}
}

package components

import (
	"image"
	"image/color"

	"github.com/mulavdm/poem/pkg/render/types"
)

// SplitPane is a layout container that splits space horizontally between two children.
// The boundary between them can be dragged by the user.
type SplitPane struct {
	CompID         string
	Rect           image.Rectangle
	LeftChild      types.Component
	RightChild     types.Component
	SplitOffset    int // Absolute X offset from Rect.Min.X
	SplitBarWidth  int
	SplitBarColor  color.RGBA
	HoverBarColor  color.RGBA
	ActiveBarColor color.RGBA
}

func (sp *SplitPane) ID() string              { return sp.CompID }
func (sp *SplitPane) GetID() string           { return sp.CompID }
func (sp *SplitPane) Bounds() image.Rectangle { return sp.Rect }

func (sp *SplitPane) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	if sp.SplitBarWidth == 0 {
		sp.SplitBarWidth = 6
	}

	availLeft := image.Pt(sp.SplitOffset, avail.Y)
	availRight := image.Pt(avail.X-sp.SplitOffset-sp.SplitBarWidth, avail.Y)

	var leftRes, rightRes types.MeasureResult
	if sp.LeftChild != nil {
		if mr, ok := sp.LeftChild.(interface {
			Measure(image.Point, *types.ApplicationState) types.MeasureResult
		}); ok {
			leftRes = mr.Measure(availLeft, state)
		} else {
			leftRes = types.MeasureResult{Preferred: sp.LeftChild.Bounds().Size(), Min: sp.LeftChild.Bounds().Size()}
		}
	}
	if sp.RightChild != nil {
		if mr, ok := sp.RightChild.(interface {
			Measure(image.Point, *types.ApplicationState) types.MeasureResult
		}); ok {
			rightRes = mr.Measure(availRight, state)
		} else {
			rightRes = types.MeasureResult{Preferred: sp.RightChild.Bounds().Size(), Min: sp.RightChild.Bounds().Size()}
		}
	}

	w := leftRes.Preferred.X + rightRes.Preferred.X + sp.SplitBarWidth
	h := maxInt(leftRes.Preferred.Y, rightRes.Preferred.Y)
	minW := leftRes.Min.X + rightRes.Min.X + sp.SplitBarWidth
	minH := maxInt(leftRes.Min.Y, rightRes.Min.Y)

	size := applyExplicitSize(explicitSize(sp.Rect), image.Pt(w, h))
	return types.MeasureResult{
		Preferred: size,
		Min:       image.Pt(minValueInt(size.X, minW), minValueInt(size.Y, minH)),
	}
}

func (sp *SplitPane) SetBounds(r image.Rectangle) {
	sp.Rect = r
	if sp.SplitBarWidth == 0 {
		sp.SplitBarWidth = 6
	}
	if sp.SplitOffset <= 0 || sp.SplitOffset > r.Dx() {
		sp.SplitOffset = r.Dx() / 2
	}

	// Layout children
	if sp.LeftChild != nil {
		sp.LeftChild.SetBounds(image.Rect(r.Min.X, r.Min.Y, r.Min.X+sp.SplitOffset, r.Max.Y))
	}
	if sp.RightChild != nil {
		sp.RightChild.SetBounds(image.Rect(r.Min.X+sp.SplitOffset+sp.SplitBarWidth, r.Min.Y, r.Max.X, r.Max.Y))
	}
}

func (sp *SplitPane) HitTest(pt image.Point) string {
	if !pt.In(sp.Rect) {
		return ""
	}

	// Check splitter bar
	barRect := image.Rect(sp.Rect.Min.X+sp.SplitOffset, sp.Rect.Min.Y, sp.Rect.Min.X+sp.SplitOffset+sp.SplitBarWidth, sp.Rect.Max.Y)
	if pt.In(barRect) {
		return sp.CompID + "_splitter"
	}

	if sp.RightChild != nil {
		if hit := sp.RightChild.HitTest(pt); hit != "" {
			return hit
		}
	}
	if sp.LeftChild != nil {
		if hit := sp.LeftChild.HitTest(pt); hit != "" {
			return hit
		}
	}

	return sp.CompID
}

func (sp *SplitPane) Draw(pnt types.Painter, state *types.ApplicationState) {
	if sp.LeftChild != nil {
		pnt.PushClip(sp.LeftChild.Bounds())
		sp.LeftChild.Draw(pnt, state)
		pnt.PopClip()
	}
	if sp.RightChild != nil {
		pnt.PushClip(sp.RightChild.Bounds())
		sp.RightChild.Draw(pnt, state)
		pnt.PopClip()
	}

	// Draw Splitter Bar
	barRect := image.Rect(sp.Rect.Min.X+sp.SplitOffset, sp.Rect.Min.Y, sp.Rect.Min.X+sp.SplitOffset+sp.SplitBarWidth, sp.Rect.Max.Y)

	col := sp.SplitBarColor
	if state.ActiveID == sp.CompID+"_splitter" {
		col = sp.ActiveBarColor
	} else if state.HoveredID == sp.CompID+"_splitter" {
		col = sp.HoverBarColor
		state.CursorID = state.HandCursor
	}

	if col.A > 0 {
		pnt.FillRect(barRect, col)
	}
}

func (sp *SplitPane) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	return false
}

func (sp *SplitPane) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	barRect := image.Rect(sp.Rect.Min.X+sp.SplitOffset, sp.Rect.Min.Y, sp.Rect.Min.X+sp.SplitOffset+sp.SplitBarWidth, sp.Rect.Max.Y)
	if pt.In(barRect) {
		state.ActiveID = sp.CompID + "_splitter"
		return true
	}
	if sp.RightChild != nil && sp.RightChild.OnMouseDown(pt, state) {
		return true
	}
	if sp.LeftChild != nil && sp.LeftChild.OnMouseDown(pt, state) {
		return true
	}
	return false
}

func (sp *SplitPane) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID == sp.CompID+"_splitter" {
		state.ActiveID = ""
		return true
	}
	if sp.RightChild != nil && sp.RightChild.OnMouseUp(pt, state) {
		return true
	}
	if sp.LeftChild != nil && sp.LeftChild.OnMouseUp(pt, state) {
		return true
	}
	return false
}

func (sp *SplitPane) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID == sp.CompID+"_splitter" {
		newOffset := pt.X - sp.Rect.Min.X - sp.SplitBarWidth/2
		// Constrain offset
		minOffset := 50
		maxOffset := sp.Rect.Dx() - sp.SplitBarWidth - 50
		if newOffset < minOffset {
			newOffset = minOffset
		}
		if newOffset > maxOffset {
			newOffset = maxOffset
		}
		sp.SplitOffset = newOffset

		// Re-layout children
		sp.SetBounds(sp.Rect)
		return true
	}

	if sp.RightChild != nil && sp.RightChild.OnMouseMove(pt, state) {
		return true
	}
	if sp.LeftChild != nil && sp.LeftChild.OnMouseMove(pt, state) {
		return true
	}
	return false
}

func (sp *SplitPane) Focusable() bool { return false }

func (sp *SplitPane) Walk(fn func(types.Component)) {
	fn(sp)
	if sp.LeftChild != nil {
		sp.LeftChild.Walk(fn)
	}
	if sp.RightChild != nil {
		sp.RightChild.Walk(fn)
	}
}

func (sp *SplitPane) ChildComponents() []types.Component {
	children := make([]types.Component, 0, 2)
	if sp.LeftChild != nil {
		children = append(children, sp.LeftChild)
	}
	if sp.RightChild != nil {
		children = append(children, sp.RightChild)
	}
	return children
}

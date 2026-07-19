package components

import (
	"image"

	"github.com/mulavdm/poem/pkg/render/types"
)

// Responsive selects a compact or wide component tree from its assigned
// logical-pixel width.
type Responsive struct {
	CompID     string
	Rect       image.Rectangle
	Breakpoint int
	Compact    types.Component
	Wide       types.Component
}

func (r *Responsive) ID() string              { return r.CompID }
func (r *Responsive) GetID() string           { return r.CompID }
func (r *Responsive) Bounds() image.Rectangle { return r.Rect }
func (r *Responsive) Focusable() bool         { return false }
func (r *Responsive) branch(width int) types.Component {
	if width > r.Breakpoint && r.Wide != nil {
		return r.Wide
	}
	return r.Compact
}
func (r *Responsive) active() types.Component { return r.branch(r.Rect.Dx()) }

// Measure returns the selected branch measurement for the available width.
func (r *Responsive) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	if active := r.branch(avail.X); active != nil {
		return types.MeasureComponent(active, avail, state)
	}
	return types.MeasureResult{}
}
func (r *Responsive) SetBounds(bounds image.Rectangle) {
	r.Rect = bounds
	if active := r.active(); active != nil {
		active.SetBounds(bounds)
	}
}
func (r *Responsive) Draw(p types.Painter, state *types.ApplicationState) {
	if active := r.active(); active != nil {
		active.SetBounds(r.Rect)
		active.Draw(p, state)
	}
}
func (r *Responsive) HitTest(point image.Point) string {
	if !point.In(r.Rect) {
		return ""
	}
	if active := r.active(); active != nil {
		if id := active.HitTest(point); id != "" {
			return id
		}
	}
	return r.CompID
}
func (r *Responsive) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	return r.active() != nil && r.active().OnKey(key, char, state)
}
func (r *Responsive) OnMouseDown(point image.Point, state *types.ApplicationState) bool {
	return r.active() != nil && r.active().OnMouseDown(point, state)
}
func (r *Responsive) OnMouseUp(point image.Point, state *types.ApplicationState) bool {
	return r.active() != nil && r.active().OnMouseUp(point, state)
}
func (r *Responsive) OnMouseMove(point image.Point, state *types.ApplicationState) bool {
	return r.active() != nil && r.active().OnMouseMove(point, state)
}
func (r *Responsive) Walk(fn func(types.Component)) {
	fn(r)
	if active := r.active(); active != nil {
		active.Walk(fn)
	}
}

// ChildComponents returns the currently rendered branch.
func (r *Responsive) ChildComponents() []types.Component {
	if active := r.active(); active != nil {
		return []types.Component{active}
	}
	return nil
}

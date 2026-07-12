package components

import (
	"image"
	"image/color"

	"go_native_gpu_gui/pkg/render/types"
)

// Modal is a blocking overlay container that draws a backdrop and consumes
// pointer interaction outside and inside the modal card unless a child handles it.
type Modal struct {
	CompID            string
	Rect              image.Rectangle
	CardRect          image.Rectangle
	BackdropColor     color.RGBA
	Children          []types.Component
	DismissOnBackdrop bool
	OnDismiss         func(state *types.ApplicationState)
}

func (m *Modal) ID() string              { return m.CompID }
func (m *Modal) GetID() string           { return m.CompID }
func (m *Modal) Bounds() image.Rectangle { return m.Rect }
func (m *Modal) SetBounds(r image.Rectangle) {
	m.Rect = r
}
func (m *Modal) Focusable() bool { return false }
func (m *Modal) Walk(fn func(types.Component)) {
	fn(m)
	for _, child := range m.Children {
		child.Walk(fn)
	}
}

func (m *Modal) ChildComponents() []types.Component { return m.Children }

func (m *Modal) Draw(p types.Painter, state *types.ApplicationState) {
	backdrop := m.BackdropColor
	if backdrop.A == 0 {
		backdrop = activeTheme(state).Colors.Overlay
	}
	p.DrawRoundedRect(m.Rect, 0, backdrop)
	for _, child := range m.Children {
		child.Draw(p, state)
	}
}

func (m *Modal) HitTest(pt image.Point) string {
	if !pt.In(m.Rect) {
		return ""
	}
	for i := len(m.Children) - 1; i >= 0; i-- {
		if id := m.Children[i].HitTest(pt); id != "" {
			return id
		}
	}
	return m.CompID
}

func (m *Modal) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if key == 27 && m.OnDismiss != nil {
		m.OnDismiss(state)
		return true
	}
	for i := len(m.Children) - 1; i >= 0; i-- {
		if m.Children[i].OnKey(key, char, state) {
			return true
		}
	}
	return false
}

func (m *Modal) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if !pt.In(m.Rect) {
		return false
	}

	if pt.In(m.CardRect) {
		for i := len(m.Children) - 1; i >= 0; i-- {
			child := m.Children[i]
			if child.HitTest(pt) != "" && child.OnMouseDown(pt, state) {
				return true
			}
		}
		return true
	}

	if m.DismissOnBackdrop && m.OnDismiss != nil {
		m.OnDismiss(state)
	}
	return true
}

func (m *Modal) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if !pt.In(m.Rect) {
		return false
	}
	if pt.In(m.CardRect) {
		for i := len(m.Children) - 1; i >= 0; i-- {
			child := m.Children[i]
			if child.HitTest(pt) != "" && child.OnMouseUp(pt, state) {
				return true
			}
		}
	}
	return true
}

func (m *Modal) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if !pt.In(m.Rect) {
		return false
	}
	if pt.In(m.CardRect) {
		for i := len(m.Children) - 1; i >= 0; i-- {
			child := m.Children[i]
			if child.HitTest(pt) != "" && child.OnMouseMove(pt, state) {
				return true
			}
		}
	}
	return true
}

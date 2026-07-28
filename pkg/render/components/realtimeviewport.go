package components

import (
	"image"
	"image/color"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/types"
)

const maxRealtimeViewportCommandBytes = 1024 * 1024

// RealtimeViewport reserves a native GPU viewport inside normal POEM layout.
// Command is opaque to POEM and is validated and forwarded only by an ABI v3
// native host. ABI v2 plugins continue to render with an empty command.
type RealtimeViewport struct {
	CompID    string
	Rect      image.Rectangle
	Label     string
	Command   []byte
	Disabled  bool
	OnPointer func(kind string, x, y float64, state *types.ApplicationState)
	// OnWheel receives a non-zero native wheel delta while the pointer is in
	// the viewport. The delta follows the host convention (120 per Windows
	// wheel notch); POEM does not assign it a renderer-specific meaning.
	OnWheel func(delta int, state *types.ApplicationState)
	// OnPointerButton receives pointer phases together with the host button
	// code (1 primary, 2 secondary, 3 middle). When set, it takes precedence
	// over OnPointer so advanced native views can distinguish navigation
	// gestures without changing ordinary component input behavior.
	OnPointerButton func(kind string, button int, x, y float64, state *types.ApplicationState)
	OnViewportKey   func(key uint32, char rune, state *types.ApplicationState) bool
	OnEvent         func(payload []byte, state *types.ApplicationState) bool
}

func (v *RealtimeViewport) ID() string                    { return v.CompID }
func (v *RealtimeViewport) GetID() string                 { return v.CompID }
func (v *RealtimeViewport) Bounds() image.Rectangle       { return v.Rect }
func (v *RealtimeViewport) SetBounds(r image.Rectangle)   { v.Rect = r }
func (v *RealtimeViewport) Focusable() bool               { return !v.Disabled }
func (v *RealtimeViewport) Walk(fn func(types.Component)) { fn(v) }
func (v *RealtimeViewport) HitTest(pt image.Point) string {
	if !v.Disabled && pt.In(v.Rect) {
		return v.CompID
	}
	return ""
}
func (v *RealtimeViewport) Draw(p types.Painter, state *types.ApplicationState) {
	p.PushClip(v.Rect)
	if native, ok := p.(interface {
		DrawRealtimeViewport(image.Rectangle, string, []byte, bool)
	}); ok {
		command := v.Command
		if len(command) > maxRealtimeViewportCommandBytes {
			command = nil
		}
		native.DrawRealtimeViewport(v.Rect, v.CompID, command, state != nil && state.FocusedID == v.CompID)
	} else {
		p.FillRect(v.Rect, color.RGBA{18, 24, 35, 255})
	}
	p.PopClip()
}
func (v *RealtimeViewport) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if v.Disabled {
		return false
	}
	if v.OnViewportKey != nil {
		return v.OnViewportKey(key, char, state)
	}
	return true
}
func (v *RealtimeViewport) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if v.Disabled || !pt.In(v.Rect) {
		return false
	}
	state.FocusedID = v.CompID
	state.ActiveID = v.CompID
	v.pointer("down", state.MouseButton, pt, state)
	return true
}
func (v *RealtimeViewport) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID == v.CompID {
		state.ActiveID = ""
		if v.OnPointer != nil || v.OnPointerButton != nil {
			kind := "up"
			if !pt.In(v.Rect) {
				kind = "cancel"
			}
			v.pointer(kind, state.MouseButton, pt, state)
		}
		return true
	}
	return false
}
func (v *RealtimeViewport) OnRealtimeViewportEvent(payload []byte, state *types.ApplicationState) bool {
	return v.OnEvent != nil && v.OnEvent(append([]byte(nil), payload...), state)
}
func (v *RealtimeViewport) pointer(kind string, button int, pt image.Point, state *types.ApplicationState) {
	x := float64(pt.X-v.Rect.Min.X) / float64(maxInt(1, v.Rect.Dx()))
	y := float64(pt.Y-v.Rect.Min.Y) / float64(maxInt(1, v.Rect.Dy()))
	if v.OnPointerButton != nil {
		v.OnPointerButton(kind, button, x, y, state)
	} else if v.OnPointer != nil {
		v.OnPointer(kind, x, y, state)
	}
}
func (v *RealtimeViewport) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if v.Disabled || (v.OnPointer == nil && v.OnPointerButton == nil) {
		return false
	}
	if state.ActiveID == v.CompID || pt.In(v.Rect) {
		v.pointer("move", state.MouseButton, pt, state)
		return state.ActiveID == v.CompID
	}
	return false
}
func (v *RealtimeViewport) OnMouseWheel(pt image.Point, delta int, state *types.ApplicationState) bool {
	if v.Disabled || v.OnWheel == nil || delta == 0 || !pt.In(v.Rect) {
		return false
	}
	v.OnWheel(delta, state)
	return true
}
func (v *RealtimeViewport) Semantics(_ *types.ApplicationState) semantics.Node {
	label := v.Label
	if label == "" {
		label = "Real-time viewport"
	}
	return semantics.Node{ID: v.CompID, Role: semantics.RoleGroup, Name: label, Bounds: v.Rect,
		State: semantics.State{Disabled: v.Disabled}}
}

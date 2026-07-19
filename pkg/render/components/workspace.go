package components

import (
	"image"

	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

// Workspace composes a stable application header, primary canvas, and an
// adaptive tools surface. The compact sheet height is renderer-owned state.
type Workspace struct {
	CompID  string
	Rect    image.Rectangle
	Header  types.Component
	Content types.Component
	Tools   types.Component
	dragY   int
}

type workspaceInteraction struct{ SheetFraction float64 }

func (w *Workspace) ID() string              { return w.CompID }
func (w *Workspace) GetID() string           { return w.CompID }
func (w *Workspace) Bounds() image.Rectangle { return w.Rect }
func (w *Workspace) Focusable() bool         { return false }
func (w *Workspace) interactionKey() string  { return w.CompID + "/workspace" }

func (w *Workspace) interaction(state *types.ApplicationState) workspaceInteraction {
	value := workspaceInteraction{SheetFraction: .44}
	if state != nil && state.TransientState != nil {
		if stored, ok := renderstate.Load[workspaceInteraction](state.TransientState, w.interactionKey()); ok {
			value = stored
		}
	}
	if value.SheetFraction < .22 {
		value.SheetFraction = .22
	}
	if value.SheetFraction > .78 {
		value.SheetFraction = .78
	}
	return value
}

func (w *Workspace) persist(state *types.ApplicationState, value workspaceInteraction) {
	if state == nil {
		return
	}
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, w.interactionKey(), value)
}

func (w *Workspace) Measure(avail image.Point, _ *types.ApplicationState) types.MeasureResult {
	return types.MeasureResult{Preferred: avail, Min: image.Pt(minInt(avail.X, 320), minInt(avail.Y, 360))}
}

func (w *Workspace) SetBounds(bounds image.Rectangle) { w.Rect = bounds; w.layout(nil) }

func (w *Workspace) layout(state *types.ApplicationState) {
	headerHeight := 72
	if w.Header != nil {
		w.Header.SetBounds(image.Rect(w.Rect.Min.X, w.Rect.Min.Y, w.Rect.Max.X, minInt(w.Rect.Max.Y, w.Rect.Min.Y+headerHeight)))
	}
	body := image.Rect(w.Rect.Min.X, minInt(w.Rect.Max.Y, w.Rect.Min.Y+headerHeight), w.Rect.Max.X, w.Rect.Max.Y)
	if body.Empty() {
		return
	}
	if w.Rect.Dx() >= 840 {
		toolsWidth := 380
		if w.Rect.Dx() < 1000 {
			toolsWidth = 340
		}
		if w.Tools != nil {
			w.Tools.SetBounds(image.Rect(body.Min.X, body.Min.Y, minInt(body.Max.X, body.Min.X+toolsWidth), body.Max.Y))
		}
		if w.Content != nil {
			w.Content.SetBounds(image.Rect(minInt(body.Max.X, body.Min.X+toolsWidth+1), body.Min.Y, body.Max.X, body.Max.Y))
		}
		return
	}
	interaction := w.interaction(state)
	sheetHeight := int(float64(body.Dy()) * interaction.SheetFraction)
	sheetTop := body.Max.Y - sheetHeight
	if w.Content != nil {
		w.Content.SetBounds(body)
	}
	if w.Tools != nil {
		w.Tools.SetBounds(image.Rect(body.Min.X, sheetTop+20, body.Max.X, body.Max.Y))
	}
}

func (w *Workspace) Draw(p types.Painter, state *types.ApplicationState) {
	w.layout(state)
	th := activeTheme(state)
	p.FillRect(w.Rect, th.Colors.Background)
	if w.Header != nil {
		p.PushClip(w.Header.Bounds())
		w.Header.Draw(p, state)
		p.PopClip()
	}
	if w.Content != nil {
		p.PushClip(w.Content.Bounds())
		w.Content.Draw(p, state)
		p.PopClip()
	}
	if w.Tools != nil {
		toolRect := w.Tools.Bounds()
		panel := image.Rect(toolRect.Min.X, maxInt(w.Rect.Min.Y+72, toolRect.Min.Y-20), toolRect.Max.X, toolRect.Max.Y)
		p.DrawRoundedRect(panel, th.Radii.Large, th.Colors.SurfaceRaised)
		if w.Rect.Dx() < 840 {
			handle := image.Rect(panel.Min.X+panel.Dx()/2-24, panel.Min.Y+8, panel.Min.X+panel.Dx()/2+24, panel.Min.Y+12)
			p.DrawRoundedRect(handle, th.Radii.Pill, th.Colors.BorderStrong)
		}
		p.PushClip(toolRect)
		w.Tools.Draw(p, state)
		p.PopClip()
	}
}

func (w *Workspace) HitTest(point image.Point) string {
	if !point.In(w.Rect) {
		return ""
	}
	if w.Rect.Dx() < 840 && w.Tools != nil {
		t := w.Tools.Bounds()
		handle := image.Rect(t.Min.X, t.Min.Y-24, t.Max.X, t.Min.Y)
		if point.In(handle) {
			return w.CompID + "/sheet-handle"
		}
	}
	for _, child := range []types.Component{w.Tools, w.Content, w.Header} {
		if child != nil && point.In(child.Bounds()) {
			if id := child.HitTest(point); id != "" {
				return id
			}
		}
	}
	return w.CompID
}

func (w *Workspace) OnMouseDown(point image.Point, state *types.ApplicationState) bool {
	if w.Rect.Dx() < 840 && w.Tools != nil && point.Y >= w.Tools.Bounds().Min.Y-24 && point.Y < w.Tools.Bounds().Min.Y {
		state.ActiveID = w.CompID + "/sheet-handle"
		w.dragY = point.Y
		return true
	}
	for _, child := range []types.Component{w.Tools, w.Content, w.Header} {
		if child != nil && point.In(child.Bounds()) && child.OnMouseDown(point, state) {
			return true
		}
	}
	return false
}

func (w *Workspace) OnMouseMove(point image.Point, state *types.ApplicationState) bool {
	if state != nil && state.ActiveID == w.CompID+"/sheet-handle" {
		bodyHeight := maxInt(1, w.Rect.Dy()-72)
		current := w.interaction(state)
		current.SheetFraction += float64(w.dragY-point.Y) / float64(bodyHeight)
		w.dragY = point.Y
		w.persist(state, current)
		w.layout(state)
		return true
	}
	for _, child := range []types.Component{w.Tools, w.Content, w.Header} {
		if child != nil && point.In(child.Bounds()) && child.OnMouseMove(point, state) {
			return true
		}
	}
	return false
}

func (w *Workspace) OnMouseUp(point image.Point, state *types.ApplicationState) bool {
	if state != nil && state.ActiveID == w.CompID+"/sheet-handle" {
		state.ActiveID = ""
		current := w.interaction(state)
		switch {
		case current.SheetFraction < .34:
			current.SheetFraction = .24
		case current.SheetFraction > .62:
			current.SheetFraction = .76
		default:
			current.SheetFraction = .46
		}
		w.persist(state, current)
		return true
	}
	for _, child := range []types.Component{w.Tools, w.Content, w.Header} {
		if child != nil && child.OnMouseUp(point, state) {
			return true
		}
	}
	return false
}

func (w *Workspace) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if key == 0x1B && w.Rect.Dx() < 840 {
		current := w.interaction(state)
		if current.SheetFraction > .24 {
			current.SheetFraction = .24
			w.persist(state, current)
			w.layout(state)
			return true
		}
	}
	for _, child := range []types.Component{w.Header, w.Content, w.Tools} {
		if child != nil && child.OnKey(key, char, state) {
			return true
		}
	}
	return false
}
func (w *Workspace) Walk(fn func(types.Component)) {
	fn(w)
	for _, child := range []types.Component{w.Header, w.Content, w.Tools} {
		if child != nil {
			child.Walk(fn)
		}
	}
}
func (w *Workspace) ChildComponents() []types.Component {
	out := []types.Component{}
	for _, child := range []types.Component{w.Header, w.Content, w.Tools} {
		if child != nil {
			out = append(out, child)
		}
	}
	return out
}
func (w *Workspace) Semantics(state *types.ApplicationState) semantics.Node {
	node := semantics.Node{ID: w.CompID, Role: semantics.RoleGroup, Name: "Workspace", Bounds: w.Rect}
	for _, child := range []types.Component{w.Header, w.Content, w.Tools} {
		if semanticChild, ok := child.(interface {
			Semantics(*types.ApplicationState) semantics.Node
		}); ok {
			node.Children = append(node.Children, semanticChild.Semantics(state))
		}
	}
	return node
}

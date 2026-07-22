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
	Detail  types.Component
	dragY   int
	compact bool
	class   WorkspaceWindowClass
}

// WorkspaceWindowClass is the resolved four-class adaptive shell. Widths are
// logical pixels (dp-equivalent), matching the shared adaptation guide.
type WorkspaceWindowClass uint8

const (
	WorkspaceCompact WorkspaceWindowClass = iota
	WorkspaceMedium
	WorkspaceExpanded
	WorkspaceUltraWide
)

func ClassifyWorkspaceWidth(width int) WorkspaceWindowClass {
	switch {
	case width < 600:
		return WorkspaceCompact
	case width < 840:
		return WorkspaceMedium
	case width < 1200:
		return WorkspaceExpanded
	default:
		return WorkspaceUltraWide
	}
}

func (w *Workspace) WindowClass() WorkspaceWindowClass { return w.class }

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

type workspaceMetrics struct {
	headerHeight int
	minTools     int
	maxTools     int
	minDetail    int
	maxDetail    int
	minMap       int
	sheetHandle  int
	columnGap    int
}

func workspaceLayoutMetrics(state *types.ApplicationState) workspaceMetrics {
	th := activeTheme(state)
	return workspaceMetrics{
		headerHeight: th.Controls.Large + 2*th.Spacing.MD,
		minTools:     8*th.Controls.Medium + 2*th.Spacing.LG,
		maxTools:     14*th.Controls.Medium + 2*th.Spacing.LG,
		minDetail:    7*th.Controls.Medium + 2*th.Spacing.LG,
		maxDetail:    10*th.Controls.Medium + 2*th.Spacing.LG,
		minMap:       8 * th.Controls.Medium,
		sheetHandle:  th.Spacing.XL,
		columnGap:    th.Spacing.XS,
	}
}

func (w *Workspace) desiredToolsWidth(body image.Rectangle, state *types.ApplicationState, metrics workspaceMetrics) int {
	if w.Tools == nil {
		return 0
	}
	measured := types.MeasureContent(w.Tools, image.Pt(metrics.maxTools, body.Dy()), state).X
	return clampInt(measured, metrics.minTools, metrics.maxTools)
}

func (w *Workspace) desiredDetailWidth(body image.Rectangle, state *types.ApplicationState, metrics workspaceMetrics) int {
	if w.Detail == nil {
		return 0
	}
	measured := types.MeasureContent(w.Detail, image.Pt(metrics.maxDetail, body.Dy()), state).X
	return clampInt(measured, metrics.minDetail, metrics.maxDetail)
}

func (w *Workspace) layout(state *types.ApplicationState) {
	metrics := workspaceLayoutMetrics(state)
	headerHeight := metrics.headerHeight
	if w.Header != nil {
		headerHeight = maxInt(headerHeight, types.MeasureContent(w.Header, image.Pt(w.Rect.Dx(), w.Rect.Dy()), state).Y)
		headerHeight = minInt(headerHeight, maxInt(metrics.headerHeight, w.Rect.Dy()/2))
	}
	if w.Header != nil {
		w.Header.SetBounds(image.Rect(w.Rect.Min.X, w.Rect.Min.Y, w.Rect.Max.X, minInt(w.Rect.Max.Y, w.Rect.Min.Y+headerHeight)))
	}
	body := image.Rect(w.Rect.Min.X, minInt(w.Rect.Max.Y, w.Rect.Min.Y+headerHeight), w.Rect.Max.X, w.Rect.Max.Y)
	if body.Empty() {
		return
	}
	desiredTools := w.desiredToolsWidth(body, state, metrics)
	desiredDetail := w.desiredDetailWidth(body, state, metrics)
	w.class = ClassifyWorkspaceWidth(w.Rect.Dx())

	availablePanels := body.Dx() - metrics.minMap - metrics.columnGap
	mediumRailLimit := body.Dx() * 3 / 5
	if w.class == WorkspaceMedium && desiredTools > mediumRailLimit {
		// A scaled rail that covers the canvas no longer satisfies the Medium
		// contract. Reuse the compact sheet instead of hiding map context.
		w.class = WorkspaceCompact
	}
	if w.class == WorkspaceExpanded && desiredTools > availablePanels {
		w.class = WorkspaceCompact
	}
	if w.class == WorkspaceUltraWide && desiredTools+desiredDetail+metrics.columnGap > availablePanels {
		// Increased text scale may collapse the optional detail column before
		// resorting to the single-surface compact sheet.
		if desiredTools <= availablePanels {
			w.class = WorkspaceExpanded
		} else {
			w.class = WorkspaceCompact
		}
	}
	w.compact = w.class == WorkspaceCompact

	if w.Detail != nil {
		w.Detail.SetBounds(image.Rectangle{})
	}

	switch w.class {
	case WorkspaceMedium:
		if w.Content != nil {
			w.Content.SetBounds(body)
		}
		// A non-modal rail overlays the canvas but leaves spatial context
		// visible. Long text reflows inside the bounded rail.
		toolsWidth := minInt(desiredTools, mediumRailLimit)
		if w.Tools != nil {
			w.Tools.SetBounds(image.Rect(body.Min.X, body.Min.Y, minInt(body.Max.X, body.Min.X+toolsWidth), body.Max.Y))
		}
		return
	case WorkspaceExpanded:
		toolsWidth := minInt(desiredTools, maxInt(0, availablePanels))
		toolsRight := minInt(body.Max.X, body.Min.X+toolsWidth)
		if w.Tools != nil {
			w.Tools.SetBounds(image.Rect(body.Min.X, body.Min.Y, toolsRight, body.Max.Y))
		}
		if w.Content != nil {
			w.Content.SetBounds(image.Rect(minInt(body.Max.X, toolsRight+metrics.columnGap), body.Min.Y, body.Max.X, body.Max.Y))
		}
		return
	case WorkspaceUltraWide:
		toolsWidth := minInt(desiredTools, maxInt(0, availablePanels))
		toolsRight := minInt(body.Max.X, body.Min.X+toolsWidth)
		if w.Detail == nil {
			if w.Tools != nil {
				w.Tools.SetBounds(image.Rect(body.Min.X, body.Min.Y, toolsRight, body.Max.Y))
			}
			if w.Content != nil {
				w.Content.SetBounds(image.Rect(minInt(body.Max.X, toolsRight+metrics.columnGap), body.Min.Y, body.Max.X, body.Max.Y))
			}
			return
		}
		detailWidth := minInt(desiredDetail, maxInt(0, availablePanels-toolsWidth-metrics.columnGap))
		detailLeft := minInt(body.Max.X, toolsRight+metrics.columnGap)
		detailRight := minInt(body.Max.X, detailLeft+detailWidth)
		if w.Tools != nil {
			w.Tools.SetBounds(image.Rect(body.Min.X, body.Min.Y, toolsRight, body.Max.Y))
		}
		if w.Detail != nil {
			w.Detail.SetBounds(image.Rect(detailLeft, body.Min.Y, detailRight, body.Max.Y))
		}
		if w.Content != nil {
			w.Content.SetBounds(image.Rect(minInt(body.Max.X, detailRight+metrics.columnGap), body.Min.Y, body.Max.X, body.Max.Y))
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
		w.Tools.SetBounds(image.Rect(body.Min.X, sheetTop+metrics.sheetHandle, body.Max.X, body.Max.Y))
	}
}

func (w *Workspace) Draw(p types.Painter, state *types.ApplicationState) {
	w.layout(state)
	th := activeTheme(state)
	metrics := workspaceLayoutMetrics(state)
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
		panel := image.Rect(toolRect.Min.X, maxInt(w.Rect.Min.Y+metrics.headerHeight, toolRect.Min.Y-metrics.sheetHandle), toolRect.Max.X, toolRect.Max.Y)
		p.DrawRoundedRect(panel, th.Radii.Large, th.Colors.SurfaceRaised)
		if w.compact {
			handle := image.Rect(panel.Min.X+panel.Dx()/2-24, panel.Min.Y+8, panel.Min.X+panel.Dx()/2+24, panel.Min.Y+12)
			p.DrawRoundedRect(handle, th.Radii.Pill, th.Colors.BorderStrong)
		}
		p.PushClip(toolRect)
		w.Tools.Draw(p, state)
		p.PopClip()
	}
	if w.class == WorkspaceUltraWide && w.Detail != nil && !w.Detail.Bounds().Empty() {
		detailRect := w.Detail.Bounds()
		p.DrawRoundedRect(detailRect, th.Radii.Large, th.Colors.SurfaceRaised)
		p.PushClip(detailRect)
		w.Detail.Draw(p, state)
		p.PopClip()
	}
}

func (w *Workspace) HitTest(point image.Point) string {
	if !point.In(w.Rect) {
		return ""
	}
	if w.compact && w.Tools != nil {
		t := w.Tools.Bounds()
		handle := image.Rect(t.Min.X, t.Min.Y-24, t.Max.X, t.Min.Y)
		if point.In(handle) {
			return w.CompID + "/sheet-handle"
		}
	}
	for _, child := range []types.Component{w.Detail, w.Tools, w.Content, w.Header} {
		if child != nil && point.In(child.Bounds()) {
			if id := child.HitTest(point); id != "" {
				return id
			}
		}
	}
	return w.CompID
}

func (w *Workspace) OnMouseDown(point image.Point, state *types.ApplicationState) bool {
	if w.compact && w.Tools != nil && point.Y >= w.Tools.Bounds().Min.Y-workspaceLayoutMetrics(state).sheetHandle && point.Y < w.Tools.Bounds().Min.Y {
		state.ActiveID = w.CompID + "/sheet-handle"
		w.dragY = point.Y
		return true
	}
	for _, child := range []types.Component{w.Detail, w.Tools, w.Content, w.Header} {
		if child != nil && point.In(child.Bounds()) && child.OnMouseDown(point, state) {
			return true
		}
	}
	return false
}

func (w *Workspace) OnMouseMove(point image.Point, state *types.ApplicationState) bool {
	if state != nil && state.ActiveID == w.CompID+"/sheet-handle" {
		bodyHeight := maxInt(1, w.Rect.Dy()-workspaceLayoutMetrics(state).headerHeight)
		current := w.interaction(state)
		current.SheetFraction += float64(w.dragY-point.Y) / float64(bodyHeight)
		w.dragY = point.Y
		w.persist(state, current)
		w.layout(state)
		return true
	}
	for _, child := range []types.Component{w.Detail, w.Tools, w.Content, w.Header} {
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
	for _, child := range []types.Component{w.Detail, w.Tools, w.Content, w.Header} {
		if child == w.Detail && w.class != WorkspaceUltraWide {
			continue
		}
		if child != nil && child.OnMouseUp(point, state) {
			return true
		}
	}
	return false
}

func (w *Workspace) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if key == 0x1B && w.compact {
		current := w.interaction(state)
		if current.SheetFraction > .24 {
			current.SheetFraction = .24
			w.persist(state, current)
			w.layout(state)
			return true
		}
	}
	for _, child := range w.ChildComponents() {
		if child != nil && child.OnKey(key, char, state) {
			return true
		}
	}
	return false
}
func (w *Workspace) Walk(fn func(types.Component)) {
	fn(w)
	for _, child := range w.ChildComponents() {
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
	if w.class == WorkspaceUltraWide && w.Detail != nil && !w.Detail.Bounds().Empty() {
		out = append(out, w.Detail)
	}
	return out
}
func (w *Workspace) Semantics(_ *types.ApplicationState) semantics.Node {
	// ChildComponents supplies the semantic descendants. Keeping Semantics
	// limited to this container avoids publishing each child twice.
	return semantics.Node{ID: w.CompID, Role: semantics.RoleGroup, Name: "Workspace", Bounds: w.Rect}
}

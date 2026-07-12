package components

import (
	"image"
	"math"
	"time"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/theme"
	"github.com/mulavdm/poem/pkg/render/types"
)

type Spinner struct {
	CompID string
	Rect   image.Rectangle
	Label  string
}

func NewSpinner(id, label string) *Spinner     { return &Spinner{CompID: id, Label: label} }
func (s *Spinner) ID() string                  { return s.CompID }
func (s *Spinner) GetID() string               { return s.CompID }
func (s *Spinner) Bounds() image.Rectangle     { return s.Rect }
func (s *Spinner) SetBounds(r image.Rectangle) { s.Rect = r }
func (s *Spinner) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	t := activeTheme(state)
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(s.Rect), image.Pt(120, t.Controls.Medium)), Min: image.Pt(t.Controls.Medium, t.Controls.Medium)}
}
func (s *Spinner) Draw(p types.Painter, state *types.ApplicationState) {
	t := activeTheme(state)
	size := minValueInt(s.Rect.Dy()-8, 20)
	if size < 12 {
		size = 12
	}
	cx, cy := s.Rect.Min.X+size/2+4, s.Rect.Min.Y+s.Rect.Dy()/2
	phase := 0.0
	if !prefersReducedMotion(state) {
		phase = time.Since(state.StartTime).Seconds() * 5
	}
	for i := 0; i < 8; i++ {
		angle := phase + float64(i)*math.Pi/4
		inner, outer := float64(size)*0.22, float64(size)*0.46
		col := t.Colors.Accent
		col.A = uint8(55 + i*25)
		p.DrawLine(cx+int(math.Cos(angle)*inner), cy+int(math.Sin(angle)*inner), cx+int(math.Cos(angle)*outer), cy+int(math.Sin(angle)*outer), col)
	}
	if s.Label != "" {
		p.DrawText(s.Label, cx+size/2+t.Spacing.SM, s.Rect.Min.Y+s.Rect.Dy()/2+5, t.Colors.TextMuted)
	}
}
func (s *Spinner) HitTest(image.Point) string                            { return "" }
func (s *Spinner) OnKey(uint32, rune, *types.ApplicationState) bool      { return false }
func (s *Spinner) OnMouseDown(image.Point, *types.ApplicationState) bool { return false }
func (s *Spinner) OnMouseUp(image.Point, *types.ApplicationState) bool   { return false }
func (s *Spinner) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (s *Spinner) Focusable() bool                                       { return false }
func (s *Spinner) Walk(fn func(types.Component))                         { fn(s) }
func (s *Spinner) Semantics(*types.ApplicationState) semantics.Node {
	return semantics.Node{ID: s.CompID, Role: semantics.RoleStatus, Name: s.Label, Value: "busy", Bounds: s.Rect}
}

type Skeleton struct {
	CompID string
	Rect   image.Rectangle
	Lines  int
}

func NewSkeleton(id string) *Skeleton           { return &Skeleton{CompID: id, Lines: 3} }
func (s *Skeleton) ID() string                  { return s.CompID }
func (s *Skeleton) GetID() string               { return s.CompID }
func (s *Skeleton) Bounds() image.Rectangle     { return s.Rect }
func (s *Skeleton) SetBounds(r image.Rectangle) { s.Rect = r }
func (s *Skeleton) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	h := s.Lines * 20
	if h < 40 {
		h = 40
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(s.Rect), image.Pt(avail.X, h)), Min: image.Pt(80, 40)}
}
func (s *Skeleton) Draw(p types.Painter, state *types.ApplicationState) {
	t := activeTheme(state)
	lines := s.Lines
	if lines <= 0 {
		lines = 3
	}
	lineH := 10
	gap := 10
	phase := 0.0
	if !prefersReducedMotion(state) {
		phase = (math.Sin(time.Since(state.StartTime).Seconds()*2) + 1) / 2
	}
	base := mix(t.Colors.SurfaceRaised, t.Colors.TextMuted, uint8(18+phase*12))
	for i := 0; i < lines; i++ {
		width := s.Rect.Dx()
		if i == lines-1 {
			width = width * 2 / 3
		}
		top := s.Rect.Min.Y + i*(lineH+gap)
		p.DrawRoundedRect(image.Rect(s.Rect.Min.X, top, s.Rect.Min.X+width, top+lineH), t.Radii.Small, base)
	}
}
func (s *Skeleton) HitTest(image.Point) string                            { return "" }
func (s *Skeleton) OnKey(uint32, rune, *types.ApplicationState) bool      { return false }
func (s *Skeleton) OnMouseDown(image.Point, *types.ApplicationState) bool { return false }
func (s *Skeleton) OnMouseUp(image.Point, *types.ApplicationState) bool   { return false }
func (s *Skeleton) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (s *Skeleton) Focusable() bool                                       { return false }
func (s *Skeleton) Walk(fn func(types.Component))                         { fn(s) }
func (s *Skeleton) Semantics(*types.ApplicationState) semantics.Node {
	return semantics.Node{ID: s.CompID, Role: semantics.RoleStatus, Name: "Loading content", Value: "busy", Bounds: s.Rect}
}

type Tooltip struct {
	CompID  string
	Rect    image.Rectangle
	Text    string
	Visible bool
}

func NewTooltip(id, text string) *Tooltip          { return &Tooltip{CompID: id, Text: text, Visible: true} }
func (tip *Tooltip) ID() string                    { return tip.CompID }
func (tip *Tooltip) GetID() string                 { return tip.CompID }
func (tip *Tooltip) Bounds() image.Rectangle       { return tip.Rect }
func (tip *Tooltip) SetBounds(r image.Rectangle)   { tip.Rect = r }
func (tip *Tooltip) Focusable() bool               { return false }
func (tip *Tooltip) Walk(fn func(types.Component)) { fn(tip) }
func (tip *Tooltip) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	t := activeTheme(state)
	cw := 8
	if state != nil && state.FontCharWidth > 0 {
		cw = state.FontCharWidth
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(tip.Rect), image.Pt(len([]rune(tip.Text))*cw+2*t.Spacing.SM, t.Controls.Small)), Min: image.Pt(40, t.Controls.Small)}
}
func (tip *Tooltip) Draw(p types.Painter, state *types.ApplicationState) {
	if !tip.Visible {
		return
	}
	t := activeTheme(state)
	shadow := t.Elevation.Medium
	p.SetShadow(float32(shadow.OffsetX), float32(shadow.OffsetY), float32(shadow.Blur))
	p.DrawRoundedRect(tip.Rect, t.Radii.Small, t.Colors.SurfaceRaised)
	p.SetShadow(0, 0, 0)
	p.DrawText(tip.Text, tip.Rect.Min.X+t.Spacing.SM, tip.Rect.Min.Y+tip.Rect.Dy()/2+5, t.Colors.Text)
}
func (tip *Tooltip) HitTest(image.Point) string                            { return "" }
func (tip *Tooltip) OnKey(uint32, rune, *types.ApplicationState) bool      { return false }
func (tip *Tooltip) OnMouseDown(image.Point, *types.ApplicationState) bool { return false }
func (tip *Tooltip) OnMouseUp(image.Point, *types.ApplicationState) bool   { return false }
func (tip *Tooltip) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (tip *Tooltip) Semantics(*types.ApplicationState) semantics.Node {
	return semantics.Node{ID: tip.CompID, Role: semantics.RoleToolTip, Name: tip.Text, Bounds: tip.Rect}
}

type Toast struct {
	CompID         string
	Rect           image.Rectangle
	Title, Message string
	Variant        theme.Variant
	Dismissible    bool
	OnDismiss      func(*types.ApplicationState)
}

func NewToast(id, title, message string) *Toast {
	return &Toast{CompID: id, Title: title, Message: message, Dismissible: true}
}
func (t *Toast) ID() string                    { return t.CompID }
func (t *Toast) GetID() string                 { return t.CompID }
func (t *Toast) Bounds() image.Rectangle       { return t.Rect }
func (t *Toast) SetBounds(r image.Rectangle)   { t.Rect = r }
func (t *Toast) Focusable() bool               { return t.Dismissible }
func (t *Toast) Walk(fn func(types.Component)) { fn(t) }
func (t *Toast) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(t.Rect), image.Pt(340, 82)), Min: image.Pt(220, 64)}
}
func (t *Toast) Draw(p types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	visual := buttonVisual(th, t.Variant, false, false, false, false, nil)
	visual.background = th.Colors.SurfaceRaised
	shadow := th.Elevation.High
	p.SetShadow(float32(shadow.OffsetX), float32(shadow.OffsetY), float32(shadow.Blur))
	drawControlSurface(p, t.Rect, visual, state.FocusedID == t.CompID)
	p.SetShadow(0, 0, 0)
	accent := textRoleColor(th, TextAccent)
	switch t.Variant {
	case theme.VariantDanger:
		accent = th.Colors.Danger
	case theme.VariantSuccess:
		accent = th.Colors.Success
	case theme.VariantWarning:
		accent = th.Colors.Warning
	}
	p.FillRect(image.Rect(t.Rect.Min.X, t.Rect.Min.Y, t.Rect.Min.X+4, t.Rect.Max.Y), accent)
	p.DrawText(t.Title, t.Rect.Min.X+th.Spacing.LG, t.Rect.Min.Y+27, th.Colors.Text)
	p.DrawText(t.Message, t.Rect.Min.X+th.Spacing.LG, t.Rect.Min.Y+55, th.Colors.TextMuted)
	if t.Dismissible {
		p.DrawText("x", t.Rect.Max.X-th.Spacing.LG, t.Rect.Min.Y+25, th.Colors.TextMuted)
	}
}
func (t *Toast) HitTest(pt image.Point) string {
	if t.Dismissible && pt.In(t.Rect) {
		return t.CompID
	}
	return ""
}
func (t *Toast) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if t.Dismissible && state.FocusedID == t.CompID && (key == 13 || key == 32 || key == 27) {
		if t.OnDismiss != nil {
			t.OnDismiss(state)
		}
		return true
	}
	return false
}
func (t *Toast) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if !t.Dismissible {
		return false
	}
	state.ActiveID = t.CompID
	return true
}
func (t *Toast) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID != t.CompID || !pt.In(t.Rect) {
		return false
	}
	if t.OnDismiss != nil {
		t.OnDismiss(state)
	}
	return true
}
func (t *Toast) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (t *Toast) Semantics(state *types.ApplicationState) semantics.Node {
	return semantics.Node{ID: t.CompID, Role: semantics.RoleAlert, Name: t.Title, Description: t.Message, Bounds: t.Rect, State: semantics.State{Focused: state != nil && state.FocusedID == t.CompID}}
}

type Toolbar struct {
	CompID   string
	Rect     image.Rectangle
	Children []types.Component
	Gap      int
}

func NewToolbar(id string, children ...types.Component) *Toolbar {
	return &Toolbar{CompID: id, Children: children, Gap: 8}
}
func (t *Toolbar) ID() string              { return t.CompID }
func (t *Toolbar) GetID() string           { return t.CompID }
func (t *Toolbar) Bounds() image.Rectangle { return t.Rect }
func (t *Toolbar) Focusable() bool         { return false }
func (t *Toolbar) SetBounds(r image.Rectangle) {
	t.Rect = r
	gap := t.Gap
	if gap <= 0 {
		gap = 8
	}
	x := r.Min.X + 8
	for _, child := range t.Children {
		size := types.MeasureComponent(child, image.Pt(r.Dx(), r.Dy()), nil).Preferred
		if size.Y > r.Dy()-8 {
			size.Y = r.Dy() - 8
		}
		child.SetBounds(image.Rect(x, r.Min.Y+(r.Dy()-size.Y)/2, x+size.X, r.Min.Y+(r.Dy()-size.Y)/2+size.Y))
		x += size.X + gap
	}
}
func (t *Toolbar) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	th := activeTheme(state)
	w := 2 * th.Spacing.SM
	for _, child := range t.Children {
		w += types.MeasureComponent(child, avail, state).Preferred.X + t.Gap
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(t.Rect), image.Pt(w, th.Controls.Large)), Min: image.Pt(th.Controls.Large, th.Controls.Large)}
}
func (t *Toolbar) Draw(p types.Painter, state *types.ApplicationState) {
	p.DrawRoundedRect(t.Rect, activeTheme(state).Radii.Medium, activeTheme(state).Colors.SurfaceRaised)
	for _, child := range t.Children {
		child.Draw(p, state)
	}
}
func (t *Toolbar) HitTest(pt image.Point) string {
	if !pt.In(t.Rect) {
		return ""
	}
	for i := len(t.Children) - 1; i >= 0; i-- {
		if id := t.Children[i].HitTest(pt); id != "" {
			return id
		}
	}
	return t.CompID
}
func (t *Toolbar) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	for _, child := range t.Children {
		if child.OnKey(key, char, state) {
			return true
		}
	}
	return false
}
func (t *Toolbar) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	for i := len(t.Children) - 1; i >= 0; i-- {
		if pt.In(t.Children[i].Bounds()) && t.Children[i].OnMouseDown(pt, state) {
			return true
		}
	}
	return false
}
func (t *Toolbar) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	for i := len(t.Children) - 1; i >= 0; i-- {
		if t.Children[i].OnMouseUp(pt, state) {
			return true
		}
	}
	return false
}
func (t *Toolbar) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	for i := len(t.Children) - 1; i >= 0; i-- {
		if pt.In(t.Children[i].Bounds()) && t.Children[i].OnMouseMove(pt, state) {
			return true
		}
	}
	return false
}
func (t *Toolbar) Walk(fn func(types.Component)) {
	fn(t)
	for _, child := range t.Children {
		child.Walk(fn)
	}
}

func (t *Toolbar) ChildComponents() []types.Component { return t.Children }
func (t *Toolbar) Semantics(*types.ApplicationState) semantics.Node {
	return semantics.Node{ID: t.CompID, Role: semantics.RoleToolBar, Name: "Toolbar", Bounds: t.Rect}
}

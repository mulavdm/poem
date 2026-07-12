package components

import (
	"image"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/types"
)

type Checkbox struct {
	CompID         string
	Rect           image.Rectangle
	Label          string
	Checked        bool
	Disabled       bool
	Invalid        bool
	AccessibleName string
	OnChange       func(bool, *types.ApplicationState)
}

func NewCheckbox(id, label string, checked bool, onChange func(bool, *types.ApplicationState)) *Checkbox {
	return &Checkbox{CompID: id, Label: label, Checked: checked, OnChange: onChange}
}

func (c *Checkbox) ID() string                  { return c.CompID }
func (c *Checkbox) GetID() string               { return c.CompID }
func (c *Checkbox) Bounds() image.Rectangle     { return c.Rect }
func (c *Checkbox) SetBounds(r image.Rectangle) { c.Rect = r }
func (c *Checkbox) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	t := activeTheme(state)
	charW := 8
	if state != nil && state.FontCharWidth > 0 {
		charW = state.FontCharWidth
	}
	size := image.Pt(t.Controls.Medium+len([]rune(c.Label))*charW, t.Controls.Medium)
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(c.Rect), size), Min: image.Pt(t.Controls.Medium, t.Controls.Medium)}
}
func (c *Checkbox) Draw(p types.Painter, state *types.ApplicationState) {
	t := activeTheme(state)
	boxSize := minValueInt(c.Rect.Dy()-8, 18)
	if boxSize < 12 {
		boxSize = 12
	}
	box := image.Rect(c.Rect.Min.X+2, c.Rect.Min.Y+(c.Rect.Dy()-boxSize)/2, c.Rect.Min.X+2+boxSize, c.Rect.Min.Y+(c.Rect.Dy()-boxSize)/2+boxSize)
	visual := buttonVisual(t, 0, state.HoveredID == c.CompID, state.ActiveID == c.CompID, c.Disabled, c.Checked, nil)
	if c.Checked {
		visual.background, visual.border = t.Colors.Accent, t.Colors.Accent
	}
	if c.Invalid {
		visual.border = t.Colors.Danger
	}
	drawControlSurface(p, box, visual, state.FocusedID == c.CompID && !c.Disabled)
	if c.Checked {
		for offset := 0; offset < 2; offset++ {
			p.DrawLine(box.Min.X+4, box.Min.Y+boxSize/2+offset, box.Min.X+boxSize/2-1, box.Max.Y-4+offset, t.Colors.OnAccent)
			p.DrawLine(box.Min.X+boxSize/2-1, box.Max.Y-4+offset, box.Max.X-3, box.Min.Y+4+offset, t.Colors.OnAccent)
		}
	}
	if c.Label != "" {
		p.DrawText(c.Label, box.Max.X+t.Spacing.SM, c.Rect.Min.Y+c.Rect.Dy()/2+5, visual.foreground)
	}
	if state.HoveredID == c.CompID && !c.Disabled {
		state.CursorID = state.HandCursor
	}
}
func (c *Checkbox) HitTest(pt image.Point) string {
	if pt.In(c.Rect) {
		return c.CompID
	}
	return ""
}
func (c *Checkbox) toggle(state *types.ApplicationState) bool {
	if c.Disabled {
		return false
	}
	next := !c.Checked
	if c.OnChange != nil {
		c.OnChange(next, state)
	} else {
		c.Checked = next
	}
	return true
}
func (c *Checkbox) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID == c.CompID && (key == 13 || key == 32) {
		return c.toggle(state)
	}
	return false
}
func (c *Checkbox) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if c.Disabled {
		return false
	}
	state.ActiveID = c.CompID
	return true
}
func (c *Checkbox) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID != c.CompID {
		return false
	}
	if pt.In(c.Rect) {
		return c.toggle(state)
	}
	return true
}
func (c *Checkbox) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }
func (c *Checkbox) Focusable() bool                                                { return !c.Disabled }
func (c *Checkbox) Walk(fn func(types.Component))                                  { fn(c) }
func (c *Checkbox) Semantics(state *types.ApplicationState) semantics.Node {
	name := c.AccessibleName
	if name == "" {
		name = c.Label
	}
	return semantics.Node{ID: c.CompID, Role: semantics.RoleCheckBox, Name: name, Bounds: c.Rect,
		State:   semantics.State{Checked: c.Checked, Disabled: c.Disabled, Invalid: c.Invalid, Focused: state != nil && state.FocusedID == c.CompID},
		Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionInvoke}}
}

type Switch struct {
	CompID         string
	Rect           image.Rectangle
	Label          string
	Checked        bool
	Disabled       bool
	AccessibleName string
	OnChange       func(bool, *types.ApplicationState)
}

func NewSwitch(id, label string, checked bool, onChange func(bool, *types.ApplicationState)) *Switch {
	return &Switch{CompID: id, Label: label, Checked: checked, OnChange: onChange}
}
func (s *Switch) ID() string                  { return s.CompID }
func (s *Switch) GetID() string               { return s.CompID }
func (s *Switch) Bounds() image.Rectangle     { return s.Rect }
func (s *Switch) SetBounds(r image.Rectangle) { s.Rect = r }
func (s *Switch) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	t := activeTheme(state)
	charW := 8
	if state != nil && state.FontCharWidth > 0 {
		charW = state.FontCharWidth
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(s.Rect), image.Pt(48+t.Spacing.SM+len([]rune(s.Label))*charW, t.Controls.Medium)), Min: image.Pt(48, t.Controls.Medium)}
}
func (s *Switch) Draw(p types.Painter, state *types.ApplicationState) {
	t := activeTheme(state)
	h := minValueInt(22, s.Rect.Dy()-6)
	if h < 14 {
		h = 14
	}
	w := h * 2
	track := image.Rect(s.Rect.Min.X+2, s.Rect.Min.Y+(s.Rect.Dy()-h)/2, s.Rect.Min.X+2+w, s.Rect.Min.Y+(s.Rect.Dy()-h)/2+h)
	trackColor := t.Colors.BorderStrong
	border := t.Colors.BorderStrong
	if s.Checked {
		trackColor, border = t.Colors.Accent, t.Colors.Accent
	}
	if s.Disabled {
		trackColor, border = t.Colors.Surface, t.Colors.Border
	}
	visual := controlVisual{background: trackColor, foreground: t.Colors.Text, border: border, focus: t.Colors.Focus, radius: t.Radii.Pill}
	drawControlSurface(p, track, visual, state.FocusedID == s.CompID && !s.Disabled)
	knob := h - 6
	knobX := track.Min.X + 3
	if s.Checked {
		knobX = track.Max.X - knob - 3
	}
	knobColor := t.Colors.Text
	if s.Checked {
		knobColor = t.Colors.OnAccent
	}
	if s.Disabled {
		knobColor = t.Colors.TextDisabled
	}
	knobRect := image.Rect(knobX, track.Min.Y+3, knobX+knob, track.Min.Y+3+knob)
	p.DrawRoundedRect(knobRect, roundedRadius(knobRect, t.Radii.Pill), knobColor)
	if s.Label != "" {
		p.DrawText(s.Label, track.Max.X+t.Spacing.SM, s.Rect.Min.Y+s.Rect.Dy()/2+5, visual.foreground)
	}
	if state.HoveredID == s.CompID && !s.Disabled {
		state.CursorID = state.HandCursor
	}
}
func (s *Switch) HitTest(pt image.Point) string {
	if pt.In(s.Rect) {
		return s.CompID
	}
	return ""
}
func (s *Switch) toggle(state *types.ApplicationState) bool {
	if s.Disabled {
		return false
	}
	next := !s.Checked
	if s.OnChange != nil {
		s.OnChange(next, state)
	} else {
		s.Checked = next
	}
	return true
}
func (s *Switch) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID == s.CompID && (key == 13 || key == 32) {
		return s.toggle(state)
	}
	return false
}
func (s *Switch) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if s.Disabled {
		return false
	}
	state.ActiveID = s.CompID
	return true
}
func (s *Switch) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID != s.CompID {
		return false
	}
	if pt.In(s.Rect) {
		return s.toggle(state)
	}
	return true
}
func (s *Switch) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }
func (s *Switch) Focusable() bool                                                { return !s.Disabled }
func (s *Switch) Walk(fn func(types.Component))                                  { fn(s) }
func (s *Switch) Semantics(state *types.ApplicationState) semantics.Node {
	name := s.AccessibleName
	if name == "" {
		name = s.Label
	}
	return semantics.Node{ID: s.CompID, Role: semantics.RoleSwitch, Name: name, Bounds: s.Rect, State: semantics.State{Checked: s.Checked, Disabled: s.Disabled, Focused: state != nil && state.FocusedID == s.CompID}, Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionInvoke}}
}

type Radio struct {
	CompID         string
	Rect           image.Rectangle
	Label          string
	Value          string
	Selected       bool
	Disabled       bool
	AccessibleName string
	OnSelect       func(string, *types.ApplicationState)
}

func NewRadio(id, label, value string, selected bool, onSelect func(string, *types.ApplicationState)) *Radio {
	return &Radio{CompID: id, Label: label, Value: value, Selected: selected, OnSelect: onSelect}
}
func (r *Radio) ID() string                  { return r.CompID }
func (r *Radio) GetID() string               { return r.CompID }
func (r *Radio) Bounds() image.Rectangle     { return r.Rect }
func (r *Radio) SetBounds(v image.Rectangle) { r.Rect = v }
func (r *Radio) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	t := activeTheme(state)
	cw := 8
	if state != nil && state.FontCharWidth > 0 {
		cw = state.FontCharWidth
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(r.Rect), image.Pt(t.Controls.Medium+len([]rune(r.Label))*cw, t.Controls.Medium)), Min: image.Pt(t.Controls.Medium, t.Controls.Medium)}
}
func (r *Radio) Draw(p types.Painter, state *types.ApplicationState) {
	t := activeTheme(state)
	sz := minValueInt(r.Rect.Dy()-8, 18)
	if sz < 12 {
		sz = 12
	}
	circle := image.Rect(r.Rect.Min.X+2, r.Rect.Min.Y+(r.Rect.Dy()-sz)/2, r.Rect.Min.X+2+sz, r.Rect.Min.Y+(r.Rect.Dy()-sz)/2+sz)
	visual := buttonVisual(t, 0, state.HoveredID == r.CompID, state.ActiveID == r.CompID, r.Disabled, r.Selected, nil)
	visual.radius = t.Radii.Pill
	if r.Selected {
		visual.border = t.Colors.Accent
	}
	drawControlSurface(p, circle, visual, state.FocusedID == r.CompID && !r.Disabled)
	if r.Selected {
		inset := 4
		p.DrawRoundedRect(image.Rect(circle.Min.X+inset, circle.Min.Y+inset, circle.Max.X-inset, circle.Max.Y-inset), t.Radii.Pill, t.Colors.Accent)
	}
	if r.Label != "" {
		p.DrawText(r.Label, circle.Max.X+t.Spacing.SM, r.Rect.Min.Y+r.Rect.Dy()/2+5, visual.foreground)
	}
	if state.HoveredID == r.CompID && !r.Disabled {
		state.CursorID = state.HandCursor
	}
}
func (r *Radio) HitTest(pt image.Point) string {
	if pt.In(r.Rect) {
		return r.CompID
	}
	return ""
}
func (r *Radio) selectValue(state *types.ApplicationState) bool {
	if r.Disabled {
		return false
	}
	if r.OnSelect != nil {
		r.OnSelect(r.Value, state)
	} else {
		r.Selected = true
	}
	return true
}
func (r *Radio) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID == r.CompID && (key == 13 || key == 32) {
		return r.selectValue(state)
	}
	return false
}
func (r *Radio) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if r.Disabled {
		return false
	}
	state.ActiveID = r.CompID
	return true
}
func (r *Radio) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID != r.CompID {
		return false
	}
	if pt.In(r.Rect) {
		return r.selectValue(state)
	}
	return true
}
func (r *Radio) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }
func (r *Radio) Focusable() bool                                                { return !r.Disabled }
func (r *Radio) Walk(fn func(types.Component))                                  { fn(r) }
func (r *Radio) Semantics(state *types.ApplicationState) semantics.Node {
	name := r.AccessibleName
	if name == "" {
		name = r.Label
	}
	return semantics.Node{ID: r.CompID, Role: semantics.RoleRadioButton, Name: name, Value: r.Value, Bounds: r.Rect, State: semantics.State{Selected: r.Selected, Checked: r.Selected, Disabled: r.Disabled, Focused: state != nil && state.FocusedID == r.CompID}, Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionSelect}}
}

package components

import (
	"image"
	"strings"

	"go_native_gpu_gui/pkg/render/semantics"
	"go_native_gpu_gui/pkg/render/types"
)

type MenuItem struct {
	ID        string
	Label     string
	Shortcut  string
	Disabled  bool
	Checked   bool
	Separator bool
	OnInvoke  func(*types.ApplicationState)
}

type Menu struct {
	CompID      string
	Rect        image.Rectangle
	Items       []MenuItem
	FocusedItem int
	OnDismiss   func(*types.ApplicationState)
}

func NewMenu(id string, items []MenuItem) *Menu {
	menu := &Menu{CompID: id, Items: items, FocusedItem: -1}
	menu.FocusedItem = menu.findNext(-1, 1)
	return menu
}
func (m *Menu) ID() string                    { return m.CompID }
func (m *Menu) GetID() string                 { return m.CompID }
func (m *Menu) Bounds() image.Rectangle       { return m.Rect }
func (m *Menu) Focusable() bool               { return m.findNext(-1, 1) >= 0 }
func (m *Menu) Walk(fn func(types.Component)) { fn(m) }
func (m *Menu) rowHeight(state *types.ApplicationState) int {
	return activeTheme(state).Controls.Medium
}
func (m *Menu) SetBounds(r image.Rectangle) {
	m.Rect = r
	if r.Dy() == 0 {
		m.Rect.Max.Y = r.Min.Y + len(m.Items)*36 + 8
	}
}
func (m *Menu) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	th := activeTheme(state)
	cw := 8
	if state != nil && state.FontCharWidth > 0 {
		cw = state.FontCharWidth
	}
	width := 180
	for _, item := range m.Items {
		candidate := (len([]rune(item.Label))+len([]rune(item.Shortcut)))*cw + 3*th.Spacing.LG
		if candidate > width {
			width = candidate
		}
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(m.Rect), image.Pt(width, len(m.Items)*th.Controls.Medium+2*th.Spacing.XS)), Min: image.Pt(140, th.Controls.Medium)}
}
func (m *Menu) itemRect(index int, state *types.ApplicationState) image.Rectangle {
	th := activeTheme(state)
	return image.Rect(m.Rect.Min.X+th.Spacing.XS, m.Rect.Min.Y+th.Spacing.XS+index*m.rowHeight(state), m.Rect.Max.X-th.Spacing.XS, m.Rect.Min.Y+th.Spacing.XS+(index+1)*m.rowHeight(state))
}
func (m *Menu) Draw(p types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	shadow := th.Elevation.High
	p.SetShadow(float32(shadow.OffsetX), float32(shadow.OffsetY), float32(shadow.Blur))
	p.DrawRoundedRect(m.Rect, th.Radii.Medium, th.Colors.SurfaceRaised)
	p.SetShadow(0, 0, 0)
	cw := state.FontCharWidth
	if cw <= 0 {
		cw = 8
	}
	for index, item := range m.Items {
		r := m.itemRect(index, state)
		if item.Separator {
			y := r.Min.Y + r.Dy()/2
			p.FillRect(image.Rect(r.Min.X+th.Spacing.SM, y, r.Max.X-th.Spacing.SM, y+1), th.Colors.Border)
			continue
		}
		focused := index == m.FocusedItem || state.HoveredID == m.CompID+"/"+item.ID
		visual := buttonVisual(th, 0, focused, false, item.Disabled, item.Checked, nil)
		visual.border = colorZero()
		if focused {
			p.DrawRoundedRect(r, th.Radii.Small, visual.background)
		}
		prefix := ""
		if item.Checked {
			prefix = "* "
		}
		p.DrawText(prefix+item.Label, r.Min.X+th.Spacing.SM, r.Min.Y+r.Dy()/2+5, visual.foreground)
		if item.Shortcut != "" {
			p.DrawText(item.Shortcut, r.Max.X-th.Spacing.SM-len([]rune(item.Shortcut))*cw, r.Min.Y+r.Dy()/2+5, th.Colors.TextMuted)
		}
	}
}
func (m *Menu) HitTest(pt image.Point) string {
	if !pt.In(m.Rect) {
		return ""
	}
	for index, item := range m.Items {
		if !item.Separator && pt.In(m.itemRect(index, nil)) {
			return m.CompID + "/" + item.ID
		}
	}
	return m.CompID
}
func (m *Menu) findNext(start, step int) int {
	if len(m.Items) == 0 {
		return -1
	}
	index := start
	for count := 0; count < len(m.Items); count++ {
		index = (index + step + len(m.Items)) % len(m.Items)
		if !m.Items[index].Separator && !m.Items[index].Disabled {
			return index
		}
	}
	return -1
}
func (m *Menu) invoke(index int, state *types.ApplicationState) bool {
	if index < 0 || index >= len(m.Items) || m.Items[index].Disabled || m.Items[index].Separator {
		return false
	}
	if m.Items[index].OnInvoke != nil {
		m.Items[index].OnInvoke(state)
	}
	if m.OnDismiss != nil {
		m.OnDismiss(state)
	}
	return true
}
func (m *Menu) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state == nil || state.FocusedID != m.CompID {
		return false
	}
	switch key {
	case 0x28:
		m.FocusedItem = m.findNext(m.FocusedItem, 1)
		return true
	case 0x26:
		m.FocusedItem = m.findNext(m.FocusedItem, -1)
		return true
	case 0x24: // Home
		m.FocusedItem = m.findNext(-1, 1)
		return m.FocusedItem >= 0
	case 0x23: // End
		m.FocusedItem = m.findNext(0, -1)
		return m.FocusedItem >= 0
	case 13, 32:
		return m.invoke(m.FocusedItem, state)
	case 27:
		if m.OnDismiss != nil {
			m.OnDismiss(state)
		}
		return true
	}
	if char != 0 {
		needle := strings.ToLower(string(char))
		index := m.FocusedItem
		for count := 0; count < len(m.Items); count++ {
			index = m.findNext(index, 1)
			if index < 0 {
				break
			}
			if strings.HasPrefix(strings.ToLower(m.Items[index].Label), needle) {
				m.FocusedItem = index
				return true
			}
		}
	}
	return false
}
func (m *Menu) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	id := m.HitTest(pt)
	if id == "" || id == m.CompID {
		return false
	}
	for index, item := range m.Items {
		if id == m.CompID+"/"+item.ID && !item.Disabled && !item.Separator {
			state.FocusedID = m.CompID
			m.FocusedItem = index
			state.ActiveID = id
			return true
		}
	}
	return false
}
func (m *Menu) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	id := m.HitTest(pt)
	if id == "" || state.ActiveID != id {
		return false
	}
	for index, item := range m.Items {
		if id == m.CompID+"/"+item.ID {
			return m.invoke(index, state)
		}
	}
	return false
}
func (m *Menu) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	for index, item := range m.Items {
		if !item.Separator && !item.Disabled && pt.In(m.itemRect(index, state)) {
			m.FocusedItem = index
			return true
		}
	}
	return false
}
func (m *Menu) Semantics(state *types.ApplicationState) semantics.Node {
	focused := state != nil && state.FocusedID == m.CompID
	node := semantics.Node{ID: m.CompID, Role: semantics.RoleMenu, Name: "Menu", Bounds: m.Rect,
		State: semantics.State{Focused: focused && m.FocusedItem < 0}, Actions: []semantics.Action{semantics.ActionFocus}}
	for index, item := range m.Items {
		if item.Separator {
			continue
		}
		node.Children = append(node.Children, semantics.Node{ID: m.CompID + "/" + item.ID, Role: semantics.RoleMenuItem, Name: item.Label, Description: item.Shortcut, Bounds: m.itemRect(index, state), State: semantics.State{Disabled: item.Disabled, Checked: item.Checked, Focused: focused && index == m.FocusedItem}, Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionInvoke}})
	}
	return node
}
func (m *Menu) PerformSemanticAction(targetID string, action semantics.Action, value string, state *types.ApplicationState) bool {
	if targetID == m.CompID && action == semantics.ActionFocus {
		state.FocusedID = m.CompID
		if m.FocusedItem < 0 {
			m.FocusedItem = m.findNext(-1, 1)
		}
		return true
	}
	for index, item := range m.Items {
		if targetID == m.CompID+"/"+item.ID {
			switch action {
			case semantics.ActionFocus:
				if item.Disabled || item.Separator {
					return false
				}
				state.FocusedID = m.CompID
				m.FocusedItem = index
				return true
			case semantics.ActionInvoke:
				return m.invoke(index, state)
			}
			return false
		}
	}
	return false
}

type Popover struct {
	CompID   string
	Rect     image.Rectangle
	Children []types.Component
}

func NewPopover(id string, children ...types.Component) *Popover {
	return &Popover{CompID: id, Children: children}
}
func (pv *Popover) ID() string                  { return pv.CompID }
func (pv *Popover) GetID() string               { return pv.CompID }
func (pv *Popover) Bounds() image.Rectangle     { return pv.Rect }
func (pv *Popover) SetBounds(r image.Rectangle) { pv.Rect = r }
func (pv *Popover) Focusable() bool             { return false }
func (pv *Popover) Walk(fn func(types.Component)) {
	fn(pv)
	for _, child := range pv.Children {
		child.Walk(fn)
	}
}

func (pv *Popover) ChildComponents() []types.Component { return pv.Children }
func (pv *Popover) Draw(p types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	shadow := th.Elevation.High
	p.SetShadow(float32(shadow.OffsetX), float32(shadow.OffsetY), float32(shadow.Blur))
	p.DrawRoundedRect(pv.Rect, th.Radii.Large, th.Colors.SurfaceRaised)
	p.SetShadow(0, 0, 0)
	p.PushClip(pv.Rect)
	for _, child := range pv.Children {
		child.Draw(p, state)
	}
	p.PopClip()
}
func (pv *Popover) HitTest(pt image.Point) string {
	if !pt.In(pv.Rect) {
		return ""
	}
	for i := len(pv.Children) - 1; i >= 0; i-- {
		if id := pv.Children[i].HitTest(pt); id != "" {
			return id
		}
	}
	return pv.CompID
}
func (pv *Popover) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	for _, child := range pv.Children {
		if child.OnKey(key, char, state) {
			return true
		}
	}
	return false
}
func (pv *Popover) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	for i := len(pv.Children) - 1; i >= 0; i-- {
		if pt.In(pv.Children[i].Bounds()) && pv.Children[i].OnMouseDown(pt, state) {
			return true
		}
	}
	return true
}
func (pv *Popover) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	for i := len(pv.Children) - 1; i >= 0; i-- {
		if pv.Children[i].OnMouseUp(pt, state) {
			return true
		}
	}
	return true
}
func (pv *Popover) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	for i := len(pv.Children) - 1; i >= 0; i-- {
		if pt.In(pv.Children[i].Bounds()) && pv.Children[i].OnMouseMove(pt, state) {
			return true
		}
	}
	return false
}
func (pv *Popover) Semantics(*types.ApplicationState) semantics.Node {
	return semantics.Node{ID: pv.CompID, Role: semantics.RoleGroup, Name: "Popover", Bounds: pv.Rect}
}

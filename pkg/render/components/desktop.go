package components

import (
	"image"
	"image/color"
	"strings"

	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/theme"
	"github.com/mulavdm/poem/pkg/render/types"
)

type Separator struct {
	CompID   string
	Rect     image.Rectangle
	Vertical bool
}

func NewSeparator(id string) *Separator          { return &Separator{CompID: id} }
func (s *Separator) ID() string                  { return s.CompID }
func (s *Separator) GetID() string               { return s.CompID }
func (s *Separator) Bounds() image.Rectangle     { return s.Rect }
func (s *Separator) SetBounds(r image.Rectangle) { s.Rect = r }
func (s *Separator) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	if s.Vertical {
		return types.MeasureResult{Preferred: image.Pt(1, avail.Y), Min: image.Pt(1, 1)}
	}
	return types.MeasureResult{Preferred: image.Pt(avail.X, 1), Min: image.Pt(1, 1)}
}
func (s *Separator) Draw(p types.Painter, state *types.ApplicationState) {
	p.FillRect(s.Rect, activeTheme(state).Colors.Border)
}
func (s *Separator) HitTest(image.Point) string                            { return "" }
func (s *Separator) OnKey(uint32, rune, *types.ApplicationState) bool      { return false }
func (s *Separator) OnMouseDown(image.Point, *types.ApplicationState) bool { return false }
func (s *Separator) OnMouseUp(image.Point, *types.ApplicationState) bool   { return false }
func (s *Separator) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (s *Separator) Focusable() bool                                       { return false }
func (s *Separator) Walk(fn func(types.Component))                         { fn(s) }
func (s *Separator) Semantics(*types.ApplicationState) semantics.Node {
	return semantics.Node{ID: s.CompID, Role: semantics.RoleSeparator, Bounds: s.Rect}
}

type Badge struct {
	CompID  string
	Rect    image.Rectangle
	Text    string
	Variant theme.Variant
}

func NewBadge(id, text string) *Badge        { return &Badge{CompID: id, Text: text} }
func (b *Badge) ID() string                  { return b.CompID }
func (b *Badge) GetID() string               { return b.CompID }
func (b *Badge) Bounds() image.Rectangle     { return b.Rect }
func (b *Badge) SetBounds(r image.Rectangle) { b.Rect = r }
func (b *Badge) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	t := activeTheme(state)
	cw := 8
	if state != nil && state.FontCharWidth > 0 {
		cw = state.FontCharWidth
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(b.Rect), image.Pt(len([]rune(b.Text))*cw+2*t.Spacing.SM, t.Controls.Small)), Min: image.Pt(t.Controls.Small, t.Controls.Small)}
}
func (b *Badge) Draw(p types.Painter, state *types.ApplicationState) {
	t := activeTheme(state)
	bg, fg := badgeVisual(t, b.Variant)
	p.DrawRoundedRect(b.Rect, roundedRadius(b.Rect, t.Radii.Pill), bg)
	cw := state.FontCharWidth
	if cw <= 0 {
		cw = 8
	}
	p.DrawText(b.Text, b.Rect.Min.X+(b.Rect.Dx()-len([]rune(b.Text))*cw)/2, b.Rect.Min.Y+b.Rect.Dy()/2+5, fg)
}
func (b *Badge) HitTest(image.Point) string                            { return "" }
func (b *Badge) OnKey(uint32, rune, *types.ApplicationState) bool      { return false }
func (b *Badge) OnMouseDown(image.Point, *types.ApplicationState) bool { return false }
func (b *Badge) OnMouseUp(image.Point, *types.ApplicationState) bool   { return false }
func (b *Badge) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (b *Badge) Focusable() bool                                       { return false }
func (b *Badge) Walk(fn func(types.Component))                         { fn(b) }
func (b *Badge) Semantics(*types.ApplicationState) semantics.Node {
	return semantics.Node{ID: b.CompID, Role: semantics.RoleStatus, Name: b.Text, Value: b.Text, Bounds: b.Rect}
}

type TabItem struct {
	ID, Label string
	Disabled  bool
}

type tabsInteraction struct {
	ActiveID string
}

type Tabs struct {
	CompID     string
	Rect       image.Rectangle
	Items      []TabItem
	SelectedID string
	OnChange   func(string, *types.ApplicationState)
}

func NewTabs(id string, items []TabItem, selected string, onChange func(string, *types.ApplicationState)) *Tabs {
	return &Tabs{CompID: id, Items: items, SelectedID: selected, OnChange: onChange}
}
func (t *Tabs) ID() string                  { return t.CompID }
func (t *Tabs) GetID() string               { return t.CompID }
func (t *Tabs) Bounds() image.Rectangle     { return t.Rect }
func (t *Tabs) SetBounds(r image.Rectangle) { t.Rect = r }
func (t *Tabs) interactionKey() string      { return t.CompID + "/tabs" }
func (t *Tabs) activeIndex(state *types.ApplicationState) int {
	activeID := ""
	if state != nil && state.TransientState != nil {
		if interaction, ok := renderstate.Load[tabsInteraction](state.TransientState, t.interactionKey()); ok {
			activeID = interaction.ActiveID
		}
	}
	if activeID == "" {
		activeID = t.SelectedID
	}
	for index, item := range t.Items {
		if item.ID == activeID && !item.Disabled {
			return index
		}
	}
	for index, item := range t.Items {
		if !item.Disabled {
			return index
		}
	}
	return -1
}
func (t *Tabs) storeActive(index int, state *types.ApplicationState) {
	if state == nil || index < 0 || index >= len(t.Items) || t.Items[index].Disabled {
		return
	}
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, t.interactionKey(), tabsInteraction{ActiveID: t.Items[index].ID})
}
func (t *Tabs) selectIndex(index int, state *types.ApplicationState) bool {
	if index < 0 || index >= len(t.Items) || t.Items[index].Disabled {
		return false
	}
	t.storeActive(index, state)
	selected := t.Items[index].ID
	if t.OnChange != nil {
		t.OnChange(selected, state)
	} else {
		t.SelectedID = selected
	}
	return true
}
func (t *Tabs) navigateToIndex(index int, state *types.ApplicationState) bool {
	if index < 0 || index >= len(t.Items) || t.Items[index].Disabled {
		return false
	}
	if index == t.activeIndex(state) {
		t.storeActive(index, state)
		return true
	}
	return t.selectIndex(index, state)
}
func (t *Tabs) adjacentEnabled(from, direction int) int {
	if len(t.Items) == 0 {
		return -1
	}
	for offset := 1; offset <= len(t.Items); offset++ {
		index := (from + direction*offset) % len(t.Items)
		if index < 0 {
			index += len(t.Items)
		}
		if !t.Items[index].Disabled {
			return index
		}
	}
	return -1
}
func (t *Tabs) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	th := activeTheme(state)
	w := 0
	cw := 8
	if state != nil && state.FontCharWidth > 0 {
		cw = state.FontCharWidth
	}
	for _, item := range t.Items {
		w += len([]rune(item.Label))*cw + 2*th.Spacing.MD
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(t.Rect), image.Pt(w, th.Controls.Medium)), Min: image.Pt(th.Controls.Medium, th.Controls.Medium)}
}
func (t *Tabs) itemRects(state *types.ApplicationState) []image.Rectangle {
	th := activeTheme(state)
	cw := 8
	if state != nil && state.FontCharWidth > 0 {
		cw = state.FontCharWidth
	}
	out := make([]image.Rectangle, 0, len(t.Items))
	x := t.Rect.Min.X
	for _, item := range t.Items {
		w := len([]rune(item.Label))*cw + 2*th.Spacing.MD
		out = append(out, image.Rect(x, t.Rect.Min.Y, x+w, t.Rect.Max.Y))
		x += w
	}
	return out
}
func (t *Tabs) Draw(p types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	rects := t.itemRects(state)
	active := t.activeIndex(state)
	for i, item := range t.Items {
		selected := item.ID == t.SelectedID
		visual := buttonVisual(th, 0, state.HoveredID == t.CompID+"/"+item.ID, state.ActiveID == t.CompID+"/"+item.ID, item.Disabled, selected, nil)
		visual.border = colorZero()
		drawControlSurface(p, rects[i], visual, state.FocusedID == t.CompID && i == active)
		cw := state.FontCharWidth
		if cw <= 0 {
			cw = 8
		}
		p.DrawText(item.Label, rects[i].Min.X+(rects[i].Dx()-len([]rune(item.Label))*cw)/2, rects[i].Min.Y+rects[i].Dy()/2+5, visual.foreground)
		if selected {
			p.FillRect(image.Rect(rects[i].Min.X, rects[i].Max.Y-2, rects[i].Max.X, rects[i].Max.Y), th.Colors.Accent)
		}
	}
}
func colorZero() (c color.RGBA) { return c }
func (t *Tabs) HitTest(pt image.Point) string {
	for i, r := range t.itemRects(nil) {
		if pt.In(r) && i < len(t.Items) {
			return t.CompID + "/" + t.Items[i].ID
		}
	}
	if pt.In(t.Rect) {
		return t.CompID
	}
	return ""
}
func (t *Tabs) OnKey(key uint32, _ rune, state *types.ApplicationState) bool {
	if state == nil || state.FocusedID != t.CompID {
		return false
	}
	active := t.activeIndex(state)
	if active < 0 {
		return false
	}
	switch key {
	case 0x25, 0x26: // Left or Up
		return t.navigateToIndex(t.adjacentEnabled(active, -1), state)
	case 0x27, 0x28: // Right or Down
		return t.navigateToIndex(t.adjacentEnabled(active, 1), state)
	case 0x24: // Home
		return t.navigateToIndex(t.adjacentEnabled(-1, 1), state)
	case 0x23: // End
		return t.navigateToIndex(t.adjacentEnabled(0, -1), state)
	case 13, 32: // Enter or Space
		return t.selectIndex(active, state)
	}
	return false
}
func (t *Tabs) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	id := t.HitTest(pt)
	if id == "" || id == t.CompID {
		return false
	}
	for index, item := range t.Items {
		if id == t.CompID+"/"+item.ID && !item.Disabled {
			state.FocusedID = t.CompID
			t.storeActive(index, state)
			state.ActiveID = id
			return true
		}
	}
	return false
}
func (t *Tabs) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	id := t.HitTest(pt)
	if id == "" || state.ActiveID != id {
		return false
	}
	selected := strings.TrimPrefix(id, t.CompID+"/")
	for index, item := range t.Items {
		if item.ID == selected {
			return t.selectIndex(index, state)
		}
	}
	return false
}
func (t *Tabs) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (t *Tabs) Focusable() bool {
	for _, item := range t.Items {
		if !item.Disabled {
			return true
		}
	}
	return false
}
func (t *Tabs) Walk(fn func(types.Component)) { fn(t) }
func (t *Tabs) Semantics(state *types.ApplicationState) semantics.Node {
	focused := state != nil && state.FocusedID == t.CompID
	active := t.activeIndex(state)
	node := semantics.Node{ID: t.CompID, Role: semantics.RoleTabList, Bounds: t.Rect,
		State: semantics.State{Focused: focused && active < 0}, Actions: []semantics.Action{semantics.ActionFocus},
		Collection: &semantics.CollectionValue{Selectable: true, SelectionRequired: true}}
	rects := t.itemRects(state)
	for i, item := range t.Items {
		bounds := image.Rectangle{}
		if i < len(rects) {
			bounds = rects[i]
		}
		node.Children = append(node.Children, semantics.Node{ID: t.CompID + "/" + item.ID, Role: semantics.RoleTab, Name: item.Label, Bounds: bounds, State: semantics.State{Selected: item.ID == t.SelectedID, Disabled: item.Disabled, Focused: focused && i == active}, Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionSelect}})
	}
	return node
}
func (t *Tabs) PerformSemanticAction(targetID string, action semantics.Action, value string, state *types.ApplicationState) bool {
	if targetID == t.CompID && action == semantics.ActionFocus {
		state.FocusedID = t.CompID
		if active := t.activeIndex(state); active >= 0 {
			t.storeActive(active, state)
		}
		return true
	}
	selected := strings.TrimPrefix(targetID, t.CompID+"/")
	if selected == targetID {
		return false
	}
	for index, item := range t.Items {
		if item.ID == selected && !item.Disabled {
			switch action {
			case semantics.ActionFocus:
				state.FocusedID = t.CompID
				t.storeActive(index, state)
				return true
			case semantics.ActionSelect:
				return t.selectIndex(index, state)
			}
			return false
		}
	}
	return false
}

type SelectOption struct {
	Value, Label string
	Disabled     bool
}

type selectInteraction struct {
	Highlighted int
}

type Select struct {
	CompID       string
	Rect         image.Rectangle
	Options      []SelectOption
	Value        string
	Open         bool
	Disabled     bool
	Placeholder  string
	OnChange     func(string, *types.ApplicationState)
	OnOpenChange func(bool, *types.ApplicationState)
}

func NewSelect(id string, options []SelectOption, value string, onChange func(string, *types.ApplicationState)) *Select {
	return &Select{CompID: id, Options: options, Value: value, OnChange: onChange}
}
func (s *Select) ID() string                  { return s.CompID }
func (s *Select) GetID() string               { return s.CompID }
func (s *Select) Bounds() image.Rectangle     { return s.Rect }
func (s *Select) SetBounds(r image.Rectangle) { s.Rect = r }
func (s *Select) overlayID() string           { return s.CompID + ".options" }
func (s *Select) interactionKey() string      { return s.CompID + "/select" }
func (s *Select) overlayOpen(state *types.ApplicationState) bool {
	if state == nil || state.Overlays == nil {
		return false
	}
	for _, overlay := range state.Overlays.Snapshot() {
		if overlay.ID == s.overlayID() {
			return true
		}
	}
	return false
}
func (s *Select) initialHighlighted() int {
	for index, option := range s.Options {
		if option.Value == s.Value && !option.Disabled {
			return index
		}
	}
	for index, option := range s.Options {
		if !option.Disabled {
			return index
		}
	}
	return -1
}
func (s *Select) highlighted(state *types.ApplicationState) int {
	if state != nil && state.TransientState != nil {
		if interaction, ok := renderstate.Load[selectInteraction](state.TransientState, s.interactionKey()); ok {
			if interaction.Highlighted >= 0 && interaction.Highlighted < len(s.Options) && !s.Options[interaction.Highlighted].Disabled {
				return interaction.Highlighted
			}
		}
	}
	return s.initialHighlighted()
}
func (s *Select) setHighlighted(index int, state *types.ApplicationState) {
	if state == nil || index < 0 || index >= len(s.Options) || s.Options[index].Disabled {
		return
	}
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, s.interactionKey(), selectInteraction{Highlighted: index})
}
func (s *Select) adjacentEnabled(from, direction int) int {
	if len(s.Options) == 0 {
		return -1
	}
	for offset := 1; offset <= len(s.Options); offset++ {
		index := (from + direction*offset) % len(s.Options)
		if index < 0 {
			index += len(s.Options)
		}
		if !s.Options[index].Disabled {
			return index
		}
	}
	return -1
}
func (s *Select) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	th := activeTheme(state)
	w := avail.X
	if w <= 0 {
		w = 200
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(s.Rect), image.Pt(w, th.Controls.Medium)), Min: image.Pt(120, th.Controls.Medium)}
}
func (s *Select) label() string {
	for _, o := range s.Options {
		if o.Value == s.Value {
			return o.Label
		}
	}
	return s.Placeholder
}
func (s *Select) Draw(p types.Painter, state *types.ApplicationState) {
	if s.Open && !s.overlayOpen(state) {
		s.openOptions(state, false)
	}
	th := activeTheme(state)
	open := s.overlayOpen(state)
	visual := buttonVisual(th, 0, state.HoveredID == s.CompID, state.ActiveID == s.CompID, s.Disabled, open, nil)
	drawControlSurface(p, s.Rect, visual, state.FocusedID == s.CompID && !s.Disabled)
	p.DrawText(s.label(), s.Rect.Min.X+th.Spacing.MD, s.Rect.Min.Y+s.Rect.Dy()/2+5, visual.foreground)
	indicator := "v"
	if open {
		indicator = "^"
	}
	p.DrawText(indicator, s.Rect.Max.X-th.Spacing.LG, s.Rect.Min.Y+s.Rect.Dy()/2+5, visual.foreground)
}
func (s *Select) HitTest(pt image.Point) string {
	if pt.In(s.Rect) {
		return s.CompID
	}
	return ""
}
func (s *Select) publishOpen(next bool, state *types.ApplicationState) {
	if s.OnOpenChange != nil {
		s.OnOpenChange(next, state)
	} else {
		s.Open = next
	}
}
func (s *Select) popup(state *types.ApplicationState) *selectPopup {
	highlighted := s.highlighted(state)
	rowHeight := activeTheme(state).Controls.Medium
	th := activeTheme(state)
	popup := &selectPopup{
		CompID:      s.overlayID(),
		Rect:        image.Rect(s.Rect.Min.X, s.Rect.Max.Y+th.Spacing.XS, s.Rect.Max.X, s.Rect.Max.Y+th.Spacing.XS+len(s.Options)*rowHeight+2),
		OwnerID:     s.CompID,
		Options:     s.Options,
		Value:       s.Value,
		Highlighted: highlighted,
	}
	popup.OnHighlight = func(index int, inner *types.ApplicationState) { s.setHighlighted(index, inner) }
	popup.OnSelect = func(index int, inner *types.ApplicationState) { s.selectIndex(index, inner) }
	return popup
}
func (s *Select) openOptions(state *types.ApplicationState, notify bool) bool {
	if state == nil || s.Disabled || s.initialHighlighted() < 0 {
		return false
	}
	state.OpenAnchoredOverlayWithDismiss(s.overlayID(), s.popup(state), true, func(inner *types.ApplicationState) {
		s.publishOpen(false, inner)
	})
	if notify {
		s.publishOpen(true, state)
	}
	return true
}
func (s *Select) closeOptions(state *types.ApplicationState, notify bool) bool {
	wasOpen := s.overlayOpen(state)
	if wasOpen && state != nil {
		state.CloseOverlay(s.overlayID())
	}
	if notify && !wasOpen && s.Open {
		s.publishOpen(false, state)
	}
	return wasOpen || s.Open
}
func (s *Select) setOpen(next bool, state *types.ApplicationState) bool {
	if next {
		if s.overlayOpen(state) {
			return true
		}
		return s.openOptions(state, true)
	}
	return s.closeOptions(state, true)
}
func (s *Select) selectIndex(index int, state *types.ApplicationState) bool {
	if index < 0 || index >= len(s.Options) || s.Options[index].Disabled {
		return false
	}
	s.setHighlighted(index, state)
	value := s.Options[index].Value
	if s.OnChange != nil {
		s.OnChange(value, state)
	} else {
		s.Value = value
	}
	s.closeOptions(state, true)
	return true
}
func (s *Select) moveHighlight(index int, state *types.ApplicationState) bool {
	if index < 0 {
		return false
	}
	s.setHighlighted(index, state)
	if s.overlayOpen(state) {
		return s.openOptions(state, false)
	}
	if s.Options[index].Value == s.Value {
		return true
	}
	return s.selectIndex(index, state)
}
func (s *Select) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state == nil || s.Disabled || state.FocusedID != s.CompID {
		return false
	}
	open := s.overlayOpen(state)
	highlighted := s.highlighted(state)
	switch key {
	case 13: // Enter
		if open {
			return s.selectIndex(highlighted, state)
		}
		return s.setOpen(true, state)
	case 32: // Space
		if open {
			return s.selectIndex(highlighted, state)
		}
		return s.setOpen(true, state)
	case 27: // Escape
		if open {
			return s.setOpen(false, state)
		}
	case 0x28: // Down
		return s.moveHighlight(s.adjacentEnabled(highlighted, 1), state)
	case 0x26: // Up
		return s.moveHighlight(s.adjacentEnabled(highlighted, -1), state)
	case 0x24: // Home
		return s.moveHighlight(s.adjacentEnabled(-1, 1), state)
	case 0x23: // End
		return s.moveHighlight(s.adjacentEnabled(0, -1), state)
	}
	if char != 0 {
		needle := strings.ToLower(string(char))
		index := highlighted
		for count := 0; count < len(s.Options); count++ {
			index = s.adjacentEnabled(index, 1)
			if index < 0 {
				break
			}
			if strings.HasPrefix(strings.ToLower(s.Options[index].Label), needle) {
				return s.moveHighlight(index, state)
			}
		}
	}
	return false
}
func (s *Select) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if s.Disabled {
		return false
	}
	state.FocusedID = s.CompID
	state.ActiveID = s.HitTest(pt)
	return state.ActiveID != ""
}
func (s *Select) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	id := s.HitTest(pt)
	if id == "" || id != state.ActiveID {
		return false
	}
	if id == s.CompID {
		return s.setOpen(!s.overlayOpen(state), state)
	}
	return false
}
func (s *Select) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (s *Select) Focusable() bool                                       { return !s.Disabled }
func (s *Select) Walk(fn func(types.Component))                         { fn(s) }
func (s *Select) Semantics(state *types.ApplicationState) semantics.Node {
	open := s.overlayOpen(state)
	return semantics.Node{ID: s.CompID, Role: semantics.RoleComboBox, Name: s.Placeholder, Value: s.label(), Bounds: s.Rect, State: semantics.State{Disabled: s.Disabled, Expanded: open, Focused: state != nil && state.FocusedID == s.CompID && !open}, Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionExpand, semantics.ActionCollapse, semantics.ActionSetValue}}
}
func (s *Select) PerformSemanticAction(targetID string, action semantics.Action, value string, state *types.ApplicationState) bool {
	if targetID != s.CompID || s.Disabled {
		return false
	}
	switch action {
	case semantics.ActionFocus:
		state.FocusedID = s.CompID
		return true
	case semantics.ActionExpand:
		return s.setOpen(true, state)
	case semantics.ActionCollapse:
		if !s.overlayOpen(state) {
			return true
		}
		return s.setOpen(false, state)
	case semantics.ActionSetValue:
		for index, option := range s.Options {
			if (option.Value == value || option.Label == value) && !option.Disabled {
				return s.selectIndex(index, state)
			}
		}
	}
	return false
}

type selectPopup struct {
	CompID      string
	Rect        image.Rectangle
	OwnerID     string
	Options     []SelectOption
	Value       string
	Highlighted int
	OnHighlight func(int, *types.ApplicationState)
	OnSelect    func(int, *types.ApplicationState)
}

func (p *selectPopup) ID() string                    { return p.CompID }
func (p *selectPopup) GetID() string                 { return p.CompID }
func (p *selectPopup) Bounds() image.Rectangle       { return p.Rect }
func (p *selectPopup) SetBounds(r image.Rectangle)   { p.Rect = r }
func (p *selectPopup) Focusable() bool               { return false }
func (p *selectPopup) Walk(fn func(types.Component)) { fn(p) }
func (p *selectPopup) rowHeight(state *types.ApplicationState) int {
	return activeTheme(state).Controls.Medium
}
func (p *selectPopup) rowRect(index int, state *types.ApplicationState) image.Rectangle {
	h := p.rowHeight(state)
	return image.Rect(p.Rect.Min.X+1, p.Rect.Min.Y+1+index*h, p.Rect.Max.X-1, p.Rect.Min.Y+1+(index+1)*h)
}
func (p *selectPopup) Draw(painter types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	painter.SetShadow(float32(th.Elevation.High.OffsetX), float32(th.Elevation.High.OffsetY), float32(th.Elevation.High.Blur))
	painter.DrawRoundedRect(p.Rect, th.Radii.Medium, th.Colors.SurfaceRaised)
	painter.SetShadow(0, 0, 0)
	for index, option := range p.Options {
		r := p.rowRect(index, state)
		id := p.CompID + "/" + option.Value
		visual := buttonVisual(th, 0, state.HoveredID == id, false, option.Disabled, option.Value == p.Value, nil)
		visual.border = colorZero()
		if state.HoveredID == id || index == p.Highlighted {
			drawControlSurface(painter, r, visual, false)
		}
		painter.DrawText(option.Label, r.Min.X+th.Spacing.MD, r.Min.Y+r.Dy()/2+5, visual.foreground)
	}
}
func (p *selectPopup) HitTest(pt image.Point) string {
	if !pt.In(p.Rect) {
		return ""
	}
	index := (pt.Y - p.Rect.Min.Y - 1) / p.rowHeight(nil)
	if index >= 0 && index < len(p.Options) {
		return p.CompID + "/" + p.Options[index].Value
	}
	return p.CompID
}
func (p *selectPopup) OnKey(uint32, rune, *types.ApplicationState) bool { return false }
func (p *selectPopup) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	id := p.HitTest(pt)
	for index, option := range p.Options {
		if id == p.CompID+"/"+option.Value && !option.Disabled {
			state.ActiveID = id
			if p.OnHighlight != nil {
				p.OnHighlight(index, state)
			}
			return true
		}
	}
	return false
}
func (p *selectPopup) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	id := p.HitTest(pt)
	if id == "" || id != state.ActiveID {
		return false
	}
	return p.PerformSemanticAction(id, semantics.ActionSelect, "", state)
}
func (p *selectPopup) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	id := p.HitTest(pt)
	for index, option := range p.Options {
		if id == p.CompID+"/"+option.Value && !option.Disabled {
			p.Highlighted = index
			if p.OnHighlight != nil {
				p.OnHighlight(index, state)
			}
			return true
		}
	}
	return false
}
func (p *selectPopup) Semantics(state *types.ApplicationState) semantics.Node {
	node := semantics.Node{ID: p.CompID, Role: semantics.RoleListBox, Name: "Options", Bounds: p.Rect,
		Collection: &semantics.CollectionValue{Selectable: true, SelectionRequired: true}}
	for index, option := range p.Options {
		node.Children = append(node.Children, semantics.Node{ID: p.CompID + "/" + option.Value, Role: semantics.RoleOption, Name: option.Label, Value: option.Value, Bounds: p.rowRect(index, state), State: semantics.State{Disabled: option.Disabled, Selected: option.Value == p.Value, Focused: state != nil && state.FocusedID == p.OwnerID && index == p.Highlighted}, Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionSelect}})
	}
	return node
}
func (p *selectPopup) PerformSemanticAction(targetID string, action semantics.Action, _ string, state *types.ApplicationState) bool {
	value := strings.TrimPrefix(targetID, p.CompID+"/")
	if value == targetID {
		return false
	}
	for index, option := range p.Options {
		if option.Value != value || option.Disabled {
			continue
		}
		switch action {
		case semantics.ActionFocus:
			state.FocusedID = p.OwnerID
			p.Highlighted = index
			if p.OnHighlight != nil {
				p.OnHighlight(index, state)
			}
			return true
		case semantics.ActionSelect:
			if p.OnSelect != nil {
				p.OnSelect(index, state)
			}
			return true
		}
		return false
	}
	return false
}

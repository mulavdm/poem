package components

import (
	"image"
	"strings"

	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

type AccordionItem struct {
	ID       string
	Title    string
	Disabled bool
	Content  types.Component
}

type accordionInteraction struct {
	HeaderIndex int
}

type accordionLayoutItem struct {
	header  image.Rectangle
	content image.Rectangle
}

// Accordion organizes content into keyboard-accessible disclosure sections.
// Expanded values are application-owned when OnToggle is provided.
type Accordion struct {
	CompID   string
	Rect     image.Rectangle
	Items    []AccordionItem
	Expanded map[string]bool
	Multiple bool
	OnToggle func(string, bool, *types.ApplicationState)

	layout []accordionLayoutItem
}

func NewAccordion(id string, items []AccordionItem, expanded map[string]bool, onToggle func(string, bool, *types.ApplicationState)) *Accordion {
	if expanded == nil {
		expanded = make(map[string]bool)
	}
	return &Accordion{CompID: id, Items: items, Expanded: expanded, Multiple: true, OnToggle: onToggle}
}
func (a *Accordion) ID() string                  { return a.CompID }
func (a *Accordion) GetID() string               { return a.CompID }
func (a *Accordion) Bounds() image.Rectangle     { return a.Rect }
func (a *Accordion) SetBounds(r image.Rectangle) { a.Rect = r }
func (a *Accordion) Focusable() bool             { return a.firstEnabled() >= 0 }
func (a *Accordion) Walk(fn func(types.Component)) {
	fn(a)
	for _, item := range a.Items {
		if a.Expanded[item.ID] && item.Content != nil {
			item.Content.Walk(fn)
		}
	}
}
func (a *Accordion) ChildComponents() []types.Component {
	children := make([]types.Component, 0, len(a.Items))
	for _, item := range a.Items {
		if a.Expanded[item.ID] && item.Content != nil {
			children = append(children, item.Content)
		}
	}
	return children
}
func (a *Accordion) interactionKey() string { return a.CompID + "/accordion" }
func (a *Accordion) firstEnabled() int {
	for index, item := range a.Items {
		if !item.Disabled {
			return index
		}
	}
	return -1
}
func (a *Accordion) lastEnabled() int {
	for index := len(a.Items) - 1; index >= 0; index-- {
		if !a.Items[index].Disabled {
			return index
		}
	}
	return -1
}
func (a *Accordion) nextEnabled(from, direction int) int {
	if len(a.Items) == 0 {
		return -1
	}
	for offset := 1; offset <= len(a.Items); offset++ {
		index := (from + direction*offset) % len(a.Items)
		if index < 0 {
			index += len(a.Items)
		}
		if !a.Items[index].Disabled {
			return index
		}
	}
	return -1
}
func (a *Accordion) activeHeader(state *types.ApplicationState) int {
	interaction := a.interaction(state)
	if interaction.HeaderIndex >= 0 && interaction.HeaderIndex < len(a.Items) && !a.Items[interaction.HeaderIndex].Disabled {
		return interaction.HeaderIndex
	}
	return a.firstEnabled()
}
func (a *Accordion) interaction(state *types.ApplicationState) accordionInteraction {
	if state == nil || state.TransientState == nil {
		return accordionInteraction{HeaderIndex: a.firstEnabled()}
	}
	if interaction, ok := renderstate.Load[accordionInteraction](state.TransientState, a.interactionKey()); ok {
		return interaction
	}
	interaction := accordionInteraction{HeaderIndex: a.firstEnabled()}
	renderstate.StoreValue(state.TransientState, a.interactionKey(), interaction)
	return interaction
}
func (a *Accordion) storeInteraction(state *types.ApplicationState, interaction accordionInteraction) {
	if state == nil {
		return
	}
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, a.interactionKey(), interaction)
}
func (a *Accordion) contentHeight(item AccordionItem, width int, state *types.ApplicationState) int {
	if item.Content == nil {
		return 0
	}
	if measurable, ok := item.Content.(types.MeasurableComponent); ok {
		measured := measurable.Measure(image.Pt(width, 10000), state)
		if measured.Preferred.Y > 0 {
			return measured.Preferred.Y
		}
	}
	if height := item.Content.Bounds().Dy(); height > 0 {
		return height
	}
	return activeTheme(state).Controls.Large
}
func (a *Accordion) rebuildLayout(state *types.ApplicationState) []accordionLayoutItem {
	if cap(a.layout) < len(a.Items) {
		a.layout = make([]accordionLayoutItem, len(a.Items))
	} else {
		a.layout = a.layout[:len(a.Items)]
	}
	th := activeTheme(state)
	headerHeight := th.Controls.Medium
	y := a.Rect.Min.Y
	contentWidth := maxInt(0, a.Rect.Dx()-2*th.Spacing.MD)
	for index, item := range a.Items {
		a.layout[index].header = image.Rect(a.Rect.Min.X, y, a.Rect.Max.X, y+headerHeight)
		y += headerHeight
		a.layout[index].content = image.Rectangle{}
		if a.Expanded[item.ID] && item.Content != nil {
			height := a.contentHeight(item, contentWidth, state)
			a.layout[index].content = image.Rect(a.Rect.Min.X+th.Spacing.MD, y+th.Spacing.SM, a.Rect.Max.X-th.Spacing.MD, y+th.Spacing.SM+height)
			item.Content.SetBounds(a.layout[index].content)
			y = a.layout[index].content.Max.Y + th.Spacing.SM
		}
	}
	return a.layout
}
func (a *Accordion) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	th := activeTheme(state)
	width := avail.X
	if width <= 0 {
		width = 320
	}
	height := len(a.Items) * th.Controls.Medium
	contentWidth := maxInt(0, width-2*th.Spacing.MD)
	for _, item := range a.Items {
		if a.Expanded[item.ID] && item.Content != nil {
			height += a.contentHeight(item, contentWidth, state) + 2*th.Spacing.SM
		}
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(a.Rect), image.Pt(width, height)), Min: image.Pt(160, len(a.Items)*th.Controls.Medium)}
}
func (a *Accordion) Draw(p types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	active := a.activeHeader(state)
	layouts := a.rebuildLayout(state)
	for index, item := range a.Items {
		layout := layouts[index]
		id := a.CompID + "/" + item.ID
		expanded := a.Expanded[item.ID]
		visual := buttonVisual(th, 0, state != nil && state.HoveredID == id, state != nil && state.ActiveID == id, item.Disabled, false, nil)
		visual.border = colorZero()
		focused := state != nil && state.FocusedID == a.CompID && active == index
		if expanded || focused || state != nil && state.HoveredID == id {
			drawControlSurface(p, layout.header, visual, focused)
		}
		marker := ">"
		if expanded {
			marker = "v"
		}
		p.DrawText(marker, layout.header.Min.X+th.Spacing.SM, layout.header.Min.Y+layout.header.Dy()/2+5, th.Colors.TextMuted)
		p.DrawText(item.Title, layout.header.Min.X+th.Spacing.XL, layout.header.Min.Y+layout.header.Dy()/2+5, visual.foreground)
		p.FillRect(image.Rect(layout.header.Min.X, layout.header.Max.Y-1, layout.header.Max.X, layout.header.Max.Y), th.Colors.Border)
		if expanded && item.Content != nil {
			p.PushClip(layout.content)
			item.Content.Draw(p, state)
			p.PopClip()
		}
	}
}
func (a *Accordion) headerAt(pt image.Point, state *types.ApplicationState) int {
	for index, layout := range a.rebuildLayout(state) {
		if pt.In(layout.header) {
			return index
		}
	}
	return -1
}
func (a *Accordion) HitTest(pt image.Point) string {
	layouts := a.layout
	if len(layouts) != len(a.Items) {
		layouts = a.rebuildLayout(nil)
	}
	for index, layout := range layouts {
		if pt.In(layout.header) {
			return a.CompID + "/" + a.Items[index].ID
		}
		if !layout.content.Empty() && pt.In(layout.content) && a.Items[index].Content != nil {
			return a.Items[index].Content.HitTest(pt)
		}
	}
	return ""
}
func (a *Accordion) toggle(index int, state *types.ApplicationState) bool {
	if index < 0 || index >= len(a.Items) || a.Items[index].Disabled {
		return false
	}
	item := a.Items[index]
	next := !a.Expanded[item.ID]
	if a.OnToggle != nil {
		a.OnToggle(item.ID, next, state)
		return true
	}
	if next && !a.Multiple {
		for id := range a.Expanded {
			a.Expanded[id] = false
		}
	}
	a.Expanded[item.ID] = next
	return true
}
func (a *Accordion) OnKey(key uint32, _ rune, state *types.ApplicationState) bool {
	if state == nil || state.FocusedID != a.CompID || a.firstEnabled() < 0 {
		return false
	}
	interaction := accordionInteraction{HeaderIndex: a.activeHeader(state)}
	switch key {
	case 0x28:
		interaction.HeaderIndex = a.nextEnabled(interaction.HeaderIndex, 1)
		a.storeInteraction(state, interaction)
		return true
	case 0x26:
		interaction.HeaderIndex = a.nextEnabled(interaction.HeaderIndex, -1)
		a.storeInteraction(state, interaction)
		return true
	case 0x24:
		interaction.HeaderIndex = a.firstEnabled()
		a.storeInteraction(state, interaction)
		return true
	case 0x23:
		interaction.HeaderIndex = a.lastEnabled()
		a.storeInteraction(state, interaction)
		return true
	case 13, 32:
		return a.toggle(interaction.HeaderIndex, state)
	}
	return false
}
func (a *Accordion) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if index := a.headerAt(pt, state); index >= 0 {
		if a.Items[index].Disabled {
			return false
		}
		state.FocusedID = a.CompID
		state.ActiveID = a.CompID + "/" + a.Items[index].ID
		a.storeInteraction(state, accordionInteraction{HeaderIndex: index})
		return true
	}
	for index, layout := range a.rebuildLayout(state) {
		if !layout.content.Empty() && pt.In(layout.content) && a.Items[index].Content != nil {
			return a.Items[index].Content.OnMouseDown(pt, state)
		}
	}
	return false
}
func (a *Accordion) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if index := a.headerAt(pt, state); index >= 0 {
		id := a.CompID + "/" + a.Items[index].ID
		return state.ActiveID == id && a.toggle(index, state)
	}
	for index, layout := range a.rebuildLayout(state) {
		if !layout.content.Empty() && pt.In(layout.content) && a.Items[index].Content != nil && a.Items[index].Content.OnMouseUp(pt, state) {
			return true
		}
	}
	return false
}
func (a *Accordion) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	for index, layout := range a.rebuildLayout(state) {
		if !layout.content.Empty() && pt.In(layout.content) && a.Items[index].Content != nil {
			return a.Items[index].Content.OnMouseMove(pt, state)
		}
	}
	return false
}
func (a *Accordion) Semantics(state *types.ApplicationState) semantics.Node {
	focused := state != nil && state.FocusedID == a.CompID
	active := a.activeHeader(state)
	actions := []semantics.Action(nil)
	if active >= 0 {
		actions = []semantics.Action{semantics.ActionFocus}
	}
	node := semantics.Node{ID: a.CompID, Role: semantics.RoleGroup, Name: "Accordion", Bounds: a.Rect, State: semantics.State{Focused: focused && active < 0}, Actions: actions}
	layouts := a.rebuildLayout(state)
	for index, item := range a.Items {
		itemActions := []semantics.Action(nil)
		if !item.Disabled {
			itemActions = append(itemActions, semantics.ActionFocus)
			if a.Expanded[item.ID] {
				itemActions = append(itemActions, semantics.ActionCollapse)
			} else {
				itemActions = append(itemActions, semantics.ActionExpand)
			}
		}
		node.Children = append(node.Children, semantics.Node{ID: a.CompID + "/" + item.ID, Role: semantics.RoleButton, Name: item.Title, Bounds: layouts[index].header, State: semantics.State{Disabled: item.Disabled, Expanded: a.Expanded[item.ID], Focused: focused && active == index}, Actions: itemActions})
	}
	return node
}
func (a *Accordion) PerformSemanticAction(targetID string, action semantics.Action, _ string, state *types.ApplicationState) bool {
	if targetID == a.CompID && action == semantics.ActionFocus {
		active := a.activeHeader(state)
		if active < 0 {
			return false
		}
		state.FocusedID = a.CompID
		a.storeInteraction(state, accordionInteraction{HeaderIndex: active})
		return true
	}
	id := strings.TrimPrefix(targetID, a.CompID+"/")
	if id == targetID {
		return false
	}
	for index, item := range a.Items {
		if item.ID != id || item.Disabled {
			continue
		}
		switch action {
		case semantics.ActionFocus:
			state.FocusedID = a.CompID
			a.storeInteraction(state, accordionInteraction{HeaderIndex: index})
			return true
		case semantics.ActionExpand:
			if a.Expanded[item.ID] {
				return true
			}
			return a.toggle(index, state)
		case semantics.ActionCollapse:
			if !a.Expanded[item.ID] {
				return true
			}
			return a.toggle(index, state)
		}
		return false
	}
	return false
}

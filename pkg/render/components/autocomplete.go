package components

import (
	"image"
	"strings"

	"go_native_gpu_gui/pkg/render/semantics"
	renderstate "go_native_gpu_gui/pkg/render/state"
	"go_native_gpu_gui/pkg/render/types"
)

type AutocompleteOption struct {
	Value       string
	Label       string
	Description string
	Disabled    bool
}

type autocompleteInteraction struct {
	Highlighted int
}

// Autocomplete is a controlled text field with an overlay-backed suggestion
// list. Query and selected values remain application-owned; POEM only keeps
// transient highlight and input interaction state.
type Autocomplete struct {
	CompID         string
	Rect           image.Rectangle
	Query          string
	Placeholder    string
	Options        []AutocompleteOption
	Disabled       bool
	ReadOnly       bool
	Invalid        bool
	AccessibleName string
	MaxVisible     int
	Highlighted    int
	OnQueryChange  func(string, *types.ApplicationState)
	OnSelect       func(string, *types.ApplicationState)
	OnOpenChange   func(bool, *types.ApplicationState)

	input TextInput
}

func NewAutocomplete(id, placeholder, query string, options []AutocompleteOption, onQueryChange func(string, *types.ApplicationState), onSelect func(string, *types.ApplicationState)) *Autocomplete {
	input := NewTextInput(id, placeholder)
	input.Text = query
	input.CursorIndex = len([]rune(query))
	return &Autocomplete{CompID: id, Placeholder: placeholder, Query: query, Options: options, MaxVisible: 6, Highlighted: -1, OnQueryChange: onQueryChange, OnSelect: onSelect, input: *input}
}
func (a *Autocomplete) ID() string                    { return a.CompID }
func (a *Autocomplete) GetID() string                 { return a.CompID }
func (a *Autocomplete) Bounds() image.Rectangle       { return a.Rect }
func (a *Autocomplete) SetBounds(r image.Rectangle)   { a.Rect = r }
func (a *Autocomplete) Focusable() bool               { return !a.Disabled }
func (a *Autocomplete) Walk(fn func(types.Component)) { fn(a) }
func (a *Autocomplete) overlayID() string             { return a.CompID + ".suggestions" }
func (a *Autocomplete) interactionKey() string        { return a.CompID + "/autocomplete" }
func (a *Autocomplete) highlighted(state *types.ApplicationState) int {
	if state == nil || state.TransientState == nil {
		return a.Highlighted
	}
	if interaction, ok := renderstate.Load[autocompleteInteraction](state.TransientState, a.interactionKey()); ok {
		return interaction.Highlighted
	}
	renderstate.StoreValue(state.TransientState, a.interactionKey(), autocompleteInteraction{Highlighted: a.Highlighted})
	return a.Highlighted
}
func (a *Autocomplete) setHighlighted(index int, state *types.ApplicationState) {
	a.Highlighted = index
	if state == nil {
		return
	}
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, a.interactionKey(), autocompleteInteraction{Highlighted: index})
}
func (a *Autocomplete) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	a.syncInput(nil)
	return a.input.Measure(avail, state)
}
func (a *Autocomplete) syncInput(state *types.ApplicationState) {
	a.input.CompID = a.CompID
	a.input.Rect = a.Rect
	a.input.Placeholder = a.Placeholder
	a.input.Disabled = a.Disabled
	a.input.ReadOnly = a.ReadOnly
	a.input.Invalid = a.Invalid
	if a.input.Text != a.Query && (state == nil || state.FocusedID != a.CompID) {
		a.input.Text = a.Query
		a.input.CursorIndex = len([]rune(a.Query))
		if state != nil && state.TextInputValues != nil {
			state.TextInputValues[a.CompID] = a.Query
		}
	}
	a.input.OnChange = func(next string, inner *types.ApplicationState) {
		if a.OnQueryChange != nil {
			a.OnQueryChange(next, inner)
		} else {
			a.Query = next
		}
	}
}
func (a *Autocomplete) matching(query string) []AutocompleteOption {
	query = strings.TrimSpace(query)
	result := make([]AutocompleteOption, 0, len(a.Options))
	for _, option := range a.Options {
		if query == "" || strings.Contains(strings.ToLower(option.Label), strings.ToLower(query)) || strings.Contains(strings.ToLower(option.Value), strings.ToLower(query)) {
			result = append(result, option)
		}
	}
	return result
}
func (a *Autocomplete) visibleOptions(query string) []AutocompleteOption {
	options := a.matching(query)
	limit := a.MaxVisible
	if limit <= 0 {
		limit = 6
	}
	if len(options) > limit {
		options = options[:limit]
	}
	return options
}
func (a *Autocomplete) setOpen(open bool, state *types.ApplicationState) {
	if !open {
		if !state.CloseOverlay(a.overlayID()) && a.OnOpenChange != nil {
			a.OnOpenChange(false, state)
		}
	}
}
func (a *Autocomplete) openSuggestions(state *types.ApplicationState) bool {
	if a.Disabled || a.ReadOnly {
		return false
	}
	options := a.visibleOptions(a.input.Text)
	if len(options) == 0 {
		a.setOpen(false, state)
		return false
	}
	maxVisible := a.MaxVisible
	if maxVisible <= 0 {
		maxVisible = 6
	}
	if len(options) < maxVisible {
		maxVisible = len(options)
	}
	rowHeight := activeTheme(state).Controls.Medium
	popup := &autocompletePopup{CompID: a.overlayID(), Rect: image.Rect(a.Rect.Min.X, a.Rect.Max.Y+activeTheme(state).Spacing.XS, a.Rect.Max.X, a.Rect.Max.Y+activeTheme(state).Spacing.XS+maxVisible*rowHeight+2), OwnerID: a.CompID, Options: options, Highlighted: a.highlighted(state)}
	popup.OnSelect = func(value string, inner *types.ApplicationState) {
		a.setHighlighted(-1, inner)
		if a.OnSelect != nil {
			a.OnSelect(value, inner)
		}
		a.setOpen(false, inner)
	}
	state.OpenAnchoredOverlayWithDismiss(a.overlayID(), popup, true, func(inner *types.ApplicationState) {
		if a.OnOpenChange != nil {
			a.OnOpenChange(false, inner)
		}
	})
	if a.OnOpenChange != nil {
		a.OnOpenChange(true, state)
	}
	return true
}
func (a *Autocomplete) Draw(p types.Painter, state *types.ApplicationState) {
	a.syncInput(state)
	a.input.Draw(p, state)
	th := activeTheme(state)
	p.DrawText("v", a.Rect.Max.X-th.Spacing.LG, a.Rect.Min.Y+a.Rect.Dy()/2+5, th.Colors.TextMuted)
}
func (a *Autocomplete) HitTest(pt image.Point) string {
	if pt.In(a.Rect) {
		return a.CompID
	}
	return ""
}
func (a *Autocomplete) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	a.syncInput(state)
	if !a.input.OnMouseDown(pt, state) {
		return false
	}
	a.openSuggestions(state)
	return true
}
func (a *Autocomplete) OnMouseUp(image.Point, *types.ApplicationState) bool   { return false }
func (a *Autocomplete) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (a *Autocomplete) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID != a.CompID || a.Disabled || a.ReadOnly {
		return false
	}
	options := a.visibleOptions(a.input.Text)
	highlighted := a.highlighted(state)
	switch key {
	case 0x28: // down
		if len(options) == 0 {
			return false
		}
		highlighted = (highlighted + 1) % len(options)
		a.setHighlighted(highlighted, state)
		a.openSuggestions(state)
		return true
	case 0x26: // up
		if len(options) == 0 {
			return false
		}
		if highlighted <= 0 {
			highlighted = len(options) - 1
		} else {
			highlighted--
		}
		a.setHighlighted(highlighted, state)
		a.openSuggestions(state)
		return true
	case 0x1B: // escape
		a.setOpen(false, state)
		return true
	case 0x0D: // enter
		if highlighted >= 0 && highlighted < len(options) && !options[highlighted].Disabled {
			if a.OnSelect != nil {
				a.OnSelect(options[highlighted].Value, state)
			}
			a.setHighlighted(-1, state)
			a.setOpen(false, state)
			return true
		}
	}
	a.syncInput(state)
	handled := a.input.OnKey(key, char, state)
	if handled {
		a.setHighlighted(-1, state)
		a.openSuggestions(state)
	}
	return handled
}
func (a *Autocomplete) Semantics(state *types.ApplicationState) semantics.Node {
	expanded := false
	if state != nil && state.Overlays != nil {
		for _, overlay := range state.Overlays.Snapshot() {
			if overlay.ID == a.overlayID() {
				expanded = true
				break
			}
		}
	}
	name := a.AccessibleName
	if name == "" {
		name = a.Placeholder
	}
	return semantics.Node{ID: a.CompID, Role: semantics.RoleComboBox, Name: name, Value: a.Query, Bounds: a.Rect, State: semantics.State{Disabled: a.Disabled, ReadOnly: a.ReadOnly, Invalid: a.Invalid, Focused: state != nil && state.FocusedID == a.CompID, Expanded: expanded}, Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionSetValue, semantics.ActionExpand, semantics.ActionCollapse}}
}
func (a *Autocomplete) PerformSemanticAction(targetID string, action semantics.Action, value string, state *types.ApplicationState) bool {
	if targetID != a.CompID || a.Disabled {
		return false
	}
	switch action {
	case semantics.ActionFocus:
		state.FocusedID = a.CompID
		return true
	case semantics.ActionExpand:
		a.syncInput(state)
		return a.openSuggestions(state)
	case semantics.ActionCollapse:
		a.setOpen(false, state)
		return true
	case semantics.ActionSetValue:
		if a.ReadOnly {
			return false
		}
		if a.OnQueryChange != nil {
			a.OnQueryChange(value, state)
		} else {
			a.Query = value
			a.input.Text = value
		}
		return true
	}
	return false
}

type autocompletePopup struct {
	CompID      string
	Rect        image.Rectangle
	OwnerID     string
	Options     []AutocompleteOption
	Highlighted int
	OnSelect    func(string, *types.ApplicationState)
}

func (p *autocompletePopup) ID() string                    { return p.CompID }
func (p *autocompletePopup) GetID() string                 { return p.CompID }
func (p *autocompletePopup) Bounds() image.Rectangle       { return p.Rect }
func (p *autocompletePopup) SetBounds(r image.Rectangle)   { p.Rect = r }
func (p *autocompletePopup) Focusable() bool               { return false }
func (p *autocompletePopup) Walk(fn func(types.Component)) { fn(p) }
func (p *autocompletePopup) rowHeight(state *types.ApplicationState) int {
	return activeTheme(state).Controls.Medium
}
func (p *autocompletePopup) rowRect(index int, state *types.ApplicationState) image.Rectangle {
	h := p.rowHeight(state)
	return image.Rect(p.Rect.Min.X+1, p.Rect.Min.Y+1+index*h, p.Rect.Max.X-1, p.Rect.Min.Y+1+(index+1)*h)
}
func (p *autocompletePopup) Draw(painter types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	painter.SetShadow(float32(th.Elevation.High.OffsetX), float32(th.Elevation.High.OffsetY), float32(th.Elevation.High.Blur))
	painter.DrawRoundedRect(p.Rect, th.Radii.Medium, th.Colors.SurfaceRaised)
	painter.SetShadow(0, 0, 0)
	for i, option := range p.Options {
		r := p.rowRect(i, state)
		id := p.CompID + "/" + option.Value
		visual := buttonVisual(th, 0, state.HoveredID == id, false, option.Disabled, i == p.Highlighted, nil)
		visual.border = colorZero()
		if state.HoveredID == id || i == p.Highlighted {
			drawControlSurface(painter, r, visual, false)
		}
		painter.DrawText(option.Label, r.Min.X+th.Spacing.MD, r.Min.Y+r.Dy()/2+5, visual.foreground)
	}
}
func (p *autocompletePopup) HitTest(pt image.Point) string {
	if !pt.In(p.Rect) {
		return ""
	}
	index := (pt.Y - p.Rect.Min.Y - 1) / p.rowHeight(nil)
	if index >= 0 && index < len(p.Options) {
		return p.CompID + "/" + p.Options[index].Value
	}
	return p.CompID
}
func (p *autocompletePopup) OnKey(uint32, rune, *types.ApplicationState) bool { return false }
func (p *autocompletePopup) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	state.ActiveID = p.HitTest(pt)
	return state.ActiveID != ""
}
func (p *autocompletePopup) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	id := p.HitTest(pt)
	if id == "" || id != state.ActiveID {
		return false
	}
	return p.PerformSemanticAction(id, semantics.ActionSelect, "", state)
}
func (p *autocompletePopup) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (p *autocompletePopup) Semantics(state *types.ApplicationState) semantics.Node {
	node := semantics.Node{ID: p.CompID, Role: semantics.RoleListBox, Name: "Suggestions", Bounds: p.Rect}
	for i, option := range p.Options {
		node.Children = append(node.Children, semantics.Node{ID: p.CompID + "/" + option.Value, Role: semantics.RoleOption, Name: option.Label, Description: option.Description, Value: option.Value, Bounds: p.rowRect(i, state), State: semantics.State{Disabled: option.Disabled, Selected: i == p.Highlighted}, Actions: []semantics.Action{semantics.ActionSelect}})
	}
	return node
}
func (p *autocompletePopup) PerformSemanticAction(targetID string, _ semantics.Action, _ string, state *types.ApplicationState) bool {
	value := strings.TrimPrefix(targetID, p.CompID+"/")
	if value == targetID {
		return false
	}
	for _, option := range p.Options {
		if option.Value == value && !option.Disabled {
			if p.OnSelect != nil {
				p.OnSelect(value, state)
			}
			return true
		}
	}
	return false
}

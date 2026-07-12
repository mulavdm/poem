package components

import (
	renderstate "go_native_gpu_gui/pkg/render/state"
	"go_native_gpu_gui/pkg/render/types"
)

func (t *TextArea) compositionKey() string { return t.CompID + "/textarea-composition" }

func (t *TextArea) composition(state *types.ApplicationState) (textInputComposition, bool) {
	if state == nil || state.TransientState == nil {
		return textInputComposition{}, false
	}
	composition, ok := renderstate.Load[textInputComposition](state.TransientState, t.compositionKey())
	return composition, ok && composition.Active
}

func (t *TextArea) storeComposition(state *types.ApplicationState, composition textInputComposition) {
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, t.compositionKey(), composition)
}

func (t *TextArea) StartComposition(state *types.ApplicationState) bool {
	if state == nil || state.FocusedID != t.CompID || t.Disabled || t.ReadOnly {
		return false
	}
	start, end := t.Selection(state)
	t.storeComposition(state, textInputComposition{Active: true, SelectionStart: start, SelectionEnd: end})
	return true
}

func (t *TextArea) UpdateComposition(text string, state *types.ApplicationState) bool {
	if state == nil || state.FocusedID != t.CompID || t.Disabled || t.ReadOnly {
		return false
	}
	composition, ok := t.composition(state)
	if !ok {
		if !t.StartComposition(state) {
			return false
		}
		composition, _ = t.composition(state)
	}
	if len([]rune(text)) > maxTextInputRunes {
		return false
	}
	composition.Text = text
	t.storeComposition(state, composition)
	return true
}

func (t *TextArea) EndComposition(committedText string, state *types.ApplicationState) bool {
	if state == nil || t.Disabled || t.ReadOnly {
		return false
	}
	composition, ok := t.composition(state)
	if state.TransientState != nil {
		state.TransientState.Delete(t.compositionKey())
	}
	if committedText == "" {
		return ok
	}
	if !ok {
		start, end := t.Selection(state)
		composition = textInputComposition{SelectionStart: start, SelectionEnd: end}
	}
	runes := []rune(t.Value)
	selection := textInputSelection{
		Anchor: clampTextIndex(composition.SelectionStart, len(runes)),
		Caret:  clampTextIndex(composition.SelectionEnd, len(runes)),
	}
	replacement := []rune(committedText)
	start, end := selectionBounds(selection)
	if len(runes)-(end-start)+len(replacement) > maxTextInputRunes {
		return false
	}
	before := t.Value
	next, caret := replaceSelectedRunes(runes, selection, replacement)
	t.syncEditedText(state, next, caret)
	if t.Value != before && t.OnChange != nil {
		t.OnChange(t.Value, state)
	}
	return true
}

func (t *TextArea) compositionDisplay(state *types.ApplicationState) (text string, start, end int, active bool) {
	composition, ok := t.composition(state)
	if !ok {
		return t.Value, 0, 0, false
	}
	runes := []rune(t.Value)
	start = clampTextIndex(composition.SelectionStart, len(runes))
	selectionEnd := clampTextIndex(composition.SelectionEnd, len(runes))
	if start > selectionEnd {
		start, selectionEnd = selectionEnd, start
	}
	preview := []rune(composition.Text)
	result := make([]rune, 0, len(runes)-(selectionEnd-start)+len(preview))
	result = append(result, runes[:start]...)
	result = append(result, preview...)
	result = append(result, runes[selectionEnd:]...)
	return string(result), start, start + len(preview), true
}

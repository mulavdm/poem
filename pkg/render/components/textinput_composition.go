package components

import (
	renderstate "go_native_gpu_gui/pkg/render/state"
	"go_native_gpu_gui/pkg/render/types"
)

type textInputComposition struct {
	Active         bool
	Text           string
	SelectionStart int
	SelectionEnd   int
}

func (t *TextInput) compositionKey() string { return t.CompID + "/text-composition" }

func (t *TextInput) composition(state *types.ApplicationState) (textInputComposition, bool) {
	if state == nil || state.TransientState == nil {
		return textInputComposition{}, false
	}
	composition, ok := renderstate.Load[textInputComposition](state.TransientState, t.compositionKey())
	return composition, ok && composition.Active
}

func (t *TextInput) storeComposition(state *types.ApplicationState, composition textInputComposition) {
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, t.compositionKey(), composition)
}

func (t *TextInput) StartComposition(state *types.ApplicationState) bool {
	if state == nil || state.FocusedID != t.CompID || t.Disabled || t.ReadOnly {
		return false
	}
	start, end := t.Selection(state)
	t.storeComposition(state, textInputComposition{Active: true, SelectionStart: start, SelectionEnd: end})
	return true
}

func (t *TextInput) UpdateComposition(text string, state *types.ApplicationState) bool {
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

func (t *TextInput) EndComposition(committedText string, state *types.ApplicationState) bool {
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
	runes := []rune(t.Text)
	selection := textInputSelection{Anchor: clampTextIndex(composition.SelectionStart, len(runes)), Caret: clampTextIndex(composition.SelectionEnd, len(runes))}
	replacement := []rune(committedText)
	start, end := selectionBounds(selection)
	if len(runes)-(end-start)+len(replacement) > maxTextInputRunes {
		return false
	}
	before := t.Text
	next, caret := replaceSelectedRunes(runes, selection, replacement)
	t.syncEditedText(state, next, caret)
	if t.Text != before && t.OnChange != nil {
		t.OnChange(t.Text, state)
	}
	return true
}

func (t *TextInput) compositionDisplay(state *types.ApplicationState) (text string, startWidth, endWidth int, active bool) {
	composition, ok := t.composition(state)
	if !ok || t.Masked {
		return t.Text, 0, 0, false
	}
	runes := []rune(t.Text)
	start := clampTextIndex(composition.SelectionStart, len(runes))
	end := clampTextIndex(composition.SelectionEnd, len(runes))
	if start > end {
		start, end = end, start
	}
	prefix := string(runes[:start])
	text = prefix + composition.Text + string(runes[end:])
	startWidth = inputTextWidth(state, prefix)
	endWidth = startWidth + inputTextWidth(state, composition.Text)
	return text, startWidth, endWidth, true
}

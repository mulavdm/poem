package components

import (
	"context"
	"strings"
	"time"

	renderstate "go_native_gpu_gui/pkg/render/state"
	"go_native_gpu_gui/pkg/render/types"
)

func (t *TextArea) selectionKey() string { return t.CompID + "/textarea-selection" }

func (t *TextArea) editingSelection(state *types.ApplicationState, length int) textInputSelection {
	if state != nil && state.TransientState != nil {
		if selection, ok := renderstate.Load[textInputSelection](state.TransientState, t.selectionKey()); ok {
			selection.Anchor = clampTextIndex(selection.Anchor, length)
			selection.Caret = clampTextIndex(selection.Caret, length)
			return selection
		}
	}
	cursor := clampTextIndex(t.CursorIndex, length)
	return textInputSelection{Anchor: cursor, Caret: cursor}
}

func (t *TextArea) setEditingSelection(state *types.ApplicationState, selection textInputSelection, length int) {
	selection.Anchor = clampTextIndex(selection.Anchor, length)
	selection.Caret = clampTextIndex(selection.Caret, length)
	t.CursorIndex = selection.Caret
	if state == nil {
		return
	}
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, t.selectionKey(), selection)
	if state.TextInputValues != nil {
		state.TextInputValues[t.CompID+"_cursor"] = intString(t.CursorIndex)
	}
}

func (t *TextArea) Selection(state *types.ApplicationState) (start, end int) {
	return selectionBounds(t.editingSelection(state, len([]rune(t.Text))))
}

func (t *TextArea) SetSelection(anchor, caret int, state *types.ApplicationState) {
	t.setEditingSelection(state, textInputSelection{Anchor: anchor, Caret: caret}, len([]rune(t.Text)))
}

func (t *TextArea) syncEditedText(state *types.ApplicationState, runes []rune, caret int) {
	t.Text = string(runes)
	t.setEditingSelection(state, textInputSelection{Anchor: caret, Caret: caret}, len(runes))
	if state != nil && state.TextInputValues != nil {
		state.TextInputValues[t.CompID] = t.Text
	}
}

func (t *TextArea) copySelection(state *types.ApplicationState, runes []rune, selection textInputSelection) bool {
	if state == nil || state.Services.Clipboard == nil {
		return false
	}
	start, end := selectionBounds(selection)
	if start == end {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	return state.Services.Clipboard.WriteText(ctx, string(runes[start:end])) == nil
}

func normalizeMultilineClipboardText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.ReplaceAll(value, "\x00", "")
}

func (t *TextArea) pasteSelection(state *types.ApplicationState, runes []rune, selection textInputSelection) bool {
	if t.ReadOnly || state == nil || state.Services.Clipboard == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	value, err := state.Services.Clipboard.ReadText(ctx)
	if err != nil {
		return false
	}
	replacement := []rune(normalizeMultilineClipboardText(value))
	start, end := selectionBounds(selection)
	if len(runes)-(end-start)+len(replacement) > maxTextInputRunes {
		return false
	}
	next, caret := replaceSelectedRunes(runes, selection, replacement)
	t.syncEditedText(state, next, caret)
	return true
}

func (t *TextArea) handleEditingKey(key uint32, char rune, state *types.ApplicationState) bool {
	const (
		vkBack    = 0x08
		vkReturn  = 0x0D
		vkControl = 0x11
		vkShift   = 0x10
		vkEnd     = 0x23
		vkHome    = 0x24
		vkLeft    = 0x25
		vkRight   = 0x27
		vkDelete  = 0x2E
	)
	runes := []rune(t.Text)
	selection := t.editingSelection(state, len(runes))
	control := modifierPressed(state, vkControl)
	shift := modifierPressed(state, vkShift)

	if control {
		switch key {
		case 'A':
			t.setEditingSelection(state, textInputSelection{Anchor: 0, Caret: len(runes)}, len(runes))
			return true
		case 'C':
			return t.copySelection(state, runes, selection)
		case 'X':
			if t.ReadOnly || !t.copySelection(state, runes, selection) {
				return false
			}
			start, end := selectionBounds(selection)
			if start != end {
				next, caret := replaceSelectedRunes(runes, selection, nil)
				t.syncEditedText(state, next, caret)
			}
			return true
		case 'V':
			return t.pasteSelection(state, runes, selection)
		}
	}

	if key == vkLeft || key == vkRight || key == vkHome || key == vkEnd {
		start, end := selectionBounds(selection)
		next := selection.Caret
		switch key {
		case vkLeft:
			if !shift && start != end {
				next = start
			} else if control {
				next = wordLeft(runes, next)
			} else if next > 0 {
				next--
			}
		case vkRight:
			if !shift && start != end {
				next = end
			} else if control {
				next = wordRight(runes, next)
			} else if next < len(runes) {
				next++
			}
		case vkHome:
			next = 0
		case vkEnd:
			next = len(runes)
		}
		anchor := next
		if shift {
			anchor = selection.Anchor
		}
		t.setEditingSelection(state, textInputSelection{Anchor: anchor, Caret: next}, len(runes))
		return true
	}

	if t.ReadOnly {
		return false
	}
	if key == vkBack || key == vkDelete {
		start, end := selectionBounds(selection)
		if start == end {
			if key == vkBack && selection.Caret > 0 {
				selection.Anchor = selection.Caret - 1
			} else if key == vkDelete && selection.Caret < len(runes) {
				selection.Anchor = selection.Caret + 1
			} else {
				return true
			}
		}
		next, caret := replaceSelectedRunes(runes, selection, nil)
		t.syncEditedText(state, next, caret)
		return true
	}
	if key == vkReturn {
		next, caret := replaceSelectedRunes(runes, selection, []rune{'\n'})
		if len(next) > maxTextInputRunes {
			return false
		}
		t.syncEditedText(state, next, caret)
		return true
	}
	if char >= 32 && char != 127 {
		start, end := selectionBounds(selection)
		if len(runes)-(end-start)+1 > maxTextInputRunes {
			return false
		}
		next, caret := replaceSelectedRunes(runes, selection, []rune{char})
		t.syncEditedText(state, next, caret)
		return true
	}
	return false
}

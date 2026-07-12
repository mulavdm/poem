package components

import (
	"context"
	"image"
	"strings"
	"time"
	"unicode"

	"github.com/mulavdm/poem/pkg/render/draw"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

const maxTextInputRunes = 1 << 20

func inputTextWidth(state *types.ApplicationState, text string) int {
	if state != nil && state.Services.TextShaper != nil {
		fontSize := activeTheme(state).Typography.Body.Size
		if measured, err := state.Services.TextShaper.Measure(context.Background(), draw.TextRun{Text: text, Font: draw.Font{Family: activeTheme(state).Typography.BodyFamily, Size: fontSize}}, image.Point{}); err == nil {
			return measured.X
		}
	}
	width := 8
	if state != nil && state.FontCharWidth > 0 {
		width = state.FontCharWidth
	}
	return len([]rune(text)) * width
}

func inputTextIndexAtX(state *types.ApplicationState, text string, target int) int {
	if target <= 0 {
		return 0
	}
	if state != nil && state.Services.TextShaper != nil {
		fontSize := activeTheme(state).Typography.Body.Size
		if shaped, err := state.Services.TextShaper.Shape(context.Background(), draw.TextRun{Text: text, Font: draw.Font{Family: activeTheme(state).Typography.BodyFamily, Size: fontSize}}); err == nil {
			for index, glyph := range shaped.Glyphs {
				if target < glyph.Offset.X+int(glyph.Advance/2) {
					return index
				}
			}
			return len(shaped.Glyphs)
		}
	}
	charWidth := 8
	if state != nil && state.FontCharWidth > 0 {
		charWidth = state.FontCharWidth
	}
	return clampTextIndex((target+charWidth/2)/charWidth, len([]rune(text)))
}

type textInputSelection struct {
	Anchor int
	Caret  int
}

func (t *TextInput) selectionKey() string { return t.CompID + "/text-selection" }

func clampTextIndex(index, length int) int {
	if index < 0 {
		return 0
	}
	if index > length {
		return length
	}
	return index
}

func (t *TextInput) editingSelection(state *types.ApplicationState, length int) textInputSelection {
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

func (t *TextInput) setEditingSelection(state *types.ApplicationState, selection textInputSelection, length int) {
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

// Selection returns the normalized rune-index selection range.
func (t *TextInput) Selection(state *types.ApplicationState) (start, end int) {
	return selectionBounds(t.editingSelection(state, len([]rune(t.Value))))
}

// SetSelection replaces the transient selection using rune indexes. Values are
// clamped to the current text length.
func (t *TextInput) SetSelection(anchor, caret int, state *types.ApplicationState) {
	t.setEditingSelection(state, textInputSelection{Anchor: anchor, Caret: caret}, len([]rune(t.Value)))
}

func selectionBounds(selection textInputSelection) (int, int) {
	if selection.Anchor < selection.Caret {
		return selection.Anchor, selection.Caret
	}
	return selection.Caret, selection.Anchor
}

func (t *TextInput) syncEditedText(state *types.ApplicationState, runes []rune, caret int) {
	t.Value = string(runes)
	t.setEditingSelection(state, textInputSelection{Anchor: caret, Caret: caret}, len(runes))
	if state != nil && state.TextInputValues != nil {
		state.TextInputValues[t.CompID] = t.Value
	}
}

func replaceSelectedRunes(runes []rune, selection textInputSelection, replacement []rune) ([]rune, int) {
	start, end := selectionBounds(selection)
	result := make([]rune, 0, len(runes)-(end-start)+len(replacement))
	result = append(result, runes[:start]...)
	result = append(result, replacement...)
	result = append(result, runes[end:]...)
	return result, start + len(replacement)
}

func modifierPressed(state *types.ApplicationState, key uint32) bool {
	return state != nil && state.KeysPressed != nil && state.KeysPressed[key]
}

func wordLeft(runes []rune, index int) int {
	index = clampTextIndex(index, len(runes))
	for index > 0 && unicode.IsSpace(runes[index-1]) {
		index--
	}
	for index > 0 && !unicode.IsSpace(runes[index-1]) {
		index--
	}
	return index
}

func wordRight(runes []rune, index int) int {
	index = clampTextIndex(index, len(runes))
	for index < len(runes) && !unicode.IsSpace(runes[index]) {
		index++
	}
	for index < len(runes) && unicode.IsSpace(runes[index]) {
		index++
	}
	return index
}

func singleLineClipboardText(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', 0:
			return ' '
		default:
			return r
		}
	}, value)
}

func (t *TextInput) copySelection(state *types.ApplicationState, runes []rune, selection textInputSelection) bool {
	if t.Masked || state == nil || state.Services.Clipboard == nil {
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

func (t *TextInput) pasteSelection(state *types.ApplicationState, runes []rune, selection textInputSelection) bool {
	if t.ReadOnly || state == nil || state.Services.Clipboard == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	value, err := state.Services.Clipboard.ReadText(ctx)
	if err != nil {
		return false
	}
	replacement := []rune(singleLineClipboardText(value))
	start, end := selectionBounds(selection)
	if len(runes)-(end-start)+len(replacement) > maxTextInputRunes {
		return false
	}
	next, caret := replaceSelectedRunes(runes, selection, replacement)
	t.syncEditedText(state, next, caret)
	return true
}

func (t *TextInput) handleEditingKey(key uint32, char rune, state *types.ApplicationState) bool {
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
	runes := []rune(t.Value)
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
			if t.ReadOnly || t.Masked || !t.copySelection(state, runes, selection) {
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

	if key == vkReturn {
		if t.OnSubmit != nil {
			t.OnSubmit(t.Value, state)
			return true
		}
		return false
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
		if start != end {
			next, caret := replaceSelectedRunes(runes, selection, nil)
			t.syncEditedText(state, next, caret)
			return true
		}
		if key == vkBack && selection.Caret > 0 {
			selection.Anchor = selection.Caret - 1
		} else if key == vkDelete && selection.Caret < len(runes) {
			selection.Anchor = selection.Caret + 1
		} else {
			return true
		}
		next, caret := replaceSelectedRunes(runes, selection, nil)
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

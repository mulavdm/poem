package components

import (
	"context"
	"image"
	"strings"
	"testing"

	"github.com/mulavdm/poem/pkg/render/platform"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

type memoryClipboard struct {
	text     string
	readErr  error
	writeErr error
}

func (c *memoryClipboard) ReadText(context.Context) (string, error) { return c.text, c.readErr }
func (c *memoryClipboard) WriteText(_ context.Context, text string) error {
	if c.writeErr != nil {
		return c.writeErr
	}
	c.text = text
	return nil
}

func editingTestState(clipboard platform.Clipboard) *types.ApplicationState {
	return &types.ApplicationState{
		FocusedID:       "input",
		KeysPressed:     make(map[uint32]bool),
		TextInputValues: make(map[string]string),
		TransientState:  renderstate.NewStore(),
		Services:        platform.Services{Clipboard: clipboard},
	}
}

func TestTextInputUnicodeSelectionCutAndPaste(t *testing.T) {
	clipboard := &memoryClipboard{}
	state := editingTestState(clipboard)
	input := NewTextInput("input", "")
	input.Value = "Go 日本語"
	input.CursorIndex = len([]rune(input.Value))
	input.setEditingSelection(state, textInputSelection{Anchor: 3, Caret: 6}, len([]rune(input.Value)))
	state.KeysPressed[0x11] = true
	if !input.OnKey('C', 0, state) || clipboard.text != "日本語" {
		t.Fatalf("copied %q", clipboard.text)
	}
	if !input.OnKey('X', 0, state) || input.Value != "Go " {
		t.Fatalf("cut text %q", input.Value)
	}
	clipboard.text = "Rust\r\n言語"
	if !input.OnKey('V', 0, state) || input.Value != "Go Rust  言語" {
		t.Fatalf("pasted text %q", input.Value)
	}
}

func TestTextInputSelectAllReplacement(t *testing.T) {
	state := editingTestState(&memoryClipboard{})
	input := NewTextInput("input", "")
	input.Value = "replace me"
	input.CursorIndex = len([]rune(input.Value))
	state.KeysPressed[0x11] = true
	if !input.OnKey('A', 0, state) {
		t.Fatal("select-all was not handled")
	}
	state.KeysPressed[0x11] = false
	if !input.OnKey(0, '✓', state) || input.Value != "✓" {
		t.Fatalf("replacement text %q", input.Value)
	}
}

func TestTextInputShiftSelectionSurvivesRebuild(t *testing.T) {
	state := editingTestState(&memoryClipboard{})
	state.KeysPressed[0x10] = true
	first := NewTextInput("input", "")
	first.Value, first.CursorIndex = "abcd", 4
	if !first.OnKey(0x25, 0, state) {
		t.Fatal("shift-left was not handled")
	}
	rebuilt := NewTextInput("input", "")
	rebuilt.Value = "abcd"
	if !rebuilt.OnKey(0x25, 0, state) {
		t.Fatal("rebuilt shift-left was not handled")
	}
	selection := rebuilt.editingSelection(state, 4)
	start, end := selectionBounds(selection)
	if start != 2 || end != 4 {
		t.Fatalf("retained selection = %d:%d (%+v)", start, end, selection)
	}
}

func TestTextInputReadOnlyAllowsCopyButNotMutation(t *testing.T) {
	clipboard := &memoryClipboard{}
	state := editingTestState(clipboard)
	state.KeysPressed[0x11] = true
	input := NewTextInput("input", "")
	input.Value, input.ReadOnly = "locked", true
	input.setEditingSelection(state, textInputSelection{Anchor: 0, Caret: 6}, 6)
	if !input.OnKey('C', 0, state) || clipboard.text != "locked" {
		t.Fatalf("read-only copy = %q", clipboard.text)
	}
	clipboard.text = "changed"
	if input.OnKey('V', 0, state) || input.Value != "locked" {
		t.Fatalf("read-only input mutated to %q", input.Value)
	}
}

func TestTextInputMaskedValueCannotBeCopied(t *testing.T) {
	clipboard := &memoryClipboard{}
	state := editingTestState(clipboard)
	state.KeysPressed[0x11] = true
	input := NewTextInput("input", "")
	input.Value, input.Masked = "secret", true
	input.setEditingSelection(state, textInputSelection{Anchor: 0, Caret: 6}, 6)
	if input.OnKey('C', 0, state) || clipboard.text != "" {
		t.Fatal("masked input exposed clipboard text")
	}
}

func TestTextInputMaskedSemanticsDoNotExposeSecret(t *testing.T) {
	state := editingTestState(&memoryClipboard{})
	input := NewTextInput("input", "Password")
	input.Value, input.Masked = "secret", true
	node := input.Semantics(state)
	if node.Value != "" || !node.State.Password || node.Text != nil {
		t.Fatalf("masked semantic node exposed data: %+v", node)
	}
}

func TestTextInputRejectsOversizedPaste(t *testing.T) {
	clipboard := &memoryClipboard{text: strings.Repeat("x", maxTextInputRunes+1)}
	state := editingTestState(clipboard)
	state.KeysPressed[0x11] = true
	input := NewTextInput("input", "")
	if input.OnKey('V', 0, state) || input.Value != "" {
		t.Fatal("oversized paste was accepted")
	}
}

func TestTextInputMouseDragSelectsRunes(t *testing.T) {
	state := editingTestState(&memoryClipboard{})
	state.FontCharWidth = 8
	input := NewTextInput("input", "")
	input.Value = "abcdef"
	input.Rect.Min.X, input.Rect.Min.Y, input.Rect.Max.X, input.Rect.Max.Y = 0, 0, 200, 36
	if !input.OnMouseDown(image.Pt(18, 10), state) || !input.OnMouseMove(image.Pt(42, 10), state) {
		t.Fatal("mouse selection was not handled")
	}
	start, end := selectionBounds(input.editingSelection(state, 6))
	if start != 1 || end != 4 {
		t.Fatalf("mouse selection = %d:%d", start, end)
	}
}

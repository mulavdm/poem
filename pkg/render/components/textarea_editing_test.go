package components

import (
	"strings"
	"testing"
)

func TestTextAreaUnicodeSelectionCutAndMultilinePaste(t *testing.T) {
	clipboard := &memoryClipboard{}
	state := editingTestState(clipboard)
	area := NewTextArea("input", "")
	area.Value = "Go 日本語\nsecond"
	area.CursorIndex = len([]rune(area.Value))
	area.SetSelection(3, 6, state)
	state.KeysPressed[0x11] = true
	if !area.OnKey('C', 0, state) || clipboard.text != "日本語" {
		t.Fatalf("copied %q", clipboard.text)
	}
	if !area.OnKey('X', 0, state) || area.Value != "Go \nsecond" {
		t.Fatalf("cut text %q", area.Value)
	}
	clipboard.text = "first\r\n世界"
	if !area.OnKey('V', 0, state) || area.Value != "Go first\n世界\nsecond" {
		t.Fatalf("pasted text %q", area.Value)
	}
}

func TestTextAreaSelectionReplacementAndReadOnlyCopy(t *testing.T) {
	clipboard := &memoryClipboard{}
	state := editingTestState(clipboard)
	area := NewTextArea("input", "")
	area.Value = "replace me"
	area.SetSelection(0, len([]rune(area.Value)), state)
	if !area.OnKey(0, '✓', state) || area.Value != "✓" {
		t.Fatalf("replacement text %q", area.Value)
	}
	area.Value, area.ReadOnly = "locked", true
	area.SetSelection(0, 6, state)
	state.KeysPressed[0x11] = true
	if !area.OnKey('C', 0, state) || clipboard.text != "locked" {
		t.Fatalf("read-only copy %q", clipboard.text)
	}
	clipboard.text = "mutated"
	if area.OnKey('V', 0, state) || area.Value != "locked" {
		t.Fatalf("read-only text mutated to %q", area.Value)
	}
}

func TestTextAreaRejectsOversizedPaste(t *testing.T) {
	clipboard := &memoryClipboard{text: strings.Repeat("x", maxTextInputRunes+1)}
	state := editingTestState(clipboard)
	state.KeysPressed[0x11] = true
	area := NewTextArea("input", "")
	if area.OnKey('V', 0, state) || area.Value != "" {
		t.Fatal("oversized paste was accepted")
	}
}

func TestTextAreaCompositionPreviewCommitAndCancel(t *testing.T) {
	state := compositionTestState()
	area := NewTextArea("input", "")
	area.Value = "one two"
	area.SetSelection(4, 7, state)
	if !area.StartComposition(state) || !area.UpdateComposition("日本", state) {
		t.Fatal("composition did not start")
	}
	if display, start, end, active := area.compositionDisplay(state); !active || display != "one 日本" || start != 4 || end != 6 {
		t.Fatalf("preview=%q range=%d:%d active=%v", display, start, end, active)
	}
	if !area.EndComposition("日本", state) || area.Value != "one 日本" {
		t.Fatalf("committed %q", area.Value)
	}
	area.SetSelection(6, 6, state)
	if !area.StartComposition(state) || !area.UpdateComposition("語", state) || !area.EndComposition("", state) || area.Value != "one 日本" {
		t.Fatalf("cancel mutated %q", area.Value)
	}
}

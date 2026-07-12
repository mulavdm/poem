package components

import (
	"strings"
	"testing"

	renderstate "go_native_gpu_gui/pkg/render/state"
	"go_native_gpu_gui/pkg/render/types"
)

func compositionTestState() *types.ApplicationState {
	return &types.ApplicationState{FocusedID: "input", TextInputValues: make(map[string]string), TransientState: renderstate.NewStore()}
}

func TestTextInputCompositionPreviewAndCommit(t *testing.T) {
	state := compositionTestState()
	changed := ""
	input := NewTextInput("input", "")
	input.Value = "abcd"
	input.OnChange = func(value string, _ *types.ApplicationState) { changed = value }
	input.SetSelection(1, 3, state)
	if !input.StartComposition(state) || !input.UpdateComposition("日本", state) {
		t.Fatal("composition did not start")
	}
	display, start, end, active := input.compositionDisplay(state)
	if !active || display != "a日本d" || end <= start {
		t.Fatalf("preview=%q range=%d:%d active=%v", display, start, end, active)
	}
	if !input.EndComposition("日本", state) || input.Value != "a日本d" || changed != "a日本d" {
		t.Fatalf("committed=%q callback=%q", input.Value, changed)
	}
	if _, active := input.composition(state); active {
		t.Fatal("composition state remained after commit")
	}
}

func TestTextInputCompositionSurvivesRebuildAndCanCancel(t *testing.T) {
	state := compositionTestState()
	first := NewTextInput("input", "")
	first.Value = "hello"
	first.SetSelection(5, 5, state)
	if !first.StartComposition(state) || !first.UpdateComposition("世界", state) {
		t.Fatal("composition did not start")
	}
	rebuilt := NewTextInput("input", "")
	rebuilt.Value = "hello"
	if display, _, _, active := rebuilt.compositionDisplay(state); !active || display != "hello世界" {
		t.Fatalf("rebuilt preview=%q active=%v", display, active)
	}
	if !rebuilt.EndComposition("", state) || rebuilt.Value != "hello" {
		t.Fatalf("cancel mutated text to %q", rebuilt.Value)
	}
}

func TestTextInputCompositionRejectsReadOnlyAndOversizedText(t *testing.T) {
	state := compositionTestState()
	input := NewTextInput("input", "")
	input.ReadOnly = true
	if input.StartComposition(state) {
		t.Fatal("read-only input accepted composition")
	}
	input.ReadOnly = false
	if !input.StartComposition(state) || input.UpdateComposition(strings.Repeat("x", maxTextInputRunes+1), state) {
		t.Fatal("oversized composition was accepted")
	}
}

package components

import (
	"image"
	"testing"

	"go_native_gpu_gui/pkg/render/semantics"
	renderstate "go_native_gpu_gui/pkg/render/state"
	"go_native_gpu_gui/pkg/render/types"
)

func autocompleteTestState() *types.ApplicationState {
	return &types.ApplicationState{TextInputValues: make(map[string]string), Overlays: types.NewOverlayManager(), TransientState: renderstate.NewStore()}
}

func TestAutocompleteFiltersAndPublishesQuery(t *testing.T) {
	state := autocompleteTestState()
	state.FocusedID = "language"
	query := ""
	control := NewAutocomplete("language", "Language", "", []AutocompleteOption{{Value: "go", Label: "Go"}, {Value: "rust", Label: "Rust"}}, func(next string, _ *types.ApplicationState) { query = next }, nil)
	control.Rect = image.Rect(0, 0, 240, 36)
	if !control.OnKey(0, 'r', state) || query != "r" {
		t.Fatalf("query callback = %q", query)
	}
	overlays := state.Overlays.Snapshot()
	if len(overlays) != 1 {
		t.Fatalf("expected suggestion overlay, got %+v", overlays)
	}
	popup, ok := overlays[0].Component.(*autocompletePopup)
	if !ok || len(popup.Options) != 1 || popup.Options[0].Value != "rust" {
		t.Fatalf("unexpected filtered popup: %#v", overlays[0].Component)
	}
}

func TestAutocompleteKeyboardSelectsHighlightedOption(t *testing.T) {
	state := autocompleteTestState()
	state.FocusedID = "language"
	selected := ""
	control := NewAutocomplete("language", "Language", "", []AutocompleteOption{{Value: "go", Label: "Go"}, {Value: "rust", Label: "Rust"}}, nil, func(value string, _ *types.ApplicationState) { selected = value })
	control.Rect = image.Rect(0, 0, 240, 36)
	if !control.OnKey(0x28, 0, state) || !control.OnKey(0x0D, 0, state) {
		t.Fatal("keyboard interaction was not handled")
	}
	if selected != "go" {
		t.Fatalf("selected option = %q", selected)
	}
	if len(state.Overlays.Snapshot()) != 0 {
		t.Fatal("suggestion overlay remained open after selection")
	}
}

func TestAutocompletePopupSemanticSelection(t *testing.T) {
	state := autocompleteTestState()
	selected := ""
	popup := &autocompletePopup{CompID: "language.suggestions", Options: []AutocompleteOption{{Value: "go", Label: "Go"}}, OnSelect: func(value string, _ *types.ApplicationState) { selected = value }}
	if !popup.PerformSemanticAction("language.suggestions/go", semantics.ActionSelect, "", state) || selected != "go" {
		t.Fatalf("semantic selection = %q", selected)
	}
}

func TestAutocompleteSemanticValueAndExpansion(t *testing.T) {
	state := autocompleteTestState()
	query := ""
	control := NewAutocomplete("language", "Language", "", []AutocompleteOption{{Value: "go", Label: "Go"}}, func(next string, _ *types.ApplicationState) { query = next }, nil)
	control.Rect = image.Rect(0, 0, 240, 36)
	if !control.PerformSemanticAction("language", semantics.ActionSetValue, "g", state) || query != "g" {
		t.Fatalf("semantic query = %q", query)
	}
	if !control.PerformSemanticAction("language", semantics.ActionExpand, "", state) || len(state.Overlays.Snapshot()) != 1 {
		t.Fatal("semantic expand did not open suggestions")
	}
	if !control.PerformSemanticAction("language", semantics.ActionCollapse, "", state) || len(state.Overlays.Snapshot()) != 0 {
		t.Fatal("semantic collapse did not close suggestions")
	}
}

func TestAutocompleteHighlightSurvivesDeclarativeRebuild(t *testing.T) {
	state := autocompleteTestState()
	state.FocusedID = "language"
	options := []AutocompleteOption{{Value: "go", Label: "Go"}, {Value: "rust", Label: "Rust"}}
	first := NewAutocomplete("language", "Language", "", options, nil, nil)
	first.Rect = image.Rect(0, 0, 240, 36)
	if !first.OnKey(0x28, 0, state) {
		t.Fatal("first down key was not handled")
	}
	selected := ""
	rebuilt := NewAutocomplete("language", "Language", "", options, nil, func(value string, _ *types.ApplicationState) { selected = value })
	rebuilt.Rect = first.Rect
	if !rebuilt.OnKey(0x28, 0, state) || !rebuilt.OnKey(0x0D, 0, state) {
		t.Fatal("rebuilt control lost keyboard interaction")
	}
	if selected != "rust" {
		t.Fatalf("retained highlight selected %q", selected)
	}
}

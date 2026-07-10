package components

import (
	"testing"

	"go_native_gpu_gui/pkg/render/types"
)

func TestButtonMnemonicFocusesInvokesAndPublishesAccessKey(t *testing.T) {
	state := &types.ApplicationState{}
	calls := 0
	button := NewButton("publish", "Publish", func(*types.ApplicationState) { calls++ })
	button.Mnemonic = 'P'
	if !button.ActivateMnemonic(state) || state.FocusedID != "publish" || calls != 1 {
		t.Fatalf("focus=%q calls=%d", state.FocusedID, calls)
	}
	if node := button.Semantics(state); node.AccessKey != "Alt+P" {
		t.Fatalf("access key=%q", node.AccessKey)
	}
	button.Disabled = true
	if button.ActivateMnemonic(state) || calls != 1 {
		t.Fatal("disabled mnemonic activated")
	}
}

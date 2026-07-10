package types

import "testing"

func TestApplicationStateNormalizesAndDispatchesShortcuts(t *testing.T) {
	state := &ApplicationState{}
	calls := 0
	if err := state.RegisterShortcut("shift+ctrl+s", func(*ApplicationState) { calls++ }); err != nil {
		t.Fatal(err)
	}
	dispatched := state.DispatchShortcut("Control+Shift+S")
	if !dispatched || calls != 1 {
		t.Fatalf("dispatched=%v calls=%d", dispatched, calls)
	}
	if state.DispatchShortcut("Ctrl+S") {
		t.Fatal("unregistered shortcut dispatched")
	}
}

func TestApplicationStateRejectsInvalidShortcut(t *testing.T) {
	state := &ApplicationState{}
	if err := state.RegisterShortcut("Ctrl", func(*ApplicationState) {}); err == nil || len(state.Hotkeys) != 0 {
		t.Fatalf("invalid shortcut error=%v hotkeys=%v", err, state.Hotkeys)
	}
}

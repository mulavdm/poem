package types

import (
	"image"
	"testing"
)

type overlayTestComponent struct {
	id        string
	focusable bool
}

func (c *overlayTestComponent) ID() string                                      { return c.id }
func (c *overlayTestComponent) GetID() string                                   { return c.id }
func (c *overlayTestComponent) Bounds() image.Rectangle                         { return image.Rect(0, 0, 10, 10) }
func (c *overlayTestComponent) SetBounds(image.Rectangle)                       {}
func (c *overlayTestComponent) Draw(Painter, *ApplicationState)                 {}
func (c *overlayTestComponent) HitTest(image.Point) string                      { return c.id }
func (c *overlayTestComponent) OnKey(uint32, rune, *ApplicationState) bool      { return false }
func (c *overlayTestComponent) OnMouseDown(image.Point, *ApplicationState) bool { return false }
func (c *overlayTestComponent) OnMouseUp(image.Point, *ApplicationState) bool   { return false }
func (c *overlayTestComponent) OnMouseMove(image.Point, *ApplicationState) bool { return false }
func (c *overlayTestComponent) Focusable() bool                                 { return c.focusable }
func (c *overlayTestComponent) Walk(fn func(Component))                         { fn(c) }

func TestOverlayManagerReplacesStableIDAndRestoresTopOrder(t *testing.T) {
	mgr := NewOverlayManager()
	first := &overlayTestComponent{id: "first"}
	replacement := &overlayTestComponent{id: "replacement"}
	mgr.Open(OverlayEntry{ID: "menu", Component: first})
	mgr.Open(OverlayEntry{ID: "menu", Component: replacement, DismissOnEscape: true})
	got := mgr.Snapshot()
	if len(got) != 1 || got[0].Component.ID() != "replacement" {
		t.Fatalf("overlay replacement failed: %#v", got)
	}
	if dismissed, ok := mgr.DismissTop(); !ok || dismissed.ID != "menu" {
		t.Fatal("top overlay was not dismissed")
	}
}

func TestFocusedOverlayCapturesAndRestoresFocus(t *testing.T) {
	state := &ApplicationState{FocusedID: "launcher", Overlays: NewOverlayManager()}
	menu := &overlayTestComponent{id: "menu", focusable: true}
	state.OpenFocusedOverlay("menu-overlay", menu, true)
	if state.FocusedID != "menu" {
		t.Fatalf("focused overlay target=%q", state.FocusedID)
	}
	entries := state.Overlays.Snapshot()
	if len(entries) != 1 || entries[0].Modal || !entries[0].DismissOnFocusLoss || entries[0].RestoreFocusID != "launcher" {
		t.Fatalf("focused overlay entry=%+v", entries)
	}
	if !state.CloseOverlay("menu-overlay") || state.FocusedID != "launcher" {
		t.Fatalf("restored focus=%q", state.FocusedID)
	}
}

func TestCycleFocusDismissesAnchoredOverlayButKeepsInformation(t *testing.T) {
	first := &overlayTestComponent{id: "first", focusable: true}
	second := &overlayTestComponent{id: "second", focusable: true}
	popup := &overlayTestComponent{id: "popup"}
	toast := &overlayTestComponent{id: "toast"}
	state := &ApplicationState{
		FocusedID:   "first",
		CurrentPage: "page",
		Pages:       map[string][]Component{"page": {first, second}},
		Overlays:    NewOverlayManager(),
	}
	state.OpenAnchoredOverlay("popup", popup, true)
	state.OpenOverlay("toast", toast, false, true)
	state.CycleFocus(false)
	if state.FocusedID != "second" {
		t.Fatalf("focus after anchored dismissal=%q", state.FocusedID)
	}
	entries := state.Overlays.Snapshot()
	if len(entries) != 1 || entries[0].ID != "toast" {
		t.Fatalf("remaining overlays=%+v", entries)
	}
}

func TestFocusLossDismissalRestoresNestedOwnersTopDown(t *testing.T) {
	state := &ApplicationState{FocusedID: "launcher", Overlays: NewOverlayManager()}
	menu := &overlayTestComponent{id: "menu", focusable: true}
	popup := &overlayTestComponent{id: "popup"}
	state.OpenFocusedOverlay("menu", menu, true)
	state.OpenAnchoredOverlay("popup", popup, true)
	if dismissed := state.DismissFocusLossOverlays(); dismissed != 2 || state.FocusedID != "launcher" || len(state.Overlays.Snapshot()) != 0 {
		t.Fatalf("dismissed=%d focus=%q overlays=%+v", dismissed, state.FocusedID, state.Overlays.Snapshot())
	}
}

func TestPointerFocusLossPreservesPopupAndOwnerTargets(t *testing.T) {
	state := &ApplicationState{FocusedID: "owner", Overlays: NewOverlayManager()}
	popup := &overlayTestComponent{id: "popup"}
	state.OpenAnchoredOverlay("popup", popup, true)
	if dismissed := state.DismissFocusLossOverlaysForPointer("popup/option", image.Pt(5, 5)); dismissed != 0 || len(state.Overlays.Snapshot()) != 1 {
		t.Fatalf("popup target dismissed=%d overlays=%+v", dismissed, state.Overlays.Snapshot())
	}
	if dismissed := state.DismissFocusLossOverlaysForPointer("owner", image.Pt(20, 20)); dismissed != 0 || len(state.Overlays.Snapshot()) != 1 {
		t.Fatalf("owner target dismissed=%d overlays=%+v", dismissed, state.Overlays.Snapshot())
	}
}

func TestPointerFocusLossDismissesOutsideButKeepsToast(t *testing.T) {
	state := &ApplicationState{FocusedID: "owner", Overlays: NewOverlayManager()}
	state.OpenAnchoredOverlay("popup", &overlayTestComponent{id: "popup"}, true)
	state.OpenOverlay("toast", &overlayTestComponent{id: "toast"}, false, true)
	if dismissed := state.DismissFocusLossOverlaysForPointer("outside", image.Pt(20, 20)); dismissed != 1 || state.FocusedID != "owner" {
		t.Fatalf("dismissed=%d focus=%q", dismissed, state.FocusedID)
	}
	entries := state.Overlays.Snapshot()
	if len(entries) != 1 || entries[0].ID != "toast" {
		t.Fatalf("remaining overlays=%+v", entries)
	}
}

func TestPointerInsideNestedPopupPreservesAnchoredAncestors(t *testing.T) {
	state := &ApplicationState{FocusedID: "launcher", Overlays: NewOverlayManager()}
	state.OpenFocusedOverlay("menu", &overlayTestComponent{id: "menu", focusable: true}, true)
	state.OpenAnchoredOverlay("submenu", &overlayTestComponent{id: "submenu"}, true)
	if dismissed := state.DismissFocusLossOverlaysForPointer("submenu/item", image.Pt(5, 5)); dismissed != 0 || len(state.Overlays.Snapshot()) != 2 {
		t.Fatalf("dismissed=%d overlays=%+v", dismissed, state.Overlays.Snapshot())
	}
}

func TestOverlayDismissCallbackRunsOnceForEveryClosePath(t *testing.T) {
	for _, test := range []struct {
		name    string
		dismiss func(*ApplicationState)
	}{
		{name: "explicit", dismiss: func(state *ApplicationState) { state.CloseOverlay("popup") }},
		{name: "focus-loss", dismiss: func(state *ApplicationState) { state.DismissFocusLossOverlaysForTarget("outside") }},
		{name: "escape", dismiss: func(state *ApplicationState) { state.DismissTopOverlay() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := &ApplicationState{FocusedID: "owner", Overlays: NewOverlayManager()}
			calls := 0
			state.OpenAnchoredOverlayWithDismiss("popup", &overlayTestComponent{id: "popup"}, true, func(*ApplicationState) { calls++ })
			test.dismiss(state)
			if calls != 1 || len(state.Overlays.Snapshot()) != 0 || state.FocusedID != "owner" {
				t.Fatalf("calls=%d overlays=%+v focus=%q", calls, state.Overlays.Snapshot(), state.FocusedID)
			}
		})
	}
}

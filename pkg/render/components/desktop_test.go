package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestTabsPublishSelection(t *testing.T) {
	state := &types.ApplicationState{}
	var selected string
	tabs := NewTabs("views", []TabItem{{ID: "one", Label: "One"}, {ID: "two", Label: "Two"}}, "one", func(id string, _ *types.ApplicationState) { selected = id })
	tabs.Rect = image.Rect(0, 0, 200, 36)
	second := tabs.itemRects(state)[1]
	if !tabs.OnMouseDown(second.Min.Add(image.Pt(2, 2)), state) || !tabs.OnMouseUp(second.Min.Add(image.Pt(2, 2)), state) || selected != "two" {
		t.Fatalf("tab selection = %q", selected)
	}
}

func TestTabsPublishRequiredSingleSelectionCollection(t *testing.T) {
	tabs := NewTabs("views", []TabItem{{ID: "one", Label: "One"}, {ID: "two", Label: "Two"}}, "one", nil)
	node := tabs.Semantics(&types.ApplicationState{})
	if node.Collection == nil || !node.Collection.Selectable || !node.Collection.SelectionRequired || node.Collection.CanSelectMultiple {
		t.Fatalf("tab collection=%+v", node.Collection)
	}
}

func TestTabsKeyboardNavigationSkipsDisabledAndSurvivesRebuild(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "views", TransientState: renderstate.NewStore()}
	items := []TabItem{{ID: "one", Label: "One"}, {ID: "two", Label: "Two", Disabled: true}, {ID: "three", Label: "Three"}}
	requested := ""
	newTabs := func() *Tabs {
		return NewTabs("views", items, "one", func(id string, _ *types.ApplicationState) { requested = id })
	}
	first := newTabs()
	if !first.Focusable() || !first.OnKey(0x27, 0, state) || requested != "three" {
		t.Fatalf("right navigation requested %q", requested)
	}
	if first.SelectedID != "one" {
		t.Fatal("controlled tab navigation mutated application selection")
	}
	requested = ""
	rebuilt := newTabs()
	if !rebuilt.OnKey(0x27, 0, state) || requested != "one" {
		t.Fatalf("wrapped rebuilt navigation requested %q", requested)
	}
	requested = ""
	if !rebuilt.OnKey(0x23, 0, state) || requested != "three" {
		t.Fatalf("end navigation requested %q", requested)
	}
	requested = ""
	if !rebuilt.OnKey(0x24, 0, state) || requested != "one" {
		t.Fatalf("home navigation requested %q", requested)
	}
}

func TestTabsControlledSelectionHonorsApplicationChange(t *testing.T) {
	state := &types.ApplicationState{TransientState: renderstate.NewStore()}
	items := []TabItem{{ID: "one", Label: "One"}, {ID: "two", Label: "Two"}, {ID: "three", Label: "Three"}}
	selected := "one"
	newTabs := func() *Tabs {
		return NewTabs("views", items, selected, func(id string, _ *types.ApplicationState) { selected = id })
	}

	// The user clicks "three": the roving position moves and the application
	// applies the requested selection, so both agree on the next rebuild.
	clicked := newTabs()
	clicked.Rect = image.Rect(0, 0, 300, 36)
	third := clicked.itemRects(state)[2]
	if !clicked.OnMouseDown(third.Min.Add(image.Pt(2, 2)), state) || !clicked.OnMouseUp(third.Min.Add(image.Pt(2, 2)), state) {
		t.Fatal("tab click was not handled")
	}
	if selected != "three" {
		t.Fatalf("click requested %q, want three", selected)
	}
	if rebuilt := newTabs(); rebuilt.activeIndex(state) != 2 {
		t.Fatalf("after click+apply, active = %d, want 2", rebuilt.activeIndex(state))
	}

	// Now the application moves the selection on its own — no click, no key —
	// the way a "jump to this tab after saving" flow would. The stale roving
	// position ("three") must not shadow it.
	selected = "one"
	programmatic := newTabs()
	if got := programmatic.activeIndex(state); got != 0 {
		t.Fatalf("application-driven selection ignored: active = %d, want 0", got)
	}
}

func TestTabsSemanticFocusTargetsActiveTab(t *testing.T) {
	state := &types.ApplicationState{TransientState: renderstate.NewStore()}
	tabs := NewTabs("views", []TabItem{{ID: "one", Label: "One"}, {ID: "two", Label: "Two"}}, "one", nil)
	tabs.Rect = image.Rect(0, 0, 200, 36)
	if !tabs.PerformSemanticAction("views/two", semantics.ActionFocus, "", state) {
		t.Fatal("tab focus action was not handled")
	}
	node := tabs.Semantics(state)
	if state.FocusedID != "views" || node.State.Focused || node.Children[0].State.Focused || !node.Children[1].State.Focused || node.Children[1].State.Selected {
		t.Fatalf("focus state=%q root=%+v children=%+v", state.FocusedID, node.State, node.Children)
	}
	if tabs.PerformSemanticAction("views/two", semantics.ActionInvoke, "", state) {
		t.Fatal("unsupported tab action was accepted")
	}
	if !tabs.PerformSemanticAction("views/two", semantics.ActionSelect, "", state) || tabs.SelectedID != "two" {
		t.Fatalf("semantic selection=%q", tabs.SelectedID)
	}
}

func TestTabsWithoutEnabledItemsAreNotFocusable(t *testing.T) {
	tabs := NewTabs("views", []TabItem{{ID: "disabled", Label: "Disabled", Disabled: true}}, "", nil)
	if tabs.Focusable() {
		t.Fatal("disabled-only tabs should not enter sequential focus")
	}
}

func TestSelectDisabledOptionDoesNotPublish(t *testing.T) {
	state := &types.ApplicationState{Overlays: types.NewOverlayManager(), TransientState: renderstate.NewStore()}
	var selected string
	selectControl := NewSelect("format", []SelectOption{{Value: "pdf", Label: "PDF", Disabled: true}}, "", func(value string, _ *types.ApplicationState) { selected = value })
	selectControl.Rect = image.Rect(0, 0, 180, 36)
	selectControl.Open = true
	if selectControl.openOptions(state, false) {
		t.Fatal("disabled-only select reported an open popup")
	}
	overlays := state.Overlays.Snapshot()
	if len(overlays) != 0 {
		t.Fatalf("disabled-only select opened an unusable popup: %+v", overlays)
	}
	if selected != "" {
		t.Fatalf("disabled option published %q", selected)
	}
}

func TestSelectPopupRejectsDisabledPointerOption(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "format", Overlays: types.NewOverlayManager(), TransientState: renderstate.NewStore()}
	control := NewSelect("format", []SelectOption{{Value: "png", Label: "PNG"}, {Value: "jpg", Label: "JPEG", Disabled: true}}, "png", nil)
	control.Rect = image.Rect(0, 0, 180, 36)
	if !control.OnKey(13, 0, state) {
		t.Fatal("select did not open")
	}
	overlays := state.Overlays.Snapshot()
	popup, ok := overlays[0].Component.(*selectPopup)
	if !ok {
		t.Fatalf("overlay=%T", overlays[0].Component)
	}
	point := popup.rowRect(1, state).Min.Add(image.Pt(2, 2))
	if popup.OnMouseDown(point, state) {
		t.Fatal("disabled popup option captured pointer input")
	}
}

func TestSelectKeyboardStateSurvivesControlledRebuild(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "format", Overlays: types.NewOverlayManager(), TransientState: renderstate.NewStore()}
	options := []SelectOption{{Value: "png", Label: "PNG"}, {Value: "jpg", Label: "JPEG", Disabled: true}, {Value: "webp", Label: "WebP"}}
	requested := ""
	newControl := func() *Select {
		control := NewSelect("format", options, "png", func(value string, _ *types.ApplicationState) { requested = value })
		control.Rect = image.Rect(0, 0, 180, 36)
		return control
	}
	first := newControl()
	if !first.OnKey(0x28, 0, state) || requested != "webp" || first.Value != "png" {
		t.Fatalf("closed Down requested=%q value=%q", requested, first.Value)
	}
	requested = ""
	rebuilt := newControl()
	if !rebuilt.OnKey(13, 0, state) || len(state.Overlays.Snapshot()) != 1 {
		t.Fatal("rebuilt select did not open")
	}
	if !rebuilt.OnKey(0x26, 0, state) || !rebuilt.OnKey(13, 0, state) || requested != "png" {
		t.Fatalf("open keyboard selection requested=%q", requested)
	}
	if len(state.Overlays.Snapshot()) != 0 || state.FocusedID != "format" {
		t.Fatalf("popup remained open or focus changed: overlays=%d focus=%q", len(state.Overlays.Snapshot()), state.FocusedID)
	}
}

func TestSelectSemanticExpansionFocusAndSelection(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "format", Overlays: types.NewOverlayManager(), TransientState: renderstate.NewStore()}
	selected := ""
	control := NewSelect("format", []SelectOption{{Value: "png", Label: "PNG"}, {Value: "webp", Label: "WebP"}}, "png", func(value string, _ *types.ApplicationState) { selected = value })
	control.Rect = image.Rect(0, 0, 180, 36)
	if !control.PerformSemanticAction("format", semantics.ActionExpand, "", state) {
		t.Fatal("semantic expansion failed")
	}
	owner := control.Semantics(state)
	if !owner.State.Expanded || owner.State.Focused || len(owner.Children) != 0 {
		t.Fatalf("expanded owner semantics=%+v", owner)
	}
	popup := state.Overlays.Snapshot()[0].Component.(*selectPopup)
	node := popup.Semantics(state)
	if len(node.Children) != 2 || !node.Children[0].State.Focused || !node.Children[0].State.Selected {
		t.Fatalf("popup semantics=%+v", node.Children)
	}
	if !popup.PerformSemanticAction("format.options/webp", semantics.ActionFocus, "", state) || !popup.PerformSemanticAction("format.options/webp", semantics.ActionSelect, "", state) || selected != "webp" {
		t.Fatalf("semantic option selection=%q", selected)
	}
	if len(state.Overlays.Snapshot()) != 0 {
		t.Fatal("semantic option selection did not close popup")
	}
	if control.PerformSemanticAction("format", semantics.ActionInvoke, "", state) {
		t.Fatal("unadvertised combo-box action was accepted")
	}
}

func TestControlledSelectOutsideDismissalPublishesClosedState(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "format", Overlays: types.NewOverlayManager(), TransientState: renderstate.NewStore()}
	open := false
	control := NewSelect("format", []SelectOption{{Value: "png", Label: "PNG"}}, "png", nil)
	control.Rect = image.Rect(0, 0, 180, 36)
	control.OnOpenChange = func(next bool, _ *types.ApplicationState) { open = next }
	if !control.PerformSemanticAction("format", semantics.ActionExpand, "", state) || !open || len(state.Overlays.Snapshot()) != 1 {
		t.Fatalf("expanded open=%v overlays=%+v", open, state.Overlays.Snapshot())
	}
	if dismissed := state.DismissFocusLossOverlaysForTarget("outside"); dismissed != 1 || open || len(state.Overlays.Snapshot()) != 0 {
		t.Fatalf("dismissed=%d open=%v overlays=%+v", dismissed, open, state.Overlays.Snapshot())
	}
	rebuilt := NewSelect("format", control.Options, "png", nil)
	rebuilt.Open = open
	if rebuilt.overlayOpen(state) {
		t.Fatal("controlled rebuild reopened an externally dismissed popup")
	}
}

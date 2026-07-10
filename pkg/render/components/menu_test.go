package components

import (
	"image"
	"testing"

	"go_native_gpu_gui/pkg/render/semantics"
	"go_native_gpu_gui/pkg/render/types"
)

func TestMenuKeyboardSkipsDisabledAndSeparators(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "file"}
	selected := ""
	menu := NewMenu("file", []MenuItem{{ID: "disabled", Label: "Disabled", Disabled: true}, {Separator: true}, {ID: "open", Label: "Open", OnInvoke: func(*types.ApplicationState) { selected = "open" }}})
	if !menu.OnKey(0x28, 0, state) || menu.FocusedItem != 2 {
		t.Fatalf("focused item %d", menu.FocusedItem)
	}
	if !menu.OnKey(13, 0, state) || selected != "open" {
		t.Fatalf("selected %q", selected)
	}
}

func TestMenuKeyboardWrapsUsesHomeEndAndTypeahead(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "file"}
	menu := NewMenu("file", []MenuItem{
		{ID: "new", Label: "New"},
		{ID: "disabled", Label: "Never", Disabled: true},
		{Separator: true},
		{ID: "open", Label: "Open"},
		{ID: "save", Label: "Save"},
	})
	if menu.FocusedItem != 0 || !menu.OnKey(0x26, 0, state) || menu.FocusedItem != 4 {
		t.Fatalf("up did not wrap to last enabled item: %d", menu.FocusedItem)
	}
	if !menu.OnKey(0x24, 0, state) || menu.FocusedItem != 0 {
		t.Fatalf("home focused %d", menu.FocusedItem)
	}
	if !menu.OnKey(0x23, 0, state) || menu.FocusedItem != 4 {
		t.Fatalf("end focused %d", menu.FocusedItem)
	}
	if !menu.OnKey(0, 'o', state) || menu.FocusedItem != 3 {
		t.Fatalf("typeahead focused %d", menu.FocusedItem)
	}
	state.FocusedID = "elsewhere"
	if menu.OnKey(0x28, 0, state) || menu.FocusedItem != 3 {
		t.Fatal("unfocused menu consumed keyboard input")
	}
}

func TestMenuSemanticFocusAndInvokeActions(t *testing.T) {
	state := &types.ApplicationState{}
	invoked := false
	menu := NewMenu("file", []MenuItem{{ID: "new", Label: "New"}, {ID: "open", Label: "Open", OnInvoke: func(*types.ApplicationState) { invoked = true }}})
	menu.Rect = image.Rect(0, 0, 200, 80)
	if !menu.PerformSemanticAction("file/open", semantics.ActionFocus, "", state) {
		t.Fatal("menu-item focus was not handled")
	}
	node := menu.Semantics(state)
	if state.FocusedID != "file" || node.State.Focused || node.Children[0].State.Focused || !node.Children[1].State.Focused {
		t.Fatalf("menu focus=%q root=%+v children=%+v", state.FocusedID, node.State, node.Children)
	}
	if menu.PerformSemanticAction("file/open", semantics.ActionSelect, "", state) {
		t.Fatal("unadvertised selection action was accepted")
	}
	if !menu.PerformSemanticAction("file/open", semantics.ActionInvoke, "", state) || !invoked {
		t.Fatal("menu invoke was not handled")
	}
}

func TestMenuPointerFocusSkipsDisabledItems(t *testing.T) {
	state := &types.ApplicationState{}
	menu := NewMenu("file", []MenuItem{{ID: "disabled", Label: "Disabled", Disabled: true}, {ID: "open", Label: "Open"}})
	menu.Rect = image.Rect(0, 0, 200, 80)
	if menu.OnMouseDown(menu.itemRect(0, state).Min.Add(image.Pt(2, 2)), state) {
		t.Fatal("disabled menu item captured pointer focus")
	}
	if !menu.OnMouseDown(menu.itemRect(1, state).Min.Add(image.Pt(2, 2)), state) || state.FocusedID != "file" || menu.FocusedItem != 1 {
		t.Fatalf("pointer focus=%q item=%d", state.FocusedID, menu.FocusedItem)
	}
}
func TestToastDismissesOnPointerRelease(t *testing.T) {
	state := &types.ApplicationState{}
	dismissed := false
	toast := NewToast("saved", "Saved", "Document saved")
	toast.Rect = image.Rect(0, 0, 300, 80)
	toast.OnDismiss = func(*types.ApplicationState) { dismissed = true }
	if !toast.OnMouseDown(image.Pt(10, 10), state) || !toast.OnMouseUp(image.Pt(10, 10), state) || !dismissed {
		t.Fatal("toast did not dismiss")
	}
}

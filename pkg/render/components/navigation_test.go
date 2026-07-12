package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestBreadcrumbInvokesDestination(t *testing.T) {
	state := &types.ApplicationState{}
	invoked := false
	trail := NewBreadcrumbs("path", []BreadcrumbItem{
		{ID: "home", Label: "Home", OnInvoke: func(*types.ApplicationState) { invoked = true }},
		{ID: "current", Label: "Current"},
	})
	trail.Rect = image.Rect(0, 0, 220, 32)
	point := trail.itemRects(state)[0].Min.Add(image.Pt(2, 2))
	if !trail.OnMouseDown(point, state) || !trail.OnMouseUp(point, state) || !invoked {
		t.Fatal("breadcrumb did not invoke its destination")
	}
	if trail.HitTest(trail.itemRects(state)[1].Min.Add(image.Pt(2, 2))) != "" {
		t.Fatal("current location should not be interactive")
	}
}

func TestTreeKeyboardNavigationAndExpansion(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "files"}
	tree := NewTree("files", []TreeNode{
		{ID: "docs", Label: "Documents", Children: []TreeNode{{ID: "draft", Label: "Draft"}}},
		{ID: "archive", Label: "Archive"},
	}, "docs", nil, nil)
	tree.Rect = image.Rect(0, 0, 240, 160)
	if !tree.OnKey(0x27, 0, state) || !tree.Expanded["docs"] {
		t.Fatal("right arrow did not expand selected branch")
	}
	if !tree.OnKey(0x28, 0, state) || tree.SelectedID != "draft" {
		t.Fatalf("down arrow selection = %q", tree.SelectedID)
	}
	if !tree.OnKey(0x25, 0, state) || tree.SelectedID != "docs" {
		t.Fatalf("left arrow parent selection = %q", tree.SelectedID)
	}
}

func TestTreeSemanticActionsAreControlled(t *testing.T) {
	state := &types.ApplicationState{}
	selected := ""
	toggledID := ""
	toggledValue := false
	tree := NewTree("files", []TreeNode{{ID: "docs", Label: "Documents", Children: []TreeNode{{ID: "draft", Label: "Draft"}}}}, "", map[string]bool{}, func(id string, _ *types.ApplicationState) { selected = id })
	tree.OnToggle = func(id string, expanded bool, _ *types.ApplicationState) {
		toggledID, toggledValue = id, expanded
	}
	if !tree.PerformSemanticAction("files/docs", semantics.ActionExpand, "", state) || toggledID != "docs" || !toggledValue {
		t.Fatalf("toggle callback = %q %v", toggledID, toggledValue)
	}
	if tree.Expanded["docs"] {
		t.Fatal("controlled tree mutated expansion value")
	}
	if !tree.PerformSemanticAction("files/docs", semantics.ActionSelect, "", state) || selected != "docs" {
		t.Fatalf("selection callback = %q", selected)
	}
}

func TestTreeActiveItemSurvivesControlledRebuild(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "files", TransientState: renderstate.NewStore()}
	nodes := []TreeNode{{ID: "docs", Label: "Documents", Children: []TreeNode{{ID: "draft", Label: "Draft"}, {ID: "final", Label: "Final"}}}, {ID: "archive", Label: "Archive"}}
	expanded := map[string]bool{"docs": true}
	requested := ""
	newTree := func() *Tree {
		return NewTree("files", nodes, "docs", expanded, func(id string, _ *types.ApplicationState) { requested = id })
	}
	first := newTree()
	if !first.OnKey(0x28, 0, state) || requested != "draft" || first.SelectedID != "docs" {
		t.Fatalf("first Down requested=%q selected=%q", requested, first.SelectedID)
	}
	requested = ""
	rebuilt := newTree()
	if !rebuilt.OnKey(0x28, 0, state) || requested != "final" {
		t.Fatalf("rebuilt Down requested=%q", requested)
	}
	requested = ""
	if !rebuilt.OnKey(0x23, 0, state) || requested != "archive" {
		t.Fatalf("End requested=%q", requested)
	}
	requested = ""
	if !rebuilt.OnKey(0, 'd', state) || requested != "docs" {
		t.Fatalf("typeahead requested=%q", requested)
	}
}

func TestTreeSemanticsPreserveHierarchyAndSingleActiveFocus(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "files", TransientState: renderstate.NewStore()}
	tree := NewTree("files", []TreeNode{{ID: "docs", Label: "Documents", Children: []TreeNode{{ID: "draft", Label: "Draft"}}}, {ID: "archive", Label: "Archive"}}, "docs", map[string]bool{"docs": true}, nil)
	tree.Rect = image.Rect(0, 0, 240, 120)
	if !tree.PerformSemanticAction("files/draft", semantics.ActionFocus, "", state) {
		t.Fatal("tree-item focus failed")
	}
	node := tree.Semantics(state)
	if node.State.Focused || len(node.Children) != 2 || len(node.Children[0].Children) != 1 || node.Children[0].Children[0].ID != "files/draft" || !node.Children[0].Children[0].State.Focused {
		t.Fatalf("tree hierarchy/focus root=%+v children=%+v", node.State, node.Children)
	}
	if node.Children[0].State.Selected != true || node.Children[0].Children[0].State.Selected {
		t.Fatalf("focus changed controlled selection: %+v", node.Children)
	}
	if tree.PerformSemanticAction("files/draft", semantics.ActionInvoke, "", state) {
		t.Fatal("tree item accepted unadvertised invoke action")
	}
}

func TestTreeDisabledItemsDoNotCapturePointerOrFocus(t *testing.T) {
	state := &types.ApplicationState{TransientState: renderstate.NewStore()}
	tree := NewTree("files", []TreeNode{{ID: "disabled", Label: "Disabled", Disabled: true}, {ID: "open", Label: "Open"}}, "", nil, nil)
	tree.Rect = image.Rect(0, 0, 240, 72)
	if tree.OnMouseDown(image.Pt(10, 10), state) {
		t.Fatal("disabled tree item captured pointer focus")
	}
	if !tree.OnMouseDown(image.Pt(10, tree.rowHeight(state)+5), state) || state.FocusedID != "files" {
		t.Fatalf("enabled tree item focus=%q", state.FocusedID)
	}
}

func BenchmarkTreeVisibleRowsStable(b *testing.B) {
	tree := NewTree("files", []TreeNode{
		{ID: "docs", Label: "Documents", Children: []TreeNode{{ID: "draft", Label: "Draft"}, {ID: "final", Label: "Final"}}},
		{ID: "archive", Label: "Archive"},
	}, "docs", map[string]bool{"docs": true}, nil)
	tree.visibleRows() // establish reusable capacity
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if len(tree.visibleRows()) != 4 {
			b.Fatal("unexpected row count")
		}
	}
}

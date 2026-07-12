package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestPaginationKeyboardPublishesRequestedPage(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "pages"}
	requested := 0
	pages := NewPagination("pages", 3, 10, func(page int, _ *types.ApplicationState) { requested = page })
	if !pages.OnKey(0x27, 0, state) || requested != 4 {
		t.Fatalf("requested page = %d", requested)
	}
	requested = 0
	if !pages.OnKey(0x24, 0, state) || requested != 1 {
		t.Fatalf("home requested page = %d", requested)
	}
}

func TestPaginationBoundaryActionsAreDisabled(t *testing.T) {
	state := &types.ApplicationState{}
	pages := NewPagination("pages", 1, 3, nil)
	pages.Rect = image.Rect(0, 0, 240, 36)
	sem := pages.Semantics(state)
	if len(sem.Children) != 5 || !sem.Children[0].State.Disabled {
		t.Fatalf("unexpected pagination semantics: %+v", sem.Children)
	}
	if pages.PerformSemanticAction("pages/previous", semantics.ActionInvoke, "", state) {
		t.Fatal("previous action should be disabled on first page")
	}
	if !pages.PerformSemanticAction("pages/page/3", semantics.ActionInvoke, "", state) || pages.Page != 3 {
		t.Fatalf("page action did not update uncontrolled value: %d", pages.Page)
	}
}

func TestPaginationActivePageSurvivesControlledRebuild(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "pages", TransientState: renderstate.NewStore()}
	requested := 0
	newPages := func() *Pagination {
		return NewPagination("pages", 3, 12, func(page int, _ *types.ApplicationState) { requested = page })
	}
	first := newPages()
	if !first.OnKey(0x27, 0, state) || requested != 4 || first.Page != 3 {
		t.Fatalf("first Right requested=%d page=%d", requested, first.Page)
	}
	requested = 0
	rebuilt := newPages()
	if !rebuilt.OnKey(0x27, 0, state) || requested != 5 {
		t.Fatalf("rebuilt Right requested=%d", requested)
	}
	requested = 0
	if !rebuilt.OnKey(0x23, 0, state) || requested != 12 || rebuilt.activePage(state) != 12 {
		t.Fatalf("End requested=%d active=%d", requested, rebuilt.activePage(state))
	}
	requested = 0
	if !rebuilt.OnKey(0x24, 0, state) || requested != 1 || rebuilt.activePage(state) != 1 {
		t.Fatalf("Home requested=%d active=%d", requested, rebuilt.activePage(state))
	}
}

func TestPaginationSemanticsExposeSingleActivePageFocus(t *testing.T) {
	state := &types.ApplicationState{TransientState: renderstate.NewStore()}
	pages := NewPagination("pages", 3, 12, nil)
	pages.Rect = image.Rect(0, 0, 400, 36)
	if !pages.PerformSemanticAction("pages/page/5", semantics.ActionFocus, "", state) {
		t.Fatal("page focus action failed")
	}
	node := pages.Semantics(state)
	if node.State.Focused || state.FocusedID != "pages" {
		t.Fatalf("pagination owner focus=%+v id=%q", node.State, state.FocusedID)
	}
	focusedCount := 0
	for _, child := range node.Children {
		if child.State.Focused {
			focusedCount++
			if child.ID != "pages/page/5" || child.State.Selected {
				t.Fatalf("focused page=%+v", child)
			}
		}
	}
	if focusedCount != 1 {
		t.Fatalf("focused descendants=%d children=%+v", focusedCount, node.Children)
	}
	if pages.PerformSemanticAction("pages/page/5", semantics.ActionSelect, "", state) || pages.Page != 3 {
		t.Fatal("page accepted unadvertised select action")
	}
	if !pages.PerformSemanticAction("pages/page/5", semantics.ActionInvoke, "", state) || pages.Page != 5 {
		t.Fatalf("page invoke selected %d", pages.Page)
	}
}

func TestPaginationDisabledBoundaryDoesNotCapturePointer(t *testing.T) {
	state := &types.ApplicationState{TransientState: renderstate.NewStore()}
	pages := NewPagination("pages", 1, 3, nil)
	pages.Rect = image.Rect(0, 0, 240, 36)
	items := pages.rebuildItems(state)
	if pages.OnMouseDown(items[0].bounds.Min.Add(image.Pt(2, 2)), state) {
		t.Fatal("disabled previous button captured pointer")
	}
	if !pages.OnMouseDown(items[1].bounds.Min.Add(image.Pt(2, 2)), state) || state.FocusedID != "pages" {
		t.Fatalf("current page pointer focus=%q", state.FocusedID)
	}
}

func TestEmptyPaginationIsNotFocusable(t *testing.T) {
	if NewPagination("pages", 0, 0, nil).Focusable() {
		t.Fatal("empty pagination entered sequential focus")
	}
}

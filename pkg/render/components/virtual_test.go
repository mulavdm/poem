package components

import (
	"fmt"
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestVirtualListBuildsOnlyVisibleItems(t *testing.T) {
	built := 0
	list := NewVirtualList("items", 10000, 32, func(index int) types.Component {
		built++
		return NewLabel(fmt.Sprintf("item-%d", index), fmt.Sprintf("Item %d", index))
	})
	list.SetBounds(image.Rect(0, 0, 300, 320))
	list.layoutVisible()
	if built > 14 {
		t.Fatalf("built %d items for 10-row viewport", built)
	}
	if len(list.visible) == 0 {
		t.Fatal("no visible items")
	}
}

func TestDataTablePublishesGridCoordinatesAndOffscreenState(t *testing.T) {
	table := NewDataTable("table", []TableColumn{{Key: "name", Label: "Name", Width: 120}, {Key: "status", Label: "Status"}}, []TableRow{
		{ID: "a", Values: map[string]string{"name": "Alpha", "status": "Ready"}},
		{ID: "b", Values: map[string]string{"name": "Beta", "status": "Busy"}},
		{ID: "c", Values: map[string]string{"name": "Gamma", "status": "Done"}},
	})
	table.SelectedID = "b"
	table.SetBounds(image.Rect(0, 0, 300, 80))
	node := table.Semantics(&types.ApplicationState{})
	if node.Grid == nil || node.Grid.Rows != 3 || node.Grid.Columns != 2 || node.Collection == nil {
		t.Fatalf("table metadata=%+v collection=%+v", node.Grid, node.Collection)
	}
	if len(node.Children) != 4 || len(node.Children[2].Children) != 2 {
		t.Fatalf("semantic table shape: %+v", node)
	}
	cell := node.Children[2].Children[1]
	if cell.GridItem == nil || cell.GridItem.Row != 1 || cell.GridItem.Column != 1 || cell.Value != "Busy" || !cell.State.ReadOnly {
		t.Fatalf("cell metadata=%+v", cell)
	}
	if !node.Children[2].State.Selected || node.Children[2].State.Offscreen || !node.Children[3].State.Offscreen {
		t.Fatalf("row states selected=%+v offscreen=%+v", node.Children[2].State, node.Children[3].State)
	}
}

func TestDataTableKeyboardNavigationPersistsAcrossControlledRebuild(t *testing.T) {
	state := &types.ApplicationState{
		FocusedID:       "table",
		TransientState:  renderstate.NewStore(),
		ScrollPositions: make(map[string]int),
		ScrollCurrent:   make(map[string]float64),
	}
	rows := make([]TableRow, 10)
	for index := range rows {
		rows[index] = TableRow{ID: fmt.Sprintf("row-%d", index), Values: map[string]string{"name": fmt.Sprintf("Row %d", index)}}
	}
	requested := ""
	newTable := func() *DataTable {
		table := NewDataTable("table", []TableColumn{{Key: "name", Label: "Name"}}, rows)
		table.Rect = image.Rect(0, 0, 300, 172) // four body rows
		table.OnSelect = func(id string, _ *types.ApplicationState) { requested = id }
		return table
	}

	first := newTable()
	if !first.Focusable() || !first.OnKey(0x28, 0, state) || requested != "row-1" {
		t.Fatalf("down navigation requested %q", requested)
	}
	if first.SelectedID != "" {
		t.Fatal("controlled navigation mutated the application-owned selection")
	}

	requested = ""
	rebuilt := newTable()
	if !rebuilt.OnKey(0x22, 0, state) || requested != "row-4" {
		t.Fatalf("retained page navigation requested %q", requested)
	}
	if rebuilt.ScrollY == 0 || state.ScrollPositions["table"] != rebuilt.ScrollY || state.ScrollCurrent["table"] != float64(rebuilt.ScrollY) {
		t.Fatalf("active row was not scrolled into view: scroll=%d positions=%v current=%v", rebuilt.ScrollY, state.ScrollPositions, state.ScrollCurrent)
	}

	requested = ""
	if !rebuilt.OnKey(0x23, 0, state) || requested != "row-9" || rebuilt.ScrollY != rebuilt.maxScroll() {
		t.Fatalf("end navigation requested=%q scroll=%d max=%d", requested, rebuilt.ScrollY, rebuilt.maxScroll())
	}
	requested = ""
	if !rebuilt.OnKey(0x24, 0, state) || requested != "row-0" || rebuilt.ScrollY != 0 {
		t.Fatalf("home navigation requested=%q scroll=%d", requested, rebuilt.ScrollY)
	}
}

func TestDataTableSemanticFocusAndSelectionActions(t *testing.T) {
	state := &types.ApplicationState{TransientState: renderstate.NewStore()}
	table := NewDataTable("table", []TableColumn{{Key: "name", Label: "Name"}}, []TableRow{
		{ID: "a", Values: map[string]string{"name": "Alpha"}},
		{ID: "b", Values: map[string]string{"name": "Beta"}},
		{ID: "c", Values: map[string]string{"name": "Gamma"}},
	})
	table.Rect = image.Rect(0, 0, 240, 70)
	if !table.PerformSemanticAction("table/row/c", semantics.ActionFocus, "", state) {
		t.Fatal("row focus action was not handled")
	}
	if state.FocusedID != "table" || table.ScrollY != table.maxScroll() {
		t.Fatalf("focus=%q scroll=%d max=%d", state.FocusedID, table.ScrollY, table.maxScroll())
	}
	node := table.Semantics(state)
	if node.State.Focused || !node.Children[3].State.Focused || node.Children[3].State.Selected {
		t.Fatalf("focused row semantics root=%+v row=%+v", node.State, node.Children[3].State)
	}
	if !table.PerformSemanticAction("table/row/c", semantics.ActionSelect, "", state) || table.SelectedID != "c" {
		t.Fatalf("semantic selection=%q", table.SelectedID)
	}
	if table.PerformSemanticAction("table/row/c", semantics.ActionInvoke, "", state) {
		t.Fatal("unsupported row action was accepted")
	}
}

func TestDataTablePointerFocusUsesSingleGridTabStop(t *testing.T) {
	state := &types.ApplicationState{TransientState: renderstate.NewStore()}
	table := NewDataTable("table", []TableColumn{{Key: "name", Label: "Name"}}, []TableRow{{ID: "a", Values: map[string]string{"name": "Alpha"}}})
	table.Rect = image.Rect(0, 0, 240, 100)
	point := image.Pt(20, table.headerHeight()+10)
	if !table.OnMouseDown(point, state) || state.FocusedID != "table" {
		t.Fatalf("pointer focus=%q", state.FocusedID)
	}
	if !table.OnMouseUp(point, state) || table.SelectedID != "a" {
		t.Fatalf("pointer selection=%q", table.SelectedID)
	}
}

func BenchmarkVirtualListLayout(b *testing.B) {
	list := NewVirtualList("items", 100000, 32, func(index int) types.Component { return NewLabel("item", "") })
	list.SetBounds(image.Rect(0, 0, 400, 640))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		list.ScrollY = (i * 17) % list.maxScroll()
		list.layoutVisible()
	}
}

func BenchmarkVirtualListStableFrame(b *testing.B) {
	list := NewVirtualList("items", 100000, 32, func(index int) types.Component { return NewLabel("item", "") })
	list.SetBounds(image.Rect(0, 0, 400, 640))
	list.layoutVisible()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.layoutVisible()
	}
}

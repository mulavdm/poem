package components

import (
	"fmt"
	"image"
	"math"

	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

type virtualItem struct {
	index     int
	component types.Component
}

type VirtualList struct {
	CompID     string
	Rect       image.Rectangle
	ItemCount  int
	ItemHeight int
	Overscan   int
	ScrollY    int
	BuildItem  func(index int) types.Component
	visible    []virtualItem
	cache      map[int]types.Component
}

func NewVirtualList(id string, itemCount, itemHeight int, build func(int) types.Component) *VirtualList {
	return &VirtualList{CompID: id, ItemCount: itemCount, ItemHeight: itemHeight, Overscan: 2, BuildItem: build, cache: make(map[int]types.Component)}
}
func (v *VirtualList) ID() string                  { return v.CompID }
func (v *VirtualList) GetID() string               { return v.CompID }
func (v *VirtualList) Bounds() image.Rectangle     { return v.Rect }
func (v *VirtualList) SetBounds(r image.Rectangle) { v.Rect = r }
func (v *VirtualList) Focusable() bool             { return false }
func (v *VirtualList) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	w, h := avail.X, avail.Y
	if w <= 0 {
		w = 240
	}
	if h <= 0 {
		h = minValueInt(v.ItemCount*v.itemHeight(), 320)
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(v.Rect), image.Pt(w, h)), Min: image.Pt(120, minValueInt(h, 80))}
}
func (v *VirtualList) itemHeight() int {
	if v.ItemHeight <= 0 {
		return 36
	}
	return v.ItemHeight
}
func (v *VirtualList) maxScroll() int {
	max := v.ItemCount*v.itemHeight() - v.Rect.Dy()
	if max < 0 {
		return 0
	}
	return max
}
func (v *VirtualList) layoutVisible() {
	v.visible = v.visible[:0]
	if v.BuildItem == nil || v.ItemCount <= 0 || v.Rect.Empty() {
		return
	}
	h := v.itemHeight()
	first := v.ScrollY/h - v.Overscan
	if first < 0 {
		first = 0
	}
	last := (v.ScrollY+v.Rect.Dy()+h-1)/h + v.Overscan
	if last > v.ItemCount {
		last = v.ItemCount
	}
	for index := first; index < last; index++ {
		component := v.cache[index]
		if component == nil {
			component = v.BuildItem(index)
			if component != nil {
				v.cache[index] = component
			}
		}
		if component == nil {
			continue
		}
		top := v.Rect.Min.Y + index*h - v.ScrollY
		component.SetBounds(image.Rect(v.Rect.Min.X, top, v.Rect.Max.X, top+h))
		v.visible = append(v.visible, virtualItem{index: index, component: component})
	}
	maxCached := (last - first) * 3
	if maxCached > 0 && len(v.cache) > maxCached {
		keepFrom, keepTo := first-(last-first), last+(last-first)
		for index := range v.cache {
			if index < keepFrom || index >= keepTo {
				delete(v.cache, index)
			}
		}
	}
}
func (v *VirtualList) Draw(p types.Painter, state *types.ApplicationState) {
	if state.ScrollPositions != nil {
		v.ScrollY = state.ScrollPositions[v.CompID]
	}
	if v.ScrollY < 0 {
		v.ScrollY = 0
	}
	if v.ScrollY > v.maxScroll() {
		v.ScrollY = v.maxScroll()
	}
	v.layoutVisible()
	p.FillRect(v.Rect, activeTheme(state).Colors.SurfaceSunken)
	p.PushClip(v.Rect)
	for _, item := range v.visible {
		item.component.Draw(p, state)
	}
	p.PopClip()
}
func (v *VirtualList) HitTest(pt image.Point) string {
	if !pt.In(v.Rect) {
		return ""
	}
	v.layoutVisible()
	for index := len(v.visible) - 1; index >= 0; index-- {
		if id := v.visible[index].component.HitTest(pt); id != "" {
			return id
		}
	}
	return v.CompID
}
func (v *VirtualList) OnMouseWheel(pt image.Point, delta int, state *types.ApplicationState) bool {
	if !pt.In(v.Rect) {
		return false
	}
	step := v.itemHeight() * 3
	if delta > 0 {
		v.ScrollY -= step
	} else {
		v.ScrollY += step
	}
	if v.ScrollY < 0 {
		v.ScrollY = 0
	}
	if v.ScrollY > v.maxScroll() {
		v.ScrollY = v.maxScroll()
	}
	if state.ScrollPositions != nil {
		state.ScrollPositions[v.CompID] = v.ScrollY
	}
	return true
}
func (v *VirtualList) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	for _, item := range v.visible {
		if item.component.OnKey(key, char, state) {
			return true
		}
	}
	return false
}
func (v *VirtualList) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	for i := len(v.visible) - 1; i >= 0; i-- {
		if pt.In(v.visible[i].component.Bounds()) && v.visible[i].component.OnMouseDown(pt, state) {
			return true
		}
	}
	return false
}
func (v *VirtualList) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	for i := len(v.visible) - 1; i >= 0; i-- {
		if v.visible[i].component.OnMouseUp(pt, state) {
			return true
		}
	}
	return false
}
func (v *VirtualList) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	for i := len(v.visible) - 1; i >= 0; i-- {
		if pt.In(v.visible[i].component.Bounds()) && v.visible[i].component.OnMouseMove(pt, state) {
			return true
		}
	}
	return false
}
func (v *VirtualList) Walk(fn func(types.Component)) {
	fn(v)
	for _, item := range v.visible {
		item.component.Walk(fn)
	}
}

func (v *VirtualList) ChildComponents() []types.Component {
	if len(v.visible) == 0 {
		v.layoutVisible()
	}
	children := make([]types.Component, 0, len(v.visible))
	for _, item := range v.visible {
		children = append(children, item.component)
	}
	return children
}
func (v *VirtualList) Semantics(state *types.ApplicationState) semantics.Node {
	node := semantics.Node{ID: v.CompID, Role: semantics.RoleList, Name: v.CompID, Bounds: v.Rect}
	for _, item := range v.visible {
		node.Children = append(node.Children, semantics.Node{ID: fmt.Sprintf("%s/item/%d", v.CompID, item.index), Role: semantics.RoleListItem, Name: fmt.Sprintf("Item %d", item.index+1), Bounds: item.component.Bounds()})
	}
	return node
}

type TableColumn struct {
	Key, Label string
	Width      int
}
type TableRow struct {
	ID     string
	Values map[string]string
}

type dataTableInteraction struct {
	ActiveRowID string
}

type DataTable struct {
	CompID       string
	Rect         image.Rectangle
	Columns      []TableColumn
	Rows         []TableRow
	RowHeight    int
	HeaderHeight int
	ScrollY      int
	SelectedID   string
	OnSelect     func(string, *types.ApplicationState)
}

func NewDataTable(id string, columns []TableColumn, rows []TableRow) *DataTable {
	return &DataTable{CompID: id, Columns: columns, Rows: rows, RowHeight: 34, HeaderHeight: 36}
}
func (t *DataTable) ID() string                  { return t.CompID }
func (t *DataTable) GetID() string               { return t.CompID }
func (t *DataTable) Bounds() image.Rectangle     { return t.Rect }
func (t *DataTable) SetBounds(r image.Rectangle) { t.Rect = r }
func (t *DataTable) Focusable() bool             { return true }
func (t *DataTable) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	w, h := avail.X, avail.Y
	if w <= 0 {
		for _, c := range t.Columns {
			w += c.Width
		}
	}
	if h <= 0 {
		h = minValueInt(t.HeaderHeight+len(t.Rows)*t.RowHeight, 360)
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(t.Rect), image.Pt(w, h)), Min: image.Pt(240, 100)}
}
func (t *DataTable) columnWidths() []int {
	out := make([]int, len(t.Columns))
	fixed, flex := 0, 0
	for i, c := range t.Columns {
		if c.Width > 0 {
			out[i] = c.Width
			fixed += c.Width
		} else {
			flex++
		}
	}
	remaining := t.Rect.Dx() - fixed
	if remaining < 0 {
		remaining = 0
	}
	if flex > 0 {
		for i, c := range t.Columns {
			if c.Width <= 0 {
				out[i] = remaining / flex
			}
		}
	}
	return out
}
func (t *DataTable) rowHeight() int {
	if t.RowHeight <= 0 {
		return 34
	}
	return t.RowHeight
}
func (t *DataTable) headerHeight() int {
	if t.HeaderHeight <= 0 {
		return 36
	}
	return t.HeaderHeight
}
func (t *DataTable) maxScroll() int {
	max := len(t.Rows)*t.rowHeight() - (t.Rect.Dy() - t.headerHeight())
	if max < 0 {
		return 0
	}
	return max
}

func (t *DataTable) interactionKey() string { return t.CompID + "/table" }

func (t *DataTable) activeRowIndex(state *types.ApplicationState) int {
	activeID := ""
	if state != nil && state.TransientState != nil {
		if interaction, ok := renderstate.Load[dataTableInteraction](state.TransientState, t.interactionKey()); ok {
			activeID = interaction.ActiveRowID
		}
	}
	if activeID == "" {
		activeID = t.SelectedID
	}
	for index := range t.Rows {
		if t.Rows[index].ID == activeID {
			return index
		}
	}
	if len(t.Rows) > 0 {
		return 0
	}
	return -1
}

func (t *DataTable) storeActiveRow(index int, state *types.ApplicationState) {
	if state == nil || index < 0 || index >= len(t.Rows) {
		return
	}
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, t.interactionKey(), dataTableInteraction{ActiveRowID: t.Rows[index].ID})
}

func (t *DataTable) storeScroll(state *types.ApplicationState) {
	if state == nil {
		return
	}
	if state.ScrollPositions == nil {
		state.ScrollPositions = make(map[string]int)
	}
	state.ScrollPositions[t.CompID] = t.ScrollY
	if state.ScrollCurrent == nil {
		state.ScrollCurrent = make(map[string]float64)
	}
	state.ScrollCurrent[t.CompID] = float64(t.ScrollY)
}

func (t *DataTable) ensureRowVisible(index int, state *types.ApplicationState) {
	if index < 0 || index >= len(t.Rows) {
		return
	}
	viewportHeight := maxInt(0, t.Rect.Dy()-t.headerHeight())
	rowTop := index * t.rowHeight()
	rowBottom := rowTop + t.rowHeight()
	if rowTop < t.ScrollY {
		t.ScrollY = rowTop
	} else if rowBottom > t.ScrollY+viewportHeight {
		t.ScrollY = rowBottom - viewportHeight
	}
	if t.ScrollY < 0 {
		t.ScrollY = 0
	} else if t.ScrollY > t.maxScroll() {
		t.ScrollY = t.maxScroll()
	}
	t.storeScroll(state)
}

func (t *DataTable) selectRow(index int, state *types.ApplicationState) bool {
	if index < 0 || index >= len(t.Rows) {
		return false
	}
	t.storeActiveRow(index, state)
	t.ensureRowVisible(index, state)
	id := t.Rows[index].ID
	if t.OnSelect != nil {
		t.OnSelect(id, state)
	} else {
		t.SelectedID = id
	}
	return true
}

func (t *DataTable) navigateToRow(index int, state *types.ApplicationState) bool {
	if index < 0 || index >= len(t.Rows) {
		return false
	}
	if index == t.activeRowIndex(state) {
		t.storeActiveRow(index, state)
		t.ensureRowVisible(index, state)
		return true
	}
	return t.selectRow(index, state)
}

func (t *DataTable) Draw(p types.Painter, state *types.ApplicationState) {
	if state.ScrollPositions != nil {
		t.ScrollY = state.ScrollPositions[t.CompID]
	}
	if t.ScrollY < 0 {
		t.ScrollY = 0
	} else if t.ScrollY > t.maxScroll() {
		t.ScrollY = t.maxScroll()
	}
	th := activeTheme(state)
	p.FillRect(t.Rect, th.Colors.SurfaceSunken)
	widths := t.columnWidths()
	x := t.Rect.Min.X
	header := image.Rect(t.Rect.Min.X, t.Rect.Min.Y, t.Rect.Max.X, t.Rect.Min.Y+t.headerHeight())
	p.FillRect(header, th.Colors.SurfaceRaised)
	for i, c := range t.Columns {
		p.DrawText(c.Label, x+th.Spacing.SM, header.Min.Y+header.Dy()/2+5, th.Colors.TextMuted)
		x += widths[i]
	}
	body := image.Rect(t.Rect.Min.X, header.Max.Y, t.Rect.Max.X, t.Rect.Max.Y)
	p.PushClip(body)
	first := t.ScrollY / t.rowHeight()
	last := (t.ScrollY + body.Dy() + t.rowHeight() - 1) / t.rowHeight()
	if last > len(t.Rows) {
		last = len(t.Rows)
	}
	for rowIndex := first; rowIndex < last; rowIndex++ {
		row := t.Rows[rowIndex]
		top := body.Min.Y + rowIndex*t.rowHeight() - t.ScrollY
		r := image.Rect(body.Min.X, top, body.Max.X, top+t.rowHeight())
		if row.ID == t.SelectedID {
			p.FillRect(r, th.Colors.Selection)
		} else if rowIndex%2 == 1 {
			// Resolve the zebra tone before it reaches the presenter. An
			// alpha-only stripe is composited differently by native backends
			// when a surrounding surface also carries effects, which made
			// light Android tables alternate to near-black rows.
			stripe := mix(th.Colors.SurfaceSunken, th.Colors.Surface, 96)
			p.FillRect(r, stripe)
		}
		x = body.Min.X
		for colIndex, c := range t.Columns {
			value := row.Values[c.Key]
			p.DrawText(fitButtonLabel(value, widths[colIndex], state.FontCharWidth), x+th.Spacing.SM, r.Min.Y+r.Dy()/2+5, th.Colors.Text)
			x += widths[colIndex]
		}
		p.FillRect(image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), th.Colors.Border)
	}
	p.PopClip()
	if state != nil && state.FocusedID == t.CompID {
		focus := th.Colors.Focus
		if active := t.activeRowIndex(state); active >= 0 {
			top := body.Min.Y + active*t.rowHeight() - t.ScrollY
			r := image.Rect(body.Min.X, top, body.Max.X, top+t.rowHeight()).Intersect(body)
			if !r.Empty() {
				p.FillRect(image.Rect(r.Min.X, r.Min.Y, r.Max.X, minValueInt(r.Min.Y+2, r.Max.Y)), focus)
				p.FillRect(image.Rect(r.Min.X, maxInt(r.Min.Y, r.Max.Y-2), r.Max.X, r.Max.Y), focus)
				p.FillRect(image.Rect(r.Min.X, r.Min.Y, minValueInt(r.Min.X+2, r.Max.X), r.Max.Y), focus)
				p.FillRect(image.Rect(maxInt(r.Min.X, r.Max.X-2), r.Min.Y, r.Max.X, r.Max.Y), focus)
			}
		} else {
			p.FillRect(image.Rect(t.Rect.Min.X, t.Rect.Min.Y, t.Rect.Max.X, minValueInt(t.Rect.Min.Y+2, t.Rect.Max.Y)), focus)
			p.FillRect(image.Rect(t.Rect.Min.X, maxInt(t.Rect.Min.Y, t.Rect.Max.Y-2), t.Rect.Max.X, t.Rect.Max.Y), focus)
		}
	}
}
func (t *DataTable) rowAt(pt image.Point) int {
	bodyTop := t.Rect.Min.Y + t.headerHeight()
	if !pt.In(image.Rect(t.Rect.Min.X, bodyTop, t.Rect.Max.X, t.Rect.Max.Y)) {
		return -1
	}
	index := (pt.Y - bodyTop + t.ScrollY) / t.rowHeight()
	if index < 0 || index >= len(t.Rows) {
		return -1
	}
	return index
}
func (t *DataTable) HitTest(pt image.Point) string {
	if index := t.rowAt(pt); index >= 0 {
		return t.CompID + "/row/" + t.Rows[index].ID
	}
	if pt.In(t.Rect) {
		return t.CompID
	}
	return ""
}
func (t *DataTable) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	index := t.rowAt(pt)
	if index < 0 {
		return false
	}
	state.FocusedID = t.CompID
	t.storeActiveRow(index, state)
	state.ActiveID = t.CompID + "/row/" + t.Rows[index].ID
	return true
}
func (t *DataTable) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	index := t.rowAt(pt)
	if index < 0 {
		return false
	}
	id := t.Rows[index].ID
	if state.ActiveID != t.CompID+"/row/"+id {
		return false
	}
	return t.selectRow(index, state)
}
func (t *DataTable) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (t *DataTable) OnKey(key uint32, _ rune, state *types.ApplicationState) bool {
	if state == nil || state.FocusedID != t.CompID || len(t.Rows) == 0 {
		return false
	}
	index := t.activeRowIndex(state)
	pageRows := maxInt(1, maxInt(0, t.Rect.Dy()-t.headerHeight())/t.rowHeight()-1)
	switch key {
	case 0x28: // Down
		return t.navigateToRow(minValueInt(len(t.Rows)-1, index+1), state)
	case 0x26: // Up
		return t.navigateToRow(maxInt(0, index-1), state)
	case 0x24: // Home
		return t.navigateToRow(0, state)
	case 0x23: // End
		return t.navigateToRow(len(t.Rows)-1, state)
	case 0x22: // Page Down
		return t.navigateToRow(minValueInt(len(t.Rows)-1, index+pageRows), state)
	case 0x21: // Page Up
		return t.navigateToRow(maxInt(0, index-pageRows), state)
	case 13, 32: // Enter or Space
		return t.selectRow(index, state)
	}
	return false
}
func (t *DataTable) OnMouseWheel(pt image.Point, delta int, state *types.ApplicationState) bool {
	if !pt.In(t.Rect) {
		return false
	}
	if delta > 0 {
		t.ScrollY -= t.rowHeight() * 3
	} else {
		t.ScrollY += t.rowHeight() * 3
	}
	if t.ScrollY < 0 {
		t.ScrollY = 0
	}
	if t.ScrollY > t.maxScroll() {
		t.ScrollY = t.maxScroll()
	}
	t.storeScroll(state)
	return true
}
func (t *DataTable) Walk(fn func(types.Component)) { fn(t) }
func (t *DataTable) Semantics(state *types.ApplicationState) semantics.Node {
	maxScroll := t.maxScroll()
	scrollable := maxScroll > 0
	scrollPercent := -1.0
	viewSize := 100.0
	if scrollable {
		scrollPercent = float64(t.ScrollY) * 100 / float64(maxScroll)
		contentHeight := len(t.Rows) * t.rowHeight()
		viewSize = float64(t.Rect.Dy()-t.headerHeight()) * 100 / float64(contentHeight)
	}
	focused := state != nil && state.FocusedID == t.CompID
	activeRow := t.activeRowIndex(state)
	node := semantics.Node{ID: t.CompID, Role: semantics.RoleTable, Name: t.CompID, Bounds: t.Rect,
		State:      semantics.State{Focused: focused && activeRow < 0},
		Actions:    []semantics.Action{semantics.ActionFocus},
		Collection: &semantics.CollectionValue{Selectable: true},
		Grid:       &semantics.GridValue{Rows: len(t.Rows), Columns: len(t.Columns)},
		Scroll:     &semantics.ScrollValue{VerticallyScrollable: scrollable, HorizontalPercent: -1, VerticalPercent: scrollPercent, HorizontalViewSize: 100, VerticalViewSize: viewSize}}
	if scrollable {
		node.Actions = append(node.Actions, semantics.ActionSetScroll)
	}
	widths := t.columnWidths()
	headerBounds := image.Rect(t.Rect.Min.X, t.Rect.Min.Y, t.Rect.Max.X, t.Rect.Min.Y+t.headerHeight())
	header := semantics.Node{ID: t.CompID + "/header", Role: semantics.RoleRow, Name: "Columns", Bounds: headerBounds}
	x := headerBounds.Min.X
	for columnIndex, column := range t.Columns {
		width := 0
		if columnIndex < len(widths) {
			width = widths[columnIndex]
		}
		cellBounds := image.Rect(x, headerBounds.Min.Y, x+width, headerBounds.Max.Y)
		header.Children = append(header.Children, semantics.Node{ID: t.CompID + "/header/" + column.Key, Role: semantics.RoleColumnHeader, Name: column.Label, Bounds: cellBounds})
		x += width
	}
	node.Children = append(node.Children, header)
	bodyBounds := image.Rect(t.Rect.Min.X, headerBounds.Max.Y, t.Rect.Max.X, t.Rect.Max.Y)
	for rowIndex, row := range t.Rows {
		top := bodyBounds.Min.Y + rowIndex*t.rowHeight() - t.ScrollY
		rowBounds := image.Rect(bodyBounds.Min.X, top, bodyBounds.Max.X, top+t.rowHeight())
		offscreen := rowBounds.Max.Y <= bodyBounds.Min.Y || rowBounds.Min.Y >= bodyBounds.Max.Y
		rowNode := semantics.Node{ID: t.CompID + "/row/" + row.ID, Role: semantics.RoleRow, Name: row.ID, Bounds: rowBounds,
			State: semantics.State{Selected: row.ID == t.SelectedID, Focused: focused && rowIndex == activeRow, Offscreen: offscreen}, Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionSelect},
			GridItem: &semantics.GridItemValue{Row: rowIndex, Column: 0, RowSpan: 1, ColumnSpan: maxInt(1, len(t.Columns))}}
		x = rowBounds.Min.X
		for columnIndex, column := range t.Columns {
			width := 0
			if columnIndex < len(widths) {
				width = widths[columnIndex]
			}
			cellBounds := image.Rect(x, rowBounds.Min.Y, x+width, rowBounds.Max.Y)
			rowNode.Children = append(rowNode.Children, semantics.Node{ID: t.CompID + "/row/" + row.ID + "/" + column.Key, Role: semantics.RoleCell, Name: column.Label, Value: row.Values[column.Key], Bounds: cellBounds,
				State: semantics.State{Offscreen: offscreen, ReadOnly: true}, GridItem: &semantics.GridItemValue{Row: rowIndex, Column: columnIndex, RowSpan: 1, ColumnSpan: 1}})
			x += width
		}
		node.Children = append(node.Children, rowNode)
	}
	return node
}
func (t *DataTable) PerformSemanticAction(targetID string, action semantics.Action, value string, state *types.ApplicationState) bool {
	if targetID == t.CompID && action == semantics.ActionFocus {
		state.FocusedID = t.CompID
		if index := t.activeRowIndex(state); index >= 0 {
			t.storeActiveRow(index, state)
			t.ensureRowVisible(index, state)
		}
		return true
	}
	if targetID == t.CompID && action == semantics.ActionSetScroll {
		var horizontal, vertical float64
		if _, err := fmt.Sscanf(value, "%f:%f", &horizontal, &vertical); err != nil || vertical < 0 || t.maxScroll() <= 0 {
			return false
		}
		if vertical > 100 {
			vertical = 100
		}
		t.ScrollY = int(math.Round(vertical * float64(t.maxScroll()) / 100))
		t.storeScroll(state)
		return true
	}
	for index, row := range t.Rows {
		if targetID == t.CompID+"/row/"+row.ID {
			switch action {
			case semantics.ActionFocus:
				state.FocusedID = t.CompID
				t.storeActiveRow(index, state)
				t.ensureRowVisible(index, state)
				return true
			case semantics.ActionSelect:
				return t.selectRow(index, state)
			}
			return false
		}
	}
	return false
}

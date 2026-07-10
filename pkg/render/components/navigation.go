package components

import (
	"image"
	"strings"

	"go_native_gpu_gui/pkg/render/semantics"
	renderstate "go_native_gpu_gui/pkg/render/state"
	"go_native_gpu_gui/pkg/render/types"
)

// BreadcrumbItem is one destination in a breadcrumb trail. The final item is
// normally the current location and therefore has no activation callback.
type BreadcrumbItem struct {
	ID       string
	Label    string
	Disabled bool
	OnInvoke func(*types.ApplicationState)
}

type Breadcrumbs struct {
	CompID string
	Rect   image.Rectangle
	Items  []BreadcrumbItem
}

func NewBreadcrumbs(id string, items []BreadcrumbItem) *Breadcrumbs {
	return &Breadcrumbs{CompID: id, Items: items}
}
func (b *Breadcrumbs) ID() string                    { return b.CompID }
func (b *Breadcrumbs) GetID() string                 { return b.CompID }
func (b *Breadcrumbs) Bounds() image.Rectangle       { return b.Rect }
func (b *Breadcrumbs) SetBounds(r image.Rectangle)   { b.Rect = r }
func (b *Breadcrumbs) Focusable() bool               { return false }
func (b *Breadcrumbs) Walk(fn func(types.Component)) { fn(b) }
func (b *Breadcrumbs) Measure(_ image.Point, state *types.ApplicationState) types.MeasureResult {
	t := activeTheme(state)
	cw := fontCharWidth(state)
	width := 0
	for i, item := range b.Items {
		width += len([]rune(item.Label))*cw + 2*t.Spacing.SM
		if i+1 < len(b.Items) {
			width += cw
		}
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(b.Rect), image.Pt(width, t.Controls.Small)), Min: image.Pt(t.Controls.Small, t.Controls.Small)}
}
func fontCharWidth(state *types.ApplicationState) int {
	if state != nil && state.FontCharWidth > 0 {
		return state.FontCharWidth
	}
	return 8
}
func (b *Breadcrumbs) itemRects(state *types.ApplicationState) []image.Rectangle {
	t := activeTheme(state)
	cw := fontCharWidth(state)
	rects := make([]image.Rectangle, len(b.Items))
	x := b.Rect.Min.X
	for i, item := range b.Items {
		width := len([]rune(item.Label))*cw + 2*t.Spacing.SM
		rects[i] = image.Rect(x, b.Rect.Min.Y, x+width, b.Rect.Max.Y)
		x += width + cw
	}
	return rects
}
func (b *Breadcrumbs) Draw(p types.Painter, state *types.ApplicationState) {
	t := activeTheme(state)
	rects := b.itemRects(state)
	for i, item := range b.Items {
		col := t.Colors.TextMuted
		if i == len(b.Items)-1 {
			col = t.Colors.Text
		} else if !item.Disabled && state != nil && state.HoveredID == b.CompID+"/"+item.ID {
			col = t.Colors.Accent
		}
		p.DrawText(item.Label, rects[i].Min.X+t.Spacing.SM, rects[i].Min.Y+rects[i].Dy()/2+5, col)
		if i+1 < len(b.Items) {
			p.DrawText(">", rects[i].Max.X, rects[i].Min.Y+rects[i].Dy()/2+5, t.Colors.TextDisabled)
		}
	}
}
func (b *Breadcrumbs) HitTest(pt image.Point) string {
	for i, r := range b.itemRects(nil) {
		if pt.In(r) && i < len(b.Items) && b.Items[i].OnInvoke != nil && !b.Items[i].Disabled {
			return b.CompID + "/" + b.Items[i].ID
		}
	}
	return ""
}
func (b *Breadcrumbs) OnKey(uint32, rune, *types.ApplicationState) bool { return false }
func (b *Breadcrumbs) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	id := b.HitTest(pt)
	if id == "" {
		return false
	}
	state.ActiveID = id
	return true
}
func (b *Breadcrumbs) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	id := b.HitTest(pt)
	if id == "" || state.ActiveID != id {
		return false
	}
	return b.PerformSemanticAction(id, semantics.ActionInvoke, "", state)
}
func (b *Breadcrumbs) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (b *Breadcrumbs) Semantics(state *types.ApplicationState) semantics.Node {
	node := semantics.Node{ID: b.CompID, Role: semantics.RoleGroup, Name: "Breadcrumbs", Bounds: b.Rect}
	rects := b.itemRects(state)
	for i, item := range b.Items {
		actions := []semantics.Action(nil)
		if item.OnInvoke != nil && !item.Disabled {
			actions = []semantics.Action{semantics.ActionInvoke}
		}
		node.Children = append(node.Children, semantics.Node{ID: b.CompID + "/" + item.ID, Role: semantics.RoleLink, Name: item.Label, Bounds: rects[i], State: semantics.State{Disabled: item.Disabled, Selected: i == len(b.Items)-1}, Actions: actions})
	}
	return node
}
func (b *Breadcrumbs) PerformSemanticAction(targetID string, _ semantics.Action, _ string, state *types.ApplicationState) bool {
	id := strings.TrimPrefix(targetID, b.CompID+"/")
	if id == targetID {
		return false
	}
	for _, item := range b.Items {
		if item.ID == id && !item.Disabled && item.OnInvoke != nil {
			item.OnInvoke(state)
			return true
		}
	}
	return false
}

type TreeNode struct {
	ID       string
	Label    string
	Disabled bool
	Children []TreeNode
}

type treeRow struct {
	node  *TreeNode
	depth int
}

type treeInteraction struct {
	ActiveID string
}

// Tree is a keyboard-navigable hierarchical selector. Selection and expanded
// IDs are controlled values; callbacks publish requested changes to the app.
type Tree struct {
	CompID     string
	Rect       image.Rectangle
	Nodes      []TreeNode
	SelectedID string
	Expanded   map[string]bool
	OnSelect   func(string, *types.ApplicationState)
	OnToggle   func(string, bool, *types.ApplicationState)
	rows       []treeRow
}

func NewTree(id string, nodes []TreeNode, selected string, expanded map[string]bool, onSelect func(string, *types.ApplicationState)) *Tree {
	if expanded == nil {
		expanded = make(map[string]bool)
	}
	return &Tree{CompID: id, Nodes: nodes, SelectedID: selected, Expanded: expanded, OnSelect: onSelect}
}
func (t *Tree) ID() string                  { return t.CompID }
func (t *Tree) GetID() string               { return t.CompID }
func (t *Tree) Bounds() image.Rectangle     { return t.Rect }
func (t *Tree) SetBounds(r image.Rectangle) { t.Rect = r }
func (t *Tree) Focusable() bool {
	for _, row := range t.visibleRows() {
		if !row.node.Disabled {
			return true
		}
	}
	return false
}
func (t *Tree) Walk(fn func(types.Component)) { fn(t) }
func (t *Tree) interactionKey() string        { return t.CompID + "/tree" }
func (t *Tree) activeIndex(state *types.ApplicationState, rows []treeRow) int {
	activeID := ""
	if state != nil && state.TransientState != nil {
		if interaction, ok := renderstate.Load[treeInteraction](state.TransientState, t.interactionKey()); ok {
			activeID = interaction.ActiveID
		}
	}
	if activeID == "" {
		activeID = t.SelectedID
	}
	for index, row := range rows {
		if row.node.ID == activeID && !row.node.Disabled {
			return index
		}
	}
	for index, row := range rows {
		if !row.node.Disabled {
			return index
		}
	}
	return -1
}
func (t *Tree) storeActive(id string, state *types.ApplicationState) {
	if state == nil || id == "" {
		return
	}
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, t.interactionKey(), treeInteraction{ActiveID: id})
}
func (t *Tree) rowHeight(state *types.ApplicationState) int {
	return activeTheme(state).Controls.Medium
}
func (t *Tree) visibleRows() []treeRow {
	t.rows = t.rows[:0]
	var appendNodes func([]TreeNode, int)
	appendNodes = func(nodes []TreeNode, depth int) {
		for i := range nodes {
			node := &nodes[i]
			t.rows = append(t.rows, treeRow{node: node, depth: depth})
			if len(node.Children) > 0 && t.Expanded[node.ID] {
				appendNodes(node.Children, depth+1)
			}
		}
	}
	appendNodes(t.Nodes, 0)
	return t.rows
}
func (t *Tree) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	rows := t.visibleRows()
	width := avail.X
	if width <= 0 {
		width = 220
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(t.Rect), image.Pt(width, len(rows)*t.rowHeight(state))), Min: image.Pt(140, t.rowHeight(state))}
}
func (t *Tree) rowRect(index int, state *types.ApplicationState) image.Rectangle {
	h := t.rowHeight(state)
	return image.Rect(t.Rect.Min.X, t.Rect.Min.Y+index*h, t.Rect.Max.X, t.Rect.Min.Y+(index+1)*h)
}
func (t *Tree) Draw(p types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	rows := t.visibleRows()
	active := t.activeIndex(state, rows)
	for i, row := range rows {
		r := t.rowRect(i, state)
		id := t.CompID + "/" + row.node.ID
		visual := buttonVisual(th, 0, state != nil && state.HoveredID == id, state != nil && state.ActiveID == id, row.node.Disabled, row.node.ID == t.SelectedID, nil)
		visual.border = colorZero()
		focused := state != nil && state.FocusedID == t.CompID && i == active
		if row.node.ID == t.SelectedID || focused || state != nil && state.HoveredID == id {
			drawControlSurface(p, r, visual, focused)
		}
		x := r.Min.X + th.Spacing.SM + row.depth*th.Spacing.LG
		if len(row.node.Children) > 0 {
			marker := ">"
			if t.Expanded[row.node.ID] {
				marker = "v"
			}
			p.DrawText(marker, x, r.Min.Y+r.Dy()/2+5, th.Colors.TextMuted)
			x += th.Spacing.LG
		}
		p.DrawText(row.node.Label, x, r.Min.Y+r.Dy()/2+5, visual.foreground)
	}
}
func (t *Tree) HitTest(pt image.Point) string {
	if !pt.In(t.Rect) {
		return ""
	}
	rows := t.visibleRows()
	index := (pt.Y - t.Rect.Min.Y) / t.rowHeight(nil)
	if index < 0 || index >= len(rows) {
		return t.CompID
	}
	return t.CompID + "/" + rows[index].node.ID
}
func (t *Tree) selectNode(id string, state *types.ApplicationState) bool {
	for _, row := range t.visibleRows() {
		if row.node.ID == id && !row.node.Disabled {
			t.storeActive(id, state)
			if t.OnSelect != nil {
				t.OnSelect(id, state)
			} else {
				t.SelectedID = id
			}
			return true
		}
	}
	return false
}
func (t *Tree) focusNode(id string, state *types.ApplicationState) bool {
	for _, row := range t.visibleRows() {
		if row.node.ID == id && !row.node.Disabled {
			state.FocusedID = t.CompID
			t.storeActive(id, state)
			return true
		}
	}
	return false
}
func (t *Tree) toggleNode(id string, state *types.ApplicationState) bool {
	for _, row := range t.visibleRows() {
		if row.node.ID == id && len(row.node.Children) > 0 && !row.node.Disabled {
			next := !t.Expanded[id]
			if t.OnToggle != nil {
				t.OnToggle(id, next, state)
			} else {
				t.Expanded[id] = next
			}
			return true
		}
	}
	return false
}
func (t *Tree) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state == nil || state.FocusedID != t.CompID {
		return false
	}
	rows := t.visibleRows()
	if len(rows) == 0 {
		return false
	}
	index := t.activeIndex(state, rows)
	if index < 0 {
		return false
	}
	switch key {
	case 0x28: // down
		for i := index + 1; i < len(rows); i++ {
			if !rows[i].node.Disabled {
				return t.selectNode(rows[i].node.ID, state)
			}
		}
	case 0x26: // up
		for i := index - 1; i >= 0; i-- {
			if !rows[i].node.Disabled {
				return t.selectNode(rows[i].node.ID, state)
			}
		}
	case 0x27: // right
		if len(rows[index].node.Children) > 0 && !t.Expanded[rows[index].node.ID] {
			return t.toggleNode(rows[index].node.ID, state)
		}
		for candidate := index + 1; candidate < len(rows) && rows[candidate].depth > rows[index].depth; candidate++ {
			if rows[candidate].depth == rows[index].depth+1 && !rows[candidate].node.Disabled {
				return t.selectNode(rows[candidate].node.ID, state)
			}
		}
	case 0x25: // left
		if t.Expanded[rows[index].node.ID] {
			return t.toggleNode(rows[index].node.ID, state)
		}
		if rows[index].depth > 0 {
			for i := index - 1; i >= 0; i-- {
				if rows[i].depth == rows[index].depth-1 {
					return t.selectNode(rows[i].node.ID, state)
				}
			}
		}
	case 13, 32:
		if t.toggleNode(rows[index].node.ID, state) {
			return true
		}
		return t.selectNode(rows[index].node.ID, state)
	case 0x24: // Home
		for _, row := range rows {
			if !row.node.Disabled {
				return t.selectNode(row.node.ID, state)
			}
		}
	case 0x23: // End
		for i := len(rows) - 1; i >= 0; i-- {
			if !rows[i].node.Disabled {
				return t.selectNode(rows[i].node.ID, state)
			}
		}
	}
	if char != 0 {
		needle := strings.ToLower(string(char))
		for offset := 1; offset <= len(rows); offset++ {
			candidate := (index + offset) % len(rows)
			if !rows[candidate].node.Disabled && strings.HasPrefix(strings.ToLower(rows[candidate].node.Label), needle) {
				return t.selectNode(rows[candidate].node.ID, state)
			}
		}
	}
	return false
}
func (t *Tree) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	id := t.HitTest(pt)
	if id == "" || id == t.CompID {
		return false
	}
	if !t.focusNode(strings.TrimPrefix(id, t.CompID+"/"), state) {
		return false
	}
	state.ActiveID = id
	return true
}
func (t *Tree) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	id := t.HitTest(pt)
	if id == "" || state.ActiveID != id {
		return false
	}
	nodeID := strings.TrimPrefix(id, t.CompID+"/")
	rows := t.visibleRows()
	for i, row := range rows {
		if row.node.ID != nodeID {
			continue
		}
		r := t.rowRect(i, state)
		indentEnd := r.Min.X + activeTheme(state).Spacing.SM + row.depth*activeTheme(state).Spacing.LG + activeTheme(state).Spacing.LG
		if len(row.node.Children) > 0 && pt.X <= indentEnd && t.toggleNode(nodeID, state) {
			return true
		}
		return t.selectNode(nodeID, state)
	}
	return false
}
func (t *Tree) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (t *Tree) Semantics(state *types.ApplicationState) semantics.Node {
	rows := t.visibleRows()
	active := t.activeIndex(state, rows)
	activeID := ""
	if active >= 0 {
		activeID = rows[active].node.ID
	}
	focused := state != nil && state.FocusedID == t.CompID
	actions := []semantics.Action(nil)
	if active >= 0 {
		actions = []semantics.Action{semantics.ActionFocus}
	}
	root := semantics.Node{ID: t.CompID, Role: semantics.RoleTree, Name: "Tree", Bounds: t.Rect, State: semantics.State{Focused: focused && active < 0}, Actions: actions,
		Collection: &semantics.CollectionValue{Selectable: true}}
	rowIndex := 0
	var buildNodes func([]TreeNode) []semantics.Node
	buildNodes = func(nodes []TreeNode) []semantics.Node {
		result := make([]semantics.Node, 0, len(nodes))
		for _, item := range nodes {
			index := rowIndex
			rowIndex++
			itemActions := []semantics.Action(nil)
			if !item.Disabled {
				itemActions = append(itemActions, semantics.ActionFocus, semantics.ActionSelect)
				if len(item.Children) > 0 {
					if t.Expanded[item.ID] {
						itemActions = append(itemActions, semantics.ActionCollapse)
					} else {
						itemActions = append(itemActions, semantics.ActionExpand)
					}
				}
			}
			node := semantics.Node{ID: t.CompID + "/" + item.ID, Role: semantics.RoleTreeItem, Name: item.Label, Bounds: t.rowRect(index, state), State: semantics.State{Disabled: item.Disabled, Selected: item.ID == t.SelectedID, Focused: focused && item.ID == activeID, Expanded: t.Expanded[item.ID]}, Actions: itemActions}
			if len(item.Children) > 0 && t.Expanded[item.ID] {
				node.Children = buildNodes(item.Children)
			}
			result = append(result, node)
		}
		return result
	}
	root.Children = buildNodes(t.Nodes)
	return root
}
func (t *Tree) PerformSemanticAction(targetID string, action semantics.Action, _ string, state *types.ApplicationState) bool {
	if targetID == t.CompID && action == semantics.ActionFocus {
		rows := t.visibleRows()
		active := t.activeIndex(state, rows)
		if active < 0 {
			return false
		}
		return t.focusNode(rows[active].node.ID, state)
	}
	id := strings.TrimPrefix(targetID, t.CompID+"/")
	if id == targetID {
		return false
	}
	switch action {
	case semantics.ActionFocus:
		return t.focusNode(id, state)
	case semantics.ActionSelect:
		return t.selectNode(id, state)
	case semantics.ActionExpand:
		if !t.Expanded[id] {
			return t.toggleNode(id, state)
		}
		return true
	case semantics.ActionCollapse:
		if t.Expanded[id] {
			return t.toggleNode(id, state)
		}
		return true
	}
	return false
}

type paginationItem struct {
	id       string
	label    string
	page     int
	disabled bool
	bounds   image.Rectangle
}

type paginationInteraction struct {
	ActivePage int
}

// Pagination exposes a compact window of pages with previous/next actions.
// Page is one-based and application-owned when OnChange is provided.
type Pagination struct {
	CompID     string
	Rect       image.Rectangle
	Page       int
	PageCount  int
	MaxButtons int
	Disabled   bool
	OnChange   func(int, *types.ApplicationState)
	items      []paginationItem
}

func NewPagination(id string, page, pageCount int, onChange func(int, *types.ApplicationState)) *Pagination {
	return &Pagination{CompID: id, Page: page, PageCount: pageCount, MaxButtons: 7, OnChange: onChange}
}
func (p *Pagination) ID() string                    { return p.CompID }
func (p *Pagination) GetID() string                 { return p.CompID }
func (p *Pagination) Bounds() image.Rectangle       { return p.Rect }
func (p *Pagination) SetBounds(r image.Rectangle)   { p.Rect = r }
func (p *Pagination) Focusable() bool               { return !p.Disabled && p.PageCount > 0 }
func (p *Pagination) Walk(fn func(types.Component)) { fn(p) }
func (p *Pagination) interactionKey() string        { return p.CompID + "/pagination" }
func (p *Pagination) activePage(state *types.ApplicationState) int {
	if state != nil && state.TransientState != nil {
		if interaction, ok := renderstate.Load[paginationInteraction](state.TransientState, p.interactionKey()); ok && interaction.ActivePage >= 1 && interaction.ActivePage <= p.PageCount {
			return interaction.ActivePage
		}
	}
	return p.normalizedPage()
}
func (p *Pagination) storeActive(page int, state *types.ApplicationState) {
	if state == nil || page < 1 || page > p.PageCount {
		return
	}
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, p.interactionKey(), paginationInteraction{ActivePage: page})
}
func (p *Pagination) normalizedPage() int {
	if p.PageCount <= 0 {
		return 0
	}
	if p.Page < 1 {
		return 1
	}
	if p.Page > p.PageCount {
		return p.PageCount
	}
	return p.Page
}
func (p *Pagination) rebuildItems(state *types.ApplicationState) []paginationItem {
	p.items = p.items[:0]
	if p.PageCount <= 0 {
		return p.items
	}
	page := p.activePage(state)
	maxButtons := p.MaxButtons
	if maxButtons <= 0 {
		maxButtons = 7
	}
	if maxButtons > p.PageCount {
		maxButtons = p.PageCount
	}
	start := page - maxButtons/2
	if start < 1 {
		start = 1
	}
	end := start + maxButtons - 1
	if end > p.PageCount {
		end = p.PageCount
		start = end - maxButtons + 1
	}
	p.items = append(p.items, paginationItem{id: "previous", label: "<", page: page - 1, disabled: p.Disabled || page <= 1})
	for candidate := start; candidate <= end; candidate++ {
		p.items = append(p.items, paginationItem{id: "page/" + intString(candidate), label: intString(candidate), page: candidate, disabled: p.Disabled})
	}
	p.items = append(p.items, paginationItem{id: "next", label: ">", page: page + 1, disabled: p.Disabled || page >= p.PageCount})
	th := activeTheme(state)
	size := th.Controls.Medium
	x := p.Rect.Min.X
	for i := range p.items {
		p.items[i].bounds = image.Rect(x, p.Rect.Min.Y, x+size, p.Rect.Min.Y+size)
		x += size + th.Spacing.XS
	}
	return p.items
}
func intString(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}
func (p *Pagination) Measure(_ image.Point, state *types.ApplicationState) types.MeasureResult {
	th := activeTheme(state)
	count := p.MaxButtons
	if count <= 0 {
		count = 7
	}
	if p.PageCount < count {
		count = p.PageCount
	}
	if count < 0 {
		count = 0
	}
	width := (count+2)*th.Controls.Medium + (count+1)*th.Spacing.XS
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(p.Rect), image.Pt(width, th.Controls.Medium)), Min: image.Pt(2*th.Controls.Medium+th.Spacing.XS, th.Controls.Medium)}
}
func (p *Pagination) Draw(painter types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	current := p.normalizedPage()
	active := p.activePage(state)
	for _, item := range p.rebuildItems(state) {
		id := p.CompID + "/" + item.id
		visual := buttonVisual(th, 0, state != nil && state.HoveredID == id, state != nil && state.ActiveID == id, item.disabled, item.page == current && strings.HasPrefix(item.id, "page/"), nil)
		focused := state != nil && state.FocusedID == p.CompID && strings.HasPrefix(item.id, "page/") && item.page == active
		drawControlSurface(painter, item.bounds, visual, focused)
		cw := fontCharWidth(state)
		painter.DrawText(item.label, item.bounds.Min.X+(item.bounds.Dx()-len([]rune(item.label))*cw)/2, item.bounds.Min.Y+item.bounds.Dy()/2+5, visual.foreground)
	}
}
func (p *Pagination) HitTest(pt image.Point) string {
	items := p.items
	if len(items) == 0 {
		items = p.rebuildItems(nil)
	}
	for _, item := range items {
		if pt.In(item.bounds) {
			return p.CompID + "/" + item.id
		}
	}
	return ""
}
func (p *Pagination) request(page int, state *types.ApplicationState) bool {
	if p.Disabled || page < 1 || page > p.PageCount {
		return false
	}
	p.storeActive(page, state)
	if page == p.normalizedPage() {
		return true
	}
	if p.OnChange != nil {
		p.OnChange(page, state)
	} else {
		p.Page = page
	}
	return true
}
func (p *Pagination) OnKey(key uint32, _ rune, state *types.ApplicationState) bool {
	if state == nil || state.FocusedID != p.CompID || !p.Focusable() {
		return false
	}
	active := p.activePage(state)
	switch key {
	case 0x25:
		return p.request(maxInt(1, active-1), state)
	case 0x27:
		return p.request(minValueInt(p.PageCount, active+1), state)
	case 0x24:
		return p.request(1, state)
	case 0x23:
		return p.request(p.PageCount, state)
	case 13, 32:
		return p.request(active, state)
	}
	return false
}
func (p *Pagination) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	id := p.HitTest(pt)
	if id == "" {
		return false
	}
	for _, item := range p.items {
		if id == p.CompID+"/"+item.id && !item.disabled {
			state.FocusedID = p.CompID
			p.storeActive(item.page, state)
			state.ActiveID = id
			return true
		}
	}
	return false
}
func (p *Pagination) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	id := p.HitTest(pt)
	if id == "" || state.ActiveID != id {
		return false
	}
	return p.PerformSemanticAction(id, semantics.ActionInvoke, "", state)
}
func (p *Pagination) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (p *Pagination) Semantics(state *types.ApplicationState) semantics.Node {
	active := p.activePage(state)
	focused := state != nil && state.FocusedID == p.CompID
	actions := []semantics.Action(nil)
	if p.Focusable() {
		actions = []semantics.Action{semantics.ActionFocus}
	}
	node := semantics.Node{ID: p.CompID, Role: semantics.RoleNavigation, Name: "Pagination", Value: intString(p.normalizedPage()), Bounds: p.Rect, State: semantics.State{Disabled: p.Disabled, Focused: focused && active == 0}, Actions: actions}
	current := p.normalizedPage()
	for _, item := range p.rebuildItems(state) {
		itemActions := []semantics.Action(nil)
		if !item.disabled {
			itemActions = []semantics.Action{semantics.ActionInvoke}
			if strings.HasPrefix(item.id, "page/") {
				itemActions = append([]semantics.Action{semantics.ActionFocus}, itemActions...)
			}
		}
		node.Children = append(node.Children, semantics.Node{ID: p.CompID + "/" + item.id, Role: semantics.RoleButton, Name: item.label, Value: intString(item.page), Bounds: item.bounds, State: semantics.State{Disabled: item.disabled, Selected: item.page == current && strings.HasPrefix(item.id, "page/"), Focused: focused && item.page == active && strings.HasPrefix(item.id, "page/")}, Actions: itemActions})
	}
	return node
}
func (p *Pagination) PerformSemanticAction(targetID string, action semantics.Action, _ string, state *types.ApplicationState) bool {
	if targetID == p.CompID && action == semantics.ActionFocus {
		if !p.Focusable() {
			return false
		}
		state.FocusedID = p.CompID
		p.storeActive(p.activePage(state), state)
		return true
	}
	id := strings.TrimPrefix(targetID, p.CompID+"/")
	if id == targetID {
		return false
	}
	for _, item := range p.rebuildItems(state) {
		if item.id == id && !item.disabled {
			switch action {
			case semantics.ActionFocus:
				if !strings.HasPrefix(item.id, "page/") {
					return false
				}
				state.FocusedID = p.CompID
				p.storeActive(item.page, state)
				return true
			case semantics.ActionInvoke:
				return p.request(item.page, state)
			}
			return false
		}
	}
	return false
}

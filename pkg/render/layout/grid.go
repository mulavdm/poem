package layout

import (
	"image"

	"go_native_gpu_gui/pkg/render/types"
)

type GridLengthType int

const (
	Pixel GridLengthType = iota // Fixed pixel size
	Auto                        // Size to contents
	Star                        // Fractional weight of remaining space
)

type GridLength struct {
	Type  GridLengthType
	Value float32
}

// GridLength helpers
func NewPixel(val float32) GridLength   { return GridLength{Type: Pixel, Value: val} }
func NewAuto() GridLength               { return GridLength{Type: Auto} }
func NewStar(weight float32) GridLength { return GridLength{Type: Star, Value: weight} }

type GridAlignment int

const (
	Start GridAlignment = iota
	Center
	End
	Stretch
)

type GridChild struct {
	Row        int
	Col        int
	RowSpan    int
	ColSpan    int
	Horizontal GridAlignment
	Vertical   GridAlignment
	Child      types.Component
}

// Grid arranges child components in rows and columns
type Grid struct {
	CompID   string
	Rect     image.Rectangle
	Rows     []GridLength
	Columns  []GridLength
	Padding  int
	RowGap   int
	ColGap   int
	Children []GridChild
}

func (g *Grid) ID() string    { return g.CompID }
func (g *Grid) GetID() string { return g.CompID }

func (g *Grid) Bounds() image.Rectangle {
	if g.Rect.Dx() == 0 || g.Rect.Dy() == 0 {
		// Calculate preferred size based on layout definitions if no bounds are explicitly set
		size := g.measure(image.Point{}, nil).Preferred
		w := size.X
		h := size.Y
		return image.Rect(g.Rect.Min.X, g.Rect.Min.Y, g.Rect.Min.X+w, g.Rect.Min.Y+h)
	}
	return g.Rect
}

func (g *Grid) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	if g.Rect.Dx() > 0 || g.Rect.Dy() > 0 {
		measured := g.measure(avail, state).Preferred
		if g.Rect.Dx() > 0 {
			measured.X = g.Rect.Dx()
		}
		if g.Rect.Dy() > 0 {
			measured.Y = g.Rect.Dy()
		}
		return types.MeasureResult{Preferred: measured, Min: measured}
	}
	size := g.measure(avail, state).Preferred
	return types.MeasureResult{Preferred: size, Min: size}
}

func (g *Grid) ContentSize(avail image.Point, state *types.ApplicationState) image.Point {
	return g.measure(avail, state).Preferred
}

func (g *Grid) SetBounds(r image.Rectangle) {
	g.Rect = r
	g.performLayout(nil)
}

func (g *Grid) Draw(p types.Painter, state *types.ApplicationState) {
	g.performLayout(state)
	for _, child := range g.Children {
		if child.Child != nil {
			child.Child.Draw(p, state)
		}
	}
}

func (g *Grid) HitTest(pt image.Point) string {
	if !pt.In(g.Rect) {
		return ""
	}
	// Check children in reverse order
	for i := len(g.Children) - 1; i >= 0; i-- {
		child := g.Children[i]
		if child.Child != nil {
			if id := child.Child.HitTest(pt); id != "" {
				return id
			}
		}
	}
	return g.CompID
}

func (g *Grid) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID == "" {
		return false
	}
	for _, child := range g.Children {
		if child.Child != nil && child.Child.OnKey(key, char, state) {
			return true
		}
	}
	return false
}

func (g *Grid) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	for i := len(g.Children) - 1; i >= 0; i-- {
		child := g.Children[i]
		if child.Child != nil && pt.In(child.Child.Bounds()) {
			if child.Child.OnMouseDown(pt, state) {
				return true
			}
		}
	}
	return false
}

func (g *Grid) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	for i := len(g.Children) - 1; i >= 0; i-- {
		child := g.Children[i]
		if child.Child != nil && child.Child.OnMouseUp(pt, state) {
			return true
		}
	}
	return false
}

func (g *Grid) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	for i := len(g.Children) - 1; i >= 0; i-- {
		child := g.Children[i]
		if child.Child != nil && pt.In(child.Child.Bounds()) {
			if child.Child.OnMouseMove(pt, state) {
				return true
			}
		}
	}
	return false
}

func (g *Grid) Focusable() bool { return false }

func (g *Grid) Walk(fn func(types.Component)) {
	fn(g)
	for _, child := range g.Children {
		if child.Child != nil {
			child.Child.Walk(fn)
		}
	}
}

func (g *Grid) ChildComponents() []types.Component {
	children := make([]types.Component, 0, len(g.Children))
	for _, child := range g.Children {
		if child.Child != nil {
			children = append(children, child.Child)
		}
	}
	return children
}

func (g *Grid) measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	// If Grid dimensions are explicitly defined, use them as constraint limits
	if g.Rect.Dx() > 0 && g.Rect.Dy() > 0 {
		avail = image.Pt(g.Rect.Dx(), g.Rect.Dy())
	}

	numCols := len(g.Columns)
	if numCols == 0 {
		numCols = 1
	}
	numRows := len(g.Rows)
	if numRows == 0 {
		numRows = 1
	}

	colWidths := make([]int, numCols)
	rowHeights := make([]int, numRows)

	// Step 1: Initialize fixed sizes
	for i, col := range g.Columns {
		if col.Type == Pixel {
			colWidths[i] = int(col.Value)
		}
	}
	for i, row := range g.Rows {
		if row.Type == Pixel {
			rowHeights[i] = int(row.Value)
		}
	}

	// Step 2: Measure Auto columns/rows (ignoring spans for simplicity during sizing calculations)
	for _, child := range g.Children {
		if child.Child == nil {
			continue
		}
		cSpan := child.ColSpan
		if cSpan <= 0 {
			cSpan = 1
		}
		rSpan := child.RowSpan
		if rSpan <= 0 {
			rSpan = 1
		}

		// Row/col range safety checks
		cIdx := child.Col
		if cIdx < 0 {
			cIdx = 0
		}
		if cIdx >= numCols {
			cIdx = numCols - 1
		}
		rIdx := child.Row
		if rIdx < 0 {
			rIdx = 0
		}
		if rIdx >= numRows {
			rIdx = numRows - 1
		}

		childAvail := image.Point{} // unconstrained
		measurement := types.MeasureComponent(child.Child, childAvail, state)

		if cSpan == 1 && len(g.Columns) > 0 && g.Columns[cIdx].Type == Auto {
			if measurement.Preferred.X > colWidths[cIdx] {
				colWidths[cIdx] = measurement.Preferred.X
			}
		}
		if rSpan == 1 && len(g.Rows) > 0 && g.Rows[rIdx].Type == Auto {
			if measurement.Preferred.Y > rowHeights[rIdx] {
				rowHeights[rIdx] = measurement.Preferred.Y
			}
		}
	}

	// Step 3: Handle Star columns/rows by evaluating remaining available bounds
	totalGapsW := g.ColGap * (numCols - 1)
	totalGapsH := g.RowGap * (numRows - 1)
	clientW := avail.X - (2 * g.Padding) - totalGapsW
	clientH := avail.Y - (2 * g.Padding) - totalGapsH
	if clientW < 0 {
		clientW = 0
	}
	if clientH < 0 {
		clientH = 0
	}

	// Sum up non-star sizes
	fixedAutoW := 0
	for _, w := range colWidths {
		fixedAutoW += w
	}
	fixedAutoH := 0
	for _, h := range rowHeights {
		fixedAutoH += h
	}

	starWeightW := float32(0)
	for _, col := range g.Columns {
		if col.Type == Star {
			starWeightW += col.Value
		}
	}
	starWeightH := float32(0)
	for _, row := range g.Rows {
		if row.Type == Star {
			starWeightH += row.Value
		}
	}

	remainingW := clientW - fixedAutoW
	if remainingW < 0 {
		remainingW = 0
	}
	remainingH := clientH - fixedAutoH
	if remainingH < 0 {
		remainingH = 0
	}

	for i, col := range g.Columns {
		if col.Type == Star && starWeightW > 0 {
			colWidths[i] = int(float32(remainingW) * col.Value / starWeightW)
		}
	}
	for i, row := range g.Rows {
		if row.Type == Star && starWeightH > 0 {
			rowHeights[i] = int(float32(remainingH) * row.Value / starWeightH)
		}
	}

	// Sum total layout size
	totalW := 0
	for _, w := range colWidths {
		totalW += w
	}
	totalH := 0
	for _, h := range rowHeights {
		totalH += h
	}

	preferredW := totalW + (2 * g.Padding) + totalGapsW
	preferredH := totalH + (2 * g.Padding) + totalGapsH

	return types.MeasureResult{
		Preferred: image.Pt(preferredW, preferredH),
		Min:       image.Pt(preferredW, preferredH),
	}
}

func (g *Grid) performLayout(state *types.ApplicationState) {
	if len(g.Children) == 0 {
		return
	}

	numCols := len(g.Columns)
	if numCols == 0 {
		numCols = 1
	}
	numRows := len(g.Rows)
	if numRows == 0 {
		numRows = 1
	}

	avail := image.Pt(g.Rect.Dx(), g.Rect.Dy())
	colWidths := make([]int, numCols)
	rowHeights := make([]int, numRows)

	// Step 1: Initialize fixed sizes
	for i, col := range g.Columns {
		if col.Type == Pixel {
			colWidths[i] = int(col.Value)
		}
	}
	for i, row := range g.Rows {
		if row.Type == Pixel {
			rowHeights[i] = int(row.Value)
		}
	}

	// Step 2: Measure Auto columns/rows
	for _, child := range g.Children {
		if child.Child == nil {
			continue
		}
		cSpan := child.ColSpan
		if cSpan <= 0 {
			cSpan = 1
		}
		rSpan := child.RowSpan
		if rSpan <= 0 {
			rSpan = 1
		}

		cIdx := child.Col
		if cIdx < 0 {
			cIdx = 0
		}
		if cIdx >= numCols {
			cIdx = numCols - 1
		}
		rIdx := child.Row
		if rIdx < 0 {
			rIdx = 0
		}
		if rIdx >= numRows {
			rIdx = numRows - 1
		}

		childAvail := image.Point{}
		measurement := types.MeasureComponent(child.Child, childAvail, state)

		if cSpan == 1 && len(g.Columns) > 0 && g.Columns[cIdx].Type == Auto {
			if measurement.Preferred.X > colWidths[cIdx] {
				colWidths[cIdx] = measurement.Preferred.X
			}
		}
		if rSpan == 1 && len(g.Rows) > 0 && g.Rows[rIdx].Type == Auto {
			if measurement.Preferred.Y > rowHeights[rIdx] {
				rowHeights[rIdx] = measurement.Preferred.Y
			}
		}
	}

	// Step 3: Handle Star columns/rows
	totalGapsW := g.ColGap * (numCols - 1)
	totalGapsH := g.RowGap * (numRows - 1)
	clientW := avail.X - (2 * g.Padding) - totalGapsW
	clientH := avail.Y - (2 * g.Padding) - totalGapsH
	if clientW < 0 {
		clientW = 0
	}
	if clientH < 0 {
		clientH = 0
	}

	fixedAutoW := 0
	for _, w := range colWidths {
		fixedAutoW += w
	}
	fixedAutoH := 0
	for _, h := range rowHeights {
		fixedAutoH += h
	}

	starWeightW := float32(0)
	for _, col := range g.Columns {
		if col.Type == Star {
			starWeightW += col.Value
		}
	}
	starWeightH := float32(0)
	for _, row := range g.Rows {
		if row.Type == Star {
			starWeightH += row.Value
		}
	}

	remainingW := clientW - fixedAutoW
	if remainingW < 0 {
		remainingW = 0
	}
	remainingH := clientH - fixedAutoH
	if remainingH < 0 {
		remainingH = 0
	}

	for i, col := range g.Columns {
		if col.Type == Star && starWeightW > 0 {
			colWidths[i] = int(float32(remainingW) * col.Value / starWeightW)
		}
	}
	for i, row := range g.Rows {
		if row.Type == Star && starWeightH > 0 {
			rowHeights[i] = int(float32(remainingH) * row.Value / starWeightH)
		}
	}

	// Step 4: Compute cell coordinate bounds
	colLeft := make([]int, numCols)
	colRight := make([]int, numCols)
	cursorX := g.Padding
	for i := 0; i < numCols; i++ {
		colLeft[i] = cursorX
		colRight[i] = cursorX + colWidths[i]
		cursorX += colWidths[i] + g.ColGap
	}

	rowTop := make([]int, numRows)
	rowBottom := make([]int, numRows)
	cursorY := g.Padding
	for i := 0; i < numRows; i++ {
		rowTop[i] = cursorY
		rowBottom[i] = cursorY + rowHeights[i]
		cursorY += rowHeights[i] + g.RowGap
	}

	// Step 5: Place child components inside cells
	for _, child := range g.Children {
		if child.Child == nil {
			continue
		}

		cSpan := child.ColSpan
		if cSpan <= 0 {
			cSpan = 1
		}
		rSpan := child.RowSpan
		if rSpan <= 0 {
			rSpan = 1
		}

		cStart := child.Col
		if cStart < 0 {
			cStart = 0
		}
		if cStart >= numCols {
			cStart = numCols - 1
		}
		cEnd := cStart + cSpan - 1
		if cEnd >= numCols {
			cEnd = numCols - 1
		}

		rStart := child.Row
		if rStart < 0 {
			rStart = 0
		}
		if rStart >= numRows {
			rStart = numRows - 1
		}
		rEnd := rStart + rSpan - 1
		if rEnd >= numRows {
			rEnd = numRows - 1
		}

		minX := colLeft[cStart]
		maxX := colRight[cEnd]
		minY := rowTop[rStart]
		maxY := rowBottom[rEnd]
		cellRect := image.Rect(minX, minY, maxX, maxY)

		childMeasure := types.MeasureComponent(child.Child, cellRect.Size(), state)

		// Apply alignment inside cell
		w := childMeasure.Preferred.X
		h := childMeasure.Preferred.Y

		x := cellRect.Min.X
		if w > cellRect.Dx() {
			w = cellRect.Dx()
		}
		switch child.Horizontal {
		case Start:
			x = cellRect.Min.X
		case Center:
			x = cellRect.Min.X + (cellRect.Dx()-w)/2
		case End:
			x = cellRect.Max.X - w
		case Stretch:
			w = cellRect.Dx()
			x = cellRect.Min.X
		}

		y := cellRect.Min.Y
		if h > cellRect.Dy() {
			h = cellRect.Dy()
		}
		switch child.Vertical {
		case Start:
			y = cellRect.Min.Y
		case Center:
			y = cellRect.Min.Y + (cellRect.Dy()-h)/2
		case End:
			y = cellRect.Max.Y - h
		case Stretch:
			h = cellRect.Dy()
			y = cellRect.Min.Y
		}

		child.Child.SetBounds(image.Rect(
			g.Rect.Min.X+x,
			g.Rect.Min.Y+y,
			g.Rect.Min.X+x+w,
			g.Rect.Min.Y+y+h,
		))
	}
}

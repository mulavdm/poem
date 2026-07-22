package layout

import (
	"image"

	"github.com/mulavdm/poem/pkg/render/types"
)

type LayoutDirection int

const (
	Vertical LayoutDirection = iota
	Horizontal
)

type FlexAlign int

const (
	AlignStart FlexAlign = iota
	AlignCenter
	AlignEnd
	AlignStretch
)

type FlexJustify int

const (
	JustifyStart FlexJustify = iota
	JustifyCenter
	JustifyEnd
	JustifySpaceBetween
)

// FlexBox is a container that arranges its children in a specific direction.
// It implements the Component interface and allows for modular layout nesting.
type FlexBox struct {
	CompID         string
	Rect           image.Rectangle
	Direction      LayoutDirection
	Wrap           bool
	AlignItems     FlexAlign   // Cross-axis alignment
	JustifyContent FlexJustify // Main-axis distribution
	Padding        int
	Gap            int
	LineGap        int
	Children       []types.Component
}

func (f *FlexBox) ID() string    { return f.CompID }
func (f *FlexBox) GetID() string { return f.CompID }

func (f *FlexBox) Bounds() image.Rectangle {
	if f.Rect.Dx() == 0 || f.Rect.Dy() == 0 {
		size := f.measure(image.Point{}, nil).Preferred
		w := size.X
		h := size.Y
		return image.Rect(f.Rect.Min.X, f.Rect.Min.Y, f.Rect.Min.X+w, f.Rect.Min.Y+h)
	}
	return f.Rect
}

func (f *FlexBox) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	// A nil-state SetBounds pass may intentionally query the assigned viewport,
	// while a live state pass must re-measure font and control metrics instead
	// of freezing geometry from boot or the previous text scale.
	if state == nil && (f.Rect.Dx() > 0 || f.Rect.Dy() > 0) {
		measured := f.measure(avail, state).Preferred
		if f.Rect.Dx() > 0 {
			measured.X = f.Rect.Dx()
		}
		if f.Rect.Dy() > 0 {
			measured.Y = f.Rect.Dy()
		}
		return types.MeasureResult{Preferred: measured, Min: measured}
	}
	size := f.measure(avail, state).Preferred
	return types.MeasureResult{Preferred: size, Min: size}
}

func (f *FlexBox) ContentSize(avail image.Point, state *types.ApplicationState) image.Point {
	return f.measure(avail, state).Preferred
}

func (f *FlexBox) SetBounds(r image.Rectangle) {
	f.Rect = r
	f.performLayout(nil)
}

func (f *FlexBox) Draw(p types.Painter, state *types.ApplicationState) {
	// Re-layout just in case Rect changed outside of SetBounds
	f.performLayout(state)

	for _, child := range f.Children {
		child.Draw(p, state)
	}
}

func (f *FlexBox) HitTest(pt image.Point) string {
	if !pt.In(f.Rect) {
		return ""
	}

	// Check children in reverse order (top-most first)
	for i := len(f.Children) - 1; i >= 0; i-- {
		if id := f.Children[i].HitTest(pt); id != "" {
			return id
		}
	}

	return f.CompID
}

func (f *FlexBox) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID == "" {
		return false
	}
	for _, child := range f.Children {
		if child.OnKey(key, char, state) {
			return true
		}
	}
	return false
}

func (f *FlexBox) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	for i := len(f.Children) - 1; i >= 0; i-- {
		child := f.Children[i]
		if pt.In(child.Bounds()) {
			if child.OnMouseDown(pt, state) {
				return true
			}
		}
	}
	return false
}

func (f *FlexBox) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	for i := len(f.Children) - 1; i >= 0; i-- {
		child := f.Children[i]
		if child.OnMouseUp(pt, state) {
			return true
		}
	}
	return false
}

func (f *FlexBox) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	for i := len(f.Children) - 1; i >= 0; i-- {
		child := f.Children[i]
		if pt.In(child.Bounds()) {
			if child.OnMouseMove(pt, state) {
				return true
			}
		}
	}
	return false
}

func (f *FlexBox) Focusable() bool { return false }

func (f *FlexBox) Walk(fn func(types.Component)) {
	fn(f)
	for _, child := range f.Children {
		child.Walk(fn)
	}
}

func (f *FlexBox) ChildComponents() []types.Component { return f.Children }

func (f *FlexBox) measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	if len(f.Children) == 0 {
		size := explicitAvailable(avail)
		return types.MeasureResult{Preferred: size, Min: size}
	}

	mainAvail := axisSize(avail, f.Direction) - (2 * f.Padding)
	crossAvail := crossAxisSize(avail, f.Direction) - (2 * f.Padding)
	if mainAvail < 0 {
		mainAvail = 0
	}
	if crossAvail < 0 {
		crossAvail = 0
	}

	if f.Direction == Horizontal && f.Wrap {
		return f.measureWrappedHorizontal(mainAvail, crossAvail, state)
	}

	totalMain := 0
	maxCross := 0
	for _, child := range f.Children {
		childAvail := image.Point{}
		if f.Direction == Vertical {
			childAvail = image.Pt(crossAvail, mainAvail)
		} else {
			childAvail = image.Pt(mainAvail, crossAvail)
		}
		measurement := types.MeasureComponent(child, childAvail, state)
		main := axisPoint(measurement.Preferred, f.Direction)
		cross := crossAxisPoint(measurement.Preferred, f.Direction)
		totalMain += main
		if cross > maxCross {
			maxCross = cross
		}
	}
	if len(f.Children) > 1 {
		totalMain += f.Gap * (len(f.Children) - 1)
	}

	width := maxCross + 2*f.Padding
	height := totalMain + 2*f.Padding
	if f.Direction == Horizontal {
		width = totalMain + 2*f.Padding
		height = maxCross + 2*f.Padding
	}
	return types.MeasureResult{
		Preferred: image.Pt(width, height),
		Min:       image.Pt(width, height),
	}
}

func (f *FlexBox) performLayout(state *types.ApplicationState) {
	if len(f.Children) == 0 {
		return
	}

	// 1. Measure children before placement so stale Bounds() do not poison flow.
	availableMainSize := 0
	availableCrossSize := 0
	if f.Direction == Vertical {
		availableMainSize = f.Rect.Dy() - (2 * f.Padding)
		availableCrossSize = f.Rect.Dx() - (2 * f.Padding)
	} else {
		availableMainSize = f.Rect.Dx() - (2 * f.Padding)
		availableCrossSize = f.Rect.Dy() - (2 * f.Padding)
	}
	if availableMainSize < 0 {
		availableMainSize = 0
	}
	if availableCrossSize < 0 {
		availableCrossSize = 0
	}

	measurements := make([]types.MeasureResult, len(f.Children))
	totalMainSize := 0
	for idx, child := range f.Children {
		childAvail := image.Point{}
		if f.Direction == Vertical {
			childAvail = image.Pt(availableCrossSize, availableMainSize)
		} else {
			childAvail = image.Pt(availableMainSize, availableCrossSize)
		}
		measurement := types.MeasureComponent(child, childAvail, state)
		if f.Direction == Vertical {
			if availableCrossSize > 0 && measurement.Preferred.X > availableCrossSize {
				measurement.Preferred.X = availableCrossSize
			}
		} else {
			if availableMainSize > 0 && measurement.Preferred.X > availableMainSize {
				measurement.Preferred.X = availableMainSize
			}
		}
		measurements[idx] = measurement
		totalMainSize += axisPoint(measurement.Preferred, f.Direction)
	}

	if f.Direction == Horizontal && f.Wrap {
		f.performWrappedHorizontalLayout(measurements, availableMainSize, availableCrossSize)
		return
	}

	totalGaps := f.Gap * (len(f.Children) - 1)

	// 2. Main-axis justification (JustifyContent)
	cursorMain := 0
	extraGap := 0

	switch f.JustifyContent {
	case JustifyStart:
		cursorMain = f.Padding
	case JustifyCenter:
		cursorMain = f.Padding + (availableMainSize-totalMainSize-totalGaps)/2
	case JustifyEnd:
		cursorMain = f.Padding + (availableMainSize - totalMainSize - totalGaps)
	case JustifySpaceBetween:
		cursorMain = f.Padding
		if len(f.Children) > 1 {
			extraGap = (availableMainSize - totalMainSize) / (len(f.Children) - 1)
		}
	}

	// 3. Position children
	for idx, child := range f.Children {
		measurement := measurements[idx]
		var newRect image.Rectangle

		// Cross-axis offset
		crossOffset := f.Padding
		crossSize := availableCrossSize
		mainSize := axisPoint(measurement.Preferred, f.Direction)
		crossPreferred := crossAxisPoint(measurement.Preferred, f.Direction)
		if crossPreferred < 0 {
			crossPreferred = 0
		}
		if crossPreferred > crossSize && crossSize > 0 {
			crossPreferred = crossSize
		}

		switch f.AlignItems {
		case AlignStart:
			// crossOffset remains f.Padding
		case AlignCenter:
			if f.Direction == Vertical {
				crossOffset = f.Padding + (crossSize-crossPreferred)/2
			} else {
				crossOffset = f.Padding + (crossSize-crossPreferred)/2
			}
		case AlignEnd:
			crossOffset = f.Padding + (crossSize - crossPreferred)
		case AlignStretch:
			crossOffset = f.Padding
		}

		if f.Direction == Vertical {
			childWidth := crossPreferred
			if f.AlignItems == AlignStretch {
				childWidth = crossSize
			}
			newRect = image.Rect(
				f.Rect.Min.X+crossOffset,
				f.Rect.Min.Y+cursorMain,
				f.Rect.Min.X+crossOffset+childWidth,
				f.Rect.Min.Y+cursorMain+mainSize,
			)
			cursorMain += mainSize + f.Gap + extraGap
		} else {
			childHeight := crossPreferred
			if f.AlignItems == AlignStretch {
				childHeight = crossSize
			}
			newRect = image.Rect(
				f.Rect.Min.X+cursorMain,
				f.Rect.Min.Y+crossOffset,
				f.Rect.Min.X+cursorMain+mainSize,
				f.Rect.Min.Y+crossOffset+childHeight,
			)
			cursorMain += mainSize + f.Gap + extraGap
		}

		child.SetBounds(newRect)
	}
}

func (f *FlexBox) measureWrappedHorizontal(mainAvail, crossAvail int, state *types.ApplicationState) types.MeasureResult {
	lineGap := f.LineGap
	if lineGap <= 0 {
		lineGap = f.Gap
	}

	type line struct {
		width  int
		height int
	}

	lines := []line{{}}
	for _, child := range f.Children {
		measurement := types.MeasureComponent(child, image.Pt(mainAvail, crossAvail), state)
		childW := measurement.Preferred.X
		childH := measurement.Preferred.Y
		if mainAvail > 0 && childW > mainAvail {
			childW = mainAvail
		}
		current := &lines[len(lines)-1]
		projected := childW
		if current.width > 0 {
			projected = current.width + f.Gap + childW
		}
		if mainAvail > 0 && current.width > 0 && projected > mainAvail {
			lines = append(lines, line{width: childW, height: childH})
			continue
		}
		current.width = projected
		if childH > current.height {
			current.height = childH
		}
	}

	totalHeight := 0
	maxWidth := 0
	for idx, line := range lines {
		totalHeight += line.height
		if idx < len(lines)-1 {
			totalHeight += lineGap
		}
		if line.width > maxWidth {
			maxWidth = line.width
		}
	}

	return types.MeasureResult{
		Preferred: image.Pt(maxWidth+2*f.Padding, totalHeight+2*f.Padding),
		Min:       image.Pt(maxWidth+2*f.Padding, totalHeight+2*f.Padding),
	}
}

func (f *FlexBox) performWrappedHorizontalLayout(measurements []types.MeasureResult, availableMainSize, availableCrossSize int) {
	lineGap := f.LineGap
	if lineGap <= 0 {
		lineGap = f.Gap
	}

	type wrappedItem struct {
		index  int
		width  int
		height int
	}
	type wrappedLine struct {
		items  []wrappedItem
		width  int
		height int
	}

	lines := []wrappedLine{{}}
	for idx, measurement := range measurements {
		childW := measurement.Preferred.X
		childH := measurement.Preferred.Y
		if availableMainSize > 0 && childW > availableMainSize {
			childW = availableMainSize
		}
		current := &lines[len(lines)-1]
		projected := childW
		if current.width > 0 {
			projected = current.width + f.Gap + childW
		}
		if availableMainSize > 0 && current.width > 0 && projected > availableMainSize {
			lines = append(lines, wrappedLine{
				items:  []wrappedItem{{index: idx, width: childW, height: childH}},
				width:  childW,
				height: childH,
			})
			continue
		}
		current.items = append(current.items, wrappedItem{index: idx, width: childW, height: childH})
		current.width = projected
		if childH > current.height {
			current.height = childH
		}
	}

	cursorY := f.Padding
	for _, line := range lines {
		cursorX := f.Padding
		for _, item := range line.items {
			child := f.Children[item.index]
			childHeight := item.height
			childY := f.Padding
			switch f.AlignItems {
			case AlignCenter:
				childY = f.Padding + (line.height-childHeight)/2
			case AlignEnd:
				childY = f.Padding + (line.height - childHeight)
			case AlignStretch:
				childHeight = line.height
			}
			newRect := image.Rect(
				f.Rect.Min.X+cursorX,
				f.Rect.Min.Y+cursorY+childY-f.Padding,
				f.Rect.Min.X+cursorX+item.width,
				f.Rect.Min.Y+cursorY+childY-f.Padding+childHeight,
			)
			child.SetBounds(newRect)
			cursorX += item.width + f.Gap
		}
		cursorY += line.height + lineGap
	}
}

func axisSize(pt image.Point, direction LayoutDirection) int {
	if direction == Vertical {
		return pt.Y
	}
	return pt.X
}

func crossAxisSize(pt image.Point, direction LayoutDirection) int {
	if direction == Vertical {
		return pt.X
	}
	return pt.Y
}

func axisPoint(pt image.Point, direction LayoutDirection) int {
	if direction == Vertical {
		return pt.Y
	}
	return pt.X
}

func crossAxisPoint(pt image.Point, direction LayoutDirection) int {
	if direction == Vertical {
		return pt.X
	}
	return pt.Y
}

func explicitAvailable(avail image.Point) image.Point {
	if avail.X < 0 {
		avail.X = 0
	}
	if avail.Y < 0 {
		avail.Y = 0
	}
	return avail
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

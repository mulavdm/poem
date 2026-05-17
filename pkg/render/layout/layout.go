package layout

import (
	"image"

	"go_native_gpu_gui/pkg/render/types"
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
	AlignItems     FlexAlign   // Cross-axis alignment
	JustifyContent FlexJustify // Main-axis distribution
	Padding        int
	Gap            int
	Children       []types.Component
}

func (f *FlexBox) ID() string    { return f.CompID }
func (f *FlexBox) GetID() string { return f.CompID }

func (f *FlexBox) Bounds() image.Rectangle {
	if f.Rect.Dx() == 0 || f.Rect.Dy() == 0 {
		w := 0
		h := 0
		if f.Direction == Vertical {
			h = 2 * f.Padding
			for i, child := range f.Children {
				cb := child.Bounds()
				h += cb.Dy()
				if cb.Dx() > w {
					w = cb.Dx()
				}
				if i < len(f.Children)-1 {
					h += f.Gap
				}
			}
			w += 2 * f.Padding
		} else {
			w = 2 * f.Padding
			for i, child := range f.Children {
				cb := child.Bounds()
				w += cb.Dx()
				if cb.Dy() > h {
					h = cb.Dy()
				}
				if i < len(f.Children)-1 {
					w += f.Gap
				}
			}
			h += 2 * f.Padding
		}
		return image.Rect(f.Rect.Min.X, f.Rect.Min.Y, f.Rect.Min.X+w, f.Rect.Min.Y+h)
	}
	return f.Rect
}

func (f *FlexBox) SetBounds(r image.Rectangle) {
	f.Rect = r
	f.performLayout()
}

func (f *FlexBox) Draw(p types.Painter, state *types.ApplicationState) {
	// Re-layout just in case Rect changed outside of SetBounds
	f.performLayout()

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
	for _, child := range f.Children {
		if pt.In(child.Bounds()) {
			if child.OnMouseDown(pt, state) {
				return true
			}
		}
	}
	return false
}

func (f *FlexBox) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	for _, child := range f.Children {
		if child.OnMouseUp(pt, state) {
			return true
		}
	}
	return false
}

func (f *FlexBox) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	for _, child := range f.Children {
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

func (f *FlexBox) performLayout() {
	if len(f.Children) == 0 {
		return
	}

	// 1. Calculate total child size and count
	totalMainSize := 0
	maxCrossSize := 0
	for _, child := range f.Children {
		b := child.Bounds()
		if f.Direction == Vertical {
			totalMainSize += b.Dy()
			if b.Dx() > maxCrossSize {
				maxCrossSize = b.Dx()
			}
		} else {
			totalMainSize += b.Dx()
			if b.Dy() > maxCrossSize {
				maxCrossSize = b.Dy()
			}
		}
	}

	totalGaps := f.Gap * (len(f.Children) - 1)
	availableMainSize := 0
	if f.Direction == Vertical {
		availableMainSize = f.Rect.Dy() - (2 * f.Padding)
	} else {
		availableMainSize = f.Rect.Dx() - (2 * f.Padding)
	}

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
	for _, child := range f.Children {
		b := child.Bounds()
		var newRect image.Rectangle

		// Cross-axis offset
		crossOffset := f.Padding
		crossSize := 0
		if f.Direction == Vertical {
			crossSize = f.Rect.Dx() - (2 * f.Padding)
		} else {
			crossSize = f.Rect.Dy() - (2 * f.Padding)
		}

		switch f.AlignItems {
		case AlignStart:
			// crossOffset remains f.Padding
		case AlignCenter:
			if f.Direction == Vertical {
				crossOffset = f.Padding + (crossSize-b.Dx())/2
			} else {
				crossOffset = f.Padding + (crossSize-b.Dy())/2
			}
		case AlignEnd:
			if f.Direction == Vertical {
				crossOffset = f.Padding + (crossSize - b.Dx())
			} else {
				crossOffset = f.Padding + (crossSize - b.Dy())
			}
		case AlignStretch:
			// Stretching would require updating child bounds width/height
			// For now, we just align start.
		}

		if f.Direction == Vertical {
			newRect = image.Rect(
				f.Rect.Min.X+crossOffset,
				f.Rect.Min.Y+cursorMain,
				f.Rect.Min.X+crossOffset+b.Dx(),
				f.Rect.Min.Y+cursorMain+b.Dy(),
			)
			cursorMain += b.Dy() + f.Gap + extraGap
		} else {
			newRect = image.Rect(
				f.Rect.Min.X+cursorMain,
				f.Rect.Min.Y+crossOffset,
				f.Rect.Min.X+cursorMain+b.Dx(),
				f.Rect.Min.Y+crossOffset+b.Dy(),
			)
			cursorMain += b.Dx() + f.Gap + extraGap
		}

		child.SetBounds(newRect)
	}
}

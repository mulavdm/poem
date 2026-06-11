package components

import (
	"image"
	"image/color"
	"math"

	"go_native_gpu_gui/pkg/render/types"
)

// ScrollView is a container that clips its children to its bounding box
// and allows vertical scrolling via mouse wheel or dragging a scrollbar.
type ScrollView struct {
	CompID         string
	Rect           image.Rectangle
	Children       []types.Component
	ScrollY        int
	CurrentScrollY int // Smoothly animated/interpolated scroll offset
	ContentH       int
	ScrollbarW     int

	// Drag state
	dragging     bool
	dragStartY   int
	scrollStartY int
}

func (s *ScrollView) ID() string              { return s.CompID }
func (s *ScrollView) GetID() string           { return s.CompID }
func (s *ScrollView) Bounds() image.Rectangle { return s.Rect }
func (s *ScrollView) Focusable() bool         { return false }
func (s *ScrollView) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	if size := explicitSize(s.Rect); size.X > 0 && size.Y > 0 {
		return types.MeasureResult{Preferred: size, Min: image.Pt(minValueInt(size.X, 120), minValueInt(size.Y, 120))}
	}
	if len(s.Children) > 0 {
		child := types.MeasureComponent(s.Children[0], avail, state)
		width := child.Preferred.X
		height := child.Preferred.Y
		if avail.X > 0 {
			width = avail.X
		}
		if avail.Y > 0 {
			height = minValueInt(child.Preferred.Y, avail.Y)
		}
		return types.MeasureResult{
			Preferred: image.Pt(width, height),
			Min:       image.Pt(minValueInt(width, 120), minValueInt(height, 120)),
		}
	}
	width := avail.X
	height := avail.Y
	if width <= 0 {
		width = 240
	}
	if height <= 0 {
		height = 180
	}
	return types.MeasureResult{
		Preferred: image.Pt(width, height),
		Min:       image.Pt(minValueInt(width, 120), minValueInt(height, 120)),
	}
}

func (s *ScrollView) SetBounds(r image.Rectangle) {
	s.Rect = r
	if s.ScrollbarW <= 0 {
		s.ScrollbarW = 12
	}

	if len(s.Children) > 0 {
		childWidth := s.Rect.Dx() - 15 // baseline gutter
		if childWidth < 0 {
			childWidth = 0
		}
		contentSize := types.MeasureContent(s.Children[0], image.Pt(childWidth, s.Rect.Dy()), nil)
		s.ContentH = contentSize.Y
		if s.ContentH > s.Rect.Dy() {
			childWidth = s.Rect.Dx() - (s.ScrollbarW + 15) // extra gutter when scrollbar is present
			if childWidth < 0 {
				childWidth = 0
			}
			contentSize = types.MeasureContent(s.Children[0], image.Pt(childWidth, s.Rect.Dy()), nil)
			s.ContentH = contentSize.Y
		}
		if s.ContentH < s.Rect.Dy() {
			s.ContentH = s.Rect.Dy()
		}

		childRect := image.Rect(
			s.Rect.Min.X,
			s.Rect.Min.Y,
			s.Rect.Min.X+childWidth,
			s.Rect.Min.Y+s.ContentH,
		)

		s.Children[0].SetBounds(childRect)

		// Final bounds can expand the scrollable extent, but must not shrink
		// measured content back to a fixed viewport-sized container rect.
		if boundsH := s.Children[0].Bounds().Dy(); boundsH > s.ContentH {
			s.ContentH = boundsH
		}
	}
}

func (s *ScrollView) hasScrollbar() bool {
	return s.ContentH > s.Rect.Dy()
}

func (s *ScrollView) scrollbarThumbRect() image.Rectangle {
	if !s.hasScrollbar() {
		return image.Rectangle{}
	}

	viewH := s.Rect.Dy()
	trackH := viewH

	thumbH := int(float64(viewH) * (float64(viewH) / float64(s.ContentH)))
	if thumbH < 30 {
		thumbH = 30 // Minimum height to grab
	}

	maxScroll := s.ContentH - viewH

	// Use CurrentScrollY (animated offset) for visual thumb positioning
	scrollRatio := float64(s.CurrentScrollY) / float64(maxScroll)

	thumbY := s.Rect.Min.Y + int(scrollRatio*float64(trackH-thumbH))

	return image.Rect(
		s.Rect.Max.X-s.ScrollbarW+2,
		thumbY,
		s.Rect.Max.X-2,
		thumbY+thumbH,
	)
}

func (s *ScrollView) Draw(p types.Painter, state *types.ApplicationState) {
	// Restore scroll state from persistent maps
	if state.ScrollPositions != nil {
		s.ScrollY = state.ScrollPositions[s.CompID]
	}

	// Clamp scroll target before processing interpolation
	if s.ScrollY < 0 {
		s.ScrollY = 0
	}
	maxScroll := s.ContentH - s.Rect.Dy()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if s.ScrollY > maxScroll {
		s.ScrollY = maxScroll
	}
	if state.ScrollPositions != nil {
		state.ScrollPositions[s.CompID] = s.ScrollY
	}

	// Smoothly interpolate (LERP) current position towards target scroll
	if state.ScrollCurrent != nil {
		if _, exists := state.ScrollCurrent[s.CompID]; !exists {
			state.ScrollCurrent[s.CompID] = float64(s.ScrollY)
		}

		targetY := float64(s.ScrollY)
		currentY := state.ScrollCurrent[s.CompID]
		diff := targetY - currentY

		if state.ActiveID == s.CompID && state.ScrollDragStart != nil && state.ScrollDragStart[s.CompID] > 0 {
			// Scrollbar dragging - stay perfectly pinned to cursor instantly without lag
			currentY = targetY
			s.dragging = true
			s.dragStartY = state.ScrollDragStart[s.CompID]
			s.scrollStartY = state.ScrollStartY[s.CompID]
		} else if math.Abs(diff) > 0.1 {
			// Delta-time independent LERP factor: 1 - (1 - f)^(dt / dt_standard)
			// Standard f = 0.15 at 60 FPS (dt = 0.0166 seconds)
			dt := 0.0166
			if state.RenderDt > 0 {
				dt = state.RenderDt
			}
			// Clamp dt to avoid huge jumps on sudden stutter frames
			if dt > 0.1 {
				dt = 0.1
			}
			f := 1.0 - math.Pow(1.0-0.07, dt/0.0166)
			currentY += diff * f
		} else {
			currentY = targetY
		}

		state.ScrollCurrent[s.CompID] = currentY
		s.CurrentScrollY = int(math.Round(currentY))
	} else {
		s.CurrentScrollY = s.ScrollY
	}

	// Draw viewport backdrop
	p.FillRect(s.Rect, color.RGBA{15, 17, 26, 255})

	if len(s.Children) == 0 {
		return
	}

	// Apply scoped clip and offset for child rendering using the animated offset.
	p.PushClip(s.Rect)
	p.SetOffset(0, -float32(s.CurrentScrollY))

	// Draw Children
	for _, child := range s.Children {
		child.Draw(p, state)
	}

	// Reset clip and offset.
	p.PopClip()
	p.SetOffset(0, 0)

	// Draw Scrollbar on top of clipped content
	if s.hasScrollbar() {
		// Draw Scrollbar Track
		trackRect := image.Rect(
			s.Rect.Max.X-s.ScrollbarW,
			s.Rect.Min.Y,
			s.Rect.Max.X,
			s.Rect.Max.Y,
		)
		p.FillRect(trackRect, color.RGBA{255, 255, 255, 10})

		// Draw Scrollbar Thumb using animated scrollbar bounds
		thumb := s.scrollbarThumbRect()
		thumbColor := color.RGBA{255, 255, 255, 80}

		// If hovering or dragging, glow neon green!
		mPt := image.Point{state.MouseX, state.MouseY}
		isHovered := mPt.In(thumb) || s.dragging
		if isHovered {
			thumbColor = color.RGBA{0, 255, 150, 150}
		}

		p.DrawRoundedRect(thumb, 4, thumbColor)
	}
}

func (s *ScrollView) HitTest(pt image.Point) string {
	if !pt.In(s.Rect) {
		return ""
	}

	// Translate coordinate space for children using the animated offset
	scrolledPt := pt.Add(image.Point{0, s.CurrentScrollY})
	for i := len(s.Children) - 1; i >= 0; i-- {
		if id := s.Children[i].HitTest(scrolledPt); id != "" {
			return id
		}
	}

	return s.CompID
}

func (s *ScrollView) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	// Propagate key events
	for _, child := range s.Children {
		if child.OnKey(key, char, state) {
			return true
		}
	}
	return false
}

func (s *ScrollView) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if !pt.In(s.Rect) {
		return false
	}

	if state != nil && state.ScrollCurrent != nil {
		if cur, ok := state.ScrollCurrent[s.CompID]; ok {
			s.CurrentScrollY = int(math.Round(cur))
		}
	}

	// Scrollbar dragging
	if s.hasScrollbar() && pt.In(s.scrollbarThumbRect()) {
		s.dragging = true
		s.dragStartY = pt.Y
		s.scrollStartY = s.ScrollY
		state.ActiveID = s.CompID
		if state.ScrollPositions != nil {
			state.ScrollPositions[s.CompID] = s.ScrollY
			state.ScrollDragStart[s.CompID] = pt.Y
			state.ScrollStartY[s.CompID] = s.ScrollY
		}
		return true
	}

	// Propagate mouse down using animated coordinate space
	scrolledPt := pt.Add(image.Point{0, s.CurrentScrollY})
	for _, child := range s.Children {
		if child.OnMouseDown(scrolledPt, state) {
			return true
		}
	}

	return false
}

func (s *ScrollView) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if state != nil && state.ScrollCurrent != nil {
		if cur, ok := state.ScrollCurrent[s.CompID]; ok {
			s.CurrentScrollY = int(math.Round(cur))
		}
	}

	if state.ActiveID == s.CompID || s.dragging {
		s.dragging = false
		state.ActiveID = ""
		if state.ScrollDragStart != nil {
			state.ScrollDragStart[s.CompID] = 0
		}
		return true
	}

	// Propagate mouse up using animated coordinate space
	scrolledPt := pt.Add(image.Point{0, s.CurrentScrollY})
	for _, child := range s.Children {
		if child.OnMouseUp(scrolledPt, state) {
			return true
		}
	}
	return false
}

func (s *ScrollView) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if state != nil && state.ScrollCurrent != nil {
		if cur, ok := state.ScrollCurrent[s.CompID]; ok {
			s.CurrentScrollY = int(math.Round(cur))
		}
	}

	// Restore drag state if active
	if state.ActiveID == s.CompID && state.ScrollDragStart != nil && state.ScrollDragStart[s.CompID] > 0 {
		s.dragging = true
		s.dragStartY = state.ScrollDragStart[s.CompID]
		s.scrollStartY = state.ScrollStartY[s.CompID]
	}

	if s.dragging {
		deltaY := pt.Y - s.dragStartY
		viewH := s.Rect.Dy()
		thumbH := s.scrollbarThumbRect().Dy()

		travel := viewH - thumbH
		if travel > 0 {
			ratio := float64(deltaY) / float64(travel)
			maxScroll := s.ContentH - viewH
			s.ScrollY = s.scrollStartY + int(ratio*float64(maxScroll))

			// Clamp scroll
			if s.ScrollY < 0 {
				s.ScrollY = 0
			}
			if s.ScrollY > maxScroll {
				s.ScrollY = maxScroll
			}
			if state.ScrollPositions != nil {
				state.ScrollPositions[s.CompID] = s.ScrollY
			}
		}
		return true
	}

	// Propagate mouse move using animated coordinate space
	scrolledPt := pt.Add(image.Point{0, s.CurrentScrollY})
	for _, child := range s.Children {
		child.OnMouseMove(scrolledPt, state)
	}

	return false
}

func (s *ScrollView) OnMouseWheel(pt image.Point, delta int, state *types.ApplicationState) bool {
	if !pt.In(s.Rect) {
		return false
	}
	if !s.hasScrollbar() {
		return false
	}

	if state.ScrollPositions != nil {
		s.ScrollY = state.ScrollPositions[s.CompID]
	}

	// delta > 0 is scrolling up; delta < 0 is scrolling down
	// Scroll speed: 100px per notch
	scrollAmount := 100
	if delta > 0 {
		s.ScrollY -= scrollAmount
	} else {
		s.ScrollY += scrollAmount
	}

	// Clamp ScrollY
	maxScroll := s.ContentH - s.Rect.Dy()
	if s.ScrollY < 0 {
		s.ScrollY = 0
	}
	if s.ScrollY > maxScroll {
		s.ScrollY = maxScroll
	}

	if state.ScrollPositions != nil {
		state.ScrollPositions[s.CompID] = s.ScrollY
	}

	return true
}

func (s *ScrollView) Walk(fn func(types.Component)) {
	fn(s)
	for _, child := range s.Children {
		child.Walk(fn)
	}
}

func (s *ScrollView) ScrollToChild(childID string, childBounds image.Rectangle, state *types.ApplicationState) bool {
	var found bool
	s.Walk(func(c types.Component) {
		if c.ID() == childID {
			found = true
		}
	})
	if !found {
		return false
	}

	// Calculate child's Y bounds relative to the ScrollView's content
	relMinY := childBounds.Min.Y - s.Rect.Min.Y
	relMaxY := childBounds.Max.Y - s.Rect.Min.Y
	viewH := s.Rect.Dy()

	// Scroll to make child fully visible with beautiful breathing room padding
	padding := 20
	if relMinY-padding < s.ScrollY {
		s.ScrollY = relMinY - padding
	} else if relMaxY+padding > s.ScrollY+viewH {
		s.ScrollY = relMaxY + padding - viewH
	}

	// Clamp ScrollY
	maxScroll := s.ContentH - viewH
	if s.ScrollY < 0 {
		s.ScrollY = 0
	}
	if maxScroll > 0 && s.ScrollY > maxScroll {
		s.ScrollY = maxScroll
	}

	if state.ScrollPositions != nil {
		state.ScrollPositions[s.CompID] = s.ScrollY
	}
	return true
}

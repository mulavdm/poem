package components

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/types"
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
	UseTheme       bool

	// Drag state
	dragging     bool
	dragStartY   int
	scrollStartY int
}

func NewScrollView(id string, children ...types.Component) *ScrollView {
	return &ScrollView{CompID: id, Children: children, UseTheme: true}
}

func (s *ScrollView) ID() string              { return s.CompID }
func (s *ScrollView) GetID() string           { return s.CompID }
func (s *ScrollView) Bounds() image.Rectangle { return s.Rect }
func (s *ScrollView) Focusable() bool         { return false }
func (s *ScrollView) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	if len(s.Children) > 0 {
		content := s.ContentSize(avail, state)
		width := content.X
		height := content.Y
		if avail.X > 0 {
			width = minValueInt(width, avail.X)
		}
		if avail.Y > 0 {
			height = minValueInt(height, avail.Y)
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

// ContentSize reports the intrinsic child extent plus the viewport gutter.
// Rect is deliberately ignored: it is the result of a previous layout pass,
// not an author-specified size, and must not freeze an adaptive parent width.
func (s *ScrollView) ContentSize(avail image.Point, state *types.ApplicationState) image.Point {
	if len(s.Children) == 0 {
		return image.Point{}
	}
	current := activeTheme(state)
	scrollbar := s.ScrollbarW
	if scrollbar <= 0 {
		scrollbar = current.Controls.Scrollbar
	}
	gutter := current.Spacing.MD + scrollbar
	childAvail := avail
	if childAvail.X > gutter {
		childAvail.X -= gutter
	}
	content := types.MeasureContent(s.Children[0], childAvail, state)
	content.X += gutter
	return content
}

func (s *ScrollView) SetBounds(r image.Rectangle) {
	s.Rect = r
	s.layout(nil)
}

func (s *ScrollView) layout(state *types.ApplicationState) {
	if s.ScrollbarW <= 0 {
		s.ScrollbarW = activeTheme(state).Controls.Scrollbar
	}

	if len(s.Children) > 0 {
		gutter := activeTheme(state).Spacing.MD
		childWidth := s.Rect.Dx() - gutter
		if childWidth < 0 {
			childWidth = 0
		}
		contentSize := types.MeasureContent(s.Children[0], image.Pt(childWidth, s.Rect.Dy()), nil)
		s.ContentH = contentSize.Y
		if s.ContentH > s.Rect.Dy() {
			childWidth = s.Rect.Dx() - (s.ScrollbarW + gutter) // extra gutter when scrollbar is present
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
	// Text scale, font metrics, and theme controls can change after SetBounds.
	// Re-measure before drawing so the scroll extent and child layout stay
	// state-aware instead of retaining the boot-time nil-state geometry.
	s.layout(state)

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
	current := activeTheme(state)
	backdrop := color.RGBA{15, 17, 26, 255}
	if themed(s.UseTheme, state) {
		backdrop = current.Colors.SurfaceSunken
	}
	p.FillRect(s.Rect, backdrop)

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
	// Descendant containers can reflow during their state-aware Draw pass
	// (notably after a text-scale change). Include their final geometry in the
	// scroll extent so controls that moved below the initial estimate remain
	// reachable on this frame.
	for _, child := range s.Children {
		child.Walk(func(descendant types.Component) {
			if extent := descendant.Bounds().Max.Y - s.Rect.Min.Y; extent > s.ContentH {
				s.ContentH = extent
			}
		})
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
		trackColor := color.RGBA{255, 255, 255, 10}
		if themed(s.UseTheme, state) {
			trackColor = current.Colors.ScrollbarTrack
		}
		p.FillRect(trackRect, trackColor)

		// Draw Scrollbar Thumb using animated scrollbar bounds
		thumb := s.scrollbarThumbRect()
		thumbColor := color.RGBA{255, 255, 255, 80}
		if themed(s.UseTheme, state) {
			thumbColor = current.Colors.ScrollbarThumb
		}

		// If hovering or dragging, glow neon green!
		mPt := image.Point{state.MouseX, state.MouseY}
		isHovered := mPt.In(thumb) || s.dragging
		if isHovered {
			if themed(s.UseTheme, state) {
				thumbColor = current.Colors.ScrollbarActive
			} else {
				thumbColor = color.RGBA{0, 255, 150, 150}
			}
			state.CursorID = state.HandCursor
		}

		p.DrawRoundedRect(thumb, roundedRadius(thumb, current.Radii.Pill), thumbColor)
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

// PointerForChild maps a viewport-space pointer into the ScrollView's
// content-space coordinates. Mouse propagation has always done this locally;
// exposing the same transform lets the shared wheel/pan/pinch router avoid
// comparing screen coordinates with scrolled child bounds.
func (s *ScrollView) PointerForChild(pt image.Point, state *types.ApplicationState) image.Point {
	offset := s.CurrentScrollY
	if state != nil && state.ScrollCurrent != nil {
		if current, ok := state.ScrollCurrent[s.CompID]; ok {
			offset = int(math.Round(current))
		}
	}
	return pt.Add(image.Pt(0, offset))
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

	// delta > 0 is scrolling up; delta < 0 is scrolling down. The distance
	// scales with the delta magnitude so pixel-accurate sources track 1:1:
	// a Windows wheel notch (±120) keeps its historical 100px step, while
	// the Android presenter streams per-move touch deltas (finger px ×1.2)
	// that land back as exactly the dragged distance.
	scrollAmount := delta * 100 / 120
	if scrollAmount < 0 {
		scrollAmount = -scrollAmount
	}
	if scrollAmount < 1 {
		scrollAmount = 1
	}
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

func (s *ScrollView) ChildComponents() []types.Component { return s.Children }

func (s *ScrollView) Semantics(*types.ApplicationState) semantics.Node {
	maxScroll := s.ContentH - s.Rect.Dy()
	if maxScroll < 0 {
		maxScroll = 0
	}
	scrollable := maxScroll > 0
	percent := -1.0
	viewSize := 100.0
	if scrollable {
		percent = float64(s.ScrollY) * 100 / float64(maxScroll)
		viewSize = float64(s.Rect.Dy()) * 100 / float64(s.ContentH)
	}
	actions := []semantics.Action(nil)
	if scrollable {
		actions = []semantics.Action{semantics.ActionSetScroll}
	}
	return semantics.Node{ID: s.CompID, Role: semantics.RoleGroup, Name: "Scrollable content", Bounds: s.Rect, Actions: actions,
		Scroll: &semantics.ScrollValue{VerticallyScrollable: scrollable, HorizontalPercent: -1, VerticalPercent: percent, HorizontalViewSize: 100, VerticalViewSize: viewSize}}
}

func (s *ScrollView) PerformSemanticAction(targetID string, action semantics.Action, value string, state *types.ApplicationState) bool {
	if targetID != s.CompID || action != semantics.ActionSetScroll {
		return false
	}
	var horizontal, vertical float64
	if _, err := fmt.Sscanf(value, "%f:%f", &horizontal, &vertical); err != nil {
		return false
	}
	maxScroll := s.ContentH - s.Rect.Dy()
	if maxScroll <= 0 || vertical < 0 {
		return false
	}
	if vertical > 100 {
		vertical = 100
	}
	s.ScrollY = int(math.Round(vertical * float64(maxScroll) / 100))
	s.CurrentScrollY = s.ScrollY
	if state != nil {
		if state.ScrollPositions == nil {
			state.ScrollPositions = make(map[string]int)
		}
		state.ScrollPositions[s.CompID] = s.ScrollY
		if state.ScrollCurrent == nil {
			state.ScrollCurrent = make(map[string]float64)
		}
		state.ScrollCurrent[s.CompID] = float64(s.ScrollY)
	}
	return true
}

func (s *ScrollView) TransformSemanticChild(node *semantics.Node, state *types.ApplicationState) {
	offset := s.CurrentScrollY
	if state != nil && state.ScrollCurrent != nil {
		if current, ok := state.ScrollCurrent[s.CompID]; ok {
			offset = int(math.Round(current))
		}
	}
	transformSemanticViewportNode(node, image.Pt(0, -offset), s.Rect, false)
}

func transformSemanticViewportNode(node *semantics.Node, offset image.Point, viewport image.Rectangle, parentOffscreen bool) {
	if node == nil {
		return
	}
	if !node.Bounds.Empty() {
		node.Bounds = node.Bounds.Add(offset)
		visible := node.Bounds.Intersect(viewport)
		node.State.Offscreen = node.State.Offscreen || parentOffscreen || visible.Empty()
		if !visible.Empty() {
			node.Bounds = visible
		}
	} else if parentOffscreen {
		node.State.Offscreen = true
	}
	for index := range node.Children {
		transformSemanticViewportNode(&node.Children[index], offset, viewport, node.State.Offscreen)
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

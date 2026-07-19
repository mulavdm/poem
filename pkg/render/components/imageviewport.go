package components

import (
	"image"
	"image/color"
	"math"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/types"
)

// ImageTransform is a normalized image transform. Offsets are fractions of
// the viewport size and Scale is relative to the contain-fitted image.
type ImageTransform struct {
	OffsetX float64
	OffsetY float64
	Scale   float64
}

// ImageMarkerVariant selects a marker's semantic color.
type ImageMarkerVariant int

const (
	// ImageMarkerNeutral draws a neutral marker.
	ImageMarkerNeutral ImageMarkerVariant = iota
	// ImageMarkerPrimary draws a primary marker.
	ImageMarkerPrimary
	// ImageMarkerSecondary draws a secondary marker.
	ImageMarkerSecondary
	// ImageMarkerDanger draws a danger marker.
	ImageMarkerDanger
	// ImageMarkerSuccess draws a success marker.
	ImageMarkerSuccess
	// ImageMarkerWarning draws a warning marker.
	ImageMarkerWarning
)

// ImageMarker is an accessible annotation anchored to normalized image
// coordinates.
type ImageMarker struct {
	// ID is stable and unique inside the viewport.
	ID string
	// Label is the accessible marker name.
	Label string
	// X and Y are normalized image coordinates.
	X, Y float64
	// Variant selects the marker color.
	Variant ImageMarkerVariant
	// Selected emphasizes the marker.
	Selected bool
	// Disabled suppresses marker activation.
	Disabled bool
}

// ImageViewport presents decoded RGBA pixels in an interactive clipped view.
type ImageViewport struct {
	CompID      string
	Rect        image.Rectangle
	ImageWidth  int
	ImageHeight int
	Pixels      []byte
	BGColor     color.RGBA
	Transform   ImageTransform
	MinScale    float64
	MaxScale    float64
	Disabled    bool
	Alt         string
	OnChange    func(ImageTransform, *types.ApplicationState)
	// OnActivate receives normalized viewport coordinates for a short
	// background click or tap.
	OnActivate func(float64, float64, *types.ApplicationState)
	// Markers are annotations transformed with the image.
	Markers []ImageMarker
	// OnMarker receives the activated marker ID.
	OnMarker func(string, *types.ApplicationState)

	dragging      bool
	dragMoved     bool
	dragStart     image.Point
	start         ImageTransform
	pressedMarker int
}

func (i *ImageViewport) ID() string                    { return i.CompID }
func (i *ImageViewport) GetID() string                 { return i.CompID }
func (i *ImageViewport) Bounds() image.Rectangle       { return i.Rect }
func (i *ImageViewport) SetBounds(r image.Rectangle)   { i.Rect = r }
func (i *ImageViewport) Focusable() bool               { return !i.Disabled }
func (i *ImageViewport) Walk(fn func(types.Component)) { fn(i) }

func (i *ImageViewport) normalized() ImageTransform {
	t := i.Transform
	if t.Scale == 0 {
		t.Scale = 1
	}
	minScale, maxScale := i.MinScale, i.MaxScale
	if minScale <= 0 {
		minScale = 0.5
	}
	if maxScale < minScale {
		maxScale = math.Max(8, minScale)
	}
	t.Scale = math.Max(minScale, math.Min(maxScale, t.Scale))
	t.OffsetX = math.Max(-4, math.Min(4, t.OffsetX))
	t.OffsetY = math.Max(-4, math.Min(4, t.OffsetY))
	return t
}

func (i *ImageViewport) Measure(avail image.Point, _ *types.ApplicationState) types.MeasureResult {
	width, height := avail.X, avail.Y
	if width <= 0 {
		width = 320
	}
	if height <= 0 {
		height = 240
	}
	if i.ImageWidth > 0 && i.ImageHeight > 0 {
		contained := containRect(image.Rect(0, 0, width, height), i.ImageWidth, i.ImageHeight)
		width, height = contained.Dx(), contained.Dy()
	}
	size := applyExplicitSize(explicitSize(i.Rect), image.Pt(width, height))
	return types.MeasureResult{Preferred: size, Min: image.Pt(minValueInt(size.X, 120), minValueInt(size.Y, 120))}
}

func (i *ImageViewport) Draw(p types.Painter, _ *types.ApplicationState) {
	if i.BGColor.A != 0 {
		p.FillRect(i.Rect, i.BGColor)
	}
	if len(i.Pixels) == 0 || i.ImageWidth <= 0 || i.ImageHeight <= 0 {
		return
	}
	t := i.normalized()
	base := containRect(i.Rect, i.ImageWidth, i.ImageHeight)
	w := int(math.Round(float64(base.Dx()) * t.Scale))
	h := int(math.Round(float64(base.Dy()) * t.Scale))
	cx := i.Rect.Min.X + i.Rect.Dx()/2 + int(math.Round(t.OffsetX*float64(i.Rect.Dx())))
	cy := i.Rect.Min.Y + i.Rect.Dy()/2 + int(math.Round(t.OffsetY*float64(i.Rect.Dy())))
	dest := image.Rect(cx-w/2, cy-h/2, cx+(w-w/2), cy+(h-h/2))
	p.PushClip(i.Rect)
	p.DrawImage(dest, i.ImageWidth, i.ImageHeight, i.Pixels)
	for index := range i.Markers {
		i.drawMarker(p, index, t)
	}
	p.PopClip()
}

func (i *ImageViewport) HitTest(pt image.Point) string {
	if pt.In(i.Rect) {
		return i.CompID
	}
	return ""
}

func (i *ImageViewport) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if i.Disabled || !pt.In(i.Rect) {
		return false
	}
	i.dragging = true
	i.dragMoved = false
	i.dragStart = pt
	i.start = i.normalized()
	i.pressedMarker = i.markerIndexAt(pt, i.start)
	state.ActiveID = i.CompID
	return true
}

func (i *ImageViewport) OnMouseMove(pt image.Point, _ *types.ApplicationState) bool {
	if !i.dragging {
		return false
	}
	if !i.dragMoved {
		dx, dy := pt.X-i.dragStart.X, pt.Y-i.dragStart.Y
		if dx*dx+dy*dy <= 36 {
			return true
		}
		i.dragMoved = true
		i.pressedMarker = -1
	}
	i.Transform = i.start
	if i.Rect.Dx() > 0 {
		i.Transform.OffsetX += float64(pt.X-i.dragStart.X) / float64(i.Rect.Dx())
	}
	if i.Rect.Dy() > 0 {
		i.Transform.OffsetY += float64(pt.Y-i.dragStart.Y) / float64(i.Rect.Dy())
	}
	i.Transform = i.normalized()
	return true
}

func (i *ImageViewport) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if !i.dragging {
		return false
	}
	i.dragging = false
	if i.dragMoved {
		i.commit(state)
		return true
	}
	if index := i.markerIndexAt(pt, i.normalized()); index >= 0 && index == i.pressedMarker {
		marker := i.Markers[index]
		if !marker.Disabled && i.OnMarker != nil {
			i.OnMarker(marker.ID, state)
		}
		return true
	}
	if i.OnActivate != nil && pt.In(i.Rect) {
		x := float64(pt.X-i.Rect.Min.X) / float64(maxInt(1, i.Rect.Dx()))
		y := float64(pt.Y-i.Rect.Min.Y) / float64(maxInt(1, i.Rect.Dy()))
		i.OnActivate(math.Max(0, math.Min(1, x)), math.Max(0, math.Min(1, y)), state)
	}
	return true
}

func (i *ImageViewport) OnMouseWheel(pt image.Point, delta int, state *types.ApplicationState) bool {
	if i.Disabled || !pt.In(i.Rect) || delta == 0 {
		return false
	}
	t := i.normalized()
	factor := math.Pow(1.2, float64(delta)/120)
	newScale := math.Max(i.minScale(), math.Min(i.maxScale(), t.Scale*factor))
	if newScale == t.Scale {
		return true
	}
	// Keep the content below the pointer stationary while scaling.
	fx := float64(pt.X-i.Rect.Min.X)/float64(maxInt(1, i.Rect.Dx())) - 0.5
	fy := float64(pt.Y-i.Rect.Min.Y)/float64(maxInt(1, i.Rect.Dy())) - 0.5
	ratio := newScale/t.Scale - 1
	t.OffsetX -= (fx - t.OffsetX) * ratio
	t.OffsetY -= (fy - t.OffsetY) * ratio
	t.Scale = newScale
	i.Transform = t
	i.commit(state)
	return true
}

func (i *ImageViewport) OnPanGesture(pt, delta image.Point, phase types.GesturePhase, state *types.ApplicationState) bool {
	if i.Disabled || !pt.In(i.Rect) {
		return false
	}
	switch phase {
	case types.GestureBegin:
		i.start = i.normalized()
	case types.GestureUpdate:
		t := i.normalized()
		t.OffsetX += float64(delta.X) / float64(maxInt(1, i.Rect.Dx()))
		t.OffsetY += float64(delta.Y) / float64(maxInt(1, i.Rect.Dy()))
		i.Transform = t
	case types.GestureEnd:
		i.commit(state)
	case types.GestureCancel:
		i.Transform = i.start
	}
	return true
}

func (i *ImageViewport) OnPinchGesture(pt, delta image.Point, scale float64, phase types.GesturePhase, state *types.ApplicationState) bool {
	if i.Disabled || !pt.In(i.Rect) || scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return false
	}
	switch phase {
	case types.GestureBegin:
		i.start = i.normalized()
	case types.GestureUpdate:
		t := i.normalized()
		oldScale := t.Scale
		newScale := math.Max(i.minScale(), math.Min(i.maxScale(), oldScale*scale))
		fx := float64(pt.X-i.Rect.Min.X)/float64(maxInt(1, i.Rect.Dx())) - 0.5
		fy := float64(pt.Y-i.Rect.Min.Y)/float64(maxInt(1, i.Rect.Dy())) - 0.5
		ratio := newScale/oldScale - 1
		t.OffsetX -= (fx - t.OffsetX) * ratio
		t.OffsetY -= (fy - t.OffsetY) * ratio
		t.OffsetX += float64(delta.X) / float64(maxInt(1, i.Rect.Dx()))
		t.OffsetY += float64(delta.Y) / float64(maxInt(1, i.Rect.Dy()))
		t.Scale = newScale
		i.Transform = t
	case types.GestureEnd:
		i.commit(state)
	case types.GestureCancel:
		i.Transform = i.start
	}
	return true
}

func (i *ImageViewport) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if i.Disabled {
		return false
	}
	switch {
	case char == '+' || char == '=' || key == 0x6B:
		return i.zoomAtCenter(1.25, state)
	case char == '-' || key == 0x6D:
		return i.zoomAtCenter(0.8, state)
	case char == '0':
		i.Transform = ImageTransform{Scale: 1}
		i.commit(state)
		return true
	}
	return false
}

func (i *ImageViewport) zoomAtCenter(factor float64, state *types.ApplicationState) bool {
	t := i.normalized()
	t.Scale = math.Max(i.minScale(), math.Min(i.maxScale(), t.Scale*factor))
	i.Transform = t
	i.commit(state)
	return true
}

func (i *ImageViewport) minScale() float64 {
	if i.MinScale > 0 {
		return i.MinScale
	}
	return 0.5
}

func (i *ImageViewport) maxScale() float64 {
	if i.MaxScale >= i.minScale() {
		return i.MaxScale
	}
	return math.Max(8, i.minScale())
}

func (i *ImageViewport) commit(state *types.ApplicationState) {
	i.Transform = i.normalized()
	if i.OnChange != nil {
		i.OnChange(i.Transform, state)
	}
}

func (i *ImageViewport) Semantics(_ *types.ApplicationState) semantics.Node {
	node := semantics.Node{ID: i.CompID, Role: semantics.RoleGroup, Name: i.Alt, Bounds: i.Rect}
	t := i.normalized()
	for index, marker := range i.Markers {
		if marker.ID == "" || !validMarker(marker) {
			continue
		}
		bounds := markerHitRect(i.markerCenter(index, t)).Intersect(i.Rect)
		actions := []semantics.Action(nil)
		if !marker.Disabled {
			actions = []semantics.Action{semantics.ActionInvoke}
		}
		node.Children = append(node.Children, semantics.Node{ID: i.markerSemanticID(marker.ID), Role: semantics.RoleButton,
			Name: marker.Label, Bounds: bounds, State: semantics.State{Disabled: marker.Disabled, Selected: marker.Selected}, Actions: actions})
	}
	return node
}

// PerformSemanticAction invokes an accessible marker descendant.
func (i *ImageViewport) PerformSemanticAction(targetID string, action semantics.Action, _ string, state *types.ApplicationState) bool {
	if action != semantics.ActionInvoke || i.Disabled {
		return false
	}
	for _, marker := range i.Markers {
		if targetID == i.markerSemanticID(marker.ID) && !marker.Disabled && i.OnMarker != nil {
			i.OnMarker(marker.ID, state)
			return true
		}
	}
	return false
}

func validMarker(marker ImageMarker) bool {
	return !math.IsNaN(marker.X) && !math.IsInf(marker.X, 0) && !math.IsNaN(marker.Y) && !math.IsInf(marker.Y, 0) &&
		marker.X >= 0 && marker.X <= 1 && marker.Y >= 0 && marker.Y <= 1
}

func (i *ImageViewport) markerCenter(index int, transform ImageTransform) image.Point {
	marker := i.Markers[index]
	base := containRect(i.Rect, i.ImageWidth, i.ImageHeight)
	baseX := float64(base.Min.X) + marker.X*float64(base.Dx())
	baseY := float64(base.Min.Y) + marker.Y*float64(base.Dy())
	cx := float64(i.Rect.Min.X+i.Rect.Dx()/2) + transform.OffsetX*float64(i.Rect.Dx())
	cy := float64(i.Rect.Min.Y+i.Rect.Dy()/2) + transform.OffsetY*float64(i.Rect.Dy())
	return image.Pt(int(math.Round(cx+(baseX-float64(i.Rect.Min.X+i.Rect.Dx()/2))*transform.Scale)),
		int(math.Round(cy+(baseY-float64(i.Rect.Min.Y+i.Rect.Dy()/2))*transform.Scale)))
}

func markerHitRect(center image.Point) image.Rectangle {
	return image.Rect(center.X-22, center.Y-22, center.X+22, center.Y+22)
}

func (i *ImageViewport) markerIndexAt(pt image.Point, transform ImageTransform) int {
	for index := len(i.Markers) - 1; index >= 0; index-- {
		marker := i.Markers[index]
		if marker.Disabled || !validMarker(marker) {
			continue
		}
		if pt.In(markerHitRect(i.markerCenter(index, transform)).Intersect(i.Rect)) {
			return index
		}
	}
	return -1
}

func (i *ImageViewport) markerSemanticID(id string) string { return i.CompID + "/marker/" + id }

func (i *ImageViewport) drawMarker(p types.Painter, index int, transform ImageTransform) {
	marker := i.Markers[index]
	if !validMarker(marker) {
		return
	}
	center := i.markerCenter(index, transform)
	if !center.In(i.Rect) {
		return
	}
	fill := markerColor(marker.Variant)
	if marker.Disabled {
		fill.A = 120
	}
	if marker.Selected {
		p.DrawRoundedRect(image.Rect(center.X-12, center.Y-12, center.X+12, center.Y+12), 12, color.RGBA{255, 255, 255, 255})
	}
	p.DrawRoundedRect(image.Rect(center.X-9, center.Y-9, center.X+9, center.Y+9), 9, fill)
	if marker.Label != "" && marker.Selected {
		labelWidth := minValueInt(len([]rune(marker.Label))*8+12, 220)
		box := image.Rect(center.X+14, center.Y-13, center.X+14+labelWidth, center.Y+13).Intersect(i.Rect)
		p.DrawRoundedRect(box, 5, color.RGBA{20, 23, 27, 235})
		p.DrawText(marker.Label, box.Min.X+6, box.Min.Y+18, color.RGBA{245, 247, 250, 255})
	}
}

func markerColor(variant ImageMarkerVariant) color.RGBA {
	switch variant {
	case ImageMarkerPrimary:
		return color.RGBA{86, 132, 247, 255}
	case ImageMarkerSecondary:
		return color.RGBA{126, 104, 196, 255}
	case ImageMarkerDanger:
		return color.RGBA{225, 79, 89, 255}
	case ImageMarkerSuccess:
		return color.RGBA{68, 177, 112, 255}
	case ImageMarkerWarning:
		return color.RGBA{224, 166, 55, 255}
	default:
		return color.RGBA{151, 157, 166, 255}
	}
}

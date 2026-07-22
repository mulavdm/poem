package layout

import (
	"image"

	"github.com/mulavdm/poem/pkg/render/types"
)

// OverlayAnchor names where a floating layer sits within the overlay box.
type OverlayAnchor int

const (
	AnchorTopLeft OverlayAnchor = iota
	AnchorTop
	AnchorTopRight
	AnchorLeft
	AnchorCenter
	AnchorRight
	AnchorBottomLeft
	AnchorBottom
	AnchorBottomRight
)

// OverlayLayer is one floating layer drawn above the base. Its content is sized
// to its own preferred measurement (never stretched to the box) and positioned
// by Anchor, held Inset logical pixels clear of the box edges it hugs.
type OverlayLayer struct {
	Anchor  OverlayAnchor
	Inset   int
	Content types.Component
}

// Overlay stacks floating layers on top of a Base within a single box. Base
// fills the box and alone determines the box's measured size; layers float
// above it without affecting layout, so a map keeps its full canvas while its
// controls sit in a corner. Layers paint in order (last on top) and are
// hit-tested before the base, so a control always wins the click over the
// canvas beneath it.
type Overlay struct {
	CompID string
	Rect   image.Rectangle
	Base   types.Component
	Layers []OverlayLayer
}

func (o *Overlay) ID() string    { return o.CompID }
func (o *Overlay) GetID() string { return o.CompID }

func (o *Overlay) Bounds() image.Rectangle {
	if o.Rect.Dx() == 0 || o.Rect.Dy() == 0 {
		if o.Base == nil {
			return o.Rect
		}
		size := o.Base.Bounds()
		return image.Rect(o.Rect.Min.X, o.Rect.Min.Y, o.Rect.Min.X+size.Dx(), o.Rect.Min.Y+size.Dy())
	}
	return o.Rect
}

// Measure delegates entirely to the base: floating layers never enlarge the box.
func (o *Overlay) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	if o.Base == nil {
		return types.MeasureResult{}
	}
	return types.MeasureComponent(o.Base, avail, state)
}

func (o *Overlay) ContentSize(avail image.Point, state *types.ApplicationState) image.Point {
	if o.Base == nil {
		return image.Point{}
	}
	return types.MeasureComponent(o.Base, avail, state).Preferred
}

func (o *Overlay) SetBounds(r image.Rectangle) {
	o.Rect = r
	o.performLayout(nil)
}

func (o *Overlay) performLayout(state *types.ApplicationState) {
	if o.Base != nil {
		o.Base.SetBounds(o.Rect)
	}
	for _, layer := range o.Layers {
		if layer.Content == nil {
			continue
		}
		layer.Content.SetBounds(o.layerRect(layer, state))
	}
}

// layerRect places a layer's preferred-sized content against its anchor, inset
// from the hugged edges and clamped to stay inside the box.
func (o *Overlay) layerRect(layer OverlayLayer, state *types.ApplicationState) image.Rectangle {
	inset := layer.Inset
	availW := o.Rect.Dx() - 2*inset
	availH := o.Rect.Dy() - 2*inset
	if availW < 0 {
		availW = 0
	}
	if availH < 0 {
		availH = 0
	}
	pref := types.MeasureComponent(layer.Content, image.Pt(availW, availH), state).Preferred
	w, h := pref.X, pref.Y
	if w > availW {
		w = availW
	}
	if h > availH {
		h = availH
	}

	left, right := o.Rect.Min.X+inset, o.Rect.Max.X-inset
	top, bottom := o.Rect.Min.Y+inset, o.Rect.Max.Y-inset

	var x, y int
	switch layer.Anchor {
	case AnchorTopLeft, AnchorLeft, AnchorBottomLeft:
		x = left
	case AnchorTop, AnchorCenter, AnchorBottom:
		x = o.Rect.Min.X + (o.Rect.Dx()-w)/2
	default: // right column
		x = right - w
	}
	switch layer.Anchor {
	case AnchorTopLeft, AnchorTop, AnchorTopRight:
		y = top
	case AnchorLeft, AnchorCenter, AnchorRight:
		y = o.Rect.Min.Y + (o.Rect.Dy()-h)/2
	default: // bottom row
		y = bottom - h
	}
	return image.Rect(x, y, x+w, y+h)
}

func (o *Overlay) Draw(p types.Painter, state *types.ApplicationState) {
	o.performLayout(state)
	if o.Base != nil {
		o.Base.Draw(p, state)
	}
	for _, layer := range o.Layers {
		if layer.Content != nil {
			layer.Content.Draw(p, state)
		}
	}
}

func (o *Overlay) HitTest(pt image.Point) string {
	if !pt.In(o.Rect) {
		return ""
	}
	for i := len(o.Layers) - 1; i >= 0; i-- {
		if o.Layers[i].Content == nil {
			continue
		}
		if id := o.Layers[i].Content.HitTest(pt); id != "" {
			return id
		}
	}
	if o.Base != nil {
		if id := o.Base.HitTest(pt); id != "" {
			return id
		}
	}
	return o.CompID
}

func (o *Overlay) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	for i := len(o.Layers) - 1; i >= 0; i-- {
		if o.Layers[i].Content != nil && o.Layers[i].Content.OnKey(key, char, state) {
			return true
		}
	}
	if o.Base != nil {
		return o.Base.OnKey(key, char, state)
	}
	return false
}

func (o *Overlay) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	for i := len(o.Layers) - 1; i >= 0; i-- {
		layer := o.Layers[i]
		if layer.Content != nil && pt.In(layer.Content.Bounds()) && layer.Content.OnMouseDown(pt, state) {
			return true
		}
	}
	if o.Base != nil && pt.In(o.Base.Bounds()) {
		return o.Base.OnMouseDown(pt, state)
	}
	return false
}

func (o *Overlay) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	for i := len(o.Layers) - 1; i >= 0; i-- {
		if o.Layers[i].Content != nil && o.Layers[i].Content.OnMouseUp(pt, state) {
			return true
		}
	}
	if o.Base != nil {
		return o.Base.OnMouseUp(pt, state)
	}
	return false
}

func (o *Overlay) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	for i := len(o.Layers) - 1; i >= 0; i-- {
		layer := o.Layers[i]
		if layer.Content != nil && pt.In(layer.Content.Bounds()) && layer.Content.OnMouseMove(pt, state) {
			return true
		}
	}
	if o.Base != nil && pt.In(o.Base.Bounds()) {
		return o.Base.OnMouseMove(pt, state)
	}
	return false
}

func (o *Overlay) Focusable() bool { return false }

func (o *Overlay) Walk(fn func(types.Component)) {
	fn(o)
	if o.Base != nil {
		o.Base.Walk(fn)
	}
	for _, layer := range o.Layers {
		if layer.Content != nil {
			layer.Content.Walk(fn)
		}
	}
}

func (o *Overlay) ChildComponents() []types.Component {
	children := make([]types.Component, 0, len(o.Layers)+1)
	if o.Base != nil {
		children = append(children, o.Base)
	}
	for _, layer := range o.Layers {
		if layer.Content != nil {
			children = append(children, layer.Content)
		}
	}
	return children
}

package components

import (
	"image"
	"image/color"

	"go_native_gpu_gui/pkg/render/types"
)

type ImageView struct {
	CompID      string
	Rect        image.Rectangle
	ImageWidth  int
	ImageHeight int
	Pixels      []byte
	BGColor     color.RGBA
	Rounding    int
}

func (i *ImageView) ID() string              { return i.CompID }
func (i *ImageView) GetID() string           { return i.CompID }
func (i *ImageView) Bounds() image.Rectangle { return i.Rect }
func (i *ImageView) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	width := avail.X
	height := avail.Y
	if width <= 0 {
		width = 320
	}
	if height <= 0 {
		height = 240
	}
	if i.ImageWidth > 0 && i.ImageHeight > 0 {
		rect := containRect(image.Rect(0, 0, width, height), i.ImageWidth, i.ImageHeight)
		width = rect.Dx()
		height = rect.Dy()
	}
	size := applyExplicitSize(explicitSize(i.Rect), image.Pt(width, height))
	return types.MeasureResult{
		Preferred: size,
		Min:       image.Pt(minValueInt(size.X, 120), minValueInt(size.Y, 120)),
	}
}
func (i *ImageView) SetBounds(r image.Rectangle) {
	i.Rect = r
}

func (i *ImageView) Draw(p types.Painter, state *types.ApplicationState) {
	if i.BGColor.A != 0 {
		p.DrawRoundedRect(i.Rect, i.Rounding, i.BGColor)
	}
	if len(i.Pixels) == 0 || i.ImageWidth <= 0 || i.ImageHeight <= 0 {
		return
	}

	dest := containRect(i.Rect, i.ImageWidth, i.ImageHeight)
	p.DrawImage(dest, i.ImageWidth, i.ImageHeight, i.Pixels)
}

func (i *ImageView) HitTest(pt image.Point) string {
	if pt.In(i.Rect) {
		return i.CompID
	}
	return ""
}

func (i *ImageView) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }
func (i *ImageView) OnMouseDown(pt image.Point, state *types.ApplicationState) bool  { return false }
func (i *ImageView) OnMouseUp(pt image.Point, state *types.ApplicationState) bool    { return false }
func (i *ImageView) OnMouseMove(pt image.Point, state *types.ApplicationState) bool  { return false }
func (i *ImageView) Focusable() bool                                                 { return false }
func (i *ImageView) Walk(fn func(types.Component))                                   { fn(i) }

func containRect(bounds image.Rectangle, imageWidth, imageHeight int) image.Rectangle {
	if imageWidth <= 0 || imageHeight <= 0 || bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return bounds
	}

	availableW := float64(bounds.Dx())
	availableH := float64(bounds.Dy())
	imageW := float64(imageWidth)
	imageH := float64(imageHeight)
	scale := minFloat(availableW/imageW, availableH/imageH)
	drawW := int(imageW * scale)
	drawH := int(imageH * scale)
	offsetX := bounds.Min.X + (bounds.Dx()-drawW)/2
	offsetY := bounds.Min.Y + (bounds.Dy()-drawH)/2
	return image.Rect(offsetX, offsetY, offsetX+drawW, offsetY+drawH)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

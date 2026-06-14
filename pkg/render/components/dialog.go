package components

import (
	"image"
	"image/color"

	"go_native_gpu_gui/pkg/render/types"
)

// Dialog is a reusable popup for alerts and prompts, wrapping Modal.
type Dialog struct {
	CompID        string
	Rect          image.Rectangle
	Title         string
	Message       string
	Buttons       []*Button
	BackdropColor color.RGBA
	CardColor     color.RGBA
	TitleColor    color.RGBA
	TextColor     color.RGBA

	// internal composed modal
	modal *Modal
}

func NewAlertDialog(id, title, message string, onDismiss func(state *types.ApplicationState)) *Dialog {
	btn := &Button{
		CompID:     id + "_btn_ok",
		Label:      "OK",
		BaseColor:  color.RGBA{80, 80, 80, 255},
		HoverColor: color.RGBA{120, 120, 120, 255},
		Rounding:   6,
		OnClick:    onDismiss,
	}

	return &Dialog{
		CompID:        id,
		Title:         title,
		Message:       message,
		Buttons:       []*Button{btn},
		BackdropColor: color.RGBA{0, 0, 0, 180},
		CardColor:     color.RGBA{40, 40, 40, 255},
		TitleColor:    color.RGBA{255, 255, 255, 255},
		TextColor:     color.RGBA{200, 200, 200, 255},
	}
}

func (d *Dialog) ID() string              { return d.CompID }
func (d *Dialog) GetID() string           { return d.CompID }
func (d *Dialog) Bounds() image.Rectangle { return d.Rect }
func (d *Dialog) SetBounds(r image.Rectangle) {
	d.Rect = r
	d.buildModal() // rebuild layout on resize
}

func (d *Dialog) buildModal() {
	cardW, cardH := 400, 250
	
	// Center the card in the rect
	cx := d.Rect.Min.X + d.Rect.Dx()/2
	cy := d.Rect.Min.Y + d.Rect.Dy()/2
	cardRect := image.Rect(cx-cardW/2, cy-cardH/2, cx+cardW/2, cy+cardH/2)

	card := &Panel{
		CompID:   d.CompID + "_card",
		Rect:     cardRect,
		BGColor:  d.CardColor,
		Rounding: 12,
	}

	titleLabel := &Label{
		CompID: d.CompID + "_title",
		Pos:    image.Pt(cardRect.Min.X+20, cardRect.Min.Y+20),
		Text:   d.Title,
		Color:  d.TitleColor,
	}

	msgPara := &Paragraph{
		CompID: d.CompID + "_msg",
		Rect:   image.Rect(cardRect.Min.X+20, cardRect.Min.Y+60, cardRect.Max.X-20, cardRect.Max.Y-80),
		Text:      d.Message,
		BaseColor: d.TextColor,
	}

	children := []types.Component{card, titleLabel, msgPara}

	// Layout buttons horizontally at the bottom right
	btnX := cardRect.Max.X - 20
	btnY := cardRect.Max.Y - 60
	for i := len(d.Buttons) - 1; i >= 0; i-- {
		b := d.Buttons[i]
		// Hardcode preferred sizes for typical dialog buttons or measure them
		bw, bh := 100, 40
		btnX -= bw
		b.SetBounds(image.Rect(btnX, btnY, btnX+bw, btnY+bh))
		children = append(children, b)
		btnX -= 10 // spacing
	}

	d.modal = &Modal{
		CompID:        d.CompID + "_modal",
		Rect:          d.Rect,
		CardRect:      cardRect,
		BackdropColor: d.BackdropColor,
		Children:      children,
	}
}

func (d *Dialog) Draw(p types.Painter, state *types.ApplicationState) {
	if d.modal == nil || d.modal.Rect != d.Rect {
		d.buildModal()
	}
	d.modal.Draw(p, state)
}

func (d *Dialog) HitTest(pt image.Point) string {
	if d.modal != nil {
		if hit := d.modal.HitTest(pt); hit != "" {
			// Do not return modal's own id if it doesn't hit a child,
			// instead we want to block underneath but maybe we return dialog ID
			if hit == d.modal.CompID {
				return d.CompID
			}
			return hit
		}
	}
	return ""
}

func (d *Dialog) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if d.modal != nil {
		return d.modal.OnKey(key, char, state)
	}
	return false
}

func (d *Dialog) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if d.modal != nil {
		return d.modal.OnMouseDown(pt, state)
	}
	return false
}

func (d *Dialog) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if d.modal != nil {
		return d.modal.OnMouseUp(pt, state)
	}
	return false
}

func (d *Dialog) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if d.modal != nil {
		return d.modal.OnMouseMove(pt, state)
	}
	return false
}

func (d *Dialog) Focusable() bool { return false }
func (d *Dialog) Walk(fn func(types.Component)) {
	fn(d)
	if d.modal != nil {
		for _, child := range d.modal.Children {
			child.Walk(fn)
		}
	}
}

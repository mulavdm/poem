package components

import (
	"image"
	"image/color"
	"strings"

	"go_native_gpu_gui/pkg/render/semantics"
	"go_native_gpu_gui/pkg/render/theme"
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
	UseTheme      bool

	// internal composed modal
	modal         *Modal
	themeRevision uint64
	themeApplied  bool
}

func NewAlertDialog(id, title, message string, onDismiss func(state *types.ApplicationState)) *Dialog {
	btn := NewButton(id+"_btn_ok", "OK", onDismiss)
	btn.Variant = theme.VariantPrimary

	return &Dialog{
		CompID:        id,
		Title:         title,
		Message:       message,
		Buttons:       []*Button{btn},
		BackdropColor: color.RGBA{0, 0, 0, 180},
		CardColor:     color.RGBA{40, 40, 40, 255},
		TitleColor:    color.RGBA{255, 255, 255, 255},
		TextColor:     color.RGBA{200, 200, 200, 255},
		UseTheme:      true,
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

	children := []types.Component{card, titleLabel}
	for index, line := range dialogMessageLines(d.Message, 44) {
		message := NewLabel(d.CompID+"_msg_"+intString(index), line)
		message.SetBounds(image.Rect(cardRect.Min.X+20, cardRect.Min.Y+60+index*24, cardRect.Max.X-20, cardRect.Min.Y+84+index*24))
		message.Role = TextMuted
		message.Typography = TypographyBody
		message.UseTheme = true
		children = append(children, message)
	}

	// Layout buttons horizontally at the bottom right
	btnX := cardRect.Max.X - 20
	btnY := cardRect.Max.Y - 60
	for i := len(d.Buttons) - 1; i >= 0; i-- {
		b := d.Buttons[i]
		// Size actions from their labels so translated or descriptive text is not
		// silently clipped. The dialog retains a predictable minimum target size.
		bw, bh := maxInt(100, len([]rune(b.Label))*9+28), 40
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

func dialogMessageLines(text string, maxRunes int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := make([]string, 0, 2)
	line := words[0]
	for _, word := range words[1:] {
		candidate := line + " " + word
		if len([]rune(candidate)) <= maxRunes {
			line = candidate
			continue
		}
		lines = append(lines, line)
		line = word
	}
	return append(lines, line)
}

func (d *Dialog) Draw(p types.Painter, state *types.ApplicationState) {
	d.applyTheme(state)
	if d.modal == nil || d.modal.Rect != d.Rect {
		d.buildModal()
	}
	d.modal.Draw(p, state)
}

func (d *Dialog) applyTheme(state *types.ApplicationState) {
	_, revision := state.CurrentTheme()
	if themed(d.UseTheme, state) && (!d.themeApplied || revision != d.themeRevision) {
		current := activeTheme(state)
		d.BackdropColor, d.CardColor = current.Colors.Overlay, current.Colors.SurfaceRaised
		d.TitleColor, d.TextColor = current.Colors.Text, current.Colors.TextMuted
		d.themeRevision = revision
		d.themeApplied = true
		d.modal = nil
	}
}

func (d *Dialog) Semantics(state *types.ApplicationState) semantics.Node {
	return semantics.Node{ID: d.CompID, Role: semantics.RoleDialog, Name: d.Title, Description: d.Message, Bounds: d.Rect}
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

func (d *Dialog) ChildComponents() []types.Component {
	if d.modal == nil {
		d.buildModal()
	}
	if d.modal == nil {
		return nil
	}
	return d.modal.Children
}

package components

import (
	"image"
	"image/color"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/theme"
	"github.com/mulavdm/poem/pkg/render/types"
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
	OnDismiss     func(state *types.ApplicationState)

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
		OnDismiss:     onDismiss,
	}
}

func (d *Dialog) ID() string              { return d.CompID }
func (d *Dialog) GetID() string           { return d.CompID }
func (d *Dialog) Bounds() image.Rectangle { return d.Rect }
func (d *Dialog) SetBounds(r image.Rectangle) {
	d.Rect = r
	d.buildModal() // rebuild layout on resize
}

// dialogDefaultColors returns default, always-visible colors for whichever
// of the four fields are still Go zero-value (fully transparent). A bare
// &Dialog{...} literal with UseTheme left false never runs applyTheme's
// color population, so without this fallback its card, title, and backdrop
// render invisible -- only buttons (which default UseTheme=true themselves)
// would show. This only fills in colors nobody set; an applied theme or an
// explicitly-chosen color is never overwritten, since neither is zero-value.
func dialogDefaultColors(backdrop, card, title, text color.RGBA) (color.RGBA, color.RGBA, color.RGBA, color.RGBA) {
	if backdrop.A == 0 {
		backdrop = color.RGBA{0, 0, 0, 180}
	}
	if card.A == 0 {
		card = color.RGBA{40, 40, 40, 255}
	}
	if title.A == 0 {
		title = color.RGBA{255, 255, 255, 255}
	}
	if text.A == 0 {
		text = color.RGBA{200, 200, 200, 255}
	}
	return backdrop, card, title, text
}

func (d *Dialog) buildModal() {
	const pad = 20
	const messageTop, messageLineH, messageCharW = 60, 24, 8
	const btnH, btnGap, btnRowGap = 40, 10, 10

	d.BackdropColor, d.CardColor, d.TitleColor, d.TextColor = dialogDefaultColors(d.BackdropColor, d.CardColor, d.TitleColor, d.TextColor)

	// cardW is a typical dialog width, but shrinks to fit narrow windows
	// rather than overflowing them.
	cardW := 400
	if available := d.Rect.Dx() - 2*pad; available > 0 {
		cardW = minInt(cardW, maxInt(240, available))
	}

	// estimatedLines uses the same wrapPlainText algorithm the message
	// Label itself wraps with at Draw time (see Label.Draw), rather than an
	// independent rune-count heuristic -- the two disagreeing is what
	// previously caused a per-pre-wrapped-line Label to internally re-wrap
	// into more lines than its fixed 24px slot held, overlapping the label
	// above or below it. The message now gets one label spanning a rect
	// sized to this estimate, so a residual mismatch (state's real font
	// metrics aren't known until Draw) just adds a little breathing room
	// rather than causing an overlap.
	estimatedLines := maxInt(1, wrapLineCount(d.Message, maxInt(4, (cardW-2*pad)/messageCharW)))
	messageH := estimatedLines * messageLineH

	// Lay out buttons left-to-right, wrapping onto a new row instead of
	// overflowing past the card edge when there are more buttons than fit
	// one row -- the card grows to fit however many rows that takes, rather
	// than a fixed height clipping or overlapping button rows.
	type placedButton struct {
		btn        *Button
		x, y, w, h int
	}
	placed := make([]placedButton, 0, len(d.Buttons))
	x, y := pad, messageTop+messageH+20
	for _, b := range d.Buttons {
		bw := maxInt(100, len([]rune(b.Text))*9+28)
		if x != pad && x+bw > cardW-pad {
			x, y = pad, y+btnH+btnRowGap
		}
		placed = append(placed, placedButton{b, x, y, bw, btnH})
		x += bw + btnGap
	}
	cardH := maxInt(160, y+btnH+pad)

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
	if d.Message != "" {
		message := NewLabel(d.CompID+"_msg", d.Message)
		message.SetBounds(image.Rect(cardRect.Min.X+20, cardRect.Min.Y+messageTop, cardRect.Max.X-20, cardRect.Min.Y+messageTop+messageH))
		message.Role = TextMuted
		message.Typography = TypographyBody
		message.UseTheme = true
		children = append(children, message)
	}

	for _, p := range placed {
		// Size actions from their labels so translated or descriptive text
		// is not silently clipped. The dialog retains a predictable minimum
		// target size.
		p.btn.SetBounds(image.Rect(cardRect.Min.X+p.x, cardRect.Min.Y+p.y, cardRect.Min.X+p.x+p.w, cardRect.Min.Y+p.y+p.h))
		children = append(children, p.btn)
	}

	d.modal = &Modal{
		CompID:        d.CompID + "_modal",
		Rect:          d.Rect,
		CardRect:      cardRect,
		BackdropColor: d.BackdropColor,
		Children:      children,
		OnDismiss:     d.OnDismiss,
	}
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

package components

import (
	"image"
	"image/color"
	"time"

	"go_native_gpu_gui/pkg/render/types"
)

// Panel is a simple container with a background color
type Panel struct {
	CompID   string
	Rect     image.Rectangle
	BGColor  color.RGBA
	Rounding int
}

func (p *Panel) ID() string { return p.CompID }
func (p *Panel) GetID() string { return p.CompID }
func (p *Panel) Bounds() image.Rectangle { return p.Rect }
func (p *Panel) Draw(pnt types.Painter, state *types.ApplicationState) {
	pnt.SetGlow(2.0) // Subtle ambient glow for panels
	pnt.DrawRoundedRect(p.Rect, p.Rounding, p.BGColor)
	pnt.SetGlow(0)
}
func (p *Panel) SetBounds(r image.Rectangle) { p.Rect = r }
func (p *Panel) HitTest(pt image.Point) string {
	if pt.In(p.Rect) {
		return p.CompID
	}
	return ""
}
func (p *Panel) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }
func (p *Panel) OnMouseDown(pt image.Point, state *types.ApplicationState) bool { return false }
func (p *Panel) OnMouseUp(pt image.Point, state *types.ApplicationState) bool { return false }
func (p *Panel) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

func (p *Panel) Focusable() bool { return false }
func (p *Panel) Walk(fn func(types.Component)) { fn(p) }

// GlassPanel is a premium panel with real-time blur background
type GlassPanel struct {
	Panel
	Opacity uint8
}

func (p *GlassPanel) Draw(pnt types.Painter, state *types.ApplicationState) {
	pnt.SetGlass(state.GlassEnabled)
	pnt.SetShadow(0, 4, 15) // Deep shadow for depth
	col := p.BGColor
	col.A = p.Opacity
	pnt.DrawRoundedRect(p.Rect, p.Rounding, col)
	pnt.SetShadow(0, 0, 0) // Reset shadow
	pnt.SetGlass(false)
}

func (p *GlassPanel) Walk(fn func(types.Component)) { fn(p) }

// Button is an interactive element with hover states
type Button struct {
	CompID      string
	Rect        image.Rectangle
	Label       string
	BaseColor   color.RGBA
	HoverColor  color.RGBA
	Rounding    int
	OnClick     func(state *types.ApplicationState)
}

func (b *Button) ID() string { return b.CompID }
func (b *Button) GetID() string { return b.CompID }
func (b *Button) Bounds() image.Rectangle { return b.Rect }
func (b *Button) Draw(pnt types.Painter, state *types.ApplicationState) {
	c := b.BaseColor
	if state.HoveredID == b.CompID {
		c = b.HoverColor
		pnt.SetGlow(8.0) // Intense glow on hover
		pnt.SetShadow(0, 4, 12)
		state.CursorID = state.HandCursor
	} else {
		pnt.SetShadow(0, 2, 8)
	}
	pnt.DrawRoundedRect(b.Rect, b.Rounding, c)
	pnt.SetGlow(0)
	pnt.SetShadow(0, 0, 0)
	
	// Center text manually for now
	tx := b.Rect.Min.X + (b.Rect.Dx()/2) - (len(b.Label)*4)
	ty := b.Rect.Min.Y + (b.Rect.Dy()/2) + 5
	pnt.DrawText(b.Label, tx, ty, color.RGBA{255, 255, 255, 255})
}
func (b *Button) SetBounds(r image.Rectangle) { b.Rect = r }
func (b *Button) HitTest(pt image.Point) string {
	if pt.In(b.Rect) {
		return b.CompID
	}
	return ""
}
func (b *Button) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }
func (b *Button) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if b.OnClick != nil {
		b.OnClick(state)
		return true
	}
	return false
}
func (b *Button) OnMouseUp(pt image.Point, state *types.ApplicationState) bool { return false }
func (b *Button) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

func (b *Button) Focusable() bool { return true }
func (b *Button) Walk(fn func(types.Component)) { fn(b) }

// Label is a simple text element
type Label struct {
	CompID string
	Pos    image.Point
	Text   string
	Color  color.RGBA
}

func (l *Label) ID() string { return l.CompID }
func (l *Label) GetID() string { return l.CompID }
func (l *Label) Bounds() image.Rectangle {
	return image.Rect(l.Pos.X, l.Pos.Y, l.Pos.X+len(l.Text)*10, l.Pos.Y+20)
}
func (l *Label) Draw(pnt types.Painter, state *types.ApplicationState) {
	pnt.DrawText(l.Text, l.Pos.X, l.Pos.Y, l.Color)
}
func (l *Label) SetBounds(r image.Rectangle) { l.Pos = r.Min }
func (l *Label) HitTest(pt image.Point) string {
	if pt.In(l.Bounds()) {
		return l.CompID
	}
	return ""
}
func (l *Label) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }
func (l *Label) OnMouseDown(pt image.Point, state *types.ApplicationState) bool { return false }
func (l *Label) OnMouseUp(pt image.Point, state *types.ApplicationState) bool { return false }
func (l *Label) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

func (l *Label) Focusable() bool { return false }
func (l *Label) Walk(fn func(types.Component)) { fn(l) }

// DynamicLabel is a label that fetches its text from a function
type DynamicLabel struct {
	CompID  string
	Pos     image.Point
	GetText func(state *types.ApplicationState) string
	Color   color.RGBA
}

func (dl *DynamicLabel) ID() string { return dl.CompID }
func (dl *DynamicLabel) GetID() string { return dl.CompID }
func (dl *DynamicLabel) Bounds() image.Rectangle {
	return image.Rect(dl.Pos.X, dl.Pos.Y, dl.Pos.X+300, dl.Pos.Y+20)
}
func (dl *DynamicLabel) Draw(pnt types.Painter, state *types.ApplicationState) {
	if dl.GetText != nil {
		pnt.DrawText(dl.GetText(state), dl.Pos.X, dl.Pos.Y, dl.Color)
	}
}
func (dl *DynamicLabel) SetBounds(r image.Rectangle) { dl.Pos = r.Min }
func (dl *DynamicLabel) HitTest(pt image.Point) string {
	if pt.In(dl.Bounds()) {
		return dl.CompID
	}
	return ""
}
func (dl *DynamicLabel) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }
func (dl *DynamicLabel) OnMouseDown(pt image.Point, state *types.ApplicationState) bool { return false }
func (dl *DynamicLabel) OnMouseUp(pt image.Point, state *types.ApplicationState) bool { return false }
func (dl *DynamicLabel) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

func (dl *DynamicLabel) Focusable() bool { return false }
func (dl *DynamicLabel) Walk(fn func(types.Component)) { fn(dl) }

// TextInput is an editable text field
type TextInput struct {
	CompID      string
	Rect        image.Rectangle
	Text        string
	Placeholder string
	BGColor     color.RGBA
	TextColor   color.RGBA
	Rounding    int
}

func (t *TextInput) ID() string { return t.CompID }
func (t *TextInput) GetID() string { return t.CompID }
func (t *TextInput) Bounds() image.Rectangle { return t.Rect }
func (t *TextInput) SetBounds(r image.Rectangle) { t.Rect = r }
func (t *TextInput) HitTest(pt image.Point) string {
	if pt.In(t.Rect) {
		return t.CompID
	}
	return ""
}

func (t *TextInput) Draw(pnt types.Painter, state *types.ApplicationState) {
	// Draw background box
	pnt.DrawRoundedRect(t.Rect, t.Rounding, t.BGColor)
	
	if state.HoveredID == t.CompID {
		state.CursorID = state.IBeamCursor
	}
	
	// Draw text or placeholder
	disp := t.Text
	col := t.TextColor
	if disp == "" {
		disp = t.Placeholder
		col.A = 120 // Fade placeholder
	}
	
	pnt.DrawText(disp, t.Rect.Min.X + 10, t.Rect.Min.Y + 20, col)
	
	// Draw cursor if focused
	if state.FocusedID == t.CompID {
		// Simple blinking logic based on time
		if (time.Since(state.StartTime).Milliseconds() / 500) % 2 == 0 {
			cursorX := t.Rect.Min.X + 10 + (len(t.Text) * 7)
			pnt.FillRect(image.Rect(cursorX, t.Rect.Min.Y + 8, cursorX+2, t.Rect.Min.Y + 25), t.TextColor)
		}
	}
}

func (t *TextInput) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID != t.CompID {
		return false
	}
	
	const VK_BACK = 0x08
	
	if key == VK_BACK {
		if len(t.Text) > 0 {
			t.Text = t.Text[:len(t.Text)-1]
		}
		return true
	}
	
	if char >= 32 && char <= 126 { // Printable ASCII
		t.Text += string(char)
		return true
	}
	
	return false
}

func (t *TextInput) OnMouseDown(pt image.Point, state *types.ApplicationState) bool { return false }
func (t *TextInput) OnMouseUp(pt image.Point, state *types.ApplicationState) bool { return false }
func (t *TextInput) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

func (t *TextInput) Focusable() bool { return true }
func (t *TextInput) Walk(fn func(types.Component)) { fn(t) }

// Slider is a range input element
type Slider struct {
	CompID     string
	Rect       image.Rectangle
	Min, Max   float32
	Value      float32
	TrackColor color.RGBA
	ThumbColor color.RGBA
}

func (s *Slider) ID() string { return s.CompID }
func (s *Slider) GetID() string { return s.CompID }
func (s *Slider) Bounds() image.Rectangle { return s.Rect }
func (s *Slider) SetBounds(r image.Rectangle) { s.Rect = r }
func (s *Slider) HitTest(pt image.Point) string {
	if pt.In(s.Rect) {
		return s.CompID
	}
	return ""
}
func (s *Slider) Draw(pnt types.Painter, state *types.ApplicationState) {
	if state.HoveredID == s.CompID {
		state.CursorID = state.HandCursor
	}

	// Draw track
	trackH := 4
	trackRect := image.Rect(s.Rect.Min.X, s.Rect.Min.Y + s.Rect.Dy()/2 - trackH/2, s.Rect.Max.X, s.Rect.Min.Y + s.Rect.Dy()/2 + trackH/2)
	pnt.FillRect(trackRect, s.TrackColor)
	
	// Draw thumb
	thumbW := 12
	percent := (s.Value - s.Min) / (s.Max - s.Min)
	thumbX := s.Rect.Min.X + int(percent * float32(s.Rect.Dx() - thumbW))
	thumbRect := image.Rect(thumbX, s.Rect.Min.Y, thumbX + thumbW, s.Rect.Max.Y)
	
	col := s.ThumbColor
	if state.ActiveID == s.CompID {
		col.A = 200 // Brighter when dragging
	}
	pnt.DrawRoundedRect(thumbRect, 4, col)
}

func (s *Slider) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }

func (s *Slider) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	state.ActiveID = s.CompID
	s.updateValue(pt.X)
	return true
}

func (s *Slider) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	state.ActiveID = ""
	return true
}

func (s *Slider) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID == s.CompID {
		s.updateValue(pt.X)
		return true
	}
	return false
}

func (s *Slider) updateValue(mouseX int) {
	percent := float32(mouseX - s.Rect.Min.X) / float32(s.Rect.Dx())
	if percent < 0 { percent = 0 }
	if percent > 1 { percent = 1 }
	s.Value = s.Min + percent * (s.Max - s.Min)
}

func (s *Slider) Focusable() bool { return true }
func (s *Slider) Walk(fn func(types.Component)) { fn(s) }

// ParticleComponent is a UI element wrapping the ParticleSystem
type ParticleComponent struct {
	CompID string
	System *types.ParticleSystem
}

func (c *ParticleComponent) GetID() string { return c.CompID }
func (c *ParticleComponent) ID() string    { return c.CompID }
func (c *ParticleComponent) Bounds() image.Rectangle { return c.System.Bounds }
func (c *ParticleComponent) SetBounds(r image.Rectangle) { c.System.Bounds = r }
func (c *ParticleComponent) HitTest(pt image.Point) string { return "" }
func (c *ParticleComponent) Focusable() bool { return false }
func (c *ParticleComponent) Walk(fn func(types.Component)) { fn(c) }

func (c *ParticleComponent) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }
func (c *ParticleComponent) OnMouseDown(pt image.Point, state *types.ApplicationState) bool { return false }
func (c *ParticleComponent) OnMouseUp(pt image.Point, state *types.ApplicationState) bool { return false }
func (c *ParticleComponent) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

func (c *ParticleComponent) Draw(p types.Painter, state *types.ApplicationState) {
	if c.System != nil {
		c.System.Draw(p, state)
	}
}

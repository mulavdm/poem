package components

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
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

func (p *Panel) ID() string              { return p.CompID }
func (p *Panel) GetID() string           { return p.CompID }
func (p *Panel) Bounds() image.Rectangle { return p.Rect }
func (p *Panel) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	size := applyExplicitSize(explicitSize(p.Rect), avail)
	return types.MeasureResult{Preferred: size, Min: size}
}
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
func (p *Panel) OnMouseDown(pt image.Point, state *types.ApplicationState) bool  { return false }
func (p *Panel) OnMouseUp(pt image.Point, state *types.ApplicationState) bool    { return false }
func (p *Panel) OnMouseMove(pt image.Point, state *types.ApplicationState) bool  { return false }

func (p *Panel) Focusable() bool               { return false }
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
	CompID     string
	Rect       image.Rectangle
	Label      string
	BaseColor  color.RGBA
	HoverColor color.RGBA
	Rounding   int
	OnClick    func(state *types.ApplicationState)
}

func (b *Button) ID() string              { return b.CompID }
func (b *Button) GetID() string           { return b.CompID }
func (b *Button) Bounds() image.Rectangle { return b.Rect }
func (b *Button) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	charW := 8
	if state != nil && state.FontCharWidth > 0 {
		charW = state.FontCharWidth
	}
	labelWidth := len([]rune(strings.TrimSpace(b.Label))) * charW
	width := maxInt(140, labelWidth+24)
	if avail.X > 0 {
		width = clampInt(width, 90, avail.X)
	}
	height := 38
	size := applyExplicitSize(explicitSize(b.Rect), image.Pt(width, height))
	return types.MeasureResult{
		Preferred: size,
		Min:       image.Pt(minValueInt(size.X, 90), size.Y),
	}
}
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

	// Draw glowing focus outline ring if focused
	if state.FocusedID == b.CompID {
		pnt.SetGlow(6.0)
		pnt.DrawRoundedRect(image.Rect(b.Rect.Min.X-2, b.Rect.Min.Y-2, b.Rect.Max.X+2, b.Rect.Max.Y+2), b.Rounding+2, color.RGBA{0, 150, 255, 200})
		pnt.SetGlow(0)
	}

	pnt.DrawRoundedRect(b.Rect, b.Rounding, c)
	pnt.SetGlow(0)
	pnt.SetShadow(0, 0, 0)

	charW := state.FontCharWidth
	if charW <= 0 {
		charW = 8
	}
	label := fitButtonLabel(b.Label, b.Rect.Dx(), charW)
	labelWidth := len([]rune(label)) * charW
	tx := b.Rect.Min.X + 10
	if labelWidth <= b.Rect.Dx()-20 {
		// Center short labels while keeping long labels anchored visibly inside the button.
		tx = b.Rect.Min.X + (b.Rect.Dx() / 2) - (labelWidth / 2)
	}
	ty := b.Rect.Min.Y + (b.Rect.Dy() / 2) + 5
	pnt.PushClip(b.Rect)
	pnt.DrawText(label, tx, ty, color.RGBA{255, 255, 255, 255})
	pnt.PopClip()
}
func (b *Button) SetBounds(r image.Rectangle) { b.Rect = r }
func (b *Button) HitTest(pt image.Point) string {
	if pt.In(b.Rect) {
		return b.CompID
	}
	return ""
}
func (b *Button) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID == b.CompID && (key == 13 || key == 32) { // Enter or Space
		if b.OnClick != nil {
			b.OnClick(state)
			return true
		}
	}
	return false
}
func (b *Button) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if b.OnClick != nil {
		b.OnClick(state)
		return true
	}
	return false
}
func (b *Button) OnMouseUp(pt image.Point, state *types.ApplicationState) bool   { return false }
func (b *Button) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

func (b *Button) Focusable() bool               { return true }
func (b *Button) Walk(fn func(types.Component)) { fn(b) }

func fitButtonLabel(label string, width, charW int) string {
	if strings.TrimSpace(label) == "" || width <= 0 || charW <= 0 {
		return label
	}

	maxChars := (width - 20) / charW
	if maxChars <= 0 {
		return ""
	}

	runes := []rune(label)
	if len(runes) <= maxChars {
		return label
	}
	if maxChars <= 3 {
		return string(runes[:maxChars])
	}
	return string(runes[:maxChars-3]) + "..."
}

const (
	defaultFontHeight   = 20
	defaultFontBaseline = 15
)

// Label is a simple text element
type Label struct {
	CompID string
	Pos    image.Point
	Text   string
	Color  color.RGBA

	rect   image.Rectangle
}

func (l *Label) ID() string    { return l.CompID }
func (l *Label) GetID() string { return l.CompID }
func (l *Label) Bounds() image.Rectangle {
	if !l.rect.Empty() {
		return l.rect
	}
	return image.Rect(l.Pos.X, l.Pos.Y, l.Pos.X+len(l.Text)*10, l.Pos.Y+defaultFontHeight)
}
func (l *Label) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	charW := 8
	if state != nil && state.FontCharWidth > 0 {
		charW = state.FontCharWidth
	}
	size := measurePlainText(l.Text, charW, defaultFontHeight)
	return types.MeasureResult{Preferred: size, Min: size}
}
func (l *Label) Draw(pnt types.Painter, state *types.ApplicationState) {
	pnt.DrawText(l.Text, l.Pos.X, l.Pos.Y, l.Color)
}
func (l *Label) SetBounds(r image.Rectangle) {
	l.rect = r
	l.Pos = image.Point{X: r.Min.X, Y: r.Min.Y + (r.Dy()-defaultFontHeight)/2 + defaultFontBaseline}
}
func (l *Label) HitTest(pt image.Point) string {
	if pt.In(l.Bounds()) {
		return l.CompID
	}
	return ""
}
func (l *Label) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }
func (l *Label) OnMouseDown(pt image.Point, state *types.ApplicationState) bool  { return false }
func (l *Label) OnMouseUp(pt image.Point, state *types.ApplicationState) bool    { return false }
func (l *Label) OnMouseMove(pt image.Point, state *types.ApplicationState) bool  { return false }

func (l *Label) Focusable() bool               { return false }
func (l *Label) Walk(fn func(types.Component)) { fn(l) }

// DynamicLabel is a label that fetches its text from a function
type DynamicLabel struct {
	CompID  string
	Pos     image.Point
	GetText func(state *types.ApplicationState) string
	Color   color.RGBA

	rect    image.Rectangle
}

func (dl *DynamicLabel) ID() string    { return dl.CompID }
func (dl *DynamicLabel) GetID() string { return dl.CompID }
func (dl *DynamicLabel) Bounds() image.Rectangle {
	if !dl.rect.Empty() {
		return dl.rect
	}
	return image.Rect(dl.Pos.X, dl.Pos.Y, dl.Pos.X+300, dl.Pos.Y+defaultFontHeight)
}
func (dl *DynamicLabel) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	charW := 8
	if state != nil && state.FontCharWidth > 0 {
		charW = state.FontCharWidth
	}
	text := ""
	if dl.GetText != nil && state != nil {
		text = dl.GetText(state)
	}
	width := maxInt(120, len([]rune(text))*charW)
	if width == 120 && avail.X > 0 {
		width = min(width, avail.X)
	}
	return types.MeasureResult{
		Preferred: image.Pt(width, defaultFontHeight),
		Min:       image.Pt(minValueInt(width, 120), defaultFontHeight),
	}
}
func (dl *DynamicLabel) Draw(pnt types.Painter, state *types.ApplicationState) {
	if dl.GetText != nil {
		pnt.DrawText(dl.GetText(state), dl.Pos.X, dl.Pos.Y, dl.Color)
	}
}
func (dl *DynamicLabel) SetBounds(r image.Rectangle) {
	dl.rect = r
	dl.Pos = image.Point{X: r.Min.X, Y: r.Min.Y + (r.Dy()-defaultFontHeight)/2 + defaultFontBaseline}
}
func (dl *DynamicLabel) HitTest(pt image.Point) string {
	if pt.In(dl.Bounds()) {
		return dl.CompID
	}
	return ""
}
func (dl *DynamicLabel) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	return false
}
func (dl *DynamicLabel) OnMouseDown(pt image.Point, state *types.ApplicationState) bool { return false }
func (dl *DynamicLabel) OnMouseUp(pt image.Point, state *types.ApplicationState) bool   { return false }
func (dl *DynamicLabel) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

func (dl *DynamicLabel) Focusable() bool               { return false }
func (dl *DynamicLabel) Walk(fn func(types.Component)) { fn(dl) }

// TextInput is an editable text field
type TextInput struct {
	CompID      string
	Rect        image.Rectangle
	Text        string
	Placeholder string
	Masked      bool
	BGColor     color.RGBA
	TextColor   color.RGBA
	Rounding    int
	CursorIndex int
	OnSubmit    func(text string, state *types.ApplicationState)
}

func (t *TextInput) ID() string              { return t.CompID }
func (t *TextInput) GetID() string           { return t.CompID }
func (t *TextInput) Bounds() image.Rectangle { return t.Rect }
func (t *TextInput) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	charW := 8
	if state != nil && state.FontCharWidth > 0 {
		charW = state.FontCharWidth
	}
	content := t.Text
	if strings.TrimSpace(content) == "" {
		content = t.Placeholder
	}
	width := maxInt(160, len([]rune(content))*charW+20)
	if avail.X > 0 {
		width = clampInt(width, 120, avail.X)
	}
	height := 34
	size := applyExplicitSize(explicitSize(t.Rect), image.Pt(width, height))
	return types.MeasureResult{
		Preferred: size,
		Min:       image.Pt(minValueInt(size.X, 120), size.Y),
	}
}
func (t *TextInput) SetBounds(r image.Rectangle) { t.Rect = r }
func (t *TextInput) HitTest(pt image.Point) string {
	if pt.In(t.Rect) {
		return t.CompID
	}
	return ""
}

func (t *TextInput) Draw(pnt types.Painter, state *types.ApplicationState) {
	// Restore state if present
	if state.TextInputValues != nil {
		if val, ok := state.TextInputValues[t.CompID]; ok {
			t.Text = val
		} else {
			state.TextInputValues[t.CompID] = t.Text
		}
		if cursorVal, ok := state.TextInputValues[t.CompID+"_cursor"]; ok {
			var restoredCursor int
			if n, err := fmt.Sscanf(cursorVal, "%d", &restoredCursor); err == nil && n == 1 {
				t.CursorIndex = restoredCursor
			}
		}
	}
	runes := []rune(t.Text)
	if t.CursorIndex < 0 || t.CursorIndex > len(runes) {
		t.CursorIndex = len(runes)
	}

	// Draw glowing focus outline ring if focused
	if state.FocusedID == t.CompID {
		pnt.SetGlow(6.0)
		pnt.DrawRoundedRect(image.Rect(t.Rect.Min.X-2, t.Rect.Min.Y-2, t.Rect.Max.X+2, t.Rect.Max.Y+2), t.Rounding+2, color.RGBA{0, 150, 255, 200})
		pnt.SetGlow(0)
	}

	// Draw background box
	pnt.DrawRoundedRect(t.Rect, t.Rounding, t.BGColor)

	if state.HoveredID == t.CompID {
		state.CursorID = state.IBeamCursor
	}

	// Draw text or placeholder
	disp := t.Text
	if t.Masked && disp != "" {
		disp = strings.Repeat("*", len([]rune(disp)))
	}
	col := t.TextColor
	if disp == "" {
		disp = t.Placeholder
		col.A = 120 // Fade placeholder
	}

	pnt.PushClip(t.Rect)
	textY := t.Rect.Min.Y + (t.Rect.Dy()-defaultFontHeight)/2 + defaultFontBaseline
	pnt.DrawText(disp, t.Rect.Min.X+10, textY, col)
	pnt.PopClip()

	// Draw cursor if focused
	if state.FocusedID == t.CompID {
		// Simple blinking logic based on time
		if (time.Since(state.StartTime).Milliseconds()/500)%2 == 0 {
			charW := state.FontCharWidth
			if charW <= 0 {
				charW = 8
			}
			cursorX := t.Rect.Min.X + 10 + (t.CursorIndex * charW)
			cursorTop := t.Rect.Min.Y + (t.Rect.Dy()-defaultFontHeight)/2
			pnt.FillRect(image.Rect(cursorX, cursorTop, cursorX+2, cursorTop+defaultFontHeight), t.TextColor)
		}
	}
}

func (t *TextInput) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID != t.CompID {
		return false
	}

	const VK_BACK = 0x08
	const VK_RETURN = 0x0D
	const VK_LEFT = 0x25
	const VK_RIGHT = 0x27
	const VK_HOME = 0x24
	const VK_END = 0x23

	rawRunes := []rune(t.Text)
	if t.CursorIndex < 0 || t.CursorIndex > len(rawRunes) {
		t.CursorIndex = len(rawRunes)
	}

	if key == VK_RETURN {
		if t.OnSubmit != nil {
			t.OnSubmit(t.Text, state)
			return true
		}
		return false
	}

	if key == VK_LEFT {
		if t.CursorIndex > 0 {
			t.CursorIndex--
		}
		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
		}
		return true
	}

	if key == VK_RIGHT {
		if t.CursorIndex < len(rawRunes) {
			t.CursorIndex++
		}
		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
		}
		return true
	}

	if key == VK_HOME {
		t.CursorIndex = 0
		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
		}
		return true
	}

	if key == VK_END {
		t.CursorIndex = len(rawRunes)
		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
		}
		return true
	}

	if key == VK_BACK {
		if t.CursorIndex > 0 {
			rawRunes = append(rawRunes[:t.CursorIndex-1], rawRunes[t.CursorIndex:]...)
			t.Text = string(rawRunes)
			t.CursorIndex--
		}
		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID] = t.Text
			state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
		}
		return true
	}

	if char >= 32 && char != 127 {
		rawRunes = append(rawRunes[:t.CursorIndex], append([]rune{char}, rawRunes[t.CursorIndex:]...)...)
		t.Text = string(rawRunes)
		t.CursorIndex++
		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID] = t.Text
			state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
		}
		return true
	}

	return false
}

func (t *TextInput) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	state.FocusedID = t.CompID

	charW := state.FontCharWidth
	if charW <= 0 {
		charW = 8
	}

	clickX := pt.X - t.Rect.Min.X - 10
	cursor := (clickX + charW/2) / charW
	runes := []rune(t.Text)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	t.CursorIndex = cursor

	if state.TextInputValues != nil {
		state.TextInputValues[t.CompID] = t.Text
		state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
	}
	return true
}
func (t *TextInput) OnMouseUp(pt image.Point, state *types.ApplicationState) bool   { return false }
func (t *TextInput) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

func (t *TextInput) Focusable() bool               { return true }
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

func (s *Slider) ID() string                  { return s.CompID }
func (s *Slider) GetID() string               { return s.CompID }
func (s *Slider) Bounds() image.Rectangle     { return s.Rect }
func (s *Slider) SetBounds(r image.Rectangle) { s.Rect = r }
func (s *Slider) HitTest(pt image.Point) string {
	if pt.In(s.Rect) {
		return s.CompID
	}
	return ""
}
func (s *Slider) Draw(pnt types.Painter, state *types.ApplicationState) {
	// Restore state if present
	if s.CompID == "sld_vol" {
		s.Value = state.Volume
	} else if state.SliderValues != nil {
		if val, ok := state.SliderValues[s.CompID]; ok {
			s.Value = val
		} else {
			state.SliderValues[s.CompID] = s.Value
		}
	}

	// Draw glowing focus outline ring if focused
	if state.FocusedID == s.CompID {
		pnt.SetGlow(6.0)
		pnt.DrawRoundedRect(image.Rect(s.Rect.Min.X-2, s.Rect.Min.Y-2, s.Rect.Max.X+2, s.Rect.Max.Y+2), 4, color.RGBA{0, 150, 255, 200})
		pnt.SetGlow(0)
	}

	if state.HoveredID == s.CompID {
		state.CursorID = state.HandCursor
	}

	// Draw track
	trackH := 4
	trackRect := image.Rect(s.Rect.Min.X, s.Rect.Min.Y+s.Rect.Dy()/2-trackH/2, s.Rect.Max.X, s.Rect.Min.Y+s.Rect.Dy()/2+trackH/2)
	pnt.FillRect(trackRect, s.TrackColor)

	// Draw thumb
	thumbW := 12
	percent := (s.Value - s.Min) / (s.Max - s.Min)
	thumbX := s.Rect.Min.X + int(percent*float32(s.Rect.Dx()-thumbW))
	thumbRect := image.Rect(thumbX, s.Rect.Min.Y, thumbX+thumbW, s.Rect.Max.Y)

	col := s.ThumbColor
	if state.ActiveID == s.CompID {
		col.A = 200 // Brighter when dragging
	}
	pnt.DrawRoundedRect(thumbRect, 4, col)
}

func (s *Slider) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID != s.CompID {
		return false
	}
	const VK_LEFT = 0x25
	const VK_RIGHT = 0x27

	step := (s.Max - s.Min) * 0.05 // 5% step size
	if step <= 0 {
		step = 1.0
	}

	if key == VK_LEFT {
		val := math.Round(float64(s.Value-step)/float64(step)) * float64(step)
		s.Value = float32(val)
		if s.Value < s.Min {
			s.Value = s.Min
		}
		if s.CompID == "sld_vol" {
			state.Volume = s.Value
		} else if state.SliderValues != nil {
			state.SliderValues[s.CompID] = s.Value
		}
		return true
	}
	if key == VK_RIGHT {
		val := math.Round(float64(s.Value+step)/float64(step)) * float64(step)
		s.Value = float32(val)
		if s.Value > s.Max {
			s.Value = s.Max
		}
		if s.CompID == "sld_vol" {
			state.Volume = s.Value
		} else if state.SliderValues != nil {
			state.SliderValues[s.CompID] = s.Value
		}
		return true
	}
	return false
}

func (s *Slider) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	state.ActiveID = s.CompID
	s.updateValue(pt.X)
	if s.CompID == "sld_vol" {
		state.Volume = s.Value
	} else if state.SliderValues != nil {
		state.SliderValues[s.CompID] = s.Value
	}
	return true
}

func (s *Slider) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	state.ActiveID = ""
	return true
}

func (s *Slider) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID == s.CompID {
		s.updateValue(pt.X)
		if s.CompID == "sld_vol" {
			state.Volume = s.Value
		} else if state.SliderValues != nil {
			state.SliderValues[s.CompID] = s.Value
		}
		return true
	}
	return false
}

func (s *Slider) updateValue(mouseX int) {
	percent := float32(mouseX-s.Rect.Min.X) / float32(s.Rect.Dx())
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	s.Value = s.Min + percent*(s.Max-s.Min)
}

func (s *Slider) Focusable() bool               { return true }
func (s *Slider) Walk(fn func(types.Component)) { fn(s) }

// ParticleComponent is a UI element wrapping the ParticleSystem
type ParticleComponent struct {
	CompID string
	System *types.ParticleSystem
}

func (c *ParticleComponent) GetID() string                 { return c.CompID }
func (c *ParticleComponent) ID() string                    { return c.CompID }
func (c *ParticleComponent) Bounds() image.Rectangle       { return c.System.Bounds }
func (c *ParticleComponent) SetBounds(r image.Rectangle)   { c.System.Bounds = r }
func (c *ParticleComponent) HitTest(pt image.Point) string { return "" }
func (c *ParticleComponent) Focusable() bool               { return false }
func (c *ParticleComponent) Walk(fn func(types.Component)) { fn(c) }

func (c *ParticleComponent) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	return false
}
func (c *ParticleComponent) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	return false
}
func (c *ParticleComponent) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	return false
}
func (c *ParticleComponent) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	return false
}

func (c *ParticleComponent) Draw(p types.Painter, state *types.ApplicationState) {
	if c.System != nil {
		c.System.Draw(p, state)
	}
}

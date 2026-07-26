package components

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/theme"
	"github.com/mulavdm/poem/pkg/render/types"
)

// Panel is a simple container with a background color
type Panel struct {
	CompID   string
	Rect     image.Rectangle
	BGColor  color.RGBA
	Rounding int
	UseTheme bool
	Raised   bool
	Style    *StyleOverride
}

func NewPanel(id string) *Panel { return &Panel{CompID: id, UseTheme: true} }

func (p *Panel) ID() string              { return p.CompID }
func (p *Panel) GetID() string           { return p.CompID }
func (p *Panel) Bounds() image.Rectangle { return p.Rect }
func (p *Panel) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	size := applyExplicitSize(explicitSize(p.Rect), avail)
	return types.MeasureResult{Preferred: size, Min: size}
}
func (p *Panel) Draw(pnt types.Painter, state *types.ApplicationState) {
	background := p.BGColor
	radius := p.Rounding
	if themed(p.UseTheme, state) {
		current := activeTheme(state)
		background = current.Colors.Surface
		if p.Raised {
			background = current.Colors.SurfaceRaised
		}
		if radius == 0 {
			radius = current.Radii.Large
		}
		if p.Style != nil {
			if p.Style.Background != nil {
				background = *p.Style.Background
			}
			if p.Style.Radius != nil {
				radius = *p.Style.Radius
			}
		}
	}
	if p.Style != nil && p.Style.Border != nil {
		pnt.DrawRoundedRect(p.Rect, radius, *p.Style.Border)
		inner := image.Rect(p.Rect.Min.X+1, p.Rect.Min.Y+1, p.Rect.Max.X-1, p.Rect.Max.Y-1)
		pnt.DrawRoundedRect(inner, maxInt(0, radius-1), background)
		return
	}
	if themed(p.UseTheme, state) && p.Raised {
		borderColor := activeTheme(state).Colors.Border
		pnt.DrawRoundedRect(p.Rect, radius, borderColor)
		inner := image.Rect(p.Rect.Min.X+1, p.Rect.Min.Y+1, p.Rect.Max.X-1, p.Rect.Max.Y-1)
		pnt.DrawRoundedRect(inner, maxInt(0, radius-1), background)
		return
	}
	pnt.DrawRoundedRect(p.Rect, radius, background)
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

func (p *Panel) Semantics(state *types.ApplicationState) semantics.Node {
	return semantics.Node{ID: p.CompID, Role: semantics.RoleGroup, Bounds: p.Rect}
}

// GlassPanel is a premium panel with real-time blur background
type GlassPanel struct {
	Panel
	Opacity uint8
}

func (p *GlassPanel) Draw(pnt types.Painter, state *types.ApplicationState) {
	pnt.SetGlass(state.GlassEnabled)
	current := activeTheme(state)
	shadow := current.Elevation.Medium
	pnt.SetShadow(float32(shadow.OffsetX), float32(shadow.OffsetY), float32(shadow.Blur))
	col := p.BGColor
	radius := p.Rounding
	if themed(p.UseTheme, state) {
		col = current.Colors.SurfaceRaised
		if radius == 0 {
			radius = current.Radii.Large
		}
	}
	col.A = p.Opacity
	pnt.DrawRoundedRect(p.Rect, radius, col)
	pnt.SetShadow(0, 0, 0) // Reset shadow
	pnt.SetGlass(false)
}

func (p *GlassPanel) Walk(fn func(types.Component)) { fn(p) }

// Button is an interactive element with hover states
type Button struct {
	CompID         string
	Rect           image.Rectangle
	Text           string
	BaseColor      color.RGBA
	HoverColor     color.RGBA
	Rounding       int
	OnClick        func(state *types.ApplicationState)
	OnDrag         func(phase string, point image.Point, state *types.ApplicationState)
	UseTheme       bool
	Variant        theme.Variant
	Size           ControlSize
	Disabled       bool
	Loading        bool
	Selected       bool
	Invalid        bool
	AccessibleName string
	Mnemonic       rune
	Relations      semantics.Relationships
	Style          *StyleOverride
	// FixedWidth pins the button width to an author-specified value. It is
	// distinct from Rect: Rect holds the bounds layout assigns, which Measure
	// must NOT treat as a requested size — doing so froze a button at whatever
	// width it was first laid out at, so after a window resize the label kept
	// truncating in a panel with room to spare. Zero means "size to content".
	FixedWidth int
}

func NewButton(id, label string, onClick func(*types.ApplicationState)) *Button {
	return &Button{CompID: id, Text: label, OnClick: onClick, UseTheme: true, Size: ControlMedium}
}

func (b *Button) ID() string              { return b.CompID }
func (b *Button) GetID() string           { return b.CompID }
func (b *Button) Bounds() image.Rectangle { return b.Rect }
func (b *Button) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	charW := 8
	if state != nil && state.FontCharWidth > 0 {
		charW = state.FontCharWidth
	}
	labelWidth := len([]rune(strings.TrimSpace(b.Text))) * charW
	// intrinsicMin is the width the label actually needs plus horizontal
	// padding. A control's minimum has to hold its own text, or a flex row
	// under pressure shrinks the button until fitButtonLabel eats the label —
	// which is exactly how "Calculate route" shipped as "Calculate …" with
	// free space beside it (design guide TOKENS "scale independence", lint
	// UI023). Truncation stays only as a genuine last resort below intrinsicMin.
	intrinsicWidth := labelWidth + buttonLabelPadding
	minimumWidth := longestWordRunes(strings.TrimSpace(b.Text))*charW + buttonLabelPadding
	width := maxInt(140, intrinsicWidth)
	height := 38
	if themed(b.UseTheme, state) {
		height = controlHeight(activeTheme(state), b.Size)
	}
	if avail.X > 0 && width > avail.X {
		width = maxInt(minimumWidth, avail.X)
		maxChars := maxInt(1, (width-buttonLabelPadding)/charW)
		lineCount := len(wrapPlainText(strings.TrimSpace(b.Text), maxChars))
		if lineCount > 1 {
			th := activeTheme(state)
			height = maxInt(height, lineCount*int(th.Typography.Body.LineHeight)+2*th.Spacing.SM)
		}
	}
	minWidth := intrinsicWidth
	// Only an author-set FixedWidth overrides content sizing. Rect is where
	// layout put the button on a previous pass, not a size request, so it is
	// deliberately NOT read here — treating it as explicit froze the button at
	// its first-laid-out width, so after a resize the label kept truncating in
	// a panel with room to spare (design guide: TYPOGRAPHY "support user text
	// scaling without clipping", lint UI023).
	if b.FixedWidth > 0 {
		width = b.FixedWidth
		minWidth = b.FixedWidth
	}
	return types.MeasureResult{
		Preferred: image.Pt(width, height),
		Min:       image.Pt(minWidth, height),
	}
}

// buttonLabelPadding is the horizontal room a button reserves around its label.
// fitButtonLabel subtracts 20 before truncating, so the intrinsic minimum keeps
// a little more, leaving the label uncut at the reported minimum width.
const buttonLabelPadding = 24

func (b *Button) Draw(pnt types.Painter, state *types.ApplicationState) {
	if themed(b.UseTheme, state) {
		hovered := state != nil && state.HoveredID == b.CompID
		pressed := state != nil && state.ActiveID == b.CompID
		visual := buttonVisual(activeTheme(state), b.Variant, hovered, pressed, b.Disabled, b.Selected, b.Style)
		if b.Invalid {
			visual.border = activeTheme(state).Colors.Danger
		}
		drawControlSurface(pnt, b.Rect, visual, state != nil && state.FocusedID == b.CompID)
		if hovered && !b.Disabled {
			state.CursorID = state.HandCursor
		}
		label := b.Text
		if b.Loading {
			label = "Working..."
		}
		charW := state.FontCharWidth
		if charW <= 0 {
			charW = 8
		}
		pnt.PushClip(b.Rect)
		drawWrappedButtonLabel(pnt, b.Rect, label, charW, activeTheme(state), visual.foreground)
		pnt.PopClip()
		return
	}
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
	label := fitButtonLabel(b.Text, b.Rect.Dx(), charW)
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

func drawWrappedButtonLabel(pnt types.Painter, bounds image.Rectangle, label string, charW int, th theme.Theme, col color.RGBA) {
	maxChars := maxInt(1, (bounds.Dx()-buttonLabelPadding)/charW)
	lines := wrapPlainText(strings.TrimSpace(label), maxChars)
	lineHeight := int(th.Typography.Body.LineHeight)
	if lineHeight <= 0 {
		lineHeight = defaultFontHeight
	}
	totalHeight := len(lines) * lineHeight
	baseline := bounds.Min.Y + (bounds.Dy()-totalHeight)/2 + defaultFontBaseline
	for index, line := range lines {
		tx := bounds.Min.X + maxInt(10, (bounds.Dx()-len([]rune(line))*charW)/2)
		pnt.DrawText(line, tx, baseline+index*lineHeight, col)
	}
}
func (b *Button) SetBounds(r image.Rectangle) { b.Rect = r }
func (b *Button) HitTest(pt image.Point) string {
	if pt.In(b.Rect) {
		return b.CompID
	}
	return ""
}
func (b *Button) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if !b.Disabled && !b.Loading && state.FocusedID == b.CompID && (key == 13 || key == 32) { // Enter or Space
		if b.OnClick != nil {
			b.OnClick(state)
			return true
		}
	}
	return false
}
func (b *Button) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if b.Disabled || b.Loading {
		return false
	}
	state.ActiveID = b.CompID
	if b.OnDrag != nil {
		b.OnDrag("begin", pt, state)
	}
	return true
}
func (b *Button) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	wasActive := state.ActiveID == b.CompID
	if wasActive && pt.In(b.Rect) && b.OnClick != nil && !b.Disabled && !b.Loading {
		b.OnClick(state)
	}
	if wasActive && b.OnDrag != nil {
		b.OnDrag("end", pt, state)
	}
	return wasActive
}
func (b *Button) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID == b.CompID && b.OnDrag != nil {
		b.OnDrag("update", pt, state)
		return true
	}
	return false
}

func (b *Button) Focusable() bool               { return !b.Disabled }
func (b *Button) Walk(fn func(types.Component)) { fn(b) }

func (b *Button) MnemonicKey() rune { return b.Mnemonic }
func (b *Button) ActivateMnemonic(state *types.ApplicationState) bool {
	if b.Disabled || b.Loading || b.Mnemonic == 0 {
		return false
	}
	state.FocusedID = b.CompID
	if b.OnClick != nil {
		b.OnClick(state)
	}
	return true
}

func (b *Button) Semantics(state *types.ApplicationState) semantics.Node {
	name := b.AccessibleName
	if name == "" {
		name = b.Text
	}
	accessKey := ""
	if b.Mnemonic != 0 {
		accessKey = "Alt+" + strings.ToUpper(string(b.Mnemonic))
	}
	return semantics.Node{ID: b.CompID, Role: semantics.RoleButton, Name: name, AccessKey: accessKey, Bounds: b.Rect,
		State: semantics.State{Disabled: b.Disabled, Focused: state != nil && state.FocusedID == b.CompID,
			Selected: b.Selected, Invalid: b.Invalid}, Relations: b.Relations, Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionInvoke}}
}

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
	CompID         string
	Pos            image.Point
	Text           string
	Color          color.RGBA
	UseTheme       bool
	Role           TextRole
	AccessibleName string
	Typography     TypographyRole

	rect       image.Rectangle
	lineHeight int
	baseline   int
}

func NewLabel(id, text string) *Label { return &Label{CompID: id, Text: text, UseTheme: true} }

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
	lineHeight := defaultFontHeight
	if themed(l.UseTheme, state) {
		style := typographyStyle(activeTheme(state), l.Typography)
		scale := style.Size / activeTheme(state).Typography.Body.Size
		charW = int(float32(charW) * scale)
		lineHeight = int(style.LineHeight)
		l.baseline = int(float32(defaultFontBaseline) * scale)
	}
	l.lineHeight = lineHeight
	size := measurePlainText(l.Text, charW, lineHeight)
	minimumWidth := longestWordRunes(l.Text) * charW
	if avail.X > 0 && size.X > avail.X {
		size.X = maxInt(minimumWidth, avail.X)
		size.Y = len(wrapPlainText(l.Text, maxInt(1, size.X/charW))) * lineHeight
	}
	return types.MeasureResult{Preferred: size, Min: image.Pt(minimumWidth, lineHeight)}
}
func (l *Label) Draw(pnt types.Painter, state *types.ApplicationState) {
	col := l.Color
	if themed(l.UseTheme, state) {
		col = textRoleColor(activeTheme(state), l.Role)
	}
	if !l.rect.Empty() {
		charW := 8
		if state != nil && state.FontCharWidth > 0 {
			charW = state.FontCharWidth
		}
		style := typographyStyle(activeTheme(state), l.Typography)
		scale := style.Size / activeTheme(state).Typography.Body.Size
		charW = int(float32(charW) * scale)
		lineHeight := maxInt(1, int(style.LineHeight))
		lines := wrapPlainText(l.Text, maxInt(1, l.rect.Dx()/maxInt(1, charW)))
		baseline := l.rect.Min.Y + int(float32(defaultFontBaseline)*scale)
		for index, line := range lines {
			drawTypography(pnt, line, l.rect.Min.X, baseline+index*lineHeight, col, activeTheme(state), l.Typography)
		}
		return
	}
	drawTypography(pnt, l.Text, l.Pos.X, l.Pos.Y, col, activeTheme(state), l.Typography)
}
func (l *Label) SetBounds(r image.Rectangle) {
	l.rect = r
	lineHeight, baseline := l.lineHeight, l.baseline
	if lineHeight <= 0 {
		lineHeight = defaultFontHeight
	}
	if baseline <= 0 {
		baseline = defaultFontBaseline
	}
	l.Pos = image.Point{X: r.Min.X, Y: r.Min.Y + (r.Dy()-lineHeight)/2 + baseline}
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

func (l *Label) Semantics(state *types.ApplicationState) semantics.Node {
	name := l.AccessibleName
	if name == "" {
		name = l.Text
	}
	return semantics.Node{ID: l.CompID, Role: semantics.RoleText, Name: name, Value: l.Text, Bounds: l.Bounds()}
}

// DynamicLabel is a label that fetches its text from a function
type DynamicLabel struct {
	CompID         string
	Pos            image.Point
	GetText        func(state *types.ApplicationState) string
	Color          color.RGBA
	UseTheme       bool
	Role           TextRole
	AccessibleName string
	Typography     TypographyRole

	rect       image.Rectangle
	lineHeight int
	baseline   int
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
	lineHeight := defaultFontHeight
	if themed(dl.UseTheme, state) {
		style := typographyStyle(activeTheme(state), dl.Typography)
		scale := style.Size / activeTheme(state).Typography.Body.Size
		charW = int(float32(charW) * scale)
		lineHeight = int(style.LineHeight)
		dl.baseline = int(float32(defaultFontBaseline) * scale)
	}
	dl.lineHeight = lineHeight
	text := ""
	if dl.GetText != nil && state != nil {
		text = dl.GetText(state)
	}
	width := maxInt(120, len([]rune(text))*charW)
	minimumWidth := longestWordRunes(text) * charW
	height := lineHeight
	if avail.X > 0 && width > avail.X {
		width = maxInt(minimumWidth, avail.X)
		height = len(wrapPlainText(text, maxInt(1, width/charW))) * lineHeight
	}
	return types.MeasureResult{
		Preferred: image.Pt(width, height),
		Min:       image.Pt(minimumWidth, lineHeight),
	}
}
func (dl *DynamicLabel) Draw(pnt types.Painter, state *types.ApplicationState) {
	if dl.GetText != nil {
		col := dl.Color
		if themed(dl.UseTheme, state) {
			col = textRoleColor(activeTheme(state), dl.Role)
		}
		drawTypography(pnt, dl.GetText(state), dl.Pos.X, dl.Pos.Y, col, activeTheme(state), dl.Typography)
	}
}
func (dl *DynamicLabel) SetBounds(r image.Rectangle) {
	dl.rect = r
	lineHeight, baseline := dl.lineHeight, dl.baseline
	if lineHeight <= 0 {
		lineHeight = defaultFontHeight
	}
	if baseline <= 0 {
		baseline = defaultFontBaseline
	}
	dl.Pos = image.Point{X: r.Min.X, Y: r.Min.Y + (r.Dy()-lineHeight)/2 + baseline}
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

func (dl *DynamicLabel) Semantics(state *types.ApplicationState) semantics.Node {
	value := ""
	if dl.GetText != nil {
		value = dl.GetText(state)
	}
	name := dl.AccessibleName
	if name == "" {
		name = value
	}
	return semantics.Node{ID: dl.CompID, Role: semantics.RoleText, Name: name, Value: value, Bounds: dl.Bounds()}
}

// TextInput is an editable text field
type TextInput struct {
	CompID         string
	Rect           image.Rectangle
	Value          string
	Placeholder    string
	Masked         bool
	BGColor        color.RGBA
	TextColor      color.RGBA
	Rounding       int
	CursorIndex    int
	OnSubmit       func(text string, state *types.ApplicationState)
	OnChange       func(text string, state *types.ApplicationState)
	UseTheme       bool
	Disabled       bool
	ReadOnly       bool
	Invalid        bool
	Required       bool
	AccessibleName string
	Style          *StyleOverride
}

func NewTextInput(id, placeholder string) *TextInput {
	return &TextInput{CompID: id, Placeholder: placeholder, UseTheme: true}
}

func (t *TextInput) ID() string              { return t.CompID }
func (t *TextInput) GetID() string           { return t.CompID }
func (t *TextInput) Bounds() image.Rectangle { return t.Rect }
func (t *TextInput) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	charW := 8
	if state != nil && state.FontCharWidth > 0 {
		charW = state.FontCharWidth
	}
	content := t.Value
	if strings.TrimSpace(content) == "" {
		content = t.Placeholder
	}
	contentWidth := len([]rune(content)) * charW
	if state != nil {
		contentWidth = inputTextWidth(state, content)
	}
	width := maxInt(160, contentWidth+20)
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
			t.Value = val
		} else {
			state.TextInputValues[t.CompID] = t.Value
		}
		if cursorVal, ok := state.TextInputValues[t.CompID+"_cursor"]; ok {
			var restoredCursor int
			if n, err := fmt.Sscanf(cursorVal, "%d", &restoredCursor); err == nil && n == 1 {
				t.CursorIndex = restoredCursor
			}
		}
	}
	runes := []rune(t.Value)
	if t.CursorIndex < 0 || t.CursorIndex > len(runes) {
		t.CursorIndex = len(runes)
	}

	if themed(t.UseTheme, state) {
		current := activeTheme(state)
		visual := controlVisual{background: current.Colors.SurfaceSunken, foreground: current.Colors.Text,
			border: current.Colors.Border, focus: current.Colors.Focus, radius: current.Radii.Medium}
		if state.HoveredID == t.CompID {
			visual.border = current.Colors.BorderStrong
		}
		if t.Invalid {
			visual.border = current.Colors.Danger
		}
		if t.Disabled {
			visual.background, visual.foreground = current.Colors.Surface, current.Colors.TextDisabled
		}
		applyStyleOverride(&visual, t.Style, state.HoveredID == t.CompID, false)
		drawControlSurface(pnt, t.Rect, visual, state.FocusedID == t.CompID && !t.Disabled)
		t.TextColor = visual.foreground
		t.BGColor = visual.background
	} else if state.FocusedID == t.CompID {
		// Draw legacy glowing focus outline ring if focused
		pnt.SetGlow(6.0)
		pnt.DrawRoundedRect(image.Rect(t.Rect.Min.X-2, t.Rect.Min.Y-2, t.Rect.Max.X+2, t.Rect.Max.Y+2), t.Rounding+2, color.RGBA{0, 150, 255, 200})
		pnt.SetGlow(0)
	}

	if !themed(t.UseTheme, state) {
		pnt.DrawRoundedRect(t.Rect, t.Rounding, t.BGColor)
	}

	if state.HoveredID == t.CompID {
		state.CursorID = state.IBeamCursor
	}

	// Draw text or placeholder
	disp := t.Value
	compositionStart, compositionEnd := 0, 0
	compositionActive := false
	if composed, startWidth, endWidth, active := t.compositionDisplay(state); active {
		disp, compositionStart, compositionEnd, compositionActive = composed, startWidth, endWidth, true
	}
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
	if state.FocusedID == t.CompID && t.Value != "" && !compositionActive {
		selection := t.editingSelection(state, len([]rune(t.Value)))
		start, end := selectionBounds(selection)
		if start != end {
			textRunes := []rune(t.Value)
			selectionStartX := inputTextWidth(state, string(textRunes[:start]))
			selectionEndX := inputTextWidth(state, string(textRunes[:end]))
			selectionTop := t.Rect.Min.Y + (t.Rect.Dy()-defaultFontHeight)/2
			pnt.FillRect(image.Rect(t.Rect.Min.X+10+selectionStartX, selectionTop, t.Rect.Min.X+10+selectionEndX, selectionTop+defaultFontHeight), activeTheme(state).Colors.Selection)
		}
	}
	pnt.DrawText(disp, t.Rect.Min.X+10, textY, col)
	if compositionActive {
		underlineY := t.Rect.Min.Y + (t.Rect.Dy()+defaultFontHeight)/2
		pnt.DrawLine(t.Rect.Min.X+10+compositionStart, underlineY, t.Rect.Min.X+10+compositionEnd, underlineY, activeTheme(state).Colors.Accent)
	}
	pnt.PopClip()

	// Draw cursor if focused
	if state.FocusedID == t.CompID {
		// Simple blinking logic based on time
		if (time.Since(state.StartTime).Milliseconds()/500)%2 == 0 {
			cursorX := 0
			if compositionActive {
				cursorX = t.Rect.Min.X + 10 + compositionEnd
			} else {
				textRunes := []rune(t.Value)
				cursor := clampTextIndex(t.CursorIndex, len(textRunes))
				cursorX = t.Rect.Min.X + 10 + inputTextWidth(state, string(textRunes[:cursor]))
			}
			cursorTop := t.Rect.Min.Y + (t.Rect.Dy()-defaultFontHeight)/2
			pnt.FillRect(image.Rect(cursorX, cursorTop, cursorX+2, cursorTop+defaultFontHeight), t.TextColor)
		}
	}
}

func (t *TextInput) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID != t.CompID || t.Disabled {
		return false
	}
	before := t.Value
	defer func() {
		if t.Value != before && t.OnChange != nil {
			t.OnChange(t.Value, state)
		}
	}()

	return t.handleEditingKey(key, char, state)
}

func (t *TextInput) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if t.Disabled {
		return false
	}
	state.FocusedID = t.CompID

	clickX := pt.X - t.Rect.Min.X - 10
	runes := []rune(t.Value)
	cursor := inputTextIndexAtX(state, t.Value, clickX)
	t.CursorIndex = cursor
	selection := textInputSelection{Anchor: cursor, Caret: cursor}
	if modifierPressed(state, 0x10) {
		selection.Anchor = t.editingSelection(state, len(runes)).Anchor
	}
	t.setEditingSelection(state, selection, len(runes))
	state.ActiveID = t.CompID

	if state.TextInputValues != nil {
		state.TextInputValues[t.CompID] = t.Value
		state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
	}
	return true
}
func (t *TextInput) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	return state.ActiveID == t.CompID
}
func (t *TextInput) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID != t.CompID {
		return false
	}
	caret := inputTextIndexAtX(state, t.Value, pt.X-t.Rect.Min.X-10)
	selection := t.editingSelection(state, len([]rune(t.Value)))
	selection.Caret = caret
	t.setEditingSelection(state, selection, len([]rune(t.Value)))
	return true
}

func (t *TextInput) Focusable() bool               { return !t.Disabled }
func (t *TextInput) Walk(fn func(types.Component)) { fn(t) }

func (t *TextInput) Semantics(state *types.ApplicationState) semantics.Node {
	name := t.AccessibleName
	if name == "" {
		name = t.Placeholder
	}
	value := t.Value
	var textValue *semantics.TextValue
	actions := []semantics.Action{semantics.ActionFocus, semantics.ActionSetValue}
	if t.Masked {
		value = ""
	} else {
		start, end := t.Selection(state)
		textValue = &semantics.TextValue{SelectionStart: start, SelectionEnd: end}
		actions = append(actions, semantics.ActionSetSelection)
	}
	return semantics.Node{ID: t.CompID, Role: semantics.RoleTextField, Name: name, Value: value, Bounds: t.Rect,
		State: semantics.State{Disabled: t.Disabled, Focused: state != nil && state.FocusedID == t.CompID,
			ReadOnly: t.ReadOnly, Required: t.Required, Invalid: t.Invalid, Password: t.Masked},
		Text: textValue, Actions: actions}
}

// Slider is a range input element
type Slider struct {
	CompID         string
	Rect           image.Rectangle
	Min, Max       float32
	Value          float32
	TrackColor     color.RGBA
	ThumbColor     color.RGBA
	UseTheme       bool
	Disabled       bool
	Invalid        bool
	AccessibleName string
	Controlled     bool
	OnChange       func(float32, *types.ApplicationState)
}

func NewSlider(id string, min, max, value float32, onChange func(float32, *types.ApplicationState)) *Slider {
	return &Slider{CompID: id, Min: min, Max: max, Value: value, OnChange: onChange, Controlled: onChange != nil, UseTheme: true}
}

func (s *Slider) ID() string                  { return s.CompID }
func (s *Slider) GetID() string               { return s.CompID }
func (s *Slider) Bounds() image.Rectangle     { return s.Rect }
func (s *Slider) SetBounds(r image.Rectangle) { s.Rect = r }
func (s *Slider) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	t := activeTheme(state)
	width := avail.X
	if width <= 0 {
		width = 240
	}
	size := applyExplicitSize(explicitSize(s.Rect), image.Pt(width, t.Controls.Medium))
	return types.MeasureResult{Preferred: size, Min: image.Pt(minValueInt(size.X, 120), t.Controls.Small)}
}
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
	} else if !s.Controlled && state.SliderValues != nil {
		if val, ok := state.SliderValues[s.CompID]; ok {
			s.Value = val
		} else {
			state.SliderValues[s.CompID] = s.Value
		}
	}

	current := activeTheme(state)
	useTheme := themed(s.UseTheme, state)
	if useTheme && state.FocusedID == s.CompID && !s.Disabled {
		focus := image.Rect(s.Rect.Min.X-2, s.Rect.Min.Y-2, s.Rect.Max.X+2, s.Rect.Max.Y+2)
		pnt.DrawRoundedRect(focus, current.Radii.Medium, current.Colors.Focus)
		pnt.DrawRoundedRect(s.Rect, current.Radii.Medium, current.Colors.Background)
	} else if state.FocusedID == s.CompID {
		pnt.SetGlow(6.0)
		pnt.DrawRoundedRect(image.Rect(s.Rect.Min.X-2, s.Rect.Min.Y-2, s.Rect.Max.X+2, s.Rect.Max.Y+2), 4, color.RGBA{0, 150, 255, 200})
		pnt.SetGlow(0)
	}

	if state.HoveredID == s.CompID {
		state.CursorID = state.HandCursor
	}

	// Draw track and filled range.
	trackH := 4
	trackRect := image.Rect(s.Rect.Min.X, s.Rect.Min.Y+s.Rect.Dy()/2-trackH/2, s.Rect.Max.X, s.Rect.Min.Y+s.Rect.Dy()/2+trackH/2)
	trackColor := s.TrackColor
	if useTheme {
		trackColor = current.Colors.BorderStrong
		if s.Disabled {
			trackColor = current.Colors.Border
		}
	}
	trackRadius := roundedRadius(trackRect, current.Radii.Pill)
	pnt.DrawRoundedRect(trackRect, trackRadius, trackColor)

	// Draw thumb and active range.
	rangeValue := s.Max - s.Min
	if rangeValue <= 0 {
		rangeValue = 1
	}
	percent := (s.Value - s.Min) / rangeValue
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	thumbSize := minValueInt(16, s.Rect.Dy()-4)
	if thumbSize < 10 {
		thumbSize = 10
	}
	thumbCenter := s.Rect.Min.X + int(percent*float32(s.Rect.Dx()))
	if thumbCenter < s.Rect.Min.X+thumbSize/2 {
		thumbCenter = s.Rect.Min.X + thumbSize/2
	}
	if thumbCenter > s.Rect.Max.X-thumbSize/2 {
		thumbCenter = s.Rect.Max.X - thumbSize/2
	}
	if useTheme {
		activeTrack := image.Rect(trackRect.Min.X, trackRect.Min.Y, thumbCenter, trackRect.Max.Y)
		pnt.DrawRoundedRect(activeTrack, roundedRadius(activeTrack, current.Radii.Pill), current.Colors.Accent)
	}
	thumbRect := image.Rect(thumbCenter-thumbSize/2, s.Rect.Min.Y+(s.Rect.Dy()-thumbSize)/2, thumbCenter+thumbSize/2, s.Rect.Min.Y+(s.Rect.Dy()-thumbSize)/2+thumbSize)

	col := s.ThumbColor
	if useTheme {
		col = current.Colors.Accent
		if s.Disabled {
			col = current.Colors.TextDisabled
		}
		if s.Invalid {
			col = current.Colors.Danger
		}
	}
	if state.ActiveID == s.CompID {
		col.A = 200 // Brighter when dragging
	}
	pnt.DrawRoundedRect(thumbRect, roundedRadius(thumbRect, current.Radii.Pill), col)
}

func (s *Slider) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID != s.CompID || s.Disabled {
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
		} else if !s.Controlled && state.SliderValues != nil {
			state.SliderValues[s.CompID] = s.Value
		}
		s.notifyChange(state)
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
		} else if !s.Controlled && state.SliderValues != nil {
			state.SliderValues[s.CompID] = s.Value
		}
		s.notifyChange(state)
		return true
	}
	return false
}

func (s *Slider) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if s.Disabled {
		return false
	}
	state.ActiveID = s.CompID
	s.updateValue(pt.X)
	if s.CompID == "sld_vol" {
		state.Volume = s.Value
	} else if !s.Controlled && state.SliderValues != nil {
		state.SliderValues[s.CompID] = s.Value
	}
	s.notifyChange(state)
	return true
}

func (s *Slider) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	// Only the slider that owns the press ends it. Without this guard the
	// FlexBox mouse-up broadcast let any slider consume a release aimed at a
	// sibling control (and wipe ActiveID before the real target saw it) —
	// found when Android touch drove the first slider-bearing pkg/app tree.
	if state.ActiveID != s.CompID {
		return false
	}
	state.ActiveID = ""
	return true
}

func (s *Slider) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID == s.CompID {
		s.updateValue(pt.X)
		if s.CompID == "sld_vol" {
			state.Volume = s.Value
		} else if !s.Controlled && state.SliderValues != nil {
			state.SliderValues[s.CompID] = s.Value
		}
		s.notifyChange(state)
		return true
	}
	return false
}

func (s *Slider) notifyChange(state *types.ApplicationState) {
	if s.OnChange != nil {
		s.OnChange(s.Value, state)
	}
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

func (s *Slider) Focusable() bool               { return !s.Disabled }
func (s *Slider) Walk(fn func(types.Component)) { fn(s) }

func (s *Slider) Semantics(state *types.ApplicationState) semantics.Node {
	name := s.AccessibleName
	if name == "" {
		name = s.CompID
	}
	step := (s.Max - s.Min) * 0.05
	if step <= 0 {
		step = 1
	}
	return semantics.Node{ID: s.CompID, Role: semantics.RoleSlider, Name: name, Value: fmt.Sprintf("%.2f", s.Value), Bounds: s.Rect,
		State:   semantics.State{Disabled: s.Disabled, Invalid: s.Invalid, Focused: state != nil && state.FocusedID == s.CompID},
		Range:   &semantics.RangeValue{Minimum: float64(s.Min), Maximum: float64(s.Max), SmallChange: float64(step), LargeChange: float64(step * 10)},
		Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionIncrement, semantics.ActionDecrement, semantics.ActionSetValue}}
}

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

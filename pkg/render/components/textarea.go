package components

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/types"
)

type textareaStyledChar struct {
	char      rune
	isBold    bool
	origIndex int
}

type TextArea struct {
	CompID         string
	Rect           image.Rectangle
	Value          string
	Placeholder    string
	BGColor        color.RGBA
	TextColor      color.RGBA
	Rounding       int
	CharWidth      int // default 8 if 0
	LineHeight     int // default 24 if 0
	CursorIndex    int // character cursor offset (runes based), -1 if not focused/at end on init
	UseTheme       bool
	Disabled       bool
	ReadOnly       bool
	Invalid        bool
	Required       bool
	AccessibleName string
	OnChange       func(string, *types.ApplicationState)
}

func NewTextArea(id, placeholder string) *TextArea {
	return &TextArea{CompID: id, Placeholder: placeholder, CursorIndex: -1, UseTheme: true}
}

func (t *TextArea) ID() string              { return t.CompID }
func (t *TextArea) GetID() string           { return t.CompID }
func (t *TextArea) Bounds() image.Rectangle { return t.Rect }
func (t *TextArea) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	charW := defaultCharWidth(t.CharWidth)
	if state != nil && t.CharWidth <= 0 && state.FontCharWidth > 0 {
		charW = state.FontCharWidth
	}
	lineH := defaultLineHeight(t.LineHeight)
	padX := 24
	padY := 24
	width := avail.X
	if width <= 0 {
		width = 320
	}
	maxCharsPerLine := maxInt(1, (width-padX)/charW)
	content := t.Value
	if strings.TrimSpace(content) == "" {
		content = t.Placeholder
	}
	lines := wrapLineCount(content, maxCharsPerLine)
	height := padY + lines*lineH + 8
	size := applyExplicitSize(explicitSize(t.Rect), image.Pt(width, maxInt(96, height)))
	return types.MeasureResult{
		Preferred: size,
		Min:       image.Pt(minValueInt(size.X, 180), minValueInt(size.Y, 72)),
	}
}
func (t *TextArea) SetBounds(r image.Rectangle) {
	t.Rect = r
}

func (t *TextArea) Focusable() bool               { return !t.Disabled }
func (t *TextArea) Walk(fn func(types.Component)) { fn(t) }

func (t *TextArea) HitTest(pt image.Point) string {
	if pt.In(t.Rect) {
		return t.CompID
	}
	return ""
}

func (t *TextArea) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if t.Disabled {
		return false
	}
	state.FocusedID = t.CompID
	state.ActiveID = t.CompID

	charW := t.CharWidth
	if charW <= 0 {
		charW = state.FontCharWidth
	}
	if charW <= 0 {
		charW = 7
	}
	lineH := t.LineHeight
	if lineH <= 0 {
		lineH = 24
	}

	padX := 12
	padY := 18

	bearingX := state.FontCharBearingX
	clickX := pt.X - t.Rect.Min.X - padX - bearingX

	rawRunes := []rune(t.Value)
	paragraphs := buildTextareaParagraphs(rawRunes)

	maxWidth := t.Rect.Dx() - 2*padX
	if maxWidth <= charW {
		maxWidth = 300
	}
	maxCharsPerLine := maxWidth / charW

	// Tracing lines and absolute indexes
	type layoutLineInfo struct {
		chars       []textareaStyledChar
		startX      int
		startY      int
		startAbsIdx int
	}

	var layoutLines []layoutLineInfo
	currentY := 0
	absRuneOffset := 0

	for _, paragraph := range paragraphs {
		if len(paragraph) == 0 {
			// Empty paragraph represents an empty line
			layoutLines = append(layoutLines, layoutLineInfo{
				chars:       []textareaStyledChar{},
				startX:      t.Rect.Min.X + padX,
				startY:      t.Rect.Min.Y + padY + currentY,
				startAbsIdx: absRuneOffset,
			})
			currentY += lineH + 8
			absRuneOffset += 1 // account for the newline character
			continue
		}
		for _, line := range wrapTextareaParagraph(paragraph, maxCharsPerLine) {
			layoutLines = append(layoutLines, layoutLineInfo{
				chars:       line.chars,
				startX:      t.Rect.Min.X + padX,
				startY:      t.Rect.Min.Y + padY + currentY,
				startAbsIdx: line.startAbsIdx,
			})
			currentY += lineH
		}

		// account for paragraph runes + newline character
		absRuneOffset += len(paragraph) + 1
		currentY += 8
	}

	// 2. Find the closest line using absolute vertical text center matching (baseline - 5)
	selectedLineIdx := 0
	minDistY := 999999
	for i, l := range layoutLines {
		distY := absInt(pt.Y - (l.startY - 5))
		if distY < minDistY {
			minDistY = distY
			selectedLineIdx = i
		}
	}

	if len(layoutLines) > 0 {
		l := layoutLines[selectedLineIdx]
		// Find closest column boundary (rounding to nearest character)
		charIdx := (clickX + charW/2) / charW
		if charIdx < 0 {
			charIdx = 0
		}
		if charIdx > len(l.chars) {
			charIdx = len(l.chars)
		}
		t.CursorIndex = textareaLineBoundaryIndex(textareaLayoutLine{
			chars:       l.chars,
			startAbsIdx: l.startAbsIdx,
		}, charIdx)
		if t.CursorIndex > len(rawRunes) {
			t.CursorIndex = len(rawRunes)
		}
	} else {
		t.CursorIndex = len(rawRunes)
	}

	// Update in state text values
	if state.TextInputValues != nil {
		state.TextInputValues[t.CompID] = t.Value
		state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
	}
	selection := textInputSelection{Anchor: t.CursorIndex, Caret: t.CursorIndex}
	if modifierPressed(state, 0x10) {
		selection.Anchor = t.editingSelection(state, len(rawRunes)).Anchor
	}
	t.setEditingSelection(state, selection, len(rawRunes))

	return true
}

func absInt(val int) int {
	if val < 0 {
		return -val
	}
	return val
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type textareaLayoutLine struct {
	chars       []textareaStyledChar
	startAbsIdx int
}

func textareaLineBoundaryIndex(line textareaLayoutLine, visualBoundary int) int {
	if visualBoundary <= 0 {
		return line.startAbsIdx
	}
	if len(line.chars) == 0 {
		return line.startAbsIdx
	}
	if visualBoundary >= len(line.chars) {
		return line.chars[len(line.chars)-1].origIndex + 1
	}
	return line.chars[visualBoundary].origIndex
}

func textareaLineVisibleColumn(line textareaLayoutLine, cursorIndex int) int {
	if cursorIndex <= line.startAbsIdx {
		return 0
	}
	for idx, sc := range line.chars {
		if cursorIndex <= sc.origIndex {
			return idx
		}
	}
	return len(line.chars)
}

func textareaLineEndIndex(line textareaLayoutLine) int {
	if len(line.chars) == 0 {
		return line.startAbsIdx
	}
	return line.chars[len(line.chars)-1].origIndex + 1
}

func wrapTextareaParagraph(chars []textareaStyledChar, maxCharsPerLine int) []textareaLayoutLine {
	if maxCharsPerLine <= 0 {
		maxCharsPerLine = 1
	}
	if len(chars) == 0 {
		return []textareaLayoutLine{{chars: []textareaStyledChar{}, startAbsIdx: 0}}
	}

	lines := make([]textareaLayoutLine, 0, (len(chars)/maxCharsPerLine)+1)
	for start := 0; start < len(chars); start += maxCharsPerLine {
		end := start + maxCharsPerLine
		if end > len(chars) {
			end = len(chars)
		}
		lines = append(lines, textareaLayoutLine{
			chars:       chars[start:end],
			startAbsIdx: chars[start].origIndex,
		})
	}
	return lines
}

func buildTextareaParagraphs(rawRunes []rune) [][]textareaStyledChar {
	var parsedChars []textareaStyledChar
	isBold := false

	for i := 0; i < len(rawRunes); i++ {
		if i < len(rawRunes)-1 && rawRunes[i] == '*' && rawRunes[i+1] == '*' {
			isBold = !isBold
			i++
			continue
		}

		parsedChars = append(parsedChars, textareaStyledChar{
			char:      rawRunes[i],
			isBold:    isBold,
			origIndex: i,
		})
	}

	var paragraphs [][]textareaStyledChar
	var currentPara []textareaStyledChar
	for _, sc := range parsedChars {
		if sc.char == '\n' {
			paragraphs = append(paragraphs, currentPara)
			currentPara = nil
			continue
		}
		currentPara = append(currentPara, sc)
	}
	paragraphs = append(paragraphs, currentPara)
	return paragraphs
}

func (t *TextArea) OnKey(key uint32, char rune, state *types.ApplicationState) bool {
	if state.FocusedID != t.CompID || t.Disabled {
		return false
	}
	if key != 0x26 && key != 0x28 {
		before := t.Value
		handled := t.handleEditingKey(key, char, state)
		if handled && t.Value != before && t.OnChange != nil {
			t.OnChange(t.Value, state)
		}
		return handled
	}
	before := t.Value
	defer func() {
		if t.Value != before && t.OnChange != nil {
			t.OnChange(t.Value, state)
		}
	}()

	defer func() {
		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
		}
	}()

	const VK_BACK = 0x08
	const VK_RETURN = 0x0D
	const VK_LEFT = 0x25
	const VK_UP = 0x26
	const VK_RIGHT = 0x27
	const VK_DOWN = 0x28

	rawRunes := []rune(t.Value)

	// Clamp cursor index safely
	if t.CursorIndex < 0 || t.CursorIndex > len(rawRunes) {
		t.CursorIndex = len(rawRunes)
	}

	if key == VK_LEFT {
		if t.CursorIndex > 0 {
			t.CursorIndex--
		}
		return true
	}
	if key == VK_RIGHT {
		if t.CursorIndex < len(rawRunes) {
			t.CursorIndex++
		}
		return true
	}

	// Support UP/DOWN key navigation
	if key == VK_UP || key == VK_DOWN {
		verticalSelection := t.editingSelection(state, len(rawRunes))
		charW := t.CharWidth
		if charW <= 0 {
			charW = state.FontCharWidth
		}
		if charW <= 0 {
			charW = 7
		}
		lineH := t.LineHeight
		if lineH <= 0 {
			lineH = 24
		}

		paragraphs := buildTextareaParagraphs(rawRunes)

		padX := 12
		maxWidth := t.Rect.Dx() - 2*padX
		if maxWidth <= charW {
			maxWidth = 300
		}
		maxCharsPerLine := maxWidth / charW

		type layoutLineInfo struct {
			chars       []textareaStyledChar
			startAbsIdx int
		}

		var layoutLines []layoutLineInfo
		absRuneOffset := 0

		for _, paragraph := range paragraphs {
			if len(paragraph) == 0 {
				layoutLines = append(layoutLines, layoutLineInfo{
					chars:       []textareaStyledChar{},
					startAbsIdx: absRuneOffset,
				})
				absRuneOffset += 1
				continue
			}
			for _, line := range wrapTextareaParagraph(paragraph, maxCharsPerLine) {
				layoutLines = append(layoutLines, layoutLineInfo{
					chars:       line.chars,
					startAbsIdx: line.startAbsIdx,
				})
			}
			absRuneOffset += len(paragraph) + 1
		}

		// Find current line and column offset
		currLineIdx := 0
		currColOffset := 0
		for idx, l := range layoutLines {
			endIdx := textareaLineEndIndex(textareaLayoutLine{
				chars:       l.chars,
				startAbsIdx: l.startAbsIdx,
			})
			if t.CursorIndex >= l.startAbsIdx && t.CursorIndex <= endIdx {
				currLineIdx = idx
				currColOffset = textareaLineVisibleColumn(textareaLayoutLine{
					chars:       l.chars,
					startAbsIdx: l.startAbsIdx,
				}, t.CursorIndex)
				break
			}
		}

		if key == VK_UP {
			if currLineIdx > 0 {
				prevLine := layoutLines[currLineIdx-1]
				newCol := minInt(currColOffset, len(prevLine.chars))
				t.CursorIndex = textareaLineBoundaryIndex(textareaLayoutLine{
					chars:       prevLine.chars,
					startAbsIdx: prevLine.startAbsIdx,
				}, newCol)
			}
		} else { // VK_DOWN
			if currLineIdx < len(layoutLines)-1 {
				nextLine := layoutLines[currLineIdx+1]
				newCol := minInt(currColOffset, len(nextLine.chars))
				t.CursorIndex = textareaLineBoundaryIndex(textareaLayoutLine{
					chars:       nextLine.chars,
					startAbsIdx: nextLine.startAbsIdx,
				}, newCol)
			}
		}
		anchor := t.CursorIndex
		if modifierPressed(state, 0x10) {
			anchor = verticalSelection.Anchor
		}
		t.setEditingSelection(state, textInputSelection{Anchor: anchor, Caret: t.CursorIndex}, len(rawRunes))
		return true
	}

	if key == VK_RETURN {
		// Insert newline at CursorIndex
		newText := append(rawRunes[:t.CursorIndex], append([]rune{'\n'}, rawRunes[t.CursorIndex:]...)...)
		t.Value = string(newText)
		t.CursorIndex++

		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID] = t.Value
		}
		return true
	}

	if key == VK_BACK {
		if t.CursorIndex > 0 {
			// Delete character at CursorIndex-1
			newText := append(rawRunes[:t.CursorIndex-1], rawRunes[t.CursorIndex:]...)
			t.Value = string(newText)
			t.CursorIndex--

			if state.TextInputValues != nil {
				state.TextInputValues[t.CompID] = t.Value
			}
		}
		return true
	}

	// Handle Printable ASCII characters insert
	if char >= 32 && char <= 126 {
		newText := append(rawRunes[:t.CursorIndex], append([]rune{char}, rawRunes[t.CursorIndex:]...)...)
		t.Value = string(newText)
		t.CursorIndex++

		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID] = t.Value
		}
		return true
	}

	return false
}

func (t *TextArea) Draw(pnt types.Painter, state *types.ApplicationState) {
	// Restore state if present
	if state.TextInputValues != nil {
		if val, ok := state.TextInputValues[t.CompID]; ok {
			t.Value = val
		} else {
			state.TextInputValues[t.CompID] = t.Value
		}
		if cursorValStr, ok := state.TextInputValues[t.CompID+"_cursor"]; ok {
			var restoredCursor int
			if n, err := fmt.Sscanf(cursorValStr, "%d", &restoredCursor); err == nil && n == 1 {
				t.CursorIndex = restoredCursor
			}
		}
	}

	charW := t.CharWidth
	if charW <= 0 {
		charW = state.FontCharWidth
	}
	if charW <= 0 {
		charW = 7
	}
	lineH := t.LineHeight
	if lineH <= 0 {
		lineH = 24
	}

	padX := 12
	padY := 18

	maxWidth := t.Rect.Dx() - 2*padX
	if maxWidth <= charW {
		maxWidth = 300
	}
	maxCharsPerLine := maxWidth / charW

	current := activeTheme(state)
	useTheme := themed(t.UseTheme, state)
	if useTheme {
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
		drawControlSurface(pnt, t.Rect, visual, state.FocusedID == t.CompID && !t.Disabled)
		t.TextColor = visual.foreground
	} else if state.FocusedID == t.CompID {
		pnt.SetGlow(6.0)
		pnt.DrawRoundedRect(image.Rect(t.Rect.Min.X-2, t.Rect.Min.Y-2, t.Rect.Max.X+2, t.Rect.Max.Y+2), t.Rounding+2, color.RGBA{139, 92, 246, 200})
		pnt.SetGlow(0)
	}

	if state.HoveredID == t.CompID {
		state.CursorID = state.IBeamCursor
	}

	// Draw background box if BGColor has alpha
	if !useTheme && t.BGColor.A > 0 {
		pnt.DrawRoundedRect(t.Rect, t.Rounding, t.BGColor)
	}

	// If cursor is not set yet, set to end
	baseRunes := []rune(t.Value)
	if t.CursorIndex < 0 || t.CursorIndex > len(baseRunes) {
		t.CursorIndex = len(baseRunes)
		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
		}
	}
	displayText, compositionStart, compositionEnd, compositionActive := t.compositionDisplay(state)
	rawRunes := []rune(displayText)
	paragraphs := buildTextareaParagraphs(rawRunes)
	drawCursorIndex := t.CursorIndex
	selectionStart, selectionEnd := t.Selection(state)
	if compositionActive {
		drawCursorIndex = compositionEnd
		selectionStart, selectionEnd = 0, 0
	}

	currentY := t.Rect.Min.Y + padY

	// Caret registers to completely avoid map allocations
	var caretX int = t.Rect.Min.X + padX
	var caretY int = t.Rect.Min.Y + padY

	startAbsIdx := 0

	pnt.PushClip(t.Rect)
	for _, paragraph := range paragraphs {
		// Empty paragraph represents a single newline
		if len(paragraph) == 0 {
			if startAbsIdx == drawCursorIndex {
				caretX = t.Rect.Min.X + padX
				caretY = currentY
			}
			currentY += lineH + 8
			startAbsIdx += 1 // account for the newline
			continue
		}

		if startAbsIdx == drawCursorIndex {
			caretX = t.Rect.Min.X + padX
			caretY = currentY
		}

		for _, line := range wrapTextareaParagraph(paragraph, maxCharsPerLine) {
			if currentY+lineH > t.Rect.Max.Y-padY/2 {
				break
			}

			cursorX := t.Rect.Min.X + padX
			caretVisualCol := textareaLineVisibleColumn(line, drawCursorIndex)
			var subsegment []rune
			subIsBold := false
			lineEndAbsIdx := textareaLineEndIndex(line)
			if selectionStart < selectionEnd && selectionEnd > line.startAbsIdx && selectionStart < lineEndAbsIdx {
				highlightStart := maxInt(selectionStart, line.startAbsIdx)
				highlightEnd := minInt(selectionEnd, lineEndAbsIdx)
				startColumn := textareaLineVisibleColumn(line, highlightStart)
				endColumn := textareaLineVisibleColumn(line, highlightEnd)
				if endColumn > startColumn {
					left := t.Rect.Min.X + padX + startColumn*charW
					pnt.FillRect(image.Rect(left, currentY-lineH+4, left+(endColumn-startColumn)*charW, currentY+4), current.Colors.Selection)
				}
			}

			flushSub := func() {
				if len(subsegment) == 0 {
					return
				}
				c := t.TextColor
				if subIsBold {
					if useTheme {
						c = current.Colors.Accent
					} else {
						c = color.RGBA{196, 181, 253, 255}
						pnt.SetGlow(3.0)
					}
				}
				pnt.DrawText(string(subsegment), cursorX, currentY, c)
				pnt.SetGlow(0)
				cursorX += len(subsegment) * charW
				subsegment = nil
			}

			for idx, sc := range line.chars {
				if idx == 0 {
					subIsBold = sc.isBold
				}
				if sc.isBold != subIsBold {
					flushSub()
					subIsBold = sc.isBold
				}

				if idx == caretVisualCol {
					caretX = cursorX + len(subsegment)*charW
					caretY = currentY
				}
				subsegment = append(subsegment, sc.char)
			}
			flushSub()
			if compositionActive && compositionEnd > line.startAbsIdx && compositionStart < lineEndAbsIdx {
				underlineStart := maxInt(compositionStart, line.startAbsIdx)
				underlineEnd := minInt(compositionEnd, lineEndAbsIdx)
				startColumn := textareaLineVisibleColumn(line, underlineStart)
				endColumn := textareaLineVisibleColumn(line, underlineEnd)
				if endColumn > startColumn {
					left := t.Rect.Min.X + padX + startColumn*charW
					pnt.FillRect(image.Rect(left, currentY+5, left+(endColumn-startColumn)*charW, currentY+7), current.Colors.Focus)
				}
			}

			if lineEndAbsIdx == drawCursorIndex {
				caretX = cursorX
				caretY = currentY
			}

			currentY += lineH
		}

		paraEndAbsIdx := startAbsIdx + len(paragraph)
		if paraEndAbsIdx == drawCursorIndex {
			caretX = t.Rect.Min.X + padX
			caretY = currentY
		}

		currentY += 8
		startAbsIdx += len(paragraph) + 1 // account for the paragraph characters + the newline
	}
	pnt.PopClip()

	// Draw the caret blinking (if focused)
	if state.FocusedID == t.CompID {
		if (time.Since(state.StartTime).Milliseconds()/500)%2 == 0 {
			caretColor := current.Colors.Accent
			if !useTheme {
				caretColor = color.RGBA{139, 92, 246, 255}
			}
			if state.TextInputValues["txt_manuscript_editor_streaming"] == "true" {
				caretColor = color.RGBA{0, 255, 150, 255} // Emerald
			}

			if !useTheme {
				pnt.SetGlow(6.0)
			}
			bearingX := state.FontCharBearingX
			pnt.PushClip(t.Rect)
			pnt.FillRect(image.Rect(caretX+bearingX, caretY-13, caretX+bearingX+2, caretY+3), caretColor)
			pnt.PopClip()
			pnt.SetGlow(0)
		}
	}

}

func (t *TextArea) Semantics(state *types.ApplicationState) semantics.Node {
	name := t.AccessibleName
	if name == "" {
		name = t.Placeholder
	}
	start, end := t.Selection(state)
	return semantics.Node{ID: t.CompID, Role: semantics.RoleTextField, Name: name, Value: t.Value, Bounds: t.Rect,
		State:   semantics.State{Disabled: t.Disabled, ReadOnly: t.ReadOnly, Required: t.Required, Invalid: t.Invalid, Focused: state != nil && state.FocusedID == t.CompID},
		Text:    &semantics.TextValue{SelectionStart: start, SelectionEnd: end, Multiline: true},
		Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionSetValue, semantics.ActionSetSelection}}
}

func (t *TextArea) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID != t.CompID {
		return false
	}
	state.ActiveID = ""
	return true
}

func (t *TextArea) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID != t.CompID {
		return false
	}
	if state.KeysPressed == nil {
		state.KeysPressed = make(map[uint32]bool)
	}
	shiftWasPressed := state.KeysPressed[0x10]
	state.KeysPressed[0x10] = true
	handled := t.OnMouseDown(pt, state)
	state.KeysPressed[0x10] = shiftWasPressed
	return handled
}

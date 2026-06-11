package components

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"go_native_gpu_gui/pkg/render/types"
)

type textareaStyledChar struct {
	char      rune
	isBold    bool
	origIndex int
}

type TextArea struct {
	CompID      string
	Rect        image.Rectangle
	Text        string
	Placeholder string
	BGColor     color.RGBA
	TextColor   color.RGBA
	Rounding    int
	CharWidth   int // default 8 if 0
	LineHeight  int // default 24 if 0
	CursorIndex int // character cursor offset (runes based), -1 if not focused/at end on init
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
	content := t.Text
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

func (t *TextArea) Focusable() bool               { return true }
func (t *TextArea) Walk(fn func(types.Component)) { fn(t) }

func (t *TextArea) HitTest(pt image.Point) string {
	if pt.In(t.Rect) {
		return t.CompID
	}
	return ""
}

func (t *TextArea) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	state.FocusedID = t.CompID

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
	padY := 12

	bearingX := state.FontCharBearingX
	clickX := pt.X - t.Rect.Min.X - padX - bearingX

	rawRunes := []rune(t.Text)
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
		state.TextInputValues[t.CompID] = t.Text
		state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
	}

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
	if state.FocusedID != t.CompID {
		return false
	}

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

	rawRunes := []rune(t.Text)

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
		return true
	}

	if key == VK_RETURN {
		// Insert newline at CursorIndex
		newText := append(rawRunes[:t.CursorIndex], append([]rune{'\n'}, rawRunes[t.CursorIndex:]...)...)
		t.Text = string(newText)
		t.CursorIndex++

		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID] = t.Text
		}
		return true
	}

	if key == VK_BACK {
		if t.CursorIndex > 0 {
			// Delete character at CursorIndex-1
			newText := append(rawRunes[:t.CursorIndex-1], rawRunes[t.CursorIndex:]...)
			t.Text = string(newText)
			t.CursorIndex--

			if state.TextInputValues != nil {
				state.TextInputValues[t.CompID] = t.Text
			}
		}
		return true
	}

	// Handle Printable ASCII characters insert
	if char >= 32 && char <= 126 {
		newText := append(rawRunes[:t.CursorIndex], append([]rune{char}, rawRunes[t.CursorIndex:]...)...)
		t.Text = string(newText)
		t.CursorIndex++

		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID] = t.Text
		}
		return true
	}

	return false
}

func (t *TextArea) Draw(pnt types.Painter, state *types.ApplicationState) {
	// Restore state if present
	if state.TextInputValues != nil {
		if val, ok := state.TextInputValues[t.CompID]; ok {
			t.Text = val
		} else {
			state.TextInputValues[t.CompID] = t.Text
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
	padY := 12

	maxWidth := t.Rect.Dx() - 2*padX
	if maxWidth <= charW {
		maxWidth = 300
	}
	maxCharsPerLine := maxWidth / charW

	// Draw glowing focus outline ring if focused
	if state.FocusedID == t.CompID {
		pnt.SetGlow(6.0)
		pnt.DrawRoundedRect(image.Rect(t.Rect.Min.X-2, t.Rect.Min.Y-2, t.Rect.Max.X+2, t.Rect.Max.Y+2), t.Rounding+2, color.RGBA{139, 92, 246, 200})
		pnt.SetGlow(0)
		state.CursorID = state.IBeamCursor
	}

	// Draw background box if BGColor has alpha
	if t.BGColor.A > 0 {
		pnt.DrawRoundedRect(t.Rect, t.Rounding, t.BGColor)
	}

	rawRunes := []rune(t.Text)
	paragraphs := buildTextareaParagraphs(rawRunes)

	// If cursor is not set yet, set to end
	if t.CursorIndex < 0 || t.CursorIndex > len(rawRunes) {
		t.CursorIndex = len(rawRunes)
		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
		}
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
			if startAbsIdx == t.CursorIndex {
				caretX = t.Rect.Min.X + padX
				caretY = currentY
			}
			currentY += lineH + 8
			startAbsIdx += 1 // account for the newline
			continue
		}

		if startAbsIdx == t.CursorIndex {
			caretX = t.Rect.Min.X + padX
			caretY = currentY
		}

		for _, line := range wrapTextareaParagraph(paragraph, maxCharsPerLine) {
			if currentY+lineH > t.Rect.Max.Y-padY/2 {
				break
			}

			cursorX := t.Rect.Min.X + padX
			caretVisualCol := textareaLineVisibleColumn(line, t.CursorIndex)
			var subsegment []rune
			subIsBold := false

			flushSub := func() {
				if len(subsegment) == 0 {
					return
				}
				c := t.TextColor
				if subIsBold {
					c = color.RGBA{196, 181, 253, 255} // Glowing violet for bold
					pnt.SetGlow(3.0)
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

			lineEndAbsIdx := textareaLineEndIndex(line)
			if lineEndAbsIdx == t.CursorIndex {
				caretX = cursorX
				caretY = currentY
			}

			currentY += lineH
		}

		paraEndAbsIdx := startAbsIdx + len(paragraph)
		if paraEndAbsIdx == t.CursorIndex {
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
			caretColor := color.RGBA{139, 92, 246, 255} // Violet
			if state.TextInputValues["txt_manuscript_editor_streaming"] == "true" {
				caretColor = color.RGBA{0, 255, 150, 255} // Emerald
			}

			pnt.SetGlow(6.0)
			bearingX := state.FontCharBearingX
			pnt.PushClip(t.Rect)
			pnt.FillRect(image.Rect(caretX+bearingX, caretY-13, caretX+bearingX+2, caretY+3), caretColor)
			pnt.PopClip()
			pnt.SetGlow(0)
		}
	}

}

func (t *TextArea) OnMouseUp(pt image.Point, state *types.ApplicationState) bool   { return false }
func (t *TextArea) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

package components

import (
	"fmt"
	"image"
	"image/color"
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

	// Split by newline to preserve paragraph boundaries and wrap
	rawRunes := []rune(t.Text)
	var paragraphs [][]rune
	var currentPara []rune
	for _, r := range rawRunes {
		if r == '\n' {
			paragraphs = append(paragraphs, currentPara)
			currentPara = nil
		} else {
			currentPara = append(currentPara, r)
		}
	}
	paragraphs = append(paragraphs, currentPara)

	maxWidth := t.Rect.Dx() - 2*padX
	if maxWidth <= charW {
		maxWidth = 300
	}
	maxCharsPerLine := maxWidth / charW

	// Tracing lines and absolute indexes
	type layoutLineInfo struct {
		runes      []rune
		startX     int
		startY     int
		startAbsIdx int
	}

	var layoutLines []layoutLineInfo
	currentY := 0
	absRuneOffset := 0

	for _, paragraph := range paragraphs {
		var lines [][]rune
		if len(paragraph) == 0 {
			// Empty paragraph represents an empty line
			layoutLines = append(layoutLines, layoutLineInfo{
				runes:      []rune{},
				startX:     t.Rect.Min.X + padX,
				startY:     t.Rect.Min.Y + padY + currentY,
				startAbsIdx: absRuneOffset,
			})
			currentY += lineH + 8
			absRuneOffset += 1 // account for the newline character
			continue
		}

		// Word wrap paragraph runes
		var currentLine []rune
		var currentWord []rune

		for _, r := range paragraph {
			if r == ' ' || r == '\t' {
				if len(currentWord) > 0 {
					if len(currentLine) == 0 {
						currentLine = append(currentLine, currentWord...)
					} else if len(currentLine)+1+len(currentWord) <= maxCharsPerLine {
						currentLine = append(currentLine, ' ')
						currentLine = append(currentLine, currentWord...)
					} else {
						lines = append(lines, currentLine)
						currentLine = nil
						currentLine = append(currentLine, currentWord...)
					}
					currentWord = nil
				}
			} else {
				currentWord = append(currentWord, r)
			}
		}
		if len(currentWord) > 0 {
			if len(currentLine) == 0 {
				currentLine = append(currentLine, currentWord...)
			} else if len(currentLine)+1+len(currentWord) <= maxCharsPerLine {
				currentLine = append(currentLine, ' ')
				currentLine = append(currentLine, currentWord...)
			} else {
				lines = append(lines, currentLine)
				currentLine = nil
				currentLine = append(currentLine, currentWord...)
			}
		}
		if len(currentLine) > 0 {
			lines = append(lines, currentLine)
		}

		lineOffset := 0
		for _, line := range lines {
			layoutLines = append(layoutLines, layoutLineInfo{
				runes:      line,
				startX:     t.Rect.Min.X + padX,
				startY:     t.Rect.Min.Y + padY + currentY,
				startAbsIdx: absRuneOffset + lineOffset,
			})
			currentY += lineH
			lineOffset += len(line)
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
		if charIdx > len(l.runes) {
			charIdx = len(l.runes)
		}
		t.CursorIndex = l.startAbsIdx + charIdx
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
		if lineH <= 0 { lineH = 24 }

		var paragraphs [][]rune
		var currentPara []rune
		for _, r := range rawRunes {
			if r == '\n' {
				paragraphs = append(paragraphs, currentPara)
				currentPara = nil
			} else {
				currentPara = append(currentPara, r)
			}
		}
		paragraphs = append(paragraphs, currentPara)

		padX := 12
		maxWidth := t.Rect.Dx() - 2*padX
		if maxWidth <= charW { maxWidth = 300 }
		maxCharsPerLine := maxWidth / charW

		type layoutLineInfo struct {
			runes      []rune
			startAbsIdx int
		}

		var layoutLines []layoutLineInfo
		absRuneOffset := 0

		for _, paragraph := range paragraphs {
			var lines [][]rune
			if len(paragraph) == 0 {
				layoutLines = append(layoutLines, layoutLineInfo{
					runes:      []rune{},
					startAbsIdx: absRuneOffset,
				})
				absRuneOffset += 1
				continue
			}

			var currentLine []rune
			var currentWord []rune

			for _, r := range paragraph {
				if r == ' ' || r == '\t' {
					if len(currentWord) > 0 {
						if len(currentLine) == 0 {
							currentLine = append(currentLine, currentWord...)
						} else if len(currentLine)+1+len(currentWord) <= maxCharsPerLine {
							currentLine = append(currentLine, ' ')
							currentLine = append(currentLine, currentWord...)
						} else {
							lines = append(lines, currentLine)
							currentLine = nil
							currentLine = append(currentLine, currentWord...)
						}
						currentWord = nil
					}
				} else {
					currentWord = append(currentWord, r)
				}
			}
			if len(currentWord) > 0 {
				if len(currentLine) == 0 {
					currentLine = append(currentLine, currentWord...)
				} else if len(currentLine)+1+len(currentWord) <= maxCharsPerLine {
					currentLine = append(currentLine, ' ')
					currentLine = append(currentLine, currentWord...)
				} else {
					lines = append(lines, currentLine)
					currentLine = nil
					currentLine = append(currentLine, currentWord...)
				}
			}
			if len(currentLine) > 0 {
				lines = append(lines, currentLine)
			}

			lineOffset := 0
			for _, line := range lines {
				layoutLines = append(layoutLines, layoutLineInfo{
					runes:      line,
					startAbsIdx: absRuneOffset + lineOffset,
				})
				lineOffset += len(line)
			}
			absRuneOffset += len(paragraph) + 1
		}

		// Find current line and column offset
		currLineIdx := 0
		currColOffset := 0
		for idx, l := range layoutLines {
			endIdx := l.startAbsIdx + len(l.runes)
			if t.CursorIndex >= l.startAbsIdx && t.CursorIndex <= endIdx {
				currLineIdx = idx
				currColOffset = t.CursorIndex - l.startAbsIdx
				break
			}
		}

		if key == VK_UP {
			if currLineIdx > 0 {
				prevLine := layoutLines[currLineIdx-1]
				newCol := minInt(currColOffset, len(prevLine.runes))
				t.CursorIndex = prevLine.startAbsIdx + newCol
			}
		} else { // VK_DOWN
			if currLineIdx < len(layoutLines)-1 {
				nextLine := layoutLines[currLineIdx+1]
				newCol := minInt(currColOffset, len(nextLine.runes))
				t.CursorIndex = nextLine.startAbsIdx + newCol
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

	// Rich text parsing for paragraph display
	rawRunes := []rune(t.Text)
	var parsedChars []textareaStyledChar
	isBold := false

	// If cursor is not set yet, set to end
	if t.CursorIndex < 0 || t.CursorIndex > len(rawRunes) {
		t.CursorIndex = len(rawRunes)
		if state.TextInputValues != nil {
			state.TextInputValues[t.CompID+"_cursor"] = fmt.Sprintf("%d", t.CursorIndex)
		}
	}

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

	// Split paragraphs
	var paragraphs [][]textareaStyledChar
	var currentPara []textareaStyledChar

	for _, sc := range parsedChars {
		if sc.char == '\n' {
			paragraphs = append(paragraphs, currentPara)
			currentPara = nil
		} else {
			currentPara = append(currentPara, sc)
		}
	}
	paragraphs = append(paragraphs, currentPara)

	currentY := t.Rect.Min.Y + padY

	// Caret registers to completely avoid map allocations
	var caretX int = t.Rect.Min.X + padX
	var caretY int = t.Rect.Min.Y + padY

	startAbsIdx := 0

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

		// Word wrap styled paragraph characters
		var lines [][]textareaStyledChar
		var currentLine []textareaStyledChar
		var currentWord []textareaStyledChar

		for _, sc := range paragraph {
			if sc.char == ' ' || sc.char == '\t' {
				if len(currentWord) > 0 {
					if len(currentLine) == 0 {
						currentLine = append(currentLine, currentWord...)
					} else if len(currentLine)+1+len(currentWord) <= maxCharsPerLine {
						currentLine = append(currentLine, textareaStyledChar{char: ' '})
						currentLine = append(currentLine, currentWord...)
					} else {
						lines = append(lines, currentLine)
						currentLine = nil
						currentLine = append(currentLine, currentWord...)
					}
					currentWord = nil
				}
			} else {
				currentWord = append(currentWord, sc)
			}
		}
		if len(currentWord) > 0 {
			if len(currentLine) == 0 {
				currentLine = append(currentLine, currentWord...)
			} else if len(currentLine)+1+len(currentWord) <= maxCharsPerLine {
				currentLine = append(currentLine, textareaStyledChar{char: ' '})
				currentLine = append(currentLine, currentWord...)
			} else {
				lines = append(lines, currentLine)
				currentLine = nil
				currentLine = append(currentLine, currentWord...)
			}
		}
		if len(currentLine) > 0 {
			lines = append(lines, currentLine)
		}

		// Render wrapped lines
		lineAbsOffset := 0
		for _, line := range lines {
			if currentY > t.Rect.Max.Y+lineH {
				break
			}

			cursorX := t.Rect.Min.X + padX
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

			for idx, sc := range line {
				if idx == 0 {
					subIsBold = sc.isBold
				}
				if sc.isBold != subIsBold {
					flushSub()
					subIsBold = sc.isBold
				}

				if sc.origIndex == t.CursorIndex {
					caretX = cursorX + len(subsegment)*charW
					caretY = currentY
				}
				subsegment = append(subsegment, sc.char)
			}
			flushSub()

			lineEndAbsIdx := startAbsIdx + lineAbsOffset + len(line)
			if lineEndAbsIdx == t.CursorIndex {
				caretX = cursorX
				caretY = currentY
			}

			currentY += lineH
			lineAbsOffset += len(line)
		}

		paraEndAbsIdx := startAbsIdx + len(paragraph)
		if paraEndAbsIdx == t.CursorIndex {
			caretX = t.Rect.Min.X + padX
			caretY = currentY
		}

		currentY += 8
		startAbsIdx += len(paragraph) + 1 // account for the paragraph characters + the newline
	}

	// Draw the caret blinking (if focused)
	if state.FocusedID == t.CompID {
		if (time.Since(state.StartTime).Milliseconds()/500)%2 == 0 {
			caretColor := color.RGBA{139, 92, 246, 255} // Violet
			if state.TextInputValues["txt_manuscript_editor_streaming"] == "true" {
				caretColor = color.RGBA{0, 255, 150, 255} // Emerald
			}

			pnt.SetGlow(6.0)
			bearingX := state.FontCharBearingX
			pnt.FillRect(image.Rect(caretX + bearingX, caretY - 13, caretX + bearingX + 2, caretY + 3), caretColor)
			pnt.SetGlow(0)
		}
	}

	finalHeight := currentY - t.Rect.Min.Y + padY
	t.Rect.Max.Y = t.Rect.Min.Y + finalHeight
}

func (t *TextArea) OnMouseUp(pt image.Point, state *types.ApplicationState) bool   { return false }
func (t *TextArea) OnMouseMove(pt image.Point, state *types.ApplicationState) bool { return false }

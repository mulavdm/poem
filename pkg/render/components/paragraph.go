package components

import (
	"image"
	"image/color"
	"time"

	"go_native_gpu_gui/pkg/render/types"
)

type styledChar struct {
	char      rune
	isBold    bool
	isHigh    bool
	origIndex int
}

// Paragraph represents a multi-line wrapping text component.
// It supports suffix highlighting (e.g. real-time Scribe token streaming)
// and an optional blinking cursor.
type Paragraph struct {
	CompID         string
	Rect           image.Rectangle
	Text           string
	BaseColor      color.RGBA
	HighlightStart int // -1 if disabled, otherwise character index where highlight begins
	HighlightColor color.RGBA
	ShowCursor     bool
	CharWidth      int // Default 8 if 0
	LineHeight     int // Default 24 if 0
}

func (p *Paragraph) ID() string              { return p.CompID }
func (p *Paragraph) GetID() string           { return p.CompID }
func (p *Paragraph) Bounds() image.Rectangle { return p.Rect }

func (p *Paragraph) SetBounds(r image.Rectangle) {
	p.Rect = r
}

func (p *Paragraph) HitTest(pt image.Point) string {
	if pt.In(p.Rect) {
		return p.CompID
	}
	return ""
}

func (p *Paragraph) Focusable() bool               { return false }
func (p *Paragraph) Walk(fn func(types.Component)) { fn(p) }

func (p *Paragraph) OnKey(key uint32, char rune, state *types.ApplicationState) bool { return false }
func (p *Paragraph) OnMouseDown(pt image.Point, state *types.ApplicationState) bool  { return false }
func (p *Paragraph) OnMouseUp(pt image.Point, state *types.ApplicationState) bool    { return false }
func (p *Paragraph) OnMouseMove(pt image.Point, state *types.ApplicationState) bool  { return false }

func (p *Paragraph) Draw(pnt types.Painter, state *types.ApplicationState) {
	charW := p.CharWidth
	if charW <= 0 {
		charW = 8
	}
	lineH := p.LineHeight
	if lineH <= 0 {
		lineH = 24
	}

	maxWidth := p.Rect.Dx()
	if maxWidth <= charW {
		maxWidth = 300 // Safe fallback
	}

	maxCharsPerLine := maxWidth / charW

	// 1. High-fidelity rich text parsing (Markdown **bold** detection)
	var parsedChars []styledChar
	rawRunes := []rune(p.Text)
	isBold := false

	for i := 0; i < len(rawRunes); i++ {
		if i < len(rawRunes)-1 && rawRunes[i] == '*' && rawRunes[i+1] == '*' {
			isBold = !isBold
			i++ // Skip second asterisk
			continue
		}

		isHigh := p.HighlightStart >= 0 && i >= p.HighlightStart

		parsedChars = append(parsedChars, styledChar{
			char:      rawRunes[i],
			isBold:    isBold,
			isHigh:    isHigh,
			origIndex: i,
		})
	}

	// 2. Split by newline to preserve paragraph boundaries
	var paragraphs [][]styledChar
	var currentPara []styledChar

	for _, sc := range parsedChars {
		if sc.char == '\n' {
			paragraphs = append(paragraphs, currentPara)
			currentPara = nil
		} else {
			currentPara = append(currentPara, sc)
		}
	}
	// Append remaining paragraph
	paragraphs = append(paragraphs, currentPara)

	currentY := p.Rect.Min.Y + lineH - 4 // vertical offset with baseline adjustment

	for _, paragraph := range paragraphs {
		// If empty paragraph, skip a line height
		if len(paragraph) == 0 {
			currentY += lineH
			continue
		}

		// 3. Word-Wrap the styled character slice
		var lines [][]styledChar
		var currentLine []styledChar
		var currentWord []styledChar

		for _, sc := range paragraph {
			if sc.char == ' ' || sc.char == '\t' {
				if len(currentWord) > 0 {
					if len(currentLine) == 0 {
						currentLine = append(currentLine, currentWord...)
					} else if len(currentLine)+1+len(currentWord) <= maxCharsPerLine {
						currentLine = append(currentLine, styledChar{char: ' '})
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
				currentLine = append(currentLine, styledChar{char: ' '})
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

		// 4. Draw each wrapped line of styled segments
		for _, line := range lines {
			if currentY > p.Rect.Max.Y+lineH {
				break
			}

			cursorX := p.Rect.Min.X
			var subsegment []rune
			subIsBold := false
			subIsHigh := false

			flushSub := func() {
				if len(subsegment) == 0 {
					return
				}
				c := p.BaseColor
				if subIsHigh {
					c = p.HighlightColor
					pnt.SetGlow(6.0)
				} else if subIsBold {
					c = color.RGBA{196, 181, 253, 255} // Beautiful light violet for bold sections!
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
					subIsHigh = sc.isHigh
				}
				if sc.isBold != subIsBold || sc.isHigh != subIsHigh {
					flushSub()
					subIsBold = sc.isBold
					subIsHigh = sc.isHigh
				}
				subsegment = append(subsegment, sc.char)
			}
			flushSub()

			currentY += lineH
		}

		// Add space between paragraphs
		currentY += 8
	}

	// 5. Draw blinking cursor if Scribe is streaming
	if p.ShowCursor {
		if (time.Since(state.StartTime).Milliseconds()/500)%2 == 0 {
			// Find the end coordinates of the last line drawn
			paragraphsCount := len(paragraphs)
			if paragraphsCount > 0 {
				lastPara := paragraphs[paragraphsCount-1]
				var lines [][]styledChar
				var currentLine []styledChar
				var currentWord []styledChar

				for _, sc := range lastPara {
					if sc.char == ' ' || sc.char == '\t' {
						if len(currentWord) > 0 {
							if len(currentLine) == 0 {
								currentLine = append(currentLine, currentWord...)
							} else if len(currentLine)+1+len(currentWord) <= maxCharsPerLine {
								currentLine = append(currentLine, styledChar{char: ' '})
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
						currentLine = append(currentLine, styledChar{char: ' '})
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

				lastLineTextLen := 0
				if len(lines) > 0 {
					lastLineTextLen = len(lines[len(lines)-1])
				}

				cursorX := p.Rect.Min.X + lastLineTextLen*charW
				cursorY := currentY - lineH - 8 // align back to the last line

				pnt.FillRect(image.Rect(cursorX, cursorY+4, cursorX+2, cursorY+lineH-4), p.HighlightColor)
			}
		}
	}

	// Update bounds height dynamically so flex layouts scroll perfectly
	finalHeight := currentY - p.Rect.Min.Y
	p.Rect.Max.Y = p.Rect.Min.Y + finalHeight
}

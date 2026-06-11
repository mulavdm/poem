package components

import (
	"image"
	"strings"
)

func defaultCharWidth(charW int) int {
	if charW <= 0 {
		return 8
	}
	return charW
}

func defaultLineHeight(lineH int) int {
	if lineH <= 0 {
		return 24
	}
	return lineH
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minValueInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func explicitSize(rect image.Rectangle) image.Point {
	return image.Pt(rect.Dx(), rect.Dy())
}

func applyExplicitSize(explicit, preferred image.Point) image.Point {
	if explicit.X > 0 {
		preferred.X = explicit.X
	}
	if explicit.Y > 0 {
		preferred.Y = explicit.Y
	}
	return preferred
}

func measurePlainText(text string, charW, lineH int) image.Point {
	runes := []rune(text)
	width := len(runes) * charW
	if width == 0 {
		width = charW
	}
	return image.Pt(width, lineH)
}

func wrapLineCount(text string, maxCharsPerLine int) int {
	if maxCharsPerLine <= 0 {
		maxCharsPerLine = 1
	}

	lines := 0
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines++
			continue
		}

		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines++
			continue
		}

		currentLen := 0
		for _, word := range words {
			wordLen := len([]rune(word))
			if currentLen == 0 {
				currentLen = wordLen
				lines++
				continue
			}
			if currentLen+1+wordLen <= maxCharsPerLine {
				currentLen += 1 + wordLen
				continue
			}
			lines++
			currentLen = wordLen
		}
	}

	if lines == 0 {
		return 1
	}
	return lines
}

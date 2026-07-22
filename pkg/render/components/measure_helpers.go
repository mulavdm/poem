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
	return len(wrapPlainText(text, maxCharsPerLine))
}

func wrapPlainText(text string, maxCharsPerLine int) []string {
	if maxCharsPerLine <= 0 {
		maxCharsPerLine = 1
	}

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}

		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		current := ""
		for _, word := range words {
			if current == "" {
				current = word
				continue
			}
			if len([]rune(current))+1+len([]rune(word)) <= maxCharsPerLine {
				current += " " + word
				continue
			}
			lines = append(lines, current)
			current = word
		}
		if current != "" {
			lines = append(lines, current)
		}
	}

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func longestWordRunes(text string) int {
	longest := 1
	for _, word := range strings.Fields(text) {
		if length := len([]rune(word)); length > longest {
			longest = length
		}
	}
	return longest
}

package cartography

import (
	"bytes"
	"errors"
	"math"
	"sync"
	"unicode/utf8"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

const MaxFontBytes = 32 << 20

type ShapedGlyph struct {
	ID                                   uint32
	Cluster, RuneCount, GlyphCount       int
	XAdvance, YAdvance, XOffset, YOffset float32
}

type ShapedText struct {
	Glyphs                   []ShapedGlyph
	Advance, Ascent, Descent float32
	RightToLeft              bool
}

// TextShaper wraps pinned go-text OpenType shaping. A shaper is safe for
// concurrent tile workers and retains the library's bounded font cache.
type TextShaper struct {
	mu     sync.Mutex
	face   *font.Face
	shaper shaping.HarfbuzzShaper
}

func NewTextShaper(fontBytes []byte) (*TextShaper, error) {
	if len(fontBytes) == 0 || len(fontBytes) > MaxFontBytes {
		return nil, errors.New("cartography: invalid font size")
	}
	face, err := font.ParseTTF(bytes.NewReader(fontBytes))
	if err != nil {
		return nil, err
	}
	result := &TextShaper{face: face}
	result.shaper.SetFontCacheSize(4)
	return result, nil
}

func (shaper *TextShaper) Shape(text, locale string, size float64) (ShapedText, error) {
	if shaper == nil || shaper.face == nil || text == "" || len(text) > 16<<10 || utf8.RuneCountInString(text) > 4096 || math.IsNaN(size) || math.IsInf(size, 0) || size < 6 || size > 256 {
		return ShapedText{}, errors.New("cartography: invalid shaping input")
	}
	runes := []rune(text)
	script := language.LookupScript(runes[0])
	direction := di.DirectionLTR
	if script == language.Arabic || script == language.Hebrew {
		direction = di.DirectionRTL
	}
	input := shaping.Input{Text: runes, RunStart: 0, RunEnd: len(runes), Direction: direction, Face: shaper.face, Size: fixed.Int26_6(math.Round(size * 64)), Script: script, Language: language.NewLanguage(locale)}
	shaper.mu.Lock()
	output := shaper.shaper.Shape(input)
	shaper.mu.Unlock()
	result := ShapedText{Glyphs: make([]ShapedGlyph, len(output.Glyphs)), RightToLeft: direction == di.DirectionRTL, Ascent: float32(output.LineBounds.Ascent) / 64, Descent: float32(output.LineBounds.Descent) / 64}
	for index, glyph := range output.Glyphs {
		result.Glyphs[index] = ShapedGlyph{ID: uint32(glyph.GlyphID), Cluster: glyph.ClusterIndex, RuneCount: glyph.RunesCount(), GlyphCount: glyph.GlyphsCount(), XAdvance: float32(glyph.XAdvance) / 64, YAdvance: float32(glyph.YAdvance) / 64, XOffset: float32(glyph.XOffset) / 64, YOffset: float32(glyph.YOffset) / 64}
		result.Advance += float32(glyph.Advance) / 64
	}
	return result, nil
}

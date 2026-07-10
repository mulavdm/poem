package render

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sort"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"go_native_gpu_gui/pkg/render/draw"
	"go_native_gpu_gui/pkg/render/protocol"
)

type fontAtlasManager struct {
	mu            sync.RWMutex
	primaryPath   string
	fallbackPaths []string
	size          float64
	known         map[rune]struct{}
	dirty         bool
	advances      map[rune]float32
}

func newFontAtlasManager(primaryPath string, fallbackPaths []string, size float64) *fontAtlasManager {
	manager := &fontAtlasManager{primaryPath: primaryPath, fallbackPaths: append([]string(nil), fallbackPaths...), size: size, known: make(map[rune]struct{}, 128), advances: make(map[rune]float32, 128)}
	for r := rune(32); r < 127; r++ {
		manager.known[r] = struct{}{}
	}
	return manager
}

func (m *fontAtlasManager) Observe(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range text {
		if r < 32 {
			continue
		}
		if _, exists := m.known[r]; exists {
			continue
		}
		m.known[r] = struct{}{}
		m.dirty = true
	}
}

func (m *fontAtlasManager) runes() []rune {
	runes := make([]rune, 0, len(m.known))
	for r := range m.known {
		runes = append(runes, r)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })
	return runes
}

func (m *fontAtlasManager) Build(width, height int) (protocol.InitEngine, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pixels, chars, atlasWidth, atlasHeight, lsb, err := buildFallbackFontAtlas(m.primaryPath, m.fallbackPaths, m.size, m.runes())
	if err != nil {
		return protocol.InitEngine{}, 0, err
	}
	out := protocol.InitEngine{Width: int32(width), Height: int32(height), AtlasWidth: int32(atlasWidth), AtlasHeight: int32(atlasHeight), AtlasPixels: pixels, Chars: make([]protocol.CharInfo, 0, len(chars))}
	for _, char := range chars {
		out.Chars = append(out.Chars, protocol.CharInfo{R: char.R, U1: char.U1, V1: char.V1, U2: char.U2, V2: char.V2, Width: char.Width, Height: char.Height, Advance: char.Advance})
		m.advances[rune(char.R)] = float32(char.Advance)
	}
	m.dirty = false
	return out, lsb, nil
}

func (m *fontAtlasManager) TakeUpdate(width, height int) (protocol.InitEngine, bool, error) {
	m.mu.RLock()
	dirty := m.dirty
	m.mu.RUnlock()
	if !dirty {
		return protocol.InitEngine{}, false, nil
	}
	update, _, err := m.Build(width, height)
	return update, err == nil, err
}

func (m *fontAtlasManager) Shape(_ context.Context, run draw.TextRun) (draw.GlyphRun, error) {
	m.Observe(run.Text)
	m.mu.RLock()
	defer m.mu.RUnlock()
	scale := float32(1)
	if run.Font.Size > 0 && m.size > 0 {
		scale = run.Font.Size / float32(m.size)
	}
	glyphs := make([]draw.Glyph, 0, len([]rune(run.Text)))
	position := float32(0)
	fallback := m.advances['?']
	if fallback <= 0 {
		fallback = 8
	}
	for _, r := range run.Text {
		advance := m.advances[r]
		if advance <= 0 {
			advance = fallback
		}
		advance = advance*scale + run.LetterSpacing
		glyphs = append(glyphs, draw.Glyph{Rune: r, GlyphID: uint32(r), Offset: image.Pt(int(position+0.5), 0), Advance: advance})
		position += advance
	}
	return draw.GlyphRun{Glyphs: glyphs, Size: image.Pt(int(position+0.5), int(float32(m.size)*scale+0.5)), Baseline: int(float32(m.size)*0.8*scale + 0.5)}, nil
}

func (m *fontAtlasManager) Measure(ctx context.Context, run draw.TextRun, _ image.Point) (image.Point, error) {
	if err := ctx.Err(); err != nil {
		return image.Point{}, err
	}
	m.Observe(run.Text)
	m.mu.RLock()
	defer m.mu.RUnlock()
	scale := float32(1)
	if run.Font.Size > 0 && m.size > 0 {
		scale = run.Font.Size / float32(m.size)
	}
	fallback := m.advances['?']
	if fallback <= 0 {
		fallback = 8
	}
	width := float32(0)
	for _, r := range run.Text {
		advance := m.advances[r]
		if advance <= 0 {
			advance = fallback
		}
		width += advance*scale + run.LetterSpacing
	}
	return image.Pt(int(width+0.5), int(float32(m.size)*scale+0.5)), nil
}

type atlasFace struct{ face font.Face }
type preparedGlyph struct {
	r                                      rune
	face                                   font.Face
	x, baseline, advancePx, logicalAdvance int
}

func nextPowerOfTwo(value int) int {
	result := 1
	for result < value {
		result <<= 1
	}
	return result
}

func buildFallbackFontAtlas(primary string, fallbacks []string, size float64, runes []rune) ([]byte, []fontCharInfo, int, int, int, error) {
	if size <= 0 {
		size = 14
	}
	const oversample = 2
	paths := append([]string{primary}, fallbacks...)
	faces := make([]atlasFace, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		face, err := loadTTFFace(path, size*oversample)
		if err == nil {
			faces = append(faces, atlasFace{face: face})
		}
	}
	if len(faces) == 0 {
		return nil, nil, 0, 0, 0, fmt.Errorf("no usable font faces")
	}
	defer func() {
		for _, candidate := range faces {
			_ = candidate.face.Close()
		}
	}()

	maxAscent, maxDescent, maxHeight := 0, 0, 0
	for _, candidate := range faces {
		metrics := candidate.face.Metrics()
		ascent := int((metrics.Ascent + 63) >> 6)
		descent := int((metrics.Descent + 63) >> 6)
		height := int((metrics.Height + 63) >> 6)
		if ascent > maxAscent {
			maxAscent = ascent
		}
		if descent > maxDescent {
			maxDescent = descent
		}
		if height > maxHeight {
			maxHeight = height
		}
	}
	if maxHeight <= 0 {
		maxHeight = maxAscent + maxDescent + 4
	}
	lineHeight := maxHeight + 12
	const atlasWidth = 1024
	x, baseline := 2, maxAscent+6
	prepared := make([]preparedGlyph, 0, len(runes))
	measuredLSB := 0
	for _, r := range runes {
		var selected font.Face
		var advance fixed.Int26_6
		for _, candidate := range faces {
			if candidateAdvance, ok := candidate.face.GlyphAdvance(r); ok {
				selected, advance = candidate.face, candidateAdvance
				break
			}
		}
		if selected == nil {
			continue
		}
		advancePx := int((advance + 63) >> 6)
		if advancePx <= 0 {
			advancePx = 2 * oversample
		}
		if x+advancePx+2 > atlasWidth {
			x = 2
			baseline += lineHeight
		}
		logicalAdvance := (advancePx + oversample/2) / oversample
		prepared = append(prepared, preparedGlyph{r: r, face: selected, x: x, baseline: baseline, advancePx: advancePx, logicalAdvance: logicalAdvance})
		if r == 'A' {
			if bounds, _, ok := selected.GlyphBounds(r); ok {
				lsb := (int(bounds.Min.X>>6) + oversample/2) / oversample
				if lsb > 0 && lsb < logicalAdvance {
					measuredLSB = lsb
				}
			}
		}
		x += advancePx + 2
	}
	atlasHeight := nextPowerOfTwo(baseline + maxDescent + 8)
	if atlasHeight < 128 {
		atlasHeight = 128
	}
	if atlasHeight > 2048 {
		return nil, nil, 0, 0, 0, fmt.Errorf("font atlas exceeds 1024x2048 resource limit")
	}
	rgba := image.NewRGBA(image.Rect(0, 0, atlasWidth, atlasHeight))
	chars := make([]fontCharInfo, 0, len(prepared))
	topInset, bottomInset := maxAscent+4, maxDescent+4
	for _, glyph := range prepared {
		drawer := font.Drawer{Dst: rgba, Src: image.NewUniform(color.White), Face: glyph.face, Dot: fixed.P(glyph.x, glyph.baseline)}
		drawer.DrawString(string(glyph.r))
		top, bottom := glyph.baseline-topInset, glyph.baseline+bottomInset
		if top < 0 {
			top = 0
		}
		if bottom > atlasHeight {
			bottom = atlasHeight
		}
		chars = append(chars, fontCharInfo{R: int32(glyph.r), U1: float32(glyph.x) / atlasWidth, V1: float32(top) / float32(atlasHeight), U2: float32(glyph.x+glyph.advancePx) / atlasWidth, V2: float32(bottom) / float32(atlasHeight), Width: int32(glyph.logicalAdvance), Height: int32((bottom - top + oversample/2) / oversample), Advance: int32(glyph.logicalAdvance)})
	}
	return rgba.Pix, chars, atlasWidth, atlasHeight, measuredLSB, nil
}

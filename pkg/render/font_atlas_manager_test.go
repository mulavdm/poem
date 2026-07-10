package render

import (
	"context"
	"image"
	"testing"

	"go_native_gpu_gui/pkg/render/draw"
)

func TestFontAtlasAddsObservedUnicodeGlyphs(t *testing.T) {
	manager := newFontAtlasManager(resolveDefaultFontPath(), resolveDefaultFallbackFontPaths(), 14)
	initial, _, err := manager.Build(800, 600)
	if err != nil {
		t.Fatal(err)
	}
	manager.Observe("日本語")
	update, changed, err := manager.TakeUpdate(800, 600)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || len(update.Chars) <= len(initial.Chars) {
		t.Fatalf("unicode atlas did not grow: %d -> %d", len(initial.Chars), len(update.Chars))
	}
	wanted := map[int32]bool{'日': false, '本': false, '語': false}
	for _, char := range update.Chars {
		if _, ok := wanted[char.R]; ok {
			wanted[char.R] = true
			left := int(char.U1 * float32(update.AtlasWidth))
			right := int(char.U2 * float32(update.AtlasWidth))
			top := int(char.V1 * float32(update.AtlasHeight))
			bottom := int(char.V2 * float32(update.AtlasHeight))
			nonzero := false
			for y := top; y < bottom && !nonzero; y++ {
				for x := left; x < right; x++ {
					if update.AtlasPixels[(y*int(update.AtlasWidth)+x)*4+3] != 0 {
						nonzero = true
						break
					}
				}
			}
			if !nonzero {
				t.Fatalf("glyph %q has no rasterized pixels", rune(char.R))
			}
		}
	}
	for char, found := range wanted {
		if !found {
			t.Fatalf("glyph %q missing from update", rune(char))
		}
	}
}

func TestFontAtlasObserverDoesNotRebuildKnownGlyphs(t *testing.T) {
	manager := newFontAtlasManager(resolveDefaultFontPath(), resolveDefaultFallbackFontPaths(), 14)
	if _, _, err := manager.Build(800, 600); err != nil {
		t.Fatal(err)
	}
	manager.Observe("ASCII only")
	if _, changed, err := manager.TakeUpdate(800, 600); err != nil || changed {
		t.Fatalf("known glyphs caused update changed=%v err=%v", changed, err)
	}
}

func TestFontAtlasShaperUsesPerGlyphAdvances(t *testing.T) {
	manager := newFontAtlasManager(resolveDefaultFontPath(), resolveDefaultFallbackFontPaths(), 14)
	manager.Observe("A日本語")
	if _, _, err := manager.Build(800, 600); err != nil {
		t.Fatal(err)
	}
	run := draw.TextRun{Text: "A日本語", Font: draw.Font{Size: 14}}
	shaped, err := manager.Shape(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	measured, err := manager.Measure(context.Background(), run, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if len(shaped.Glyphs) != 4 || measured.X != shaped.Size.X {
		t.Fatalf("glyphs=%d shaped=%v measured=%v", len(shaped.Glyphs), shaped.Size, measured)
	}
	if shaped.Glyphs[1].Advance <= 0 || shaped.Glyphs[3].Offset.X <= shaped.Glyphs[1].Offset.X {
		t.Fatalf("invalid unicode advances: %+v", shaped.Glyphs)
	}
}

func BenchmarkFontAtlasMeasureUnicode(b *testing.B) {
	manager := newFontAtlasManager(resolveDefaultFontPath(), resolveDefaultFallbackFontPaths(), 14)
	manager.Observe("Professional 日本語")
	if _, _, err := manager.Build(800, 600); err != nil {
		b.Fatal(err)
	}
	run := draw.TextRun{Text: "Professional 日本語", Font: draw.Font{Size: 14}}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := manager.Measure(context.Background(), run, image.Point{}); err != nil {
			b.Fatal(err)
		}
	}
}

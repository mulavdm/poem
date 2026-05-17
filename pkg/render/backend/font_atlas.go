//go:build gpu

package backend

import (
	"image"

	"github.com/go-gl/gl/v4.1-core/gl"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type FontAtlas struct {
	TextureID uint32
	CharMap   map[rune]CharInfo
	Width     int
	Height    int
}

type CharInfo struct {
	U1, V1  float32 // Top-left
	U2, V2  float32 // Bottom-right
	Width   int
	Height  int
	Advance int
}

func NewFontAtlas() *FontAtlas {
	fa := &FontAtlas{
		CharMap: make(map[rune]CharInfo),
	}

	// We'll render printable ASCII for now
	face := basicfont.Face7x13
	atlasW := 128
	atlasH := 128
	fa.Width = atlasW
	fa.Height = atlasH

	rgba := image.NewRGBA(image.Rect(0, 0, atlasW, atlasH))
	d := &font.Drawer{
		Dst:  rgba,
		Src:  image.White,
		Face: face,
	}

	x, y := 0, 13
	for r := rune(32); r < 127; r++ {
		advance, ok := face.GlyphAdvance(r)
		if !ok {
			continue
		}

		// Wrap to next line if needed
		if x+int(advance>>6) > atlasW {
			x = 0
			y += 15
		}

		d.Dot = fixed.P(x, y)
		d.DrawString(string(r))

		fa.CharMap[r] = CharInfo{
			U1:      float32(x) / float32(atlasW),
			V1:      float32(y-11) / float32(atlasH),
			U2:      float32(x+int(advance>>6)) / float32(atlasW),
			V2:      float32(y+2) / float32(atlasH),
			Width:   int(advance >> 6),
			Height:  13,
			Advance: int(advance >> 6),
		}

		x += int(advance >> 6)
	}

	// Upload to OpenGL
	gl.GenTextures(1, &fa.TextureID)
	gl.BindTexture(gl.TEXTURE_2D, fa.TextureID)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(atlasW), int32(atlasH), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))

	return fa
}

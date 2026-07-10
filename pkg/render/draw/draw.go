// Package draw defines backend-neutral rendering values. The current protocol
// painter adapts these concepts to D3D11; future backends can consume them too.
package draw

import (
	"image"
	"image/color"
)

type Font struct {
	Family string
	Size   float32
	Weight uint16
	Italic bool
}

type TextRun struct {
	Text          string
	Font          Font
	Color         color.RGBA
	LetterSpacing float32
}

type Glyph struct {
	Rune    rune
	GlyphID uint32
	Offset  image.Point
	Advance float32
}

type GlyphRun struct {
	Glyphs   []Glyph
	Size     image.Point
	Baseline int
}

type Stop struct {
	Position float32
	Color    color.RGBA
}

type LinearGradient struct {
	From, To image.Point
	Stops    []Stop
}

type Paint struct {
	Fill        color.RGBA
	Stroke      color.RGBA
	StrokeWidth float32
	Radius      float32
	Opacity     float32
	Gradient    *LinearGradient
}

type PathVerb uint8

const (
	MoveTo PathVerb = iota
	LineTo
	QuadTo
	CubicTo
	Close
)

type PathCommand struct {
	Verb   PathVerb
	Points [3]image.Point
}

type Path struct{ Commands []PathCommand }

type Operation interface{ isOperation() }

type Rect struct {
	Bounds image.Rectangle
	Paint  Paint
}

func (Rect) isOperation() {}

type Text struct {
	Position image.Point
	Run      TextRun
}

func (Text) isOperation() {}

type VectorPath struct {
	Path  Path
	Paint Paint
}

func (VectorPath) isOperation() {}

type Frame struct {
	Size       image.Point
	Operations []Operation
}

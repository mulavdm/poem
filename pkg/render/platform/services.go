// Package platform defines optional OS capabilities without exposing OS types.
package platform

import (
	"context"
	"errors"
	"image"

	"go_native_gpu_gui/pkg/render/draw"
	"go_native_gpu_gui/pkg/render/semantics"
	"go_native_gpu_gui/pkg/render/theme"
)

var ErrUnsupported = errors.New("platform capability is not supported")

type Clipboard interface {
	ReadText(context.Context) (string, error)
	WriteText(context.Context, string) error
}

type TextInput interface {
	StartComposition(context.Context, string) error
	StopComposition(context.Context) error
}

type TextShaper interface {
	Shape(context.Context, draw.TextRun) (draw.GlyphRun, error)
	Measure(context.Context, draw.TextRun, image.Point) (image.Point, error)
}

type Presenter interface {
	Present(context.Context, draw.Frame) error
	Resize(context.Context, image.Point) error
}

type Cursor string

const (
	CursorArrow     Cursor = "arrow"
	CursorHand      Cursor = "hand"
	CursorText      Cursor = "text"
	CursorResizeNS  Cursor = "resize-ns"
	CursorResizeEW  Cursor = "resize-ew"
	CursorResizeAll Cursor = "resize-all"
)

type Cursors interface {
	Set(context.Context, Cursor) error
}

type DialogOptions struct{ Title, InitialPath string }
type Dialogs interface {
	OpenFile(context.Context, DialogOptions) (string, error)
	OpenDirectory(context.Context, DialogOptions) (string, error)
	SaveFile(context.Context, DialogOptions) (string, error)
}

type Accessibility interface {
	Publish(context.Context, semantics.Tree) error
}

type SystemTheme interface {
	Current(context.Context) (theme.Mode, error)
}

type PreferenceSnapshot struct {
	ThemeMode     theme.Mode
	ReducedMotion bool
}

type SystemPreferences interface {
	Current(context.Context) (PreferenceSnapshot, error)
}

type Window interface {
	SetTitle(context.Context, string) error
	RequestAttention(context.Context) error
}

// Services is a capability set. Nil services are unsupported by definition.
type Services struct {
	Presenter     Presenter
	TextShaper    TextShaper
	Clipboard     Clipboard
	TextInput     TextInput
	Cursors       Cursors
	Dialogs       Dialogs
	Accessibility Accessibility
	SystemTheme   SystemTheme
	Preferences   SystemPreferences
	Window        Window
}

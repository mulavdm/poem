// Package events contains platform-neutral input events delivered to controls.
package events

import "image"

type Type uint8

const (
	PointerMove Type = iota
	PointerDown
	PointerUp
	PointerWheel
	KeyDown
	KeyUp
	TextInput
	CompositionStart
	CompositionUpdate
	CompositionEnd
	FocusIn
	FocusOut
)

type Modifiers struct{ Shift, Control, Alt, Meta bool }

type Event struct {
	Type      Type
	Position  image.Point
	Delta     image.Point
	Button    int
	Key       string
	Text      string
	Modifiers Modifiers
}

type Result struct {
	Handled        bool
	RequestFocusID string
	RequestRepaint bool
}

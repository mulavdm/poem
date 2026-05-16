package render

import (
	"image"
	"image/color"
	"time"
)

// UIRenderer defines the structural contract both engines must fulfill
type UIRenderer interface {
	Setup(hdc uintptr) error
	SetSize(w, h int)
	Paint(hdc uintptr, appState *ApplicationState)
}

// Painter abstracts basic drawing operations for components
type Painter interface {
	DrawRoundedRect(r image.Rectangle, radius int, col color.RGBA)
	DrawText(s string, x, y int, col color.RGBA)
	FillRect(r image.Rectangle, col color.RGBA)
}

// ApplicationState acts as your shared backend state data framework
type ApplicationState struct {
	ClickCount int
	StatusText string
	StartTime  time.Time
	
	// Input State
	MouseX    int
	MouseY    int
	HoveredID string
	FocusedID string
	ActiveID  string // ID of the component currently capturing the mouse (e.g., for dragging)
	CursorID  uintptr // Current cursor handle
	
	// Registry for buttons and interactive elements
	Components []Component

	// Navigation State
	CurrentPage string

	// App Data
	Volume float32

	// Telemetry
	CurrentFPS float64
	CoreMask   uint64
}

const (
	PageDashboard = "dashboard"
	PageAnalytics = "analytics"
	PageSettings  = "settings"
)

func (s *ApplicationState) CycleFocus(reverse bool) {
	var focusable []string
	for _, c := range s.Components {
		c.Walk(func(comp Component) {
			if comp.Focusable() {
				focusable = append(focusable, comp.ID())
			}
		})
	}

	if len(focusable) == 0 {
		return
	}

	idx := -1
	for i, id := range focusable {
		if id == s.FocusedID {
			idx = i
			break
		}
	}

	if reverse {
		if idx == -1 {
			idx = len(focusable) - 1
		} else {
			idx = (idx - 1 + len(focusable)) % len(focusable)
		}
	} else {
		if idx == -1 {
			idx = 0
		} else {
			idx = (idx + 1) % len(focusable)
		}
	}
	s.FocusedID = focusable[idx]
}

// Component represents a UI element that can be drawn and interacted with
type Component interface {
	ID() string
	GetID() string
	Bounds() image.Rectangle
	SetBounds(r image.Rectangle)
	Draw(p Painter, state *ApplicationState)
	HitTest(pt image.Point) string
	OnKey(key uint32, char rune, state *ApplicationState) bool
	OnMouseDown(pt image.Point, state *ApplicationState) bool
	OnMouseUp(pt image.Point, state *ApplicationState) bool
	OnMouseMove(pt image.Point, state *ApplicationState) bool
	Focusable() bool
	Walk(fn func(Component))
}

var (
	Width  = 1024
	Height = 768
)

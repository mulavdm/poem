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
	DrawText(text string, x, y int, col color.RGBA)
	FillRect(r image.Rectangle, col color.RGBA)
	DrawLine(x1, y1, x2, y2 int, col color.RGBA)
	SetGlow(strength float32)
	SetGlass(enabled bool)
	SetShadow(ox, oy, blur float32)
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
	Components []Component // Legacy/Global components
	
	// Page Management
	Pages       map[string][]Component
	CurrentPage string
	TargetPage  string
	PrevPage    string
	
	// Animation State
	TransitionProgress float32 // 0.0 to 1.0
	IsTransitioning    bool
	TransitionType     int // 0: Fade, 1: Slide

	// App Data
	Volume float32

	// Telemetry
	CurrentFPS float64
	CoreMask   uint64

	// VFX
	Particles    *ParticleSystem
	GlassEnabled bool
	FrameTime    time.Duration
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

func (s *ApplicationState) NavigateTo(page string) {
	if s.CurrentPage == page || s.IsTransitioning {
		return
	}
	s.PrevPage = s.CurrentPage
	s.TargetPage = page
	s.IsTransitioning = true
	s.TransitionProgress = 0
}

func (s *ApplicationState) UpdateAnimations(dt float32) {
	if s.IsTransitioning {
		s.TransitionProgress += dt * 2.0 // 0.5s transition
		if s.TransitionProgress >= 1.0 {
			s.TransitionProgress = 1.0
			s.CurrentPage = s.TargetPage
			s.IsTransitioning = false
		}
	}
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

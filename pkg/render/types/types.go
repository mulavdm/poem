package types

import (
	"image"
	"image/color"
	"math"
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
	SetOffset(x, y float32)
	Flush()
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
	ActiveID  string  // ID of the component currently capturing the mouse (e.g., for dragging)
	CursorID  uintptr // Active dynamic cursor handle
	ArrowCursor uintptr // System IDC_ARROW cursor
	HandCursor  uintptr // System IDC_HAND cursor
	IBeamCursor uintptr // System IDC_IBEAM cursor

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

	// Live Telemetry Stream History Slices
	FPSHistory  []float32
	HeapHistory []float32
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

func (s *ApplicationState) NavigateTo(pageID string) {
	if s.CurrentPage == pageID || s.IsTransitioning {
		return
	}
	s.PrevPage = s.CurrentPage
	s.TargetPage = pageID
	s.CurrentPage = pageID
	s.TransitionProgress = 0
	s.IsTransitioning = true
}

func (s *ApplicationState) UpdateAnimations(dt float32) {
	if s.IsTransitioning {
		s.TransitionProgress += dt * 2.5 // Speed of transition
		if s.TransitionProgress >= 1.0 {
			s.TransitionProgress = 1.0
			s.IsTransitioning = false
		}
	}
}

// RenderPipeline is the universal orchestration logic for the POEM engine.
// It ensures that both CPU and GPU backends follow the exact same drawing sequence.
func RenderPipeline(p Painter, s *ApplicationState) {
	// Reset active cursor to default arrow at the beginning of each drawing tick
	s.CursorID = s.ArrowCursor

	// 1. BACKGROUND PARTICLES
	if s.Particles != nil {
		s.Particles.Draw(p, s)
	}
	p.Flush()

	// 2. HIT TESTING (Universal)
	mousePoint := image.Point{s.MouseX, s.MouseY}
	s.HoveredID = ""
	if page, ok := s.Pages[s.CurrentPage]; ok {
		// Run a recursive layout pass first to ensure all child component bounds are calculated
		for _, comp := range page {
			comp.SetBounds(comp.Bounds())
		}
		for i := len(page) - 1; i >= 0; i-- {
			if id := page[i].HitTest(mousePoint); id != "" {
				s.HoveredID = id
				break
			}
		}
	}

	// 3. UI PASS (Draw current page with transitions)
	p.SetOffset(0, 0)

	if s.IsTransitioning && s.TransitionProgress < 1.0 {
		progress := s.TransitionProgress

		// Draw previous page (sliding out)
		if comps, ok := s.Pages[s.PrevPage]; ok {
			p.SetOffset(-progress*float32(Width), 0)
			for _, comp := range comps {
				comp.Draw(p, s)
			}
			p.Flush()
		}

		// Draw current page (sliding in)
		if comps, ok := s.Pages[s.CurrentPage]; ok {
			p.SetOffset((1.0-progress)*float32(Width), 0)
			for _, comp := range comps {
				comp.Draw(p, s)
			}
			p.Flush()
		}
	} else {
		// Draw current page normally
		if page, ok := s.Pages[s.CurrentPage]; ok {
			for _, comp := range page {
				comp.Draw(p, s)
			}
			p.Flush()
		}
	}

	// 4. OVERLAYS (Independent of scroll/transition)
	p.SetOffset(0, 0)

	// Global System Status (Bottom Right)
	statusCol := color.RGBA{0, 255, 150, 180}
	elapsed := time.Since(s.StartTime).Seconds()
	pulse := uint8(150 + math.Sin(elapsed*5)*100)
	statusCol.A = pulse

	p.DrawText("SYSTEM OPERATIONAL // ENCRYPTED", Width-280, Height-25, statusCol)

	// Small diagnostic line
	p.DrawLine(Width-285, Height-15, Width-20, Height-15, color.RGBA{0, 255, 150, 50})
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

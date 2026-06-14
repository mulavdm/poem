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
	DrawImage(r image.Rectangle, imageWidth, imageHeight int, pixels []byte)
	DrawRaycaster(r image.Rectangle, playerX, playerY, playerAngle float32)
	DrawRaycasterStyled(r image.Rectangle, playerX, playerY, playerAngle float32, accent color.RGBA)
	DrawRaycasterMapStyled(r image.Rectangle, playerX, playerY, playerAngle float32, accent color.RGBA, mapData string)
	DrawBillboard3D(viewportRect image.Rectangle, worldX, worldY float32, kind int, col color.RGBA)
	DrawSeed3D(viewportRect image.Rectangle, seedX, seedY, playerX, playerY, playerAngle float32)
	DrawSentry3D(viewportRect image.Rectangle, sentryX, sentryY, playerX, playerY, playerAngle float32, col color.RGBA)
	SetGlow(strength float32)
	SetGlass(enabled bool)
	SetShadow(ox, oy, blur float32)
	SetOffset(x, y float32)
	SetClip(r image.Rectangle)
	PushClip(r image.Rectangle)
	PopClip()
	Flush()
}

// ScrollContainer defines the interface ScrollViews implement to scroll focused items into view
type ScrollContainer interface {
	Component
	ScrollToChild(childID string, childBounds image.Rectangle, state *ApplicationState) bool
}

// ApplicationState acts as your shared backend state data framework
type ApplicationState struct {
	ClickCount int
	StatusText string
	StartTime  time.Time

	// Input State
	MouseX      int
	MouseY      int
	HoveredID   string
	FocusedID   string
	ActiveID    string // ID of the component currently capturing the mouse (e.g., for dragging)
	KeysPressed map[uint32]bool
	CursorID    uintptr // Active dynamic cursor handle
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
	Particles        *ParticleSystem
	ParticlesEnabled bool
	BGColor          *color.RGBA // If set, overrides the default background/clear color
	GlassEnabled     bool
	FrameTime        time.Duration
	LastDt           float64

	// Window Dimensions
	WindowWidth          int
	WindowHeight         int
	PhysicalWindowWidth  int
	PhysicalWindowHeight int
	LastPaintTime        time.Time
	RenderDt             float64
	NeedsRepaint         bool

	// Live Telemetry Stream History Slices
	FPSHistory  []float32
	HeapHistory []float32

	// Persistent Scroll Viewport State
	ScrollPositions map[string]int
	ScrollDragStart map[string]int
	ScrollStartY    map[string]int
	ScrollCurrent   map[string]float64

	// Persistent Input State
	TextInputValues map[string]string
	SliderValues    map[string]float32

	// Hotkeys registry
	Hotkeys map[string]HotkeyHandler

	// Acoustic Native Sound Engine
	AudioEnabled bool
	PlaySoundFn  func(soundType int8)

	// Dynamic font metrics detected at boot time
	FontCharWidth    int // Full cell advance width in pixels
	FontCharBearingX int // Left Side Bearing (LSB) in pixels
}

func (s *ApplicationState) PlayHover() {
	if s.PlaySoundFn != nil && s.AudioEnabled {
		s.PlaySoundFn(0) // SoundTypeHover
	}
}

func (s *ApplicationState) PlayClick() {
	if s.PlaySoundFn != nil && s.AudioEnabled {
		s.PlaySoundFn(1) // SoundTypeClick
	}
}

func (s *ApplicationState) PlaySuccess() {
	if s.PlaySoundFn != nil && s.AudioEnabled {
		s.PlaySoundFn(2) // SoundTypeSuccess
	}
}

type HotkeyHandler func(state *ApplicationState)

func (s *ApplicationState) RegisterHotkey(shortcut string, handler HotkeyHandler) {
	if s.Hotkeys == nil {
		s.Hotkeys = make(map[string]HotkeyHandler)
	}
	s.Hotkeys[shortcut] = handler
}

const (
	PageDashboard = "dashboard"
	PageAnalytics = "analytics"
	PageSettings  = "settings"
)

func (s *ApplicationState) CycleFocus(reverse bool) {
	var focusable []string
	var focusableBounds = make(map[string]image.Rectangle)

	if comps, ok := s.Pages[s.CurrentPage]; ok {
		for _, c := range comps {
			c.Walk(func(comp Component) {
				if comp.Focusable() {
					focusable = append(focusable, comp.ID())
					focusableBounds[comp.ID()] = comp.Bounds()
				}
			})
		}
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

	// Automatically scroll the focused component into view if it is inside a ScrollContainer
	if comps, ok := s.Pages[s.CurrentPage]; ok {
		for _, c := range comps {
			c.Walk(func(comp Component) {
				if sc, ok := comp.(ScrollContainer); ok {
					sc.ScrollToChild(s.FocusedID, focusableBounds[s.FocusedID], s)
				}
			})
		}
	}
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

// GetWindowSize returns the dynamic logical width and height of the window
func (s *ApplicationState) GetWindowSize() (int, int) {
	if s.WindowWidth <= 0 {
		return 1024, 768
	}
	return s.WindowWidth, s.WindowHeight
}

// RenderPipeline is the universal orchestration logic for the POEM engine.
// It ensures that both CPU and GPU backends follow the exact same drawing sequence.
func RenderPipeline(p Painter, s *ApplicationState) {
	// Reset active cursor to default arrow at the beginning of each drawing tick
	s.CursorID = s.ArrowCursor

	w, h := s.GetWindowSize()

	// 0. CUSTOM BACKGROUND OVERRIDE
	if s.BGColor != nil {
		p.FillRect(image.Rect(0, 0, w, h), *s.BGColor)
	}

	// 1. BACKGROUND PARTICLES
	if s.Particles != nil && s.ParticlesEnabled {
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
			p.SetOffset(-progress*float32(w), 0)
			for _, comp := range comps {
				comp.Draw(p, s)
			}
			p.Flush()
		}

		// Draw current page (sliding in)
		if comps, ok := s.Pages[s.CurrentPage]; ok {
			p.SetOffset((1.0-progress)*float32(w), 0)
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

	statusText := "SYSTEM OPERATIONAL // ENCRYPTED"
	charWidth := s.FontCharWidth
	if charWidth <= 0 {
		charWidth = 8
	}
	statusWidth := len([]rune(statusText)) * charWidth
	statusX := w - statusWidth - 20
	if statusX < 20 {
		statusX = 20
	}

	p.DrawText(statusText, statusX, h-25, statusCol)

	// Small diagnostic line
	lineStartX := statusX - 5
	if lineStartX < 20 {
		lineStartX = 20
	}
	p.DrawLine(lineStartX, h-15, w-20, h-15, color.RGBA{0, 255, 150, 50})
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

// MeasureResult describes a component's preferred and minimum size without
// introducing browser-style layout semantics.
type MeasureResult struct {
	Preferred image.Point
	Min       image.Point
}

// MeasurableComponent is an optional contract for components that can report
// preferred sizing more accurately than raw Bounds().
type MeasurableComponent interface {
	Measure(avail image.Point, state *ApplicationState) MeasureResult
}

// ContentSizedComponent is an optional companion to Measure for containers
// whose assigned Bounds() may intentionally be smaller than their laid-out
// content, such as a FlexBox inside a ScrollView.
type ContentSizedComponent interface {
	ContentSize(avail image.Point, state *ApplicationState) image.Point
}

func clampMeasurePoint(pt image.Point) image.Point {
	if pt.X < 0 {
		pt.X = 0
	}
	if pt.Y < 0 {
		pt.Y = 0
	}
	return pt
}

// NormalizeMeasureResult ensures a measurement always has a usable preferred
// size and that min sizes never exceed preferred sizes.
func NormalizeMeasureResult(result MeasureResult, fallback image.Rectangle) MeasureResult {
	result.Preferred = clampMeasurePoint(result.Preferred)
	result.Min = clampMeasurePoint(result.Min)

	if result.Preferred.X == 0 && fallback.Dx() > 0 {
		result.Preferred.X = fallback.Dx()
	}
	if result.Preferred.Y == 0 && fallback.Dy() > 0 {
		result.Preferred.Y = fallback.Dy()
	}
	if result.Min.X == 0 {
		result.Min.X = result.Preferred.X
	}
	if result.Min.Y == 0 {
		result.Min.Y = result.Preferred.Y
	}
	if result.Min.X > result.Preferred.X {
		result.Min.X = result.Preferred.X
	}
	if result.Min.Y > result.Preferred.Y {
		result.Min.Y = result.Preferred.Y
	}
	return result
}

// MeasureComponent returns a component measurement, falling back to Bounds()
// when the component has not opted into the native measurement contract yet.
func MeasureComponent(comp Component, avail image.Point, state *ApplicationState) MeasureResult {
	if measurable, ok := comp.(MeasurableComponent); ok {
		return NormalizeMeasureResult(measurable.Measure(clampMeasurePoint(avail), state), comp.Bounds())
	}
	bounds := comp.Bounds()
	return NormalizeMeasureResult(MeasureResult{
		Preferred: image.Pt(bounds.Dx(), bounds.Dy()),
		Min:       image.Pt(bounds.Dx(), bounds.Dy()),
	}, bounds)
}

// MeasureContent returns the full laid-out content size for components that
// can distinguish content size from their assigned drawing bounds.
func MeasureContent(comp Component, avail image.Point, state *ApplicationState) image.Point {
	if contentSized, ok := comp.(ContentSizedComponent); ok {
		return clampMeasurePoint(contentSized.ContentSize(clampMeasurePoint(avail), state))
	}
	return MeasureComponent(comp, avail, state).Preferred
}

// ScrollableComponent represents a component that responds to mouse wheel actions
type ScrollableComponent interface {
	Component
	OnMouseWheel(pt image.Point, delta int, state *ApplicationState) bool
}

var (
	Width  = 1024
	Height = 768
)

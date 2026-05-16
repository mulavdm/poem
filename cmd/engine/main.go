package main

import (
	"fmt"
	"image"
	"net/http"
	_ "net/http/pprof" // Profiling
	"runtime"
	"syscall"
	"time"

	"go_native_gpu_gui/internal/render"
	"go_native_gpu_gui/internal/win32"
)

var (
	hdc         uintptr
	activeState = &render.ApplicationState{
		StatusText:   "Engine Running Synchronized Component Tree",
		Volume:       75.0,
		GlassEnabled: true,
	}
	uiEngine    render.UIRenderer
)

func findComponent(id string) render.Component {
	if id == "" {
		return nil
	}
	var found render.Component
	// Search in current page components
	if comps, ok := activeState.Pages[activeState.CurrentPage]; ok {
		for _, c := range comps {
			c.Walk(func(comp render.Component) {
				if comp.ID() == id {
					found = comp
				}
			})
		}
	}
	return found
}

func findHoveredComponent(pt image.Point) string {
	if comps, ok := activeState.Pages[activeState.CurrentPage]; ok {
		for i := len(comps) - 1; i >= 0; i-- {
			if id := comps[i].HitTest(pt); id != "" {
				return id
			}
		}
	}
	return ""
}

func wndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case 0x000F: // WM_PAINT
		uiEngine.Paint(hdc, activeState)
		return 0
	case 0x0005: // WM_SIZE
		w := int(lparam & 0xFFFF)
		h := int(lparam >> 16)
		if w > 0 && h > 0 {
			render.Width = w
			render.Height = h
			if uiEngine != nil {
				uiEngine.SetSize(w, h)
				// Re-generate layout to adapt to new dimensions
				BuildAllPages(activeState)
			}
		}
		return 0
	case 0x0201: // WM_LBUTTONDOWN
		activeState.ClickCount++
		pt := image.Point{activeState.MouseX, activeState.MouseY}
		newFocus := findHoveredComponent(pt)
		activeState.FocusedID = newFocus

		pt = image.Point{X: int(win32.GET_X_LPARAM(lparam)), Y: int(win32.GET_Y_LPARAM(lparam))}
		activeState.ActiveID = findHoveredComponent(pt)
		if activeState.ActiveID != "" {
			if comp := findComponent(activeState.ActiveID); comp != nil {
				if comp.OnMouseDown(pt, activeState) {
					// Sync state-driven components before layout refresh
					if s, ok := comp.(*render.Slider); ok {
						activeState.Volume = s.Value
					}
					// Only rebuild if it's a structural change, but for simplicity:
					BuildAllPages(activeState)
				}
				win32.SetCapture(hwnd)
			}
		}

		activeState.StatusText = fmt.Sprintf("Interaction Captured: %d Clicks Recorded | Focus: %s", activeState.ClickCount, newFocus)
		win32.InvalidateRect(hwnd, nil, false)
		return 0
	case 0x0202: // WM_LBUTTONUP
		activeState.ActiveID = ""
		win32.ReleaseCapture()
		win32.InvalidateRect(hwnd, nil, false)
		return 0
	case 0x0200: // WM_MOUSEMOVE
		activeState.MouseX = int(int16(lparam & 0xFFFF))
		activeState.MouseY = int(int16(lparam >> 16))
		pt := image.Point{activeState.MouseX, activeState.MouseY}

		// Identify hovered component
		newHover := ""
		if comps, ok := activeState.Pages[activeState.CurrentPage]; ok {
			for i := len(comps) - 1; i >= 0; i-- {
				if id := comps[i].HitTest(pt); id != "" {
					newHover = id
					break
				}
			}
		}
		activeState.HoveredID = newHover

		// Route to active component if dragging
		if activeState.ActiveID != "" {
			if comp := findComponent(activeState.ActiveID); comp != nil {
				if comp.OnMouseMove(pt, activeState) {
					// Sync dragging state
					if s, ok := comp.(*render.Slider); ok {
						activeState.Volume = s.Value
					}
				}
			}
		} else if newHover != "" {
			if comp := findComponent(newHover); comp != nil {
				comp.OnMouseMove(pt, activeState)
			}
		}

		return 0
	case 0x0100: // WM_KEYDOWN
		const VK_TAB = 0x09
		const VK_SHIFT = 0x10
		if wparam == VK_TAB {
			reverse := win32.GetKeyState(VK_SHIFT) < 0
			activeState.CycleFocus(reverse)
			win32.InvalidateRect(hwnd, nil, false)
			return 0
		}

		// Forward special keys to focused component
		if activeState.FocusedID != "" {
			if comps, ok := activeState.Pages[activeState.CurrentPage]; ok {
				for _, c := range comps {
					if c.OnKey(uint32(wparam), 0, activeState) {
						BuildAllPages(activeState)
						win32.InvalidateRect(hwnd, nil, false)
						break
					}
				}
			}
		}
		return 0
	case 0x0102: // WM_CHAR
		// Forward character input to focused component
		if activeState.FocusedID != "" {
			if comps, ok := activeState.Pages[activeState.CurrentPage]; ok {
				for _, c := range comps {
					if c.OnKey(0, rune(wparam), activeState) {
						// State might have changed (e.g. navigation), refresh layout
						BuildAllPages(activeState)
						win32.InvalidateRect(hwnd, nil, false)
						break
					}
				}
			}
		}
		return 0
	case 0x0002: // WM_DESTROY
		win32.PostQuitMessage(0)
		syscall.Exit(0)
		return 0
	}
	return win32.DefWindowProc(hwnd, msg, wparam, lparam)
}

func init() {
	runtime.LockOSThread()
}

func main() {
	// Start Profiling Server (Internal Performance Tracking)
	go func() {
		fmt.Println("📊 Performance Profiler active at http://127.0.0.1:6060/debug/pprof/")
		if err := http.ListenAndServe("127.0.0.1:6060", nil); err != nil {
			fmt.Printf("Profiler server failed: %v\n", err)
		}
	}()

	className, _ := syscall.UTF16PtrFromString("SwitchableEngineWrapper")
	windowName, _ := syscall.UTF16PtrFromString("Polymorphic UI Engine Matrix (Modular)")

	wc := win32.WNDCLASS{
		Style:         3,
		PfnWndProc:    syscall.NewCallback(wndProc),
		ClassName:     className,
	}
	win32.RegisterClass(&wc)

	hwnd, err := win32.CreateWindow(
		className, windowName,
		0x00CF0000, // WS_OVERLAPPEDWINDOW
		100, 100, int32(render.Width+16), int32(render.Height+39),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create window: %v", err))
	}

	hdc, err = win32.GetDC(hwnd)
	if err != nil {
		panic(fmt.Sprintf("Failed to get DC: %v", err))
	}

	// Factory handles build-tag selection automatically
	uiEngine, err = render.New(hdc)
	if err != nil {
		panic(fmt.Sprintf("Renderer Configuration Pipeline Refused Interface Matrix: %v", err))
	}

	if err := uiEngine.Setup(hdc); err != nil {
		panic(fmt.Sprintf("Renderer Setup Failed: %v", err))
	}

	activeState.StartTime = time.Now()
	activeState.CoreMask = (1 << uint(runtime.NumCPU())) - 1
	activeState.Particles = render.NewParticleSystem(100, image.Rect(0, 0, render.Width, render.Height))

	// Initialize UI Components
	BuildAllPages(activeState)

	win32.ShowWindow(hwnd, 5) // SW_SHOW

	// Drive animation loop
	lastFrame := time.Now()
	go func() {
		for {
			start := time.Now()
			time.Sleep(16 * time.Millisecond)
			now := time.Now()
			dt := now.Sub(lastFrame).Seconds()
			if dt > 0 {
				activeState.CurrentFPS = 1.0 / dt
				activeState.Particles.Update(dt)
				activeState.UpdateAnimations(float32(dt))
			}
			lastFrame = now
			activeState.FrameTime = time.Since(start)
			win32.InvalidateRect(hwnd, nil, false)
		}
	}()

	var msg [7]uintptr
	for {
		ret, _ := win32.GetMessage(&msg[0])
		if ret == 0 {
			break
		}
		win32.TranslateMessage(&msg[0])
		win32.DispatchMessage(&msg[0])
	}
}

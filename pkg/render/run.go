package render

import (
	"fmt"
	"image"
	"net/http"
	_ "net/http/pprof" // Profiling
	"runtime"
	"syscall"
	"time"

	"go_native_gpu_gui/internal/win32"
	"go_native_gpu_gui/pkg/render/backend"
	"go_native_gpu_gui/pkg/render/components"
	"go_native_gpu_gui/pkg/render/types"
)

type AppConfig struct {
	Title        string
	Width        int
	Height       int
	BuildPagesFn func(state *types.ApplicationState)
}

// Package-level runner variables
var (
	globalHdc        uintptr
	globalState      *types.ApplicationState
	globalEngine     types.UIRenderer
	globalBuildPages func(state *types.ApplicationState)
)

func Run(config AppConfig) {
	runtime.LockOSThread()

	// 1. Initialize dimensions
	if config.Width > 0 {
		Width = config.Width
		types.Width = config.Width
	}
	if config.Height > 0 {
		Height = config.Height
		types.Height = config.Height
	}

	// 2. Start Profiling Server
	go func() {
		fmt.Println("📊 Performance Profiler active at http://127.0.0.1:6060/debug/pprof/")
		if err := http.ListenAndServe("127.0.0.1:6060", nil); err != nil {
			fmt.Printf("Profiler server failed: %v\n", err)
		}
	}()

	// 3. Initialize application state
	globalState = &types.ApplicationState{
		StatusText:      "Engine Running Synchronized Component Tree",
		Volume:          75.0,
		GlassEnabled:    true,
		ArrowCursor:     win32.LoadCursor(0, 32512), // IDC_ARROW (32512)
		HandCursor:      win32.LoadCursor(0, 32649), // IDC_HAND (32649)
		IBeamCursor:     win32.LoadCursor(0, 32513), // IDC_IBEAM (32513)
		ScrollPositions: make(map[string]int),
		ScrollDragStart: make(map[string]int),
		ScrollStartY:    make(map[string]int),
		ScrollCurrent:   make(map[string]float64),
		TextInputValues: make(map[string]string),
		SliderValues:    make(map[string]float32),
	}
	globalState.CursorID = globalState.ArrowCursor
	globalBuildPages = config.BuildPagesFn

	// 4. Create Win32 window
	className, _ := syscall.UTF16PtrFromString("SwitchableEngineWrapper")
	windowTitle := "POEM Application Framework"
	if config.Title != "" {
		windowTitle = config.Title
	}
	windowName, _ := syscall.UTF16PtrFromString(windowTitle)

	wc := win32.WNDCLASS{
		Style:      3,
		PfnWndProc: syscall.NewCallback(libWndProc),
		ClassName:  className,
	}
	win32.RegisterClass(&wc)

	hwnd, err := win32.CreateWindow(
		className, windowName,
		0x00CF0000, // WS_OVERLAPPEDWINDOW
		100, 100, int32(Width+16), int32(Height+39),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create window: %v", err))
	}

	globalHdc, err = win32.GetDC(hwnd)
	if err != nil {
		panic(fmt.Sprintf("Failed to get DC: %v", err))
	}

	// Factory handles build-tag selection automatically (CPU vs GPU backend)
	globalEngine, err = backend.New(globalHdc)
	if err != nil {
		panic(fmt.Sprintf("Renderer Configuration Pipeline Refused Interface Matrix: %v", err))
	}

	if err := globalEngine.Setup(globalHdc); err != nil {
		panic(fmt.Sprintf("Renderer Setup Failed: %v", err))
	}

	globalState.StartTime = time.Now()
	globalState.CoreMask = (1 << uint(runtime.NumCPU())) - 1
	globalState.Particles = types.NewParticleSystem(100, image.Rect(0, 0, Width, Height))

	// Build the pages initially
	if globalBuildPages != nil {
		globalBuildPages(globalState)
	}

	win32.ShowWindow(hwnd, 5) // SW_SHOW

	// Drive animation loop
	lastFrame := time.Now()
	go func() {
		var telemetryTimer float64
		for {
			start := time.Now()
			time.Sleep(16 * time.Millisecond)
			now := time.Now()
			dt := now.Sub(lastFrame).Seconds()
			if dt > 0 {
				globalState.CurrentFPS = 1.0 / dt
				globalState.LastDt = dt
				globalState.Particles.Update(dt)
				globalState.UpdateAnimations(float32(dt))

				telemetryTimer += dt
				if telemetryTimer >= 0.1 { // Track heap and FPS every 100ms
					telemetryTimer = 0

					// Update FPS history (cap at 100)
					globalState.FPSHistory = append(globalState.FPSHistory, float32(globalState.CurrentFPS))
					if len(globalState.FPSHistory) > 100 {
						globalState.FPSHistory = globalState.FPSHistory[1:]
					}

					// Update Heap allocation history in MB (cap at 100)
					var m runtime.MemStats
					runtime.ReadMemStats(&m)
					heapMB := float32(m.Alloc) / 1024 / 1024
					globalState.HeapHistory = append(globalState.HeapHistory, heapMB)
					if len(globalState.HeapHistory) > 100 {
						globalState.HeapHistory = globalState.HeapHistory[1:]
					}
				}
			}
			lastFrame = now
			globalState.FrameTime = time.Since(start)
			triggerRepaint(hwnd)
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

func libWndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case 0x0020: // WM_SETCURSOR
		// HTCLIENT (client area) is 1. If cursor is inside client area, force pointer refresh
		if (lparam&0xFFFF) == 1 && globalState != nil && globalState.CursorID != 0 {
			win32.SetCursor(globalState.CursorID)
			return 1 // Return TRUE to indicate handled
		}
		break
	case 0x000F: // WM_PAINT
		if globalEngine != nil && globalState != nil {
			now := time.Now()
			var paintDt float64 = 0.0166 // default fallback
			if !globalState.LastPaintTime.IsZero() {
				paintDt = now.Sub(globalState.LastPaintTime).Seconds()
			}
			globalState.LastPaintTime = now
			if paintDt > 0.1 {
				paintDt = 0.1
			}
			globalState.RenderDt = paintDt

			if globalBuildPages != nil {
				globalBuildPages(globalState)
			}
			globalEngine.Paint(globalHdc, globalState)
			win32.ValidateRect(hwnd, nil)
		}
		return 0
	case 0x0005: // WM_SIZE
		w := int(lparam & 0xFFFF)
		h := int(lparam >> 16)
		if w > 0 && h > 0 {
			Width = w
			Height = h
			types.Width = w
			types.Height = h
			if globalEngine != nil {
				globalEngine.SetSize(w, h)
				if globalBuildPages != nil && globalState != nil {
					globalBuildPages(globalState)
				}
			}
		}
		return 0
	case 0x0201: // WM_LBUTTONDOWN
		if globalState == nil {
			return 0
		}
		globalState.ClickCount++
		pt := image.Point{globalState.MouseX, globalState.MouseY}
		newFocus := libFindHoveredComponent(pt)
		globalState.FocusedID = newFocus

		pt = image.Point{X: int(win32.GET_X_LPARAM(lparam)), Y: int(win32.GET_Y_LPARAM(lparam))}
		globalState.ActiveID = libFindHoveredComponent(pt)
		if globalState.ActiveID != "" {
			if comp := libFindComponent(globalState.ActiveID); comp != nil {
				if comp.OnMouseDown(pt, globalState) {
					if s, ok := comp.(*components.Slider); ok {
						globalState.Volume = s.Value
					}
					if globalBuildPages != nil {
						globalBuildPages(globalState)
					}
				}
				win32.SetCapture(hwnd)
			}
		}

		globalState.StatusText = fmt.Sprintf("Interaction Captured: %d Clicks Recorded | Focus: %s", globalState.ClickCount, newFocus)
		triggerRepaint(hwnd)
		return 0
	case 0x0202: // WM_LBUTTONUP
		if globalState == nil {
			return 0
		}
		pt := image.Point{X: int(win32.GET_X_LPARAM(lparam)), Y: int(win32.GET_Y_LPARAM(lparam))}
		if globalState.ActiveID != "" {
			if comp := libFindComponent(globalState.ActiveID); comp != nil {
				comp.OnMouseUp(pt, globalState)
				if globalBuildPages != nil {
					globalBuildPages(globalState)
				}
			}
			globalState.ActiveID = ""
		}
		win32.ReleaseCapture()
		triggerRepaint(hwnd)
		return 0
	case 0x0200: // WM_MOUSEMOVE
		if globalState == nil {
			return 0
		}
		globalState.MouseX = int(int16(lparam & 0xFFFF))
		globalState.MouseY = int(int16(lparam >> 16))
		pt := image.Point{globalState.MouseX, globalState.MouseY}

		newHover := ""
		if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
			// Layout sync pass
			for _, comp := range comps {
				comp.SetBounds(comp.Bounds())
			}
			for i := len(comps) - 1; i >= 0; i-- {
				if id := comps[i].HitTest(pt); id != "" {
					newHover = id
					break
				}
			}
		}
		globalState.HoveredID = newHover

		if globalState.ActiveID != "" {
			if comp := libFindComponent(globalState.ActiveID); comp != nil {
				if comp.OnMouseMove(pt, globalState) {
					if s, ok := comp.(*components.Slider); ok {
						globalState.Volume = s.Value
					}
					if globalBuildPages != nil {
						globalBuildPages(globalState)
					}
					triggerRepaint(hwnd)
				}
			}
		} else if newHover != "" {
			if comp := libFindComponent(newHover); comp != nil {
				comp.OnMouseMove(pt, globalState)
			}
		}
		return 0
	case 0x020A: // WM_MOUSEWHEEL
		if globalState == nil {
			return 0
		}
		delta := int(int16(wparam >> 16))
		pt := image.Point{globalState.MouseX, globalState.MouseY}

		if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
			for _, comp := range comps {
				comp.Walk(func(c types.Component) {
					if sc, ok := c.(types.ScrollableComponent); ok {
						if pt.In(c.Bounds()) {
							if sc.OnMouseWheel(pt, delta, globalState) {
								if globalBuildPages != nil {
									globalBuildPages(globalState)
								}
								triggerRepaint(hwnd)
							}
						}
					}
				})
			}
		}
		return 0
	case 0x0100: // WM_KEYDOWN
		if globalState == nil {
			return 0
		}
		const VK_TAB = 0x09
		const VK_SHIFT = 0x10
		const VK_ESCAPE = 0x1B
		const VK_CONTROL = 0x11

		ctrlPressed := (win32.GetKeyState(VK_CONTROL) < 0) || (win32.GetKeyState(0xA2) < 0) || (win32.GetKeyState(0xA3) < 0)
		fmt.Printf("[ENGINE KEYLOG] WM_KEYDOWN: wparam=%d (0x%02X) | CtrlPressed=%t | FocusedID=%q\n",
			wparam, wparam, ctrlPressed, globalState.FocusedID)

		// 1. Escape clears active focus
		if wparam == VK_ESCAPE {
			globalState.FocusedID = ""
			triggerRepaint(hwnd)
			return 0
		}

		// 2. Tab cycling focus
		if wparam == VK_TAB {
			reverse := win32.GetKeyState(VK_SHIFT) < 0
			globalState.CycleFocus(reverse)
			triggerRepaint(hwnd)
			return 0
		}

		// 3. Global hotkeys (e.g. Ctrl+S)
		if ctrlPressed {
			shortcut := ""
			if wparam == 'S' || wparam == 's' {
				shortcut = "Ctrl+S"
			}
			if shortcut != "" && globalState.Hotkeys != nil {
				if handler, ok := globalState.Hotkeys[shortcut]; ok {
					handler(globalState)
					if globalBuildPages != nil {
						globalBuildPages(globalState)
					}
					triggerRepaint(hwnd)
					return 0
				}
			}
		}

		// 4. Keyboard propagation to active component
		if globalState.FocusedID != "" {
			if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
				for _, c := range comps {
					if c.OnKey(uint32(wparam), 0, globalState) {
						if comp := libFindComponent(globalState.FocusedID); comp != nil {
							if s, ok := comp.(*components.Slider); ok && s.CompID == "sld_vol" {
								globalState.Volume = s.Value
							}
						}
						if globalBuildPages != nil {
							globalBuildPages(globalState)
						}
						triggerRepaint(hwnd)
						break
					}
				}
			}
		}
		return 0
	case 0x0102: // WM_CHAR
		if globalState == nil {
			return 0
		}
		if globalState.FocusedID != "" {
			if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
				for _, c := range comps {
					if c.OnKey(0, rune(wparam), globalState) {
						if globalBuildPages != nil {
							globalBuildPages(globalState)
						}
						triggerRepaint(hwnd)
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

func libFindComponent(id string) types.Component {
	if id == "" || globalState == nil {
		return nil
	}
	var found types.Component
	if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
		for _, c := range comps {
			c.Walk(func(comp types.Component) {
				if comp.ID() == id {
					found = comp
				}
			})
		}
	}
	return found
}

func libFindHoveredComponent(pt image.Point) string {
	if globalState == nil {
		return ""
	}
	if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
		// Layout sync pass
		for _, comp := range comps {
			comp.SetBounds(comp.Bounds())
		}
		for i := len(comps) - 1; i >= 0; i-- {
			if id := comps[i].HitTest(pt); id != "" {
				return id
			}
		}
	}
	return ""
}

func triggerRepaint(hwnd uintptr) {
	win32.InvalidateRect(hwnd, nil, false)
	win32.PostMessage(hwnd, 0, 0, 0) // WM_NULL dummy message to wake up GetMessage loop asynchronously
}

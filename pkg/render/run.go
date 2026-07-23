package render

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/mulavdm/poem/pkg/design"
	"github.com/mulavdm/poem/pkg/render/components"
	"github.com/mulavdm/poem/pkg/render/events"
	"github.com/mulavdm/poem/pkg/render/platform"
	"github.com/mulavdm/poem/pkg/render/protocol"
	"github.com/mulavdm/poem/pkg/render/semantics"
	"github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/theme"
	"github.com/mulavdm/poem/pkg/render/types"
)

type AccessibilityConfig struct {
	Enabled             bool
	TextScale           float32
	ReducedMotion       bool
	FollowSystemTheme   bool
	FollowReducedMotion bool
	SystemThemeResolver func(theme.Mode) theme.Theme
}

type EffectsConfig struct {
	Glass, Particles, Audio bool
}

func pannableTargetsAt(root types.Component, point image.Point) []types.PannableComponent {
	if root == nil {
		return nil
	}
	hitID := root.HitTest(point)
	var targets []types.PannableComponent
	root.Walk(func(component types.Component) {
		// A canvas can geometrically sit behind an overlay. Only let it claim
		// the pan when it owns the topmost hit target; otherwise event routing
		// can fall back to an overlaid ScrollView.
		if target, ok := component.(types.PannableComponent); ok && point.In(component.Bounds()) && component.ID() == hitID {
			targets = append(targets, target)
		}
	})
	return targets
}

type TypographyConfig struct {
	PrimaryFontPath   string
	FallbackFontPaths []string
}

type AppConfig struct {
	Title                 string
	Width                 int
	Height                int
	BuildPagesFn          func(state *types.ApplicationState)
	FontPath              string
	FontSize              float64
	Typography            TypographyConfig
	Automation            *AutomationConfig
	Theme                 *theme.Manager
	Services              platform.Services
	Locale                string
	Accessibility         AccessibilityConfig
	Effects               EffectsConfig
	ShowDiagnostics       bool
	LegacyComponentStyles bool
	// Design resolves platform, density, accessibility, and semantic tokens.
	Design *design.System
	// Platform identifies the presentation convention without exposing a toolkit.
	Platform design.Platform
	// Input describes the host's available input paths.
	Input design.InputCapabilities
	// OnStop releases application-owned resources when the hosted engine exits.
	OnStop func()
}

// Package-level orchestrator variables
var (
	globalState             *types.ApplicationState
	globalBuildPages        func(state *types.ApplicationState)
	pipeHandle              uintptr
	pipeWriteMutex          sync.Mutex
	stateMutex              sync.Mutex // Protects globalState, component layouts, and painter from concurrent races
	globalFontPath          string
	globalFontSize          float64
	globalFontFallbackPaths []string
	globalFontAtlas         *fontAtlasManager
	globalPainter           *ProtocolPainter
	globalRenderConn        io.Writer
	globalLastFrame         protocol.RenderFrame
	globalSemanticRevision  uint64
	globalRepaintRequested  atomic.Bool
	nativeDebugReqMu        sync.Mutex
	nativeDebugMu           sync.Mutex
	nativeDebugRespCh       chan protocol.NativeDebugResponse
	nativeDialogReqMu       sync.Mutex
	nativeDialogMu          sync.Mutex
	nativeDialogRespCh      chan protocol.NativeDialogResponse
)

func resolveSystemTheme(config AccessibilityConfig, mode theme.Mode) theme.Theme {
	if config.SystemThemeResolver != nil {
		return config.SystemThemeResolver(mode)
	}
	switch mode {
	case theme.ModeLight:
		return theme.ModernLight()
	case theme.ModeHighContrast:
		return theme.HighContrast()
	default:
		return theme.ModernDark()
	}
}

func applySystemPreferences(state *types.ApplicationState, manager *theme.Manager, config AccessibilityConfig, snapshot platform.PreferenceSnapshot) bool {
	changed := false
	if config.FollowSystemTheme {
		next := resolveSystemTheme(config, snapshot.ThemeMode)
		current, _ := manager.Current()
		if current != next {
			if manager.Set(next) == nil {
				changed = true
			}
		}
	}
	if config.FollowReducedMotion {
		next := config.ReducedMotion || snapshot.ReducedMotion
		if state.ReducedMotion != next {
			state.ReducedMotion = next
			changed = true
		}
	}
	return changed
}

func pollSystemPreferences(service platform.SystemPreferences, interval time.Duration) (<-chan platform.PreferenceSnapshot, chan struct{}) {
	updates := make(chan platform.PreferenceSnapshot, 1)
	stop := make(chan struct{})
	if interval <= 0 {
		interval = time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var previous platform.PreferenceSnapshot
		havePrevious := false
		for {
			ctx, cancel := context.WithTimeout(context.Background(), min(interval, 250*time.Millisecond))
			snapshot, err := service.Current(ctx)
			cancel()
			if err == nil && (!havePrevious || snapshot != previous) {
				select {
				case updates <- snapshot:
				default:
					select {
					case <-updates:
					default:
					}
					updates <- snapshot
				}
				previous, havePrevious = snapshot, true
			}
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
	return updates, stop
}

// RunHosted drives an already-connected presenter over a caller-owned
// transport instead of spawning a sidecar process. It exists for embedding
// hosts where the presenter lives in the same process as the engine — the
// Android engine, where the C++ presenter and this Go orchestration share one
// APK and exchange the same POEM byte protocol over an in-process pipe pair.
// The caller owns both transports and closes them to stop the engine.
func RunHosted(config AppConfig, toPresenter io.ReadWriteCloser, fromPresenter io.ReadWriteCloser) {
	if config.OnStop != nil {
		defer config.OnStop()
	}
	globalHostedMode = true
	config.Services = withDefaultPlatformServices(config.Services)
	manager, preferenceUpdates, stopPreferencePolling := prepareEngineState(config)
	if stopPreferencePolling != nil {
		defer close(stopPreferencePolling)
	}
	runEngine(config, manager, preferenceUpdates, toPresenter, fromPresenter)
}

// prepareEngineState resolves theme/typography/accessibility configuration
// and initializes the global application state shared by every presenter
// transport. It returns the theme manager plus the system-preference polling
// channel (and its stop channel, nil when polling is disabled).
func prepareEngineState(config AppConfig) (*theme.Manager, <-chan platform.PreferenceSnapshot, chan struct{}) {
	if config.Platform == design.PlatformUnknown {
		switch runtime.GOOS {
		case "windows":
			config.Platform = design.PlatformWindows
		case "android":
			config.Platform = design.PlatformAndroid
		}
	}
	if config.Design == nil {
		config.Design = design.DefaultSystem()
	}
	designEnvironment := design.Environment{Platform: config.Platform, Width: config.Width, Height: config.Height, Input: config.Input,
		ReducedMotion: config.Accessibility.ReducedMotion, TextScale: config.Accessibility.TextScale}
	resolvedDesign := config.Design.Resolve(designEnvironment)
	manager := config.Theme
	if manager == nil {
		manager = theme.NewManager(resolvedDesign.Theme)
	}
	initialReducedMotion := config.Accessibility.ReducedMotion
	if (config.Accessibility.FollowSystemTheme || config.Accessibility.FollowReducedMotion) && config.Services.Preferences != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		if snapshot, err := config.Services.Preferences.Current(ctx); err == nil {
			if config.Accessibility.FollowSystemTheme {
				_ = manager.Set(resolveSystemTheme(config.Accessibility, snapshot.ThemeMode))
			}
			if config.Accessibility.FollowReducedMotion {
				initialReducedMotion = initialReducedMotion || snapshot.ReducedMotion
			}
		}
		cancel()
	}
	activeTheme, _ := manager.Current()

	// 1. Initialize dimensions
	if config.Width > 0 {
		Width = config.Width
		types.Width = config.Width
	}
	if config.Height > 0 {
		Height = config.Height
		types.Height = config.Height
	}
	globalFontPath = config.FontPath
	if config.Typography.PrimaryFontPath != "" {
		globalFontPath = config.Typography.PrimaryFontPath
	}
	if globalFontPath == "" {
		globalFontPath = resolveDefaultFontPath()
	}
	globalFontSize = config.FontSize
	if globalFontSize <= 0 {
		globalFontSize = float64(activeTheme.Typography.Body.Size)
	}
	globalFontFallbackPaths = append([]string(nil), config.Typography.FallbackFontPaths...)
	if len(globalFontFallbackPaths) == 0 {
		globalFontFallbackPaths = resolveDefaultFallbackFontPaths()
	}
	locale := config.Locale
	if locale == "" {
		locale = "en-US"
	}
	textScale := config.Accessibility.TextScale
	if textScale <= 0 {
		textScale = 1
	}
	globalFontSize *= float64(textScale)

	// 2. Initialize application state with portable design/platform services.
	globalState = &types.ApplicationState{
		StatusText:            "Orchestrator Matrix Running headlessly",
		Volume:                75.0,
		GlassEnabled:          config.Effects.Glass,
		ArrowCursor:           1, // Abstract ID for Arrow
		HandCursor:            2, // Abstract ID for Hand
		IBeamCursor:           3, // Abstract ID for IBeam
		ScrollPositions:       make(map[string]int),
		ScrollDragStart:       make(map[string]int),
		ScrollStartY:          make(map[string]int),
		ScrollCurrent:         make(map[string]float64),
		TextInputValues:       make(map[string]string),
		SliderValues:          make(map[string]float32),
		AudioEnabled:          config.Effects.Audio,
		KeysPressed:           make(map[uint32]bool),
		ParticlesEnabled:      config.Effects.Particles,
		WindowWidth:           Width,
		WindowHeight:          Height,
		PhysicalWindowWidth:   Width,
		PhysicalWindowHeight:  Height,
		ApplicationName:       config.Title,
		ThemeManager:          manager,
		Services:              config.Services,
		Locale:                locale,
		DPIScale:              1,
		TextScale:             textScale,
		ReducedMotion:         initialReducedMotion,
		ShowDiagnostics:       config.ShowDiagnostics,
		LegacyComponentStyles: config.LegacyComponentStyles,
		DesignSystem:          config.Design,
		DesignEnvironment:     resolvedDesign.Environment,
		Overlays:              types.NewOverlayManager(),
		TransientState:        state.NewStore(),
	}
	globalState.CursorID = globalState.ArrowCursor
	globalBuildPages = config.BuildPagesFn

	globalState.StartTime = time.Now()
	globalState.CoreMask = (1 << uint(runtime.NumCPU())) - 1
	globalState.Particles = types.NewParticleSystem(100, image.Rect(0, 0, Width, Height))

	var preferenceUpdates <-chan platform.PreferenceSnapshot
	var stopPreferencePolling chan struct{}
	if (config.Accessibility.FollowSystemTheme || config.Accessibility.FollowReducedMotion) && config.Services.Preferences != nil {
		preferenceUpdates, stopPreferencePolling = pollSystemPreferences(config.Services.Preferences, time.Second)
	}

	if globalBuildPages != nil {
		globalBuildPages(globalState)
	}
	return manager, preferenceUpdates, stopPreferencePolling
}

// runEngine drives an already-connected presenter: it transmits the font
// atlas bootstrap and every subsequent frame over toPresenter, and consumes
// the presenter's event stream from fromPresenter until it closes. It is
// platform-neutral — Run hands it named pipes bound to the Windows sidecar,
// RunHosted hands it whatever in-process transport the embedding host owns.
func runEngine(config AppConfig, manager *theme.Manager, preferenceUpdates <-chan platform.PreferenceSnapshot, toPresenter io.ReadWriteCloser, fromPresenter io.ReadWriteCloser) {
	engineDone := make(chan struct{})
	defer close(engineDone)
	pipeConnGoToSidecar := toPresenter
	pipeConnSidecarToGo := fromPresenter
	globalRenderConn = pipeConnGoToSidecar

	globalState.PlaySoundFn = func(soundType int8) {
		sendSoundEvent(pipeConnGoToSidecar, protocol.SoundType(soundType))
	}

	// 6. Generate and transmit dynamically-rasterized Font Atlas
	globalFontAtlas = newFontAtlasManager(globalFontPath, globalFontFallbackPaths, globalFontSize)
	if globalState.Services.TextShaper == nil {
		globalState.Services.TextShaper = globalFontAtlas
	}
	atlasInit, measuredLSB, err := globalFontAtlas.Build(Width, Height)
	if err != nil {
		panic(fmt.Sprintf("Failed to build font atlas: %v", err))
	}

	// Dynamically resolve monospaced character width and LSB from loaded font metrics
	detectedWidth := 7
	for _, char := range atlasInit.Chars {
		if char.R == 'A' {
			detectedWidth = int(char.Advance) // Advance = full cell width used for layout
			break
		}
	}
	globalState.FontCharWidth = detectedWidth
	globalState.FontCharBearingX = measuredLSB
	fmt.Printf("❖ Font Engine: CharWidth=%dpx, BearingX=%dpx\n", detectedWidth, measuredLSB)

	initBytes, err := protocol.EncodeInitEngine(atlasInit)
	if err != nil {
		panic(fmt.Sprintf("Failed to serialize InitEngine bootstrap package: %v", err))
	}

	if err := writeMessage(pipeConnGoToSidecar, initBytes); err != nil {
		panic(fmt.Sprintf("Failed to transmit InitEngine bootstrap package: %v", err))
	}
	fmt.Println("🔤 Font Atlas & Metrics bootstrap context loaded successfully to presenter.")

	// 7. Initialize protocol-backed painter
	painter := NewProtocolPainter()
	painter.SetTextObserver(globalFontAtlas.Observe)
	globalPainter = painter

	if config.Automation != nil && config.Automation.Enabled {
		startAutomationServer(resolveAutomationConfig(config.Automation))
	}

	// 8. Start Background Physics & Repaint loop (headful updates mapped back to presenter)
	lastFrame := time.Now()
	go func() {
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		var telemetryTimer float64
		for {
			select {
			case <-engineDone:
				return
			case <-ticker.C:
			}
			start := time.Now()

			stateMutex.Lock()
			if preferenceUpdates != nil {
				select {
				case snapshot := <-preferenceUpdates:
					if applySystemPreferences(globalState, manager, config.Accessibility, snapshot) {
						globalState.NeedsRepaint = true
					}
				default:
				}
			}
			now := time.Now()
			dt := now.Sub(lastFrame).Seconds()
			if dt > 0 {
				globalState.CurrentFPS = 1.0 / dt
				globalState.LastDt = dt
				motionAllowed := !globalState.RenderContext().ReducedMotion
				if globalState.Particles != nil && motionAllowed {
					globalState.Particles.Update(dt)
				}
				globalState.UpdateAnimations(float32(dt))

				telemetryTimer += dt
				if telemetryTimer >= 0.1 {
					telemetryTimer = 0

					// Update FPS history (cap 100)
					globalState.FPSHistory = append(globalState.FPSHistory, float32(globalState.CurrentFPS))
					if len(globalState.FPSHistory) > 100 {
						globalState.FPSHistory = globalState.FPSHistory[1:]
					}

					// Update Heap history (cap 100)
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
			if globalRepaintRequested.Swap(false) {
				globalState.NeedsRepaint = true
			}

			motionAllowed := !globalState.RenderContext().ReducedMotion
			shouldRepaint := globalState.NeedsRepaint || (motionAllowed && globalState.ParticlesEnabled) || globalState.IsTransitioning
			// The presenter pipe is a synchronous io.Pipe: a Write blocks until
			// the host drains it. The host drains it on the same thread that
			// runs window operations for a NativeDebugRequest (foreground
			// activation, restore), so while /prepare-window is in flight that
			// thread is not reading. Writing the frame here, under stateMutex,
			// would then block with the lock held and wedge every stateMutex
			// reader — /components and /state stop answering until the window
			// op returns. So the frame is serialized into a buffer under the
			// lock and flushed after it is released; the write can still block,
			// but no longer behind the lock.
			var outbound bytes.Buffer
			if shouldRepaint {
				atlasChanged := triggerRepaintFrame(&outbound, painter)
				globalState.NeedsRepaint = atlasChanged
			}
			updateImeVisibility(&outbound)
			if imeShown {
				ensureFocusedVisible()
			}
			stateMutex.Unlock()

			if outbound.Len() > 0 {
				flushBufferedFrames(pipeConnGoToSidecar, outbound.Bytes())
			}
		}
	}()

	// 9. Main event receiver loop (blocks on incoming event batches from the presenter)
	stopRequested := false
	for !stopRequested {
		payload, err := readMessage(pipeConnSidecarToGo)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Native presentation engine disconnected gracefully.")
				break
			}
			fmt.Printf("⚠️ IPC Read Loop Error: %v\n", err)
			break
		}

		msgType, _, err := protocol.DecodeEnvelope(payload)
		if err != nil {
			fmt.Printf("IPC decode error: %v\n", err)
			continue
		}
		switch msgType {
		case protocol.MessageEventBatch:
			batch, err := protocol.DecodeEventBatch(payload)
			if err != nil {
				fmt.Printf("IPC decode error: %v\n", err)
				continue
			}
			stateMutex.Lock()
			stopRequested = processEventBatch(batch, pipeConnGoToSidecar, painter)
			stateMutex.Unlock()
		case protocol.MessageNativeDebugResponse:
			resp, err := protocol.DecodeNativeDebugResponse(payload)
			if err != nil {
				fmt.Printf("IPC native debug decode error: %v\n", err)
				continue
			}
			nativeDebugMu.Lock()
			ch := nativeDebugRespCh
			nativeDebugMu.Unlock()
			if ch != nil {
				select {
				case ch <- resp:
				default:
				}
			}
		case protocol.MessageNativeDialogResponse:
			resp, err := protocol.DecodeNativeDialogResponse(payload)
			if err != nil {
				fmt.Printf("IPC native dialog decode error: %v\n", err)
				continue
			}
			nativeDialogMu.Lock()
			ch := nativeDialogRespCh
			nativeDialogMu.Unlock()
			if ch != nil {
				select {
				case ch <- resp:
				default:
				}
			}
		default:
			fmt.Printf("IPC unexpected message type: %d\n", msgType)
		}
	}
}

type DirectoryDialogOptions struct {
	Title      string
	InitialDir string
}

func OpenDirectoryDialog(opts DirectoryDialogOptions) (string, bool, error) {
	resp, err := requestNativeDialog(protocol.NativeDialogRequest{
		Kind:       "directory",
		Title:      opts.Title,
		InitialDir: opts.InitialDir,
	})
	if err != nil {
		return "", false, err
	}
	return resp.Path, resp.Canceled, nil
}

func requestNativeDebug(captureFrame bool, capturePresentedFrame bool, captureDesktopFrame bool) (protocol.NativeDebugResponse, error) {
	return requestNativeDebugWithOptions(protocol.NativeDebugRequest{
		CaptureFrame:          captureFrame,
		CapturePresentedFrame: capturePresentedFrame,
		CaptureDesktopFrame:   captureDesktopFrame,
	})
}

func requestNativeDebugWithOptions(req protocol.NativeDebugRequest) (protocol.NativeDebugResponse, error) {
	if globalRenderConn == nil {
		return protocol.NativeDebugResponse{}, fmt.Errorf("render connection not initialized")
	}

	nativeDebugReqMu.Lock()
	defer nativeDebugReqMu.Unlock()

	nativeDebugMu.Lock()
	respCh := make(chan protocol.NativeDebugResponse, 1)
	nativeDebugRespCh = respCh
	nativeDebugMu.Unlock()

	payload, err := protocol.EncodeNativeDebugRequest(req)
	if err != nil {
		nativeDebugMu.Lock()
		nativeDebugRespCh = nil
		nativeDebugMu.Unlock()
		return protocol.NativeDebugResponse{}, err
	}
	if err := writeMessage(globalRenderConn, payload); err != nil {
		nativeDebugMu.Lock()
		nativeDebugRespCh = nil
		nativeDebugMu.Unlock()
		return protocol.NativeDebugResponse{}, err
	}

	clearResponseCh := func() {
		nativeDebugMu.Lock()
		nativeDebugRespCh = nil
		nativeDebugMu.Unlock()
	}

	select {
	case resp := <-respCh:
		clearResponseCh()
		if resp.Error != "" {
			return protocol.NativeDebugResponse{}, fmt.Errorf("%s", resp.Error)
		}
		return resp, nil
	case <-time.After(5 * time.Second):
		clearResponseCh()
		return protocol.NativeDebugResponse{}, fmt.Errorf("native debug request timed out")
	}
}

func requestNativeDialog(req protocol.NativeDialogRequest) (protocol.NativeDialogResponse, error) {
	if globalRenderConn == nil {
		return protocol.NativeDialogResponse{}, fmt.Errorf("render connection not initialized")
	}

	nativeDialogReqMu.Lock()
	defer nativeDialogReqMu.Unlock()

	nativeDialogMu.Lock()
	respCh := make(chan protocol.NativeDialogResponse, 1)
	nativeDialogRespCh = respCh
	nativeDialogMu.Unlock()

	payload, err := protocol.EncodeNativeDialogRequest(req)
	if err != nil {
		nativeDialogMu.Lock()
		nativeDialogRespCh = nil
		nativeDialogMu.Unlock()
		return protocol.NativeDialogResponse{}, err
	}
	if err := writeMessage(globalRenderConn, payload); err != nil {
		nativeDialogMu.Lock()
		nativeDialogRespCh = nil
		nativeDialogMu.Unlock()
		return protocol.NativeDialogResponse{}, err
	}

	clearResponseCh := func() {
		nativeDialogMu.Lock()
		nativeDialogRespCh = nil
		nativeDialogMu.Unlock()
	}

	select {
	case resp := <-respCh:
		clearResponseCh()
		if resp.Error != "" {
			return protocol.NativeDialogResponse{}, fmt.Errorf("%s", resp.Error)
		}
		return resp, nil
	case <-time.After(2 * time.Minute):
		clearResponseCh()
		return protocol.NativeDialogResponse{}, fmt.Errorf("native dialog request timed out")
	}
}

// Write helper
func writeMessage(conn io.Writer, payload []byte) error {
	pipeWriteMutex.Lock()
	defer pipeWriteMutex.Unlock()

	length := uint32(len(payload))
	buf := make([]byte, 4+length)
	binary.LittleEndian.PutUint32(buf[0:4], length)
	copy(buf[4:], payload)
	_, err := conn.Write(buf)
	return err
}

// flushBufferedFrames writes a run of already-framed messages to the presenter
// pipe in one operation. The bytes were produced by writeMessage into a buffer
// (each already length-prefixed), so they are handed to the pipe verbatim.
//
// The single Write is held under pipeWriteMutex so a concurrent native-debug
// response cannot interleave itself between the frames — the reader decodes one
// length-prefixed message at a time, and a spliced write would corrupt the
// stream. This is the only place the repaint loop touches the real pipe, and it
// runs after stateMutex is released, so a blocked presenter no longer stalls
// state readers such as /components.
func flushBufferedFrames(conn io.Writer, framed []byte) {
	pipeWriteMutex.Lock()
	defer pipeWriteMutex.Unlock()
	_, _ = conn.Write(framed)
}

// SubmitMapScene sends one retained vector-scene generation to the active
// hosted presenter. It is safe to call from a bounded cartography worker and
// shares frame transport serialization with ordinary UI rendering.
func SubmitMapScene(scene protocol.MapSceneDelta) error {
	if globalRenderConn == nil {
		return fmt.Errorf("render: no active presenter")
	}
	payload, err := protocol.EncodeMapSceneDelta(scene)
	if err != nil {
		return err
	}
	return writeMessage(globalRenderConn, payload)
}

// Read helper
func readMessage(conn io.Reader) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(lenBuf[:])
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

type fontCharInfo struct {
	R       int32
	U1      float32
	V1      float32
	U2      float32
	V2      float32
	Width   int32
	Height  int32
	Advance int32
}

func loadTTFFace(path string, size float64) (font.Face, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := sfnt.Parse(bytes)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	return face, err
}

// Generate the pixel matrix and mapping values for custom fonts or fallback
func buildFontAtlasPixels(fontPath string, fontSize float64) ([]byte, []fontCharInfo, int, int, int) {
	if fontSize <= 0 {
		fontSize = 17
	}
	atlasW := 512
	atlasH := 512

	var face font.Face
	if fontPath != "" {
		if loadedFace, err := loadTTFFace(fontPath, fontSize); err == nil {
			face = loadedFace
			fmt.Printf("❖ Font Engine: Successfully loaded custom font from %s\n", fontPath)
		} else {
			fmt.Printf("❖ Font Engine WARNING: Failed to load custom font %s: %v. Falling back to basicfont.\n", fontPath, err)
		}
	}

	if face == nil {
		face = basicfont.Face7x13
		atlasW = 128
		atlasH = 128
	}

	rgba := image.NewRGBA(image.Rect(0, 0, atlasW, atlasH))
	d := &font.Drawer{
		Dst:  rgba,
		Src:  image.White,
		Face: face,
	}

	metrics := face.Metrics()
	ascent := int((metrics.Ascent + 63) >> 6)
	descent := int((metrics.Descent + 63) >> 6)
	height := int((metrics.Height + 63) >> 6)
	if ascent <= 0 {
		ascent = 11
	}
	if descent <= 0 {
		descent = 3
	}
	if height <= 0 {
		height = ascent + descent + 2
	}

	// Starting Y offsets
	yStart := ascent + 2
	lineInc := height + 4
	yOffsetV1 := ascent + 1
	yOffsetV2 := descent + 2
	if fontPath != "" && face != basicfont.Face7x13 {
		yStart = ascent + 3
		lineInc = height + 6
		yOffsetV1 = ascent + 2
		yOffsetV2 = descent + 3
	}

	x, y := 0, yStart
	var chars []fontCharInfo
	measuredLSB := 0
	lsbMeasured := false

	for r := rune(32); r < 127; r++ {
		advance, ok := face.GlyphAdvance(r)
		if !ok {
			continue
		}

		advancePixels := int(advance >> 6)
		if advancePixels <= 0 {
			advancePixels = 8
		}

		if x+advancePixels > atlasW {
			x = 0
			y += lineInc
		}

		// Measure LSB once from a representative character ('A')
		if !lsbMeasured && r == 'A' {
			bounds, _, ok2 := face.GlyphBounds(r)
			if ok2 {
				lsb := int(bounds.Min.X >> 6)
				if lsb > 0 && lsb < advancePixels {
					measuredLSB = lsb
				}
			}
			lsbMeasured = true
		}

		d.Dot = fixed.P(x, y)
		d.DrawString(string(r))

		chars = append(chars, fontCharInfo{
			R:       int32(r),
			U1:      float32(x) / float32(atlasW),
			V1:      float32(y-yOffsetV1) / float32(atlasH),
			U2:      float32(x+advancePixels) / float32(atlasW),
			V2:      float32(y+yOffsetV2) / float32(atlasH),
			Width:   int32(advancePixels),
			Height:  int32(yOffsetV1 + yOffsetV2),
			Advance: int32(advancePixels),
		})

		x += advancePixels
	}

	fmt.Printf("❖ Font Engine: Measured LSB = %d px for active font.\n", measuredLSB)
	return rgba.Pix, chars, atlasW, atlasH, measuredLSB
}

func buildHiDPIFontAtlasPixels(fontPath string, fontSize float64) ([]byte, []fontCharInfo, int, int, int) {
	if fontSize <= 0 {
		fontSize = 17
	}

	const oversample = 2
	if fontPath == "" {
		return buildFontAtlasPixels(fontPath, fontSize)
	}

	face, err := loadTTFFace(fontPath, fontSize*oversample)
	if err != nil {
		fmt.Printf("Font Engine WARNING: Failed to load HiDPI custom font %s: %v. Falling back.\n", fontPath, err)
		return buildFontAtlasPixels(fontPath, fontSize)
	}

	atlasW := 1024
	atlasH := 1024
	rgba := image.NewRGBA(image.Rect(0, 0, atlasW, atlasH))
	d := &font.Drawer{
		Dst:  rgba,
		Src:  image.White,
		Face: face,
	}

	scaleDown := func(px int) int {
		return (px + oversample/2) / oversample
	}

	metrics := face.Metrics()
	ascentPx := int((metrics.Ascent + 63) >> 6)
	descentPx := int((metrics.Descent + 63) >> 6)
	heightPx := int((metrics.Height + 63) >> 6)
	if ascentPx <= 0 {
		ascentPx = 22
	}
	if descentPx <= 0 {
		descentPx = 6
	}
	if heightPx <= 0 {
		heightPx = ascentPx + descentPx + 4
	}

	ascent := scaleDown(ascentPx)
	descent := scaleDown(descentPx)
	height := scaleDown(heightPx)
	yStartPx := ascentPx + 6
	lineIncPx := heightPx + 12
	topInsetPx := (ascent + 2) * oversample
	bottomInsetPx := (descent + 3) * oversample

	x, y := 0, yStartPx
	var chars []fontCharInfo
	measuredLSB := 0
	lsbMeasured := false

	for r := rune(32); r < 127; r++ {
		advance, ok := face.GlyphAdvance(r)
		if !ok {
			continue
		}

		advancePx := int(advance >> 6)
		if advancePx <= 0 {
			advancePx = 8 * oversample
		}
		logicalAdvance := scaleDown(advancePx)
		if logicalAdvance <= 0 {
			logicalAdvance = 8
		}

		if x+advancePx > atlasW {
			x = 0
			y += lineIncPx
		}

		if !lsbMeasured && r == 'A' {
			bounds, _, ok2 := face.GlyphBounds(r)
			if ok2 {
				lsb := scaleDown(int(bounds.Min.X >> 6))
				if lsb > 0 && lsb < logicalAdvance {
					measuredLSB = lsb
				}
			}
			lsbMeasured = true
		}

		d.Dot = fixed.P(x, y)
		d.DrawString(string(r))

		topPx := y - topInsetPx
		bottomPx := y + bottomInsetPx
		if topPx < 0 {
			topPx = 0
		}
		if bottomPx > atlasH {
			bottomPx = atlasH
		}

		chars = append(chars, fontCharInfo{
			R:       int32(r),
			U1:      float32(x) / float32(atlasW),
			V1:      float32(topPx) / float32(atlasH),
			U2:      float32(x+advancePx) / float32(atlasW),
			V2:      float32(bottomPx) / float32(atlasH),
			Width:   int32(logicalAdvance),
			Height:  int32(height + 5),
			Advance: int32(logicalAdvance),
		})

		x += advancePx
	}

	fmt.Printf("Font Engine: Measured HiDPI LSB = %d px for active font.\n", measuredLSB)
	return rgba.Pix, chars, atlasW, atlasH, measuredLSB
}

func serializeInitEngine(width, height int, pixels []byte, chars []fontCharInfo, atlasW, atlasH int) ([]byte, error) {
	outChars := make([]protocol.CharInfo, 0, len(chars))
	for _, char := range chars {
		outChars = append(outChars, protocol.CharInfo{
			R:       char.R,
			U1:      char.U1,
			V1:      char.V1,
			U2:      char.U2,
			V2:      char.V2,
			Width:   char.Width,
			Height:  char.Height,
			Advance: char.Advance,
		})
	}
	return protocol.EncodeInitEngine(protocol.InitEngine{
		Width:       int32(width),
		Height:      int32(height),
		AtlasWidth:  int32(atlasW),
		AtlasHeight: int32(atlasH),
		AtlasPixels: pixels,
		Chars:       outChars,
	})
}

func sendSoundEvent(conn io.Writer, soundType protocol.SoundType) {
	payload, err := protocol.EncodePlaySound(protocol.PlaySound{Type: soundType})
	if err != nil {
		fmt.Printf("failed to encode sound event: %v\n", err)
		return
	}
	_ = writeMessage(conn, payload)
}

// Main paint event dispatcher
var lastPrintTime time.Time

func triggerRepaintFrame(conn io.Writer, painter *ProtocolPainter) (atlasChanged bool) {
	if globalState == nil {
		return false
	}
	frameStart := time.Now()

	painter.Reset()

	// Rebuild dynamic page descriptors
	buildPagesStart := time.Now()
	if globalBuildPages != nil {
		globalBuildPages(globalState)
	}
	buildPagesDuration := time.Since(buildPagesStart)

	// Trigger layout logic & flat drawing tree population
	renderStart := time.Now()
	types.RenderPipeline(painter, globalState)
	renderDuration := time.Since(renderStart)
	if globalFontAtlas != nil {
		if atlas, changed, atlasErr := globalFontAtlas.TakeUpdate(Width, Height); atlasErr != nil {
			fmt.Printf("font atlas update failed: %v\n", atlasErr)
		} else if changed {
			if payload, encodeErr := protocol.EncodeFontAtlas(atlas); encodeErr == nil {
				_ = writeMessage(conn, payload)
				atlasChanged = true
			}
		}
	}

	semanticTree := types.BuildSemanticsTree(globalState)
	if semanticTree.Revision != globalSemanticRevision {
		if validationErr := semanticTree.Validate(); validationErr != nil {
			fmt.Printf("semantic tree validation failed: %v\n", validationErr)
			globalSemanticRevision = semanticTree.Revision
		} else if payload, encodeErr := protocol.EncodeSemanticTree(toProtocolSemanticTree(semanticTree)); encodeErr == nil {
			_ = writeMessage(conn, payload)
			globalSemanticRevision = semanticTree.Revision
		} else {
			fmt.Printf("semantic tree encoding failed: %v\n", encodeErr)
			globalSemanticRevision = semanticTree.Revision
		}
	}

	// Map cursor IDs to protocol cursor values.
	var cursorVal byte = 0 // Arrow
	switch globalState.CursorID {
	case globalState.HandCursor:
		cursorVal = 1 // Hand
	case globalState.IBeamCursor:
		cursorVal = 2 // IBeam
	}

	// Diagnostic print every 1 second
	if time.Since(lastPrintTime) > 1*time.Second {
		lastPrintTime = time.Now()
		fmt.Printf("🎨 [GO DIAGNOSTIC] triggerRepaintFrame: commands count = %d\n", len(painter.commands))
		if len(painter.commands) > 0 {
			fmt.Printf("   -> First 5 commands:\n")
			for idx := 0; idx < len(painter.commands) && idx < 5; idx++ {
				cmd := painter.commands[idx]
				fmt.Printf("      [%d] Type=%v, Rect=(%d,%d,%d,%d), Color=(%d,%d,%d,%d)\n",
					idx, cmd.Type, cmd.X1, cmd.Y1, cmd.X2, cmd.Y2, cmd.R, cmd.G, cmd.B, cmd.A)
			}
			fmt.Printf("   -> Last 15 commands:\n")
			startIdx := len(painter.commands) - 15
			if startIdx < 0 {
				startIdx = 0
			}
			for idx := startIdx; idx < len(painter.commands); idx++ {
				cmd := painter.commands[idx]
				fmt.Printf("      [%d] Type=%v, Rect=(%d,%d,%d,%d), Color=(%d,%d,%d,%d), Text=%q\n",
					idx, cmd.Type, cmd.X1, cmd.Y1, cmd.X2, cmd.Y2, cmd.R, cmd.G, cmd.B, cmd.A, cmd.Text)
			}
		}
	}

	// Serialize
	serializeStart := time.Now()
	frame := painter.ToRenderFrame(Width, Height, cursorVal)
	globalLastFrame = frame
	payload, err := protocol.EncodeRenderFrame(frame)
	serializeDuration := time.Since(serializeStart)
	if err != nil {
		fmt.Printf("failed to serialize render frame: %v\n", err)
		return atlasChanged
	}
	writeStart := time.Now()
	_ = writeMessage(conn, payload)
	writeDuration := time.Since(writeStart)
	globalPerfTracker.recordFrame(buildPagesDuration, renderDuration, serializeDuration, writeDuration, time.Since(frameStart), len(painter.commands))
	return atlasChanged
}

func toProtocolSemanticTree(tree semantics.Tree) protocol.SemanticTree {
	out := protocol.SemanticTree{Revision: tree.Revision}
	var appendNode func(semantics.Node, int32)
	appendNode = func(node semantics.Node, parent int32) {
		index := int32(len(out.Nodes))
		state := uint32(0)
		flags := []bool{node.State.Disabled, node.State.Focused, node.State.Selected, node.State.Checked,
			node.State.Expanded, node.State.ReadOnly, node.State.Required, node.State.Invalid, node.State.Password, node.State.Offscreen}
		for bit, enabled := range flags {
			if enabled {
				state |= 1 << bit
			}
		}
		wire := protocol.SemanticNode{Parent: parent, ID: node.ID, Role: string(node.Role), Name: node.Name,
			Description: node.Description, AccessKey: node.AccessKey, Value: node.Value, X1: int32(node.Bounds.Min.X), Y1: int32(node.Bounds.Min.Y),
			X2: int32(node.Bounds.Max.X), Y2: int32(node.Bounds.Max.Y), State: state}
		if node.Range != nil {
			wire.HasRange = true
			wire.RangeMin = node.Range.Minimum
			wire.RangeMax = node.Range.Maximum
			wire.SmallChange = node.Range.SmallChange
			wire.LargeChange = node.Range.LargeChange
		}
		if node.Text != nil {
			wire.HasText = true
			wire.SelectionStart = int32(node.Text.SelectionStart)
			wire.SelectionEnd = int32(node.Text.SelectionEnd)
			wire.Multiline = node.Text.Multiline
		}
		if node.Collection != nil && node.Collection.Selectable {
			wire.HasCollection = true
			wire.CanSelectMultiple = node.Collection.CanSelectMultiple
			wire.SelectionRequired = node.Collection.SelectionRequired
		}
		if node.Grid != nil {
			wire.HasGrid = true
			wire.GridRows = int32(node.Grid.Rows)
			wire.GridColumns = int32(node.Grid.Columns)
		}
		if node.GridItem != nil {
			wire.HasGridItem = true
			wire.GridRow = int32(node.GridItem.Row)
			wire.GridColumn = int32(node.GridItem.Column)
			wire.GridRowSpan = int32(node.GridItem.RowSpan)
			wire.GridColumnSpan = int32(node.GridItem.ColumnSpan)
		}
		if node.Scroll != nil {
			wire.HasScroll = true
			wire.HScrollable = node.Scroll.HorizontallyScrollable
			wire.VScrollable = node.Scroll.VerticallyScrollable
			wire.HScrollPercent = node.Scroll.HorizontalPercent
			wire.VScrollPercent = node.Scroll.VerticalPercent
			wire.HViewSize = node.Scroll.HorizontalViewSize
			wire.VViewSize = node.Scroll.VerticalViewSize
		}
		wire.LabeledBy = append(wire.LabeledBy, node.Relations.LabeledBy...)
		wire.DescribedBy = append(wire.DescribedBy, node.Relations.DescribedBy...)
		wire.Controls = append(wire.Controls, node.Relations.Controls...)
		wire.FlowsTo = append(wire.FlowsTo, node.Relations.FlowsTo...)
		for _, action := range node.Actions {
			wire.Actions = append(wire.Actions, string(action))
		}
		out.Nodes = append(out.Nodes, wire)
		for _, child := range node.Children {
			appendNode(child, index)
		}
	}
	appendNode(tree.Root, -1)
	return out
}

// Input event processor & state mapping
func processEventBatch(batch protocol.EventBatch, conn io.Writer, painter *ProtocolPainter) bool {
	if globalState == nil {
		return false
	}
	batchStart := time.Now()

	stateChanged := false
	resizeEvents := 0
	mouseEvents := 0
	keyboardEvents := 0

	for _, ev := range batch.Events {
		stateChanged = true

		switch ev.Type {
		case protocol.EventTypeWindowClose:
			fmt.Println("Native host requested window termination. Shutting down Go...")
			return true

		case protocol.EventTypeWindowSize:
			resizeEvents++
			w := int(ev.Width)
			h := int(ev.Height)
			if w > 0 && h > 0 {
				Width = w
				Height = h
				types.Width = w
				types.Height = h
				globalState.WindowWidth = w
				globalState.WindowHeight = h
				if ev.X > 0 && ev.Y > 0 {
					globalState.PhysicalWindowWidth = int(ev.X)
					globalState.PhysicalWindowHeight = int(ev.Y)
				}
				globalState.ResolveDesign(w, h)
			}

		case protocol.EventTypeCapabilities:
			capabilities, err := design.DecodeCapabilityUpdate(ev.Value)
			if err == nil {
				globalState.DesignEnvironment.Input = capabilities.Input()
				globalState.DesignEnvironment.Density = capabilities.Density
				globalState.DesignEnvironment.HighContrast = capabilities.HighContrast
				if capabilities.TextScale > 0 {
					globalState.TextScale = capabilities.TextScale
				}
				globalState.ReducedMotion = capabilities.ReducedMotion
				globalState.ResolveDesign(globalState.WindowWidth, globalState.WindowHeight)
			}

		case protocol.EventTypeMouseDown:
			mouseEvents++
			globalState.ClickCount++
			globalState.MouseX = int(ev.X)
			globalState.MouseY = int(ev.Y)

			pt := image.Point{globalState.MouseX, globalState.MouseY}
			initialTarget := libFindHoveredComponent(pt)
			globalState.DismissFocusLossOverlaysForPointer(initialTarget, pt)
			newFocus := libFindHoveredComponent(pt)
			globalState.FocusedID = newFocus

			// Acoustic feedback click hook
			if newFocus != "" && globalState.AudioEnabled {
				sendSoundEvent(conn, protocol.SoundTypeClick)
			}

			globalState.ActiveID = newFocus
			if globalState.ActiveID != "" {
				handled := false
				comps := interactionRoots()
				{
					for i := len(comps) - 1; i >= 0; i-- {
						c := comps[i]
						if c.HitTest(pt) != "" {
							if c.OnMouseDown(pt, globalState) {
								handled = true
								break
							}
						}
					}
				}
				if !handled {
					if comp := libFindComponent(globalState.ActiveID); comp != nil {
						if comp.OnMouseDown(pt, globalState) {
							if s, ok := comp.(*components.Slider); ok {
								globalState.Volume = s.Value
							}
						}
					}
				}
			}
			globalState.StatusText = fmt.Sprintf("Interaction Captured: %d Clicks | Active target: %q", globalState.ClickCount, newFocus)

		case protocol.EventTypeMouseUp:
			mouseEvents++
			globalState.MouseX = int(ev.X)
			globalState.MouseY = int(ev.Y)
			pt := image.Point{globalState.MouseX, globalState.MouseY}

			if globalState.ActiveID != "" {
				handled := false
				comps := interactionRoots()
				{
					for i := len(comps) - 1; i >= 0; i-- {
						c := comps[i]
						if c.HitTest(pt) != "" {
							if c.OnMouseUp(pt, globalState) {
								handled = true
								break
							}
						}
					}
				}
				if !handled {
					if comp := libFindComponent(globalState.ActiveID); comp != nil {
						comp.OnMouseUp(pt, globalState)
					}
				}
				globalState.ActiveID = ""
			}

		case protocol.EventTypeMouseMove:
			mouseEvents++
			globalState.MouseX = int(ev.X)
			globalState.MouseY = int(ev.Y)
			pt := image.Point{globalState.MouseX, globalState.MouseY}

			newHover := libFindHoveredComponent(pt)
			if newHover != "" && newHover != globalState.HoveredID && globalState.AudioEnabled {
				if comp := libFindComponent(newHover); comp != nil && comp.Focusable() {
					sendSoundEvent(conn, protocol.SoundTypeHover)
				}
			}
			globalState.HoveredID = newHover

			if globalState.ActiveID != "" {
				handled := false
				comps := interactionRoots()
				{
					for i := len(comps) - 1; i >= 0; i-- {
						c := comps[i]
						if c.HitTest(pt) != "" {
							if c.OnMouseMove(pt, globalState) {
								handled = true
								break
							}
						}
					}
				}
				if !handled {
					if comp := libFindComponent(globalState.ActiveID); comp != nil {
						if comp.OnMouseMove(pt, globalState) {
							if s, ok := comp.(*components.Slider); ok {
								globalState.Volume = s.Value
							}
						}
					}
				}
			} else if newHover != "" {
				handled := false
				comps := interactionRoots()
				{
					for i := len(comps) - 1; i >= 0; i-- {
						c := comps[i]
						if c.HitTest(pt) != "" {
							if c.OnMouseMove(pt, globalState) {
								handled = true
								break
							}
						}
					}
				}
				if !handled {
					if comp := libFindComponent(newHover); comp != nil {
						comp.OnMouseMove(pt, globalState)
					}
				}
			}

		case protocol.EventTypeMouseWheel:
			mouseEvents++
			delta := int(ev.Delta)
			pt := image.Point{globalState.MouseX, globalState.MouseY}

			comps := interactionRoots()
			for rootIndex := len(comps) - 1; rootIndex >= 0; rootIndex-- {
				var targets []types.ScrollableComponent
				comps[rootIndex].Walk(func(c types.Component) {
					if sc, ok := c.(types.ScrollableComponent); ok && pt.In(c.Bounds()) {
						targets = append(targets, sc)
					}
				})
				for targetIndex := len(targets) - 1; targetIndex >= 0; targetIndex-- {
					if targets[targetIndex].OnMouseWheel(pt, delta, globalState) {
						targetIndex = -1
						rootIndex = -1
					}
				}
			}

		case protocol.EventTypePanGesture:
			mouseEvents++
			if ev.Phase > protocol.GesturePhaseCancel {
				break
			}
			pt := image.Pt(int(ev.X), int(ev.Y))
			delta := image.Pt(int(ev.DeltaX), int(ev.DeltaY))
			phase := types.GesturePhase(ev.Phase)
			handled := false
			comps := interactionRoots()
			for rootIndex := len(comps) - 1; rootIndex >= 0 && !handled; rootIndex-- {
				targets := pannableTargetsAt(comps[rootIndex], pt)
				for targetIndex := len(targets) - 1; targetIndex >= 0; targetIndex-- {
					if targets[targetIndex].OnPanGesture(pt, delta, phase, globalState) {
						handled = true
						break
					}
				}
			}
			if !handled && phase == types.GestureUpdate && delta.Y != 0 {
				// Preserve ordinary Android page scrolling when no two-axis
				// gesture target consumes the event.
				for rootIndex := len(comps) - 1; rootIndex >= 0 && !handled; rootIndex-- {
					var targets []types.ScrollableComponent
					comps[rootIndex].Walk(func(c types.Component) {
						if target, ok := c.(types.ScrollableComponent); ok && pt.In(c.Bounds()) {
							targets = append(targets, target)
						}
					})
					for targetIndex := len(targets) - 1; targetIndex >= 0; targetIndex-- {
						if targets[targetIndex].OnMouseWheel(pt, delta.Y*120/100, globalState) {
							handled = true
							break
						}
					}
				}
			}

		case protocol.EventTypePinchGesture:
			mouseEvents++
			if ev.Phase > protocol.GesturePhaseCancel || ev.Scale <= 0 || math.IsNaN(float64(ev.Scale)) || math.IsInf(float64(ev.Scale), 0) {
				break
			}
			pt := image.Pt(int(ev.X), int(ev.Y))
			delta := image.Pt(int(ev.DeltaX), int(ev.DeltaY))
			phase := types.GesturePhase(ev.Phase)
			comps := interactionRoots()
			handled := false
			for rootIndex := len(comps) - 1; rootIndex >= 0 && !handled; rootIndex-- {
				var targets []types.PinchableComponent
				comps[rootIndex].Walk(func(c types.Component) {
					if target, ok := c.(types.PinchableComponent); ok && pt.In(c.Bounds()) {
						targets = append(targets, target)
					}
				})
				for targetIndex := len(targets) - 1; targetIndex >= 0; targetIndex-- {
					if targets[targetIndex].OnPinchGesture(pt, delta, float64(ev.Scale), phase, globalState) {
						handled = true
						break
					}
				}
			}

		case protocol.EventTypeKeyDown:
			keyboardEvents++
			wparam := ev.Keycode
			if globalState.KeysPressed == nil {
				globalState.KeysPressed = make(map[uint32]bool)
			}
			globalState.KeysPressed[wparam] = true

			const VK_ESCAPE = 0x1B
			const VK_TAB = 0x09

			modifiers := events.Modifiers{
				Control: (ev.Button & 1) != 0,
				Shift:   (ev.Button & 2) != 0,
				Alt:     (ev.Button & 4) != 0,
			}
			shortcut, hasShortcut := events.ShortcutFromVirtualKey(wparam, modifiers)

			if wparam == VK_ESCAPE && !modifiers.Control && !modifiers.Shift && !modifiers.Alt {
				if _, ok := globalState.DismissTopOverlay(); ok {
				} else {
					globalState.FocusedID = ""
				}
			} else if wparam == VK_TAB && !modifiers.Control && !modifiers.Alt {
				globalState.CycleFocus(modifiers.Shift)
			} else if hasShortcut && globalState.DispatchShortcut(shortcut.String()) {
				globalState.NeedsRepaint = true
			} else if keyRunes := []rune(shortcut.Key); hasShortcut && modifiers.Alt && len(keyRunes) == 1 && activateMnemonic(keyRunes[0]) {
				globalState.NeedsRepaint = true
			} else if globalState.FocusedID != "" {
				comps := interactionRoots()
				{
					for _, c := range comps {
						if c.OnKey(uint32(wparam), 0, globalState) {
							if comp := libFindComponent(globalState.FocusedID); comp != nil {
								if s, ok := comp.(*components.Slider); ok && s.CompID == "sld_vol" {
									globalState.Volume = s.Value
								}
							}
							break
						}
					}
				}
			}

		case protocol.EventTypeKeyUp:
			keyboardEvents++
			wparam := ev.Keycode
			if globalState.KeysPressed == nil {
				globalState.KeysPressed = make(map[uint32]bool)
			}
			globalState.KeysPressed[wparam] = false

		case protocol.EventTypeKeyChar:
			keyboardEvents++
			charRune := rune(ev.Char)
			if globalState.FocusedID != "" {
				comps := interactionRoots()
				{
					for _, c := range comps {
						if c.OnKey(0, charRune, globalState) {
							break
						}
					}
				}
			}
		case protocol.EventTypeCompositionStart, protocol.EventTypeCompositionUpdate, protocol.EventTypeCompositionEnd:
			keyboardEvents++
			if globalState.FocusedID != "" {
				if component := libFindComponent(globalState.FocusedID); component != nil {
					if target, ok := component.(types.TextCompositionComponent); ok {
						switch ev.Type {
						case protocol.EventTypeCompositionStart:
							target.StartComposition(globalState)
						case protocol.EventTypeCompositionUpdate:
							target.UpdateComposition(ev.Text, globalState)
						case protocol.EventTypeCompositionEnd:
							target.EndComposition(ev.Text, globalState)
						}
						globalState.NeedsRepaint = true
					}
				}
			}
		case protocol.EventTypeSemanticAction:
			if performSemanticAction(ev.Target, semantics.Action(ev.Action), ev.Value) {
				globalState.NeedsRepaint = true
			}
		}
	}
	globalPerfTracker.recordEventBatch(len(batch.Events), time.Since(batchStart), resizeEvents, mouseEvents, keyboardEvents)
	if stateChanged {
		globalState.NeedsRepaint = true
	}
	return false
}

func activateMnemonic(key rune) bool {
	if globalState == nil || key == 0 {
		return false
	}
	want := unicode.ToUpper(key)
	for _, root := range interactionRoots() {
		handled := false
		root.Walk(func(candidate types.Component) {
			if handled {
				return
			}
			if mnemonic, ok := candidate.(types.MnemonicComponent); ok && unicode.ToUpper(mnemonic.MnemonicKey()) == want {
				handled = mnemonic.ActivateMnemonic(globalState)
			}
		})
		if handled {
			return true
		}
	}
	return false
}

func performSemanticAction(target string, action semantics.Action, value string) bool {
	if globalState == nil || target == "" {
		return false
	}
	if action == semantics.ActionFocus {
		globalState.DismissFocusLossOverlaysForTarget(target)
	}
	for _, root := range interactionRoots() {
		handled := false
		root.Walk(func(candidate types.Component) {
			if handled {
				return
			}
			if semantic, ok := candidate.(types.SemanticActionComponent); ok {
				handled = semantic.PerformSemanticAction(target, action, value, globalState)
			}
		})
		if handled {
			return true
		}
	}

	component := libFindComponent(target)
	if component == nil {
		return false
	}
	switch action {
	case semantics.ActionFocus:
		return automationFocusComponent(target) == nil
	case semantics.ActionInvoke, semantics.ActionSelect:
		return automationClickComponent(target) == nil
	case semantics.ActionSetValue:
		if automationSetText(target, value) == nil {
			return true
		}
		slider, ok := component.(*components.Slider)
		if !ok || slider.Max <= slider.Min {
			return false
		}
		number, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return false
		}
		fraction := (float32(number) - slider.Min) / (slider.Max - slider.Min)
		if fraction < 0 {
			fraction = 0
		} else if fraction > 1 {
			fraction = 1
		}
		point := image.Pt(slider.Bounds().Min.X+int(fraction*float32(slider.Bounds().Dx())), slider.Bounds().Min.Y+slider.Bounds().Dy()/2)
		return slider.OnMouseDown(point, globalState) && slider.OnMouseUp(point, globalState)
	case semantics.ActionSetSelection:
		var start, end int
		if _, err := fmt.Sscanf(value, "%d:%d", &start, &end); err != nil {
			return false
		}
		return automationSelectText(target, start, end) == nil
	case semantics.ActionIncrement, semantics.ActionDecrement:
		globalState.FocusedID = target
		key := uint32(0x27)
		if action == semantics.ActionDecrement {
			key = 0x25
		}
		return component.OnKey(key, 0, globalState)
	}
	return false
}

// globalHostedMode is true when the engine runs inside a presenter's process
// (RunHosted — the Android APK). Some engine→presenter messages only make
// sense there and would confuse the Windows sidecar's strict decoder, so
// their emission is gated on it.
var globalHostedMode bool

// imeLastFocus / imeShown track the text-entry focus state most recently
// reconciled with the presenter, guarded by stateMutex like the focus itself.
var (
	imeLastFocus string
	imeShown     bool
)

// updateImeVisibility tells a hosted presenter to summon or dismiss the
// platform text-input method whenever keyboard focus enters or leaves a
// text-entry component. Called from the frame loop with stateMutex held; it
// only walks the tree when the focused ID actually changed.
func updateImeVisibility(conn io.Writer) {
	if !globalHostedMode || globalState == nil {
		return
	}
	if globalState.FocusedID == imeLastFocus {
		return
	}
	imeLastFocus = globalState.FocusedID
	wants := false
	if comp := libFindComponent(imeLastFocus); comp != nil {
		switch comp.(type) {
		case *components.TextInput, *components.TextArea, *components.Autocomplete:
			wants = true
		}
	}
	if wants == imeShown {
		return
	}
	imeShown = wants
	if payload, err := protocol.EncodeSetImeVisible(protocol.SetImeVisible{Visible: wants}); err == nil {
		_ = writeMessage(conn, payload)
	}
}

// ensureFocusedVisible scrolls the ScrollView containing the focused
// component until the component sits inside the viewport. Called from the
// frame loop (stateMutex held) while the IME is up: when the soft keyboard
// shrinks the viewport, the field being edited must not be left underneath
// it. ScrollView children keep content-space bounds anchored at the viewport
// origin, so a component is visible iff bounds−ScrollY falls within Rect.
func ensureFocusedVisible() {
	if globalState == nil || globalState.FocusedID == "" || globalState.ScrollPositions == nil {
		return
	}
	focused := libFindComponent(globalState.FocusedID)
	if focused == nil {
		return
	}
	fb := focused.Bounds()
	if fb.Empty() {
		return
	}
	for _, root := range interactionRoots() {
		root.Walk(func(c types.Component) {
			sv, ok := c.(*components.ScrollView)
			if !ok || sv.ContentH <= sv.Rect.Dy() {
				return
			}
			contains := false
			for _, child := range sv.Children {
				child.Walk(func(cc types.Component) {
					if cc.ID() == globalState.FocusedID {
						contains = true
					}
				})
			}
			if !contains {
				return
			}
			const margin = 12
			scrollY := globalState.ScrollPositions[sv.CompID]
			target := scrollY
			if fb.Max.Y-target > sv.Rect.Max.Y-margin {
				target = fb.Max.Y - sv.Rect.Max.Y + margin
			}
			if fb.Min.Y-target < sv.Rect.Min.Y+margin {
				target = fb.Min.Y - sv.Rect.Min.Y - margin
			}
			if max := sv.ContentH - sv.Rect.Dy(); target > max {
				target = max
			}
			if target < 0 {
				target = 0
			}
			if target != scrollY {
				globalState.ScrollPositions[sv.CompID] = target
				globalState.NeedsRepaint = true
			}
		})
	}
}

func libFindComponent(id string) types.Component {
	if id == "" || globalState == nil {
		return nil
	}
	var found types.Component
	for _, c := range interactionRoots() {
		c.Walk(func(comp types.Component) {
			if comp.ID() == id {
				found = comp
			}
		})
	}
	return found
}

func libFindHoveredComponent(pt image.Point) string {
	if globalState == nil {
		return ""
	}
	comps := interactionRoots()
	if len(comps) > 0 {
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

func interactionRoots() []types.Component {
	if globalState == nil {
		return nil
	}
	page := globalState.Pages[globalState.CurrentPage]
	entries := globalState.Overlays.Snapshot()
	roots := make([]types.Component, 0, len(page)+len(entries))
	roots = append(roots, page...)
	for _, entry := range entries {
		roots = append(roots, entry.Component)
	}
	return roots
}

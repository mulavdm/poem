package render

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"go_native_gpu_gui/internal/win32"
	"go_native_gpu_gui/pkg/render/components"
	"go_native_gpu_gui/pkg/render/protocol"
	"go_native_gpu_gui/pkg/render/types"
)

type AppConfig struct {
	Title        string
	Width        int
	Height       int
	BuildPagesFn func(state *types.ApplicationState)
	FontPath     string
	FontSize     float64
	Automation   *AutomationConfig
}

// Package-level orchestrator variables
var (
	globalState        *types.ApplicationState
	globalBuildPages   func(state *types.ApplicationState)
	pipeHandle         uintptr
	pipeWriteMutex     sync.Mutex
	stateMutex         sync.Mutex // Protects globalState, component layouts, and painter from concurrent races
	globalFontPath     string
	globalFontSize     float64
	globalPainter      *ProtocolPainter
	globalRenderConn   io.Writer
	globalLastFrame    protocol.RenderFrame
	nativeDebugReqMu   sync.Mutex
	nativeDebugMu      sync.Mutex
	nativeDebugRespCh  chan protocol.NativeDebugResponse
	nativeDialogReqMu  sync.Mutex
	nativeDialogMu     sync.Mutex
	nativeDialogRespCh chan protocol.NativeDialogResponse
)

const (
	pipeNameGoToSidecar = `\\.\pipe\poem_ipc_go_to_sidecar`
	pipeNameSidecarToGo = `\\.\pipe\poem_ipc_sidecar_to_go`
)

func Run(config AppConfig) {
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
	globalFontSize = config.FontSize
	if globalFontSize <= 0 {
		globalFontSize = 17
	}

	// 2. Initialize application state (headless settings, GDI sound stubs removed)
	globalState = &types.ApplicationState{
		StatusText:           "Orchestrator Matrix Running headlessly",
		Volume:               75.0,
		GlassEnabled:         true,
		ArrowCursor:          1, // Abstract ID for Arrow
		HandCursor:           2, // Abstract ID for Hand
		IBeamCursor:          3, // Abstract ID for IBeam
		ScrollPositions:      make(map[string]int),
		ScrollDragStart:      make(map[string]int),
		ScrollStartY:         make(map[string]int),
		ScrollCurrent:        make(map[string]float64),
		TextInputValues:      make(map[string]string),
		SliderValues:         make(map[string]float32),
		AudioEnabled:         true,
		KeysPressed:          make(map[uint32]bool),
		ParticlesEnabled:     true,
		WindowWidth:          Width,
		WindowHeight:         Height,
		PhysicalWindowWidth:  Width,
		PhysicalWindowHeight: Height,
	}
	globalState.CursorID = globalState.ArrowCursor
	globalBuildPages = config.BuildPagesFn

	globalState.StartTime = time.Now()
	globalState.CoreMask = (1 << uint(runtime.NumCPU())) - 1
	globalState.Particles = types.NewParticleSystem(100, image.Rect(0, 0, Width, Height))

	if globalBuildPages != nil {
		globalBuildPages(globalState)
	}

	// 3. Create Windows named pipes (separate directions avoid duplex blocking deadlocks)
	pipeNameGoToSidecarUTF16, _ := syscall.UTF16PtrFromString(pipeNameGoToSidecar)
	pipeHandleGoToSidecar, err := win32.CreateNamedPipe(
		pipeNameGoToSidecarUTF16,
		win32.PIPE_ACCESS_DUPLEX,
		win32.PIPE_TYPE_BYTE|win32.PIPE_READMODE_BYTE|win32.PIPE_WAIT,
		win32.PIPE_UNLIMITED_INSTANCES,
		1024*1024, // 1MB Output buffer
		1024*1024, // 1MB Input buffer
		0,
		0,
	)
	if err != nil || pipeHandleGoToSidecar == 0 {
		panic(fmt.Sprintf("orchestrator failed to create Go->sidecar IPC pipe: %v", err))
	}
	defer win32.CloseHandle(pipeHandleGoToSidecar)

	pipeNameSidecarToGoUTF16, _ := syscall.UTF16PtrFromString(pipeNameSidecarToGo)
	pipeHandleSidecarToGo, err := win32.CreateNamedPipe(
		pipeNameSidecarToGoUTF16,
		win32.PIPE_ACCESS_DUPLEX,
		win32.PIPE_TYPE_BYTE|win32.PIPE_READMODE_BYTE|win32.PIPE_WAIT,
		win32.PIPE_UNLIMITED_INSTANCES,
		1024*1024,
		1024*1024,
		0,
		0,
	)
	if err != nil || pipeHandleSidecarToGo == 0 {
		panic(fmt.Sprintf("orchestrator failed to create sidecar->Go IPC pipe: %v", err))
	}
	defer win32.CloseHandle(pipeHandleSidecarToGo)

	fmt.Println("IPC named pipes created. Awaiting native presentation sidecar connection...")

	// 4. Spawn the native presentation engine child process
	var sidecarCmd *exec.Cmd
	sidecarPath, err := resolveSidecarPath()
	if err != nil {
		panic(fmt.Sprintf("failed to locate native presentation sidecar: %v", err))
	}
	fmt.Printf("Spawning native presentation sidecar: %s\n", sidecarPath)
	sidecarCmd = exec.Command(sidecarPath, config.Title)

	sidecarCmd.Stdout = os.Stdout
	sidecarCmd.Stderr = os.Stderr
	if err := sidecarCmd.Start(); err != nil {
		panic(fmt.Sprintf("failed to spawn native presentation sidecar: %v", err))
	}

	// Clean shutdown zombie mitigation
	defer func() {
		if sidecarCmd.Process != nil {
			fmt.Println("Terminating native presentation sidecar...")
			sidecarCmd.Process.Kill()
		}
	}()

	// 5. Establish Named Pipe client links
	connected1, err := win32.ConnectNamedPipe(pipeHandleGoToSidecar, 0)
	if err != nil || !connected1 {
		panic(fmt.Sprintf("failed to lock client connection on Go->sidecar named pipe: %v", err))
	}
	connected2, err := win32.ConnectNamedPipe(pipeHandleSidecarToGo, 0)
	if err != nil || !connected2 {
		panic(fmt.Sprintf("failed to lock client connection on sidecar->Go named pipe: %v", err))
	}
	fmt.Println("IPC handshake synchronized. Native presentation core bound successfully.")

	// Wrap our raw pipe handles in io.ReadWriter
	pipeConnGoToSidecar := &pipeReadWriteCloser{handle: pipeHandleGoToSidecar}
	pipeConnSidecarToGo := &pipeReadWriteCloser{handle: pipeHandleSidecarToGo}
	globalRenderConn = pipeConnGoToSidecar

	globalState.PlaySoundFn = func(soundType int8) {
		sendSoundEvent(pipeConnGoToSidecar, protocol.SoundType(soundType))
	}

	// 6. Generate and transmit dynamically-rasterized Font Atlas
	atlasPixels, chars, atlasW, atlasH, measuredLSB := buildHiDPIFontAtlasPixels(globalFontPath, globalFontSize)

	// Dynamically resolve monospaced character width and LSB from loaded font metrics
	detectedWidth := 7
	for _, char := range chars {
		if char.R == 'A' {
			detectedWidth = int(char.Advance) // Advance = full cell width used for layout
			break
		}
	}
	globalState.FontCharWidth = detectedWidth
	globalState.FontCharBearingX = measuredLSB
	fmt.Printf("❖ Font Engine: CharWidth=%dpx, BearingX=%dpx\n", detectedWidth, measuredLSB)

	initBytes, err := serializeInitEngine(Width, Height, atlasPixels, chars, atlasW, atlasH)
	if err != nil {
		panic(fmt.Sprintf("Failed to serialize InitEngine bootstrap package: %v", err))
	}

	if err := writeMessage(pipeConnGoToSidecar, initBytes); err != nil {
		panic(fmt.Sprintf("Failed to transmit InitEngine bootstrap package: %v", err))
	}
	fmt.Println("🔤 Font Atlas & Metrics bootstrap context loaded successfully to sidecar.")

	// 7. Initialize protocol-backed painter
	painter := NewProtocolPainter()
	globalPainter = painter

	if config.Automation != nil && config.Automation.Enabled {
		startAutomationServer(resolveAutomationConfig(config.Automation))
	}

	// 8. Start Background Physics & Repaint loop (headful updates mapped back to sidecar)
	lastFrame := time.Now()
	go func() {
		var telemetryTimer float64
		for {
			start := time.Now()
			time.Sleep(16 * time.Millisecond) // ~60 FPS

			stateMutex.Lock()
			now := time.Now()
			dt := now.Sub(lastFrame).Seconds()
			if dt > 0 {
				globalState.CurrentFPS = 1.0 / dt
				globalState.LastDt = dt
				if globalState.Particles != nil {
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

			shouldRepaint := globalState.NeedsRepaint || globalState.ParticlesEnabled || globalState.IsTransitioning
			if shouldRepaint {
				triggerRepaintFrame(pipeConnGoToSidecar, painter)
				globalState.NeedsRepaint = false
			}
			stateMutex.Unlock()
		}
	}()

	// 9. Main event receiver loop (blocks on incoming event batches from the sidecar)
	for {
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
			processEventBatch(batch, pipeConnGoToSidecar, painter)
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

// Named Pipe wrapper implementing io.ReadWriteCloser
type pipeReadWriteCloser struct {
	handle uintptr
}

func (p *pipeReadWriteCloser) Read(b []byte) (int, error) {
	var read uint32
	// Use Win32 ReadFile API under the hood
	err := syscall.ReadFile(syscall.Handle(p.handle), b, &read, nil)
	if err != nil {
		if err == syscall.ERROR_BROKEN_PIPE {
			return 0, io.EOF
		}
		return 0, err
	}
	return int(read), nil
}

func (p *pipeReadWriteCloser) Write(b []byte) (int, error) {
	var written uint32
	err := syscall.WriteFile(syscall.Handle(p.handle), b, &written, nil)
	if err != nil {
		return 0, err
	}
	return int(written), nil
}

func (p *pipeReadWriteCloser) Close() error {
	win32.DisconnectNamedPipe(p.handle)
	win32.CloseHandle(p.handle)
	return nil
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

func triggerRepaintFrame(conn io.Writer, painter *ProtocolPainter) {
	if globalState == nil {
		return
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
		return
	}
	writeStart := time.Now()
	_ = writeMessage(conn, payload)
	writeDuration := time.Since(writeStart)
	globalPerfTracker.recordFrame(buildPagesDuration, renderDuration, serializeDuration, writeDuration, time.Since(frameStart), len(painter.commands))
}

// Input event processor & state mapping
func processEventBatch(batch protocol.EventBatch, conn io.Writer, painter *ProtocolPainter) {
	if globalState == nil {
		return
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
			fmt.Println("Sidecar requested window termination. Shutting down Go...")
			os.Exit(0)

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
			}

		case protocol.EventTypeMouseDown:
			mouseEvents++
			globalState.ClickCount++
			globalState.MouseX = int(ev.X)
			globalState.MouseY = int(ev.Y)

			pt := image.Point{globalState.MouseX, globalState.MouseY}
			newFocus := libFindHoveredComponent(pt)
			globalState.FocusedID = newFocus

			// Acoustic feedback click hook
			if newFocus != "" && globalState.AudioEnabled {
				sendSoundEvent(conn, protocol.SoundTypeClick)
			}

			globalState.ActiveID = newFocus
			if globalState.ActiveID != "" {
				handled := false
				if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
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
				if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
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
				if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
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
				if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
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

			if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
				for _, comp := range comps {
					comp.Walk(func(c types.Component) {
						if sc, ok := c.(types.ScrollableComponent); ok {
							if pt.In(c.Bounds()) {
								sc.OnMouseWheel(pt, delta, globalState)
							}
						}
					})
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

			ctrlPressed := (ev.Button & 1) != 0 // Control state passed through button field

			if wparam == VK_ESCAPE {
				globalState.FocusedID = ""
			} else if wparam == VK_TAB {
				reverse := (ev.Button & 2) != 0 // Shift state passed through button field
				globalState.CycleFocus(reverse)
			} else if ctrlPressed && (wparam == 'S' || wparam == 's') && globalState.AudioEnabled {
				if globalState.Hotkeys != nil {
					if handler, ok := globalState.Hotkeys["Ctrl+S"]; ok {
						handler(globalState)
						sendSoundEvent(conn, protocol.SoundTypeSuccess)
					}
				}
			} else if globalState.FocusedID != "" {
				if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
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
				if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
					for _, c := range comps {
						if c.OnKey(0, charRune, globalState) {
							break
						}
					}
				}
			}
		}
	}
	globalPerfTracker.recordEventBatch(len(batch.Events), time.Since(batchStart), resizeEvents, mouseEvents, keyboardEvents)
	if stateChanged {
		globalState.NeedsRepaint = true
	}
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

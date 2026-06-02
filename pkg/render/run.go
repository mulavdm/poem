package render

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"go_native_gpu_gui/internal/win32"
	"go_native_gpu_gui/pkg/render/components"
	"go_native_gpu_gui/pkg/render/poem"
	"go_native_gpu_gui/pkg/render/types"
)

type AppConfig struct {
	Title        string
	Width        int
	Height       int
	BuildPagesFn func(state *types.ApplicationState)
	FontPath     string
	FontSize     float64
}

// Package-level orchestrator variables
var (
	globalState      *types.ApplicationState
	globalBuildPages func(state *types.ApplicationState)
	pipeHandle       uintptr
	pipeWriteMutex   sync.Mutex
	stateMutex       sync.Mutex // Protects globalState, component layouts, and painter from concurrent races
	globalFontPath   string
	globalFontSize   float64
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
		StatusText:       "Orchestrator Matrix Running headlessly",
		Volume:           75.0,
		GlassEnabled:     true,
		ArrowCursor:      1, // Abstract ID for Arrow
		HandCursor:       2, // Abstract ID for Hand
		IBeamCursor:      3, // Abstract ID for IBeam
		ScrollPositions:  make(map[string]int),
		ScrollDragStart:  make(map[string]int),
		ScrollStartY:     make(map[string]int),
		ScrollCurrent:    make(map[string]float64),
		TextInputValues:  make(map[string]string),
		SliderValues:     make(map[string]float32),
		AudioEnabled:     true,
		KeysPressed:      make(map[uint32]bool),
		ParticlesEnabled: true,
		WindowWidth:      Width,
		WindowHeight:     Height,
	}
	globalState.CursorID = globalState.ArrowCursor
	globalBuildPages = config.BuildPagesFn

	globalState.StartTime = time.Now()
	globalState.CoreMask = (1 << uint(runtime.NumCPU())) - 1
	globalState.Particles = types.NewParticleSystem(100, image.Rect(0, 0, Width, Height))

	if globalBuildPages != nil {
		globalBuildPages(globalState)
	}

	// 3. Create Windows Named Pipes (Separate GoToRust and RustToGo to avoid duplex blocking deadlocks)
	pipeNameGoToRust, _ := syscall.UTF16PtrFromString(`\\.\pipe\poem_ipc_go_to_rust`)
	pipeHandleGoToRust, err := win32.CreateNamedPipe(
		pipeNameGoToRust,
		win32.PIPE_ACCESS_DUPLEX,
		win32.PIPE_TYPE_BYTE|win32.PIPE_READMODE_BYTE|win32.PIPE_WAIT,
		win32.PIPE_UNLIMITED_INSTANCES,
		1024*1024, // 1MB Output buffer
		1024*1024, // 1MB Input buffer
		0,
		0,
	)
	if err != nil || pipeHandleGoToRust == 0 {
		panic(fmt.Sprintf("Orchestrator failed to create GoToRust IPC Pipe: %v", err))
	}
	defer win32.CloseHandle(pipeHandleGoToRust)

	pipeNameRustToGo, _ := syscall.UTF16PtrFromString(`\\.\pipe\poem_ipc_rust_to_go`)
	pipeHandleRustToGo, err := win32.CreateNamedPipe(
		pipeNameRustToGo,
		win32.PIPE_ACCESS_DUPLEX,
		win32.PIPE_TYPE_BYTE|win32.PIPE_READMODE_BYTE|win32.PIPE_WAIT,
		win32.PIPE_UNLIMITED_INSTANCES,
		1024*1024,
		1024*1024,
		0,
		0,
	)
	if err != nil || pipeHandleRustToGo == 0 {
		panic(fmt.Sprintf("Orchestrator failed to create RustToGo IPC Pipe: %v", err))
	}
	defer win32.CloseHandle(pipeHandleRustToGo)

	fmt.Println("🔒 IPC Named Pipes Created. Awaiting Rust presentation core connection...")

	// 4. Spawn the Rust presentation engine child process
	var rustCmd *exec.Cmd
	var sidecarPath string

	// Step A: Check Environment Variable Override
	if envPath := os.Getenv("POEM_SIDECAR_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			sidecarPath = envPath
		}
	}

	// Step B: Check same folder as current executable (standard downstream distribution)
	if sidecarPath == "" {
		if exePath, err := os.Executable(); err == nil {
			localPath := filepath.Join(filepath.Dir(exePath), "poem_rust_engine.exe")
			if _, err := os.Stat(localPath); err == nil {
				sidecarPath = localPath
			}
		}
	}

	// Step C: Development workspace fallback (relative to Working Directory)
	if sidecarPath == "" {
		wd, _ := os.Getwd()
		releasePath := filepath.Join(wd, "rust_engine", "target", "release", "poem_rust_engine.exe")
		debugPath := filepath.Join(wd, "rust_engine", "target", "debug", "poem_rust_engine.exe")

		if _, err := os.Stat(releasePath); err == nil {
			sidecarPath = releasePath
		} else if _, err := os.Stat(debugPath); err == nil {
			sidecarPath = debugPath
		}
	}

	if sidecarPath != "" {
		fmt.Printf("🚀 Spawning Rust presentation engine: %s\n", sidecarPath)
		rustCmd = exec.Command(sidecarPath, config.Title)
	} else {
		// Step D: Fallback to local dev cargo run
		fmt.Println("⚠️ Target Rust binary not found in cached paths. Attempting default 'cargo run' target check...")
		wd, _ := os.Getwd()
		rustCmd = exec.Command("cargo", "run", "--manifest-path", filepath.Join(wd, "rust_engine", "Cargo.toml"), "--", config.Title)
	}

	rustCmd.Stdout = os.Stdout
	rustCmd.Stderr = os.Stderr
	if err := rustCmd.Start(); err != nil {
		panic(fmt.Sprintf("Failed to spawn Rust engine: %v", err))
	}

	// Clean shutdown zombie mitigation
	defer func() {
		if rustCmd.Process != nil {
			fmt.Println("🛑 Terminating Rust presentation engine sidecar...")
			rustCmd.Process.Kill()
		}
	}()

	// 5. Establish Named Pipe client links
	connected1, err := win32.ConnectNamedPipe(pipeHandleGoToRust, 0)
	if err != nil || !connected1 {
		panic(fmt.Sprintf("Failed to lock client connection on GoToRust Named Pipe: %v", err))
	}
	connected2, err := win32.ConnectNamedPipe(pipeHandleRustToGo, 0)
	if err != nil || !connected2 {
		panic(fmt.Sprintf("Failed to lock client connection on RustToGo Named Pipe: %v", err))
	}
	fmt.Println("🤝 IPC Handshake Synchronized! Rust presentation core bound successfully.")

	// Wrap our raw pipe handles in io.ReadWriter
	pipeConnGoToRust := &pipeReadWriteCloser{handle: pipeHandleGoToRust}
	pipeConnRustToGo := &pipeReadWriteCloser{handle: pipeHandleRustToGo}

	globalState.PlaySoundFn = func(soundType int8) {
		sendSoundEvent(pipeConnGoToRust, poem.SoundType(soundType))
	}

	// 6. Generate and transmit dynamically-rasterized Font Atlas
	atlasPixels, chars, atlasW, atlasH, measuredLSB := buildFontAtlasPixels(globalFontPath, globalFontSize)

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

	builder := flatbuffers.NewBuilder(1024 * 128)
	initOffset := serializeInitEngine(builder, Width, Height, atlasPixels, chars, atlasW, atlasH)
	builder.Finish(initOffset)
	initBytes := builder.FinishedBytes()

	if err := writeMessage(pipeConnGoToRust, initBytes); err != nil {
		panic(fmt.Sprintf("Failed to transmit InitEngine bootstrap package: %v", err))
	}
	fmt.Println("🔤 Font Atlas & Metrics bootstrap context loaded successfully to sidecar.")

	// 7. Initialize FlatBuffer painter
	painter := NewFlatBufferPainter()

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

			// Trigger automatic repaint batch to synchronize rendering loop updates
			triggerRepaintFrame(pipeConnGoToRust, painter)
			stateMutex.Unlock()
		}
	}()

	// 9. Main Event Receiver Loop (Blocks on incoming event FlatBuffers from Rust)
	for {
		payload, err := readMessage(pipeConnRustToGo)
		if err != nil {
			if err == io.EOF {
				fmt.Println("🔌 Rust presentation engine disconnected gracefully.")
				break
			}
			fmt.Printf("⚠️ IPC Read Loop Error: %v\n", err)
			break
		}

		// Decode Event Batch
		msgEnvelope := poem.GetRootAsRustToGoMessage(payload, 0)
		unionTable := new(flatbuffers.Table)
		if msgEnvelope.Message(unionTable) {
			if msgEnvelope.MessageType() == poem.RustToGoUnionEventBatch {
				// Extract our event batch
				batch := new(poem.EventBatch)
				batch.Init(unionTable.Bytes, unionTable.Pos)

				stateMutex.Lock()
				// Process events
				processEventBatch(batch, pipeConnGoToRust, painter)
				stateMutex.Unlock()
			}
		}
	}
}

// Write helper
func writeMessage(conn io.Writer, payload []byte) error {
	pipeWriteMutex.Lock()
	defer pipeWriteMutex.Unlock()

	length := uint32(len(payload))
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, length)
	if _, err := conn.Write(lenBuf); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}

// Read helper
func readMessage(conn io.Reader) ([]byte, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(lenBuf)
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

// FlatBuffers InitEngine compiler
func serializeInitEngine(builder *flatbuffers.Builder, width, height int, pixels []byte, chars []fontCharInfo, atlasW, atlasH int) flatbuffers.UOffsetT {
	pixelsOffset := builder.CreateByteVector(pixels)

	charOffsets := make([]flatbuffers.UOffsetT, len(chars))
	for i, char := range chars {
		poem.CharInfoStart(builder)
		poem.CharInfoAddR(builder, char.R)
		poem.CharInfoAddU1(builder, char.U1)
		poem.CharInfoAddV1(builder, char.V1)
		poem.CharInfoAddU2(builder, char.U2)
		poem.CharInfoAddV2(builder, char.V2)
		poem.CharInfoAddWidth(builder, char.Width)
		poem.CharInfoAddHeight(builder, char.Height)
		poem.CharInfoAddAdvance(builder, char.Advance)
		charOffsets[i] = poem.CharInfoEnd(builder)
	}

	poem.InitEngineStartCharsVector(builder, len(charOffsets))
	for i := len(charOffsets) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(charOffsets[i])
	}
	charsVector := builder.EndVector(len(charOffsets))

	poem.InitEngineStart(builder)
	poem.InitEngineAddWidth(builder, int32(width))
	poem.InitEngineAddHeight(builder, int32(height))
	poem.InitEngineAddAtlasWidth(builder, int32(atlasW))
	poem.InitEngineAddAtlasHeight(builder, int32(atlasH))
	poem.InitEngineAddAtlasPixels(builder, pixelsOffset)
	poem.InitEngineAddChars(builder, charsVector)
	initOffset := poem.InitEngineEnd(builder)

	poem.GoToRustMessageStart(builder)
	poem.GoToRustMessageAddMessageType(builder, poem.GoToRustUnionInitEngine)
	poem.GoToRustMessageAddMessage(builder, initOffset)
	return poem.GoToRustMessageEnd(builder)
}

// Sound triggering helper over Named Pipe FlatBuffers
func sendSoundEvent(conn io.Writer, soundType poem.SoundType) {
	builder := flatbuffers.NewBuilder(128)
	poem.PlaySoundStart(builder)
	poem.PlaySoundAddType(builder, soundType)
	soundOffset := poem.PlaySoundEnd(builder)

	poem.GoToRustMessageStart(builder)
	poem.GoToRustMessageAddMessageType(builder, poem.GoToRustUnionPlaySound)
	poem.GoToRustMessageAddMessage(builder, soundOffset)
	msgOffset := poem.GoToRustMessageEnd(builder)

	builder.Finish(msgOffset)
	writeMessage(conn, builder.FinishedBytes())
}

// Main paint event dispatcher
var lastPrintTime time.Time

func triggerRepaintFrame(conn io.Writer, painter *FlatBufferPainter) {
	if globalState == nil {
		return
	}

	painter.Reset()

	// Rebuild dynamic page descriptors
	if globalBuildPages != nil {
		globalBuildPages(globalState)
	}

	// Trigger layout logic & flat drawing tree population
	types.RenderPipeline(painter, globalState)

	// Update dynamic hover-based cursor type automatically on every frame
	globalState.CursorID = globalState.ArrowCursor
	pt := image.Point{globalState.MouseX, globalState.MouseY}
	if hoveredID := libFindHoveredComponent(pt); hoveredID != "" {
		if comp := libFindComponent(hoveredID); comp != nil {
			if _, ok := comp.(*components.TextArea); ok {
				globalState.CursorID = globalState.IBeamCursor
			} else if _, ok := comp.(*components.TextInput); ok {
				globalState.CursorID = globalState.IBeamCursor
			} else if comp.Focusable() {
				globalState.CursorID = globalState.HandCursor
			}
		}
	}

	// Map GDI cursor handles to abstract FlatBuffer cursor types
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
	builder := flatbuffers.NewBuilder(1024 * 128)
	frameOffset := painter.Serialize(builder, Width, Height, cursorVal)
	builder.Finish(frameOffset)
	writeMessage(conn, builder.FinishedBytes())
}

// Input event processor & state mapping
func processEventBatch(batch *poem.EventBatch, conn io.Writer, painter *FlatBufferPainter) {
	if globalState == nil {
		return
	}

	stateChanged := false

	for i := 0; i < batch.EventsLength(); i++ {
		ev := new(poem.Event)
		if batch.Events(ev, i) {
			stateChanged = true

			switch ev.Type() {
			case poem.EventTypeWindowClose:
				fmt.Println("🚪 Rust requested window termination. Shutting down Go...")
				os.Exit(0)

			case poem.EventTypeWindowSize:
				w := int(ev.Width())
				h := int(ev.Height())
				if w > 0 && h > 0 {
					Width = w
					Height = h
					types.Width = w
					types.Height = h
					globalState.WindowWidth = w
					globalState.WindowHeight = h
				}

			case poem.EventTypeMouseDown:
				globalState.ClickCount++
				globalState.MouseX = int(ev.X())
				globalState.MouseY = int(ev.Y())

				pt := image.Point{globalState.MouseX, globalState.MouseY}
				newFocus := libFindHoveredComponent(pt)
				globalState.FocusedID = newFocus

				// Acoustic feedback click hook
				if newFocus != "" && globalState.AudioEnabled {
					sendSoundEvent(conn, poem.SoundTypeClick)
				}

				globalState.ActiveID = newFocus
				if globalState.ActiveID != "" {
					handled := false
					if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
						for _, c := range comps {
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

			case poem.EventTypeMouseUp:
				globalState.MouseX = int(ev.X())
				globalState.MouseY = int(ev.Y())
				pt := image.Point{globalState.MouseX, globalState.MouseY}

				if globalState.ActiveID != "" {
					handled := false
					if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
						for _, c := range comps {
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

			case poem.EventTypeMouseMove:
				globalState.MouseX = int(ev.X())
				globalState.MouseY = int(ev.Y())
				pt := image.Point{globalState.MouseX, globalState.MouseY}

				newHover := libFindHoveredComponent(pt)
				if newHover != "" && newHover != globalState.HoveredID && globalState.AudioEnabled {
					if comp := libFindComponent(newHover); comp != nil && comp.Focusable() {
						sendSoundEvent(conn, poem.SoundTypeHover)
					}
				}
				globalState.HoveredID = newHover

				if globalState.ActiveID != "" {
					handled := false
					if comps, ok := globalState.Pages[globalState.CurrentPage]; ok {
						for _, c := range comps {
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
						for _, c := range comps {
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

			case poem.EventTypeMouseWheel:
				delta := int(ev.Delta())
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

			case poem.EventTypeKeyDown:
				wparam := ev.Keycode()
				if globalState.KeysPressed == nil {
					globalState.KeysPressed = make(map[uint32]bool)
				}
				globalState.KeysPressed[wparam] = true

				const VK_ESCAPE = 0x1B
				const VK_TAB = 0x09
				const VK_CONTROL = 0x11

				ctrlPressed := (ev.Button() & 1) != 0 // Control state passed through button field

				if wparam == VK_ESCAPE {
					globalState.FocusedID = ""
				} else if wparam == VK_TAB {
					reverse := (ev.Button() & 2) != 0 // Shift state passed through button field
					globalState.CycleFocus(reverse)
				} else if ctrlPressed && (wparam == 'S' || wparam == 's') && globalState.AudioEnabled {
					if globalState.Hotkeys != nil {
						if handler, ok := globalState.Hotkeys["Ctrl+S"]; ok {
							handler(globalState)
							sendSoundEvent(conn, poem.SoundTypeSuccess)
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

			case poem.EventTypeKeyUp:
				wparam := ev.Keycode()
				if globalState.KeysPressed == nil {
					globalState.KeysPressed = make(map[uint32]bool)
				}
				globalState.KeysPressed[wparam] = false

			case poem.EventTypeKeyChar:
				charRune := rune(ev.Char())
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
	}

	if stateChanged {
		triggerRepaintFrame(conn, painter)
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

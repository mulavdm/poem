package render

import (
	"image"
	"image/color"

	"github.com/mulavdm/poem/pkg/render/protocol"
)

type ProtocolPainter struct {
	commands         []DrawCmdData
	offsetX          float32
	offsetY          float32
	glow             float32
	glass            bool
	shadowOx         float32
	shadowOy         float32
	shadowBl         float32
	clipX            int
	clipY            int
	clipW            int
	clipH            int
	clipEn           bool
	clipStack        []image.Rectangle
	clipEnabledStack []bool
	observeText      func(string)
}

func (f *ProtocolPainter) SetTextObserver(observer func(string)) { f.observeText = observer }

type DrawCmdData struct {
	Type   protocol.DrawCommandType
	X1     int
	Y1     int
	X2     int
	Y2     int
	W      int
	H      int
	Radius int
	R      byte
	G      byte
	B      byte
	A      byte
	Text   string
	Bytes  []byte
	Val1   float32
	Val2   float32
	Val3   float32
	Flag   bool
}

const (
	BillboardSeed = iota + 1
	BillboardSentry
	BillboardStash
	BillboardCue
	BillboardTubeRing
	BillboardNest
	BillboardBowl
	BillboardWheelGate
)

func NewProtocolPainter() *ProtocolPainter {
	return &ProtocolPainter{
		commands:         make([]DrawCmdData, 0, 256),
		clipStack:        make([]image.Rectangle, 0, 8),
		clipEnabledStack: make([]bool, 0, 8),
	}
}

func (f *ProtocolPainter) Reset() {
	f.commands = f.commands[:0]
	f.offsetX = 0
	f.offsetY = 0
	f.glow = 0
	f.glass = false
	f.shadowOx = 0
	f.shadowOy = 0
	f.shadowBl = 0
	f.clipX = 0
	f.clipY = 0
	f.clipW = 0
	f.clipH = 0
	f.clipEn = false
	f.clipStack = f.clipStack[:0]
	f.clipEnabledStack = f.clipEnabledStack[:0]
}

func (f *ProtocolPainter) DrawRoundedRect(r image.Rectangle, radius int, col color.RGBA) {
	f.commands = append(f.commands, DrawCmdData{
		Type:   protocol.DrawCommandTypeDrawRoundedRect,
		X1:     r.Min.X,
		Y1:     r.Min.Y,
		X2:     r.Max.X,
		Y2:     r.Max.Y,
		W:      r.Dx(),
		H:      r.Dy(),
		Radius: radius,
		R:      col.R,
		G:      col.G,
		B:      col.B,
		A:      col.A,
	})
}

// DrawRealtimeViewport reserves a clipped native real-time viewport in the
// ordinary POEM display list. Bytes are a bounded opaque ABI v3 command.
func (f *ProtocolPainter) DrawRealtimeViewport(r image.Rectangle, id string, command []byte) {
	f.commands = append(f.commands, DrawCmdData{
		Type: protocol.DrawCommandTypeDrawRealtimeViewport,
		X1:   r.Min.X, Y1: r.Min.Y, X2: r.Max.X, Y2: r.Max.Y,
		W: r.Dx(), H: r.Dy(), Text: id, Bytes: append([]byte(nil), command...),
	})
}

func (f *ProtocolPainter) DrawRaycaster(r image.Rectangle, playerX, playerY, playerAngle float32) {
	f.DrawRaycasterStyled(r, playerX, playerY, playerAngle, color.RGBA{255, 155, 70, 255})
}

func (f *ProtocolPainter) DrawRaycasterStyled(r image.Rectangle, playerX, playerY, playerAngle float32, accent color.RGBA) {
	f.DrawRaycasterMapStyled(r, playerX, playerY, playerAngle, accent, "")
}

func (f *ProtocolPainter) DrawRaycasterMapStyled(r image.Rectangle, playerX, playerY, playerAngle float32, accent color.RGBA, mapData string) {
	f.commands = append(f.commands, DrawCmdData{
		Type:   protocol.DrawCommandTypeDrawRoundedRect,
		X1:     r.Min.X,
		Y1:     r.Min.Y,
		X2:     r.Max.X,
		Y2:     r.Max.Y,
		W:      r.Dx(),
		H:      r.Dy(),
		Radius: -999,
		Val1:   playerX,
		Val2:   playerY,
		Val3:   playerAngle,
		R:      accent.R,
		G:      accent.G,
		B:      accent.B,
		A:      accent.A,
		Text:   mapData,
	})
}

func (f *ProtocolPainter) DrawSeed3D(viewportRect image.Rectangle, seedX, seedY, playerX, playerY, playerAngle float32) {
	f.DrawBillboard3D(viewportRect, seedX, seedY, BillboardSeed, color.RGBA{255, 215, 30, 255})
}

func (f *ProtocolPainter) DrawSentry3D(viewportRect image.Rectangle, sentryX, sentryY, playerX, playerY, playerAngle float32, col color.RGBA) {
	f.DrawBillboard3D(viewportRect, sentryX, sentryY, BillboardSentry, col)
}

func (f *ProtocolPainter) DrawBillboard3D(viewportRect image.Rectangle, worldX, worldY float32, kind int, col color.RGBA) {
	radius := -998
	switch kind {
	case BillboardSentry:
		radius = -997
	case BillboardStash:
		radius = -996
	case BillboardCue:
		radius = -995
	case BillboardTubeRing:
		radius = -994
	case BillboardNest:
		radius = -993
	case BillboardBowl:
		radius = -992
	case BillboardWheelGate:
		radius = -991
	}

	f.commands = append(f.commands, DrawCmdData{
		Type:   protocol.DrawCommandTypeDrawRoundedRect,
		X1:     viewportRect.Min.X,
		Y1:     viewportRect.Min.Y,
		X2:     viewportRect.Max.X,
		Y2:     viewportRect.Max.Y,
		W:      viewportRect.Dx(),
		H:      viewportRect.Dy(),
		Radius: radius,
		Val1:   worldX,
		Val2:   worldY,
		R:      col.R,
		G:      col.G,
		B:      col.B,
		A:      col.A,
	})
}

func (f *ProtocolPainter) DrawText(text string, x, y int, col color.RGBA) {
	f.DrawStyledText(text, x, y, col, 1)
}

func (f *ProtocolPainter) DrawStyledText(text string, x, y int, col color.RGBA, scale float32) {
	if scale <= 0 {
		scale = 1
	}
	if f.observeText != nil {
		f.observeText(text)
	}
	f.commands = append(f.commands, DrawCmdData{
		Type: protocol.DrawCommandTypeDrawText,
		X1:   x,
		Y1:   y,
		R:    col.R,
		G:    col.G,
		B:    col.B,
		A:    col.A,
		Text: text,
		Val1: scale,
	})
}

func (f *ProtocolPainter) FillRect(r image.Rectangle, col color.RGBA) {
	f.commands = append(f.commands, DrawCmdData{
		Type: protocol.DrawCommandTypeFillRect,
		X1:   r.Min.X,
		Y1:   r.Min.Y,
		X2:   r.Max.X,
		Y2:   r.Max.Y,
		W:    r.Dx(),
		H:    r.Dy(),
		R:    col.R,
		G:    col.G,
		B:    col.B,
		A:    col.A,
	})
}

func (f *ProtocolPainter) DrawLine(x1, y1, x2, y2 int, col color.RGBA) {
	f.commands = append(f.commands, DrawCmdData{
		Type: protocol.DrawCommandTypeDrawLine,
		X1:   x1,
		Y1:   y1,
		X2:   x2,
		Y2:   y2,
		R:    col.R,
		G:    col.G,
		B:    col.B,
		A:    col.A,
	})
}

func (f *ProtocolPainter) DrawImage(r image.Rectangle, imageWidth, imageHeight int, pixels []byte) {
	if len(pixels) == 0 || imageWidth <= 0 || imageHeight <= 0 {
		return
	}
	cloned := make([]byte, len(pixels))
	copy(cloned, pixels)
	f.commands = append(f.commands, DrawCmdData{
		Type:   protocol.DrawCommandTypeDrawImage,
		X1:     r.Min.X,
		Y1:     r.Min.Y,
		X2:     r.Max.X,
		Y2:     r.Max.Y,
		W:      imageWidth,
		H:      imageHeight,
		Bytes:  cloned,
		Radius: 0,
	})
}

// DrawMapScene places a retained scene in a clipped component rectangle. The
// fallback pixels are carried for presenters that have not accepted a scene.
func (f *ProtocolPainter) DrawMapScene(r image.Rectangle, viewportID string, imageWidth, imageHeight int, fallback []byte, previewScale float64) {
	cloned := append([]byte(nil), fallback...)
	f.commands = append(f.commands, DrawCmdData{Type: protocol.DrawCommandTypeDrawMapScene, X1: r.Min.X, Y1: r.Min.Y, X2: r.Max.X, Y2: r.Max.Y, W: imageWidth, H: imageHeight, Text: viewportID, Bytes: cloned, Val1: float32(previewScale)})
}

func (f *ProtocolPainter) SetGlow(strength float32) {
	f.glow = strength
	f.commands = append(f.commands, DrawCmdData{
		Type: protocol.DrawCommandTypeSetGlow,
		Val1: strength,
	})
}

func (f *ProtocolPainter) SetGlass(enabled bool) {
	f.glass = enabled
	f.commands = append(f.commands, DrawCmdData{
		Type: protocol.DrawCommandTypeSetGlass,
		Flag: enabled,
	})
}

func (f *ProtocolPainter) SetShadow(ox, oy, blur float32) {
	f.shadowOx = ox
	f.shadowOy = oy
	f.shadowBl = blur
	f.commands = append(f.commands, DrawCmdData{
		Type: protocol.DrawCommandTypeSetShadow,
		Val1: ox,
		Val2: oy,
		Val3: blur,
	})
}

func (f *ProtocolPainter) SetOffset(x, y float32) {
	f.offsetX = x
	f.offsetY = y
	f.commands = append(f.commands, DrawCmdData{
		Type: protocol.DrawCommandTypeSetOffset,
		Val1: x,
		Val2: y,
	})
}

func (f *ProtocolPainter) setClipScreen(rScreen image.Rectangle, enabled bool) {
	f.clipEn = enabled
	if !enabled {
		f.commands = append(f.commands, DrawCmdData{
			Type: protocol.DrawCommandTypeSetClip,
			Flag: false,
		})
	} else {
		f.clipX = rScreen.Min.X
		f.clipY = rScreen.Min.Y
		f.clipW = rScreen.Dx()
		f.clipH = rScreen.Dy()
		f.commands = append(f.commands, DrawCmdData{
			Type: protocol.DrawCommandTypeSetClip,
			X1:   rScreen.Min.X,
			Y1:   rScreen.Min.Y,
			W:    rScreen.Dx(),
			H:    rScreen.Dy(),
			Flag: true,
		})
	}
}

func (f *ProtocolPainter) SetClip(r image.Rectangle) {
	if r.Empty() {
		f.setClipScreen(image.Rectangle{}, false)
		return
	}
	rScreen := image.Rect(
		r.Min.X+int(f.offsetX),
		r.Min.Y+int(f.offsetY),
		r.Max.X+int(f.offsetX),
		r.Max.Y+int(f.offsetY),
	)
	f.setClipScreen(rScreen, true)
}

func (f *ProtocolPainter) PushClip(r image.Rectangle) {
	f.clipStack = append(f.clipStack, image.Rect(f.clipX, f.clipY, f.clipX+f.clipW, f.clipY+f.clipH))
	f.clipEnabledStack = append(f.clipEnabledStack, f.clipEn)

	if r.Empty() {
		f.setClipScreen(image.Rectangle{}, true)
		return
	}

	rScreen := image.Rect(
		r.Min.X+int(f.offsetX),
		r.Min.Y+int(f.offsetY),
		r.Max.X+int(f.offsetX),
		r.Max.Y+int(f.offsetY),
	)

	if f.clipEn {
		current := image.Rect(f.clipX, f.clipY, f.clipX+f.clipW, f.clipY+f.clipH)
		f.setClipScreen(current.Intersect(rScreen), true)
	} else {
		f.setClipScreen(rScreen, true)
	}
}

func (f *ProtocolPainter) PopClip() {
	if len(f.clipStack) == 0 {
		f.setClipScreen(image.Rectangle{}, false)
		return
	}

	prevRect := f.clipStack[len(f.clipStack)-1]
	f.clipStack = f.clipStack[:len(f.clipStack)-1]

	prevEnabled := f.clipEnabledStack[len(f.clipEnabledStack)-1]
	f.clipEnabledStack = f.clipEnabledStack[:len(f.clipEnabledStack)-1]

	f.setClipScreen(prevRect, prevEnabled)
}

func (f *ProtocolPainter) Flush() {
	// Immediate translation to drawing queue complete, no-op
}

func (f *ProtocolPainter) ToRenderFrame(width, height int, cursor byte) protocol.RenderFrame {
	commands := make([]protocol.DrawCommand, 0, len(f.commands))
	for _, cmd := range f.commands {
		commands = append(commands, protocol.DrawCommand{
			Type:   cmd.Type,
			X1:     int32(cmd.X1),
			Y1:     int32(cmd.Y1),
			X2:     int32(cmd.X2),
			Y2:     int32(cmd.Y2),
			W:      int32(cmd.W),
			H:      int32(cmd.H),
			Radius: int32(cmd.Radius),
			R:      cmd.R,
			G:      cmd.G,
			B:      cmd.B,
			A:      cmd.A,
			Text:   cmd.Text,
			Bytes:  cmd.Bytes,
			Val1:   cmd.Val1,
			Val2:   cmd.Val2,
			Val3:   cmd.Val3,
			Flag:   cmd.Flag,
		})
	}
	return protocol.RenderFrame{
		Width:    int32(width),
		Height:   int32(height),
		Commands: commands,
		Cursor:   cursor,
	}
}

func (f *ProtocolPainter) Serialize(width, height int, cursor byte) ([]byte, error) {
	return protocol.EncodeRenderFrame(f.ToRenderFrame(width, height, cursor))
}

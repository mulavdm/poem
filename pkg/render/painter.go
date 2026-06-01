package render

import (
	"image"
	"image/color"

	flatbuffers "github.com/google/flatbuffers/go"
	"go_native_gpu_gui/pkg/render/poem"
)

type FlatBufferPainter struct {
	commands []DrawCmdData
	offsetX  float32
	offsetY  float32
	glow     float32
	glass    bool
	shadowOx float32
	shadowOy float32
	shadowBl float32
	clipX    int
	clipY    int
	clipW    int
	clipH    int
	clipEn   bool
}

type DrawCmdData struct {
	Type   poem.DrawCommandType
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
)

func NewFlatBufferPainter() *FlatBufferPainter {
	return &FlatBufferPainter{
		commands: make([]DrawCmdData, 0, 256),
	}
}

func (f *FlatBufferPainter) Reset() {
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
}

func (f *FlatBufferPainter) DrawRoundedRect(r image.Rectangle, radius int, col color.RGBA) {
	f.commands = append(f.commands, DrawCmdData{
		Type:   poem.DrawCommandTypeDrawRoundedRect,
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

func (f *FlatBufferPainter) DrawRaycaster(r image.Rectangle, playerX, playerY, playerAngle float32) {
	f.DrawRaycasterStyled(r, playerX, playerY, playerAngle, color.RGBA{255, 155, 70, 255})
}

func (f *FlatBufferPainter) DrawRaycasterStyled(r image.Rectangle, playerX, playerY, playerAngle float32, accent color.RGBA) {
	f.commands = append(f.commands, DrawCmdData{
		Type:   poem.DrawCommandTypeDrawRoundedRect,
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
	})
}

func (f *FlatBufferPainter) DrawSeed3D(viewportRect image.Rectangle, seedX, seedY, playerX, playerY, playerAngle float32) {
	f.DrawBillboard3D(viewportRect, seedX, seedY, BillboardSeed, color.RGBA{255, 215, 30, 255})
}

func (f *FlatBufferPainter) DrawSentry3D(viewportRect image.Rectangle, sentryX, sentryY, playerX, playerY, playerAngle float32, col color.RGBA) {
	f.DrawBillboard3D(viewportRect, sentryX, sentryY, BillboardSentry, col)
}

func (f *FlatBufferPainter) DrawBillboard3D(viewportRect image.Rectangle, worldX, worldY float32, kind int, col color.RGBA) {
	radius := -998
	switch kind {
	case BillboardSentry:
		radius = -997
	case BillboardStash:
		radius = -996
	case BillboardCue:
		radius = -995
	}

	f.commands = append(f.commands, DrawCmdData{
		Type:   poem.DrawCommandTypeDrawRoundedRect,
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

func (f *FlatBufferPainter) DrawText(text string, x, y int, col color.RGBA) {
	f.commands = append(f.commands, DrawCmdData{
		Type: poem.DrawCommandTypeDrawText,
		X1:   x,
		Y1:   y,
		R:    col.R,
		G:    col.G,
		B:    col.B,
		A:    col.A,
		Text: text,
	})
}

func (f *FlatBufferPainter) FillRect(r image.Rectangle, col color.RGBA) {
	f.commands = append(f.commands, DrawCmdData{
		Type: poem.DrawCommandTypeFillRect,
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

func (f *FlatBufferPainter) DrawLine(x1, y1, x2, y2 int, col color.RGBA) {
	f.commands = append(f.commands, DrawCmdData{
		Type: poem.DrawCommandTypeDrawLine,
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

func (f *FlatBufferPainter) SetGlow(strength float32) {
	f.glow = strength
	f.commands = append(f.commands, DrawCmdData{
		Type: poem.DrawCommandTypeSetGlow,
		Val1: strength,
	})
}

func (f *FlatBufferPainter) SetGlass(enabled bool) {
	f.glass = enabled
	f.commands = append(f.commands, DrawCmdData{
		Type: poem.DrawCommandTypeSetGlass,
		Flag: enabled,
	})
}

func (f *FlatBufferPainter) SetShadow(ox, oy, blur float32) {
	f.shadowOx = ox
	f.shadowOy = oy
	f.shadowBl = blur
	f.commands = append(f.commands, DrawCmdData{
		Type: poem.DrawCommandTypeSetShadow,
		Val1: ox,
		Val2: oy,
		Val3: blur,
	})
}

func (f *FlatBufferPainter) SetOffset(x, y float32) {
	f.offsetX = x
	f.offsetY = y
	f.commands = append(f.commands, DrawCmdData{
		Type: poem.DrawCommandTypeSetOffset,
		Val1: x,
		Val2: y,
	})
}

func (f *FlatBufferPainter) SetClip(r image.Rectangle) {
	if r.Empty() {
		f.clipEn = false
		f.commands = append(f.commands, DrawCmdData{
			Type: poem.DrawCommandTypeSetClip,
			Flag: false,
		})
	} else {
		f.clipEn = true
		f.clipX = r.Min.X
		f.clipY = r.Min.Y
		f.clipW = r.Dx()
		f.clipH = r.Dy()
		f.commands = append(f.commands, DrawCmdData{
			Type: poem.DrawCommandTypeSetClip,
			X1:   r.Min.X,
			Y1:   r.Min.Y,
			W:    r.Dx(),
			H:    r.Dy(),
			Flag: true,
		})
	}
}

func (f *FlatBufferPainter) Flush() {
	// Immediate translation to drawing queue complete, no-op
}

func (f *FlatBufferPainter) Serialize(builder *flatbuffers.Builder, width, height int, cursor byte) flatbuffers.UOffsetT {
	// 1. Create string offsets first (flatbuffers forbids nested builder tables)
	textOffsets := make([]flatbuffers.UOffsetT, len(f.commands))
	for i, cmd := range f.commands {
		if cmd.Text != "" {
			textOffsets[i] = builder.CreateString(cmd.Text)
		}
	}

	// 2. Build DrawCommand tables
	cmdOffsets := make([]flatbuffers.UOffsetT, len(f.commands))
	for i, cmd := range f.commands {
		poem.DrawCommandStart(builder)
		poem.DrawCommandAddType(builder, cmd.Type)
		poem.DrawCommandAddX1(builder, int32(cmd.X1))
		poem.DrawCommandAddY1(builder, int32(cmd.Y1))
		poem.DrawCommandAddX2(builder, int32(cmd.X2))
		poem.DrawCommandAddY2(builder, int32(cmd.Y2))
		poem.DrawCommandAddW(builder, int32(cmd.W))
		poem.DrawCommandAddH(builder, int32(cmd.H))
		poem.DrawCommandAddRadius(builder, int32(cmd.Radius))
		poem.DrawCommandAddR(builder, cmd.R)
		poem.DrawCommandAddG(builder, cmd.G)
		poem.DrawCommandAddB(builder, cmd.B)
		poem.DrawCommandAddA(builder, cmd.A)
		if textOffsets[i] != 0 {
			poem.DrawCommandAddText(builder, textOffsets[i])
		}
		poem.DrawCommandAddVal1(builder, cmd.Val1)
		poem.DrawCommandAddVal2(builder, cmd.Val2)
		poem.DrawCommandAddVal3(builder, cmd.Val3)
		poem.DrawCommandAddFlag(builder, cmd.Flag)
		cmdOffsets[i] = poem.DrawCommandEnd(builder)
	}

	// 3. Build commands vector
	poem.RenderFrameStartCommandsVector(builder, len(cmdOffsets))
	for i := len(cmdOffsets) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(cmdOffsets[i])
	}
	commandsVector := builder.EndVector(len(cmdOffsets))

	// 4. Build RenderFrame
	poem.RenderFrameStart(builder)
	poem.RenderFrameAddWidth(builder, int32(width))
	poem.RenderFrameAddHeight(builder, int32(height))
	poem.RenderFrameAddCommands(builder, commandsVector)
	poem.RenderFrameAddCursor(builder, int8(cursor))
	frameOffset := poem.RenderFrameEnd(builder)

	// 5. Wrap in GoToRustMessage envelope
	poem.GoToRustMessageStart(builder)
	poem.GoToRustMessageAddMessageType(builder, poem.GoToRustUnionRenderFrame)
	poem.GoToRustMessageAddMessage(builder, frameOffset)
	return poem.GoToRustMessageEnd(builder)
}

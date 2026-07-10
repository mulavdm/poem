package components

import (
	"image"
	"image/color"
	"testing"

	"go_native_gpu_gui/pkg/render/theme"
	"go_native_gpu_gui/pkg/render/types"
)

type recordingPainter struct {
	glowCalls []float32
}

func (p *recordingPainter) DrawRoundedRect(image.Rectangle, int, color.RGBA) {}
func (p *recordingPainter) DrawText(string, int, int, color.RGBA)            {}
func (p *recordingPainter) FillRect(image.Rectangle, color.RGBA)             {}
func (p *recordingPainter) DrawLine(int, int, int, int, color.RGBA)          {}
func (p *recordingPainter) DrawImage(image.Rectangle, int, int, []byte)      {}
func (p *recordingPainter) DrawRaycaster(image.Rectangle, float32, float32, float32) {
}
func (p *recordingPainter) DrawRaycasterStyled(image.Rectangle, float32, float32, float32, color.RGBA) {
}
func (p *recordingPainter) DrawRaycasterMapStyled(image.Rectangle, float32, float32, float32, color.RGBA, string) {
}
func (p *recordingPainter) DrawBillboard3D(image.Rectangle, float32, float32, int, color.RGBA) {
}
func (p *recordingPainter) DrawSeed3D(image.Rectangle, float32, float32, float32, float32, float32) {
}
func (p *recordingPainter) DrawSentry3D(image.Rectangle, float32, float32, float32, float32, float32, color.RGBA) {
}
func (p *recordingPainter) SetGlow(strength float32) { p.glowCalls = append(p.glowCalls, strength) }
func (p *recordingPainter) SetGlass(bool)            {}
func (p *recordingPainter) SetShadow(float32, float32, float32) {
}
func (p *recordingPainter) SetOffset(float32, float32) {}
func (p *recordingPainter) SetClip(image.Rectangle)    {}
func (p *recordingPainter) PushClip(image.Rectangle)   {}
func (p *recordingPainter) PopClip()                   {}
func (p *recordingPainter) Flush()                     {}

func professionalState() *types.ApplicationState {
	return &types.ApplicationState{
		ThemeManager:    theme.NewManager(theme.ModernDark()),
		FontCharWidth:   8,
		ScrollPositions: make(map[string]int),
		ScrollCurrent:   make(map[string]float64),
	}
}

func TestProfessionalConstructorsDoNotEmitGlowByDefault(t *testing.T) {
	state := professionalState()
	button := NewButton("save", "Save", nil)
	button.Rect = image.Rect(0, 0, 120, 36)
	state.HoveredID = "save"

	painter := &recordingPainter{}
	button.Draw(painter, state)

	if len(painter.glowCalls) > 0 {
		t.Fatalf("theme-native button emitted glow commands: %v", painter.glowCalls)
	}
}

func TestThemeNativeScrollbarDoesNotEmitGlowWhenHovered(t *testing.T) {
	state := professionalState()
	state.MouseX = 196
	state.MouseY = 20

	content := NewPanel("content")
	content.Rect = image.Rect(0, 0, 180, 320)

	scroll := NewScrollView("scroll", content)
	scroll.Rect = image.Rect(0, 0, 200, 120)
	scroll.ScrollbarW = 10
	scroll.ContentH = 320

	painter := &recordingPainter{}
	scroll.Draw(painter, state)

	if len(painter.glowCalls) > 0 {
		t.Fatalf("theme-native scrollbar emitted glow commands: %v", painter.glowCalls)
	}
}

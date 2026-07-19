package render

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/protocol"
)

func TestProtocolPainterPlacesMapSceneWithFallback(t *testing.T) {
	painter := NewProtocolPainter()
	fallback := []byte{1, 2, 3, 4}
	painter.DrawMapScene(image.Rect(10, 20, 310, 220), "route-map", 64, 32, fallback, 1)
	fallback[0] = 9

	frame := painter.ToRenderFrame(800, 600, 0)
	if len(frame.Commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(frame.Commands))
	}
	command := frame.Commands[0]
	if command.Type != protocol.DrawCommandTypeDrawMapScene || command.Text != "route-map" || command.X1 != 10 || command.Y1 != 20 || command.X2 != 310 || command.Y2 != 220 || command.W != 64 || command.H != 32 {
		t.Fatalf("map command = %+v", command)
	}
	if len(command.Bytes) != 4 || command.Bytes[0] != 1 {
		t.Fatalf("fallback was not retained independently: %v", command.Bytes)
	}
}

func TestImageViewportAppliesPreviewTransformToRetainedMapPlacement(t *testing.T) {
	painter := NewProtocolPainter()
	view := &ImageViewport{CompID: "map", MapViewportID: "route-map", Rect: image.Rect(0, 0, 200, 100), Transform: ImageTransform{OffsetX: .1, OffsetY: -.2, Scale: 2}}
	view.Draw(painter, &ApplicationState{})
	frame := painter.ToRenderFrame(200, 100, 0)
	var mapCommand *protocol.DrawCommand
	for index := range frame.Commands {
		if frame.Commands[index].Type == protocol.DrawCommandTypeDrawMapScene {
			mapCommand = &frame.Commands[index]
		}
	}
	if mapCommand == nil {
		t.Fatal("map command missing")
	}
	if mapCommand.X1 != -80 || mapCommand.Y1 != -70 || mapCommand.X2 != 320 || mapCommand.Y2 != 130 {
		t.Fatalf("transformed map rect = (%d,%d)-(%d,%d)", mapCommand.X1, mapCommand.Y1, mapCommand.X2, mapCommand.Y2)
	}
	if mapCommand.Val1 != 2 {
		t.Fatalf("preview scale = %v, want 2", mapCommand.Val1)
	}
}

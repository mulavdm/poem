package components_test

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render"
	"github.com/mulavdm/poem/pkg/render/components"
	"github.com/mulavdm/poem/pkg/render/protocol"
	"github.com/mulavdm/poem/pkg/render/types"
)

func TestRealtimeViewportLayoutFocusAndCommand(t *testing.T) {
	v := &components.RealtimeViewport{CompID: "scene_viewport", Rect: image.Rect(100, 50, 900, 650), Label: "Scene viewport", Command: []byte("pick")}
	state := &types.ApplicationState{}
	if v.HitTest(image.Pt(200, 200)) != "scene_viewport" || !v.OnMouseDown(image.Pt(200, 200), state) || state.FocusedID != "scene_viewport" {
		t.Fatal("viewport did not participate in hit testing and focus")
	}
	p := render.NewProtocolPainter()
	v.Draw(p, state)
	frame := p.ToRenderFrame(1000, 700, 0)
	found := false
	for _, command := range frame.Commands {
		if command.Type == protocol.DrawCommandTypeDrawRealtimeViewport {
			found = true
			if command.X1 != 100 || command.Y1 != 50 || command.W != 800 || command.H != 600 || string(command.Bytes) != "pick" {
				t.Fatalf("bad viewport command: %+v", command)
			}
			if !command.Flag {
				t.Fatal("focused viewport flag missing")
			}
		}
	}
	if !found {
		t.Fatal("missing realtime viewport draw command")
	}
	if v.Semantics(state).Name != "Scene viewport" {
		t.Fatal("missing accessible name")
	}
	called := false
	v.OnEvent = func(payload []byte, _ *types.ApplicationState) bool {
		called = string(payload) == "event"
		return called
	}
	if !v.OnRealtimeViewportEvent([]byte("event"), state) || !called {
		t.Fatal("opaque event was not routed")
	}
	var phases []string
	v.OnPointer = func(kind string, _, _ float64, _ *types.ApplicationState) { phases = append(phases, kind) }
	if !v.OnMouseDown(image.Pt(200, 200), state) || !v.OnMouseMove(image.Pt(300, 250), state) || !v.OnMouseUp(image.Pt(950, 700), state) {
		t.Fatal("viewport pointer capture did not survive an outside release")
	}
	if len(phases) != 3 || phases[0] != "down" || phases[1] != "move" || phases[2] != "cancel" {
		t.Fatalf("pointer phases %v", phases)
	}
}

package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/types"
)

func TestCheckboxControlledChange(t *testing.T) {
	state := &types.ApplicationState{}
	var got bool
	box := NewCheckbox("remember", "Remember", false, func(next bool, _ *types.ApplicationState) { got = next })
	box.Rect = image.Rect(0, 0, 100, 36)
	state.ActiveID = box.CompID
	if !box.OnMouseUp(image.Pt(10, 10), state) || !got {
		t.Fatal("checkbox did not publish controlled value")
	}
	if box.Checked {
		t.Fatal("controlled checkbox mutated its own value")
	}
}

func TestDisabledSwitchCannotToggle(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "network"}
	control := NewSwitch("network", "Network", false, nil)
	control.Disabled = true
	if control.OnKey(32, 0, state) || control.Checked {
		t.Fatal("disabled switch toggled")
	}
}

func TestRadioPublishesValue(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "choice"}
	var got string
	radio := NewRadio("choice", "Choice", "a", false, func(value string, _ *types.ApplicationState) { got = value })
	if !radio.OnKey(13, 0, state) || got != "a" {
		t.Fatalf("radio selection = %q", got)
	}
}

func TestPillRadiusClampsToControlGeometry(t *testing.T) {
	if got := roundedRadius(image.Rect(0, 0, 44, 20), 999); got != 10 {
		t.Fatalf("pill radius = %d", got)
	}
}

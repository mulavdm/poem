package components

import (
	"testing"

	"github.com/mulavdm/poem/pkg/render/types"
)

func TestControlledSliderDoesNotRestoreLegacyApplicationValue(t *testing.T) {
	state := &types.ApplicationState{FocusedID: "quality", SliderValues: map[string]float32{"quality": 12}}
	changed := float32(0)
	slider := NewSlider("quality", 0, 100, 77, func(value float32, _ *types.ApplicationState) { changed = value })
	if !slider.OnKey(0x27, 0, state) {
		t.Fatal("controlled slider rejected keyboard change")
	}
	if changed != slider.Value || state.SliderValues["quality"] != 12 {
		t.Fatalf("controlled value=%v legacy=%v", slider.Value, state.SliderValues["quality"])
	}
}

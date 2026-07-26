package components

import (
	"image"
	"reflect"
	"testing"

	"github.com/mulavdm/poem/pkg/render/types"
)

func TestButtonDragLifecycleRetainsPointerCapture(t *testing.T) {
	state := &types.ApplicationState{}
	var phases []string
	button := NewButton("row", "Row", nil)
	button.SetBounds(image.Rect(0, 0, 100, 30))
	button.OnDrag = func(phase string, _ image.Point, _ *types.ApplicationState) {
		phases = append(phases, phase)
	}
	if !button.OnMouseDown(image.Pt(10, 10), state) ||
		!button.OnMouseMove(image.Pt(120, 40), state) ||
		!button.OnMouseUp(image.Pt(120, 40), state) {
		t.Fatal("drag lifecycle did not retain pointer capture")
	}
	if want := []string{"begin", "update", "end"}; !reflect.DeepEqual(phases, want) {
		t.Fatalf("drag phases %v, want %v", phases, want)
	}
}

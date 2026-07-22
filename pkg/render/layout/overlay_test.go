package layout

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/types"
)

// hitComponent is a testComponent that reports its id from HitTest/OnMouseDown
// whenever the point falls inside its bounds, so overlay z-ordering is testable.
type hitComponent struct {
	testComponent
	downID *string
}

func (c *hitComponent) HitTest(pt image.Point) string {
	if pt.In(c.rect) {
		return c.id
	}
	return ""
}

func (c *hitComponent) OnMouseDown(pt image.Point, _ *types.ApplicationState) bool {
	if pt.In(c.rect) {
		if c.downID != nil {
			*c.downID = c.id
		}
		return true
	}
	return false
}

func TestOverlayBaseFillsBoxAndLayerAnchorsInset(t *testing.T) {
	base := &hitComponent{testComponent: testComponent{id: "map"}}
	control := &hitComponent{testComponent: testComponent{id: "zoom", measured: &types.MeasureResult{Preferred: image.Pt(40, 120)}}}
	o := &Overlay{
		CompID: "overlay",
		Base:   base,
		Layers: []OverlayLayer{{Anchor: AnchorTopRight, Inset: 12, Content: control}},
	}
	o.SetBounds(image.Rect(0, 0, 800, 600))

	if got := base.Bounds(); got != image.Rect(0, 0, 800, 600) {
		t.Fatalf("base should fill the box, got %v", got)
	}
	// Top-right, inset 12: right edge at 800-12=788, top edge at 12.
	want := image.Rect(788-40, 12, 788, 12+120)
	if got := control.Bounds(); got != want {
		t.Fatalf("control anchor: got %v want %v", got, want)
	}
}

func TestOverlayLayerWinsHitTestOverBase(t *testing.T) {
	base := &hitComponent{testComponent: testComponent{id: "map"}}
	control := &hitComponent{testComponent: testComponent{id: "zoom", measured: &types.MeasureResult{Preferred: image.Pt(40, 120)}}}
	o := &Overlay{CompID: "overlay", Base: base, Layers: []OverlayLayer{{Anchor: AnchorTopRight, Inset: 12, Content: control}}}
	o.SetBounds(image.Rect(0, 0, 800, 600))

	// A point over the floating control must resolve to the control, not the map
	// beneath it.
	over := image.Pt(770, 40)
	if id := o.HitTest(over); id != "zoom" {
		t.Fatalf("overlay layer should win hit-test, got %q", id)
	}
	var pressed string
	control.downID, base.downID = &pressed, &pressed
	if !o.OnMouseDown(over, nil) || pressed != "zoom" {
		t.Fatalf("mouse-down should reach the control, got %q", pressed)
	}
	// A point on the bare canvas falls through to the base.
	if id := o.HitTest(image.Pt(100, 500)); id != "map" {
		t.Fatalf("bare canvas should hit the base, got %q", id)
	}
}

func TestOverlayMeasureIgnoresLayers(t *testing.T) {
	base := &hitComponent{testComponent: testComponent{id: "map", measured: &types.MeasureResult{Preferred: image.Pt(640, 480)}}}
	huge := &hitComponent{testComponent: testComponent{id: "big", measured: &types.MeasureResult{Preferred: image.Pt(9999, 9999)}}}
	o := &Overlay{CompID: "overlay", Base: base, Layers: []OverlayLayer{{Anchor: AnchorCenter, Content: huge}}}
	if got := o.Measure(image.Pt(1000, 1000), nil).Preferred; got != image.Pt(640, 480) {
		t.Fatalf("overlay must measure to base only, got %v", got)
	}
}

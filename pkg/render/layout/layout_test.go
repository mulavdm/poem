package layout

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/types"
)

type testComponent struct {
	id       string
	rect     image.Rectangle
	measured *types.MeasureResult
}

func (c *testComponent) ID() string              { return c.id }
func (c *testComponent) GetID() string           { return c.id }
func (c *testComponent) Bounds() image.Rectangle { return c.rect }
func (c *testComponent) SetBounds(r image.Rectangle) {
	c.rect = r
}
func (c *testComponent) Draw(types.Painter, *types.ApplicationState)           {}
func (c *testComponent) HitTest(image.Point) string                            { return "" }
func (c *testComponent) OnKey(uint32, rune, *types.ApplicationState) bool      { return false }
func (c *testComponent) OnMouseDown(image.Point, *types.ApplicationState) bool { return false }
func (c *testComponent) OnMouseUp(image.Point, *types.ApplicationState) bool   { return false }
func (c *testComponent) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (c *testComponent) Focusable() bool                                       { return false }
func (c *testComponent) Walk(fn func(types.Component))                         { fn(c) }
func (c *testComponent) Measure(image.Point, *types.ApplicationState) types.MeasureResult {
	if c.measured != nil {
		return *c.measured
	}
	return types.MeasureResult{}
}

func TestFlexBoxUsesMeasureWhenAvailable(t *testing.T) {
	child := &testComponent{
		id:   "child",
		rect: image.Rect(0, 0, 400, 20),
		measured: &types.MeasureResult{
			Preferred: image.Pt(120, 36),
			Min:       image.Pt(80, 36),
		},
	}

	flex := &FlexBox{
		Rect:           image.Rect(0, 0, 200, 80),
		Direction:      Vertical,
		AlignItems:     AlignStart,
		JustifyContent: JustifyStart,
		Children:       []types.Component{child},
	}

	flex.SetBounds(flex.Rect)

	if got := child.Bounds().Dx(); got != 120 {
		t.Fatalf("expected measured width 120, got %d", got)
	}
	if got := child.Bounds().Dy(); got != 36 {
		t.Fatalf("expected measured height 36, got %d", got)
	}
}

func TestFlexBoxFallsBackToBoundsForLegacyComponents(t *testing.T) {
	child := &legacyComponent{
		id:   "legacy",
		rect: image.Rect(0, 0, 90, 28),
	}

	flex := &FlexBox{
		Rect:           image.Rect(0, 0, 200, 80),
		Direction:      Vertical,
		AlignItems:     AlignStart,
		JustifyContent: JustifyStart,
		Children:       []types.Component{child},
	}

	flex.SetBounds(flex.Rect)

	if got := child.Bounds().Dx(); got != 90 {
		t.Fatalf("expected fallback width 90, got %d", got)
	}
	if got := child.Bounds().Dy(); got != 28 {
		t.Fatalf("expected fallback height 28, got %d", got)
	}
}

func TestFlexBoxClampsOversizedMeasuredWidth(t *testing.T) {
	left := &testComponent{
		id:   "left",
		rect: image.Rect(0, 0, 999, 30),
		measured: &types.MeasureResult{
			Preferred: image.Pt(999, 30),
			Min:       image.Pt(80, 30),
		},
	}
	right := &testComponent{
		id:   "right",
		rect: image.Rect(0, 0, 80, 30),
		measured: &types.MeasureResult{
			Preferred: image.Pt(80, 30),
			Min:       image.Pt(80, 30),
		},
	}

	flex := &FlexBox{
		Rect:           image.Rect(0, 0, 300, 80),
		Direction:      Horizontal,
		AlignItems:     AlignStart,
		JustifyContent: JustifyStart,
		Padding:        10,
		Gap:            10,
		Children:       []types.Component{left, right},
	}

	flex.SetBounds(flex.Rect)

	if left.Bounds().Dx() > 280 {
		t.Fatalf("expected left child to be clamped to available width, got %d", left.Bounds().Dx())
	}
	if right.Bounds().Min.X < left.Bounds().Max.X {
		t.Fatalf("expected right child to remain placed after left child, got left=%v right=%v", left.Bounds(), right.Bounds())
	}
}

func TestFlexBoxVerticalLayoutRemainsStable(t *testing.T) {
	top := &testComponent{id: "top", measured: &types.MeasureResult{Preferred: image.Pt(100, 40), Min: image.Pt(100, 40)}}
	bottom := &testComponent{id: "bottom", measured: &types.MeasureResult{Preferred: image.Pt(100, 50), Min: image.Pt(100, 50)}}

	flex := &FlexBox{
		Rect:           image.Rect(0, 0, 220, 200),
		Direction:      Vertical,
		AlignItems:     AlignStretch,
		JustifyContent: JustifyStart,
		Padding:        12,
		Gap:            8,
		Children:       []types.Component{top, bottom},
	}

	flex.SetBounds(flex.Rect)

	if top.Bounds().Min.Y != 12 {
		t.Fatalf("expected top child to start at padding, got %v", top.Bounds())
	}
	if bottom.Bounds().Min.Y != top.Bounds().Max.Y+8 {
		t.Fatalf("expected vertical gap of 8, got top=%v bottom=%v", top.Bounds(), bottom.Bounds())
	}
	if top.Bounds().Dx() != 196 || bottom.Bounds().Dx() != 196 {
		t.Fatalf("expected stretched widths 196, got top=%d bottom=%d", top.Bounds().Dx(), bottom.Bounds().Dx())
	}
}

func TestFlexBoxWrapsHorizontalChildren(t *testing.T) {
	a := &testComponent{id: "a", measured: &types.MeasureResult{Preferred: image.Pt(120, 30), Min: image.Pt(120, 30)}}
	b := &testComponent{id: "b", measured: &types.MeasureResult{Preferred: image.Pt(120, 30), Min: image.Pt(120, 30)}}
	c := &testComponent{id: "c", measured: &types.MeasureResult{Preferred: image.Pt(120, 30), Min: image.Pt(120, 30)}}

	flex := &FlexBox{
		Rect:           image.Rect(0, 0, 280, 200),
		Direction:      Horizontal,
		Wrap:           true,
		Gap:            10,
		LineGap:        12,
		JustifyContent: JustifyStart,
		AlignItems:     AlignStart,
		Padding:        8,
		Children:       []types.Component{a, b, c},
	}

	flex.SetBounds(flex.Rect)

	if b.Bounds().Min.Y != a.Bounds().Min.Y {
		t.Fatalf("expected first two children on same row, got a=%v b=%v", a.Bounds(), b.Bounds())
	}
	if c.Bounds().Min.Y <= a.Bounds().Min.Y {
		t.Fatalf("expected third child to wrap to next row, got a=%v c=%v", a.Bounds(), c.Bounds())
	}
}

func TestFlexBoxWrappedMeasureReflectsMultipleRows(t *testing.T) {
	a := &testComponent{id: "a", measured: &types.MeasureResult{Preferred: image.Pt(120, 30), Min: image.Pt(120, 30)}}
	b := &testComponent{id: "b", measured: &types.MeasureResult{Preferred: image.Pt(120, 30), Min: image.Pt(120, 30)}}
	c := &testComponent{id: "c", measured: &types.MeasureResult{Preferred: image.Pt(120, 30), Min: image.Pt(120, 30)}}

	flex := &FlexBox{
		Direction: Horizontal,
		Wrap:      true,
		Gap:       10,
		LineGap:   12,
		Padding:   8,
		Children:  []types.Component{a, b, c},
	}

	result := flex.Measure(image.Pt(280, 0), nil)
	if result.Preferred.Y <= 60 {
		t.Fatalf("expected wrapped measure to include multiple rows, got %v", result.Preferred)
	}
}

func TestFlexBoxContentSizeIgnoresAssignedHeight(t *testing.T) {
	a := &testComponent{id: "a", measured: &types.MeasureResult{Preferred: image.Pt(100, 40), Min: image.Pt(100, 40)}}
	b := &testComponent{id: "b", measured: &types.MeasureResult{Preferred: image.Pt(100, 40), Min: image.Pt(100, 40)}}
	c := &testComponent{id: "c", measured: &types.MeasureResult{Preferred: image.Pt(100, 40), Min: image.Pt(100, 40)}}

	flex := &FlexBox{
		Rect:      image.Rect(0, 0, 160, 60),
		Direction: Vertical,
		Gap:       10,
		Padding:   4,
		Children:  []types.Component{a, b, c},
	}

	measured := flex.Measure(image.Pt(160, 60), nil)
	content := flex.ContentSize(image.Pt(160, 60), nil)

	if measured.Preferred.Y != 60 {
		t.Fatalf("expected regular measure to honor assigned height, got %v", measured.Preferred)
	}
	if content.Y <= measured.Preferred.Y {
		t.Fatalf("expected content size to expose overflow, got content=%v measured=%v", content, measured.Preferred)
	}
}

type legacyComponent struct {
	id   string
	rect image.Rectangle
}

func (c *legacyComponent) ID() string                                       { return c.id }
func (c *legacyComponent) GetID() string                                    { return c.id }
func (c *legacyComponent) Bounds() image.Rectangle                          { return c.rect }
func (c *legacyComponent) SetBounds(r image.Rectangle)                      { c.rect = r }
func (c *legacyComponent) Draw(types.Painter, *types.ApplicationState)      {}
func (c *legacyComponent) HitTest(image.Point) string                       { return "" }
func (c *legacyComponent) OnKey(uint32, rune, *types.ApplicationState) bool { return false }
func (c *legacyComponent) OnMouseDown(image.Point, *types.ApplicationState) bool {
	return false
}
func (c *legacyComponent) OnMouseUp(image.Point, *types.ApplicationState) bool   { return false }
func (c *legacyComponent) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (c *legacyComponent) Focusable() bool                                       { return false }
func (c *legacyComponent) Walk(fn func(types.Component))                         { fn(c) }

package layout

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/types"
)

type mockComponent struct {
	id     string
	rect   image.Rectangle
	pref   image.Point
	bounds image.Rectangle
}

func (m *mockComponent) ID() string                                      { return m.id }
func (m *mockComponent) GetID() string                                   { return m.id }
func (m *mockComponent) Bounds() image.Rectangle                         { return m.bounds }
func (m *mockComponent) SetBounds(r image.Rectangle)                     { m.bounds = r }
func (m *mockComponent) Draw(p types.Painter, s *types.ApplicationState) {}
func (m *mockComponent) Measure(avail image.Point, s *types.ApplicationState) types.MeasureResult {
	return types.MeasureResult{Preferred: m.pref, Min: m.pref}
}
func (m *mockComponent) HitTest(pt image.Point) string                               { return "" }
func (m *mockComponent) Focusable() bool                                             { return false }
func (m *mockComponent) Walk(fn func(types.Component))                               { fn(m) }
func (m *mockComponent) OnKey(key uint32, char rune, s *types.ApplicationState) bool { return false }
func (m *mockComponent) OnMouseDown(pt image.Point, s *types.ApplicationState) bool  { return false }
func (m *mockComponent) OnMouseUp(pt image.Point, s *types.ApplicationState) bool    { return false }
func (m *mockComponent) OnMouseMove(pt image.Point, s *types.ApplicationState) bool  { return false }

func TestGridSizingFixedAndStar(t *testing.T) {
	c1 := &mockComponent{id: "c1", pref: image.Pt(50, 40)}
	c2 := &mockComponent{id: "c2", pref: image.Pt(80, 60)}

	grid := &Grid{
		CompID: "grid1",
		Rect:   image.Rect(0, 0, 300, 200),
		Columns: []GridLength{
			NewPixel(100), // Col 0: 100px
			NewStar(1),    // Col 1: remaining (300 - 100 = 200px)
		},
		Rows: []GridLength{
			NewPixel(50), // Row 0: 50px
			NewStar(1),   // Row 1: remaining (200 - 50 = 150px)
		},
		Padding: 0,
		Children: []GridChild{
			{Row: 0, Col: 0, Child: c1, Horizontal: Stretch, Vertical: Stretch},
			{Row: 1, Col: 1, Child: c2, Horizontal: Stretch, Vertical: Stretch},
		},
	}

	grid.SetBounds(image.Rect(0, 0, 300, 200))

	// Col 0 left/right should be [0, 100]
	// Col 1 left/right should be [100, 300]
	// Row 0 top/bottom should be [0, 50]
	// Row 1 top/bottom should be [50, 200]

	if c1.Bounds() != image.Rect(0, 0, 100, 50) {
		t.Errorf("c1 expected bounds (0,0,100,50), got %v", c1.Bounds())
	}
	if c2.Bounds() != image.Rect(100, 50, 300, 200) {
		t.Errorf("c2 expected bounds (100,50,300,200), got %v", c2.Bounds())
	}
}

func TestGridSizingAuto(t *testing.T) {
	c1 := &mockComponent{id: "c1", pref: image.Pt(80, 40)}
	c2 := &mockComponent{id: "c2", pref: image.Pt(110, 70)}

	grid := &Grid{
		CompID: "grid_auto",
		Columns: []GridLength{
			NewAuto(),
			NewPixel(50),
		},
		Rows: []GridLength{
			NewAuto(),
			NewPixel(30),
		},
		Children: []GridChild{
			{Row: 0, Col: 0, Child: c1, Horizontal: Stretch, Vertical: Stretch},
			{Row: 0, Col: 1, Child: c2, Horizontal: Stretch, Vertical: Stretch},
		},
	}

	// Measuring the grid:
	// Col 0 auto width should be max preferred width of children in col 0 (c1) = 80px
	// Col 1 width is fixed 50px
	// Total measured width = 80 + 50 = 130px
	// Row 0 auto height should be max preferred height of children in row 0 (c1/c2) = 70px
	// Row 1 height is fixed 30px
	// Total measured height = 70 + 30 = 100px

	measurement := grid.Measure(image.Point{}, nil)
	if measurement.Preferred.X != 130 {
		t.Errorf("expected preferred width 130, got %d", measurement.Preferred.X)
	}
	if measurement.Preferred.Y != 100 {
		t.Errorf("expected preferred height 100, got %d", measurement.Preferred.Y)
	}
}

func TestGridChildAlignment(t *testing.T) {
	c1 := &mockComponent{id: "c1", pref: image.Pt(50, 40)}

	grid := &Grid{
		CompID: "grid_align",
		Rect:   image.Rect(0, 0, 200, 200),
		Columns: []GridLength{
			NewPixel(200),
		},
		Rows: []GridLength{
			NewPixel(200),
		},
		Children: []GridChild{
			{Row: 0, Col: 0, Child: c1, Horizontal: Center, Vertical: End},
		},
	}

	grid.SetBounds(image.Rect(0, 0, 200, 200))

	// Cell is [0, 0, 200, 200]
	// c1 size is 50x40
	// Horizontal = Center => x = 0 + (200 - 50)/2 = 75
	// Vertical = End => y = 200 - 40 = 160
	// Bounds should be [75, 160, 125, 200]

	expected := image.Rect(75, 160, 125, 200)
	if c1.Bounds() != expected {
		t.Errorf("expected alignment bounds %v, got %v", expected, c1.Bounds())
	}
}

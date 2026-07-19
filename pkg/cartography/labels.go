package cartography

import (
	"context"
	"errors"
	"math"
	"sort"
)

const (
	maxLabelCandidates = 50_000
	maxLabelPlacements = 16_384
	labelGridCellSize  = 64.0
	maxLabelDimension  = 8192.0
)

// LabelCandidate is immutable style output waiting for screen-space collision.
// WorldX and WorldY are normalized Web-Mercator coordinates.
type LabelCandidate struct {
	ID           string
	Text         string
	Locale       string
	WorldX       float64
	WorldY       float64
	Size         float64
	Priority     int32
	AllowOverlap bool
}

// LabelBounds is an axis-aligned logical-pixel collision box.
type LabelBounds struct{ Left, Top, Right, Bottom float32 }

// LabelPlacement is a shaped, accepted screen-space label. Glyph order and
// offsets are retained exactly as returned by the shared OpenType shaper.
type LabelPlacement struct {
	Candidate LabelCandidate
	Shaped    ShapedText
	AnchorX   float32
	AnchorY   float32
	Bounds    LabelBounds
}

// LimitLabelCandidates applies a deterministic density and quality cap before
// shaping. It never mutates the published candidate slice.
func LimitLabelCandidates(candidates []LabelCandidate, density float64, maximum int) ([]LabelCandidate, error) {
	if !finite(density) || density < 0 || density > 4 || maximum < 0 || maximum > maxLabelPlacements || len(candidates) > maxLabelCandidates {
		return nil, errors.New("cartography: invalid label density")
	}
	limit := min(maximum, int(math.Round(4096*density)))
	if limit <= 0 || len(candidates) == 0 {
		return nil, nil
	}
	ordered := append([]LabelCandidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority != ordered[j].Priority {
			return ordered[i].Priority > ordered[j].Priority
		}
		return ordered[i].ID < ordered[j].ID
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	return ordered, nil
}

// PlaceLabels shapes and deterministically collides label candidates. It is
// intentionally independent of a GPU backend and compiles unchanged to WASM.
func PlaceLabels(ctx context.Context, camera Camera, shaper *TextShaper, candidates []LabelCandidate) ([]LabelPlacement, error) {
	if err := camera.Validate(); err != nil {
		return nil, err
	}
	if ctx == nil || shaper == nil || len(candidates) > maxLabelCandidates {
		return nil, errors.New("cartography: invalid label placement input")
	}
	ordered := append([]LabelCandidate(nil), candidates...)
	for _, candidate := range ordered {
		if candidate.ID == "" || len(candidate.ID) > 1024 || candidate.Text == "" || len(candidate.Text) > 16<<10 ||
			!finite(candidate.WorldX) || !finite(candidate.WorldY) || candidate.WorldX < 0 || candidate.WorldX > 1 || candidate.WorldY < 0 || candidate.WorldY > 1 ||
			!finite(candidate.Size) || candidate.Size < 6 || candidate.Size > 256 || len(candidate.Locale) > 128 {
			return nil, errors.New("cartography: invalid label candidate")
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority != ordered[j].Priority {
			return ordered[i].Priority > ordered[j].Priority
		}
		return ordered[i].ID < ordered[j].ID
	})
	grid := make(map[[2]int][]LabelBounds)
	placements := make([]LabelPlacement, 0, min(len(ordered), maxLabelPlacements))
	for _, candidate := range ordered {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		anchor, projectErr := ProjectWorldPoint(camera, candidate.WorldX, candidate.WorldY, 0)
		if projectErr != nil {
			return nil, projectErr
		}
		if !anchor.Visible {
			continue
		}
		anchorX, anchorY := anchor.X, anchor.Y
		shaped, err := shaper.Shape(candidate.Text, candidate.Locale, candidate.Size)
		if err != nil {
			return nil, err
		}
		width := math.Abs(float64(shaped.Advance))
		height := float64(shaped.Ascent - shaped.Descent)
		if height <= 0 {
			height = candidate.Size
		}
		if !finite(width) || !finite(height) || width > maxLabelDimension || height > maxLabelDimension {
			return nil, errors.New("cartography: shaped label exceeds bounds")
		}
		bounds := LabelBounds{Left: float32(anchorX - width/2 - 2), Top: float32(anchorY - height/2 - 2), Right: float32(anchorX + width/2 + 2), Bottom: float32(anchorY + height/2 + 2)}
		if bounds.Right < 0 || bounds.Bottom < 0 || bounds.Left > float32(camera.Width) || bounds.Top > float32(camera.Height) {
			continue
		}
		cells := labelCells(bounds)
		collides := false
		if !candidate.AllowOverlap {
			for _, cell := range cells {
				for _, occupied := range grid[cell] {
					if labelBoundsOverlap(bounds, occupied) {
						collides = true
						break
					}
				}
				if collides {
					break
				}
			}
		}
		if collides {
			continue
		}
		placements = append(placements, LabelPlacement{Candidate: candidate, Shaped: shaped, AnchorX: float32(anchorX), AnchorY: float32(anchorY), Bounds: bounds})
		if len(placements) > maxLabelPlacements {
			return nil, errors.New("cartography: label placements exceed bounds")
		}
		if !candidate.AllowOverlap {
			for _, cell := range cells {
				grid[cell] = append(grid[cell], bounds)
			}
		}
	}
	return placements, nil
}

func labelCells(bounds LabelBounds) [][2]int {
	left := int(math.Floor(float64(bounds.Left) / labelGridCellSize))
	right := int(math.Floor(float64(bounds.Right) / labelGridCellSize))
	top := int(math.Floor(float64(bounds.Top) / labelGridCellSize))
	bottom := int(math.Floor(float64(bounds.Bottom) / labelGridCellSize))
	cells := make([][2]int, 0, (right-left+1)*(bottom-top+1))
	for y := top; y <= bottom; y++ {
		for x := left; x <= right; x++ {
			cells = append(cells, [2]int{x, y})
		}
	}
	return cells
}

func labelBoundsOverlap(a, b LabelBounds) bool {
	return a.Left < b.Right && a.Right > b.Left && a.Top < b.Bottom && a.Bottom > b.Top
}

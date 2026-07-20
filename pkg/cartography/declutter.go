package cartography

import "math"

// Dot declutter policy for point (POI) layers. Two rules, both deterministic:
//
//  1. Screen-density thinning: at most one dot per kDotCellPixels-sized cell of
//     the unpitched screen grid, first feature in stable tile order wins. This
//     caps dot count independent of how POI-dense the data is.
//  2. Distance fade: dots fade out with ground distance from the camera and are
//     culled entirely past the fade end. On a pitched camera the far field
//     otherwise accumulates into an opaque band of markers at the horizon; a
//     top-down view spans less than a viewport of ground, so it is unaffected.
//
// Labels have their own collision/density pass (LimitLabelCandidates); this is
// the equivalent for the dot geometry itself.

// kDotCellPixels is the screen cell granting at most one dot (unpitched px).
const kDotCellPixels = 24.0

// kDotFadeStartViewports and kDotFadeEndViewports bound the fade band, measured
// in "ground metres covered by one viewport height at the camera's zoom".
const (
	kDotFadeStartViewports = 1.0
	kDotFadeEndViewports   = 2.5
)

type pointDeclutter struct {
	centerX, centerY float64
	cellSize         float64 // world units per thinning cell
	metersPerUnit    float64 // ground metres per world unit at camera latitude
	fadeStartMeters  float64
	fadeEndMeters    float64
	cells            map[[2]int32]struct{}
}

func newPointDeclutter(camera Camera) *pointDeclutter {
	centerX, centerY := Project(camera.Longitude, camera.Latitude)
	worldPixels := 512.0 * math.Pow(2.0, camera.Zoom)
	height := float64(camera.Height)
	if height <= 0 {
		height = 512
	}
	metersPerUnit := 40075016.68557849 * math.Max(.01, math.Cos(camera.Latitude*math.Pi/180))
	viewportGroundMeters := height / worldPixels * metersPerUnit
	return &pointDeclutter{
		centerX: centerX, centerY: centerY,
		cellSize:        kDotCellPixels / worldPixels,
		metersPerUnit:   metersPerUnit,
		fadeStartMeters: kDotFadeStartViewports * viewportGroundMeters,
		fadeEndMeters:   kDotFadeEndViewports * viewportGroundMeters,
		cells:           make(map[[2]int32]struct{}),
	}
}

// admit decides whether a dot at world position (x, y) is drawn. It returns the
// alpha scale for the distance fade and false when the dot is thinned or culled.
// A cell is only claimed by an admitted dot, so a culled far dot never blocks a
// nearer one arriving later.
func (d *pointDeclutter) admit(x, y float32) (float32, bool) {
	dx := float64(x) - d.centerX
	if dx > .5 {
		dx -= 1
	} else if dx < -.5 {
		dx += 1
	}
	dy := float64(y) - d.centerY
	meters := math.Hypot(dx, dy) * d.metersPerUnit
	if meters >= d.fadeEndMeters {
		return 0, false
	}
	scale := 1.0
	if meters > d.fadeStartMeters {
		scale = (d.fadeEndMeters - meters) / (d.fadeEndMeters - d.fadeStartMeters)
	}
	cell := [2]int32{int32(math.Floor(float64(x) / d.cellSize)), int32(math.Floor(float64(y) / d.cellSize))}
	if _, taken := d.cells[cell]; taken {
		return 0, false
	}
	d.cells[cell] = struct{}{}
	return float32(scale), true
}

// fadedColor scales a color's alpha for the distance fade.
func fadedColor(color Color, scale float32) Color {
	if scale >= 1 {
		return color
	}
	color.A = uint8(float32(color.A) * scale)
	return color
}

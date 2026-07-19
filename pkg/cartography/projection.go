package cartography

import (
	"errors"
	"math"
	"sort"
)

const (
	maxMercatorLatitude    = 85.0511287798066
	mapVerticalFieldOfView = 45 * math.Pi / 180
)

// Camera is the platform-independent map camera used during scene generation.
type Camera struct {
	Latitude, Longitude  float64
	Zoom, Bearing, Pitch float64
	Width, Height        uint32
}

// TileID identifies one Web-Mercator tile.
type TileID struct {
	Z uint8
	X uint32
	Y uint32
}

// Validate rejects non-finite or renderer-hostile cameras.
func (camera Camera) Validate() error {
	values := [...]float64{camera.Latitude, camera.Longitude, camera.Zoom, camera.Bearing, camera.Pitch}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("cartography: non-finite camera")
		}
	}
	if camera.Latitude < -maxMercatorLatitude || camera.Latitude > maxMercatorLatitude || camera.Longitude < -180 || camera.Longitude > 180 || camera.Zoom < 0 || camera.Zoom > 24 || camera.Pitch < 0 || camera.Pitch > 85 || camera.Width == 0 || camera.Height == 0 || camera.Width > 16384 || camera.Height > 16384 {
		return errors.New("cartography: camera out of range")
	}
	return nil
}

// ScreenPoint is a perspective-projected logical viewport coordinate. Depth is
// measured in camera-local pixels and Visible is false behind the near plane.
type ScreenPoint struct {
	X, Y    float64
	Depth   float64
	Visible bool
}

// ProjectWorldPoint applies the shared bearing, pitch, perspective, and
// elevation transform used by map label placement and target presenters.
func ProjectWorldPoint(camera Camera, worldX, worldY, elevationMetres float64) (ScreenPoint, error) {
	if err := camera.Validate(); err != nil {
		return ScreenPoint{}, err
	}
	if math.IsNaN(worldX) || math.IsInf(worldX, 0) || math.IsNaN(worldY) || math.IsInf(worldY, 0) || math.IsNaN(elevationMetres) || math.IsInf(elevationMetres, 0) || worldY < 0 || worldY > 1 || math.Abs(elevationMetres) > 100_000 {
		return ScreenPoint{}, errors.New("cartography: invalid world point")
	}
	centerX, centerY := Project(camera.Longitude, camera.Latitude)
	dx := worldX - centerX
	if dx > .5 {
		dx--
	} else if dx < -.5 {
		dx++
	}
	directY := worldY - centerY
	worldPixels := 512 * math.Exp2(camera.Zoom)
	angle := -camera.Bearing * math.Pi / 180
	cosine, sine := math.Cos(angle), math.Sin(angle)
	localX := (dx*cosine - directY*sine) * worldPixels
	localY := (dx*sine + directY*cosine) * worldPixels
	metresToPixels := worldPixels / (40075016.68557849 * math.Max(.01, math.Cos(camera.Latitude*math.Pi/180)))
	localZ := elevationMetres * metresToPixels
	pitch := camera.Pitch * math.Pi / 180
	pitchCosine, pitchSine := math.Cos(pitch), math.Sin(pitch)
	projectedY := localY*pitchCosine - localZ*pitchSine
	depth := localY*pitchSine + localZ*pitchCosine
	cameraDistance := float64(camera.Height) * .5 / math.Tan(mapVerticalFieldOfView*.5)
	denominator := cameraDistance - depth
	if denominator <= cameraDistance*.05 {
		return ScreenPoint{Depth: depth}, nil
	}
	perspective := cameraDistance / denominator
	return ScreenPoint{X: float64(camera.Width)*.5 + localX*perspective, Y: float64(camera.Height)*.5 + projectedY*perspective, Depth: depth, Visible: true}, nil
}

// Project maps longitude and latitude to normalized Web-Mercator coordinates.
func Project(longitude, latitude float64) (x, y float64) {
	latitude = math.Max(-maxMercatorLatitude, math.Min(maxMercatorLatitude, latitude))
	x = (longitude + 180) / 360
	sin := math.Sin(latitude * math.Pi / 180)
	y = .5 - math.Log((1+sin)/(1-sin))/(4*math.Pi)
	return x, y
}

// Unproject maps normalized Web-Mercator coordinates to longitude and latitude.
func Unproject(x, y float64) (longitude, latitude float64) {
	longitude = x*360 - 180
	latitude = math.Atan(math.Sinh(math.Pi*(1-2*y))) * 180 / math.Pi
	return longitude, latitude
}

// Cover returns a conservative, deterministic tile cover for the camera. The
// cover accounts for bearing and expands toward the horizon as pitch rises.
func Cover(camera Camera, maximum int) ([]TileID, error) {
	z := int(math.Floor(camera.Zoom))
	if z > 22 {
		z = 22
	}
	return CoverAtZoom(camera, z, maximum)
}

// CoverAtZoom returns the visible tile cover at an explicit native tile zoom
// while retaining the camera's logical zoom and footprint. It is used to
// overzoom a bounded source without requesting nonexistent higher-level tiles.
func CoverAtZoom(camera Camera, tileZoom, maximum int) ([]TileID, error) {
	if err := camera.Validate(); err != nil {
		return nil, err
	}
	if maximum <= 0 || maximum > 4096 {
		return nil, errors.New("cartography: invalid tile limit")
	}
	if tileZoom < 0 || tileZoom > 22 {
		return nil, errors.New("cartography: tile zoom out of range")
	}
	z := tileZoom
	n := math.Exp2(float64(z))
	cx, cy := Project(camera.Longitude, camera.Latitude)
	// One tile is 512 logical pixels at its native zoom. Fractional zoom scales
	// it; pitch expands the vertical footprint without pretending this is a
	// terrain-aware frustum test.
	scale := math.Exp2(camera.Zoom - float64(z))
	halfX := float64(camera.Width) / (1024 * scale * n)
	halfY := float64(camera.Height) / (1024 * scale * n)
	pitchExpansion := 1 + 2*math.Sin(camera.Pitch*math.Pi/180)
	halfY *= pitchExpansion
	angle := camera.Bearing * math.Pi / 180
	coverX := math.Abs(math.Cos(angle))*halfX + math.Abs(math.Sin(angle))*halfY
	coverY := math.Abs(math.Sin(angle))*halfX + math.Abs(math.Cos(angle))*halfY
	minX, maxX := int(math.Floor((cx-coverX)*n)), int(math.Floor((cx+coverX)*n))
	minY, maxY := int(math.Floor((cy-coverY)*n)), int(math.Floor((cy+coverY)*n))
	if minY < 0 {
		minY = 0
	}
	if maxY >= int(n) {
		maxY = int(n) - 1
	}
	seen := make(map[TileID]struct{})
	result := make([]TileID, 0, (maxX-minX+1)*(maxY-minY+1))
	for y := minY; y <= maxY; y++ {
		for rawX := minX; rawX <= maxX; rawX++ {
			x := rawX % int(n)
			if x < 0 {
				x += int(n)
			}
			id := TileID{Z: uint8(z), X: uint32(x), Y: uint32(y)}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			result = append(result, id)
			if len(result) > maximum {
				return nil, errors.New("cartography: tile cover exceeds limit")
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Z != result[j].Z {
			return result[i].Z < result[j].Z
		}
		if result[i].Y != result[j].Y {
			return result[i].Y < result[j].Y
		}
		return result[i].X < result[j].X
	})
	return result, nil
}

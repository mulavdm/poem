package types

import (
	"image"
	"image/color"
	"math/rand"
	"time"
)

type Particle struct {
	X    float64
	Y    float64
	VX   float64
	VY   float64
	Size int
}

type ParticleSystem struct {
	Particles     []Particle
	Bounds        image.Rectangle
	rng           *rand.Rand
	GlowIntensity float32
	WindSpeed     float32
}

func NewParticleSystem(count int, bounds image.Rectangle) *ParticleSystem {
	ps := &ParticleSystem{
		Particles:     make([]Particle, count),
		Bounds:        bounds,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
		GlowIntensity: 4.0,
		WindSpeed:     40.0,
	}

	for i := 0; i < count; i++ {
		ps.Particles[i] = Particle{
			X:    float64(ps.rng.Intn(bounds.Dx())),
			Y:    float64(ps.rng.Intn(bounds.Dy())),
			VX:   float64(ps.rng.Intn(100) - 50),
			VY:   float64(ps.rng.Intn(100) - 50),
			Size: ps.rng.Intn(3) + 1,
		}
	}
	return ps
}

func (ps *ParticleSystem) Update(dt float64) {
	boundsW := float64(ps.Bounds.Dx())
	boundsH := float64(ps.Bounds.Dy())

	speedMultiplier := 1.0
	if ps.WindSpeed > 0 {
		speedMultiplier = float64(ps.WindSpeed) / 40.0 // Normalize around 40.0 wind speed
	}

	for i := range ps.Particles {
		p := &ps.Particles[i]

		// Update position based on velocity, delta time, and speedMultiplier
		p.X += p.VX * dt * speedMultiplier
		p.Y += p.VY * dt * speedMultiplier

		// Wrap around boundaries
		if p.X < 0 {
			p.X = boundsW
		}
		if p.X > boundsW {
			p.X = 0
		}
		if p.Y < 0 {
			p.Y = boundsH
		}
		if p.Y > boundsH {
			p.Y = 0
		}
	}
}

func (ps *ParticleSystem) Draw(p Painter, state *ApplicationState) {
	dotColor := color.RGBA{0, 150, 255, 40}  // Subtle blue
	lineColor := color.RGBA{0, 100, 200, 20} // Even subtler lines

	// Scale dotColor opacity based on GlowIntensity
	if ps.GlowIntensity > 4.0 {
		dotColor = color.RGBA{0, 255, 150, 80}  // Emerald glowing dots when active
		lineColor = color.RGBA{0, 200, 150, 40} // Emerald lines
		p.SetGlow(ps.GlowIntensity)
	}

	// Draw connections (Plexus effect)
	for i := 0; i < len(ps.Particles); i++ {
		p1 := ps.Particles[i]
		for j := i + 1; j < len(ps.Particles); j++ {
			p2 := ps.Particles[j]

			dx := p1.X - p2.X
			dy := p1.Y - p2.Y
			distSq := dx*dx + dy*dy

			if distSq < 10000 { // Max distance squared (100^2)
				p.DrawLine(int(p1.X), int(p1.Y), int(p2.X), int(p2.Y), lineColor)
			}
		}
	}

	// Draw particles
	for _, part := range ps.Particles {
		px := int(part.X)
		py := int(part.Y)
		p.FillRect(image.Rect(px, py, px+part.Size, py+part.Size), dotColor)
	}

	p.SetGlow(0)
}

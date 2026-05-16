package render

import (
	"image"
	"image/color"
	"math/rand"
	"time"
)

type Particle struct {
	Pos image.Point
	Vel image.Point
	Size int
}

type ParticleSystem struct {
	Particles []Particle
	Bounds    image.Rectangle
	rng       *rand.Rand
}

func NewParticleSystem(count int, bounds image.Rectangle) *ParticleSystem {
	ps := &ParticleSystem{
		Particles: make([]Particle, count),
		Bounds:    bounds,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	for i := 0; i < count; i++ {
		ps.Particles[i] = Particle{
			Pos: image.Point{
				X: ps.rng.Intn(bounds.Dx()),
				Y: ps.rng.Intn(bounds.Dy()),
			},
			Vel: image.Point{
				X: ps.rng.Intn(100) - 50,
				Y: ps.rng.Intn(100) - 50,
			},
			Size: ps.rng.Intn(3) + 1,
		}
	}
	return ps
}

func (ps *ParticleSystem) Update(dt float64) {
	for i := range ps.Particles {
		p := &ps.Particles[i]
		
		// Update position based on velocity and delta time
		// Using fixed-point or scaled math since image.Point is int
		p.Pos.X += int(float64(p.Vel.X) * dt)
		p.Pos.Y += int(float64(p.Vel.Y) * dt)

		// Wrap around
		if p.Pos.X < 0 { p.Pos.X = ps.Bounds.Dx() }
		if p.Pos.X > ps.Bounds.Dx() { p.Pos.X = 0 }
		if p.Pos.Y < 0 { p.Pos.Y = ps.Bounds.Dy() }
		if p.Pos.Y > ps.Bounds.Dy() { p.Pos.Y = 0 }
	}
}

type ParticleComponent struct {
	CompID string
	System *ParticleSystem
}

func (c *ParticleComponent) GetID() string { return c.CompID }
func (c *ParticleComponent) ID() string    { return c.CompID }
func (c *ParticleComponent) Bounds() image.Rectangle { return c.System.Bounds }
func (c *ParticleComponent) SetBounds(r image.Rectangle) { c.System.Bounds = r }
func (c *ParticleComponent) HitTest(pt image.Point) string { return "" }
func (c *ParticleComponent) Focusable() bool { return false }
func (c *ParticleComponent) Walk(fn func(Component)) { fn(c) }

func (c *ParticleComponent) OnKey(key uint32, char rune, state *ApplicationState) bool { return false }
func (c *ParticleComponent) OnMouseDown(pt image.Point, state *ApplicationState) bool { return false }
func (c *ParticleComponent) OnMouseUp(pt image.Point, state *ApplicationState) bool { return false }
func (c *ParticleComponent) OnMouseMove(pt image.Point, state *ApplicationState) bool { return false }

func (c *ParticleComponent) Draw(p Painter, state *ApplicationState) {
	dotColor := color.RGBA{0, 150, 255, 40} // Subtle blue
	lineColor := color.RGBA{0, 100, 200, 20} // Even subtler lines

	// Draw connections (Plexus effect)
	for i := 0; i < len(c.System.Particles); i++ {
		p1 := c.System.Particles[i]
		for j := i + 1; j < len(c.System.Particles); j++ {
			p2 := c.System.Particles[j]
			
			dx := p1.Pos.X - p2.Pos.X
			dy := p1.Pos.Y - p2.Pos.Y
			distSq := dx*dx + dy*dy
			
			if distSq < 10000 { // Max distance squared
				p.DrawLine(p1.Pos.X, p1.Pos.Y, p2.Pos.X, p2.Pos.Y, lineColor)
			}
		}
	}

	// Draw particles
	for _, part := range c.System.Particles {
		p.FillRect(image.Rect(part.Pos.X, part.Pos.Y, part.Pos.X+part.Size, part.Pos.Y+part.Size), dotColor)
	}
}

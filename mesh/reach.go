package mesh

import (
	"math"

	"glonk.dev/geometry"
)

// reacher is a Light that says how far it reaches: nothing outside the ball with that center
// and radius is lit by it. Point is one, and a light of a program's own can be.
type reacher interface {
	Reach() (center geometry.Vector3, radius float64)
}

// cannotReach reports whether a light that reaches a ball, with its center and radius,
// can light no point of the ball of a mesh. It says yes only when the balls are apart by more
// than the rounding of the places involved, and no when a number is not one, so a mistake in
// a Reach can only cost time. The rounding is the 1e-9 of the size of the numbers that
// UpdateBounds keeps its sphere safe with, since the points lit are blends of corners placed
// by the Transform.
func cannotReach(lightCenter geometry.Vector3, lightRadius float64, meshCenter geometry.Vector3, meshRadius float64) bool {
	if !(lightRadius >= 0) || !(meshRadius >= 0) {
		return false // negative or not a number
	}
	slack := 1e-9 * (lightRadius + meshRadius + lightCenter.Length() + meshCenter.Length())
	return lightCenter.Sub(meshCenter).Length() > lightRadius+meshRadius+slack
}

// lightsReaching returns the lights that can light some part of the mesh once it is placed:
// all of them unless the mesh has current bounds and one of them says it cannot reach them,
// in the same order. They are a copy in the scratch, always, since a pooled triangle that held
// the caller's list would make that list, and the slice of lights that DrawShaded is called
// with, live on the heap. A light that does not say how far it reaches is kept, as is a nil
// one (which lights nothing). Leaving out a light that gives 0 everywhere the mesh is changes
// no sum in Brightness, since adding 0 is exact, so the picture is the same.
func (s *scratch) lightsReaching(m *Mesh, place placement, lights []Light) []Light {
	all := append(s.lit[:0], lights...)
	b := m.currentBounds()
	if !cullingOn || b == nil {
		s.lit = all
		return all
	}
	middle, radius := place.apply(b.center), b.radius*m.Transform.scale()
	kept := all[:0] // the lights are taken out in place: what is written is never ahead of what is read
	for _, l := range all {
		if r, ok := l.(reacher); ok {
			if c, rr := r.Reach(); cannotReach(c, rr, middle, radius) {
				continue
			}
		}
		kept = append(kept, l)
	}
	clear(all[len(kept):]) // so the pool does not keep a program's lights alive
	s.lit = kept
	return kept
}

// Reach is where the lamp is and how far it reaches: nothing farther than Range from Position
// is lit. A Range of 0, which means no fading, reaches everywhere, so the radius is infinite.
// A Range that is negative or not a number gives no light at all, which a radius of 0
// says as far as it needs to. Drawing leaves a Point out for a mesh that is wholly beyond its
// Range, which changes no picture.
func (p Point) Reach() (center geometry.Vector3, radius float64) {
	switch {
	case !(p.Range >= 0):
		return p.Position, 0
	case p.Range == 0:
		return p.Position, math.Inf(1)
	}
	return p.Position, p.Range
}

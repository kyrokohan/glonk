package mesh

import (
	"math"

	"glonk.dev/geometry"
)

// cullingOn lets tests draw with and without skipping what is off screen, to show that
// it changes no picture.
var cullingOn = true

// offscreen reports whether no part of the mesh can be on screen, so that drawing it can
// be skipped. It only says yes when sure: the mesh is inside a sphere, and the sphere is
// wholly beyond one side of the view, by more than a cell, wholly behind the near plane or
// wholly beyond the far one. The sphere is the one UpdateBounds keeps, round the vertices
// themselves, while it is current, and otherwise one round Position that holds the vertex
// farthest from the model's origin. The sphere's radius is looked at only if its middle is
// itself off the view, so a mesh that is on screen costs one point and a few multiplications.
func (m Mesh) offscreen(eye viewTransform, proj projector, near, far float64) bool {
	if !cullingOn || len(m.Vertices) == 0 {
		return false
	}
	b := m.currentBounds()
	var middle geometry.Vector3 // where the sphere is, in the world: Position for one round the model's origin
	if b == nil || b.center == (geometry.Vector3{}) {
		middle = m.Transform.Position
	} else {
		middle = m.Transform.placement().apply(b.center)
	}
	c := eye.apply(middle)

	// Inward normals of the planes through the camera that bound the view, one cell
	// outside the grid: a point inside all of them (and in front of the camera) can only
	// land on the grid or just off it. They are not unit length, which is made up for
	// below, and only when it matters.
	a, side := proj.focal*proj.h/2*proj.aspect, proj.w/2+1
	b2, up := proj.focal*proj.h/2, proj.h/2+1
	left, right := a*c.X+side*c.Z, -a*c.X+side*c.Z
	top, bottom := -b2*c.Y+up*c.Z, b2*c.Y+up*c.Z
	if left >= 0 && right >= 0 && top >= 0 && bottom >= 0 && c.Z >= near && c.Z <= far {
		return false // the middle is on screen, the usual case, so there is no need to look further
	}

	var reach float64
	if b != nil {
		reach = b.radius * m.Transform.scale()
	} else {
		reach = m.reach()
	}
	for _, p := range [...]struct{ dist, x, y, z float64 }{
		{left, a, 0, side}, {right, -a, 0, side}, {top, 0, -b2, up}, {bottom, 0, b2, up},
	} {
		if p.dist < -reach*math.Sqrt(p.x*p.x+p.y*p.y+p.z*p.z) {
			return true
		}
	}
	return c.Z+reach < near || c.Z-reach > far
}

// reach is how far from its Position the mesh can get once it is placed: its farthest
// vertex from the model's origin, made larger by the Scale. A vertex that is not a
// number does not count, as it draws nothing.
func (m Mesh) reach() float64 {
	far := 0.0
	for _, v := range m.Vertices {
		if d := v.LengthSq(); d > far {
			far = d
		}
	}
	return math.Sqrt(far) * m.Transform.scale()
}

// bounds is a sphere in model space that holds every vertex of a mesh, worked out once by
// UpdateBounds, and which vertices it was worked out from, so that a mesh whose Vertices were
// replaced since is known not to be held by it. It is never changed after it is made, so the
// copies of a Mesh can share it.
type bounds struct {
	center geometry.Vector3
	radius float64           // before the Scale
	first  *geometry.Vector3 // the first of the vertices it was worked out from
	n      int               // and how many there were
}

// UpdateBounds works out the sphere that holds the vertices, and keeps it, so that drawing can
// skip the mesh where the sphere is off the screen without looking at a vertex. [Cube],
// [Sphere], [Cylinder], [Cone], [Plane], [Grid], [LoadOBJ] and [LoadOBJFS] do it for the meshes
// they make, and so do Clone and Fit for theirs. A mesh built by hand does not have it until this
// is called, and is skipped by a sphere round its Position that is as big as the vertex
// farthest from the model's origin: right for a model built round its origin and then placed,
// and too big to skip anything for one whose vertices are written where they are in the world, as
// a level built in place is, so call UpdateBounds once, after setting its Vertices.
//
// The sphere is round the vertices themselves, the middle of their box or the model's origin
// where that is no bigger, and its radius reaches the farthest of them. The Transform places
// it as it places the vertices, so it stays right when the mesh is moved, turned or scaled. A
// vertex at infinity makes it infinite, so that the mesh is never skipped, and a vertex that is
// not a number is left out, as it draws nothing.
//
// The sphere is used while Vertices is still the same list, with the same length and first
// element, so appending to it or replacing it never makes a stale sphere skip anything. Changing
// a vertex in place does not show, so call UpdateBounds again after it: a mesh that has moved
// out of its sphere can be skipped while it is on screen. [Mesh.Validate] reports that.
func (m *Mesh) UpdateBounds() {
	m.bounds = nil
	if len(m.Vertices) == 0 {
		return
	}
	m.bounds = newBounds(m.Vertices)
}

// currentBounds returns the stored sphere if it was worked out from the vertices the mesh has now,
// and nil otherwise.
func (m Mesh) currentBounds() *bounds {
	b := m.bounds
	if b == nil || len(m.Vertices) != b.n || &m.Vertices[0] != b.first {
		return nil
	}
	return b
}

// newBounds works out the sphere for a list of vertices, which is not empty.
func newBounds(vs []geometry.Vector3) *bounds {
	b := &bounds{first: &vs[0], n: len(vs)}
	lo := geometry.Vector3{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	hi := geometry.Vector3{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	counted := false
	for _, v := range vs {
		switch {
		case math.IsNaN(v.X) || math.IsNaN(v.Y) || math.IsNaN(v.Z):
			continue // draws nothing
		case math.IsInf(v.X, 0) || math.IsInf(v.Y, 0) || math.IsInf(v.Z, 0):
			b.radius = math.Inf(1) // it cannot be ruled out
			return b
		}
		counted = true
		lo = geometry.Vector3{X: min(lo.X, v.X), Y: min(lo.Y, v.Y), Z: min(lo.Z, v.Z)}
		hi = geometry.Vector3{X: max(hi.X, v.X), Y: max(hi.Y, v.Y), Z: max(hi.Z, v.Z)}
	}
	if !counted {
		return b
	}

	// The sphere round the middle of the box, and the one round the model's origin, which costs
	// no turning to place: take the origin's if it is no bigger.
	middle := lo.Add(hi).Scale(0.5)
	var round, origin float64 // the farthest vertex from each
	for _, v := range vs {
		if math.IsNaN(v.X) || math.IsNaN(v.Y) || math.IsNaN(v.Z) {
			continue
		}
		round = max(round, v.Sub(middle).LengthSq())
		origin = max(origin, v.LengthSq())
	}
	if origin <= round*(1+1e-9) {
		middle, round = geometry.Vector3{}, origin
	}
	// A little more than the farthest vertex, for the rounding of placing it and the middle.
	b.center = middle
	b.radius = math.Sqrt(round)*(1+1e-9) + 1e-9*middle.Length()
	return b
}

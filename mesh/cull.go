package mesh

import "math"

// cullingOn lets tests draw with and without skipping what is off screen, to show that
// it changes no picture.
var cullingOn = true

// offscreen reports whether no part of the mesh can be on screen, so that drawing it can
// be skipped. It only says yes when sure: the mesh is inside a sphere around its Position,
// and the sphere is wholly beyond one side of the view, by more than a cell, wholly behind
// the near plane or wholly beyond the far one. The sphere is looked at only if the middle of
// the mesh is itself off the view, so a mesh that is on screen costs one point and a few
// multiplications.
func (m Mesh) offscreen(eye viewTransform, proj projector, near, far float64) bool {
	if !cullingOn || len(m.Vertices) == 0 {
		return false
	}
	c := eye.apply(m.Transform.Position)

	// Inward normals of the planes through the camera that bound the view, one cell
	// outside the grid: a point inside all of them (and in front of the camera) can only
	// land on the grid or just off it. They are not unit length, which is made up for
	// below, and only when it matters.
	a, side := proj.focal*proj.h/2*proj.aspect, proj.w/2+1
	b, up := proj.focal*proj.h/2, proj.h/2+1
	left, right := a*c.X+side*c.Z, -a*c.X+side*c.Z
	top, bottom := -b*c.Y+up*c.Z, b*c.Y+up*c.Z
	if left >= 0 && right >= 0 && top >= 0 && bottom >= 0 && c.Z >= near && c.Z <= far {
		return false // the middle is on screen, the usual case, so there is no need to look further
	}

	reach := m.reach()
	for _, p := range [...]struct{ dist, x, y, z float64 }{
		{left, a, 0, side}, {right, -a, 0, side}, {top, 0, -b, up}, {bottom, 0, b, up},
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
	scale := m.Transform.Scale
	if scale == 0 {
		scale = 1
	}
	return math.Sqrt(far) * scale
}

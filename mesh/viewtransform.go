package mesh

import (
	"math"
	"slices"
	"sync"

	"glonk.dev/geometry"
)

// viewTransform carries a camera's pose from world space into view space (camera at
// the origin, looking down +z) with the trigonometry worked out once. It does the same
// arithmetic, in the same order, as turning the point with geometry.Vector3's RotateY
// and RotateX. Where the compiler fuses multiply-adds (arm64) the two may still differ
// in the last bit, because it fuses by context; nowhere else.
type viewTransform struct {
	pos            geometry.Vector3
	sinYaw, cosYaw float64 // of -Yaw: undo the camera's turn first
	sinPit, cosPit float64 // of Pitch: then undo its tilt
}

// viewTransform returns the transform for c, which must be usable (see withDefaults).
func (c Camera) viewTransform() viewTransform {
	return viewTransform{
		pos:    c.Position,
		sinYaw: math.Sin(-c.Yaw), cosYaw: math.Cos(-c.Yaw),
		sinPit: math.Sin(c.Pitch), cosPit: math.Cos(c.Pitch),
	}
}

// apply moves the world point p into view space.
func (t viewTransform) apply(p geometry.Vector3) geometry.Vector3 {
	d := p.Sub(t.pos)
	// RotateY(-Yaw)
	x := d.X*t.cosYaw + d.Z*t.sinYaw
	z := -d.X*t.sinYaw + d.Z*t.cosYaw
	// RotateX(Pitch)
	return geometry.Vector3{
		X: x,
		Y: d.Y*t.cosPit - z*t.sinPit,
		Z: d.Y*t.sinPit + z*t.cosPit,
	}
}

// projector maps view-space points to fractional cell coordinates on a grid of w x h
// cells, with the camera's field of view and cell shape worked out once. It does the
// same arithmetic as Camera.projectView.
type projector struct {
	focal  float64 // 1 / tan(FOV / 2)
	w, h   float64
	aspect float64
}

// projector returns the projector for c, which must be usable (see withDefaults).
func (c Camera) projector(w, h int) projector {
	return projector{focal: 1 / math.Tan(c.FOV/2), w: float64(w), h: float64(h), aspect: c.CellAspect}
}

// project maps v, which must be at least Near in front of the camera, to cell
// coordinates.
func (p projector) project(v geometry.Vector3) (x, y float64) {
	ndcX := p.focal * v.X / v.Z
	ndcY := p.focal * v.Y / v.Z
	return p.w/2 + ndcX*p.h/2*p.aspect, p.h/2 - ndcY*p.h/2
}

// scratch holds the lists a draw call needs, kept between calls so that drawing stops
// allocating once they have grown to the size of the meshes drawn.
type scratch struct {
	world  []geometry.Vector3
	view   []geometry.Vector3
	normal []geometry.Vector3 // the mesh's normals in world space, for smooth shading
	ramp   []rune
	tri    shadedTriangle // the triangle being handed to the fill
}

var scratchPool = sync.Pool{New: func() any { return new(scratch) }}

// getScratch returns a scratch whose world and view lists are n long. Their contents
// are not cleared. Give it back with release.
func getScratch(n int) *scratch {
	s := scratchPool.Get().(*scratch)
	s.world = resize(s.world, n)
	s.view = resize(s.view, n)
	return s
}

func (s *scratch) release() {
	scratchPool.Put(s)
}

// rampFor returns the characters to shade m with, darkest first. A ramp of the
// material's own is decoded into memory the scratch keeps, so it is good until release.
func (s *scratch) rampFor(m Material) []rune {
	if m.Ramp == "" {
		return defaultRampRunes // shared and read-only: never keep it as the buffer
	}
	s.ramp = s.ramp[:0]
	for _, r := range m.Ramp { // invalid UTF-8 comes out as U+FFFD, as []rune does
		s.ramp = append(s.ramp, r)
	}
	return s.ramp
}

// resize returns list with length n, reusing its memory when it is big enough.
func resize(list []geometry.Vector3, n int) []geometry.Vector3 {
	if cap(list) < n {
		return slices.Grow(list[:0], n)[:n]
	}
	return list[:n]
}

package mesh

import (
	"math"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// SmoothNormals returns a copy of the mesh with a normal at every vertex, found by
// averaging the faces that meet there, so that shaded drawing makes a round shape
// look round. Each face counts in proportion to the angle it makes at the vertex, so
// how a surface is cut into triangles does not change how it is lit. A vertex no
// face uses gets a zero normal, which shaded drawing treats as "none here".
//
// Only faces that share a vertex are averaged. A cube built with 8 vertices comes
// out looking like a ball; one built with 24, three to a corner, keeps its sharp
// edges. Faces that name a vertex the mesh does not have, that have no area, or that
// have a vertex that is not finite are left out. Existing Normals are replaced, and
// the copy shares nothing with the original.
func (m Mesh) SmoothNormals() Mesh {
	out := m.Clone()
	sums := make([]geometry.Vector3, len(m.Vertices))

	for _, f := range m.Faces {
		if !validIndex(f[0], len(sums)) || !validIndex(f[1], len(sums)) || !validIndex(f[2], len(sums)) {
			continue
		}
		p := [3]geometry.Vector3{m.Vertices[f[0]], m.Vertices[f[1]], m.Vertices[f[2]]}
		// A face with no area has a zero normal, which adds nothing.
		normal, ok := unitDirection(p[1].Sub(p[0]).Cross(p[2].Sub(p[0])))
		if !ok {
			continue
		}
		for i := range 3 {
			// The angle between the two edges that leave this corner.
			e1, e2 := p[(i+1)%3].Sub(p[i]), p[(i+2)%3].Sub(p[i])
			angle := math.Atan2(e1.Cross(e2).Length(), e1.Dot(e2))
			sums[f[i]] = sums[f[i]].Add(normal.Scale(angle))
		}
	}

	out.Normals = make([]geometry.Vector3, len(sums))
	for i, sum := range sums {
		out.Normals[i], _ = unitDirection(sum)
	}
	return out
}

// unitDirection returns v scaled to length 1, and whether v was a finite number. The
// zero vector comes back as itself. It scales by the largest component first, so a
// vector of any finite size keeps its direction instead of overflowing or underflowing.
func unitDirection(v geometry.Vector3) (geometry.Vector3, bool) {
	if !finite(v.X, v.Y, v.Z) {
		return v, false
	}
	m := max(math.Abs(v.X), math.Abs(v.Y), math.Abs(v.Z))
	if m == 0 {
		return geometry.Vector3{}, true
	}
	return geometry.Vector3{X: v.X / m, Y: v.Y / m, Z: v.Z / m}.Normalize(), true
}

// hasNormals reports whether the mesh has one normal per vertex, which is what
// smooth shading needs. Whether they are finite is found out when they are used.
func (m Mesh) hasNormals() bool {
	return len(m.Normals) > 0 && len(m.Normals) == len(m.Vertices)
}

// direction turns the model-space direction d the way the model is turned and scales
// it by the model's Scale, but does not move it.
func (p placement) direction(d geometry.Vector3) geometry.Vector3 {
	return p.x.Scale(d.X).Add(p.y.Scale(d.Y)).Add(p.z.Scale(d.Z))
}

// worldNormals returns the mesh's normals in world space, as unit vectors (or zero,
// for a normal of zero), in memory the scratch keeps. It returns nil if there are not
// one per vertex or one is not a finite number, in which case the mesh is shaded flat.
func (s *scratch) worldNormals(m Mesh, place placement) []geometry.Vector3 {
	if !m.hasNormals() {
		return nil
	}
	s.normal = resize(s.normal, len(m.Normals))
	for i, n := range m.Normals {
		// Unit length first, so that scaling by Scale cannot push a size already near
		// the limit of a float over it. A number that is not finite stays one, and is
		// caught after the turn.
		u, _ := unitDirection(n)
		// Scale does not change a direction, but it can make a result too small to
		// measure, so bring it back to unit length.
		w, ok := unitDirection(place.direction(u))
		if !ok {
			return nil
		}
		s.normal[i] = w
	}
	return s.normal
}

// shadedTriangle is a piece of a face on screen, ready to be filled.
type shadedTriangle struct {
	corners [3]screenPoint

	// The color of the face this piece belongs to, where the light meets it fully.
	color engine.Color

	// A flat triangle is lit once, as a whole.
	smooth    bool
	intensity float64

	// A smooth triangle is lit at every cell, from its corners' places in the world
	// and the way the surface faces there (unit vectors, or zero).
	world, normal [3]geometry.Vector3
}

// brightnessAt is the brightness of a smooth triangle at the point with corner
// weights b0, b1 and b2 (see engine.FrameBuffer.FillTriangleFunc).
func (t *shadedTriangle) brightnessAt(lights []Light, b0, b1, b2 float64) float64 {
	point := t.world[0].Scale(b0).Add(t.world[1].Scale(b1)).Add(t.world[2].Scale(b2))
	// Blending unit vectors gives a shorter one; lights are promised a unit normal.
	normal := t.normal[0].Scale(b0).Add(t.normal[1].Scale(b1)).Add(t.normal[2].Scale(b2)).Normalize()
	return brightness(lights, point, normal)
}

// cornerNormal is the normal to use at a corner of a face: the vertex's own, turned
// to face the camera if the face is a back face, or the face's if the vertex has none.
// face is the face's unit normal, already turned that way.
func cornerNormal(vertex, face geometry.Vector3, front bool) geometry.Vector3 {
	switch {
	case vertex == (geometry.Vector3{}):
		return face
	case !front:
		return vertex.Neg()
	}
	return vertex
}

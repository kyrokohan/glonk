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

	// tinted is set when a corner has a vertex color: the triangle is drawn in the
	// blend of rgb, each corner's red, green and blue from 0 to 255, in place of color.
	tinted bool
	rgb    [3]geometry.Vector3

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

// colorAt is the color of the cell at weights b0, b1 and b2 of the three corners: their
// colors blended with the weights, rounded to whole numbers from 0 to 255.
func (t *shadedTriangle) colorAt(b0, b1, b2 float64) engine.Color {
	c := t.rgb[0].Scale(b0).Add(t.rgb[1].Scale(b1)).Add(t.rgb[2].Scale(b2))
	return engine.RGB(channel(c.X), channel(c.Y), channel(c.Z))
}

// channel rounds v to a whole number from 0 to 255. NaN and anything below 0 give 0.
func channel(v float64) uint8 {
	switch {
	case !(v > 0):
		return 0
	case v >= 255:
		return 255
	}
	return uint8(math.Round(v))
}

// cornerColor is the color of a corner of a face whose own color is base, which is
// FaceColors if hasFaceColor and else Material.Color. A corner with a vertex color is
// drawn in it, multiplied by the face's color if it has one; a corner without takes the
// base, which the default color makes black.
func cornerColor(vertex, base engine.Color, hasFaceColor bool) geometry.Vector3 {
	if vertex.IsDefault() {
		return colorVector(base)
	}
	v := colorVector(vertex)
	if !hasFaceColor {
		return v
	}
	f := colorVector(base)
	return geometry.Vector3{X: v.X * f.X / 255, Y: v.Y * f.Y / 255, Z: v.Z * f.Z / 255}
}

// colorAtPoint is the color at p, a point on the triangle v0, v1, v2 in view space, of
// a triangle whose corners have the colors corner: the corners' colors blended with the
// barycentric weights of p. Color changes linearly across a face in space, so this is
// the color it had before the triangle was cut, whatever the cut.
func colorAtPoint(corner [3]geometry.Vector3, v0, v1, v2, p geometry.Vector3) geometry.Vector3 {
	e1, e2, e3 := v1.Sub(v0), v2.Sub(v0), p.Sub(v0)
	d11, d12, d22 := e1.Dot(e1), e1.Dot(e2), e2.Dot(e2)
	d31, d32 := e3.Dot(e1), e3.Dot(e2)
	denom := d11*d22 - d12*d12
	if denom == 0 {
		return corner[0] // a triangle with no area: it has no cells either
	}
	w1 := (d22*d31 - d12*d32) / denom
	w2 := (d11*d32 - d12*d31) / denom
	return corner[0].Scale(1 - w1 - w2).Add(corner[1].Scale(w1)).Add(corner[2].Scale(w2))
}

// colorVector is c as red, green and blue from 0 to 255.
func colorVector(c engine.Color) geometry.Vector3 {
	r, g, b := c.RGB()
	return geometry.Vector3{X: float64(r), Y: float64(g), Z: float64(b)}
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

// fillTinted draws a triangle that has vertex colors into cells. Every cell has a color of
// its own, so, flat or smooth, each is asked for.
func fillTinted(fb *engine.FrameBuffer, t *shadedTriangle, ramp []rune, lights []Light) {
	c := &t.corners
	fb.FillTriangleFunc(c[0].x, c[0].y, c[0].z, c[1].x, c[1].y, c[1].z, c[2].x, c[2].y, c[2].z, func(b0, b1, b2 float64) engine.Cell {
		intensity := t.intensity
		if t.smooth {
			intensity = t.brightnessAt(lights, b0, b1, b2)
		}
		return engine.Cell{Character: shadeChar(ramp, intensity), Foreground: t.colorAt(b0, b1, b2)}
	})
}

// fillTintedPixels is fillTinted for a Canvas, where brightness is a shade of the color.
func fillTintedPixels(c *engine.Canvas, t *shadedTriangle, lights []Light) {
	p := &t.corners
	c.FillTriangleFunc(p[0].x, 2*p[0].y, p[0].z, p[1].x, 2*p[1].y, p[1].z, p[2].x, 2*p[2].y, p[2].z, func(b0, b1, b2 float64) engine.Color {
		intensity := t.intensity
		if t.smooth {
			intensity = t.brightnessAt(lights, b0, b1, b2)
		}
		return engine.Shade(t.colorAt(b0, b1, b2), intensity)
	})
}

package mesh

import (
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// Light lights the surfaces of a mesh. [Directional], [Ambient] and [Point] are the
// lights the package provides; a type with a Brightness method is a Light too, which
// is how a spotlight, or a lamp that fades its own way, is made.
//
// Several lights can light a mesh at once: DrawShaded and DrawShadedPixels take any
// number. They add up without passing full brightness: each light fills part of what
// the ones before it left dark, so an ambient light of 0.2 and a directional light
// that meets a face squarely at 1 give 1, and two lights of 0.5 give 0.75. The order
// does not matter. With no lights, every face is as dark as can be shown.
type Light interface {
	// Brightness returns how bright this light makes a surface, from 0 (dark) to 1
	// (fully lit). point and normal are in world space, and normal is a unit vector
	// pointing out of the surface toward the viewer. A result outside 0 to 1 is
	// clamped, and NaN counts as 0.
	//
	// For a mesh without Normals, point is the middle of a face and normal is the
	// face's normal. For a mesh with them, the light is asked about every cell
	// instead: point is where on the surface that cell is, and normal is the
	// surface's normal there, blended from the face's corners (the zero vector in the
	// rare case that they cancel out).
	Brightness(point, normal geometry.Vector3) float64
}

// Directional is light that travels one way everywhere, like the sun. A surface is
// brightest facing straight into it and dark when edge-on or turned away.
type Directional struct {
	// Direction is the way the light travels, e.g. {0, -1, 0} for a lamp straight
	// overhead. It need not be unit length. A zero Direction gives no light.
	Direction geometry.Vector3

	// Intensity is the brightness, from 0 to 1, of a surface facing straight into the
	// light. 0 means 1; to switch a light off, leave it out.
	Intensity float64
}

// Brightness is Intensity scaled by how squarely the surface faces the light.
func (d Directional) Brightness(_, normal geometry.Vector3) float64 {
	intensity := d.Intensity
	if intensity == 0 {
		intensity = 1
	}
	return clamp01(intensity) * clamp01(normal.Dot(d.Direction.Normalize().Neg()))
}

// Ambient is light from everywhere at once, which fills in the faces no other light
// reaches so that they do not go dark. It does not depend on which way a face points.
type Ambient struct {
	// Level is the brightness, from 0 to 1, of every surface. The zero Ambient gives
	// no light.
	Level float64
}

// Brightness is Level.
func (a Ambient) Brightness(_, _ geometry.Vector3) float64 {
	return clamp01(a.Level)
}

// Point is light from one place that spreads every way and fades with distance, like
// a lamp. A surface is brightest facing straight at the lamp, and dark when edge-on to
// it or turned away, as with [Directional], and dimmer the farther it is.
//
// A Point is asked about the place on the surface being lit, so it lights a mesh
// as well as the mesh lets it. With Normals every cell is lit by itself and a lamp
// close to a big flat surface makes a pool of light on it ([Plane] has Normals). Without
// them a face is lit once, from its middle, and is one shade from edge to edge.
type Point struct {
	// Position is where the lamp is, in world space.
	Position geometry.Vector3

	// Intensity is the brightness, from 0 to 1, of a surface right at the lamp and
	// facing it squarely. 0 means 1; to switch a light off, leave it out.
	Intensity float64

	// Range is how far the light reaches. The light fades smoothly with distance and
	// is gone at Range, so a lamp lights a pool around it. 0 means no fading: the
	// light is as bright a long way off as it is nearby. A Range that is negative or
	// not a number gives no light.
	Range float64
}

// Brightness is Intensity, times how squarely the surface faces the lamp, times a
// fade that is 1 at the lamp and falls to 0 at Range as (1 - distance/Range) squared.
// A surface exactly at the lamp is lit fully, as there is no direction to be turned
// away from.
func (p Point) Brightness(point, normal geometry.Vector3) float64 {
	if !(p.Range >= 0) { // negative, or NaN
		return 0
	}
	intensity := p.Intensity
	if intensity == 0 {
		intensity = 1
	}

	toLamp := p.Position.Sub(point)
	distance := toLamp.Length()
	facing := 1.0
	if distance != 0 {
		facing = normal.Dot(toLamp) / distance
	}

	fade := 1.0
	if p.Range != 0 {
		fade = 1 - distance/p.Range
		if !(fade > 0) {
			return 0
		}
		fade *= fade
	}
	return clamp01(intensity) * clamp01(facing) * fade
}

// clamp01 limits v to 0 to 1. NaN gives 0.
func clamp01(v float64) float64 {
	switch {
	case !(v > 0):
		return 0
	case v > 1:
		return 1
	}
	return v
}

// brightness adds up what lights give a surface; see Light. A nil entry gives
// nothing.
func brightness(lights []Light, point, normal geometry.Vector3) float64 {
	total := 0.0
	for _, l := range lights {
		if l == nil {
			continue
		}
		total += (1 - total) * clamp01(l.Brightness(point, normal))
	}
	return total
}

// frontFacing reports whether a face is turned toward the camera. normal points
// out of the face and v0 is one of its corners, both in view space, where the
// camera sits at the origin: the face is visible when the normal opposes the
// direction from the camera to the face.
func frontFacing(normal, v0 geometry.Vector3) bool {
	return normal.Dot(v0) < 0
}

// screenPoint is a mesh corner on screen: x and y in cell coordinates, z the
// distance from the camera.
type screenPoint struct {
	x, y, z float64
}

// eachShadedTriangle does the work shared by every solid drawing mode. For each
// face turned toward the camera it cuts the face at the near plane, projects the
// pieces onto a w x h grid of cells, and hands each to fn. A mesh without Normals is
// lit here, once for each face, and fn gets the brightness from 0 to 1; a mesh with
// them is lit by fn, cell by cell, from the corners' places and normals. A face that
// names a vertex the mesh does not have is skipped.
func (m Mesh) eachShadedTriangle(cam Camera, lights []Light, w, h int, fn func(t *shadedTriangle)) {
	cam, ok := cam.withDefaults()
	if !ok || !m.Transform.valid() {
		return
	}

	eye := cam.viewTransform()
	proj := cam.projector(w, h)
	if m.offscreen(eye, proj, cam.Near) {
		return
	}

	s := getScratch(len(m.Vertices))
	defer s.release()
	place := m.Transform.placement()
	world, view := s.world, s.view
	for i, v := range m.Vertices {
		world[i] = place.apply(v)
		view[i] = eye.apply(world[i])
	}
	normals := s.worldNormals(m, place) // nil: shade flat
	smooth := normals != nil

	var tris [2][3]clipVert
	tri := &s.tri // in the scratch, because a pointer handed to fn would move a local to the heap
	tri.smooth = smooth
	faceColors := len(m.FaceColors) == len(m.Faces) // otherwise they are not used
	vertexColors := len(m.VertexColors) == len(m.Vertices) && len(m.Vertices) > 0
	tri.tinted = false
	var corner [3]geometry.Vector3 // the colors of the corners of the face being drawn, if it has any
	for i, f := range m.Faces {
		if !validIndex(f[0], len(view)) || !validIndex(f[1], len(view)) || !validIndex(f[2], len(view)) {
			continue
		}
		v0, v1, v2 := view[f[0]], view[f[1]], view[f[2]]

		front := frontFacing(v1.Sub(v0).Cross(v2.Sub(v0)), v0)
		if !front && !m.DoubleSided {
			continue
		}

		// Lighting needs the normal in world space, where the light lives.
		w0, w1, w2 := world[f[0]], world[f[1]], world[f[2]]
		normal := w1.Sub(w0).Cross(w2.Sub(w0)).Normalize()
		if !front {
			normal = normal.Neg() // light the side the camera is looking at
		}

		tri.color = m.Material.Color
		hasFaceColor := faceColors && !m.FaceColors[i].IsDefault()
		if hasFaceColor {
			tri.color = m.FaceColors[i]
		}

		var cut bool // the near plane crosses a tinted triangle
		if vertexColors {
			vc := [3]engine.Color{m.VertexColors[f[0]], m.VertexColors[f[1]], m.VertexColors[f[2]]}
			if tri.tinted = !vc[0].IsDefault() || !vc[1].IsDefault() || !vc[2].IsDefault(); tri.tinted {
				for k := range corner {
					corner[k] = cornerColor(vc[k], tri.color, hasFaceColor)
				}
				cut = v0.Z < cam.Near || v1.Z < cam.Near || v2.Z < cam.Near
			}
		}

		a, b, c := clipVert{view: v0}, clipVert{view: v1}, clipVert{view: v2}
		if smooth {
			a.world, b.world, c.world = w0, w1, w2
			a.normal = cornerNormal(normals[f[0]], normal, front)
			b.normal = cornerNormal(normals[f[1]], normal, front)
			c.normal = cornerNormal(normals[f[2]], normal, front)
		} else {
			center := w0.Add(w1).Add(w2).Scale(1.0 / 3)
			tri.intensity = brightness(lights, center, normal)
		}

		n := clipTriangleNear(a, b, c, cam.Near, smooth, &tris)
		for _, t := range tris[:n] {
			for i, v := range t {
				x, y := proj.project(v.view)
				tri.corners[i] = screenPoint{x, y, v.view.Z}
				if smooth {
					tri.world[i], tri.normal[i] = v.world, v.normal
				}
				if tri.tinted {
					// An uncut triangle comes out with its corners as they went in. A cut
					// one has corners on the near plane, given the color at that place.
					if cut {
						tri.rgb[i] = colorAtPoint(corner, v0, v1, v2, v.view)
					} else {
						tri.rgb[i] = corner[i]
					}
				}
			}
			fn(tri)
		}
	}
}

// DrawShaded renders the mesh's faces as solid triangles, shaded by the lights and
// drawn in the mesh's Material. Nearer surfaces hide farther ones, faces turned away
// from the camera are skipped (unless the mesh is DoubleSided), and faces crossing
// the near plane are cut where they cross it. A mesh without Faces draws nothing; use
// Draw for wireframes.
//
// A mesh without Normals is shaded flat, each face lit as a whole. One with Normals
// is shaded smooth, every cell lit on its own; see Mesh.Normals.
//
// Brightness is shown by the character: a ramp from dim to bright, Material.Ramp.
func (m Mesh) DrawShaded(fb *engine.FrameBuffer, cam Camera, lights ...Light) {
	rs := getScratch(0)
	defer rs.release()
	ramp := rs.rampFor(m.Material)
	m.eachShadedTriangle(cam, lights, fb.Width(), fb.Height(), func(t *shadedTriangle) {
		c := &t.corners
		if t.tinted {
			fillTinted(fb, t, ramp, lights)
			return
		}
		if !t.smooth {
			cell := engine.Cell{Character: shadeChar(ramp, t.intensity), Foreground: t.color}
			fb.FillTriangle(c[0].x, c[0].y, c[0].z, c[1].x, c[1].y, c[1].z, c[2].x, c[2].y, c[2].z, cell)
			return
		}
		fb.FillTriangleFunc(c[0].x, c[0].y, c[0].z, c[1].x, c[1].y, c[1].z, c[2].x, c[2].y, c[2].z, func(b0, b1, b2 float64) engine.Cell {
			return engine.Cell{Character: shadeChar(ramp, t.brightnessAt(lights, b0, b1, b2)), Foreground: t.color}
		})
	})
}

// DrawShadedPixels is DrawShaded for a pixel Canvas: the same faces, culling,
// clipping, depth and smoothing rules, but brightness is shown by the color (a darker
// or lighter shade of Material.Color) and there are twice as many rows, so edges are
// finer. Call Canvas.Compose afterwards to put the pixels into the FrameBuffer.
func (m Mesh) DrawShadedPixels(c *engine.Canvas, cam Camera, lights ...Light) {
	// Project onto the cell grid the canvas belongs to, then stretch y to pixel rows.
	m.eachShadedTriangle(cam, lights, c.Width(), c.Height()/2, func(t *shadedTriangle) {
		p := &t.corners
		if t.tinted {
			fillTintedPixels(c, t, lights)
			return
		}
		if !t.smooth {
			c.FillTriangle(p[0].x, 2*p[0].y, p[0].z, p[1].x, 2*p[1].y, p[1].z, p[2].x, 2*p[2].y, p[2].z, engine.Shade(t.color, t.intensity))
			return
		}
		c.FillTriangleFunc(p[0].x, 2*p[0].y, p[0].z, p[1].x, 2*p[1].y, p[1].z, p[2].x, 2*p[2].y, p[2].z, func(b0, b1, b2 float64) engine.Color {
			return engine.Shade(t.color, t.brightnessAt(lights, b0, b1, b2))
		})
	})
}

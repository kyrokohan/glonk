package mesh

import (
	"math"

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
// does not matter. With no lights, every face is as dark as can be shown. [Brightness]
// adds lights up this way for a program that lights things of its own.
//
// A light that reaches only so far can say so with a method Reach() (center
// geometry.Vector3, radius float64): it gives nothing outside the ball with that center and
// radius, in world space, and a negative or NaN radius is not believed. Drawing then leaves
// it out for a mesh that is wholly beyond the ball, so a scene can have many lamps and each
// mesh asks only the few that reach it. The mesh must have its bounds ([Mesh.UpdateBounds];
// the shapes and models the package makes do), and a light that does not have the method is
// always asked. [Point] has it. A Reach that promises too little loses light, and a picture
// that is wrong, so promise only what the light gives: 1e-9 of the size of the numbers is
// allowed for rounding.
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

// Brightness returns how bright lights make a surface at point, from 0 (dark) to 1 (fully
// lit): the rule DrawShaded and DrawShadedPixels light a face with. normal is a unit vector
// pointing out of the surface toward the viewer, and point and normal are in world space,
// as for [Light]. Each light fills part of what the ones before it left dark, so they add
// up without passing 1 and the order does not matter; a light's own result outside 0 to 1
// is clamped, and NaN counts as 0. A nil light gives nothing, and with no lights the result
// is 0.
//
// A program that draws things of its own into a lit scene can call it to light them the
// way the meshes are lit, and keep doing so if the rule changes.
func Brightness(lights []Light, point, normal geometry.Vector3) float64 {
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

// screenPoint is a mesh corner on screen: x and y in cell coordinates, z its depth, the
// distance from the camera along the view direction.
type screenPoint struct {
	x, y, z float64
}

// eachShadedTriangle does the work shared by every solid drawing mode. For each
// face turned toward the camera it cuts the face at the near plane, and at the far one
// where the camera has one, projects the pieces onto a w x h grid of cells, and hands
// each to fn. A mesh without Normals is lit here, once for each face, and fn gets the
// brightness from 0 to 1; a mesh with them is lit by fn, cell by cell, from the
// corners' places and normals. A face that names a vertex the mesh does not have is
// skipped.
//
// The mesh is taken by pointer, as it is by the helpers it calls (offscreen, currentBounds,
// worldNormals, lightsReaching): a Mesh is 280 bytes, and a copy for each of them, for every mesh
// drawn, was 1 to 2% of a scene of many small ones. The methods that are exported keep their value
// receivers, and copy the mesh once.
func (m *Mesh) eachShadedTriangle(cam Camera, lights []Light, w, h int, fn func(t *shadedTriangle)) {
	cam, ok := cam.withDefaults()
	if !ok || !m.Transform.valid() {
		return
	}

	eye := cam.viewTransform()
	proj := cam.projector(w, h)
	if m.offscreen(eye, proj, cam.Near, cam.Far) {
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

	limited := !math.IsInf(cam.Far, 1) // there is a far plane to cut at

	var tris [2][3]clipVert
	tri := &s.tri // in the scratch, because a pointer handed to fn would move a local to the heap
	tri.smooth = smooth
	lit := s.lightsReaching(m, place, lights) // not assigned to lights: that would make the caller's list escape
	tri.lights = lit
	faceColors := len(m.FaceColors) == len(m.Faces) // otherwise they are not used
	vertexColors := len(m.VertexColors) == len(m.Vertices) && len(m.Vertices) > 0
	pictured := len(m.FaceTexCoords) == len(m.Faces) && m.Material.Texture.Width() > 0
	// A plain mesh, the common one, has nothing to work out for its colors, and takes one test a face for it. The
	// tests for the picture and for the cut, each its own and each made for every face, cost it 3 to 6%.
	special := pictured || vertexColors
	tri.tinted, tri.tex = false, nil
	tri.shown = !m.Material.Texture.Plain() // asked once for the mesh, so a plain picture costs what it did
	tri.opacity = m.Material.seeThrough()
	var corner [3]geometry.Vector3 // the colors of the corners of the face being drawn, if it has any
	var faceUV [3]geometry.Vector2 // the places of the picture its corners are pinned to, if it has one
	for i, f := range m.Faces {
		if !validIndex(f[0], len(view)) || !validIndex(f[1], len(view)) || !validIndex(f[2], len(view)) {
			continue
		}
		v0, v1, v2 := view[f[0]], view[f[1]], view[f[2]]

		// A face wholly beyond the far plane is skipped before it is lit, and one that crosses
		// it is cut below; one wholly in front of it needs nothing done.
		crossesFar := false
		if limited {
			if v0.Z > cam.Far && v1.Z > cam.Far && v2.Z > cam.Far {
				continue
			}
			crossesFar = v0.Z > cam.Far || v1.Z > cam.Far || v2.Z > cam.Far
		}

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

		varying := false // the face has colors or a picture of its own, so every cell is asked for its color
		cut := false     // the near or the far plane crosses a face that does, which gives its new corners theirs
		if special {
			if pictured {
				tri.tex = nil
				// A coordinate that is not finite makes every cell of the face none, which shows the face's own
				// color. The triple is compared a corner at a time, through a pointer: comparing the array as
				// a whole costs every mesh, picture or not, 9% on a mesh of many flat faces.
				if p := &m.FaceTexCoords[i]; p[0] != (geometry.Vector2{}) || p[1] != (geometry.Vector2{}) || p[2] != (geometry.Vector2{}) {
					tri.tex, faceUV, varying = m.Material.Texture, *p, true
				}
			}
			if vertexColors {
				vc := [3]engine.Color{m.VertexColors[f[0]], m.VertexColors[f[1]], m.VertexColors[f[2]]}
				if tri.tinted = !vc[0].IsDefault() || !vc[1].IsDefault() || !vc[2].IsDefault(); tri.tinted {
					varying = true
					for k := range corner {
						if tri.tex != nil {
							// A corner without a color lets all of the picture through.
							corner[k] = cornerColor(vc[k], engine.RGB(255, 255, 255), true)
						} else {
							corner[k] = cornerColor(vc[k], tri.color, hasFaceColor)
						}
					}
				}
			}
			cut = varying && (crossesFar || v0.Z < cam.Near || v1.Z < cam.Near || v2.Z < cam.Near)
		}

		a, b, c := clipVert{view: v0}, clipVert{view: v1}, clipVert{view: v2}
		if smooth {
			a.world, b.world, c.world = w0, w1, w2
			a.normal = cornerNormal(normals[f[0]], normal, front)
			b.normal = cornerNormal(normals[f[1]], normal, front)
			c.normal = cornerNormal(normals[f[2]], normal, front)
		} else {
			center := w0.Add(w1).Add(w2).Scale(1.0 / 3)
			tri.intensity = Brightness(lit, center, normal)
		}

		n := clipTriangleNear(a, b, c, cam.Near, smooth, &tris)
		pieces := tris[:n]
		if crossesFar {
			pieces = s.far[:clipTrianglesFar(pieces, cam.Far, smooth, &s.far)]
		}
		for _, t := range pieces {
			for i, v := range t {
				x, y := proj.project(v.view)
				tri.corners[i] = screenPoint{x, y, v.view.Z}
				if smooth {
					tri.world[i], tri.normal[i] = v.world, v.normal
				}
				if !varying {
					continue
				}
				// An uncut triangle comes out with its corners as they went in. A cut one has
				// corners on the near or far plane, given the color and the place in the picture
				// that the face has there.
				if !cut {
					tri.rgb[i], tri.uv[i] = corner[i], faceUV[i]
					continue
				}
				if tri.tinted {
					tri.rgb[i] = colorAtPoint(corner, v0, v1, v2, v.view)
				}
				if tri.tex != nil {
					p0, p1, p2 := pointWeights(v0, v1, v2, v.view)
					tri.uv[i] = faceUV[0].Scale(p0).Add(faceUV[1].Scale(p1)).Add(faceUV[2].Scale(p2))
				}
			}
			fn(tri)
		}
	}
}

// DrawShaded renders the mesh's faces as solid triangles, shaded by the lights and
// drawn in the mesh's Material. Nearer surfaces hide farther ones, faces turned away
// from the camera are skipped (unless the mesh is DoubleSided), and faces crossing
// the near plane, or the far plane of a camera that has one, are cut where they cross
// it. A mesh without Faces draws nothing; use Draw for wireframes.
//
// A mesh without Normals is shaded flat, each face lit as a whole. One with Normals
// is shaded smooth, every cell lit on its own; see Mesh.Normals.
//
// A face with texture coordinates takes the color of each cell from the Material.Texture at
// that place on the face (see Mesh.FaceTexCoords), and a face with vertex colors fades from one
// to the next, or both, the colors tinting the picture. A hole in the picture draws nothing.
//
// A mesh with a Material.Opacity between 0 and 1 is mixed over what is already drawn, in place of
// covering it, and does not store its depth: draw the solid meshes first, and the see-through ones
// from far to near. In cells the character behind it stays and its colors are mixed.
//
// Brightness is shown by the character: a ramp from dim to bright, Material.Ramp. A camera
// with a Fog fades the color of each cell toward the fog's, by how deep the cell is, and the
// character stays.
func (m Mesh) DrawShaded(fb *engine.FrameBuffer, cam Camera, lights ...Light) {
	rs := getScratch(0)
	defer rs.release()
	ramp := rs.rampFor(m.Material)
	fog := cam.Fog
	m.eachShadedTriangle(cam, lights, fb.Width(), fb.Height(), func(t *shadedTriangle) {
		c := &t.corners
		if t.varies() {
			fillTinted(fb, t, ramp, fog)
			return
		}
		if fog != nil {
			fillFogged(fb, t, ramp, fog)
			return
		}
		if !t.smooth {
			cell := engine.Cell{Character: shadeChar(ramp, t.intensity), Foreground: t.color}
			fb.FillTriangle(c[0].x, c[0].y, c[0].z, c[1].x, c[1].y, c[1].z, c[2].x, c[2].y, c[2].z, cell)
			return
		}
		fb.FillTriangleFunc(c[0].x, c[0].y, c[0].z, c[1].x, c[1].y, c[1].z, c[2].x, c[2].y, c[2].z, func(b0, b1, b2 float64) engine.Cell {
			return engine.Cell{Character: shadeChar(ramp, t.brightnessAt(b0, b1, b2)), Foreground: t.color}
		})
	})
}

// DrawShadedPixels is DrawShaded for a pixel Canvas: the same faces, culling,
// clipping, depth and smoothing rules, but brightness is shown by the color (a darker
// or lighter shade of Material.Color) and there are twice as many rows, so edges are
// finer. Call Canvas.Compose afterwards to put the pixels into the FrameBuffer. A camera with
// a Fog fades the shaded color of each pixel toward the fog's, by how deep the pixel is. A face
// with texture coordinates is drawn in the pixels of its picture, one for each pixel of the
// canvas, so a picture of 16 by 16 on a wall that fills a screen is seen as 16 by 16 blocks. A mesh
// with a Material.Opacity between 0 and 1 mixes its color with each pixel that is already there, and
// with black over an empty one, and does not store depth.
func (m Mesh) DrawShadedPixels(c *engine.Canvas, cam Camera, lights ...Light) {
	// Project onto the cell grid the canvas belongs to, then stretch y to pixel rows.
	fog := cam.Fog
	m.eachShadedTriangle(cam, lights, c.Width(), c.Height()/2, func(t *shadedTriangle) {
		p := &t.corners
		if t.varies() {
			fillTintedPixels(c, t, fog)
			return
		}
		if fog != nil {
			fillFoggedPixels(c, t, fog)
			return
		}
		if !t.smooth {
			c.FillTriangle(p[0].x, 2*p[0].y, p[0].z, p[1].x, 2*p[1].y, p[1].z, p[2].x, 2*p[2].y, p[2].z, engine.Shade(t.color, t.intensity))
			return
		}
		c.FillTriangleFunc(p[0].x, 2*p[0].y, p[0].z, p[1].x, 2*p[1].y, p[1].z, p[2].x, 2*p[2].y, p[2].z, func(b0, b1, b2 float64) engine.Color {
			return engine.Shade(t.color, t.brightnessAt(b0, b1, b2))
		})
	})
}

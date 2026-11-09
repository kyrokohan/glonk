package mesh

import (
	"math"
	"slices"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// Mesh describes an object that holds
// model-space vertices, edges between them, triangle faces,
// its placement in the world and its look.
//
// Edges drive wireframe drawing and Faces drive shaded drawing; a mesh can have
// either or both. Faces index into Vertices and are wound so the face normal,
// (b-a)x(c-a), points out of the surface.
type Mesh struct {
	// Vertices are the points of the model, in model space: the space before
	// Transform is applied.
	Vertices []geometry.Vector3

	// Edges are pairs of indices into Vertices. Draw draws each as a line.
	Edges [][2]int

	// Faces are triples of indices into Vertices. DrawShaded fills each as a
	// triangle.
	Faces [][3]int

	// Normals are the directions the surface faces at each vertex, in model space,
	// and turn shaded drawing smooth. With none (the default) every face is lit as
	// a whole, one brightness per face, so a round shape shows its facets. With one
	// normal per vertex the lights are asked about every cell instead, using the
	// normal and position blended across the face from its corners, so a round
	// shape looks round. Normals can be any length: a zero normal means "none
	// here" and that corner takes the face's own normal.
	//
	// Where a surface has a hard edge, as a cube does, give each side its own
	// vertices so that each can face its own way; SmoothNormals averages across
	// shared vertices only. A list of any other length, or with a number that is
	// not finite, is not used (the mesh is shaded flat) and Validate reports it.
	// Normals follow Transform: turned with the model, unaffected by its scale.
	Normals []geometry.Vector3

	// FaceColors are the colors of the faces, in the order of Faces: one for each, or
	// none. A face's color is the color of its surface where the light meets it
	// fully, as Material.Color is for the whole mesh, and replaces it for that face;
	// brightness is still shown by the character or the shade. A zero entry means
	// "none here", and that face takes Material.Color. A list of any other length is
	// not used (every face takes Material.Color) and Validate reports it. LoadOBJFS
	// fills it from a model's materials.
	FaceColors []engine.Color

	// Transform places the model in the world.
	Transform Transform

	// Material is how the model looks when it is drawn shaded.
	Material Material

	// DoubleSided draws faces from behind as well as from the front, lit as the
	// side the camera sees. Use it for models wound the wrong way round (they
	// look inside out or vanish), and for open surfaces such as a flat leaf.
	// It only affects shaded drawing, not wireframes.
	DoubleSided bool
}

func corner(i, bit int, size float64) float64 {
	if i&bit == 0 {
		return -size / 2
	}
	return size / 2
}

// clipNear returns where the segment from out to in crosses z = near.
// out must be behind the plane (Z < near) and in on or in front of it,
// so the divisor is never zero.
func clipNear(out, in geometry.Vector3, near float64) geometry.Vector3 {
	t := (near - out.Z) / (in.Z - out.Z)
	p := out.Lerp(in, t)
	// Lerp rounding can leave Z a hair under near, which projectView does not tolerate.
	p.Z = near
	return p
}

// Cube returns a cube with edges of length
// size, centered on the origin.
func Cube(size float64) Mesh {
	vertices := make([]geometry.Vector3, 0, 8)

	for i := range 8 {
		x := corner(i, 1, size)
		y := corner(i, 2, size)
		z := corner(i, 4, size)
		v3 := geometry.Vector3{X: x, Y: y, Z: z}
		vertices = append(vertices, v3)
	}

	var edges [][2]int
	for i := range 8 {
		for _, b := range []int{1, 2, 4} {
			j := i | b
			if j != i {
				edges = append(edges, [2]int{i, j})
			}
		}
	}

	// Vertex i sits at +size/2 on x, y and z when bit 1, 2 and 4 of i is set.
	// Each side is two triangles, listed by the quad's corners in outward order.
	var faces [][3]int
	for _, quad := range [][4]int{
		{0, 4, 6, 2}, // -x
		{1, 3, 7, 5}, // +x
		{0, 1, 5, 4}, // -y
		{2, 6, 7, 3}, // +y
		{0, 2, 3, 1}, // -z
		{4, 5, 7, 6}, // +z
	} {
		faces = append(faces, [3]int{quad[0], quad[1], quad[2]}, [3]int{quad[0], quad[2], quad[3]})
	}

	return Mesh{Vertices: vertices, Edges: edges, Faces: faces}
}

// Clone returns a copy of the mesh that shares nothing with it, so changing the
// copy's Vertices, Edges, Faces, Normals or FaceColors leaves the original alone.
// Assigning a Mesh to a new variable copies the struct but not these lists, which
// both would still share.
func (m Mesh) Clone() Mesh {
	out := m
	out.Vertices = slices.Clone(m.Vertices)
	out.Edges = slices.Clone(m.Edges)
	out.Faces = slices.Clone(m.Faces)
	out.Normals = slices.Clone(m.Normals)
	out.FaceColors = slices.Clone(m.FaceColors)
	return out
}

// validIndex reports whether i names one of n items.
func validIndex(i, n int) bool {
	return i >= 0 && i < n
}

// Draw renders the mesh's edges as lines, all in cell. A mesh whose Transform is
// not valid is not drawn.
// Edges crossing the near plane are cut where they cross it;
// edges entirely behind it are skipped, and so are edges that name a vertex the
// mesh does not have (Validate finds them). An edge that ends far off screen
// costs no more than one that stays on screen.
//
// Draw ignores depth: the lines land on top of whatever is already drawn, so a
// wireframe shows through solids, and it neither hides nor is hidden by anything drawn
// after it. Use DrawDepth for lines that are hidden by nearer surfaces.
func (m Mesh) Draw(fb *engine.FrameBuffer, cam Camera, cell engine.Cell) {
	m.drawEdges(fb, cam, cell, false)
}

// DrawDepth is Draw with depth: an edge is drawn where it is nearer than what is
// already there, and a surface or line drawn later hides it where that is nearer. So
// the order of drawing does not matter, a wireframe behind a solid is hidden by it, and
// it hides what is behind it. Solids drawn into a Canvas hide it too, once Canvas.Compose
// has been called.
//
// The depth of each cell is perspective-correct along the edge (see
// FrameBuffer.DrawLineDepth), and the edges are stored with their depth, so a farther
// solid drawn later does not cover them. A tie keeps what is there, so a line lying
// exactly on a surface, a grid on a floor, flickers between the two: lift it a little.
// Edges drawn over the mesh's own faces are best drawn with Draw, which does not depend
// on how those cells happen to compare.
func (m Mesh) DrawDepth(fb *engine.FrameBuffer, cam Camera, cell engine.Cell) {
	m.drawEdges(fb, cam, cell, true)
}

// drawEdges is Draw and DrawDepth.
func (m Mesh) drawEdges(fb *engine.FrameBuffer, cam Camera, cell engine.Cell, depth bool) {
	cam, ok := cam.withDefaults()
	if !ok || !m.Transform.valid() {
		return
	}

	eye := cam.viewTransform()
	proj := cam.projector(fb.Width(), fb.Height())
	if m.offscreen(eye, proj, cam.Near) {
		return
	}

	s := getScratch(len(m.Vertices))
	defer s.release()
	place := m.Transform.placement()
	view := s.view
	for i, v := range m.Vertices {
		view[i] = eye.apply(place.apply(v))
	}

	for _, e := range m.Edges {
		if !validIndex(e[0], len(view)) || !validIndex(e[1], len(view)) {
			continue
		}
		a, b := view[e[0]], view[e[1]]
		aIn := a.Z >= cam.Near
		bIn := b.Z >= cam.Near

		if !aIn && !bIn {
			continue
		}
		if !aIn {
			a = clipNear(a, b, cam.Near)
		}
		if !bIn {
			b = clipNear(b, a, cam.Near)
		}

		ax, ay := proj.project(a)
		bx, by := proj.project(b)
		if depth {
			drawEdgeDepth(fb, ax, ay, a.Z, bx, by, b.Z, cell)
		} else {
			drawEdge(fb, ax, ay, bx, by, cell)
		}
	}
}

// farEdge is how far from the origin, in cells, a projected point can be before
// drawEdge trims the edge to the screen. It is well inside the range DrawLine
// handles exactly.
const farEdge = 1e8

// trimEdge returns the part of the edge between two projected points that is worth
// handing to the line drawer, and whether there is any. A point almost on the camera's
// plane projects absurdly far away, or to infinity; such an edge is cut back to the
// screen first so that it keeps its direction instead of being bent by DrawLine's
// coordinate limit. An edge with a non-finite end cannot be drawn.
func trimEdge(w, h int, ax, ay, bx, by float64) (cx0, cy0, cx1, cy1 float64, ok bool) {
	for _, v := range [...]float64{ax, ay, bx, by} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, 0, 0, 0, false
		}
	}
	if math.Abs(ax) > farEdge || math.Abs(ay) > farEdge || math.Abs(bx) > farEdge || math.Abs(by) > farEdge {
		// A cell of margin keeps the cut ends outside the visible area.
		return clipSegment(ax, ay, bx, by, -1, -1, float64(w), float64(h))
	}
	return ax, ay, bx, by, true
}

// drawEdge draws the line between two projected points; see trimEdge.
func drawEdge(fb *engine.FrameBuffer, ax, ay, bx, by float64, cell engine.Cell) {
	cx0, cy0, cx1, cy1, ok := trimEdge(fb.Width(), fb.Height(), ax, ay, bx, by)
	if !ok {
		return
	}
	fb.DrawLine(int(math.Floor(cx0)), int(math.Floor(cy0)), int(math.Floor(cx1)), int(math.Floor(cy1)), cell)
}

// drawEdgeDepth is drawEdge for an edge whose ends are za and zb away from the camera.
// 1/z runs evenly along the edge on the screen, so where trimming has cut an end short,
// the depth there is worked out from how far along the edge the cut is, and the
// line drawer is handed the depth the edge has at the place it now starts and stops.
func drawEdgeDepth(fb *engine.FrameBuffer, ax, ay, za, bx, by, zb float64, cell engine.Cell) {
	cx0, cy0, cx1, cy1, ok := trimEdge(fb.Width(), fb.Height(), ax, ay, bx, by)
	if !ok {
		return
	}

	// How far along the edge a point on it is, from 0 at a to 1 at b, by the longer axis.
	along := func(x, y float64) float64 {
		dx, dy := bx-ax, by-ay
		switch {
		case math.Abs(dx) >= math.Abs(dy) && dx != 0:
			return (x - ax) / dx
		case math.Abs(dy) > math.Abs(dx):
			return (y - ay) / dy
		}
		return 0 // the edge is a point
	}
	// An end that trimming left alone is exactly a or b.
	t0, t1 := 0.0, 1.0
	if cx0 != ax || cy0 != ay {
		t0 = along(cx0, cy0)
	}
	if cx1 != bx || cy1 != by {
		t1 = along(cx1, cy1)
	}
	fb.DrawLineDepth(
		int(math.Floor(cx0)), int(math.Floor(cy0)), depthAlong(za, zb, t0),
		int(math.Floor(cx1)), int(math.Floor(cy1)), depthAlong(za, zb, t1),
		cell)
}

// depthAlong is the depth a fraction t of the way along an edge from depth za to depth
// zb, on the screen: 1/z is what runs evenly. The ends are exact.
func depthAlong(za, zb, t float64) float64 {
	switch {
	case t <= 0:
		return za
	case t >= 1:
		return zb
	}
	return 1 / ((1-t)/za + t/zb)
}

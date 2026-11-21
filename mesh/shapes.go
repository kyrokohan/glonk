package mesh

import (
	"math"

	"glonk.dev/geometry"
)

// The shapes below are solid: they have faces, so DrawShaded and DrawShadedPixels
// draw them, and edges that follow the form (rings, meridians, rims) for Draw. They
// come with Normals, so shaded drawing makes the round ones look round, and with
// hard edges where a surface has one: a cylinder's caps do not share vertices with its
// side, so each can face its own way. They stand on the y axis and are centered on the
// origin. Place and turn them with Mesh.Transform.
//
// They come with FaceTexCoords as well, so a picture set as Material.Texture is drawn on them,
// facing as on a Box: the round sides of Sphere, Cylinder and Cone wrap the picture round them as
// a map of the world is wrapped round a globe, upright and not mirrored seen from outside, with
// its middle on the front (+z, the way a model faces) and its left and right edges meeting at the
// back (-z). Down the picture is from the top of the shape to its bottom: pole to pole, rim to
// rim, tip to base. A pole, or the tip, takes the middle of each slice it meets, so the picture
// does not smear into a fan there. The caps of a Cylinder and the base of a Cone show no picture
// and are drawn in Material.Color, and a Plane shows the picture once, as a Box's Top shows its
// part. A picture repeated over a shape, a floor tiled ten times, is its coordinates scaled:
//
//	for i, t := range floor.FaceTexCoords {
//		floor.FaceTexCoords[i] = [3]geometry.Vector2{t[0].Scale(10), t[1].Scale(10), t[2].Scale(10)}
//	}
//
// Sizes that are negative or not a finite number count as 0, which gives a shape with
// no size: nothing to see shaded, and a dot as a wireframe. A count of segments or
// rings outside the range a shape takes is brought into it. Neither panics, and the
// result always passes Validate.

const (
	minSegments = 3
	minRings    = 2

	// maxDetail is the most segments or rings a shape takes. A 512 by 512 sphere has
	// a quarter of a million vertices; this keeps a careless argument from asking for
	// gigabytes.
	maxDetail = 512
)

// detail brings a count of segments or rings into the range lo to maxDetail.
func detail(n, lo int) int {
	return min(max(n, lo), maxDetail)
}

// extent cleans up a size: negative, NaN and infinite come out as 0.
func extent(v float64) float64 {
	if !(v > 0) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// circle returns the unit vector at the angle that is segment s of n around the y
// axis, starting at +x and turning toward +z.
func circle(s, n int) (cos, sin float64) {
	sin, cos = math.Sincos(2 * math.Pi * float64(s) / float64(n))
	return cos, sin
}

// around is how far across the picture segment s of n is on a round side: the front (+z), a
// quarter of the way round from +x where segment 0 is, is the middle of the picture, and the back
// (-z) is its edges. The far corner of the face from s to s+1 is around(s, n) + 1/n, past 1 at
// the back, where the picture repeats, so the face across the edges is drawn whole.
func around(s, n int) float64 {
	return math.Mod(float64(s)/float64(n)+0.25, 1)
}

// Sphere returns a sphere of the given radius made of rings bands from pole to pole
// and segments slices around, with one vertex at each pole. segments is brought into
// 3 to 512 and rings into 2 to 512. Its Normals point straight out of the middle, so
// shaded drawing makes it smooth. Its edges are the rings and the meridians.
//
// A sphere has segments*(rings-1)+2 vertices and 2*segments*(rings-1) faces.
func Sphere(radius float64, segments, rings int) Mesh {
	radius = extent(radius)
	segments, rings = detail(segments, minSegments), detail(rings, minRings)

	vertices := segments*(rings-1) + 2
	m := Mesh{
		Vertices: make([]geometry.Vector3, 0, vertices),
		Normals:  make([]geometry.Vector3, 0, vertices),
		Faces:    make([][3]int, 0, 2*segments*(rings-1)),
		Edges:    make([][2]int, 0, segments*(2*rings-1)),

		FaceTexCoords: make([][3]geometry.Vector2, 0, 2*segments*(rings-1)),
	}
	add := func(direction geometry.Vector3) {
		m.Vertices = append(m.Vertices, direction.Scale(radius))
		m.Normals = append(m.Normals, direction)
	}

	// Vertex 0 is the top pole, then the rings from top to bottom, then the bottom pole.
	add(geometry.Vector3{Y: 1})
	for r := 1; r < rings; r++ {
		sinT, cosT := math.Sincos(math.Pi * float64(r) / float64(rings))
		for s := range segments {
			cosP, sinP := circle(s, segments)
			add(geometry.Vector3{X: sinT * cosP, Y: cosT, Z: sinT * sinP})
		}
	}
	bottom := len(m.Vertices)
	add(geometry.Vector3{Y: -1})

	ring := func(r, s int) int { return 1 + (r-1)*segments + s%segments } // r from 1 to rings-1
	down := func(r int) float64 { return float64(r) / float64(rings) }    // how far down the picture ring r is
	for s := range segments {
		u0 := around(s, segments)
		u1, mid := u0+1/float64(segments), u0+0.5/float64(segments)
		m.Faces = append(m.Faces, [3]int{0, ring(1, s+1), ring(1, s)})
		m.FaceTexCoords = append(m.FaceTexCoords, [3]geometry.Vector2{{X: mid}, {X: u1, Y: down(1)}, {X: u0, Y: down(1)}})
		m.Edges = append(m.Edges, [2]int{0, ring(1, s)})
		for r := 1; r < rings-1; r++ {
			a, b, c, d := ring(r, s), ring(r+1, s), ring(r+1, s+1), ring(r, s+1)
			m.Faces = append(m.Faces, [3]int{a, c, b}, [3]int{a, d, c})
			m.FaceTexCoords = append(m.FaceTexCoords,
				[3]geometry.Vector2{{X: u0, Y: down(r)}, {X: u1, Y: down(r + 1)}, {X: u0, Y: down(r + 1)}},
				[3]geometry.Vector2{{X: u0, Y: down(r)}, {X: u1, Y: down(r)}, {X: u1, Y: down(r + 1)}})
			m.Edges = append(m.Edges, [2]int{a, b})
		}
		m.Faces = append(m.Faces, [3]int{ring(rings-1, s), ring(rings-1, s+1), bottom})
		m.FaceTexCoords = append(m.FaceTexCoords, [3]geometry.Vector2{{X: u0, Y: down(rings - 1)}, {X: u1, Y: down(rings - 1)}, {X: mid, Y: 1}})
		m.Edges = append(m.Edges, [2]int{ring(rings-1, s), bottom})
		for r := 1; r < rings; r++ {
			m.Edges = append(m.Edges, [2]int{ring(r, s), ring(r, s+1)})
		}
	}
	m.UpdateBounds()
	return m
}

// Cylinder returns a cylinder of the given radius standing on the y axis, height tall
// from -height/2 to height/2, with segments sides. segments is brought into 3 to 512.
// The side is smooth, with Normals pointing straight out from the axis; the two flat
// caps have their own vertices and face straight up and down, so the rims stay sharp.
// Its edges are the two rims and a line down each corner of the side.
//
// A cylinder has 4*segments+2 vertices and 4*segments faces.
func Cylinder(radius, height float64, segments int) Mesh {
	radius, height = extent(radius), extent(height)
	segments = detail(segments, minSegments)
	top, bottom := height/2, -height/2

	m := Mesh{
		Vertices: make([]geometry.Vector3, 0, 4*segments+2),
		Normals:  make([]geometry.Vector3, 0, 4*segments+2),
		Faces:    make([][3]int, 0, 4*segments),
		Edges:    make([][2]int, 0, 3*segments),

		FaceTexCoords: make([][3]geometry.Vector2, 0, 4*segments),
	}
	add := func(x, y, z float64, normal geometry.Vector3) int {
		m.Vertices = append(m.Vertices, geometry.Vector3{X: x, Y: y, Z: z})
		m.Normals = append(m.Normals, normal)
		return len(m.Vertices) - 1
	}

	// The side: the top rim, then the bottom rim, each facing straight out.
	sideTop := len(m.Vertices)
	for s := range segments {
		c, sn := circle(s, segments)
		add(radius*c, top, radius*sn, geometry.Vector3{X: c, Z: sn})
	}
	sideBottom := len(m.Vertices)
	for s := range segments {
		c, sn := circle(s, segments)
		add(radius*c, bottom, radius*sn, geometry.Vector3{X: c, Z: sn})
	}
	// The caps: a middle and a rim each, facing up and facing down.
	up, down := geometry.Vector3{Y: 1}, geometry.Vector3{Y: -1}
	capTop := add(0, top, 0, up)
	rimTop := len(m.Vertices)
	for s := range segments {
		c, sn := circle(s, segments)
		add(radius*c, top, radius*sn, up)
	}
	capBottom := add(0, bottom, 0, down)
	rimBottom := len(m.Vertices)
	for s := range segments {
		c, sn := circle(s, segments)
		add(radius*c, bottom, radius*sn, down)
	}

	for s := range segments {
		next := (s + 1) % segments
		t0, t1, b0, b1 := sideTop+s, sideTop+next, sideBottom+s, sideBottom+next
		m.Faces = append(m.Faces,
			[3]int{t0, b1, b0}, [3]int{t0, t1, b1},
			[3]int{capTop, rimTop + next, rimTop + s},
			[3]int{capBottom, rimBottom + s, rimBottom + next})
		u0 := around(s, segments)
		u1 := u0 + 1/float64(segments)
		m.FaceTexCoords = append(m.FaceTexCoords,
			[3]geometry.Vector2{{X: u0}, {X: u1, Y: 1}, {X: u0, Y: 1}},
			[3]geometry.Vector2{{X: u0}, {X: u1}, {X: u1, Y: 1}},
			[3]geometry.Vector2{}, [3]geometry.Vector2{}) // the caps show no picture
		m.Edges = append(m.Edges, [2]int{t0, t1}, [2]int{b0, b1}, [2]int{t0, b0})
	}
	m.UpdateBounds()
	return m
}

// Cone returns a cone of the given radius standing on the y axis, height tall from
// -height/2 (the base) to height/2 (the point), with segments sides. segments is
// brought into 3 to 512. The side is smooth, with Normals that lean up along the
// slope; the point has a vertex and a normal for each side, since it faces every way
// at once; the flat base has its own vertices and faces straight down, so its rim
// stays sharp. Its edges are the rim and a line from it to the point at each corner.
//
// A cone has 3*segments+1 vertices and 2*segments faces.
func Cone(radius, height float64, segments int) Mesh {
	radius, height = extent(radius), extent(height)
	segments = detail(segments, minSegments)
	apex, base := height/2, -height/2

	// slope is the unit normal of the side at angle (c, sn): it leans up as far as the
	// side leans in. A cone with no size at all has no normal, which is allowed.
	slope := func(c, sn float64) geometry.Vector3 {
		n, _ := unitDirection(geometry.Vector3{X: height * c, Y: radius, Z: height * sn})
		return n
	}

	m := Mesh{
		Vertices: make([]geometry.Vector3, 0, 3*segments+1),
		Normals:  make([]geometry.Vector3, 0, 3*segments+1),
		Faces:    make([][3]int, 0, 2*segments),
		Edges:    make([][2]int, 0, 2*segments),

		FaceTexCoords: make([][3]geometry.Vector2, 0, 2*segments),
	}
	add := func(v, normal geometry.Vector3) int {
		m.Vertices = append(m.Vertices, v)
		m.Normals = append(m.Normals, normal)
		return len(m.Vertices) - 1
	}

	rim := len(m.Vertices)
	for s := range segments {
		c, sn := circle(s, segments)
		add(geometry.Vector3{X: radius * c, Y: base, Z: radius * sn}, slope(c, sn))
	}
	tip := len(m.Vertices)
	for s := range segments {
		// The side from corner s to corner s+1 faces the way between them.
		sinM, cosM := math.Sincos(2 * math.Pi * (float64(s) + 0.5) / float64(segments))
		add(geometry.Vector3{Y: apex}, slope(cosM, sinM))
	}
	down := geometry.Vector3{Y: -1}
	middle := add(geometry.Vector3{Y: base}, down)
	capRim := len(m.Vertices)
	for s := range segments {
		c, sn := circle(s, segments)
		add(geometry.Vector3{X: radius * c, Y: base, Z: radius * sn}, down)
	}

	for s := range segments {
		next := (s + 1) % segments
		m.Faces = append(m.Faces,
			[3]int{rim + s, tip + s, rim + next},
			[3]int{middle, capRim + s, capRim + next})
		u0 := around(s, segments)
		m.FaceTexCoords = append(m.FaceTexCoords,
			[3]geometry.Vector2{{X: u0, Y: 1}, {X: u0 + 0.5/float64(segments)}, {X: u0 + 1/float64(segments), Y: 1}},
			[3]geometry.Vector2{}) // the base shows no picture
		m.Edges = append(m.Edges, [2]int{rim + s, rim + next}, [2]int{rim + s, tip + s})
	}
	m.UpdateBounds()
	return m
}

// Plane returns a flat rectangle width along x and depth along z, in the y=0 plane
// and centered on the origin, facing up (+y). It is two triangles: with Normals,
// shaded drawing lights every cell on it, so a lamp near it makes a pool of light
// without cutting the plane up. Its edges are its four sides. It is drawn from above
// only unless DoubleSided is set. For a floor of lines, see Grid.
func Plane(width, depth float64) Mesh {
	hw, hd := extent(width)/2, extent(depth)/2
	up := geometry.Vector3{Y: 1}
	m := Mesh{
		Vertices: []geometry.Vector3{{X: -hw, Z: -hd}, {X: -hw, Z: hd}, {X: hw, Z: hd}, {X: hw, Z: -hd}},
		Normals:  []geometry.Vector3{up, up, up, up},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		Edges:    [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}},
		// As a Box's Top: the picture's top toward -z and its right toward -x, seen from above,
		// so corner 3 (+x, -z) is its top left.
		FaceTexCoords: [][3]geometry.Vector2{
			{{X: 1}, {X: 1, Y: 1}, {Y: 1}},
			{{X: 1}, {Y: 1}, {}},
		},
	}
	m.UpdateBounds()
	return m
}

// BoxParts says which part of a mesh's picture each face of a box shows, as a rectangle in
// fractions of the picture: (0, 0) is its top left and (1, 1) its bottom right, as for
// FaceTexCoords. A field left zero shows the whole picture. A part whose Max is left of or above
// its Min shows the picture mirrored that way, and one that reaches past 1 repeats it.
//
// Front faces +z, the way a model faces (see Transform), so a camera looking down +z at a box
// sees its Back. On the four sides, the top of a part is up (+y) and it is not mirrored, seen from
// outside. The Top and the Bottom read as if unfolded from the Front: the Top's part has its bottom
// edge along the front of the box, and the Bottom's part has its top edge there.
//
// A box far away reads smaller copies of the picture (see Material.Texture), in which the parts
// blur into each other across their edges, so a far die's faces run together; Material.Sharp
// keeps them apart.
type BoxParts struct {
	// Front, Back, Left, Right, Top and Bottom are the parts of the faces toward +z, -z, -x, +x,
	// +y and -y.
	Front, Back, Left, Right, Top, Bottom geometry.Rect
}

// Box returns a box width along x, height along y and depth along z, centered on the origin, with
// the whole of the mesh's picture on each face. It is BoxFaces with every part left zero; see it
// for how the box is made.
func Box(width, height, depth float64) Mesh {
	return BoxFaces(width, height, depth, BoxParts{})
}

// BoxFaces returns a box width along x, height along y and depth along z, centered on the origin,
// whose faces show the parts of the mesh's picture that parts says (see BoxParts): a crate, a die,
// a block of a world. The picture is Material.Texture; without one the box is a plain solid.
//
// The faces come in the order Front, Back, Left, Right, Top, Bottom. Face i is the vertices
// Vertices[4i] to Vertices[4i+3], its bottom left, top left, top right and bottom right corners
// as seen from outside, and the triangles Faces[2i] and Faces[2i+1]. That is 24 vertices, with
// Normals facing straight out, so its edges stay hard and a lamp lights each face on its own, and
// 12 faces and 12 edges. Cube makes a box of 8 vertices with no picture.
func BoxFaces(width, height, depth float64, parts BoxParts) Mesh {
	hx, hy, hz := extent(width)/2, extent(height)/2, extent(depth)/2
	x, y, z := geometry.Vector3{X: 1}, geometry.Vector3{Y: 1}, geometry.Vector3{Z: 1}
	half := func(axis geometry.Vector3) float64 { // half the box's size along a unit axis
		return math.Abs(axis.X)*hx + math.Abs(axis.Y)*hy + math.Abs(axis.Z)*hz
	}

	m := Mesh{
		Vertices:      make([]geometry.Vector3, 0, 24),
		Normals:       make([]geometry.Vector3, 0, 24),
		Faces:         make([][3]int, 0, 12),
		FaceTexCoords: make([][3]geometry.Vector2, 0, 12),
		Edges:         make([][2]int, 0, 12),
	}
	// Each face is its outward normal n and the way up its part is drawn, u; its right, seen from
	// outside, is n × u.
	for _, f := range []struct {
		n, u geometry.Vector3
		part geometry.Rect
	}{
		{z, y, parts.Front},
		{z.Scale(-1), y, parts.Back},
		{x.Scale(-1), y, parts.Left},
		{x, y, parts.Right},
		{y, z.Scale(-1), parts.Top},
		{y.Scale(-1), z, parts.Bottom},
	} {
		r := f.n.Cross(f.u)
		c, rr, uu := f.n.Scale(half(f.n)), r.Scale(half(r)), f.u.Scale(half(f.u))
		i := len(m.Vertices)
		for _, corner := range []geometry.Vector3{
			c.Sub(rr).Sub(uu), c.Sub(rr).Add(uu), c.Add(rr).Add(uu), c.Add(rr).Sub(uu), // bottom left, top left, top right, bottom right
		} {
			m.Vertices = append(m.Vertices, corner)
			m.Normals = append(m.Normals, f.n)
		}
		p := f.part
		if p == (geometry.Rect{}) {
			p = geometry.Rect{Max: geometry.Vector2{X: 1, Y: 1}}
		}
		bl, tl, tr, br := geometry.Vector2{X: p.Min.X, Y: p.Max.Y}, p.Min, geometry.Vector2{X: p.Max.X, Y: p.Min.Y}, p.Max
		m.Faces = append(m.Faces, [3]int{i, i + 1, i + 2}, [3]int{i, i + 2, i + 3})
		m.FaceTexCoords = append(m.FaceTexCoords, [3]geometry.Vector2{bl, tl, tr}, [3]geometry.Vector2{bl, tr, br})
	}

	// The 4 sides of the Front (vertices 0 to 3) and of the Back (4 to 7), and the 4 edges that
	// join them. The Front's right is -x and the Back's +x, so the Front's bottom left is where the
	// Back's bottom right is, and so on round.
	m.Edges = append(m.Edges,
		[2]int{0, 1}, [2]int{1, 2}, [2]int{2, 3}, [2]int{3, 0},
		[2]int{4, 5}, [2]int{5, 6}, [2]int{6, 7}, [2]int{7, 4},
		[2]int{0, 7}, [2]int{1, 6}, [2]int{2, 5}, [2]int{3, 4},
	)
	m.UpdateBounds()
	return m
}

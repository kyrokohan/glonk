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
	for s := range segments {
		m.Faces = append(m.Faces, [3]int{0, ring(1, s+1), ring(1, s)})
		m.Edges = append(m.Edges, [2]int{0, ring(1, s)})
		for r := 1; r < rings-1; r++ {
			a, b, c, d := ring(r, s), ring(r+1, s), ring(r+1, s+1), ring(r, s+1)
			m.Faces = append(m.Faces, [3]int{a, c, b}, [3]int{a, d, c})
			m.Edges = append(m.Edges, [2]int{a, b})
		}
		m.Faces = append(m.Faces, [3]int{ring(rings-1, s), ring(rings-1, s+1), bottom})
		m.Edges = append(m.Edges, [2]int{ring(rings-1, s), bottom})
		for r := 1; r < rings; r++ {
			m.Edges = append(m.Edges, [2]int{ring(r, s), ring(r, s+1)})
		}
	}
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
		m.Edges = append(m.Edges, [2]int{t0, t1}, [2]int{b0, b1}, [2]int{t0, b0})
	}
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
		m.Edges = append(m.Edges, [2]int{rim + s, rim + next}, [2]int{rim + s, tip + s})
	}
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
	return Mesh{
		Vertices: []geometry.Vector3{{X: -hw, Z: -hd}, {X: -hw, Z: hd}, {X: hw, Z: hd}, {X: hw, Z: -hd}},
		Normals:  []geometry.Vector3{up, up, up, up},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		Edges:    [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}},
	}
}

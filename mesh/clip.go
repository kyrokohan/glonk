package mesh

import (
	"math"

	"glonk.dev/geometry"
)

// clipVert is a corner of a triangle being clipped: where it is in view space and,
// for smooth shading, where it is in the world and which way the surface faces there.
type clipVert struct {
	view, world, normal geometry.Vector3
}

// crossNear returns the point where the edge from out to in crosses z = near, with the
// world position and normal carried along when withAttrs is set. out must be behind
// the plane and in on or in front of it.
func crossNear(out, in clipVert, near float64, withAttrs bool) clipVert {
	v := clipVert{view: clipNear(out.view, in.view, near)}
	if withAttrs {
		t := (near - out.view.Z) / (in.view.Z - out.view.Z)
		v.world = out.world.Lerp(in.world, t)
		v.normal = out.normal.Lerp(in.normal, t)
	}
	return v
}

// clipTriangleNear cuts the view-space triangle a, b, c against the plane z = near and
// writes the part in front of it into out as zero, one or two triangles, returning
// how many. Winding is preserved, and every written vertex has Z >= near exactly. It
// allocates nothing.
func clipTriangleNear(a, b, c clipVert, near float64, withAttrs bool, out *[2][3]clipVert) int {
	in := func(v clipVert) bool { return v.view.Z >= near }

	// Common cases first: the triangle is wholly in front or wholly behind.
	switch {
	case in(a) && in(b) && in(c):
		out[0] = [3]clipVert{a, b, c}
		return 1
	case !in(a) && !in(b) && !in(c):
		return 0
	}

	// Sutherland-Hodgman against one plane: walk the edges, keep the vertices in
	// front, and add the crossing point wherever an edge changes sides. Two corners in
	// front and two crossings is the most that can come out.
	tri := [3]clipVert{a, b, c}
	var poly [4]clipVert
	n := 0
	for i, cur := range tri {
		next := tri[(i+1)%3]

		if in(cur) {
			poly[n] = cur
			n++
		}
		switch {
		case in(cur) && !in(next):
			poly[n] = crossNear(next, cur, near, withAttrs)
			n++
		case !in(cur) && in(next):
			poly[n] = crossNear(cur, next, near, withAttrs)
			n++
		}
	}

	// A vertex lying exactly on the plane yields its own crossing point as well;
	// drop the repeats, wrap-around included.
	var clean [4]clipVert
	m := 0
	for i := range n {
		if poly[i].view != poly[(i+n-1)%n].view {
			clean[m] = poly[i]
			m++
		}
	}

	// Fan the polygon (three or four corners) into triangles.
	count := 0
	for i := 1; i+1 < m; i++ {
		out[count] = [3]clipVert{clean[0], clean[i], clean[i+1]}
		count++
	}
	return count
}

// clipSegment cuts the segment from (x0, y0) to (x1, y1) to the rectangle with
// corners (minX, minY) and (maxX, maxY), keeping its direction, and reports whether
// any of it is inside (Liang-Barsky). The ends it returns lie on the original
// segment. Non-finite input, and coordinates beyond +-1e150 whose differences could
// overflow, are rejected. Past about 1e15 the visible piece is smaller than a float
// can resolve, so it may come back as a point; it is always inside the rectangle.
func clipSegment(x0, y0, x1, y1, minX, minY, maxX, maxY float64) (cx0, cy0, cx1, cy1 float64, ok bool) {
	const limit = 1e150
	for _, v := range [...]float64{x0, y0, x1, y1} {
		if math.IsNaN(v) || math.Abs(v) > limit {
			return 0, 0, 0, 0, false
		}
	}

	dx, dy := x1-x0, y1-y0
	// Each border contributes p (how fast the segment moves toward it) and q (how
	// far the start is from it); the visible part is where all four allow it.
	p := [4]float64{-dx, dx, -dy, dy}
	q := [4]float64{x0 - minX, maxX - x0, y0 - minY, maxY - y0}

	t0, t1 := 0.0, 1.0
	for i := range p {
		if p[i] == 0 {
			if q[i] < 0 {
				return 0, 0, 0, 0, false // parallel to this border and outside it
			}
			continue
		}

		t := q[i] / p[i]
		if p[i] < 0 {
			if t > t1 {
				return 0, 0, 0, 0, false
			}
			t0 = max(t0, t)
		} else {
			if t < t0 {
				return 0, 0, 0, 0, false
			}
			t1 = min(t1, t)
		}
	}

	return x0 + t0*dx, y0 + t0*dy, x0 + t1*dx, y0 + t1*dy, true
}

package mesh

import (
	"math"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

func triArea(t [3]geometry.Vector3) float64 {
	return t[1].Sub(t[0]).Cross(t[2].Sub(t[0])).Length() / 2
}

func TestClipTriangleNear(t *testing.T) {
	const near = 0.1
	v := func(x, y, z float64) geometry.Vector3 { return geometry.Vector3{X: x, Y: y, Z: z} }

	tests := []struct {
		name     string
		a, b, c  geometry.Vector3
		wantTris int
		wantArea float64 // total area of the pieces; ignored when wantTris is 0
	}{
		{"all in front", v(0, 0, 2), v(2, 0, 3), v(0, 2, 3), 1, 0},
		{"all behind", v(0, 0, -2), v(2, 0, -1), v(0, 2, -3), 0, 0},
		// a is behind: the cut-off tip is a triangle scaled by t=0.275, so area 6*(1-0.275^2).
		{"one behind", v(0, 0, -1), v(2, 0, 3), v(0, 2, 3), 2, 6 * (1 - 0.275*0.275)},
		{"middle behind", v(2, 0, 3), v(0, 0, -1), v(0, 2, 3), 2, 6 * (1 - 0.275*0.275)},
		{"last behind", v(2, 0, 3), v(0, 2, 3), v(0, 0, -1), 2, 6 * (1 - 0.275*0.275)},
		// only a is in front: what remains is the tip scaled by t=0.725, area 6*0.725^2.
		{"two behind", v(0, 0, 3), v(2, 0, -1), v(0, 2, -1), 1, 6 * 0.725 * 0.725},
		{"two behind, first in front is last", v(2, 0, -1), v(0, 2, -1), v(0, 0, 3), 1, 6 * 0.725 * 0.725},
		{"vertex on the plane, others in front", v(0, 0, near), v(2, 0, 3), v(0, 2, 3), 1, 0},
		{"vertex on the plane, others behind", v(0, 0, near), v(2, 0, -1), v(0, 2, -1), 0, 0},
		{"edge on the plane, third behind", v(0, 0, near), v(2, 0, near), v(0, 2, -1), 0, 0},
		{"edge on the plane, third in front", v(0, 0, near), v(2, 0, near), v(0, 2, 3), 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clipTriangleNearList(tt.a, tt.b, tt.c, near)

			if len(got) != tt.wantTris {
				t.Fatalf("wanted %d triangles, got %d: %v", tt.wantTris, len(got), got)
			}

			orig := tt.b.Sub(tt.a).Cross(tt.c.Sub(tt.a))
			var area float64
			for _, tri := range got {
				for _, p := range tri {
					if p.Z < near {
						t.Errorf("wanted every vertex at Z>=%v, got %v", near, p)
					}
				}
				if triArea(tri) == 0 {
					t.Errorf("wanted no degenerate triangles, got %v", tri)
				}
				if n := tri[1].Sub(tri[0]).Cross(tri[2].Sub(tri[0])); n.Dot(orig) <= 0 {
					t.Errorf("wanted winding preserved, normal %v vs original %v", n, orig)
				}
				area += triArea(tri)
			}

			if tt.wantArea != 0 && !floatEquals(area, tt.wantArea) {
				t.Errorf("wanted total area %v, got %v", tt.wantArea, area)
			}
			if tt.wantTris == 1 && tt.wantArea == 0 && !floatEquals(area, triArea([3]geometry.Vector3{tt.a, tt.b, tt.c})) {
				t.Errorf("wanted the triangle unchanged, got area %v", area)
			}
		})
	}
}

// A vertex on the plane with one in front and one behind keeps a triangle: the
// on-plane vertex, the one in front, and the crossing of the remaining edge.
func TestClipTriangleNearVertexOnPlaneSplit(t *testing.T) {
	const near = 0.1
	a := geometry.Vector3{Z: near}
	b := geometry.Vector3{X: 2, Z: 3}
	c := geometry.Vector3{Y: 2, Z: -1}

	got := clipTriangleNearList(a, b, c, near)

	if len(got) != 1 {
		t.Fatalf("wanted 1 triangle, got %d: %v", len(got), got)
	}
	for _, p := range got[0] {
		if p.Z < near {
			t.Errorf("wanted every vertex at Z>=%v, got %v", near, p)
		}
	}
	if !slicesContains(got[0][:], a) || !slicesContains(got[0][:], b) {
		t.Errorf("wanted the on-plane and in-front vertices kept, got %v", got[0])
	}
}

func slicesContains(vs []geometry.Vector3, want geometry.Vector3) bool {
	for _, v := range vs {
		if v == want {
			return true
		}
	}
	return false
}

func TestClipSegment(t *testing.T) {
	// A 10x10 rectangle from (0,0) to (10,10).
	tests := []struct {
		name           string
		x0, y0, x1, y1 float64
		ok             bool
		want           [4]float64
	}{
		{"fully inside", 2, 3, 8, 7, true, [4]float64{2, 3, 8, 7}},
		{"touching corners", 0, 0, 10, 10, true, [4]float64{0, 0, 10, 10}},
		{"fully left", -5, 2, -1, 8, false, [4]float64{}},
		{"fully above", 2, -5, 8, -1, false, [4]float64{}},
		{"crosses the left edge", -10, 5, 5, 5, true, [4]float64{0, 5, 5, 5}},
		{"crosses the right edge", 5, 5, 20, 5, true, [4]float64{5, 5, 10, 5}},
		{"crosses both vertical edges", -10, 5, 20, 5, true, [4]float64{0, 5, 10, 5}},
		{"crosses the top edge", 5, -10, 5, 5, true, [4]float64{5, 0, 5, 5}},
		{"crosses both horizontal edges", 5, -10, 5, 20, true, [4]float64{5, 0, 5, 10}},
		{"diagonal through the whole rectangle", -5, -5, 15, 15, true, [4]float64{0, 0, 10, 10}},
		{"reversed direction", 15, 15, -5, -5, true, [4]float64{10, 10, 0, 0}},
		{"clips a slope", -10, 0, 10, 10, true, [4]float64{0, 5, 10, 10}},
		{"misses the corner", -5, 8, 2, 15, false, [4]float64{}},
		{"grazes the corner", -5, 5, 5, -5, true, [4]float64{0, 0, 0, 0}},
		{"grazes the corner, reversed", 5, -5, -5, 5, true, [4]float64{0, 0, 0, 0}},
		{"grazes the far corner", 15, 5, 5, 15, true, [4]float64{10, 10, 10, 10}},
		{"grazes the far corner, reversed", 5, 15, 15, 5, true, [4]float64{10, 10, 10, 10}},
		{"parallel just outside", -5, -1, 15, -1, false, [4]float64{}},
		{"parallel on the border", -5, 0, 15, 0, true, [4]float64{0, 0, 10, 0}},
		{"a point inside", 4, 4, 4, 4, true, [4]float64{4, 4, 4, 4}},
		{"a point outside", 14, 4, 14, 4, false, [4]float64{}},
		{"astronomical horizontal", -1e12, 5, 1e12, 5, true, [4]float64{0, 5, 10, 5}},
		{"astronomical keeps its slope", 5, 5, 1e12, 1e12, true, [4]float64{5, 5, 10, 10}},
		{"astronomical and off to the side", 20, 20, 1e12, 1e12, false, [4]float64{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x0, y0, x1, y1, ok := clipSegment(tt.x0, tt.y0, tt.x1, tt.y1, 0, 0, 10, 10)

			if ok != tt.ok {
				t.Fatalf("wanted ok=%v, got %v", tt.ok, ok)
			}
			if !ok {
				return
			}
			// Float error grows with the size of the input numbers.
			tol := 1e-9 + 1e-15*max(math.Abs(tt.x0), math.Abs(tt.y0), math.Abs(tt.x1), math.Abs(tt.y1))
			got := [4]float64{x0, y0, x1, y1}
			for i := range got {
				if math.Abs(got[i]-tt.want[i]) > tol {
					t.Errorf("wanted %v, got %v", tt.want, got)
					break
				}
			}
		})
	}
}

// Beyond float precision the visible piece is too small to resolve, but the answer
// must still lie inside the rectangle and never be garbage.
func TestClipSegmentAtTheEdgeOfFloatRange(t *testing.T) {
	for _, c := range [][4]float64{
		{-1e100, 5, 1e100, 5}, {5, -1e100, 5, 1e100}, {-1e149, -1e149, 1e149, 1e149}, {3, 4, 1e149, -1e149},
	} {
		x0, y0, x1, y1, ok := clipSegment(c[0], c[1], c[2], c[3], 0, 0, 10, 10)
		if !ok {
			continue
		}
		for _, v := range [...]float64{x0, y0, x1, y1} {
			if math.IsNaN(v) || v < -1e-6 || v > 10+1e-6 {
				t.Errorf("segment %v: wanted ends inside the rectangle, got (%v,%v)-(%v,%v)", c, x0, y0, x1, y1)
				break
			}
		}
	}
}

func TestClipSegmentRejectsBadNumbers(t *testing.T) {
	inf, nan := math.Inf(1), math.NaN()

	for _, c := range [][4]float64{
		{nan, 0, 5, 5}, {0, nan, 5, 5}, {0, 0, nan, 5}, {0, 0, 5, nan},
		{inf, 0, 5, 5}, {0, -inf, 5, 5}, {0, 0, inf, 5}, {0, 0, 5, -inf},
		{1e151, 5, 5, 5}, {0, 0, -1e151, 5},
	} {
		if _, _, _, _, ok := clipSegment(c[0], c[1], c[2], c[3], 0, 0, 10, 10); ok {
			t.Errorf("wanted %v rejected, got ok", c)
		}
	}
}

// An edge that projects absurdly far away must keep its direction on screen.
func TestDrawEdgeKeepsDirectionWhenFarAway(t *testing.T) {
	fb := newShadeBuffer(t)
	cell := engine.Cell{Character: '#'}

	// From the screen center toward a point 1e10 cells away along slope 0.3.
	drawEdge(fb, 40, 12, 1e10, 3e9+12, cell)

	cells := drawn(fb)
	if len(cells) < 20 {
		t.Fatalf("wanted a visible run of cells, got %d", len(cells))
	}
	for _, c := range cells {
		want := 12 + 0.3*float64(c[0]-40)
		if math.Abs(float64(c[1])-want) > 1.01 {
			t.Fatalf("wanted every cell within a cell of slope 0.3, got (%d,%d) vs y=%.1f", c[0], c[1], want)
		}
	}
}

func TestDrawEdgeIgnoresUndrawableEdges(t *testing.T) {
	inf, nan := math.Inf(1), math.NaN()
	cell := engine.Cell{Character: '#'}

	for _, c := range [][4]float64{
		{40, 12, inf, 12}, {40, 12, 40, -inf}, {nan, 12, 5, 5}, {40, 12, 5, nan},
		{1e12, 1e12, 2e12, 2e12}, // entirely far from the screen
		{-1e12, -50, -2e12, -60},
	} {
		fb := newShadeBuffer(t)
		drawEdge(fb, c[0], c[1], c[2], c[3], cell)

		if cells := drawn(fb); len(cells) != 0 {
			t.Errorf("wanted %v to draw nothing, got %d cells", c, len(cells))
		}
	}
}

// Edges of ordinary size take the exact path: same cells as DrawLine on the floors.
func TestDrawEdgeOrdinaryEdgesAreExact(t *testing.T) {
	cell := engine.Cell{Character: '#'}

	for _, c := range [][4]float64{{3.2, 4.7, 60.9, 20.1}, {-30.5, -10.5, 90.5, 40.5}, {40, 12, 40, 12}, {79.9, 23.9, 0.1, 0.1}} {
		a := newShadeBuffer(t)
		drawEdge(a, c[0], c[1], c[2], c[3], cell)

		b := newShadeBuffer(t)
		b.DrawLine(int(math.Floor(c[0])), int(math.Floor(c[1])), int(math.Floor(c[2])), int(math.Floor(c[3])), cell)

		for y := range shadeH {
			for x := range shadeW {
				if a.At(x, y) != b.At(x, y) {
					t.Fatalf("edge %v: wanted the same cells as DrawLine, differ at (%d,%d)", c, x, y)
				}
			}
		}
	}
}

// clipTriangleNearList is clipTriangleNear for tests: view-space corners in, the
// triangles that come out as a list.
func clipTriangleNearList(a, b, c geometry.Vector3, near float64) [][3]geometry.Vector3 {
	var out [2][3]clipVert
	n := clipTriangleNear(clipVert{view: a}, clipVert{view: b}, clipVert{view: c}, near, false, &out)
	var tris [][3]geometry.Vector3
	for _, t := range out[:n] {
		tris = append(tris, [3]geometry.Vector3{t[0].view, t[1].view, t[2].view})
	}
	return tris
}

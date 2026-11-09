package mesh

import (
	"math"
	"slices"
	"testing"

	"glonk.dev/geometry"
)

func TestBounds(t *testing.T) {
	v := func(x, y, z float64) geometry.Vector3 { return geometry.Vector3{X: x, Y: y, Z: z} }

	tests := []struct {
		name     string
		vertices []geometry.Vector3
		min, max geometry.Vector3
	}{
		{"empty", nil, geometry.Vector3{}, geometry.Vector3{}},
		{"one point", []geometry.Vector3{v(3, -4, 5)}, v(3, -4, 5), v(3, -4, 5)},
		{"two points", []geometry.Vector3{v(1, 5, -2), v(-3, 2, 4)}, v(-3, 2, -2), v(1, 5, 4)},
		{"each axis from a different vertex", []geometry.Vector3{v(0, 9, 0), v(9, 0, 0), v(0, 0, 9), v(-1, -2, -3)}, v(-1, -2, -3), v(9, 9, 9)},
		{"all negative", []geometry.Vector3{v(-5, -6, -7), v(-1, -2, -3)}, v(-5, -6, -7), v(-1, -2, -3)},
		{"first vertex is the extreme", []geometry.Vector3{v(100, 100, 100), v(1, 2, 3)}, v(1, 2, 3), v(100, 100, 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lo, hi := Mesh{Vertices: tt.vertices}.Bounds()

			if lo != tt.min || hi != tt.max {
				t.Errorf("wanted %v..%v, got %v..%v", tt.min, tt.max, lo, hi)
			}
		})
	}
}

func TestBoundsIgnoresPlacement(t *testing.T) {
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{X: 50, Y: 60, Z: 70}
	cube.Transform.Yaw, cube.Transform.Pitch = 1, 2

	lo, hi := cube.Bounds()
	if lo != (geometry.Vector3{X: -1, Y: -1, Z: -1}) || hi != (geometry.Vector3{X: 1, Y: 1, Z: 1}) {
		t.Errorf("wanted model-space bounds -1..1, got %v..%v", lo, hi)
	}
}

func boxMesh(x0, x1, y0, y1, z0, z1 float64) Mesh {
	return Mesh{
		Vertices: []geometry.Vector3{
			{X: x0, Y: y0, Z: z0}, {X: x1, Y: y0, Z: z0}, {X: x1, Y: y1, Z: z0}, {X: x0, Y: y1, Z: z0},
			{X: x0, Y: y0, Z: z1}, {X: x1, Y: y0, Z: z1}, {X: x1, Y: y1, Z: z1}, {X: x0, Y: y1, Z: z1},
		},
		Edges: [][2]int{{0, 1}, {1, 2}},
		Faces: [][3]int{{0, 1, 2}},
	}
}

func extents(m Mesh) (center, size geometry.Vector3) {
	lo, hi := m.Bounds()
	return lo.Add(hi).Scale(0.5), hi.Sub(lo)
}

func TestFit(t *testing.T) {
	tests := []struct {
		name string
		mesh Mesh
		size float64
		// the size the fitted mesh should have on each axis
		want geometry.Vector3
	}{
		{"off center, longest side is x", boxMesh(10, 14, -1, 1, 100, 101), 2, geometry.Vector3{X: 2, Y: 1, Z: 0.5}},
		{"longest side is y", boxMesh(0, 1, 0, 8, 0, 2), 4, geometry.Vector3{X: 0.5, Y: 4, Z: 1}},
		{"longest side is z", boxMesh(0, 1, 0, 2, 0, 10), 5, geometry.Vector3{X: 0.5, Y: 1, Z: 5}},
		{"scale up a tiny mesh", boxMesh(0, 1e-9, 0, 5e-10, 0, 1e-10), 5, geometry.Vector3{X: 5, Y: 2.5, Z: 0.5}},
		{"scale down a huge mesh", boxMesh(0, 1e9, 0, 5e8, 0, 1e8), 1, geometry.Vector3{X: 1, Y: 0.5, Z: 0.1}},
		{"already the right size", boxMesh(-1, 1, -1, 1, -1, 1), 2, geometry.Vector3{X: 2, Y: 2, Z: 2}},
		{"far from the origin", boxMesh(1e6, 1e6+4, 1e6, 1e6+2, 1e6, 1e6+1), 2, geometry.Vector3{X: 2, Y: 1, Z: 0.5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fitted := tt.mesh.Fit(tt.size)

			center, size := extents(fitted)
			tol := 1e-6 * tt.size
			if math.Abs(center.X) > tol || math.Abs(center.Y) > tol || math.Abs(center.Z) > tol {
				t.Errorf("wanted the center at the origin, got %v", center)
			}
			if math.Abs(size.X-tt.want.X) > tol || math.Abs(size.Y-tt.want.Y) > tol || math.Abs(size.Z-tt.want.Z) > tol {
				t.Errorf("wanted size %v, got %v", tt.want, size)
			}
		})
	}
}

func TestFitKeepsProportionsAndTopology(t *testing.T) {
	m := boxMesh(3, 9, -1, 5, 2, 4)
	fitted := m.Fit(3)

	if !slices.Equal(fitted.Edges, m.Edges) || !slices.Equal(fitted.Faces, m.Faces) {
		t.Errorf("wanted edges and faces unchanged, got %v and %v", fitted.Edges, fitted.Faces)
	}
	// Distances between any two vertices shrink by the same factor.
	factor := fitted.Vertices[0].Distance(fitted.Vertices[6]) / m.Vertices[0].Distance(m.Vertices[6])
	for i := 1; i < len(m.Vertices); i++ {
		got := fitted.Vertices[0].Distance(fitted.Vertices[i]) / m.Vertices[0].Distance(m.Vertices[i])
		if !floatEquals(got, factor) {
			t.Errorf("wanted an even scale of %v, vertex %d scaled by %v", factor, i, got)
		}
	}
}

func TestFitKeepsPlacement(t *testing.T) {
	m := Cube(4)
	m.Transform.Position = geometry.Vector3{X: 1, Y: 2, Z: 3}
	m.Transform.Yaw, m.Transform.Pitch = 0.5, -0.25

	fitted := m.Fit(2)

	if fitted.Transform != m.Transform {
		t.Errorf("wanted the Transform kept, got %+v, wanted %+v", fitted.Transform, m.Transform)
	}
}

func TestFitCubeToItself(t *testing.T) {
	cube := Cube(2)
	fitted := cube.Fit(2)

	for i := range cube.Vertices {
		if !vecEquals(fitted.Vertices[i], cube.Vertices[i]) {
			t.Errorf("wanted vertex %d unchanged, got %v vs %v", i, fitted.Vertices[i], cube.Vertices[i])
		}
	}
}

func TestFitIsIdempotent(t *testing.T) {
	once := boxMesh(10, 14, -3, 3, 100, 101).Fit(2)
	twice := once.Fit(2)

	for i := range once.Vertices {
		if !vecEquals(once.Vertices[i], twice.Vertices[i]) {
			t.Errorf("wanted fitting twice to change nothing, vertex %d moved from %v to %v", i, once.Vertices[i], twice.Vertices[i])
		}
	}
}

func TestFitDegenerateInput(t *testing.T) {
	point := Mesh{Vertices: []geometry.Vector3{{X: 5, Y: 6, Z: 7}, {X: 5, Y: 6, Z: 7}}}
	box := boxMesh(10, 14, 0, 2, 0, 2)
	nan, inf := math.NaN(), math.Inf(1)

	t.Run("empty mesh", func(t *testing.T) {
		fitted := Mesh{}.Fit(2)
		if len(fitted.Vertices) != 0 {
			t.Errorf("wanted an empty mesh, got %v", fitted.Vertices)
		}
	})

	t.Run("a single point is centered, not scaled", func(t *testing.T) {
		fitted := point.Fit(2)
		for _, v := range fitted.Vertices {
			if v != (geometry.Vector3{}) {
				t.Errorf("wanted the point moved to the origin, got %v", v)
			}
		}
	})

	for name, size := range map[string]float64{"zero": 0, "negative": -3, "NaN": nan, "infinite": inf, "negative infinite": -inf} {
		t.Run("size "+name+" centers without scaling", func(t *testing.T) {
			fitted := box.Fit(size)

			center, got := extents(fitted)
			_, want := extents(box)
			if math.Abs(center.X) > 1e-9 || got != want {
				t.Errorf("wanted the box centered at its own size %v, got center %v size %v", want, center, got)
			}
		})
	}

	t.Run("non-finite vertices come back unchanged", func(t *testing.T) {
		bad := Mesh{Vertices: []geometry.Vector3{{X: nan}, {X: 1, Y: 2, Z: 3}}}
		fitted := bad.Fit(2)

		if !math.IsNaN(fitted.Vertices[0].X) || fitted.Vertices[1] != bad.Vertices[1] {
			t.Errorf("wanted vertices untouched, got %v", fitted.Vertices)
		}

		huge := Mesh{Vertices: []geometry.Vector3{{X: -math.MaxFloat64}, {X: math.MaxFloat64}}}
		if fitted := huge.Fit(2); fitted.Vertices[0] != huge.Vertices[0] || fitted.Vertices[1] != huge.Vertices[1] {
			t.Errorf("wanted an overflowing extent left alone, got %v", fitted.Vertices)
		}
	})
}

func TestFitDoesNotAliasTheOriginal(t *testing.T) {
	m := boxMesh(0, 4, 0, 4, 0, 4)
	fitted := m.Fit(2)

	fitted.Vertices[0] = geometry.Vector3{X: 99}
	fitted.Edges[0] = [2]int{7, 7}
	fitted.Faces[0] = [3]int{7, 7, 7}

	if m.Vertices[0] != (geometry.Vector3{}) || m.Edges[0] != ([2]int{0, 1}) || m.Faces[0] != ([3]int{0, 1, 2}) {
		t.Errorf("wanted the original untouched, got %v %v %v", m.Vertices[0], m.Edges[0], m.Faces[0])
	}
}

func TestLoadOBJFitsLikeTheBuiltinCube(t *testing.T) {
	loaded := loadOBJ(t, objCube)
	lo, hi := loaded.Bounds()
	if !vecEquals(lo, geometry.Vector3{X: -1, Y: -1, Z: -1}) || !vecEquals(hi, geometry.Vector3{X: 1, Y: 1, Z: 1}) {
		t.Errorf("wanted bounds -1..1, got %v..%v", lo, hi)
	}
}

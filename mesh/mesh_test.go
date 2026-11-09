package mesh

import (
	"math"
	"math/bits"
	"slices"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

func TestCube(t *testing.T) {
	size := 2.00
	cube := Cube(size)

	vLen := len(cube.Vertices)
	if vLen != 8 {
		t.Errorf("wanted len(vertices)=8, got %d", vLen)
	}

	eLen := len(cube.Edges)
	if eLen != 12 {
		t.Errorf("wanted len(edges)=12, got %d", eLen)
	}

	seen := map[[2]int]bool{}

	for _, e := range cube.Edges {
		key := e
		if e[0] > e[1] {
			key[0] = e[1]
			key[1] = e[0]
		}
		if seen[key] {
			t.Errorf("duplicate edge %v", e)
		}
		seen[key] = true

		a := cube.Vertices[e[0]]
		b := cube.Vertices[e[1]]
		aDistB := a.Distance(b)
		if !floatEquals(aDistB, size) {
			t.Errorf("wanted edge distance=%f, got %v from edge %v", size, aDistB, e)
		}
	}
}

func TestCubeFaces(t *testing.T) {
	size := 2.0
	cube := Cube(size)

	if len(cube.Faces) != 12 {
		t.Fatalf("wanted 12 faces, got %d", len(cube.Faces))
	}

	// Each side of the cube must be covered by exactly two triangles, once.
	areaBySide := map[geometry.Vector3]float64{}
	trisBySide := map[geometry.Vector3][][3]int{}

	for _, f := range cube.Faces {
		for _, idx := range f {
			if idx < 0 || idx >= len(cube.Vertices) {
				t.Fatalf("wanted face indices in range, got %v", f)
			}
		}
		if f[0] == f[1] || f[1] == f[2] || f[0] == f[2] {
			t.Errorf("wanted three distinct corners, got %v", f)
		}

		a, b, c := cube.Vertices[f[0]], cube.Vertices[f[1]], cube.Vertices[f[2]]
		cross := b.Sub(a).Cross(c.Sub(a))
		normal := cross.Normalize()

		// The cube is centered on the origin, so outward means pointing away from it.
		if centroid := a.Add(b).Add(c).Scale(1.0 / 3); normal.Dot(centroid) <= 0 {
			t.Errorf("wanted face %v to point outward, normal %v at %v", f, normal, centroid)
		}

		// A cube side is axis aligned, so the normal is a unit axis.
		if !floatEquals(normal.Length(), 1) || !(floatEquals(math.Abs(normal.X)+math.Abs(normal.Y)+math.Abs(normal.Z), 1)) {
			t.Errorf("wanted an axis-aligned normal, got %v", normal)
		}

		areaBySide[normal] += cross.Length() / 2
		trisBySide[normal] = append(trisBySide[normal], f)
	}

	// Equal areas are not enough: two triangles that overlap also sum to the full
	// side. A real split shares only the quad's diagonal, whose corners differ in
	// two index bits (a quad edge differs in one).
	for normal, tris := range trisBySide {
		if len(tris) != 2 {
			t.Errorf("wanted 2 triangles on side %v, got %d", normal, len(tris))
			continue
		}
		var shared []int
		for _, i := range tris[0] {
			if slices.Contains(tris[1][:], i) {
				shared = append(shared, i)
			}
		}
		if len(shared) != 2 || bits.OnesCount(uint(shared[0]^shared[1])) != 2 {
			t.Errorf("wanted side %v split along its diagonal, shared corners %v", normal, shared)
		}
	}

	if len(areaBySide) != 6 {
		t.Errorf("wanted 6 distinct sides, got %d", len(areaBySide))
	}
	for normal, area := range areaBySide {
		if !floatEquals(area, size*size) {
			t.Errorf("wanted side %v to cover area %v, got %v", normal, size*size, area)
		}
	}
}

func TestMeshDrawBehindCamera(t *testing.T) {
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}

	fb, err := engine.NewFrameBuffer(80, 40)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}

	marker := engine.Cell{Character: '#'}

	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: -5}
	cube.Draw(fb, cam, marker)

	for y := range fb.Height() {
		for x := range fb.Width() {
			if fb.At(x, y) != engine.BlankCell {
				t.Errorf("wanted BlankCell, got %v at (%d, %d)", fb.At(x, y), x, y)
				return
			}
		}
	}
}

func TestMeshDrawInFront(t *testing.T) {
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}

	fb, err := engine.NewFrameBuffer(80, 40)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}

	marker := engine.Cell{Character: '#'}

	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 5}
	cube.Draw(fb, cam, marker)

	world := cube.Vertices[7].Add(cube.Transform.Position)
	x, y, ok := cam.Project(world, fb.Width(), fb.Height())
	if !ok {
		t.Fatalf("camera setup is incorrect")
	}
	cell := fb.At(int(math.Floor(x)), int(math.Floor(y)))
	if cell != marker {
		t.Errorf("wanted cell=%v, got %v", marker, cell)
	}
}

func TestClipNear(t *testing.T) {
	const near = 0.1

	tests := []struct {
		name    string
		out, in geometry.Vector3
		want    geometry.Vector3
	}{
		{
			"straight along z",
			geometry.Vector3{Z: -1}, geometry.Vector3{Z: 3},
			geometry.Vector3{Z: near},
		},
		{
			"diagonal",
			geometry.Vector3{X: -2, Y: 4, Z: -1}, geometry.Vector3{X: 2, Y: 0, Z: 3},
			geometry.Vector3{X: -0.9, Y: 2.9, Z: near},
		},
		{
			"in end exactly on the plane",
			geometry.Vector3{X: 1, Y: 1, Z: -1}, geometry.Vector3{X: 3, Y: 3, Z: near},
			geometry.Vector3{X: 3, Y: 3, Z: near},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clipNear(tt.out, tt.in, near)

			// Exact, not epsilon: projectView would misread a Z a hair under near.
			if got.Z != near {
				t.Errorf("wanted Z=%v exactly, got %v", near, got.Z)
			}
			if !vecEquals(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

func TestMeshDrawNearPlane(t *testing.T) {
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}
	marker := engine.Cell{Character: '#'}

	// All edges sit on the row at the screen's center (y=12) unless noted.
	tests := []struct {
		name  string
		verts []geometry.Vector3
		lit   func(x, y int) bool
	}{
		{
			// Far end lands at x=46; the cut end is far off the right edge.
			"first end behind",
			[]geometry.Vector3{{X: 1, Z: -1}, {X: 1, Z: 4}},
			func(x, y int) bool { return y == 12 && x >= 46 },
		},
		{
			"second end behind",
			[]geometry.Vector3{{X: 1, Z: 4}, {X: 1, Z: -1}},
			func(x, y int) bool { return y == 12 && x >= 46 },
		},
		{
			// Both ends exactly on the plane: dividing to cut them would give NaN.
			"edge lying on the near plane",
			[]geometry.Vector3{{X: -1, Z: cam.Near}, {X: 1, Z: cam.Near}},
			func(x, y int) bool { return y == 12 },
		},
		{
			// A cut here would extrapolate to the screen center and draw a stray cell.
			"edge entirely behind",
			[]geometry.Vector3{{Z: -1}, {Z: -2}},
			func(x, y int) bool { return false },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb, err := engine.NewFrameBuffer(80, 24)
			if err != nil {
				t.Fatalf("unable to initialize frame buffer: %v", err)
			}

			m := Mesh{Vertices: tt.verts, Edges: [][2]int{{0, 1}}}
			m.Draw(fb, cam, marker)

			for y := range fb.Height() {
				for x := range fb.Width() {
					want := engine.BlankCell
					if tt.lit(x, y) {
						want = marker
					}
					if got := fb.At(x, y); got != want {
						t.Fatalf("wanted %v at (%d,%d), got %v", want, x, y, got)
					}
				}
			}
		})
	}
}

package mesh

import (
	"math"
	"slices"
	"strings"
	"testing"

	"glonk.dev/geometry"
)

func v3(x, y, z float64) geometry.Vector3 { return geometry.Vector3{X: x, Y: y, Z: z} }

// A cube of 8 shared vertices comes out with each normal pointing out of its corner.
// Each side of the cube gives a corner a total of 90 degrees however it is cut into
// triangles, so the corners are exactly symmetric.
func TestSmoothNormalsOfACubePointOutOfTheCorners(t *testing.T) {
	cube := Cube(2)
	got := cube.SmoothNormals()

	if len(got.Normals) != 8 {
		t.Fatalf("wanted 8 normals, got %d", len(got.Normals))
	}
	for i, n := range got.Normals {
		want := cube.Vertices[i].Normalize()
		if !vecNear(n, want, 1e-12) {
			t.Errorf("vertex %d at %v: wanted the normal %v, got %v", i, cube.Vertices[i], want, n)
		}
	}
}

func TestSmoothNormalsOfAFlatSurfaceAreItsNormal(t *testing.T) {
	// A quad wound to face -z, cut along either diagonal.
	for _, faces := range [][][3]int{{{0, 1, 2}, {0, 2, 3}}, {{0, 1, 3}, {1, 2, 3}}} {
		m := Mesh{Vertices: []geometry.Vector3{v3(0, 0, 0), v3(0, 2, 0), v3(3, 2, 0), v3(3, 0, 0)}, Faces: faces}

		for i, n := range m.SmoothNormals().Normals {
			if !vecNear(n, v3(0, 0, -1), 1e-12) {
				t.Errorf("faces %v, vertex %d: wanted (0,0,-1), got %v", faces, i, n)
			}
		}
	}
}

// Two faces meet at the origin, one making a 90 degree angle there (facing +z) and one
// making a 45 degree angle (facing +x). Weighting by angle gives (45, 0, 90), not the
// plain average (1, 0, 1) that weighting by area or not at all would give.
func TestSmoothNormalsAreWeightedByAngle(t *testing.T) {
	m := Mesh{
		Vertices: []geometry.Vector3{v3(0, 0, 0), v3(1, 0, 0), v3(0, 1, 0), v3(0, 1, 1)},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
	}
	// The faces really are as described, with equal areas.
	if n := m.Vertices[1].Cross(m.Vertices[2]); !vecNear(n, v3(0, 0, 1), 1e-12) {
		t.Fatalf("wanted the first face to face +z, got %v", n)
	}
	if n := m.Vertices[2].Cross(m.Vertices[3]); !vecNear(n, v3(1, 0, 0), 1e-12) {
		t.Fatalf("wanted the second face to face +x, got %v", n)
	}

	got := m.SmoothNormals().Normals[0]

	want := v3(math.Pi/4, 0, math.Pi/2).Normalize()
	if !vecNear(got, want, 1e-12) {
		t.Errorf("wanted %v, got %v", want, got)
	}
}

// Cutting a face in two does not change how it is lit.
func TestSmoothNormalsDoNotDependOnHowFacesAreCut(t *testing.T) {
	whole := Mesh{
		Vertices: []geometry.Vector3{v3(0, 0, 0), v3(2, 0, 0), v3(0, 2, 0), v3(0, 1, 1)},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
	}
	// The first face split at the middle of its long side, a new vertex 4.
	split := Mesh{
		Vertices: append(slices.Clone(whole.Vertices), v3(1, 1, 0)),
		Faces:    [][3]int{{0, 1, 4}, {0, 4, 2}, {0, 2, 3}},
	}

	a, b := whole.SmoothNormals().Normals[0], split.SmoothNormals().Normals[0]

	if !vecNear(a, b, 1e-12) {
		t.Errorf("wanted the corner lit the same however its face is cut, got %v and %v", a, b)
	}
}

func TestSmoothNormalsLeaveOutFacesThatCannotBeUsed(t *testing.T) {
	good := Mesh{Vertices: []geometry.Vector3{v3(0, 0, 0), v3(0, 1, 0), v3(1, 0, 0), v3(5, 5, 5)}, Faces: [][3]int{{0, 1, 2}}}
	want := good.SmoothNormals().Normals

	bad := good.Clone()
	// 4 is NaN, 5 is infinite, and 6 and 7 are so far out that their cross product overflows.
	bad.Vertices = append(bad.Vertices, v3(math.NaN(), 0, 0), v3(math.Inf(1), 0, 0), v3(1e308, 1e308, 1e308), v3(-1e308, 1e308, 1e308))
	bad.Faces = append(bad.Faces,
		[3]int{0, 0, 1},  // two corners the same: no area
		[3]int{2, 2, 2},  // a point
		[3]int{0, 1, 99}, // names a vertex that does not exist
		[3]int{-1, 0, 1}, // and a negative one
		[3]int{0, 1, 4},
		[3]int{0, 1, 5},
		[3]int{0, 6, 7},
	)

	got := bad.SmoothNormals().Normals

	for i := range want {
		if !vecNear(got[i], want[i], 1e-12) {
			t.Errorf("vertex %d: wanted %v as if the unusable faces were not there, got %v", i, want[i], got[i])
		}
	}
	for i := len(want); i < len(got); i++ {
		if got[i] != (geometry.Vector3{}) {
			t.Errorf("vertex %d is used by no usable face: wanted a zero normal, got %v", i, got[i])
		}
	}
}

func TestSmoothNormalsAreUnitOrZero(t *testing.T) {
	m := Sphere(3, 9, 5)
	m.Vertices = append(m.Vertices, v3(100, 100, 100)) // used by nothing

	for i, n := range m.SmoothNormals().Normals {
		l := n.Length()
		if n != (geometry.Vector3{}) && math.Abs(l-1) > 1e-12 {
			t.Errorf("vertex %d: wanted a unit or zero normal, got %v of length %v", i, n, l)
		}
	}
	last := m.SmoothNormals().Normals[len(m.Vertices)-1]
	if last != (geometry.Vector3{}) {
		t.Errorf("wanted a zero normal for the vertex nothing uses, got %v", last)
	}
}

// On a sphere the normals found from the faces point out of the middle, close to the
// ones a sphere is made with.
func TestSmoothNormalsOfASphereAreCloseToTheTrueOnes(t *testing.T) {
	m := Sphere(2, 24, 12)
	smooth := m.SmoothNormals()

	for i, n := range smooth.Normals {
		if d := n.Dot(m.Normals[i]); d < 0.99 {
			t.Fatalf("vertex %d: wanted a normal within 8 degrees of %v, got %v", i, m.Normals[i], n)
		}
	}
}

func TestSmoothNormalsReplaceAndShareNothing(t *testing.T) {
	cube := Cube(1)
	cube.Normals = make([]geometry.Vector3, 3) // wrong length, to be replaced
	before := cube.Clone()

	got := cube.SmoothNormals()

	if len(got.Normals) != 8 || got.Validate() != nil {
		t.Errorf("wanted 8 valid normals replacing the old ones, got %d (%v)", len(got.Normals), got.Validate())
	}
	if len(cube.Normals) != 3 || !slices.Equal(cube.Vertices, before.Vertices) || !slices.Equal(cube.Faces, before.Faces) {
		t.Errorf("wanted the original left alone")
	}
	got.Vertices[0] = v3(9, 9, 9)
	got.Faces[0][0] = 7
	got.Normals[1] = v3(9, 9, 9)
	if cube.Vertices[0] == v3(9, 9, 9) || cube.Faces[0][0] == 7 {
		t.Errorf("wanted the copy to share nothing with the original")
	}
}

func TestSmoothNormalsKeepWhatElseTheMeshHas(t *testing.T) {
	cube := Cube(1)
	cube.Transform.Yaw = 0.5
	cube.Material.Ramp = "ab"
	cube.DoubleSided = true

	got := cube.SmoothNormals()

	if got.Transform != cube.Transform || got.Material != cube.Material || !got.DoubleSided || !slices.Equal(got.Edges, cube.Edges) {
		t.Errorf("wanted everything but the normals kept, got %+v", got)
	}
}

func TestSmoothNormalsOfNothing(t *testing.T) {
	for name, m := range map[string]Mesh{
		"empty":         {},
		"no faces":      {Vertices: []geometry.Vector3{v3(1, 2, 3)}},
		"only an edge":  {Vertices: []geometry.Vector3{v3(0, 0, 0), v3(1, 0, 0)}, Edges: [][2]int{{0, 1}}},
		"only bad face": {Vertices: []geometry.Vector3{v3(0, 0, 0)}, Faces: [][3]int{{0, 1, 2}}},
	} {
		t.Run(name, func(t *testing.T) {
			got := m.SmoothNormals()

			if len(got.Normals) != len(m.Vertices) {
				t.Errorf("wanted one normal per vertex, got %d for %d", len(got.Normals), len(m.Vertices))
			}
			for _, n := range got.Normals {
				if n != (geometry.Vector3{}) {
					t.Errorf("wanted zero normals, got %v", n)
				}
			}
			if got.Normals == nil && len(m.Vertices) > 0 {
				t.Errorf("wanted normals, got none")
			}
		})
	}
}

func TestValidateChecksNormals(t *testing.T) {
	cube := Cube(1)
	withNormals := cube.SmoothNormals()
	nan := withNormals.Clone()
	nan.Normals[3].Y = math.NaN()
	inf := withNormals.Clone()
	inf.Normals[0].X = math.Inf(-1)
	short := withNormals.Clone()
	short.Normals = short.Normals[:7]
	long := withNormals.Clone()
	long.Normals = append(long.Normals, geometry.Vector3{})
	zero := withNormals.Clone()
	zero.Normals[2] = geometry.Vector3{}
	huge := withNormals.Clone()
	huge.Normals[2] = v3(1e300, -1e300, 0)

	tests := []struct {
		name string
		m    Mesh
		want string // part of the error; empty for valid
	}{
		{"none", cube, ""},
		{"one for each vertex", withNormals, ""},
		{"a zero normal is allowed", zero, ""},
		{"a huge normal is allowed", huge, ""},
		{"too few", short, "7 normals for 8 vertices"},
		{"too many", long, "9 normals for 8 vertices"},
		{"NaN", nan, "normal 3 is not a finite number"},
		{"infinite", inf, "normal 0 is not a finite number"},
		{"normals but no vertices", Mesh{Normals: []geometry.Vector3{{}}}, "1 normals for 0 vertices"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()

			switch {
			case tt.want == "" && err != nil:
				t.Errorf("wanted a valid mesh, got %v", err)
			case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
				t.Errorf("wanted an error mentioning %q, got %v", tt.want, err)
			}
		})
	}
}

func TestCloneCopiesNormals(t *testing.T) {
	m := Cube(1).SmoothNormals()
	c := m.Clone()

	c.Normals[0] = v3(7, 7, 7)

	if m.Normals[0] == v3(7, 7, 7) {
		t.Errorf("wanted the clone's normals to be its own")
	}
	if got := Cube(1).Clone(); got.Normals != nil {
		t.Errorf("wanted a mesh without normals to clone without them, got %v", got.Normals)
	}
}

// Fit moves and scales evenly, which leaves the directions the surface faces as they were.
func TestFitKeepsNormals(t *testing.T) {
	m := Cube(10).SmoothNormals()
	m.Transform.Position = v3(1, 2, 3)

	got := m.Fit(3)

	if !slices.Equal(got.Normals, m.Normals) {
		t.Errorf("wanted the normals kept, got %v", got.Normals)
	}
	got.Normals[0] = v3(5, 5, 5)
	if m.Normals[0] == v3(5, 5, 5) {
		t.Errorf("wanted the fitted copy's normals to be its own")
	}
	if err := got.Validate(); err != nil {
		t.Errorf("wanted a valid mesh, got %v", err)
	}
}

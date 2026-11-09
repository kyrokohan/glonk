package mesh

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"glonk.dev/geometry"
)

// facing is what a loaded model's corners must carry: the place and the normal.
type facing struct{ place, normal geometry.Vector3 }

// cornerOf reads the place and normal that face corner k of a loaded mesh has.
func cornerOf(m Mesh, face [3]int, k int) facing {
	f := facing{place: m.Vertices[face[k]]}
	if m.Normals != nil {
		f.normal = m.Normals[face[k]]
	}
	return f
}

// A cube as most exporters write it: 8 places and 6 normals, one for each side, so
// that every place is used with 3 normals.
const objCubeWithNormals = `v -1 -1 -1
v 1 -1 -1
v 1 1 -1
v -1 1 -1
v -1 -1 1
v 1 -1 1
v 1 1 1
v -1 1 1
vn 0 0 -1
vn 0 0 1
vn -1 0 0
vn 1 0 0
vn 0 -1 0
vn 0 1 0
f 1//1 4//1 3//1 2//1
f 5//2 6//2 7//2 8//2
f 1//3 5//3 8//3 4//3
f 2//4 3//4 7//4 6//4
f 1//5 2//5 6//5 5//5
f 4//6 8//6 7//6 3//6
`

func TestLoadOBJNormalsOfATriangle(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nvn 0 0 1\nvn 1 0 0\nvn 0 1 -1\nf 1//1 2//2 3//3\n")

	// Z is negated like the vertices', and the corners are reversed to survive it.
	if want := []geometry.Vector3{{Z: -1}, {X: 1}, {Y: 1, Z: 1}}; !slices.Equal(m.Normals, want) {
		t.Errorf("wanted normals %v, got %v", want, m.Normals)
	}
	if want := []geometry.Vector3{{}, {X: 1}, {Y: 1}}; !slices.Equal(m.Vertices, want) {
		t.Errorf("wanted the 3 vertices left where they were, got %v", m.Vertices)
	}
	if want := [][3]int{{0, 2, 1}}; !slices.Equal(m.Faces, want) {
		t.Errorf("wanted faces %v, got %v", want, m.Faces)
	}
	if want := [][2]int{{0, 1}, {1, 2}, {0, 2}}; !slices.Equal(m.Edges, want) {
		t.Errorf("wanted edges %v, got %v", want, m.Edges)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("wanted a valid mesh, got %v", err)
	}
}

func TestLoadOBJNormalIndexForms(t *testing.T) {
	const defs = "v 0 0 0\nv 1 0 0\nv 0 1 0\nvt 0 0\nvt 1 0\nvt 0 1\nvn 0 0 1\nvn 1 0 0\nvn 0 1 0\n"
	want := []geometry.Vector3{{Z: -1}, {X: 1}, {Y: 1}}

	tests := []struct {
		name string
		face string
	}{
		{"vertex and normal", "f 1//1 2//2 3//3"},
		{"vertex, texture and normal", "f 1/1/1 2/2/2 3/3/3"},
		{"the texture can be anything", "f 1/9/1 2/x/2 3/-1/3"},
		{"negative normals", "f 1//-3 2//-2 3//-1"},
		{"negative everything", "f -3/-3/-3 -2/-2/-2 -1/-1/-1"},
		{"negative and positive mixed", "f 1//-3 2//2 -1//-1"},
		{"tabs and spaces", "f\t1//1   2//2\t3//3 "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loadOBJ(t, defs+tt.face+"\n")

			if !slices.Equal(m.Normals, want) || len(m.Vertices) != 3 {
				t.Errorf("wanted the normals %v on 3 vertices, got %v on %d", want, m.Normals, len(m.Vertices))
			}
		})
	}
}

// A negative normal index counts back from the normals defined so far, like a vertex's.
func TestLoadOBJNegativeNormalIndexIsRelativeToThatLine(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nvn 0 0 1\nf 1//-1 2//-1 3//-1\nvn 1 0 0\nf 1//-1 2//-1 3//-1\n")

	// The first face's normal is the first vn; the second's is the new one. The places
	// are shared, so the second face takes copies.
	var got []facing
	for _, f := range m.Faces {
		got = append(got, cornerOf(m, f, 0))
	}
	want := []facing{{geometry.Vector3{}, geometry.Vector3{Z: -1}}, {geometry.Vector3{}, geometry.Vector3{X: 1}}}
	if !slices.Equal(got, want) {
		t.Errorf("wanted %v, got %v", want, got)
	}
}

func TestLoadOBJNormalsOfAnyLengthAndZero(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nvn 0 0 0\nvn 5 0 0\nvn 1e300 0 0 1.0\nf 1//1 2//2 3//3\n")

	if want := []geometry.Vector3{{}, {X: 5}, {X: 1e300}}; !slices.Equal(m.Normals, want) {
		t.Errorf("wanted the normals kept as they are, with a zero one allowed, got %v", m.Normals)
	}
}

// Faces that share a place and agree on its normal share the vertex: a smooth surface
// costs nothing.
func TestLoadOBJSmoothSurfaceKeepsItsVertices(t *testing.T) {
	m := loadOBJ(t, quadGridOBJ(5, "vertex"))
	plain := loadOBJ(t, quadGridOBJ(5, ""))

	if len(m.Vertices) != 36 || !slices.Equal(m.Vertices, plain.Vertices) {
		t.Errorf("wanted the same 36 vertices as without normals, got %d", len(m.Vertices))
	}
	if !slices.Equal(m.Faces, plain.Faces) || !slices.Equal(m.Edges, plain.Edges) {
		t.Errorf("wanted the same faces and edges as without normals")
	}
	if len(m.Normals) != 36 {
		t.Errorf("wanted a normal for each vertex, got %d", len(m.Normals))
	}
}

// A cube with a normal per side has a hard edge at each of its places: 24 vertices, one
// for each place and side, and every face is wholly its own normal.
func TestLoadOBJHardEdgesSplitVertices(t *testing.T) {
	m := loadOBJ(t, objCubeWithNormals)
	plain := loadOBJ(t, strings.Split(objCubeWithNormals, "vn ")[0]+"f 1 4 3 2\nf 5 6 7 8\nf 1 5 8 4\nf 2 3 7 6\nf 1 2 6 5\nf 4 8 7 3\n")

	if len(m.Vertices) != 24 || len(m.Normals) != 24 || len(m.Faces) != 12 {
		t.Fatalf("wanted 24 vertices and normals and 12 triangles, got %d, %d and %d", len(m.Vertices), len(m.Normals), len(m.Faces))
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("wanted a valid mesh, got %v", err)
	}
	// The 8 places keep their own vertices, as without normals.
	if !slices.Equal(m.Vertices[:8], plain.Vertices) {
		t.Errorf("wanted the first 8 vertices to be the 8 places, in the file's order")
	}
	// Every face is flat with one normal, pointing the way the face does (outward).
	for i, f := range m.Faces {
		a, b, c := m.Vertices[f[0]], m.Vertices[f[1]], m.Vertices[f[2]]
		face := b.Sub(a).Cross(c.Sub(a)).Normalize()
		for k := range 3 {
			if n := m.Normals[f[k]]; !vecNear(n, face, 1e-12) {
				t.Errorf("face %d corner %d: wanted the face's own normal %v, got %v", i, k, face, n)
			}
		}
		if face.Dot(a) <= 0 {
			t.Errorf("face %d: wanted it to face out of the cube", i)
		}
	}
	// A wireframe is the cube's 12 edges, as without normals, not 12 for each side.
	if !slices.Equal(m.Edges, plain.Edges) || len(m.Edges) != 12 {
		t.Errorf("wanted the same 12 edges as without normals, got %d: %v", len(m.Edges), m.Edges)
	}
	// And no two vertices that a face uses are the same place with the same normal.
	seen := map[facing]bool{}
	for _, f := range m.Faces {
		for k := range 3 {
			seen[cornerOf(m, f, k)] = true
		}
	}
	if len(seen) != 24 {
		t.Errorf("wanted 24 different (place, normal) pairs, got %d", len(seen))
	}
}

// A model drawn with normals looks smooth; the same model without them, faceted.
func TestLoadOBJNormalsMakeADrawnModelSmooth(t *testing.T) {
	m := loadOBJ(t, objCubeWithNormals)
	m.Transform = Transform{Position: geometry.Vector3{Z: 5}, Yaw: 0.6, Pitch: -0.4}
	light := &loggingLight{level: 0.5}

	m.DrawShaded(newShadeBuffer(t), shadeCam(), light)

	if len(light.calls) < 40 { // flat shading asks once for each of at most 12 triangles
		t.Errorf("wanted the light asked about every cell, got %d questions", len(light.calls))
	}
	for _, c := range light.calls {
		if math.Abs(c.normal.Length()-1) > 1e-9 {
			t.Fatalf("wanted unit normals, got %v", c.normal)
		}
	}
	if fitted := m.Fit(2); !slices.Equal(fitted.Normals, m.Normals) {
		t.Errorf("wanted Fit to keep the normals")
	}
}

// Whether a corner has a normal can differ from face to face. A corner without one
// gets a zero normal, wherever the place was first used.
func TestLoadOBJMixedCorners(t *testing.T) {
	const defs = "v 0 0 0\nv 1 0 0\nv 0 1 0\nv 1 1 0\nvn 0 0 1\nvn 1 0 0\n"
	z, x := geometry.Vector3{Z: -1}, geometry.Vector3{X: 1}
	p := []geometry.Vector3{{}, {X: 1}, {Y: 1}, {X: 1, Y: 1}}

	tests := []struct {
		name  string
		faces string
		want  [][3]facing // for each triangle, its corners in the file's reversed order
	}{
		{"one face, some corners", "f 1//1 2 3//2\n", [][3]facing{{{p[0], z}, {p[2], x}, {p[1], geometry.Vector3{}}}}},
		{"a normal after none", "f 1 2 3\nf 1//1 2//1 4//1\n", [][3]facing{
			{{p[0], geometry.Vector3{}}, {p[2], geometry.Vector3{}}, {p[1], geometry.Vector3{}}},
			{{p[0], z}, {p[3], z}, {p[1], z}}}},
		{"none after a normal", "f 1//1 2//1 4//1\nf 1 2 3\n", [][3]facing{
			{{p[0], z}, {p[3], z}, {p[1], z}},
			{{p[0], geometry.Vector3{}}, {p[2], geometry.Vector3{}}, {p[1], geometry.Vector3{}}}}},
		{"two normals and none at one place", "f 1//1 2 3\nf 1//2 2 3\nf 1 2 3\n", [][3]facing{
			{{p[0], z}, {p[2], geometry.Vector3{}}, {p[1], geometry.Vector3{}}},
			{{p[0], x}, {p[2], geometry.Vector3{}}, {p[1], geometry.Vector3{}}},
			{{p[0], geometry.Vector3{}}, {p[2], geometry.Vector3{}}, {p[1], geometry.Vector3{}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loadOBJ(t, defs+tt.faces)

			var got [][3]facing
			for _, f := range m.Faces {
				got = append(got, [3]facing{cornerOf(m, f, 0), cornerOf(m, f, 1), cornerOf(m, f, 2)})
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
			if err := m.Validate(); err != nil {
				t.Errorf("wanted a valid mesh, got %v", err)
			}
		})
	}
}

// A normal index that cannot be used leaves the corner without one: it is no reason to
// refuse a model, which a texture index never was either.
func TestLoadOBJUnusableNormalIndicesAreIgnored(t *testing.T) {
	const verts = "v 0 0 0\nv 1 0 0\nv 0 1 0\n"

	tests := []struct {
		name string
		text string
	}{
		{"no normals defined", verts + "f 1//1 2//1 3//1\n"},
		{"zero", verts + "vn 0 0 1\nf 1//0 2//0 3//0\n"},
		{"too big", verts + "vn 0 0 1\nf 1//2 2//9 3//99999\n"},
		{"too negative", verts + "vn 0 0 1\nf 1//-2 2//-9 3//-99999\n"},
		{"not a number", verts + "vn 0 0 1\nf 1//x 2//1.5 3//1e0\n"},
		{"overflows", verts + "vn 0 0 1\nf 1//99999999999999999999 2//- 3//+\n"},
		{"empty", verts + "vn 0 0 1\nf 1// 2/1/ 3/\n"},
		{"defined after the face", verts + "f 1//1 2//1 3//1\nvn 0 0 1\n"},
		{"not used by any face", verts + "vn 0 0 1\nf 1 2 3\n"},
		{"only lines", verts + "vn 0 0 1\nl 1/1/1 2/2/1 3\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loadOBJ(t, tt.text)

			if m.Normals != nil {
				t.Errorf("wanted no Normals when no corner has a usable normal, got %v", m.Normals)
			}
			if len(m.Vertices) != 3 {
				t.Errorf("wanted the 3 vertices as they are, got %d", len(m.Vertices))
			}
		})
	}

	// Next to a usable one, an unusable index is simply a corner without a normal.
	m := loadOBJ(t, verts+"vn 0 0 1\nf 1//1 2//9 3//x\n")
	want := []geometry.Vector3{{Z: -1}, {}, {}}
	if !slices.Equal(m.Normals, want) {
		t.Errorf("wanted %v, got %v", want, m.Normals)
	}
}

func TestLoadOBJNormalErrors(t *testing.T) {
	const verts = "v 0 0 0\nv 1 0 0\nv 0 1 0\n"

	tests := []struct {
		name     string
		text     string
		wantLine int
		wantText string
	}{
		{"two numbers", verts + "vn 1 2\n", 4, "a normal needs 3 numbers, got 2"},
		{"none", verts + "vn\n", 4, "a normal needs 3 numbers, got 0"},
		{"a word", verts + "vn 1 x 3\n", 4, `bad number "x"`},
		{"NaN", verts + "vn 1 nan 3\n", 4, "bad number"},
		{"infinite", verts + "vn inf 1 3\n", 4, "bad number"},
		{"overflow", verts + "vn 1e999 0 0\n", 4, "bad number"},
		{"after a continuation", verts + "vn 1 \\\n x 3\n", 4, "bad number"},
		{"after normals are in use", verts + "vn 0 0 1\nf 1//1 2//1 3//1\nvn 1\n", 6, "a normal needs 3 numbers"},
		{"a bad vertex index with normals in use", verts + "vn 0 0 1\nf 1//1 2//1 3//1\nf 1//1 2//1 9//1\n", 6, "out of range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := LoadOBJ(strings.NewReader(tt.text))

			if err == nil {
				t.Fatalf("wanted an error, got a mesh with %d vertices", len(m.Vertices))
			}
			if !strings.Contains(err.Error(), tt.wantText) || !strings.Contains(err.Error(), fmt.Sprintf("obj line %d:", tt.wantLine)) {
				t.Errorf("wanted an error naming line %d with %q, got %q", tt.wantLine, tt.wantText, err)
			}
			if len(m.Vertices) != 0 || len(m.Faces) != 0 || len(m.Edges) != 0 || m.Normals != nil {
				t.Errorf("wanted an empty mesh with the error, got %+v", m)
			}
		})
	}
}

// A file that never refers to a normal loads as it did before normals were read.
func TestLoadOBJWithoutNormalsIsAsItWas(t *testing.T) {
	for _, text := range []string{objCube, quadGridOBJ(6, "")} {
		m := loadOBJ(t, text)
		if m.Normals != nil {
			t.Errorf("wanted no Normals, got %d", len(m.Normals))
		}
	}
	// Even with vn lines in it.
	withDefs := strings.Replace(objCube, "f ", "vn 0 0 1\nvn 0 1 0\nf ", 1)
	if m := loadOBJ(t, withDefs); m.Normals != nil || len(m.Vertices) != len(loadOBJ(t, objCube).Vertices) {
		t.Errorf("wanted normals that no face uses to change nothing")
	}
}

// One place met with a different normal by every face is one vertex per face, found in
// a moment however many there are.
func TestLoadOBJManyNormalsAtOnePlaceIsLinear(t *testing.T) {
	const faces = 40000
	var sb strings.Builder
	sb.WriteString("v 0 0 0\nv 1 0 0\nv 0 1 0\n")
	for i := range faces {
		fmt.Fprintf(&sb, "vn %d 0 1\n", i)
	}
	for i := 1; i <= faces; i++ {
		fmt.Fprintf(&sb, "f 1//%d 2//%d 3//%d\n", i, i, i)
	}

	m, err := loadWithin(t, sb.String(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if len(m.Vertices) != 3*faces || len(m.Normals) != 3*faces || len(m.Faces) != faces {
		t.Errorf("wanted %d vertices, one for each corner, got %d", 3*faces, len(m.Vertices))
	}
	if len(m.Edges) != 3 {
		t.Errorf("wanted the 3 edges of the one triangle, got %d", len(m.Edges))
	}
}

// Limits work the same with normals, including when the file is cut in the middle.
func TestLoadOBJLimitWithNormals(t *testing.T) {
	want := loadOBJ(t, objCubeWithNormals)
	size := int64(len(objCubeWithNormals))

	m, err := loadLimit(t, objCubeWithNormals, size)
	if err != nil || !slices.Equal(m.Vertices, want.Vertices) || !slices.Equal(m.Normals, want.Normals) || !slices.Equal(m.Faces, want.Faces) {
		t.Errorf("wanted the same mesh at the exact limit, got err %v", err)
	}
	for limit := int64(1); limit < size; limit += 7 {
		if _, err := loadLimit(t, objCubeWithNormals, limit); err == nil {
			t.Fatalf("limit %d: wanted an error for a file cut short", limit)
		}
	}
}

// objModelFrom builds an OBJ file out of the bytes it is given, together with what
// loading it must give. It is the oracle for the loader, written without looking at how
// the loader splits vertices. The file has up to 6 places and 4 normals (one of them
// zero) and up to 6 faces of 3 to 5 corners, each with a normal that is none, unusable
// or one of the normals, in whichever of the index forms the bytes pick.
func objModelFrom(data []byte) (text string, want []model3, expectedEdges [][2]int, expectedVertices int, anyNormal bool) {
	next := func() int {
		if len(data) == 0 {
			return 0
		}
		b := data[0]
		data = data[1:]
		return int(b)
	}

	nv, nn := 1+next()%6, next()%5
	var sb strings.Builder
	placeAt := func(i int) geometry.Vector3 {
		return geometry.Vector3{X: float64(i), Y: float64(i * i % 5), Z: float64(i) - 2}
	}
	normalAt := func(j int) geometry.Vector3 {
		if j == 3 {
			return geometry.Vector3{}
		}
		return geometry.Vector3{X: float64(j + 1), Y: 0.5, Z: float64(j)}
	}
	for i := range nv {
		p := placeAt(i)
		fmt.Fprintf(&sb, "v %g %g %g\n", p.X, p.Y, p.Z)
	}
	for j := range nn {
		n := normalAt(j)
		fmt.Fprintf(&sb, "vn %g %g %g\n", n.X, n.Y, n.Z)
	}

	// keys[v] is the set of normals (or -1 for none) that place v is used with.
	keys := make([]map[int]bool, nv)
	seenEdge := map[[2]int]bool{}
	addEdge := func(a, b int) {
		if a == b {
			return
		}
		if a > b {
			a, b = b, a
		}
		if !seenEdge[[2]int{a, b}] {
			seenEdge[[2]int{a, b}] = true
			expectedEdges = append(expectedEdges, [2]int{a, b})
		}
	}

	for range 1 + next()%6 {
		type corner struct {
			v, n int
		}
		var corners []corner
		var toks []string
		for range 3 + next()%3 {
			v, sel, form := next()%nv, next()%(nn+2), next()
			c := corner{v: v, n: -1}
			vtok := fmt.Sprint(v + 1)
			if form&1 == 1 {
				vtok = fmt.Sprint(v - nv) // negative: counts back from the places there are
			}
			tok := vtok
			switch {
			case sel == 0: // none, written in one of the forms that have no normal
				if form&2 == 2 {
					tok += "/1"
				}
			case sel == 1: // unusable
				tok += []string{"//0", "//99", "//x", "/1/", "//-99"}[form/4%5]
			default:
				j := sel - 2
				c.n = j
				ntok := fmt.Sprint(j + 1)
				if form&2 == 2 {
					ntok = fmt.Sprint(j - nn) // negative, counting back from the normals there are
				}
				if form&4 == 4 {
					tok += "/1/" + ntok
				} else {
					tok += "//" + ntok
				}
				anyNormal = true
			}
			corners = append(corners, c)
			toks = append(toks, tok)
			if keys[v] == nil {
				keys[v] = map[int]bool{}
			}
			keys[v][c.n] = true
		}
		sb.WriteString("f " + strings.Join(toks, " ") + "\n")

		for i, c := range corners {
			addEdge(c.v, corners[(i+1)%len(corners)].v)
		}
		for i := 1; i+1 < len(corners); i++ {
			var tri model3
			for k, c := range [3]corner{corners[0], corners[i+1], corners[i]} {
				p := placeAt(c.v)
				tri[k].place = geometry.Vector3{X: p.X, Y: p.Y, Z: -p.Z}
				if c.n >= 0 {
					n := normalAt(c.n)
					tri[k].normal = geometry.Vector3{X: n.X, Y: n.Y, Z: -n.Z}
				}
			}
			want = append(want, tri)
		}
	}

	expectedVertices = nv
	if anyNormal {
		for _, k := range keys {
			if len(k) > 1 {
				expectedVertices += len(k) - 1
			}
		}
	}
	return sb.String(), want, expectedEdges, expectedVertices, anyNormal
}

// model3 is the three corners of a triangle, as the oracle expects to find them.
type model3 [3]facing

// checkOBJModel loads the file objModelFrom makes from data and checks everything the
// oracle knows: every triangle's places and normals, the edges, the number of vertices
// (one for each place and normal a place is used with, and no more), and that the mesh
// is valid.
func checkOBJModel(t testing.TB, data []byte) {
	t.Helper()
	text, want, wantEdges, wantVertices, anyNormal := objModelFrom(data)

	m, err := LoadOBJ(strings.NewReader(text))
	if err != nil {
		t.Fatalf("wanted the file to load, got %v for\n%s", err, text)
	}

	if len(m.Faces) != len(want) {
		t.Fatalf("wanted %d triangles, got %d for\n%s", len(want), len(m.Faces), text)
	}
	for i, f := range m.Faces {
		got := model3{cornerOf(m, f, 0), cornerOf(m, f, 1), cornerOf(m, f, 2)}
		if got != want[i] {
			t.Fatalf("triangle %d: wanted %v, got %v for\n%s", i, want[i], got, text)
		}
	}
	if !slices.Equal(m.Edges, wantEdges) {
		t.Fatalf("wanted edges %v, got %v for\n%s", wantEdges, m.Edges, text)
	}
	if len(m.Vertices) != wantVertices {
		t.Fatalf("wanted %d vertices, got %d for\n%s", wantVertices, len(m.Vertices), text)
	}
	if anyNormal != (m.Normals != nil) || (anyNormal && len(m.Normals) != len(m.Vertices)) {
		t.Fatalf("wanted Normals only when a corner has one, and one for each vertex, got %d for\n%s", len(m.Normals), text)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("wanted a valid mesh, got %v for\n%s", err, text)
	}
}

func TestLoadOBJMatchesAnIndependentModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(21, 22))
	for range 5000 {
		data := make([]byte, 140)
		for i := range data {
			data[i] = byte(rng.IntN(256))
		}
		checkOBJModel(t, data)
	}
}

func BenchmarkLoadOBJCubeWithNormals(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := LoadOBJ(strings.NewReader(objCubeWithNormals)); err != nil {
			b.Fatal(err)
		}
	}
}

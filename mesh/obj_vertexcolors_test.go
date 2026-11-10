package mesh

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// vertexCase is a v line ending in the given extra words, followed by two plain vertices
// and a face, so that the model is a model.
func vertexCase(extra string) string {
	return "v 0 0 0" + extra + "\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"
}

func TestLoadOBJReadsVertexColors(t *testing.T) {
	text := "v 0 0 0 1 0 0\nv 1 0 0 0 1 0\nv 0 1 0 0 0 1\nf 1 2 3\n"
	m := loadOBJ(t, text)

	if want := []engine.Color{engine.RGB(255, 0, 0), engine.RGB(0, 255, 0), engine.RGB(0, 0, 255)}; !slices.Equal(m.VertexColors, want) {
		t.Errorf("wanted %v, got %v", want, m.VertexColors)
	}
	// The colors are the only difference from the same model without them.
	plain := loadOBJ(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")
	if !sameGeometry(m, plain) {
		t.Errorf("wanted the geometry of the model without colors")
	}
	if plain.VertexColors != nil {
		t.Errorf("wanted no vertex colors in a file with none, got %v", plain.VertexColors)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("wanted a valid mesh, got %v", err)
	}
}

// Exactly six numbers is a color. A w after the coordinates, or anything else, is not.
func TestLoadOBJVertexColorsNeedExactlySixNumbers(t *testing.T) {
	tests := []struct {
		extra string
		want  engine.Color
	}{
		{"", none},
		{" 1", none},            // x y z w
		{" 1 0", none},          // 5 numbers
		{" 1 0 0", pureRed},     // x y z r g b
		{" 1 0 0 1", none},      // 7: x y z w r g b is not read
		{" 1 0 0 1 1", none},    // 8
		{" 1 0 0 # 1", pureRed}, // a comment is not numbers
		{"\t1\t0\t0  ", pureRed},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.extra), func(t *testing.T) {
			m := loadOBJ(t, vertexCase(tt.extra))
			if tt.want.IsDefault() {
				if m.VertexColors != nil {
					t.Errorf("wanted no vertex colors, got %v", m.VertexColors)
				}
				return
			}
			if len(m.VertexColors) != 3 || m.VertexColors[0] != tt.want || !m.VertexColors[1].IsDefault() || !m.VertexColors[2].IsDefault() {
				t.Errorf("wanted %v then none, got %v", tt.want, m.VertexColors)
			}
		})
	}
}

// Numbers that are not understood give no color and never fail the load; bad coordinates
// still do.
func TestLoadOBJVertexColorsThatAreNotNumbersAreSkipped(t *testing.T) {
	for _, extra := range []string{" nan 0 0", " 0 inf 0", " 1e999 0 0", " a b c", " 0.5 0.5 x", " 0x1 0 0", " 1,0 0 0", " - 0 0", " 1 0 +"} {
		m := loadOBJ(t, vertexCase(extra))
		if m.VertexColors != nil {
			t.Errorf("%q: wanted no vertex colors, got %v", extra, m.VertexColors)
		}
		if len(m.Vertices) != 3 {
			t.Errorf("%q: wanted the model loaded, got %d vertices", extra, len(m.Vertices))
		}
	}

	_, err := LoadOBJ(strings.NewReader("v a 0 0 1 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"))
	if err == nil || !strings.Contains(err.Error(), "obj line 1") {
		t.Errorf("wanted bad coordinates to fail the load naming line 1, got %v", err)
	}
	if _, err := LoadOBJ(strings.NewReader("v 1 2\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")); err == nil {
		t.Errorf("wanted too few coordinates to fail the load")
	}
}

// The numbers are read as a material's Kd is.
func TestLoadOBJVertexColorValues(t *testing.T) {
	for _, tt := range []struct {
		extra string
		want  engine.Color
	}{
		{" 0.5 0.2 0.8", engine.RGB(128, 51, 204)},
		{" 1.7 -0.3 0.5", engine.RGB(255, 0, 128)},
		{" 2e-1 1E0 .5", engine.RGB(51, 255, 128)},
		{" 0 0 0", engine.RGB(0, 0, 0)}, // black is a color, not "none"
		{" 255 128 0", engine.RGB(255, 255, 0)},
		{" 1 1 1", engine.RGB(255, 255, 255)},
	} {
		m := loadOBJ(t, vertexCase(tt.extra))
		if len(m.VertexColors) != 3 || m.VertexColors[0] != tt.want {
			t.Errorf("%q: wanted %v, got %v", tt.extra, tt.want, m.VertexColors)
		}
	}
}

func TestLoadOBJVertexColorsOnSomeVerticesStartAtTheFirstOne(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0\nv 1 0 0 1 0 0\nv 0 1 0\nv 1 1 0 0 0 1\nv 2 2 0\nf 1 2 3\nf 2 4 3\n")
	if want := []engine.Color{none, pureRed, none, pureBlue, none}; !slices.Equal(m.VertexColors, want) {
		t.Errorf("wanted %v, got %v", want, m.VertexColors)
	}

	m = loadOBJ(t, "v 0 0 0 0 1 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")
	if want := []engine.Color{pureGreen, none, none}; !slices.Equal(m.VertexColors, want) {
		t.Errorf("wanted %v, got %v", want, m.VertexColors)
	}
}

func TestLoadOBJVertexColorsOfAModelOfLines(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0 1 0 0\nv 1 0 0 0 0 1\nl 1 2\n")
	if want := []engine.Color{pureRed, pureBlue}; !slices.Equal(m.VertexColors, want) {
		t.Errorf("wanted %v, got %v", want, m.VertexColors)
	}
}

// A place that is split for its normals gives a vertex for each, and every one keeps the
// color the place had.
func TestLoadOBJSplitVerticesKeepTheirColors(t *testing.T) {
	text := objCubeWithNormals
	for _, corner := range [][3]int{{-1, -1, -1}, {1, -1, -1}, {1, 1, -1}, {-1, 1, -1}, {-1, -1, 1}, {1, -1, 1}, {1, 1, 1}, {-1, 1, 1}} {
		line := fmt.Sprintf("v %d %d %d\n", corner[0], corner[1], corner[2])
		colored := fmt.Sprintf("v %d %d %d %d %d %d\n", corner[0], corner[1], corner[2], (corner[0]+1)/2, (corner[1]+1)/2, (corner[2]+1)/2)
		text = strings.Replace(text, line, colored, 1)
	}
	m := loadOBJ(t, text)

	if len(m.Vertices) != 24 || len(m.VertexColors) != 24 {
		t.Fatalf("wanted 24 vertices with a color each, got %d and %d", len(m.Vertices), len(m.VertexColors))
	}
	for i, v := range m.Vertices {
		// Z is negated on the way in, so the file's Z is -v.Z.
		want := engine.RGB(uint8(127.5+127.5*v.X+0.5), uint8(127.5+127.5*v.Y+0.5), uint8(127.5-127.5*v.Z+0.5))
		if m.VertexColors[i] != want {
			t.Fatalf("vertex %d at %v: wanted %v, got %v", i, v, want, m.VertexColors[i])
		}
	}
	if err := m.Validate(); err != nil {
		t.Errorf("wanted a valid mesh, got %v", err)
	}
}

func TestEveryLoaderReadsVertexColors(t *testing.T) {
	obj := "mtllib m.mtl\nv 0 0 0 1 0 0\nv 1 0 0 0 1 0\nv 0 1 0 0 0 1\nusemtl a\nf 1 2 3\n"
	want := []engine.Color{pureRed, pureGreen, pureBlue}

	plain, err := LoadOBJ(strings.NewReader(obj))
	if err != nil || !slices.Equal(plain.VertexColors, want) {
		t.Errorf("LoadOBJ: wanted %v, got %v (%v)", want, plain.VertexColors, err)
	}
	limited, err := LoadOBJLimit(strings.NewReader(obj), 1<<20)
	if err != nil || !slices.Equal(limited.VertexColors, want) {
		t.Errorf("LoadOBJLimit: wanted %v, got %v (%v)", want, limited.VertexColors, err)
	}

	// With materials, the face has the color of its Kd and the vertex colors tint it.
	m := loadFS(t, map[string]string{"m.obj": obj, "m.mtl": "newmtl a\nKd 0 0 1\n"}, "m.obj")
	if !slices.Equal(m.VertexColors, want) || !slices.Equal(m.FaceColors, []engine.Color{pureBlue}) {
		t.Errorf("LoadOBJFS: wanted colors %v and a face color of blue, got %v and %v", want, m.VertexColors, m.FaceColors)
	}
}

// What a file says is what is drawn: the loaded model draws as the same mesh made in code.
func TestLoadedVertexColorsDrawAsTheSameMeshMadeInCode(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	m := loadOBJ(t, "v -2 -1 0 1 0 0\nv 2 -1 0 0 1 0\nv 0 2 0 0 0 1\nf 1 3 2\n")
	built := Mesh{
		Vertices:     []geometry.Vector3{{X: -2, Y: -1}, {X: 2, Y: -1}, {Y: 2}},
		Faces:        [][3]int{{0, 1, 2}},
		VertexColors: []engine.Color{pureRed, pureGreen, pureBlue},
	}
	for _, mm := range []*Mesh{&m, &built} {
		mm.Transform = Transform{Position: geometry.Vector3{Z: 6}}
		mm.DoubleSided = true
	}

	got, want := newShadeBuffer(t), newShadeBuffer(t)
	m.DrawShaded(got, shadeCam(), Ambient{Level: 1})
	built.DrawShaded(want, shadeCam(), Ambient{Level: 1})
	if picture(got) != picture(want) {
		t.Errorf("wanted the loaded model to draw as the one made in code\n%s\ngot\n%s", picture(want), picture(got))
	}
	if strings.HasSuffix(picture(got), "legend\n") {
		t.Errorf("wanted something drawn")
	}
}

// vertexColorModel is a random model, with the colors every place must come out with.
type vertexColorModel struct {
	text string
	want []engine.Color // for each place, by its index; the zero Color for none
}

// A word of a color the way a file might write it, and what it means, or ok false.
func colorWord(rng *rand.Rand) (word string, value float64, ok bool) {
	switch rng.IntN(12) {
	case 0:
		return "nan", 0, false
	case 1:
		return "x", 0, false
	case 2:
		return "1e999", 0, false
	case 3:
		n := rng.IntN(400) - 100 // whole numbers, mostly out of range
		return fmt.Sprintf("%d", n), float64(n), true
	}
	v := rng.Float64()*1.6 - 0.3
	return fmt.Sprintf("%v", v), v, true
}

func randomVertexColorModel(rng *rand.Rand) vertexColorModel {
	places := 3 + rng.IntN(6)
	var sb strings.Builder
	want := make([]engine.Color, places)
	for i := range places {
		fmt.Fprintf(&sb, "v %d 0 0", i)
		count := []int{0, 0, 3, 3, 3, 3, 1, 2, 4, 5}[rng.IntN(10)]
		good := true
		var vals [3]float64
		for k := range count {
			word, v, ok := colorWord(rng)
			sb.WriteString(" " + word)
			good = good && ok
			if k < 3 {
				vals[k] = v
			}
		}
		sb.WriteString("\n")
		if count == 3 && good {
			// Independently of parseKd: clamp, scale and round half up.
			var c [3]uint8
			for k, v := range vals {
				c[k] = uint8(math.Floor(255*min(max(v, 0), 1) + 0.5))
			}
			want[i] = engine.RGB(c[0], c[1], c[2])
		}
	}
	if rng.IntN(2) == 0 { // normals, so that places are split
		sb.WriteString("vn 0 0 1\nvn 0 1 0\nvn 1 0 0\n")
		for range 1 + rng.IntN(6) {
			fmt.Fprintf(&sb, "f %d//%d %d//%d %d//%d\n", 1+rng.IntN(places), 1+rng.IntN(3), 1+rng.IntN(places), 1+rng.IntN(3), 1+rng.IntN(places), 1+rng.IntN(3))
		}
	} else {
		for range 1 + rng.IntN(6) {
			fmt.Fprintf(&sb, "f %d %d %d\n", 1+rng.IntN(places), 1+rng.IntN(places), 1+rng.IntN(places))
		}
	}
	return vertexColorModel{text: sb.String(), want: want}
}

func TestLoadOBJVertexColorsMatchAnIndependentReading(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 8))
	var colored, mixed, split int
	for trial := range 1000 {
		tt := randomVertexColorModel(rng)
		m := loadOBJ(t, tt.text)

		// Places are at x = their index, so every vertex, copies for normals included,
		// says which place it came from.
		anyColor := slices.ContainsFunc(tt.want, func(c engine.Color) bool { return !c.IsDefault() })
		if !anyColor {
			if m.VertexColors != nil {
				t.Fatalf("trial %d: wanted no vertex colors, got %v\n%s", trial, m.VertexColors, tt.text)
			}
			continue
		}
		colored++
		if slices.Contains(tt.want, none) {
			mixed++
		}
		if len(m.Vertices) > len(tt.want) {
			split++
		}
		if len(m.VertexColors) != len(m.Vertices) {
			t.Fatalf("trial %d: wanted a color for each of %d vertices, got %d\n%s", trial, len(m.Vertices), len(m.VertexColors), tt.text)
		}
		for i, v := range m.Vertices {
			if want := tt.want[int(v.X)]; m.VertexColors[i] != want {
				t.Fatalf("trial %d: vertex %d from place %d: wanted %v, got %v\n%s", trial, i, int(v.X), want, m.VertexColors[i], tt.text)
			}
		}
		if err := m.Validate(); err != nil {
			t.Fatalf("trial %d: wanted a valid mesh, got %v", trial, err)
		}
	}
	// Guard: the oracle reaches colored models, colors beside places without, and split places.
	if colored < 500 || mixed < 200 || split < 100 {
		t.Errorf("wanted at least 500 colored, 200 mixed and 100 split models of 1000, got %d, %d and %d", colored, mixed, split)
	}
}

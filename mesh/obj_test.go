package mesh

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

func loadOBJ(t *testing.T, text string) Mesh {
	t.Helper()
	m, err := LoadOBJ(strings.NewReader(text))
	if err != nil {
		t.Fatalf("unable to load %q: %v", text, err)
	}
	return m
}

func TestLoadOBJTriangle(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")

	wantVerts := []geometry.Vector3{{}, {X: 1}, {Y: 1}}
	if !slices.Equal(m.Vertices, wantVerts) {
		t.Errorf("wanted vertices %v, got %v", wantVerts, m.Vertices)
	}
	// Corner order is reversed to survive the Z flip.
	if want := [][3]int{{0, 2, 1}}; !slices.Equal(m.Faces, want) {
		t.Errorf("wanted faces %v, got %v", want, m.Faces)
	}
	if want := [][2]int{{0, 1}, {1, 2}, {0, 2}}; !slices.Equal(m.Edges, want) {
		t.Errorf("wanted edges %v, got %v", want, m.Edges)
	}
}

func TestLoadOBJFlipsZ(t *testing.T) {
	m := loadOBJ(t, "v 1 2 3\nv -4 5 -6\nv 0 0 0\nf 1 2 3\n")

	want := []geometry.Vector3{{X: 1, Y: 2, Z: -3}, {X: -4, Y: 5, Z: 6}, {}}
	if !slices.Equal(m.Vertices, want) {
		t.Errorf("wanted %v, got %v", want, m.Vertices)
	}
}

func TestLoadOBJPolygons(t *testing.T) {
	square := "v 0 0 0\nv 1 0 0\nv 1 1 0\nv 0 1 0\nv 0.5 2 0\n"

	tests := []struct {
		name      string
		text      string
		wantFaces [][3]int
		wantEdges [][2]int
	}{
		{
			"quad is two triangles with four outline edges",
			square + "f 1 2 3 4\n",
			[][3]int{{0, 2, 1}, {0, 3, 2}},
			[][2]int{{0, 1}, {1, 2}, {2, 3}, {0, 3}},
		},
		{
			"pentagon is three triangles with five outline edges",
			square + "f 1 2 3 5 4\n",
			[][3]int{{0, 2, 1}, {0, 4, 2}, {0, 3, 4}},
			[][2]int{{0, 1}, {1, 2}, {2, 4}, {3, 4}, {0, 3}},
		},
		{
			"two triangles sharing an edge list it once",
			square + "f 1 2 3\nf 1 3 4\n",
			[][3]int{{0, 2, 1}, {0, 3, 2}},
			[][2]int{{0, 1}, {1, 2}, {0, 2}, {2, 3}, {0, 3}},
		},
		{
			"a repeated face adds no edges",
			square + "f 1 2 3\nf 1 2 3\n",
			[][3]int{{0, 2, 1}, {0, 2, 1}},
			[][2]int{{0, 1}, {1, 2}, {0, 2}},
		},
		{
			// The zero-area triangle is kept (it draws nothing and is culled); no self edge appears.
			"a corner repeated in a face adds no self edge",
			square + "f 1 1 2 3\n",
			[][3]int{{0, 1, 0}, {0, 2, 1}},
			[][2]int{{0, 1}, {1, 2}, {0, 2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loadOBJ(t, tt.text)

			if !slices.Equal(m.Faces, tt.wantFaces) {
				t.Errorf("wanted faces %v, got %v", tt.wantFaces, m.Faces)
			}
			if !slices.Equal(m.Edges, tt.wantEdges) {
				t.Errorf("wanted edges %v, got %v", tt.wantEdges, m.Edges)
			}
		})
	}
}

func TestLoadOBJIndexForms(t *testing.T) {
	const verts = "v 0 0 0\nv 1 0 0\nv 0 1 0\nv 1 1 0\n"

	tests := []struct {
		name string
		face string
		want [][3]int
	}{
		{"plain", "f 1 2 3", [][3]int{{0, 2, 1}}},
		{"with texture", "f 1/1 2/2 3/3", [][3]int{{0, 2, 1}}},
		{"with normal", "f 1//1 2//2 3//3", [][3]int{{0, 2, 1}}},
		{"with both", "f 1/1/1 2/2/2 3/3/3", [][3]int{{0, 2, 1}}},
		{"mixed forms", "f 1 2/9 3//9", [][3]int{{0, 2, 1}}},
		{"negative", "f -4 -3 -2", [][3]int{{0, 2, 1}}},
		{"negative with extras", "f -4/1/1 -3/1/1 -2/1/1", [][3]int{{0, 2, 1}}},
		{"negative and positive mixed", "f 1 -3 3", [][3]int{{0, 2, 1}}},
		{"last vertices", "f -1 -2 -3", [][3]int{{3, 1, 2}}},
		{"tabs and extra spaces", "f\t1   2\t 3  ", [][3]int{{0, 2, 1}}},
		{"trailing slash", "f 1/ 2/ 3/", [][3]int{{0, 2, 1}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loadOBJ(t, verts+tt.face+"\n")

			if !slices.Equal(m.Faces, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, m.Faces)
			}
		})
	}
}

// A negative index counts back from the vertices defined so far, not from the whole file.
func TestLoadOBJNegativeIndexIsRelativeToThatLine(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nf -3 -2 -1\nv 5 5 5\nv 6 6 6\nv 7 7 7\nf -3 -2 -1\n")

	want := [][3]int{{0, 2, 1}, {3, 5, 4}}
	if !slices.Equal(m.Faces, want) {
		t.Errorf("wanted %v, got %v", want, m.Faces)
	}
}

func TestLoadOBJLines(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0\nv 1 0 0\nv 1 1 0\nv 0 1 0\nl 1 2 3\nl 3/1 4\nl 4 1\n")

	if len(m.Faces) != 0 {
		t.Errorf("wanted no faces, got %v", m.Faces)
	}
	if want := [][2]int{{0, 1}, {1, 2}, {2, 3}, {0, 3}}; !slices.Equal(m.Edges, want) {
		t.Errorf("wanted edges %v, got %v", want, m.Edges)
	}
}

func TestLoadOBJLinesJoinFaces(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nv 5 5 5\nf 1 2 3\nl 3 4\nl 1 2\n")

	if want := [][2]int{{0, 1}, {1, 2}, {0, 2}, {2, 3}}; !slices.Equal(m.Edges, want) {
		t.Errorf("wanted edges %v, got %v", want, m.Edges)
	}
}

func TestLoadOBJIgnoresWhatItDoesNotUse(t *testing.T) {
	text := `# a model
mtllib cube.mtl
o thing
g top
s off
usemtl red
v 0 0 0   # the origin
v 1 0 0 1.0
v 0 1 0 0.2 0.4 0.6
vt 0 0
vt 1 0
vn 0 0 1
p 1
cstype bezier
curv 0 1 1 2
f 1/1/1 2/2/1 3//1

   
# done`
	m := loadOBJ(t, text)

	if len(m.Vertices) != 3 || len(m.Faces) != 1 {
		t.Errorf("wanted 3 vertices and 1 face, got %d and %d", len(m.Vertices), len(m.Faces))
	}
}

// A trailing comment must not be read as more corners.
func TestLoadOBJTrailingComments(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0 # a\nv 1 0 0#b\nv 0 1 0\nf 1 2 3 # not a corner\nl 1 2 #c\nf 1 2 3#d\n")

	if len(m.Vertices) != 3 || len(m.Faces) != 2 || len(m.Edges) != 3 {
		t.Errorf("wanted 3 vertices, 2 faces, 3 edges, got %d, %d, %d", len(m.Vertices), len(m.Faces), len(m.Edges))
	}
}

func TestLoadOBJLineEndings(t *testing.T) {
	for name, text := range map[string]string{
		"unix":             "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n",
		"windows":          "v 0 0 0\r\nv 1 0 0\r\nv 0 1 0\r\nf 1 2 3\r\n",
		"no final newline": "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3",
		"blank lines":      "\n\nv 0 0 0\n\nv 1 0 0\n\r\nv 0 1 0\n\nf 1 2 3\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			m := loadOBJ(t, text)
			if len(m.Vertices) != 3 || len(m.Faces) != 1 {
				t.Errorf("wanted 3 vertices and 1 face, got %d and %d", len(m.Vertices), len(m.Faces))
			}
		})
	}
}

func TestLoadOBJLineContinuation(t *testing.T) {
	m := loadOBJ(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nv 1 1 0\nf 1 2 \\\n 3 \\\n 4\n")

	if len(m.Faces) != 2 {
		t.Errorf("wanted a quad from a continued face, got %v", m.Faces)
	}

	// A file that ends on a backslash still uses the statement it was building.
	m = loadOBJ(t, "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 \\\n3 \\")
	if len(m.Faces) != 1 {
		t.Errorf("wanted the last continued face kept, got %v", m.Faces)
	}
}

func TestLoadOBJErrors(t *testing.T) {
	const verts = "v 0 0 0\nv 1 0 0\nv 0 1 0\n"

	tests := []struct {
		name     string
		text     string
		wantLine int    // 0 means the error does not name a line
		wantText string // a fragment of the message
	}{
		{"vertex with two numbers", "v 1 2\n", 1, "needs 3 numbers"},
		{"vertex with no numbers", verts + "v\n", 4, "needs 3 numbers"},
		{"vertex with a word", verts + "v 1 x 3\n", 4, `bad number "x"`},
		{"vertex with NaN", verts + "v 1 nan 3\n", 4, "bad number"},
		{"vertex with Inf", verts + "v inf 1 3\n", 4, "bad number"},
		{"vertex with -Inf", verts + "v 1 -inf 3\n", 4, "bad number"},
		{"vertex that overflows", verts + "v 1e999 0 0\n", 4, "bad number"},
		{"vertex with a half number", verts + "v 1.5. 0 0\n", 4, "bad number"},
		{"face index zero", verts + "f 0 1 2\n", 4, "out of range"},
		{"face index too big", verts + "f 1 2 4\n", 4, "out of range"},
		{"face index before the vertex exists", "f 1 2 3\nv 0 0 0\nv 1 0 0\nv 0 1 0\n", 1, "out of range"},
		{"negative index too far", verts + "f -4 -2 -1\n", 4, "out of range"},
		{"index is not a number", verts + "f 1 2 x\n", 4, `bad vertex index "x"`},
		{"index is a float", verts + "f 1 2 3.5\n", 4, "bad vertex index"},
		{"index overflows", verts + "f 1 2 99999999999999999999\n", 4, "bad vertex index"},
		{"index is empty", verts + "f 1 2 /3\n", 4, "missing vertex index"},
		{"face with two corners", verts + "f 1 2\n", 4, "at least 3 corners"},
		{"face with no corners", verts + "f\n", 4, "at least 3 corners"},
		{"line with one point", verts + "l 1\n", 4, "at least 2 points"},
		{"line with a bad point", verts + "l 1 9\n", 4, "out of range"},
		{"error after a continuation names its first line", verts + "f 1 2 \\\n 9\n", 4, "out of range"},
		{"no geometry", verts, 0, "no faces or lines"},
		{"empty file", "", 0, "no faces or lines"},
		{"only comments", "# nothing\n# here\n", 0, "no faces or lines"},
		{"not an obj file", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR", 0, "no faces or lines"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := LoadOBJ(strings.NewReader(tt.text))
			if err == nil {
				t.Fatalf("wanted an error, got a mesh with %d vertices", len(m.Vertices))
			}
			if len(m.Vertices) != 0 || len(m.Faces) != 0 || len(m.Edges) != 0 {
				t.Errorf("wanted an empty mesh with the error, got %+v", m)
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("wanted the message to contain %q, got %q", tt.wantText, err)
			}
			if tt.wantLine != 0 && !strings.Contains(err.Error(), fmt.Sprintf("obj line %d:", tt.wantLine)) {
				t.Errorf("wanted the message to name line %d, got %q", tt.wantLine, err)
			}
		})
	}
}

func TestLoadOBJLongLines(t *testing.T) {
	// A polygon with ten thousand corners is a long line but a fair file.
	var sb strings.Builder
	const n = 10000
	for i := range n {
		fmt.Fprintf(&sb, "v %d %d 0\n", i, i%7)
	}
	sb.WriteString("f")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, " %d", i)
	}
	sb.WriteString("\n")

	m := loadOBJ(t, sb.String())
	if len(m.Faces) != n-2 {
		t.Errorf("wanted %d triangles, got %d", n-2, len(m.Faces))
	}

	// Past the limit it is an error, not an out-of-memory.
	_, err := LoadOBJ(strings.NewReader("v 0 0 0\n# " + strings.Repeat("x", maxOBJLine+10) + "\n"))
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("wanted bufio.ErrTooLong, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "obj line 2:") {
		t.Errorf("wanted the message to name line 2, got %q", err)
	}

	// Endless continuation lines are bounded too.
	_, err = LoadOBJ(strings.NewReader("f 1 \\\n" + strings.Repeat(strings.Repeat("1 ", 4000)+"\\\n", 400)))
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("wanted bufio.ErrTooLong for runaway continuation, got %v", err)
	}
}

// loadWithin runs LoadOBJ and fails the test, instead of waiting it out, if it takes
// longer than d. The stuck call keeps running in the background until it ends.
func loadWithin(t *testing.T, text string, d time.Duration) (Mesh, error) {
	t.Helper()

	type result struct {
		m   Mesh
		err error
	}
	done := make(chan result, 1)
	go func() {
		m, err := LoadOBJ(strings.NewReader(text))
		done <- result{m, err}
	}()

	select {
	case r := <-done:
		return r.m, r.err
	case <-time.After(d):
		t.Fatalf("wanted %d bytes to load within %v, still running", len(text), d)
		return Mesh{}, nil
	}
}

// Each continued line used to be copied into a longer string, so the cost grew with
// the square of the number of lines: a 2 MiB file of backslashes took 35 seconds.
func TestLoadOBJContinuationRunsInLinearTime(t *testing.T) {
	const limit = 1 << 20 // maxOBJLine, in bytes of glued text; each lone backslash adds one

	t.Run("a file that ends just inside the limit", func(t *testing.T) {
		_, err := loadWithin(t, strings.Repeat("\\\n", limit), 5*time.Second)
		if err == nil || !strings.Contains(err.Error(), "no faces or lines") {
			t.Errorf("wanted the file to load and find no geometry, got %v", err)
		}
	})

	t.Run("a file that crosses the limit", func(t *testing.T) {
		_, err := loadWithin(t, strings.Repeat("\\\n", limit+10), 5*time.Second)
		if !errors.Is(err, bufio.ErrTooLong) || !strings.Contains(err.Error(), "obj line 1:") {
			t.Errorf("wanted ErrTooLong naming line 1, got %v", err)
		}
	})

	t.Run("the limit is counted from the line the statement began on", func(t *testing.T) {
		text := "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 \\\n" + strings.Repeat("\\\n", limit+10)
		_, err := loadWithin(t, text, 5*time.Second)
		if !errors.Is(err, bufio.ErrTooLong) || !strings.Contains(err.Error(), "obj line 4:") {
			t.Errorf("wanted ErrTooLong naming line 4, got %v", err)
		}
	})
}

// Speed is not enough: a long continued polygon must also come out right.
func TestLoadOBJLongContinuedPolygon(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("v 0 0 0\nv 1 0 0\nv 0 1 0\nf")
	const cycles = 66666
	for range cycles {
		sb.WriteString(" 1 \\\n 2 \\\n 3 \\\n")
	}
	sb.WriteString(" 1\n")
	corners := 3*cycles + 1

	m, err := loadWithin(t, sb.String(), 5*time.Second)
	if err != nil {
		t.Fatalf("unable to load: %v", err)
	}

	if want := corners - 2; len(m.Faces) != want {
		t.Fatalf("wanted %d triangles from %d corners, got %d", want, corners, len(m.Faces))
	}
	// Corners run 1 2 3 1 2 3 ... and the fan is anchored at the first, so the first
	// triangles use corners (1,2,3) and (1,3,1) reversed.
	if want := [3]int{0, 2, 1}; m.Faces[0] != want {
		t.Errorf("wanted the first triangle %v, got %v", want, m.Faces[0])
	}
	if want := [][2]int{{0, 1}, {1, 2}, {0, 2}}; !slices.Equal(m.Edges, want) {
		t.Errorf("wanted the three distinct edges %v, got %v", want, m.Edges)
	}
}

func loadLimit(t *testing.T, text string, limit int64) (Mesh, error) {
	t.Helper()
	return LoadOBJLimit(strings.NewReader(text), limit)
}

func TestLoadOBJLimitAtAndAroundTheLimit(t *testing.T) {
	size := int64(len(objCube))
	want := loadOBJ(t, objCube)

	for _, limit := range []int64{size, size + 1, size + 1000, math.MaxInt64} {
		m, err := loadLimit(t, objCube, limit)
		if err != nil {
			t.Fatalf("limit %d: wanted the model to load, got %v", limit, err)
		}
		if !slices.Equal(m.Vertices, want.Vertices) || !slices.Equal(m.Faces, want.Faces) || !slices.Equal(m.Edges, want.Edges) {
			t.Errorf("limit %d: wanted the same mesh as LoadOBJ", limit)
		}
	}

	for _, limit := range []int64{size - 1, size / 2, 1} {
		m, err := loadLimit(t, objCube, limit)
		if !errors.Is(err, ErrOBJTooLarge) {
			t.Errorf("limit %d: wanted ErrOBJTooLarge, got %v", limit, err)
		}
		if err != nil && !strings.Contains(err.Error(), fmt.Sprintf("more than %d bytes", limit)) {
			t.Errorf("limit %d: wanted the message to name the limit, got %q", limit, err)
		}
		if len(m.Vertices) != 0 || len(m.Faces) != 0 || len(m.Edges) != 0 {
			t.Errorf("limit %d: wanted no partial mesh with the error, got %+v", limit, m)
		}
	}
}

// Cutting the input anywhere must say the input is too large. The cut-off last
// line (half a number, half a face) must never be reported as a parse error.
func TestLoadOBJLimitReportsTooLargeWhereverTheCutFalls(t *testing.T) {
	for _, text := range []string{
		"v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n",
		"v 0 0 0\r\nv 1 0 0\r\nv 0 1 0\r\nf 1 2 \\\r\n 3\r\n",
		"# a comment\nv 1.5e3 -2 3\nv 1 0 0\nv 0 1 0\nl 1 2\nf 1/1/1 2//2 3\n",
		objCube,
	} {
		for limit := int64(1); limit < int64(len(text)); limit++ {
			_, err := loadLimit(t, text, limit)
			if !errors.Is(err, ErrOBJTooLarge) {
				t.Fatalf("%q cut at %d: wanted ErrOBJTooLarge, got %v", text, limit, err)
			}
		}
		if _, err := loadLimit(t, text, int64(len(text))); err != nil {
			t.Fatalf("%q at its own length: wanted it to load, got %v", text, err)
		}
	}
}

// The scanner reads in chunks. When the limit falls in a later chunk, the loader
// already holds the start of a line from the earlier one, and hands it over as if it
// were complete. Here that start is "f 1 2", which alone is a malformed face; the
// error must still say the input was too large.
func TestLoadOBJLimitHidesTheCutOffLineBehindTheRealReason(t *testing.T) {
	const chunk = 64 * 1024 // the loader's first read

	prefix := "v 0 0 0\nv 1 0 0\nv 0 1 0\n"
	padding := "#" + strings.Repeat("x", chunk-len(prefix)-len("#")-len("\n")-len("f 1 2")) + "\n"
	text := prefix + padding + "f 1 2" + " 3\n" + strings.Repeat("# more\n", 100)
	if text[chunk-5:chunk] != "f 1 2" {
		t.Fatalf("test setup: wanted the first chunk to end in \"f 1 2\", got %q", text[chunk-5:chunk])
	}

	// Allow the first chunk plus a few bytes: the limit lands in the second read.
	_, err := loadLimit(t, text, chunk+4)
	if !errors.Is(err, ErrOBJTooLarge) {
		t.Errorf("wanted ErrOBJTooLarge, got %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "at least 3 corners") {
		t.Errorf("wanted the cut-off face not to be reported, got %v", err)
	}

	// Without a limit the same text is just a valid model.
	m, err := LoadOBJ(strings.NewReader(text))
	if err != nil || len(m.Faces) != 1 {
		t.Errorf("wanted the text to be a valid model without a limit, got %v", err)
	}
}

func TestLimitReaderStaysFailedOnceOver(t *testing.T) {
	r := &limitReader{r: strings.NewReader("0123456789"), left: 4}
	buf := make([]byte, 8)

	n, err := r.Read(buf)
	if n != 0 || !errors.Is(err, ErrOBJTooLarge) {
		t.Fatalf("wanted the first read past the limit to fail, got %d, %v", n, err)
	}
	for range 3 {
		if n, err := r.Read(buf); n != 0 || !errors.Is(err, ErrOBJTooLarge) {
			t.Errorf("wanted every later read to fail too, got %d, %v", n, err)
		}
	}
}

func TestLoadOBJLimitMustBePositive(t *testing.T) {
	for _, limit := range []int64{0, -1, math.MinInt64} {
		src := &countingReader{r: strings.NewReader(objCube)}

		_, err := LoadOBJLimit(src, limit)
		if err == nil || errors.Is(err, ErrOBJTooLarge) || !strings.Contains(err.Error(), "positive") {
			t.Errorf("limit %d: wanted a limit error, got %v", limit, err)
		}
		if src.n != 0 {
			t.Errorf("limit %d: wanted nothing read from the source, got %d bytes", limit, src.n)
		}
	}
}

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

func TestLoadOBJLimitPullsNoMoreThanLimitPlusOne(t *testing.T) {
	huge := strings.Repeat("# padding padding padding\n", 100000) // 2.6 MB

	for _, limit := range []int64{10, 1000, 70000} {
		src := &countingReader{r: strings.NewReader(huge)}

		_, err := LoadOBJLimit(src, limit)
		if !errors.Is(err, ErrOBJTooLarge) {
			t.Fatalf("limit %d: wanted ErrOBJTooLarge, got %v", limit, err)
		}
		if int64(src.n) > limit+1 {
			t.Errorf("limit %d: wanted at most %d bytes read from the source, got %d", limit, limit+1, src.n)
		}
	}
}

func TestLoadOBJLimitCountsManySmallLines(t *testing.T) {
	text := strings.Repeat("# c\n", 5000) + "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"
	size := int64(len(text))

	if _, err := loadLimit(t, text, size); err != nil {
		t.Errorf("wanted the file to load at exactly its size, got %v", err)
	}
	if _, err := loadLimit(t, text, size-1); !errors.Is(err, ErrOBJTooLarge) {
		t.Errorf("wanted ErrOBJTooLarge one byte under, got %v", err)
	}
}

func TestLoadOBJLimitKeepsSourceErrors(t *testing.T) {
	// Under the limit, a failing source is still reported as itself.
	_, err := LoadOBJLimit(&brokenReader{after: "v 0 0 0\nv 1 0 0\n"}, 1000)
	if !errors.Is(err, errBroken) || errors.Is(err, ErrOBJTooLarge) {
		t.Errorf("wanted the source's error, got %v", err)
	}

	// A source that returns its last bytes together with the end-of-file marker.
	m, err := LoadOBJLimit(iotest.DataErrReader(strings.NewReader(objCube)), int64(len(objCube)))
	if err != nil || len(m.Faces) != 12 {
		t.Errorf("wanted the cube loaded from a DataErrReader, got %v and %d faces", err, len(m.Faces))
	}

	// One byte at a time still counts every byte.
	_, err = LoadOBJLimit(iotest.OneByteReader(strings.NewReader(objCube)), int64(len(objCube)-1))
	if !errors.Is(err, ErrOBJTooLarge) {
		t.Errorf("wanted ErrOBJTooLarge from a one-byte reader, got %v", err)
	}
}

// A file over the limit always fails. Which error comes back depends on how far the
// loader had read: usually ErrOBJTooLarge, but a malformed statement found before the
// limit is reached is reported as itself, so a caller should treat any error as a
// failed load and use errors.Is only to tell the two apart.
func TestLoadOBJLimitOverTheLimitAlwaysFails(t *testing.T) {
	padding := strings.Repeat("# padding\n", 100000)
	for _, text := range []string{
		"v 0 0 0\nv oops 0 0\n" + padding,
		"v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n" + padding,
	} {
		for _, limit := range []int64{100, 1000, 100000} {
			_, err := loadLimit(t, text, limit)
			if err == nil {
				t.Fatalf("limit %d: wanted an error for a %d byte file", limit, len(text))
			}
			if !errors.Is(err, ErrOBJTooLarge) && !strings.Contains(err.Error(), `obj line 2: bad number "oops"`) {
				t.Errorf("limit %d: wanted too large or the parse error on line 2, got %v", limit, err)
			}
		}
	}
}

func TestLoadOBJIsUnlimited(t *testing.T) {
	// More than any limit a test would pick, to show LoadOBJ has none of its own.
	text := strings.Repeat("# padding padding\n", 200000) + "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"

	m, err := loadWithin(t, text, 5*time.Second)
	if err != nil || len(m.Faces) != 1 {
		t.Errorf("wanted a %d byte file to load, got %v", len(text), err)
	}
}

type brokenReader struct{ after string }

var errBroken = errors.New("disk on fire")

func (r *brokenReader) Read(p []byte) (int, error) {
	if r.after == "" {
		return 0, errBroken
	}
	n := copy(p, r.after)
	r.after = r.after[n:]
	return n, nil
}

func TestLoadOBJReaderError(t *testing.T) {
	_, err := LoadOBJ(&brokenReader{after: "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"})
	if !errors.Is(err, errBroken) {
		t.Errorf("wanted the reader's error, got %v", err)
	}

	_, err = LoadOBJ(io.MultiReader(strings.NewReader("v 0 0 0\n"), &brokenReader{}))
	if !errors.Is(err, errBroken) {
		t.Errorf("wanted the reader's error, got %v", err)
	}
}

// ---- orientation ----

// objCube is a unit-ish cube the way a right-handed exporter writes it: corners
// listed counter-clockwise when seen from outside.
const objCube = `v -1 -1 -1
v  1 -1 -1
v  1  1 -1
v -1  1 -1
v -1 -1  1
v  1 -1  1
v  1  1  1
v -1  1  1
f 5 6 7 8
f 1 4 3 2
f 2 3 7 6
f 1 5 8 4
f 4 8 7 3
f 1 2 6 5
`

func TestLoadOBJCubeFacesOutward(t *testing.T) {
	m := loadOBJ(t, objCube)

	if len(m.Vertices) != 8 || len(m.Faces) != 12 || len(m.Edges) != 12 {
		t.Fatalf("wanted 8 vertices, 12 triangles, 12 edges, got %d, %d, %d", len(m.Vertices), len(m.Faces), len(m.Edges))
	}

	var volume float64
	for _, f := range m.Faces {
		a, b, c := m.Vertices[f[0]], m.Vertices[f[1]], m.Vertices[f[2]]
		normal := b.Sub(a).Cross(c.Sub(a)).Normalize()

		// The cube is centered on the origin, so outward means pointing away from it.
		if centroid := a.Add(b).Add(c).Scale(1.0 / 3); normal.Dot(centroid) <= 0 {
			t.Errorf("wanted face %v to point outward, normal %v at %v", f, normal, centroid)
		}
		volume += a.Dot(b.Cross(c)) / 6
	}
	// Outward winding everywhere gives the cube's true, positive volume.
	if !floatEquals(volume, 8) {
		t.Errorf("wanted volume 8, got %v", volume)
	}
}

// OBJ looks down -Z, so the front of a model faces +Z there. After loading, that
// front must face a camera looking down +Z from the -Z side, with X not mirrored.
func TestLoadOBJFrontFacesTheCamera(t *testing.T) {
	// A triangle on the right (x 1..3) at z=1, counter-clockwise seen from +Z, so
	// in OBJ terms it faces +Z.
	m := loadOBJ(t, "v 1 -1 1\nv 3 -1 1\nv 2 1 1\nf 1 2 3\n")
	light := sunAndFill(geometry.Vector3{Z: 1}, 0)

	front := Camera{Position: geometry.Vector3{Z: -5}, FOV: math.Pi / 2, Near: 0.1}
	fb := newShadeBuffer(t)
	m.withColor(engine.Palette(1)).DrawShaded(fb, front, light)
	cells := drawn(fb)
	if len(cells) == 0 {
		t.Fatalf("wanted the front of the model visible from the -Z side")
	}
	for _, c := range cells {
		if c[0] <= shadeW/2 {
			t.Fatalf("wanted the model on the right half of the screen (x not mirrored), got a cell at %v", c)
		}
	}

	// From the other side it is the back, and is culled.
	back := Camera{Position: geometry.Vector3{Z: 5}, Yaw: math.Pi, FOV: math.Pi / 2, Near: 0.1}
	fb = newShadeBuffer(t)
	m.withColor(engine.Palette(1)).DrawShaded(fb, back, light)
	if cells := drawn(fb); len(cells) != 0 {
		t.Errorf("wanted the back of the model culled, got %d cells", len(cells))
	}
}

func TestLoadOBJWireframeShowsPolygonsNotDiagonals(t *testing.T) {
	// A flat quad filling the middle of the view: its wireframe must be a hollow box.
	m := loadOBJ(t, "v -2 -2 5\nv 2 -2 5\nv 2 2 5\nv -2 2 5\nf 1 2 3 4\n")
	fb := newShadeBuffer(t)

	// The quad is at OBJ z=5, which is z=-5 here, so look at it from further back.
	m.Draw(fb, Camera{Position: geometry.Vector3{Z: -10}, FOV: math.Pi / 2, Near: 0.1}, engine.Cell{Character: '#'})
	// Walk in the middle of the quad: with a diagonal edge a cell would be lit there.
	if got := fb.At(shadeW/2, shadeH/2); got != engine.BlankCell {
		t.Errorf("wanted the quad's middle empty (no diagonal), got %+v", got)
	}
	if len(drawn(fb)) == 0 {
		t.Errorf("wanted the outline drawn")
	}
}

// quadGridOBJ is a grid of side by side quads, 100k of them for side 316: roughly the
// size of a detailed model. normals says how the faces refer to normals: "" for none,
// "vertex" for one normal at each vertex (a smooth surface), "face" for one for each
// quad, so that every vertex inside the grid is met with four different normals.
func quadGridOBJ(side int, normals string) string {
	var sb strings.Builder
	for y := 0; y <= side; y++ {
		for x := 0; x <= side; x++ {
			fmt.Fprintf(&sb, "v %d %d %d\n", x, y, (x*y)%5)
		}
	}
	switch normals {
	case "vertex":
		for y := 0; y <= side; y++ {
			for x := 0; x <= side; x++ {
				fmt.Fprintf(&sb, "vn 0 0 1\n")
			}
		}
	case "face":
		for y := 0; y < side; y++ {
			for x := 0; x < side; x++ {
				fmt.Fprintf(&sb, "vn %d 0 1\n", (x*y)%7)
			}
		}
	}
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			a := y*(side+1) + x + 1
			switch normals {
			case "vertex":
				fmt.Fprintf(&sb, "f %d//%d %d//%d %d//%d %d//%d\n", a, a, a+1, a+1, a+side+2, a+side+2, a+side+1, a+side+1)
			case "face":
				n := y*side + x + 1
				fmt.Fprintf(&sb, "f %d//%d %d//%d %d//%d %d//%d\n", a, n, a+1, n, a+side+2, n, a+side+1, n)
			default:
				fmt.Fprintf(&sb, "f %d %d %d %d\n", a, a+1, a+side+2, a+side+1)
			}
		}
	}
	return sb.String()
}

func BenchmarkLoadOBJ(b *testing.B) {
	for _, tt := range []struct{ name, normals string }{
		{"no normals", ""},
		{"a normal at each vertex", "vertex"},
		{"a normal for each face", "face"},
	} {
		text := quadGridOBJ(316, tt.normals)
		b.Run(tt.name, func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := LoadOBJ(strings.NewReader(text)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

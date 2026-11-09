package mesh

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kyrokohan/glonk/engine"
)

// mapFS makes a file system out of file names and their text.
func mapFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, text := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(text)}
	}
	return fsys
}

// recordingFS remembers every name opened in it, in order.
type recordingFS struct {
	fs.FS
	opened []string
}

func (r *recordingFS) Open(name string) (fs.File, error) {
	r.opened = append(r.opened, name)
	return r.FS.Open(name)
}

// loadFS loads the OBJ called name from files with a generous limit.
func loadFS(t *testing.T, files map[string]string, name string) Mesh {
	t.Helper()
	m, err := LoadOBJFS(mapFS(files), name, 1<<24)
	if err != nil {
		t.Fatalf("unable to load %q: %v", name, err)
	}
	return m
}

// triOBJ is a file of one triangle for each material named: a library m.mtl beside it
// and, for each name, a usemtl and the triangle. A name of "" leaves out the usemtl.
func triOBJ(materials ...string) string {
	var sb strings.Builder
	sb.WriteString("mtllib m.mtl\nv 0 0 0\nv 1 0 0\nv 0 1 0\n")
	for _, name := range materials {
		if name != "" {
			sb.WriteString("usemtl " + name + "\n")
		}
		sb.WriteString("f 1 2 3\n")
	}
	return sb.String()
}

var (
	pureRed   = engine.RGB(255, 0, 0)
	pureGreen = engine.RGB(0, 255, 0)
	pureBlue  = engine.RGB(0, 0, 255)
	none      = engine.Color{}
)

func TestLoadOBJFSColorsEachFaceFromItsMaterial(t *testing.T) {
	m := loadFS(t, map[string]string{
		"house.obj": `mtllib house.mtl
v 0 0 0
v 1 0 0
v 1 1 0
v 0 1 0
v 0 0 1
usemtl wall
f 1 2 3 4
usemtl roof
f 1 2 5
usemtl nothing
f 2 3 5
f 3 4 5
`,
		"house.mtl": "newmtl wall\nKd 1 0 0\nnewmtl roof\nKd 0 0 1\n",
	}, "house.obj")

	// The quad is two triangles and gives both its color; the unknown material colors
	// nothing, and neither does the face after it.
	if want := []engine.Color{pureRed, pureRed, pureBlue, none, none}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted face colors %v, got %v", want, m.FaceColors)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("wanted a valid mesh, got %v", err)
	}
}

// Every way a Kd line can be written, with what it must give. The zero Color means the
// material has no color.
var kdForms = []struct {
	text  string
	color engine.Color
}{
	{"Kd 1 0 0", pureRed},
	{"Kd 0.5 0.2 0.8", engine.RGB(128, 51, 204)}, // 127.5 rounds up
	{"Kd 1.7 -0.3 0.5", engine.RGB(255, 0, 128)}, // clamped to 0 to 1
	{"Kd 0.2", engine.RGB(51, 51, 51)},           // one number is a gray
	{"Kd 0 0 0", engine.RGB(0, 0, 0)},            // black is a color, not "none"
	{"Kd 2e-1 1E0 .5", engine.RGB(51, 255, 128)},
	{"Kd   0.5\t0.5   0.5  ", engine.RGB(128, 128, 128)},
	{"Kd 1 0 0 0.5", pureRed}, // numbers after the third are ignored
	{"Kd 1 0 0 # red", pureRed},
	{"", none},
	{"Kd", none},
	{"Kd 0.5 0.5", none},
	{"Kd spectral file.rfl 1", none},
	{"Kd xyz 0.5 0.5 0.5", none},
	{"Kd nan 0 0", none},
	{"Kd inf 0 0", none},
	{"Kd 1 -inf 0", none},
	{"Kd 1e999 0 0", none},
	{"Kd 1 0 x", none},
	{"kd 1 0 0", none},
	{"# Kd 1 0 0", none},
}

func TestMTLKdForms(t *testing.T) {
	for _, form := range kdForms {
		m := loadFS(t, map[string]string{
			"m.obj": triOBJ("m"),
			"m.mtl": "newmtl m\n" + form.text + "\n",
		}, "m.obj")

		var want []engine.Color
		if form.color != none {
			want = []engine.Color{form.color}
		}
		if !slices.Equal(m.FaceColors, want) || (want == nil) != (m.FaceColors == nil) {
			t.Errorf("%q: wanted face colors %v, got %v", form.text, want, m.FaceColors)
		}
	}
}

func TestMTLMaterialsRedefineAndStartOver(t *testing.T) {
	for _, tt := range []struct {
		name, mtl string
		want      engine.Color
	}{
		{"a later Kd wins", "newmtl m\nKd 1 0 0\nKd 0 1 0\n", pureGreen},
		{"a later definition wins", "newmtl m\nKd 1 0 0\nnewmtl m\nKd 0 1 0\n", pureGreen},
		{"a later definition without a Kd has none", "newmtl m\nKd 1 0 0\nnewmtl m\nKa 1 1 1\n", none},
		{"a Kd belongs to the material before it", "newmtl m\nnewmtl other\nKd 1 0 0\n", none},
		{"a Kd before any material is lost", "Kd 1 0 0\nnewmtl m\n", none},
		{"other statements are skipped", "newmtl m\nKa 1 0 0\nKs 1 0 0\nTf 1 0 0\nNs 5\nd 0.5\nillum 2\nmap_Kd m.png\nKd 0 0 1\n", pureBlue},
		{"spaces before the name", "newmtl    m  \nKd 1 0 0\n", pureRed},
		{"a comment after the name", "newmtl m # the wall\nKd 1 0 0 # red\n", pureRed},
		{"CRLF line endings", "newmtl m\r\nKd 1 0 0\r\n", pureRed},
		{"no newline at the end", "newmtl m\nKd 1 0 0", pureRed},
	} {
		m := loadFS(t, map[string]string{"m.obj": triOBJ("m"), "m.mtl": tt.mtl}, "m.obj")
		got := none
		if m.FaceColors != nil {
			got = m.FaceColors[0]
		}
		if got != tt.want {
			t.Errorf("%s: wanted %v, got %v", tt.name, tt.want, got)
		}
	}
}

func TestMTLNamesWithSpacesAndNoName(t *testing.T) {
	m := loadFS(t, map[string]string{
		"m.obj": "mtllib m.mtl\nv 0 0 0\nv 1 0 0\nv 0 1 0\nusemtl my mat \r\nf 1 2 3\nusemtl\nf 1 2 3\n",
		"m.mtl": "newmtl my mat\nKd 1 0 0\nnewmtl\nKd 0 1 0\n",
	}, "m.obj")

	// A newmtl or usemtl with no name does not match the other, nor anything.
	if want := []engine.Color{pureRed, none}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted face colors %v, got %v", want, m.FaceColors)
	}
}

func TestLoadOBJFSUsemtlBeforeMtllibFindsNothing(t *testing.T) {
	m := loadFS(t, map[string]string{
		"m.obj": "v 0 0 0\nv 1 0 0\nv 0 1 0\nusemtl m\nf 1 2 3\nmtllib m.mtl\nf 1 2 3\nusemtl m\nf 1 2 3\n",
		"m.mtl": "newmtl m\nKd 1 0 0\n",
	}, "m.obj")

	// The first face asked for a material before it existed, and the second keeps that
	// choice; only the third asks again.
	if want := []engine.Color{none, none, pureRed}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted face colors %v, got %v", want, m.FaceColors)
	}
}

func TestLoadOBJFSColorsStartAtTheFirstColoredFace(t *testing.T) {
	m := loadFS(t, map[string]string{
		"m.obj": "mtllib m.mtl\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\nf 1 2 3\nusemtl m\nf 1 2 3\n",
		"m.mtl": "newmtl m\nKd 1 0 0\n",
	}, "m.obj")

	if want := []engine.Color{none, none, pureRed}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted the faces before the first color to have none, got %v", m.FaceColors)
	}
}

func TestLoadOBJFSWithoutUsableMaterialsHasNoFaceColors(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"no mtllib":        {"m.obj": "v 0 0 0\nv 1 0 0\nv 0 1 0\nusemtl m\nf 1 2 3\n"},
		"no usemtl":        {"m.obj": "mtllib m.mtl\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n", "m.mtl": "newmtl m\nKd 1 0 0\n"},
		"missing library":  {"m.obj": triOBJ("m")},
		"empty library":    {"m.obj": triOBJ("m"), "m.mtl": ""},
		"junk library":     {"m.obj": triOBJ("m"), "m.mtl": "\x00\xff\x1b[ newmtl\n\xfe"},
		"no such material": {"m.obj": triOBJ("x"), "m.mtl": "newmtl m\nKd 1 0 0\n"},
		"a directory":      {"m.obj": triOBJ("m"), "m.mtl/inside": "newmtl m\nKd 1 0 0\n"},
	} {
		m := loadFS(t, files, "m.obj")
		if m.FaceColors != nil {
			t.Errorf("%s: wanted no face colors, got %v", name, m.FaceColors)
		}
		plain, err := LoadOBJ(strings.NewReader(files["m.obj"]))
		if err != nil || !sameGeometry(m, plain) {
			t.Errorf("%s: wanted the model of LoadOBJ, got %v and err %v", name, m, err)
		}
	}
}

// sameGeometry says whether two meshes have the same vertices, faces, edges and normals.
func sameGeometry(a, b Mesh) bool {
	return slices.Equal(a.Vertices, b.Vertices) && slices.Equal(a.Faces, b.Faces) &&
		slices.Equal(a.Edges, b.Edges) && slices.Equal(a.Normals, b.Normals)
}

func TestLoadOBJFSKeepsNormalsAndSplitVerticesWithColors(t *testing.T) {
	// The cube as exporters write it: 6 normals, so 24 vertices once split, and a color
	// for each of two sides.
	text := "mtllib c.mtl\n" + strings.Replace(objCubeWithNormals, "f 1//1", "usemtl a\nf 1//1", 1)
	text = strings.Replace(text, "f 5//2", "usemtl b\nf 5//2", 1)
	m := loadFS(t, map[string]string{"c.obj": text, "c.mtl": "newmtl a\nKd 1 0 0\nnewmtl b\nKd 0 0 1\n"}, "c.obj")

	plain, err := LoadOBJ(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	if !sameGeometry(m, plain) || len(m.Vertices) != 24 || len(m.Normals) != 24 {
		t.Errorf("wanted the 24 vertices and normals of LoadOBJ, got %d and %d", len(m.Vertices), len(m.Normals))
	}
	// Side 1 and 2 are 2 triangles each; "b" stays in effect for the sides after it.
	want := []engine.Color{pureRed, pureRed, pureBlue, pureBlue, pureBlue, pureBlue, pureBlue, pureBlue, pureBlue, pureBlue, pureBlue, pureBlue}
	if !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted face colors %v, got %v", want, m.FaceColors)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("wanted a valid mesh, got %v", err)
	}
}

func TestLoadOBJAndLimitIgnoreMaterials(t *testing.T) {
	text := "mtllib m.mtl\nv 0 0 0\nv 1 0 0\nv 0 1 0\nusemtl m\nf 1 2 3\n"

	m, err := LoadOBJ(strings.NewReader(text))
	if err != nil || m.FaceColors != nil {
		t.Errorf("LoadOBJ: wanted no face colors, got %v and err %v", m.FaceColors, err)
	}
	m, err = LoadOBJLimit(strings.NewReader(text), 1<<20)
	if err != nil || m.FaceColors != nil {
		t.Errorf("LoadOBJLimit: wanted no face colors, got %v and err %v", m.FaceColors, err)
	}
}

func TestLoadOBJFSWithoutMaterialsIsLoadOBJLimit(t *testing.T) {
	for _, text := range []string{objCube, objCubeWithNormals} {
		got, err := LoadOBJFS(mapFS(map[string]string{"a/b.obj": text}), "a/b.obj", 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		want, err := LoadOBJLimit(strings.NewReader(text), 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if !sameGeometry(got, want) || got.FaceColors != nil {
			t.Errorf("wanted the mesh of LoadOBJLimit, got %v", got)
		}
	}
}

func TestLoadOBJFSRefusesNamesOutsideTheModelsFolder(t *testing.T) {
	// Each name has a file where an unchecked reader would find it: above the model,
	// at the root, with the backslash as a letter. None of them may even be opened.
	const colored = "newmtl m\nKd 1 0 0\n"
	files := map[string]string{
		"secret.mtl":          colored,
		"dir/sub/inner.mtl":   colored,
		"dir/back\\slash.mtl": colored,
	}
	for _, name := range []string{
		"../secret.mtl", "sub/../../secret.mtl", "/secret.mtl", "/dir/sub/inner.mtl",
		`back\slash.mtl`, `sub\inner.mtl`, "//secret.mtl", "../dir/sub/inner.mtl",
	} {
		fsys := mapFS(files)
		fsys["dir/m.obj"] = &fstest.MapFile{Data: []byte(strings.Replace(triOBJ("m"), "m.mtl", name, 1))}
		rec := &recordingFS{FS: fsys}

		m, err := LoadOBJFS(rec, "dir/m.obj", 1<<20)
		if err != nil || m.FaceColors != nil {
			t.Errorf("%q: wanted the model without colors, got %v and err %v", name, m.FaceColors, err)
		}
		if want := []string{"dir/m.obj"}; !slices.Equal(rec.opened, want) {
			t.Errorf("%q: wanted only %v opened, got %v", name, want, rec.opened)
		}
	}
}

func TestLoadOBJFSLooksBesideTheModel(t *testing.T) {
	rec := &recordingFS{FS: mapFS(map[string]string{
		"dir/m.obj":     "mtllib m.mtl ./n.mtl sub/o.mtl sub/../p.mtl\nv 0 0 0\nv 1 0 0\nv 0 1 0\nusemtl m\nf 1 2 3\n",
		"dir/m.mtl":     "newmtl m\nKd 1 0 0\n",
		"dir/n.mtl":     "newmtl m\nKd 0 1 0\n",
		"dir/sub/o.mtl": "newmtl m\nKd 0 0 1\n",
		"m.mtl":         "newmtl m\nKd 0.4\n", // a model's own folder is the only place to look
	})}
	m, err := LoadOBJFS(rec, "dir/m.obj", 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"dir/m.obj", "dir/m.mtl", "dir/n.mtl", "dir/sub/o.mtl", "dir/p.mtl"}; !slices.Equal(rec.opened, want) {
		t.Errorf("wanted %v opened, got %v", want, rec.opened)
	}
	// Each library is read where it stands, so the last definition of m is the one used.
	if want := []engine.Color{pureBlue}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceColors)
	}
}

func TestLoadOBJFSNameAndFileErrors(t *testing.T) {
	files := mapFS(map[string]string{"m.obj": triOBJ("m"), "d/f.obj": triOBJ("m")})

	rec := &recordingFS{FS: files}
	for _, name := range []string{"", "../m.obj", "/m.obj", "d/", "d//f.obj", "./m.obj", "d/../m.obj"} {
		if _, err := LoadOBJFS(rec, name, 1<<20); err == nil {
			t.Errorf("%q: wanted an error for a name that is not a valid path", name)
		}
	}
	if len(rec.opened) != 0 {
		t.Errorf("wanted nothing opened for invalid names, got %v", rec.opened)
	}

	if _, err := LoadOBJFS(files, "gone.obj", 1<<20); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("wanted fs.ErrNotExist for a missing file, got %v", err)
	}
	if _, err := LoadOBJFS(files, "d", 1<<20); err == nil {
		t.Errorf("wanted an error for a directory")
	}
	if _, err := LoadOBJFS(files, ".", 1<<20); err == nil {
		t.Errorf("wanted an error for the root")
	}
	for _, limit := range []int64{0, -1} {
		if _, err := LoadOBJFS(files, "m.obj", limit); err == nil || errors.Is(err, ErrOBJTooLarge) {
			t.Errorf("limit %d: wanted an error about the limit, got %v", limit, err)
		}
	}
}

func TestLoadOBJFSLimitIsLoadOBJLimits(t *testing.T) {
	text := triOBJ("m")
	files := mapFS(map[string]string{"m.obj": text, "m.mtl": "newmtl m\nKd 1 0 0\n"})

	if m, err := LoadOBJFS(files, "m.obj", int64(len(text))); err != nil || len(m.FaceColors) != 1 {
		t.Errorf("wanted a file of exactly the limit to load with its color, got %v", err)
	}
	if _, err := LoadOBJFS(files, "m.obj", int64(len(text))-1); !errors.Is(err, ErrOBJTooLarge) {
		t.Errorf("wanted ErrOBJTooLarge one byte under, got %v", err)
	}
}

func TestLoadOBJFSReadsEachLibraryUpToTheCap(t *testing.T) {
	const limit = 1 << 20 // written out, so that changing the constant changes the test
	early, edge, late := "newmtl early\nKd 1 0 0\n", "newmtl edge\nKd 0 1 0", "newmtl late\nKd 0 0 1\n"
	comment := func(n int) string { return "#" + strings.Repeat("x", n-2) + "\n" } // n bytes, n >= 2
	obj := triOBJ("early", "edge", "late")

	load := func(mtl string, maxBytes int64) []engine.Color {
		t.Helper()
		m, err := LoadOBJFS(mapFS(map[string]string{"m.obj": obj, "m.mtl": mtl}), "m.obj", maxBytes)
		if err != nil {
			t.Fatal(err)
		}
		return m.FaceColors
	}

	// A file of exactly the cap is read to its last byte, which is not a newline.
	atCap := early + comment(limit-len(early)-len(edge)) + edge
	if len(atCap) != limit {
		t.Fatalf("wanted a file of %d bytes, built %d", limit, len(atCap))
	}
	if got, want := load(atCap, 1<<30), []engine.Color{pureRed, pureGreen, none}; !slices.Equal(got, want) {
		t.Errorf("at the cap: wanted %v, got %v", want, got)
	}

	// One byte more and the file is skipped whole: the early material goes with it.
	if got := load(atCap+"\n", 1<<30); got != nil {
		t.Errorf("over the cap: wanted the whole file skipped, got %v", got)
	}
	if got := load(early+comment(limit-len(early))+late, 1<<30); got != nil {
		t.Errorf("a material past the cap: wanted the whole file skipped, got %v", got)
	}

	// The OBJ's own limit is the cap when it is smaller.
	small := early + comment(int(len(obj)))
	if got := load(small, int64(len(obj))); got != nil {
		t.Errorf("over the OBJ's limit: wanted the file skipped, got %v", got)
	}
	if got, want := load(early, int64(len(obj))), []engine.Color{pureRed, none, none}; !slices.Equal(got, want) {
		t.Errorf("under the OBJ's limit: wanted %v, got %v", want, got)
	}
}

func TestLoadOBJFSReadsAtMostEightLibraries(t *testing.T) {
	files := map[string]string{}
	var names []string
	for i := range 10 {
		files[fmt.Sprintf("l%d.mtl", i)] = fmt.Sprintf("newmtl m%d\nKd 1 0 0\n", i)
		names = append(names, fmt.Sprintf("l%d.mtl", i))
	}

	// Two statements, the second starting where the first stopped; names that do not
	// exist count as well as those that do.
	files["m.obj"] = "mtllib " + strings.Join(names[:3], " ") + " gone.mtl\nmtllib " + strings.Join(names[3:], " ") +
		"\nv 0 0 0\nv 1 0 0\nv 0 1 0\nusemtl m2\nf 1 2 3\nusemtl m3\nf 1 2 3\nusemtl m4\nf 1 2 3\nusemtl m5\nf 1 2 3\nusemtl m6\nf 1 2 3\nusemtl m7\nf 1 2 3\nusemtl m8\nf 1 2 3\n"
	m := loadFS(t, files, "m.obj")

	// l0 to l2, gone and l3 to l6 are the eight read; l7 and on are not.
	if want := []engine.Color{pureRed, pureRed, pureRed, pureRed, pureRed, none, none}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceColors)
	}
}

func TestLoadOBJFSKeepsAtMostTenThousandMaterials(t *testing.T) {
	var big strings.Builder
	for i := range 10_000 {
		fmt.Fprintf(&big, "newmtl m%d\nKd 1 0 0\n", i)
	}
	files := map[string]string{
		"m.obj": "mtllib big.mtl more.mtl\nv 0 0 0\nv 1 0 0\nv 0 1 0\n" +
			"usemtl m0\nf 1 2 3\nusemtl m9999\nf 1 2 3\nusemtl extra\nf 1 2 3\nusemtl other\nf 1 2 3\n",
		"big.mtl": big.String(),
		// With no room left, a known name can still be redefined and a new one cannot.
		"more.mtl": "newmtl extra\nKd 0 1 0\nnewmtl other\nKd 0 1 0\nnewmtl m0\nKd 0 0 1\n",
	}
	m := loadFS(t, files, "m.obj")

	// big.mtl is under the cap, so all 10000 are in; m0 was redefined after.
	if want := []engine.Color{pureBlue, pureRed, none, none}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceColors)
	}
}

func TestLoadOBJFSLongLinesAreFine(t *testing.T) {
	m := loadFS(t, map[string]string{
		"m.obj": triOBJ("m"),
		"m.mtl": "# " + strings.Repeat("x", 900_000) + "\nnewmtl m\nKd 1 0 0\n",
	}, "m.obj")
	if want := []engine.Color{pureRed}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceColors)
	}
}

// Eight libraries of the biggest size, all of the lines that cost the most, must still be
// quick to read.
func TestLoadOBJFSHostileLibrariesAreCheap(t *testing.T) {
	files := map[string]string{}
	var names []string
	for i := range 8 {
		names = append(names, fmt.Sprintf("l%d.mtl", i))
		files[names[i]] = strings.Repeat("newmtl a\nKd 1 1 1\n", maxMTLBytes/18)
	}
	files["m.obj"] = "mtllib " + strings.Join(names, " ") + "\n" + triOBJ("a")

	done := make(chan struct{})
	go func() {
		defer close(done)
		if m, err := LoadOBJFS(mapFS(files), "m.obj", 1<<24); err != nil || len(m.FaceColors) != 1 {
			t.Errorf("wanted the model to load with its color, got %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("wanted 8 MiB of libraries to load within 10 seconds, still running")
	}
}

// checkMTLModel loads the OBJ and libraries that mtlModelFrom makes from data and checks
// the color of every triangle against what the oracle says, and that the geometry is
// exactly what LoadOBJ makes of the same text.
func checkMTLModel(t testing.TB, data []byte) {
	t.Helper()
	files, text, want := mtlModelFrom(data)
	files["dir/model.obj"] = &fstest.MapFile{Data: []byte(text)}
	dump := func() string {
		s := "dir/model.obj:\n" + text
		for _, name := range []string{"dir/a.mtl", "dir/b.mtl", "dir/sub/c.mtl"} {
			s += name + ":\n" + string(files[name].Data)
		}
		return s
	}

	m, err := LoadOBJFS(files, "dir/model.obj", 1<<20)
	if err != nil {
		t.Fatalf("wanted the files to load, got %v for\n%s", err, dump())
	}
	if (want == nil) != (m.FaceColors == nil) || !slices.Equal(m.FaceColors, want) {
		t.Fatalf("wanted face colors %v, got %v for\n%s", want, m.FaceColors, dump())
	}
	plain, err := LoadOBJ(strings.NewReader(text))
	if err != nil || !sameGeometry(m, plain) {
		t.Fatalf("wanted the geometry of LoadOBJ, got err %v for\n%s", err, dump())
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("wanted a valid mesh, got %v for\n%s", err, dump())
	}
}

var (
	mtlNames = []string{"wall", "roof", "dark wood", "door"}
	// mtlLibraries are the ways an OBJ in dir/ names a library, and where that is.
	mtlLibraries = []struct{ name, file string }{
		{"a.mtl", "dir/a.mtl"}, {"./b.mtl", "dir/b.mtl"}, {"sub/c.mtl", "dir/sub/c.mtl"},
		{"sub/../a.mtl", "dir/a.mtl"}, {"gone.mtl", ""},
	}
)

// mtlModelFrom builds an OBJ in dir/ and the libraries it names from the bytes it is
// given, together with the color every triangle must come out with. It is the oracle for
// materials, written without looking at the loader: up to 6 polygons of 3 to 5 corners,
// each after a usemtl that picks a material, an unknown one, none or no change, and
// mtllib statements in between that name up to 2 of three libraries (in four spellings),
// or one that is not there. Each library defines up to 4 materials with a Kd written in
// any of the forms, and a decoy copy of each at the root holds every material in gray,
// for a loader that looks in the wrong folder.
func mtlModelFrom(data []byte) (files fstest.MapFS, text string, want []engine.Color) {
	next := func() int {
		if len(data) == 0 {
			return 0
		}
		b := data[0]
		data = data[1:]
		return int(b)
	}

	type def struct{ name, kd int }
	files = fstest.MapFS{}
	defs := map[string][]def{}
	for _, file := range []string{"dir/a.mtl", "dir/b.mtl", "dir/sub/c.mtl"} {
		var sb strings.Builder
		sb.WriteString("# a library\n")
		for range next() % 5 {
			d := def{next() % len(mtlNames), next() % len(kdForms)}
			defs[file] = append(defs[file], d)
			fmt.Fprintf(&sb, "newmtl %s\nKa 1 0 0\nKs 0 0 1\n", mtlNames[d.name])
			if kd := kdForms[d.kd].text; kd != "" {
				sb.WriteString(kd + "\n")
			}
			sb.WriteString("Ns 10\nd 1\n\n")
		}
		files[file] = &fstest.MapFile{Data: []byte(sb.String())}
	}
	var decoy strings.Builder
	for _, name := range mtlNames {
		fmt.Fprintf(&decoy, "newmtl %s\nKd 0.4\n", name)
	}
	for _, file := range []string{"a.mtl", "b.mtl", "sub/c.mtl"} {
		files[file] = &fstest.MapFile{Data: []byte(decoy.String())}
	}

	var sb strings.Builder
	for i := range 6 {
		fmt.Fprintf(&sb, "v %d %d 0\n", i, i*i%5)
	}
	table := map[string]engine.Color{} // the materials known at this point of the file
	var current engine.Color
	named := 0
	for range 1 + next()%6 {
		if next()%3 == 0 {
			var used []string
			for range 1 + next()%2 {
				if named == maxMTLLibraries {
					break
				}
				named++
				lib := mtlLibraries[next()%len(mtlLibraries)]
				used = append(used, lib.name)
				for _, d := range defs[lib.file] {
					table[mtlNames[d.name]] = kdForms[d.kd].color
				}
			}
			if len(used) > 0 {
				fmt.Fprintf(&sb, "mtllib %s\n", strings.Join(used, " "))
			}
		}

		switch k := next() % 7; {
		case k < len(mtlNames):
			fmt.Fprintf(&sb, "usemtl %s\n", mtlNames[k])
			current = table[mtlNames[k]]
		case k == len(mtlNames):
			sb.WriteString("usemtl unknown\n")
			current = none
		case k == len(mtlNames)+1:
			sb.WriteString("usemtl\n")
			current = none
		}

		corners := 3 + next()%3
		sb.WriteString("f")
		for range corners {
			fmt.Fprintf(&sb, " %d", 1+next()%6)
		}
		sb.WriteString("\n")
		for range corners - 2 {
			want = append(want, current)
		}
	}
	if !slices.ContainsFunc(want, func(c engine.Color) bool { return c != none }) {
		want = nil
	}
	return files, sb.String(), want
}

func TestLoadOBJFSMatchesAnIndependentModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 32))
	var colored, mixed int
	for range 5000 {
		data := make([]byte, 140)
		for i := range data {
			data[i] = byte(rng.IntN(256))
		}
		checkMTLModel(t, data)

		if _, _, want := mtlModelFrom(data); want != nil {
			colored++
			if slices.Contains(want, none) {
				mixed++
			}
		}
	}
	// Guard: the oracle is not a test of "nothing is colored", and it does reach faces
	// with a color beside faces without.
	if colored < 500 || mixed < 200 {
		t.Errorf("wanted at least 500 colored and 200 mixed models of 5000, got %d and %d", colored, mixed)
	}
}

func FuzzLoadOBJFSModel(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{2, 3, 0, 4, 0, 1, 2, 3, 0, 0, 1, 0, 0, 2, 5, 6, 7})
	f.Add([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})

	f.Fuzz(func(t *testing.T, data []byte) {
		checkMTLModel(t, data)
	})
}

func FuzzLoadOBJFS(f *testing.F) {
	for _, seed := range [][2]string{
		{objCube, "newmtl m\nKd 1 0 0\n"},
		{"usemtl m\nv 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n", "newmtl m\nKd 0.5\n"},
		{"v 0 0 0\nv 1 0 0\nv 0 1 0\nusemtl m\nf 1 2 3\nusemtl\nf 1 2 3 \\\n", "newmtl m\r\nKd 1 0 0\r\nnewmtl\r\n"},
		{objCubeWithNormals, "newmtl a\nKd nan 0 0\nnewmtl b\nKd 1e999\nKd xyz 1 1 1\n"},
		{"mtllib m.mtl m.mtl ../m.mtl /m.mtl m\\x.mtl\nv 0 0 0\nf 1 1 1\n", "\x00\xff newmtl \x1b["},
		{"v 0 0 0\nf 1 1 1\nusemtl a\nf 1 1 1\n", strings.Repeat("newmtl a\nKd 1 1 1\n", 500)},
	} {
		f.Add(seed[0], seed[1])
	}

	f.Fuzz(func(t *testing.T, obj, mtl string) {
		// Naming the library first makes the fuzzer's job about its text, not its name.
		text := "mtllib m.mtl\n" + obj
		files := mapFS(map[string]string{"m.obj": text, "m.mtl": mtl})

		m, err := LoadOBJFS(files, "m.obj", 1<<20)
		plain, plainErr := LoadOBJ(strings.NewReader(text))
		if (err == nil) != (plainErr == nil) && len(text) <= 1<<20 {
			t.Fatalf("wanted LoadOBJFS and LoadOBJ to agree about errors, got %v and %v", err, plainErr)
		}
		if err != nil {
			return
		}

		if !sameGeometry(m, plain) {
			t.Fatalf("wanted the geometry of LoadOBJ")
		}
		if m.FaceColors != nil && len(m.FaceColors) != len(m.Faces) {
			t.Fatalf("wanted no face colors or one for each of %d faces, got %d", len(m.Faces), len(m.FaceColors))
		}
		if err := m.Validate(); err != nil {
			t.Fatalf("wanted a valid mesh, got %v", err)
		}

		again, err := LoadOBJFS(files, "m.obj", 1<<20)
		if err != nil || !sameGeometry(again, m) || !slices.Equal(again.FaceColors, m.FaceColors) {
			t.Fatalf("wanted loading twice to give the same mesh")
		}
	})
}

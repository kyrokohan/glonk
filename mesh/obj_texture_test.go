package mesh

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"glonk.dev/engine"
)

// loadTextured loads m.obj from a file system with it, m.mtl and the given images.
func loadTextured(t testing.TB, obj, mtl string, images map[string][]byte) Mesh {
	t.Helper()
	m, err := LoadOBJFS(texturedFS(obj, mtl, images), "m.obj", 1<<26)
	if err != nil {
		t.Fatalf("unable to load the model: %v", err)
	}
	checkFaceColors(t, m)
	return m
}

func texturedFS(obj, mtl string, images map[string][]byte) fstest.MapFS {
	fsys := imageFS(images)
	fsys["m.obj"] = &fstest.MapFile{Data: []byte(obj)}
	fsys["m.mtl"] = &fstest.MapFile{Data: []byte(mtl)}
	return fsys
}

// checkFaceColors holds a loaded mesh to what every mesh with colors must be.
func checkFaceColors(t testing.TB, m Mesh) {
	t.Helper()
	if m.FaceColors != nil && len(m.FaceColors) != len(m.Faces) {
		t.Fatalf("wanted no face colors or one for each of %d faces, got %d", len(m.Faces), len(m.FaceColors))
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("wanted a valid mesh, got %v", err)
	}
}

// triOBJVerts is the three vertices every face below is drawn on.
const triOBJVerts = "v 0 0 0\nv 1 0 0\nv 0 1 0\n"

func TestTexturedQuadColorsEachTriangleAtItsOwnMiddle(t *testing.T) {
	// One texture coordinate at the middle of each of four texels, cut into two triangles:
	// the first has its middle at (0.625, 0.4167), in column 2 of the bottom row, and the
	// second at (0.375, 0.5833), in column 1 of the top row.
	obj := "mtllib m.mtl\nv 0 0 0\nv 1 0 0\nv 1 1 0\nv 0 1 0\n" +
		"vt 0.125 0.25\nvt 0.875 0.25\nvt 0.875 0.75\nvt 0.125 0.75\n" +
		"usemtl t\nf 1/1 2/2 3/3 4/4\n"
	m := loadTextured(t, obj, "newmtl t\nmap_Kd t.png\n", map[string][]byte{"t.png": encodePNG(t, gridImage())})

	if want := []engine.Color{gridColors[1][2], gridColors[0][1]}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceColors)
	}
}

func TestTexturedFacesFallBackToKdThenToNothing(t *testing.T) {
	mtl := `newmtl t
map_Kd t.png
Kd 1 0 0
newmtl plain
Kd 0 0 1
newmtl gone
map_Kd gone.png
newmtl onlytex
map_Kd t.png
newmtl broken
map_Kd bad.png
Kd 0 1 0
`
	obj := "mtllib m.mtl\n" + triOBJVerts + "vt 0.125 0.25\nvt 0.875 0.75\n" +
		"usemtl t\n" +
		"f 1/1 2/1 3/1\n" + // the texture beats Kd
		"f 1/1 2/1 3\n" + // a corner without a coordinate: Kd
		"f 1/1 2/1 3/9\n" + // a coordinate that is not there: Kd
		"f 1/1 2/1 3/0\n" + // index 0 is none: Kd
		"usemtl gone\nf 1/1 2/1 3/1\n" + // the image is missing and there is no Kd
		"usemtl plain\nf 1/1 2/1 3/1\n" + // no texture at all: Kd
		"usemtl onlytex\nf 3 2 1\n" + // a texture and no coordinates, no Kd
		"usemtl onlytex\nf 1/-1 2/-1 3/-1\n" + // -1 is the last coordinate read
		"usemtl onlytex\nf 1/-2 2/-2 3/-2\n" + // and -2 the one before
		"usemtl broken\nf 1/1 2/1 3/1\n" // not a picture: Kd
	images := map[string][]byte{"t.png": encodePNG(t, gridImage()), "bad.png": []byte("not an image")}
	m := loadTextured(t, obj, mtl, images)

	red, blue, green := pureRed, pureBlue, pureGreen
	want := []engine.Color{
		gridColors[1][0], red, red, red, none, blue, none, gridColors[0][3], gridColors[1][0], green,
	}
	if !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceColors)
	}
}

func TestTexturedCoordinatesInEveryFormKeepLaterIndicesRight(t *testing.T) {
	// Each coordinate is used by one face, and the garbage ones still take their place.
	obj := "mtllib m.mtl\n" + triOBJVerts +
		"vt x y\n" + // 1: not numbers
		"vt 0.125\n" + // 2: u only, so v is 0: the bottom row
		"vt 0.625 0.75 0.9\n" + // 3: w is ignored
		"vt\n" + // 4: nothing
		"vt 0.875 0.25\n" + // 5
		"vt 0.5 nan\n" + // 6
		"vt inf 0.5\n" + // 7
		"usemtl t\n"
	for i := 1; i <= 7; i++ {
		obj += fmt.Sprintf("f 1/%d 2/%d 3/%d\n", i, i, i)
	}
	m := loadTextured(t, obj, "newmtl t\nmap_Kd t.png\nKd 0 1 0\n", map[string][]byte{"t.png": encodePNG(t, gridImage())})

	want := []engine.Color{pureGreen, gridColors[1][0], gridColors[0][2], pureGreen, gridColors[1][3], pureGreen, pureGreen}
	if !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceColors)
	}
}

func TestTexturedFaceColorsStartAtTheFirstColoredFace(t *testing.T) {
	obj := "mtllib m.mtl\n" + triOBJVerts + "vt 0.125 0.25\nf 1 2 3\nusemtl t\nf 1/1 2/1 3/1\nusemtl nosuch\nf 1 2 3\n"
	m := loadTextured(t, obj, "newmtl t\nmap_Kd t.png\n", map[string][]byte{"t.png": encodePNG(t, gridImage())})
	if want := []engine.Color{none, gridColors[1][0], none}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceColors)
	}

	// With no image to read and no Kd there is nothing to color with, and no list.
	m = loadTextured(t, obj, "newmtl t\nmap_Kd t.png\n", nil)
	if m.FaceColors != nil {
		t.Errorf("wanted no face colors, got %v", m.FaceColors)
	}
}

func TestTexturedModelWithNormalsKeepsItsSplitVertices(t *testing.T) {
	// Three triangles, with different normals on shared places, so the places split;
	// the colors must follow the faces and not the vertices.
	obj := "mtllib m.mtl\n" + triOBJVerts + "v 1 1 0\n" +
		"vn 0 0 1\nvn 0 1 0\nvt 0.125 0.25\nvt 0.875 0.75\nusemtl t\n" +
		"f 1/1/1 2/1/1 3/1/1\n" +
		"f 1/2/2 2/2/2 4/2/2\n" +
		"f 1//1 3//2 4//2\n"
	files := map[string][]byte{"t.png": encodePNG(t, gridImage())}
	m := loadTextured(t, obj, "newmtl t\nmap_Kd t.png\n", files)

	plain, err := LoadOBJ(strings.NewReader(obj))
	if err != nil {
		t.Fatal(err)
	}
	if !sameGeometry(m, plain) {
		t.Errorf("wanted the geometry of LoadOBJ")
	}
	if m.Normals == nil || len(m.Vertices) <= 4 {
		t.Errorf("wanted vertices split for their normals, got %d with normals %v", len(m.Vertices), m.Normals != nil)
	}
	if want := []engine.Color{gridColors[1][0], gridColors[0][3], none}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceColors)
	}
}

func TestLoadOBJAndLimitIgnoreTextures(t *testing.T) {
	obj := "mtllib m.mtl\n" + triOBJVerts + "vt 0.125 0.25\nusemtl t\nf 1/1 2/1 3/1\n"
	for name, load := range map[string]func(string) (Mesh, error){
		"LoadOBJ":      func(s string) (Mesh, error) { return LoadOBJ(strings.NewReader(s)) },
		"LoadOBJLimit": func(s string) (Mesh, error) { return LoadOBJLimit(strings.NewReader(s), 1<<20) },
	} {
		m, err := load(obj)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if m.FaceColors != nil || len(m.Faces) != 1 {
			t.Errorf("%s: wanted one face with no color, got %d with %v", name, len(m.Faces), m.FaceColors)
		}
	}
}

func TestTexturesAreOpenedWhenAFaceUsesThemAndOnlyOnce(t *testing.T) {
	mtl := "newmtl a\nmap_Kd t.png\nnewmtl b\nmap_Kd t.png\nnewmtl c\nmap_Kd unused.png\nnewmtl d\nmap_Kd late.png\n"
	obj := "mtllib m.mtl\n" + triOBJVerts + "vt 0.5 0.5\n" +
		"usemtl a\nf 1/1 2/1 3/1\nusemtl b\nf 1/1 2/1 3/1\nusemtl a\nf 1/1 2/1 3/1\nusemtl c\nusemtl d\n"
	images := map[string][]byte{"t.png": encodePNG(t, gridImage()), "unused.png": []byte("x"), "late.png": []byte("x")}
	rec := &recordingFS{FS: texturedFS(obj, mtl, images)}

	if _, err := LoadOBJFS(rec, "m.obj", 1<<20); err != nil {
		t.Fatal(err)
	}
	if want := []string{"m.obj", "m.mtl", "t.png"}; !slices.Equal(rec.opened, want) {
		t.Errorf("wanted %v opened, in that order, got %v", want, rec.opened)
	}
}

func TestTexturesAreFoundBesideTheOBJNotBesideTheMTL(t *testing.T) {
	files := fstest.MapFS{
		"models/m.obj":     {Data: []byte("mtllib sub/m.mtl\n" + triOBJVerts + "vt 0.125 0.25\nusemtl t\nf 1/1 2/1 3/1\n")},
		"models/sub/m.mtl": {Data: []byte("newmtl t\nmap_Kd t.png\n")},
		"models/t.png":     {Data: encodePNG(t, gridImage())},
		"models/sub/t.png": {Data: encodePNG(t, solid(1, 1, color.NRGBA{R: 1, G: 2, B: 3, A: 255}))},
	}
	m, err := LoadOBJFS(files, "models/m.obj", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if want := []engine.Color{gridColors[1][0]}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted the picture beside the OBJ, %v, got %v", want, m.FaceColors)
	}
}

func TestATexturedMaterialWithAHugeImageStillLoads(t *testing.T) {
	obj := "mtllib m.mtl\n" + triOBJVerts + "vt 0.5 0.5\nusemtl t\nf 1/1 2/1 3/1\n"
	m := loadTextured(t, obj, "newmtl t\nmap_Kd huge.png\nKd 1 0 0\n", map[string][]byte{"huge.png": pngHeader(100_000, 100_000)})
	if want := []engine.Color{pureRed}; !slices.Equal(m.FaceColors, want) {
		t.Errorf("wanted the Kd, %v, got %v", want, m.FaceColors)
	}
}

// The model a trial of the oracle builds: its text, the files and the colors every
// triangle must come out with.
type texturedTrial struct {
	obj, mtl string
	want     []engine.Color
	sampled  int // how many of them came from the texture
}

// texturedTrialFrom builds a random model whose expected colors are worked out here, in a
// way that shares nothing with the loader: the texel is found by walking the boundaries
// between texels.
func texturedTrialFrom(rng *rand.Rand, w, h int) texturedTrial {
	// Material 0 is unknown, 1 has only a Kd, 2 only a texture, 3 both, 4 a texture that
	// is missing and a Kd, 5 a missing texture alone.
	mtl := "newmtl m1\nKd 0.2 0.4 0.6\nnewmtl m2\nmap_Kd t.png\nnewmtl m3\nKd 1 0 0\nmap_Kd t.png\n" +
		"newmtl m4\nmap_Kd gone.png\nKd 0 1 0\nnewmtl m5\nmap_Kd gone.png\n"
	kd := [6]engine.Color{none, engine.RGB(51, 102, 153), none, pureRed, pureGreen, none}
	textured := [6]bool{false, false, true, true, false, false}

	var sb strings.Builder
	sb.WriteString("mtllib m.mtl\n")
	vertices := 3 + rng.IntN(5)
	for i := range vertices {
		fmt.Fprintf(&sb, "v %d 0 0\n", i)
	}
	uvs := make([][2]float64, rng.IntN(10))
	for i := range uvs {
		uvs[i] = [2]float64{rng.Float64()*4 - 1.5, rng.Float64()*4 - 1.5}
		fmt.Fprintf(&sb, "vt %v %v\n", uvs[i][0], uvs[i][1])
	}

	var want []engine.Color
	sampled := 0
	material := 0
	for range 1 + rng.IntN(10) {
		if rng.IntN(3) == 0 {
			material = rng.IntN(6)
			if material == 0 {
				sb.WriteString("usemtl nosuch\n")
			} else {
				fmt.Fprintf(&sb, "usemtl m%d\n", material)
			}
		}

		corners := 3 + rng.IntN(4)
		uv := make([]*[2]float64, corners) // the coordinates each corner ends up with
		sb.WriteString("f")
		for c := range corners {
			vertex := 1 + rng.IntN(vertices)
			switch pick := rng.IntN(10); {
			case pick == 0 || len(uvs) == 0: // no coordinate
				fmt.Fprintf(&sb, " %d", vertex)
			case pick == 1: // one that is not there
				fmt.Fprintf(&sb, " %d/%d", vertex, len(uvs)+1+rng.IntN(3))
			case pick == 2: // counted back from the last
				k := 1 + rng.IntN(len(uvs))
				fmt.Fprintf(&sb, " %d/-%d", vertex, k)
				uv[c] = &uvs[len(uvs)-k]
			default:
				k := 1 + rng.IntN(len(uvs))
				fmt.Fprintf(&sb, " %d/%d", vertex, k)
				uv[c] = &uvs[k-1]
			}
		}
		sb.WriteString("\n")

		for i := 1; i+1 < corners; i++ {
			c := kd[material]
			if textured[material] && uv[0] != nil && uv[i] != nil && uv[i+1] != nil {
				u := (uv[0][0] + uv[i][0] + uv[i+1][0]) / 3
				v := (uv[0][1] + uv[i][1] + uv[i+1][1]) / 3
				c = engine.RGB(uint8(20+30*texelIndex(u, w)), uint8(20+30*(h-1-texelIndex(v, h))), 7)
				sampled++
			}
			want = append(want, c)
		}
	}
	return texturedTrial{obj: sb.String(), mtl: mtl, want: want, sampled: sampled}
}

// texelIndex says which of n equal slices of the repeating range [0, 1) a coordinate
// is in, counting up from 0, by walking the boundaries.
func texelIndex(c float64, n int) int {
	f := math.Mod(c, 1)
	if f < 0 {
		f++
	}
	i := 0
	for i+1 < n && f >= float64(i+1)/float64(n) {
		i++
	}
	return i
}

func TestTexturedModelsMatchAnIndependentSampler(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 41))
	var textured, mixed, sampled int
	for trial := range 1000 {
		w, h := 1+rng.IntN(6), 1+rng.IntN(6)
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				// Every texel is a different color, so a lookup one texel off shows.
				img.SetNRGBA(x, y, color.NRGBA{R: uint8(20 + 30*x), G: uint8(20 + 30*y), B: 7, A: 255})
			}
		}
		tt := texturedTrialFrom(rng, w, h)
		sampled += tt.sampled
		files := map[string][]byte{"t.png": encodePNG(t, img)}

		m := loadTextured(t, tt.obj, tt.mtl, files)
		if len(m.Faces) != len(tt.want) {
			t.Fatalf("trial %d: wanted %d triangles, got %d\n%s", trial, len(tt.want), len(m.Faces), tt.obj)
		}
		want := tt.want
		if !slices.ContainsFunc(want, func(c engine.Color) bool { return !c.IsDefault() }) {
			want = nil // a model with no colors has no list
		} else {
			textured++
			if slices.Contains(want, none) {
				mixed++
			}
		}
		if !slices.Equal(m.FaceColors, want) {
			t.Fatalf("trial %d: wanted colors %v, got %v\n%s", trial, want, m.FaceColors, tt.obj)
		}

		plain, err := LoadOBJ(strings.NewReader(tt.obj))
		if err != nil || !sameGeometry(m, plain) {
			t.Fatalf("trial %d: wanted the geometry of LoadOBJ (%v)", trial, err)
		}
	}
	// Guard: the oracle reaches models with colors, colors beside faces without, and
	// plenty of triangles colored by the texture itself.
	if textured < 500 || mixed < 250 || sampled < 1000 {
		t.Errorf("wanted at least 500 colored, 250 mixed and 1000 sampled triangles in 1000 models, got %d, %d and %d", textured, mixed, sampled)
	}
}

func FuzzLoadOBJFSTexture(f *testing.F) {
	tiny := encodePNG(f, gridImage())
	small := encodeJPEG(f, solid(16, 16, color.NRGBA{R: 200, G: 30, B: 90, A: 255}))
	obj := "usemtl m\n" + triOBJVerts + "vt 0.125 0.25\nvt 0.9 0.8\nf 1/1 2/2 3/1\nf 1/-1 2/-2 3\nusemtl n\nf 1/2 2/2 3/2\n"
	for _, seed := range []struct {
		obj, mtl string
		image    []byte
	}{
		{obj, "newmtl m\nmap_Kd t.png\nnewmtl n\nmap_Kd t.jpg\nKd 1 0 0\n", tiny},
		{obj, "newmtl m\nmap_Kd t.png\nnewmtl n\nmap_Kd t.jpg\nKd 1 0 0\n", small},
		{obj, "newmtl m\nmap_Kd t.png\nnewmtl n\nmap_Kd t.jpg\nKd 1 0 0\n", tiny[:len(tiny)/2]},
		{obj, "newmtl m\nmap_Kd t.png\nnewmtl n\nmap_Kd -s 1 1 t.jpg\n", pngHeader(100_000, 100_000)},
		{obj, "newmtl m\nmap_Kd ../t.png\nnewmtl n\nmap_Kd t.png\nmap_Kd\n", []byte("\xff\xd8\xff")},
		{objCubeWithNormals, "newmtl m\nmap_Kd t.png\n", tiny},
		{"usemtl m\nv 0 0 0\nv 1 0 0\nv 0 1 0\nvt nan inf\nvt 1e999 0\nvt\nf 1/1 2/2 3/3\n", "newmtl m\nmap_Kd t.png\n", tiny},
	} {
		f.Add(seed.obj, seed.mtl, seed.image)
	}

	f.Fuzz(func(t *testing.T, obj, mtl string, data []byte) {
		// Naming the library first makes the fuzzer's job about its text, not its name.
		text := "mtllib m.mtl\n" + obj
		files := texturedFS(text, mtl, map[string][]byte{"t.png": data, "t.jpg": data})

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
		checkFaceColors(t, m)

		again, err := LoadOBJFS(files, "m.obj", 1<<20)
		if err != nil || !slices.Equal(m.FaceColors, again.FaceColors) {
			t.Fatalf("wanted the same result twice, got %v and %v", err, slices.Equal(m.FaceColors, again.FaceColors))
		}
	})
}

func TestParseOBJFaceDoesNotAllocate(t *testing.T) {
	// "v//vn" is the common form, and a failed Atoi on its empty field would allocate an
	// error for every corner of a large model.
	args := []string{"1//1", "2//1", "3//1", "1", "2/3", "3/4/5", "2//"} // all numbers or nothing: garbage makes strconv allocate
	for _, haveUVs := range []int{0, 6} {
		buf := make([]objCorner, 0, len(args))
		allocs := testing.AllocsPerRun(100, func() {
			parseOBJFace(buf[:0], args, 3, 6, haveUVs)
		})
		if allocs != 0 {
			t.Errorf("with %d texture coordinates read: wanted no allocation, got %v", haveUVs, allocs)
		}
	}
}

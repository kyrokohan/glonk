package mesh

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"runtime"
	"sync"
	"testing"
	"testing/fstest"

	"glonk.dev/engine"
)

func encodePNG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		t.Fatalf("unable to encode a PNG: %v", err)
	}
	return buf.Bytes()
}

func encodeJPEG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatalf("unable to encode a JPEG: %v", err)
	}
	return buf.Bytes()
}

// imageFS makes a file system out of file names and their bytes.
func imageFS(files map[string][]byte) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, data := range files {
		fsys[name] = &fstest.MapFile{Data: data}
	}
	return fsys
}

// libraryOf returns a library over files with a generous limit, with the OBJ in the root.
func libraryOf(files map[string][]byte) *materialLibrary {
	return newMaterialLibrary(imageFS(files), ".", 1<<30)
}

// solid is a w by h picture of one color.
func solid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// gridColors are the eight texels of gridImage, a 4 by 2 picture whose texels are all
// different. Row 0 is the top.
var gridColors = [2][4]engine.Color{
	{engine.RGB(10, 20, 30), engine.RGB(40, 50, 60), engine.RGB(70, 80, 90), engine.RGB(100, 110, 120)},
	{engine.RGB(130, 140, 150), engine.RGB(160, 170, 180), engine.RGB(190, 200, 210), engine.RGB(220, 230, 240)},
}

func gridImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := range 2 {
		for x := range 4 {
			n := uint8(10 + 30*(x+4*y))
			img.SetNRGBA(x, y, color.NRGBA{R: n, G: n + 10, B: n + 20, A: 255})
		}
	}
	return img
}

func TestGridImageMatchesItsColors(t *testing.T) {
	img := gridImage()
	for y := range 2 {
		for x := range 4 {
			c := img.NRGBAAt(x, y)
			if got := engine.RGB(c.R, c.G, c.B); got != gridColors[y][x] {
				t.Fatalf("texel (%d, %d): wanted %v, got %v", x, y, gridColors[y][x], got)
			}
		}
	}
}

func TestTextureAtMapsCoordinatesToTexels(t *testing.T) {
	lib := libraryOf(map[string][]byte{"g.png": encodePNG(t, gridImage())})
	tex := lib.texture("g.png")
	if tex == nil || tex.w != 4 || tex.h != 2 {
		t.Fatalf("wanted the 4 by 2 picture, got %+v", tex)
	}

	nan, inf := math.NaN(), math.Inf(1)
	tests := []struct {
		name string
		u, v float64
		want engine.Color
	}{
		{"top left centre", 0.125, 0.75, gridColors[0][0]},
		{"top second", 0.375, 0.75, gridColors[0][1]},
		{"top third", 0.625, 0.75, gridColors[0][2]},
		{"top right centre", 0.875, 0.75, gridColors[0][3]},
		{"bottom left centre", 0.125, 0.25, gridColors[1][0]},
		{"bottom second", 0.375, 0.25, gridColors[1][1]},
		{"bottom third", 0.625, 0.25, gridColors[1][2]},
		{"bottom right centre", 0.875, 0.25, gridColors[1][3]},
		{"0, 0 is the bottom left", 0, 0, gridColors[1][0]},
		{"just under 1, 1 is the top right", 0.999999, 0.999999, gridColors[0][3]},
		{"1, 1 repeats to 0, 0", 1, 1, gridColors[1][0]},
		{"the edge between columns takes the right one", 0.5, 0.75, gridColors[0][2]},
		{"the edge between rows takes the lower one", 0.125, 0.5, gridColors[1][0]},
		{"negative u repeats", -0.25, 0.25, gridColors[1][3]},
		{"negative v repeats", 0.125, -0.25, gridColors[0][0]},
		{"above 1 repeats", 1.25, 1.75, gridColors[0][1]},
		{"far away repeats", 1e300, 1e300, gridColors[1][0]},
		{"a hair under 0 is the last column", -1e-20, 0.75, gridColors[0][3]},
		{"a hair above 0 in v is the last row", 0.125, 1e-20, gridColors[1][0]},
		{"u is NaN", nan, 0.5, none},
		{"v is NaN", 0.5, nan, none},
		{"u is infinite", inf, 0.5, none},
		{"v is minus infinity", 0.5, -inf, none},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tex.at(tt.u, tt.v); got != tt.want {
				t.Errorf("at(%v, %v): wanted %v, got %v", tt.u, tt.v, tt.want, got)
			}
		})
	}
}

// Every kind of picture the standard decoders return, with what its left texel must read
// as. Alpha is ignored and the colors are not converted.
func TestTextureReadsEveryKindOfPNG(t *testing.T) {
	rgb := engine.RGB(200, 100, 50)

	nrgba := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	nrgba.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
	nrgba.SetNRGBA(1, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 255})

	translucent := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	translucent.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	translucent.SetNRGBA(1, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 0})

	rgbaImg := image.NewRGBA(image.Rect(0, 0, 2, 1))
	rgbaImg.SetRGBA(0, 0, color.RGBA{R: 200, G: 100, B: 50, A: 255})

	paletted := image.NewPaletted(image.Rect(0, 0, 2, 1), color.Palette{color.NRGBA{R: 200, G: 100, B: 50, A: 255}, color.NRGBA{R: 9, G: 9, B: 9, A: 255}})
	paletted.SetColorIndex(1, 0, 1)

	gray := image.NewGray(image.Rect(0, 0, 2, 1))
	gray.SetGray(0, 0, color.Gray{Y: 0x40})

	nrgba64 := image.NewNRGBA64(image.Rect(0, 0, 2, 1))
	nrgba64.SetNRGBA64(0, 0, color.NRGBA64{R: 0xC8C8, G: 0x6464, B: 0x3232, A: 0xFFFF})

	rgba64 := image.NewRGBA64(image.Rect(0, 0, 2, 1))
	rgba64.SetRGBA64(0, 0, color.RGBA64{R: 0xC8FF, G: 0x64FF, B: 0x32FF, A: 0xFFFF})

	tests := []struct {
		name string
		img  image.Image
		want engine.Color
	}{
		{"NRGBA", nrgba, rgb},
		{"NRGBA with alpha", translucent, rgb},
		{"RGBA", rgbaImg, rgb},
		{"paletted", paletted, rgb},
		{"gray", gray, engine.RGB(0x40, 0x40, 0x40)},
		{"16-bit NRGBA", nrgba64, rgb},
		{"16-bit RGBA", rgba64, rgb},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lib := libraryOf(map[string][]byte{"t.png": encodePNG(t, tt.img)})
			tex := lib.texture("t.png")
			if tex == nil {
				t.Fatalf("wanted a texture")
			}
			if got := tex.at(0.25, 0.5); got != tt.want {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

// colorOf is the color the standard decoders' picture has at (x, y), read the way a
// texture does, to compare lookups with.
func colorOf(img image.Image, x, y int) engine.Color {
	c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	return engine.RGB(c.R, c.G, c.B)
}

// within reports whether two bytes are no more than tol apart.
func within(a, b uint8, tol int) bool {
	d := int(a) - int(b)
	return d >= -tol && d <= tol
}

func TestTextureReadsAJPEG(t *testing.T) {
	// Four blocks of one color each: JPEG blurs edges, but not the middle of a block.
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	quadrant := [2][2]color.NRGBA{
		{{R: 255, G: 0, B: 0, A: 255}, {R: 0, G: 255, B: 0, A: 255}},
		{{R: 0, G: 0, B: 255, A: 255}, {R: 255, G: 255, B: 0, A: 255}},
	}
	for y := range 32 {
		for x := range 32 {
			img.SetNRGBA(x, y, quadrant[y/16][x/16])
		}
	}
	data := encodeJPEG(t, img)
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	lib := libraryOf(map[string][]byte{"t.jpg": data})
	tex := lib.texture("t.jpg")
	if tex == nil || tex.w != 32 || tex.h != 32 {
		t.Fatalf("wanted the 32 by 32 picture, got %+v", tex)
	}
	// The top row of the picture is v near 1. The lookup must give what the decoder has
	// at that texel, and that must be about the color that was drawn there.
	for _, tt := range []struct {
		u, v   float64
		x, y   int
		wanted color.NRGBA
	}{
		{0.25, 0.75, 8, 8, quadrant[0][0]},
		{0.75, 0.75, 24, 8, quadrant[0][1]},
		{0.25, 0.25, 8, 24, quadrant[1][0]},
		{0.75, 0.25, 24, 24, quadrant[1][1]},
	} {
		got := tex.at(tt.u, tt.v)
		if want := colorOf(decoded, tt.x, tt.y); got != want {
			t.Errorf("at(%v, %v): wanted what the decoder has at (%d, %d), %v, got %v", tt.u, tt.v, tt.x, tt.y, want, got)
		}
		c := color.NRGBAModel.Convert(decoded.At(tt.x, tt.y)).(color.NRGBA)
		if !within(c.R, tt.wanted.R, 8) || !within(c.G, tt.wanted.G, 8) || !within(c.B, tt.wanted.B, 8) {
			t.Errorf("(%d, %d): wanted about %v in the picture, got %v", tt.x, tt.y, tt.wanted, c)
		}
	}
}

func TestTextureIsFoundByContentNotByName(t *testing.T) {
	data := encodePNG(t, solid(2, 2, color.NRGBA{R: 5, G: 6, B: 7, A: 255}))
	jpg := encodeJPEG(t, solid(8, 8, color.NRGBA{R: 5, G: 6, B: 7, A: 255}))
	decoded, err := jpeg.Decode(bytes.NewReader(jpg))
	if err != nil {
		t.Fatal(err)
	}
	lib := libraryOf(map[string][]byte{"png-called.jpg": data, "jpeg-called.png": jpg, "noname": data})

	for name, want := range map[string]engine.Color{
		"png-called.jpg":  engine.RGB(5, 6, 7),
		"jpeg-called.png": colorOf(decoded, 4, 4),
		"noname":          engine.RGB(5, 6, 7),
	} {
		tex := lib.texture(name)
		if tex == nil {
			t.Errorf("%s: wanted a texture", name)
			continue
		}
		if got := tex.at(0.5, 0.5); got != want {
			t.Errorf("%s: wanted %v, got %v", name, want, got)
		}
	}
}

func TestTextureRefusesWhatIsNotAWholePicture(t *testing.T) {
	good := encodePNG(t, gridImage())
	goodJPEG := encodeJPEG(t, solid(64, 64, color.NRGBA{R: 5, G: 6, B: 7, A: 255}))

	corrupt := bytes.Clone(good)
	corrupt[len(corrupt)/2] ^= 0xFF // the data's checksum no longer matches

	tests := map[string][]byte{
		"empty":               {},
		"text":                []byte("not a picture"),
		"GIF":                 []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"BMP":                 []byte("BM\x1a\x00\x00\x00\x00\x00\x00\x00\x1a\x00\x00\x00"),
		"PNG signature alone": []byte("\x89PNG\r\n\x1a\n"),
		"JPEG marker alone":   {0xFF, 0xD8},
		"PNG cut short":       good[:len(good)-20],
		"PNG cut after IHDR":  good[:33],
		"PNG with a bad CRC":  corrupt,
		"JPEG cut short":      goodJPEG[:len(goodJPEG)/2],
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			lib := libraryOf(map[string][]byte{"t": data})
			if tex := lib.texture("t"); tex != nil {
				t.Errorf("wanted no texture, got %+v", tex)
			}
		})
	}
}

// pngHeader is the start of a PNG that says it is w by h pixels of 8-bit RGBA and has
// nothing else: all the decoder needs to say how big the picture is.
func pngHeader(w, h uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6 // bit depth, color type RGBA

	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	binary.Write(&buf, binary.BigEndian, uint32(len(ihdr)))
	body := append([]byte("IHDR"), ihdr...)
	buf.Write(body)
	binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(body))
	return buf.Bytes()
}

func TestTextureSizeIsCheckedBeforeDecoding(t *testing.T) {
	lib := libraryOf(map[string][]byte{
		"huge.png":  pngHeader(100_000, 100_000), // 40 GB as pixels
		"wide.png":  pngHeader(1<<31-1, 1),
		"zero.png":  pngHeader(0, 5),
		"tall.png":  pngHeader(1, 1<<24+1),
		"limit.png": pngHeader(1<<12, 1<<12+1),
	})

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for _, name := range []string{"huge.png", "wide.png", "zero.png", "tall.png", "limit.png"} {
		if tex := lib.texture(name); tex != nil {
			t.Errorf("%s: wanted no texture", name)
		}
	}
	runtime.ReadMemStats(&after)
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 1<<20 {
		t.Errorf("wanted the sizes refused without allocating the pictures, %d bytes were allocated", grew)
	}
	if lib.pixels != 0 {
		t.Errorf("wanted nothing spent from the budget by pictures that were refused, got %d", lib.pixels)
	}
}

var bigPNGs sync.Map // [2]int to []byte: encoding a picture of 16 million pixels is slow under -race

// bigPNG is a paletted picture of one color, which is tiny as a file and big as pixels.
func bigPNG(t testing.TB, w, h int) []byte {
	t.Helper()
	if data, ok := bigPNGs.Load([2]int{w, h}); ok {
		return data.([]byte)
	}
	pal := color.Palette{color.NRGBA{R: 1, G: 2, B: 3, A: 255}, color.NRGBA{A: 255}}
	data := encodePNG(t, image.NewPaletted(image.Rect(0, 0, w, h), pal))
	bigPNGs.Store([2]int{w, h}, data)
	return data
}

func TestTexturePixelBudgetIsPerOBJAndExact(t *testing.T) {
	one := encodePNG(t, solid(1, 1, color.NRGBA{A: 255}))

	t.Run("one picture of exactly the budget", func(t *testing.T) {
		lib := libraryOf(map[string][]byte{"a.png": bigPNG(t, 4096, 4096)})
		if tex := lib.texture("a.png"); tex == nil || tex.w != 4096 || tex.h != 4096 {
			t.Errorf("wanted a 4096 by 4096 texture, got %+v", tex)
		}
	})

	t.Run("one pixel over", func(t *testing.T) {
		lib := libraryOf(map[string][]byte{"a.png": bigPNG(t, 4097, 4096)})
		if tex := lib.texture("a.png"); tex != nil {
			t.Errorf("wanted no texture, got %+v", tex)
		}
	})

	t.Run("pictures add up", func(t *testing.T) {
		half := bigPNG(t, 4096, 2048)
		lib := libraryOf(map[string][]byte{"a.png": half, "b.png": half, "c.png": one})
		if lib.texture("a.png") == nil || lib.texture("b.png") == nil {
			t.Fatalf("wanted two halves of the budget to fit")
		}
		if tex := lib.texture("c.png"); tex != nil {
			t.Errorf("wanted nothing left for a third, got %+v", tex)
		}
	})

	t.Run("a picture that fails still spends its pixels", func(t *testing.T) {
		half := bigPNG(t, 4096, 2048)
		lib := libraryOf(map[string][]byte{"cut.png": half[:len(half)/2], "b.png": half, "c.png": one})
		if tex := lib.texture("cut.png"); tex != nil {
			t.Fatalf("wanted the cut picture refused, got %+v", tex)
		}
		if lib.texture("b.png") == nil {
			t.Fatalf("wanted the second half of the budget to fit")
		}
		if tex := lib.texture("c.png"); tex != nil {
			t.Errorf("wanted the cut picture to have spent its half, got %+v", tex)
		}
	})
}

func TestTextureTriesAtMostEightNames(t *testing.T) {
	one := encodePNG(t, solid(1, 1, color.NRGBA{A: 255}))
	files := map[string][]byte{}
	for i := range 10 {
		files[fmt.Sprintf("t%d.png", i)] = one
	}
	lib := libraryOf(files)

	for i := range 8 {
		if lib.texture(fmt.Sprintf("t%d.png", i)) == nil {
			t.Fatalf("wanted name %d to be tried", i)
		}
	}
	if lib.texture("t8.png") != nil || lib.texture("t9.png") != nil {
		t.Errorf("wanted names past the eighth to be refused")
	}
	if lib.texture("t3.png") == nil {
		t.Errorf("wanted a name already read to stay available")
	}

	// A name that fails is a name tried.
	lib = libraryOf(files)
	for i := range 8 {
		lib.texture(fmt.Sprintf("missing%d.png", i))
	}
	if lib.texture("t0.png") != nil {
		t.Errorf("wanted eight missing names to use up the tries")
	}
	if len(lib.textures) != 8 {
		t.Errorf("wanted at most 8 names remembered, got %d", len(lib.textures))
	}
}

func TestTextureEachNameIsOpenedOnce(t *testing.T) {
	rec := &recordingFS{FS: imageFS(map[string][]byte{"models/a.png": encodePNG(t, solid(1, 1, color.NRGBA{A: 255})), "models/bad.png": []byte("x")})}
	lib := newMaterialLibrary(rec, "models", 1<<20)

	for range 3 {
		lib.texture("a.png")
		lib.texture("bad.png")
		lib.texture("missing.png")
	}
	if want := []string{"models/a.png", "models/bad.png", "models/missing.png"}; !equalStrings(rec.opened, want) {
		t.Errorf("wanted %v opened once each beside the OBJ, got %v", want, rec.opened)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTextureNamesAreHeldToTheSameRulesAsMTLNames(t *testing.T) {
	data := encodePNG(t, solid(1, 1, color.NRGBA{A: 255}))
	files := imageFS(map[string][]byte{"a.png": data, "sub/a.png": data})

	rec := &recordingFS{FS: files}
	lib := newMaterialLibrary(rec, ".", 1<<20)
	for _, name := range []string{"", "../a.png", "/a.png", `x\a.png`, "sub/../../a.png"} {
		if tex := lib.texture(name); tex != nil {
			t.Errorf("%q: wanted no texture", name)
		}
	}
	if len(rec.opened) != 0 {
		t.Errorf("wanted nothing opened for names outside the folder, got %v", rec.opened)
	}

	lib = newMaterialLibrary(files, ".", 1<<20)
	for _, name := range []string{"./a.png", "sub/a.png", "sub/../a.png"} {
		if lib.texture(name) == nil {
			t.Errorf("%q: wanted a texture", name)
		}
	}
}

func TestTextureFileSizeLimit(t *testing.T) {
	data := encodePNG(t, gridImage())
	fsys := imageFS(map[string][]byte{"t.png": data})

	if newMaterialLibrary(fsys, ".", int64(len(data))).texture("t.png") == nil {
		t.Errorf("wanted a file of exactly the limit to be read")
	}
	if tex := newMaterialLibrary(fsys, ".", int64(len(data))-1).texture("t.png"); tex != nil {
		t.Errorf("wanted a file one byte over the limit to be refused, got %+v", tex)
	}
	if got := newMaterialLibrary(fsys, ".", 1<<40).textureLimit; got != maxTextureBytes {
		t.Errorf("wanted the limit of an image to be at most %d, got %d", maxTextureBytes, got)
	}
}

// What map_Kd does to the material it is in, and what it leaves alone.
func TestMTLMapKdFillsTheMaterialsTexture(t *testing.T) {
	tests := []struct {
		name string
		mtl  string
		want material
	}{
		{"only a texture", "newmtl a\nmap_Kd hull.jpg\n", material{texture: "hull.jpg"}},
		{"Kd then texture", "newmtl a\nKd 1 0 0\nmap_Kd t.png\n", material{color: pureRed, texture: "t.png"}},
		{"texture then Kd", "newmtl a\nmap_Kd t.png\nKd 1 0 0\n", material{color: pureRed, texture: "t.png"}},
		{"the later texture wins", "newmtl a\nmap_Kd t.png\nmap_Kd u.png\n", material{texture: "u.png"}},
		{"options are not read", "newmtl a\nmap_Kd -s 1 1 1 t.png\n", material{}},
		{"options leave Kd", "newmtl a\nKd 0 0 1\nmap_Kd -o 0 0 t.png\n", material{color: pureBlue}},
		{"a name with spaces", "newmtl a\nmap_Kd my picture.png\n", material{texture: "my picture.png"}},
		{"tab after the keyword", "newmtl a\nmap_Kd\tt.png\n", material{texture: "t.png"}},
		{"a comment after it", "newmtl a\nmap_Kd t.png # the hull\n", material{texture: "t.png"}},
		{"no name", "newmtl a\nmap_Kd\n", material{}},
		{"before any material", "map_Kd t.png\nnewmtl a\n", material{}},
		{"keywords are case sensitive", "newmtl a\nmap_kd t.png\n", material{}},
		{"a redefinition starts over", "newmtl a\nmap_Kd t.png\nnewmtl a\n", material{}},
		{"another material's texture stays with it", "newmtl a\nmap_Kd t.png\nnewmtl b\nKd 1 0 0\n", material{texture: "t.png"}},
		{"a commented out texture", "newmtl a\n# map_Kd t.png\n", material{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lib := libraryOf(nil)
			lib.parse(tt.mtl)
			if got := lib.materials["a"]; got != tt.want {
				t.Errorf("wanted %+v, got %+v", tt.want, got)
			}
		})
	}
}

func TestMTLMapKdOfARefusedMaterialCreatesNothing(t *testing.T) {
	// A newmtl with no name is not kept, and what follows it belongs to nothing.
	lib := libraryOf(nil)
	lib.parse("newmtl a\nmap_Kd t.png\nnewmtl\nmap_Kd u.png\nKd 1 0 0\n")
	if want := (map[string]material{"a": {texture: "t.png"}}); len(lib.materials) != 1 || lib.materials["a"] != want["a"] {
		t.Errorf("wanted only a, with its texture, got %+v", lib.materials)
	}
}

package mesh

import (
	"bytes"
	"image/color"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// checkerboard is an n by n picture of black and white pixels, one pixel each.
func checkerboard(n int) *bitmap.Bitmap {
	pic := bitmap.New(n, n)
	for y := range n {
		for x := range n {
			c := engine.RGB(255, 255, 255)
			if (x+y)%2 == 1 {
				c = engine.RGB(0, 0, 0)
			}
			pic.Set(x, y, c)
		}
	}
	return pic
}

// The level is the floor of log2 of how many of the picture's pixels a step across or down passes
// over, the longer of the two, with a step down a cell in cells and half a cell in pixels.
func TestMipLevelIsTheLongerStepInCellsAndInPixels(t *testing.T) {
	// A triangle square to the camera, 10 cells on a side, whose picture of 10 by 10 runs perX of
	// its pixels across each cell and perY down each cell.
	tri := func(perX, perY float64) *shadedTriangle {
		t := &shadedTriangle{texW: 10, texH: 10}
		t.corners = [3]screenPoint{{0, 0, 1}, {10, 0, 1}, {0, 10, 1}}
		t.uv = [3]geometry.Vector2{{}, {X: perX}, {Y: perY}} // 10 cells times perX/10 of the picture
		t.mipGradients()
		return t
	}
	for _, tt := range []struct {
		name             string
		perX, perY       float64
		cells, pixelsLvl int
	}{
		{"one to one", 1, 1, 0, 0},
		{"two across", 2, 1, 1, 1},
		{"eight down", 4, 8, 3, 2}, // down is half as far in pixels
		{"sixteen across", 16, 1, 4, 4},
		{"three down", 1, 3, 1, 0}, // 3 down a cell, 1.5 down a pixel
		{"smaller than the screen", 0.25, 0.25, 0, 0},
	} {
		tr := tri(tt.perX, tt.perY)
		for _, b := range [][3]float64{{1, 0, 0}, {0.2, 0.3, 0.5}, {0, 0, 1}} {
			if got := tr.mipLevel(b[0], b[1], b[2], geometry.Vector2{}, 1); got != tt.cells {
				t.Errorf("%s in cells: wanted level %d, got %d", tt.name, tt.cells, got)
			}
			if got := tr.mipLevel(b[0], b[1], b[2], geometry.Vector2{}, 0.5); got != tt.pixelsLvl {
				t.Errorf("%s in pixels: wanted level %d, got %d", tt.name, tt.pixelsLvl, got)
			}
		}
	}
}

// pictureWall is a wall square to a camera at the origin, 2 by 2, dist away, with pic on it once.
func pictureWall(pic *bitmap.Bitmap, dist float64, sharp bool) Mesh {
	return Mesh{
		Vertices: []geometry.Vector3{{X: -1, Y: -1, Z: dist}, {X: -1, Y: 1, Z: dist}, {X: 1, Y: 1, Z: dist}, {X: 1, Y: -1, Z: dist}},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		FaceTexCoords: [][3]geometry.Vector2{
			{{X: 0, Y: 1}, {X: 0, Y: 0}, {X: 1, Y: 0}},
			{{X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}},
		},
		Material: Material{Texture: pic, Sharp: sharp},
	}
}

// wallPixels draws m in pixels on a canvas of 80 by 24 cells under full light.
func wallPixels(t *testing.T, m Mesh) *engine.Canvas {
	t.Helper()
	fb, err := engine.NewFrameBuffer(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	cv := fb.Canvas()
	m.DrawShadedPixels(cv, Camera{}, Ambient{Level: 1})
	return cv
}

// A wall far away, drawn much smaller than its picture of black and white pixels, shows their mean,
// gray, where Sharp shows the black or white pixel that happens to fall there; near, drawn bigger
// than the picture, the two are the same.
func TestAFarPictureReadsASmallerCopy(t *testing.T) {
	pic := checkerboard(64)
	far, farSharp := wallPixels(t, pictureWall(pic, 30, false)), wallPixels(t, pictureWall(pic, 30, true))
	r, _, _ := far.At(40, 24).RGB()
	if r < 64 || r > 192 {
		t.Errorf("wanted the middle of a far wall gray, got %v", far.At(40, 24))
	}
	if c := farSharp.At(40, 24); c != engine.RGB(0, 0, 0) && c != engine.RGB(255, 255, 255) {
		t.Errorf("wanted a Sharp wall to show a pixel of the picture, got %v", c)
	}

	small := checkerboard(8)
	near, nearSharp := wallPixels(t, pictureWall(small, 1.2, false)), wallPixels(t, pictureWall(small, 1.2, true))
	for y := range near.Height() {
		for x := range near.Width() {
			if near.At(x, y) != nearSharp.At(x, y) {
				t.Fatalf("wanted a wall drawn bigger than its picture to read the picture itself, pixel (%d, %d) differs", x, y)
			}
		}
	}

	// A tinted view's copies are in its look.
	white := wallPixels(t, pictureWall(pic.Tinted(engine.RGB(255, 255, 255), 1), 30, false))
	if c := white.At(40, 24); c != engine.RGB(255, 255, 255) {
		t.Errorf("wanted a far wall with a view tinted white to be white, got %v", c)
	}
}

// receding is a floor of two triangles going away from a camera above it, with pic on it 32 times
// over.
func receding(pic *bitmap.Bitmap, sharp bool) (Mesh, Camera) {
	floor := Plane(60, 60)
	for i, t := range floor.FaceTexCoords {
		floor.FaceTexCoords[i] = [3]geometry.Vector2{t[0].Scale(32), t[1].Scale(32), t[2].Scale(32)}
	}
	floor.Material = Material{Texture: pic, Sharp: sharp}
	floor.Transform.Position = geometry.Vector3{Y: -1, Z: 30}
	return floor, Camera{Near: 0.1}
}

// One floor reads the picture itself near the camera and smaller copies far from it: its near rows
// are black and white and its far rows gray, in one face; with Sharp, the far rows are still black
// and white.
func TestARecedingFloorReadsCopiesFromNearToFar(t *testing.T) {
	grays := func(cv *engine.Canvas, y0, y1 int) (gray, drawn int) {
		for y := y0; y < y1; y++ {
			for x := range cv.Width() {
				c := cv.At(x, y)
				if c.IsDefault() {
					continue
				}
				drawn++
				if c != engine.RGB(0, 0, 0) && c != engine.RGB(255, 255, 255) {
					gray++
				}
			}
		}
		return gray, drawn
	}
	for _, sharp := range []bool{false, true} {
		floor, cam := receding(checkerboard(16), sharp)
		fb, err := engine.NewFrameBuffer(80, 24)
		if err != nil {
			t.Fatal(err)
		}
		cv := fb.Canvas()
		floor.DrawShadedPixels(cv, cam, Ambient{Level: 1})
		nearGray, nearDrawn := grays(cv, 44, 48)
		farGray, farDrawn := grays(cv, 25, 28)
		if nearDrawn == 0 || farDrawn == 0 {
			t.Fatalf("sharp %v: wanted the floor near and far, got %d and %d pixels", sharp, nearDrawn, farDrawn)
		}
		if nearGray != 0 {
			t.Errorf("sharp %v: wanted the near rows black and white, got %d gray of %d", sharp, nearGray, nearDrawn)
		}
		if sharp && farGray != 0 {
			t.Errorf("sharp: wanted the far rows black and white too, got %d gray of %d", farGray, farDrawn)
		}
		if !sharp && farGray*2 < farDrawn {
			t.Errorf("wanted most of the far rows gray, got %d of %d", farGray, farDrawn)
		}
	}
}

// noisy is an n by n picture of colors at random, from a fixed seed: every smaller copy of it is
// different, so a pixel that reads the wrong one shows.
func noisy(n int) *bitmap.Bitmap {
	rng := rand.New(rand.NewPCG(118, 1))
	pic := bitmap.New(n, n)
	for y := range n {
		for x := range n {
			pic.Set(x, y, engine.RGB(uint8(rng.IntN(256)), uint8(rng.IntN(256)), uint8(rng.IntN(256))))
		}
	}
	return pic
}

// The receding floor as a picture, with a picture of colors at random on it, stored under
// testdata/golden and compared pixel by pixel, on every platform: a pixel that reads another copy
// than the one that fits it is another color. Run with -update to write it again.
func TestARecedingFloorMatchesTheStoredPicture(t *testing.T) {
	floor, cam := receding(noisy(16), false)
	fb, err := engine.NewFrameBuffer(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	cv := fb.Canvas()
	floor.DrawShadedPixels(cv, cam, Ambient{Level: 1})
	cv.Compose()
	img := fb.Image(engine.ImageOptions{Scale: 2})
	path := filepath.Join("testdata", "golden", "mip-floor.png")
	if *updateGolden {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("wanted the stored picture (run with -update to write it): %v", err)
	}
	defer f.Close()
	stored, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Bounds() != img.Bounds() {
		t.Fatalf("wanted a picture of %v, got %v", stored.Bounds(), img.Bounds())
	}
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			r, g, b, a := stored.At(x, y).RGBA()
			want := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
			if got := img.RGBAAt(x, y); got != want {
				t.Fatalf("pixel (%d, %d): wanted %v, got %v", x, y, want, got)
			}
		}
	}
}

// A footprint whose square, rounded square by square, is exactly 16, level 2, but just under it
// when arm64 fuses a square into the sum, level 1: found by a search, it keeps the explicit
// roundings honest, so that every platform chooses the same copy.
func TestMipLevelIsTheSameOnEveryPlatform(t *testing.T) {
	tri := &shadedTriangle{texW: 1, texH: 1}
	tri.corners = [3]screenPoint{{0, 0, 1}, {1, 0, 1}, {0, 1, 1}}
	tri.ux, tri.vx = 3.6829152288705171, 1.5608124220910164 // a step across passes over this much of the picture
	if got := tri.mipLevel(1, 0, 0, geometry.Vector2{}, 1); got != 2 {
		t.Errorf("wanted level 2, got %d", got)
	}
}

// A face that reads one copy for the whole of it does not leave it, or the copy's plain colors,
// to the next face: a far wall and then a near one in one mesh, with a view tinted white, are both
// white.
func TestTheNextFaceReadsThePictureAgain(t *testing.T) {
	white := engine.RGB(255, 255, 255)
	far, near := pictureWall(nil, 30, false), pictureWall(nil, 1.2, false)
	m := far
	m.Vertices = append(append([]geometry.Vector3{}, far.Vertices...), near.Vertices...)
	m.Faces = append(append([][3]int{}, far.Faces...), [3]int{4, 5, 6}, [3]int{4, 6, 7})
	m.FaceTexCoords = append(append([][3]geometry.Vector2{}, far.FaceTexCoords...), near.FaceTexCoords...)
	m.Material.Texture = checkerboard(8).Tinted(white, 1)
	m.DoubleSided = true
	cv := wallPixels(t, m)
	drawn := 0
	for y := range cv.Height() {
		for x := range cv.Width() {
			if c := cv.At(x, y); !c.IsDefault() {
				drawn++
				if c != white {
					t.Fatalf("pixel (%d, %d): wanted white, got %v", x, y, c)
				}
			}
		}
	}
	if drawn == 0 {
		t.Fatal("wanted the walls drawn")
	}
}

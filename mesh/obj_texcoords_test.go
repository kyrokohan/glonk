package mesh

import (
	"image"
	"image/color"
	"slices"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// fourColors is a picture of 2 by 2 pixels: red at the top left, green at the top right, blue at
// the bottom left and yellow at the bottom right.
func fourColors() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 255})
	img.SetNRGBA(0, 1, color.NRGBA{B: 255, A: 255})
	img.SetNRGBA(1, 1, color.NRGBA{R: 255, G: 255, A: 255})
	return img
}

// A model whose picture is pinned to a quad is seen the right way up, not mirrored and not
// upside down, in cells and in pixels. A file counts v up from the bottom of the picture and a
// mesh counts it down from the top, and the loader turns Z and the winding of every face over, so
// this is the end of the road that each of those is tested on alone.
func TestALoadedTexturedQuadIsDrawnTheRightWayUp(t *testing.T) {
	obj := "mtllib m.mtl\nv -1 -1 0\nv 1 -1 0\nv 1 1 0\nv -1 1 0\n" +
		"vt 0 0\nvt 1 0\nvt 1 1\nvt 0 1\nusemtl t\nf 1/1 2/2 3/3 4/4\n"
	m := loadTextured(t, obj, "newmtl t\nmap_Kd t.png\n", map[string][]byte{"t.png": encodePNG(t, fourColors())})
	m.Transform.Position = geometry.Vector3{Z: 3}
	if len(m.Faces) != 2 {
		t.Fatalf("wanted a quad of two triangles, got %d faces", len(m.Faces))
	}

	for _, pixels := range []bool{false, true} {
		t.Setenv("TERM_PROGRAM", "")
		fb := newShadeBuffer(t)
		at := func(x, y int) engine.Color { return fb.At(x, y).Foreground }
		width, height := fb.Width(), fb.Height()
		if pixels {
			canvas := fb.Canvas()
			m.DrawShadedPixels(canvas, shadeCam(), Ambient{Level: 1})
			at = func(x, y int) engine.Color { return canvas.At(x, y) }
			height = canvas.Height()
		} else {
			m.DrawShaded(fb, shadeCam(), Ambient{Level: 1})
		}

		// The box that was drawn in.
		x0, y0, x1, y1 := width, height, -1, -1
		for y := range height {
			for x := range width {
				if !at(x, y).IsDefault() {
					x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
				}
			}
		}
		if x1-x0 < 8 || y1-y0 < 4 {
			t.Fatalf("pixels %v: wanted a quad of some size, got the box (%d, %d) to (%d, %d)", pixels, x0, y0, x1, y1)
		}
		shade := func(c engine.Color) engine.Color {
			if pixels {
				return engine.Shade(c, 1)
			}
			return c
		}
		for _, q := range []struct {
			name         string
			fx, fy       float64 // a quarter of the way across and down the box, or three quarters
			want         engine.Color
			whereInImage string
		}{
			{"top left", 0.25, 0.25, engine.RGB(255, 0, 0), "red"},
			{"top right", 0.75, 0.25, engine.RGB(0, 255, 0), "green"},
			{"bottom left", 0.25, 0.75, engine.RGB(0, 0, 255), "blue"},
			{"bottom right", 0.75, 0.75, engine.RGB(255, 255, 0), "yellow"},
		} {
			x := x0 + int(q.fx*float64(x1-x0+1))
			y := y0 + int(q.fy*float64(y1-y0+1))
			if got := at(x, y); got != shade(q.want) {
				t.Errorf("pixels %v: the %s of the quad at (%d, %d): wanted %s %v, got %v", pixels, q.name, x, y, q.whereInImage, shade(q.want), got)
			}
		}
	}
}

// The coordinates a loaded model keeps are the file's, in the order of each face's own corners,
// with v turned over. A quad is cut into two triangles that share a corner, and its list is
// worked out by hand here.
func TestLoadedTextureCoordinatesAreTheFilesInTheOrderOfEachFacesCorners(t *testing.T) {
	obj := "mtllib m.mtl\nv 0 0 0\nv 1 0 0\nv 1 1 0\nv 0 1 0\n" +
		"vt 0.125 0.25\nvt 0.875 0.25\nvt 0.875 0.75\nvt 0.125 0.75\nusemtl t\nf 1/1 2/2 3/3 4/4\n"
	m := loadTextured(t, obj, "newmtl t\nmap_Kd t.png\n", map[string][]byte{"t.png": encodePNG(t, gridImage())})

	// Corners 0 to 3 of the quad, with v turned over.
	c0, c1, c2, c3 := uv(0.125, 0.75), uv(0.875, 0.75), uv(0.875, 0.25), uv(0.125, 0.25)
	want := [][3]geometry.Vector2{{c0, c2, c1}, {c0, c3, c2}} // the faces are {0, 2, 1} and {0, 3, 2}
	if !slices.Equal(m.FaceTexCoords, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceTexCoords)
	}
	if want := [][3]int{{0, 2, 1}, {0, 3, 2}}; !slices.Equal(m.Faces, want) {
		t.Errorf("wanted the faces %v, got %v", want, m.Faces)
	}
	// The picture is the image, at its own size and in its own colors, top row first.
	pic := m.Material.Texture
	if pic == nil || pic.Width() != 4 || pic.Height() != 2 {
		t.Fatalf("wanted the picture of 4 by 2, got %v", pic)
	}
	for y := range 2 {
		for x := range 4 {
			if got := pic.At(x, y); got != gridColors[y][x] {
				t.Errorf("pixel (%d, %d): wanted %v, got %v", x, y, gridColors[y][x], got)
			}
		}
	}
}

// A mesh has one picture. The first that a face has coordinates in is the mesh's, and a face
// of a material with another keeps its one color.
func TestOnlyTheFirstPictureIsKept(t *testing.T) {
	mtl := "newmtl a\nmap_Kd a.png\nnewmtl b\nmap_Kd b.png\nKd 0 0 1\n"
	obj := "mtllib m.mtl\n" + triOBJVerts + "vt 0.2 0.2\nvt 0.7 0.9\n" +
		"usemtl b\nf 1 2 3\n" + // b, but with no coordinates: it does not claim the mesh
		"usemtl a\nf 1/1 2/2 3/1\n" + // a, the first with coordinates
		"usemtl b\nf 1/1 2/2 3/1\n" + // b: one color
		"usemtl a\nf 3/2 2/2 1/1\n"
	images := map[string][]byte{"a.png": encodePNG(t, gridImage()), "b.png": encodePNG(t, solid(3, 3, color.NRGBA{R: 9, G: 8, B: 7, A: 255}))}
	m := loadTextured(t, obj, mtl, images)

	zero := [3]geometry.Vector2{}
	// Each face lists its corners as the fan does, the first, the third and the second, and v is
	// turned over: vt 1 is (0.2, 0.2) and vt 2 is (0.7, 0.9). These are variables so that 1 - v is
	// worked out as the loader does, not exactly as a constant is.
	v1, v2 := 0.2, 0.9
	p1, p2 := uv(0.2, 1-v1), uv(0.7, 1-v2)
	want := [][3]geometry.Vector2{zero, {p1, p1, p2}, zero, {p2, p1, p2}}
	if !slices.Equal(m.FaceTexCoords, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceTexCoords)
	}
	if pic := m.Material.Texture; pic == nil || pic.Width() != 4 || pic.Height() != 2 {
		t.Errorf("wanted the picture of a, 4 by 2, got %v", pic)
	}
	// The one colors are as they were: b's Kd for the face of b that has no coordinates, and the
	// picture's texel for the others, b's own picture included, which a material with both shows.
	// The middle of face 1 is at (0.367, 0.433): the second column, in the bottom row.
	wantColors := []engine.Color{engine.RGB(0, 0, 255), gridColors[1][1], engine.RGB(9, 8, 7), m.FaceColors[3]}
	if !slices.Equal(m.FaceColors, wantColors) || m.FaceColors[3].IsDefault() {
		t.Errorf("wanted the colors %v, got %v", wantColors, m.FaceColors)
	}
}

// The list starts at the first face that has coordinates, with zero triples before it, and has a
// triple for every face after it.
func TestTextureCoordinatesStartAtTheFirstFaceThatHasThem(t *testing.T) {
	obj := "mtllib m.mtl\n" + triOBJVerts + "vt 0.125 0.25\nf 1 2 3\nusemtl t\nf 1/1 2/1 3/1\nusemtl nosuch\nf 1 2 3\n"
	m := loadTextured(t, obj, "newmtl t\nmap_Kd t.png\n", map[string][]byte{"t.png": encodePNG(t, gridImage())})
	p := uv(0.125, 0.75)
	if want := [][3]geometry.Vector2{{}, {p, p, p}, {}}; !slices.Equal(m.FaceTexCoords, want) {
		t.Errorf("wanted %v, got %v", want, m.FaceTexCoords)
	}

	// No coordinates anywhere: no list and no picture, even though the material has an image.
	obj = "mtllib m.mtl\n" + triOBJVerts + "usemtl t\nf 1 2 3\n"
	m = loadTextured(t, obj, "newmtl t\nmap_Kd t.png\n", map[string][]byte{"t.png": encodePNG(t, gridImage())})
	if m.FaceTexCoords != nil || m.Material.Texture != nil {
		t.Errorf("wanted no coordinates and no picture, got %v and %v", m.FaceTexCoords, m.Material.Texture)
	}
}

// A picture that cannot be had, and a model without materials, leave the mesh without a picture.
func TestAPictureThatCannotBeHadLeavesNoTexture(t *testing.T) {
	obj := "mtllib m.mtl\n" + triOBJVerts + "vt 0.2 0.2\nusemtl t\nf 1/1 2/1 3/1\n"
	for name, images := range map[string]map[string][]byte{
		"missing": nil,
		"broken":  {"t.png": []byte("not an image")},
		"huge":    {"t.png": pngHeader(100_000, 100_000)},
	} {
		m := loadTextured(t, obj, "newmtl t\nmap_Kd t.png\nKd 1 0 0\n", images)
		if m.FaceTexCoords != nil || m.Material.Texture != nil {
			t.Errorf("%s: wanted no coordinates and no picture, got %v and %v", name, m.FaceTexCoords, m.Material.Texture)
		}
		if want := []engine.Color{pureRed}; !slices.Equal(m.FaceColors, want) {
			t.Errorf("%s: wanted the Kd %v, got %v", name, want, m.FaceColors)
		}
	}

	// The loaders that skip materials give none either.
	for name, load := range map[string]func(string) (Mesh, error){
		"LoadOBJ":      func(s string) (Mesh, error) { return LoadOBJ(strings.NewReader(s)) },
		"LoadOBJLimit": func(s string) (Mesh, error) { return LoadOBJLimit(strings.NewReader(s), 1<<20) },
	} {
		m, err := load(obj)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if m.FaceTexCoords != nil || m.Material.Texture != nil {
			t.Errorf("%s: wanted no coordinates and no picture, got %v and %v", name, m.FaceTexCoords, m.Material.Texture)
		}
	}
}

// A pixel of a model's picture that is mostly transparent is see-through in the mesh's picture,
// by the rule bitmap has (128 or more is drawn), and the color of the face shows there.
func TestTransparencyOfAModelsPictureIsKept(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 40, G: 50, B: 60, A: 127})
	img.SetNRGBA(2, 0, color.NRGBA{R: 70, G: 80, B: 90, A: 128})
	img.SetNRGBA(3, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 0})
	obj := "mtllib m.mtl\n" + triOBJVerts + "vt 0.1 0.5\nusemtl t\nf 1/1 2/1 3/1\n"
	m := loadTextured(t, obj, "newmtl t\nmap_Kd t.png\n", map[string][]byte{"t.png": encodePNG(t, img)})

	pic := m.Material.Texture
	if pic == nil {
		t.Fatal("wanted a picture")
	}
	want := []engine.Color{engine.RGB(10, 20, 30), none, engine.RGB(70, 80, 90), none}
	for x, w := range want {
		if got := pic.At(x, 0); got != w {
			t.Errorf("pixel %d: wanted %v, got %v", x, w, got)
		}
	}
}

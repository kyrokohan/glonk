package mesh

import (
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// uniquePictureOf is a picture of w by h pixels, each up to 9, with a color of its own for every
// one, so that the color that is drawn says which pixel it came from.
func uniquePictureOf(w, h int) *bitmap.Bitmap {
	b := bitmap.New(w, h)
	for y := range h {
		for x := range w {
			b.Set(x, y, engine.RGB(uint8(20+25*x), uint8(20+25*y), 128))
		}
	}
	return b
}

// uniquePicture is a picture of 8 pixels across and 5 down, which is not square, so that a width
// taken for a height shows.
func uniquePicture() *bitmap.Bitmap { return uniquePictureOf(8, 5) }

// uv is a texture coordinate.
func uv(u, v float64) geometry.Vector2 { return geometry.Vector2{X: u, Y: v} }

// texturedTriangle is a triangle seen at a slant, its corners at different depths, with a
// place in uniquePicture pinned to each corner.
func texturedTriangle(corners [3]geometry.Vector2) Mesh {
	return Mesh{
		Vertices:      []geometry.Vector3{{X: -2, Y: -1.5, Z: 5}, {X: 2.5, Y: -1, Z: 9}, {X: 0, Y: 2.2, Z: 6}},
		Faces:         [][3]int{{0, 1, 2}},
		FaceTexCoords: [][3]geometry.Vector2{corners},
		Material:      Material{Texture: uniquePicture()},
		DoubleSided:   true, // so that the winding does not matter
	}
}

// texelWalk is the pixel of an n-pixel row that a coordinate in [0, 1) falls in, found by walking
// the boundaries between pixels, so that it shares nothing with the code that divides.
func texelWalk(f float64, n int) int {
	i := 0
	for i < n-1 && f >= float64(i+1)/float64(n) {
		i++
	}
	return i
}

// nearBoundary reports whether the coordinate f, in texels, is so close to a boundary between
// two that the weights it was worked out from, which the code under test finds another way,
// could put it on either side.
func nearBoundary(f float64) bool {
	return math.Abs(f-math.Round(f)) < 1e-4
}

// checkTextured draws a one-face mesh and holds every cell (or pixel) well inside the face to the
// pixel of the picture at the place that a ray through it meets the face: the coordinates of the
// corners blended with the weights there, with perspective and whatever the near and far planes
// cut away. The mesh has its corners at its vertices in order and is seen from the origin. It
// returns how many it compared.
func checkTextured(t *testing.T, m Mesh, cam Camera, pixels bool) int {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "")
	fb := newShadeBuffer(t)
	vm := newViewMap(cam, fb.Width(), fb.Height())
	cam, _ = cam.withDefaults()
	pic := m.Material.Texture
	coords := m.FaceTexCoords[0]

	var tri [3]geometry.Vector3
	for k, i := range m.Faces[0] {
		tri[k] = m.Vertices[i].Add(m.Transform.Position)
	}

	var compared int
	check := func(px, py int, x, y float64, got engine.Color) {
		b, z := vm.hit(x, y, tri)
		inside := min(b[0], b[1], b[2])
		if z < cam.Near+0.05 || z > cam.Far-0.05 || inside < 0.03 {
			outside := inside < -0.1 || (z > cam.Far+0.05 && inside > 0.03)
			if outside && z > cam.Near && !got.IsDefault() {
				t.Errorf("(%d, %d): wanted nothing outside the face or past the far plane, got %v", px, py, got)
			}
			return
		}
		u := b[0]*coords[0].X + b[1]*coords[1].X + b[2]*coords[2].X
		v := b[0]*coords[0].Y + b[1]*coords[1].Y + b[2]*coords[2].Y
		fu, fv := u-math.Floor(u), v-math.Floor(v)
		if nearBoundary(fu*float64(pic.Width())) || nearBoundary(fv*float64(pic.Height())) {
			return
		}
		compared++
		want := pic.At(texelWalk(fu, pic.Width()), texelWalk(fv, pic.Height()))
		if pixels {
			want = engine.Shade(want, 1) // a pixel shows full light as a shade of 1
		}
		if got != want {
			t.Errorf("(%d, %d): wanted %v for the place (%.4f, %.4f) of the picture, got %v", px, py, want, u, v, got)
		}
	}

	light := Ambient{Level: 1}
	if pixels {
		canvas := fb.Canvas()
		m.DrawShadedPixels(canvas, cam, light)
		for py := range canvas.Height() {
			for px := range canvas.Width() {
				check(px, py, float64(px)+0.5, (float64(py)+0.5)/2, canvas.At(px, py))
			}
		}
		return compared
	}
	m.DrawShaded(fb, cam, light)
	for y := range fb.Height() {
		for x := range fb.Width() {
			check(x, y, float64(x)+0.5, float64(y)+0.5, fb.At(x, y).Foreground)
			// Full light is the last character of the ramp, whatever the picture is.
			if c := fb.At(x, y); !c.Foreground.IsDefault() && c.Character != '@' {
				t.Errorf("(%d, %d): wanted the brightest character, got %q", x, y, c.Character)
			}
		}
	}
	return compared
}

// A face with a picture is drawn in the pixel of it that a viewer would see at every cell: the
// places of the corners blended with perspective-correct weights, from either side, flat or
// smooth, in cells and in pixels, with the near and the far plane cutting a corner off, and with
// a picture that repeats.
func TestTexturesAreDrawnAtThePlaceEveryCellIsSeenAt(t *testing.T) {
	plain := texturedTriangle([3]geometry.Vector2{uv(0.05, 0.1), uv(0.98, 0.05), uv(0.4, 0.95)})
	smooth := plain
	smooth.Normals = []geometry.Vector3{{X: 0.3, Z: -1}, {Y: 0.4, Z: -1}, {X: -0.5, Y: -0.2, Z: -1}}
	backwards := plain
	backwards.Faces = [][3]int{{0, 2, 1}}
	repeating := texturedTriangle([3]geometry.Vector2{uv(0, 0), uv(3, 0.5), uv(1.5, 2.5)})
	negative := texturedTriangle([3]geometry.Vector2{uv(-1.2, 0.3), uv(0.7, -2.1), uv(1.1, 1.9)})
	// A corner at (0, 0) is a place in the picture like any other, as long as one of the three is not.
	zeroMiddle := texturedTriangle([3]geometry.Vector2{uv(0.4, 0.9), uv(0, 0), uv(1.3, 0.6)})
	zeroLast := texturedTriangle([3]geometry.Vector2{uv(0.4, 0.9), uv(1.3, 0.6), uv(0, 0)})
	behind := func(corner int) Mesh { // the one corner that is cut off is not always the first
		m := plain
		m.Vertices = []geometry.Vector3{{X: -2, Y: -1, Z: 5}, {X: 3, Y: -1, Z: 4}, {X: 0, Y: 3, Z: 6}}
		m.Vertices[corner].Z = -1.5
		return m
	}
	twoBehind := plain
	twoBehind.Vertices = []geometry.Vector3{{X: -2, Y: -1, Z: -1}, {X: 3, Y: -1, Z: -2}, {X: 0, Y: 3, Z: 7}}
	pastFar := plain // a corner beyond the far plane, and one behind the camera
	pastFar.Vertices = []geometry.Vector3{{X: -2, Y: -1, Z: -1}, {X: 3, Y: -1, Z: 12}, {X: 0, Y: 3, Z: 6}}
	farOnly := plain
	farOnly.Vertices = []geometry.Vector3{{X: -2, Y: -1, Z: 4}, {X: 3, Y: -1, Z: 12}, {X: 0, Y: 3, Z: 6}}

	withFar := blendCam()
	withFar.Far = 8
	for _, tt := range []struct {
		name string
		m    Mesh
		cam  Camera
	}{
		{"flat", plain, blendCam()},
		{"smooth", smooth, blendCam()},
		{"seen from behind", backwards, blendCam()},
		{"repeating", repeating, blendCam()},
		{"negative coordinates", negative, blendCam()},
		{"the middle corner at the picture's origin", zeroMiddle, blendCam()},
		{"the last corner at the picture's origin", zeroLast, blendCam()},
		{"one corner cut off", behind(0), blendCam()},
		{"the middle corner cut off", behind(1), blendCam()},
		{"the last corner cut off", behind(2), blendCam()},
		{"two corners cut off", twoBehind, blendCam()},
		{"a corner past the far plane", farOnly, withFar},
		{"cut at both planes", pastFar, withFar},
		{"smooth, cut at both planes", func() Mesh {
			m := pastFar
			m.Normals = smooth.Normals
			return m
		}(), withFar},
	} {
		for _, pixels := range []bool{false, true} {
			name := tt.name + ", cells"
			if pixels {
				name = tt.name + ", pixels"
			}
			t.Run(name, func(t *testing.T) {
				if n := checkTextured(t, tt.m, tt.cam, pixels); n < 30 {
					t.Fatalf("wanted at least 30 places to compare, got %d", n)
				}
			})
		}
	}
}

// The same, for faces at random: a triangle with corners in front of and behind the camera, at
// any slant, with coordinates well outside the picture, from either side.
func TestTexturesAreDrawnAtThePlaceEveryCellIsSeenAtOnRandomFaces(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 1))
	coord := func(lo, hi float64) float64 { return lo + (hi-lo)*rng.Float64() }
	var compared int
	for trial := range 60 {
		m := texturedTriangle([3]geometry.Vector2{
			uv(coord(-2, 3), coord(-2, 3)), uv(coord(-2, 3), coord(-2, 3)), uv(coord(-2, 3), coord(-2, 3)),
		})
		m.Material.Texture = uniquePictureOf(1+rng.IntN(9), 1+rng.IntN(9))
		m.Vertices = nil
		for range 3 {
			m.Vertices = append(m.Vertices, geometry.Vector3{X: coord(-3, 3), Y: coord(-2, 2), Z: coord(-1, 10)})
		}
		if rng.IntN(2) == 0 {
			m.Normals = []geometry.Vector3{{X: coord(-1, 1), Y: coord(-1, 1), Z: -1}, {X: coord(-1, 1), Z: -1}, {Y: coord(-1, 1), Z: -1}}
		}
		cam := Camera{FOV: coord(0.8, 1.8), Near: coord(0.2, 1)}
		if rng.IntN(2) == 0 {
			cam.Far = coord(6, 12)
		}
		for _, pixels := range []bool{false, true} {
			compared += checkTextured(t, m, cam, pixels)
		}
		if t.Failed() {
			t.Fatalf("trial %d: %+v, %+v", trial, m.Vertices, cam)
		}
	}
	if compared < 5000 {
		t.Errorf("wanted at least 5000 places compared in all, got %d", compared)
	}
}

// onePictureOf is a picture of w by h pixels that are all c.
func onePictureOf(c engine.Color, w, h int) *bitmap.Bitmap {
	b := bitmap.New(w, h)
	for y := range h {
		for x := range w {
			b.Set(x, y, c)
		}
	}
	return b
}

// texturedWith returns a copy of m that is drawn in the picture where its faces are listed
// in textured, and every face has coordinates (which a face that is not listed has as a zero
// triple, which is none).
func texturedWith(m Mesh, c engine.Color, textured func(face int) bool) Mesh {
	out := m.Clone()
	out.FaceTexCoords = make([][3]geometry.Vector2, len(m.Faces))
	for i := range out.FaceTexCoords {
		if textured(i) {
			out.FaceTexCoords[i] = [3]geometry.Vector2{uv(0.1, 0.2), uv(0.6, 0.3), uv(0.4, 0.8)}
		}
	}
	out.Material.Texture = onePictureOf(c, 3, 2)
	return out
}

// framesOf draws the mesh into cells and into pixels and returns what was drawn, as two grids.
func framesOf(t *testing.T, m Mesh, cam Camera, lights ...Light) (cells []engine.Cell, pixels []engine.Color) {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "")
	fb := newShadeBuffer(t)
	m.DrawShaded(fb, cam, lights...)
	for y := range fb.Height() {
		for x := range fb.Width() {
			cells = append(cells, fb.At(x, y))
		}
	}
	canvas := newShadeBuffer(t).Canvas()
	m.DrawShadedPixels(canvas, cam, lights...)
	for y := range canvas.Height() {
		for x := range canvas.Width() {
			pixels = append(pixels, canvas.At(x, y))
		}
	}
	return cells, pixels
}

// shapesToTexture are the shapes the pictured faces are compared on, placed where the camera
// sees them, with their faces turned both ways and cut by the near plane.
func shapesToTexture() map[string]Mesh {
	place := func(m Mesh, z float64) Mesh {
		m.Transform = Transform{Position: geometry.Vector3{Z: z}, Yaw: 0.6, Pitch: 0.3}
		return m
	}
	flatSphere := place(Sphere(1.6, 10, 6), 4)
	flatSphere.Normals = nil
	cube := place(Cube(2), 4)
	cube.DoubleSided = true
	return map[string]Mesh{
		"smooth sphere":      place(Sphere(1.6, 10, 6), 4),
		"flat sphere":        flatSphere,
		"cube from outside":  place(Cube(2), 4),
		"cube both sides":    cube,
		"cube cut by near":   place(Cube(2), 0.2),
		"plane":              place(Plane(4, 4), 4),
		"big sphere cut off": place(Sphere(3, 12, 8), 1.5),
	}
}

// A face whose picture is one color is the same as a face of that color. This is the whole
// picture drawn in cells and in pixels, shapes, lights and fog included, and it pins down what
// the picture does not change: coverage, depth, the character, the shade, the fog and the
// near and far cuts.
func TestAPictureOfOneColorDrawsAsAFaceOfThatColor(t *testing.T) {
	c := engine.RGB(200, 120, 40)
	lights := []Light{sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2), Point{Position: geometry.Vector3{X: -1, Y: 1, Z: 2}, Range: 8}}
	fog := cam3(&Fog{Color: engine.RGB(30, 30, 90), Start: 2, End: 7})
	far := cam3(nil)
	far.Far = 5.5

	for name, m := range shapesToTexture() {
		for _, camera := range []struct {
			name string
			cam  Camera
		}{{"no fog", cam3(nil)}, {"fog", fog}, {"far plane", far}} {
			t.Run(name+", "+camera.name, func(t *testing.T) {
				// Every face has the picture, though FaceColors says otherwise on them.
				textured := texturedWith(m, c, func(int) bool { return true })
				textured.FaceColors = repeatColor(pureBlue, len(m.Faces))
				plain := m.Clone()
				plain.Material.Color = c

				wantCells, wantPixels := framesOf(t, plain, camera.cam, lights...)
				gotCells, gotPixels := framesOf(t, textured, camera.cam, lights...)
				sameFrames(t, wantCells, wantPixels, gotCells, gotPixels)
			})
		}
	}
}

// cam3 is the camera the comparisons look through, with a fog if there is one.
func cam3(fog *Fog) Camera {
	cam := shadeCam()
	cam.Fog = fog
	return cam
}

func repeatColor(c engine.Color, n int) []engine.Color {
	out := make([]engine.Color, n)
	for i := range out {
		out[i] = c
	}
	return out
}

// sameFrames fails the test for the first cell and the first pixel that differ.
func sameFrames(t *testing.T, wantCells []engine.Cell, wantPixels []engine.Color, gotCells []engine.Cell, gotPixels []engine.Color) {
	t.Helper()
	drawnAny := false
	for i := range wantCells {
		if wantCells[i] != gotCells[i] {
			t.Fatalf("cell %d (column %d, row %d): wanted %+v, got %+v", i, i%shadeW, i/shadeW, wantCells[i], gotCells[i])
		}
		drawnAny = drawnAny || wantCells[i].Character != 0
	}
	for i := range wantPixels {
		if wantPixels[i] != gotPixels[i] {
			t.Fatalf("pixel %d (column %d, row %d): wanted %v, got %v", i, i%shadeW, i/shadeW, wantPixels[i], gotPixels[i])
		}
	}
	if !drawnAny {
		t.Fatal("wanted a picture to compare, got an empty frame")
	}
}

// Where a face has no coordinates it is drawn as it is without a picture, in its FaceColors or
// the material's color; where it has them the picture is its color and FaceColors is not.
func TestFacesWithoutCoordinatesKeepTheirOwnColor(t *testing.T) {
	c := engine.RGB(200, 120, 40)
	lights := []Light{sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)}
	for name, m := range shapesToTexture() {
		t.Run(name, func(t *testing.T) {
			even := func(i int) bool { return i%2 == 0 }
			textured := texturedWith(m, c, even)
			textured.FaceColors = repeatColor(pureGreen, len(m.Faces))
			// Odd faces keep their green, even ones are the picture's color.
			plain := m.Clone()
			plain.FaceColors = make([]engine.Color, len(m.Faces))
			for i := range plain.FaceColors {
				plain.FaceColors[i] = pureGreen
				if even(i) {
					plain.FaceColors[i] = c
				}
			}
			// And without FaceColors the others take the material's color.
			bare := textured.Clone()
			bare.FaceColors = nil
			bare.Material.Color = pureRed
			bareWant := plain.Clone()
			bareWant.FaceColors = nil
			bareWant.Material.Color = pureRed
			for i := range bareWant.Faces {
				if even(i) {
					bareWant.FaceColors = repeatColor(none, len(m.Faces))
					break
				}
			}
			for i := range bareWant.FaceColors {
				if even(i) {
					bareWant.FaceColors[i] = c
				}
			}

			for _, pair := range [][2]Mesh{{plain, textured}, {bareWant, bare}} {
				wc, wp := framesOf(t, pair[0], shadeCam(), lights...)
				gc, gp := framesOf(t, pair[1], shadeCam(), lights...)
				sameFrames(t, wc, wp, gc, gp)
			}
		})
	}
}

// Vertex colors tint a picture, each channel multiplied, a corner without one lets all of it
// through, and a face none of whose corners has one is not tinted at all. A face with a color
// of its own is tinted the same way, so a picture of one color is the same as that color.
func TestVertexColorsTintAPicture(t *testing.T) {
	c := engine.RGB(200, 120, 40)
	lights := []Light{sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)}
	for name, m := range shapesToTexture() {
		t.Run(name, func(t *testing.T) {
			m.VertexColors = make([]engine.Color, len(m.Vertices))
			for i := range m.VertexColors {
				switch i % 4 {
				case 0: // none
				case 1:
					m.VertexColors[i] = engine.RGB(255, 255, 255)
				case 2:
					m.VertexColors[i] = engine.RGB(uint8(i*7), uint8(255-i*5), uint8(i*3))
				default:
					m.VertexColors[i] = engine.RGB(128, 64, 32)
				}
			}
			textured := texturedWith(m, c, func(int) bool { return true })
			plain := m.Clone()
			plain.FaceColors = repeatColor(c, len(m.Faces))

			wantCells, wantPixels := framesOf(t, plain, shadeCam(), lights...)
			gotCells, gotPixels := framesOf(t, textured, shadeCam(), lights...)

			// The rounding of a tint is done in another order, so a channel may differ by one.
			drawn := 0
			close := func(a, b engine.Color) bool {
				ar, ag, ab := a.RGB()
				br, bg, bb := b.RGB()
				d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
				return a.IsDefault() == b.IsDefault() && d(ar, br) <= 1 && d(ag, bg) <= 1 && d(ab, bb) <= 1
			}
			for i := range wantCells {
				if wantCells[i].Character != gotCells[i].Character || !close(wantCells[i].Foreground, gotCells[i].Foreground) {
					t.Fatalf("cell %d: wanted %+v, got %+v", i, wantCells[i], gotCells[i])
				}
				if wantCells[i].Character != 0 {
					drawn++
				}
			}
			for i := range wantPixels {
				if !close(wantPixels[i], gotPixels[i]) {
					t.Fatalf("pixel %d: wanted %v, got %v", i, wantPixels[i], gotPixels[i])
				}
			}
			if drawn < 50 {
				t.Fatalf("wanted at least 50 cells compared, got %d", drawn)
			}
		})
	}

	// A tint on a picture of many colors: each channel of the corner's color times the pixel's.
	m := texturedTriangle([3]geometry.Vector2{uv(0.05, 0.1), uv(0.98, 0.05), uv(0.4, 0.95)})
	m.VertexColors = []engine.Color{engine.RGB(255, 0, 0), engine.RGB(255, 0, 0), engine.RGB(255, 0, 0)}
	fb := newShadeBuffer(t)
	m.DrawShaded(fb, blendCam(), Ambient{Level: 1})
	seen := 0
	for y := range fb.Height() {
		for x := range fb.Width() {
			if f := fb.At(x, y).Foreground; !f.IsDefault() {
				r, g, b := f.RGB()
				if g != 0 || b != 0 || r < 20 {
					t.Fatalf("(%d, %d): wanted red alone, from a picture whose reds are at least 20, got %v", x, y, f)
				}
				seen++
			}
		}
	}
	if seen < 30 {
		t.Errorf("wanted at least 30 cells in a red tint, got %d", seen)
	}
}

// A pixel of the picture that is see-through shows the face as it would be without the picture.
func TestSeeThroughPixelsShowTheFacesOwnColor(t *testing.T) {
	pic := bitmap.New(2, 1)
	pic.Set(0, 0, pureRed) // the right pixel is left unset
	wall := Mesh{
		Vertices: []geometry.Vector3{{X: -1, Y: -1, Z: 3}, {X: -1, Y: 1, Z: 3}, {X: 1, Y: 1, Z: 3}, {X: 1, Y: -1, Z: 3}},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		FaceTexCoords: [][3]geometry.Vector2{
			{uv(0, 1), uv(0, 0), uv(1, 0)},
			{uv(0, 1), uv(1, 0), uv(1, 1)},
		},
		Material:    Material{Color: pureBlue, Texture: pic},
		DoubleSided: true,
	}
	for _, withFaceColor := range []bool{false, true} {
		m := wall
		want := pureBlue
		if withFaceColor {
			m.FaceColors = repeatColor(pureGreen, 2)
			want = pureGreen
		}
		fb := newShadeBuffer(t)
		m.DrawShaded(fb, shadeCam(), Ambient{Level: 1})
		left, right := 0, 0
		for y := range fb.Height() {
			for x := range fb.Width() {
				f := fb.At(x, y).Foreground
				switch {
				case f.IsDefault():
				case x < shadeW/2-3 && f == pureRed:
					left++
				case x > shadeW/2+3 && f == want:
					right++
				case x < shadeW/2-3 || x > shadeW/2+3:
					t.Fatalf("(%d, %d): wanted red on the left and %v on the right, got %v", x, y, want, f)
				}
			}
		}
		if left < 20 || right < 20 {
			t.Errorf("face color %v: wanted at least 20 cells on each side, got %d and %d", withFaceColor, left, right)
		}
	}
}

// What is not used: a list of another length than Faces, a face with a coordinate that is not
// finite, no picture, a picture with no pixels. Each is drawn as the mesh is without them, and
// Validate says what is wrong.
func TestTextureCoordinatesThatCannotBeUsedAreLeftOut(t *testing.T) {
	good := [3]geometry.Vector2{uv(0.1, 0.2), uv(0.6, 0.3), uv(0.4, 0.8)}
	base := Sphere(1.6, 10, 6)
	base.Transform.Position = geometry.Vector3{Z: 4}
	base.Material.Color = pureBlue
	lights := []Light{sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)}

	listOf := func(n int) [][3]geometry.Vector2 {
		out := make([][3]geometry.Vector2, n)
		for i := range out {
			out[i] = good
		}
		return out
	}
	bad := func(v float64) [][3]geometry.Vector2 {
		out := listOf(len(base.Faces))
		out[len(out)/2][1].Y = v
		return out
	}
	for _, tt := range []struct {
		name     string
		coords   [][3]geometry.Vector2
		picture  *bitmap.Bitmap
		wantErr  bool
		wantSame bool // the same as without any coordinates
	}{
		{"too few", listOf(len(base.Faces) - 1), onePictureOf(pureRed, 2, 2), true, true},
		{"too many", listOf(len(base.Faces) + 1), onePictureOf(pureRed, 2, 2), true, true},
		{"no picture", listOf(len(base.Faces)), nil, false, true},
		{"an empty picture", listOf(len(base.Faces)), bitmap.New(0, 0), false, true},
		{"a NaN", bad(math.NaN()), onePictureOf(pureRed, 2, 2), true, false},
		{"an infinity", bad(math.Inf(1)), onePictureOf(pureRed, 2, 2), true, false},
		{"all as they should be", listOf(len(base.Faces)), onePictureOf(pureRed, 2, 2), false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := base.Clone()
			m.FaceTexCoords, m.Material.Texture = tt.coords, tt.picture
			if err := m.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("wanted an error: %v, got %v", tt.wantErr, err)
			}

			gc, gp := framesOf(t, m, shadeCam(), lights...)
			wc, wp := framesOf(t, base, shadeCam(), lights...)
			same := slicesEqual(gc, wc) && slicesEqual(gp, wp)
			if tt.wantSame && !same {
				t.Errorf("wanted what the mesh draws without them")
			}
			if tt.name == "all as they should be" && same {
				t.Errorf("wanted a different picture with them")
			}
			if tt.name == "a NaN" || tt.name == "an infinity" {
				// Only the face that has the bad number is left out: the others are drawn in red,
				// and it is drawn as a face with no coordinates is.
				if red := countColor(gc, pureRed); red == 0 {
					t.Errorf("wanted the other faces drawn from the picture")
				}
				zeroed := m.Clone()
				zeroed.FaceTexCoords[len(zeroed.FaceTexCoords)/2] = [3]geometry.Vector2{}
				zc, zp := framesOf(t, zeroed, shadeCam(), lights...)
				if !slicesEqual(gc, zc) || !slicesEqual(gp, zp) {
					t.Errorf("wanted the face with the bad coordinate drawn as one with none")
				}
			}
		})
	}

	// A picture and no coordinates, or the reverse, is a mesh that Validate has no complaint about.
	m := base.Clone()
	m.Material.Texture = onePictureOf(pureRed, 2, 2)
	if err := m.Validate(); err != nil {
		t.Errorf("wanted no error for a picture without coordinates, got %v", err)
	}
	m = base.Clone()
	m.FaceTexCoords = listOf(len(base.Faces))
	if err := m.Validate(); err != nil {
		t.Errorf("wanted no error for coordinates without a picture, got %v", err)
	}
}

func slicesEqual[T comparable](a, b []T) bool {
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

func countColor(cells []engine.Cell, c engine.Color) int {
	n := 0
	for _, cell := range cells {
		if cell.Foreground == c {
			n++
		}
	}
	return n
}

// Wireframes ignore pictures and coordinates.
func TestWireframesIgnoreTextures(t *testing.T) {
	m := texturedTriangle([3]geometry.Vector2{uv(0.05, 0.1), uv(0.98, 0.05), uv(0.4, 0.95)})
	m.Edges = [][2]int{{0, 1}, {1, 2}, {2, 0}}
	bare := m
	bare.FaceTexCoords, bare.Material.Texture = nil, nil

	draw := func(m Mesh) (a, b []engine.Cell) {
		fb := newShadeBuffer(t)
		m.Draw(fb, blendCam(), engine.Cell{Character: '#'})
		fb2 := newShadeBuffer(t)
		m.DrawDepth(fb2, blendCam(), engine.Cell{Character: '#'})
		for y := range fb.Height() {
			for x := range fb.Width() {
				a = append(a, fb.At(x, y))
				b = append(b, fb2.At(x, y))
			}
		}
		return a, b
	}
	wa, wb := draw(bare)
	ga, gb := draw(m)
	if !slicesEqual(wa, ga) || !slicesEqual(wb, gb) {
		t.Errorf("wanted the wireframe of a mesh without a picture")
	}
}

// A clone has coordinates of its own and shares the picture.
func TestCloneCopiesTextureCoordinates(t *testing.T) {
	m := texturedTriangle([3]geometry.Vector2{uv(0.05, 0.1), uv(0.98, 0.05), uv(0.4, 0.95)})
	c := m.Clone()
	c.FaceTexCoords[0][0] = uv(7, 7)
	if m.FaceTexCoords[0][0] != uv(0.05, 0.1) {
		t.Errorf("wanted the original's coordinates left alone, got %v", m.FaceTexCoords[0][0])
	}
	if c.Material.Texture != m.Material.Texture {
		t.Errorf("wanted the same picture in the clone")
	}
	if got := (Mesh{}).Clone(); got.FaceTexCoords != nil {
		t.Errorf("wanted no coordinates in the clone of an empty mesh, got %v", got.FaceTexCoords)
	}
}

func TestTexelPicksTheNearestPixelAndRepeats(t *testing.T) {
	pic := uniquePicture() // 8 across and 5 down
	px := func(x, y int) engine.Color { return pic.At(x, y) }
	for _, tt := range []struct {
		name string
		u, v float64
		want engine.Color
	}{
		{"the top left", 0, 0, px(0, 0)},
		{"just inside the first pixel", 0.12, 0.12, px(0, 0)},
		{"the second column starts at 1/8", 0.125, 0.21, px(1, 1)},
		{"the middle of a pixel", 0.3125, 0.7, px(2, 3)},
		{"the last pixel", 0.99, 0.99, px(7, 4)},
		{"exactly 1 is the top left again", 1, 1, px(0, 0)},
		{"2.5 is the middle", 2.5, 3.5, px(4, 2)},
		{"-0.1 is the end of the previous repeat", -0.1, -0.1, px(7, 4)},
		{"-1 is the start", -1, -3, px(0, 0)},
		{"a tiny negative that rounds up to 1", -1e-20, -1e-20, px(7, 4)},
		{"the largest below 1", math.Nextafter(1, 0), math.Nextafter(1, 0), px(7, 4)},
		{"a huge number", 1e300, 1e300, px(0, 0)},
		{"NaN u", math.NaN(), 0.5, none},
		{"NaN v", 0.5, math.NaN(), none},
		{"+Inf", math.Inf(1), 0.5, none},
		{"-Inf", 0.5, math.Inf(-1), none},
	} {
		if got := texel(pic, tt.u, tt.v); got != tt.want {
			t.Errorf("%s: texel(%v, %v): wanted %v, got %v", tt.name, tt.u, tt.v, tt.want, got)
		}
	}

	if got := texel(nil, 0.5, 0.5); !got.IsDefault() {
		t.Errorf("wanted none for no picture, got %v", got)
	}
	if got := texel(bitmap.New(0, 0), 0.5, 0.5); !got.IsDefault() {
		t.Errorf("wanted none for an empty picture, got %v", got)
	}
	one := onePictureOf(pureRed, 1, 1)
	for _, u := range []float64{0, 0.5, 0.999, 1, -7.3} {
		if got := texel(one, u, u); got != pureRed {
			t.Errorf("wanted the one pixel for %v, got %v", u, got)
		}
	}
	// A pixel that is not set is none, and a view counts from its own corner and repeats in itself.
	holes := bitmap.New(2, 2)
	holes.Set(1, 0, pureGreen)
	if got := texel(holes, 0.25, 0.25); !got.IsDefault() {
		t.Errorf("wanted none for a see-through pixel, got %v", got)
	}
	if got := texel(holes, 0.75, 0.25); got != pureGreen {
		t.Errorf("wanted the green pixel, got %v", got)
	}
	view := pic.Sub(2, 3, 2, 2)
	if got := texel(view, 0.75, 1.25); got != px(3, 3) {
		t.Errorf("wanted the pixel (3, 3) of the picture through a view, got %v", got)
	}
	mirrored := pic.Mirrored()
	if got := texel(mirrored, 0.01, 0.01); got != px(7, 0) {
		t.Errorf("wanted the top right pixel through a mirrored view, got %v", got)
	}
}

func TestDrawingTexturesAllocatesNothingOnceWarm(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector makes sync.Pool drop items, so allocation counts mean nothing")
	}
	t.Setenv("TERM_PROGRAM", "")
	m := Sphere(1.6, 10, 6)
	m.Transform = Transform{Position: geometry.Vector3{Z: 4}}
	m.FaceTexCoords = make([][3]geometry.Vector2, len(m.Faces))
	for i := range m.FaceTexCoords {
		m.FaceTexCoords[i] = [3]geometry.Vector2{uv(0.1, 0.2), uv(float64(i%5), 0.3), uv(0.4, float64(i%3))}
	}
	m.Material.Texture = uniquePicture()
	m.VertexColors = make([]engine.Color, len(m.Vertices))
	for i := range m.VertexColors {
		m.VertexColors[i] = engine.RGB(uint8(i*7), uint8(255-i*5), uint8(i*3))
	}
	flat := m.Clone()
	flat.Normals = nil
	untinted := m.Clone()
	untinted.VertexColors = nil
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)
	fb := newShadeBuffer(t)

	cam := shadeCam()
	fogged := cam
	fogged.Fog = &Fog{Color: pureBlue, Start: 1, End: 8}
	for name, draw := range map[string]func(){
		"smooth glyphs":   func() { m.DrawShaded(fb, cam, light) },
		"smooth pixels":   func() { m.DrawShadedPixels(fb.Canvas(), cam, light) },
		"flat glyphs":     func() { flat.DrawShaded(fb, cam, light) },
		"flat pixels":     func() { flat.DrawShadedPixels(fb.Canvas(), cam, light) },
		"untinted glyphs": func() { untinted.DrawShaded(fb, cam, light) },
		"untinted pixels": func() { untinted.DrawShadedPixels(fb.Canvas(), cam, light) },
		"fogged glyphs":   func() { untinted.DrawShaded(fb, fogged, light) },
		"fogged pixels":   func() { untinted.DrawShadedPixels(fb.Canvas(), fogged, light) },
	} {
		draw()
		if n := testing.AllocsPerRun(20, draw); n != 0 {
			t.Errorf("%s: wanted no allocation, got %v", name, n)
		}
	}
}

func TestPointWeightsAreTheBarycentricWeightsOfAPointOnATriangle(t *testing.T) {
	v0, v1, v2 := geometry.Vector3{X: 1, Y: 2, Z: 5}, geometry.Vector3{X: 4, Y: 2, Z: 6}, geometry.Vector3{X: 2, Y: 6, Z: 7}
	third := 1.0 / 3
	for _, tt := range []struct {
		name string
		p    geometry.Vector3
		want [3]float64
	}{
		{"the first corner", v0, [3]float64{1, 0, 0}},
		{"the second corner", v1, [3]float64{0, 1, 0}},
		{"the third corner", v2, [3]float64{0, 0, 1}},
		{"the middle of the first edge", v0.Add(v1).Scale(0.5), [3]float64{0.5, 0.5, 0}},
		{"the middle of the last edge", v0.Add(v2).Scale(0.5), [3]float64{0.5, 0, 0.5}},
		{"the middle", v0.Add(v1).Add(v2).Scale(third), [3]float64{third, third, third}},
		{"beyond the second corner", v0.Add(v1.Sub(v0).Scale(2)), [3]float64{-1, 2, 0}},
	} {
		w0, w1, w2 := pointWeights(v0, v1, v2, tt.p)
		got := [3]float64{w0, w1, w2}
		for i := range got {
			if math.Abs(got[i]-tt.want[i]) > 1e-12 {
				t.Errorf("%s: wanted %v, got %v", tt.name, tt.want, got)
				break
			}
		}
		if math.Abs(w0+w1+w2-1) > 1e-12 {
			t.Errorf("%s: wanted weights that sum to 1, got %v", tt.name, got)
		}
	}

	// A triangle with no area has no cells; all of the weight goes to its first corner, so a color
	// asked for is that corner's.
	line := geometry.Vector3{X: 2, Y: 2, Z: 2}
	for name, tri := range map[string][3]geometry.Vector3{
		"a point":     {line, line, line},
		"a line":      {line, line.Scale(2), line.Scale(3)},
		"two corners": {v0, v0, v1},
	} {
		if w0, w1, w2 := pointWeights(tri[0], tri[1], tri[2], line); w0 != 1 || w1 != 0 || w2 != 0 {
			t.Errorf("%s: wanted all the weight on the first corner, got %v, %v, %v", name, w0, w1, w2)
		}
		corners := [3]geometry.Vector3{{X: 10, Y: 20, Z: 30}, {X: 1}, {Y: 2}}
		if got := colorAtPoint(corners, tri[0], tri[1], tri[2], line); got != corners[0] {
			t.Errorf("%s: wanted the color of the first corner, got %v", name, got)
		}
	}
}

// Validate looks at all six numbers of a triple.
func TestValidateFindsABadNumberInAnyTextureCoordinate(t *testing.T) {
	for corner := range 3 {
		for _, axis := range []string{"u", "v"} {
			for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
				m := texturedTriangle([3]geometry.Vector2{uv(0.1, 0.2), uv(0.6, 0.3), uv(0.4, 0.8)})
				p := &m.FaceTexCoords[0][corner].X
				if axis == "v" {
					p = &m.FaceTexCoords[0][corner].Y
				}
				if err := m.Validate(); err != nil {
					t.Fatalf("wanted a valid mesh to start with, got %v", err)
				}
				*p = bad
				if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "face 0") {
					t.Errorf("%v as %s of corner %d: wanted an error that names face 0, got %v", bad, axis, corner, err)
				}
			}
		}
	}
}

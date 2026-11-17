package mesh

import (
	"math"
	"testing"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// drawMeshes draws the meshes in order, into cells and into pixels, lit by one light of full
// strength, and returns what was drawn.
func drawMeshes(t *testing.T, cam Camera, meshes ...Mesh) (cells []engine.Cell, pixels []engine.Color) {
	t.Helper()
	return drawLit(t, cam, Ambient{Level: 1}, meshes...)
}

// drawLit draws the meshes in order, into cells and into pixels, lit by light, and returns what was
// drawn: every cell, and every pixel, row by row.
func drawLit(t *testing.T, cam Camera, light Light, meshes ...Mesh) (cells []engine.Cell, pixels []engine.Color) {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "")
	fb := newShadeBuffer(t)
	for _, m := range meshes {
		m.DrawShaded(fb, cam, light)
	}
	for y := range fb.Height() {
		for x := range fb.Width() {
			cells = append(cells, fb.At(x, y))
		}
	}
	canvas := newShadeBuffer(t).Canvas()
	for _, m := range meshes {
		m.DrawShadedPixels(canvas, cam, light)
	}
	for y := range canvas.Height() {
		for x := range canvas.Width() {
			pixels = append(pixels, canvas.At(x, y))
		}
	}
	return cells, pixels
}

// wallAt is a wall facing the camera, w wide and h tall, at depth z, painted in color.
func wallAt(z, w, h float64, color engine.Color) Mesh {
	return Mesh{
		Vertices:    []geometry.Vector3{{X: -w / 2, Y: -h / 2, Z: z}, {X: -w / 2, Y: h / 2, Z: z}, {X: w / 2, Y: h / 2, Z: z}, {X: w / 2, Y: -h / 2, Z: z}},
		Faces:       [][3]int{{0, 1, 2}, {0, 2, 3}},
		Material:    Material{Color: color},
		DoubleSided: true,
	}
}

// A pixel of the picture that is see-through is a hole: nothing is drawn there, and no depth is
// stored, so what is behind shows, whichever was drawn first.
func TestSeeThroughPixelsAreHoles(t *testing.T) {
	picture := func(right engine.Color) *bitmap.Bitmap {
		pic := bitmap.New(2, 1)
		pic.Set(0, 0, pureRed)
		pic.Set(1, 0, right)
		return pic
	}
	pin := func(m Mesh, pic *bitmap.Bitmap) Mesh {
		m.FaceTexCoords = [][3]geometry.Vector2{{uv(0, 1), uv(0, 0), uv(1, 0)}, {uv(0, 1), uv(1, 0), uv(1, 1)}}
		m.Material.Texture = pic
		return m
	}
	fence := pin(wallAt(3, 2, 2, pureBlue), picture(engine.Color{})) // the right half is a hole
	fence.FaceColors = repeatColor(pureYellow, 2)                    // which is not what shows there
	whole := pin(wallAt(3, 2, 2, pureBlue), picture(pureRed))        // the same wall with no hole
	backdrop := wallAt(6, 9, 6, pureGreen)

	// Where the wall is, found from the one with no hole.
	cells, _ := drawMeshes(t, shadeCam(), whole)
	x0, x1, y0, y1 := shadeW, -1, shadeH, -1
	for i, c := range cells {
		if c.Foreground == pureRed {
			x, y := i%shadeW, i/shadeW
			x0, x1, y0, y1 = min(x0, x), max(x1, x), min(y0, y), max(y1, y)
		}
	}
	if x1-x0 < 10 || y1-y0 < 3 {
		t.Fatalf("wanted a wall of some size, got the box (%d, %d) to (%d, %d)", x0, y0, x1, y1)
	}
	middle := (x0 + x1) / 2

	for name, order := range map[string][]Mesh{"wall then backdrop": {fence, backdrop}, "backdrop then wall": {backdrop, fence}} {
		cells, pixels := drawMeshes(t, shadeCam(), order...)
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				cell, pixel := cells[y*shadeW+x].Foreground, pixels[2*y*shadeW+x]
				switch {
				case x < middle-1: // the red half
					if cell != pureRed || pixel != engine.Shade(pureRed, 1) {
						t.Fatalf("%s: (%d, %d): wanted the red pixel of the picture, got %v and %v", name, x, y, cell, pixel)
					}
				case x > middle+1: // the hole: the backdrop shows
					if cell != pureGreen || pixel != engine.Shade(pureGreen, 1) {
						t.Fatalf("%s: (%d, %d): wanted the backdrop through the hole, got %v and %v", name, x, y, cell, pixel)
					}
				}
			}
		}
	}

	// A solid nearer than the wall still hides it, hole or not.
	cells, _ = drawMeshes(t, shadeCam(), backdrop, fence, wallAt(2, 4, 3, pureBlue))
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if got := cells[y*shadeW+x].Foreground; got != pureBlue {
				t.Fatalf("(%d, %d): wanted the nearer solid, got %v", x, y, got)
			}
		}
	}
}

// A face with a coordinate that is not a finite number has no place in the picture to look up, so
// it draws nothing; the same goes for each of its six numbers.
func TestAFaceWithACoordinateThatIsNotFiniteDrawsNothing(t *testing.T) {
	good := [3]geometry.Vector2{uv(0.05, 0.1), uv(0.98, 0.05), uv(0.4, 0.95)}
	drawnAny := func(m Mesh) bool {
		cells, pixels := drawMeshes(t, blendCam(), m)
		for _, c := range cells {
			if !c.Foreground.IsDefault() {
				return true
			}
		}
		for _, p := range pixels {
			if !p.IsDefault() {
				return true
			}
		}
		return false
	}
	if !drawnAny(texturedTriangle(good)) {
		t.Fatal("wanted a face with good coordinates to be drawn")
	}
	for corner := range 3 {
		for _, axis := range []string{"u", "v"} {
			for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
				m := texturedTriangle(good)
				p := &m.FaceTexCoords[0][corner].X
				if axis == "v" {
					p = &m.FaceTexCoords[0][corner].Y
				}
				*p = bad
				if drawnAny(m) {
					t.Errorf("%v as %s of corner %d: wanted nothing drawn", bad, axis, corner)
				}
			}
		}
	}
}

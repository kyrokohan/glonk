package mesh

import (
	"testing"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// viewWall is a wall 4 wide and 2 tall at z 3, square to a camera at the origin, with pic on it
// once: the left half of the screen shows its left pixel and the right half its right one.
func viewWall(pic *bitmap.Bitmap) Mesh {
	return Mesh{
		Vertices: []geometry.Vector3{{X: -2, Y: -1, Z: 3}, {X: -2, Y: 1, Z: 3}, {X: 2, Y: 1, Z: 3}, {X: 2, Y: -1, Z: 3}},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		FaceTexCoords: [][3]geometry.Vector2{
			{{X: 0, Y: 1}, {X: 0, Y: 0}, {X: 1, Y: 0}},
			{{X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}},
		},
		Material: Material{Texture: pic},
	}
}

// halves draws the wall with its picture under one ambient light, in pixels and in cells, and
// returns the colors of its left and right halves in each.
func halves(t *testing.T, pic *bitmap.Bitmap, light float64) (pixels, cells [2]engine.Color) {
	t.Helper()
	fb, err := engine.NewFrameBuffer(40, 12)
	if err != nil {
		t.Fatal(err)
	}
	m := viewWall(pic)
	cv := fb.Canvas()
	cv.FillRect(0, 0, 40, 24, engine.RGB(9, 9, 9)) // what is behind, to see a hole by
	m.DrawShadedPixels(cv, Camera{}, Ambient{Level: light})
	m.DrawShaded(fb, Camera{}, Ambient{Level: light})
	return [2]engine.Color{cv.At(14, 12), cv.At(25, 12)}, [2]engine.Color{fb.At(14, 6).Foreground, fb.At(25, 6).Foreground}
}

// A picture that is a Shaded or Tinted view is drawn on a mesh as the view shows it, under the
// mesh's light; a see-through pixel of it is still a hole.
func TestAViewIsDrawnOnAMeshAsItLooks(t *testing.T) {
	red, blue := engine.RGB(220, 40, 40), engine.RGB(40, 60, 220)
	white := engine.RGB(255, 255, 255)
	pic := bitmap.New(2, 1)
	pic.Set(0, 0, red)
	pic.Set(1, 0, blue)

	for _, tt := range []struct {
		name  string
		pic   *bitmap.Bitmap
		light float64
		want  [2]engine.Color
	}{
		{"plain", pic, 1, [2]engine.Color{red, blue}},
		{"tinted white", pic.Tinted(white, 1), 1, [2]engine.Color{white, white}},
		{"half tinted", pic.Tinted(white, 0.5), 1, [2]engine.Color{engine.Mix(red, white, 0.5), engine.Mix(blue, white, 0.5)}},
		{"shaded", pic.Shaded(0.5), 1, [2]engine.Color{engine.Shade(red, 0.5), engine.Shade(blue, 0.5)}},
		// The view's color is the surface's, and the light comes after it: a white flash in half
		// light is as dark as a white wall in it.
		{"tinted white in half light", pic.Tinted(white, 1), 0.5, [2]engine.Color{litWhite(t, 0.5), litWhite(t, 0.5)}},
	} {
		pixels, _ := halves(t, tt.pic, tt.light)
		if pixels != tt.want {
			t.Errorf("%s: wanted the halves %v, got %v", tt.name, tt.want, pixels)
		}
	}

	// In cells the color is the view's too.
	if _, cells := halves(t, pic.Tinted(white, 1), 1); cells != [2]engine.Color{white, white} {
		t.Errorf("in cells: wanted both halves white, got %v", cells)
	}

	// A see-through pixel of a tinted view is still a hole: what is behind shows.
	holed := bitmap.New(2, 1)
	holed.Set(0, 0, red)
	if pixels, _ := halves(t, holed.Tinted(white, 1), 1); pixels[0] != white || pixels[1] != engine.RGB(9, 9, 9) {
		t.Errorf("wanted the left half white and a hole on the right, got %v", pixels)
	}
}

// litWhite is the color a plain white wall is drawn in under one ambient light.
func litWhite(t *testing.T, light float64) engine.Color {
	t.Helper()
	pic := bitmap.New(1, 1)
	pic.Set(0, 0, engine.RGB(255, 255, 255))
	pixels, _ := halves(t, pic, light)
	if pixels[0] == engine.RGB(255, 255, 255) {
		t.Fatalf("wanted a white wall in a light of %v drawn darker than white", light)
	}
	return pixels[0]
}

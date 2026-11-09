package mesh

import (
	"fmt"
	"math"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

func TestShadeChar(t *testing.T) {
	tests := []struct {
		name      string
		intensity float64
		want      rune
	}{
		{"darkest", 0, '.'},
		{"brightest", 1, '@'},
		{"middle", 0.5, '+'},
		{"just above darkest rounds down", 0.05, '.'},
		{"just below brightest rounds up", 0.95, '@'},
		{"below range clamps", -3, '.'},
		{"above range clamps", 3, '@'},
		{"NaN is dark", math.NaN(), '.'},
		{"negative infinity clamps", math.Inf(-1), '.'},
		{"positive infinity clamps", math.Inf(1), '@'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shadeChar(defaultRampRunes, tt.intensity); got != tt.want {
				t.Errorf("wanted %q, got %q", tt.want, got)
			}
		})
	}

	// A lit face must never turn into the blank background.
	for i := 0; i <= 100; i++ {
		if got := shadeChar(defaultRampRunes, float64(i)/100); got == ' ' {
			t.Fatalf("wanted no blank in the ramp, got one at %d%%", i)
		}
	}
}

func TestAmbientPlusDirectionalBrightness(t *testing.T) {
	down := geometry.Vector3{Y: -1}
	up := geometry.Vector3{Y: 1}
	side := geometry.Vector3{X: 1}
	diag := geometry.Vector3{X: math.Sqrt2 / 2, Y: math.Sqrt2 / 2}

	tests := []struct {
		name   string
		light  Light
		normal geometry.Vector3
		want   float64
	}{
		{"facing the light", sunAndFill(down, 0.2), up, 1},
		{"turned away", sunAndFill(down, 0.2), down, 0.2},
		{"edge on", sunAndFill(down, 0.2), side, 0.2},
		{"at 45 degrees", sunAndFill(down, 0.2), diag, 0.2 + 0.8*math.Sqrt2/2},
		{"direction length does not matter", sunAndFill(down.Scale(5), 0.2), up, 1},
		{"direction length does not matter at an angle", sunAndFill(down.Scale(0.01), 0.2), diag, 0.2 + 0.8*math.Sqrt2/2},
		{"no ambient, away", sunAndFill(down, 0), down, 0},
		{"no ambient, facing", sunAndFill(down, 0), up, 1},
		{"zero direction is ambient only", sunAndFill(geometry.Vector3{}, 0.3), up, 0.3},
		{"full ambient", sunAndFill(down, 1), down, 1},
		{"ambient above 1 clamps", sunAndFill(down, 4), down, 1},
		{"negative ambient clamps", sunAndFill(down, -2), down, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.light.Brightness(geometry.Vector3{}, tt.normal); !floatEquals(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

func TestFrontFacing(t *testing.T) {
	tests := []struct {
		name   string
		normal geometry.Vector3
		v0     geometry.Vector3
		want   bool
	}{
		{"straight at the camera", geometry.Vector3{Z: -1}, geometry.Vector3{Z: 5}, true},
		{"straight away", geometry.Vector3{Z: 1}, geometry.Vector3{Z: 5}, false},
		{"edge on", geometry.Vector3{X: 1}, geometry.Vector3{Z: 5}, false},
		{"off to the side, turned toward the camera", geometry.Vector3{X: -1}, geometry.Vector3{X: 10, Z: 5}, true},
		{"off to the side, turned away", geometry.Vector3{X: 1}, geometry.Vector3{X: 10, Z: 5}, false},
		{"normal length does not matter", geometry.Vector3{Z: -100}, geometry.Vector3{Z: 5}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := frontFacing(tt.normal, tt.v0); got != tt.want {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

// ---- drawing ----

const shadeW, shadeH = 80, 24

func shadeCam() Camera {
	return Camera{FOV: math.Pi / 2, Near: 0.1}
}

// lightSet is lights that light a surface together, as one Light.
type lightSet []Light

func (s lightSet) Brightness(point, normal geometry.Vector3) float64 {
	return brightness(s, point, normal)
}

// sunAndFill is a Directional light with some Ambient fill under it, the pair most
// of these tests light with.
func sunAndFill(direction geometry.Vector3, ambient float64) Light {
	return lightSet{Ambient{Level: ambient}, Directional{Direction: direction}}
}

// withColor returns a copy of m that is drawn in color.
func (m Mesh) withColor(color engine.Color) Mesh {
	m.Material.Color = color
	return m
}

func newShadeBuffer(t *testing.T) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(shadeW, shadeH)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

// drawn lists the coordinates of every cell that is not blank.
func drawn(fb *engine.FrameBuffer) [][2]int {
	var cells [][2]int
	for y := range fb.Height() {
		for x := range fb.Width() {
			if fb.At(x, y) != engine.BlankCell {
				cells = append(cells, [2]int{x, y})
			}
		}
	}
	return cells
}

func TestDrawShadedCullsBackFaces(t *testing.T) {
	verts := []geometry.Vector3{{X: -1, Y: -1, Z: 5}, {X: 1, Y: -1, Z: 5}, {X: 0, Y: 1, Z: 5}}
	light := sunAndFill(geometry.Vector3{Z: 1}, 0.1)

	tests := []struct {
		name string
		face [3]int
		want bool
	}{
		// (b-a)x(c-a) is +z for 0,1,2: pointing away from a camera at the origin.
		{"wound away from the camera", [3]int{0, 1, 2}, false},
		{"wound toward the camera", [3]int{0, 2, 1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := newShadeBuffer(t)
			m := Mesh{Vertices: verts, Faces: [][3]int{tt.face}}

			m.withColor(engine.Palette(7)).DrawShaded(fb, shadeCam(), light)

			if got := len(drawn(fb)) > 0; got != tt.want {
				t.Errorf("wanted drawn=%v, got %v", tt.want, got)
			}
		})
	}
}

func TestDrawShadedUsesColorAndShade(t *testing.T) {
	fb := newShadeBuffer(t)
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 5}
	// The light travels along +z, straight into the face that looks back at the camera.
	light := sunAndFill(geometry.Vector3{Z: 1}, 0)

	cube.withColor(engine.Palette(208)).DrawShaded(fb, shadeCam(), light)

	cells := drawn(fb)
	if len(cells) == 0 {
		t.Fatalf("wanted the cube drawn, got a blank frame")
	}
	for _, c := range cells {
		got := fb.At(c[0], c[1])
		if got.Foreground != engine.Palette(208) {
			t.Errorf("wanted palette 208 at %v, got %v", c, got.Foreground)
		}
		// Seen head on, only the front face shows, lit at full strength.
		if got.Character != '@' {
			t.Errorf("wanted '@' at %v, got %q", c, got.Character)
		}
	}
}

func TestDrawShadedFacesDiffer(t *testing.T) {
	fb := newShadeBuffer(t)
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 5}
	cube.Transform.Yaw, cube.Transform.Pitch = 0.6, 0.4
	light := sunAndFill(geometry.Vector3{X: 0.3, Y: -1, Z: 0.5}, 0.1)

	cube.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), light)

	shades := map[rune]bool{}
	for _, c := range drawn(fb) {
		shades[fb.At(c[0], c[1]).Character] = true
	}
	// Turned to show three faces, each lit differently; never more than three faces show.
	if len(shades) < 2 || len(shades) > 3 {
		t.Errorf("wanted 2 or 3 distinct shades, got %d: %v", len(shades), shades)
	}
}

func TestDrawShadedDepthOrder(t *testing.T) {
	near := Cube(2)
	near.Transform.Position = geometry.Vector3{Z: 4}
	far := Cube(2)
	far.Transform.Position = geometry.Vector3{Z: 10}
	light := sunAndFill(geometry.Vector3{Z: 1}, 0.2)

	for _, nearFirst := range []bool{true, false} {
		fb := newShadeBuffer(t)
		if nearFirst {
			near.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), light)
			far.withColor(engine.Palette(2)).DrawShaded(fb, shadeCam(), light)
		} else {
			far.withColor(engine.Palette(2)).DrawShaded(fb, shadeCam(), light)
			near.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), light)
		}

		// The far cube sits entirely behind the near one on screen.
		if got := fb.At(shadeW/2, shadeH/2).Foreground; got != engine.Palette(1) {
			t.Errorf("nearFirst=%v: wanted the near cube at the centre, got color %d", nearFirst, got)
		}
		for _, c := range drawn(fb) {
			if got := fb.At(c[0], c[1]).Foreground; got != engine.Palette(1) {
				t.Fatalf("nearFirst=%v: wanted only the near cube, got color %d at %v", nearFirst, got, c)
			}
		}
	}
}

func TestDrawShadedPlacement(t *testing.T) {
	light := sunAndFill(geometry.Vector3{Z: 1}, 0)

	for _, tt := range []struct {
		name  string
		x     float64
		right bool
	}{
		{"to the right", 3, true},
		{"to the left", -3, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fb := newShadeBuffer(t)
			cube := Cube(1)
			cube.Transform.Position = geometry.Vector3{X: tt.x, Z: 5}

			cube.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), light)

			cells := drawn(fb)
			if len(cells) == 0 {
				t.Fatalf("wanted the cube drawn, got a blank frame")
			}
			for _, c := range cells {
				if (c[0] > shadeW/2) != tt.right {
					t.Fatalf("wanted every cell on the %v side, got %v", tt.name, c)
				}
			}
		})
	}
}

func TestDrawShadedBehindCamera(t *testing.T) {
	fb := newShadeBuffer(t)
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: -5}

	cube.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), sunAndFill(geometry.Vector3{Z: 1}, 0))

	if cells := drawn(fb); len(cells) != 0 {
		t.Errorf("wanted a blank frame, got %d cells", len(cells))
	}
}

// A floor running from behind the camera to far ahead must be cut at the near
// plane, not dropped: it fills the lower screen and never rises above the horizon.
func TestDrawShadedFloorCrossingTheNearPlane(t *testing.T) {
	fb := newShadeBuffer(t)
	floor := Mesh{
		Vertices: []geometry.Vector3{
			{X: -5, Y: -1.5, Z: -5}, {X: -5, Y: -1.5, Z: 10}, {X: 5, Y: -1.5, Z: 10}, {X: 5, Y: -1.5, Z: -5},
		},
		// (b-a)x(c-a) is +y, facing up toward the camera above it.
		Faces: [][3]int{{0, 1, 2}, {0, 2, 3}},
	}

	floor.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), sunAndFill(geometry.Vector3{Y: -1}, 0))

	// The far edge is at z=10: 1.5/10 below the horizon, so row 13 and down.
	for y := range 13 {
		for x := range shadeW {
			if fb.At(x, y) != engine.BlankCell {
				t.Fatalf("wanted rows above the floor's far edge blank, got a cell at (%d,%d)", x, y)
			}
		}
	}
	if fb.At(shadeW/2, shadeH-1).Character != '@' {
		t.Errorf("wanted the floor to reach the bottom row, got %q", fb.At(shadeW/2, shadeH-1).Character)
	}
	if fb.At(shadeW/2, 14) == engine.BlankCell {
		t.Errorf("wanted the floor to fill the view just below its far edge")
	}
}

func TestDrawShadedWithoutFaces(t *testing.T) {
	fb := newShadeBuffer(t)
	grid := Grid(10, 5)
	grid.Transform.Position = geometry.Vector3{Y: -1, Z: 5}

	grid.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), sunAndFill(geometry.Vector3{Y: -1}, 0))

	if cells := drawn(fb); len(cells) != 0 {
		t.Errorf("wanted a mesh without faces to draw nothing, got %d cells", len(cells))
	}
}

// A floor's depth changes across the screen. Where a wall stands on it, the part of
// the floor behind the wall must be hidden and the part in front must not be.
func TestDrawShadedDepthVariesAcrossAFace(t *testing.T) {
	floor := Mesh{
		Vertices: []geometry.Vector3{
			{X: -5, Y: -1.5, Z: -5}, {X: -5, Y: -1.5, Z: 10}, {X: 5, Y: -1.5, Z: 10}, {X: 5, Y: -1.5, Z: -5},
		},
		Faces: [][3]int{{0, 1, 2}, {0, 2, 3}}, // faces up
	}
	wall := Mesh{
		Vertices: []geometry.Vector3{
			{X: -1, Y: -1.5, Z: 5}, {X: -1, Y: 0.5, Z: 5}, {X: 1, Y: 0.5, Z: 5}, {X: 1, Y: -1.5, Z: 5},
		},
		Faces: [][3]int{{0, 1, 2}, {0, 2, 3}}, // faces the camera
	}
	// Lights the wall harder than the floor, so their characters differ.
	light := sunAndFill(geometry.Vector3{Y: -1, Z: 2}, 0)
	wallChar := shadeChar(defaultRampRunes, light.Brightness(geometry.Vector3{}, geometry.Vector3{Z: -1}))
	floorChar := shadeChar(defaultRampRunes, light.Brightness(geometry.Vector3{}, geometry.Vector3{Y: 1}))
	if wallChar == floorChar {
		t.Fatalf("test setup: wall and floor need different shades, both %q", wallChar)
	}

	for _, floorFirst := range []bool{true, false} {
		fb := newShadeBuffer(t)
		if floorFirst {
			floor.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), light)
			wall.withColor(engine.Palette(2)).DrawShaded(fb, shadeCam(), light)
		} else {
			wall.withColor(engine.Palette(2)).DrawShaded(fb, shadeCam(), light)
			floor.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), light)
		}

		// Row 14 on the wall's face: the floor there lies at z of about 7, behind the wall.
		if got := fb.At(shadeW/2, 14).Character; got != wallChar {
			t.Errorf("floorFirst=%v: wanted the wall %q at (40,14), got %q", floorFirst, wallChar, got)
		}
		// Row 18 is below the wall's base: the floor there is closer than the wall.
		if got := fb.At(shadeW/2, 18).Character; got != floorChar {
			t.Errorf("floorFirst=%v: wanted the floor %q at (40,18), got %q", floorFirst, floorChar, got)
		}
	}
}

func BenchmarkDrawShaded(b *testing.B) {
	for _, size := range []struct{ w, h int }{{80, 24}, {200, 60}} {
		b.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(b *testing.B) {
			fb, err := engine.NewFrameBuffer(size.w, size.h)
			if err != nil {
				b.Fatalf("unable to initialize frame buffer: %v", err)
			}
			cam := shadeCam()
			light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)

			// Six cubes at mixed distances and turns, one close enough to fill the view.
			var cubes []Mesh
			for i, z := range []float64{1.5, 3, 5, 7, 9, 12} {
				c := Cube(2)
				c.Transform.Position = geometry.Vector3{X: float64(i%3-1) * 1.5, Z: z}
				c.Transform.Yaw, c.Transform.Pitch = float64(i)*0.5, float64(i)*0.3
				cubes = append(cubes, c)
			}

			b.ReportAllocs()
			for b.Loop() {
				fb.Clear()
				for _, c := range cubes {
					c.withColor(engine.Palette(1)).DrawShaded(fb, cam, light)
				}
			}
		})
	}
}

// ---- pixel drawing ----

func newShadeCanvas(t *testing.T) (*engine.FrameBuffer, *engine.Canvas) {
	t.Helper()
	// The canvas's Seamless default depends on the terminal running the tests.
	t.Setenv("TERM_PROGRAM", "")
	fb := newShadeBuffer(t)
	return fb, fb.Canvas()
}

// pixelsDrawn lists the pixel coordinates that are not empty.
func pixelsDrawn(c *engine.Canvas) [][2]int {
	var out [][2]int
	for y := range c.Height() {
		for x := range c.Width() {
			if c.At(x, y) != (engine.Color{}) {
				out = append(out, [2]int{x, y})
			}
		}
	}
	return out
}

func distinctRows(cells [][2]int) int {
	rows := map[int]bool{}
	for _, c := range cells {
		rows[c[1]] = true
	}
	return len(rows)
}

func TestDrawShadedPixelsHasTwiceTheRows(t *testing.T) {
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 5}
	light := sunAndFill(geometry.Vector3{Z: 1}, 0)

	fb := newShadeBuffer(t)
	cube.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), light)
	_, c := newShadeCanvas(t)
	cube.withColor(engine.Palette(1)).DrawShadedPixels(c, shadeCam(), light)

	glyphRows, pixelRows := distinctRows(drawn(fb)), distinctRows(pixelsDrawn(c))
	if glyphRows == 0 {
		t.Fatalf("test setup: wanted the glyph cube drawn")
	}
	if pixelRows != 2*glyphRows {
		t.Errorf("wanted %d pixel rows for %d cell rows, got %d", 2*glyphRows, glyphRows, pixelRows)
	}

	// The width is not stretched: same columns either way.
	glyphCols, pixelCols := map[int]bool{}, map[int]bool{}
	for _, p := range drawn(fb) {
		glyphCols[p[0]] = true
	}
	for _, p := range pixelsDrawn(c) {
		pixelCols[p[0]] = true
	}
	if len(glyphCols) != len(pixelCols) {
		t.Errorf("wanted the same %d columns, got %d", len(glyphCols), len(pixelCols))
	}
}

func TestDrawShadedPixelsColorFollowsLight(t *testing.T) {
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 5}

	tests := []struct {
		name      string
		light     Light
		intensity float64
	}{
		{"light straight at the face", sunAndFill(geometry.Vector3{Z: 1}, 0), 1},
		{"light at an angle", sunAndFill(geometry.Vector3{Y: -1, Z: 1}, 0), math.Sqrt2 / 2},
		{"light from behind the face", sunAndFill(geometry.Vector3{Z: -1}, 0.25), 0.25},
		{"ambient only", sunAndFill(geometry.Vector3{}, 0.4), 0.4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := newShadeCanvas(t)
			cube.withColor(engine.Palette(208)).DrawShadedPixels(c, shadeCam(), tt.light)

			want := engine.Shade(engine.Palette(208), tt.intensity)
			pixels := pixelsDrawn(c)
			if len(pixels) == 0 {
				t.Fatalf("wanted the cube drawn")
			}
			for _, p := range pixels {
				if got := c.At(p[0], p[1]); got != want {
					t.Fatalf("wanted every pixel %v, got %v at %v", want, got, p)
				}
			}
		})
	}
}

func TestDrawShadedPixelsFacesDiffer(t *testing.T) {
	_, c := newShadeCanvas(t)
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 5}
	cube.Transform.Yaw, cube.Transform.Pitch = 0.6, 0.4
	light := sunAndFill(geometry.Vector3{X: 0.3, Y: -1, Z: 0.5}, 0.1)

	cube.withColor(engine.Palette(208)).DrawShadedPixels(c, shadeCam(), light)

	shades := map[engine.Color]bool{}
	for _, p := range pixelsDrawn(c) {
		shades[c.At(p[0], p[1])] = true
	}
	if len(shades) < 2 || len(shades) > 3 {
		t.Errorf("wanted 2 or 3 distinct shades, got %d: %v", len(shades), shades)
	}
}

func TestDrawShadedPixelsDepthOrder(t *testing.T) {
	near := Cube(2)
	near.Transform.Position = geometry.Vector3{Z: 4}
	far := Cube(2)
	far.Transform.Position = geometry.Vector3{Z: 10}
	light := sunAndFill(geometry.Vector3{Z: 1}, 0.2)

	for _, nearFirst := range []bool{true, false} {
		_, c := newShadeCanvas(t)
		if nearFirst {
			near.withColor(engine.Palette(196)).DrawShadedPixels(c, shadeCam(), light)
			far.withColor(engine.Palette(46)).DrawShadedPixels(c, shadeCam(), light)
		} else {
			far.withColor(engine.Palette(46)).DrawShadedPixels(c, shadeCam(), light)
			near.withColor(engine.Palette(196)).DrawShadedPixels(c, shadeCam(), light)
		}

		// The far cube sits entirely behind the near one on screen.
		for _, p := range pixelsDrawn(c) {
			if got := c.At(p[0], p[1]); got != engine.Shade(engine.Palette(196), 1) {
				t.Fatalf("nearFirst=%v: wanted only the near cube, got color %d at %v", nearFirst, got, p)
			}
		}
	}
}

func TestDrawShadedPixelsCullsBackFaces(t *testing.T) {
	verts := []geometry.Vector3{{X: -1, Y: -1, Z: 5}, {X: 1, Y: -1, Z: 5}, {X: 0, Y: 1, Z: 5}}
	light := sunAndFill(geometry.Vector3{Z: 1}, 0.1)

	for _, tt := range []struct {
		name string
		face [3]int
		want bool
	}{
		{"wound away from the camera", [3]int{0, 1, 2}, false},
		{"wound toward the camera", [3]int{0, 2, 1}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, c := newShadeCanvas(t)
			m := Mesh{Vertices: verts, Faces: [][3]int{tt.face}}

			m.withColor(engine.Palette(7)).DrawShadedPixels(c, shadeCam(), light)

			if got := len(pixelsDrawn(c)) > 0; got != tt.want {
				t.Errorf("wanted drawn=%v, got %v", tt.want, got)
			}
		})
	}
}

func TestDrawShadedPixelsPlacement(t *testing.T) {
	light := sunAndFill(geometry.Vector3{Z: 1}, 0)

	for _, tt := range []struct {
		name  string
		x, y  float64
		right bool
		below bool
	}{
		{"right and above", 3, 1, true, false},
		{"left and below", -3, -1, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, c := newShadeCanvas(t)
			cube := Cube(1)
			cube.Transform.Position = geometry.Vector3{X: tt.x, Y: tt.y, Z: 5}

			cube.withColor(engine.Palette(1)).DrawShadedPixels(c, shadeCam(), light)

			pixels := pixelsDrawn(c)
			if len(pixels) == 0 {
				t.Fatalf("wanted the cube drawn")
			}
			for _, p := range pixels {
				if (p[0] > shadeW/2) != tt.right {
					t.Fatalf("wanted every pixel on the right=%v side, got %v", tt.right, p)
				}
				if (p[1] > shadeH) != tt.below {
					t.Fatalf("wanted every pixel below the middle=%v, got %v", tt.below, p)
				}
			}
		})
	}
}

func TestDrawShadedPixelsBehindCamera(t *testing.T) {
	_, c := newShadeCanvas(t)
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: -5}

	cube.withColor(engine.Palette(1)).DrawShadedPixels(c, shadeCam(), sunAndFill(geometry.Vector3{Z: 1}, 0))

	if pixels := pixelsDrawn(c); len(pixels) != 0 {
		t.Errorf("wanted an empty canvas, got %d pixels", len(pixels))
	}
}

func TestDrawShadedPixelsFloorCrossingTheNearPlane(t *testing.T) {
	_, c := newShadeCanvas(t)
	floor := Mesh{
		Vertices: []geometry.Vector3{
			{X: -5, Y: -1.5, Z: -5}, {X: -5, Y: -1.5, Z: 10}, {X: 5, Y: -1.5, Z: 10}, {X: 5, Y: -1.5, Z: -5},
		},
		Faces: [][3]int{{0, 1, 2}, {0, 2, 3}},
	}

	floor.withColor(engine.Palette(1)).DrawShadedPixels(c, shadeCam(), sunAndFill(geometry.Vector3{Y: -1}, 0))

	// The far edge is at z=10: 1.5/10 below the horizon, which is pixel row 27.6.
	for y := range 27 {
		for x := range c.Width() {
			if c.At(x, y) != (engine.Color{}) {
				t.Fatalf("wanted pixel rows above the floor's far edge empty, got a pixel at (%d,%d)", x, y)
			}
		}
	}
	if c.At(shadeW/2, 28) == (engine.Color{}) {
		t.Errorf("wanted the floor to fill the view just below its far edge")
	}
	if c.At(shadeW/2, c.Height()-1) == (engine.Color{}) {
		t.Errorf("wanted the floor to reach the bottom pixel row")
	}
}

func TestDrawShadedPixelsWithoutFaces(t *testing.T) {
	_, c := newShadeCanvas(t)
	grid := Grid(10, 5)
	grid.Transform.Position = geometry.Vector3{Y: -1, Z: 5}

	grid.withColor(engine.Palette(1)).DrawShadedPixels(c, shadeCam(), sunAndFill(geometry.Vector3{Y: -1}, 0))

	if pixels := pixelsDrawn(c); len(pixels) != 0 {
		t.Errorf("wanted a mesh without faces to draw nothing, got %d pixels", len(pixels))
	}
}

// End to end: pixels composed into cells give solid background cells inside the
// face and half blocks along its top and bottom edges.
func TestDrawShadedPixelsComposes(t *testing.T) {
	fb, c := newShadeCanvas(t)
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 5}
	light := sunAndFill(geometry.Vector3{Z: 1}, 0)

	cube.withColor(engine.Palette(208)).DrawShadedPixels(c, shadeCam(), light)
	c.Compose()

	shade := engine.Shade(engine.Palette(208), 1)
	if got, want := fb.At(shadeW/2, shadeH/2), (engine.Cell{Character: ' ', Background: shade}); got != want {
		t.Errorf("wanted a solid cell in the middle of the face, got %+v", got)
	}
	if got := fb.At(2, 2); got != engine.BlankCell {
		t.Errorf("wanted untouched cells to stay blank, got %+v", got)
	}
}

// A floor's depth changes across the screen. Where a wall stands on it, the part of
// the floor behind the wall must be hidden and the part in front must not be.
func TestDrawShadedPixelsDepthVariesAcrossAFace(t *testing.T) {
	floor := Mesh{
		Vertices: []geometry.Vector3{
			{X: -5, Y: -1.5, Z: -5}, {X: -5, Y: -1.5, Z: 10}, {X: 5, Y: -1.5, Z: 10}, {X: 5, Y: -1.5, Z: -5},
		},
		Faces: [][3]int{{0, 1, 2}, {0, 2, 3}},
	}
	wall := Mesh{
		Vertices: []geometry.Vector3{
			{X: -1, Y: -1.5, Z: 5}, {X: -1, Y: 0.5, Z: 5}, {X: 1, Y: 0.5, Z: 5}, {X: 1, Y: -1.5, Z: 5},
		},
		Faces: [][3]int{{0, 1, 2}, {0, 2, 3}},
	}
	light := sunAndFill(geometry.Vector3{Y: -1, Z: 2}, 0)
	wallColor := engine.Shade(engine.Palette(196), light.Brightness(geometry.Vector3{}, geometry.Vector3{Z: -1}))
	floorColor := engine.Shade(engine.Palette(46), light.Brightness(geometry.Vector3{}, geometry.Vector3{Y: 1}))

	for _, floorFirst := range []bool{true, false} {
		_, c := newShadeCanvas(t)
		if floorFirst {
			floor.withColor(engine.Palette(46)).DrawShadedPixels(c, shadeCam(), light)
			wall.withColor(engine.Palette(196)).DrawShadedPixels(c, shadeCam(), light)
		} else {
			wall.withColor(engine.Palette(196)).DrawShadedPixels(c, shadeCam(), light)
			floor.withColor(engine.Palette(46)).DrawShadedPixels(c, shadeCam(), light)
		}

		// Pixel row 29 is on the wall's face; the floor there lies at about z=6.5, behind it.
		if got := c.At(shadeW/2, 29); got != wallColor {
			t.Errorf("floorFirst=%v: wanted the wall (%v) at (40,29), got %v", floorFirst, wallColor, got)
		}
		// Pixel row 36 is below the wall's base; the floor there is at about z=2.9, in front.
		if got := c.At(shadeW/2, 36); got != floorColor {
			t.Errorf("floorFirst=%v: wanted the floor (%v) at (40,36), got %v", floorFirst, floorColor, got)
		}
	}
}

func centroid(cells [][2]int) (x, y float64) {
	for _, c := range cells {
		x += float64(c[0])
		y += float64(c[1])
	}
	n := float64(len(cells))
	return x / n, y / n
}

// A tilted camera must still see what it points at, in every drawing mode.
func TestDrawWithPitchedCamera(t *testing.T) {
	light := sunAndFill(geometry.Vector3{X: 0.3, Y: -1, Z: 0.5}, 0.2)

	for _, pitch := range []float64{-0.8, 0.8} {
		cam := Camera{Position: geometry.Vector3{Y: 1}, Pitch: pitch, FOV: math.Pi / 2, Near: 0.1}

		aimed := Cube(2)
		aimed.Transform.Position = cam.Position.Add(cam.Look().Scale(6))
		level := Cube(2)
		level.Transform.Position = cam.Position.Add(geometry.Vector3{Z: 6})

		draws := map[string]func(m Mesh) [][2]int{
			"wireframe": func(m Mesh) [][2]int {
				fb := newShadeBuffer(t)
				m.Draw(fb, cam, engine.Cell{Character: '#'})
				return drawn(fb)
			},
			"glyph shaded": func(m Mesh) [][2]int {
				fb := newShadeBuffer(t)
				m.withColor(engine.Palette(1)).DrawShaded(fb, cam, light)
				return drawn(fb)
			},
			"pixel shaded": func(m Mesh) [][2]int {
				_, c := newShadeCanvas(t)
				m.withColor(engine.Palette(1)).DrawShadedPixels(c, cam, light)
				// Pixel rows are half cells; fold them back to cell rows for the comparison.
				var cells [][2]int
				for _, p := range pixelsDrawn(c) {
					cells = append(cells, [2]int{p[0], p[1] / 2})
				}
				return cells
			},
		}

		for name, draw := range draws {
			// What the camera points at lands in the middle of the screen.
			cells := draw(aimed)
			if len(cells) == 0 {
				t.Fatalf("pitch %v, %s: wanted the aimed cube drawn", pitch, name)
			}
			cx, cy := centroid(cells)
			if math.Abs(cx-shadeW/2) > 2 || math.Abs(cy-shadeH/2) > 2 {
				t.Errorf("pitch %v, %s: wanted the aimed cube near the center, got (%.1f,%.1f)", pitch, name, cx, cy)
			}

			// What is straight ahead on the level slides the opposite way: down when the
			// camera looks up, up when it looks down.
			cells = draw(level)
			if len(cells) == 0 {
				continue // pushed off screen, which is also the right direction
			}
			_, cy = centroid(cells)
			if (pitch > 0) != (cy > shadeH/2) {
				t.Errorf("pitch %v, %s: wanted the level cube to move away from the look direction, got y=%.1f", pitch, name, cy)
			}
		}
	}
}

// ---- double sided ----

// insideOut returns the cube with every face wound the other way round.
func insideOut(m Mesh) Mesh {
	out := m
	out.Faces = make([][3]int, len(m.Faces))
	for i, f := range m.Faces {
		out.Faces[i] = [3]int{f[0], f[2], f[1]}
	}
	return out
}

func sameFrame(a, b *engine.FrameBuffer) bool {
	for y := range a.Height() {
		for x := range a.Width() {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}

func samePixels(a, b *engine.Canvas) bool {
	for y := range a.Height() {
		for x := range a.Width() {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}

func posedCube(doubleSided bool) Mesh {
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{X: 0.3, Y: 0.2, Z: 5}
	cube.Transform.Yaw, cube.Transform.Pitch = 0.6, 0.4
	cube.DoubleSided = doubleSided
	return cube
}

var doubleSidedLight = sunAndFill(geometry.Vector3{X: 0.3, Y: -1, Z: 0.5}, 0.1)

// Without DoubleSided an inside-out solid shows the wrong faces: the near ones are
// culled and the far ones, whose back is now their front, show through. Its outline
// is the same but the shading is not, so it must not look like the plain solid.
func TestInsideOutMeshLooksWrongUnlessDoubleSided(t *testing.T) {
	plain := newShadeBuffer(t)
	posedCube(false).withColor(engine.Palette(1)).DrawShaded(plain, shadeCam(), doubleSidedLight)

	flipped := newShadeBuffer(t)
	insideOut(posedCube(false)).withColor(engine.Palette(1)).DrawShaded(flipped, shadeCam(), doubleSidedLight)
	if sameFrame(flipped, plain) {
		t.Errorf("wanted an inside-out mesh to look different from the plain one without DoubleSided")
	}

	_, plainPixels := newShadeCanvas(t)
	posedCube(false).withColor(engine.Palette(1)).DrawShadedPixels(plainPixels, shadeCam(), doubleSidedLight)
	_, flippedPixels := newShadeCanvas(t)
	insideOut(posedCube(false)).withColor(engine.Palette(1)).DrawShadedPixels(flippedPixels, shadeCam(), doubleSidedLight)
	if samePixels(flippedPixels, plainPixels) {
		t.Errorf("wanted an inside-out mesh to look different in pixels too")
	}
}

// Seen from outside, an inside-out solid with both sides drawn looks exactly like the
// right way round: its near faces are lit as the camera sees them, and the far ones
// are hidden behind them.
func TestDoubleSidedInsideOutLooksLikeTheOriginal(t *testing.T) {
	want := newShadeBuffer(t)
	posedCube(false).withColor(engine.Palette(1)).DrawShaded(want, shadeCam(), doubleSidedLight)
	if len(drawn(want)) == 0 {
		t.Fatalf("test setup: wanted the plain cube drawn")
	}

	got := newShadeBuffer(t)
	insideOut(posedCube(true)).withColor(engine.Palette(1)).DrawShaded(got, shadeCam(), doubleSidedLight)
	if !sameFrame(got, want) {
		t.Errorf("wanted the double-sided inside-out cube to match the plain one")
	}

	_, wantPixels := newShadeCanvas(t)
	posedCube(false).withColor(engine.Palette(1)).DrawShadedPixels(wantPixels, shadeCam(), doubleSidedLight)
	_, gotPixels := newShadeCanvas(t)
	insideOut(posedCube(true)).withColor(engine.Palette(1)).DrawShadedPixels(gotPixels, shadeCam(), doubleSidedLight)
	if !samePixels(gotPixels, wantPixels) {
		t.Errorf("wanted the double-sided inside-out cube to match the plain one in pixels")
	}
}

// Turning DoubleSided on for a mesh that is already right must change nothing.
func TestDoubleSidedChangesNothingForAClosedOutwardMesh(t *testing.T) {
	plain, both := newShadeBuffer(t), newShadeBuffer(t)
	posedCube(false).withColor(engine.Palette(1)).DrawShaded(plain, shadeCam(), doubleSidedLight)
	posedCube(true).withColor(engine.Palette(1)).DrawShaded(both, shadeCam(), doubleSidedLight)
	if !sameFrame(plain, both) {
		t.Errorf("wanted DoubleSided to leave a closed outward mesh unchanged")
	}

	_, plainPixels := newShadeCanvas(t)
	_, bothPixels := newShadeCanvas(t)
	posedCube(false).withColor(engine.Palette(1)).DrawShadedPixels(plainPixels, shadeCam(), doubleSidedLight)
	posedCube(true).withColor(engine.Palette(1)).DrawShadedPixels(bothPixels, shadeCam(), doubleSidedLight)
	if !samePixels(plainPixels, bothPixels) {
		t.Errorf("wanted DoubleSided to leave a closed outward mesh unchanged in pixels")
	}
}

// A single sheet: seen from either side it is lit as the side being looked at.
func TestDoubleSidedSheetIsLitFromTheSideYouSee(t *testing.T) {
	verts := []geometry.Vector3{{X: -1, Y: -1, Z: 5}, {X: 1, Y: -1, Z: 5}, {X: 0, Y: 1, Z: 5}}
	// The sheet faces the camera at the origin when wound 0,2,1, and faces away when wound 0,1,2.
	towards := Mesh{Vertices: verts, Faces: [][3]int{{0, 2, 1}}}
	away := Mesh{Vertices: verts, Faces: [][3]int{{0, 1, 2}}, DoubleSided: true}
	awayCulled := Mesh{Vertices: verts, Faces: [][3]int{{0, 1, 2}}}

	// The light travels along +z: it hits the side facing the camera at full strength.
	light := sunAndFill(geometry.Vector3{Z: 1}, 0)

	front := newShadeBuffer(t)
	towards.withColor(engine.Palette(1)).DrawShaded(front, shadeCam(), light)
	back := newShadeBuffer(t)
	away.withColor(engine.Palette(1)).DrawShaded(back, shadeCam(), light)
	culled := newShadeBuffer(t)
	awayCulled.withColor(engine.Palette(1)).DrawShaded(culled, shadeCam(), light)

	if len(drawn(front)) == 0 {
		t.Fatalf("test setup: wanted the sheet drawn from the front")
	}
	if !sameFrame(front, back) {
		t.Errorf("wanted the back of the sheet lit like the front")
	}
	if len(drawn(culled)) != 0 {
		t.Errorf("wanted the back culled without DoubleSided")
	}

	// And with the light on the far side, the near side is dark (ambient only).
	dark := sunAndFill(geometry.Vector3{Z: -1}, 0.2)
	lit := newShadeBuffer(t)
	away.withColor(engine.Palette(1)).DrawShaded(lit, shadeCam(), dark)
	cells := drawn(lit)
	if len(cells) == 0 {
		t.Fatalf("wanted the sheet drawn")
	}
	if got, want := lit.At(cells[0][0], cells[0][1]).Character, shadeChar(defaultRampRunes, 0.2); got != want {
		t.Errorf("wanted the unlit side drawn with ambient shade %q, got %q", want, got)
	}
}

func TestDoubleSidedDoesNotAffectWireframes(t *testing.T) {
	plain, both := newShadeBuffer(t), newShadeBuffer(t)
	posedCube(false).Draw(plain, shadeCam(), engine.Cell{Character: '#'})
	posedCube(true).Draw(both, shadeCam(), engine.Cell{Character: '#'})

	if len(drawn(plain)) == 0 {
		t.Fatalf("test setup: wanted a wireframe drawn")
	}
	if !sameFrame(plain, both) {
		t.Errorf("wanted wireframes unchanged by DoubleSided")
	}
}

func TestDoubleSidedSurvivesFit(t *testing.T) {
	m := Cube(3)
	m.DoubleSided = true

	if !m.Fit(2).DoubleSided {
		t.Errorf("wanted Fit to keep DoubleSided")
	}
}

// An RGB color works anywhere a palette color does: cells carry it as it is, and
// pixels carry it scaled by the light.
func TestDrawShadedAcceptsRGBColors(t *testing.T) {
	color := engine.RGB(200, 100, 50)
	light := sunAndFill(geometry.Vector3{Z: 1}, 0) // straight into the front face
	cube := Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 5}

	fb := newShadeBuffer(t)
	cube.withColor(color).DrawShaded(fb, shadeCam(), light)
	cells := drawn(fb)
	if len(cells) == 0 {
		t.Fatalf("wanted the cube drawn")
	}
	for _, c := range cells {
		if got := fb.At(c[0], c[1]).Foreground; got != color {
			t.Fatalf("wanted the RGB color kept on cells, got %v at %v", got, c)
		}
	}

	_, canvas := newShadeCanvas(t)
	cube.withColor(color).DrawShadedPixels(canvas, shadeCam(), light)
	want := engine.Shade(color, 1)
	if want != color {
		t.Fatalf("wanted full light to leave an RGB color as it is, got %v", want)
	}
	pixels := pixelsDrawn(canvas)
	if len(pixels) == 0 {
		t.Fatalf("wanted pixels drawn")
	}
	for _, p := range pixels {
		if got := canvas.At(p[0], p[1]); got != want {
			t.Fatalf("wanted every pixel %v, got %v at %v", want, got, p)
		}
	}

	dim := sunAndFill(geometry.Vector3{Z: -1}, 0.5) // light from behind: ambient only
	_, canvas = newShadeCanvas(t)
	cube.withColor(color).DrawShadedPixels(canvas, shadeCam(), dim)
	for _, p := range pixelsDrawn(canvas) {
		if got, want := canvas.At(p[0], p[1]), engine.RGB(100, 50, 25); got != want {
			t.Fatalf("wanted the RGB color scaled by the ambient light to %v, got %v at %v", want, got, p)
		}
	}
}

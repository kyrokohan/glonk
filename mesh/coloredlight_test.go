package mesh

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// plainLight is a light of a program's own with no LightColor, which is white.
type plainLight struct{ level float64 }

func (p plainLight) Brightness(_, _ geometry.Vector3) float64 { return p.level }

var (
	lightRed   = engine.RGB(255, 0, 0)
	lightBlue  = engine.RGB(0, 0, 255)
	lightWhite = engine.RGB(255, 255, 255)
)

// White lights, given as the zero Color, as white, or by a light with no LightColor, light every
// channel as Brightness lights the whole, exactly.
func TestBrightnessRGBOfWhiteLightsIsBrightness(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	for range 2000 {
		white := []engine.Color{{}, lightWhite}[rng.IntN(2)]
		var lights []Light
		for range 1 + rng.IntN(4) {
			switch rng.IntN(4) {
			case 0:
				lights = append(lights, Ambient{Level: rng.Float64(), Color: white})
			case 1:
				lights = append(lights, Directional{Direction: v3(rng.Float64()-0.5, -1, rng.Float64()-0.5), Intensity: rng.Float64(), Color: white})
			case 2:
				lights = append(lights, Point{Position: v3(rng.Float64()*4, 2, rng.Float64()*4), Range: 10, Color: white})
			default:
				lights = append(lights, plainLight{rng.Float64()})
			}
		}
		point, normal := v3(rng.Float64(), rng.Float64(), rng.Float64()), v3(rng.Float64()-0.5, 1, rng.Float64()-0.5).Normalize()
		want := Brightness(lights, point, normal)
		if r, g, b := BrightnessRGB(lights, point, normal); r != want || g != want || b != want {
			t.Fatalf("lights %v: wanted every channel %v, got %v %v %v", lights, want, r, g, b)
		}
	}
}

// A colored light lights only its own channels, by how much of each it has, and the channels add
// up apart, each by the rule Brightness adds up by.
func TestBrightnessRGBLightsEachChannel(t *testing.T) {
	p, n := v3(0, 0, 0), v3(0, 1, 0)
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-12 }
	for _, tt := range []struct {
		name    string
		lights  []Light
		r, g, b float64
	}{
		{"a red light", []Light{Ambient{Level: 0.5, Color: lightRed}}, 0.5, 0, 0},
		{"red and blue", []Light{Ambient{Level: 1, Color: lightRed}, Ambient{Level: 0.4, Color: lightBlue}}, 1, 0, 0.4},
		{"two reds add up", []Light{Ambient{Level: 0.5, Color: lightRed}, Ambient{Level: 0.5, Color: lightRed}}, 0.75, 0, 0},
		{"orange", []Light{Ambient{Level: 1, Color: engine.RGB(255, 102, 0)}}, 1, 0.4, 0},
		{"a palette color", []Light{Ambient{Level: 1, Color: engine.Palette(196)}}, 1, 0, 0},
		{"red over white", []Light{Ambient{Level: 0.5}, Ambient{Level: 0.5, Color: lightRed}}, 0.75, 0.5, 0.5},
		{"a nil light", []Light{nil, Ambient{Level: 0.5, Color: lightBlue}}, 0, 0, 0.5},
		{"no light", nil, 0, 0, 0},
	} {
		r, g, b := BrightnessRGB(tt.lights, p, n)
		if !near(r, tt.r) || !near(g, tt.g) || !near(b, tt.b) {
			t.Errorf("%s: wanted %v %v %v, got %v %v %v", tt.name, tt.r, tt.g, tt.b, r, g, b)
		}
	}
}

// litWall is a wall square to a camera at the origin, 3 away, in color, without a picture.
func litWall(color engine.Color, normals bool) Mesh {
	m := Mesh{
		Vertices: []geometry.Vector3{{X: -2, Y: -1, Z: 3}, {X: -2, Y: 1, Z: 3}, {X: 2, Y: 1, Z: 3}, {X: 2, Y: -1, Z: 3}},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		Material: Material{Color: color},
	}
	if normals {
		back := v3(0, 0, -1)
		m.Normals = []geometry.Vector3{back, back, back, back}
	}
	return m
}

// wallSeen draws m in pixels and in cells under lights and returns the middle pixel and cell.
func wallSeen(t *testing.T, m Mesh, lights ...Light) (engine.Color, engine.Cell) {
	t.Helper()
	fb, err := engine.NewFrameBuffer(40, 12)
	if err != nil {
		t.Fatal(err)
	}
	cv := fb.Canvas()
	m.DrawShadedPixels(cv, Camera{}, lights...)
	m.DrawShaded(fb, Camera{}, lights...)
	return cv.At(20, 12), fb.At(20, 6)
}

// A white wall under a red light is red: in pixels, the color shaded channel by channel, and in
// cells, the brightest character in a red foreground, flat or smooth.
func TestAColoredLightColorsAWall(t *testing.T) {
	ramp := []rune(defaultRamp)
	for _, normals := range []bool{false, true} {
		pixel, cell := wallSeen(t, litWall(lightWhite, normals), Ambient{Level: 1, Color: lightRed})
		if pixel != lightRed {
			t.Errorf("normals %v: wanted a red pixel, got %v", normals, pixel)
		}
		if cell.Foreground != lightRed || cell.Character != ramp[len(ramp)-1] {
			t.Errorf("normals %v: wanted the brightest character in red, got %q in %v", normals, cell.Character, cell.Foreground)
		}

		// Half a red light: the pixel is half red, and the cell shows half the brightness in a
		// foreground that is red at full strength, since the brightness is in the character.
		pixel, cell = wallSeen(t, litWall(lightWhite, normals), Ambient{Level: 0.5, Color: lightRed})
		if want := engine.Shade(lightRed, 0.5); pixel != want {
			t.Errorf("normals %v: wanted %v in half a red light, got %v", normals, want, pixel)
		}
		if cell.Foreground != lightRed || cell.Character != shadeChar(ramp, 0.5) {
			t.Errorf("normals %v: wanted %q in red in half a red light, got %q in %v", normals, shadeChar(ramp, 0.5), cell.Character, cell.Foreground)
		}

		// The terminal's default color cannot be colored: in cells it stays, and the character
		// shows the light.
		if _, cell := wallSeen(t, litWall(engine.Color{}, normals), Ambient{Level: 1, Color: lightRed}); !cell.Foreground.IsDefault() || cell.Character != ramp[len(ramp)-1] {
			t.Errorf("normals %v: wanted the default color kept, got %q in %v", normals, cell.Character, cell.Foreground)
		}
	}
}

// A sphere lit red from its left and blue from its right is red on one side and blue on the other.
func TestTwoColoredLightsLightTwoSides(t *testing.T) {
	ball := Sphere(1.5, 32, 16)
	ball.Material.Color = lightWhite
	ball.Transform.Position = v3(0, 0, 5)
	lights := []Light{
		Directional{Direction: v3(1, 0, 0), Color: lightRed},   // from the left, traveling right
		Directional{Direction: v3(-1, 0, 0), Color: lightBlue}, // from the right
	}
	fb, err := engine.NewFrameBuffer(60, 20)
	if err != nil {
		t.Fatal(err)
	}
	cv := fb.Canvas()
	ball.DrawShadedPixels(cv, Camera{}, lights...)
	cam := Camera{}
	for _, side := range []struct {
		x    float64
		want string
	}{{-1.2, "red"}, {1.2, "blue"}} {
		px, py, _, ok := cam.ProjectPixels(v3(side.x, 0, 5-math.Sqrt(1.5*1.5-side.x*side.x)), cv)
		if !ok {
			t.Fatal("the side is not on the screen")
		}
		r, _, b := cv.At(int(px), int(py)).RGB()
		if (side.want == "red") != (r > b) || r == b {
			t.Errorf("the side at x %v: wanted %s, got red %d and blue %d", side.x, side.want, r, b)
		}
	}
}

// White lights draw a mesh as before, whichever way they are white: the frames are the same with
// the zero Color, with white, and with lights that have no color at all.
func TestWhiteLightsDrawAsBefore(t *testing.T) {
	ball := Sphere(1.5, 24, 12)
	ball.Material.Color = engine.RGB(200, 120, 40)
	ball.Transform.Position = v3(0.3, -0.2, 5)
	draw := func(lights ...Light) (string, []engine.Color) {
		fb, err := engine.NewFrameBuffer(50, 20)
		if err != nil {
			t.Fatal(err)
		}
		ball.DrawShaded(fb, Camera{}, lights...)
		cv := fb.Canvas()
		ball.DrawShadedPixels(cv, Camera{}, lights...)
		var pixels []engine.Color
		for y := range cv.Height() {
			for x := range cv.Width() {
				pixels = append(pixels, cv.At(x, y))
			}
		}
		return fb.String(), pixels
	}
	sun := v3(0.4, -1, 0.6)
	c1, p1 := draw(Ambient{Level: 0.2}, Directional{Direction: sun})
	c2, p2 := draw(Ambient{Level: 0.2, Color: lightWhite}, Directional{Direction: sun, Color: lightWhite})
	if c1 != c2 || !slices.Equal(p1, p2) {
		t.Errorf("wanted white given as white to draw as the zero Color does")
	}
}

// BenchmarkDrawColoredLight is a smooth sphere under a colored lamp and a colored fill, which is
// drawn cell by cell, beside the same sphere under the same lights in white: the cost of colored
// light.
func BenchmarkDrawColoredLight(b *testing.B) {
	ball := Sphere(2, 24, 12)
	ball.Material.Color = engine.RGB(220, 220, 220)
	ball.Transform.Position = v3(0, 0, 5)
	lit := func(sky, lamp engine.Color) []Light {
		return []Light{Ambient{Level: 0.2, Color: sky}, Point{Position: v3(-3, 3, 2), Range: 12, Color: lamp}}
	}
	for _, size := range [][2]int{{80, 24}, {200, 60}} {
		fb, err := engine.NewFrameBuffer(size[0], size[1])
		if err != nil {
			b.Fatal(err)
		}
		cv := fb.Canvas()
		for _, set := range []struct {
			name   string
			lights []Light
		}{
			{"colored", lit(engine.RGB(60, 80, 255), engine.RGB(255, 170, 90))},
			{"white", lit(engine.Color{}, engine.Color{})},
		} {
			at := " " + dims(size[0], 0) + "x" + dims(size[1], 0)
			b.Run(set.name+" cells"+at, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					fb.Clear()
					ball.DrawShaded(fb, Camera{}, set.lights...)
				}
			})
			b.Run(set.name+" pixels"+at, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					cv.Clear()
					ball.DrawShadedPixels(cv, Camera{}, set.lights...)
				}
			})
		}
	}
}

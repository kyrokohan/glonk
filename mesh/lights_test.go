package mesh

import (
	"math"
	"math/rand"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

var (
	down = geometry.Vector3{Y: -1}
	up   = geometry.Vector3{Y: 1}
	side = geometry.Vector3{X: 1}
	diag = geometry.Vector3{X: math.Sqrt2 / 2, Y: math.Sqrt2 / 2}
	nan  = math.NaN()
)

func TestDirectionalBrightness(t *testing.T) {
	tests := []struct {
		name   string
		light  Directional
		normal geometry.Vector3
		want   float64
	}{
		{"facing the light", Directional{Direction: down}, up, 1},
		{"turned away", Directional{Direction: down}, down, 0},
		{"edge on", Directional{Direction: down}, side, 0},
		{"at 45 degrees", Directional{Direction: down}, diag, math.Sqrt2 / 2},
		{"direction length does not matter", Directional{Direction: down.Scale(5)}, diag, math.Sqrt2 / 2},
		{"a tiny direction still points", Directional{Direction: down.Scale(1e-9)}, up, 1},
		{"zero direction gives no light", Directional{}, up, 0},
		{"intensity 0 means 1", Directional{Direction: down, Intensity: 0}, up, 1},
		{"half intensity", Directional{Direction: down, Intensity: 0.5}, up, 0.5},
		{"half intensity at 45 degrees", Directional{Direction: down, Intensity: 0.5}, diag, 0.5 * math.Sqrt2 / 2},
		{"intensity above 1 clamps", Directional{Direction: down, Intensity: 9}, up, 1},
		{"negative intensity is dark", Directional{Direction: down, Intensity: -1}, up, 0},
		{"NaN intensity is dark", Directional{Direction: down, Intensity: nan}, up, 0},
		{"NaN direction is dark", Directional{Direction: geometry.Vector3{X: nan}}, up, 0},
		{"infinite direction is dark", Directional{Direction: geometry.Vector3{Y: math.Inf(-1)}}, up, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.light.Brightness(geometry.Vector3{X: 7, Y: -3, Z: 2}, tt.normal); !floatEquals(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

func TestAmbientBrightness(t *testing.T) {
	tests := []struct {
		name  string
		level float64
		want  float64
	}{
		{"none", 0, 0},
		{"a little", 0.2, 0.2},
		{"full", 1, 1},
		{"above 1 clamps", 4, 1},
		{"negative is none", -2, 0},
		{"NaN is none", nan, 0},
		{"infinite clamps", math.Inf(1), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// It does not matter where the surface is or which way it faces.
			for _, n := range []geometry.Vector3{up, down, side} {
				if got := (Ambient{Level: tt.level}).Brightness(geometry.Vector3{Z: 9}, n); got != tt.want {
					t.Errorf("normal %v: wanted %v, got %v", n, tt.want, got)
				}
			}
		})
	}
}

// fixedLight gives the same brightness everywhere.
type fixedLight float64

func (f fixedLight) Brightness(_, _ geometry.Vector3) float64 { return float64(f) }

func TestLightsAddUpWithoutPassingFull(t *testing.T) {
	tests := []struct {
		name   string
		lights []Light
		want   float64
	}{
		{"no lights", nil, 0},
		{"one light", []Light{fixedLight(0.3)}, 0.3},
		{"two halves make three quarters", []Light{fixedLight(0.5), fixedLight(0.5)}, 0.75},
		{"three thirds", []Light{fixedLight(1.0 / 3), fixedLight(1.0 / 3), fixedLight(1.0 / 3)}, 1 - 8.0/27},
		{"a full light is full whatever else shines", []Light{fixedLight(0.2), fixedLight(1), fixedLight(0.7)}, 1},
		{"a dark light adds nothing", []Light{fixedLight(0.4), fixedLight(0)}, 0.4},
		{"a nil entry is skipped", []Light{nil, fixedLight(0.4), nil}, 0.4},
		{"above 1 counts as 1", []Light{fixedLight(7)}, 1},
		{"below 0 counts as 0", []Light{fixedLight(-3), fixedLight(0.25)}, 0.25},
		{"NaN counts as 0", []Light{fixedLight(nan), fixedLight(0.25)}, 0.25},
		{"infinity counts as 1", []Light{fixedLight(math.Inf(1))}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := brightness(tt.lights, geometry.Vector3{}, up); !floatEquals(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

func TestLightOrderDoesNotMatterAndBrightnessStaysInRange(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for range 500 {
		var lights []Light
		for range 1 + rng.Intn(5) {
			switch rng.Intn(3) {
			case 0:
				lights = append(lights, Ambient{Level: rng.Float64()*1.4 - 0.2})
			case 1:
				lights = append(lights, Directional{Direction: geometry.Vector3{X: rng.NormFloat64(), Y: rng.NormFloat64(), Z: rng.NormFloat64()}, Intensity: rng.Float64()*1.4 - 0.2})
			default:
				lights = append(lights, fixedLight(rng.Float64()))
			}
		}
		normal := geometry.Vector3{X: rng.NormFloat64(), Y: rng.NormFloat64(), Z: rng.NormFloat64()}.Normalize()

		got := brightness(lights, geometry.Vector3{}, normal)
		rng.Shuffle(len(lights), func(i, j int) { lights[i], lights[j] = lights[j], lights[i] })
		shuffled := brightness(lights, geometry.Vector3{}, normal)

		if got < 0 || got > 1 {
			t.Fatalf("wanted brightness in 0 to 1, got %v", got)
		}
		if math.Abs(got-shuffled) > 1e-12 {
			t.Fatalf("wanted the same brightness in any order, got %v and %v", got, shuffled)
		}
	}
}

// One ambient light and one directional light add up to what the single combined light
// of earlier versions gave: ambient + (1 - ambient) * facing.
func TestAmbientAndDirectionalMatchTheOldCombinedLight(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for range 2000 {
		ambient := rng.Float64()
		dir := geometry.Vector3{X: rng.NormFloat64(), Y: rng.NormFloat64(), Z: rng.NormFloat64()}
		normal := geometry.Vector3{X: rng.NormFloat64(), Y: rng.NormFloat64(), Z: rng.NormFloat64()}.Normalize()

		facing := max(0, normal.Dot(dir.Normalize().Neg()))
		want := ambient + (1-ambient)*facing

		got := brightness([]Light{Ambient{Level: ambient}, Directional{Direction: dir}}, geometry.Vector3{}, normal)

		// Equal to within rounding: where the compiler fuses multiply and add differs.
		if math.Abs(got-want) > 1e-12 {
			t.Fatalf("ambient %v, direction %v, normal %v: wanted %v, got %v", ambient, dir, normal, want, got)
		}
	}
}

// recordingLight remembers what it was asked about.
type recordingLight struct {
	point, normal geometry.Vector3
	asked         int
}

func (r *recordingLight) Brightness(point, normal geometry.Vector3) float64 {
	r.point, r.normal = point, normal
	r.asked++
	return 0.5
}

// A light of your own is asked about the middle of each face and its unit normal, in
// world space, pointing toward the viewer.
func TestACustomLightSeesTheFaceItLights(t *testing.T) {
	// A triangle two units across, 5 away, wound so that it faces the camera at the origin.
	tri := Mesh{
		Vertices: []geometry.Vector3{{X: 0, Y: 0, Z: 5}, {X: 0, Y: 3, Z: 5}, {X: 3, Y: 0, Z: 5}},
		Faces:    [][3]int{{0, 1, 2}},
	}
	tri.Transform.Position = geometry.Vector3{X: -1, Y: 1}
	rec := &recordingLight{}

	fb := newShadeBuffer(t)
	tri.DrawShaded(fb, shadeCam(), rec)

	if rec.asked != 1 {
		t.Fatalf("wanted the light asked once for one face, got %d", rec.asked)
	}
	if want := (geometry.Vector3{X: 0, Y: 2, Z: 5}); !vecEquals(rec.point, want) {
		t.Errorf("wanted the middle of the face %v in world space, got %v", want, rec.point)
	}
	if want := (geometry.Vector3{Z: -1}); !vecEquals(rec.normal, want) {
		t.Errorf("wanted the unit normal %v, got %v", want, rec.normal)
	}
}

// For a face seen from behind (DoubleSided), the normal is the side the camera sees.
func TestACustomLightSeesTheNormalOfTheSideShown(t *testing.T) {
	tri := Mesh{
		Vertices:    []geometry.Vector3{{X: 0, Y: 0, Z: 5}, {X: 3, Y: 0, Z: 5}, {X: 0, Y: 3, Z: 5}}, // wound away from the camera
		Faces:       [][3]int{{0, 1, 2}},
		DoubleSided: true,
	}
	rec := &recordingLight{}

	tri.DrawShaded(newShadeBuffer(t), shadeCam(), rec)

	if want := (geometry.Vector3{Z: -1}); rec.asked != 1 || !vecEquals(rec.normal, want) {
		t.Errorf("wanted one question with normal %v, got %d with %v", want, rec.asked, rec.normal)
	}
}

func TestDrawShadedWithNoLightsIsAsDarkAsCanBe(t *testing.T) {
	fb := newShadeBuffer(t)

	turnedCube().DrawShaded(fb, shadeCam())

	cells := drawn(fb)
	if len(cells) == 0 {
		t.Fatalf("wanted something drawn")
	}
	for _, c := range cells {
		if got := fb.At(c[0], c[1]).Character; got != '.' {
			t.Fatalf("wanted the darkest character '.', got %q", got)
		}
	}
}

func TestDrawShadedLightsPassedSeparatelyOrAsASlice(t *testing.T) {
	lights := []Light{Ambient{Level: 0.2}, Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}}
	a, b, c := newShadeBuffer(t), newShadeBuffer(t), newShadeBuffer(t)
	cube := turnedCube().withColor(engine.Palette(208))

	cube.DrawShaded(a, shadeCam(), lights...)
	cube.DrawShaded(b, shadeCam(), lights[0], lights[1])
	cube.DrawShaded(c, shadeCam(), lights[1], lights[0])

	if picture(a) != picture(b) || picture(a) != picture(c) {
		t.Errorf("wanted the same picture however the lights are passed")
	}
	if len(drawn(a)) == 0 {
		t.Errorf("wanted something drawn")
	}
}

// Each light adds to what is shown: a second light never makes a face darker, and here
// makes some brighter.
func TestASecondLightBrightensTheFaces(t *testing.T) {
	cube := turnedCube()
	lone, both := newShadeBuffer(t), newShadeBuffer(t)
	sun := Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}

	cube.DrawShaded(lone, shadeCam(), sun)
	cube.DrawShaded(both, shadeCam(), sun, Ambient{Level: 0.3})

	ramp := string(defaultRampRunes)
	brighter := 0
	for _, c := range drawn(lone) {
		before := indexOfRune(ramp, lone.At(c[0], c[1]).Character)
		after := indexOfRune(ramp, both.At(c[0], c[1]).Character)
		if after < before {
			t.Fatalf("wanted no cell darker with a second light, but %v went from %d to %d", c, before, after)
		}
		if after > before {
			brighter++
		}
	}
	if brighter == 0 {
		t.Errorf("wanted some cells brighter with the ambient light added")
	}
}

func indexOfRune(s string, r rune) int {
	for i, c := range []rune(s) {
		if c == r {
			return i
		}
	}
	return -1
}

func TestDrawShadedPixelsTakesSeveralLights(t *testing.T) {
	_, dim := newShadeCanvas(t)
	_, lit := newShadeCanvas(t)
	cube := turnedCube().withColor(engine.Palette(208))

	cube.DrawShadedPixels(dim, shadeCam(), Ambient{Level: 0.1})
	cube.DrawShadedPixels(lit, shadeCam(), Ambient{Level: 0.1}, Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}})

	same := true
	for y := range dim.Height() {
		for x := range dim.Width() {
			if dim.At(x, y) != lit.At(x, y) {
				same = false
			}
		}
	}
	if same || len(pixelsDrawn(lit)) == 0 {
		t.Errorf("wanted the second light to change the pixels (drawn: %d)", len(pixelsDrawn(lit)))
	}
}

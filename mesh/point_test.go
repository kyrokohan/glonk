package mesh

import (
	"math"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

var _ Light = Point{}

func TestPointBrightness(t *testing.T) {
	up := v3(0, 1, 0)
	origin := v3(0, 0, 0)
	inf, nan := math.Inf(1), math.NaN()

	tests := []struct {
		name          string
		lamp          Point
		point, normal geometry.Vector3
		want          float64
	}{
		// Intensity.
		{"straight on", Point{Position: v3(0, 2, 0)}, origin, up, 1},
		{"intensity 0 means 1", Point{Position: v3(0, 2, 0), Intensity: 0}, origin, up, 1},
		{"half intensity", Point{Position: v3(0, 2, 0), Intensity: 0.5}, origin, up, 0.5},
		{"intensity over 1 is 1", Point{Position: v3(0, 2, 0), Intensity: 3}, origin, up, 1},
		{"intensity over 1 is 1, not more, when oblique", Point{Position: v3(1, 1, 0), Intensity: 3}, origin, up, math.Sqrt2 / 2},
		{"negative intensity is dark", Point{Position: v3(0, 2, 0), Intensity: -1}, origin, up, 0},
		{"NaN intensity is dark", Point{Position: v3(0, 2, 0), Intensity: nan}, origin, up, 0},

		// Facing the lamp.
		{"at 45 degrees", Point{Position: v3(1, 1, 0)}, origin, up, math.Sqrt2 / 2},
		{"at 60 degrees", Point{Position: v3(math.Sqrt(3), 1, 0)}, origin, up, 0.5},
		{"edge-on", Point{Position: v3(5, 0, 0)}, origin, up, 0},
		{"turned away", Point{Position: v3(0, 2, 0)}, origin, v3(0, -1, 0), 0},
		{"the lamp is behind the surface", Point{Position: v3(0, -2, 0)}, origin, up, 0},
		{"turned a little away", Point{Position: v3(1, -1, 0)}, origin, up, 0},

		// Where the surface is: the light is asked about the place, not the lamp.
		{"beside the lamp", Point{Position: v3(0, 2, 0)}, v3(2, 0, 0), up, math.Sqrt2 / 2},
		{"farther along", Point{Position: v3(0, 2, 0)}, v3(2, 0, 2), up, 2 / math.Sqrt(12)},
		{"the surface is lit from where it is", Point{Position: v3(10, 2, -4)}, v3(10, 0, -4), up, 1},
		{"the place is in the world, not at the origin", Point{Position: v3(10, 2, -4)}, origin, up, 2 / math.Sqrt(120)},

		// Range: no fading, then fading to nothing.
		{"no range, far away", Point{Position: v3(0, 1000, 0)}, origin, up, 1},
		{"infinite range", Point{Position: v3(0, 1000, 0), Range: inf}, origin, up, 1},
		{"at the lamp", Point{Position: v3(0, 0, 0), Range: 10}, origin, up, 1},
		{"a fifth of the way", Point{Position: v3(0, 2, 0), Range: 10}, origin, up, 0.64},
		{"halfway", Point{Position: v3(0, 5, 0), Range: 10}, origin, up, 0.25},
		{"nine tenths of the way", Point{Position: v3(0, 9, 0), Range: 10}, origin, up, 0.01},
		{"at the range", Point{Position: v3(0, 10, 0), Range: 10}, origin, up, 0},
		{"past the range", Point{Position: v3(0, 15, 0), Range: 10}, origin, up, 0},
		{"far past the range", Point{Position: v3(0, 1e6, 0), Range: 10}, origin, up, 0},
		{"range is a distance, whatever the direction", Point{Position: v3(3, 4, 0), Range: 10}, origin, v3(0.6, 0.8, 0), 0.25},
		{"fading and facing and intensity multiply", Point{Position: v3(3, 4, 0), Range: 10, Intensity: 0.8}, origin, up, 0.8 * 0.8 * 0.25},
		{"negative range is dark", Point{Position: v3(0, 2, 0), Range: -10}, origin, up, 0},
		{"NaN range is dark", Point{Position: v3(0, 2, 0), Range: nan}, origin, up, 0},

		// Nonsense is dark, not a crash.
		{"the lamp is nowhere", Point{Position: v3(nan, 0, 0)}, origin, up, 0},
		{"the lamp is infinitely far", Point{Position: v3(0, inf, 0)}, origin, up, 0},
		{"the lamp is infinitely far, with a range", Point{Position: v3(0, inf, 0), Range: 5}, origin, up, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := brightness([]Light{tt.lamp}, tt.point, tt.normal)
			// The light itself also stays from 0 to 1 (or NaN, which counts as 0), so that
			// code which calls it directly need not clamp.
			direct := tt.lamp.Brightness(tt.point, tt.normal)
			if math.IsNaN(direct) {
				direct = 0
			}

			if math.Abs(got-tt.want) > 1e-12 || math.Abs(direct-tt.want) > 1e-12 {
				t.Errorf("wanted %v, got %v (%v from the light itself)", tt.want, got, direct)
			}
		})
	}
}

// A lamp adds to the other lights as they add to each other.
func TestPointAddsUpWithOtherLights(t *testing.T) {
	lamp := Point{Position: v3(0, 5, 0), Range: 10} // 0.25 at the origin
	ambient := Ambient{Level: 0.2}

	got := brightness([]Light{lamp, ambient}, v3(0, 0, 0), v3(0, 1, 0))
	other := brightness([]Light{ambient, lamp}, v3(0, 0, 0), v3(0, 1, 0))

	if want := 0.25 + (1-0.25)*0.2; math.Abs(got-want) > 1e-12 || got != other {
		t.Errorf("wanted %v whichever way round, got %v and %v", want, got, other)
	}
}

// brightestAt returns the cell with the brightest character in a frame, by the ramp,
// and the characters' levels along the row of cells through it.
func brightestAt(fb *engine.FrameBuffer, ramp string) (x, y int) {
	best := -1
	for cy := range fb.Height() {
		for cx := range fb.Width() {
			c := fb.At(cx, cy)
			if c == engine.BlankCell {
				continue
			}
			if level := indexOfRune(ramp, c.Character); level > best {
				best, x, y = level, cx, cy
			}
		}
	}
	return x, y
}

// lookingDown is a camera high above the origin looking straight down.
func lookingDown() Camera {
	return Camera{Position: geometry.Vector3{Y: 8}, Pitch: -math.Pi / 2, FOV: math.Pi / 2, Near: 0.1}
}

// A lamp over a plane makes a pool of light that is brightest right below it, fades
// outward in every direction, and moves with the lamp. The same plane without Normals
// is lit from its middle and has no pool.
func TestPointLightMakesAPoolOnAPlane(t *testing.T) {
	floor := Plane(12, 12)
	lamp := func(x float64) Light { return Point{Position: v3(x, 1.5, 0), Range: 9} }

	centre := newShadeBuffer(t)
	floor.DrawShaded(centre, lookingDown(), lamp(0))

	// The brightest cell is below the lamp, which is in the middle of the picture.
	x, y := brightestAt(centre, defaultRamp)
	if math.Abs(float64(x-shadeW/2)) > 2 || math.Abs(float64(y-shadeH/2)) > 2 {
		t.Errorf("wanted the brightest cell near the middle (%d,%d), got (%d,%d)\n%s", shadeW/2, shadeH/2, x, y, centre.String())
	}
	// Along the row through it, the light only fades as it goes out.
	for _, dir := range []int{-1, 1} {
		prev := indexOfRune(defaultRamp, centre.At(x, y).Character)
		for cx := x + dir; cx >= 0 && cx < shadeW; cx += dir {
			c := centre.At(cx, y)
			if c == engine.BlankCell {
				break
			}
			level := indexOfRune(defaultRamp, c.Character)
			if level > prev {
				t.Fatalf("wanted the light to fade going out from the lamp, but it brightens at x=%d\n%s", cx, centre.String())
			}
			prev = level
		}
	}
	if got := distinctChars(centre); got < 5 {
		t.Errorf("wanted a pool with at least 5 shades, got %d\n%s", got, centre.String())
	}

	// Moving the lamp moves the pool the same way.
	for _, dx := range []float64{-3, 3} {
		moved := newShadeBuffer(t)
		floor.DrawShaded(moved, lookingDown(), lamp(dx))
		mx, _ := brightestAt(moved, defaultRamp)
		if (dx < 0) != (mx < x) || mx == x {
			t.Errorf("lamp moved by %v: wanted the brightest cell to move the same way from x=%d, got x=%d", dx, x, mx)
		}
	}

	// Without Normals the plane's two triangles are lit once each.
	flat := floor.Clone()
	flat.Normals = nil
	flatFB := newShadeBuffer(t)
	flat.DrawShaded(flatFB, lookingDown(), lamp(0))
	if got := distinctChars(flatFB); got > 2 {
		t.Errorf("wanted at most 2 shades on a plane lit face by face, got %d", got)
	}
}

func distinctColors(fb *engine.FrameBuffer) int {
	seen := map[engine.Color]bool{}
	for y := range fb.Height() {
		for x := range fb.Width() {
			c := fb.At(x, y)
			if c == engine.BlankCell {
				continue
			}
			seen[c.Foreground], seen[c.Background] = true, true
		}
	}
	return len(seen)
}

func TestPointLightMakesAPoolInPixelsToo(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	floor := Plane(12, 12).withColor(engine.Palette(33))
	lamp := Point{Position: v3(0, 1.5, 0), Range: 9}

	smooth, flat := newShadeBuffer(t), newShadeBuffer(t)
	canvas := smooth.Canvas()
	floor.DrawShadedPixels(canvas, lookingDown(), lamp)
	canvas.Compose()
	floor.Normals = nil
	canvas = flat.Canvas()
	floor.DrawShadedPixels(canvas, lookingDown(), lamp)
	canvas.Compose()

	if s, f := distinctColors(smooth), distinctColors(flat); s < 12 || f > 6 {
		t.Errorf("wanted a pool of many colors and a flat plane of few, got %d and %d", s, f)
	}
}

// The reach of a lamp is the edge of its pool: nothing beyond it is lit.
func TestPointLightReachesOnlyAsFarAsItsRange(t *testing.T) {
	floor := Plane(12, 12)
	lights := Point{Position: v3(0, 1, 0), Range: 3}
	fb := newShadeBuffer(t)
	floor.DrawShaded(fb, lookingDown(), lights)

	// A dark cell is the first of the ramp. At a height of 1 and a range of 3, the pool
	// reaches sqrt(3*3 - 1*1) = 2.83 out, which is a third of the picture's height from
	// its middle: far cells must be as dark as can be shown.
	dark := []rune(defaultRamp)[0]
	for _, c := range [][2]int{{shadeW/2 + 30, shadeH / 2}, {shadeW/2 - 30, shadeH / 2}, {shadeW / 2, shadeH - 1}} {
		if got := fb.At(c[0], c[1]); got != engine.BlankCell && got.Character != dark {
			t.Errorf("wanted cell %v beyond the lamp's reach to be dark, got %q", c, got.Character)
		}
	}
	if x, y := brightestAt(fb, defaultRamp); fb.At(x, y).Character == dark {
		t.Errorf("wanted the pool to be lit")
	}
}

// For any lamp, point and normal, Brightness is a number from 0 to 1 or NaN, which the
// package counts as 0: never negative, never past 1, never infinite.
func FuzzPointBrightness(f *testing.F) {
	nan, inf := math.NaN(), math.Inf(1)
	f.Add(0.0, 2.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 10.0)
	f.Add(1.0, 1.0, 1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0)
	f.Add(nan, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 1.0, 1.0)
	f.Add(inf, inf, inf, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 1.0, inf)
	f.Add(1e308, 1e308, 1e308, -1e308, 0.0, 0.0, 1e308, 1e308, 1e308, 1e308, 1e308)
	f.Add(0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, -1.0, -1.0)
	f.Add(5e-324, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 5e-324, 5e-324)

	f.Fuzz(func(t *testing.T, lx, ly, lz, px, py, pz, nx, ny, nz, intensity, rng float64) {
		lamp := Point{Position: v3(lx, ly, lz), Intensity: intensity, Range: rng}

		b := lamp.Brightness(v3(px, py, pz), v3(nx, ny, nz))

		if !math.IsNaN(b) && (b < 0 || b > 1) {
			t.Fatalf("wanted NaN or a number from 0 to 1, got %v", b)
		}
		if got := brightness([]Light{lamp}, v3(px, py, pz), v3(nx, ny, nz)); !(got >= 0 && got <= 1) {
			t.Fatalf("wanted the clamped brightness from 0 to 1, got %v", got)
		}
	})
}

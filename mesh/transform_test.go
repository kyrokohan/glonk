package mesh

import (
	"math"
	"math/rand"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// The doc comments of Transform and Camera say which way their angles turn and in
// what order they apply. This keeps those sentences true.
func TestTransformMatchesItsDocs(t *testing.T) {
	const quarter = math.Pi / 2
	pos := geometry.Vector3{X: 10, Y: 20, Z: 30}

	tests := []struct {
		name string
		t    Transform
		in   geometry.Vector3
		want geometry.Vector3
	}{
		{"positive Pitch turns +z toward +y", Transform{Pitch: quarter}, geometry.Vector3{Z: 1}, geometry.Vector3{Y: 1}},
		{"positive Yaw turns +z toward +x", Transform{Yaw: quarter}, geometry.Vector3{Z: 1}, geometry.Vector3{X: 1}},
		{"positive Roll turns +y toward +x", Transform{Roll: quarter}, geometry.Vector3{Y: 1}, geometry.Vector3{X: 1}},
		{"positive Roll puts the right side down", Transform{Roll: quarter}, geometry.Vector3{X: 1}, geometry.Vector3{Y: -1}},
		{"Roll leaves the way the model faces alone", Transform{Roll: 1}, geometry.Vector3{Z: 1}, geometry.Vector3{Z: 1}},
		{"Scale grows the model about its origin", Transform{Scale: 3}, geometry.Vector3{X: 1, Y: 2}, geometry.Vector3{X: 3, Y: 6}},
		{"Scale 0 means 1", Transform{}, geometry.Vector3{X: 1, Y: 2}, geometry.Vector3{X: 1, Y: 2}},
		{"Scale is applied before Position", Transform{Scale: 2, Position: pos}, geometry.Vector3{X: 1}, geometry.Vector3{X: 12, Y: 20, Z: 30}},
		// Roll first: (0,1,0) leans to (1,0,0), which Pitch leaves alone. Pitch first
		// would tip (0,1,0) to (0,0,-1), and Roll would then leave that alone.
		{"Roll applies before Pitch", Transform{Roll: quarter, Pitch: quarter}, geometry.Vector3{Y: 1}, geometry.Vector3{X: 1}},
		{"Position moves the model", Transform{Position: pos}, geometry.Vector3{X: 1}, geometry.Vector3{X: 11, Y: 20, Z: 30}},
		// Pitch first: (0,0,1) goes to (0,1,0), which Yaw leaves alone. Yaw first would
		// take (0,0,1) to (1,0,0), and Pitch would then leave that alone.
		{"Pitch applies before Yaw", Transform{Pitch: quarter, Yaw: quarter}, geometry.Vector3{Z: 1}, geometry.Vector3{Y: 1}},
		{"Position applies after both", Transform{Pitch: quarter, Yaw: quarter, Position: pos}, geometry.Vector3{Z: 1}, geometry.Vector3{X: 10, Y: 21, Z: 30}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.Apply(tt.in); !vecEquals(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

func TestCameraAnglesMatchTheirDocs(t *testing.T) {
	const eighth = math.Pi / 4

	if f := (Camera{Yaw: eighth}).Forward(); !(f.X > 0.5 && f.Z > 0.5) {
		t.Errorf("wanted a positive Yaw to turn the camera right, toward +x, got %v", f)
	}
	if l := (Camera{Pitch: eighth}).Look(); !(l.Y > 0.5 && l.Z > 0.5) {
		t.Errorf("wanted a positive Pitch to tilt the camera up, got %v", l)
	}
	if f := (Camera{}).Forward(); !vecEquals(f, geometry.Vector3{Z: 1}) {
		t.Errorf("wanted a camera at Yaw 0 to face +z, got %v", f)
	}
}

// A model and a camera given the same Pitch and Yaw tip and turn the same way: the
// model's front points where the camera looks.
func TestTransformAndCameraAgreeOnPitchAndYaw(t *testing.T) {
	for _, pitch := range []float64{-1.2, -0.4, 0, 0.4, 1.2} {
		for _, yaw := range []float64{-2, 0, 0.7, 3} {
			front := Transform{Pitch: pitch, Yaw: yaw}.Apply(geometry.Vector3{Z: 1})
			look := Camera{Pitch: pitch, Yaw: yaw}.Look()
			if !vecEquals(front, look) {
				t.Errorf("pitch %v yaw %v: wanted the model's front %v to match the camera's look %v", pitch, yaw, front, look)
			}
		}
	}
}

// Apply works out the rotation once and combines the model's axes; it must agree with
// turning the point one step at a time, which is what the docs describe.
func TestTransformApplyMatchesTurningStepByStep(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for range 500 {
		tr := Transform{
			Position: geometry.Vector3{X: rng.NormFloat64() * 5, Y: rng.NormFloat64() * 5, Z: rng.NormFloat64() * 5},
			Yaw:      rng.NormFloat64() * 3, Pitch: rng.NormFloat64() * 3, Roll: rng.NormFloat64() * 3,
			Scale: rng.Float64() * 4,
		}
		p := geometry.Vector3{X: rng.NormFloat64() * 3, Y: rng.NormFloat64() * 3, Z: rng.NormFloat64() * 3}
		scale := tr.Scale
		if scale == 0 {
			scale = 1
		}

		want := p.Scale(scale).RotateZ(-tr.Roll).RotateX(-tr.Pitch).RotateY(tr.Yaw).Add(tr.Position)

		if got := tr.Apply(p); !vecEquals(got, want) {
			t.Fatalf("%+v applied to %v: wanted %v, got %v", tr, p, want, got)
		}
	}
}

func TestTransformValid(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	tests := []struct {
		name string
		t    Transform
		want bool
	}{
		{"zero", Transform{}, true},
		{"everything set", Transform{Position: geometry.Vector3{X: 1, Y: -2, Z: 3}, Yaw: 1, Pitch: -1, Roll: 2, Scale: 0.5}, true},
		{"a huge scale is finite", Transform{Scale: 1e300}, true},
		{"angles are not limited", Transform{Yaw: 1000, Pitch: -1000, Roll: 1e6}, true},
		{"NaN position", Transform{Position: geometry.Vector3{Y: nan}}, false},
		{"infinite position", Transform{Position: geometry.Vector3{Z: inf}}, false},
		{"NaN yaw", Transform{Yaw: nan}, false},
		{"infinite pitch", Transform{Pitch: -inf}, false},
		{"NaN roll", Transform{Roll: nan}, false},
		{"NaN scale", Transform{Scale: nan}, false},
		{"infinite scale", Transform{Scale: inf}, false},
		{"negative scale", Transform{Scale: -1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.valid(); got != tt.want {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

// A mesh with a transform that cannot be applied draws nothing, by every route, and
// Validate says why.
func TestMeshWithAnInvalidTransformDrawsNothing(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "") // Canvas.Seamless depends on the terminal running the tests
	nan := math.NaN()
	for name, tr := range map[string]Transform{
		"NaN position":   {Position: geometry.Vector3{X: nan, Z: 5}},
		"infinite yaw":   {Position: geometry.Vector3{Z: 5}, Yaw: math.Inf(1)},
		"NaN roll":       {Position: geometry.Vector3{Z: 5}, Roll: nan},
		"NaN scale":      {Position: geometry.Vector3{Z: 5}, Scale: nan},
		"negative scale": {Position: geometry.Vector3{Z: 5}, Scale: -1},
	} {
		t.Run(name, func(t *testing.T) {
			cube := Cube(2)
			cube.Transform = tr
			fb := newShadeBuffer(t)
			canvas := fb.Canvas()

			cube.Draw(fb, shadeCam(), engine.Cell{Character: '#'})
			cube.withColor(engine.Palette(1)).DrawShaded(fb, shadeCam(), goldenSun)
			cube.withColor(engine.Palette(1)).DrawShadedPixels(canvas, shadeCam(), goldenSun)
			canvas.Compose()

			if n := len(drawn(fb)); n != 0 {
				t.Errorf("wanted nothing drawn, got %d cells", n)
			}
			if cube.Validate() == nil {
				t.Errorf("wanted Validate to report the transform")
			}
		})
	}
}

// Scale makes a model larger without changing what it is: Cube(1) at twice the scale
// is Cube(2).
func TestScaledMeshDrawsLikeALargerOne(t *testing.T) {
	small, large := Cube(1), Cube(2)
	small.Transform = Transform{Scale: 2, Position: geometry.Vector3{Z: 5}, Yaw: 0.5}
	large.Transform = Transform{Position: geometry.Vector3{Z: 5}, Yaw: 0.5}

	a, b := newShadeBuffer(t), newShadeBuffer(t)
	small.Draw(a, shadeCam(), engine.Cell{Character: '#'})
	large.Draw(b, shadeCam(), engine.Cell{Character: '#'})
	small.withColor(engine.Palette(1)).DrawShaded(a, shadeCam(), goldenSun)
	large.withColor(engine.Palette(1)).DrawShaded(b, shadeCam(), goldenSun)

	if picture(a) != picture(b) {
		t.Errorf("wanted a scaled Cube(1) to draw like Cube(2), got\n%s\nwanted\n%s", picture(a), picture(b))
	}
	if len(drawn(a)) == 0 {
		t.Errorf("wanted something drawn")
	}
}

// A quarter-turn roll swaps the width and height of a model that faces the camera.
func TestRollTurnsAFlatModelOnItsSide(t *testing.T) {
	wide := Mesh{
		Vertices: []geometry.Vector3{{X: -2, Y: -1}, {X: 2, Y: -1}, {X: 2, Y: 1}, {X: -2, Y: 1}},
		Edges:    [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}},
	}
	extent := func(m Mesh) (w, h int) {
		fb := newShadeBuffer(t)
		m.Draw(fb, shadeCam(), engine.Cell{Character: '#'})
		minX, maxX, minY, maxY := fb.Width(), -1, fb.Height(), -1
		for _, c := range drawn(fb) {
			minX, maxX = min(minX, c[0]), max(maxX, c[0])
			minY, maxY = min(minY, c[1]), max(maxY, c[1])
		}
		return maxX - minX + 1, maxY - minY + 1
	}

	wide.Transform.Position = geometry.Vector3{Z: 8}
	w0, h0 := extent(wide)
	wide.Transform.Roll = math.Pi / 2
	w1, h1 := extent(wide)

	if !(w0 > 2*h0) {
		t.Fatalf("wanted the unrolled model much wider than tall, got %dx%d", w0, h0)
	}
	if !(h1 > h0 && w1 < w0) {
		t.Errorf("wanted the rolled model taller and narrower than before, got %dx%d after %dx%d", w1, h1, w0, h0)
	}
}

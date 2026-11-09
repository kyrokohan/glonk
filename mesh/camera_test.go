package mesh

import (
	"math"
	"testing"

	"glonk.dev/geometry"
)

type cameraOutput struct {
	x, y float64
	ok   bool
}

func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestCameraProject(t *testing.T) {
	var tests = []struct {
		name  string
		yaw   float64
		point geometry.Vector3
		want  cameraOutput
	}{
		{"straight ahead", 0.00, geometry.Vector3{X: 0, Y: 0, Z: 5}, cameraOutput{40, 12, true}},
		{"behind", 0.00, geometry.Vector3{X: 0, Y: 0, Z: -5}, cameraOutput{0, 0, false}},
		{"right", 0.00, geometry.Vector3{X: 1, Y: 0, Z: 2}, cameraOutput{52, 12, true}},
		{"up", 0.00, geometry.Vector3{X: 0, Y: 1, Z: 2}, cameraOutput{40, 6, true}},
		{"turned", math.Pi / 2, geometry.Vector3{X: 5, Y: 0, Z: 0}, cameraOutput{40, 12, true}},
		{"inside near plane", 0.00, geometry.Vector3{X: 0, Y: 0, Z: 0.05}, cameraOutput{0, 0, false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Camera{
				Position: geometry.Vector3{X: 0, Y: 0, Z: 0},
				FOV:      math.Pi / 2,
				Yaw:      tt.yaw,
				Near:     0.1,
			}

			x, y, ok := c.Project(tt.point, 80, 24)
			if ok != tt.want.ok {
				t.Fatalf("wanted ok=%v, got %v", tt.want.ok, ok)
			}
			if !ok {
				return
			}

			if !floatEquals(x, tt.want.x) {
				t.Errorf("wanted x=%v, got %v", tt.want.x, x)
			}

			if !floatEquals(y, tt.want.y) {
				t.Errorf("wanted y=%v, got %v", tt.want.y, y)
			}
		})
	}
}

func vecEquals(a, b geometry.Vector3) bool {
	return floatEquals(a.X, b.X) && floatEquals(a.Y, b.Y) && floatEquals(a.Z, b.Z)
}

func TestCameraForwardRight(t *testing.T) {
	tests := []struct {
		name           string
		yaw            float64
		forward, right geometry.Vector3
	}{
		{"yaw 0", 0, geometry.Vector3{Z: 1}, geometry.Vector3{X: 1}},
		{"yaw quarter turn", math.Pi / 2, geometry.Vector3{X: 1}, geometry.Vector3{Z: -1}},
		{"yaw half turn", math.Pi, geometry.Vector3{Z: -1}, geometry.Vector3{X: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam := Camera{Yaw: tt.yaw}

			if got := cam.Forward(); !vecEquals(got, tt.forward) {
				t.Errorf("wanted Forward()=%v, got %v", tt.forward, got)
			}
			if got := cam.Right(); !vecEquals(got, tt.right) {
				t.Errorf("wanted Right()=%v, got %v", tt.right, got)
			}
		})
	}
}

// Forward and Right must agree with Project, or WASD moves the wrong way.
func TestCameraForwardRightMatchProject(t *testing.T) {
	const w, h = 80, 24

	for _, yaw := range []float64{0, 0.7, math.Pi / 2, 2.5, -1.2} {
		cam := Camera{
			Position: geometry.Vector3{X: 3, Y: 1, Z: -2},
			Yaw:      yaw,
			FOV:      math.Pi / 2,
			Near:     0.1,
		}
		ahead := cam.Position.Add(cam.Forward().Scale(5))

		x, y, ok := cam.Project(ahead, w, h)
		if !ok || !floatEquals(x, w/2) || !floatEquals(y, h/2) {
			t.Errorf("yaw %v: wanted point ahead at center (%v,%v,true), got (%v,%v,%v)", yaw, w/2, h/2, x, y, ok)
		}

		x, _, ok = cam.Project(ahead.Add(cam.Right()), w, h)
		if !ok || x <= w/2 {
			t.Errorf("yaw %v: wanted point to the right at x>%v, got x=%v ok=%v", yaw, w/2, x, ok)
		}
	}
}

func TestCameraPitchProject(t *testing.T) {
	// A point straight ahead on a camera tilted 45 degrees lands on the edge of the
	// screen: its view-space y over z is exactly -+1, which is a half screen away.
	tests := []struct {
		name  string
		pitch float64
		wantY float64
	}{
		{"level", 0, 12},
		{"tilted up, the point falls to the bottom edge", math.Pi / 4, 24},
		{"tilted down, the point rises to the top edge", -math.Pi / 4, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam := Camera{Pitch: tt.pitch, FOV: math.Pi / 2, Near: 0.1}

			x, y, ok := cam.Project(geometry.Vector3{Z: 5}, 80, 24)
			if !ok {
				t.Fatalf("wanted the point in view")
			}
			if !floatEquals(x, 40) || !floatEquals(y, tt.wantY) {
				t.Errorf("wanted (40,%v), got (%v,%v)", tt.wantY, x, y)
			}
		})
	}
}

// Tilting turns the world about the camera's own sideways axis, so sideways
// distances in view space must not change.
func TestCameraPitchLeavesViewXAlone(t *testing.T) {
	p := geometry.Vector3{X: 2, Y: 1, Z: 4}

	for _, pitch := range []float64{-1.2, -0.4, 0.6, 1.3} {
		level := Camera{}
		tilted := Camera{Pitch: pitch}

		if got, want := tilted.toView(p).X, level.toView(p).X; !floatEquals(got, want) {
			t.Errorf("pitch %v: wanted view x=%v, got %v", pitch, want, got)
		}
		if got, want := tilted.toView(p).Length(), level.toView(p).Length(); !floatEquals(got, want) {
			t.Errorf("pitch %v: wanted tilting to keep distances, got %v vs %v", pitch, got, want)
		}
	}
}

func TestCameraLook(t *testing.T) {
	tests := []struct {
		name       string
		yaw, pitch float64
		want       geometry.Vector3
	}{
		{"level", 0, 0, geometry.Vector3{Z: 1}},
		{"turned right", math.Pi / 2, 0, geometry.Vector3{X: 1}},
		{"straight up", 0, math.Pi / 2, geometry.Vector3{Y: 1}},
		{"straight down", 0, -math.Pi / 2, geometry.Vector3{Y: -1}},
		{"up and turned", math.Pi / 2, math.Pi / 4, geometry.Vector3{X: math.Sqrt2 / 2, Y: math.Sqrt2 / 2}},
		{"down and half turned", math.Pi, -math.Pi / 4, geometry.Vector3{Y: -math.Sqrt2 / 2, Z: -math.Sqrt2 / 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam := Camera{Yaw: tt.yaw, Pitch: tt.pitch}

			got := cam.Look()
			if !vecEquals(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
			if !floatEquals(got.Length(), 1) {
				t.Errorf("wanted a unit vector, got length %v", got.Length())
			}
		})
	}
}

func TestCameraForwardAndRightIgnorePitch(t *testing.T) {
	for _, pitch := range []float64{-1.2, -0.3, 0.5, 1.4} {
		level := Camera{Yaw: 0.9}
		tilted := Camera{Yaw: 0.9, Pitch: pitch}

		if tilted.Forward() != level.Forward() {
			t.Errorf("pitch %v: wanted Forward unchanged, got %v vs %v", pitch, tilted.Forward(), level.Forward())
		}
		if tilted.Right() != level.Right() {
			t.Errorf("pitch %v: wanted Right unchanged, got %v vs %v", pitch, tilted.Right(), level.Right())
		}
		if tilted.Forward().Y != 0 {
			t.Errorf("pitch %v: wanted Forward flat, got y=%v", pitch, tilted.Forward().Y)
		}
	}
}

// Look, Right and Project must agree for every posture, or the world tilts the
// wrong way when the player looks around.
func TestCameraLookMatchesProject(t *testing.T) {
	const w, h = 80, 24

	for _, yaw := range []float64{0, 0.7, math.Pi / 2, 2.5, -1.2} {
		for _, pitch := range []float64{-1.2, -0.5, 0, 0.4, 1.1} {
			cam := Camera{
				Position: geometry.Vector3{X: 3, Y: 1, Z: -2},
				Yaw:      yaw,
				Pitch:    pitch,
				FOV:      math.Pi / 2,
				Near:     0.1,
			}
			ahead := cam.Position.Add(cam.Look().Scale(5))

			x, y, ok := cam.Project(ahead, w, h)
			if !ok || !floatEquals(x, w/2) || !floatEquals(y, h/2) {
				t.Errorf("yaw %v pitch %v: wanted the look point at the center, got (%v,%v,%v)", yaw, pitch, x, y, ok)
			}

			x, _, ok = cam.Project(ahead.Add(cam.Right()), w, h)
			if !ok || x <= w/2 {
				t.Errorf("yaw %v pitch %v: wanted a point to the right at x>%v, got %v", yaw, pitch, w/2, x)
			}

			// World up stays up on screen however the camera is tilted (within +-pi/2).
			_, y, ok = cam.Project(ahead.Add(geometry.Vector3{Y: 0.5}), w, h)
			if !ok || y >= h/2 {
				t.Errorf("yaw %v pitch %v: wanted a point above at y<%v, got %v", yaw, pitch, h/2, y)
			}
		}
	}
}

func TestCellAspectScalesTheWidthOfThePicture(t *testing.T) {
	p := geometry.Vector3{X: 1, Y: 0.5, Z: 5}
	x2, y2, ok2 := Camera{CellAspect: 2}.Project(p, 80, 24)
	x3, y3, ok3 := Camera{CellAspect: 3}.Project(p, 80, 24)
	x1, y1, ok1 := Camera{CellAspect: 1}.Project(p, 80, 24)
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("wanted the point seen, got %v %v %v", ok1, ok2, ok3)
	}

	// The distance from the middle column is in proportion to the aspect, and rows
	// do not depend on it.
	if !floatEquals((x3-40)/(x2-40), 1.5) || !floatEquals((x1-40)/(x2-40), 0.5) {
		t.Errorf("wanted the offsets from the middle to be in proportion to CellAspect, got %v, %v, %v", x1-40, x2-40, x3-40)
	}
	if y1 != y2 || y2 != y3 {
		t.Errorf("wanted the row unchanged by CellAspect, got %v %v %v", y1, y2, y3)
	}
}

func TestCellAspectZeroIsTwo(t *testing.T) {
	p := geometry.Vector3{X: 1.5, Y: -0.5, Z: 4}
	zx, zy, zok := Camera{}.Project(p, 60, 20)
	tx, ty, tok := Camera{CellAspect: 2}.Project(p, 60, 20)

	if zx != tx || zy != ty || zok != tok {
		t.Errorf("wanted a CellAspect of 0 to project like 2, got (%v, %v, %v) and (%v, %v, %v)", zx, zy, zok, tx, ty, tok)
	}
}

// Other checks would catch most of these further along, but the camera itself says
// which values it cannot use.
func TestWithDefaultsRejectsNonsenseCellAspect(t *testing.T) {
	for name, aspect := range map[string]float64{
		"negative":          -2,
		"negative infinity": math.Inf(-1),
		"infinity":          math.Inf(1),
		"NaN":               math.NaN(),
	} {
		if _, ok := (Camera{CellAspect: aspect}).withDefaults(); ok {
			t.Errorf("%s: wanted the camera unusable", name)
		}
	}
}

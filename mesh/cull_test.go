package mesh

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// offscreenFor runs the culling test the way a draw does, on a w x h grid.
func offscreenFor(m Mesh, cam Camera, w, h int) bool {
	cam, _ = cam.withDefaults()
	return m.offscreen(cam.viewTransform(), cam.projector(w, h), cam.Near)
}

// With a 90 degree lens on an 80x24 grid, the edge of the view (a cell beyond the grid) is 1.7
// units to the side for every unit of distance, the cells being twice as tall as wide, and
// 1.08 up or down. A cube of size 2 reaches 1.73 from its middle.
func TestOffscreenSaysYesOnlyWhenNothingCanBeOnScreen(t *testing.T) {
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}
	at := func(x, y, z float64) Mesh {
		m := Cube(2)
		m.Transform.Position = geometry.Vector3{X: x, Y: y, Z: z}
		return m
	}
	scaled := func(m Mesh, s float64) Mesh { m.Transform.Scale = s; return m }

	tests := []struct {
		name string
		m    Mesh
		cam  Camera
		want bool
	}{
		{"in the middle", at(0, 0, 10), cam, false},
		{"at the edge", at(17, 0, 10), cam, false},
		{"far to the left", at(-40, 0, 10), cam, true},
		{"far to the right", at(40, 0, 10), cam, true},
		{"far above", at(0, 40, 10), cam, true},
		{"far below", at(0, -40, 10), cam, true},
		{"just outside on the left, but its corners reach in", at(-20, 0, 10), cam, false},
		{"outside on the left by more than it reaches", at(-22, 0, 10), cam, true},
		{"the same, but three times the size", scaled(at(-22, 0, 10), 3), cam, false},
		{"the same, but a tenth the size", scaled(at(-19, 0, 10), 0.1), cam, true},
		{"just outside above, but reaching in", at(0, 12, 10), cam, false},
		{"outside above by more than it reaches", at(0, 15, 10), cam, true},
		{"above, its reach just getting in", at(0, 13, 10), cam, false},
		{"below, its reach just getting in", at(0, -13, 10), cam, false},
		{"to the left, its reach just getting in", at(-20.5, 0, 10), cam, false},
		{"to the right, its reach just getting in", at(20.5, 0, 10), cam, false},
		{"behind the camera", at(0, 0, -10), cam, true},
		{"astride the camera", at(0, 0, 0), cam, false},
		{"a speck in front of the camera but not as far as the near plane", scaled(at(0, 0, 0.05), 0.01), cam, true},
		{"behind the near plane but reaching past it", at(0, 0, -1), cam, false},
		{"turned away", at(0, 0, 10), Camera{Yaw: math.Pi, FOV: math.Pi / 2, Near: 0.1}, true},
		{"seen from the side", at(10, 0, 0), Camera{Yaw: math.Pi / 2, FOV: math.Pi / 2, Near: 0.1}, false},
		{"a narrow lens makes the same place off screen", at(5, 0, 10), Camera{FOV: 0.3, Near: 0.1}, true},
		{"square cells see further to the side", at(-22, 0, 10), Camera{FOV: math.Pi / 2, Near: 0.1, CellAspect: 1}, false},
		{"cells four times as tall see less of the side", at(-12, 0, 10), Camera{FOV: math.Pi / 2, Near: 0.1, CellAspect: 4}, true},
		{"empty", Mesh{}, cam, false},
		{"its vertices are far from the model's origin", Mesh{Vertices: []geometry.Vector3{{X: -30}, {X: 30}}, Edges: [][2]int{{0, 1}}, Transform: Transform{Position: geometry.Vector3{X: -40, Z: 10}}}, cam, false},
		{"a vertex at infinity cannot be ruled out", Mesh{Vertices: []geometry.Vector3{{}, {X: math.Inf(1)}}, Transform: Transform{Position: geometry.Vector3{X: -500, Z: 10}}}, cam, false},
		{"a vertex that is not a number is ignored", Mesh{Vertices: []geometry.Vector3{{X: math.NaN()}, {X: 1}}, Transform: Transform{Position: geometry.Vector3{X: -500, Z: 10}}}, cam, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := offscreenFor(tt.m, tt.cam, 80, 24); got != tt.want {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

// The test follows the grid it is asked about, not a fixed one: the same place is on
// screen on a wide grid and off it on a narrow one.
func TestOffscreenFollowsTheGrid(t *testing.T) {
	m := Cube(2)
	m.Transform.Position = geometry.Vector3{X: 30, Z: 10}
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}

	if !offscreenFor(m, cam, 80, 24) {
		t.Errorf("wanted a cube far to the side to be off an 80 wide grid")
	}
	if offscreenFor(m, cam, 400, 24) {
		t.Errorf("wanted the same cube on a 400 wide grid to be possibly on screen")
	}
}

// Skipping what is off screen must change no picture: the same scenes drawn with and without
// it, in every way a mesh is drawn, are the same pictures. The scenes are spread so that
// many meshes are wholly off screen, some are kept though nothing shows, and many are in view;
// those counts are checked so that the test cannot pass by drawing nothing.
func TestCullingChangesNoPicture(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	defer func(on bool) { cullingOn = on }(cullingOn)
	rng := rand.New(rand.NewPCG(51, 52))
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)
	skipped, drawn, keptButEmpty := 0, 0, 0

	for scene := range 1000 {
		var m Mesh
		switch scene % 7 {
		case 0:
			m = Cube(0.3 + 3*rng.Float64())
		case 1:
			m = Sphere(0.3+1.5*rng.Float64(), 3+rng.IntN(12), 2+rng.IntN(8))
		case 2:
			m = Cylinder(0.3+rng.Float64(), 0.5+2*rng.Float64(), 3+rng.IntN(12))
		case 3:
			m = Cone(0.3+rng.Float64(), 0.5+2*rng.Float64(), 3+rng.IntN(12))
		case 4:
			m = Plane(1+8*rng.Float64(), 1+8*rng.Float64())
		case 5:
			m = Grid(2+14*rng.Float64(), 1+rng.IntN(8))
		default:
			m = Cube(1 + rng.Float64()).SmoothNormals() // a Cube with shared vertices, smoothed
		}
		// Half the scenes are spread widely, the rest are put where the camera is likely to see them.
		spread := geometry.Vector3{X: 60, Y: 30, Z: 50}
		centre := geometry.Vector3{X: -30, Y: -15, Z: -15}
		if scene%2 == 0 {
			spread, centre = geometry.Vector3{X: 12, Y: 6, Z: 12}, geometry.Vector3{X: -6, Y: -3, Z: 2}
		}
		m.Transform = Transform{
			Position: geometry.Vector3{X: rng.Float64()*spread.X + centre.X, Y: rng.Float64()*spread.Y + centre.Y, Z: rng.Float64()*spread.Z + centre.Z},
			Yaw:      rng.Float64() * 6, Pitch: rng.Float64() * 3, Roll: rng.Float64() * 3,
		}
		if rng.IntN(3) == 0 {
			m.Transform.Scale = []float64{0, 0.2, 1, 3, 12}[rng.IntN(5)]
		}
		m.DoubleSided = rng.IntN(3) == 0
		cam := Camera{
			Position: geometry.Vector3{X: rng.Float64()*10 - 5, Y: rng.Float64()*4 - 2, Z: rng.Float64()*10 - 5},
			Yaw:      rng.Float64() * 6.3, Pitch: rng.Float64() - 0.5,
			FOV: 0.3 + 2.5*rng.Float64(), Near: 0.05 + rng.Float64(),
		}
		if rng.IntN(4) == 0 {
			cam.CellAspect = 1 + 3*rng.Float64()
		}
		w, h := 4+rng.IntN(100), 2+rng.IntN(40)

		draw := func() [4]string {
			var out [4]string
			wire, depth, glyphs, pixels := mustBuffer(t, w, h), mustBuffer(t, w, h), mustBuffer(t, w, h), mustBuffer(t, w, h)
			m.Draw(wire, cam, wireCell)
			m.DrawDepth(depth, cam, wireCell)
			m.withColor(engine.Palette(33)).DrawShaded(glyphs, cam, light)
			canvas := pixels.Canvas()
			m.withColor(engine.Palette(33)).DrawShadedPixels(canvas, cam, light)
			canvas.Compose()
			for i, fb := range []*engine.FrameBuffer{wire, depth, glyphs, pixels} {
				out[i] = picture(fb)
			}
			return out
		}
		cullingOn = true
		with := draw()
		cullingOn = false
		without := draw()
		cullingOn = true

		for i, name := range []string{"Draw", "DrawDepth", "DrawShaded", "DrawShadedPixels"} {
			if with[i] != without[i] {
				t.Fatalf("scene %d (%dx%d): wanted %s to draw the same with and without skipping, got\n%s\nwanted\n%s", scene, w, h, name, with[i], without[i])
			}
		}

		c, _ := cam.withDefaults()
		switch off := m.offscreen(c.viewTransform(), c.projector(w, h), c.Near); {
		case off:
			skipped++
		case without[2] != picture(mustBuffer(t, w, h)):
			drawn++
		default:
			keptButEmpty++ // not skipped, though nothing of it turned out to be on screen
		}
	}
	if skipped < 200 || drawn < 150 || keptButEmpty < 30 {
		t.Errorf("wanted plenty of scenes skipped, drawn, and kept though empty, got %d, %d and %d", skipped, drawn, keptButEmpty)
	}
}

func mustBuffer(t testing.TB, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

// Whatever the mesh, camera or grid, skipping what is off screen changes nothing.
func FuzzCulling(f *testing.F) {
	nan, inf := math.NaN(), math.Inf(1)
	f.Add(0.0, 0.0, -4.0, 0.0, 0.0, 1.57, 0.1, 0.0, 0.0, 5.0, 2.0, 0.3, 0.2, uint8(40), uint8(12))
	f.Add(0.0, 0.0, 0.0, 0.0, 0.0, 1.57, 0.1, -30.0, 0.0, 10.0, 2.0, 0.0, 0.0, uint8(80), uint8(24)) // off the left
	f.Add(0.0, 0.0, 0.0, 0.0, 0.0, 1.57, 0.1, 0.0, 0.0, -10.0, 2.0, 0.0, 0.0, uint8(80), uint8(24))  // behind
	f.Add(0.0, 0.0, 0.0, 3.14, 0.0, 1.57, 0.1, 0.0, 0.0, 10.0, 2.0, 0.0, 0.0, uint8(80), uint8(24))  // turned away
	f.Add(0.0, 0.0, 0.0, 0.0, 0.0, 1.57, 0.5, 0.0, 0.0, 0.2, 4.0, 0.0, 0.0, uint8(20), uint8(8))     // across the near plane
	f.Add(nan, 0.0, 0.0, 0.0, 0.0, 1.57, 0.1, 0.0, 0.0, 5.0, 2.0, 0.0, 0.0, uint8(20), uint8(8))
	f.Add(0.0, 0.0, 0.0, 0.0, 0.0, 1.57, 0.1, inf, 0.0, 5.0, 2.0, 0.0, 0.0, uint8(20), uint8(8))
	f.Add(0.0, 0.0, 0.0, 0.0, 0.0, 1.57, 0.1, 0.0, 0.0, 5.0, 1e300, 0.0, 0.0, uint8(20), uint8(8))
	f.Add(0.0, 0.0, 0.0, 0.0, 0.0, 1.57, 0.1, 1e300, 1e300, 1e300, 1e-300, 0.0, 0.0, uint8(20), uint8(8))

	f.Fuzz(func(t *testing.T, cx, cy, cz, yaw, pitch, fov, near, mx, my, mz, size, myaw, mpitch float64, w, h uint8) {
		t.Setenv("TERM_PROGRAM", "")
		defer func(on bool) { cullingOn = on }(cullingOn)
		cam := Camera{Position: geometry.Vector3{X: cx, Y: cy, Z: cz}, Yaw: yaw, Pitch: pitch, FOV: fov, Near: near}
		light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)
		gw, gh := 2+int(w)%60, 2+int(h)%30

		for _, m := range []Mesh{Cube(size), Sphere(size, 6, 4)} {
			m.Transform.Position = geometry.Vector3{X: mx, Y: my, Z: mz}
			m.Transform.Yaw, m.Transform.Pitch = myaw, mpitch
			draw := func() string {
				wire, glyphs, pixels := mustBuffer(t, gw, gh), mustBuffer(t, gw, gh), mustBuffer(t, gw, gh)
				m.DrawDepth(wire, cam, wireCell)
				m.DrawShaded(glyphs, cam, light)
				canvas := pixels.Canvas()
				m.DrawShadedPixels(canvas, cam, light)
				canvas.Compose()
				return picture(wire) + picture(glyphs) + picture(pixels)
			}
			cullingOn = true
			with := draw()
			cullingOn = false
			without := draw()
			cullingOn = true
			if with != without {
				t.Fatalf("wanted the same pictures with and without skipping, camera %+v mesh at %v size %v", cam, m.Transform.Position, size)
			}
		}
	})
}

// The switch the other tests use really does turn skipping off.
func TestOffscreenIsOffWhenCullingIsSwitchedOff(t *testing.T) {
	defer func(on bool) { cullingOn = on }(cullingOn)
	m := Cube(2)
	m.Transform.Position = geometry.Vector3{X: -100, Z: 10}
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}

	cullingOn = false
	if offscreenFor(m, cam, 80, 24) {
		t.Errorf("wanted nothing skipped with culling off")
	}
	cullingOn = true
	if !offscreenFor(m, cam, 80, 24) {
		t.Errorf("wanted a cube far to the side skipped with culling on")
	}
}

// Skipping a mesh must mean the work is not done. The shaded draw asks a light about each
// face it lights, so a light that is never asked shows a mesh was skipped before that.
func TestShadedDrawingOfAMeshOffScreenDoesNotLightIt(t *testing.T) {
	defer func(on bool) { cullingOn = on }(cullingOn)
	m := Sphere(1, 12, 6)
	m.Normals = nil // lit once for each face
	m.Transform.Position = geometry.Vector3{X: -100, Z: 10}

	asked := func(culling bool) int {
		cullingOn = culling
		light := &loggingLight{level: 0.5}
		m.DoubleSided = true
		m.DrawShaded(newShadeBuffer(t), shadeCam(), light)
		m.DrawShadedPixels(newShadeBuffer(t).Canvas(), shadeCam(), light)
		return len(light.calls)
	}

	if with, without := asked(true), asked(false); with != 0 || without == 0 {
		t.Errorf("wanted no questions with skipping and some without, got %d and %d", with, without)
	}
}

// And the wireframe draw skips the work too. A mesh of half a million vertices well off to the
// side is drawn in far less time when it is skipped (all it costs is a look at each vertex).
func TestWireframeOfAMeshOffScreenIsSkipped(t *testing.T) {
	defer func(on bool) { cullingOn = on }(cullingOn)
	m := Mesh{Vertices: make([]geometry.Vector3, 500_000)}
	for i := range m.Vertices {
		m.Vertices[i] = geometry.Vector3{X: float64(i % 100), Y: float64(i % 7), Z: float64(i % 13)}
		if i > 0 {
			m.Edges = append(m.Edges, [2]int{i - 1, i})
		}
	}
	m.Transform.Position = geometry.Vector3{X: -5000, Z: 10}
	fb := newShadeBuffer(t)

	best := func(culling bool) time.Duration {
		cullingOn = culling
		fastest := time.Hour
		for range 3 {
			start := time.Now()
			m.Draw(fb, shadeCam(), wireCell)
			fastest = min(fastest, time.Since(start))
		}
		return fastest
	}
	with, without := best(true), best(false)
	t.Logf("skipped %v, drawn %v", with, without)
	if with*3 > without {
		t.Errorf("wanted skipping to make drawing a mesh off screen at least 3 times faster, got %v with and %v without", with, without)
	}
}

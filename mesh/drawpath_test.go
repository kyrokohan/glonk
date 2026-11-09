package mesh

import (
	"math"
	"math/rand/v2"
	"sync"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// closeVector says whether a and b agree to a few bits of a float64, relative to the
// size of the vector, which is what a different order of rounding can change.
func closeVector(a, b geometry.Vector3) bool {
	tol := 1e-13 * (1 + a.Length())
	return a.Sub(b).Length() <= tol
}

func randomVector(rng *rand.Rand, scale float64) geometry.Vector3 {
	return geometry.Vector3{X: rng.NormFloat64() * scale, Y: rng.NormFloat64() * scale, Z: rng.NormFloat64() * scale}
}

// The view transform works the trigonometry out once but must give the numbers that
// turning the point with RotateY and then RotateX gives. Equal to the last bit is not
// asked: where multiply-adds are fused (arm64) the compiler fuses by context, so the
// two can differ by one bit. The pictures staying the same is the golden tests' job.
func TestViewTransformIsExactlyTheTurnItReplaces(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 20000 {
		cam := Camera{Position: randomVector(rng, 10), Yaw: rng.NormFloat64() * 3, Pitch: rng.NormFloat64()}
		p := randomVector(rng, 20)

		want := p.Sub(cam.Position).RotateY(-cam.Yaw).RotateX(cam.Pitch)
		got := cam.viewTransform().apply(p)

		if !closeVector(got, want) {
			t.Fatalf("camera %+v, point %v: wanted %v, got %v", cam, p, want, got)
		}
	}
}

// The projector is plain arithmetic with no fusable chain, so it must match exactly.
func TestProjectorIsExactlyTheFormulaItReplaces(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for range 20000 {
		cam, _ := Camera{FOV: 0.3 + rng.Float64()*2.5, CellAspect: 1 + rng.Float64()*2}.withDefaults()
		w, h := 1+rng.IntN(300), 1+rng.IntN(100)
		v := geometry.Vector3{X: rng.NormFloat64() * 5, Y: rng.NormFloat64() * 5, Z: 0.1 + rng.Float64()*50}

		f := 1 / math.Tan(cam.FOV/2)
		ndcX, ndcY := f*v.X/v.Z, f*v.Y/v.Z
		wantX := float64(w)/2 + ndcX*float64(h)/2*cam.CellAspect
		wantY := float64(h)/2 - ndcY*float64(h)/2

		gotX, gotY := cam.projector(w, h).project(v)

		if gotX != wantX || gotY != wantY {
			t.Fatalf("%+v at %dx%d, %v: wanted (%v, %v), got (%v, %v)", cam, w, h, v, wantX, wantY, gotX, gotY)
		}
	}
}

// drawPathMesh is a mesh that gets clipped by the near plane, hides itself and is
// seen from behind, so that every branch of the draw path runs.
func drawPathMesh() Mesh {
	m := Cube(2)
	m.Transform = Transform{Position: geometry.Vector3{X: 0.4, Y: 0.2, Z: 1.2}, Yaw: 0.6, Pitch: -0.3, Roll: 0.2}
	m.Material = Material{Color: engine.Palette(208)}
	return m
}

// Once the scratch lists have grown, drawing allocates nothing at all.
func TestDrawingAllocatesNothing(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector makes sync.Pool drop items at random")
	}
	t.Setenv("TERM_PROGRAM", "") // Canvas.Seamless depends on the terminal running the tests
	cam := Camera{Position: geometry.Vector3{Z: -2}, Near: 0.5}
	lights := []Light{Ambient{Level: 0.2}, Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}, Point{Position: geometry.Vector3{X: -1, Y: 2, Z: -1}, Range: 20}}
	fb := newShadeBuffer(t)
	canvas := fb.Canvas()
	m := drawPathMesh()
	ramped := m
	ramped.Material.Ramp = " .oO@"
	both := m
	both.DoubleSided = true
	smooth := m.SmoothNormals()
	colored := m.Clone()
	colored.FaceColors = colorsCycling(len(colored.Faces))
	smoothColored := smooth.Clone()
	smoothColored.FaceColors = colorsCycling(len(smoothColored.Faces))
	smoothRamped := ramped.SmoothNormals()
	smoothBoth := both.SmoothNormals()
	sphere := Sphere(1.5, 12, 6)
	sphere.Transform = Transform{Position: geometry.Vector3{X: 0.3, Z: 1}, Yaw: 0.4, Scale: 1.2}

	tests := map[string]func(){
		"wireframe":              func() { m.Draw(fb, cam, engine.Cell{Character: '#'}) },
		"depth wireframe":        func() { m.DrawDepth(fb, cam, engine.Cell{Character: '#'}) },
		"pixel wireframe":        func() { m.DrawPixels(canvas, cam, engine.RGB(10, 250, 10)) },
		"pixel depth wireframe":  func() { m.DrawDepthPixels(canvas, cam, engine.RGB(10, 250, 10)) },
		"shaded":                 func() { m.DrawShaded(fb, cam, lights...) },
		"shaded, a ramp":         func() { ramped.DrawShaded(fb, cam, lights...) },
		"shaded, two-sided":      func() { both.DrawShaded(fb, cam, lights...) },
		"pixels":                 func() { m.DrawShadedPixels(canvas, cam, lights...) },
		"smooth":                 func() { smooth.DrawShaded(fb, cam, lights...) },
		"face colors":            func() { colored.DrawShaded(fb, cam, lights...) },
		"face colors, pixels":    func() { colored.DrawShadedPixels(canvas, cam, lights...) },
		"smooth, face colors":    func() { smoothColored.DrawShaded(fb, cam, lights...) },
		"smooth, colors, pixels": func() { smoothColored.DrawShadedPixels(canvas, cam, lights...) },
		"smooth, a ramp":         func() { smoothRamped.DrawShaded(fb, cam, lights...) },
		"smooth, two-sided":      func() { smoothBoth.DrawShaded(fb, cam, lights...) },
		"smooth sphere":          func() { sphere.DrawShaded(fb, cam, lights...) },
		"smooth pixels":          func() { smooth.DrawShadedPixels(canvas, cam, lights...) },
		"smooth sphere, pixels":  func() { sphere.DrawShadedPixels(canvas, cam, lights...) },
		"nothing to draw":        func() { Mesh{}.DrawShaded(fb, cam, lights...) },
	}
	for name, draw := range tests {
		t.Run(name, func(t *testing.T) {
			if n := testing.AllocsPerRun(50, draw); n != 0 {
				t.Errorf("wanted no allocations per draw, got %v", n)
			}
		})
	}
}

// Draw calls from several goroutines share the scratch lists through a pool, and none
// may see another's numbers.
func TestDrawingFromSeveralGoroutinesGivesTheSamePictures(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	cam := Camera{Position: geometry.Vector3{Z: -2}, Near: 0.5}
	lights := []Light{Ambient{Level: 0.2}, Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}}
	meshes := []Mesh{drawPathMesh(), Cube(1), Grid(6, 3), Sphere(1.5, 12, 6), drawPathMesh().SmoothNormals()}
	meshes[1].Transform.Position = geometry.Vector3{Z: 4}
	meshes[2].Transform = Transform{Position: geometry.Vector3{Y: -1, Z: 5}}
	meshes[3].Transform = Transform{Position: geometry.Vector3{X: 0.3, Z: 3}, Yaw: 0.4}

	render := func(m Mesh) string {
		fb := newShadeBuffer(t)
		m.Draw(fb, cam, engine.Cell{Character: '#'})
		m.DrawShaded(fb, cam, lights...)
		canvas := fb.Canvas()
		m.DrawPixels(canvas, cam, engine.RGB(10, 250, 10))
		m.DrawDepthPixels(canvas, cam, engine.RGB(250, 10, 10))
		canvas.Compose()
		return picture(fb)
	}
	want := make([]string, len(meshes))
	for i, m := range meshes {
		want[i] = render(m)
	}

	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				i := g % len(meshes)
				if got := render(meshes[i]); got != want[i] {
					t.Errorf("goroutine %d: wanted the same picture as drawing alone", g)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestClipTriangleNearAllocatesNothing(t *testing.T) {
	a := clipVert{view: geometry.Vector3{X: -1, Z: -1}}
	b := clipVert{view: geometry.Vector3{X: 1, Z: 2}}
	c := clipVert{view: geometry.Vector3{Y: 1, Z: 3}}
	var out [2][3]clipVert

	if n := testing.AllocsPerRun(100, func() { clipTriangleNear(a, b, c, 0.1, true, &out) }); n != 0 {
		t.Errorf("wanted no allocations, got %v", n)
	}
}

// The world position and the normal are cut along with the corners: at a crossing they
// are the same blend of the two ends that the view position is. The crossings are a
// quarter of the way along, not halfway, so that a blend from the wrong end shows.
func TestClipTriangleNearCarriesTheAttributesToTheCrossing(t *testing.T) {
	a := clipVert{view: geometry.Vector3{X: 0, Y: 0, Z: -1}, world: geometry.Vector3{X: 10}, normal: geometry.Vector3{Y: 1}}
	b := clipVert{view: geometry.Vector3{X: 2, Y: 0, Z: 3}, world: geometry.Vector3{X: 20}, normal: geometry.Vector3{X: 1}}
	c := clipVert{view: geometry.Vector3{X: 0, Y: 2, Z: 3}, world: geometry.Vector3{X: 30}, normal: geometry.Vector3{Z: 1}}
	var out [2][3]clipVert

	n := clipTriangleNear(a, b, c, 0, true, &out)
	if n != 2 {
		t.Fatalf("wanted a quad (two triangles), got %d triangles", n)
	}

	crossings := []clipVert{
		{view: geometry.Vector3{X: 0.5}, world: geometry.Vector3{X: 12.5}, normal: geometry.Vector3{X: 0.25, Y: 0.75}}, // on a-b
		{view: geometry.Vector3{Y: 0.5}, world: geometry.Vector3{X: 15}, normal: geometry.Vector3{Y: 0.75, Z: 0.25}},   // on a-c
	}
	found := make([]bool, len(crossings))
	for _, tri := range out[:n] {
		for _, v := range tri {
			if v.view.Z < 0 {
				t.Fatalf("wanted every corner at or past the plane, got %v", v.view)
			}
			for k, want := range crossings {
				if vecEquals(v.view, want.view) {
					found[k] = true
					if !vecEquals(v.world, want.world) || !vecEquals(v.normal, want.normal) {
						t.Errorf("crossing %v: wanted world %v and normal %v, got world %v and normal %v", want.view, want.world, want.normal, v.world, v.normal)
					}
				}
			}
		}
	}
	for k, ok := range found {
		if !ok {
			t.Errorf("wanted a corner at the crossing %v, found none", crossings[k].view)
		}
	}

	// Without attributes they are left alone.
	n = clipTriangleNear(a, b, c, 0, false, &out)
	for _, tri := range out[:n] {
		for _, v := range tri {
			if v.view.Z == 0 && (v.world != (geometry.Vector3{}) || v.normal != (geometry.Vector3{})) {
				t.Errorf("wanted no attributes carried when not asked, got %v %v", v.world, v.normal)
			}
		}
	}
}

// A material's own ramp is decoded into the scratch; the shared default ramp must never
// end up as that memory, or the next custom ramp would be written over it.
func TestACustomRampNeverTouchesTheDefaultOne(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	cam := Camera{Position: geometry.Vector3{Z: -3}}
	lights := []Light{Ambient{Level: 0.3}, Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}}
	plain := Cube(2)
	custom := Cube(2)
	custom.Material.Ramp = "ab"
	before := string(defaultRampRunes)

	draw := func(m Mesh) string {
		fb := newShadeBuffer(t)
		m.DrawShaded(fb, cam, lights...)
		return picture(fb)
	}
	want := draw(plain)
	for range 20 { // the pool hands the same scratch back, so order matters
		draw(plain)
		draw(custom)
	}

	if got := string(defaultRampRunes); got != before {
		t.Fatalf("wanted the default ramp left alone as %q, got %q", before, got)
	}
	if got := draw(plain); got != want {
		t.Errorf("wanted the default ramp's picture after custom ones, got a different one")
	}
}

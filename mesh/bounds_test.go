package mesh

import (
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

func vec(x, y, z float64) geometry.Vector3 { return geometry.Vector3{X: x, Y: y, Z: z} }

// atTheWorld returns a mesh with its vertices written where they are in the world, no Transform:
// the vertices of the mesh moved by offset, as a level built in place is.
func atTheWorld(m Mesh, offset geometry.Vector3) Mesh {
	m = m.Clone()
	for i, v := range m.Vertices {
		m.Vertices[i] = v.Add(offset)
	}
	m.bounds = nil
	return m
}

func TestUpdateBoundsHoldsEveryVertexAndNoMore(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	for name, tt := range map[string]struct {
		vs        []geometry.Vector3
		center    geometry.Vector3 // wanted, where it matters
		hasCenter bool
	}{
		"a cube round its origin":     {Cube(2).Vertices, vec(0, 0, 0), true},
		"a cube written in the world": {atTheWorld(Cube(2), vec(40, -3, 12)).Vertices, vec(40, -3, 12), true},
		"a long thin one":             {[]geometry.Vector3{vec(100, 0, 0), vec(300, 1, 1), vec(200, 0.5, 0.5)}, vec(200, 0.5, 0.5), true},
		"one vertex":                  {[]geometry.Vector3{vec(5, 6, 7)}, vec(5, 6, 7), true},
		"one vertex at the origin":    {[]geometry.Vector3{{}}, vec(0, 0, 0), true},
		"a model off its origin":      {[]geometry.Vector3{vec(-1, 0, 0), vec(3, 0, 0)}, vec(1, 0, 0), true},
		"two apart on one side":       {[]geometry.Vector3{vec(1, 0, 0), vec(3, 0, 0)}, vec(2, 0, 0), true},
		"a NaN is left out":           {[]geometry.Vector3{vec(10, 10, 10), vec(nan, 0, 0), vec(12, 10, 10)}, vec(11, 10, 10), true},
		"all of them NaN":             {[]geometry.Vector3{vec(nan, nan, nan)}, vec(0, 0, 0), true},
		"a vertex at infinity":        {[]geometry.Vector3{vec(1, 0, 0), vec(inf, 0, 0)}, vec(0, 0, 0), true},
		"a very big one":              {[]geometry.Vector3{vec(-1e300, 0, 0), vec(1e300, 0, 0)}, vec(0, 0, 0), true},
		"a tiny one":                  {[]geometry.Vector3{vec(1e-300, 0, 0), vec(2e-300, 0, 0)}, vec(1.5e-300, 0, 0), false},
	} {
		t.Run(name, func(t *testing.T) {
			m := Mesh{Vertices: tt.vs}
			m.UpdateBounds()
			b := m.currentBounds()
			if b == nil {
				t.Fatal("wanted bounds")
			}
			if tt.hasCenter && b.center != tt.center {
				t.Errorf("wanted the center %v, got %v", tt.center, b.center)
			}
			lo, hi := math.Inf(1), math.Inf(-1)
			_ = lo
			_ = hi
			for i, v := range tt.vs {
				if math.IsNaN(v.X) || math.IsNaN(v.Y) || math.IsNaN(v.Z) {
					continue
				}
				if d := v.Sub(b.center).Length(); !(d <= b.radius) {
					t.Errorf("vertex %d at %v is %v from the center, past the radius %v", i, v, d, b.radius)
				}
			}
			// No bigger than the sphere round the middle of the box, which is half its diagonal, and no
			// more than a rounding bigger than the farthest vertex.
			var far, half float64
			var min3, max3 geometry.Vector3
			first := true
			for _, v := range tt.vs {
				if math.IsNaN(v.X) || math.IsNaN(v.Y) || math.IsNaN(v.Z) || math.IsInf(v.X, 0) || math.IsInf(v.Y, 0) || math.IsInf(v.Z, 0) {
					continue
				}
				if first {
					min3, max3, first = v, v, false
				}
				min3 = vec(min(min3.X, v.X), min(min3.Y, v.Y), min(min3.Z, v.Z))
				max3 = vec(max(max3.X, v.X), max(max3.Y, v.Y), max(max3.Z, v.Z))
			}
			half = max3.Sub(min3).Length() / 2
			for _, v := range tt.vs {
				if !math.IsNaN(v.X) && !math.IsNaN(v.Y) && !math.IsNaN(v.Z) && !math.IsInf(v.X, 0) {
					far = max(far, v.Sub(b.center).Length())
				}
			}
			if !math.IsInf(b.radius, 1) && (b.radius > half*(1+1e-6)+1e-6*b.center.Length() || b.radius < far) {
				t.Errorf("wanted a radius between the farthest vertex %v and half the diagonal %v, got %v", far, half, b.radius)
			}
		})
	}
	var empty Mesh
	empty.UpdateBounds()
	if empty.bounds != nil {
		t.Errorf("wanted no bounds for a mesh with no vertices")
	}
	// A mesh that had bounds and then lost its vertices has none after UpdateBounds either.
	emptied := Cube(1)
	emptied.Vertices = nil
	emptied.UpdateBounds()
	if emptied.bounds != nil {
		t.Errorf("wanted UpdateBounds to drop the sphere of vertices that are gone")
	}
	// The origin is the center where it is no bigger, even where the middle of the box is a
	// rounding away from it.
	almost := Mesh{Vertices: []geometry.Vector3{vec(-1, 0, 0), vec(1+1e-12, 0, 0)}}
	almost.UpdateBounds()
	if almost.bounds.center != (geometry.Vector3{}) {
		t.Errorf("wanted the origin for a model that is round it to a rounding, got %v", almost.bounds.center)
	}
	infinite := Mesh{Vertices: []geometry.Vector3{vec(1, 0, 0), vec(inf, 0, 0)}}
	infinite.UpdateBounds()
	if !math.IsInf(infinite.bounds.radius, 1) {
		t.Errorf("wanted an infinite radius for a vertex at infinity, so that nothing is skipped")
	}
}

func TestTheModelsOriginIsTheCenterWhereItIsNoBigger(t *testing.T) {
	for name, m := range map[string]Mesh{"Sphere": Sphere(3, 16, 8), "Cube": Cube(5), "Cylinder": Cylinder(2, 4, 12), "Cone": Cone(2, 4, 12), "Plane": Plane(6, 4), "Grid": Grid(10, 5)} {
		if b := m.currentBounds(); b == nil || b.center != (geometry.Vector3{}) {
			t.Errorf("%s: wanted the model's origin as the center, which costs no turning, got %+v", name, b)
		}
	}
	off := atTheWorld(Sphere(3, 16, 8), vec(50, 0, 0))
	off.UpdateBounds()
	if b := off.currentBounds(); b == nil || math.Abs(b.center.X-50) > 1e-9 || b.radius > 3.0001 {
		t.Errorf("wanted a sphere of 3 round (50, 0, 0), got %+v", b)
	}
}

// The level of the issue: its vertices are written in the world, with no transform, so a sphere
// round its Position, which is the world's origin, holds the camera and everything, and a mesh
// that is wholly behind the camera or to the side is not skipped. With the bounds it is.
func TestAMeshWrittenInTheWorldIsSkippedWithItsBounds(t *testing.T) {
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}
	chunk := func(x, y, z float64) Mesh { return atTheWorld(Plane(8, 8), vec(x, y, z)) }
	for name, tt := range map[string]struct {
		m    Mesh
		want bool // with the bounds
	}{
		"behind the camera":     {chunk(0, -2, -30), true},
		"far to the left":       {chunk(-80, -2, 20), true},
		"far to the right":      {chunk(80, -2, 20), true},
		"far above":             {chunk(0, 90, 20), true},
		"ahead, in view":        {chunk(0, -2, 20), false},
		"ahead, partly in view": {chunk(-18, -2, 10), false},
		"beyond the far plane":  {chunk(0, -2, 1000), false}, // the camera has no far plane
	} {
		t.Run(name, func(t *testing.T) {
			m := tt.m
			if offscreenFor(m, cam, 80, 24) && tt.want {
				t.Errorf("wanted a mesh written in the world not to be skipped before UpdateBounds, as it was")
			}
			m.UpdateBounds()
			if got := offscreenFor(m, cam, 80, 24); got != tt.want {
				t.Errorf("wanted %v with UpdateBounds, got %v", tt.want, got)
			}
		})
	}
	far := Camera{FOV: math.Pi / 2, Near: 0.1, Far: 100}
	m := chunk(0, -2, 1000)
	m.UpdateBounds()
	if !offscreenFor(m, far, 80, 24) {
		t.Errorf("wanted a chunk past a far plane of 100 to be skipped")
	}
}

// Skipping by the bounds changes no picture, for meshes written anywhere in the world and then
// moved, turned and scaled by a Transform as well, drawn every way a mesh is drawn.
func TestSkippingByBoundsChangesNoPicture(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	defer func(on bool) { cullingOn = on }(cullingOn)
	rng := rand.New(rand.NewPCG(61, 62))
	light := sunAndFill(vec(0.4, -1, 0.6), 0.2)
	skipped, drawn := 0, 0

	for scene := range 600 {
		var m Mesh
		switch scene % 4 {
		case 0:
			m = Cube(0.3 + 3*rng.Float64())
		case 1:
			m = Sphere(0.3+3*rng.Float64(), 3+rng.IntN(10), 2+rng.IntN(6))
		case 2:
			m = Plane(1+12*rng.Float64(), 1+12*rng.Float64())
		default:
			m = Cylinder(0.3+rng.Float64(), 0.5+3*rng.Float64(), 3+rng.IntN(10))
		}
		// Written where it is in the world, near or far, as a level in place is.
		m = atTheWorld(m, vec(rng.Float64()*50-25, rng.Float64()*14-7, rng.Float64()*70-20))
		m.UpdateBounds()
		if rng.IntN(2) == 0 { // and then placed as well
			m.Transform = Transform{
				Position: vec(rng.Float64()*40-20, rng.Float64()*20-10, rng.Float64()*40-20),
				Yaw:      rng.Float64() * 6, Pitch: rng.Float64() * 3, Roll: rng.Float64() * 3,
			}
			if rng.IntN(2) == 0 {
				m.Transform.Scale = []float64{0.2, 1, 3}[rng.IntN(3)]
			}
		}
		cam := Camera{
			Position: vec(rng.Float64()*20-10, rng.Float64()*6-3, rng.Float64()*20-10),
			Yaw:      rng.Float64() * 6.3, Pitch: rng.Float64() - 0.5,
			FOV: 0.4 + 2*rng.Float64(), Near: 0.05 + rng.Float64(),
		}
		if rng.IntN(3) == 0 {
			cam.Far = 20 + 100*rng.Float64()
		}
		w, h := 8+rng.IntN(90), 4+rng.IntN(30)

		draw := func() [4]string {
			wire, glyphs, pixels, pixelWire := mustBuffer(t, w, h), mustBuffer(t, w, h), mustBuffer(t, w, h), mustBuffer(t, w, h)
			m.DrawDepth(wire, cam, wireCell)
			m.withColor(engine.Palette(33)).DrawShaded(glyphs, cam, light)
			canvas := pixels.Canvas()
			m.withColor(engine.Palette(33)).DrawShadedPixels(canvas, cam, light)
			canvas.Compose()
			m.DrawPixels(pixelWire.Canvas(), cam, engine.Palette(33))
			pixelWire.Canvas().Compose()
			return [4]string{picture(wire), picture(glyphs), picture(pixels), picture(pixelWire)}
		}
		cullingOn = true
		with := draw()
		cullingOn = false
		without := draw()
		cullingOn = true
		for i, name := range []string{"DrawDepth", "DrawShaded", "DrawShadedPixels", "DrawPixels"} {
			if with[i] != without[i] {
				t.Fatalf("scene %d (%dx%d): wanted %s to draw the same with and without skipping, got\n%s\nwanted\n%s", scene, w, h, name, with[i], without[i])
			}
		}

		c, _ := cam.withDefaults()
		switch {
		case m.offscreen(c.viewTransform(), c.projector(w, h), c.Near, c.Far):
			skipped++
		case without[1] != picture(mustBuffer(t, w, h)):
			drawn++
		}
	}
	if skipped < 120 || drawn < 60 {
		t.Errorf("wanted plenty of meshes skipped and drawn, got %d and %d", skipped, drawn)
	}
}

// A sphere that was worked out from other vertices is not used, so a mesh whose Vertices were
// replaced, or grown, or cut is never skipped by a sphere that does not hold it.
func TestBoundsAreNotUsedWhenTheyAreStale(t *testing.T) {
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}
	base := atTheWorld(Plane(4, 4), vec(0, -2, -40)) // behind the camera
	base.UpdateBounds()
	if !offscreenFor(base, cam, 80, 24) {
		t.Fatal("wanted the mesh behind the camera to be skipped with its bounds")
	}

	// Vertices that make it be in front of the camera now, in each way a program changes them.
	front := func(vs []geometry.Vector3) []geometry.Vector3 {
		out := slices.Clone(vs)
		for i := range out {
			out[i].Z += 60
		}
		return out
	}
	for name, edit := range map[string]func(m *Mesh){
		"replaced by another list of the same length": func(m *Mesh) { m.Vertices = front(m.Vertices) },
		"grown":   func(m *Mesh) { m.Vertices = append(m.Vertices, vec(0, -2, 20)) },
		"cut":     func(m *Mesh) { m.Vertices = m.Vertices[:3] },
		"emptied": func(m *Mesh) { m.Vertices = nil },
	} {
		m := base
		edit(&m)
		if m.currentBounds() != nil {
			t.Errorf("%s: wanted the bounds not to be current", name)
		}
	}
	m := base
	m.Vertices = front(m.Vertices)
	if offscreenFor(m, cam, 80, 24) {
		t.Errorf("wanted a mesh moved in front of the camera, by a new list, not to be skipped by the old sphere")
	}
	// Cut from the front: the same array from its second element, so another first vertex.
	m = base
	m.Vertices = m.Vertices[1:]
	if m.currentBounds() != nil {
		t.Errorf("wanted bounds not to be current for a list that starts at another element")
	}
}

// A vertex changed in place is the one thing the bounds cannot see: the mesh is skipped by the old
// sphere until UpdateBounds, and Validate says so.
func TestAnEditInPlaceIsFoundByValidate(t *testing.T) {
	m := atTheWorld(Plane(4, 4), vec(0, -2, -40))
	m.UpdateBounds()
	if err := m.Validate(); err != nil {
		t.Fatalf("wanted a valid mesh, got %v", err)
	}
	m.Vertices[2] = vec(0, -2, 20) // in front of the camera, outside the sphere
	err := m.Validate()
	if err == nil || !strings.Contains(err.Error(), "vertex 2") || !strings.Contains(err.Error(), "UpdateBounds") {
		t.Fatalf("wanted Validate to name the vertex and UpdateBounds, got %v", err)
	}
	m.UpdateBounds()
	if err := m.Validate(); err != nil {
		t.Errorf("wanted the mesh valid again after UpdateBounds, got %v", err)
	}
}

func TestEverythingThatMakesAMeshHasBounds(t *testing.T) {
	obj := "v 0 0 0\nv 4 0 0\nv 0 4 0\nf 1 2 3\n"
	loaded, err := LoadOBJ(strings.NewReader(obj))
	if err != nil {
		t.Fatal(err)
	}
	for name, m := range map[string]Mesh{
		"Cube": Cube(2), "Sphere": Sphere(1, 8, 4), "Cylinder": Cylinder(1, 2, 8), "Cone": Cone(1, 2, 8), "Plane": Plane(2, 2), "Grid": Grid(4, 2),
		"LoadOBJ": loaded, "Fit": loaded.Fit(2), "Clone": loaded.Clone(), "SmoothNormals": loaded.SmoothNormals(), "a copy": loaded,
	} {
		b := m.currentBounds()
		if b == nil {
			t.Errorf("%s: wanted current bounds", name)
			continue
		}
		for i, v := range m.Vertices {
			if v.Sub(b.center).Length() > b.radius {
				t.Errorf("%s: vertex %d is outside the bounds", name, i)
			}
		}
		if err := m.Validate(); err != nil {
			t.Errorf("%s: wanted a valid mesh, got %v", name, err)
		}
	}

	// A mesh built by hand has none until it asks.
	hand := Mesh{Vertices: []geometry.Vector3{vec(1, 2, 3)}}
	if hand.currentBounds() != nil {
		t.Errorf("wanted a mesh built by hand to have no bounds")
	}
	// A clone is its own: changing it leaves the original's alone, and its bounds are about its own vertices.
	clone := loaded.Clone()
	if clone.bounds == loaded.bounds || clone.bounds.first != &clone.Vertices[0] {
		t.Errorf("wanted a clone to have bounds of its own about its own vertices")
	}
	// A clone of a mesh with stale bounds does not pretend they are current.
	stale := loaded
	stale.Vertices = slices.Clone(stale.Vertices)
	if stale.Clone().bounds != nil {
		t.Errorf("wanted a clone of a mesh with stale bounds to have none")
	}
	// Fit moves and scales the vertices, so the sphere is a different one.
	fit := loaded.Fit(2)
	if fit.bounds.radius == loaded.bounds.radius {
		t.Errorf("wanted Fit to work the bounds out again, got the same radius %v", fit.bounds.radius)
	}
}

// The sphere is placed as the vertices are: scaled by Scale, with 0 meaning 1, and turned about the
// model's origin, so its middle lands where the middle of the vertices does.
func TestTheSphereIsPlacedLikeTheVertices(t *testing.T) {
	m := atTheWorld(Cube(2), vec(10, 0, 0))
	m.UpdateBounds()
	for _, tr := range []Transform{
		{}, {Scale: 2}, {Yaw: 1.2}, {Pitch: 0.7, Roll: 0.3}, {Position: vec(1, 2, 3), Yaw: 2, Scale: 0.5},
	} {
		m.Transform = tr
		b := m.currentBounds()
		want := m.Transform.Apply(vec(10, 0, 0))
		got := m.Transform.Apply(b.center)
		if got.Sub(want).Length() > 1e-9 {
			t.Errorf("%+v: wanted the middle at %v, got %v", tr, want, got)
		}
		reach := b.radius * m.Transform.scale()
		for _, v := range m.Vertices {
			if d := m.Transform.Apply(v).Sub(got).Length(); d > reach {
				t.Errorf("%+v: a vertex is %v from the middle, past the reach %v", tr, d, reach)
			}
		}
	}
	if (Transform{}).scale() != 1 || (Transform{Scale: 3}).scale() != 3 {
		t.Errorf("wanted a Scale of 0 to be 1")
	}
}

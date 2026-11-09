package mesh

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// benchScene is what cube3d draws: a floor of lines and six cubes at mixed distances
// and turns, one close enough to fill the view, seen from a camera that looks down a
// little.
func benchScene() (floor Mesh, cubes []Mesh, cam Camera, lights []Light) {
	floor = Grid(40, 20)
	floor.Transform.Position = geometry.Vector3{Y: -1.5}
	for i, z := range []float64{1.5, 3, 5, 7, 9, 12} {
		c := Cube(2)
		c.Transform = Transform{Position: geometry.Vector3{X: float64(i%3-1) * 1.5, Z: z}, Yaw: float64(i) * 0.5, Pitch: float64(i) * -0.3}
		c.Material.Color = engine.Palette(uint8(30 + i*30))
		cubes = append(cubes, c)
	}
	cam = Camera{Position: geometry.Vector3{Y: 0.5, Z: -3}, Pitch: -0.1, FOV: math.Pi / 2, Near: 0.1}
	lights = []Light{Ambient{Level: 0.2}, Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}}
	return floor, cubes, cam, lights
}

// BenchmarkDraw3D measures one frame of the cube3d scene drawn as wireframe, shaded
// into cells and shaded into half-block pixels, at two sizes.
func BenchmarkDraw3D(b *testing.B) {
	floor, cubes, cam, lights := benchScene()
	for _, size := range []struct{ w, h int }{{80, 24}, {200, 60}} {
		fb, err := engine.NewFrameBuffer(size.w, size.h)
		if err != nil {
			b.Fatal(err)
		}
		draws := map[string]func(){
			"wireframe": func() {
				floor.Draw(fb, cam, engine.Cell{Character: '.'})
				for _, c := range cubes {
					c.Draw(fb, cam, engine.Cell{Character: '#'})
				}
			},
			"shaded": func() {
				for _, c := range cubes {
					c.DrawShaded(fb, cam, lights...)
				}
			},
			"pixels": func() {
				for _, c := range cubes {
					c.DrawShadedPixels(fb.Canvas(), cam, lights...)
				}
			},
		}
		for _, name := range []string{"wireframe", "shaded", "pixels"} {
			draw := draws[name]
			b.Run(fmt.Sprintf("%s %dx%d", name, size.w, size.h), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					fb.Clear()
					draw()
				}
			})
		}
	}
}

// BenchmarkDrawSmooth3D draws the same scene with a sphere of 24 by 12 segments in
// place of each cube, once with normals (lit at every cell) and once without (lit at
// every face), into cells and into pixels, at two sizes.
func BenchmarkDrawSmooth3D(b *testing.B) {
	_, cubes, cam, lights := benchScene()
	var smooth, faceted []Mesh
	for _, c := range cubes {
		s := Sphere(1.2, 24, 12)
		s.Transform, s.Material = c.Transform, c.Material
		f := s.Clone()
		f.Normals = nil
		smooth, faceted = append(smooth, s), append(faceted, f)
	}
	for _, size := range []struct{ w, h int }{{80, 24}, {200, 60}} {
		fb, err := engine.NewFrameBuffer(size.w, size.h)
		if err != nil {
			b.Fatal(err)
		}
		for _, v := range []struct {
			name   string
			meshes []Mesh
			pixels bool
		}{
			{"faceted shaded", faceted, false},
			{"smooth shaded", smooth, false},
			{"faceted pixels", faceted, true},
			{"smooth pixels", smooth, true},
		} {
			b.Run(fmt.Sprintf("%s %dx%d", v.name, size.w, size.h), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					fb.Clear()
					for _, m := range v.meshes {
						if v.pixels {
							m.DrawShadedPixels(fb.Canvas(), cam, lights...)
						} else {
							m.DrawShaded(fb, cam, lights...)
						}
					}
				}
			})
		}
	}
}

// manyScene is a thousand unit cubes scattered around the camera in every direction, 5 to
// 40 away, turned every which way: the scene of a game, where most of what exists is off
// screen. The camera looks down +z through a narrow lens, so about one cube in twenty is
// in view at the sizes the benchmarks use.
func manyScene() (cubes []Mesh, cam Camera, lights []Light) {
	rng := rand.New(rand.NewPCG(31, 41))
	for range 1000 {
		// A direction chosen evenly over the sphere, and a distance along it.
		dz := 2*rng.Float64() - 1
		phi := 2 * math.Pi * rng.Float64()
		r := math.Sqrt(1 - dz*dz)
		dir := geometry.Vector3{X: r * math.Cos(phi), Y: r * math.Sin(phi), Z: dz}

		c := Cube(1)
		c.Transform = Transform{Position: dir.Scale(5 + 35*rng.Float64()), Yaw: 2 * math.Pi * rng.Float64(), Pitch: math.Pi * rng.Float64()}
		c.Material.Color = engine.Palette(uint8(16 + rng.IntN(200)))
		cubes = append(cubes, c)
	}
	cam = Camera{FOV: 0.6, Near: 0.1}
	lights = []Light{Ambient{Level: 0.2}, Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}}
	return cubes, cam, lights
}

// The benchmark scene stays what its comment says: a small share of the cubes in view,
// and the same cubes every run, so numbers from different days compare.
func TestManySceneHasAboutOneCubeInTwentyInView(t *testing.T) {
	cubes, cam, _ := manyScene()
	again, _, _ := manyScene()

	inView := 0
	for i, c := range cubes {
		if c.Transform != again[i].Transform {
			t.Fatalf("wanted the same scene every time, cube %d differs", i)
		}
		if x, y, ok := cam.Project(c.Transform.Position, 80, 24); ok && x >= 0 && x < 80 && y >= 0 && y < 24 {
			inView++
		}
	}
	if len(cubes) != 1000 || inView < 20 || inView > 80 {
		t.Errorf("wanted 1000 cubes with 20 to 80 of their middles in view, got %d with %d", len(cubes), inView)
	}
}

// BenchmarkDrawMany draws the thousand cubes as wireframes, shaded into cells and
// shaded into pixels, at two sizes. Skipping what is off screen should show here.
func BenchmarkDrawMany(b *testing.B) {
	cubes, cam, lights := manyScene()
	for _, size := range []struct{ w, h int }{{80, 24}, {200, 60}} {
		fb, err := engine.NewFrameBuffer(size.w, size.h)
		if err != nil {
			b.Fatal(err)
		}
		draws := map[string]func(){
			"wireframe": func() {
				for _, c := range cubes {
					c.Draw(fb, cam, engine.Cell{Character: '#'})
				}
			},
			"shaded": func() {
				for _, c := range cubes {
					c.DrawShaded(fb, cam, lights...)
				}
			},
			"pixels": func() {
				for _, c := range cubes {
					c.DrawShadedPixels(fb.Canvas(), cam, lights...)
				}
			},
		}
		for _, name := range []string{"wireframe", "shaded", "pixels"} {
			draw := draws[name]
			b.Run(fmt.Sprintf("%s %dx%d", name, size.w, size.h), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					fb.Clear()
					draw()
				}
			})
		}
	}
}

// BenchmarkDrawBig draws one big smooth sphere (about 16 thousand faces) where it is
// all in view, where half of it is off the right edge, and where it is behind the
// camera, as a wireframe and shaded. A model that is fully in view must not get slower
// for the sake of one that is not.
func BenchmarkDrawBig(b *testing.B) {
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}
	lights := []Light{Ambient{Level: 0.2}, Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}}
	fb, err := engine.NewFrameBuffer(80, 24)
	if err != nil {
		b.Fatal(err)
	}

	for _, where := range []struct {
		name string
		at   geometry.Vector3
	}{
		{"in view", geometry.Vector3{Z: 8}},
		{"half off screen", geometry.Vector3{X: 13.3, Z: 8}}, // the right edge at depth 8 is about x=13.3
		{"behind the camera", geometry.Vector3{Z: -10}},
	} {
		ball := Sphere(3, 128, 64)
		ball.Transform.Position = where.at
		for _, mode := range []string{"wireframe", "shaded"} {
			b.Run(where.name+" "+mode, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					fb.Clear()
					if mode == "wireframe" {
						ball.Draw(fb, cam, engine.Cell{Character: '#'})
					} else {
						ball.DrawShaded(fb, cam, lights...)
					}
				}
			})
		}
	}
}

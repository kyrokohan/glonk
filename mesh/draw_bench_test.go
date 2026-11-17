package mesh

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"glonk.dev/bitmap"
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

// BenchmarkDrawFar draws a long scene with and without a far plane: the thousand cubes, 5 to 40
// away, with a Far of 20 that skips about half of them, and the big smooth sphere with a Far
// that cuts through the middle of it, so that the faces it crosses are cut as well as skipped.
func BenchmarkDrawFar(b *testing.B) {
	cubes, cam, lights := manyScene()
	fb, err := engine.NewFrameBuffer(80, 24)
	if err != nil {
		b.Fatal(err)
	}

	for _, far := range []float64{0, 20} {
		cam := cam
		cam.Far = far
		for _, mode := range []string{"wireframe", "shaded", "pixels"} {
			b.Run(fmt.Sprintf("many cubes %s far %v", mode, far), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					fb.Clear()
					for _, c := range cubes {
						switch mode {
						case "wireframe":
							c.Draw(fb, cam, engine.Cell{Character: '#'})
						case "shaded":
							c.DrawShaded(fb, cam, lights...)
						default:
							c.DrawShadedPixels(fb.Canvas(), cam, lights...)
						}
					}
				}
			})
		}
	}

	ball := Sphere(3, 128, 64)
	ball.Transform.Position = geometry.Vector3{Z: 8} // from 5 to 11 away
	for _, far := range []float64{0, 8} {
		cam := Camera{FOV: math.Pi / 2, Near: 0.1, Far: far}
		for _, mode := range []string{"wireframe", "shaded"} {
			b.Run(fmt.Sprintf("big sphere %s far %v", mode, far), func(b *testing.B) {
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

// BenchmarkDrawFog draws with fog and without it: the thousand cubes, and a big flat floor seen
// at a pitch, which is where a fog changes the color of a face from cell to cell, as cells and as
// pixels. Without fog the code is the code there was; with it, every cell and pixel is asked for
// its color.
func BenchmarkDrawFog(b *testing.B) {
	cubes, cam, lights := manyScene()
	fb, err := engine.NewFrameBuffer(80, 24)
	if err != nil {
		b.Fatal(err)
	}
	fog := &Fog{Color: engine.RGB(30, 30, 60), Start: 8, End: 30}
	floor := Plane(400, 400)
	floor.Normals = nil
	floor.Material.Color = engine.Palette(70)
	floorCam := Camera{Position: geometry.Vector3{Y: 3}, Pitch: -0.6}

	for _, withFog := range []bool{false, true} {
		for _, scene := range []struct {
			name   string
			cam    Camera
			meshes []Mesh
		}{{"many cubes", cam, cubes}, {"floor", floorCam, []Mesh{floor}}} {
			cam := scene.cam
			if withFog {
				cam.Fog = fog
			}
			for _, mode := range []string{"shaded", "pixels"} {
				b.Run(fmt.Sprintf("%s %s fog %v", scene.name, mode, withFog), func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						fb.Clear()
						for _, m := range scene.meshes {
							if mode == "shaded" {
								m.DrawShaded(fb, cam, lights...)
							} else {
								m.DrawShadedPixels(fb.Canvas(), cam, lights...)
							}
						}
					}
				})
			}
		}
	}
}

// BenchmarkDrawBig draws one big smooth sphere (about 16 thousand faces) where it is
// all in view, where half of it is off the right edge, and where it is behind the
// camera, as a wireframe and shaded; and the same sphere with its vertices written where
// they are in the world, as a level built in place is, behind the camera, with the bounds that
// UpdateBounds keeps and without them. A model that is fully in view must not get slower
// for the sake of one that is not, and one written in the world should be as cheap to skip
// as one that is placed.
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

	// Written in the world, behind the camera: a Transform of nothing, the vertices in place.
	written := atTheWorld(Sphere(3, 128, 64), geometry.Vector3{Z: -10})
	for _, kept := range []bool{true, false} {
		ball := written
		name := "behind the camera, written in the world, without bounds"
		if kept {
			ball.UpdateBounds()
			name = "behind the camera, written in the world, with bounds"
		}
		for _, mode := range []string{"wireframe", "shaded"} {
			b.Run(name+" "+mode, func(b *testing.B) {
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

// BenchmarkDrawLamps draws a corridor of ten smooth floor chunks, every cell lit, by sixteen lamps
// along it: lamps that each reach only the chunks near them, as a level of many lamps has, and lamps
// that reach every chunk, where nothing can be left out. "no bounds" is the same chunks without the
// bounds that UpdateBounds keeps, which asks every lamp about every chunk, as it was before lights were
// left out. With the bounds the first should cost much less, and the second no more.
func BenchmarkDrawLamps(b *testing.B) {
	cam := Camera{Position: geometry.Vector3{Y: 2}, Pitch: -0.3, FOV: 1.4, Near: 0.1}
	fb, err := engine.NewFrameBuffer(80, 24)
	if err != nil {
		b.Fatal(err)
	}
	for _, reach := range []struct {
		name string
		rng  float64
	}{{"each reaching few chunks", 6}, {"each reaching every chunk", 1000}} {
		var lights []Light
		for i := range 16 {
			lights = append(lights, Point{Position: geometry.Vector3{X: float64(i%2*2 - 1), Y: 2, Z: 2 + float64(i)*4}, Range: reach.rng})
		}
		for _, bounds := range []bool{true, false} {
			var chunks []Mesh
			for i := range 10 {
				c := Plane(6, 6)
				c.Transform.Position = geometry.Vector3{Z: 3 + float64(i)*6}
				c.Material.Color = engine.Palette(33)
				if !bounds {
					c.bounds = nil
				}
				chunks = append(chunks, c)
			}
			name := reach.name + ", bounds"
			if !bounds {
				name = reach.name + ", no bounds"
			}
			for _, pixels := range []bool{false, true} {
				kind := "cells"
				if pixels {
					kind = "pixels"
				}
				b.Run(name+" "+kind, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						fb.Clear()
						for _, c := range chunks {
							if pixels {
								c.DrawShadedPixels(fb.Canvas(), cam, lights...)
							} else {
								c.DrawShaded(fb, cam, lights...)
							}
						}
					}
				})
			}
		}
	}
}

// BenchmarkDrawTextured draws a wall that shows a picture of 16 by 16 pixels: as two triangles
// with the picture on them, and as the grid of 512 small triangles, one for each pixel, that a wall
// had to be made of when a face had one color. Both show the same picture; the first should cost
// much less.
func BenchmarkDrawTextured(b *testing.B) {
	rng := rand.New(rand.NewPCG(31, 7))
	pic := bitmap.New(16, 16)
	for y := range 16 {
		for x := range 16 {
			pic.Set(x, y, engine.RGB(uint8(40+rng.IntN(200)), uint8(40+rng.IntN(200)), uint8(40+rng.IntN(200))))
		}
	}
	const wallW, wallH = 3.2, 2.0

	// The wall as two triangles with the picture on them.
	two := Mesh{
		Vertices: []geometry.Vector3{{X: -wallW / 2, Y: -wallH / 2}, {X: -wallW / 2, Y: wallH / 2}, {X: wallW / 2, Y: wallH / 2}, {X: wallW / 2, Y: -wallH / 2}},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		FaceTexCoords: [][3]geometry.Vector2{
			{{X: 0, Y: 1}, {X: 0, Y: 0}, {X: 1, Y: 0}},
			{{X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}},
		},
		Material: Material{Texture: pic},
	}
	two.UpdateBounds()

	// The wall as a grid of faces, each with the color of one pixel.
	grid := Mesh{}
	for y := 0; y <= 16; y++ {
		for x := 0; x <= 16; x++ {
			grid.Vertices = append(grid.Vertices, geometry.Vector3{X: -wallW/2 + wallW*float64(x)/16, Y: wallH/2 - wallH*float64(y)/16})
		}
	}
	for y := range 16 {
		for x := range 16 {
			a, d := y*17+x, (y+1)*17+x // the top left and the bottom left of the pixel
			grid.Faces = append(grid.Faces, [3]int{d, a, a + 1}, [3]int{d, a + 1, d + 1})
			grid.FaceColors = append(grid.FaceColors, pic.At(x, y), pic.At(x, y))
		}
	}
	grid.UpdateBounds()

	cam := Camera{FOV: math.Pi / 2, Near: 0.1}
	lights := []Light{Ambient{Level: 0.2}, Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}}
	for _, size := range []struct{ w, h int }{{80, 24}, {200, 60}} {
		fb, err := engine.NewFrameBuffer(size.w, size.h)
		if err != nil {
			b.Fatal(err)
		}
		for _, wall := range []struct {
			name string
			m    Mesh
		}{{"textured", two}, {"grid of faces", grid}} {
			m := wall.m
			m.Transform.Position = geometry.Vector3{Z: 3}
			for _, pixels := range []bool{false, true} {
				kind := "cells"
				if pixels {
					kind = "pixels"
				}
				b.Run(fmt.Sprintf("%s %s %dx%d", wall.name, kind, size.w, size.h), func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						fb.Clear()
						if pixels {
							m.DrawShadedPixels(fb.Canvas(), cam, lights...)
						} else {
							m.DrawShaded(fb, cam, lights...)
						}
					}
				})
			}
		}
	}
}

// BenchmarkDrawSeeThrough draws a fence, a wall that shows a picture of 16 by 16 pixels, half of
// them holes in alternate columns, as two triangles with the picture on them, and as the faces that
// such a wall had to be cut into when a face had one color: one for each pixel that is not a hole.
// And a pane of glass in front of a solid wall, against the same pane solid. The fence should cost
// much less cut as two triangles, and glass should cost about what a solid does.
func BenchmarkDrawSeeThrough(b *testing.B) {
	rng := rand.New(rand.NewPCG(51, 7))
	pic := bitmap.New(16, 16)
	for y := range 16 {
		for x := 0; x < 16; x += 2 { // the odd columns stay see-through
			pic.Set(x, y, engine.RGB(uint8(40+rng.IntN(200)), uint8(40+rng.IntN(200)), uint8(40+rng.IntN(200))))
		}
	}
	const wallW, wallH = 3.2, 2.0

	fence := Mesh{
		Vertices: []geometry.Vector3{{X: -wallW / 2, Y: -wallH / 2}, {X: -wallW / 2, Y: wallH / 2}, {X: wallW / 2, Y: wallH / 2}, {X: wallW / 2, Y: -wallH / 2}},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		FaceTexCoords: [][3]geometry.Vector2{
			{{X: 0, Y: 1}, {X: 0, Y: 0}, {X: 1, Y: 0}},
			{{X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}},
		},
		Material: Material{Texture: pic},
	}
	fence.UpdateBounds()

	cut := Mesh{} // a face for each pixel that is there
	for y := 0; y <= 16; y++ {
		for x := 0; x <= 16; x++ {
			cut.Vertices = append(cut.Vertices, geometry.Vector3{X: -wallW/2 + wallW*float64(x)/16, Y: wallH/2 - wallH*float64(y)/16})
		}
	}
	for y := range 16 {
		for x := 0; x < 16; x += 2 {
			a, d := y*17+x, (y+1)*17+x
			cut.Faces = append(cut.Faces, [3]int{d, a, a + 1}, [3]int{d, a + 1, d + 1})
			cut.FaceColors = append(cut.FaceColors, pic.At(x, y), pic.At(x, y))
		}
	}
	cut.UpdateBounds()

	// Glass in front of a wall, and the same pane solid.
	wall := Mesh{
		Vertices: []geometry.Vector3{{X: -6, Y: -4, Z: 6}, {X: -6, Y: 4, Z: 6}, {X: 6, Y: 4, Z: 6}, {X: 6, Y: -4, Z: 6}},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		Material: Material{Color: engine.RGB(200, 120, 60)},
	}
	wall.UpdateBounds()
	pane := Mesh{
		Vertices: []geometry.Vector3{{X: -wallW / 2, Y: -wallH / 2, Z: 3}, {X: -wallW / 2, Y: wallH / 2, Z: 3}, {X: wallW / 2, Y: wallH / 2, Z: 3}, {X: wallW / 2, Y: -wallH / 2, Z: 3}},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		Material: Material{Color: engine.RGB(0, 100, 240), Opacity: 0.4},
	}
	pane.UpdateBounds()
	solid := pane
	solid.Material.Opacity = 0

	cam := Camera{FOV: math.Pi / 2, Near: 0.1}
	lights := []Light{Ambient{Level: 0.2}, Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}}
	for _, size := range []struct{ w, h int }{{80, 24}, {200, 60}} {
		fb, err := engine.NewFrameBuffer(size.w, size.h)
		if err != nil {
			b.Fatal(err)
		}
		for _, scene := range []struct {
			name   string
			meshes []Mesh
		}{
			{"fence", []Mesh{fence}},
			{"fence cut in faces", []Mesh{cut}},
			{"glass over a wall", []Mesh{wall, pane}},
			{"solid over a wall", []Mesh{wall, solid}},
		} {
			meshes := make([]Mesh, len(scene.meshes))
			for i, m := range scene.meshes {
				m.Transform.Position = geometry.Vector3{Z: 0}
				if scene.name == "fence" || scene.name == "fence cut in faces" {
					m.Transform.Position = geometry.Vector3{Z: 3}
				}
				meshes[i] = m
			}
			for _, pixels := range []bool{false, true} {
				kind := "cells"
				if pixels {
					kind = "pixels"
				}
				b.Run(fmt.Sprintf("%s %s %dx%d", scene.name, kind, size.w, size.h), func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						fb.Clear()
						for _, m := range meshes {
							if pixels {
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
}

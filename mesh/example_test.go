package mesh_test

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"strings"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/mesh"
)

func ExampleLoadOBJ() {
	const square = `# a flat rectangle, 4 wide and 2 tall
v 0 0 0
v 4 0 0
v 4 2 0
v 0 2 0
f 1 2 3 4
`
	m, err := mesh.LoadOBJ(strings.NewReader(square))
	if err != nil {
		panic(err)
	}
	m = m.Fit(2) // centered on the origin, longest side 2

	lo, hi := m.Bounds()
	size := hi.Sub(lo)
	fmt.Printf("%d vertices, %d edges, %d triangles\n", len(m.Vertices), len(m.Edges), len(m.Faces))
	fmt.Printf("%.1f wide, %.1f tall\n", size.X, size.Y)
	fmt.Println(m.Validate())
	// Output:
	// 4 vertices, 4 edges, 2 triangles
	// 2.0 wide, 1.0 tall
	// <nil>
}

// A Camera left at zero sees 90 degrees from the origin, looking down +z.
func ExampleMesh_Draw() {
	fb, err := engine.NewFrameBuffer(30, 11)
	if err != nil {
		panic(err)
	}
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})

	cube := mesh.Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 3} // three units in front of the camera
	cube.Draw(fb, mesh.Camera{}, engine.Cell{Character: '#'})

	fmt.Println(fb)
	// Output:
	// ┌────────────────────────────┐
	// │                            │
	// │        ############        │
	// │        ###      ###        │
	// │        #  ######  #        │
	// │        #  #    #  #        │
	// │        #  ######  #        │
	// │        ###      ###        │
	// │        ############        │
	// │                            │
	// └────────────────────────────┘
}

func ExampleMesh_DrawShaded() {
	fb, err := engine.NewFrameBuffer(30, 11)
	if err != nil {
		panic(err)
	}
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})

	cube := mesh.Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 3}
	cube.Transform.Yaw = 0.6 // turn it so that two sides show
	cube.Material.Color = engine.Palette(208)
	cube.DrawShaded(fb, mesh.Camera{},
		mesh.Ambient{Level: 0.2},
		mesh.Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}})

	fmt.Println(fb)
	// Output:
	// ┌────────────────────────────┐
	// │                            │
	// │              **            │
	// │        ********--          │
	// │        ********---         │
	// │        ********---         │
	// │        ********---         │
	// │        ********--          │
	// │              **            │
	// │                            │
	// └────────────────────────────┘
}

func ExampleMesh_Validate() {
	m := mesh.Mesh{
		Vertices: []geometry.Vector3{{}, {X: 1}, {Y: 1}},
		Faces:    [][3]int{{0, 1, 5}}, // there is no vertex 5
	}

	fmt.Println(m.Validate())
	// Output:
	// mesh: face 0 uses vertex 5, but only vertices 0 to 2 exist
}

func ExampleMesh_Clone() {
	a := mesh.Cube(2)
	b := a.Clone()
	b.Vertices[0].X = 99

	fmt.Println(a.Vertices[0].X, b.Vertices[0].X)
	// Output:
	// -1 99
}

func ExampleCamera_Project() {
	cam := mesh.Camera{Position: geometry.Vector3{Z: -5}}

	x, y, ok := cam.Project(geometry.Vector3{X: 1}, 80, 24)
	fmt.Printf("%.1f %.1f %v\n", x, y, ok)

	_, _, ok = cam.Project(geometry.Vector3{Z: -10}, 80, 24) // behind the camera
	fmt.Println(ok)
	// Output:
	// 44.8 12.0 true
	// false
}

// A marker, a billboard or a particle drawn into the canvas of the solids takes its pixel and
// its depth from ProjectPixels, so that a solid in front of it hides it and it hides one
// behind it. Floor the pixel with math.Floor.
func ExampleCamera_ProjectPixels() {
	fb, err := engine.NewFrameBuffer(40, 12)
	if err != nil {
		panic(err)
	}
	canvas := fb.Canvas()
	cam := mesh.Camera{} // at the origin, looking down +z

	cube := mesh.Cube(2)
	cube.Transform.Position = geometry.Vector3{Z: 5} // its near face is 4 away
	cube.Material.Color = engine.RGB(200, 100, 50)
	cube.DrawShadedPixels(canvas, cam, mesh.Ambient{Level: 1})

	marker := engine.RGB(0, 255, 0)
	for _, z := range []float64{3, 7} { // in front of the cube, and behind it
		x, y, depth, ok := cam.ProjectPixels(geometry.Vector3{Z: z}, canvas)
		if !ok {
			continue
		}
		drawn := canvas.SetDepth(int(math.Floor(x)), int(math.Floor(y)), depth, marker)
		fmt.Printf("z=%.0f: pixel (%.0f, %.0f), depth %.0f, drawn: %v\n", z, x, y, depth, drawn)
	}
	// Output:
	// z=3: pixel (20, 12), depth 3, drawn: true
	// z=7: pixel (20, 12), depth 7, drawn: false
}

// The ray through a place on the screen is the reverse of Project. Here it finds what is under
// the middle of the screen: a camera 1.5 up, looking down at the floor 6 ahead, and the place on
// the floor (y = 0) that the player is looking at, which is where to put something down. The
// direction is one deep along Look, so t is also how deep that place is.
func ExampleCamera_Ray() {
	cam := mesh.Camera{Position: geometry.Vector3{Y: 1.5}, Pitch: -math.Atan2(1.5, 6)}

	origin, direction, ok := cam.Ray(40, 12, 80, 24) // the middle of a screen of 80 by 24 cells
	if !ok {
		return
	}
	t := -origin.Y / direction.Y // how far along the ray the floor is
	floor := origin.Add(direction.Scale(t))
	fmt.Printf("%.2f ahead, %.2f deep\n", floor.Z, t)
	// Output:
	// 6.00 ahead, 6.18 deep
}

// A camera that is told what to look at, one that follows a thing, watches a door or circles a
// model, is pointed at it with LookAt, and Project then puts the thing in the middle.
func ExampleCamera_LookAt() {
	cam := mesh.Camera{Position: geometry.Vector3{X: 4, Y: 3, Z: -4}}
	cam.LookAt(geometry.Vector3{}) // the origin

	fmt.Printf("yaw %.2f, pitch %.2f\n", cam.Yaw, cam.Pitch)
	x, y, ok := cam.Project(geometry.Vector3{}, 80, 24)
	fmt.Printf("%.1f %.1f %v\n", x, y, ok)
	// Output:
	// yaw -0.79, pitch -0.49
	// 40.0 12.0 true
}

// A fog fades what is deeper toward its color: nothing before Start, the fog's color from End,
// and a straight line between. A camera with a Fog fades every shaded mesh it draws by it, and a
// program that draws things of its own calls Apply to fade them in step.
func ExampleFog() {
	fog := mesh.Fog{Color: engine.RGB(40, 40, 60), Start: 10, End: 30}
	red := engine.RGB(200, 0, 0)
	for _, depth := range []float64{5, 10, 20, 30, 50} {
		r, g, b := fog.Apply(red, depth).RGB()
		fmt.Printf("%2.0f deep: %3d %3d %3d\n", depth, r, g, b)
	}
	// Output:
	//  5 deep: 200   0   0
	// 10 deep: 200   0   0
	// 20 deep: 120  20  30
	// 30 deep:  40  40  60
	// 50 deep:  40  40  60
}

// A Transform scales a model, turns it by Roll, Pitch and Yaw in that order, and moves
// it to Position. Apply says where a point of the model ends up.
func ExampleTransform() {
	t := mesh.Transform{
		Position: geometry.Vector3{X: 10},
		Yaw:      math.Pi / 2, // swings the front, +z, around to +x
		Scale:    2,
	}

	p := t.Apply(geometry.Vector3{Z: 1}) // a point one unit in front of the model
	fmt.Printf("%.1f %.1f %.1f\n", p.X, p.Y, p.Z)
	// Output:
	// 12.0 0.0 0.0
}

// A Material gives a mesh its color and the characters that show how bright it is.
func ExampleMaterial() {
	fb, err := engine.NewFrameBuffer(30, 11)
	if err != nil {
		panic(err)
	}

	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})

	cube := mesh.Cube(2)
	cube.Transform = mesh.Transform{Position: geometry.Vector3{Z: 3}, Yaw: 0.6}
	cube.Material = mesh.Material{Color: engine.Palette(208), Ramp: ".oO@"}
	cube.DrawShaded(fb, mesh.Camera{},
		mesh.Ambient{Level: 0.2},
		mesh.Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}})

	fmt.Println(fb)
	// Output:
	// ┌────────────────────────────┐
	// │                            │
	// │              OO            │
	// │        OOOOOOOOoo          │
	// │        OOOOOOOOooo         │
	// │        OOOOOOOOooo         │
	// │        OOOOOOOOooo         │
	// │        OOOOOOOOoo          │
	// │              OO            │
	// │                            │
	// └────────────────────────────┘
}

// A picture on a wall: Material.Texture is the picture and Mesh.FaceTexCoords says which place in
// it each corner of each face is pinned to, so two triangles show a picture of any size, and every
// cell takes the color of the picture where it is.
func ExampleMaterial_texture() {
	// A picture of two pixels side by side, red and blue.
	flag := bitmap.New(2, 1)
	flag.Set(0, 0, engine.RGB(255, 0, 0))
	flag.Set(1, 0, engine.RGB(0, 0, 255))

	// (0, 0) is the top left of the picture and (1, 1) the bottom right.
	wall := mesh.Mesh{
		Vertices: []geometry.Vector3{{X: -2, Y: -1}, {X: -2, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: -1}},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		FaceTexCoords: [][3]geometry.Vector2{
			{{X: 0, Y: 1}, {X: 0, Y: 0}, {X: 1, Y: 0}},
			{{X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}},
		},
		Material: mesh.Material{Texture: flag},
	}
	wall.Transform.Position = geometry.Vector3{Z: 3}

	fb, err := engine.NewFrameBuffer(40, 12)
	if err != nil {
		panic(err)
	}
	wall.DrawShaded(fb, mesh.Camera{}, mesh.Ambient{Level: 1})

	// The left half of the wall is the red pixel and the right half the blue one.
	fmt.Println(fb.At(14, 5).Foreground, fb.At(25, 5).Foreground)
	// Output:
	// #ff0000 #0000ff
}

// examplePanel is a rectangle w wide and h tall facing the camera at depth z, as two triangles.
func examplePanel(z, w, h float64) mesh.Mesh {
	return mesh.Mesh{
		Vertices:    []geometry.Vector3{{X: -w / 2, Y: -h / 2, Z: z}, {X: -w / 2, Y: h / 2, Z: z}, {X: w / 2, Y: h / 2, Z: z}, {X: w / 2, Y: -h / 2, Z: z}},
		Faces:       [][3]int{{0, 1, 2}, {0, 2, 3}},
		DoubleSided: true,
	}
}

// A mesh can be seen through in two ways. A pixel of its picture that is see-through is a hole, where
// nothing is drawn and what is behind shows, as in a fence. And Material.Opacity mixes the whole mesh
// over what is already drawn, as glass does, so the solid things are drawn first and then the
// see-through ones, from far to near. This draws a wall, a fence in front of it and a pane of glass in
// front of both, and writes the picture to the path in GLONK_SCREENSHOT, if there is one.
func ExampleMaterial_opacity() {
	fb, err := engine.NewFrameBuffer(48, 14)
	if err != nil {
		panic(err)
	}
	canvas := fb.Canvas()
	cam := mesh.Camera{}
	light := mesh.Ambient{Level: 1}

	// A wall of orange.
	wall := examplePanel(6, 9, 5)
	wall.Material.Color = engine.RGB(200, 100, 40)
	wall.DrawShadedPixels(canvas, cam, light)

	// A fence: a picture of four pixels, bars where they are set and holes where they are not.
	bars := bitmap.New(4, 1)
	bars.Set(0, 0, engine.RGB(120, 60, 20))
	bars.Set(2, 0, engine.RGB(120, 60, 20))
	fence := examplePanel(4, 6, 3)
	fence.FaceTexCoords = [][3]geometry.Vector2{
		{{X: 0, Y: 1}, {X: 0, Y: 0}, {X: 1, Y: 0}},
		{{X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}},
	}
	fence.Material.Texture = bars
	fence.Transform.Position = geometry.Vector3{X: -1}
	fence.DrawShadedPixels(canvas, cam, light)

	// Glass last, 40% blue, over the wall, the fence and the empty space beside them.
	glass := examplePanel(3, 3, 3)
	glass.Material = mesh.Material{Color: engine.RGB(0, 100, 240), Opacity: 0.4}
	glass.Transform.Position = geometry.Vector3{X: 1.6}
	glass.DrawShadedPixels(canvas, cam, light)

	fmt.Println("a bar of the fence:", canvas.At(10, 9))
	fmt.Println("the wall through a hole:", canvas.At(16, 9))
	fmt.Println("glass over the wall:", canvas.At(26, 9))
	fmt.Println("glass over nothing:", canvas.At(36, 9))

	if path := os.Getenv("GLONK_SCREENSHOT"); path != "" {
		canvas.Compose()
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := png.Encode(f, fb.Image(engine.ImageOptions{Scale: 10})); err != nil {
			panic(err)
		}
	}
	// Output:
	// a bar of the fence: #783c14
	// the wall through a hole: #c86428
	// glass over the wall: #786478
	// glass over nothing: #002860
}

// A line that lies on a surface, a grid on a floor or a box round the thing that is selected, ties with
// the surface under DrawDepth and is broken wherever it is a hair behind it, which across a floor seen at
// a low angle is much of it. DrawOnSurface draws it whole, and a solid that is nearer still hides it. This
// draws a grid on a floor both ways, in pixels, counts what each keeps, and writes the two pictures side by
// side to the path in GLONK_SCREENSHOT, if there is one.
func ExampleMesh_DrawOnSurface() {
	floor := mesh.Plane(60, 60)
	floor.Material.Color = engine.RGB(60, 80, 60)
	grid := mesh.Grid(24, 12)
	grid.Transform.Position = geometry.Vector3{Z: 14}
	box := mesh.Cube(2)
	box.Material.Color = engine.RGB(200, 140, 60)
	box.Transform.Position = geometry.Vector3{X: 1.5, Y: 1, Z: 6}
	cam := mesh.Camera{Position: geometry.Vector3{Y: 1.6}, Pitch: -0.12, FOV: math.Pi / 3}
	light := mesh.Ambient{Level: 1}
	line := engine.RGB(255, 0, 255)

	scene := func(drawGrid func(*engine.Canvas)) *engine.FrameBuffer {
		fb, err := engine.NewFrameBuffer(80, 24)
		if err != nil {
			panic(err)
		}
		canvas := fb.Canvas()
		floor.DrawShadedPixels(canvas, cam, light)
		box.DrawShadedPixels(canvas, cam, light)
		drawGrid(canvas)
		canvas.Compose()
		return fb
	}
	count := func(fb *engine.FrameBuffer) (n int) {
		img := fb.Image(engine.ImageOptions{})
		r, g, b := line.RGB()
		for y := range img.Bounds().Dy() {
			for x := range img.Bounds().Dx() {
				if p := img.RGBAAt(x, y); p.R == r && p.G == g && p.B == b {
					n++
				}
			}
		}
		return n
	}
	// The whole grid, with no depth, on an empty frame: every pixel it has, behind the box too.
	alone, err := engine.NewFrameBuffer(80, 24)
	if err != nil {
		panic(err)
	}
	grid.DrawPixels(alone.Canvas(), cam, line)
	alone.Canvas().Compose()

	byDepth := scene(func(c *engine.Canvas) { grid.DrawDepthPixels(c, cam, line) })
	onSurface := scene(func(c *engine.Canvas) { grid.DrawOnSurfacePixels(c, cam, line) })
	fmt.Println("the whole grid:", count(alone), "pixels, some of them behind the box")
	fmt.Println("drawn by DrawDepthPixels:", count(byDepth))
	fmt.Println("drawn by DrawOnSurfacePixels:", count(onSurface))

	if path := os.Getenv("GLONK_SCREENSHOT"); path != "" {
		left, right := byDepth.Image(engine.ImageOptions{Scale: 4}), onSurface.Image(engine.ImageOptions{Scale: 4})
		both := image.NewRGBA(image.Rect(0, 0, left.Bounds().Dx()*2+8, left.Bounds().Dy()))
		draw.Draw(both, left.Bounds(), left, image.Point{}, draw.Src)
		draw.Draw(both, left.Bounds().Add(image.Point{X: left.Bounds().Dx() + 8}), right, image.Point{}, draw.Src)
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := png.Encode(f, both); err != nil {
			panic(err)
		}
	}
	// Output:
	// the whole grid: 707 pixels, some of them behind the box
	// drawn by DrawDepthPixels: 411
	// drawn by DrawOnSurfacePixels: 580
}

// spot is a light of your own: a spotlight, which lights what is inside a cone from
// where it is and nothing outside it.
type spot struct {
	position, direction geometry.Vector3 // where it is, and the unit vector it points along
	cos                 float64          // cosine of the angle from the middle of the beam to its edge
}

func (s spot) Brightness(point, normal geometry.Vector3) float64 {
	toSurface := point.Sub(s.position).Normalize()
	if toSurface.Dot(s.direction) < s.cos {
		return 0 // outside the beam
	}
	return max(0, normal.Dot(toSurface.Neg()))
}

// Any type with a Brightness method is a Light. Lights add up without passing full
// brightness.
func ExampleLight() {
	fb, err := engine.NewFrameBuffer(30, 11)
	if err != nil {
		panic(err)
	}

	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})

	// A sphere has normals, so the beam picks out a round patch of it.
	ball := mesh.Sphere(1.5, 24, 12)
	ball.Transform.Position = geometry.Vector3{Z: 3}
	beam := spot{
		position:  geometry.Vector3{X: -3, Y: 2, Z: 0},
		direction: geometry.Vector3{X: 3.6, Y: -2.2, Z: 3}.Normalize(), // at the sphere, a little off its middle
		cos:       math.Cos(0.2),
	}
	ball.DrawShaded(fb, mesh.Camera{}, mesh.Ambient{Level: 0.1}, beam)

	fmt.Println(fb)
	// Output:
	// ┌────────────────────────────┐
	// │                            │
	// │            #:::            │
	// │         @%%#*+::::         │
	// │        @%%#**::::::        │
	// │        %##:::::::::        │
	// │        ::::::::::::        │
	// │         ::::::::::         │
	// │            ::::            │
	// │                            │
	// └────────────────────────────┘
}

// A Point light is a lamp: it lights what is near it, brightest straight below it, and
// fades out at its Range. Because a Plane has normals, every cell is lit by itself.
func ExamplePoint() {
	fb, err := engine.NewFrameBuffer(40, 12)
	if err != nil {
		panic(err)
	}
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})

	floor := mesh.Plane(10, 8)
	floor.Transform.Position = geometry.Vector3{Z: 5}
	cam := mesh.Camera{Position: geometry.Vector3{Y: 4}, Pitch: -0.8} // high up, looking down
	lamp := mesh.Point{Position: geometry.Vector3{X: -1.5, Y: 1.2, Z: 5}, Range: 6}
	floor.DrawShaded(fb, cam, mesh.Ambient{Level: 0.05}, lamp)

	fmt.Println(fb)
	// Output:
	// ┌──────────────────────────────────────┐
	// │                                      │
	// │                                      │
	// │                                      │
	// │           :::--=---:::....           │
	// │         ::--=+**+=--::::....         │
	// │       :::::-------::::::......       │
	// │      :::::::::::::::::::.......      │
	// │    ..:::::::::::::::::...........    │
	// │                                      │
	// │                                      │
	// └──────────────────────────────────────┘
}

// A box shows a part of one picture on each face: here an atlas of six colors, a face each. A
// camera looking down +z sees the box's Back, which faces it.
func ExampleBoxFaces() {
	atlas := bitmap.New(3, 2)
	for i, c := range []engine.Color{
		engine.RGB(255, 0, 0), engine.RGB(0, 255, 0), engine.RGB(0, 0, 255),
		engine.RGB(255, 255, 0), engine.RGB(255, 0, 255), engine.RGB(0, 255, 255),
	} {
		atlas.Set(i%3, i/3, c)
	}
	part := func(i int) geometry.Rect { return geometry.NewRect(float64(i%3)/3, float64(i/3)/2, 1.0/3, 0.5) }

	box := mesh.BoxFaces(2, 2, 2, mesh.BoxParts{
		Front: part(0), Back: part(1), Left: part(2), Right: part(3), Top: part(4), Bottom: part(5),
	})
	box.Material.Texture = atlas
	box.Transform.Position = geometry.Vector3{Z: 5}

	fb, err := engine.NewFrameBuffer(40, 20)
	if err != nil {
		panic(err)
	}
	canvas := fb.Canvas()
	box.DrawShadedPixels(canvas, mesh.Camera{}, mesh.Ambient{Level: 1})
	fmt.Println(canvas.At(20, 20)) // the middle of the screen: the Back's part, green
	// Output:
	// #00ff00
}

func ExampleSphere() {
	fb, err := engine.NewFrameBuffer(30, 13)
	if err != nil {
		panic(err)
	}
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})

	// A sphere comes with normals, so it is shaded smooth.
	ball := mesh.Sphere(1.6, 24, 12)
	ball.Transform.Position = geometry.Vector3{Z: 2.7}
	ball.DrawShaded(fb, mesh.Camera{},
		mesh.Ambient{Level: 0.1},
		mesh.Directional{Direction: geometry.Vector3{X: 0.5, Y: -0.7, Z: 0.5}})

	fmt.Println(fb)
	// Output:
	// ┌────────────────────────────┐
	// │                            │
	// │         @%%%%####*         │
	// │       %%%%%####****+       │
	// │      %%%#####****+++=      │
	// │     %#####*****++++==-     │
	// │     ###******++++===--     │
	// │     *****++++++===---:     │
	// │      ++++++=====---::      │
	// │       ======----::::       │
	// │         :---::::::         │
	// │                            │
	// └────────────────────────────┘
}

func ExampleCylinder() {
	can := mesh.Cylinder(1, 2, 16)

	fmt.Println(len(can.Vertices), "vertices,", len(can.Faces), "faces,", len(can.Edges), "edges")
	fmt.Println(can.Validate())
	// Output:
	// 66 vertices, 64 faces, 48 edges
	// <nil>
}

func ExampleCone() {
	cone := mesh.Cone(1, 2, 16)
	lo, hi := cone.Bounds()

	fmt.Println(len(cone.Vertices), "vertices,", len(cone.Faces), "faces")
	fmt.Println("base at y", lo.Y, "and tip at y", hi.Y)
	// Output:
	// 49 vertices, 32 faces
	// base at y -1 and tip at y 1
}

func ExamplePlane() {
	floor := mesh.Plane(10, 6)
	floor.Transform.Position = geometry.Vector3{Y: -1.5}

	lo, hi := floor.Bounds()
	fmt.Println("x from", lo.X, "to", hi.X, "and z from", lo.Z, "to", hi.Z)
	fmt.Println(floor.Normals[0])
	// Output:
	// x from -5 to 5 and z from -3 to 3
	// {0 1 0}
}

// DrawDepth is Draw with depth: a wireframe is hidden by solids in front of it, whichever
// is drawn first.
func ExampleMesh_DrawDepth() {
	fb, err := engine.NewFrameBuffer(50, 15)
	if err != nil {
		panic(err)
	}
	cam := mesh.Camera{Position: geometry.Vector3{Y: 2.5}, Pitch: -0.5, FOV: 0.9}

	cube := mesh.Cube(2.4)
	cube.Transform.Position = geometry.Vector3{X: -0.8, Z: 5}
	floor := mesh.Grid(9, 6)
	floor.Transform.Position = geometry.Vector3{Y: -1.2, Z: 6}

	// The floor comes second. Draw would run its lines on across the cube; DrawDepth
	// stops them at the cube's edge, because the cube is nearer.
	cube.DrawShaded(fb, cam, mesh.Ambient{Level: 0.2}, mesh.Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}})
	floor.DrawDepth(fb, cam, engine.Cell{Character: '+'})

	fmt.Println(fb)
	// Output:
	//              %%%%%%%%%%%%%%
	//           %%%%%%%%%%%%%%%%%%+++++++++++
	//          +******************+++++++++++++
	//        +++******************+++++++++++++++
	//      ++    *****************   +     +     ++
	//    ++++++++*****************+++++++++++++++++++
	//  ++      +  ****************    +       +      ++
	// ++++++++++++****************++++++++++++++++++++++
	//       ++     **************       +       ++
	//      +         +         +        +         +
	// ++++++++++++++++++++++++++++++++++++++++++++++++++
	//   +          +           +          +          +
}

package docs

import (
	"fmt"
	"math"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/mesh"
)

// sun is the light the scenes of this guide are lit by: a little from everywhere, and the rest
// from above, a little from the right and from behind the camera.
var sun = []mesh.Light{
	mesh.Ambient{Level: 0.2},
	mesh.Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}},
}

// A scene is a camera, meshes placed in the world, and lights.
func Example_scene() {
	fb, _ := engine.NewFrameBuffer(30, 12)
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})

	cam := mesh.Camera{Position: geometry.Vector3{Y: 1.5, Z: -3.5}}
	cam.LookAt(geometry.Vector3{}) // at the origin

	cube := mesh.Cube(2)
	cube.Transform.Yaw = 0.6 // turned, so that two sides show
	cube.Material.Color = engine.Palette(208)
	cube.DrawShaded(fb, cam, sun...)

	fmt.Println(fb)
	// Output:
	// ┌────────────────────────────┐
	// │                            │
	// │                            │
	// │                            │
	// │         %%%%%%%%%-         │
	// │         ******---          │
	// │         ******---          │
	// │          *****--           │
	// │             **-            │
	// │                            │
	// │                            │
	// └────────────────────────────┘
}

// A shape with a normal at each vertex is lit at every cell, so it looks round. One without is
// lit face by face.
func Example_smoothShading() {
	fb, _ := engine.NewFrameBuffer(40, 11)
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	cam := mesh.Camera{Position: geometry.Vector3{Z: -4}}

	ball := mesh.Sphere(1.3, 24, 12) // radius, segments around, rings from pole to pole
	ball.Transform.Position = geometry.Vector3{X: -1.6}
	ball.DrawShaded(fb, cam, sun...)

	facets := mesh.Sphere(1.3, 8, 5)
	facets.Normals = nil // lit face by face
	facets.Transform.Position = geometry.Vector3{X: 1.6}
	facets.DrawShaded(fb, cam, sun...)

	fmt.Println(fb)
	// Output:
	// ┌──────────────────────────────────────┐
	// │                                      │
	// │                                      │
	// │                                      │
	// │           %%##*+=   %@@@##           │
	// │          ***++=--  *****+++          │
	// │           =------   ------           │
	// │                                      │
	// │                                      │
	// │                                      │
	// └──────────────────────────────────────┘
}

// A lamp lights what is near it, and fades out at its Range.
func Example_lamp() {
	fb, _ := engine.NewFrameBuffer(40, 9)
	cam := mesh.Camera{Position: geometry.Vector3{Y: 3, Z: -2}, Pitch: -1}

	floor := mesh.Plane(12, 6)
	lamp := mesh.Point{Position: geometry.Vector3{X: -2.5, Y: 1}, Range: 4}
	floor.DrawShaded(fb, cam, mesh.Ambient{Level: 0.05}, lamp)
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{}) // a frame round the picture

	fmt.Println(fb)
	// Output:
	// ┌──────────────────────────────────────┐
	// │                                      │
	// │        ....:::::.............        │
	// │      .::::--=--::::............      │
	// │    .::::-=+*+=--:::..............    │
	// │  .:::::--===---::::................  │
	// │..:::::::----::::::...................│
	// │..::::::::::::::::....................│
	// └──────────────────────────────────────┘
}

// A scene in pixels is drawn into the canvas, and Compose turns the pixels into cells.
func Example_sceneInPixels() {
	fb, _ := engine.NewFrameBuffer(24, 7)
	cam := mesh.Camera{Position: geometry.Vector3{Y: 1.5, Z: -3.5}}
	cam.LookAt(geometry.Vector3{})

	cube := mesh.Cube(2)
	cube.Transform.Yaw = 0.6
	cube.Material.Color = engine.RGB(255, 135, 0)

	canvas := fb.Canvas()
	cube.DrawShadedPixels(canvas, cam, sun...)
	canvas.Compose()

	show(canvas)
	// Output:
	// ........................
	// ........................
	// ........................
	// ........................
	// ........................
	// .........######.........
	// .........#####..........
	// .........#####..........
	// .........#####..........
	// ...........##...........
	// ........................
	// ........................
	// ........................
	// ........................
}

// DrawDepth hides a wireframe behind nearer solids, whatever the order of drawing.
// DrawOnSurface draws lines that lie on a surface, such as a grid on a floor.
func Example_wireframes() {
	fb, _ := engine.NewFrameBuffer(42, 11)
	cam := mesh.Camera{Position: geometry.Vector3{Y: 2.5, Z: -4}, Pitch: -0.5, FOV: 1}

	floor := mesh.Plane(8, 6)
	floor.Transform.Position = geometry.Vector3{Y: -1}
	grid := mesh.Grid(8, 6)
	grid.Transform.Position = floor.Transform.Position
	cube := mesh.Cube(1.6)
	cube.Transform.Position = geometry.Vector3{X: 0.3, Y: -0.2}
	cube.Transform.Yaw = 0.5

	floor.DrawShaded(fb, cam, mesh.Ambient{Level: 0.1})
	grid.DrawOnSurface(fb, cam, engine.Cell{Character: '+'}) // on the floor, whole at any distance
	cube.DrawShaded(fb, cam, sun...)
	cube.DrawOnSurface(fb, cam, engine.Cell{Character: '#'}) // its edges, on its own faces
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})

	fmt.Println(fb)
	// Output:
	// ┌────────────────────────────────────────┐
	// │                                        │
	// │                                        │
	// │                                        │
	// │         +++++++++########+++++         │
	// │       +++++++++####%%%%##+++++++       │
	// │     +++++++++++#***####-#+++++++++     │
	// │   +++++++++++++##*****###+++++++++++   │
	// │ ++:::::+::::+::###***###:+::::+:::::++ │
	// │+++++++++++++++++++####+++++++++++++++++│
	// └────────────────────────────────────────┘
}

// Fog fades a shaded mesh toward a color by how deep it is, and a far plane ends the world.
func Example_fog() {
	cam := mesh.Camera{Position: geometry.Vector3{Y: 1.5}, Far: 30}
	cam.Fog = &mesh.Fog{Color: engine.RGB(40, 40, 60), Start: 4, End: 30}

	for _, depth := range []float64{2, 17, 30} {
		r, g, b := cam.Fog.Apply(engine.RGB(200, 200, 200), depth).RGB()
		fmt.Printf("%2.0f deep: %3d %3d %3d\n", depth, r, g, b)
	}
	// Output:
	// 2 deep: 200 200 200
	// 17 deep: 120 120 130
	// 30 deep:  40  40  60
}

// ProjectPixels says which pixel a point of the world is in and how deep it is. RayPixels is
// the way back: the ray through a pixel.
func Example_projectAndRays() {
	fb, _ := engine.NewFrameBuffer(40, 12)
	canvas := fb.Canvas() // 40 by 24 pixels
	cam := mesh.Camera{}  // at the origin, looking down +z

	x, y, depth, ok := cam.ProjectPixels(geometry.Vector3{X: 1, Y: 1, Z: 4}, canvas)
	fmt.Printf("in pixel (%.0f, %.0f), %.0f deep, on screen %v\n", math.Floor(x), math.Floor(y), depth, ok)

	origin, dir, _ := cam.RayPixels(20, 12, canvas) // the middle of the canvas
	fmt.Println("the ray starts at", origin, "and goes", dir)
	// Output:
	// in pixel (23, 9), 4 deep, on screen true
	// the ray starts at {0 0 0} and goes {0 0 1}
}

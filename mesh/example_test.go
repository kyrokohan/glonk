package mesh_test

import (
	"fmt"
	"math"
	"strings"

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

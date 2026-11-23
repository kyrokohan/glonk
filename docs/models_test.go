package docs

import (
	"embed"
	"fmt"
	"strings"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/mesh"
)

// A model is an OBJ file, read from any io.Reader: a file, an embedded asset, a download.
func Example_loadOBJ() {
	const pyramid = `# four triangles on a square
v -1 0 -1
v 1 0 -1
v 1 0 1
v -1 0 1
v 0 1.5 0
f 1 2 5
f 2 3 5
f 3 4 5
f 4 1 5
f 1 4 3 2
`
	model, err := mesh.LoadOBJ(strings.NewReader(pyramid))
	if err != nil {
		fmt.Println(err) // it names the line that went wrong: "obj line 12: bad number"
		return
	}
	model = model.Fit(2) // centered on the origin, its longest side 2
	model.Transform.Position = geometry.Vector3{Z: 4}

	fmt.Println(len(model.Faces), "triangles,", len(model.Edges), "edges")
	// Output:
	// 6 triangles, 8 edges
}

//go:embed testdata/crate.obj testdata/crate.mtl
var crateFiles embed.FS

// LoadOBJFS reads a model with its materials, from an fs.FS: an embed.FS, os.DirFS, a zip.
func Example_loadOBJFS() {
	crate, err := mesh.LoadOBJFS(crateFiles, "testdata/crate.obj", 1<<20)
	if err != nil {
		fmt.Println(err)
		return
	}

	wood, lid := crate.FaceColors[0], crate.FaceColors[len(crate.FaceColors)-1]
	fmt.Println("the sides:", wood, "the top:", lid)
	// Output:
	// the sides: #996633 the top: #e6cc4d
}

// A face fades from one corner's color to the next.
func Example_vertexColors() {
	fb, _ := engine.NewFrameBuffer(20, 8)
	cam := mesh.Camera{}

	tri := mesh.Mesh{
		Vertices:     []geometry.Vector3{{X: -2, Y: -1}, {X: 2, Y: -1}, {Y: 2}},
		Faces:        [][3]int{{0, 1, 2}},
		VertexColors: []engine.Color{engine.RGB(255, 0, 0), engine.RGB(0, 255, 0), engine.RGB(0, 0, 255)},
		DoubleSided:  true,
	}
	tri.Transform.Position = geometry.Vector3{Z: 3}

	canvas := fb.Canvas()
	tri.DrawShadedPixels(canvas, cam, mesh.Ambient{Level: 1})
	middle := geometry.Vector3{Y: 0, Z: 3}
	for _, corner := range tri.Vertices { // a little in from each corner
		near := corner.Add(tri.Transform.Position).Lerp(middle, 0.2)
		x, y, _, _ := cam.ProjectPixels(near, canvas)
		r, g, b := canvas.At(int(x), int(y)).RGB()
		fmt.Println(r, g, b)
	}
	// Output:
	// 232 17 5
	// 17 232 5
	// 1 25 228
}

// checker is a picture made in code: four squares, two of them holes.
func checker() *bitmap.Bitmap {
	pic := bitmap.New(2, 2)
	pic.Set(0, 0, engine.RGB(220, 220, 220))
	pic.Set(1, 1, engine.RGB(220, 220, 220))
	return pic
}

// A face shows a picture: FaceTexCoords pins each corner to a place in it.
func Example_texture() {
	fb, _ := engine.NewFrameBuffer(26, 10)
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	cam := mesh.Camera{}

	wall := mesh.Mesh{
		Vertices: []geometry.Vector3{{X: -2, Y: -1}, {X: -2, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: -1}},
		Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
		FaceTexCoords: [][3]geometry.Vector2{
			{{X: 0, Y: 1}, {X: 0, Y: 0}, {X: 2, Y: 0}}, // the bottom left, top left, top right,
			{{X: 0, Y: 1}, {X: 2, Y: 0}, {X: 2, Y: 1}}, // and the picture twice across
		},
		Material: mesh.Material{Texture: checker(), Sharp: true},
	}
	wall.Transform.Position = geometry.Vector3{Z: 2.2}
	wall.DrawShaded(fb, cam, mesh.Ambient{Level: 1})

	fmt.Println(fb)
	// Output:
	// ┌────────────────────────┐
	// │                        │
	// │                        │
	// │   @@@@     @@@@@       │
	// │   @@@@     @@@@@       │
	// │       @@@@@     @@@@   │
	// │       @@@@@     @@@@   │
	// │                        │
	// │                        │
	// └────────────────────────┘
}

// Opacity mixes a mesh's color over what is already drawn. Draw the solids first, then the
// see-through meshes from far to near.
func Example_seeThrough() {
	fb, _ := engine.NewFrameBuffer(24, 6)
	cam := mesh.Camera{}
	square := func(size, depth float64, material mesh.Material) mesh.Mesh {
		m := mesh.Mesh{
			Vertices: []geometry.Vector3{{X: -size, Y: -size}, {X: -size, Y: size}, {X: size, Y: size}, {X: size, Y: -size}},
			Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
			Material: material,
		}
		m.Transform.Position = geometry.Vector3{Z: depth}
		return m
	}
	wall := square(4, 6, mesh.Material{Color: engine.RGB(200, 0, 0)})
	pane := square(1, 3, mesh.Material{Color: engine.RGB(0, 0, 200), Opacity: 0.5}) // half blue glass

	canvas := fb.Canvas()
	wall.DrawShadedPixels(canvas, cam, mesh.Ambient{Level: 1}) // solids first
	pane.DrawShadedPixels(canvas, cam, mesh.Ambient{Level: 1}) // then see-through meshes, far to near

	at := func(p geometry.Vector3) engine.Color {
		x, y, _, _ := cam.ProjectPixels(p, canvas)
		return canvas.At(int(x), int(y))
	}
	fmt.Println("through the glass:", at(pane.Transform.Position), "beside it:", at(geometry.Vector3{X: 3, Z: 6}))
	// Output:
	// through the glass: #640064 beside it: #c80000
}

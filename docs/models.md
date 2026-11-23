# Models and textures

A mesh can come from an OBJ file, carry a color on each vertex, show a picture on its faces, or be see-through. This guide builds on [A 3D scene](3d.md).

## An OBJ file

```go
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
```

- **`LoadOBJ` reads any `io.Reader`:** a file, an embedded asset, a download.
- **An error names the line that went wrong.** Malformed input is an error, never a panic.
- **`Fit(n)`** centers a model on the origin, its longest side `n` long.
- **What is read:** vertices, with a color after them if the file has one; normals; faces of any number of corners; lines.
- **What is skipped:** groups and curves. Everything in the file becomes one mesh.
- **Faces with more than three corners** are cut into triangles for shading, and the wireframe keeps the polygons.
- **OBJ looks down -Z and glonk down +Z.** `LoadOBJ` flips Z and the winding, so a model is not mirrored. Its front faces a camera on the -Z side.
- **A model that looks inside out** was wound the other way. Set `DoubleSided` to draw both sides.

## With its materials

```go
//go:embed testdata/crate.obj testdata/crate.mtl
var crateFiles embed.FS
```

```go
	crate, err := mesh.LoadOBJFS(crateFiles, "testdata/crate.obj", 1<<20)
	if err != nil {
		fmt.Println(err)
		return
	}

	wood, lid := crate.FaceColors[0], crate.FaceColors[len(crate.FaceColors)-1]
	fmt.Println("the sides:", wood, "the top:", lid)
	// Output:
	// the sides: #996633 the top: #e6cc4d
```

- **`LoadOBJFS(fsys, name, maxBytes)`** reads a model from an `fs.FS`, and follows its `mtllib` to the `.mtl` file beside it.
- **Each face gets its material's color,** in `FaceColors`, still lit. Faces without one use `Material.Color`.
- **Only `Kd`, the color, and `map_Kd`, the picture, are read** from a material.
- **A name must be a plain path inside the model's folder.** A `.mtl` that is missing, too large or broken never fails the load.
- **`os.DirFS` follows symbolic links,** so give it only a folder you trust.

## Files that cannot be trusted

- **`LoadOBJLimit(r, maxBytes)`** fails with `ErrOBJTooLarge` past the limit. A plain `io.LimitReader` would quietly cut the model short.
- **`LoadOBJFS` bounds what it reads too:** the model, each `.mtl` and each picture.
- **The time taken grows in step with the size of the file.** Memory is about 25 times the file's size, so bound the input when you do not trust it.
- The documentation of [LoadOBJFS](https://pkg.go.dev/glonk.dev/mesh#LoadOBJFS) has every limit.

## A color on each vertex

```go
	tri := mesh.Mesh{
		Vertices:     []geometry.Vector3{{X: -2, Y: -1}, {X: 2, Y: -1}, {Y: 2}},
		Faces:        [][3]int{{0, 1, 2}},
		VertexColors: []engine.Color{engine.RGB(255, 0, 0), engine.RGB(0, 255, 0), engine.RGB(0, 0, 255)},
		DoubleSided:  true,
	}
	tri.Transform.Position = geometry.Vector3{Z: 3}
```

- **`VertexColors`** gives each vertex a color, and a face fades from one corner's color to the next, with perspective.
- **The blend is linear in red, green and blue,** with no gamma.
- **A face with a color of its own** is tinted by its vertex colors: each channel is multiplied.
- **Wireframes ignore vertex colors.**
- **In a file,** a vertex line of exactly six numbers, `v x y z r g b`, gives a color. The numbers go from 0 to 1.

## A picture on a face

```go
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
```

- **`Material.Texture`** is a picture, a `*bitmap.Bitmap`. **`FaceTexCoords`** pins each corner of each face to a place in it.
- **(0, 0) is the top left of the picture and (1, 1) the bottom right,** and the picture repeats: 2 is twice over.
- **Each cell takes the picture's nearest pixel,** seen at a slant, with no blending.
- **A pixel with an alpha under 128 is a hole:** nothing is drawn there, and what is behind shows. That is a leaf or a fence.
- **Far away, a face reads a smaller copy of its picture,** so a far wall does not crawl as you walk. `Material.Sharp` turns that off, for pixel art.
- **One picture for a mesh.** For two, make two meshes, or put both in one picture. Wireframes ignore it.
- **`Sphere`, `Cylinder`, `Cone` and `Plane` come with coordinates,** so setting `Texture` is enough.
- **`Box` has the whole picture on each face,** and `BoxFaces` a part of it on each: the six faces of a die from one picture.
- **`LoadOBJFS` keeps a model's `map_Kd` picture** and its coordinates. A picture is JPEG or PNG, and a bad one never fails the load.

## See-through faces

```go
	wall := square(4, 6, mesh.Material{Color: engine.RGB(200, 0, 0)})
	pane := square(1, 3, mesh.Material{Color: engine.RGB(0, 0, 200), Opacity: 0.5}) // half blue glass

	canvas := fb.Canvas()
	wall.DrawShadedPixels(canvas, cam, mesh.Ambient{Level: 1}) // solids first
	pane.DrawShadedPixels(canvas, cam, mesh.Ambient{Level: 1}) // then see-through meshes, far to near
```

- **`Material.Opacity`** is the part of a mesh's color that shows, from 0 to 1, mixed over what is drawn behind it.
- **The order is yours.** Draw the solids first, then the see-through meshes from far to near. glonk does not sort.
- **In cells the character behind stays,** and the colors are mixed.
- **An empty cell or pixel counts as black.** Fill the background first for another.
- **`Opacity` 0 means solid,** as does anything not between 0 and 1. There is one value for a mesh, and an MTL file's `d` is not read.

`examples/viewer` has a model with a color on each corner, and a die from one picture. `go run ./examples/viewer model.obj` shows a model of your own.

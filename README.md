<h1>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="brand/glonk-night.svg">
    <img src="brand/glonk-paper.svg" alt="glonk" width="336" height="80">
  </picture>
</h1>

Every game starts with a block.

A terminal rendering engine for Go. Draw 2D, 3D, or both in the same frame, using plain terminal cells.

![A walk among shaded blocks, past a tree and into a cube drawn as lines: half-block pixels in a terminal, with lines of text above and below.](site/media/cube3d.gif)

Website: [glonk.dev](https://glonk.dev)

> Early and pre-1.0: the API may change.

## What it does

- **2D:** cells with foreground and background colors (the 256-color palette, or exact RGB on terminals with true color) and text attributes (bold, dim, underline, reverse), lines, boxes, filled rectangles, text, and sprites (several cells, see-through, animated).
- **3D:** wireframe or solid meshes through a perspective camera, with near-plane clipping and an optional far plane. Solid meshes are shaded by any number of lights (directional, ambient, point, or your own) into an ASCII brightness ramp, face by face or, with normals, smoothly across each face. They hide each other by depth and skip faces turned away. A wireframe drawn with depth is hidden by nearer solids too.
- **Shapes and models:** a cube, sphere, cylinder, cone, plane and grid, or OBJ files from any `io.Reader` (a file, `//go:embed`, a download), centered and scaled and drawn wireframe or shaded, in the colors and texture pictures of their materials when loaded from an `fs.FS`, and with a color on each vertex.
- **Half blocks:** a pixel `Canvas` with two square-ish pixels per cell, drawn with `▀` and a background color, for twice the vertical detail and shading by color instead of by character. Lines, rectangles, circles, wireframes and pictures (JPEG or PNG) can be drawn into the pixels too. See [Pixel shapes and pictures](#pixel-shapes-and-pictures).
- **Mixed:** draw a 3D scene first, then a 2D HUD on top, into the same frame.
- **Widgets:** a label, a panel, a bar and a menu for a HUD or a pause screen, drawn in screen space. See [Widgets](#widgets).
- **Input:** held keys, one-shot presses (`Input.Started`, which a repeat does not set off again) and key repeats (`Input.Pressed`), including the arrows, Home, End, Page Up, Page Down, Insert, Delete and F1 to F12. `Input.KeyEvents` gives the frame's key events in order, each with its modifiers (Ctrl, Alt, Shift, Super) and whether it was a press, a repeat or a release, for shortcuts such as Ctrl+S; `Input.Text` is the characters typed, for a text field. A chord, a character key with Ctrl, Alt or Super held, is in those and not in `IsDown`, `Pressed` or `Started`, so Ctrl+W does not walk a player who walks on W. Terminals with the [kitty keyboard protocol](https://sw.kovidgoyal.net/kitty/keyboard-protocol/) (Ghostty, kitty, WezTerm, foot, iTerm2, Alacritty) report real key releases, so you can hold several keys at once. `Input.Paste` is what was pasted, as one string: a terminal with bracketed paste marks a paste, which glonk asks for, so a newline in it does not press Enter, an escape sequence in it is not read as keys, and a Ctrl-C in it does not end the run. With `Config.Mouse`, `Input.MousePos` is the cell the pointer is in, `MouseDown`, `MouseStarted` and `MouseReleased` are the left, middle and right buttons (a click shorter than a frame is not lost), and `Wheel` is how far the wheel turned in the frame.
- **Resizing:** the frame follows the terminal window, and a game with a `Resize(width, height int)` method is told its size.
- **Frame numbers:** a game with a `Stats(engine.Stats)` method is told after every frame how it went: frames a second, the time of `Update`, `Draw` and the write to the terminal, and the bytes sent. See [Frame numbers](#frame-numbers).
- **Testing:** `engine.Keyboard` presses keys for a game's `Update`, a `FrameBuffer` prints as text or draws as a picture, and `Config.Input` and `Config.Output` let a whole `Run` be driven without a terminal. See [Testing a game](#testing-a-game).

## Install

```
go get glonk.dev
```

Requires Go 1.26 or newer. glonk has no package at its root, so import the packages you use (`glonk.dev/engine`, and `glonk.dev/mesh` for 3D) and run `go mod tidy` once, which records the dependencies they need; without it `go run` stops with a missing `go.sum` entry.

### Moved to glonk.dev

Up to v0.1.0 glonk was imported as `github.com/kyrokohan/glonk`. From v0.2.0 it is `glonk.dev`; the code is still hosted on GitHub. To move a project that uses v0.1.0, run this in its module root (it needs `perl`, which macOS and most Linux systems have):

```
go mod edit -droprequire github.com/kyrokohan/glonk
grep -rl 'github.com/kyrokohan/glonk' --include='*.go' . | xargs perl -pi -e 's#github.com/kyrokohan/glonk#glonk.dev#g'
go get glonk.dev@latest
go mod tidy
```

## Run the examples

| | |
|---|---|
| [![Shaded blocks, a tree and a cube of lines on a dotted floor.](site/media/cube3d.gif)](site/media/cube3d.gif)<br>**cube3d**: walk around a floor of shaded cubes, a tree that is a picture, and a cube drawn as lines. | [![A small house with a red roof, turning.](site/media/viewer.gif)](site/media/viewer.gif)<br>**viewer**: look at the bundled OBJ models and shapes, solid or as lines, faceted or smooth. |
| [![A ship of text characters steered round a rock on a blue sea.](site/media/sprites.gif)](site/media/sprites.gif)<br>**sprites**: a ship you steer round a rock, a walking figure and a blinking ghost. | [![A small figure walking a level of walls, grass and water, collecting coins.](site/media/world.gif)](site/media/world.gif)<br>**world**: a level bigger than the screen, with walls, coins, a coin panel and a pause menu. |

The pictures are what the engine drew: each example plays a script of key presses with no terminal, and its frames are shown as a terminal shows them.

An example runs without a clone:

```
go run glonk.dev/examples/viewer@latest
```

and so do `cube3d`, `sprites`, `world`, `bounce2d` and `input`. In a clone:

```
make run                                      # bounce2d: a ball in a box that fills the window (Esc quits)
make run EXAMPLE=./examples/sprites          # sprites: a ship you steer around a rock, a walking figure and a blinking ghost on a colored floor
make run EXAMPLE=./examples/world            # world: a level bigger than the screen to walk around, with walls to slide along, coins to collect, a coin panel with a coin picture in it, and a pause menu (Esc)
make run EXAMPLE=./examples/cube3d            # cube3d: walk around a floor with six shaded cubes and a wireframe one
make run EXAMPLE=./examples/viewer            # viewer: look at the bundled OBJ models and shapes
make run EXAMPLE=./examples/viewer ARGS=my.obj  # viewer: look at your own model (its .mtl beside it gives it colors)
make run EXAMPLE=./examples/input             # input: see what your terminal sends, as glonk reads it: every key event with its modifiers, the text typed, what is pasted, and which keys are held
```

cube3d controls:

| Keys | Action |
|---|---|
| W / S | move forward / back |
| A / D | strafe left / right |
| Q / E, Left / Right | turn left / right |
| Up / Down | look up / down |
| M | switch between character shading and half-block pixels (the wireframe cube becomes pixel lines too) |
| B | in half-block mode, switch between exact and solid edges |
| Esc | quit |

viewer controls:

| Keys | Action |
|---|---|
| Arrows | orbit the model |
| + / - or W / S | zoom |
| R | spin on and off |
| N | next model (the bundled ones end with a sphere, a cylinder and a cone) |
| V | wireframe or solid |
| M, B | character shading or half-block pixels (with V, the wireframe is pixel lines); exact or solid pixel edges |
| D | draw both sides of every face (for models that look inside out) |
| C | change color (a model with colors of its own, like the bundled house and octahedron or one with a texture, goes from them through the six palette colors and back) |
| F | smooth or faceted shading for this model (a model without normals is smoothed from its faces) |
| L | light from the sun, or from a lamp held up and to the left of the camera |
| Esc | quit |

Letters work in either case. Ctrl-C quits any program built on glonk.

## Loading models

This sits inside your `Draw` method, with `fb`, `cam` and `lights` (a `[]mesh.Light`) set up as in the examples; `ExampleLoadOBJ` in `mesh/example_test.go` is a complete, runnable version.

```go
//go:embed ship.obj
var shipOBJ []byte

ship, err := mesh.LoadOBJ(bytes.NewReader(shipOBJ))
if err != nil {
	log.Fatal(err) // names the line that went wrong, e.g. "obj line 12: bad number"
}
ship = ship.Fit(2) // centered on the origin, longest side 2
ship.Transform.Position = geometry.Vector3{Z: 6}

ship.Material.Color = engine.Palette(208)
ship.DrawShaded(fb, cam, lights...) // or ship.Draw for a wireframe
```

To have the colors of a model's materials, load it with `LoadOBJFS(fsys, name, maxBytes)` from an `fs.FS` (`os.DirFS`, an `embed.FS`, a zip). It follows the OBJ's `mtllib` to the `.mtl` beside it and gives each face the color of its `usemtl` material in `Mesh.FaceColors`: the `Kd`, or the color of the material's `map_Kd` picture (a JPEG or PNG beside the OBJ) at the face's texture coordinates. A face with a color is drawn in it, still lit, and the others use `Material.Color`; the limit and errors are those of `LoadOBJLimit`. The picture itself comes with the mesh, in `Material.Texture` and `Mesh.FaceTexCoords`, and a face is drawn in its pixels (see "Textures" below).

```go
//go:embed ship.obj ship.mtl
var shipFiles embed.FS

ship, err := mesh.LoadOBJFS(shipFiles, "ship.obj", 8<<20)
```

A mesh is placed by its `Transform` (`Position`, `Yaw`, `Pitch`, `Roll`, `Scale`; angles in radians, and positive pitch tips the front up, as it does for the camera) and looks the way its `Material` says (`Color`, and a `Ramp` of brightness characters). `DrawShaded` takes any number of lights: `mesh.Ambient`, `mesh.Directional`, `mesh.Point` (a lamp that fades with distance), or a type of your own with a `Brightness` method.

`LoadOBJ` reads any `io.Reader`, so a file, an embedded asset or a download all work. For input you do not control, use `LoadOBJLimit(r, maxBytes)`: it fails with `mesh.ErrOBJTooLarge` instead of reading without end (a plain `io.LimitReader` would quietly cut the model short).

- **Understood:** vertices (`v`, with a color after them if the file has one: `v x y z r g b`), normals (`vn`), faces (`f`, with or without texture and normal indices, any number of corners, negative indices) and lines (`l`). Faces with more than three corners are split into triangles for shading, and the wireframe shows the original polygons.
- **Skipped:** groups and curves, and texture coordinates and materials too in `LoadOBJ` and `LoadOBJLimit`. Everything in the file becomes one mesh, in the color of its `Material` unless `LoadOBJFS` gave faces colors of their own. If the faces use normals the mesh is shaded smooth, with a vertex split into one per normal where the file lights it differently on different faces (the sides of a cube); if not, it is shaded flat.
- **Colors:** `Kd`, the diffuse color (three numbers from 0 to 1, or one for a gray), and `map_Kd`, the picture, are read; the rest of a material is skipped. Colors are used as RGB with no gamma conversion. Each `.mtl` is read up to 1 MiB (or `maxBytes`, if less), at most 8 per model, with 10,000 materials; a name must be a plain path inside the OBJ's folder (no `..`, no leading `/`, no backslash); `mtllib` must come before the `usemtl` that uses it. A `.mtl` that is missing, too large or broken never fails the load: the model comes out in its `Material` color. `os.DirFS` follows symbolic links in its folder, so give it a folder you trust.
- **Textures:** the picture is kept as `Material.Texture`, with the texture coordinates (`vt`) of each triangle's three corners in `Mesh.FaceTexCoords`, and drawing takes the color of every cell or pixel from the picture's nearest pixel at its place on the triangle, with the picture repeating beyond (1, 1). A large triangle shows the picture at its own size, not one flat color. `FaceColors` still holds the color of the picture at the middle of each triangle, for a program that drops the picture. A mesh has one picture, the first a triangle has coordinates in, and the triangles of another keep one color. A pixel with an alpha under 128 is a hole, as in a leaf or a fence: nothing is drawn there and what is behind shows. Where a corner has no coordinates, or the picture could not be had, the triangle keeps the material's `Kd`, if any; a material with both shows the picture. Only the plain `map_Kd name` form is read, and only JPEG and PNG, found by what is in the file; the picture sits beside the OBJ, under the same name rules as `mtllib`, and is read when the first face that uses it comes. A picture is refused by the size in its header, before it is decoded: 4096 by 4096 pixels (16,777,216) may be decoded for one model in all, 8 pictures are tried, and each file is read up to 64 MiB (or `maxBytes`, if less). A picture that is missing, broken, cut short, of another kind or too big never fails the load: its faces keep their `Kd` or the `Material` color. The `vt` lines and the `mtllib` must come before the faces that use them.
- **Orientation:** OBJ files are right-handed and look down -Z; glonk looks down +Z. `LoadOBJ` flips Z and reverses each face so a model keeps its outside and is not mirrored left to right. Its front faces a camera placed on the -Z side.
- **Inside out or invisible?** Some exporters wind faces the other way. Set `Mesh.DoubleSided = true` to draw both sides.
- **Untrusted files:** malformed input returns an error naming the line, never a panic, and the time taken grows in step with the input size. Memory use is roughly 25 times the input size, up to 35 for a file that gives most corners a normal of their own, so bound the input with `LoadOBJLimit` (or `LoadOBJFS`, which also bounds the `.mtl` files and pictures it reads, and while it loads holds a decoded picture, up to 128 MiB, and 16 bytes for each `vt` line, then keeps the picture it uses as a `Bitmap`, up to 64 MiB, and 48 bytes for each triangle) when you do not trust the source.

## Shapes and smooth shading

`mesh.Cube`, `Box` (any width, height and depth, with a picture on its faces), `Sphere`, `Cylinder`, `Cone`, `Plane` (a flat rectangle facing up) and `Grid` (lines for a floor) make meshes without a file. They stand on the y axis, centered on the origin, and are placed with `Transform` like any model.

```go
ball := mesh.Sphere(1.5, 24, 12) // radius, segments around, rings from pole to pole
ball.Transform.Position = geometry.Vector3{Z: 4}
lamp := mesh.Point{Position: geometry.Vector3{X: -2, Y: 2}, Range: 8}
ball.DrawShaded(fb, cam, mesh.Ambient{Level: 0.1}, lamp)
```

A mesh with no `Normals` is lit face by face, one brightness for each, so a round shape shows its facets. With a normal for each vertex (`Mesh.Normals`, in model space, any length) every cell is lit on its own, from the position and the normal blended across the face, so the shape looks round, and a lamp near a big flat face makes a pool of light on it.

- **Where normals come from:** `Box`, `Sphere`, `Cylinder`, `Cone` and `Plane`; a model file that has them; or `Mesh.SmoothNormals()`, which returns a copy with each vertex's normal averaged over the faces that share it, weighted by angle so that how a surface is cut into triangles does not change how it is lit.
- **Hard edges need their own vertices.** `SmoothNormals` averages across shared vertices only, so a cube made of 8 comes out looking like a ball. A cylinder's caps and a cone's base have vertices of their own for the same reason, and `LoadOBJ` splits a vertex that a file lights differently on different faces.
- **Back to facets:** draw a copy with `Normals` set to nil. A zero normal means none here: that corner takes the face's own. Normals turn with the `Transform`, whatever its scale.
- **Cost:** about 12 to 23% more than facets on a scene of spheres, and none for a mesh without normals. Drawing allocates nothing either way.
- **Shape limits:** segments go from 3 to 512 and rings from 2 to 512, so a careless argument cannot ask for gigabytes, and a size that is negative or not a finite number counts as 0. `Point` lights: `Range` 0 means no fading, and a negative `Range` gives no light.
- **Your own lights:** `Light.Brightness(point, normal)` is asked about every cell of a mesh with normals, with the place on the surface and the unit normal there, and once for each face of one without. `mesh.Brightness(lights, point, normal)` adds lights up the way the drawing does, so that a marker, a billboard or anything else you draw into the scene is lit as the meshes around it are, whatever the rule becomes.
- **Many lamps are cheap.** A `Point` with a `Range` is not asked about a mesh that is wholly beyond it, so a level can have a lamp in every room and each mesh asks the few that reach it (16 lamps along a corridor of floor chunks, each reaching a few of them: 40 microseconds a frame in cells and 71 in pixels, where it took 86 and 168). The picture is bit for bit the same. A light of your own opts in with a method `Reach() (center geometry.Vector3, radius float64)`, a ball outside which it gives nothing; promise only what it gives, since a light that gives more than it says loses light at the edge. It applies to a mesh with its bounds: the shapes and models the package makes have them, and a mesh built by hand calls `UpdateBounds`. A light without `Reach`, `Directional` and `Ambient` are always asked.

## Vertex colors

`Mesh.VertexColors` gives each vertex a color, and a face fades from one corner's color to the next.

```go
tri := mesh.Mesh{
	Vertices:     []geometry.Vector3{{X: -2, Y: -1}, {X: 2, Y: -1}, {Y: 2}},
	Faces:        [][3]int{{0, 1, 2}},
	VertexColors: []engine.Color{engine.RGB(255, 0, 0), engine.RGB(0, 255, 0), engine.RGB(0, 0, 255)},
	DoubleSided:  true,
}
tri.Transform.Position = geometry.Vector3{Z: 6}
tri.DrawShaded(fb, cam, mesh.Ambient{Level: 1}) // or DrawShadedPixels into a Canvas
```

- **A blend at every cell.** Each cell is drawn in the blend of its face's three corner colors, worked out like the blend of normals in smooth shading, with perspective, flat or smooth, in cells and in pixels. Brightness is still shown by the character or the shade. The blend is linear in the colors' red, green and blue, with no gamma, and a face cut by the near plane blends as the whole face would.
- **With face colors.** A face that has its own color (`FaceColors`, from a model's material) is tinted by the vertex colors: each channel is multiplied, so white leaves it as it is and gray darkens it. A face without one is drawn in the vertex colors as they are, and `Material.Color` is only what a corner without a color takes. A corner with the zero `Color` is "none here" and takes what the face would be without vertex colors, and a default color reads as black, as `Shade` reads it. A face none of whose corners has a color is drawn as before.
- **Limits.** Wireframes (`Draw`, `DrawDepth`) ignore vertex colors. A list that is not one color for each vertex is not used, and `Validate` says so. `Clone` copies it, and `Fit` and `SmoothNormals` keep it.
- **From OBJ files.** A vertex line of the form `v x y z r g b`, exactly six numbers, gives the vertex a color; `LoadOBJ`, `LoadOBJLimit` and `LoadOBJFS` all read it. The numbers go from 0 to 1 and are clamped, so a tool that writes 0 to 255 gives white. Any other count (a `w` after the coordinates is one), or numbers that are not understood, gives none and never fails the load. A model with colors only on some vertices has the zero `Color` on the rest, and a vertex split for its normals keeps its color. Files that carried colors which `LoadOBJ` used to ignore now load with them.
- **Cost.** A face with a colored corner asks for a color in every cell, which costs more than a flat color; a mesh without vertex colors draws the same cells and allocates nothing, but when vertex colors arrived in v0.4.0 the shaded benchmarks took 1 to 4% longer, which was put down to `Mesh` being 24 bytes larger and being copied (the wireframe benchmark, whose code did not change, moved 1 to 2% in the same runs). Since v0.11.0 a draw copies a `Mesh` once, not once for each helper it calls (#86). Reading a six-number vertex line takes about 77 ns longer.
- **See it:** the viewer's bundled octahedron has a color on each corner (`N` twice); `C` puts its colors aside for the palette and back.

## Textures

`Material.Texture` is a `*bitmap.Bitmap`, and `Mesh.FaceTexCoords` pins the three corners of each face to places in it, so that a wall of two triangles shows a picture of any size.

```go
wall := mesh.Mesh{
	Vertices: []geometry.Vector3{{X: -2, Y: -1}, {X: -2, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: -1}},
	Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
	FaceTexCoords: [][3]geometry.Vector2{
		{{X: 0, Y: 1}, {X: 0, Y: 0}, {X: 2, Y: 0}}, // the bottom left, top left, top right of the wall,
		{{X: 0, Y: 1}, {X: 2, Y: 0}, {X: 2, Y: 1}}, // and the picture twice across
	},
	Material: mesh.Material{Texture: brick}, // a *bitmap.Bitmap: bitmap.Decode, or bitmap.New and Set
}
wall.Transform.Position = geometry.Vector3{Z: 5}
wall.DrawShaded(fb, cam, lights...) // or DrawShadedPixels into a Canvas
```

- **Coordinates.** Corner `k` of face `i` is at `FaceTexCoords[i][k]`, one triple for each face, so a wall that shares its corners with the ones beside it needs no vertices of its own. (0, 0) is the top left of the picture and (1, 1) the bottom right, as `Bitmap.At` counts, and the picture repeats: 2 is the picture twice over. A zero triple means none, and that face is drawn as before.
- **A color for every cell.** Each cell or pixel takes the nearest pixel of the picture at its own place on the face, worked out with the perspective-correct weights that smooth shading uses, so the picture is seen at a slant and a face cut by the near or far plane keeps its coordinates. There is no blending between pixels. In cells a picture of 16 by 16 on a wall that fills the screen is 16 by 16 blocks of characters; with `DrawShadedPixels` the blocks are twice as fine from top to bottom.
- **Lit like a color.** The picture's color is the face's color, in place of `FaceColors` and `Material.Color`, and is lit and faded in fog as they are. A vertex color tints it, each channel multiplied, and a corner without one lets all of it through. A pixel that is see-through is a hole: nothing is drawn there, depth included, so what is behind shows through it, whichever was drawn first (a pixel of a `bitmap.New` picture that was never set, or with an alpha under 128 in one that was decoded). A face with a coordinate that is not finite draws nothing.
- **Limits.** One picture for a mesh; for two, make two meshes, or put both in one picture. Wireframes ignore it. A `Bitmap` that is a `Shaded` or `Tinted` view is drawn as it looks, as `Bitmap.Shown` gives its pixels: that is the color of the surface, which a vertex color then tints, light shades and fog fades, so a view tinted white for a flash is as dark as a white wall in a dark room. A list that is not one triple for each face, or a face with a coordinate that is not finite, is not used, and `Validate` says so. `Clone` copies the list and shares the picture, and `Fit` and `SmoothNormals` keep both.
- **Pictures on the shapes.** `Sphere`, `Cylinder`, `Cone` and `Plane` come with coordinates too, so setting `Material.Texture` is enough: a planet, a can, a roof, a floor. The round sides wrap the picture round as a map is wrapped round a globe, upright and not mirrored from outside, with its middle on the front (+z, as on a `Box`) and its edges meeting at the back, where they join with no seam to see. A pole, or a cone's tip, takes the middle of each slice, so the picture does not smear into a fan there. A cylinder's caps and a cone's base show no picture and are drawn in `Material.Color`. A `Plane` shows the picture once, as a box's top. To repeat it, scale the coordinates: `for i, t := range floor.FaceTexCoords { floor.FaceTexCoords[i] = [3]geometry.Vector2{t[0].Scale(10), t[1].Scale(10), t[2].Scale(10)} }` tiles a floor ten times each way.
- **A box with a picture.** `mesh.Box(w, h, d)` has the whole picture on each of its six faces, and `mesh.BoxFaces(w, h, d, mesh.BoxParts{Front: ..., Top: ...})` a part of it on each, as a `geometry.Rect` in fractions of the picture, (0, 0) its top left and (1, 1) its bottom right: an atlas of a die's six faces, the sides and the top of a block. A part left zero is the whole picture, one turned inside out is mirrored, and one past 1 repeats. Front faces +z, the way a model faces, so a camera looking down +z sees the Back. The sides are upright and not mirrored seen from outside, and the top and the bottom read as if unfolded from the front. Face i of Front, Back, Left, Right, Top, Bottom is `Vertices[4i:4i+4]` and `Faces[2i]` and `Faces[2i+1]`. `examples/viewer` has a die.
- **From OBJ files.** `LoadOBJFS` keeps a model's `map_Kd` picture and its `vt` coordinates this way, with `v` turned over because an OBJ counts it from the bottom. `FaceColors` still holds the one color for each triangle that it gave before.
- **Cost.** A face with a picture asks for a color in every cell, which costs more than one flat color and far less than the faces a picture used to be cut into: a wall of two triangles with a picture of 16 by 16 takes 7 microseconds in cells on 80 by 24 and 13 in pixels, where the same wall made of 512 faces of one color takes 60 and 62 (on 200 by 60: 41 and 76, against 92 and 102). A face with a picture goes through a fill that can leave a pixel alone, for holes, which costs 5 to 10% more than a color for each pixel. A mesh without a picture draws the same cells and allocates nothing, and costs what it did in v0.9.0 (v0.10.0 drew it 3 to 6% slower, which v0.11.0 took back; see the CHANGELOG and #86); `Mesh` is 40 bytes larger with the fields of this and of `Opacity`.
- **See it:** `go run ./examples/viewer model.obj` draws a model that has a `map_Kd` picture in its pixels; `C` puts the picture aside for the palette and back.

## See-through faces

A pixel of a picture that is see-through is a hole (see "Textures"), and `Material.Opacity` makes a whole mesh see-through: the part of its color that shows, from 0 to 1, mixed over what is already drawn.

```go
wall.DrawShaded(fb, cam, lights...) // solids first
pane := mesh.Mesh{
	Vertices: []geometry.Vector3{{X: -1, Y: -1}, {X: -1, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: -1}},
	Faces:    [][3]int{{0, 1, 2}, {0, 2, 3}},
	Material: mesh.Material{Color: engine.RGB(0, 100, 240), Opacity: 0.4}, // 40% blue glass
}
pane.Transform.Position = geometry.Vector3{Z: 3}
pane.DrawShaded(fb, cam, lights...) // then see-through meshes, from far to near
```

- **Mixed over what is there.** Each cell or pixel of the face is `engine.Mix(behind, color, Opacity)`, where the color is the face's own, picture or vertex colors included, lit and faded in fog first. In pixels the pixel is mixed. In cells the character and attributes of the cell behind stay, and its foreground and background are each mixed, so glass over a blank cell shows as a block of its color.
- **Over nothing.** An empty pixel or blank cell counts as black, as `Mix` and `Shade` read the default color, so a ghost shows against the background at its part of its color. Fill the canvas first for another background. (`Canvas.BlendRect` leaves an empty pixel empty, since a rectangle has no color of its own.)
- **The order is yours.** A see-through face is tested against depth, so a nearer solid hides it, but does not store it. Draw the solid meshes first, and then the see-through ones from far to near: a see-through mesh drawn before a solid that is behind it is covered by it, and two panes are mixed in the order drawn. glonk does not sort, between meshes or the faces of one, which blend in the order of `Faces`.
- **From behind.** As for a solid, a face that faces away is skipped unless `DoubleSided`, and then it is lit as the side the camera sees. A closed shape such as a glass ball, drawn on both sides, is mixed in face order, so list the faces of its far side first, or make two meshes.
- **Values.** `Opacity` 0, the default, means 1: the mesh is solid, as `Light.Intensity` 0 means 1. NaN and anything not between 0 and 1 is solid too. One value for a mesh; there is no opacity for each face or for a pixel of a picture, and an MTL file's `d` is not read.
- **Cost.** About what a solid costs, a little more for reading what is behind: a pane of two triangles over a wall takes 17 microseconds in cells on 80 by 24 and 26 in pixels, where the same pane solid takes 14 and 22. A fence of 16 by 16 pixels, half of them holes, takes 7 microseconds in cells on 80 by 24 as two triangles with a picture, against 34 as the 256 faces it had to be cut into, and 13 against 34 in pixels. On 200 by 60 it is 38 against 61 in cells, and 72 against 66 in pixels, where the picture's lookup in every pixel costs more than the cut-up wall's flat colors.

## Wireframes and depth

`Mesh.Draw` paints a mesh's edges over whatever is already on screen, so a wireframe shows through solids and has to be drawn first to stay behind them. `Mesh.DrawDepth` gives each cell of each edge a depth instead: a nearer surface hides it, it hides a farther one, and the order of drawing stops mattering.

```go
cube.DrawShaded(fb, cam, lights...)
ghost.DrawDepth(fb, cam, engine.Cell{Character: '#'}) // hidden where the cube is nearer
```

And as pixels, into the same canvas as the solids and before `Compose`:

```go
canvas := fb.Canvas()
cube.DrawShadedPixels(canvas, cam, lights...)
ghost.DrawDepthPixels(canvas, cam, engine.RGB(0, 220, 220)) // hidden to the pixel
canvas.Compose()
```

- **Half-block pixels count.** `Canvas.Compose` gives each cell it fills the depth of its nearer pixel, so anything drawn with a depth afterwards (`DrawDepth`, `FillTriangle`, `FrameBuffer.DrawLineDepth`, `SetDepth`) is hidden behind pixel solids that are nearer.
- **Drawing over a scene.** A tool in the player's hand, a model turning in a menu, arrows in a corner of an editor: things drawn after the scene that it must not hide. `Canvas.ClearDepth()` sets every depth back to far and keeps the pixels, so a mesh drawn next, with a camera of its own, lands on top of the scene while its own faces still hide each other. `FrameBuffer.ClearDepth()` does the same for the cells, and for the canvas too, as `Clear` empties both. The scene's pixels then compose with a far depth, so cells drawn with a depth after `Compose` land on top of them.

```go
world.DrawShadedPixels(canvas, cam, lights...)
canvas.ClearDepth()
tool.DrawShadedPixels(canvas, handCam, lights...) // over the world, however near it comes
canvas.Compose()
```

- **Wireframes can be pixels too.** `Mesh.DrawPixels` and `Mesh.DrawDepthPixels` are `Draw` and `DrawDepth` for a `Canvas`: the same edges, drawn as lines of pixels, twice as fine from top to bottom, and `DrawDepthPixels` is hidden by the solids drawn into the same canvas to the pixel. `Canvas.DrawLine` and `Canvas.DrawLineDepth` draw your own lines. A line is one pixel thick, so half a cell tall where it runs sideways; on a `Seamless` canvas (Apple's Terminal) it is a row of whole cells.
- **Your own drawing joins in.** `Camera.ProjectPixels(p, canvas)` says which pixel a world point is in, as fractional coordinates to floor with `math.Floor`, and how deep it is: its distance along `Camera.Look`, not the straight-line distance. Give both to `Canvas.SetDepth`, or the depth to `Canvas.DrawLineDepth`, and a marker, a billboard or a particle is hidden by the solids in front of it and hides those behind it, as the mesh's own pixels do.
- **And the way back.** `Camera.Ray(x, y, w, h)` and `Camera.RayPixels(x, y, canvas)` give the ray through a place on the screen, the reverse of `Project` and `ProjectPixels`: an origin, which is the camera's `Position`, and a direction. The direction is one deep along `Look`, not a unit vector, so the point `origin + t*direction` is exactly `t` deep, the depth `SetDepth` takes, and the ray through the middle of the screen is `Look` itself; call `Normalize` for a unit vector. Use it to find what is under the crosshair or the pointer, or to drop something where the player points. The ray through the middle of pixel `(i, j)` is `RayPixels(i+0.5, j+0.5, canvas)`, and a place off the screen has a ray too. To draw a whole canvas by rays, a sky by direction, a height map, a ray marcher, `Camera.RaysPixels(canvas)` and `Camera.Rays(w, h)` give every ray at once: the ray through the middle of pixel `(i, j)` goes from `Origin` along `First + i*Across + j*Down`, the direction `RayPixels` gives, to within rounding, and the steps keep every ray one deep. A canvas of 240 by 134 pixels takes 0.14 ms that way, and 2 ms with `RayPixels` for each pixel. `Camera.Up()` is the unit vector up, beside `Forward`, `Right` and `Look`.
- **Pointing the camera.** `cam.LookAt(target)` sets `Yaw` and `Pitch` so that the camera looks from its `Position` at a place, for a camera that follows a thing, watches a door or circles a model: call it each frame after moving the camera. A target straight above or below keeps the yaw, and one at the camera's own position changes nothing.
- **Ties keep what is there.** With `DrawDepth` a line that lies on a surface ties with it: a grid on a floor, a box round the thing that is selected, a mark on a wall, the edges of a mesh over its own faces. Where the line is a hair behind the surface it is lost, which across a floor seen at a low angle is half of it or all of it, and no lift is right at every distance: measured with a 1 by 1 outline on a floor at five distances from 1.5 to 12 away, only a lift of a tenth of its size kept all of it, and then it floats above the floor. Use `DrawOnSurface` for it.
- **Lines on a surface.** `Mesh.DrawOnSurface` and `DrawOnSurfacePixels`, and `FrameBuffer.DrawLineOnSurface` and `Canvas.DrawLineOnSurface` for lines of your own, draw the same lines as `DrawDepth`, after the surface, to lie on it. A cell of a line is drawn if it is no farther than what is stored there plus the slope of that surface across a cell, which is read from the depths stored around it, so it works on any surface that is drawn into the same `FrameBuffer` or `Canvas`: a floor under a grid, a wall under a mark, a mesh under its own edges. The outline in the measurement above is whole at every distance with no lift, and a nearer solid still hides it, as do the faces of a cube in front of its own far edges. A line that lies on the surface leaves its depth as it was, and one in front of it keeps its own, as `DrawDepth`'s does.

```go
floor.DrawShaded(fb, cam, lights...)
cube.DrawShaded(fb, cam, lights...)
grid.DrawOnSurface(fb, cam, engine.Cell{Character: '+'}) // on the floor, whole at any distance
cube.DrawOnSurface(fb, cam, engine.Cell{Character: '#'}) // its edges, on its own faces
```

- **What it does not do.** The most a line may be behind a surface is a quarter of the depth there, whatever the slope, so a line farther behind is hidden; a line a hair behind a surface that is seen edge-on shows, since no depth test can tell the two apart. The slope is read from what is drawn, so draw the surface first. Only a mesh's `Edges` are drawn, one pixel thick. It costs a little more than `DrawDepth` for the depths it reads: the 12 edges of a cube take 2.5 microseconds in cells on 80 by 24, against 2.0.
- **A far plane ends the world.** `Camera.Far` is the distance, along the view direction, beyond which nothing is drawn: a face or an edge that crosses it is cut where it crosses, as at the near plane, a mesh wholly beyond it is skipped without being looked at, and a point beyond it is not visible to `Project` and `ProjectPixels`. A point exactly `Far` away is drawn. It saves the time to draw the far end of a scene, and hides where a floor that runs on to the horizon ends. `Far` must be beyond `Near`; 0, the default, means no limit. A `Ray` is not limited by it.
- **Fog fades the far end.** `Camera.Fog = &mesh.Fog{Color: c, Start: 10, End: 40}` fades every shaded mesh toward `c` by how deep it is: nothing before `Start`, `c` itself from `End`, a straight line between. In cells the color fades and the character, which shows the brightness, stays; in pixels the shaded color fades. Give the camera a `Far` that `End` is at or before, and the world ends in the fog and not in a line. The distance is the depth along `Look`, the same number as `Far`, `SetDepth` and `ProjectPixels`, so `Fog.Apply(color, depth)` fades a marker or a billboard you draw yourself in step with the meshes. Wireframes are not fogged. It costs a color for every cell and pixel: a big floor takes about 46 percent longer in cells and 65 in pixels, a thousand cubes 6 and 15; a camera with no `Fog` draws as it did. `engine.Mix(a, b, t)` is the color arithmetic, public.
- **Off-screen meshes are skipped.** Drawing starts by asking whether any of a mesh can be on screen, by a sphere that holds its vertices, so a scene of many meshes costs little for those out of view. No picture changes. The shapes and models the package makes know their sphere; a mesh you build by hand with its vertices where they are in the world, as a level built in place is, should call `level.UpdateBounds()` once after setting `Vertices`, or its sphere is round the world's origin and holds everything. An 8,000-vertex mesh written in the world and behind the camera then costs about 2 microseconds, where it took 71 for a wireframe and 297 for a shaded one. A vertex changed in place needs `UpdateBounds` again, and `Validate` says so.

## Pixel shapes and pictures

The pixel `Canvas` draws rectangles and circles next to lines and triangles, blends a color over what is there, and the `bitmap` package draws a JPEG or PNG on it, or a picture made in code:

```go
canvas := fb.Canvas()
canvas.FillRect(2, 1, 20, 6, engine.RGB(40, 60, 220))
canvas.DrawRect(0, 0, canvas.Width(), canvas.Height(), engine.RGB(255, 255, 255)) // a border
canvas.FillCircle(40, 10, 8, engine.RGB(230, 210, 40))
canvas.DrawCircle(40, 10, 12, engine.RGB(230, 210, 40)) // a ring around it
canvas.FillDisc(ball.X, ball.Y, 2.5, engine.RGB(240, 240, 240)) // a ball between pixels, with a smooth edge
canvas.DrawText(2, 2, "SCORE 4210", engine.RGB(255, 255, 255)) // letters 5 by 7 pixels, in a font that comes with glonk
title := "GAME OVER"
canvas.DrawTextScaled((canvas.Width()-engine.PixelTextWidth(title, 3))/2, 20, title, engine.RGB(255, 80, 80), 3) // centered, three times the size
canvas.BlendRect(0, 0, canvas.Width(), canvas.Height(), engine.RGB(0, 0, 0), 0.5) // all of it half way to black

// once, at start-up: a //go:embed []byte, a file, a download
logo, err := bitmap.Decode(bytes.NewReader(logoPNG))
if err != nil {
	return err
}
logo.Draw(canvas, 2, 2)                // at its own size
logo.DrawScaled(canvas, 60, 2, 32, 16) // or stretched
canvas.Compose()

// over cells that are already in the frame, such as inside a filled panel: draw the pixels, then put that rectangle down
widget.Panel{Title: "coins", Fill: engine.Cell{Background: engine.Palette(236)}}.Draw(fb, 0, 0, 20, 5)
coin.Draw(canvas, 2, 2) // pixel (2, 2) is cell (2, 1): a cell is 1 pixel across and 2 down
canvas.ComposeRect(2, 2, coin.Width(), coin.Height())
```

- **Rectangles.** `FillRect(x, y, w, h, color)` fills the pixels from `(x, y)`, and `DrawRect` draws a border one pixel thick inside the same rectangle: one under 3 pixels wide or tall is all border. A width or height that is not positive draws nothing.
- **Circles.** `FillCircle(cx, cy, r, color)` is the pixels with (x-cx)² + (y-cy)² <= r², so it is `2r+1` across and round, since the pixels of a canvas are square. `DrawCircle` is its outline, a ring one pixel thick. A radius of 0 is one pixel and a negative one draws nothing. The rows come from integer arithmetic, so the same pixels are drawn on every platform.
- **Soft discs.** `FillDisc(cx, cy, r, color)` fills the disc of radius `r` around `(cx, cy)`, which can be fractions of a pixel, so a small round thing moves and grows smoothly instead of a pixel at a time. Pixel `(x, y)` is the square from `(x, y)` to `(x+1, y+1)`, as `Camera.ProjectPixels` places a point, so a projected point is a center as it is. A pixel the disc covers wholly is `color`, and one on its edge is mixed toward it by the part of 64 points in it that the disc covers. An empty pixel there has nothing to mix with, so it is `color` from half covered and stays empty below that: fill a background first for a smooth edge everywhere. `BlendDisc(cx, cy, r, color, t)` mixes `color` in by the part `t` as well, for a shadow or a glow, and leaves empty pixels empty, as `BlendRect` does. The same pixels are drawn on every platform. A disc costs more than `FillCircle`, about 1 microsecond at a radius of 2.5 and 60 at 50.5, and allocates nothing.
- **Text.** `Canvas.DrawText(x, y, text, color)` draws the printable ASCII characters, space to `~`, in a pixel font that comes with glonk: letters 5 pixels wide and 7 tall, 6 apart, so one blank column is left between them. `DrawTextScaled(x, y, text, color, scale)` makes every pixel of a letter a block of `scale` by `scale`, for 2 or 3 times the size, and `engine.PixelTextWidth(text, scale)` is how wide that is, from the left of the first letter to the right of the last with no gap after it, so a title centers; the height is always `7*scale`. Only the pixels of the letters are written, so what is behind them shows between the strokes (to have a background, `FillRect` one first, `PixelTextWidth` wide), and the zero `Color` erases them. A character with no letter, an accent, a Chinese character, an emoji, a tab, is drawn as `?`, as `FrameBuffer.DrawText` draws what it cannot show, a NUL as a space, and a mark with no width of its own is skipped; there is no wrapping. It takes about 100 nanoseconds a letter at the smallest size and 200 at 3 times, and allocates nothing. The font is data in the repo, drawn for glonk. `FrameBuffer.DrawText` still draws cells; use it where the text is part of the grid, and this where it is part of a picture.
- **Over cells.** The canvas is under every cell, so a filled `Panel` drawn after a picture covers it. `Canvas.ComposeRect(x, y, w, h)` puts one rectangle of the canvas down over the cells that are already in the frame, in pixels, the numbers the picture was drawn with: draw the panel, draw the picture (or the text, or a shape) on the canvas, then `ComposeRect`. Every cell the rectangle touches is put down, a cell being two pixels tall, so a picture on an odd pixel row works. A see-through pixel shows what the cell showed in that half: a half block keeps its other half, a space or a letter its background, and `Reverse` is read. **A cell that holds a letter loses it when a pixel is put down in it**, since a half block cannot hold one, while a cell whose two pixels are both empty is not touched and keeps its letter. On a `Seamless` canvas a cell with a pixel is one solid color, as for `Compose`, so what was under it is not kept. Where the canvas also holds a world that `Compose` has put down, `Clear` it first and draw the picture on the empty canvas. `Compose` is as it was and still puts the whole canvas down. Only the cells on the frame are visited, no position or size can overflow, and nothing is allocated. `examples/world` has a coin in its filled panel.
- **Blending.** `BlendRect(x, y, w, h, color, t)` mixes `color` over the pixels of a rectangle by the part `t` from 0 to 1: each pixel that is there becomes `engine.Mix(pixel, color, t)`. A dim behind a menu, a fade to black, a red flash and a see-through panel are one call, and the whole canvas is `BlendRect(0, 0, canvas.Width(), canvas.Height(), ...)`. A pixel that is empty stays empty, since the terminal's own background is not known and there is nothing to mix with, so fill first where the tint should show there. A palette pixel becomes the exact RGB color it stands for, depth is never read or written, and nothing allocates: a whole 200 by 120 canvas takes about 44 microseconds.
- **As lines are.** Shapes and pictures clip, replace the pixels under them whatever their depth (they never read or write it), and erase with the zero `Color`. Only the pixels on the canvas are visited, so a shape far off screen costs nothing and no number is too large. None allocates: a rectangle over a whole 200 by 120 canvas takes about 9 microseconds, a disc of radius 50 about 4, its ring about 1.
- **Pictures.** `bitmap.Decode` reads a JPEG or PNG, found by what it holds and not by its name, once, into the colors of the canvas. `Draw` and `DrawScaled` then copy pixels and allocate nothing: 64 by 64 pixels take about 4 microseconds, or about 29 stretched to 128 by 96. A PNG pixel with an alpha of 128 or more is drawn as an opaque color and a lighter one is left alone, so what is under it shows. There is no blending, since a canvas pixel has no alpha and the terminal's own background is not known. Black is a color and is drawn. `DrawScaled` takes the pixel of the picture under the middle of each pixel it draws, with no smoothing, so a picture at its own size comes out exactly.
- **Pictures made in code.** `bitmap.New(w, h)` makes a picture of see-through pixels, and `Set(x, y, color)` colors one: a generated texture, a sprite in other colors, a small map of a level. `bitmap.FromImage(img)` copies any `image.Image` with the alpha rule of `Decode`, so nothing has to be written out as a PNG and read back. `FromImage` has no limits, since its pixels are already in memory, but `New` gives an empty picture for a size of more than 4,194,304 pixels, as a size can come from a file.
- **Part of a picture.** `Sub(x, y, w, h)` is a view of one rectangle of a picture that shares its pixels, so a strip of animation frames is one picture drawn a frame at a time: `strip.Sub(2*16, 0, 16, 16).Draw(canvas, x, y)`. Every call works on a view as on a picture, `Set` on either changes the other, a rectangle that runs off the picture is cut to it, and nothing is copied, so make the views once, at start-up. `Mirrored()` is a view with its columns reversed, for a thing that faces the other way: `frame.Mirrored().Draw(canvas, x, y)`.
- **Darker, and in one color.** `Shaded(brightness)` is a view drawn darker, for a thing in a lit scene, and `Tinted(color, part)` one mixed toward a color: a part of 1 draws the shape all in the color, for the letters of a font made from one white picture or a hit that flashes white. They share the pixels, `Sub` and `Mirrored` keep them, setting again replaces rather than adds, and the shade comes first and the tint over it. `At` and `Set` keep the picture's own colors, `Shown(x, y)` is a pixel as the view draws it, and `Plain()` says whether a picture has no look, so a loop over many pixels can ask once and use the cheaper `At`. A mesh whose picture is a view draws it that way too, under its own light. Each pixel is a `Shade` and a `Mix`, so a 64 by 64 picture takes about 20 microseconds instead of 4.
- **Pictures in a 3D scene.** `Draw` and `DrawScaled` never read or write depth, which is right for a logo or a HUD. `DrawScaledDepth(canvas, x, y, w, h, depth)` is `DrawScaled` for a picture of a thing that stands in the scene, an enemy, a pickup or a tree: a nearer surface hides it, it hides a farther one, in whatever order they are drawn, and a tie keeps what is there. The depth is one number for the whole picture, the distance along the camera's view direction, and `Camera.ProjectPixels` gives it with the pixel to draw at: project the foot and the top of the thing, and stretch the picture between them. It costs about 4 to 5 percent more than `DrawScaled` and allocates nothing.
- **Files that cannot be trusted.** A small file can claim a picture of billions of pixels, so `Decode` reads the size from the header and refuses a picture over 32 MiB, or of more than 4,194,304 pixels (2,048 by 2,048), before decoding it. `DecodeLimit` takes limits of your own, and a source that never ends is read only up to the limit. The decoders are linked only into programs that import `bitmap`, which adds about 220 KB to a stripped program.
- **Not included.** Smooth scaling and anti-aliasing other than the edge of a disc, animated formats, ellipses, rounded rectangles, lines thicker than a pixel, and blending partial alpha.

## Sprites

A sprite is a picture at a position: one `Cell`, a `Frame` of several made from text art, or an `Animation` of frames. A `Layer` draws its sprites in the order they were added.

```go
ship := sprite.FromLines(engine.Cell{Foreground: engine.Palette(208)},
	` /\ `,
	`/__\`,
)
walk := sprite.NewAnimation(8, frame0, frame1, frame2, frame3) // 8 frames a second
walk.Loop = true

layer := sprite.NewLayer()
layer.Add(&sprite.Sprite{Position: geometry.Vector2{X: 2, Y: 1}, Frame: ship, Overlay: true})
layer.Add(&sprite.Sprite{Position: geometry.Vector2{X: 20, Y: 5}, Animation: walk, Overlay: true})

// in Update:
walk.Advance(dt)
// in Draw:
layer.Draw(fb)
```

- **A space is a hole.** `FromLines` puts the style's colors on every rune that is not a space; per-cell colors come from `NewFrame` and `Frame.Set`.
- **`Overlay` makes a sprite see-through.** Off, a sprite replaces the cells under it, blanks included. On, its blank cells show what is behind them, and a colored letter keeps the background of the floor it stands on. It is off by default, so every sprite written before it draws as it did. Do not overlay sprites on half-block pixels: only the background of those cells is kept.
- **Animations run in time.** `Advance(dt)` from `Update` plays them at the same speed at any frame rate. They stop on the last frame (`Done`) unless `Loop` is set.
- **The position is the top left of the frame**, and a frame that runs off an edge is cut off cell by cell.
- **Frames are shared by pointer.** Two sprites with one `Frame` see each other's `Set`; `Clone` makes a copy.

`examples/sprites` has a ship, a walking figure and a ghost, on a sea and a beach, and a rock the ship cannot sail through. Against a grid of tiles, `tilemap.Map.Move` does the pushing for you; see Tile maps and a camera.

## Collision

A sprite's box is where it is, as wide and tall as what it draws, and `geometry.Rect` says what two boxes share.

```go
wall := geometry.NewRect(30, 0, 2, 20)

// in Update, after moving the player:
if push := player.Bounds().PushOutOf(wall); push != (geometry.Vector2{}) {
	player.Position = player.Position.Add(push)
}
for enemy := range enemies.Hits(player.Bounds()) {
	hurt(enemy)
}
```

- **Boxes.** `Sprite.Bounds()` is a `Rect` from `Position`, as it is and not floored, by the size of the animation's current frame, else the frame, else 1 by 1. The whole frame counts, holes included, so a ghost with empty corners still touches at its corners. A sprite with nothing to draw, or at a NaN or infinite position, has an empty box and touches nothing.
- **Half-open.** A `Rect` covers `Min` up to but not including `Max`, like the cells a sprite covers: a sprite at X=2 that is 3 wide covers 2 up to 5, and one at X=5 does not touch it.
- **Asking.** `Rect.Overlaps`, `Rect.Intersect`, `Rect.Contains`, `Sprite.Overlaps` and `Layer.Hits(box)`, which lists the sprites whose boxes overlap one, in draw order and allocating nothing (1,000 sprites take about 4 microseconds). A sprite in the layer hits its own box, so skip it by comparing pointers; `Layer.All()` ranges over the whole layer. The layer must not change while either runs.
- **Pushing out.** `Rect.PushOutOf(wall)` is the smallest move, along one axis, that takes a box out of another: the axis on which they overlap least and the way that is shorter, so steering into a wall at an angle slides along it. It is the zero vector when they do not overlap.
- **Two habits.** A push leaves a sprite against the wall to within rounding, which can be a hair short of a whole cell, and drawing floors the position, so round it after pushing (to a millionth) or it can rest a cell short. And a push does not know where the sprite came from, so one that moves farther than its own size in a frame can come out the other side: cap the time of the step. `examples/sprites` does both.
- **Not included.** No pixel-perfect or swept collision, no collision layers: boxes only, and you keep a separate `Layer` or list for each kind of thing.

## Tile maps and a camera

A `tilemap.Map` is a level bigger than the screen: a grid of tiles, written as text art, drawn through a camera that is just a `Rect`, with walls that stop a moving box.

```go
level := tilemap.FromLines(map[rune]tilemap.Tile{
	'#': {Cell: engine.Cell{Character: '#'}, Solid: true},
	'.': {Cell: engine.Cell{Character: '.'}},
},
	"##########",
	"#........#",
	"##########",
)

// in Update, after reading the keys:
moved, hitX, hitY := level.Move(player.Bounds(), geometry.Vector2{X: dx, Y: dy})
player.Position = moved.Min

// the way round the walls to the player, for every enemy at once
route.To(level, player.Bounds().Center()) // when the player's tile changes
if next, ok := route.Next(enemy.Bounds().Center()); ok { /* steer toward next, then level.Move */ }

// can the enemy see the player, and what does a shot hit?
sees := level.Sees(enemy.Bounds().Center(), player.Bounds().Center())
hit, _ := level.Cast(muzzle, aim) // hit.X, hit.Y, hit.Point, hit.Distance

// the camera: the screen's rectangle, centered on the player, kept inside the level
screen := geometry.NewRect(0, 0, float64(fb.Width()), float64(fb.Height()))
view := screen.CenterOn(player.Bounds().Center()).Within(level.Bounds())

// in Draw:
level.Draw(fb, view)
sprites.DrawView(fb, view)
// a score or a hint is text for the screen, drawn after, with no view
```

- **A level as text art.** `FromLines(legend, lines...)` makes a map from a legend of what each rune is; a rune that is not in it, a space included, is the empty tile. `New(w, h, fill)` makes a map of one tile, and `Set` changes a cell. A `Tile` is an `engine.Cell`, whether it is `Solid`, and its `Kind`. A map holds up to 4,194,304 cells and 65,535 kinds of tile (two bytes a cell); a bigger one is empty.
- **A tile says what it is.** `FromLines` gives each tile of its legend the rune that made it as its `Kind`, so `level.At(x, y).Kind == 'X'` tells a crate from a wall that looks and acts the same, and a game need not keep the text of its level beside the map. A `Kind` the legend has is kept, and a rune the legend does not list is the empty tile, with no kind: list a marker such as `P` as `'P': {}` to find it. Tiles that differ only in kind are different tiles, so runes whose tiles look alike are no longer one tile, though a cell is still two bytes.
- **Holes and layers.** A tile whose cell is a hole (blank, no background, no attributes) is not drawn and shows what is under it, as a see-through sprite does, so one map can be laid over another. Any other tile replaces the cell under it.
- **The edge of the world is a wall.** `Solid` is true outside the map, and a map is empty and solid on a nil pointer.
- **The camera is a `Rect`.** `Rect.CenterOn(p)` centers it on a point and `Rect.Within(level)` keeps it inside, or centers it where the level is smaller than the screen. `Map.Draw(fb, view)` and `Layer.DrawView(fb, view)` use only its top left corner, floored in the world, so a sprite stands on its tile however the view moves and the picture moves a whole cell at a time. For a smooth follow, ease the view toward the player yourself with `Vector2.Lerp`.
- **Moving through the map.** `Map.Move(box, delta)` moves a box along X, stopping at the first solid tile, then along Y, so a box that goes into a wall at an angle slides along it. It looks at every tile on the way, so a long frame cannot carry the box through a thin wall, and it stops with the box's edge exactly on the tile's edge, so there is no rounding gap and the box rests against a wall the same at any frame rate: neither habit of the Collision section is needed. It reports whether it was stopped on each axis. Only the tiles a box moves into stop it, so one that starts inside a wall can move on, and one outside the map cannot move.
- **Line of sight and ray casts.** `Map.Sees(a, b)` says whether a straight line between two points crosses no solid tile, and `Map.Cast(from, direction)` shoots a ray and returns a `Hit`: the first solid tile it meets, where it enters it (`Point`, on the tile's edge) and how far (`Distance`, in cells). Both walk the grid one tile at a time, exactly, so no wall is thin enough to miss and the work is at most the size of the map. A point is in the tile it floors to, so a point at whole numbers is in its own tile, and a line through a corner is stopped only by the tile that holds the corner. Outside the map is solid, so a ray from inside always hits something. A 400 by 200 map crossed corner to corner takes about 1.4 microseconds and none of it allocates.
- **Walking to a target.** `Route.To(level, targets...)` searches the tiles that are not solid, breadth first, for how many steps each is from the nearest target, and `Route.Next(point)` gives the center of the neighbor tile to step to be one step nearer, to steer to and `Move`: an enemy that walks straight at the player is stopped by the first wall, and one that follows the route goes round it. `Route.Steps(point)` is the distance. One `Route` serves every mover that wants the same target, so search again when the target changes tile, not once a mover. A step goes to any of 8 neighbors, a diagonal one only if both tiles beside it are open, so that a mover does not cut through the corner of a wall. A route is a snapshot of the map, reuses its memory, and searching a 400 by 200 map takes about 1.6 to 2.8 milliseconds with none allocated.
- **Cost.** Drawing 200 by 60 cells takes about 20 microseconds and a move about 24 nanoseconds, neither allocating. `Layer.Draw` is as fast as before: `DrawView` has a loop of its own.
- **Not included.** Tiles of other sizes than one cell (write a tile twice across for a squarer look), tile animation (`Set` the cell in your own `Update`), loading maps from files, parallax and camera shake.

`examples/world` is a level a screen and a half either way: walk it, slide along the walls, and collect the coins.

## Widgets

The `widget` package draws the pieces of a HUD and a menu in screen space: a `Label`, a `Panel`, a `Bar` and a `Menu`. Each is a value with exported fields and a `Draw` method that takes a position in cells, like `DrawBox`.

```go
// in Draw, after the world:
widget.Panel{Title: "Status"}.Draw(fb, 0, 0, 20, 4)
widget.Label{Text: "coins 3/12"}.Draw(fb, 2, 1, 16)
widget.Bar{Value: float64(coins) / float64(total)}.Draw(fb, 2, 2, 16)

// a menu keeps its selection, so keep it in the game:
menu := widget.Menu{Items: []string{"Resume", "Quit"}}

// in Update:
if choice, chosen := menu.Update(in); chosen && choice == 1 {
	return engine.ErrQuit
}

// in Draw:
menu.Draw(fb, 2, 5, 16, 2)
```

- **Style.** A style is an `engine.Cell`: its colors and attributes are used and its `Character` is not. A `Bar` is made of the cells `Full` and `Empty`, which are `█` and `░` by default.
- **Text.** Labels, titles and menu items are counted in cells as `DrawText` counts them (`engine.TextWidth` returns that number), cut off at their width, and aligned `Left`, `Center` or `Right` for a `Label`.
- **A panel's fill.** A `Fill` that is blank, with no background and no attributes, as the zero value is, is a hole: the panel leaves its inside alone and shows what is under it, as tiles and sprites do. A title is shown from 7 cells wide.
- **Panels that share an edge join it.** A border drawn over a light box line that is already there, another panel's or a `DrawBox`'s, meets it as `├ ┤ ┬ ┴ ┼`, so a list of rows, a table or a view split in two is panels drawn edge on edge. `fb.SetJoined(x, y, cell)` draws one cell that way, for lines of your own. Double, heavy and rounded lines are drawn over, not joined.
- **The menu.** Up and Down move the selection and wrap around, Enter chooses, and `Update` returns the index of the item. Escape and every other key are left to the game, which can use Escape to open and close the menu. The selected row is drawn reversed, and a list longer than its rows scrolls to keep the selected item in the middle row. Test it with `engine.Keyboard`.
- **Always safe.** What runs off the frame is cut off, a size that is not positive draws nothing, and numbers as large as an `int` cannot wrap around onto the frame. Nothing allocates.
- **Not included.** Mouse input, text entry, layouts, and focus or nesting: a widget is drawn where you say.

`examples/world` has a coin panel with a coin picture, a count and a bar, and a pause menu.

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"os"

	"glonk.dev/engine"
)

type game struct{ x float64 }

func (g *game) Update(dt float64, in *engine.Input) error {
	if in.Pressed(engine.KeyEscape) {
		return engine.ErrQuit
	}
	g.x += 20 * dt // cells per second
	return nil
}

func (g *game) Draw(fb *engine.FrameBuffer) {
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{}) // the default color
	fb.Set(int(g.x)%fb.Width(), fb.Height()/2, engine.Cell{Character: '@', Foreground: engine.Palette(208)})
}

func main() {
	// Width and Height of 0 fill the terminal and follow resizes.
	if err := engine.Run(context.Background(), &game{}, engine.Config{FPS: 30}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

`Run` owns the terminal. Each frame it calls `Update` (move things, read input) and then `Draw` (write cells into the `FrameBuffer`), then prints the frame. The terminal is restored when `Run` returns, and Ctrl-C, `kill` and a closing window (SIGINT, SIGTERM, SIGHUP) end it cleanly (`Run` returns `nil`), so the game needs no signal handler. When your `context` ends it, `Run` returns the context's error. The full API is in each package's documentation, with runnable examples: `go doc glonk.dev/engine`, or the package pages on pkg.go.dev.

## Frame numbers

A game that has a `Stats(engine.Stats)` method is called after every frame, once it has been written, with how that frame went:

```go
func (g *game) Stats(s engine.Stats) { g.stats = s }

// in Draw:
fb.DrawText(1, 1, fmt.Sprintf("%.0f fps  %d bytes", g.stats.FPS, g.stats.Bytes), engine.Color{})
```

`Stats` has `FPS` (frames a second, averaged over about the last ten), `Update`, `Draw` and `Write` (the time of each part of the frame; `Write` is working out which cells changed and writing them, which is where a slow terminal shows) and `Bytes` (what was sent for the frame, 0 when nothing on the screen changed). The bytes decide whether a program works over ssh: a moving 3D scene in half-block pixels is about 7 KB a frame, and a frame where every pixel changes is 66 KB in 256 colors and 110 KB in true color for 100 by 30 cells, so a program that can see the number can draw less on a slow link. A game without the method is not timed, so asking is the only way to pay for it. A headless `Run` gives the same numbers, with every frame sent whole, so a test can check the bytes of a frame; see `ExampleStatsReceiver`. `cube3d` and the viewer show them.

## Testing a game

`Update` and `Draw` are plain methods, so a game is tested by calling them. `engine.Keyboard` stands in for the terminal's keyboard, and a `FrameBuffer` prints as text, one line per row:

```go
func TestEscapeQuits(t *testing.T) {
	kb := engine.NewKeyboard()
	kb.Press(engine.KeyEscape)

	err := (&game{}).Update(1.0/30, kb.Input())

	if !errors.Is(err, engine.ErrQuit) {
		t.Errorf("wanted ErrQuit, got %v", err)
	}
}

func TestTheDotMovesAcrossTheScreen(t *testing.T) {
	g := &game{}
	g.Update(0.25, engine.NewKeyboard().Input()) // a quarter of a second at 20 cells a second

	fb, _ := engine.NewFrameBuffer(12, 3)
	g.Draw(fb)

	want := "┌──────────┐\n" +
		"│    @     │\n" +
		"└──────────┘"
	if got := fb.String(); got != want {
		t.Errorf("wanted\n%s\ngot\n%s", want, got)
	}
}
```

A `Keyboard` from `NewKeyboard` acts like a terminal that reports key releases: a key is down from `Press` until `Release`, `Input.Pressed` and `Input.Started` are true for one frame, and `EndFrame` ends that frame the way `Run` does. A `Press` of a key that is already down is a key repeat: `Pressed` is true again, `Started` is not, so a test can hold a key. `PressWith(mods, keys...)` presses a chord such as Ctrl+S, and `Type(text)` types characters, for a game that reads `KeyEvents` and `Text`, and `Paste(text)` pastes. Tests are exact and need no clock. `NewLegacyKeyboard` acts like a terminal that reports none, for a game that asks `Input.Exact` and changes what it does: a key stays held for about 150 ms after its last `Press`, `Release` does nothing, and `Advance` moves the clock so that a test can let a key go. `String` leaves off the trailing spaces of each line and shows characters only, not colors.

To test a whole run (start-up, key bytes in, frames out), give `Config.Input` and `Config.Output` a reader and a writer that are not a terminal: `Run` then runs headless at the `Width` and `Height` you set. See `ExampleRun_headless`.

A frame can also be a picture, which `String()` cannot show: colors, and the two pixels of every half-block cell. `fb.Image` returns an `*image.RGBA` that is `Width*Scale` by `2*Height*Scale` pixels, so a frame drawn with a `Canvas` comes out pixel for pixel at `Scale: 1`. A test can write it for a person to look at when it is asked to, and compare it with a picture it stored earlier:

```go
func TestSceneLooksRight(t *testing.T) {
	g := newScene()
	fb, _ := engine.NewFrameBuffer(40, 12)
	g.Draw(fb)
	img := fb.Image(engine.ImageOptions{Scale: 4})

	if path := os.Getenv("GLONK_SCREENSHOT"); path != "" { // GLONK_SCREENSHOT=/tmp/scene.png go test
		f, _ := os.Create(path)
		defer f.Close()
		png.Encode(f, img)
	}
	// Compare with a stored picture: decode it with png.Decode and check each pixel of img.
}
```

What it draws: a half-block cell as its two pixels, a space as its background and `█` as its foreground. Any other character is a mark, a rectangle in its foreground color over the middle of the cell, which shows where text is and in what color but not what it says (`String()` says that). With `ImageOptions{Scale: 6, Text: true}` or a larger scale, a printable ASCII character is drawn as a letter of the font of `Canvas.DrawText` instead, `Scale/6` times its 5 by 7 pixels, in the middle of the cell, so a screenshot can be read; a wide or control character is a `?`, and box lines and other characters stay marks. `Reverse` swaps a cell's colors; Bold, Dim and Underline are not drawn. The default colors of a terminal are its own, so a cell that leaves them out is light gray on black unless `ImageOptions.Foreground` and `Background` say otherwise, and `Colors: engine.Colors256` shows RGB as the nearest of the 256 palette colors, as a terminal without true color does. A picture of more than 4,194,304 pixels is not made. Pixels set on a `Canvas` are in the picture once `Compose` has put them in the cells. `ExampleFrameBuffer_Image` is a runnable version.

## Packages

| Package | Purpose |
|---|---|
| `engine` | The loop, `FrameBuffer`, pixel `Canvas` (lines, rectangles, circles, triangles), drawing helpers, keyboard `Input` |
| `geometry` | 2D and 3D vector math, and `Rect` for boxes, collision and cameras |
| `mesh` | 3D meshes (wireframe and shaded), shapes, OBJ loading, lights, and the perspective `Camera` |
| `sprite` | Sprites (a `Cell`, a text-art `Frame` or an `Animation` at a position, optionally see-through) drawn in layers, through a view, with bounds and collision queries |
| `tilemap` | Tile maps written as text art, drawn through a camera, with solid tiles that stop a moving box |
| `widget` | A label, a panel, a bar and a menu for a HUD or a pause screen, drawn in screen space |
| `bitmap` | JPEG and PNG pictures, or ones made in code, drawn on the pixel `Canvas` at their own size or stretched, with limits for files that cannot be trusted |

## Compatibility and limits

Built to work in most terminals. The basics need only the alternate screen, cursor movement, and 256-color escape codes (24-bit colors are used on top of that where the terminal says it has them); line wrapping is turned off while a program runs, and the kitty keyboard protocol and focus reporting are used where the terminal has them. All of it is undone when `Run` returns. It is developed on macOS and tested mainly in Ghostty.

- **Without key releases, held keys are a timing guess.** Most terminals report presses and repeats but never releases, so a key counts as held until about 150 ms pass without a repeat, with a short pause when you first hold one. `Input.Started` has the same limit: it is true once when a key goes down and not for the repeats, but the OS waits longer than 150 ms before it repeats, so a key held past that delay starts twice. glonk detects the kitty keyboard protocol at startup and uses it where it exists; everywhere else this is what you get.
- **Without key releases, only one key stays held at a time.** The OS repeats just the most recently pressed key, so holding W and then pressing E stops W and you cannot walk and turn together (Terminal.app, for one). Set `Config.LegacyKeys` to force this behavior if a terminal misbehaves with the protocol. `Input.Exact` says which of the two a run has, so a game can warn the player or change to controls that work one key at a time; it is true in a headless `Run` unless `LegacyKeys` is set.
- **Escape waits 50 ms, and without the kitty protocol Alt and Ctrl are guesses.** An Escape press and the start of an arrow key are the same byte, so the Escape key counts only after 50 ms with nothing following it. Alt+key arrives as Escape plus the key, and glonk reads it as Alt+key in `Input.KeyEvents` (so an Escape pressed and then a key within one read is read as Alt+key too). Ctrl+letter arrives as a control byte, which is why Ctrl+I is Tab, Ctrl+M is Enter and Ctrl+H is Backspace. Shift is the case of a letter there, not a modifier of it; the arrows, Home, End, Page Up and Down, Insert, Delete and F1 to F12 carry their modifiers in the sequence they send. Only the kitty protocol says every modifier exactly. `Input.Text` is the characters as they arrive, or, on a terminal with the protocol but not its text flag, the keys with Shift applied to ASCII letters only. Hyper and Meta chords are not read.
- **A paste is one string only where the terminal has bracketed paste.** glonk asks for it as it takes the terminal over (and turns it off when it gives it back, and across Ctrl-Z) and reads the markers whether it asked or not, but a terminal that does not know the mode sends a paste as the key presses it is, with a newline as Enter, and `Input.Paste` is empty. A paste is cut at 1 MiB, with what is beyond it dropped; its text is valid UTF-8 with U+FFFD for what is not, and its line breaks are newlines. A paste that stalls for half a second before its end is delivered as it stands, so that a broken one cannot swallow every key after it.
- **The mouse is opt-in, and needs the terminal's decimal mouse mode.** Set `Config.Mouse` and glonk asks for every movement, the buttons and the wheel (modes 1003 and 1006, with focus reporting) as it takes the terminal over, and turns them off when it gives it back and across Ctrl-Z. While it is on, the terminal no longer selects text with the mouse for you (Shift with the mouse does, in most terminals), which is why a program that does not read the mouse is left as it was. The cell is counted from 0 at the top left of the terminal, so a canvas pixel at the top of it is `(x, 2*y)`, and it can be outside a frame smaller than the terminal. A terminal sends nothing for a pointer outside its window, so the pointer stays where it was last reported, and a button held when the window loses focus is let go then, not when you let go of it. Only the left, middle and right buttons and the wheel are reported, with no modifiers (Shift and a drag is often the terminal's own), and no pixel position. A terminal with the mouse but not mode 1006 sends an older form, which is dropped, so `MousePos` says `ok` is false there. Terminal.app and Ghostty were tried on macOS and both send the decimal form; `examples/input` shows what yours does.
- **Ctrl-Z suspends, and some endings cannot be cleaned up.** Ctrl-Z restores the terminal, stops the program, and takes the terminal over again on `fg`; Windows has no suspend. `os.Exit`, `log.Fatal`, a panic in a goroutine you started, `kill -9` and `kill -QUIT` skip the cleanup and leave the terminal raw; type `reset` to repair it. After the first quit signal a second one kills a game that is stuck, unless your program catches those signals itself.
- **Bad meshes and cameras draw nothing instead of failing.** Edges and faces that name a missing vertex are skipped, and normals that are not one per vertex or not finite numbers are not used (the mesh is shaded flat); `Mesh.Validate` finds both, a `Camera` field left at 0 uses a default (a 90 degree FOV, a Near of 0.1, a `CellAspect` of 2, no `Far`), a nonsense FOV, Near, Far or `CellAspect` sees nothing, a mesh whose `Transform` has a number that is not finite or a negative `Scale` is not drawn, and sprites at NaN or infinite positions are not drawn.
- **One character is one column.** A cell holds a single character one column wide. Wide characters (CJK, most emoji) are drawn as `?`, combining marks and joiners are skipped by `DrawText`, and control characters, escape sequences in text included, are drawn as `?` too, so nothing a file name or a game contains can send commands to the terminal. A NUL character is shown as a space, so `Cell{Background: engine.Palette(4)}` is a colored blank. Wide characters are not supported.
- **True color is used where the terminal says it has it.** An `RGB` color is sent exactly when `COLORTERM` is `truecolor` or `24bit` (terminals that can show it set that themselves) and as the closest of palette colors 16 to 255 otherwise; `Attrs` adds bold, dim, underline and reverse. Inside tmux or over ssh `COLORTERM` often does not arrive: export `COLORTERM=truecolor`, or set `Config.Colors` to `engine.ColorsTrue`, if the terminal can show it (and to `engine.Colors256` to turn it off). A terminal that cannot but is told it can shows wrong colors. A `Palette` color is always sent as a palette color, so colors 0 to 15 follow the user's theme. `Shade` gives exact RGB, so shades are finer on a true color terminal. Apple's Terminal does not set `COLORTERM` and keeps the 256-color look.
- **Cells are taken to be twice as tall as wide.** That is what makes a cube look like a cube in most fonts; set `Camera.CellAspect` for a font that differs.
- **Box drawing and half blocks need a Unicode font.** `DrawBox` uses `┌─┐│└┘` and the pixel canvas uses `▀▄`; nearly every monospace font has them. Character shading (`DrawShaded`) needs only ASCII and stays the default in the examples.
- **Apple's Terminal draws a faint line along half-block edges.** Its block characters fall a little short of the cell. glonk detects it (`TERM_PROGRAM=Apple_Terminal`) and `Canvas.Compose` then draws solid cells instead of half blocks, so there is no line but silhouettes are a pixel blockier and pixel lines are rows of whole cells. Set `Canvas.Seamless` yourself to override, for instance inside tmux, which hides that variable.
- **A fixed-size frame larger than the terminal is cut off.** Only the top-left part that fits is shown, and the screen is cleared when the window changes size. Leave `Width` and `Height` at 0 to fill the terminal.
- **A terminal that reports no size is taken as 80 by 24.** A pseudo-terminal that nobody gave a size reports 0 by 0, as under `script` started from a CI step or in some container consoles. `Run` then uses 80 columns or 24 rows for each axis you left at 0, until the terminal reports a size, and follows it from then on. Set `Width` and `Height` to choose a size yourself.
- **`FPS` runs from 1 to 1000;** `Run` returns an error outside that range. A key pressed during start-up, while glonk asks the terminal about the keyboard protocol, arrives as a tap rather than a hold.
- **Without a terminal there is no terminal handling.** A headless `Run` (see `Config.Output`) treats a key as held from its press to its release, as the kitty keyboard protocol reports them, unless `LegacyKeys` is set; a plain letter with no release then stays held. Ctrl-Z does nothing and quit signals are left to your program. Reading from an input that never ends a read (a pipe nobody closes) leaves one goroutine waiting after `Run` returns.
- **Only changes are sent, so do not print to the terminal yourself.** Each frame sends just the cells that differ from the last, as one synchronized update (terminals that do not know that mode ignore it). Text that something else writes while `Run` is running stays on screen until those cells change, the window is resized, or the window comes back to the front; log to a file instead.
- **Resizing can flicker for one frame.**
- **A build for the browser runs headless only.** glonk compiles for `GOOS=js GOARCH=wasm`, and CI checks that it still does. A browser has no terminal, so there `Run` needs `Config.Input`, `Output`, `Width` and `Height`, and the page has to pass the keys in and draw the frames it writes.
- **Windows is untested.**

## Planned

What is planned is in [GitHub issues](https://github.com/kyrokohan/glonk/issues), grouped by [milestone](https://github.com/kyrokohan/glonk/milestones) for the release it is aimed at. Ideas, not promises.

## Development

```
make check    # formatting, vet, and tests with the race detector
make fuzz     # fuzz the parsers, rasterizers and clipping; FUZZTIME=1m for longer
```

The race detector needs cgo. Where there is no C compiler, run `make test RACE=`.

## Contributing

Issues and pull requests are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md), and everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md). To report a security problem privately, see [SECURITY.md](SECURITY.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md). If you built something with glonk, it can be listed on [glonk.dev/showcase](https://glonk.dev/showcase/).

## License

MIT. See [LICENSE](LICENSE).

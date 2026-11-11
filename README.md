# glonk

A terminal rendering engine for Go. Draw 2D, 3D, or both in the same frame, using plain terminal cells.

Website: [glonk.dev](https://glonk.dev)

> Early and pre-1.0: the API may change.

## What it does

- **2D:** cells with foreground and background colors (the 256-color palette, or exact RGB on terminals with true color) and text attributes (bold, dim, underline, reverse), lines, boxes, text, and sprites (several cells, see-through, animated).
- **3D:** wireframe or solid meshes through a perspective camera, with near-plane clipping. Solid meshes are shaded by any number of lights (directional, ambient, point, or your own) into an ASCII brightness ramp, face by face or, with normals, smoothly across each face. They hide each other by depth and skip faces turned away. A wireframe drawn with depth is hidden by nearer solids too.
- **Shapes and models:** a cube, sphere, cylinder, cone, plane and grid, or OBJ files from any `io.Reader` (a file, `//go:embed`, a download), centered and scaled and drawn wireframe or shaded, in the colors and texture pictures of their materials when loaded from an `fs.FS`, and with a color on each vertex.
- **Half blocks:** a pixel `Canvas` with two square-ish pixels per cell, drawn with `▀` and a background color, for twice the vertical detail and shading by color instead of by character. Lines and wireframes can be drawn into the pixels too.
- **Mixed:** draw a 3D scene first, then a 2D HUD on top, into the same frame.
- **Input:** held keys and one-shot presses, including arrows. Terminals with the [kitty keyboard protocol](https://sw.kovidgoyal.net/kitty/keyboard-protocol/) (Ghostty, kitty, WezTerm, foot, iTerm2, Alacritty) report real key releases, so you can hold several keys at once.
- **Resizing:** the frame follows the terminal window, and a game with a `Resize(width, height int)` method is told its size.
- **Testing:** `engine.Keyboard` presses keys for a game's `Update`, a `FrameBuffer` prints as text, and `Config.Input` and `Config.Output` let a whole `Run` be driven without a terminal. See [Testing a game](#testing-a-game).

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

```
make run                                      # bounce2d: a ball in a box that fills the window (Esc quits)
make run EXAMPLE=./examples/sprites          # sprites: a ship you steer around a rock, a walking figure and a blinking ghost on a colored floor
make run EXAMPLE=./examples/world            # world: a level bigger than the screen to walk around, with walls to slide along and coins to collect
make run EXAMPLE=./examples/cube3d            # cube3d: walk around a floor with six shaded cubes and a wireframe one
make run EXAMPLE=./examples/viewer            # viewer: look at the bundled OBJ models and shapes
make run EXAMPLE=./examples/viewer ARGS=my.obj  # viewer: look at your own model (its .mtl beside it gives it colors)
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

To have the colors of a model's materials, load it with `LoadOBJFS(fsys, name, maxBytes)` from an `fs.FS` (`os.DirFS`, an `embed.FS`, a zip). It follows the OBJ's `mtllib` to the `.mtl` beside it and gives each face the color of its `usemtl` material in `Mesh.FaceColors`: the `Kd`, or the color of the material's `map_Kd` picture (a JPEG or PNG beside the OBJ) at the face's texture coordinates. A face with a color is drawn in it, still lit, and the others use `Material.Color`; the limit and errors are those of `LoadOBJLimit`.

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
- **Textures:** a triangle takes the color of the picture's nearest pixel at the middle of its three corners' texture coordinates (`vt`), with (0, 0) at the bottom left, the picture repeating beyond (1, 1) and its own transparency ignored. That is one color for a triangle, so a large triangle is flat however fine the picture; models with small triangles, like scans, look textured. Where a corner has no coordinates, or the picture could not be had, the triangle keeps the material's `Kd`, if any; a material with both shows the picture. Only the plain `map_Kd name` form is read, and only JPEG and PNG, found by what is in the file; the picture sits beside the OBJ, under the same name rules as `mtllib`, and is read when the first face that uses it comes. A picture is refused by the size in its header, before it is decoded: 4096 by 4096 pixels (16,777,216) may be decoded for one model in all, 8 pictures are tried, and each file is read up to 64 MiB (or `maxBytes`, if less). A picture that is missing, broken, cut short, of another kind or too big never fails the load: its faces keep their `Kd` or the `Material` color. The `vt` lines and the `mtllib` must come before the faces that use them.
- **Orientation:** OBJ files are right-handed and look down -Z; glonk looks down +Z. `LoadOBJ` flips Z and reverses each face so a model keeps its outside and is not mirrored left to right. Its front faces a camera placed on the -Z side.
- **Inside out or invisible?** Some exporters wind faces the other way. Set `Mesh.DoubleSided = true` to draw both sides.
- **Untrusted files:** malformed input returns an error naming the line, never a panic, and the time taken grows in step with the input size. Memory use is roughly 25 times the input size, up to 35 for a file that gives most corners a normal of their own, so bound the input with `LoadOBJLimit` (or `LoadOBJFS`, which also bounds the `.mtl` files and pictures it reads, and while it loads holds a decoded picture, up to 128 MiB, and 16 bytes for each `vt` line) when you do not trust the source.

## Shapes and smooth shading

`mesh.Cube`, `Sphere`, `Cylinder`, `Cone`, `Plane` (a flat rectangle facing up) and `Grid` (lines for a floor) make meshes without a file. They stand on the y axis, centered on the origin, and are placed with `Transform` like any model.

```go
ball := mesh.Sphere(1.5, 24, 12) // radius, segments around, rings from pole to pole
ball.Transform.Position = geometry.Vector3{Z: 4}
lamp := mesh.Point{Position: geometry.Vector3{X: -2, Y: 2}, Range: 8}
ball.DrawShaded(fb, cam, mesh.Ambient{Level: 0.1}, lamp)
```

A mesh with no `Normals` is lit face by face, one brightness for each, so a round shape shows its facets. With a normal for each vertex (`Mesh.Normals`, in model space, any length) every cell is lit on its own, from the position and the normal blended across the face, so the shape looks round, and a lamp near a big flat face makes a pool of light on it.

- **Where normals come from:** `Sphere`, `Cylinder`, `Cone` and `Plane`; a model file that has them; or `Mesh.SmoothNormals()`, which returns a copy with each vertex's normal averaged over the faces that share it, weighted by angle so that how a surface is cut into triangles does not change how it is lit.
- **Hard edges need their own vertices.** `SmoothNormals` averages across shared vertices only, so a cube made of 8 comes out looking like a ball. A cylinder's caps and a cone's base have vertices of their own for the same reason, and `LoadOBJ` splits a vertex that a file lights differently on different faces.
- **Back to facets:** draw a copy with `Normals` set to nil. A zero normal means none here: that corner takes the face's own. Normals turn with the `Transform`, whatever its scale.
- **Cost:** about 12 to 23% more than facets on a scene of spheres, and none for a mesh without normals. Drawing allocates nothing either way.
- **Shape limits:** segments go from 3 to 512 and rings from 2 to 512, so a careless argument cannot ask for gigabytes, and a size that is negative or not a finite number counts as 0. `Point` lights: `Range` 0 means no fading, and a negative `Range` gives no light.
- **Your own lights:** `Light.Brightness(point, normal)` is asked about every cell of a mesh with normals, with the place on the surface and the unit normal there, and once for each face of one without.

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
- **Cost.** A face with a colored corner asks for a color in every cell, which costs more than a flat color; a mesh without vertex colors draws the same cells and allocates nothing, but the shaded benchmarks take 1 to 4% longer, because `Mesh` is 24 bytes larger and is copied for every draw (the wireframe benchmark, whose code did not change, moved 1 to 2% in the same runs). Reading a six-number vertex line takes about 77 ns longer.
- **See it:** the viewer's bundled octahedron has a color on each corner (`N` twice); `C` puts its colors aside for the palette and back.

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
- **Wireframes can be pixels too.** `Mesh.DrawPixels` and `Mesh.DrawDepthPixels` are `Draw` and `DrawDepth` for a `Canvas`: the same edges, drawn as lines of pixels, twice as fine from top to bottom, and `DrawDepthPixels` is hidden by the solids drawn into the same canvas to the pixel. `Canvas.DrawLine` and `Canvas.DrawLineDepth` draw your own lines. A line is one pixel thick, so half a cell tall where it runs sideways; on a `Seamless` canvas (Apple's Terminal) it is a row of whole cells.
- **Ties keep what is there.** A line lying exactly on a surface, a grid on a floor, flickers between the two: lift it a little. There is no bias, because depth changes too fast across a floor seen at a shallow angle for any constant to cover.
- **Not for a mesh's own faces.** To draw a mesh's edges over its own shaded faces, use `Draw`, which does not depend on how those cells compare.
- **Off-screen meshes are skipped.** Drawing starts by asking whether any of a mesh can be on screen, so a scene of many meshes costs little for those out of view. No picture changes.

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

// the camera: the screen's rectangle, centered on the player, kept inside the level
screen := geometry.NewRect(0, 0, float64(fb.Width()), float64(fb.Height()))
view := screen.CenterOn(player.Bounds().Center()).Within(level.Bounds())

// in Draw:
level.Draw(fb, view)
sprites.DrawView(fb, view)
// a score or a hint is text for the screen, drawn after, with no view
```

- **A level as text art.** `FromLines(legend, lines...)` makes a map from a legend of what each rune is; a rune that is not in it, a space included, is the empty tile. `New(w, h, fill)` makes a map of one tile, and `Set` changes a cell. A `Tile` is an `engine.Cell` and whether it is `Solid`. A map holds up to 4,194,304 cells and 65,535 kinds of tile (two bytes a cell); a bigger one is empty.
- **Holes and layers.** A tile whose cell is a hole (blank, no background, no attributes) is not drawn and shows what is under it, as a see-through sprite does, so one map can be laid over another. Any other tile replaces the cell under it.
- **The edge of the world is a wall.** `Solid` is true outside the map, and a map is empty and solid on a nil pointer.
- **The camera is a `Rect`.** `Rect.CenterOn(p)` centers it on a point and `Rect.Within(level)` keeps it inside, or centers it where the level is smaller than the screen. `Map.Draw(fb, view)` and `Layer.DrawView(fb, view)` use only its top left corner, floored in the world, so a sprite stands on its tile however the view moves and the picture moves a whole cell at a time. For a smooth follow, ease the view toward the player yourself with `Vector2.Lerp`.
- **Moving through the map.** `Map.Move(box, delta)` moves a box along X, stopping at the first solid tile, then along Y, so a box that goes into a wall at an angle slides along it. It looks at every tile on the way, so a long frame cannot carry the box through a thin wall, and it stops with the box's edge exactly on the tile's edge, so there is no rounding gap and the box rests against a wall the same at any frame rate: neither habit of the Collision section is needed. It reports whether it was stopped on each axis. Only the tiles a box moves into stop it, so one that starts inside a wall can move on, and one outside the map cannot move.
- **Cost.** Drawing 200 by 60 cells takes about 20 microseconds and a move about 24 nanoseconds, neither allocating. `Layer.Draw` is as fast as before: `DrawView` has a loop of its own.
- **Not included.** Tiles of other sizes than one cell (write a tile twice across for a squarer look), tile animation (`Set` the cell in your own `Update`), loading maps from files, parallax and camera shake.

`examples/world` is a level a screen and a half either way: walk it, slide along the walls, and collect the coins.

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

A `Keyboard` acts like a terminal that reports key releases: a key is down from `Press` until `Release`, `Input.Pressed` is true for one frame, and `EndFrame` ends that frame the way `Run` does. Tests are exact and need no clock. `String` leaves off the trailing spaces of each line and shows characters only, not colors.

To test a whole run (start-up, key bytes in, frames out), give `Config.Input` and `Config.Output` a reader and a writer that are not a terminal: `Run` then runs headless at the `Width` and `Height` you set. See `ExampleRun_headless`.

## Packages

| Package | Purpose |
|---|---|
| `engine` | The loop, `FrameBuffer`, pixel `Canvas`, drawing helpers, keyboard `Input` |
| `geometry` | 2D and 3D vector math, and `Rect` for boxes, collision and cameras |
| `mesh` | 3D meshes (wireframe and shaded), shapes, OBJ loading, lights, and the perspective `Camera` |
| `sprite` | Sprites (a `Cell`, a text-art `Frame` or an `Animation` at a position, optionally see-through) drawn in layers, through a view, with bounds and collision queries |
| `tilemap` | Tile maps written as text art, drawn through a camera, with solid tiles that stop a moving box |

## Compatibility and limits

Built to work in most terminals. The basics need only the alternate screen, cursor movement, and 256-color escape codes (24-bit colors are used on top of that where the terminal says it has them); line wrapping is turned off while a program runs, and the kitty keyboard protocol and focus reporting are used where the terminal has them. All of it is undone when `Run` returns. It is developed on macOS and tested mainly in Ghostty.

- **Without key releases, held keys are a timing guess.** Most terminals report presses and repeats but never releases, so a key counts as held until about 150 ms pass without a repeat, with a short pause when you first hold one. glonk detects the kitty keyboard protocol at startup and uses it where it exists; everywhere else this is what you get.
- **Without key releases, only one key stays held at a time.** The OS repeats just the most recently pressed key, so holding W and then pressing E stops W and you cannot walk and turn together (Terminal.app, for one). Set `Config.LegacyKeys` to force this behavior if a terminal misbehaves with the protocol.
- **Escape waits 50 ms, and Alt chords are ignored.** An Escape press and the start of an arrow key are the same byte, so the Escape key counts only after 50 ms with nothing following it. Alt+key arrives as Escape plus the key and is dropped rather than read as Escape.
- **Ctrl-Z suspends, and some endings cannot be cleaned up.** Ctrl-Z restores the terminal, stops the program, and takes the terminal over again on `fg`; Windows has no suspend. `os.Exit`, `log.Fatal`, a panic in a goroutine you started, `kill -9` and `kill -QUIT` skip the cleanup and leave the terminal raw; type `reset` to repair it. After the first quit signal a second one kills a game that is stuck, unless your program catches those signals itself.
- **Bad meshes and cameras draw nothing instead of failing.** Edges and faces that name a missing vertex are skipped, and normals that are not one per vertex or not finite numbers are not used (the mesh is shaded flat); `Mesh.Validate` finds both, a `Camera` field left at 0 uses a default (a 90 degree FOV, a Near of 0.1, a `CellAspect` of 2), a nonsense FOV, Near or `CellAspect` sees nothing, a mesh whose `Transform` has a number that is not finite or a negative `Scale` is not drawn, and sprites at NaN or infinite positions are not drawn.
- **One character is one column.** A cell holds a single character one column wide. Wide characters (CJK, most emoji) are drawn as `?`, combining marks and joiners are skipped by `DrawText`, and control characters, escape sequences in text included, are drawn as `?` too, so nothing a file name or a game contains can send commands to the terminal. A NUL character is shown as a space, so `Cell{Background: engine.Palette(4)}` is a colored blank. Wide characters are not supported.
- **True color is used where the terminal says it has it.** An `RGB` color is sent exactly when `COLORTERM` is `truecolor` or `24bit` (terminals that can show it set that themselves) and as the closest of palette colors 16 to 255 otherwise; `Attrs` adds bold, dim, underline and reverse. Inside tmux or over ssh `COLORTERM` often does not arrive: export `COLORTERM=truecolor`, or set `Config.Colors` to `engine.ColorsTrue`, if the terminal can show it (and to `engine.Colors256` to turn it off). A terminal that cannot but is told it can shows wrong colors. A `Palette` color is always sent as a palette color, so colors 0 to 15 follow the user's theme. `Shade` gives exact RGB, so shades are finer on a true color terminal. Apple's Terminal does not set `COLORTERM` and keeps the 256-color look.
- **Cells are taken to be twice as tall as wide.** That is what makes a cube look like a cube in most fonts; set `Camera.CellAspect` for a font that differs.
- **Box drawing and half blocks need a Unicode font.** `DrawBox` uses `┌─┐│└┘` and the pixel canvas uses `▀▄`; nearly every monospace font has them. Character shading (`DrawShaded`) needs only ASCII and stays the default in the examples.
- **Apple's Terminal draws a faint line along half-block edges.** Its block characters fall a little short of the cell. glonk detects it (`TERM_PROGRAM=Apple_Terminal`) and `Canvas.Compose` then draws solid cells instead of half blocks, so there is no line but silhouettes are a pixel blockier and pixel lines are rows of whole cells. Set `Canvas.Seamless` yourself to override, for instance inside tmux, which hides that variable.
- **A fixed-size frame larger than the terminal is cut off.** Only the top-left part that fits is shown, and the screen is cleared when the window changes size. Leave `Width` and `Height` at 0 to fill the terminal.
- **`FPS` runs from 1 to 1000;** `Run` returns an error outside that range. A key pressed during start-up, while glonk asks the terminal about the keyboard protocol, arrives as a tap rather than a hold.
- **Without a terminal there is no terminal handling.** A headless `Run` (see `Config.Output`) treats a key as held from its press to its release, as the kitty keyboard protocol reports them, unless `LegacyKeys` is set; a plain letter with no release then stays held. Ctrl-Z does nothing and quit signals are left to your program. Reading from an input that never ends a read (a pipe nobody closes) leaves one goroutine waiting after `Run` returns.
- **Only changes are sent, so do not print to the terminal yourself.** Each frame sends just the cells that differ from the last, as one synchronized update (terminals that do not know that mode ignore it). Text that something else writes while `Run` is running stays on screen until those cells change, the window is resized, or the window comes back to the front; log to a file instead.
- **Resizing can flicker for one frame.**
- **Windows is untested.**

## Planned

Ideas, not promises: drawing images on the canvas.

## Development

```
make check    # formatting, vet, and tests with the race detector
make fuzz     # fuzz the parsers, rasterizers and clipping; FUZZTIME=1m for longer
```

The race detector needs cgo. Where there is no C compiler, run `make test RACE=`.

## Contributing

Issues and pull requests are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md), and everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md). To report a security problem privately, see [SECURITY.md](SECURITY.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md).

## License

MIT. See [LICENSE](LICENSE).

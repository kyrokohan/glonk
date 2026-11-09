# glonk

A terminal rendering engine for Go. Draw 2D, 3D, or both in the same frame, using plain terminal cells.

Website: [glonk.dev](https://glonk.dev)

> Early and pre-1.0: the API may change.

## What it does

- **2D:** cells with foreground and background colors (the 256-color palette, or exact RGB on terminals with true color) and text attributes (bold, dim, underline, reverse), lines, boxes, text, and sprites.
- **3D:** wireframe or solid meshes through a perspective camera, with near-plane clipping. Solid meshes are shaded by any number of lights (directional, ambient, point, or your own) into an ASCII brightness ramp, face by face or, with normals, smoothly across each face. They hide each other by depth and skip faces turned away. A wireframe drawn with depth is hidden by nearer solids too.
- **Shapes and models:** a cube, sphere, cylinder, cone, plane and grid, or OBJ files from any `io.Reader` (a file, `//go:embed`, a download), centered and scaled and drawn wireframe or shaded, in the colors of their materials when loaded from an `fs.FS`.
- **Half blocks:** a pixel `Canvas` with two square-ish pixels per cell, drawn with `▀` and a background color, for twice the vertical detail and shading by color instead of by character.
- **Mixed:** draw a 3D scene first, then a 2D HUD on top, into the same frame.
- **Input:** held keys and one-shot presses, including arrows. Terminals with the [kitty keyboard protocol](https://sw.kovidgoyal.net/kitty/keyboard-protocol/) (Ghostty, kitty, WezTerm, foot, iTerm2, Alacritty) report real key releases, so you can hold several keys at once.
- **Resizing:** the frame follows the terminal window, and a game with a `Resize(width, height int)` method is told its size.
- **Testing:** `engine.Keyboard` presses keys for a game's `Update`, a `FrameBuffer` prints as text, and `Config.Input` and `Config.Output` let a whole `Run` be driven without a terminal. See [Testing a game](#testing-a-game).

## Install

```
go get glonk.dev
```

Requires Go 1.26 or newer.

## Run the examples

```
make run                                      # bounce2d: a ball in a box that fills the window (Esc quits)
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
| M | switch between character shading and half-block pixels |
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
| M, B | character shading or half-block pixels; exact or solid pixel edges |
| D | draw both sides of every face (for models that look inside out) |
| C | change color (a model with colors of its own, like the bundled house, goes from them through the six palette colors and back) |
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

To have the colors of a model's materials, load it with `LoadOBJFS(fsys, name, maxBytes)` from an `fs.FS` (`os.DirFS`, an `embed.FS`, a zip). It follows the OBJ's `mtllib` to the `.mtl` beside it and gives each face the `Kd` of its `usemtl` material in `Mesh.FaceColors`. A face with a color is drawn in it, still lit, and the others use `Material.Color`; the limit and errors are those of `LoadOBJLimit`.

```go
//go:embed ship.obj ship.mtl
var shipFiles embed.FS

ship, err := mesh.LoadOBJFS(shipFiles, "ship.obj", 8<<20)
```

A mesh is placed by its `Transform` (`Position`, `Yaw`, `Pitch`, `Roll`, `Scale`; angles in radians, and positive pitch tips the front up, as it does for the camera) and looks the way its `Material` says (`Color`, and a `Ramp` of brightness characters). `DrawShaded` takes any number of lights: `mesh.Ambient`, `mesh.Directional`, `mesh.Point` (a lamp that fades with distance), or a type of your own with a `Brightness` method.

`LoadOBJ` reads any `io.Reader`, so a file, an embedded asset or a download all work. For input you do not control, use `LoadOBJLimit(r, maxBytes)`: it fails with `mesh.ErrOBJTooLarge` instead of reading without end (a plain `io.LimitReader` would quietly cut the model short).

- **Understood:** vertices (`v`), normals (`vn`), faces (`f`, with or without texture and normal indices, any number of corners, negative indices) and lines (`l`). Faces with more than three corners are split into triangles for shading, and the wireframe shows the original polygons.
- **Skipped:** texture coordinates, groups and curves, and materials too in `LoadOBJ` and `LoadOBJLimit`. Everything in the file becomes one mesh, in the color of its `Material` unless `LoadOBJFS` gave faces colors of their own. If the faces use normals the mesh is shaded smooth, with a vertex split into one per normal where the file lights it differently on different faces (the sides of a cube); if not, it is shaded flat.
- **Colors:** only `Kd`, the diffuse color (three numbers from 0 to 1, or one for a gray), is read, and used as RGB with no gamma conversion. Textures and everything else in a material are skipped, so a material that is only a texture has no color and its faces keep the `Material` color. Each `.mtl` is read up to 1 MiB (or `maxBytes`, if less), at most 8 per model, with 10,000 materials; a name must be a plain path inside the OBJ's folder (no `..`, no leading `/`, no backslash); `mtllib` must come before the `usemtl` that uses it. A `.mtl` that is missing, too large or broken never fails the load: the model comes out in its `Material` color. `os.DirFS` follows symbolic links in its folder, so give it a folder you trust.
- **Orientation:** OBJ files are right-handed and look down -Z; glonk looks down +Z. `LoadOBJ` flips Z and reverses each face so a model keeps its outside and is not mirrored left to right. Its front faces a camera placed on the -Z side.
- **Inside out or invisible?** Some exporters wind faces the other way. Set `Mesh.DoubleSided = true` to draw both sides.
- **Untrusted files:** malformed input returns an error naming the line, never a panic, and the time taken grows in step with the input size. Memory use is roughly 25 times the input size, up to 35 for a file that gives most corners a normal of their own, so bound the input with `LoadOBJLimit` (or `LoadOBJFS`, which also bounds the `.mtl` files it reads) when you do not trust the source.

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

## Wireframes and depth

`Mesh.Draw` paints a mesh's edges over whatever is already on screen, so a wireframe shows through solids and has to be drawn first to stay behind them. `Mesh.DrawDepth` gives each cell of each edge a depth instead: a nearer surface hides it, it hides a farther one, and the order of drawing stops mattering.

```go
cube.DrawShaded(fb, cam, lights...)
ghost.DrawDepth(fb, cam, engine.Cell{Character: '#'}) // hidden where the cube is nearer
```

- **Half-block pixels count.** `Canvas.Compose` gives each cell it fills the depth of its nearer pixel, so anything drawn with a depth afterwards (`DrawDepth`, `FillTriangle`, `FrameBuffer.DrawLineDepth`, `SetDepth`) is hidden behind pixel solids that are nearer. The wireframe itself is still drawn in cells, as characters.
- **Ties keep what is there.** A line lying exactly on a surface, a grid on a floor, flickers between the two: lift it a little. There is no bias, because depth changes too fast across a floor seen at a shallow angle for any constant to cover.
- **Not for a mesh's own faces.** To draw a mesh's edges over its own shaded faces, use `Draw`, which does not depend on how those cells compare.
- **Off-screen meshes are skipped.** Drawing starts by asking whether any of a mesh can be on screen, so a scene of many meshes costs little for those out of view. No picture changes.

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
| `geometry` | 2D and 3D vector math |
| `mesh` | 3D meshes (wireframe and shaded), shapes, OBJ loading, lights, and the perspective `Camera` |
| `sprite` | Sprites (a `Cell` at a position) drawn in layers |

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
- **Apple's Terminal draws a faint line along half-block edges.** Its block characters fall a little short of the cell. glonk detects it (`TERM_PROGRAM=Apple_Terminal`) and `Canvas.Compose` then draws solid cells instead of half blocks, so there is no line but silhouettes are a pixel blockier. Set `Canvas.Seamless` yourself to override, for instance inside tmux, which hides that variable.
- **A fixed-size frame larger than the terminal is cut off.** Only the top-left part that fits is shown, and the screen is cleared when the window changes size. Leave `Width` and `Height` at 0 to fill the terminal.
- **`FPS` runs from 1 to 1000;** `Run` returns an error outside that range. A key pressed during start-up, while glonk asks the terminal about the keyboard protocol, arrives as a tap rather than a hold.
- **Without a terminal there is no terminal handling.** A headless `Run` (see `Config.Output`) treats a key as held from its press to its release, as the kitty keyboard protocol reports them, unless `LegacyKeys` is set; a plain letter with no release then stays held. Ctrl-Z does nothing and quit signals are left to your program. Reading from an input that never ends a read (a pipe nobody closes) leaves one goroutine waiting after `Run` returns.
- **Only changes are sent, so do not print to the terminal yourself.** Each frame sends just the cells that differ from the last, as one synchronized update (terminals that do not know that mode ignore it). Text that something else writes while `Run` is running stays on screen until those cells change, the window is resized, or the window comes back to the front; log to a file instead.
- **Resizing can flicker for one frame.**
- **Windows is untested.**

## Planned

Ideas, not promises: colors for each vertex of a model, drawing images on the canvas.

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

# Examples

Each example runs without a clone:

```
go run glonk.dev/examples/viewer@latest
```

In a clone, `go run ./examples/viewer`, or `make run EXAMPLE=./examples/viewer`.

| Example | What it shows |
|---|---|
| [hello](hello) | The smallest game: a box that fills the window, and an @ that walks across it. |
| [bounce2d](bounce2d) | A ball that bounces round a box that fills the window. |
| [input](input) | What your terminal sends, as glonk reads it: key events, typed text, a paste, the mouse, and a text field. |
| [sprites](sprites) | A ship you steer round a rock, a walking figure and a blinking ghost. |
| [world](world) | A level bigger than the screen, with walls, coins, a coin panel and a pause menu. |
| [cube3d](cube3d) | A walk among shaded cubes, a tree that is a picture, and a cube drawn as lines. |
| [viewer](viewer) | The bundled models and shapes, solid or as lines, faceted or smooth. `go run ./examples/viewer my.obj` shows your own. |

Letters work in either case, Esc quits, and Ctrl+C quits any program built on glonk.

## cube3d

| Keys | What they do |
|---|---|
| W, S | Move forward and back |
| A, D | Step left and right |
| Q, E, Left, Right | Turn |
| Up, Down | Look up and down |
| M | Character shading or the pixel canvas. The wireframe cube becomes pixel lines too. |
| B | On the pixel canvas, exact or solid edges |

Where the terminal reports no key releases only one key stays held, so walking and turning together does not work, and the game says so.

## viewer

| Keys | What they do |
|---|---|
| Arrows | Orbit the model |
| + and -, W and S | Zoom |
| R | Spin on and off |
| N | The next model. The bundled ones end with a sphere, a cylinder, a cone and a die. |
| V | Wireframe or solid |
| M, B | Character shading or the pixel canvas; exact or solid edges |
| D | Draw both sides of every face, for a model that looks inside out |
| C | Change color. A model with colors or a picture of its own goes from them through six colors and back. |
| F | Smooth or faceted shading |
| L | Light from the sun, from a lamp held up and to the left, or from a sunset: an orange sun and a blue sky |

## The pictures

The recordings in the README and on glonk.dev are what the engine drew. cube3d, viewer, sprites and world each play a script of key presses with no terminal, in their `record_test.go`, and `make record` plays them again.

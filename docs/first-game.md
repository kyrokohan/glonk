# Your first game

A glonk game is a type with two methods, `Update` and `Draw`. `engine.Run` calls them and puts each frame on the terminal. This guide gets one on screen and says what each part does.

## Install

```
go get glonk.dev
```

- glonk needs Go 1.26 or newer.
- It has no package at its root. Import the packages you use, `glonk.dev/engine` first.
- Run `go mod tidy` once. Without it, `go run` stops on a missing `go.sum` entry.

## The smallest game

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
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.Set(int(g.x)%fb.Width(), fb.Height()/2,
		engine.Cell{Character: '@', Foreground: engine.Palette(208)})
}

func main() {
	err := engine.Run(context.Background(), &game{}, engine.Config{FPS: 30})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Run it with `go run .` and a box fills the window, with an @ walking across it. Esc quits. It is [examples/hello](../examples/hello) in the repository.

## The loop

- **`engine.Run`** takes over the terminal and calls `Update`, then `Draw`, `FPS` times a second. It sends each frame after `Draw`.
- **`Update(dt, in)`** moves the game on. `dt` is the seconds since the frame before, a quarter of a second at most.
- **`in`** is the keyboard and the mouse: see [Keys, text and the mouse](input.md).
- **`Draw(fb)`** writes the frame. `fb` is blank at the start of every call, so draw everything each time.
- **`FPS`** runs from 1 to 1000. Outside that, `Run` returns an error.
- `Run` calls the two one at a time, from the goroutine that called it.

## Cells and colors

A frame is a grid of cells. Each cell holds a character, a foreground color, a background color and attributes.

```go
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.DrawText(2, 1, "glonk", engine.Palette(220))
	fb.Set(2, 3, engine.Cell{Character: '@', Foreground: engine.RGB(255, 135, 0), Attrs: engine.Bold})
	fb.Set(4, 3, engine.Cell{Character: '~', Background: engine.Palette(27)})

	fmt.Println(fb.String())
	// Output:
	// ┌──────────────────────┐
	// │ glonk                │
	// │                      │
	// │ @ ~                  │
	// └──────────────────────┘
```

- **`Set(x, y, cell)`** writes one cell. Cells count from 0 at the top left, and a write outside the frame is ignored.
- **`DrawText`** writes a line of text and **`DrawBox`** a border. `DrawLine` and `FillRect` draw with a cell.
- **`engine.Color{}`**, the zero color, is the terminal's own default.
- **`engine.Palette(n)`** is one of the terminal's 256 colors.
- **`engine.RGB(r, g, b)`** is an exact color where the terminal has true color, and the nearest of the 256 where it does not.
- **The attributes** are `Bold`, `Dim`, `Underline` and `Reverse`. Join them with `|`.
- **One character takes one column.** A wide character, such as CJK or most emoji, is drawn as `?`.
- **A control character is drawn as `?` too,** so text can never send the terminal a command.

## How a run ends

- **`engine.ErrQuit`** from `Update` ends the run, and `Run` returns `nil`. Any other error ends it too, and `Run` returns that error.
- **Ctrl+C ends any glonk game.** So do `kill` and a closing window: SIGINT, SIGTERM and SIGHUP. `Run` returns `nil`, so a game needs no signal handler.
- **When your `context` ends,** `Run` returns the context's error.
- **`Run` restores the terminal** when it returns.
- **Ctrl+Z suspends.** It restores the terminal and stops the program, and `fg` takes the terminal over again. Windows has no suspend.
- **Some endings skip the cleanup:** `os.Exit`, `log.Fatal`, a panic in a goroutine you started, `kill -9` and `kill -QUIT`. They leave the terminal raw. Type `reset` to repair it.
- **A second quit signal kills a game that is stuck,** unless your program catches those signals itself.

## The size of the frame

```go
func (g *sized) Resize(width, height int) { g.width, g.height = width, height }
```

- **`Width` and `Height` of 0 fill the terminal,** and the frame follows the window when its size changes.
- **A game with a `Resize` method is told the size:** before the first `Update`, and again whenever it changes.
- **A fixed size larger than the terminal is cut off.** Only the top left part that fits is shown.

## Nothing else prints

`Run` sends only the cells that changed since the last frame. So text that something else prints while it runs, a `Println` or a log line, is not wiped by the next frame: it stays until those cells change. Log to a file instead.

## Frame numbers

A game with a `Stats` method is told how each frame went, after the frame was sent:

```go
func (g *sized) Stats(s engine.Stats) { g.stats = s }
```

It keeps the numbers and shows them in its next `Draw`:

```go
	fb.DrawText(0, 0, fmt.Sprintf("%.0f fps  %d bytes", g.stats.FPS, g.stats.Bytes), engine.Color{})
```

- **`FPS`** is the frames a second, averaged over about the last ten.
- **`Update`, `Draw` and `Write`** are how long each part of the frame took. A slow terminal shows in `Write`.
- **`Bytes`** is what was sent for the frame. It is 0 when nothing on screen changed.
- **The bytes decide whether a game works over ssh.** A moving 3D scene on the pixel canvas is about 7 KB a frame.
- **A frame where every pixel changes is far more:** at 100 by 30 cells, 66 KB in 256 colors and 110 KB in true color.
- **A game without the method is not timed,** so asking is the only way to pay for it.
- `cube3d` and the viewer show these numbers on screen.

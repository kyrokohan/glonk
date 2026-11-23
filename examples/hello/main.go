// Command hello is the smallest glonk program: a box that fills the window, and an @ that walks
// across it.
//
//	go run ./examples/hello
//
// Esc quits.
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

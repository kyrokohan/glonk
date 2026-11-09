// Command bounce2d is a ball bouncing around inside a box that fills the window,
// drawn as a sprite.
//
//	go run ./examples/bounce2d
//
// Esc quits.
package main

import (
	"context"
	"fmt"
	"math"
	"os"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/sprite"
)

// hint is the text on the bottom border.
const hint = " Esc quits "

type bounceGame struct {
	width, height int              // the frame's size, from Resize
	velocity      geometry.Vector2 // cells per second
	ball          *sprite.Sprite
	layer         *sprite.Layer
}

// newBounceGame returns the game main runs. It has no size until Run calls Resize.
func newBounceGame() *bounceGame {
	game := &bounceGame{
		velocity: geometry.Vector2{X: 30, Y: 15},
		ball:     sprite.New(geometry.Vector2{X: 10, Y: 10}, 'o'),
		layer:    sprite.NewLayer(),
	}
	game.layer.Add(game.ball)
	return game
}

// A game with a wrong Resize signature would compile and never be told its size.
var _ engine.Resizer = (*bounceGame)(nil)

// Resize keeps the ball bouncing inside the frame, which fills the window.
func (bg *bounceGame) Resize(width, height int) {
	bg.width, bg.height = width, height
}

func (bg *bounceGame) Update(dt float64, in *engine.Input) error {
	if in.Pressed(engine.KeyEscape) {
		return engine.ErrQuit
	}
	bg.ball.Position = bg.ball.Position.Add(bg.velocity.Scale(dt))

	if bg.ball.Position.X >= float64(bg.width-2) {
		bg.velocity.X = -math.Abs(bg.velocity.X)
		bg.ball.Position.X = float64(bg.width - 2)
	}
	if bg.ball.Position.X < 1 {
		bg.velocity.X = math.Abs(bg.velocity.X)
		bg.ball.Position.X = 1
	}

	if bg.ball.Position.Y >= float64(bg.height-2) {
		bg.velocity.Y = -math.Abs(bg.velocity.Y)
		bg.ball.Position.Y = float64(bg.height - 2)
	}
	if bg.ball.Position.Y < 1 {
		bg.velocity.Y = math.Abs(bg.velocity.Y)
		bg.ball.Position.Y = 1
	}

	return nil
}

func (bg *bounceGame) Draw(fb *engine.FrameBuffer) {
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	bg.layer.Draw(fb)

	// On the bottom border, where the ball never goes. Skipped when it would run into
	// the corners.
	if fb.Width() >= len(hint)+4 {
		fb.DrawText(2, fb.Height()-1, hint, engine.Color{})
	}
}

func main() {
	// Width and Height of 0 fill the terminal and follow resizes.
	err := engine.Run(context.Background(), newBounceGame(), engine.Config{FPS: 30})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

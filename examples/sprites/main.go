// Command sprites shows sprites that are more than one character: a ship drawn from text
// art that you steer, a figure with a walk cycle, and a see-through ghost that blinks and
// floats over everything.
//
//	go run ./examples/sprites
//
// The arrow keys steer the ship, and Esc quits.
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
const hint = " arrows steer the ship  Esc quits "

const (
	shipSpeed   = 24.0 // cells a second
	walkerSpeed = 9.0
)

type spriteGame struct {
	width, height int     // the frame's size, from Resize
	t             float64 // seconds since the start, for the ghost's path

	ship, walker, ghost *sprite.Sprite
	walk, blink         *sprite.Animation
	layer               *sprite.Layer
}

// newSpriteGame returns the game main runs. It has no size until Run calls Resize.
func newSpriteGame() *spriteGame {
	orange, yellow, white := engine.Cell{Foreground: engine.Palette(208)}, engine.Cell{Foreground: engine.Palette(226)}, engine.Cell{Foreground: engine.Palette(255)}

	// A walk cycle of four frames. The spaces in the art are holes, because the sprite is an
	// Overlay: the sand shows around and between the limbs.
	walk := sprite.NewAnimation(8,
		sprite.FromLines(yellow, ` o `, `/|\`, `/ \`),
		sprite.FromLines(yellow, ` o `, `/|\`, ` | `),
		sprite.FromLines(yellow, ` o `, `/|\`, `\ /`),
		sprite.FromLines(yellow, ` o `, `/|\`, ` | `),
	)
	walk.Loop = true

	// The ghost blinks every so often. The gap between its eyes and the corners of its
	// art are holes, so the sea shows through it.
	blink := sprite.NewAnimation(1.5,
		sprite.FromLines(white, ` .-. `, `(o o)`, ` |_| `),
		sprite.FromLines(white, ` .-. `, `(- -)`, ` |_| `),
	)
	blink.Loop = true

	g := &spriteGame{
		walk:  walk,
		blink: blink,
		ship: &sprite.Sprite{
			Position: geometry.Vector2{X: 10, Y: 6},
			Overlay:  true,
			Frame: sprite.FromLines(orange,
				`   |\   `,
				`   | \  `,
				`___|__\_`,
				`\______/`,
			),
		},
		layer: sprite.NewLayer(),
	}
	g.walker = &sprite.Sprite{Position: geometry.Vector2{X: 2}, Animation: walk, Overlay: true}
	g.ghost = &sprite.Sprite{Animation: blink, Overlay: true}
	g.layer.Add(g.ship)
	g.layer.Add(g.walker)
	g.layer.Add(g.ghost)
	return g
}

// A game with a wrong Resize signature would compile and never be told its size.
var _ engine.Resizer = (*spriteGame)(nil)

// Resize keeps everything inside the frame, which fills the window.
func (g *spriteGame) Resize(width, height int) {
	g.width, g.height = width, height
	g.place()
}

func (g *spriteGame) Update(dt float64, in *engine.Input) error {
	if in.Pressed(engine.KeyEscape) {
		return engine.ErrQuit
	}

	var steer geometry.Vector2
	if in.IsDown(engine.KeyLeft) {
		steer.X--
	}
	if in.IsDown(engine.KeyRight) {
		steer.X++
	}
	if in.IsDown(engine.KeyUp) {
		steer.Y--
	}
	if in.IsDown(engine.KeyDown) {
		steer.Y++
	}
	// Normalized, so that steering diagonally is no faster. The ship stays on the sea, inside
	// the border and above the sand.
	g.ship.Position = g.ship.Position.Add(steer.Normalize().Scale(shipSpeed * dt))
	shipW, shipH := g.ship.Frame.Width(), g.ship.Frame.Height()
	g.ship.Position.X = max(1, min(g.ship.Position.X, float64(g.width-shipW-1)))
	g.ship.Position.Y = max(1, min(g.ship.Position.Y, float64(g.height-shipH-sandRows-1)))

	// The figure walks along the sand and comes back in from the left, a cell at a time.
	g.walk.Advance(dt)
	g.walker.Position.X += walkerSpeed * dt
	if g.walker.Position.X >= float64(g.width-1) {
		g.walker.Position.X = -float64(g.walk.Frame().Width())
	}

	g.blink.Advance(dt)
	g.t += dt
	g.place()
	return nil
}

// place puts the walker on the sand and the ghost on its slow loop over the sea, for the
// current size and time.
func (g *spriteGame) place() {
	g.walker.Position.Y = float64(g.height - sandRows - 1)
	g.ghost.Position = geometry.Vector2{
		X: float64(g.width)/2 + float64(g.width)/3*math.Cos(0.4*g.t) - 2,
		Y: float64(g.height)/3 + 3*math.Sin(0.8*g.t),
	}
}

// sandRows is how many rows of sand run along the bottom, above the border.
const sandRows = 4

// drawSea fills the frame with sea above and sand below, which is what the sprites are
// see-through onto.
func drawSea(fb *engine.FrameBuffer) {
	for y := range fb.Height() {
		for x := range fb.Width() {
			cell := engine.Cell{Background: engine.Palette(17)} // sea
			if (x+2*y)%6 == 0 {
				cell.Character, cell.Foreground = '~', engine.Palette(24)
			}
			if y >= fb.Height()-sandRows-1 {
				cell = engine.Cell{Background: engine.Palette(94)} // sand
				if (3*x+y)%7 == 0 {
					cell.Character, cell.Foreground = '.', engine.Palette(136)
				}
			}
			fb.Set(x, y, cell)
		}
	}
}

func (g *spriteGame) Draw(fb *engine.FrameBuffer) {
	drawSea(fb)
	g.layer.Draw(fb)
	// The border goes on top, so the figure walks out from behind it instead of over it.
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})

	// On the bottom border, where nothing goes. Skipped when it would run into the corners.
	if fb.Width() >= len(hint)+4 {
		fb.DrawText(2, fb.Height()-1, hint, engine.Color{})
	}
}

func main() {
	// Width and Height of 0 fill the terminal and follow resizes.
	err := engine.Run(context.Background(), newSpriteGame(), engine.Config{FPS: 30})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

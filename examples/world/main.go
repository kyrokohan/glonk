// Command world is a small world bigger than the screen: a level written as text art, drawn
// through a camera that follows you, with walls and water that stop you and let you slide
// along them, and coins to collect.
//
//	go run ./examples/world
//
// The arrow keys walk, and Esc quits.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/sprite"
	"glonk.dev/tilemap"
)

const (
	speedX = 24.0 // cells a second sideways
	speedY = 12.0 // and up and down: cells are about twice as tall as wide
)

// legend is what each rune of levelText is. A coin and the start are floor under the sprite
// that is put there.
var legend = map[rune]tilemap.Tile{
	'#': {Cell: engine.Cell{Character: '#', Foreground: engine.Palette(240), Background: engine.Palette(236)}, Solid: true},
	'.': {Cell: engine.Cell{Character: '.', Foreground: engine.Palette(238), Background: engine.Palette(234)}},
	'"': {Cell: engine.Cell{Character: '"', Foreground: engine.Palette(34), Background: engine.Palette(22)}},
	'~': {Cell: engine.Cell{Character: '~', Foreground: engine.Palette(39), Background: engine.Palette(18)}, Solid: true},
	'o': {Cell: engine.Cell{Character: '.', Foreground: engine.Palette(238), Background: engine.Palette(234)}},
	'@': {Cell: engine.Cell{Character: '.', Foreground: engine.Palette(238), Background: engine.Palette(234)}},
}

type worldGame struct {
	width, height int // the frame's size, from Resize

	level  *tilemap.Map
	player *sprite.Sprite
	hero   *sprite.Layer // the player, a layer of its own so that it draws over the coins
	coins  *sprite.Layer
	spin   *sprite.Animation // every coin shows the same frame
	score  int
	total  int

	// view is the part of the world on screen: the screen's rectangle, centered on the
	// player and kept inside the level.
	view geometry.Rect
}

// newWorldGame returns the game main runs: the level, a coin for every o in its text and the
// player at the @. It has no size until Run calls Resize.
func newWorldGame() *worldGame {
	lines := strings.Split(strings.Trim(levelText, "\n"), "\n")
	g := &worldGame{
		level: tilemap.FromLines(legend, lines...),
		hero:  sprite.NewLayer(),
		coins: sprite.NewLayer(),
	}

	yellow := engine.Cell{Foreground: engine.Palette(226)}
	g.spin = sprite.NewAnimation(6,
		sprite.FromLines(yellow, `o`),
		sprite.FromLines(yellow, `O`),
		sprite.FromLines(yellow, `0`),
		sprite.FromLines(yellow, `O`),
	)
	g.spin.Loop = true

	g.player = &sprite.Sprite{
		Frame:   sprite.FromLines(engine.Cell{Foreground: engine.Palette(208)}, `(@)`),
		Overlay: true, // the floor shows behind the letters
	}
	for y, line := range lines {
		x := 0
		for _, r := range line {
			at := geometry.Vector2{X: float64(x), Y: float64(y)}
			switch r {
			case 'o':
				g.coins.Add(&sprite.Sprite{Position: at, Animation: g.spin, Overlay: true})
				g.total++
			case '@':
				g.player.Position = at
			}
			x++
		}
	}
	g.hero.Add(g.player)
	return g
}

// A game with a wrong Resize signature would compile and never be told its size.
var _ engine.Resizer = (*worldGame)(nil)

// Resize keeps the camera right for the new size of the window.
func (g *worldGame) Resize(width, height int) {
	g.width, g.height = width, height
	g.follow()
}

// follow puts the camera on the player: the screen's rectangle centered on the middle of the
// player, and moved back inside the level where it would show past its edge. Where the level
// is smaller than the screen it is centered on that axis.
func (g *worldGame) follow() {
	screen := geometry.NewRect(0, 0, float64(g.width), float64(g.height))
	g.view = screen.CenterOn(g.player.Bounds().Center()).Within(g.level.Bounds())
}

func (g *worldGame) Update(dt float64, in *engine.Input) error {
	if in.Pressed(engine.KeyEscape) {
		return engine.ErrQuit
	}

	var d geometry.Vector2
	if in.IsDown(engine.KeyLeft) {
		d.X -= speedX * dt
	}
	if in.IsDown(engine.KeyRight) {
		d.X += speedX * dt
	}
	if in.IsDown(engine.KeyUp) {
		d.Y -= speedY * dt
	}
	if in.IsDown(engine.KeyDown) {
		d.Y += speedY * dt
	}
	// Move stops the player's box at walls and water, exactly on their edges, and lets it slide
	// along them. It looks at every tile on the way, so it needs no cap on dt, however long a
	// frame took.
	moved, _, _ := g.level.Move(g.player.Bounds(), d)
	g.player.Position = moved.Min

	// Coins the player is on. They are collected first and taken out after, because a layer must
	// not change while it is being asked.
	var got []*sprite.Sprite
	for coin := range g.coins.Hits(g.player.Bounds()) {
		got = append(got, coin)
	}
	for _, coin := range got {
		g.coins.Remove(coin)
		g.score++
	}

	g.spin.Advance(dt)
	g.follow()
	return nil
}

func (g *worldGame) Draw(fb *engine.FrameBuffer) {
	g.level.Draw(fb, g.view)
	g.coins.DrawView(fb, g.view)
	g.hero.DrawView(fb, g.view)

	// The text goes on the screen, not in the world, so it is drawn without the view.
	status := fmt.Sprintf(" coins %d/%d ", g.score, g.total)
	if g.score == g.total {
		status = fmt.Sprintf(" all %d coins found! ", g.total)
	}
	fb.DrawText(1, 0, status, engine.Color{})
	if hint := " arrows walk  Esc quits "; fb.Width() >= len(hint)+2 {
		fb.DrawText(1, fb.Height()-1, hint, engine.Color{})
	}
}

func main() {
	// Width and Height of 0 fill the terminal and follow resizes.
	err := engine.Run(context.Background(), newWorldGame(), engine.Config{FPS: 30})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

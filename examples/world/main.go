// Command world is a small world bigger than the screen: a level written as text art, drawn
// through a camera that follows you, with walls and water that stop you and let you slide
// along them, and coins to collect.
//
//	go run ./examples/world
//
// The arrow keys walk, and Esc pauses with a menu to resume or quit. The coin count is a panel
// of the widget package, drawn over the world.
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
	"glonk.dev/widget"
)

const (
	speedX = 24.0 // cells a second sideways
	speedY = 12.0 // and up and down: cells are about twice as tall as wide
)

// The pause menu's items, in order.
const (
	resumeItem = iota
	quitItem
)

// The heads-up display: the count of coins, as text and as a bar, in a panel at the top left.
const (
	barWidth = 16
	hudRows  = 3 // the panel's border and the one row inside it
)

// hudStyle is the colors of the panel, its text and the pause menu: light on dark, so that
// the world does not show through.
var hudStyle = engine.Cell{Foreground: engine.Palette(252), Background: engine.Palette(235)}

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

	paused bool
	menu   widget.Menu // the pause menu, which Update runs while paused

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
		menu:  widget.Menu{Items: []string{"Resume", "Quit"}, Style: hudStyle},
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
	if g.paused {
		return g.updatePaused(in)
	}
	if in.Pressed(engine.KeyEscape) {
		g.paused, g.menu.Selected = true, resumeItem
		return nil
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

// updatePaused runs the pause menu while the world stands still: Resume or Esc goes back to
// the game, and Quit ends it.
func (g *worldGame) updatePaused(in *engine.Input) error {
	choice, chosen := g.menu.Update(in)
	switch {
	case in.Pressed(engine.KeyEscape), chosen && choice == resumeItem:
		g.paused = false
	case chosen && choice == quitItem:
		return engine.ErrQuit
	}
	return nil
}

func (g *worldGame) Draw(fb *engine.FrameBuffer) {
	g.level.Draw(fb, g.view)
	g.coins.DrawView(fb, g.view)
	g.hero.DrawView(fb, g.view)

	// What follows goes on the screen, not in the world, so it is drawn without the view.
	g.drawHUD(fb)
	hint := " arrows walk  Esc pauses "
	if g.paused {
		hint = " arrows choose  Enter picks  Esc resumes "
	}
	if fb.Width() >= len(hint)+2 {
		fb.DrawText(1, fb.Height()-1, hint, engine.Color{})
	}
	if g.paused {
		g.drawPause(fb)
	}
}

// drawHUD draws the panel at the top left: the coins collected, as a count and as a bar, and
// in the title once there are no more.
func (g *worldGame) drawHUD(fb *engine.FrameBuffer) {
	title := "coins"
	if g.score == g.total {
		title = fmt.Sprintf("all %d coins found!", g.total)
	}
	// The count has as many cells as the last count will, so the bar does not move as it grows.
	count := fmt.Sprintf("%d/%d", g.score, g.total)
	countWidth := len(fmt.Sprintf("%d/%d", g.total, g.total))

	widget.Panel{Title: title, Style: hudStyle, Fill: engine.Cell{Background: hudStyle.Background}}.
		Draw(fb, 0, 0, countWidth+barWidth+5, hudRows)
	widget.Label{Text: count, Align: widget.Right, Style: hudStyle}.Draw(fb, 2, 1, countWidth)
	widget.Bar{
		Value: float64(g.score) / float64(g.total), // NaN with no coins at all, which is an empty bar
		Full:  engine.Cell{Character: '█', Foreground: engine.Palette(226), Background: hudStyle.Background},
		Empty: engine.Cell{Character: '░', Foreground: engine.Palette(240), Background: hudStyle.Background},
	}.Draw(fb, countWidth+3, 1, barWidth)
}

// drawPause draws the pause menu in a panel in the middle of the screen.
func (g *worldGame) drawPause(fb *engine.FrameBuffer) {
	const w, h = 16, 4 // the border and a row for each of the two items
	x, y := (fb.Width()-w)/2, (fb.Height()-h)/2
	widget.Panel{Title: "paused", Style: hudStyle, Fill: engine.Cell{Background: hudStyle.Background}}.Draw(fb, x, y, w, h)
	g.menu.Draw(fb, x+2, y+1, w-4, h-2)
}

func main() {
	// Width and Height of 0 fill the terminal and follow resizes.
	err := engine.Run(context.Background(), newWorldGame(), engine.Config{FPS: 30})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

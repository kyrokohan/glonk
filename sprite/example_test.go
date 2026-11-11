package sprite_test

import (
	"fmt"
	"math"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/sprite"
)

// A sprite is a Cell at a position, so it can have colors and attributes too. A layer
// draws its sprites in order, and sprites can be taken out of it again.
func ExampleLayer() {
	fb, err := engine.NewFrameBuffer(12, 5)
	if err != nil {
		panic(err)
	}

	layer := sprite.NewLayer()
	layer.Add(sprite.New(geometry.Vector2{X: 1.9, Y: 1}, 'a')) // a fraction is floored: column 1
	layer.Add(&sprite.Sprite{
		Position: geometry.Vector2{X: 7, Y: 3},
		Cell:     engine.Cell{Character: 'b', Foreground: engine.Palette(208), Attrs: engine.Bold},
	})
	enemy := sprite.New(geometry.Vector2{X: 4, Y: 2}, 'x')
	layer.Add(enemy)
	layer.Add(sprite.New(geometry.Vector2{X: -3, Y: 2}, 'c'))         // off screen: not drawn
	layer.Add(sprite.New(geometry.Vector2{X: math.NaN(), Y: 2}, 'd')) // no position: not drawn

	draw := func() {
		fb.Clear()
		fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
		layer.Draw(fb)
		fmt.Println(fb)
	}

	draw()
	fmt.Println(layer.Len(), "sprites")
	layer.Remove(enemy)
	draw()
	fmt.Println(layer.Len(), "sprites")
	// Output:
	// ┌──────────┐
	// │a         │
	// │   x      │
	// │      b   │
	// └──────────┘
	// 5 sprites
	// ┌──────────┐
	// │a         │
	// │          │
	// │      b   │
	// └──────────┘
	// 4 sprites
}

// A frame is a picture of several cells, made here from text art: a space is a blank
// cell, and the style gives the letters their color. A sprite shows its frame with the
// top left cell at its position, and whatever is off the frame is cut off, cell by cell.
func ExampleFromLines() {
	fb, err := engine.NewFrameBuffer(12, 6)
	if err != nil {
		panic(err)
	}
	ship := sprite.FromLines(engine.Cell{Foreground: engine.Palette(208)},
		` /\ `,
		`/__\`,
	)

	layer := sprite.NewLayer()
	layer.Add(&sprite.Sprite{Position: geometry.Vector2{X: 2, Y: 1}, Frame: ship})
	layer.Add(&sprite.Sprite{Position: geometry.Vector2{X: 10, Y: 3}, Frame: ship}) // runs off the right edge

	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	layer.Draw(fb)
	fmt.Println(fb)
	fmt.Println(ship.Width(), "by", ship.Height())
	// Output:
	// ┌──────────┐
	// │  /\      │
	// │ /__\     │
	// │          /
	// │         /_
	// └──────────┘
	// 4 by 2
}

// Overlay makes a sprite see-through. Without it, a sprite stamps over the cells under
// it, and the holes of its art erase what is behind them; with it, the holes show the
// floor, and a letter keeps the color of the floor it stands on.
func ExampleSprite_overlay() {
	ghost := sprite.FromLines(engine.Cell{Foreground: engine.Palette(15)},
		`.-.`,
		`o o`,
		`|_|`,
	)
	floor := func() *engine.FrameBuffer {
		fb, err := engine.NewFrameBuffer(7, 3)
		if err != nil {
			panic(err)
		}
		for y := range 3 {
			for x := range 7 {
				fb.Set(x, y, engine.Cell{Character: '#', Background: engine.Palette(22)})
			}
		}
		return fb
	}

	for _, overlay := range []bool{false, true} {
		fb := floor()
		layer := sprite.NewLayer()
		layer.Add(&sprite.Sprite{Position: geometry.Vector2{X: 2, Y: 0}, Frame: ghost, Overlay: overlay})
		layer.Draw(fb)
		fmt.Printf("overlay %v:\n%v\n", overlay, fb)
	}
	// Output:
	// overlay false:
	// ##.-.##
	// ##o o##
	// ##|_|##
	// overlay true:
	// ##.-.##
	// ##o#o##
	// ##|_|##
}

// An Animation is frames and a speed. Advance it from the game's Update with the time that
// passed, and a sprite that has it shows the current frame. It plays once, or over and
// over if Loop is set.
func ExampleAnimation() {
	walk := sprite.NewAnimation(4, // four frames a second
		sprite.FromLines(engine.Cell{}, `o`, `|`),
		sprite.FromLines(engine.Cell{}, `o`, `/`),
		sprite.FromLines(engine.Cell{}, `o`, `|`),
		sprite.FromLines(engine.Cell{}, `o`, `\`),
	)
	walk.Loop = true

	fb, err := engine.NewFrameBuffer(4, 2)
	if err != nil {
		panic(err)
	}
	layer := sprite.NewLayer()
	layer.Add(&sprite.Sprite{Position: geometry.Vector2{X: 1}, Animation: walk})

	for range 5 {
		fb.Clear()
		layer.Draw(fb)
		fmt.Printf("frame %d:%q\n", walk.Index(), fb.String())
		walk.Advance(0.25) // a quarter of a second
	}
	fmt.Println("done:", walk.Done())
	// Output:
	// frame 0:" o\n |"
	// frame 1:" o\n /"
	// frame 2:" o\n |"
	// frame 3:" o\n \\"
	// frame 0:" o\n |"
	// done: false
}

// A sprite's box is where it is, as wide and tall as what it draws, from its Position as it
// is, not floored to the cell it is drawn in.
func ExampleSprite_Bounds() {
	ship := &sprite.Sprite{
		Position: geometry.Vector2{X: 2.5, Y: 1},
		Frame:    sprite.FromLines(engine.Cell{}, " /\\ ", "/__\\"),
	}
	fmt.Println(ship.Bounds())
	fmt.Println(ship.Bounds().Size())
	// Output:
	// {{2.5 1} {6.5 3}}
	// {4 2}
}

// Hits asks a layer which sprites touch a box, in the order they are drawn. A sprite hits its
// own box, and one that only touches along an edge does not hit.
func ExampleLayer_Hits() {
	player := &sprite.Sprite{
		Position: geometry.Vector2{X: 5, Y: 2},
		Frame:    sprite.FromLines(engine.Cell{}, "##", "##"), // covers 5 up to 7, 2 up to 4
	}
	layer := sprite.NewLayer()
	layer.Add(player)
	layer.Add(sprite.New(geometry.Vector2{X: 6, Y: 3}, 'a'))  // inside the player
	layer.Add(sprite.New(geometry.Vector2{X: 7, Y: 2}, 'b'))  // starts where the player ends
	layer.Add(sprite.New(geometry.Vector2{X: 20, Y: 2}, 'c')) // far away

	for other := range layer.Hits(player.Bounds()) {
		if other != player {
			fmt.Printf("%c at %v\n", other.Cell.Character, other.Position)
		}
	}
	// Output:
	// a at {6 3}
}

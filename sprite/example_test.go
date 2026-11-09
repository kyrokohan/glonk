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

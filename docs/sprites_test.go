package docs

import (
	"fmt"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/sprite"
	"glonk.dev/tilemap"
)

// A sprite is a picture at a position, and a layer draws its sprites in the order they were added.
func Example_sprites() {
	fb, _ := engine.NewFrameBuffer(18, 5)

	ship := sprite.FromLines(engine.Cell{Foreground: engine.Palette(208)},
		` /\ `,
		`/__\`,
	)
	layer := sprite.NewLayer()
	layer.Add(&sprite.Sprite{Position: geometry.Vector2{X: 3, Y: 2}, Frame: ship, Overlay: true})
	layer.Add(sprite.New(geometry.Vector2{X: 12, Y: 3}, '@')) // a sprite of one cell

	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.DrawText(1, 3, "~~~~~~~~~~~~~~~~", engine.Palette(27)) // the sea, drawn before the sprites
	layer.Draw(fb)

	fmt.Println(fb)
	// Output:
	// ┌────────────────┐
	// │                │
	// │   /\           │
	// │~~/__\~~~~~@~~~~│
	// └────────────────┘
}

// An animation plays its frames in time, so it runs at the same speed at any frame rate.
func Example_animation() {
	walk := sprite.NewAnimation(4, // four frames a second
		sprite.FromLines(engine.Cell{}, `o`, `|`),
		sprite.FromLines(engine.Cell{}, `o`, `/`),
		sprite.FromLines(engine.Cell{}, `o`, `\`),
	)
	walk.Loop = true

	for range 4 {
		fmt.Print(walk.Index(), " ")
		walk.Advance(0.25) // in Update: the dt it was given
	}
	// Output:
	// 0 1 2 0
}

// A sprite's box is where it is, and as large as what it draws. PushOutOf moves a box out of a
// wall, and Hits lists the sprites a box touches.
func Example_collision() {
	wall := geometry.NewRect(10, 0, 2, 8)
	player := sprite.New(geometry.Vector2{X: 9.5, Y: 3}, '@') // half a cell into the wall

	if push := player.Bounds().PushOutOf(wall); push != (geometry.Vector2{}) {
		player.Position = player.Position.Add(push)
	}
	fmt.Println("pushed out to", player.Position.X, player.Position.Y)

	enemies := sprite.NewLayer()
	enemies.Add(sprite.New(geometry.Vector2{X: 8.5, Y: 3.5}, 'x'))
	enemies.Add(sprite.New(geometry.Vector2{X: 2, Y: 6}, 'x'))
	for enemy := range enemies.Hits(player.Bounds()) {
		fmt.Println("touching the enemy at", enemy.Position.X, enemy.Position.Y)
	}
	// Output:
	// pushed out to 9 3
	// touching the enemy at 8.5 3.5
}

// smallLevel is a level written as text art: walls all round it, and one inside.
func smallLevel() *tilemap.Map {
	return tilemap.FromLines(map[rune]tilemap.Tile{
		'#': {Cell: engine.Cell{Character: '#'}, Solid: true},
		'.': {Cell: engine.Cell{Character: '.'}},
	},
		"################",
		"#......#.......#",
		"#......#.......#",
		"#..............#",
		"################",
	)
}

// Move takes a box through the map and stops it at a wall. The view is the part of the map on
// screen: a rectangle, centered on the player and kept inside the level.
func Example_level() {
	level := smallLevel()
	player := sprite.New(geometry.Vector2{X: 2, Y: 1}, '@')

	step := geometry.Vector2{X: 9, Y: 0} // nine cells to the right, where a wall is in the way
	moved, hitX, hitY := level.Move(player.Bounds(), step)
	player.Position = moved.Min
	fmt.Println("now at", player.Position.X, player.Position.Y, "stopped across:", hitX, "down:", hitY)

	fb, _ := engine.NewFrameBuffer(10, 3)
	screen := geometry.NewRect(0, 0, float64(fb.Width()), float64(fb.Height()))
	view := screen.CenterOn(player.Bounds().Center()).Within(level.Bounds())

	sprites := sprite.NewLayer()
	sprites.Add(player)
	level.Draw(fb, view)
	sprites.DrawView(fb, view)

	fmt.Println(fb)
	// Output:
	// now at 6 1 stopped across: true down: false
	// ##########
	// .....@#...
	// ......#...
}

// Sees says whether a wall is between two points, Cast what a ray hits, and a Route the way
// round the walls.
func Example_sightAndRoutes() {
	level := smallLevel()
	enemy, player := geometry.Vector2{X: 2.5, Y: 1.5}, geometry.Vector2{X: 10.5, Y: 1.5}

	fmt.Println("the enemy sees the player:", level.Sees(enemy, player))

	hit, _ := level.Cast(enemy, geometry.Vector2{X: 1}) // a shot to the right
	fmt.Println("a shot hits the tile", hit.X, hit.Y, "after", hit.Distance, "cells")

	var route tilemap.Route
	route.To(level, player) // searched once, for every mover that wants this target
	steps, _ := route.Steps(enemy)
	next, _ := route.Next(enemy)
	fmt.Println(steps, "steps round the wall, the first to", next.X, next.Y)
	// Output:
	// the enemy sees the player: false
	// a shot hits the tile 7 1 after 4.5 cells
	// 8 steps round the wall, the first to 3.5 1.5
}

package tilemap_test

import (
	"fmt"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/tilemap"
)

// A level is written as text art, with a legend of what each rune is. The view is the part
// of it on screen, so a level can be larger than the window.
func ExampleFromLines() {
	level := tilemap.FromLines(map[rune]tilemap.Tile{
		'#': {Cell: engine.Cell{Character: '#'}, Solid: true},
		'.': {Cell: engine.Cell{Character: '.'}},
	},
		"##########",
		"#........#",
		"#..####..#",
		"#........#",
		"##########",
	)
	fmt.Println(level.Width(), "by", level.Height(), level.Solid(0, 0), level.Solid(1, 1), level.Solid(-1, 0))

	fb, err := engine.NewFrameBuffer(6, 4)
	if err != nil {
		panic(err)
	}
	level.Draw(fb, geometry.NewRect(2, 1, 6, 4)) // 2 columns in and 1 row down
	fmt.Println(fb)
	// Output:
	// 10 by 5 true false true
	// ......
	// .####.
	// ......
	// ######
}

// A tile says what it is: FromLines gives each tile of its legend the rune that made it as its
// Kind, so a game can ask the map instead of keeping the text of the level beside it. Here a
// wall and a crate look and act alike, and only the Kind tells them apart.
func ExampleMap_At() {
	level := tilemap.FromLines(map[rune]tilemap.Tile{
		'#': {Cell: engine.Cell{Character: '#'}, Solid: true},
		'X': {Cell: engine.Cell{Character: '#'}, Solid: true},
		'.': {Cell: engine.Cell{Character: '.'}},
	},
		"#####",
		"#.X.#",
		"#####",
	)
	wall, crate := level.At(0, 0), level.At(2, 1)
	fmt.Println(string(wall.Kind), string(crate.Kind), wall.Cell == crate.Cell, wall.Solid == crate.Solid)
	// Output:
	// # X true true
}

// Sees says whether a straight line between two points crosses no solid tile. Here two people in a
// room with a pillar: the one at the left sees the one above, but not the one behind the pillar.
func ExampleMap_Sees() {
	room := tilemap.FromLines(map[rune]tilemap.Tile{'#': {Solid: true}},
		"#########",
		"#.......#",
		"#...#...#",
		"#.......#",
		"#########",
	)
	left := geometry.Vector2{X: 1.5, Y: 2.5}
	fmt.Println(room.Sees(left, geometry.Vector2{X: 3.5, Y: 1.5})) // above and to the right
	fmt.Println(room.Sees(left, geometry.Vector2{X: 7.5, Y: 2.5})) // behind the pillar
	// Output:
	// true
	// false
}

// Cast shoots a ray and returns the first solid tile it meets, where it enters it and how far it
// went. The direction can be any length. Here a shot down a corridor ends at the wall.
func ExampleMap_Cast() {
	corridor := tilemap.FromLines(map[rune]tilemap.Tile{'#': {Solid: true}},
		"##########",
		"#........#",
		"##########",
	)
	hit, ok := corridor.Cast(geometry.Vector2{X: 2.5, Y: 1.5}, geometry.Vector2{X: 1, Y: 0})
	fmt.Println(ok, hit.X, hit.Y, hit.Point, hit.Distance)
	// Output:
	// true 9 1 {9 1.5} 6.5
}

// A Route says how to walk to a target round the walls: To searches the tiles once, and Next gives
// every mover the center of the neighbor tile to step to. Here the enemy at the left reaches the
// player at the right by the doorway at the top of the wall, as a diagonal step does not cut its
// corner.
func ExampleRoute() {
	level := tilemap.FromLines(map[rune]tilemap.Tile{'#': {Solid: true}},
		"........",
		"...#....",
		"...#....",
		"########",
	)
	var route tilemap.Route
	route.To(level, geometry.Vector2{X: 6.5, Y: 1.5}) // the player's tile

	enemy := geometry.Vector2{X: 0.5, Y: 2.5}
	steps, _ := route.Steps(enemy)
	fmt.Println(steps, "steps away")
	for {
		next, ok := route.Next(enemy)
		if !ok {
			break
		}
		fmt.Printf("(%d, %d) ", int(next.X), int(next.Y))
		enemy = next
	}
	fmt.Println()
	// Output:
	// 6 steps away
	// (1, 1) (2, 0) (3, 0) (4, 0) (5, 0) (6, 1)
}

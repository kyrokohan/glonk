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

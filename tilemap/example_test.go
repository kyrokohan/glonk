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

# Sprites, collision and tile maps

A sprite is a picture at a position. A tile map is a level bigger than the screen. Both are drawn in cells, and a rectangle says which part of the world is on screen.

## Sprites

```go
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
```

- **A sprite** is one cell (`sprite.New`), a `Frame` of several made from text art (`FromLines`), or an `Animation`.
- **A `Layer`** draws its sprites in the order they were added.
- **A space is a hole.** `FromLines` puts the style's colors on every other character. For a color of its own in each cell, use `NewFrame` and `Frame.Set`.
- **`Overlay` shows what is behind a sprite's holes.** Without it, a sprite replaces the cells under it, holes included.
- **Do not overlay sprites on the pixel canvas.** Only the background of those cells is kept.
- **The position is the top left of the frame.** A fraction is floored when it is drawn, and a frame that runs off an edge is cut off cell by cell.
- **Frames are shared by pointer.** Two sprites with one `Frame` see each other's `Set`. `Clone` makes a copy.

## Animations

```go
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
```

- **`Advance(dt)`** from `Update` plays an animation in time, so it runs at the same speed at any frame rate.
- **It stops on its last frame,** and `Done` says so, unless `Loop` is set.

## Collision

```go
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
```

- **`Bounds()`** is a sprite's box: a `geometry.Rect` from its position, as it is and not floored, the size of what it draws.
- **The whole frame counts,** holes included, so a ghost with empty corners still touches at its corners.
- **A `Rect` is half-open.** It covers `Min` up to but not including `Max`: a sprite at X=2 that is 3 wide covers 2 up to 5, and one at X=5 does not touch it.
- **`PushOutOf(wall)`** is the smallest move, along one axis, that takes a box out of another. Steering into a wall at an angle slides along it.
- **`Hits(box)`** lists the sprites of a layer whose boxes overlap one, in draw order. It allocates nothing.
- **A sprite in the layer hits its own box.** Skip it by comparing pointers. The layer must not change while `Hits` runs.

Two habits go with pushing:

- **Round the position after a push,** to a millionth. A push can leave a sprite a hair short of a whole cell, and drawing floors it.
- **Cap the time of a step.** A sprite that moves farther than its own size in one frame can come out the other side of a wall.

`examples/sprites` does both. Not included: collision by pixel or along the whole path of a move, and collision layers. Keep a `Layer` or a list for each kind of thing.

## A level

```go
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
```

- **`FromLines(legend, lines...)`** makes a map from text art. A rune that is not in the legend, a space included, is the empty tile.
- **A `Tile`** is an `engine.Cell`, whether it is `Solid`, and its `Kind`.
- **`Kind` is the rune that made the tile,** so `level.At(x, y).Kind == 'X'` tells a crate from a wall that looks the same. List a marker as `'P': {}` to find it.
- **A tile whose cell is a hole is not drawn,** so one map can be laid over another.
- **The edge of the world is a wall.** `Solid` is true outside the map.
- **A map holds up to 4,194,304 cells and 65,535 kinds of tile.** A bigger one is empty.
- `New(w, h, fill)` makes a map of one tile, and `Set` changes a cell.

## Moving through it

```go
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
```

- **`Move(box, delta)`** moves a box along X, then along Y, and stops it at the first solid tile. A box that goes into a wall at an angle slides along it.
- **It looks at every tile on the way,** so a long frame cannot carry a box through a thin wall.
- **It stops the box exactly on the tile's edge,** the same at any frame rate. Neither habit of pushing is needed here.
- **It says whether it was stopped** on each axis.
- **Only the tiles a box moves into stop it.** One that starts inside a wall can move on, and one outside the map cannot move.

## The view

```go
	screen := geometry.NewRect(0, 0, float64(fb.Width()), float64(fb.Height()))
	view := screen.CenterOn(player.Bounds().Center()).Within(level.Bounds())

	sprites := sprite.NewLayer()
	sprites.Add(player)
	level.Draw(fb, view)
	sprites.DrawView(fb, view)
```

- **The view is a `Rect`:** the screen's rectangle, centered on the player, kept inside the level.
- **`Within`** centers the view where the level is smaller than the screen.
- **`Map.Draw` and `Layer.DrawView` use only its top left corner, floored,** so the picture moves a whole cell at a time. For a smooth follow, ease the view toward the player with `Vector2.Lerp`.
- A score or a hint is text for the screen. Draw it after, with no view.

## Sight and routes

```go
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
```

- **`Sees(a, b)`** says whether a straight line between two points crosses no solid tile.
- **`Cast(from, direction)`** shoots a ray and gives the first solid tile it meets: its `X` and `Y`, the `Point` where the ray enters it, and the `Distance` in cells.
- **Both walk the grid exactly,** so no wall is thin enough to miss. Outside the map is solid, so a ray from inside always hits something.
- **`Route.To(level, targets...)`** searches for the way to the nearest target. `Next(point)` is the center of the tile to step to, and `Steps(point)` how far it is.
- **One `Route` serves every mover that wants the same target.** Search again when the target changes tile, not once a mover.
- **A step goes to any of 8 neighbors,** a diagonal one only if both tiles beside it are open, so a mover does not cut a wall's corner.
- **A search of a large map takes milliseconds.** Do not search every frame. A route is a snapshot of the map.
- Drawing a map, moving a box, a ray and a search allocate nothing.

## Not included

Tiles of other sizes than one cell, tile animation, loading maps from files, parallax and camera shake. `examples/world` is a level a screen and a half either way, with coins to collect.

// Package tilemap is a grid of tiles for a game to be played on: it draws the part that is in
// view, and moves boxes through it, stopping at the solid tiles.
//
// A [Map] is a grid with one tile in each cell, and a [Tile] is what a cell looks like (an
// [engine.Cell]), whether it is solid, and what it is, its Kind. Write a level as text art
// with [FromLines], and draw the part of it that is in view with [Map.Draw]. A view is a [geometry.Rect] in
// the map's cells, so the camera is a rectangle that moves over the map, and sprites are drawn
// through the same view by sprite.Layer.DrawView, so that they stay on their tiles:
//
//	view := screen.CenterOn(player.Bounds().Center()).Within(level.Bounds())
//	level.Draw(fb, view)
//	sprites.DrawView(fb, view)
//
// A tile whose cell is a hole, blank with no background and no attributes, is not drawn
// and shows what is under it, as a see-through sprite does, so one map can be laid over
// another. Any other tile replaces the cell under it. Outside the map, there is nothing to
// draw and everything is solid: the edge of the world is a wall.
//
// # What a tile is
//
// [FromLines] gives each tile of its legend the rune that made it as its [Tile].Kind, so a game
// asks the map which tile a cell is, and a wall and a crate that look and act alike are told
// apart, where it would otherwise keep the text of the level beside the map. A Kind the legend
// has is kept, and a rune the legend does not list is the empty tile, with no Kind: list a
// marker such as 'P' in the legend, as 'P': {}, to find it. Tiles that differ only in Kind are
// different tiles to the map.
//
//	if level.At(x, y).Kind == 'X' { // a crate, though it looks and is solid as a wall does
//
// # Moving through the map
//
// [Map.Move] moves a box, a [geometry.Rect], and stops it at solid tiles. It moves along X
// first and then along Y, so a box that goes into a wall at an angle slides along it. It looks
// at every tile on the way, so a long move cannot carry the box through a thin wall, and it
// stops with the box's edge exactly on the tile's edge, so there is no gap to round away and
// the box rests against the wall the same at any frame rate. Unlike pushing a box out of
// another with [geometry.Rect.PushOutOf], it needs neither a rounding fix nor a cap on the
// time step.
//
//	moved, hitX, hitY := level.Move(player.Bounds(), geometry.Vector2{X: dx, Y: dy})
//	player.Position = moved.Min
//
// # Walking to a target
//
// An enemy that walks straight at the player is stopped by the first wall between them, and
// stays there. A [Route] is the way round: [Route.To] searches the tiles that are not solid, breadth
// first, for how many steps each is from a target or several, and [Route.Next] says, from any
// point, the center of the neighbor tile to step to be one step nearer, to steer to and
// [Map.Move]. One Route serves every mover that wants the same target, so search again when the
// target changes tile, not once a mover; it reuses its memory, so that costs nothing to allocate,
// and the work is a few steps for each tile of the map at the most.
//
//	var route tilemap.Route
//	route.To(level, player.Bounds().Center())          // when the player's tile changes
//	if next, ok := route.Next(enemy.Bounds().Center()); ok { // else it has arrived, or cannot
//		// move toward next with level.Move
//	}
//
// A step goes to any of the 8 neighbors of a tile, a diagonal one only if both tiles beside it
// are open, so that a mover does not cut through the corner of a wall, and a diagonal counts as
// one step. A Route is a snapshot of the map: search again after a tile changes.
//
// # Seeing through the map
//
// [Map.Sees] says whether a straight line between two points crosses no solid tile, for whether
// an enemy can see the player or a shot is clear, and [Map.Cast] shoots a ray and says which solid
// tile it meets first, where, and how far, for what a shot hits. Both walk the grid one tile at
// a time, exactly, so no wall is thin enough to miss and the work is at most the size of the map,
// where stepping along the line a tenth of a tile at a time can see past the tip of a corner and
// does far more work.
//
//	if level.Sees(enemy.Bounds().Center(), player.Bounds().Center()) { // nothing solid in the way
//	hit, _ := level.Cast(muzzle, aim)                // hit.X, hit.Y: the tile; hit.Point; hit.Distance
//
// A point is in the tile it floors to, as for [Map.Solid], [Map.Draw] and sprites, so a point on
// the line between tiles is in the tile to its right or below it, and a point at whole numbers,
// the top left of its tile, is in that tile alone. A line through a corner is stopped only by the
// tile that holds the corner, so it passes between two solid tiles that touch only there when
// that tile is not solid. Outside the map is solid, so a ray from inside it always hits
// something, at the edge of the map at the latest. The walk is exact for numbers that a computer
// holds exactly, such as whole numbers, halves and sixty-fourths, which is what makes a corner a
// corner.
//
// # Size
//
// A map holds up to 4,194,304 cells, and a bigger one is empty. It stores two bytes for each
// cell, a number that names one of the different tiles in the map, and holds up to 65,535 of
// those. [Map.Set] adds a tile that is new to the map.
package tilemap

// Package tilemap is a grid of tiles for a game to be played on: it draws the part that is in
// view, and moves boxes through it, stopping at the solid tiles.
//
// A [Map] is a grid with one tile in each cell, and a [Tile] is what a cell looks like (an
// [engine.Cell]) and whether it is solid. Write a level as text art with [FromLines], and
// draw the part of it that is in view with [Map.Draw]. A view is a [geometry.Rect] in
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
// # Size
//
// A map holds up to 4,194,304 cells, and a bigger one is empty. It stores two bytes for each
// cell, a number that names one of the different tiles in the map, and holds up to 65,535 of
// those. [Map.Set] adds a tile that is new to the map.
package tilemap

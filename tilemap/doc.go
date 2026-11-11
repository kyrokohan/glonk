// Package tilemap draws and searches a grid of tiles, the ground a game is played on.
//
// A [Map] is a grid with one tile in each cell, and a [Tile] is what a cell looks like (an
// [engine.Cell]) and whether it is solid. Write a level as text art with [FromLines], and
// draw the part of it that is in view with [Map.Draw]. A view is a [geometry.Rect] in
// the map's cells, so the camera is a rectangle that moves over the map, as in
// [geometry.Rect.CenterOn] and [geometry.Rect.Within].
//
// A tile whose cell is a hole, blank with no background and no attributes, is not drawn
// and shows what is under it, as a see-through sprite does, so one map can be laid over
// another. Any other tile replaces the cell under it. Outside the map, there is nothing to
// draw and everything is solid: the edge of the world is a wall.
package tilemap

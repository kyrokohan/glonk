package tilemap

import (
	"math"
	"unicode/utf8"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// maxCells is the most cells a Map holds, four million: 2,048 by 2,048, far more than any
// terminal shows. A larger map is empty, so that a size taken from outside cannot ask for
// gigabytes.
const maxCells = 1 << 22

// maxTiles is how many different tiles one Map can hold.
const maxTiles = 1 << 16

// Tile is what a cell of a map is: how it looks, and whether it stops a mover.
type Tile struct {
	// Cell is what is drawn for the tile. A hole, a blank with no background and no
	// attributes, draws nothing and shows what is under it; the zero Cell is one.
	Cell engine.Cell

	// Solid says that the tile stops a mover, as a wall does.
	Solid bool
}

// Map is a grid of tiles. The grid holds a small number for each cell, which names one of
// the different tiles in it, so a large map of a few kinds of tile costs two bytes a cell.
// Every method is safe on a nil *Map, which is a map of no cells.
type Map struct {
	width, height int
	tiles         []Tile          // the different tiles; the first is the one that cells start as
	ids           []uint16        // row by row, the index in tiles of each cell
	index         map[Tile]uint16 // the place of each tile in tiles
}

// fits reports whether a map of w by h cells is allowed, without overflowing.
func fits(w, h int) bool {
	return w > 0 && h > 0 && h <= maxCells/w
}

// New returns a map of w by h cells, every one of them fill. A map with no columns or no
// rows is empty, 0 by 0, and so is one with a negative size or more than 4,194,304 cells.
func New(w, h int, fill Tile) *Map {
	m := &Map{index: map[Tile]uint16{fill: 0}, tiles: []Tile{fill}}
	if fits(w, h) {
		m.width, m.height = w, h
		m.ids = make([]uint16, w*h)
	}
	return m
}

// FromLines makes a map from text art, a level written out: one cell for each rune, each
// line a row, a short line padded to the longest with the empty tile. legend says what tile
// each rune is. A rune that is not in it, a space included, is the empty tile, the zero
// Tile, which is a hole that is not solid. Text with no lines or no runes, or more than
// 4,194,304 cells, makes an empty map, and so do more than 65,535 different tiles in
// the text, whose later ones come out as the empty tile.
//
//	level := tilemap.FromLines(map[rune]tilemap.Tile{
//		'#': {Cell: engine.Cell{Character: '#'}, Solid: true},
//		'.': {Cell: engine.Cell{Character: '.'}},
//	},
//		"#####",
//		"#...#",
//		"#####",
//	)
func FromLines(legend map[rune]Tile, lines ...string) *Map {
	width := 0
	for _, line := range lines {
		width = max(width, utf8.RuneCountInString(line))
	}

	m := New(0, 0, Tile{})
	if !fits(width, len(lines)) {
		return m
	}
	m.width, m.height = width, len(lines)
	m.ids = make([]uint16, width*len(lines))
	for y, line := range lines {
		x := 0
		for _, r := range line {
			if t, ok := legend[r]; ok {
				m.ids[y*width+x] = m.idOf(t)
			}
			x++
		}
	}
	return m
}

// idOf returns the place of t in the map's tiles, adding it if it is new. When the map
// holds as many different tiles as it can, a new one is the first tile.
func (m *Map) idOf(t Tile) uint16 {
	if id, ok := m.index[t]; ok {
		return id
	}
	if len(m.tiles) >= maxTiles {
		return 0
	}
	id := uint16(len(m.tiles))
	m.tiles = append(m.tiles, t)
	m.index[t] = id
	return id
}

// Width returns the number of columns.
func (m *Map) Width() int {
	if m == nil {
		return 0
	}
	return m.width
}

// Height returns the number of rows.
func (m *Map) Height() int {
	if m == nil {
		return 0
	}
	return m.height
}

// Bounds returns the rectangle the map covers, from (0, 0): the world in cells.
func (m *Map) Bounds() geometry.Rect {
	return geometry.NewRect(0, 0, float64(m.Width()), float64(m.Height()))
}

// inside reports whether (x, y) is a cell of the map.
func (m *Map) inside(x, y int) bool {
	return m != nil && x >= 0 && x < m.width && y >= 0 && y < m.height
}

// At returns the tile in the cell (x, y), or the zero Tile outside the map.
func (m *Map) At(x, y int) Tile {
	if !m.inside(x, y) {
		return Tile{}
	}
	return m.tiles[m.ids[y*m.width+x]]
}

// Set puts t in the cell (x, y). A cell outside the map is left alone. A tile that is new
// to the map is added to its tiles, up to 65,535 different ones; past that, Set does
// nothing.
func (m *Map) Set(x, y int, t Tile) {
	if !m.inside(x, y) {
		return
	}
	id, known := m.index[t]
	if !known {
		if id = m.idOf(t); id == 0 { // the map has no room for another kind of tile
			return
		}
	}
	m.ids[y*m.width+x] = id
}

// Solid reports whether the cell (x, y) is solid. Everywhere outside the map is: the edge of
// the world is a wall.
func (m *Map) Solid(x, y int) bool {
	if !m.inside(x, y) {
		return true
	}
	return m.tiles[m.ids[y*m.width+x]].Solid
}

// isHole reports whether a cell shows nothing by itself: a blank character, no background
// and no attributes, the rule that makes a part of a see-through sprite a hole.
func isHole(c engine.Cell) bool {
	return (c.Character == ' ' || c.Character == 0) && c.Background.IsDefault() && c.Attrs == 0
}

// Draw draws the part of the map that is in view into fb. view is in the map's cells, and only
// its top left corner is used, since the frame is the size of the view: the cell of the map
// that view.Min is floored to is drawn at the top left of fb. A tile that is a hole is not
// drawn, and neither is anything outside the map. A view whose corner is NaN or infinite
// draws nothing.
//
// The corner is floored in the map, as it is for sprites by sprite.Layer.DrawView, so a
// sprite on a tile stays on it as the view moves.
func (m *Map) Draw(fb *engine.FrameBuffer, view geometry.Rect) {
	if m == nil || len(m.ids) == 0 {
		return
	}
	ox, oy := math.Floor(view.Min.X), math.Floor(view.Min.Y)

	// The cells of the map that are on screen, worked out in floats and checked before
	// any is turned into an int, so that NaN and infinity, and numbers far out of range,
	// leave nothing to draw instead of depending on the CPU.
	x0, x1 := max(ox, 0), min(ox+float64(fb.Width()), float64(m.width))
	y0, y1 := max(oy, 0), min(oy+float64(fb.Height()), float64(m.height))
	if !(x0 < x1 && y0 < y1) {
		return
	}
	cols, rows := int(x1)-int(x0), int(y1)-int(y0)
	shiftX, shiftY := int(ox), int(oy) // both small: the view overlaps the map and the screen

	for row := range rows {
		y := int(y0) + row
		ids := m.ids[y*m.width+int(x0) : y*m.width+int(x0)+cols]
		for col, id := range ids {
			if cell := m.tiles[id].Cell; !isHole(cell) {
				fb.Set(int(x0)+col-shiftX, y-shiftY, cell)
			}
		}
	}
}

// Move moves box by delta, stopping at solid tiles, and returns where it ended up and whether
// it was stopped along X and along Y. It moves along X first, as far as it can, and then
// along Y from there, so a box that goes into a wall at an angle slides along it instead of
// sticking. It looks at every tile along the way, so a long move cannot carry the box through
// a thin wall, and it stops with its edge exactly on the edge of the tile, a whole number,
// so there is no gap to round away and the box rests against the wall the same at any frame
// rate. It stops at the edge of the map, which is solid.
//
//	moved, hitX, hitY := level.Move(player.Bounds(), geometry.Vector2{X: dx, Y: dy})
//	player.Position = moved.Min
//
// Only tiles the box is moving into stop it. One that it already overlaps does not, so a
// box that starts inside a solid tile can move on, and one that starts outside the map is
// outside a wall on every side and cannot move. An empty box or one with a NaN or infinite
// number, and a delta with one, is not moved.
func (m *Map) Move(box geometry.Rect, delta geometry.Vector2) (moved geometry.Rect, hitX, hitY bool) {
	if box.Empty() || !finite(box.Min.X, box.Min.Y, box.Max.X, box.Max.Y, delta.X, delta.Y) {
		return box, false, false
	}

	// Along X, the tiles in the rows the box covers, then along Y, the tiles in the columns
	// it then covers: the same walk with the axes swapped.
	moved.Min.X, moved.Max.X, hitX = m.slide(box.Min.X, box.Max.X, delta.X, box.Min.Y, box.Max.Y, false)
	moved.Min.Y, moved.Max.Y, hitY = m.slide(box.Min.Y, box.Max.Y, delta.Y, moved.Min.X, moved.Max.X, true)
	return moved, hitX, hitY
}

// finite reports whether none of vs is NaN or infinite.
func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// slide moves the span from lo to hi by d along one axis, through the tiles in the span from
// crossLo to crossHi on the other axis, and stops it at the first solid tile. With down set,
// the axes are swapped: the span is along Y and the cross span along X. It returns the span
// where it ends and whether it was stopped. A span that goes nowhere, with d of 0, has no
// cells ahead and is not stopped.
func (m *Map) slide(lo, hi, d, crossLo, crossHi float64, down bool) (newLo, newHi float64, stopped bool) {
	// The tiles across the path: the cross span's first and last cell. Outside the map is
	// solid, so a span that reaches past it has solid tiles at every step.
	first, last := math.Floor(crossLo), math.Ceil(crossHi)-1
	crossOutside := first < 0 || last >= m.size(!down)
	// The sides of the map along the path, in cells.
	end := m.size(down)

	// blocked reports whether the cell c along the path is solid somewhere across it.
	blocked := func(c float64) bool {
		if c < 0 || c >= end || crossOutside {
			return true
		}
		for x := int(first); x <= int(last); x++ {
			if down && m.Solid(x, int(c)) || !down && m.Solid(int(c), x) {
				return true
			}
		}
		return false
	}

	if d > 0 {
		// The cells whose near edge the span can reach: from the first at or past its far
		// edge, to the last that starts before where it would end.
		for c, stop := math.Ceil(hi), math.Ceil(hi+d)-1; c <= stop; c++ {
			if blocked(c) {
				return lo + (c - hi), c, true // its far edge on the cell's near edge
			}
		}
		return lo + d, hi + d, false
	}
	for c, stop := math.Floor(lo)-1, math.Floor(lo+d); c >= stop; c-- {
		if blocked(c) {
			return c + 1, hi + (c + 1 - lo), true // its near edge on the cell's far edge
		}
	}
	return lo + d, hi + d, false
}

// size returns the number of columns, or of rows if rows is set, as a float.
func (m *Map) size(rows bool) float64 {
	if rows {
		return float64(m.Height())
	}
	return float64(m.Width())
}

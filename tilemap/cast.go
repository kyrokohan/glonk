package tilemap

import (
	"math"

	"glonk.dev/geometry"
)

// farCell is how far from the origin the number of a tile outside the map is held, so that a
// point of any size has a tile number that fits an int, even where an int has 32 bits. No map
// is anywhere near this big.
const farCell = 1 << 30

// Hit says where a ray first met a solid tile.
type Hit struct {
	// Point is where the ray enters the tile, on its edge: the coordinate of the edge it crosses
	// is exact. For a ray that starts in a solid tile it is the start of the ray.
	Point geometry.Vector2

	// X and Y are the tile, which can be outside the map, since outside the map is solid. For
	// a point far outside, they are held to a billion.
	X, Y int

	// Distance is how far the ray went from its start to Point, in cells, in a straight line.
	Distance float64
}

// cellOf returns the cell that the coordinate v is in, its floor, held to farCell.
func cellOf(v float64) int {
	return int(min(max(math.Floor(v), -farCell), farCell))
}

// rayHit is where trace found a solid tile: the tile, how far along the ray, in lengths of
// its direction, and the point.
type rayHit struct {
	x, y  int
	t     float64
	point geometry.Vector2
}

// trace walks the ray from o along d through the tiles it passes, one at a time, and returns
// the first solid one that a point of the ray with a parameter t of at most limit is in, where
// the point is o + t*d, and whether there is one. A point is in the tile it floors to, so one
// on the line between tiles is in the tile to its right or below it, as for Solid. The tile is
// entered at its first point, which for a tile on the right or below is on the edge it is
// entered by, and for one on the left or above is just past the edge, so its point is that
// edge. Where the ray goes exactly through the corner of four tiles, it is in the one that holds
// the corner at that instant, and then in the one after it, and passes the others.
//
// The next crossing of a tile edge is worked out from the start by a division each time, not
// added to the last, so that no error builds up and two crossings that are equal are equal,
// which is what makes a ray through a corner of numbers a computer holds exactly go through it.
// Outside the map is solid, so a ray that starts inside it ends in at most width plus height
// steps, and one that starts outside is in a solid tile at once.
func (m *Map) trace(o, d geometry.Vector2, limit float64) (rayHit, bool) {
	cx, cy := cellOf(o.X), cellOf(o.Y)
	if m.Solid(cx, cy) {
		return rayHit{x: cx, y: cy, point: o}, true
	}
	sx, sy := sign(d.X), sign(d.Y)

	for {
		// Where the ray crosses the next vertical and the next horizontal edge: the right edge
		// of the tile when it goes right, the left when it goes left.
		tx, ty := math.Inf(1), math.Inf(1)
		var ex, ey float64
		if sx != 0 {
			ex = float64(cx + max(sx, 0))
			tx = (ex - o.X) / d.X
		}
		if sy != 0 {
			ey = float64(cy + max(sy, 0))
			ty = (ey - o.Y) / d.Y
		}

		t := min(tx, ty)
		if t > limit { // also when the ray goes nowhere, so that t is infinite
			return rayHit{}, false
		}

		switch {
		case tx < ty:
			// A tile on the right holds the point of the edge, so it is entered at t, and one on
			// the left only after it.
			if t == limit && sx < 0 {
				return rayHit{}, false
			}
			cx += sx
			if m.Solid(cx, cy) {
				return rayHit{cx, cy, t, geometry.Vector2{X: ex, Y: o.Y + t*d.Y}}, true
			}
		case ty < tx:
			if t == limit && sy < 0 {
				return rayHit{}, false
			}
			cy += sy
			if m.Solid(cx, cy) {
				return rayHit{cx, cy, t, geometry.Vector2{X: o.X + t*d.X, Y: ey}}, true
			}
		default:
			// Through a corner. The tile that holds the corner is the one to the right of the edge
			// and below the other, which is the next tile if the ray goes right and down, the
			// tile it is in if it goes left and up, and a tile at the side if it goes the other
			// two ways, which holds the corner and nothing else of the ray.
			corner := geometry.Vector2{X: ex, Y: ey}
			if sx*sy < 0 {
				if m.Solid(int(ex), int(ey)) {
					return rayHit{int(ex), int(ey), t, corner}, true
				}
			}
			if t == limit && (sx < 0 || sy < 0) {
				return rayHit{}, false
			}
			cx, cy = cx+sx, cy+sy
			if m.Solid(cx, cy) {
				return rayHit{cx, cy, t, corner}, true
			}
		}
	}
}

// sign returns -1, 0 or 1 for the sign of v.
func sign(v float64) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// Sees reports whether no solid tile holds a point of the straight line from a to b, the ends
// included: whether something at a could see something at b, or a shot between them is clear.
// The points are in the map's cells, as for Move. A point is in the tile it floors to, as for
// Solid, Draw and the sprites of a layer, so a point on the line between tiles is in the tile to
// its right or below it. A line through a corner is stopped only by the tile that holds the
// corner, so it passes between two solid tiles that touch only there when that tile is not
// solid, and a point at whole numbers, the top left of its tile, is in that tile alone. The
// line is walked tile by tile, so no wall is thin enough to miss, and the work is at most the
// size of the map. Outside the map is solid, so a point outside it, or a line that leaves it,
// is not seen, and neither is a point that is NaN or infinite.
//
// The answer does not depend on which end is which. It is exact for numbers that a computer
// holds exactly, such as whole numbers, halves and sixty-fourths, which is what makes a corner a
// corner; for other numbers a line within a rounding error of one may pass either side of it.
func (m *Map) Sees(a, b geometry.Vector2) bool {
	d := geometry.Vector2{X: b.X - a.X, Y: b.Y - a.Y}
	// Far enough apart to overflow, one end is far outside the map.
	if !finite(a.X, a.Y, b.X, b.Y, d.X, d.Y) {
		return false
	}
	_, blocked := m.trace(a, d, 1)
	return !blocked
}

// Cast shoots a ray from the point from in the direction, which can be any length but not 0,
// and returns the first solid tile it meets: where it enters it, which tile, and how far it
// went. It is for what a shot hits, how far a mover can see, and where something standing
// at a point would be pushed to. The point and the distance are in the map's cells, as for Move,
// and the distance is a straight line.
//
// Outside the map is solid, so a ray from inside the map always hits something, at the edge of
// the map at the latest, as a tile outside it, and a ray that starts in a solid tile, or outside
// the map, hits at once with a distance of 0 and the point it started at. The false result is for a
// point or direction that is NaN or infinite, and for a direction of 0. Everything else about the
// walk, the point being in the tile it floors to, a corner, the exactness and the work being at
// most the size of the map, is as for Sees.
func (m *Map) Cast(from, direction geometry.Vector2) (Hit, bool) {
	if !finite(from.X, from.Y, direction.X, direction.Y) {
		return Hit{}, false
	}
	// The direction is used as it is, so that a ray along (3, 1) through a corner is exact, but
	// scaled by a power of two, which changes nothing, to between a half and 1, so that a very
	// big or very small one cannot overflow or underflow the walk.
	scale := max(math.Abs(direction.X), math.Abs(direction.Y))
	if scale == 0 {
		return Hit{}, false
	}
	_, exp := math.Frexp(scale)
	d := geometry.Vector2{X: math.Ldexp(direction.X, -exp), Y: math.Ldexp(direction.Y, -exp)}

	h, ok := m.trace(from, d, math.Inf(1))
	if !ok {
		return Hit{}, false
	}
	// t is in lengths of d, so the distance is t of them.
	return Hit{Point: h.point, X: h.x, Y: h.y, Distance: h.t * math.Hypot(d.X, d.Y)}, true
}

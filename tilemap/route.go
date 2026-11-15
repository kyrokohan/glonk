package tilemap

import "glonk.dev/geometry"

// unreached is the number of steps of a tile that no target can be walked to, or that is solid.
const unreached = -1

// neighbors lists the neighbors of a tile: the four straight ones first, so that where several are
// one step nearer a straight step is chosen before a diagonal one, and the diagonals after.
var neighbors = [8]struct{ dx, dy int }{
	{1, 0}, {0, 1}, {-1, 0}, {0, -1},
	{1, 1}, {-1, 1}, {-1, -1}, {1, -1},
}

// Route is the way to a target, for anything on a Map that wants to walk there. [Route.To]
// finds, by a search of the tiles that are not solid, how many steps every one of them is from
// the nearest of some targets, and [Route.Next] says, from any point, which neighbor tile to
// step to be one step nearer. One Route serves every mover that wants the same target, so it is
// searched again when the target changes tile, not once a mover.
//
// A step goes to any of the 8 neighbors of a tile, and a diagonal step only if both tiles beside
// it are open, so a mover does not cut through the corner of a wall. A diagonal counts as one
// step, as a straight one does.
//
// The zero Route has found nothing. A Route holds the numbers of one search and no pointer to the
// map, so it does not see tiles that are changed after, and it keeps its memory to search again
// without allocating.
type Route struct {
	w, h  int
	steps []int32 // row by row: the steps of each tile to the nearest target, or unreached
	queue []int32 // the tiles the search has reached, as indexes into steps, in order of steps
}

// To searches the map for the way to the targets, which are points in the map's cells, and
// replaces what the Route knew. Every tile that is not solid is given the number of steps to the
// nearest target, breadth first, so that the work is a few steps for each tile of the map at the
// most, however many targets there are. A point is in the tile it floors to, as everywhere in
// the map, and a target that is in a solid tile, outside the map, or NaN or infinite is left out;
// with no target left, nothing can be reached. A nil map has no tiles.
//
// It reuses the memory of the last search, so it allocates nothing once it has searched a map
// of that size or bigger.
func (r *Route) To(m *Map, targets ...geometry.Vector2) {
	w, h := m.Width(), m.Height()
	n := w * h
	r.w, r.h = w, h
	if cap(r.steps) < n {
		r.steps = make([]int32, n)
	}
	r.steps = r.steps[:n]
	for i := range r.steps {
		r.steps[i] = unreached
	}
	if cap(r.queue) < n {
		r.queue = make([]int32, 0, n)
	}
	r.queue = r.queue[:0]

	for _, t := range targets {
		if !finite(t.X, t.Y) {
			continue
		}
		x, y := cellOf(t.X), cellOf(t.Y)
		if m.Solid(x, y) { // outside the map is solid
			continue
		}
		if i := y*w + x; r.steps[i] != 0 { // the same tile twice is one target
			r.steps[i] = 0
			r.queue = append(r.queue, int32(i))
		}
	}

	// Each tile is queued once, so the queue never needs more than n places and never grows.
	for head := 0; head < len(r.queue); head++ {
		i := int(r.queue[head])
		x, y := i%w, i/w
		next := r.steps[i] + 1
		for _, d := range neighbors {
			nx, ny := x+d.dx, y+d.dy
			if m.Solid(nx, ny) {
				continue
			}
			j := ny*w + nx
			if r.steps[j] != unreached {
				continue
			}
			if d.dx != 0 && d.dy != 0 && (m.Solid(nx, y) || m.Solid(x, ny)) { // the corner of a wall
				continue
			}
			r.steps[j] = next
			r.queue = append(r.queue, int32(j))
		}
	}
}

// tile returns the tile the point p is in, and whether it is inside the map that was searched.
func (r *Route) tile(p geometry.Vector2) (x, y int, ok bool) {
	if !finite(p.X, p.Y) {
		return 0, 0, false
	}
	x, y = cellOf(p.X), cellOf(p.Y)
	return x, y, x >= 0 && x < r.w && y >= 0 && y < r.h
}

// Steps returns how many steps the tile that the point p is in is from the nearest target, 0
// for a tile that has one in it. The second result is false for a point outside the map, in a
// solid tile, in a tile that no target can be walked to, with no target at all, or before any
// search, and for a point that is NaN or infinite.
func (r *Route) Steps(p geometry.Vector2) (int, bool) {
	x, y, ok := r.tile(p)
	if !ok {
		return 0, false
	}
	s := r.steps[y*r.w+x]
	return int(s), s != unreached
}

// Next returns the center of the neighbor tile that a mover at the point from should step to,
// the one step nearer to the nearest target, for it to steer to, with Map.Move, for instance.
// Ask again every frame, since a mover is somewhere in its tile and not at the center, and the
// answer changes when it crosses into the next. Where several neighbors are one step nearer, a
// straight one is chosen before a diagonal one, and the choice is the same every time.
//
// The second result is false where there is no step to take: the mover is in a tile with a target
// in it, so it has arrived, or Steps is false for the point.
func (r *Route) Next(from geometry.Vector2) (geometry.Vector2, bool) {
	x, y, ok := r.tile(from)
	if !ok {
		return geometry.Vector2{}, false
	}
	s := r.steps[y*r.w+x]
	if s <= 0 { // unreached, or arrived
		return geometry.Vector2{}, false
	}
	for _, d := range neighbors {
		nx, ny := x+d.dx, y+d.dy
		if nx < 0 || nx >= r.w || ny < 0 || ny >= r.h || r.steps[ny*r.w+nx] != s-1 {
			continue
		}
		// A diagonal step is not through the corner of a wall: both tiles beside it are open. An
		// open tile beside a tile that can reach a target can too, so open is a step of 0 or more.
		if d.dx != 0 && d.dy != 0 && (r.steps[y*r.w+nx] == unreached || r.steps[ny*r.w+x] == unreached) {
			continue
		}
		return geometry.Vector2{X: float64(nx) + 0.5, Y: float64(ny) + 0.5}, true
	}
	return geometry.Vector2{}, false // not reached by a route from To, which gives every tile a step nearer
}

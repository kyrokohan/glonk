package geometry

import "math"

// Rect is an axis-aligned rectangle: the points from Min up to, but not including, Max.
// It is half-open, like the cells a sprite covers: one that starts at X=2 and is 3 wide
// covers 2 up to 5, and one that starts at X=5 does not touch it. A Rect whose Max is not
// beyond its Min on both axes, or that has a NaN in it, has no points and is empty; the
// zero Rect is empty.
type Rect struct {
	// Min is the corner with the smallest X and Y, which is in the rectangle.
	Min Vector2

	// Max is the corner with the largest X and Y, which is just outside it.
	Max Vector2
}

// NewRect returns the rectangle with its top left corner at (x, y), w wide and h tall.
// A negative or NaN size makes an empty rectangle.
func NewRect(x, y, w, h float64) Rect {
	return Rect{Min: Vector2{X: x, Y: y}, Max: Vector2{X: x + w, Y: y + h}}
}

// Width returns Max.X - Min.X, which is negative for a rectangle that is turned inside out.
func (r Rect) Width() float64 {
	return r.Max.X - r.Min.X
}

// Height returns Max.Y - Min.Y, which is negative for a rectangle that is turned inside out.
func (r Rect) Height() float64 {
	return r.Max.Y - r.Min.Y
}

// Size returns the width and the height as a vector.
func (r Rect) Size() Vector2 {
	return Vector2{X: r.Width(), Y: r.Height()}
}

// Empty reports whether r has no points: it has no width or no height, is turned inside
// out, or has a NaN in it.
func (r Rect) Empty() bool {
	// Written as the opposite of what a rectangle with area satisfies, so that NaN is empty.
	return !(r.Min.X < r.Max.X && r.Min.Y < r.Max.Y)
}

// Contains reports whether p is in r: on or past Min, and before Max. A point with a NaN
// in it is in no rectangle.
func (r Rect) Contains(p Vector2) bool {
	return p.X >= r.Min.X && p.X < r.Max.X && p.Y >= r.Min.Y && p.Y < r.Max.Y
}

// Overlaps reports whether r and o have any points in common, which is whether they
// share some area. Rectangles that only touch along an edge or at a corner do not, and an
// empty rectangle overlaps nothing. It does not matter which is the receiver.
func (r Rect) Overlaps(o Rect) bool {
	if r.Empty() || o.Empty() {
		return false
	}
	return r.Min.X < o.Max.X && o.Min.X < r.Max.X && r.Min.Y < o.Max.Y && o.Min.Y < r.Max.Y
}

// Intersect returns the points r and o have in common, which is a rectangle inside both,
// or the zero Rect if they have none.
func (r Rect) Intersect(o Rect) Rect {
	if !r.Overlaps(o) {
		return Rect{}
	}
	return Rect{
		Min: Vector2{X: max(r.Min.X, o.Min.X), Y: max(r.Min.Y, o.Min.Y)},
		Max: Vector2{X: min(r.Max.X, o.Max.X), Y: min(r.Max.Y, o.Max.Y)},
	}
}

// Translate returns r moved by v.
func (r Rect) Translate(v Vector2) Rect {
	return Rect{Min: r.Min.Add(v), Max: r.Max.Add(v)}
}

// PushOutOf returns the smallest move that takes r out of o, along one axis: the one on
// which r overlaps o the least, and the way that is shorter. When both ways, or both axes,
// are equally short, it moves toward larger X, then toward larger Y, and prefers X to Y.
// It is the zero vector if r and o do not overlap (touching is not overlapping) or if no
// move that is a finite number would do, as when o is infinite on both axes. Apply it to
// r's position to stand r against o, as against a wall or a paddle:
//
//	player.Position = player.Position.Add(player.Bounds().PushOutOf(wall))
//
// The two rectangles touch afterwards to within the rounding of the sums, so Overlaps may
// still report a hair of overlap for numbers that are not exact, and does not for whole
// numbers and halves. A mover that goes farther than its own size in one frame can skip
// over a thin wall before this is asked; it does not look at where r has been.
func (r Rect) PushOutOf(o Rect) Vector2 {
	if !r.Overlaps(o) {
		return Vector2{}
	}

	// How far r must go to clear o on each axis: back past o's near edge, or on past its
	// far edge. The first is negative and the second positive, as r overlaps o.
	left, right := o.Min.X-r.Max.X, o.Max.X-r.Min.X
	up, down := o.Min.Y-r.Max.Y, o.Max.Y-r.Min.Y
	dx, dy := right, down
	if -left < right {
		dx = left
	}
	if -up < down {
		dy = up
	}

	move := Vector2{X: dx}
	if math.Abs(dy) < math.Abs(dx) {
		move = Vector2{Y: dy}
	}
	// Y is taken only when it is shorter than X, so a move that is infinite is along X, and
	// then both axes are. There is no NaN: the rectangles overlap, so no edge is
	// subtracted from one of the same infinity.
	if math.IsInf(move.X, 0) {
		return Vector2{}
	}
	return move
}

// Center returns the middle of r's corners. For a rectangle with no points that is still the
// middle of its corners.
func (r Rect) Center() Vector2 {
	// Halved before adding, so that corners near the largest float do not overflow.
	return Vector2{X: r.Min.X/2 + r.Max.X/2, Y: r.Min.Y/2 + r.Max.Y/2}
}

// CenterOn returns r moved so that its center is p, with the same size. A rectangle with a
// NaN in it, or a p that is NaN or infinite, is returned as it is. Together with Within it
// makes a camera: the rectangle of the screen, centered on a player and kept inside the level.
//
//	view := screen.CenterOn(player.Bounds().Center()).Within(level.Bounds())
func (r Rect) CenterOn(p Vector2) Rect {
	if !finite2(p) || math.IsNaN(r.Min.X+r.Min.Y+r.Max.X+r.Max.Y) {
		return r
	}
	halfW, halfH := r.Width()/2, r.Height()/2
	return Rect{
		Min: Vector2{X: p.X - halfW, Y: p.Y - halfH},
		Max: Vector2{X: p.X + halfW, Y: p.Y + halfH},
	}
}

// Within returns r moved, not resized, to lie inside bounds, by the shortest move on each
// axis. On an axis where r is larger than bounds it cannot, and is centered on bounds
// instead. An empty r or empty bounds, or one with a NaN in it, gives r as it is.
func (r Rect) Within(bounds Rect) Rect {
	if r.Empty() || bounds.Empty() {
		return r
	}
	r.Min.X, r.Max.X = fitWithin(r.Min.X, r.Max.X, bounds.Min.X, bounds.Max.X)
	r.Min.Y, r.Max.Y = fitWithin(r.Min.Y, r.Max.Y, bounds.Min.Y, bounds.Max.Y)
	return r
}

// fitWithin moves the span from lo to hi inside the span from blo to bhi, as Rect.Within
// says. A span that is moved to an edge is put exactly on it.
func fitWithin(lo, hi, blo, bhi float64) (float64, float64) {
	width := hi - lo
	switch {
	case width > bhi-blo: // too big to fit: line up the middles
		mid := blo/2 + bhi/2
		return mid - width/2, mid + width/2
	case lo < blo:
		return blo, blo + width
	case hi > bhi:
		return bhi - width, bhi
	}
	return lo, hi
}

// finite2 reports whether neither coordinate of p is NaN or infinite.
func finite2(p Vector2) bool {
	return !math.IsNaN(p.X+p.Y) && !math.IsInf(p.X, 0) && !math.IsInf(p.Y, 0)
}

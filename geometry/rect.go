package geometry

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

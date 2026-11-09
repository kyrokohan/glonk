package engine

import (
	"math"
	"sort"
)

func intAbs(a int) int {
	if a >= 0 {
		return a
	}
	return -a
}

// lineLimit bounds the coordinates walkLine works with. Its arithmetic multiplies
// two distances and doubles the result, which comes to as much as 2^61; see
// minorOffset for why that matters on systems where int is 32 bits.
const lineLimit = 1 << 29

// DrawLine draws a line from (x0, y0) to (x1, y1) using Bresenham's algorithm.
//
// Only the part inside the buffer is visited, so a line with a far-away end costs
// no more than one that stays on screen, and the cells drawn are exactly the ones
// the whole line would draw there. Coordinates beyond +-2^29 are clamped to it.
func (fb *FrameBuffer) DrawLine(x0, y0, x1, y1 int, cell Cell) {
	walkLineAt(fb.width, fb.height, x0, y0, x1, y1, func(x, y, _, _ int) {
		fb.Set(x, y, cell)
	})
}

// DrawLineDepth draws the same line as DrawLine, but each cell is stored with SetDepth,
// so a nearer surface hides it and it hides a farther one. z0 and z1 are the distances
// from the camera of the two ends, as for FillTriangle, and must be positive. A line
// is drawn where it is nearer than what is there, and where it ties, what is there
// stays.
//
// The depth of a cell is perspective-correct: 1/z, not z, changes at an even pace along
// the line, so a line that runs away from the camera has its cells' depth bunched up
// toward the near end. It is worked out along the cells of the whole line, so the part
// inside the buffer has the depth it would have if the line were all drawn, and the
// end cells have exactly z0 and z1. A line that is a single point takes the nearer end.
//
// A depth that is not positive, or is NaN, at either end draws nothing. An infinite
// depth is infinitely far: nothing is nearer than it, so those cells are not drawn.
func (fb *FrameBuffer) DrawLineDepth(x0, y0 int, z0 float64, x1, y1 int, z1 float64, cell Cell) {
	d, ok := newLineDepth(z0, z1)
	if !ok {
		return
	}
	walkLineAt(fb.width, fb.height, x0, y0, x1, y1, func(x, y, i, n int) {
		fb.SetDepth(x, y, d.at(i, n), cell)
	})
}

// lineDepth is the distance from the camera along a line whose ends are z0 and z1 away.
// DrawLineDepth of a FrameBuffer and of a Canvas both take it from here, so that they
// cannot disagree; see FrameBuffer.DrawLineDepth for what it is.
type lineDepth struct {
	z0, z1     float64
	inv0, inv1 float64 // 1/z0 and 1/z1
	near, far  float64 // the smaller and the larger of the ends
}

// newLineDepth returns the depth along a line from z0 to z1 away, and false if either
// is not positive, or is NaN, which draws nothing.
func newLineDepth(z0, z1 float64) (lineDepth, bool) {
	if !(z0 > 0 && z1 > 0) {
		return lineDepth{}, false
	}
	return lineDepth{z0: z0, z1: z1, inv0: 1 / z0, inv1: 1 / z1, near: min(z0, z1), far: max(z0, z1)}, true
}

// at is the depth of step i of a line n steps long, as walkLineAt counts them.
func (d *lineDepth) at(i, n int) float64 {
	switch {
	case n == 0:
		return d.near
	case i == 0:
		return d.z0
	case i == n:
		return d.z1
	}
	t := float64(i) / float64(n)
	z := 1 / ((1-t)*d.inv0 + t*d.inv1)
	// Depths at the limits of a float can overflow or underflow the sum; the
	// answer is always between the ends.
	if !(z >= d.near) {
		return d.near
	}
	return min(z, d.far)
}

// minorOffset is how far the shorter axis of a Bresenham line has moved after i
// steps along the longer one, which is n steps long and covers db on the shorter:
// round(i*db/n), with halves rounding away from the start.
//
// The product i*db can pass 2^31 even though both factors fit in 31 bits, so it is
// worked out in 64 bits; in an int it would overflow where int is 32 bits wide.
func minorOffset(i, db, n int) int {
	if n == 0 {
		return 0
	}
	return int((2*int64(i)*int64(db) + int64(n)) / (2 * int64(n)))
}

// walkLine calls plot for every cell of the Bresenham line from (x0, y0) to
// (x1, y1) that lies inside a w x h grid, in order from the first end to the last.
//
// It never steps outside the grid. Instead of walking the whole line it works out
// which steps land inside, which is possible because the minor coordinate of a
// Bresenham line has a closed form: after i steps along the major axis it has moved
// round(i * minorDelta / majorDelta), halves rounding away from the start.
func walkLine(w, h, x0, y0, x1, y1 int, plot func(x, y int)) {
	walkLineAt(w, h, x0, y0, x1, y1, func(x, y, _, _ int) { plot(x, y) })
}

// walkLineAt is walkLine that also says where on the line each cell is: it was step i
// of n along the longer axis, counting from the first end at step 0 to the last end at
// step n, where n is the length of the whole line and not only of the part inside the
// grid. A line that is one point is n = 0.
func walkLineAt(w, h, x0, y0, x1, y1 int, plot func(x, y, i, n int)) {
	x0, y0 = min(max(x0, -lineLimit), lineLimit), min(max(y0, -lineLimit), lineLimit)
	x1, y1 = min(max(x1, -lineLimit), lineLimit), min(max(y1, -lineLimit), lineLimit)

	// Walk along the longer axis (a) and derive the shorter one (b). A line exactly
	// diagonal walks along x, as the step-by-step algorithm does.
	steep := intAbs(y1-y0) > intAbs(x1-x0)
	a0, a1, b0, b1, aSize, bSize := x0, x1, y0, y1, w, h
	if steep {
		a0, a1, b0, b1, aSize, bSize = y0, y1, x0, x1, h, w
	}

	n := intAbs(a1 - a0)  // steps along a
	db := intAbs(b1 - b0) // distance along b, never more than n
	sa, sb := 1, 1
	if a1 < a0 {
		sa = -1
	}
	if b1 < b0 {
		sb = -1
	}

	// off(i) is how far b has moved after i steps along a.
	off := func(i int) int { return minorOffset(i, db, n) }

	// Steps whose a lies inside the grid.
	lo, hi := 0, n
	if sa > 0 {
		lo, hi = max(lo, -a0), min(hi, aSize-1-a0)
	} else {
		lo, hi = max(lo, a0-(aSize-1)), min(hi, a0)
	}
	if lo > hi {
		return
	}

	// off only ever grows, so the steps whose b lies inside the grid are one
	// unbroken run, found by searching for where it enters and leaves the range.
	offLo, offHi := -b0, bSize-1-b0
	if sb < 0 {
		offLo, offHi = b0-(bSize-1), b0
	}
	lo = max(lo, sort.Search(n+1, func(i int) bool { return off(i) >= offLo }))
	hi = min(hi, sort.Search(n+1, func(i int) bool { return off(i) > offHi })-1)

	for i := lo; i <= hi; i++ {
		a, b := a0+sa*i, b0+sb*off(i)
		if steep {
			plot(b, a, i, n)
		} else {
			plot(a, b, i, n)
		}
	}
}

// DrawText draws text one cell per character, starting at (x, y). Off-screen text
// is clipped.
//
// Only characters one column wide are drawn as they are. A control character
// (tab, newline, escape) or a wide one (CJK, emoji) is drawn as '?', and NUL as a
// space. Combining marks and joiners have no column of their own and are skipped,
// so "e" followed by an accent draws a plain "e".
func (fb *FrameBuffer) DrawText(x, y int, text string, color Color) {
	col := x

	for _, r := range text {
		if col >= fb.width {
			break
		}
		if runeWidth(r) == 0 {
			continue
		}
		fb.Set(col, y, Cell{Character: safeRune(r), Foreground: color})
		col++
	}
}

// DrawBox draws a w x h border at (x, y). A box smaller than 2x2 draws nothing.
func (fb *FrameBuffer) DrawBox(x, y, w, h int, color Color) {
	if w < 2 || h < 2 {
		return
	}
	x1 := x + w - 1
	y1 := y + h - 1

	fb.DrawLine(x, y, x1, y, Cell{Character: '─', Foreground: color})
	fb.DrawLine(x, y1, x1, y1, Cell{Character: '─', Foreground: color})
	fb.DrawLine(x, y, x, y1, Cell{Character: '│', Foreground: color})
	fb.DrawLine(x1, y, x1, y1, Cell{Character: '│', Foreground: color})
	fb.Set(x, y, Cell{Character: '┌', Foreground: color})
	fb.Set(x1, y, Cell{Character: '┐', Foreground: color})
	fb.Set(x, y1, Cell{Character: '└', Foreground: color})
	fb.Set(x1, y1, Cell{Character: '┘', Foreground: color})
}

// triangleEps lets a cell centre that sits exactly on an edge count as inside
// despite rounding, so triangles that share an edge leave no gaps between them.
const triangleEps = 1e-9

// FillTriangle fills the triangle with corners (x0, y0), (x1, y1), (x2, y2) in
// cell coordinates, drawing cell into every cell whose centre lies inside it.
// z0, z1 and z2 are the corners' distances from the camera and must be positive;
// each cell is drawn with SetDepth, so nearer surfaces win. Either winding works.
// A triangle with no area, or with a non-positive or NaN corner depth, draws nothing.
func (fb *FrameBuffer) FillTriangle(x0, y0, z0, x1, y1, z1, x2, y2, z2 float64, cell Cell) {
	scanTriangle(fb.width, fb.height, x0, y0, z0, x1, y1, z1, x2, y2, z2, func(x, y int, z, _, _, _ float64) {
		fb.SetDepth(x, y, z, cell)
	})
}

// FillTriangleFunc fills the triangle like FillTriangle, but asks cell what to draw
// in each covered cell instead of drawing one cell everywhere. cell is given the
// weights b0, b1 and b2 of the three corners at the middle of that cell: they sum to
// 1, and b0*a0 + b1*a1 + b2*a2 is the value at the cell of anything that has the
// value a0, a1, a2 at the corners, taking the perspective of z0, z1 and z2 into
// account. A cell centre exactly on an edge can get a weight a hair below zero.
//
// The depth test comes first: cell is called only for a cell that is nearer than what
// is there, and so is never asked about one that would be hidden. The cell it returns
// is drawn, with the depth, as SetDepth would. A nil cell draws nothing.
func (fb *FrameBuffer) FillTriangleFunc(x0, y0, z0, x1, y1, z1, x2, y2, z2 float64, cell func(b0, b1, b2 float64) Cell) {
	if cell == nil {
		return
	}
	scanTriangle(fb.width, fb.height, x0, y0, z0, x1, y1, z1, x2, y2, z2, func(x, y int, z, p0, p1, p2 float64) {
		// scanTriangle only visits cells inside the buffer.
		i := y*fb.width + x
		if !(z < fb.depth[i]) {
			return
		}
		c := cell(p0*z, p1*z, p2*z)
		fb.depth[i] = z
		fb.frame[i] = c
	})
}

// scanTriangle walks the w x h grid and calls plot for every cell whose centre
// lies inside the triangle, with the interpolated depth z there. It does the
// geometry for every kind of target; the caller decides what a cell holds.
//
// p0, p1 and p2 are the corners' screen-space weights divided by their depths. Times
// z they are the perspective-correct weights of the corners: they sum to 1 and
// attributes blend with them as they do in the scene. They are left unmultiplied so
// that a fill that does not want them pays nothing for them.
func scanTriangle(w, h int, x0, y0, z0, x1, y1, z1, x2, y2, z2 float64, plot func(x, y int, z, p0, p1, p2 float64)) {
	if !(z0 > 0 && z1 > 0 && z2 > 0) {
		return
	}

	// Twice the signed area. The sign depends on winding; dividing by it below
	// makes the weights positive inside the triangle either way.
	area := (x1-x0)*(y2-y0) - (x2-x0)*(y1-y0)
	if !(math.Abs(area) > 0) {
		return
	}

	// Clamp as floats before converting: corners can be astronomically far off
	// screen, and a float that big has no valid int.
	fw, fh := float64(w), float64(h)
	left := int(max(0, min(math.Floor(min(x0, x1, x2)), fw)))
	right := int(max(-1, min(math.Ceil(max(x0, x1, x2)), fw-1)))
	top := int(max(0, min(math.Floor(min(y0, y1, y2)), fh)))
	bottom := int(max(-1, min(math.Ceil(max(y0, y1, y2)), fh-1)))

	for y := top; y <= bottom; y++ {
		for x := left; x <= right; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5

			w0 := ((x1-px)*(y2-py) - (x2-px)*(y1-py)) / area
			w1 := ((x2-px)*(y0-py) - (x0-px)*(y2-py)) / area
			w2 := ((x0-px)*(y1-py) - (x1-px)*(y0-py)) / area
			if w0 < -triangleEps || w1 < -triangleEps || w2 < -triangleEps {
				continue
			}

			// 1/z is the quantity that varies linearly across the screen, so
			// interpolate it and invert; interpolating z itself would skew depth.
			p0, p1, p2 := w0/z0, w1/z1, w2/z2
			plot(x, y, 1/(p0+p1+p2), p0, p1, p2)
		}
	}
}

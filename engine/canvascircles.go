package engine

import "math"

// maxRadius is the largest radius a circle is drawn with, and a larger one is taken as it.
// Its square fits in 62 bits, and with it every sum a circle makes fits in an int of 32.
const maxRadius = 1 << 29

// isqrt returns the largest s with s*s <= n, for 0 <= n < 2^60.
func isqrt(n int64) int64 {
	// A float64 has 53 bits, so the root can be a little off for a large n, and is put right.
	s := int64(math.Sqrt(float64(n)))
	for s*s > n {
		s--
	}
	for (s+1)*(s+1) <= n {
		s++
	}
	return s
}

// halfWidth returns how far left and right of the middle of a circle, whose radius squared is
// r2, its pixels reach in a row d pixels above or below it: the most dx with dx*dx + d*d <=
// r2, or -1 for a row beyond the circle, which has none.
func halfWidth(d, r2 int64) int64 {
	if d*d > r2 {
		return -1
	}
	return isqrt(r2 - d*d)
}

// FillCircle fills the disc of radius r around pixel (cx, cy): the pixels (x, y) with
// (x-cx)² + (y-cy)² <= r². It is 2r+1 pixels across and round, since the pixels of a Canvas
// are square. A negative radius draws nothing, a radius of 0 is the one pixel (cx, cy), and
// a radius over 2^29 is taken as 2^29. The rows are worked out with integers, exactly, so
// the same pixels are drawn on every platform. Only the part inside the canvas is visited,
// so a circle with a far-away edge costs no more than one that stays on screen, and no
// position or radius can overflow.
//
// Depth, the zero Color and what is replaced are as for FillRect.
func (c *Canvas) FillCircle(cx, cy, r int, color Color) {
	c.circle(cx, cy, r, false, color)
}

// DrawCircle draws the outline of the disc FillCircle fills: those of its pixels that have
// a neighbor above, below, left or right that is not in it. That is a ring one pixel
// thick, whose pixels touch along a side or at a corner, and which keeps inside the disc.
// A radius of 0 is the one pixel. Everything else is as for FillCircle.
func (c *Canvas) DrawCircle(cx, cy, r int, color Color) {
	c.circle(cx, cy, r, true, color)
}

// circle draws the disc, or only its outline.
func (c *Canvas) circle(cx, cy, r int, outline bool, color Color) {
	if r < 0 {
		return
	}
	r = min(r, maxRadius)
	// Missing the canvas altogether. Asked before anything is added, so that nothing can
	// overflow: after it the center is within r of the canvas, and every sum below is small.
	if cx < -r || cx > c.width-1+r || cy < -r || cy > c.height-1+r {
		return
	}
	r2 := int64(r) * int64(r)

	for py := max(cy-r, 0); py <= min(cy+r, c.height-1); py++ {
		d := int64(py - cy)
		if d < 0 {
			d = -d
		}
		half := int(halfWidth(d, r2)) // at least 0: the row is within r of the center

		if !outline {
			x0, x1 := span(cx-half, 2*half+1, c.width)
			c.fillRow(py, x0, x1, color)
			continue
		}

		// A pixel of the row is on the outline if its neighbor beside it is outside, which only
		// the last pixel of the row has, or if its neighbor in the next row, away from the
		// center, is. That row reaches inner pixels from the middle, so this row's pixels past
		// inner, out to its end, are on the outline, and the last one always is. From 0 is the
		// whole row.
		inner := int(halfWidth(d+1, r2))
		from := min(inner+1, half)
		if from == 0 {
			x0, x1 := span(cx-half, 2*half+1, c.width)
			c.fillRow(py, x0, x1, color)
			continue
		}
		n := half - from + 1
		if x0, x1 := span(cx-half, n, c.width); x0 < x1 {
			c.fillRow(py, x0, x1, color)
		}
		if x0, x1 := span(cx+from, n, c.width); x0 < x1 {
			c.fillRow(py, x0, x1, color)
		}
	}
}

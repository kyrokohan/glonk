package engine

import "math"

// span returns the part of the n values from start that is inside [0, limit), as the first
// value and one past the last. It is empty, lo == hi, when there is none, and it cannot
// overflow, whatever start and n are: a sum that would pass the largest int is held at it.
func span(start, n, limit int) (lo, hi int) {
	if n <= 0 || start >= limit {
		return 0, 0
	}
	end := math.MaxInt
	if start <= math.MaxInt-n {
		end = start + n
	}
	lo, hi = max(start, 0), min(end, limit)
	if lo >= hi {
		return 0, 0
	}
	return lo, hi
}

// reach returns start+n for n >= 0, held at the largest int instead of wrapping around to a
// negative one, so that the far edge of a shape is never before its near edge.
func reach(start, n int) int {
	if start > math.MaxInt-n {
		return math.MaxInt
	}
	return start + n
}

// FillRect fills the rectangle w pixels wide and h tall whose top left pixel is (x, y): the
// pixels from x up to but not including x+w, and the same down. A width or height that is
// not positive draws nothing. Only the part inside the canvas is visited, so a rectangle
// with a far-away edge costs no more than one that stays on screen, and no position or
// size can overflow.
//
// Like Set it never reads or writes depth: the pixels are replaced whatever is there, and
// a solid or a line drawn afterwards with depth is hidden only by the depth that was there
// before. The zero Color erases the pixels it covers.
func (c *Canvas) FillRect(x, y, w, h int, color Color) {
	x0, x1 := span(x, w, c.width)
	y0, y1 := span(y, h, c.height)
	for py := y0; py < y1; py++ {
		c.fillRow(py, x0, x1, color)
	}
}

// fillRow sets the pixels of row y from x0 up to but not including x1, which must be inside
// the canvas.
func (c *Canvas) fillRow(y, x0, x1 int, color Color) {
	row := c.pixels[y*c.width+x0 : y*c.width+x1]
	for i := range row {
		row[i] = color
	}
}

// DrawRect draws the border of the rectangle FillRect would fill, one pixel thick and
// inside it, in the same pixels: a rectangle under 3 pixels wide or tall has no inside, so
// it is all border and DrawRect fills it. A width or height that is not positive draws
// nothing. Depth, the zero Color, clipping and the size of the numbers are as for FillRect.
func (c *Canvas) DrawRect(x, y, w, h int, color Color) {
	if w < 3 || h < 3 {
		c.FillRect(x, y, w, h, color)
		return
	}
	c.FillRect(x, y, w, 1, color)
	c.FillRect(x, reach(y, h-1), w, 1, color)
	c.FillRect(x, reach(y, 1), 1, h-2, color)
	c.FillRect(reach(x, w-1), reach(y, 1), 1, h-2, color)
}

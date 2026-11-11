package widget

import "glonk.dev/engine"

// Panel is a box with a border, a title on its top edge and a filled inside.
type Panel struct {
	// Title is written on the top edge between two blanks and cut to fit. A panel with no
	// room for it, fewer than 7 cells wide, or a title with nothing to show, has a plain edge.
	Title string

	// Style is the colors and attributes of the border and the title.
	Style engine.Cell

	// Fill is what the inside is filled with, character included. A blank with no
	// background and no attributes, which the zero Cell is, leaves the inside alone.
	Fill engine.Cell
}

// Draw draws the panel w cells wide and h tall with its top left corner at (x, y), cutting
// off what is off the frame. A panel smaller than 2 by 2 draws nothing, as DrawBox does.
func (p Panel) Draw(fb *engine.FrameBuffer, x, y, w, h int) {
	if w < 2 || h < 2 {
		return
	}
	right, bottom := reach(x, w-1), reach(y, h-1)
	if x >= fb.Width() || y >= fb.Height() || right < 0 || bottom < 0 {
		return // none of it is on the frame
	}

	// The columns of the horizontal edges, and the columns and rows of the inside, that
	// are on the frame. After the checks above none of these sums can overflow.
	x0, x1 := max(x, 0), min(right, fb.Width()-1)
	ix0, ix1 := max(x+1, 0), min(right-1, fb.Width()-1)
	iy0, iy1 := max(y+1, 0), min(bottom-1, fb.Height()-1)

	if !isHole(p.Fill) {
		for cy := iy0; cy <= iy1; cy++ {
			for cx := ix0; cx <= ix1; cx++ {
				fb.Set(cx, cy, p.Fill)
			}
		}
	}

	for cx := x0; cx <= x1; cx++ {
		fb.Set(cx, y, styled(p.Style, edge(cx, x, right, '┌', '─', '┐')))
		fb.Set(cx, bottom, styled(p.Style, edge(cx, x, right, '└', '─', '┘')))
	}
	for cy := iy0; cy <= iy1; cy++ {
		fb.Set(x, cy, styled(p.Style, '│'))
		fb.Set(right, cy, styled(p.Style, '│'))
	}

	// Corner, a dash, a blank, the title, a blank, a dash and the corner: the title has
	// w-6 cells at most.
	if w >= 7 {
		title := clip(p.Title, w-6)
		if n := engine.TextWidth(title); n > 0 {
			fb.Set(reach(x, 2), y, styled(p.Style, ' '))
			Label{Text: title, Style: p.Style}.Draw(fb, reach(x, 3), y, n)
			fb.Set(reach(x, 3+n), y, styled(p.Style, ' '))
		}
	}
}

// edge returns the character of column c on a horizontal edge that runs from lo to hi: the
// first at lo, the last at hi, and mid between.
func edge(c, lo, hi int, first, mid, last rune) rune {
	switch c {
	case lo:
		return first
	case hi:
		return last
	}
	return mid
}

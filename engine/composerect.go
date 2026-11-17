package engine

import "math"

// ComposeRect is Compose for one rectangle of the canvas, to put pixels down over cells that are
// already in the frame: a picture inside a filled panel, an icon beside a label, the pixel text of
// DrawText in a menu. Compose puts the whole canvas down, and what is drawn into cells after it covers
// the pixels; ComposeRect, called after the cells, puts a part down over them.
//
// The rectangle is in pixels, w wide and h tall with its top left pixel at (x, y), the numbers the
// picture was drawn with. Every cell it touches is put down, and a cell is two pixels tall, so a
// rectangle that starts on an odd row, or ends on an even one, puts down the cell's other pixel too,
// as the canvas holds it. A width or height that is not positive does nothing, and only the part on
// the canvas is visited, so no position or size can overflow or make it slow.
//
// A cell whose two pixels are both empty is not touched, so a letter that is there stays. Any other
// cell is replaced as Compose replaces it, except that an empty pixel shows what the cell showed in
// that half, so a see-through part of a picture shows the panel it is on: a space or a letter shows
// its Background in both halves, a half block (upper or lower) keeps its other half, and a full
// block its Foreground, with Reverse swapping the two colors first. A cell that holds a letter loses it as
// soon as one of its pixels is put down, since a half block cannot hold one, and keeps its
// Background. Text attributes are dropped. A half that the cell showed in the terminal's default
// foreground, a block character with no Foreground, comes out in the default background, since a
// background cannot be the default foreground.
//
// With Seamless set a cell with a pixel is one solid color, as for Compose, the nearer pixel's or the
// only one, so what was under it is not kept. A cell takes the depth of its nearer pixel, as for
// Compose: infinite for a pixel drawn with Set after a Clear, so depth-tested drawing afterwards lands
// on top. The canvas itself is not changed, so Compose, called later, puts the whole of it down again.
//
// The canvas must hold only what is to be put down. Where it also holds a scene that Compose has
// already put down, Clear it first and draw the picture on the empty canvas:
//
//	canvas.Compose()                 // the world, drawn as pixels
//	widget.Panel{Fill: fill}.Draw(fb, 0, 0, 12, 5)
//	canvas.Clear()                   // the world is in the cells now
//	coin.Draw(canvas, 2, 2)          // a bitmap.Bitmap, drawn in pixels
//	canvas.ComposeRect(2, 2, coin.Width(), coin.Height())
func (c *Canvas) ComposeRect(x, y, w, h int) {
	x0, x1 := span(x, w, c.width)
	y0, y1 := span(y, h, c.height)
	if x0 == x1 || y0 == y1 {
		return
	}

	fb := c.fb
	for row := y0 / 2; row <= (y1-1)/2; row++ {
		for cx := x0; cx < x1; cx++ {
			topIdx, bottomIdx := (2*row)*c.width+cx, (2*row+1)*c.width+cx
			top, bottom := c.pixels[topIdx], c.pixels[bottomIdx]
			if top.IsDefault() && bottom.IsDefault() {
				continue
			}
			cellIdx := row*fb.width + cx

			// The depth of the cell is its nearer pixel's, as Compose gives it.
			z := math.Inf(1)
			if !top.IsDefault() {
				z = c.depth[topIdx]
			}
			if !bottom.IsDefault() {
				z = min(z, c.depth[bottomIdx])
			}
			fb.depth[cellIdx] = z

			if c.Seamless {
				color := top
				if top.IsDefault() || (!bottom.IsDefault() && c.depth[bottomIdx] < c.depth[topIdx]) {
					color = bottom
				}
				fb.frame[cellIdx] = Cell{Character: ' ', Background: color}
				continue
			}

			// An empty pixel is what the cell showed in its place.
			shownTop, shownBottom := cellHalves(fb.frame[cellIdx])
			if top.IsDefault() {
				top = shownTop
			}
			if bottom.IsDefault() {
				bottom = shownBottom
			}
			fb.frame[cellIdx] = halfBlockCell(top, bottom)
		}
	}
}

// cellHalves returns the colors that cell shows in its top and bottom halves, read as
// FrameBuffer.Image reads a cell: a half block is its two colors, a full block is its Foreground, and
// anything else, a space or a letter, is its Background. Reverse swaps the two colors first. A color that
// is the default is the zero Color, which is how ComposeRect tells that nothing is there.
func cellHalves(cell Cell) (top, bottom Color) {
	fg, bg := cell.Foreground, cell.Background
	if cell.Attrs.Has(Reverse) {
		fg, bg = bg, fg
	}
	switch cell.Character {
	case upperHalfBlock:
		return fg, bg
	case lowerHalfBlock:
		return bg, fg
	case 0x2588: // a full block
		return fg, fg
	}
	return bg, bg
}

// halfBlockCell returns the cell that shows top over bottom, built as Compose builds one: a space on a
// background where the two are the same, and a half block in the one that is set where the other is the
// default. At least one of the two is set.
func halfBlockCell(top, bottom Color) Cell {
	switch {
	case top == bottom:
		// A space on a background is solid however the font draws blocks.
		return Cell{Character: ' ', Background: top}
	case !top.IsDefault() && !bottom.IsDefault():
		return Cell{Character: upperHalfBlock, Foreground: top, Background: bottom}
	case !top.IsDefault():
		return Cell{Character: upperHalfBlock, Foreground: top}
	}
	return Cell{Character: lowerHalfBlock, Foreground: bottom}
}

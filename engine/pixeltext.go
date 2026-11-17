package engine

import "math"

// The most a letter is scaled by: a letter is then bigger than any canvas, so a larger scale shows no
// more, and no size that a position is added to can overflow.
const maxTextScale = 4096

// clampTextScale holds a scale to 1 up to maxTextScale.
func clampTextScale(scale int) int {
	return min(max(scale, 1), maxTextScale)
}

// glyphOf is the glyph that a character is drawn with: its own if it is printable ASCII, a space for NUL,
// as FrameBuffer.DrawText draws it, and a question mark for every other, as it draws what it has no cell for.
func glyphOf(r rune) *[fontHeight]uint8 {
	switch {
	case r >= ' ' && r <= '~':
		return &font[r-' ']
	case r == 0:
		return &font[0]
	}
	return &font['?'-' ']
}

// DrawText draws text in the pixel font that comes with glonk, with the top left of the first letter at
// pixel (x, y). A letter is 5 pixels wide and 7 tall, and the next starts 6 pixels on, so a column is
// left blank between them. Only the pixels of the letters are written, in color, so what is on the canvas
// shows between the strokes, and the zero Color erases them, as for FillRect. Like Set it never reads or
// writes depth.
//
// The font has the printable ASCII characters, from the space to the tilde. A NUL is a space, and a
// character with no width of its own, a combining mark or a joiner, is skipped, as FrameBuffer.DrawText
// skips it. Every other character, an accented letter, a Chinese character, an emoji, a tab or a newline,
// is drawn as a question mark: it has no letter, and the engine draws a question mark for what it cannot
// show. Lines are not wrapped, and a newline does not start one.
//
// Only the part inside the canvas is visited, so a text that starts far off it, or is long, costs no more
// than the part that shows, and no position can overflow. PixelTextWidth says how wide it is.
func (c *Canvas) DrawText(x, y int, text string, color Color) {
	c.DrawTextScaled(x, y, text, color, 1)
}

// DrawTextScaled is DrawText with each pixel of a letter drawn as a block of scale by scale pixels, so that
// a scale of 2 or 3 is a letter 10 by 14 or 15 by 21, with the next 12 or 18 pixels on. A scale below 1 is
// 1, and one above 4096, where a letter is bigger than any canvas, is 4096.
func (c *Canvas) DrawTextScaled(x, y int, text string, color Color, scale int) {
	scale = clampTextScale(scale)
	// Past the bottom or the right edge nothing more is drawn, so what is added to x and y below is a
	// small number to one that is inside the canvas, and cannot overflow.
	if y >= c.height || y+fontHeight*scale <= 0 {
		return // wholly below or above
	}
	for _, r := range text {
		if runeWidth(r) == 0 {
			continue
		}
		if x >= c.width {
			return // this letter and the ones after it start past the right edge
		}
		if x+fontWidth*scale > 0 {
			c.drawGlyph(x, y, glyphOf(r), color, scale)
		}
		x += fontAdvance * scale
	}
}

// drawGlyph draws one letter, each run of pixels along a row as one rectangle scale rows tall. Its top
// left is inside the canvas or above or to the left of it, never past the bottom or the right.
func (c *Canvas) drawGlyph(x, y int, glyph *[fontHeight]uint8, color Color, scale int) {
	for row, bits := range glyph {
		top := y + row*scale
		for col := 0; col < fontWidth; col++ {
			if bits&(1<<(fontWidth-1-col)) == 0 {
				continue
			}
			start := col
			for col+1 < fontWidth && bits&(1<<(fontWidth-2-col)) != 0 {
				col++
			}
			c.FillRect(x+start*scale, top, (col-start+1)*scale, scale, color)
		}
	}
}

// PixelTextWidth returns how many pixels wide DrawTextScaled draws text at scale: from the left edge of the
// first letter's cell to the right edge of the last, with no gap after it, so that a title centers:
//
//	c.DrawTextScaled(x+(w-engine.PixelTextWidth(title, 2))/2, y, title, color, 2)
//
// It counts what DrawText draws, which is one letter for each character except the ones with no width of
// their own, and 0 for a text with none. The scale is held as DrawTextScaled holds it. A width that is more
// than the largest int is the largest int. The height is always 7 times the scale.
func PixelTextWidth(text string, scale int) int {
	n := 0
	for _, r := range text {
		if runeWidth(r) != 0 {
			n++
		}
	}
	if n == 0 {
		return 0
	}
	width := (int64(n)*fontAdvance - 1) * int64(clampTextScale(scale))
	if width > math.MaxInt {
		return math.MaxInt
	}
	return int(width)
}

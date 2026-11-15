package engine

// BlendRect blends color over the rectangle w pixels wide and h tall whose top left pixel is
// (x, y), the pixels FillRect would fill, by the part t from 0 to 1: each pixel that is there
// becomes Mix(pixel, color, t), so 0 leaves its color and 1 makes it color. A dim behind a
// menu, a fade to black, a red flash and a see-through panel are all this, and the whole canvas
// is the rectangle (0, 0, Width, Height). As for Mix, t outside 0 to 1 is clamped, NaN is 0,
// a palette color is read as xterm defines it, and the result is an exact RGB color, so a
// palette pixel becomes the RGB color it stands for even at t of 0. The zero Color as color is
// read as black, as Mix reads it, and does not erase.
//
// A pixel that is empty stays empty. The terminal's own background is not known, so there is
// nothing to mix with; a program that wants the tint there fills the pixels first. A width or
// height that is not positive draws nothing. Only the part inside the canvas is visited, so
// a rectangle with a far-away edge costs no more than one that stays on screen, and no
// position or size can overflow.
//
// Like Set it never reads or writes depth.
func (c *Canvas) BlendRect(x, y, w, h int, color Color, t float64) {
	x0, x1 := span(x, w, c.width)
	y0, y1 := span(y, h, c.height)
	t = clamp01(t)
	cr, cg, cb := color.RGB()
	for py := y0; py < y1; py++ {
		row := c.pixels[py*c.width+x0 : py*c.width+x1]
		for i, p := range row {
			if p.IsDefault() {
				continue
			}
			r, g, b := p.RGB()
			row[i] = RGB(mixChannel(r, cr, t), mixChannel(g, cg, t), mixChannel(b, cb, t))
		}
	}
}

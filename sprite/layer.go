package sprite

import (
	"math"
	"slices"

	"glonk.dev/engine"
)

// Layer is an ordered set of sprites drawn together; later sprites draw over earlier ones.
type Layer struct {
	sprites []*Sprite
}

// NewLayer returns an empty Layer.
func NewLayer() *Layer {
	return &Layer{}
}

// Add appends obj to the layer. A nil sprite is ignored.
func (l *Layer) Add(obj *Sprite) {
	if obj == nil {
		return
	}
	l.sprites = append(l.sprites, obj)
}

// Remove takes obj out of the layer, every time it was added, and reports whether it
// was there. The sprites left keep their order, so they still draw over one another
// as before. A nil sprite removes nothing.
func (l *Layer) Remove(obj *Sprite) bool {
	before := len(l.sprites) // a nil sprite is never in the list, so it finds nothing
	l.sprites = slices.DeleteFunc(l.sprites, func(sp *Sprite) bool { return sp == obj })
	return len(l.sprites) < before
}

// Clear takes every sprite out of the layer.
func (l *Layer) Clear() {
	clear(l.sprites) // so the old slots do not keep the sprites alive
	l.sprites = l.sprites[:0]
}

// Len returns how many sprites are in the layer. A sprite added twice counts twice.
func (l *Layer) Len() int {
	return len(l.sprites)
}

// Draw sets each sprite's Cell, or the cells of its Frame, in fb, clipping anything off
// screen. A sprite replaces the whole cell under it, or is see-through if it has Overlay
// set. A sprite whose Position has a NaN or infinite X or Y is not drawn, like one that is
// off screen.
func (l *Layer) Draw(fb *engine.FrameBuffer) {
	w, h := float64(fb.Width()), float64(fb.Height())

	for _, sp := range l.sprites {
		x, y := math.Floor(sp.Position.X), math.Floor(sp.Position.Y)

		// Everything but a plain single cell is drawn out of the loop, which keeps the
		// common case as short as it was.
		if sp.Frame != nil || sp.Overlay {
			drawSprite(fb, sp, x, y, w, h)
			continue
		}

		// Checked before converting to int, and written so that NaN fails: what
		// int(NaN) or int(1e300) gives differs from one CPU to the next.
		if x >= 0 && x < w && y >= 0 && y < h {
			fb.Set(int(x), int(y), sp.Cell)
		}
	}
}

// drawSprite draws a sprite that has a Frame or is an Overlay, with its position
// already floored, on a buffer w by h in float64.
func drawSprite(fb *engine.FrameBuffer, sp *Sprite, x, y, w, h float64) {
	if sp.Frame != nil {
		drawFrame(fb, sp.Frame, x, y, w, h, sp.Overlay)
		return
	}
	if x >= 0 && x < w && y >= 0 && y < h { // as for a plain cell
		putOverlay(fb, int(x), int(y), sp.Cell)
	}
}

// putOverlay draws one cell of a sprite that is see-through, as Sprite.Overlay says, at
// (x, y), which is on the buffer.
func putOverlay(fb *engine.FrameBuffer, x, y int, cell engine.Cell) {
	if isHole(cell) {
		return
	}
	if cell.Background.IsDefault() {
		cell.Background = fb.At(x, y).Background
	}
	fb.Set(x, y, cell)
}

// isHole reports whether a cell shows nothing by itself: a blank character, no
// background and no attributes. The foreground of a blank is not seen, so it does not count.
func isHole(c engine.Cell) bool {
	return (c.Character == ' ' || c.Character == 0) && c.Background.IsDefault() && c.Attrs == 0
}

// drawFrame sets the cells of f in fb with its top left cell at (x, y), which are
// already floored, and clips what is off the frame. The frame is w by h, in float64.
func drawFrame(fb *engine.FrameBuffer, f *Frame, x, y, w, h float64, overlay bool) {
	if f.width == 0 || f.height == 0 {
		return
	}

	// Checked before converting to int, as for a single cell; the frame reaches onto the
	// buffer when its left edge is less than w and its right edge is more than 0.
	if !(x > -float64(f.width) && x < w && y > -float64(f.height) && y < h) {
		return
	}

	ix, iy := int(x), int(y)
	cols := max(-ix, 0)
	colEnd := min(f.width, fb.Width()-ix)
	for row := max(-iy, 0); row < min(f.height, fb.Height()-iy); row++ {
		for col := cols; col < colEnd; col++ {
			if overlay {
				putOverlay(fb, ix+col, iy+row, f.cells[row*f.width+col])
			} else {
				fb.Set(ix+col, iy+row, f.cells[row*f.width+col])
			}
		}
	}
}

package bitmap

import "glonk.dev/engine"

// halvedCopies are a view's copies, each half the size of the one before, and the count of the
// source's changes they were made at.
type halvedCopies struct {
	changes uint64
	levels  []*Bitmap // levels[0] is halved once, the last is 1 by 1
}

// Halved returns the picture halved n times, for something that is drawn smaller than it is: a
// mesh far away reads its picture from the copy that fits, so it does not crawl as it moves. Each
// copy is (w+1)/2 by (h+1)/2 pixels of the one before, down to 1 by 1; Halved(0), and below, is
// the picture itself, and an n past the smallest copy gives the smallest.
//
// A pixel of a copy stands for up to four of the one before, fewer along an odd edge. It is
// see-through where most of those are: three or four of four, both of two, the one of one; where
// two of four are, it is kept, so that a line one pixel wide is not lost. Otherwise its red, green
// and blue are the means of the ones that are there, rounded to the nearest, with a palette color
// counted as the RGB it stands for, so the same copy is made on every platform.
//
// The copies are made from the colors the picture is drawn in, Shown, so a Shaded or Tinted
// view's copies have its look, and a Sub view's are of its part. They are made the first time
// they are asked for, kept by the view, and made again after Set changes a pixel of the picture
// or of any view of it. A copy is shared by everything that asks for it, and Set does nothing on
// it; a copy has no copies of its own, and Halved of it is itself. A picture's copies take about a
// third more memory than it does. Asking from several goroutines at once is safe, as long as
// nothing is set meanwhile.
func (b *Bitmap) Halved(n int) *Bitmap {
	if b == nil || n <= 0 || b.src == nil || (b.w <= 1 && b.h <= 1) {
		return b
	}
	levels := b.halvedLevels()
	return levels[min(n, len(levels))-1]
}

// halvedLevels returns the view's copies, made again if a pixel has changed since.
func (b *Bitmap) halvedLevels() []*Bitmap {
	changes := b.src.changes
	if c := b.halved.Load(); c != nil && c.changes == changes {
		return c.levels
	}
	// Made aside and stored whole, so that a draw on another goroutine sees either the old copies
	// or these, never half of them. Two that make them at once make the same, and one is kept.
	var levels []*Bitmap
	prev := halve(b.w, b.h, b.Shown)
	levels = append(levels, prev)
	for prev.w > 1 || prev.h > 1 {
		prev = halve(prev.w, prev.h, prev.At)
		levels = append(levels, prev)
	}
	b.halved.Store(&halvedCopies{changes: changes, levels: levels})
	return levels
}

// halve returns the copy of a w by h picture, whose pixels at gives, at half the size: see Halved
// for the rule.
func halve(w, h int, at func(x, y int) engine.Color) *Bitmap {
	nw, nh := (w+1)/2, (h+1)/2
	out := &Bitmap{w: nw, h: nh, stride: nw, pixels: make([]engine.Color, nw*nh)}
	for y := range nh {
		for x := range nw {
			var r, g, bl, there, holes int
			for dy := range 2 {
				for dx := range 2 {
					sx, sy := 2*x+dx, 2*y+dy
					if sx >= w || sy >= h {
						continue
					}
					c := at(sx, sy)
					if c.IsDefault() {
						holes++
						continue
					}
					cr, cg, cb := c.RGB()
					r, g, bl, there = r+int(cr), g+int(cg), bl+int(cb), there+1
				}
			}
			if 2*holes > holes+there {
				continue // most of them are holes
			}
			out.pixels[y*nw+x] = engine.RGB(uint8((r+there/2)/there), uint8((g+there/2)/there), uint8((bl+there/2)/there))
		}
	}
	return out
}

package sprite

import "glonk.dev/geometry"

// Bounds returns the box the sprite covers: from Position, as it is and not floored to a
// cell, as wide and tall as what is drawn. That is the Animation's current frame if it has
// one, else the Frame, else the Cell, which is 1 by 1. The whole frame counts, holes in its
// art included, so a ghost with empty corners still touches at its corners. A sprite with
// nothing to draw, a nil or empty frame or an animation with no frame to show, has an empty
// box at its Position, and so has one whose Position is NaN or infinite: it touches
// nothing. A nil sprite has the zero Rect.
//
// The sprite is drawn in the cell that Position is in, so it can overlap what it is not
// drawn over by up to a cell: one at X=2.9 is drawn in column 2 and covers 2.9 up to its
// width. Use the box for physics and let drawing snap to cells.
func (s *Sprite) Bounds() geometry.Rect {
	if s == nil {
		return geometry.Rect{}
	}
	w, h := 1.0, 1.0 // a Cell
	switch {
	case s.Animation != nil:
		f := s.Animation.Frame() // nil if there is none, which is 0 by 0
		w, h = float64(f.Width()), float64(f.Height())
	case s.Frame != nil:
		w, h = float64(s.Frame.Width()), float64(s.Frame.Height())
	}
	return geometry.NewRect(s.Position.X, s.Position.Y, w, h)
}

// Overlaps reports whether the boxes of s and o share any area, so whether they collide.
// Sprites that only touch along an edge do not. A nil sprite, or one with nothing to
// draw, overlaps nothing.
func (s *Sprite) Overlaps(o *Sprite) bool {
	return s.Bounds().Overlaps(o.Bounds())
}

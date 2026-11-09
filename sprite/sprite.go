package sprite

import (
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// Sprite is a picture at a fractional position, floored to a cell of the frame when
// drawn: one Cell, or a Frame of several.
type Sprite struct {
	// Position is where the sprite is: X is the column and Y the row, in cells from
	// the top left, and for a Frame the place of its top left cell. Fractions are
	// floored when the sprite is drawn.
	Position geometry.Vector2

	// Cell is what is drawn when Frame is nil: a character with its colors and
	// attributes. It replaces the whole cell beneath it, background included, as
	// FrameBuffer.Set does. A Cell with a Background and no Character draws a blank of
	// that color.
	Cell engine.Cell

	// Frame, if it is not nil, is drawn in place of Cell: a grid of cells with its top
	// left cell at Position. The part that is off the frame is clipped, cell by cell. Every
	// cell replaces the cell beneath it, blanks included, as Cell does. A Frame with no
	// cells draws nothing, and does not fall back to Cell.
	Frame *Frame

	// Overlay makes the sprite see-through, for a Cell and for a Frame. Off, a sprite
	// replaces the cells under it whole, blanks included. On, a blank cell of the sprite,
	// a space or NUL with no background and no attributes, is a hole: nothing is drawn
	// there and what is behind it shows. Any other cell is drawn, and one that has no
	// Background of its own keeps the background of the cell under it, so a colored letter
	// sits on a colored floor without a box in the default color around it. A blank with a
	// background or an attribute (Reverse, Underline) shows something, so it is not a hole.
	//
	// On a cell that Canvas.Compose wrote, only the background is kept: the pixel above it
	// is lost, so do not overlay sprites on half-block pixels.
	Overlay bool
}

// New returns a Sprite showing the character r at position, in the terminal's
// default colors. Set Cell to give it colors or attributes.
func New(position geometry.Vector2, r rune) *Sprite {
	return &Sprite{Position: position, Cell: engine.Cell{Character: r}}
}

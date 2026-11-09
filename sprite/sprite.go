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
}

// New returns a Sprite showing the character r at position, in the terminal's
// default colors. Set Cell to give it colors or attributes.
func New(position geometry.Vector2, r rune) *Sprite {
	return &Sprite{Position: position, Cell: engine.Cell{Character: r}}
}

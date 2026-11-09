package sprite

import (
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// Sprite is one Cell at a fractional position, floored to a cell of the frame when
// drawn.
type Sprite struct {
	// Position is where the sprite is: X is the column and Y the row, in cells from
	// the top left. Fractions are floored when the sprite is drawn.
	Position geometry.Vector2

	// Cell is what is drawn: a character with its colors and attributes. It replaces
	// the whole cell beneath it, background included, as FrameBuffer.Set does. A Cell
	// with a Background and no Character draws a blank of that color.
	Cell engine.Cell
}

// New returns a Sprite showing the character r at position, in the terminal's
// default colors. Set Cell to give it colors or attributes.
func New(position geometry.Vector2, r rune) *Sprite {
	return &Sprite{Position: position, Cell: engine.Cell{Character: r}}
}

package sprite

import "glonk.dev/engine"

// maxFrameCells is the most cells a Frame holds: a thousand by a thousand, far more than
// any terminal shows. Asking for more gives an empty frame, so that a size taken from
// outside cannot ask for gigabytes.
const maxFrameCells = 1 << 20

// fits reports whether a frame of w by h cells is allowed, without overflowing.
func fits(w, h int) bool {
	return w >= 0 && h >= 0 && (w == 0 || h <= maxFrameCells/w)
}

// Frame is a small grid of cells: the picture of a sprite that is more than one
// character. Make one from text art with FromLines or cell by cell with NewFrame and
// Set, and show it by setting Sprite.Frame. A Frame is drawn, never changed, by the
// sprites that show it, so one Frame can be shared; Set changes it for all of them
// (Clone makes a copy that is not shared).
//
// Every method is safe on a nil *Frame, which is a frame of no cells.
type Frame struct {
	width, height int
	cells         []engine.Cell // row by row, width*height of them
}

// NewFrame returns a frame of w by h blank cells. A frame with no columns or no rows is
// empty, 0 by 0, and so is one with a negative size or more than 1,048,576 cells.
func NewFrame(w, h int) *Frame {
	if w <= 0 || h <= 0 || !fits(w, h) {
		return &Frame{}
	}
	f := &Frame{width: w, height: h, cells: make([]engine.Cell, w*h)}
	for i := range f.cells {
		f.cells[i] = engine.BlankCell
	}
	return f
}

// FromLines returns a frame of text art: each line is a row, each character a cell. The
// frame is as wide as the longest line, and shorter lines are padded with blank cells on
// the right. A space is a blank cell, which is a hole in a sprite that is see-through (see
// Sprite.Overlay).
//
// The foreground color, background color and attributes of style go on every cell that
// holds a character other than a space, so a background shows behind the letters and not
// behind the holes; the Character of style is not used. For a blank cell with a color, or
// cells of different colors, use NewFrame and Set. Characters that are not shown as they
// are in a cell (tabs, wide characters) are drawn as '?', as in any Cell.
//
// A frame with no text or with more than 1,048,576 cells is not made: the result is
// empty.
func FromLines(style engine.Cell, lines ...string) *Frame {
	rows := make([][]rune, len(lines))
	width := 0
	for i, line := range lines {
		rows[i] = []rune(line)
		width = max(width, len(rows[i]))
	}
	f := NewFrame(width, len(rows))
	if f.width == 0 { // no text, or too much
		return f
	}
	for y, row := range rows {
		for x, r := range row {
			if r == ' ' {
				continue
			}
			f.cells[y*width+x] = engine.Cell{Character: r, Foreground: style.Foreground, Background: style.Background, Attrs: style.Attrs}
		}
	}
	return f
}

// Width returns the number of columns.
func (f *Frame) Width() int {
	if f == nil {
		return 0
	}
	return f.width
}

// Height returns the number of rows.
func (f *Frame) Height() int {
	if f == nil {
		return 0
	}
	return f.height
}

// At returns the cell at column x of row y, or a blank cell outside the frame.
func (f *Frame) At(x, y int) engine.Cell {
	if f == nil || x < 0 || x >= f.width || y < 0 || y >= f.height {
		return engine.BlankCell
	}
	return f.cells[y*f.width+x]
}

// Set stores cell at column x of row y. Writes outside the frame are ignored.
func (f *Frame) Set(x, y int, cell engine.Cell) {
	if f == nil || x < 0 || x >= f.width || y < 0 || y >= f.height {
		return
	}
	f.cells[y*f.width+x] = cell
}

// Clone returns a copy of the frame that shares nothing with it.
func (f *Frame) Clone() *Frame {
	if f == nil {
		return &Frame{}
	}
	return &Frame{width: f.width, height: f.height, cells: append([]engine.Cell(nil), f.cells...)}
}

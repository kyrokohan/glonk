package engine

import (
	"errors"
	"io"
	"math"
	"strings"
)

// FrameBuffer is a width x height grid of Cells that a game draws into each
// frame; Run presents it to the terminal.
//
// It also keeps a depth per cell, used only by SetDepth, so solid 3D surfaces
// can hide what is behind them. Everything else ignores depth.
type FrameBuffer struct {
	width, height int
	frame         []Cell
	depth         []float64
	canvas        *Canvas // created on first use by Canvas
}

func (fb *FrameBuffer) inBounds(x, y int) bool {
	return x >= 0 && x < fb.width && y >= 0 && y < fb.height
}

// NewFrameBuffer returns a blank FrameBuffer of w x h cells. Both must be positive.
func NewFrameBuffer(w, h int) (*FrameBuffer, error) {
	if w <= 0 || h <= 0 {
		return nil, errors.New("width and height must be positive integers")
	}
	allocatedFrame := make([]Cell, w*h)
	fb := &FrameBuffer{
		width:  w,
		height: h,
		frame:  allocatedFrame,
		depth:  make([]float64, w*h),
	}
	fb.Clear()

	return fb, nil
}

// resized returns a blank FrameBuffer of w x h cells that carries over the settings
// of fb, namely Canvas.Seamless if the game has used the canvas. Run uses it when
// the window changes size, so a game's setting is not lost with the old frame.
func (fb *FrameBuffer) resized(w, h int) (*FrameBuffer, error) {
	out, err := NewFrameBuffer(w, h)
	if err != nil {
		return nil, err
	}
	if fb.canvas != nil {
		out.Canvas().Seamless = fb.canvas.Seamless
	}
	return out, nil
}

// Clear resets every cell to BlankCell and every depth to infinitely far, and
// empties the pixel Canvas if one has been used.
func (fb *FrameBuffer) Clear() {
	for i := range fb.frame {
		fb.frame[i] = BlankCell
		fb.depth[i] = math.Inf(1)
	}
	if fb.canvas != nil {
		fb.canvas.Clear()
	}
}

// Set stores cell at (x, y). Writes outside the buffer are ignored.
func (fb *FrameBuffer) Set(x, y int, cell Cell) {
	if !fb.inBounds(x, y) {
		return
	}

	fb.frame[y*fb.width+x] = cell
}

// SetDepth stores cell at (x, y) only if z is nearer than what is already there,
// and reports whether it did. Smaller z is nearer; a tie keeps the existing cell.
// Writes outside the buffer, and a NaN z, are ignored. Set never reads or writes
// depth, so 2D drawing and HUDs are unaffected. See Canvas.SetDepth for what a depth is
// in a scene drawn through a mesh.Camera.
func (fb *FrameBuffer) SetDepth(x, y int, z float64, cell Cell) bool {
	if !fb.inBounds(x, y) {
		return false
	}

	i := y*fb.width + x
	// Written as !(z < ...) so a NaN fails the test instead of slipping through.
	if !(z < fb.depth[i]) {
		return false
	}

	fb.depth[i] = z
	fb.frame[i] = cell
	return true
}

// At returns the cell at (x, y), or BlankCell if that is outside the buffer.
func (fb *FrameBuffer) At(x, y int) Cell {
	if !fb.inBounds(x, y) {
		return BlankCell
	}

	return fb.frame[y*fb.width+x]
}

// String returns the frame as text, for tests and for printing: the characters of the
// cells as the terminal would be sent them (NUL as a space, control characters and
// wide ones as '?'), one line per row, joined by newlines with none at the end.
// Trailing spaces are removed from each line, so a blank frame is just newlines.
// Colors and attributes are not shown, and neither are the pixels of a Canvas until
// Compose has turned them into cells.
func (fb *FrameBuffer) String() string {
	var sb strings.Builder
	sb.Grow(len(fb.frame) + fb.height)
	for y := range fb.height {
		if y > 0 {
			sb.WriteByte('\n')
		}
		row := fb.frame[y*fb.width : (y+1)*fb.width]
		end := len(row)
		for end > 0 && safeRune(row[end-1].Character) == ' ' {
			end--
		}
		for _, c := range row[:end] {
			sb.WriteRune(safeRune(c.Character))
		}
	}
	return sb.String()
}

// Width returns the number of columns.
func (fb *FrameBuffer) Width() int {
	return fb.width
}

// Height returns the number of rows.
func (fb *FrameBuffer) Height() int {
	return fb.height
}

func (fb *FrameBuffer) present(w io.Writer) error {
	return fb.presentClipped(w, fb.width, fb.height)
}

// presentClipped is present for a terminal smaller than the frame: only the first
// maxW columns of the first maxH rows are written, so nothing runs past the edge.
func (fb *FrameBuffer) presentClipped(w io.Writer, maxW, maxH int) error {
	width := min(fb.width, max(maxW, 0))
	height := min(fb.height, max(maxH, 0))

	sb := newSequenceBuilder()
	fb.appendFrame(sb, width, height)
	return sb.Flush(w)
}

// appendFrame adds to sb the whole of the top left width x height cells: each row
// starts with a cursor move and ends with the style reset, so rows do not depend on
// one another.
func (fb *FrameBuffer) appendFrame(sb *sequenceBuilder, width, height int) {
	// 3 bytes covers box-drawing runes; 32 per row covers the cursor move and color escapes.
	// An undershoot is harmless, the builder just grows once.
	sb.Grow(width*height*3 + height*32)

	for y := range height {
		sb.MoveTo(y+1, 1)
		var current style
		for x := range width {
			cell := fb.frame[y*fb.width+x]

			// Most cells share the style of the one before; only a change costs a call.
			if cell.Foreground != current.fg || cell.Background != current.bg || cell.Attrs != current.attrs {
				next := cell.style()
				sb.SetStyle(current, next)
				current = next
			}

			// Nothing but a character one column wide may reach the terminal; the
			// check is skipped for printable ASCII, nearly all of what is sent.
			r := cell.Character
			if r < 0x20 || r >= 0x7f {
				r = safeRune(r)
			}
			sb.PrintRune(r)
		}

		// Leave nothing set at the end of a row, so a later row or the shell starts clean.
		if current != (style{}) {
			sb.ResetStyle()
		}
	}
}

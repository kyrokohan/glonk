package widget

import (
	"math"

	"glonk.dev/engine"
)

// Bar is a value from 0 to 1 drawn as a run of cells, filled from the left: health, progress,
// a charge.
type Bar struct {
	// Value is how full the bar is, from 0, empty, to 1, full. A smaller number is 0, a
	// larger one is 1, and NaN is 0.
	Value float64

	// Full is the cell of the filled part, character included. The zero Cell is a block,
	// '█', in the terminal's default colors.
	Full engine.Cell

	// Empty is the cell of the rest, character included. The zero Cell is a light shade,
	// '░', in the terminal's default colors. Both are drawn as they are, so a blank with a
	// background is a bar made of color alone.
	Empty engine.Cell
}

// Draw draws the bar w cells wide from (x, y) to the right, cut off at the frame. The filled
// part is Value times w, rounded to the nearest cell.
func (b Bar) Draw(fb *engine.FrameBuffer, x, y, w int) {
	if w <= 0 {
		return
	}
	full, empty := b.Full, b.Empty
	if full == (engine.Cell{}) {
		full = engine.Cell{Character: '█'}
	}
	if empty == (engine.Cell{}) {
		empty = engine.Cell{Character: '░'}
	}

	v := b.Value
	if !(v > 0) { // not v <= 0, so that NaN is 0
		v = 0
	}
	filled := w
	if v < 1 {
		filled = int(math.Round(v * float64(w)))
	}

	fillEnd := reach(x, filled)
	for cx := max(x, 0); cx < min(reach(x, w), fb.Width()); cx++ {
		if cx < fillEnd {
			fb.Set(cx, y, full)
		} else {
			fb.Set(cx, y, empty)
		}
	}
}

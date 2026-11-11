package widget

import "glonk.dev/engine"

// Align says where text sits in the width it is given.
type Align uint8

// The alignments of text in a width.
const (
	// Left puts the text at the left edge. It is the zero value, and what any number that
	// is not one of these means.
	Left Align = iota
	// Center puts the text in the middle. When the room left over is odd, the extra cell
	// is on the right.
	Center
	// Right puts the text at the right edge.
	Right
)

// Label is a line of text.
type Label struct {
	// Text is what to write: one line, with nothing done for a newline or a tab, which are
	// drawn as '?' like any control character.
	Text string

	// Align is where the text sits in the width given to Draw.
	Align Align

	// Style is the colors and attributes of the text.
	Style engine.Cell
}

// Draw writes the label in the w cells from (x, y) to the right, aligned in them and cut off
// where they end. Only the cells of the text are drawn, not the rest of the w.
func (l Label) Draw(fb *engine.FrameBuffer, x, y, w int) {
	if w <= 0 {
		return
	}
	text := clip(l.Text, w)
	n := engine.TextWidth(text)
	switch l.Align {
	case Center:
		x = reach(x, (w-n)/2)
	case Right:
		x = reach(x, w-n)
	}

	fb.DrawText(x, y, text, l.Style.Foreground)

	// DrawText takes a foreground only, so give the cells it filled the rest of the style.
	if l.Style.Background.IsDefault() && l.Style.Attrs == 0 {
		return
	}
	for cx := max(x, 0); cx < min(reach(x, n), fb.Width()); cx++ {
		c := fb.At(cx, y)
		c.Background, c.Attrs = l.Style.Background, l.Style.Attrs
		fb.Set(cx, y, c)
	}
}

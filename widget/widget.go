package widget

import (
	"math"
	"unicode/utf8"

	"glonk.dev/engine"
)

// clip returns the longest start of text that fills at most w cells, counted as
// engine.TextWidth counts them. A combining mark has no cell, so it stays with the character
// before it.
func clip(text string, w int) string {
	cells := 0
	for i := 0; i < len(text); {
		_, size := utf8.DecodeRuneInString(text[i:])
		cells += engine.TextWidth(text[i : i+size])
		if cells > w {
			return text[:i]
		}
		i += size
	}
	return text
}

// reach returns start+n for n >= 0, held at the largest int instead of wrapping around, so
// that a huge position or size cannot come out the other side of the number line as a small
// one that lands on the frame.
func reach(start, n int) int {
	if start > math.MaxInt-n {
		return math.MaxInt
	}
	return start + n
}

// styled returns style with its Character set to r: the cell a widget draws.
func styled(style engine.Cell, r rune) engine.Cell {
	style.Character = r
	return style
}

// isHole reports whether c is blank with no background and no attributes, which draws
// nothing as the Fill of a Panel.
func isHole(c engine.Cell) bool {
	return (c.Character == ' ' || c.Character == 0) && c.Background.IsDefault() && c.Attrs == 0
}

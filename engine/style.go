package engine

import "strconv"

// Attr is a set of text attributes. Combine them with |, as in Bold | Underline. The
// zero Attr is plain text.
//
// How an attribute looks is up to the terminal and its font; Bold, for one, may be a
// heavier weight, a brighter color, or both.
type Attr uint16

// The text attributes.
const (
	// Bold draws text heavier or brighter.
	Bold Attr = 1 << iota
	// Dim draws text fainter.
	Dim
	// Underline draws a line under text.
	Underline
	// Reverse swaps the foreground and background colors.
	Reverse
)

// Has reports whether every attribute in want is set.
func (a Attr) Has(want Attr) bool {
	return a&want == want
}

// style is everything that decides how a cell is drawn besides its character.
type style struct {
	fg, bg Color
	attrs  Attr
}

// attrCodes lists, in the order they are written, the SGR code that turns each
// attribute on.
var attrCodes = [...]struct {
	attr Attr
	code string
}{
	{Bold, "1"},
	{Dim, "2"},
	{Underline, "4"},
	{Reverse, "7"},
}

// paletteIndex returns the palette color to write for c. An RGB color is written as
// the nearest palette color until true color is supported. ok is false for the default.
func (c Color) paletteIndex() (index uint8, ok bool) {
	switch c.kind() {
	case colorPalette:
		return uint8(c.bits), true
	case colorRGB:
		r, g, b := c.RGB()
		return nearestPalette(float64(r), float64(g), float64(b)), true
	}
	return 0, false
}

// SetStyle writes what changes the terminal's current style from to to, and nothing
// when they are the same. It is a single control sequence, with a code for each thing
// that changes. Going back to the plain default style takes the single reset code. An
// attribute can only be turned off by starting over (the codes that turn bold and dim
// off share one code), so then the sequence starts with a reset and writes the whole
// target style.
func (sb *sequenceBuilder) SetStyle(from, to style) {
	if from == to {
		return
	}
	if to == (style{}) {
		sb.ResetStyle()
		return
	}

	sb.buf = append(sb.buf, escapeSeq...)
	start := len(sb.buf)
	// param starts a code, after a ';' if it is not the first.
	param := func() {
		if len(sb.buf) > start {
			sb.buf = append(sb.buf, ';')
		}
	}

	if from.attrs&^to.attrs != 0 {
		param()
		sb.buf = append(sb.buf, '0')
		from = style{}
	}
	if added := to.attrs &^ from.attrs; added != 0 {
		for _, a := range attrCodes {
			if added&a.attr != 0 {
				param()
				sb.buf = append(sb.buf, a.code...)
			}
		}
	}
	if to.fg != from.fg {
		param()
		sb.appendColor("38", "39", to.fg)
	}
	if to.bg != from.bg {
		param()
		sb.appendColor("48", "49", to.bg)
	}
	sb.buf = append(sb.buf, 'm')
}

// appendColor writes the parameters that set the foreground (layer 38) or the
// background (layer 48) to c, or back to the default (code 39 or 49). An RGB color is
// written exactly when the builder is set to true color, and as the nearest palette
// color otherwise.
func (sb *sequenceBuilder) appendColor(layer, def string, c Color) {
	if c.kind() == colorRGB && sb.trueColor {
		r, g, b := c.RGB()
		sb.buf = append(sb.buf, layer...)
		sb.buf = append(sb.buf, ";2;"...)
		sb.buf = strconv.AppendUint(sb.buf, uint64(r), 10)
		sb.buf = append(sb.buf, ';')
		sb.buf = strconv.AppendUint(sb.buf, uint64(g), 10)
		sb.buf = append(sb.buf, ';')
		sb.buf = strconv.AppendUint(sb.buf, uint64(b), 10)
		return
	}
	if n, ok := c.paletteIndex(); ok {
		sb.buf = append(sb.buf, layer...)
		sb.buf = append(sb.buf, ";5;"...)
		sb.buf = strconv.AppendUint(sb.buf, uint64(n), 10)
		return
	}
	sb.buf = append(sb.buf, def...)
}

package engine

import (
	"slices"
	"unicode"
)

// replacement is drawn in place of a character that cannot be shown in one column.
// It is '?' because U+FFFD is of "ambiguous" width and takes two columns on some terminals.
const replacement = '?'

// runeWidth says how the terminal treats r, which decides whether it is safe to send:
//
//	-1  not printable: a control (which a terminal would act on), a surrogate, a value
//	    outside Unicode, or an unassigned code point
//	 0  zero width: a combining mark, or a format character such as a joiner or a
//	    bidi override; it attaches to the previous cell instead of using its own
//	 1  one column
//	 2  two columns
//
// Private use characters count as one column: fonts patched for Powerline and Nerd
// Fonts draw their icons there.
func runeWidth(r rune) int {
	switch {
	case r < 0x20 || (r >= 0x7f && r < 0xa0):
		return -1
	case r > unicode.MaxRune || (r >= 0xd800 && r <= 0xdfff):
		return -1
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		return 0
	case isWide(r):
		return 2
	case unicode.IsGraphic(r) || unicode.Is(unicode.Co, r):
		return 1
	}
	return -1
}

// TextWidth returns how many cells [FrameBuffer.DrawText] fills with text, not counting
// any it would clip: one for each character except combining marks and joiners, which have
// no cell of their own. A wide character (CJK, most emoji) is one cell, drawn as '?', and
// so is a control character, as DrawText draws them. Use it to center or right-align text,
// or to cut it to a width:
//
//	fb.DrawText(x+(w-engine.TextWidth(title))/2, y, title, color)
func TextWidth(text string) int {
	n := 0
	for _, r := range text {
		if runeWidth(r) != 0 {
			n++
		}
	}
	return n
}

// safeRune returns what to send the terminal for a cell holding r: r itself when it
// is a printable character one column wide, a space for NUL (the zero Cell, so that
// Cell{Background: c} is a coloured blank), and replacement for everything else.
func safeRune(r rune) rune {
	switch {
	case r >= 0x20 && r < 0x7f:
		return r
	case r >= 0x2500 && r <= 0x259f, r >= 0x2800 && r <= 0x28ff:
		// Box drawing, block elements and braille: borders and the pixel canvas draw
		// these every frame, and all of them are one column wide.
		return r
	case r == 0:
		return ' '
	case runeWidth(r) == 1:
		return r
	}
	return replacement
}

// isWide reports whether r is East Asian Wide or Fullwidth, which takes two columns.
func isWide(r rune) bool {
	if r < wideRanges[0][0] {
		return false
	}
	_, found := slices.BinarySearchFunc(wideRanges, r, func(rg [2]rune, r rune) int {
		switch {
		case r < rg[0]:
			return 1
		case r > rg[1]:
			return -1
		}
		return 0
	})
	return found
}

// wideRanges lists the East Asian Wide and Fullwidth code points, as inclusive
// ranges in order. Generated from golang.org/x/text/width v0.42.0 (Unicode 17.0.0):
// walk every code point with width.LookupRune(r).Kind(), keep EastAsianWide and
// EastAsianFullwidth, and merge neighbours. Regenerate when Unicode moves on.
var wideRanges = [][2]rune{
	{0x1100, 0x115F},
	{0x231A, 0x231B},
	{0x2329, 0x232A},
	{0x23E9, 0x23EC},
	{0x23F0, 0x23F0},
	{0x23F3, 0x23F3},
	{0x25FD, 0x25FE},
	{0x2614, 0x2615},
	{0x2630, 0x2637},
	{0x2648, 0x2653},
	{0x267F, 0x267F},
	{0x268A, 0x268F},
	{0x2693, 0x2693},
	{0x26A1, 0x26A1},
	{0x26AA, 0x26AB},
	{0x26BD, 0x26BE},
	{0x26C4, 0x26C5},
	{0x26CE, 0x26CE},
	{0x26D4, 0x26D4},
	{0x26EA, 0x26EA},
	{0x26F2, 0x26F3},
	{0x26F5, 0x26F5},
	{0x26FA, 0x26FA},
	{0x26FD, 0x26FD},
	{0x2705, 0x2705},
	{0x270A, 0x270B},
	{0x2728, 0x2728},
	{0x274C, 0x274C},
	{0x274E, 0x274E},
	{0x2753, 0x2755},
	{0x2757, 0x2757},
	{0x2795, 0x2797},
	{0x27B0, 0x27B0},
	{0x27BF, 0x27BF},
	{0x2B1B, 0x2B1C},
	{0x2B50, 0x2B50},
	{0x2B55, 0x2B55},
	{0x2E80, 0x2E99},
	{0x2E9B, 0x2EF3},
	{0x2F00, 0x2FD5},
	{0x2FF0, 0x303E},
	{0x3041, 0x3096},
	{0x3099, 0x30FF},
	{0x3105, 0x312F},
	{0x3131, 0x318E},
	{0x3190, 0x31E5},
	{0x31EF, 0x321E},
	{0x3220, 0x3247},
	{0x3250, 0xA48C},
	{0xA490, 0xA4C6},
	{0xA960, 0xA97C},
	{0xAC00, 0xD7A3},
	{0xF900, 0xFAFF},
	{0xFE10, 0xFE19},
	{0xFE30, 0xFE52},
	{0xFE54, 0xFE66},
	{0xFE68, 0xFE6B},
	{0xFF01, 0xFF60},
	{0xFFE0, 0xFFE6},
	{0x16FE0, 0x16FE4},
	{0x16FF0, 0x16FF6},
	{0x17000, 0x18CD5},
	{0x18CFF, 0x18D1E},
	{0x18D80, 0x18DF2},
	{0x1AFF0, 0x1AFF3},
	{0x1AFF5, 0x1AFFB},
	{0x1AFFD, 0x1AFFE},
	{0x1B000, 0x1B122},
	{0x1B132, 0x1B132},
	{0x1B150, 0x1B152},
	{0x1B155, 0x1B155},
	{0x1B164, 0x1B167},
	{0x1B170, 0x1B2FB},
	{0x1D300, 0x1D356},
	{0x1D360, 0x1D376},
	{0x1F004, 0x1F004},
	{0x1F0CF, 0x1F0CF},
	{0x1F18E, 0x1F18E},
	{0x1F191, 0x1F19A},
	{0x1F200, 0x1F202},
	{0x1F210, 0x1F23B},
	{0x1F240, 0x1F248},
	{0x1F250, 0x1F251},
	{0x1F260, 0x1F265},
	{0x1F300, 0x1F320},
	{0x1F32D, 0x1F335},
	{0x1F337, 0x1F37C},
	{0x1F37E, 0x1F393},
	{0x1F3A0, 0x1F3CA},
	{0x1F3CF, 0x1F3D3},
	{0x1F3E0, 0x1F3F0},
	{0x1F3F4, 0x1F3F4},
	{0x1F3F8, 0x1F43E},
	{0x1F440, 0x1F440},
	{0x1F442, 0x1F4FC},
	{0x1F4FF, 0x1F53D},
	{0x1F54B, 0x1F54E},
	{0x1F550, 0x1F567},
	{0x1F57A, 0x1F57A},
	{0x1F595, 0x1F596},
	{0x1F5A4, 0x1F5A4},
	{0x1F5FB, 0x1F64F},
	{0x1F680, 0x1F6C5},
	{0x1F6CC, 0x1F6CC},
	{0x1F6D0, 0x1F6D2},
	{0x1F6D5, 0x1F6D8},
	{0x1F6DC, 0x1F6DF},
	{0x1F6EB, 0x1F6EC},
	{0x1F6F4, 0x1F6FC},
	{0x1F7E0, 0x1F7EB},
	{0x1F7F0, 0x1F7F0},
	{0x1F90C, 0x1F93A},
	{0x1F93C, 0x1F945},
	{0x1F947, 0x1F9FF},
	{0x1FA70, 0x1FA7C},
	{0x1FA80, 0x1FA8A},
	{0x1FA8E, 0x1FAC6},
	{0x1FAC8, 0x1FAC8},
	{0x1FACD, 0x1FADC},
	{0x1FADF, 0x1FAEA},
	{0x1FAEF, 0x1FAF8},
	{0x20000, 0x3FFFF},
}

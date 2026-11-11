package engine

import "testing"

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want int
	}{
		{"nul", 0, -1},
		{"escape", 0x1b, -1},
		{"last c0", 0x1f, -1},
		{"delete", 0x7f, -1},
		{"first c1", 0x80, -1},
		{"c1 csi", 0x9b, -1},
		{"last c1", 0x9f, -1},
		{"first surrogate", 0xd800, -1},
		{"last surrogate", 0xdfff, -1},
		{"negative", -1, -1},
		{"past unicode", 0x110000, -1},
		{"unassigned", 0x0378, -1},
		{"noncharacter", 0xfdd0, -1},
		{"noncharacter at the end of a plane", 0xffff, -1},

		{"combining acute", 0x0301, 0},
		{"last combining diacritical", 0x036f, 0},
		{"enclosing mark", 0x20dd, 0},
		{"variation selector 16", 0xfe0f, 0},
		{"zero width space", 0x200b, 0},
		{"zero width joiner", 0x200d, 0},
		{"left to right mark", 0x200e, 0},
		{"right to left override", 0x202e, 0},
		{"word joiner", 0x2060, 0},
		{"byte order mark", 0xfeff, 0},

		{"space", ' ', 1},
		{"tilde", '~', 1},
		{"no break space", 0xa0, 1},
		{"latin accent", 'é', 1},
		{"greek", 'α', 1},
		{"cyrillic", 'я', 1},
		{"arrow, ambiguous width", '→', 1},
		{"box drawing", '─', 1},
		{"half block", '▀', 1},
		{"braille", '⣿', 1},
		{"halfwidth katakana", 'ｱ', 1},
		{"thai", 'ก', 1},
		{"spacing combining mark", 0x093e, 1},
		{"hangul medial vowel, narrow", 0x1160, 1},
		{"private use, first", 0xe000, 1},
		{"private use, powerline", 0xe0b0, 1},
		{"private use, last of the bmp block", 0xf8ff, 1},

		{"cjk ideograph", '中', 2},
		{"hangul syllable", '가', 2},
		{"fullwidth latin", 'Ａ', 2},
		{"ideographic space", 0x3000, 2},
		{"first hangul jamo", 0x1100, 2},
		{"last wide hangul jamo", 0x115f, 2},
		{"watch emoji", 0x231a, 2},
		{"left angle bracket", 0x2329, 2},
		{"grinning face", 0x1f600, 2},
		{"first of plane 2", 0x20000, 2},
		{"inside plane 2", 0x2fffd, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runeWidth(tt.r); got != tt.want {
				t.Errorf("wanted runeWidth(%U) = %d, got %d", tt.r, tt.want, got)
			}
		})
	}
}

func TestTextWidth(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 5},
		{"spaces count", "a b", 3},
		{"accent as a combining mark takes no cell", "e\u0301", 1},
		{"precomposed accent takes one", "é", 1},
		{"zero width joiner takes none", "a\u200db", 2},
		{"only combining marks", "\u0301\u200b", 0},
		{"wide character is one cell, drawn as a question mark", "中a", 2},
		{"emoji is one cell", "😀", 1},
		{"control character is one cell", "a\tb\n", 4},
		{"nul is one cell", "\x00", 1},
		{"invalid utf-8 is one cell a byte", "\xff\xfe", 2},
		{"box drawing", "┌─┐", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TextWidth(tt.text); got != tt.want {
				t.Errorf("wanted TextWidth(%q) = %d, got %d", tt.text, tt.want, got)
			}
		})
	}
}

func TestWideRangesAreOrderedAndExact(t *testing.T) {
	for i, rg := range wideRanges {
		if rg[0] > rg[1] {
			t.Fatalf("wanted range %d to run upward, got %U-%U", i, rg[0], rg[1])
		}
		if i > 0 && rg[0] <= wideRanges[i-1][1]+1 {
			t.Fatalf("wanted range %d to start after range %d with a gap, got %U after %U", i, i-1, rg[0], wideRanges[i-1][1])
		}
		for _, r := range []rune{rg[0], rg[1]} {
			if !isWide(r) {
				t.Errorf("wanted %U inside %U-%U to be wide", r, rg[0], rg[1])
			}
		}
		if isWide(rg[0] - 1) {
			t.Errorf("wanted %U, just before range %U-%U, not to be wide", rg[0]-1, rg[0], rg[1])
		}
		if isWide(rg[1] + 1) {
			t.Errorf("wanted %U, just after range %U-%U, not to be wide", rg[1]+1, rg[0], rg[1])
		}
	}
}

func TestSafeRune(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want rune
	}{
		{"printable ascii", 'g', 'g'},
		{"space", ' ', ' '},
		{"tilde", '~', '~'},
		{"nul becomes a space", 0, ' '},
		{"escape", 0x1b, '?'},
		{"delete", 0x7f, '?'},
		{"c1 csi", 0x9b, '?'},
		{"latin accent kept", 'é', 'é'},
		{"box drawing kept", '┌', '┌'},
		{"first block element", 0x2580, 0x2580},
		{"last block element", 0x259f, 0x259f},
		{"just past block elements is checked", 0x25a0, 0x25a0},
		{"braille kept", 0x2800, 0x2800},
		{"last braille", 0x28ff, 0x28ff},
		{"wide", '中', '?'},
		{"wide just above braille", 0x2e80, '?'},
		{"narrow just above braille", 0x2900, 0x2900},
		{"combining", 0x0301, '?'},
		{"bidi override", 0x202e, '?'},
		{"private use kept", 0xe0b0, 0xe0b0},
		{"invalid", -5, '?'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeRune(tt.r); got != tt.want {
				t.Errorf("wanted safeRune(%U) = %q, got %q", tt.r, tt.want, got)
			}
		})
	}
}

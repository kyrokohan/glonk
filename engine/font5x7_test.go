package engine

import (
	"slices"
	"strings"
	"testing"
)

// glyphRows is the glyph of a character as seven rows of five characters, from the bits the font is
// drawn from, so that the tests below read as pictures.
func glyphRows(r rune) [fontHeight]string {
	var rows [fontHeight]string
	for y, bits := range font[r-' '] {
		var sb strings.Builder
		for x := range fontWidth {
			if bits&(1<<(fontWidth-1-x)) != 0 {
				sb.WriteByte('#')
			} else {
				sb.WriteByte('.')
			}
		}
		rows[y] = sb.String()
	}
	return rows
}

// inkRows is the first and last row of a glyph that have a pixel, or -1 and -1 for none.
func inkRows(r rune) (first, last int) {
	first, last = -1, -1
	for y, row := range glyphRows(r) {
		if strings.Contains(row, "#") {
			if first < 0 {
				first = y
			}
			last = y
		}
	}
	return first, last
}

func mirrored(rows [fontHeight]string) (out [fontHeight]string) {
	for y, row := range rows {
		b := []byte(row)
		slices.Reverse(b)
		out[y] = string(b)
	}
	return out
}

// The font has a glyph for every character from the space to the tilde, the space is blank and every
// other has pixels, and no two are alike.
func TestEveryPrintableCharacterHasAGlyphOfItsOwn(t *testing.T) {
	seen := map[[fontHeight]string]rune{}
	for r := ' '; r <= '~'; r++ {
		if strings.TrimSpace(glyphArt[r-' ']) == "" {
			t.Errorf("%q: wanted a glyph", r)
			continue
		}
		rows := glyphRows(r)
		ink := strings.Contains(strings.Join(rows[:], ""), "#")
		if (r == ' ') == ink {
			t.Errorf("%q: wanted the space to be blank and every other glyph to have a pixel, got %v", r, rows)
		}
		if other, dup := seen[rows]; dup {
			t.Errorf("%q and %q are drawn the same: %v", r, other, rows)
		}
		seen[rows] = r
	}
}

// parseGlyph refuses what is not a glyph, and reads what is.
func TestParseGlyph(t *testing.T) {
	good := ".....\n#...#\n#####\n.....\n.....\n.....\n#...."
	bits, err := parseGlyph(good)
	if err != nil {
		t.Fatalf("wanted the glyph to be read, got %v", err)
	}
	if want := [fontHeight]uint8{0, 0b10001, 0b11111, 0, 0, 0, 0b10000}; bits != want {
		t.Errorf("wanted %05b, got %05b", want, bits)
	}
	for name, art := range map[string]string{
		"six rows":      ".....\n.....\n.....\n.....\n.....\n.....",
		"eight rows":    ".....\n.....\n.....\n.....\n.....\n.....\n.....\n.....",
		"a short row":   ".....\n....\n.....\n.....\n.....\n.....\n.....",
		"a long row":    ".....\n......\n.....\n.....\n.....\n.....\n.....",
		"another mark":  ".....\n..x..\n.....\n.....\n.....\n.....\n.....",
		"a space mark":  ".....\n.. ..\n.....\n.....\n.....\n.....\n.....",
		"nothing":       "",
		"only a border": "#####\n#####",
	} {
		if _, err := parseGlyph(art); err == nil {
			t.Errorf("%s: wanted an error", name)
		}
	}
}

// Where the ink of a glyph is: capitals and digits fill the seven rows, a lowercase letter without a stem
// fills rows 2 to 6, those with a stem up all seven, and the ones with a tail keep to the rows they were
// drawn in. Every baseline is row 6, and g, whose hook needs two rows, is the one a row higher.
func TestGlyphsStandOnTheBaseline(t *testing.T) {
	span := func(chars string, first, last int) {
		for _, r := range chars {
			if f, l := inkRows(r); f != first || l != last {
				t.Errorf("%q: wanted ink on rows %d to %d, got %d to %d: %v", r, first, last, f, l, glyphRows(r))
			}
		}
	}
	// Capitals and digits.
	span("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", 0, 6)
	// Lowercase with no stem up or down.
	span("acemnorsuvwxz", 2, 6)
	// Lowercase with a stem up.
	span("bdfhklt", 0, 6)
	// With a tail: the body on rows 2 to 5 and the tail on row 6, and the j with its dot on row 0.
	span("pqy", 2, 6)
	span("j", 0, 6)
	span("g", 1, 6)
	// The i has a dot on row 0 and a body on rows 2 to 6.
	span("i", 0, 6)
	for _, r := range "ij" {
		if glyphRows(r)[1] != "....." {
			t.Errorf("%q: wanted the dot separate from the body, got %v", r, glyphRows(r))
		}
	}
}

// Pairs that face each other are exact mirrors, so that a bracket or a stem never leans the wrong way.
func TestGlyphsThatFaceEachOtherAreMirrors(t *testing.T) {
	for _, pair := range []string{"bd", "pq", "()", "[]", "{}", "<>", "/\\"} {
		a, b := rune(pair[0]), rune(pair[1])
		if got, want := glyphRows(b), mirrored(glyphRows(a)); got != want {
			t.Errorf("%q is not the mirror of %q:\n%v\n%v", b, a, got, want)
		}
	}
}

// Glyphs that are the same left and right are drawn so.
func TestSymmetricGlyphsAreSymmetric(t *testing.T) {
	for _, r := range " HIOTUVWXYAM8!+*-=#|^_" {
		if rows := glyphRows(r); rows != mirrored(rows) {
			t.Errorf("%q: wanted the same left and right, got %v", r, rows)
		}
	}
	// And the ones that are not are not, so the check above is not a check of nothing.
	for _, r := range "bdpqNZ2/" {
		if rows := glyphRows(r); rows == mirrored(rows) {
			t.Errorf("%q: wanted it to lean one way", r)
		}
	}
}

// Ten glyphs, typed again here from how they were drawn, to catch a row that was changed by accident.
func TestSomeGlyphsAreAsDrawn(t *testing.T) {
	for r, want := range map[rune]string{
		'A': ".###.\n#...#\n#...#\n#####\n#...#\n#...#\n#...#",
		'E': "#####\n#....\n#....\n####.\n#....\n#....\n#####",
		'K': "#...#\n#..#.\n#.#..\n##...\n#.#..\n#..#.\n#...#",
		'R': "####.\n#...#\n#...#\n####.\n#.#..\n#..#.\n#...#",
		'S': ".####\n#....\n#....\n.###.\n....#\n....#\n####.",
		'0': ".###.\n#...#\n#..##\n#.#.#\n##..#\n#...#\n.###.",
		'5': "#####\n#....\n####.\n....#\n....#\n#...#\n.###.",
		'g': ".....\n.###.\n#...#\n#...#\n.####\n....#\n####.",
		'y': ".....\n.....\n#...#\n#...#\n.#.#.\n..#..\n.#...",
		'?': ".###.\n#...#\n....#\n...#.\n..#..\n.....\n..#..",
	} {
		rows := glyphRows(r)
		if got := strings.Join(rows[:], "\n"); got != want {
			t.Errorf("%q: wanted\n%s\ngot\n%s", r, want, got)
		}
	}
}

package engine

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unicode"
)

// stamp draws text the long way, to hold DrawText to: every pixel of every letter, as a block of
// scale by scale, with Set, which does its own clipping, from the pictures the font is drawn as and not
// from the bits it is drawn with. It decides what is drawn for a character on its own: a mark with no
// width is skipped, a NUL is a space, and what is not printable ASCII is a question mark.
func stamp(c *Canvas, x, y int, text string, color Color, scale int) {
	for _, r := range text {
		if unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) {
			continue
		}
		g := r
		switch {
		case r == 0:
			g = ' '
		case r < ' ' || r > '~':
			g = '?'
		}
		for gy, row := range strings.Fields(glyphArt[g-' ']) {
			for gx, ch := range row {
				if ch != '#' {
					continue
				}
				for dy := range scale {
					for dx := range scale {
						c.Set(x+gx*scale+dx, y+gy*scale+dy, color)
					}
				}
			}
		}
		x += 6 * scale
	}
}

// pixelsOf is every pixel of a canvas, row by row.
func pixelsOf(c *Canvas) []Color {
	out := make([]Color, 0, c.Width()*c.Height())
	for y := range c.Height() {
		for x := range c.Width() {
			out = append(out, c.At(x, y))
		}
	}
	return out
}

// DrawText draws the letters of the font, the whole of any text and any part of one that is clipped, as
// the long way draws them, wherever it is on the canvas and at any small scale: over a thousand strings
// of every kind of character.
func TestDrawTextIsWhatStampingEveryPixelDraws(t *testing.T) {
	rng := rand.New(rand.NewPCG(38, 1))
	alphabet := []rune("abcxyzXYZ0189 !?@~Hg.\u00e9\u65e5\U0001F600\t\n\x00\x7f\u0301\u200d\ufffd")
	ink := RGB(250, 80, 20)
	for trial := range 1200 {
		var sb strings.Builder
		for range rng.IntN(9) {
			sb.WriteRune(alphabet[rng.IntN(len(alphabet))])
		}
		text := sb.String()
		x, y, scale := rng.IntN(121)-70, rng.IntN(41)-20, 1+rng.IntN(5)

		_, got := newTestCanvas(t, 30, 8) // 30 by 16 pixels
		_, want := newTestCanvas(t, 30, 8)
		got.DrawTextScaled(x, y, text, ink, scale)
		stamp(want, x, y, text, ink, scale)
		if !slices.Equal(pixelsOf(got), pixelsOf(want)) {
			t.Fatalf("trial %d: %q at (%d, %d) at scale %d: wanted what stamping every pixel draws", trial, text, x, y, scale)
		}
	}
}

// The letter A, by itself, at a position, is exactly its picture and nothing else: typed here as it is
// drawn, from the outside.
func TestDrawTextDrawsALetterExactly(t *testing.T) {
	_, c := newTestCanvas(t, 14, 6) // 14 by 12 pixels
	ink := RGB(1, 2, 3)
	c.DrawText(3, 2, "A", ink)
	want := []string{
		"..............",
		"..............",
		"....###.......",
		"...#...#......",
		"...#...#......",
		"...#####......",
		"...#...#......",
		"...#...#......",
		"...#...#......",
		"..............",
		"..............",
		"..............",
	}
	for y, row := range want {
		for x, ch := range row {
			if drawn := c.At(x, y) == ink; drawn != (ch == '#') {
				t.Errorf("pixel (%d, %d): wanted drawn to be %v", x, y, ch == '#')
			}
		}
	}
}

// The letters are 6 pixels apart, and a scale of s is every pixel a block of s by s with the letters 6s apart.
func TestDrawTextAdvancesBySixTimesTheScale(t *testing.T) {
	ink := RGB(9, 9, 9)
	for _, scale := range []int{1, 2, 3, 7} {
		_, c := newTestCanvas(t, 100, 10)
		c.DrawTextScaled(4, 3, "II", ink, scale)
		// The I has a bar of three pixels across its top row, from the second column to the fourth.
		for _, tt := range []struct {
			letter int
			col    int
			want   bool
		}{{0, 0, false}, {0, 1, true}, {0, 3, true}, {0, 4, false}, {1, 0, false}, {1, 1, true}, {1, 3, true}, {1, 4, false}} {
			x := 4 + tt.letter*6*scale + tt.col*scale
			for _, dx := range []int{0, scale - 1} {
				if got := c.At(x+dx, 3) == ink; got != tt.want {
					t.Errorf("scale %d: letter %d column %d: pixel (%d, 3): wanted drawn to be %v", scale, tt.letter, tt.col, x+dx, tt.want)
				}
			}
			// A block is scale rows tall: the row above it and the row after it are not drawn.
			if scale > 1 && c.At(x, 3+scale-1) == ink != tt.want {
				t.Errorf("scale %d: letter %d column %d: the last row of the block", scale, tt.letter, tt.col)
			}
		}
	}
}

// A text at scale s is the picture at scale 1 with every pixel a block of s by s, worked out by
// replicating it, not by drawing it again.
func TestDrawTextScaledIsTheSmallPictureMadeBigger(t *testing.T) {
	ink := RGB(30, 60, 90)
	for _, text := range []string{"Glonk 38", "pqgy,;!?"} {
		for scale := 1; scale <= 4; scale++ {
			_, small := newTestCanvas(t, 60, 8)
			small.DrawText(0, 0, text, ink)
			_, big := newTestCanvas(t, 60*scale, 8*scale)
			big.DrawTextScaled(5, 4, text, ink, scale)
			for py := range big.Height() {
				for px := range big.Width() {
					want := Color{}
					if px >= 5 && py >= 4 {
						want = small.At((px-5)/scale, (py-4)/scale)
					}
					if got := big.At(px, py); got != want {
						t.Fatalf("%q at scale %d: pixel (%d, %d): wanted %v, got %v", text, scale, px, py, want, got)
					}
				}
			}
		}
	}
}

// Only the pixels of the letters are written: what was on the canvas in the gaps stays, and depth is
// neither read nor written. The zero Color erases the pixels it covers, as FillRect does.
func TestDrawTextWritesOnlyTheLettersAndNoDepth(t *testing.T) {
	_, c := newTestCanvas(t, 30, 6)
	for y := range c.Height() {
		for x := range c.Width() {
			c.SetDepth(x, y, float64(1+x+y), Palette(9)) // a background with a depth of its own
		}
	}
	before, depth := pixelsOf(c), slices.Clone(c.depth)
	ink := RGB(255, 255, 0)

	c.DrawTextScaled(2, 1, "Hi!", ink, 1)
	_, want := newTestCanvas(t, 30, 6)
	stamp(want, 2, 1, "Hi!", ink, 1)
	drawn := 0
	for i, p := range pixelsOf(c) {
		if x, y := i%30, i/30; want.At(x, y) == ink {
			drawn++
			if p != ink {
				t.Fatalf("pixel (%d, %d): wanted a pixel of a letter", x, y)
			}
		} else if p != before[i] {
			t.Fatalf("pixel (%d, %d): wanted what was behind the letters, %v, got %v", x, y, before[i], p)
		}
	}
	if drawn < 12 {
		t.Errorf("wanted letters of at least 12 pixels, got %d", drawn)
	}
	if !slices.Equal(c.depth, depth) {
		t.Errorf("wanted the depth left as it was")
	}

	// The zero Color erases the same pixels.
	c.DrawTextScaled(2, 1, "Hi!", Color{}, 1)
	for i, p := range pixelsOf(c) {
		x, y := i%30, i/30
		if want.At(x, y) == ink {
			if p != (Color{}) {
				t.Fatalf("pixel (%d, %d): wanted it erased, got %v", x, y, p)
			}
		} else if p != before[i] {
			t.Fatalf("pixel (%d, %d): wanted what was behind the letters", x, y)
		}
	}
}

// Nothing is drawn outside the cells of the letters: not to the left of x, not past x + the width, not
// above y or below y + 7 times the scale. The text is wholly on the canvas here, so nothing is clipped.
func TestDrawTextStaysInsideItsWidthAndHeight(t *testing.T) {
	rng := rand.New(rand.NewPCG(38, 2))
	alphabet := []rune("abcxyzHgM.! \u00e9\u65e5\u0301")
	for range 300 {
		var sb strings.Builder
		for range 1 + rng.IntN(6) {
			sb.WriteRune(alphabet[rng.IntN(len(alphabet))])
		}
		text := sb.String()
		scale := 1 + rng.IntN(3)
		x, y := 5+rng.IntN(5), 4+rng.IntN(5)

		_, c := newTestCanvas(t, 90, 20)
		c.DrawTextScaled(x, y, text, RGB(1, 1, 1), scale)
		w := PixelTextWidth(text, scale)
		for i, p := range pixelsOf(c) {
			px, py := i%c.Width(), i/c.Width()
			if p != (Color{}) && (px < x || px >= x+w || py < y || py >= y+7*scale) {
				t.Fatalf("%q at scale %d from (%d, %d): pixel (%d, %d) is drawn, outside %d by %d", text, scale, x, y, px, py, w, 7*scale)
			}
		}
	}
}

// A character with no letter is a question mark, a NUL is a space, and one with no width of its own is not
// there at all, neither drawn nor counted.
func TestCharactersThatHaveNoLetter(t *testing.T) {
	question := func(c *Canvas, text string) []Color {
		c.Clear()
		c.DrawText(2, 1, text, RGB(7, 7, 7))
		return pixelsOf(c)
	}
	_, c := newTestCanvas(t, 30, 6)
	want := question(c, "A?B")
	for name, ch := range map[string]string{
		"an accented letter": "\u00e9",
		"a Chinese letter":   "\u65e5",
		"an emoji":           "\U0001F600",
		"a tab":              "\t",
		"a newline":          "\n",
		"a delete":           "\x7f",
		"an escape":          "\x1b",
		"an invalid byte":    "\xff",
		"a replacement":      "\ufffd",
		"a no-break space":   "\u00a0",
		"a private use":      "\ue000",
	} {
		if got := question(c, "A"+ch+"B"); !slices.Equal(got, want) {
			t.Errorf("%s: wanted it drawn as a question mark", name)
		}
		if got, wantW := PixelTextWidth("A"+ch+"B", 1), PixelTextWidth("A?B", 1); got != wantW {
			t.Errorf("%s: wanted the width of a question mark, %d, got %d", name, wantW, got)
		}
	}

	// A NUL is a space.
	if got, want := question(c, "A\x00B"), question(c, "A B"); !slices.Equal(got, want) {
		t.Errorf("wanted a NUL drawn as a space")
	}
	if got := PixelTextWidth("A\x00B", 1); got != PixelTextWidth("A B", 1) {
		t.Errorf("wanted a NUL as wide as a space, got %d", got)
	}

	// A mark with no width of its own, and a joiner, are skipped: the text is the one without them.
	for name, mark := range map[string]string{"a combining accent": "\u0301", "a joiner": "\u200d", "a variation selector": "\ufe0f", "a zero width space": "\u200b"} {
		if got, want := question(c, "e"+mark+"B"), question(c, "eB"); !slices.Equal(got, want) {
			t.Errorf("%s: wanted it skipped", name)
		}
		if got, want := PixelTextWidth("e"+mark+"B", 1), PixelTextWidth("eB", 1); got != want {
			t.Errorf("%s: wanted it not counted, got %d for %d", name, got, want)
		}
		if got := PixelTextWidth(mark+mark, 3); got != 0 {
			t.Errorf("%s: wanted a text of nothing else to have no width, got %d", name, got)
		}
	}
}

// How wide a text is: 5 pixels for a letter, and 6 more for each after it, times the scale.
func TestPixelTextWidth(t *testing.T) {
	for _, tt := range []struct {
		text  string
		scale int
		want  int
	}{
		{"", 1, 0},
		{"A", 1, 5},
		{"AB", 1, 11},
		{"ABC", 1, 17},
		{"Hello", 1, 29},
		{"A", 2, 10},
		{"AB", 2, 22},
		{"AB", 3, 33},
		{" ", 1, 5},
		{"e\u0301", 1, 5},
		{"\u65e5", 1, 5},
		{"\u65e5\u672c", 1, 11},
		{"\u0301", 1, 0},
		{"A", 0, 5},
		{"A", -4, 5},
		{"AB", math.MinInt, 11},
		{"A", 4096, 5 * 4096},
		{"A", 5000, 5 * 4096},
		{"A", math.MaxInt, 5 * 4096},
	} {
		if got := PixelTextWidth(tt.text, tt.scale); got != tt.want {
			t.Errorf("PixelTextWidth(%q, %d): wanted %d, got %d", tt.text, tt.scale, tt.want, got)
		}
	}

	// A width that does not fit an int is the largest one; where an int is 32 bits that is within reach.
	text := strings.Repeat("A", 400_000)
	exact := (int64(400_000)*6 - 1) * 4096
	want := exact
	if exact > math.MaxInt {
		want = math.MaxInt
	}
	if got := PixelTextWidth(text, math.MaxInt/2); int64(got) != want {
		t.Errorf("wanted %d for a text of 400,000 letters at the largest scale, got %d", want, got)
	}
}

// Whatever the position and the scale, nothing is out of range and nothing takes long: the part that is
// on the canvas is all that is visited, and a number that would pass the largest int is held at it.
func TestDrawTextSurvivesTheLargestNumbers(t *testing.T) {
	ink := RGB(1, 2, 3)
	for _, x := range []int{math.MinInt, math.MinInt + 1, math.MinInt / 2, -1, 0, 1, math.MaxInt / 2, math.MaxInt - 1, math.MaxInt} {
		for _, y := range []int{math.MinInt, math.MinInt / 2, -3, 0, 2, math.MaxInt / 2, math.MaxInt} {
			for _, scale := range []int{math.MinInt, -1, 0, 1, 3, 4096, 4097, math.MaxInt / 2, math.MaxInt} {
				_, c := newTestCanvas(t, 12, 4)
				c.DrawTextScaled(x, y, "Go \u65e5", ink, scale)
				c.DrawText(x, y, "x", ink)
			}
		}
	}

	// A long text far off to one side is looked through at the speed of the text, and costs nothing more.
	long := strings.Repeat("glonk ", 50_000)
	_, c := newTestCanvas(t, 12, 4)
	c.DrawTextScaled(math.MinInt, 0, long, ink, 4096)
	c.DrawTextScaled(math.MaxInt-3, 0, long, ink, 3)
	c.DrawText(math.MinInt/2, 0, long, ink)
	c.DrawText(math.MaxInt/2, 0, long, ink)
	for _, p := range pixelsOf(c) {
		if p != (Color{}) {
			t.Fatalf("wanted nothing drawn from far off the canvas, got %v", p)
		}
	}

	// At the biggest scale a letter is bigger than the canvas: one pixel of it covers the whole of it.
	_, big := newTestCanvas(t, 12, 4)
	big.DrawTextScaled(0, 0, "I", ink, 4096) // the top bar of an I begins at its second column
	if got := big.At(5, 3); got != (Color{}) {
		t.Errorf("wanted the blank first column of an I at the largest scale to show nothing, got %v", got)
	}
	big.DrawTextScaled(-4096, 0, "I", ink, 4096)
	for _, p := range pixelsOf(big) {
		if p != ink {
			t.Fatalf("wanted the bar of an I to cover the canvas, got %v", p)
		}
	}
}

func TestDrawingTextAllocatesNothing(t *testing.T) {
	_, c := newTestCanvas(t, 80, 24)
	ink := RGB(255, 255, 255)
	text := "Score: 4210 - Hello, glonk! \u65e5 \u0301"
	for name, f := range map[string]func(){
		"DrawText":          func() { c.DrawText(3, 2, text, ink) },
		"DrawTextScaled":    func() { c.DrawTextScaled(3, 2, text, ink, 3) },
		"clipped":           func() { c.DrawTextScaled(-40, 20, text, ink, 2) },
		"PixelTextWidth":    func() { _ = PixelTextWidth(text, 2) },
		"far off the right": func() { c.DrawText(200, 0, text, ink) },
	} {
		if n := testing.AllocsPerRun(50, f); n != 0 {
			t.Errorf("%s: wanted no allocation, got %v", name, n)
		}
	}
}

// BenchmarkCanvasDrawText draws a row of letters as wide as the canvas, so none is clipped, at scale 1 and 3.
func BenchmarkCanvasDrawText(b *testing.B) {
	for _, size := range []struct{ cols, rows int }{{80, 24}, {200, 60}} {
		fb, err := NewFrameBuffer(size.cols, size.rows)
		if err != nil {
			b.Fatal(err)
		}
		c := fb.Canvas()
		for _, scale := range []int{1, 3} {
			letters := (size.cols + 1) / (6 * scale)
			text := strings.Repeat("Ag", letters)[:letters]
			b.Run(testName(size.cols, size.rows, scale)+fmt.Sprintf(" %d letters", letters), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					c.DrawTextScaled(0, 2, text, RGB(255, 255, 255), scale)
				}
			})
		}
	}
}

func testName(cols, rows, scale int) string {
	return fmt.Sprintf("%dx%d scale %d", cols, rows, scale)
}

// DrawText does not panic on any text, position or scale, and writes nothing outside the cells of the letters.
func FuzzCanvasDrawText(f *testing.F) {
	f.Add("Hello, glonk!", 3, 2, 1)
	f.Add("\u65e5\u0301\x00\xff", -9, -3, 3)
	f.Add("", 0, 0, 0)
	f.Add("gy", 1<<30, -1<<30, 1<<30)
	f.Fuzz(func(t *testing.T, text string, x, y, scale int) {
		_, c := newTestCanvas(t, 20, 5)
		c.DrawTextScaled(x, y, text, RGB(1, 2, 3), scale)
		w, s := PixelTextWidth(text, scale), clampTextScale(scale)
		for i, p := range pixelsOf(c) {
			px, py := i%c.Width(), i/c.Width()
			if p == (Color{}) {
				continue
			}
			// x + w and y + 7s can pass the largest int; compare by differences, which cannot.
			if px < x || px-x >= w || py < y || py-y >= 7*s {
				t.Fatalf("%q at (%d, %d) scale %d: pixel (%d, %d) is outside the text", text, x, y, scale, px, py)
			}
		}
		if s < 1 || s > maxTextScale {
			t.Fatalf("scale %d held to %d", scale, s)
		}
	})
}

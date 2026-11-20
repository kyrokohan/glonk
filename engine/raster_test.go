package engine

import (
	"math"
	"math/big"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
)

type point struct {
	X, Y int
}

func TestDrawLine(t *testing.T) {
	width := 10
	height := 10

	var tests = []struct {
		name           string
		x0, y0, x1, y1 int
		want           []point
	}{
		{
			"point",
			2, 2, 2, 2,
			[]point{{2, 2}},
		},
		{
			"horizontal",
			0, 0, 3, 0,
			[]point{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
		},
		{
			"vertical",
			0, 0, 0, 3,
			[]point{{0, 0}, {0, 1}, {0, 2}, {0, 3}},
		},
		{
			"diagonal",
			0, 0, 3, 3,
			[]point{{0, 0}, {1, 1}, {2, 2}, {3, 3}},
		},
		{
			"steep",
			0, 0, 1, 3,
			[]point{{0, 0}, {0, 1}, {1, 2}, {1, 3}},
		},
		{
			"reversed",
			3, 0, 0, 0,
			[]point{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
		},
		{
			"partly-off-screen",
			-2, 0, 2, 0,
			[]point{{0, 0}, {1, 0}, {2, 0}},
		},
		{
			"tie",
			0, 0, 1, 2,
			[]point{{0, 0}, {1, 1}, {1, 2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nfb, err := NewFrameBuffer(width, height)
			if err != nil {
				t.Fatalf("failed to initialize framebuffer: %v", err)
			}

			nfb.DrawLine(tt.x0, tt.y0, tt.x1, tt.y1, Cell{Character: '#'})

			wantCells := map[int]Cell{}
			for _, p := range tt.want {
				wantCells[getIndex(p.X, p.Y, width)] = Cell{Character: '#'}
			}

			assertFrameEquals(t, wantCells, nfb)
		})
	}
}

type pointCell struct {
	X, Y  int
	Value rune
}

func TestDrawText(t *testing.T) {
	width := 10
	height := 10
	color := Palette(7)

	var tests = []struct {
		name string
		x, y int
		text string
		want []pointCell
	}{
		{
			"basic", 1, 1, "hi",
			[]pointCell{{1, 1, 'h'}, {2, 1, 'i'}},
		},
		{
			"multibyte", 0, 0, "é→",
			[]pointCell{{0, 0, 'é'}, {1, 0, '→'}},
		},
		{
			"clips-right", 8, 0, "abcd",
			[]pointCell{{8, 0, 'a'}, {9, 0, 'b'}},
		},
		{
			"clips-left", -1, 0, "ab",
			[]pointCell{{0, 0, 'b'}},
		},
		{
			"empty", 0, 0, "",
			[]pointCell{},
		},
		{
			"tab and newline are drawn as question marks", 0, 0, "a\tb\nc",
			[]pointCell{{0, 0, 'a'}, {1, 0, '?'}, {2, 0, 'b'}, {3, 0, '?'}, {4, 0, 'c'}},
		},
		{
			"an escape sequence is shown, not run", 0, 0, "\x1b[2Jx",
			[]pointCell{{0, 0, '?'}, {1, 0, '['}, {2, 0, '2'}, {3, 0, 'J'}, {4, 0, 'x'}},
		},
		{
			"c1 csi and delete", 0, 0, "\u009b\x7f",
			[]pointCell{{0, 0, '?'}, {1, 0, '?'}},
		},
		{
			"nul is a space", 0, 0, "a\x00b",
			[]pointCell{{0, 0, 'a'}, {1, 0, ' '}, {2, 0, 'b'}},
		},
		{
			"combining accent is skipped", 0, 0, "e\u0301x",
			[]pointCell{{0, 0, 'e'}, {1, 0, 'x'}},
		},
		{
			"joiner and variation selector are skipped", 0, 0, "a\u200db\ufe0fc",
			[]pointCell{{0, 0, 'a'}, {1, 0, 'b'}, {2, 0, 'c'}},
		},
		{
			"bidi override is skipped", 0, 0, "a\u202eb",
			[]pointCell{{0, 0, 'a'}, {1, 0, 'b'}},
		},
		{
			"wide characters are question marks", 0, 0, "a中b😀",
			[]pointCell{{0, 0, 'a'}, {1, 0, '?'}, {2, 0, 'b'}, {3, 0, '?'}},
		},
		{
			"a skipped mark off the left edge does not shift the rest", -1, 0, "\u0301ab",
			[]pointCell{{0, 0, 'b'}},
		},
		{
			"skipped marks do not count towards the right edge", 8, 0, "a\u0301\u0301b\u0301c",
			[]pointCell{{8, 0, 'a'}, {9, 0, 'b'}},
		},
		{
			"invalid utf-8 is the replacement character", 0, 0, "a\xffb",
			[]pointCell{{0, 0, 'a'}, {1, 0, '\ufffd'}, {2, 0, 'b'}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nfb, err := NewFrameBuffer(width, height)
			if err != nil {
				t.Fatalf("failed to initialize framebuffer: %v", err)
			}

			nfb.DrawText(tt.x, tt.y, tt.text, color)

			wantCells := map[int]Cell{}
			for _, p := range tt.want {
				wantCells[getIndex(p.X, p.Y, width)] = Cell{Character: p.Value, Foreground: color}
			}

			assertFrameEquals(t, wantCells, nfb)
		})
	}
}

func assertRows(t *testing.T, fb *FrameBuffer, rows []string, color Color) {
	t.Helper()

	if len(rows) != fb.Height() {
		t.Fatalf("wanted len(rows)=%v, got %v", fb.Height(), len(rows))
	}

	for y := range len(rows) {
		runes := []rune(rows[y])

		if len(runes) != fb.Width() {
			t.Fatalf("wanted len(runes)=%v, got %v", fb.Width(), len(runes))
		}

		for x := range runes {
			r := runes[x]
			got := fb.At(x, y)
			if r == ' ' && got != BlankCell {
				t.Errorf("wanted %v at (%d,%d), got %v", BlankCell, x, y, got)
			}
			if r != ' ' {
				if r != got.Character {
					t.Errorf("wanted %v at (%d,%d), got %v", r, x, y, got.Character)
				}

				if color != got.Foreground {
					t.Errorf("wanted %v at (%d,%d), got %v", color, x, y, got.Foreground)
				}
			}
		}
	}
}

func TestDrawBox(t *testing.T) {
	var tests = []struct {
		name       string
		color      uint8
		bw, bh     int
		x, y, w, h int
		want       []string
	}{
		{
			"fills buffer", 0, 4, 3, 0, 0, 4, 3,
			[]string{"┌──┐", "│  │", "└──┘"},
		},
		{
			"offset, not square", 5, 6, 5, 1, 1, 4, 3,
			[]string{"      ", " ┌──┐ ", " │  │ ", " └──┘ ", "      "},
		},
		{
			"too small", 0, 3, 3, 0, 0, 1, 3,
			[]string{"   ", "   ", "   "},
		},
		{
			"clipped", 0, 3, 3, -1, -1, 3, 3,
			[]string{" │ ", "─┘ ", "   "},
		},
		{
			"too short", 0, 3, 3, 0, 0, 3, 1,
			[]string{"   ", "   ", "   "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nfb, err := NewFrameBuffer(tt.bw, tt.bh)
			if err != nil {
				t.Fatalf("failed to initialize framebuffer: %v", err)
			}

			nfb.DrawBox(tt.x, tt.y, tt.w, tt.h, pix(tt.color))

			assertRows(t, nfb, tt.want, pix(tt.color))
		})
	}
}

// joinArms are the arms of the eleven light lines and joins, written out here apart from the
// table SetJoined uses: up, down, left and right.
var joinArms = map[rune]string{
	'─': "lr", '│': "ud", '┌': "dr", '┐': "dl", '└': "ur", '┘': "ul",
	'├': "udr", '┤': "udl", '┬': "dlr", '┴': "ulr", '┼': "udlr",
}

// joinOf is the line that has every arm of a and of b.
func joinOf(t *testing.T, a, b rune) rune {
	t.Helper()
	has := func(r rune, arm byte) bool { return strings.IndexByte(joinArms[r], arm) >= 0 }
	for r := range joinArms {
		same := true
		for _, arm := range []byte("udlr") {
			if has(r, arm) != (has(a, arm) || has(b, arm)) {
				same = false
			}
		}
		if same {
			return r
		}
	}
	t.Fatalf("no line has the arms of %q and %q", a, b)
	return 0
}

// A line over a line is their join, in the colors of the one drawn; anything else is replaced as
// Set replaces it, and a write off the frame is ignored.
func TestSetJoined(t *testing.T) {
	fb := newFB(t, 2, 1)
	under := Cell{Foreground: pix(1), Background: pix(2), Attrs: Bold}
	over := Cell{Foreground: pix(3)}
	for a := range joinArms {
		for b := range joinArms {
			under.Character, over.Character = a, b
			fb.Set(0, 0, under)
			fb.SetJoined(0, 0, over)
			want := over
			want.Character = joinOf(t, a, b)
			if got := fb.At(0, 0); got != want {
				t.Errorf("%q over %q: wanted %+v, got %+v", b, a, want, got)
			}
		}
	}

	for _, pair := range [][2]rune{{'a', '─'}, {'═', '│'}, {'╴', '┼'}, {' ', '┌'}, {'─', 'a'}, {'│', '═'}, {'┼', '╴'}} {
		under.Character, over.Character = pair[0], pair[1]
		fb.Set(0, 0, under)
		fb.SetJoined(0, 0, over)
		if got := fb.At(0, 0); got != over {
			t.Errorf("%q over %q: wanted it replaced as by Set, got %+v", pair[1], pair[0], got)
		}
	}

	before := fb.String()
	for _, p := range [][2]int{{-1, 0}, {2, 0}, {0, 1}, {0, -1}} {
		fb.SetJoined(p[0], p[1], Cell{Character: '┼'})
	}
	if fb.String() != before {
		t.Errorf("wanted writes off the frame ignored")
	}
}

// Boxes that share a row, a column, or cross each other meet as joins, and a box alone keeps
// its corners.
func TestDrawBoxJoinsWhereItMeetsALine(t *testing.T) {
	tests := []struct {
		name  string
		boxes [][4]int
		want  []string
	}{
		{"stacked on a shared row", [][4]int{{0, 0, 4, 3}, {0, 2, 4, 3}},
			[]string{"┌──┐", "│  │", "├──┤", "│  │", "└──┘"}},
		{"side by side on a shared column", [][4]int{{0, 0, 3, 3}, {2, 0, 3, 3}},
			[]string{"┌─┬─┐", "│ │ │", "└─┴─┘"}},
		{"crossing", [][4]int{{0, 1, 5, 3}, {1, 0, 3, 5}},
			[]string{" ┌─┐", "┌┼─┼┐", "││ ││", "└┼─┼┘", " └─┘"}},
		{"the same box twice", [][4]int{{0, 0, 3, 3}, {0, 0, 3, 3}},
			[]string{"┌─┐", "│ │", "└─┘"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width := 0
			for _, row := range tt.want {
				width = max(width, len([]rune(row)))
			}
			fb := newFB(t, width, len(tt.want))
			for _, b := range tt.boxes {
				fb.DrawBox(b[0], b[1], b[2], b[3], Color{})
			}
			if got := fb.String(); got != strings.Join(tt.want, "\n") {
				t.Errorf("wanted\n%s\ngot\n%s", strings.Join(tt.want, "\n"), got)
			}
		})
	}
}

// A box covers the cells of the border of the reference rectangle, for positions and sizes in
// and around a small buffer and as large as an int goes, and draws nothing under 2 by 2.
func TestDrawBoxCoversTheBorderOfTheRectangle(t *testing.T) {
	const w, h = 5, 4
	fb := newFB(t, w, h)
	check := func(x, y, bw, bh int) {
		t.Helper()
		fb.Clear()
		fb.DrawBox(x, y, bw, bh, Color{})
		want := referenceRect(w, h, x, y, bw, bh, true)
		if bw < 2 || bh < 2 {
			want = make([]bool, w*h)
		}
		got := make([]bool, w*h)
		for i, c := range fb.frame {
			got[i] = c != BlankCell
		}
		if !slices.Equal(got, want) {
			t.Fatalf("box at (%d, %d) of %d by %d: wanted %v, got %v", x, y, bw, bh, want, got)
		}
	}
	for x := -3; x < w+3; x++ {
		for y := -3; y < h+3; y++ {
			for bw := -1; bw < w+5; bw++ {
				for bh := -1; bh < h+5; bh++ {
					check(x, y, bw, bh)
				}
			}
		}
	}
	big := []int{math.MinInt, math.MinInt + 1, -1, 0, 1, 2, 3, math.MaxInt - 1, math.MaxInt}
	for _, x := range big {
		for _, y := range big {
			for _, bw := range big {
				for _, bh := range big {
					check(x, y, bw, bh)
				}
			}
		}
	}
	check(2, 1, math.MaxInt, math.MaxInt)
}

// FillRect covers the cells of the reference, for every position and size in and around a small
// buffer, empty and negative sizes too, and for positions and sizes as large as an int goes, which
// must not wrap around onto it.
func TestFillRectIsTheRectangle(t *testing.T) {
	const w, h = 5, 3
	fb := newFB(t, w, h)
	mark := Cell{Character: '#', Foreground: pix(3)}
	check := func(x, y, rw, rh int) {
		t.Helper()
		fb.Clear()
		fb.FillRect(x, y, rw, rh, mark)
		if got, want := drawnCells(fb, mark), referenceRect(w, h, x, y, rw, rh, false); !slices.Equal(got, want) {
			t.Fatalf("at (%d, %d) of %d by %d: wanted %v, got %v", x, y, rw, rh, want, got)
		}
	}

	for x := -3; x < w+3; x++ {
		for y := -3; y < h+3; y++ {
			for rw := -1; rw < w+4; rw++ {
				for rh := -1; rh < h+4; rh++ {
					check(x, y, rw, rh)
				}
			}
		}
	}

	big := []int{math.MinInt, math.MinInt + 1, -1, 0, 1, 3, math.MaxInt - 1, math.MaxInt}
	for _, x := range big {
		for _, y := range big {
			for _, rw := range big {
				for _, rh := range big {
					check(x, y, rw, rh)
				}
			}
		}
	}

	// Starting a little in and going on as far as an int does: adding the size to the position
	// plainly would wrap around to a negative number.
	check(2, 1, math.MaxInt, math.MaxInt)
}

func TestFillRectNeverReadsOrWritesDepth(t *testing.T) {
	fb := newFB(t, 4, 3)
	near := Cell{Character: 'n'}
	fb.SetDepth(1, 1, 0.5, near)
	before := slices.Clone(fb.depth)

	fill := Cell{Character: '#'}
	fb.FillRect(0, 0, 4, 3, fill)

	if !slices.Equal(fb.depth, before) {
		t.Errorf("wanted the depths untouched")
	}
	if got := fb.At(1, 1); got != fill {
		t.Errorf("wanted the near cell replaced whatever its depth, got %v", got)
	}
	// What is drawn afterwards with depth meets the depth from before the fill.
	if fb.SetDepth(1, 1, 0.75, near) {
		t.Errorf("wanted a farther cell hidden by the depth that was there before")
	}
	if !fb.SetDepth(2, 1, 0.75, near) {
		t.Errorf("wanted a cell drawn where no depth was stored")
	}
}

func TestFillRectAllocatesNothing(t *testing.T) {
	fb := newFB(t, 80, 24)
	if n := testing.AllocsPerRun(50, func() {
		fb.FillRect(-5, 3, 60, 200, Cell{Character: '#'})
	}); n != 0 {
		t.Errorf("wanted FillRect to allocate nothing, got %v allocations", n)
	}
}

func TestFillTriangle(t *testing.T) {
	mark := Cell{Character: '#'}

	tests := []struct {
		name string
		w, h int
		// corners as x0,y0, x1,y1, x2,y2; every corner gets depth 1
		c    [6]float64
		want []string
	}{
		{
			"right triangle", 4, 4, [6]float64{0, 0, 4, 0, 0, 4},
			[]string{"####", "### ", "##  ", "#   "},
		},
		{
			"same triangle, other winding", 4, 4, [6]float64{0, 0, 0, 4, 4, 0},
			[]string{"####", "### ", "##  ", "#   "},
		},
		{
			"mirrored", 4, 4, [6]float64{4, 0, 0, 0, 4, 4},
			[]string{"####", " ###", "  ##", "   #"},
		},
		{
			"corners far off screen", 4, 4, [6]float64{-2, -2, 6, -2, -2, 6},
			[]string{"####", "### ", "##  ", "#   "},
		},
		{
			"last corner is the rightmost", 4, 4, [6]float64{0, 0, 0, 4, 4, 2},
			[]string{"#   ", "### ", "### ", "#   "},
		},
		{
			"last corner is the leftmost", 4, 4, [6]float64{4, 0, 4, 4, 0, 2},
			[]string{"   #", " ###", " ###", "   #"},
		},
		{
			"last corner is the topmost", 4, 4, [6]float64{4, 4, 0, 4, 0, 0},
			[]string{"#   ", "##  ", "### ", "####"},
		},
		{
			"last corner is the bottommost", 4, 4, [6]float64{0, 0, 4, 0, 0, 4},
			[]string{"####", "### ", "##  ", "#   "},
		},
		{
			"first corner is the bottommost", 4, 4, [6]float64{0, 4, 4, 0, 0, 0},
			[]string{"####", "### ", "##  ", "#   "},
		},
		{
			"covers everything", 3, 2, [6]float64{-100, -100, 300, -100, -100, 300},
			[]string{"###", "###"},
		},
		{
			"astronomical corners", 3, 2, [6]float64{-1e12, -1e12, 3e12, -1e12, -1e12, 3e12},
			[]string{"###", "###"},
		},
		{
			"entirely off screen", 4, 4, [6]float64{10, 10, 20, 10, 10, 20},
			[]string{"    ", "    ", "    ", "    "},
		},
		{
			"left of the screen", 4, 4, [6]float64{-10, 0, -5, 0, -10, 4},
			[]string{"    ", "    ", "    ", "    "},
		},
		{
			"above the screen", 4, 4, [6]float64{0, -10, 4, -10, 0, -5},
			[]string{"    ", "    ", "    ", "    "},
		},
		{
			"thin sliver between cell centres", 4, 4, [6]float64{0, 1.6, 4, 1.6, 0, 1.9},
			[]string{"    ", "    ", "    ", "    "},
		},
		{
			"one cell", 3, 3, [6]float64{1, 1, 2, 1, 1, 2},
			[]string{"   ", " # ", "   "},
		},
		{
			"collinear points", 4, 4, [6]float64{0, 0, 2, 2, 4, 4},
			[]string{"    ", "    ", "    ", "    "},
		},
		{
			"all corners equal", 4, 4, [6]float64{1, 1, 1, 1, 1, 1},
			[]string{"    ", "    ", "    ", "    "},
		},
		{
			"NaN corner", 4, 4, [6]float64{0, 0, math.NaN(), 0, 0, 4},
			[]string{"    ", "    ", "    ", "    "},
		},
		{
			"infinite corner", 4, 4, [6]float64{0, 0, math.Inf(1), 0, 0, 4},
			[]string{"    ", "    ", "    ", "    "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb, err := NewFrameBuffer(tt.w, tt.h)
			if err != nil {
				t.Fatalf("unable to initialize frame buffer: %v", err)
			}

			c := tt.c
			fb.FillTriangle(c[0], c[1], 1, c[2], c[3], 1, c[4], c[5], 1, mark)

			assertRows(t, fb, tt.want, Color{})
		})
	}
}

// Two triangles sharing an edge must tile a rectangle with no seam between them.
func TestFillTriangleSharedEdgeLeavesNoGap(t *testing.T) {
	mark := Cell{Character: '#'}

	for _, size := range []int{1, 2, 3, 4, 7, 16, 33} {
		fb, err := NewFrameBuffer(size, size)
		if err != nil {
			t.Fatalf("unable to initialize frame buffer: %v", err)
		}
		s := float64(size)

		fb.FillTriangle(0, 0, 1, s, 0, 1, s, s, 1, mark)
		fb.FillTriangle(0, 0, 1, s, s, 1, 0, s, 1, mark)

		for y := range size {
			for x := range size {
				if fb.At(x, y) != mark {
					t.Fatalf("size %d: wanted every cell filled, gap at (%d,%d)", size, x, y)
				}
			}
		}
	}
}

func TestFillTriangleDepth(t *testing.T) {
	near := Cell{Character: 'n'}
	far := Cell{Character: 'f'}
	// Each covers the whole 3x2 buffer.
	big := [6]float64{-100, -100, 300, -100, -100, 300}

	tests := []struct {
		name  string
		first Cell
		fz    float64
		next  Cell
		nz    float64
		want  rune
	}{
		{"near drawn second wins", far, 5, near, 2, 'n'},
		{"near drawn first stays", near, 2, far, 5, 'n'},
		{"tie keeps the first", near, 3, far, 3, 'n'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb, err := NewFrameBuffer(3, 2)
			if err != nil {
				t.Fatalf("unable to initialize frame buffer: %v", err)
			}

			fb.FillTriangle(big[0], big[1], tt.fz, big[2], big[3], tt.fz, big[4], big[5], tt.fz, tt.first)
			fb.FillTriangle(big[0], big[1], tt.nz, big[2], big[3], tt.nz, big[4], big[5], tt.nz, tt.next)

			for y := range 2 {
				for x := range 3 {
					if got := fb.At(x, y).Character; got != tt.want {
						t.Errorf("wanted %q at (%d,%d), got %q", tt.want, x, y, got)
					}
				}
			}
		})
	}
}

func TestFillTriangleOnlyHidesWhereCovered(t *testing.T) {
	fb, err := NewFrameBuffer(4, 4)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}

	// A far square behind a near triangle that covers its upper left half.
	fb.FillTriangle(0, 0, 9, 4, 0, 9, 4, 4, 9, Cell{Character: 'f'})
	fb.FillTriangle(0, 0, 9, 4, 4, 9, 0, 4, 9, Cell{Character: 'f'})
	fb.FillTriangle(0, 0, 1, 4, 0, 1, 0, 4, 1, Cell{Character: 'n'})

	assertRows(t, fb, []string{"nnnn", "nnnf", "nnff", "nfff"}, Color{})
}

// 1/z, not z, varies linearly on screen; interpolating z would give 1.375 here.
func TestFillTrianglePerspectiveDepth(t *testing.T) {
	fb, err := NewFrameBuffer(8, 8)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}

	fb.FillTriangle(0, 0, 1, 8, 0, 3, 0, 8, 1, Cell{Character: '#'})

	// Cell (1,1) has centre (1.5,1.5): weights 0.625, 0.1875, 0.1875.
	want := 1 / (0.625/1 + 0.1875/3 + 0.1875/1)
	if got := fb.depth[1*8+1]; math.Abs(got-want) > 1e-9 {
		t.Errorf("wanted depth %v, got %v", want, got)
	}
}

func TestFillTriangleRejectsBadDepth(t *testing.T) {
	for _, z := range []float64{0, -1, math.NaN(), math.Inf(-1)} {
		fb, err := NewFrameBuffer(4, 4)
		if err != nil {
			t.Fatalf("unable to initialize frame buffer: %v", err)
		}

		fb.FillTriangle(-100, -100, z, 300, -100, 1, -100, 300, 1, Cell{Character: '#'})

		assertRows(t, fb, []string{"    ", "    ", "    ", "    "}, Color{})
	}
}

// A cell centre lying exactly on the edge two triangles share must be drawn by at
// least one of them. Rounding makes the two edge tests disagree by about 1e-16,
// so without a tolerance some such cells fall through the seam.
func TestFillTriangleSeamThroughCellCentre(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	mark := Cell{Character: '#'}

	for trial := range 2000 {
		fb, err := NewFrameBuffer(8, 8)
		if err != nil {
			t.Fatalf("unable to initialize frame buffer: %v", err)
		}

		// The shared edge runs from c-d to c+d, through the centre of cell (3,4).
		cx, cy := 3.5, 4.5
		dx, dy := rng.Float64()*2-1, rng.Float64()*2-1
		if dx == 0 && dy == 0 {
			continue
		}
		// One far corner on each side of the edge, at random places along it.
		ax, ay := cx-dy*(0.2+rng.Float64())+dx*(rng.Float64()-0.5), cy+dx*(0.2+rng.Float64())+dy*(rng.Float64()-0.5)
		bx, by := cx+dy*(0.2+rng.Float64())+dx*(rng.Float64()-0.5), cy-dx*(0.2+rng.Float64())+dy*(rng.Float64()-0.5)

		fb.FillTriangle(cx-dx, cy-dy, 1, cx+dx, cy+dy, 1, ax, ay, 1, mark)
		fb.FillTriangle(cx+dx, cy+dy, 1, cx-dx, cy-dy, 1, bx, by, 1, mark)

		if fb.At(3, 4) != mark {
			t.Fatalf("trial %d: wanted the seam cell filled, got a gap (edge %v,%v to %v,%v)", trial, cx-dx, cy-dy, cx+dx, cy+dy)
		}
	}
}

// The tolerance is for rounding, not for growing the triangle: a cell whose centre is a
// full unit outside a long edge must stay empty.
func TestFillTriangleEdgeIsTight(t *testing.T) {
	fb, err := NewFrameBuffer(60, 60)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	mark := Cell{Character: '#'}

	// The long edge is x+y=100; cell centres (49.5,50.5) and (50.5,49.5) sit exactly on it.
	fb.FillTriangle(0, 0, 1, 100, 0, 1, 0, 100, 1, mark)

	for _, tc := range []struct {
		x, y int
		want bool
	}{
		{49, 49, true},  // x+y of the centre is 99, inside
		{49, 50, true},  // on the edge
		{50, 49, true},  // on the edge
		{50, 50, false}, // 101, outside
		{51, 49, false}, // 101, outside
	} {
		if got := fb.At(tc.x, tc.y) == mark; got != tc.want {
			t.Errorf("wanted cell (%d,%d) filled=%v, got %v", tc.x, tc.y, tc.want, got)
		}
	}
}

// referenceLine is the original step-by-step Bresenham, kept as the oracle that
// the windowed walkLine must match cell for cell.
func referenceLine(x0, y0, x1, y1 int) []point {
	dx := intAbs(x1 - x0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -intAbs(y1 - y0)
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	e := dx + dy

	var out []point
	for {
		out = append(out, point{x0, y0})
		if x0 == x1 && y0 == y1 {
			return out
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}

func windowOf(p []point, w, h int) []point {
	var out []point
	for _, q := range p {
		if q.X >= 0 && q.X < w && q.Y >= 0 && q.Y < h {
			out = append(out, q)
		}
	}
	return out
}

func walked(w, h, x0, y0, x1, y1 int) []point {
	var out []point
	walkLine(w, h, x0, y0, x1, y1, func(x, y int) { out = append(out, point{x, y}) })
	return out
}

func samePoints(a, b []point) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Every line between points of a small square, against several windows: all
// directions, slopes, ties, lines fully inside, half inside, and fully outside.
func TestWalkLineMatchesBresenhamExhaustively(t *testing.T) {
	const lim = 7
	windows := [][2]int{{1, 1}, {3, 2}, {5, 5}, {4, 7}, {9, 9}}

	for _, win := range windows {
		w, h := win[0], win[1]
		for x0 := -lim; x0 <= lim; x0++ {
			for y0 := -lim; y0 <= lim; y0++ {
				for x1 := -lim; x1 <= lim; x1++ {
					for y1 := -lim; y1 <= lim; y1++ {
						want := windowOf(referenceLine(x0, y0, x1, y1), w, h)
						got := walked(w, h, x0, y0, x1, y1)
						if !samePoints(got, want) {
							t.Fatalf("window %dx%d line (%d,%d)-(%d,%d): wanted %v, got %v", w, h, x0, y0, x1, y1, want, got)
						}
					}
				}
			}
		}
	}
}

func TestWalkLineMatchesBresenhamOnLongLines(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))

	for trial := range 3000 {
		w, h := 1+rng.IntN(60), 1+rng.IntN(30)
		span := 1 + rng.IntN(2000)
		x0, y0 := rng.IntN(2*span+1)-span, rng.IntN(2*span+1)-span
		x1, y1 := rng.IntN(2*span+1)-span, rng.IntN(2*span+1)-span

		want := windowOf(referenceLine(x0, y0, x1, y1), w, h)
		got := walked(w, h, x0, y0, x1, y1)
		if !samePoints(got, want) {
			t.Fatalf("trial %d, window %dx%d, line (%d,%d)-(%d,%d): wanted %d cells, got %d", trial, w, h, x0, y0, x1, y1, len(want), len(got))
		}
	}
}

// farAway is beyond the coordinate limit of walkLine on every system, including
// those where int is 32 bits and 1 << 40 is not a valid int.
const farAway = math.MaxInt / 2

// A line with a far-away end must cost only what it shows, not what it spans.
func TestWalkLineFinishesQuicklyForHugeLines(t *testing.T) {
	cases := [][4]int{
		{-1 << 29, 3, 1 << 29, 4},
		{-1 << 29, -1 << 29, 1 << 29, 1 << 29},
		{0, 0, 1 << 28, 1<<28 - 1},
		{-farAway, 5, farAway, 5},
		{math.MinInt, math.MinInt, math.MaxInt, math.MaxInt},
		{math.MaxInt, 0, math.MinInt, 0},
		{3, math.MinInt, 3, math.MaxInt},
		{-1 << 29, 7, 1 << 29, 7},
	}

	for _, c := range cases {
		start := time.Now()
		cells := walked(80, 24, c[0], c[1], c[2], c[3])

		if time.Since(start) > time.Second {
			t.Errorf("line %v: wanted it to finish quickly, took %v", c, time.Since(start))
		}
		if len(cells) > 80+24 {
			t.Errorf("line %v: wanted at most one cell per column or row, got %d cells", c, len(cells))
		}
		for _, p := range cells {
			if p.X < 0 || p.X >= 80 || p.Y < 0 || p.Y >= 24 {
				t.Errorf("line %v: wanted every cell inside the grid, got %v", c, p)
			}
		}
	}
}

func TestWalkLineExtremes(t *testing.T) {
	// A horizontal line through the middle of the grid, however far its ends are.
	cells := walked(80, 24, -1<<29, 7, 1<<29, 7)
	if len(cells) != 80 {
		t.Fatalf("wanted the full row of 80 cells, got %d", len(cells))
	}
	for i, p := range cells {
		if p != (point{i, 7}) {
			t.Fatalf("wanted cell %d at (%d,7), got %v", i, i, p)
		}
	}

	// A line that never touches the grid.
	if cells := walked(80, 24, -1<<29, 100, 1<<29, 100); len(cells) != 0 {
		t.Errorf("wanted nothing for a line below the grid, got %d cells", len(cells))
	}
	if cells := walked(80, 24, -1<<29, -1, 1<<29, -1); len(cells) != 0 {
		t.Errorf("wanted nothing for a line above the grid, got %d cells", len(cells))
	}
}

// Past +-2^29 an endpoint is treated as sitting at the limit, which is what keeps
// the arithmetic from overflowing. The documented price is a changed slope.
func TestWalkLineClampsFarEndpoints(t *testing.T) {
	const lim = 1 << 29

	cases := []struct {
		name string
		far  [4]int
		near [4]int
	}{
		{"far end", [4]int{0, 0, farAway, farAway - 1}, [4]int{0, 0, lim, lim}},
		{"far start", [4]int{-farAway, 1 - farAway, 5, 5}, [4]int{-lim, -lim, 5, 5}},
		{"both ends", [4]int{-farAway, 3, farAway, 9}, [4]int{-lim, 3, lim, 9}},
		{"negative end", [4]int{3, 2, -farAway, farAway - 1}, [4]int{3, 2, -lim, lim}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := walked(80, 24, c.far[0], c.far[1], c.far[2], c.far[3])
			want := walked(80, 24, c.near[0], c.near[1], c.near[2], c.near[3])

			if !samePoints(got, want) {
				t.Errorf("wanted %v, got %v", want, got)
			}
		})
	}
}

func TestMinorOffset(t *testing.T) {
	tests := []struct {
		name     string
		i, db, n int
		want     int
	}{
		{"no steps", 0, 0, 0, 0},
		{"no steps along a point", 5, 0, 0, 0},
		{"start", 0, 7, 10, 0},
		{"end", 10, 7, 10, 7},
		{"straight", 4, 0, 10, 0},
		{"diagonal", 6, 10, 10, 6},
		{"half rounds away from the start", 1, 1, 2, 1},
		{"just under a half", 1, 1, 3, 0},
		{"just over a half", 2, 1, 3, 1},
		{"largest line", 1 << 30, 1 << 30, 1 << 30, 1 << 30},
		{"largest line, halfway", 1 << 29, 1 << 30, 1 << 30, 1 << 29},
		{"largest line, almost flat", 1<<30 - 1, 1, 1 << 30, 1},
		{"product past 2^31", 1 << 20, 1 << 20, 1 << 25, 1 << 15},
		{"product near 2^61", 1<<30 - 1, 1<<30 - 1, 1 << 30, 1<<30 - 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := minorOffset(tt.i, tt.db, tt.n); got != tt.want {
				t.Errorf("wanted minorOffset(%d, %d, %d) = %d, got %d", tt.i, tt.db, tt.n, tt.want, got)
			}
		})
	}
}

// Against exact arithmetic, over values spread up to the largest line walkLine accepts.
func TestMinorOffsetMatchesExactArithmetic(t *testing.T) {
	exact := func(i, db, n int) int {
		num := new(big.Int).Mul(big.NewInt(2*int64(i)), big.NewInt(int64(db)))
		num.Add(num, big.NewInt(int64(n)))
		return int(num.Quo(num, big.NewInt(2*int64(n))).Int64())
	}

	rng := rand.New(rand.NewPCG(1, 2))
	for range 20000 {
		n := 1 + rng.IntN(1<<30)
		db := rng.IntN(n + 1)
		i := rng.IntN(n + 1)
		if got, want := minorOffset(i, db, n), exact(i, db, n); got != want {
			t.Fatalf("wanted minorOffset(%d, %d, %d) = %d, got %d", i, db, n, want, got)
		}
	}
}

package engine

import (
	"image"
	"image/color"
	"math"
	"math/big"
	"math/rand/v2"
	"slices"
	"testing"
)

// fullBlock is the character of a cell drawn all in its foreground.
const fullBlock = rune(0x2588)

// panelFill is the color a panel is filled with in these tests, and gold a picture's.
var (
	panelFill = Palette(236)
	gold      = RGB(255, 200, 0)
	white     = Palette(15)
)

// pixelColors are the colors a canvas and a frame are drawn with in the tests: all of them set.
var pixelColors = []Color{RGB(200, 30, 30), RGB(30, 200, 30), Palette(21), Palette(226), RGB(250, 250, 250)}

func rectColor(rng *rand.Rand) Color { return pixelColors[rng.IntN(len(pixelColors))] }

// rectCell is a cell of any kind: a space, a letter, a half block, a full block or a NUL, in a
// foreground that is always set and a background that is sometimes the default, with attributes.
func rectCell(rng *rand.Rand) Cell {
	cell := Cell{
		Character:  []rune{' ', 'A', 'x', 0, upperHalfBlock, lowerHalfBlock, fullBlock, 'e'}[rng.IntN(8)],
		Foreground: rectColor(rng),
		Attrs:      []Attr{0, 0, Bold, Reverse, Bold | Reverse, Underline}[rng.IntN(6)],
	}
	if rng.IntN(3) > 0 {
		cell.Background = rectColor(rng)
	}
	return cell
}

func cellsOf(fb *FrameBuffer) []Cell { return slices.Clone(fb.frame) }

// ComposeRect puts down what the picture is and keeps what was there where it is not: in the cells
// that the rectangle touches and that have a pixel, each half is the canvas's pixel if it is set and
// what the cell showed if not, and every other cell is exactly as it was, whatever the cells and the
// pixels are. What a cell shows is read by FrameBuffer.Image, which shares nothing with the code, at
// 4 pixels to a cell: a letter is a mark over the middle of its cell there, so the left edge of each half is
// its background.
func TestComposeRectPutsDownThePixelsAndKeepsWhatTheCellsShowed(t *testing.T) {
	rng := rand.New(rand.NewPCG(54, 1))
	for trial := range 2000 {
		cols, rows := 1+rng.IntN(8), 1+rng.IntN(5)
		fb, c := newTestCanvas(t, cols, rows)
		for cy := range rows {
			for cx := range cols {
				fb.Set(cx, cy, rectCell(rng))
			}
		}
		for py := range 2 * rows {
			for px := range cols {
				if rng.IntN(2) == 0 {
					c.Set(px, py, rectColor(rng))
				}
			}
		}
		x, y := rng.IntN(cols+7)-3, rng.IntN(2*rows+9)-4
		w, h := rng.IntN(cols+5)-1, rng.IntN(2*rows+5)-1

		cellsBefore, before := cellsOf(fb), fb.Image(ImageOptions{Scale: 4})
		c.ComposeRect(x, y, w, h)
		after := fb.Image(ImageOptions{Scale: 4})
		shown := func(img *image.RGBA, cx, cy, half int) color.RGBA {
			return img.RGBAAt(4*cx, 8*cy+7*half) // the top left pixel of the top half, the bottom left of the other
		}

		for cy := range rows {
			for cx := range cols {
				// Touched: one of the two pixels of the cell is inside the rectangle, by counting.
				touched := false
				for py := 2 * cy; py < 2*cy+2; py++ {
					if cx >= x && cx < x+w && py >= y && py < y+h {
						touched = true
					}
				}
				pixels := [2]Color{c.At(cx, 2*cy), c.At(cx, 2*cy+1)}
				if !touched || (pixels[0].IsDefault() && pixels[1].IsDefault()) {
					if got, want := fb.At(cx, cy), cellsBefore[cy*cols+cx]; got != want {
						t.Fatalf("trial %d: rectangle (%d, %d) %d by %d: cell (%d, %d) is not to be touched, wanted %+v, got %+v", trial, x, y, w, h, cx, cy, want, got)
					}
					continue
				}
				for half, pixel := range pixels {
					want := shown(before, cx, cy, half) // what the cell showed
					if !pixel.IsDefault() {
						want = exactRGBA(pixel, color.RGBA{})
					}
					if got := shown(after, cx, cy, half); got != want {
						t.Fatalf("trial %d: rectangle (%d, %d) %d by %d: cell (%d, %d) half %d was %+v and shows %v, wanted %v", trial, x, y, w, h, cx, cy, half, cellsBefore[cy*cols+cx], got, want)
					}
				}
			}
		}
	}
}

// On a frame with nothing in it ComposeRect of the whole canvas is Compose, cells and depths, with half
// blocks and with solid cells: the two put a pixel down the same way.
func TestComposeRectOfTheWholeCanvasIsComposeOnABlankFrame(t *testing.T) {
	rng := rand.New(rand.NewPCG(54, 2))
	for _, seamless := range []bool{false, true} {
		for trial := range 300 {
			cols, rows := 1+rng.IntN(9), 1+rng.IntN(6)
			a, ca := newTestCanvas(t, cols, rows)
			b, cb := newTestCanvas(t, cols, rows)
			ca.Seamless, cb.Seamless = seamless, seamless
			for py := range 2 * rows {
				for px := range cols {
					if rng.IntN(3) == 0 {
						continue
					}
					color, z := rectColor(rng), float64(1+rng.IntN(5))
					if rng.IntN(2) == 0 {
						ca.Set(px, py, color)
						cb.Set(px, py, color)
					} else {
						ca.SetDepth(px, py, z, color)
						cb.SetDepth(px, py, z, color)
					}
				}
			}
			ca.Compose()
			cb.ComposeRect(0, 0, cols, 2*rows)
			if !slices.Equal(a.frame, b.frame) || !slices.Equal(a.depth, b.depth) {
				t.Fatalf("seamless %v, trial %d: wanted ComposeRect of the whole canvas to be Compose", seamless, trial)
			}
		}
	}
}

// panelFrame is a frame of cols by rows cells all filled with the panel color.
func panelFrame(t *testing.T, cols, rows int) (*FrameBuffer, *Canvas) {
	fb, c := newTestCanvas(t, cols, rows)
	for y := range rows {
		for x := range cols {
			fb.Set(x, y, Cell{Character: ' ', Background: panelFill})
		}
	}
	return fb, c
}

// The picture of the issue: a coin 4 pixels across with see-through corners, drawn from an odd pixel row
// into a filled panel, with a label beside it. The cells of the coin are solid where both of their pixels
// are gold, and a half block in the panel's color and gold where one is, so the panel shows around it.
func TestAPictureInAFilledPanel(t *testing.T) {
	fb, c := panelFrame(t, 8, 3)
	for i, r := range "x 3" { // a label
		fb.Set(5+i, 1, Cell{Character: r, Foreground: white, Background: panelFill})
	}
	label := cellsOf(fb)

	// Pixels (1..4, 1..4):   . # # .
	//                        # # # #
	//                        # # # #
	//                        . # # .
	for _, p := range [][2]int{{2, 1}, {3, 1}, {1, 2}, {2, 2}, {3, 2}, {4, 2}, {1, 3}, {2, 3}, {3, 3}, {4, 3}, {2, 4}, {3, 4}} {
		c.Set(p[0], p[1], gold)
	}
	c.ComposeRect(1, 1, 4, 4)

	panel := Cell{Character: ' ', Background: panelFill}
	solid := Cell{Character: ' ', Background: gold}
	for y, want := range [][]Cell{
		// py 0 and 1: nothing above, so the panel shows over the top of the coin.
		{panel, panel, {Character: upperHalfBlock, Foreground: panelFill, Background: gold}, {Character: upperHalfBlock, Foreground: panelFill, Background: gold}, panel},
		// py 2 and 3: gold across.
		{panel, solid, solid, solid, solid},
		// py 4 and 5: gold on top and nothing below.
		{panel, panel, {Character: upperHalfBlock, Foreground: gold, Background: panelFill}, {Character: upperHalfBlock, Foreground: gold, Background: panelFill}, panel},
	} {
		for x, cell := range want {
			if got := fb.At(x, y); got != cell {
				t.Errorf("cell (%d, %d): wanted %+v, got %+v", x, y, cell, got)
			}
		}
	}
	// The label, and the panel on the other side of the coin, are as they were.
	for y := range 3 {
		for x := 5; x < 8; x++ {
			if got, want := fb.At(x, y), label[y*8+x]; got != want {
				t.Errorf("cell (%d, %d): wanted the label's %+v, got %+v", x, y, want, got)
			}
		}
	}
}

// A cell that holds a letter loses it when a pixel is put down, and keeps its background; one with both
// pixels empty keeps it. A half block keeps its other half, whichever way up, and a reversed cell keeps
// the color it shows.
func TestWhatACellShowedIsKeptInTheEmptyHalf(t *testing.T) {
	letter := Cell{Character: 'A', Foreground: white, Background: panelFill, Attrs: Bold}
	for _, tt := range []struct {
		name          string
		cell          Cell
		top, bottom   Color
		want          Cell
		wantUntouched bool
	}{
		{name: "a letter, top pixel", cell: letter, top: gold,
			want: Cell{Character: upperHalfBlock, Foreground: gold, Background: panelFill}},
		{name: "a letter, bottom pixel", cell: letter, bottom: gold,
			want: Cell{Character: upperHalfBlock, Foreground: panelFill, Background: gold}},
		{name: "a letter, both pixels", cell: letter, top: gold, bottom: white,
			want: Cell{Character: upperHalfBlock, Foreground: gold, Background: white}},
		{name: "a letter, the same pixel twice", cell: letter, top: gold, bottom: gold,
			want: Cell{Character: ' ', Background: gold}},
		{name: "a letter, no pixels", cell: letter, wantUntouched: true},
		{name: "a letter on the default background, top pixel", cell: Cell{Character: 'A', Foreground: white}, top: gold,
			want: Cell{Character: upperHalfBlock, Foreground: gold}},
		{name: "upper half block, top pixel keeps the bottom", cell: Cell{Character: upperHalfBlock, Foreground: white, Background: panelFill}, top: gold,
			want: Cell{Character: upperHalfBlock, Foreground: gold, Background: panelFill}},
		{name: "upper half block, bottom pixel keeps the top", cell: Cell{Character: upperHalfBlock, Foreground: white, Background: panelFill}, bottom: gold,
			want: Cell{Character: upperHalfBlock, Foreground: white, Background: gold}},
		{name: "lower half block, top pixel keeps the bottom", cell: Cell{Character: lowerHalfBlock, Foreground: white, Background: panelFill}, top: gold,
			want: Cell{Character: upperHalfBlock, Foreground: gold, Background: white}},
		{name: "lower half block, bottom pixel keeps the top", cell: Cell{Character: lowerHalfBlock, Foreground: white, Background: panelFill}, bottom: gold,
			want: Cell{Character: upperHalfBlock, Foreground: panelFill, Background: gold}},
		{name: "full block, top pixel keeps the bottom", cell: Cell{Character: fullBlock, Foreground: white, Background: panelFill}, top: gold,
			want: Cell{Character: upperHalfBlock, Foreground: gold, Background: white}},
		{name: "reversed space, top pixel keeps the foreground as the bottom", cell: Cell{Character: ' ', Foreground: white, Background: panelFill, Attrs: Reverse}, top: gold,
			want: Cell{Character: upperHalfBlock, Foreground: gold, Background: white}},
		{name: "reversed upper half block", cell: Cell{Character: upperHalfBlock, Foreground: white, Background: panelFill, Attrs: Reverse}, bottom: gold,
			want: Cell{Character: upperHalfBlock, Foreground: panelFill, Background: gold}},
		{name: "a pixel the color of what is under it is a solid cell", cell: Cell{Character: upperHalfBlock, Foreground: white, Background: gold}, top: gold,
			want: Cell{Character: ' ', Background: gold}},
	} {
		fb, c := newTestCanvas(t, 2, 1)
		fb.Set(1, 0, tt.cell)
		c.Set(1, 0, tt.top)
		c.Set(1, 1, tt.bottom)
		c.ComposeRect(1, 0, 1, 2)
		want := tt.want
		if tt.wantUntouched {
			want = tt.cell
		}
		if got := fb.At(1, 0); got != want {
			t.Errorf("%s: wanted %+v, got %+v", tt.name, want, got)
		}
		if got := fb.At(0, 0); got != BlankCell {
			t.Errorf("%s: wanted the cell beside it left alone, got %+v", tt.name, got)
		}
	}
}

// A rectangle puts down every cell it touches, whole: from an odd row, to an even one, and the pixel of
// a cell that is outside the rectangle comes with it, but a cell the rectangle does not touch stays
// as it was, though the canvas has pixels there.
func TestComposeRectPutsDownWholeCellsAndOnlyTheOnesItTouches(t *testing.T) {
	for _, tt := range []struct {
		name       string
		y, h       int
		first, end int // the cell rows put down, [first, end)
	}{
		{"one pixel on an even row", 2, 1, 1, 2},
		{"one pixel on an odd row", 3, 1, 1, 2},
		{"two pixels that are one cell", 2, 2, 1, 2},
		{"two pixels that are two cells", 3, 2, 1, 3},
		{"three from an even row", 2, 3, 1, 3},
		{"the whole canvas", 0, 6, 0, 3},
		{"more than the canvas", -9, 99, 0, 3},
		{"from above", -2, 4, 0, 1},
		{"to below", 5, 9, 2, 3},
	} {
		fb, c := newTestCanvas(t, 3, 3)
		for py := range 6 {
			for px := range 3 {
				c.Set(px, py, gold) // every pixel of the canvas is set
			}
		}
		c.ComposeRect(1, tt.y, 1, tt.h)
		for cy := range 3 {
			for cx := range 3 {
				touched := cx == 1 && cy >= tt.first && cy < tt.end
				if got := fb.At(cx, cy) != BlankCell; got != touched {
					t.Errorf("%s: cell (%d, %d): wanted it put down to be %v", tt.name, cx, cy, touched)
				}
			}
		}
	}

	// The pixel of a touched cell that is outside the rectangle is put down too.
	fb, c := newTestCanvas(t, 1, 1)
	c.Set(0, 0, gold)
	c.ComposeRect(0, 1, 1, 1) // the second pixel only, which is empty
	if got, want := fb.At(0, 0), (Cell{Character: upperHalfBlock, Foreground: gold}); got != want {
		t.Errorf("wanted the top pixel put down with the cell it is in, got %+v", got)
	}
}

// With Seamless a cell with a pixel is one solid color, the nearer pixel's or the only one, and what
// was under it is not kept.
func TestComposeRectSeamless(t *testing.T) {
	fb, c := panelFrame(t, 3, 1)
	c.Seamless = true
	fb.Set(0, 0, Cell{Character: upperHalfBlock, Foreground: white, Background: panelFill})
	c.Set(0, 0, gold) // top only
	c.SetDepth(1, 0, 3, RGB(1, 2, 3))
	c.SetDepth(1, 1, 2, RGB(4, 5, 6)) // the bottom is nearer
	// Cell 2 has no pixels.
	c.ComposeRect(0, 0, 3, 2)

	for x, want := range []Cell{
		{Character: ' ', Background: gold},
		{Character: ' ', Background: RGB(4, 5, 6)},
		{Character: ' ', Background: panelFill},
	} {
		if got := fb.At(x, 0); got != want {
			t.Errorf("cell %d: wanted %+v, got %+v", x, want, got)
		}
	}
}

// A cell takes the depth of its nearer pixel and a cell that is not put down keeps its own, so a
// depth-tested draw after ComposeRect is hidden by the pixels where they are nearer, as after Compose.
func TestComposeRectGivesACellItsNearerPixelsDepth(t *testing.T) {
	fb, c := newTestCanvas(t, 3, 1)
	c.SetDepth(0, 0, 3, gold)
	c.SetDepth(0, 1, 5, gold) // the nearer of the two is 3
	c.Set(1, 0, gold)         // no depth: as far as can be
	fb.SetDepth(2, 0, 7, Cell{Character: 'z'})
	c.ComposeRect(0, 0, 3, 2)

	for x, want := range []float64{3, math.Inf(1), 7} {
		if got := cellDepth(fb, x, 0); got != want {
			t.Errorf("cell %d: wanted depth %v, got %v", x, want, got)
		}
	}
	if fb.SetDepth(0, 0, 4, Cell{Character: 'a'}) || !fb.SetDepth(0, 0, 2, Cell{Character: 'a'}) {
		t.Errorf("wanted a draw at depth 4 hidden by the pixel at 3 and one at 2 to land on it")
	}
	if !fb.SetDepth(1, 0, 1e9, Cell{Character: 'b'}) {
		t.Errorf("wanted a draw to land on a pixel with no depth")
	}
}

// ComposeRect leaves the canvas as it was, so Compose, called after it, puts the whole of it down.
func TestComposeRectDoesNotChangeTheCanvas(t *testing.T) {
	_, c := newTestCanvas(t, 4, 3)
	for py := range 6 {
		c.SetDepth(py%4, py, float64(py+1), gold)
	}
	pixels, depth := slices.Clone(c.pixels), slices.Clone(c.depth)
	c.ComposeRect(1, 1, 2, 3)
	if !slices.Equal(c.pixels, pixels) || !slices.Equal(c.depth, depth) {
		t.Errorf("wanted the pixels and depths of the canvas as they were")
	}
}

// A rectangle with nothing in it, or off the canvas, changes nothing, and no position or size, the
// largest or the smallest, goes wrong: a sum past the largest int does not wrap to a place on the canvas.
func TestComposeRectAtAnyPositionAndSize(t *testing.T) {
	nothing := func(x, y, w, h int) {
		t.Helper()
		fb, c := newTestCanvas(t, 6, 3)
		for py := range 6 {
			for px := range 6 {
				c.Set(px, py, gold)
			}
		}
		c.ComposeRect(x, y, w, h)
		for _, cell := range fb.frame {
			if cell != BlankCell {
				t.Fatalf("(%d, %d) %d by %d: wanted nothing put down, got %+v", x, y, w, h, cell)
			}
		}
	}
	for _, size := range [][2]int{{0, 0}, {0, 4}, {4, 0}, {-1, 4}, {4, -1}, {math.MinInt, 4}, {4, math.MinInt}, {math.MinInt, math.MinInt}} {
		nothing(1, 1, size[0], size[1])
	}
	for _, x := range []int{6, 7, math.MaxInt, math.MaxInt - 1, -6, -7, math.MinInt, math.MinInt + 1} {
		nothing(x, 0, 6, 6)
		nothing(0, x, 6, 6)
		if x < 0 {
			nothing(x, 0, 2, 6)
		}
	}
	// Past an edge, and far off, whatever the size.
	nothing(math.MaxInt, 0, math.MaxInt, math.MaxInt)
	nothing(0, math.MaxInt, math.MaxInt, math.MaxInt)
	nothing(math.MinInt, 0, 5, 6)

	// A huge size from a place on the canvas reaches to its edge, and from before it, as far.
	all := func(x, y, w, h int) {
		t.Helper()
		a, ca := newTestCanvas(t, 6, 3)
		b, cb := newTestCanvas(t, 6, 3)
		for py := range 6 {
			for px := range 6 {
				ca.Set(px, py, gold)
				cb.Set(px, py, gold)
			}
		}
		ca.Compose()
		cb.ComposeRect(x, y, w, h)
		if !slices.Equal(a.frame, b.frame) {
			t.Fatalf("(%d, %d) %d by %d: wanted the whole canvas put down", x, y, w, h)
		}
	}
	all(0, 0, math.MaxInt, math.MaxInt)
	all(-5, -5, math.MaxInt, math.MaxInt)
	nothing(math.MinInt, math.MinInt, math.MaxInt, math.MaxInt) // it ends before the canvas begins
}

func TestComposeRectAllocatesNothing(t *testing.T) {
	_, c := newTestCanvas(t, 40, 12)
	c.FillRect(5, 3, 30, 14, gold)
	for name, f := range map[string]func(){
		"half blocks": func() { c.ComposeRect(4, 2, 32, 16) },
		"clipped":     func() { c.ComposeRect(-9, -9, 99, 99) },
		"off":         func() { c.ComposeRect(math.MaxInt, 0, 5, 5) },
	} {
		if n := testing.AllocsPerRun(50, f); n != 0 {
			t.Errorf("%s: wanted no allocation, got %v", name, n)
		}
	}
	c.Seamless = true
	if n := testing.AllocsPerRun(50, func() { c.ComposeRect(4, 2, 32, 16) }); n != 0 {
		t.Errorf("solid cells: wanted no allocation, got %v", n)
	}
}

// BenchmarkComposeRect puts a picture's rectangle down, and for a comparison with BenchmarkCompose the
// whole 200 by 60 canvas, over cells that hold something.
func BenchmarkComposeRect(b *testing.B) {
	for _, tt := range []struct {
		name       string
		x, y, w, h int
	}{{"16 by 12 pixels", 10, 9, 16, 12}, {"the whole 200 by 120", 0, 0, 200, 120}} {
		fb, err := NewFrameBuffer(200, 60)
		if err != nil {
			b.Fatal(err)
		}
		for i := range fb.frame {
			fb.frame[i] = Cell{Character: 'x', Foreground: white, Background: panelFill}
		}
		c := fb.Canvas()
		c.FillTriangle(0, 0, 2, 200, 0, 5, 0, 120, 3, Palette(33))
		c.FillTriangle(200, 120, 2, 200, 0, 5, 0, 120, 3, Palette(208))
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				c.ComposeRect(tt.x, tt.y, tt.w, tt.h)
			}
		})
	}
}

// ComposeRect never panics, and changes nothing outside the cells the rectangle touches, for any
// rectangle and a frame and canvas made from a seed.
func FuzzComposeRect(f *testing.F) {
	f.Add(1, 1, 4, 4, uint64(1))
	f.Add(-3, -3, 20, 20, uint64(2))
	f.Add(math.MaxInt, math.MinInt, math.MaxInt, math.MaxInt, uint64(3))
	f.Add(0, 0, math.MaxInt, math.MaxInt, uint64(4))
	f.Fuzz(func(t *testing.T, x, y, w, h int, seed uint64) {
		rng := rand.New(rand.NewPCG(seed, 5))
		cols, rows := 1+rng.IntN(7), 1+rng.IntN(4)
		fb, c := newTestCanvas(t, cols, rows)
		c.Seamless = rng.IntN(2) == 0
		for cy := range rows {
			for cx := range cols {
				fb.Set(cx, cy, rectCell(rng))
			}
		}
		for py := range 2 * rows {
			for px := range cols {
				if rng.IntN(2) == 0 {
					c.Set(px, py, rectColor(rng))
				}
			}
		}
		before := cellsOf(fb)
		c.ComposeRect(x, y, w, h)
		for cy := range rows {
			for cx := range cols {
				// start <= v < start+n, with the sum made as big integers, which cannot wrap.
				in := func(v, start, n int) bool {
					end := new(big.Int).Add(big.NewInt(int64(start)), big.NewInt(int64(n)))
					return n > 0 && v >= start && big.NewInt(int64(v)).Cmp(end) < 0
				}
				inX := in(cx, x, w)
				inY := in(2*cy, y, h) || in(2*cy+1, y, h)
				if !(inX && inY) && fb.At(cx, cy) != before[cy*cols+cx] {
					t.Fatalf("(%d, %d) %d by %d: cell (%d, %d) is outside it and was changed", x, y, w, h, cx, cy)
				}
			}
		}
	})
}

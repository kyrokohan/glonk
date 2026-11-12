package engine

import (
	"math"
	"math/big"
	"math/rand/v2"
	"slices"
	"testing"
)

// discAt returns whether a pixel is in the disc of radius r around (cx, cy), with the radius
// held at 2^29 as the canvas holds it. The arithmetic is plain, so it is for small numbers.
func discAt(cx, cy, r int) func(px, py int) bool {
	r = min(r, maxRadius)
	return func(px, py int) bool {
		dx, dy := int64(px-cx), int64(py-cy)
		return r >= 0 && dx*dx+dy*dy <= int64(r)*int64(r)
	}
}

// discAtBig is discAt for any ints, worked out with big integers so that nothing overflows.
func discAtBig(cx, cy, r int) func(px, py int) bool {
	r = min(r, maxRadius)
	rr := big.NewInt(int64(r))
	rr.Mul(rr, rr)
	return func(px, py int) bool {
		if r < 0 {
			return false
		}
		dx := new(big.Int).Sub(big.NewInt(int64(px)), big.NewInt(int64(cx)))
		dy := new(big.Int).Sub(big.NewInt(int64(py)), big.NewInt(int64(cy)))
		dx.Mul(dx, dx)
		dy.Mul(dy, dy)
		return dx.Add(dx, dy).Cmp(rr) <= 0
	}
}

// referenceCircle says, for every pixel of a w by h canvas in row order, whether a pixel of
// the disc in covers it, or, for an outline, one of the disc's pixels that has a neighbor
// above, below, left or right that is not in it.
func referenceCircle(w, h int, in func(px, py int) bool, outline bool) []bool {
	out := make([]bool, w*h)
	for py := range h {
		for px := range w {
			if !in(px, py) {
				continue
			}
			if outline && in(px-1, py) && in(px+1, py) && in(px, py-1) && in(px, py+1) {
				continue
			}
			out[py*w+px] = true
		}
	}
	return out
}

// drawCircle runs FillCircle, or DrawCircle for an outline.
func drawCircle(c *Canvas, cx, cy, r int, outline bool, color Color) {
	if outline {
		c.DrawCircle(cx, cy, r, color)
	} else {
		c.FillCircle(cx, cy, r, color)
	}
}

// rowsOf prints the w by h pixels of c from (x0, y0), # for a pixel and . for none.
func rowsOf(c *Canvas, x0, y0, w, h int) []string {
	rows := make([]string, h)
	for y := range h {
		row := make([]byte, w)
		for x := range w {
			row[x] = '.'
			if !c.At(x0+x, y0+y).IsDefault() {
				row[x] = '#'
			}
		}
		rows[y] = string(row)
	}
	return rows
}

// The shapes of the first circles, worked out by hand from the definition: the pixels
// whose distance from the center, squared, is at most r squared, and the ones of those
// that touch the outside along a side.
func TestCanvasSmallCircles(t *testing.T) {
	fill := [][]string{
		{"#"},
		{".#.", "###", ".#."},
		{"..#..", ".###.", "#####", ".###.", "..#.."},
		{"...#...", ".#####.", ".#####.", "#######", ".#####.", ".#####.", "...#..."},
	}
	outline := [][]string{
		{"#"},
		{".#.", "#.#", ".#."},
		{"..#..", ".#.#.", "#...#", ".#.#.", "..#.."},
		{"...#...", ".##.##.", ".#...#.", "#.....#", ".#...#.", ".##.##.", "...#..."},
	}

	c := canvasOf(t, 9, 5) // 9 by 10 pixels
	for r := range fill {
		c.Clear()
		c.FillCircle(4, 4, r, pixelRed)
		if got := rowsOf(c, 4-r, 4-r, 2*r+1, 2*r+1); !slices.Equal(got, fill[r]) {
			t.Errorf("FillCircle of radius %d: wanted\n%v\ngot\n%v", r, fill[r], got)
		}
		c.Clear()
		c.DrawCircle(4, 4, r, pixelRed)
		if got := rowsOf(c, 4-r, 4-r, 2*r+1, 2*r+1); !slices.Equal(got, outline[r]) {
			t.Errorf("DrawCircle of radius %d: wanted\n%v\ngot\n%v", r, outline[r], got)
		}
	}
}

func TestIsqrtIsTheFloorOfTheSquareRoot(t *testing.T) {
	for n := range int64(20000) {
		want := int64(0)
		for (want+1)*(want+1) <= n {
			want++
		}
		if got := isqrt(n); got != want {
			t.Fatalf("wanted isqrt(%d) = %d, got %d", n, want, got)
		}
	}
	// Around the squares of the large numbers, where a float64 cannot tell them apart.
	for _, s := range []int64{1 << 20, 1<<26 + 1, 1<<29 - 1, 1 << 29, 1<<29 + 1, 1<<30 - 1} {
		for _, d := range []int64{-1, 0, 1} {
			n := s*s + d
			want := s
			if d < 0 {
				want = s - 1
			}
			if got := isqrt(n); got != want {
				t.Errorf("wanted isqrt(%d) = %d, got %d", n, want, got)
			}
		}
	}
}

// The pixels are the ones of the reference, for every center and radius in and around a
// small canvas: wholly inside, half inside, around it, wholly outside, and empty.
func TestCanvasCirclesAreTheDiscAndItsOutline(t *testing.T) {
	const w, h = 5, 3 // 5 by 6 pixels
	c := canvasOf(t, w, h)
	ch := c.Height()

	for cx := -4; cx < w+4; cx++ {
		for cy := -4; cy < ch+4; cy++ {
			for r := -1; r < 12; r++ {
				for _, outline := range []bool{false, true} {
					c.Clear()
					drawCircle(c, cx, cy, r, outline, pixelRed)
					if got, want := drawnPixels(c), referenceCircle(w, ch, discAt(cx, cy, r), outline); !slices.Equal(got, want) {
						t.Fatalf("outline %v at (%d, %d) of radius %d: wanted %v, got %v", outline, cx, cy, r, want, got)
					}
				}
			}
		}
	}
}

// Every radius up to 40, on a canvas that holds them whole: the reference, and the eightfold
// symmetry of a circle, which the reference does not look at but a wrong row would break.
func TestCanvasCirclesOfEveryRadiusAreRoundAndSymmetric(t *testing.T) {
	const cx, cy = 45, 45
	c := canvasOf(t, 90, 45) // 90 by 90 pixels
	for r := range 41 {
		for _, outline := range []bool{false, true} {
			c.Clear()
			drawCircle(c, cx, cy, r, outline, pixelBlue)
			if got, want := drawnPixels(c), referenceCircle(90, 90, discAt(cx, cy, r), outline); !slices.Equal(got, want) {
				t.Fatalf("outline %v of radius %d: the pixels differ from the reference", outline, r)
			}
			for dx := 0; dx <= r+1; dx++ {
				for dy := 0; dy <= r+1; dy++ {
					p := !c.At(cx+dx, cy+dy).IsDefault()
					for _, q := range [][2]int{{-dx, dy}, {dx, -dy}, {-dx, -dy}, {dy, dx}} {
						if got := !c.At(cx+q[0], cy+q[1]).IsDefault(); got != p {
							t.Fatalf("outline %v of radius %d: (%d, %d) is %v but %v is %v", outline, r, dx, dy, p, q, got)
						}
					}
				}
			}
		}
	}
}

func TestCanvasCirclesAreTheReferenceOnABigCanvasAtRandom(t *testing.T) {
	const w, h = 40, 12 // 40 by 24 pixels
	c := canvasOf(t, w, h)
	ch := c.Height()
	rng := rand.New(rand.NewPCG(9, 10))
	for range 3000 {
		cx, cy := rng.IntN(121)-40, rng.IntN(101)-40
		r := rng.IntN(85) - 3
		outline := rng.IntN(2) == 0
		c.Clear()
		drawCircle(c, cx, cy, r, outline, pixelGrn)
		if got, want := drawnPixels(c), referenceCircle(w, ch, discAt(cx, cy, r), outline); !slices.Equal(got, want) {
			t.Fatalf("outline %v at (%d, %d) of radius %d: the pixels differ from the reference", outline, cx, cy, r)
		}
	}
}

// Centers and radii as large as an int goes must not wrap around onto the canvas, loop for
// long or panic, and must draw what the exact reference says.
func TestCanvasCirclesWithHugeNumbers(t *testing.T) {
	const w, h = 5, 3
	c := canvasOf(t, w, h)
	ch := c.Height()
	centers := []int{math.MinInt, math.MinInt + 1, -1, 0, 1, 2, 3, 1 << 29, 1<<29 + 1, math.MaxInt - 1, math.MaxInt}
	radii := []int{math.MinInt, -1, 0, 1, 3, 4, 100, 1<<29 - 1, 1 << 29, 1<<29 + 1, math.MaxInt}
	for _, cx := range centers {
		for _, cy := range centers {
			for _, r := range radii {
				for _, outline := range []bool{false, true} {
					c.Clear()
					drawCircle(c, cx, cy, r, outline, pixelRed)
					if got, want := drawnPixels(c), referenceCircle(w, ch, discAtBig(cx, cy, r), outline); !slices.Equal(got, want) {
						t.Fatalf("outline %v at (%d, %d) of radius %d: wanted %v, got %v", outline, cx, cy, r, want, got)
					}
				}
			}
		}
	}

	// A circle that covers the canvas fills all of it, and has no outline on it.
	c.Clear()
	c.FillCircle(2, 3, math.MaxInt, pixelRed)
	if n := countDrawn(c); n != w*ch {
		t.Errorf("wanted a huge disc to fill all %d pixels, got %d", w*ch, n)
	}
	c.Clear()
	c.DrawCircle(2, 3, math.MaxInt, pixelRed)
	if n := countDrawn(c); n != 0 {
		t.Errorf("wanted the far-away outline of a huge disc to draw nothing, got %d pixels", n)
	}
}

// countDrawn returns how many pixels of c are not empty.
func countDrawn(c *Canvas) int {
	n := 0
	for _, p := range c.pixels {
		if !p.IsDefault() {
			n++
		}
	}
	return n
}

func TestCanvasCirclesDrawTheColorGivenAndOnlyThat(t *testing.T) {
	c := canvasOf(t, 12, 6)
	c.FillCircle(6, 6, 5, pixelGrn)
	c.DrawCircle(6, 6, 5, pixelYlw)
	for y := range c.Height() {
		for x := range c.Width() {
			dx, dy := x-6, y-6
			got := c.At(x, y)
			inDisc := dx*dx+dy*dy <= 25
			onRing := inDisc && (!discAt(6, 6, 5)(x-1, y) || !discAt(6, 6, 5)(x+1, y) || !discAt(6, 6, 5)(x, y-1) || !discAt(6, 6, 5)(x, y+1))
			switch {
			case onRing && got != pixelYlw:
				t.Errorf("wanted the ring at (%d, %d) yellow, got %v", x, y, got)
			case inDisc && !onRing && got != pixelGrn:
				t.Errorf("wanted the inside at (%d, %d) green, got %v", x, y, got)
			case !inDisc && !got.IsDefault():
				t.Errorf("wanted (%d, %d), outside the disc, empty, got %v", x, y, got)
			}
		}
	}
}

func TestCanvasCirclesTheZeroColorErases(t *testing.T) {
	c := canvasOf(t, 12, 6)
	c.FillRect(0, 0, 12, 12, pixelRed)

	c.FillCircle(6, 6, 3, Color{})
	c.DrawCircle(6, 6, 5, Color{})

	if got := c.At(6, 6); !got.IsDefault() {
		t.Errorf("wanted the center erased, got %v", got)
	}
	if got := c.At(6, 1); !got.IsDefault() {
		t.Errorf("wanted the top of the ring erased, got %v", got)
	}
	if got := c.At(6, 3); !got.IsDefault() {
		t.Errorf("wanted the disc erased to its edge, got %v", got)
	}
	// Between the disc of 3 and the ring of 5, and beyond the ring: left as it was.
	for _, p := range [][2]int{{6, 2}, {9, 3}, {0, 0}, {11, 11}} {
		if got := c.At(p[0], p[1]); got != pixelRed {
			t.Errorf("wanted (%d, %d) left red, got %v", p[0], p[1], got)
		}
	}
}

func TestCanvasCirclesNeverReadOrWriteDepth(t *testing.T) {
	c := canvasOf(t, 12, 6)
	c.SetDepth(6, 6, 5, pixelRed)
	c.SetDepth(6, 3, 1, pixelRed)
	before := slices.Clone(c.depth)

	c.FillCircle(6, 6, 4, pixelBlue) // replaces the pixels, near ones too
	c.DrawCircle(6, 6, 3, pixelGrn)

	if !slices.Equal(c.depth, before) {
		t.Errorf("wanted the depths untouched")
	}
	if got := c.At(6, 6); got != pixelBlue {
		t.Errorf("wanted the pixel replaced whatever its depth, got %v", got)
	}
	if got := c.At(6, 3); got != pixelGrn {
		t.Errorf("wanted the nearer pixel replaced too, got %v", got)
	}
}

func TestCanvasCirclesAllocateNothing(t *testing.T) {
	c := canvasOf(t, 80, 24)
	if n := testing.AllocsPerRun(50, func() {
		c.FillCircle(30, 20, 25, pixelRed)
		c.DrawCircle(-5, 10, 60, pixelBlue)
		c.FillCircle(10, 10, math.MaxInt, pixelRed)
	}); n != 0 {
		t.Errorf("wanted circles to allocate nothing, got %v allocations", n)
	}
}

// Any center and radius, as large as an int goes, draws what the exact reference draws.
func FuzzCanvasCircles(f *testing.F) {
	f.Add(3, 3, 3)
	f.Add(0, 0, 5)
	f.Add(-2, 4, 4)
	f.Add(6, 5, 0)
	f.Add(3, 3, -1)
	f.Add(2, 2, 100)
	f.Add(math.MinInt, math.MinInt, math.MaxInt)
	f.Add(math.MaxInt, math.MaxInt, math.MaxInt)
	f.Add(1<<29, 0, 1<<29)
	f.Add(2, 3, 1<<29+1)
	f.Add(-(1 << 29), 3, 1<<29)

	f.Fuzz(func(t *testing.T, cx, cy, r int) {
		const w, h = 7, 3
		c := canvasOf(t, w, h)
		for _, outline := range []bool{false, true} {
			c.Clear()
			drawCircle(c, cx, cy, r, outline, pixelRed)
			if got, want := drawnPixels(c), referenceCircle(w, c.Height(), discAtBig(cx, cy, r), outline); !slices.Equal(got, want) {
				t.Fatalf("outline %v at (%d, %d) of radius %d: wanted %v, got %v", outline, cx, cy, r, want, got)
			}
		}
	})
}

func BenchmarkCanvasCircles(b *testing.B) {
	fb, err := NewFrameBuffer(200, 60)
	if err != nil {
		b.Fatal(err)
	}
	c := fb.Canvas()

	b.Run("FillCircle, radius 50", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.FillCircle(100, 60, 50, pixelRed)
		}
	})
	b.Run("FillCircle, radius 5", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.FillCircle(100, 60, 5, pixelRed)
		}
	})
	b.Run("DrawCircle, radius 50", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.DrawCircle(100, 60, 50, pixelRed)
		}
	})
	b.Run("FillCircle, over the whole canvas", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.FillCircle(100, 60, 1<<29, pixelRed)
		}
	})
	b.Run("FillCircle, far away", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.FillCircle(1_000_000, -1_000_000, 50, pixelRed)
		}
	})
}

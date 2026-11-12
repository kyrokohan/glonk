package engine

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// inSpan says whether p is one of the n values from start. The distance from start to p is
// worked out as an unsigned number, which is exact for any two ints and cannot wrap, so
// this is not the arithmetic that span uses.
func inSpan(p, start, n int) bool {
	return n > 0 && p >= start && uint64(p)-uint64(start) < uint64(n)
}

// onEdge says whether p is the first or the last of the n values from start, where n > 0.
func onEdge(p, start, n int) bool {
	d := uint64(p) - uint64(start)
	return p >= start && (d == 0 || d == uint64(n)-1)
}

// referenceRect says, for every pixel of a w by h canvas in row order, whether the
// rectangle of rw by rh pixels at (x, y) covers it, or its border does.
func referenceRect(w, h, x, y, rw, rh int, border bool) []bool {
	out := make([]bool, w*h)
	for py := range h {
		for px := range w {
			in := inSpan(px, x, rw) && inSpan(py, y, rh)
			if border {
				in = in && (onEdge(px, x, rw) || onEdge(py, y, rh))
			}
			out[py*w+px] = in
		}
	}
	return out
}

// drawRect runs FillRect, or DrawRect for a border.
func drawRect(c *Canvas, x, y, w, h int, border bool, color Color) {
	if border {
		c.DrawRect(x, y, w, h, color)
	} else {
		c.FillRect(x, y, w, h, color)
	}
}

// The pixels are the ones of the reference, for every position and size in and around a
// small canvas: wholly inside, half inside, around it, wholly outside, and empty.
func TestCanvasRectsAreTheRectangleAndItsBorder(t *testing.T) {
	const w, h = 5, 3 // 5 by 6 pixels
	c := canvasOf(t, w, h)
	ch := c.Height()

	for x := -3; x < w+3; x++ {
		for y := -3; y < ch+3; y++ {
			for rw := -1; rw < w+4; rw++ {
				for rh := -1; rh < ch+5; rh++ {
					for _, border := range []bool{false, true} {
						c.Clear()
						drawRect(c, x, y, rw, rh, border, pixelRed)
						if got, want := drawnPixels(c), referenceRect(w, ch, x, y, rw, rh, border); !slices.Equal(got, want) {
							t.Fatalf("border %v at (%d, %d) of %d by %d: wanted %v, got %v", border, x, y, rw, rh, want, got)
						}
					}
				}
			}
		}
	}
}

func TestCanvasRectsAreTheReferenceOnABigCanvasAtRandom(t *testing.T) {
	const w, h = 40, 12 // 40 by 24 pixels
	c := canvasOf(t, w, h)
	ch := c.Height()
	rng := rand.New(rand.NewPCG(7, 8))
	for range 3000 {
		x, y := rng.IntN(121)-40, rng.IntN(101)-40
		rw, rh := rng.IntN(100)-5, rng.IntN(80)-5
		border := rng.IntN(2) == 0
		c.Clear()
		drawRect(c, x, y, rw, rh, border, pixelBlue)
		if got, want := drawnPixels(c), referenceRect(w, ch, x, y, rw, rh, border); !slices.Equal(got, want) {
			t.Fatalf("border %v at (%d, %d) of %d by %d: the pixels differ from the reference", border, x, y, rw, rh)
		}
	}
}

// Positions and sizes as large as an int goes must not wrap around onto the canvas, loop
// for long, or panic, and must draw what the reference says.
func TestCanvasRectsWithHugeNumbers(t *testing.T) {
	const w, h = 5, 3
	c := canvasOf(t, w, h)
	ch := c.Height()
	big := []int{math.MinInt, math.MinInt + 1, -1, 0, 1, 3, math.MaxInt - 1, math.MaxInt}
	for _, x := range big {
		for _, y := range big {
			for _, rw := range big {
				for _, rh := range big {
					for _, border := range []bool{false, true} {
						c.Clear()
						drawRect(c, x, y, rw, rh, border, pixelRed)
						if got, want := drawnPixels(c), referenceRect(w, ch, x, y, rw, rh, border); !slices.Equal(got, want) {
							t.Fatalf("border %v at (%d, %d) of %d by %d: wanted %v, got %v", border, x, y, rw, rh, want, got)
						}
					}
				}
			}
		}
	}

	// A rectangle that starts a little in and goes on as far as an int does: adding the
	// size to the position plainly would wrap around to a negative number.
	c.Clear()
	c.FillRect(2, 1, math.MaxInt, math.MaxInt, pixelRed)
	if c.At(2, 1) != pixelRed || c.At(4, 5) != pixelRed || c.At(1, 1) == pixelRed || c.At(2, 0) == pixelRed {
		t.Errorf("wanted the rectangle to cover from (2, 1) to the far corner of the canvas")
	}
}

func TestCanvasRectsDrawTheColorGivenAndOnlyThat(t *testing.T) {
	c := canvasOf(t, 6, 3)
	c.FillRect(1, 1, 3, 4, pixelGrn)
	c.DrawRect(0, 0, 6, 6, pixelYlw)
	for y := range c.Height() {
		for x := range c.Width() {
			switch got := c.At(x, y); {
			case x == 0 || y == 0 || x == 5 || y == 5:
				if got != pixelYlw {
					t.Errorf("wanted the border at (%d, %d) yellow, got %v", x, y, got)
				}
			case x >= 1 && x < 4 && y >= 1 && y < 5:
				if got != pixelGrn {
					t.Errorf("wanted the inside at (%d, %d) green, got %v", x, y, got)
				}
			default:
				if !got.IsDefault() {
					t.Errorf("wanted (%d, %d) empty, got %v", x, y, got)
				}
			}
		}
	}
}

func TestCanvasRectsTheZeroColorErases(t *testing.T) {
	c := canvasOf(t, 6, 3)
	c.FillRect(-5, -5, 100, 100, pixelRed)

	c.FillRect(1, 1, 2, 2, Color{})
	c.DrawRect(3, 2, 3, 4, Color{})

	for _, p := range [][2]int{{1, 1}, {2, 2}, {3, 2}, {5, 5}, {4, 2}} {
		if got := c.At(p[0], p[1]); !got.IsDefault() {
			t.Errorf("wanted (%d, %d) erased, got %v", p[0], p[1], got)
		}
	}
	if got := c.At(0, 0); got != pixelRed {
		t.Errorf("wanted (0, 0) left as it was, got %v", got)
	}
	// The inside of the border is not part of it.
	if got := c.At(4, 3); got != pixelRed {
		t.Errorf("wanted the inside of the erased border left red, got %v", got)
	}
}

func TestCanvasRectsNeverReadOrWriteDepth(t *testing.T) {
	c := canvasOf(t, 6, 3)
	c.SetDepth(2, 2, 5, pixelRed)
	c.SetDepth(4, 4, 1, pixelRed)
	before := slices.Clone(c.depth)

	c.FillRect(0, 0, 6, 6, pixelBlue) // replaces the pixels, near ones too
	c.DrawRect(1, 1, 4, 4, pixelGrn)

	if !slices.Equal(c.depth, before) {
		t.Errorf("wanted the depths untouched")
	}
	if got := c.At(2, 2); got != pixelBlue {
		t.Errorf("wanted the pixel replaced whatever its depth, got %v", got)
	}
	if got := c.At(4, 4); got != pixelGrn {
		t.Errorf("wanted the nearer pixel replaced too, got %v", got)
	}
}

func TestCanvasRectsAllocateNothing(t *testing.T) {
	c := canvasOf(t, 80, 24)
	if n := testing.AllocsPerRun(50, func() {
		c.FillRect(-5, 3, 60, 20, pixelRed)
		c.DrawRect(2, -4, 200, 100, pixelBlue)
	}); n != 0 {
		t.Errorf("wanted rectangles to allocate nothing, got %v allocations", n)
	}
}

// Any position and size, as large as an int goes, draws what the reference draws.
func FuzzCanvasRects(f *testing.F) {
	f.Add(0, 0, 7, 6)
	f.Add(-2, -1, 4, 3)
	f.Add(6, 5, 1, 1)
	f.Add(3, 0, 2, 100)
	f.Add(-100, -100, 300, 300)
	f.Add(0, 0, 0, 5)
	f.Add(math.MinInt, math.MinInt, math.MaxInt, math.MaxInt)
	f.Add(math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt)
	f.Add(2, 1, math.MaxInt, math.MaxInt)

	f.Fuzz(func(t *testing.T, x, y, rw, rh int) {
		const w, h = 7, 3
		c := canvasOf(t, w, h)
		for _, border := range []bool{false, true} {
			c.Clear()
			drawRect(c, x, y, rw, rh, border, pixelRed)
			if got, want := drawnPixels(c), referenceRect(w, c.Height(), x, y, rw, rh, border); !slices.Equal(got, want) {
				t.Fatalf("border %v at (%d, %d) of %d by %d: wanted %v, got %v", border, x, y, rw, rh, want, got)
			}
		}
	})
}

func BenchmarkCanvasRects(b *testing.B) {
	fb, err := NewFrameBuffer(200, 60)
	if err != nil {
		b.Fatal(err)
	}
	c := fb.Canvas()

	b.Run("FillRect, the whole canvas", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.FillRect(0, 0, 200, 120, pixelRed)
		}
	})
	b.Run("FillRect, a small one", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.FillRect(90, 50, 20, 10, pixelRed)
		}
	})
	b.Run("DrawRect", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.DrawRect(5, 5, 190, 110, pixelRed)
		}
	})
	b.Run("FillRect, past every edge", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.FillRect(-1_000_000, -1_000_000, 2_000_000, 2_000_000, pixelRed)
		}
	})
}

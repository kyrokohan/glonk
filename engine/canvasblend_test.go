package engine

import (
	"math"
	"slices"
	"testing"
)

// fillVaried gives the pixels of c a pattern of every kind of color there is: empty, RGB at
// every channel level, and palette colors, so that no two neighbors are alike.
func fillVaried(c *Canvas) {
	for i := range c.pixels {
		switch i % 4 {
		case 0:
			c.pixels[i] = Color{}
		case 1, 3:
			c.pixels[i] = RGB(uint8(i*37), uint8(i*91), uint8(i*13))
		default:
			c.pixels[i] = Palette(uint8(i * 7))
		}
	}
}

// referenceBlend returns the pixels the canvas should hold after BlendRect: Mix of each pixel
// that is in the rectangle and not empty, and the pixel as it was for every other. Whether a
// pixel is in the rectangle is worked out by inSpan, not by the arithmetic span uses.
func referenceBlend(before []Color, w, x, y, rw, rh int, color Color, t float64) []Color {
	out := slices.Clone(before)
	for i, p := range before {
		if !p.IsDefault() && inSpan(i%w, x, rw) && inSpan(i/w, y, rh) {
			out[i] = Mix(p, color, t)
		}
	}
	return out
}

// For every place and size in and around a small canvas, and the parts that matter (the
// ends, between, outside 0 to 1, NaN), every pixel is the one of the reference.
func TestCanvasBlendIsMixOverTheRectangleAtEveryPlaceAndSize(t *testing.T) {
	const w, h = 5, 3 // 5 by 6 pixels
	c := canvasOf(t, w, h)
	ch := c.Height()
	parts := []float64{0, 0.25, 0.5, 1, 2, -1, math.NaN()}

	for x := -3; x < w+3; x++ {
		for y := -3; y < ch+3; y++ {
			for rw := -1; rw < w+4; rw++ {
				for rh := -1; rh < ch+5; rh++ {
					for _, part := range parts {
						fillVaried(c)
						before := slices.Clone(c.pixels)
						c.BlendRect(x, y, rw, rh, pixelBlue, part)
						if want := referenceBlend(before, w, x, y, rw, rh, pixelBlue, part); !slices.Equal(c.pixels, want) {
							t.Fatalf("at (%d, %d) of %d by %d with part %v: wanted %v, got %v", x, y, rw, rh, part, want, c.pixels)
						}
					}
				}
			}
		}
	}
}

func TestCanvasBlendWithHugeNumbers(t *testing.T) {
	const w, h = 5, 3
	c := canvasOf(t, w, h)
	big := []int{math.MinInt, math.MinInt + 1, -1, 0, 1, 3, math.MaxInt - 1, math.MaxInt}
	for _, x := range big {
		for _, y := range big {
			for _, rw := range big {
				for _, rh := range big {
					fillVaried(c)
					before := slices.Clone(c.pixels)
					c.BlendRect(x, y, rw, rh, pixelRed, 0.5)
					if want := referenceBlend(before, w, x, y, rw, rh, pixelRed, 0.5); !slices.Equal(c.pixels, want) {
						t.Fatalf("at (%d, %d) of %d by %d: wanted %v, got %v", x, y, rw, rh, want, c.pixels)
					}
				}
			}
		}
	}

	// A rectangle that starts a little in and goes on as far as an int does: adding the size
	// to the position plainly would wrap around to a negative number.
	c.Clear()
	c.FillRect(0, 0, w, c.Height(), pixelYlw)
	c.BlendRect(2, 1, math.MaxInt, math.MaxInt, pixelBlue, 1)
	if c.At(2, 1) != pixelBlue || c.At(4, 5) != pixelBlue || c.At(1, 1) != pixelYlw || c.At(2, 0) != pixelYlw {
		t.Errorf("wanted the blend to cover from (2, 1) to the far corner of the canvas")
	}
}

func TestCanvasBlendEndsAreTheOldColorAndTheNewOne(t *testing.T) {
	c := canvasOf(t, 4, 2)
	c.FillRect(0, 0, 4, 4, pixelRed)

	c.BlendRect(0, 0, 4, 4, pixelBlue, 0)
	if got := c.At(1, 1); got != pixelRed {
		t.Errorf("wanted a part of 0 to keep the color, got %v", got)
	}
	c.BlendRect(0, 0, 4, 4, pixelBlue, 1)
	if got := c.At(1, 1); got != pixelBlue {
		t.Errorf("wanted a part of 1 to give the color, got %v", got)
	}

	c.FillRect(0, 0, 4, 4, RGB(200, 0, 0))
	c.BlendRect(0, 0, 4, 4, RGB(0, 0, 200), 0.25)
	if got, want := c.At(2, 2), RGB(150, 0, 50); got != want {
		t.Errorf("wanted a quarter of the way to be %v, got %v", want, got)
	}
}

func TestCanvasBlendLeavesAnEmptyPixelEmpty(t *testing.T) {
	c := canvasOf(t, 4, 2)
	c.Set(1, 1, pixelRed)
	c.Set(2, 3, pixelGrn)
	c.BlendRect(0, 0, 4, 4, pixelBlue, 1)
	for y := range c.Height() {
		for x := range c.Width() {
			switch p := c.At(x, y); {
			case x == 1 && y == 1, x == 2 && y == 3:
				if p != pixelBlue {
					t.Errorf("pixel (%d, %d): wanted the color, got %v", x, y, p)
				}
			case !p.IsDefault():
				t.Errorf("pixel (%d, %d): wanted an empty pixel to stay empty, got %v", x, y, p)
			}
		}
	}
}

func TestCanvasBlendWithTheZeroColorGoesTowardBlackAndDoesNotErase(t *testing.T) {
	c := canvasOf(t, 2, 1)
	c.Set(0, 0, RGB(100, 100, 100))
	c.BlendRect(0, 0, 2, 2, Color{}, 0.5)
	if got, want := c.At(0, 0), RGB(50, 50, 50); got != want {
		t.Errorf("wanted the pixel half way to black, %v, got %v", want, got)
	}
	c.BlendRect(0, 0, 2, 2, Color{}, 1)
	if got := c.At(0, 0); got.IsDefault() || got != RGB(0, 0, 0) {
		t.Errorf("wanted black, which is a color and not an erased pixel, got %v", got)
	}
}

func TestCanvasBlendMakesAPalettePixelAnExactRGB(t *testing.T) {
	r, g, b := Palette(208).RGB()
	c := canvasOf(t, 2, 1)
	c.Set(0, 0, Palette(208))
	c.BlendRect(0, 0, 2, 2, pixelBlue, 0)
	if got, want := c.At(0, 0), RGB(r, g, b); got != want {
		t.Errorf("wanted palette color 208 as the RGB %v even for a part of 0, got %v", want, got)
	}

	// And a palette color given as the color is read as xterm defines it too.
	c.BlendRect(0, 0, 2, 2, Palette(196), 1)
	if got, want := c.At(0, 0), RGB(255, 0, 0); got != want {
		t.Errorf("wanted palette color 196 as the RGB %v, got %v", want, got)
	}
}

func TestCanvasBlendNeverReadsOrWritesDepth(t *testing.T) {
	c := canvasOf(t, 6, 3)
	c.SetDepth(2, 2, 5, pixelRed)
	c.SetDepth(4, 4, 1, pixelRed)
	before := slices.Clone(c.depth)

	c.BlendRect(0, 0, 6, 6, pixelBlue, 1) // changes the pixels, near ones too

	if !slices.Equal(c.depth, before) {
		t.Errorf("wanted the depths untouched")
	}
	if c.At(2, 2) != pixelBlue || c.At(4, 4) != pixelBlue {
		t.Errorf("wanted the pixels blended whatever their depth, got %v and %v", c.At(2, 2), c.At(4, 4))
	}
}

func TestCanvasBlendAllocatesNothing(t *testing.T) {
	c := canvasOf(t, 80, 24)
	fillVaried(c)
	if n := testing.AllocsPerRun(50, func() {
		c.BlendRect(-5, 3, 60, 20, pixelRed, 0.5)
		c.BlendRect(0, 0, 80, 48, pixelBlue, 0.1)
	}); n != 0 {
		t.Errorf("wanted a blend to allocate nothing, got %v allocations", n)
	}
}

// Any position, size and part, as large as an int goes, gives what the reference gives.
func FuzzCanvasBlend(f *testing.F) {
	f.Add(0, 0, 7, 6, 0.5)
	f.Add(-2, -1, 4, 3, 0.25)
	f.Add(6, 5, 1, 1, 1.0)
	f.Add(3, 0, 2, 100, 0.0)
	f.Add(-100, -100, 300, 300, 2.0)
	f.Add(0, 0, 0, 5, 0.5)
	f.Add(math.MinInt, math.MinInt, math.MaxInt, math.MaxInt, 0.5)
	f.Add(math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt, 0.5)
	f.Add(2, 1, math.MaxInt, math.MaxInt, math.NaN())
	f.Add(0, 0, 7, 6, math.Inf(-1))

	f.Fuzz(func(t *testing.T, x, y, rw, rh int, part float64) {
		const w, h = 7, 3
		c := canvasOf(t, w, h)
		fillVaried(c)
		before := slices.Clone(c.pixels)
		c.BlendRect(x, y, rw, rh, pixelGrn, part)
		if want := referenceBlend(before, w, x, y, rw, rh, pixelGrn, part); !slices.Equal(c.pixels, want) {
			t.Fatalf("at (%d, %d) of %d by %d with part %v: wanted %v, got %v", x, y, rw, rh, part, want, c.pixels)
		}
	})
}

func BenchmarkCanvasBlend(b *testing.B) {
	fb, err := NewFrameBuffer(200, 60)
	if err != nil {
		b.Fatal(err)
	}
	c := fb.Canvas()

	b.Run("the whole canvas, flat", func(b *testing.B) {
		c.FillRect(0, 0, 200, 120, pixelRed)
		b.ReportAllocs()
		for b.Loop() {
			c.BlendRect(0, 0, 200, 120, pixelBlue, 0.5)
		}
	})
	b.Run("the whole canvas, varied", func(b *testing.B) {
		fillVaried(c)
		b.ReportAllocs()
		for b.Loop() {
			c.BlendRect(0, 0, 200, 120, pixelBlue, 0.5)
		}
	})
	b.Run("a small rectangle", func(b *testing.B) {
		fillVaried(c)
		b.ReportAllocs()
		for b.Loop() {
			c.BlendRect(90, 50, 20, 10, pixelBlue, 0.5)
		}
	})
	b.Run("past every edge", func(b *testing.B) {
		fillVaried(c)
		b.ReportAllocs()
		for b.Loop() {
			c.BlendRect(-1_000_000, -1_000_000, 2_000_000, 2_000_000, pixelBlue, 0.5)
		}
	})
}

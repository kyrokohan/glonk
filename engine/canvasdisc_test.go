package engine

import (
	"hash/fnv"
	"math"
	"slices"
	"strconv"
	"testing"
	"time"
)

// refShare is the share of pixel (px, py) of the disc around (a/16, b/16) of radius c/16, counted
// with integers apart from the code under test: the samples are at (16px+2k+1)/16, so a sample is
// within the disc when (16px+2k+1-a)² + (16py+2l+1-b)² <= c².
func refShare(px, py, a, b, c int) int {
	n := 0
	for l := range discSamples {
		for k := range discSamples {
			dx, dy := int64(16*px+2*k+1-a), int64(16*py+2*l+1-b)
			if dx*dx+dy*dy <= int64(c)*int64(c) {
				n++
			}
		}
	}
	return n
}

// The pixels are what the rules say for the share the integer reference counts, for discs centered
// all over and around a small canvas, of radii from a sixteenth of a pixel to past its size: FillDisc
// on a background and on nothing, and BlendDisc at several parts.
func TestCanvasDiscsAreTheReference(t *testing.T) {
	const w, h = 12, 6 // 12 by 12 pixels
	bg, color := RGB(20, 120, 220), RGB(240, 200, 40)
	c := canvasOf(t, w, h)
	ch := c.Height()

	for a := -48; a < 16*w+48; a += 7 {
		for b := -48; b < 16*ch+48; b += 7 {
			for _, r := range []int{1, 8, 16, 23, 40, 81, 200} {
				cx, cy, cr := float64(a)/16, float64(b)/16, float64(r)/16
				share := func(px, py int) int { return refShare(px, py, a, b, r) }

				// FillDisc over a background: whole pixels are the color, the edge is mixed.
				c.FillRect(0, 0, w, ch, bg)
				c.FillDisc(cx, cy, cr, color)
				for py := range ch {
					for px := range w {
						want := bg
						switch n := share(px, py); {
						case n == 64:
							want = color
						case n > 0:
							want = Mix(bg, color, float64(n)/64)
						}
						if got := c.At(px, py); got != want {
							t.Fatalf("FillDisc(%v, %v, %v) on a background, pixel (%d, %d): wanted %v, got %v", cx, cy, cr, px, py, want, got)
						}
					}
				}

				// FillDisc on nothing: an edge pixel takes the color from half covered.
				c.Clear()
				c.FillDisc(cx, cy, cr, color)
				for py := range ch {
					for px := range w {
						want := Color{}
						if n := share(px, py); 2*n >= 64 {
							want = color
						}
						if got := c.At(px, py); got != want {
							t.Fatalf("FillDisc(%v, %v, %v) on nothing, pixel (%d, %d): wanted %v, got %v", cx, cy, cr, px, py, want, got)
						}
					}
				}

				// BlendDisc over a background, by a part.
				for _, part := range []float64{0, 0.3, 1} {
					c.FillRect(0, 0, w, ch, bg)
					c.BlendDisc(cx, cy, cr, color, part)
					for py := range ch {
						for px := range w {
							want := bg
							if n := share(px, py); n > 0 {
								want = Mix(bg, color, float64(n)/64*part)
							}
							if got := c.At(px, py); got != want {
								t.Fatalf("BlendDisc(%v, %v, %v, %v), pixel (%d, %d): wanted %v, got %v", cx, cy, cr, part, px, py, want, got)
							}
						}
					}
				}
			}
		}
	}
}

// pixelHash is a hash of every pixel of the canvas: whether it is empty, and its color.
func pixelHash(c *Canvas) uint64 {
	h := fnv.New64a()
	for _, p := range c.pixels {
		r, g, b := p.RGB()
		empty := byte(0)
		if p.IsDefault() {
			empty = 1
		}
		h.Write([]byte{empty, byte(p.kind()), r, g, b})
	}
	return h.Sum64()
}

// A scene of discs at places and sizes that are not exact in binary draws the same pixels on every
// platform: the hash was taken on arm64, and CI checks it on amd64 too.
func TestCanvasDiscsAreTheSameOnEveryPlatform(t *testing.T) {
	c := canvasOf(t, 40, 15)
	c.FillRect(0, 0, 40, 20, RGB(20, 30, 40)) // the top two thirds have a background, the rest is empty
	c.FillDisc(10.3, 7.7, 3.3, RGB(240, 200, 40))
	c.FillDisc(21.1, 13.9, 6.7, RGB(200, 60, 90))
	c.BlendDisc(11.3, 8.9, 3.3, RGB(0, 0, 0), 0.37)
	c.FillDisc(30.05, 24.6, 4.15, Palette(45))
	c.BlendDisc(33.33, 3.33, 2.22, RGB(255, 255, 255), 0.71)
	// A sample of pixel (3, 2) of this disc is within it when each square is rounded on its own,
	// and outside it when arm64 fuses a square into the sum: found by a search, it keeps the
	// explicit roundings honest on arm64.
	c.FillDisc(0.30000000000000049, 0.7, 3.6090597806076854, RGB(255, 255, 255))
	const want = 0x748cb5a54edd485b
	if got := pixelHash(c); got != want {
		t.Errorf("wanted the pixels that hash to %#x, got %#x", uint64(want), got)
	}
}

// Mix rounds the same way on every platform: these mixes, found next to a rounding tie, were one
// less on arm64, which fused the multiply and the add, than on amd64.
func TestMixIsTheSameOnEveryPlatform(t *testing.T) {
	for _, tt := range []struct {
		from, to uint8
		t        float64
		want     uint8
	}{
		{1, 8, 0.071428571428571411, 2},
		{1, 8, 0.21428571428571425, 3},
		{1, 11, 0.74999999999999989, 9},
	} {
		if got, _, _ := Mix(RGB(tt.from, 0, 0), RGB(tt.to, 0, 0), tt.t).RGB(); got != tt.want {
			t.Errorf("Mix from %d to %d at %v: wanted %d, got %d", tt.from, tt.to, tt.t, tt.want, got)
		}
	}
}

// What draws nothing draws nothing, an infinite radius covers the canvas, and numbers as large as a
// float goes neither panic nor take long.
func TestCanvasDiscsWithOddNumbers(t *testing.T) {
	const w, h = 8, 4
	c := canvasOf(t, w, h)
	inf, nan := math.Inf(1), math.NaN()
	red := RGB(220, 40, 40)

	for _, d := range [][3]float64{
		{4, 4, 0}, {4, 4, -1}, {4, 4, nan}, {4, 4, -inf},
		{nan, 4, 2}, {4, nan, 2}, {inf, 4, 2}, {4, -inf, 2}, {inf, inf, inf},
	} {
		c.Clear()
		c.FillDisc(d[0], d[1], d[2], red)
		c.BlendDisc(d[0], d[1], d[2], red, 1)
		if slices.ContainsFunc(c.pixels, func(p Color) bool { return !p.IsDefault() }) {
			t.Errorf("disc (%v, %v) of %v: wanted nothing drawn", d[0], d[1], d[2])
		}
	}

	c.Clear()
	c.FillDisc(-1e300, 1e300, inf, red)
	if slices.ContainsFunc(c.pixels, func(p Color) bool { return p != red }) {
		t.Errorf("wanted an infinite radius to cover the canvas")
	}

	start := time.Now()
	for _, v := range []float64{math.MaxFloat64, -math.MaxFloat64, 1e300, -1e300, 1e15, 0.5} {
		for _, r := range []float64{math.MaxFloat64, 1e300, 1e15, 3} {
			c.FillDisc(v, v, r, red)
			c.FillDisc(v, -v, r, red)
			c.BlendDisc(-v, v, r, red, 0.5)
		}
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("wanted huge discs drawn quickly, took %v", d)
	}

	// A part outside 0 to 1 is clamped, and NaN is 0.
	for _, tt := range []struct{ part, as float64 }{{-1, 0}, {2, 1}, {nan, 0}} {
		a, b := canvasOf(t, w, h), canvasOf(t, w, h)
		a.FillRect(0, 0, w, 2*h, RGB(20, 30, 40))
		b.FillRect(0, 0, w, 2*h, RGB(20, 30, 40))
		a.BlendDisc(4.2, 3.9, 2.6, red, tt.part)
		b.BlendDisc(4.2, 3.9, 2.6, red, tt.as)
		if !slices.Equal(a.pixels, b.pixels) {
			t.Errorf("wanted a part of %v to blend as %v", tt.part, tt.as)
		}
	}
}

// A disc moved by an eighth of a pixel is drawn differently, and one around the middle of a pixel is
// the same mirrored across and down.
func TestCanvasDiscsMoveSmoothlyAndAreSymmetric(t *testing.T) {
	const w, h = 21, 11 // 21 by 22 pixels
	draw := func(cx, cy float64) []Color {
		c := canvasOf(t, w, h)
		c.FillRect(0, 0, w, 2*h, RGB(10, 10, 10))
		c.FillDisc(cx, cy, 4.3, RGB(250, 250, 250))
		return c.pixels
	}
	before := draw(10.5, 10.5)
	if slices.Equal(before, draw(10.625, 10.5)) || slices.Equal(before, draw(10.5, 10.625)) {
		t.Errorf("wanted a disc moved by an eighth of a pixel drawn differently")
	}
	for y := range 21 { // the rows 0 to 20, which mirror each other around 10.5
		for x := range w {
			p := before[y*w+x]
			if mx, my := before[y*w+(w-1-x)], before[(20-y)*w+x]; p != mx || p != my {
				t.Fatalf("wanted the disc around (10.5, 10.5) symmetric at (%d, %d): %v, mirrored %v and %v", x, y, p, mx, my)
			}
		}
	}
}

func TestCanvasDiscsNeverReadOrWriteDepth(t *testing.T) {
	c := canvasOf(t, 10, 5)
	c.FillRect(0, 0, 10, 10, RGB(20, 30, 40))
	c.SetDepth(5, 5, 2, pixelRed)
	before := slices.Clone(c.depth)
	c.FillDisc(5.5, 5.5, 3.2, pixelBlue)
	c.BlendDisc(4.5, 4.5, 3.2, pixelGrn, 0.5)
	if !slices.Equal(c.depth, before) {
		t.Errorf("wanted the depths untouched")
	}
}

func TestCanvasDiscsAllocateNothing(t *testing.T) {
	c := canvasOf(t, 80, 24)
	if n := testing.AllocsPerRun(50, func() {
		c.FillDisc(30.3, 20.7, 9.4, pixelRed)
		c.BlendDisc(31.3, 21.7, 9.4, pixelBlue, 0.4)
	}); n != 0 {
		t.Errorf("wanted discs to allocate nothing, got %v allocations", n)
	}
}

func BenchmarkCanvasDiscs(b *testing.B) {
	fb, err := NewFrameBuffer(200, 60)
	if err != nil {
		b.Fatal(err)
	}
	c := fb.Canvas()
	c.FillRect(0, 0, 200, 120, RGB(20, 30, 40))
	for _, r := range []int{2, 10, 50} {
		b.Run("FillCircle r"+strconv.Itoa(r), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				c.FillCircle(100, 60, r, pixelRed)
			}
		})
		b.Run("FillDisc r"+strconv.Itoa(r)+".5", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				c.FillDisc(100.3, 60.6, float64(r)+0.5, pixelRed)
			}
		})
		b.Run("BlendDisc r"+strconv.Itoa(r)+".5", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				c.BlendDisc(100.3, 60.6, float64(r)+0.5, pixelBlue, 0.4)
			}
		})
	}
}

package engine

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// weighted is a corner-weight triple as FillTriangleFunc hands it out.
type weighted struct{ b0, b1, b2 float64 }

// randomTriangle returns corners in cell coordinates around a w x h grid, with
// depths from 0.5 to 20, and says whether it is big enough to have a stable shape.
func randomTriangle(rng *rand.Rand, w, h int) (c [9]float64, ok bool) {
	for i := range 3 {
		c[3*i] = rng.Float64()*float64(w+6) - 3
		c[3*i+1] = rng.Float64()*float64(h+6) - 3
		c[3*i+2] = 0.5 + rng.Float64()*19.5
	}
	area := (c[3]-c[0])*(c[7]-c[1]) - (c[6]-c[0])*(c[4]-c[1])
	return c, math.Abs(area) >= 1
}

func newFB(t *testing.T, w, h int) *FrameBuffer {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "")
	fb, err := NewFrameBuffer(w, h)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

// The function version covers the cells the plain one does, with the same depths, on
// top of whatever was already drawn.
func TestFillTriangleFuncCoversTheSameCellsAsFillTriangle(t *testing.T) {
	const w, h = 9, 6
	mark := Cell{Character: '#'}
	rng := rand.New(rand.NewPCG(7, 8))

	for range 400 {
		under, _ := randomTriangle(rng, w, h)
		c, _ := randomTriangle(rng, w, h)

		plain, fn := newFB(t, w, h), newFB(t, w, h)
		for _, fb := range []*FrameBuffer{plain, fn} {
			fb.FillTriangle(under[0], under[1], under[2], under[3], under[4], under[5], under[6], under[7], under[8], Cell{Character: '.'})
		}
		plain.FillTriangle(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], mark)
		fn.FillTriangleFunc(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(_, _, _ float64) Cell { return mark })

		if !slices.Equal(plain.frame, fn.frame) || !slices.Equal(plain.depth, fn.depth) {
			t.Fatalf("triangle %v over %v: wanted the same cells and depths as FillTriangle\n%s\nwanted\n%s", c, under, fn.String(), plain.String())
		}
	}
}

func TestCanvasFillTriangleFuncCoversTheSamePixelsAsFillTriangle(t *testing.T) {
	const w, h = 9, 4 // 9x8 pixels
	rng := rand.New(rand.NewPCG(9, 10))
	paint := RGB(1, 2, 3)

	for range 400 {
		under, _ := randomTriangle(rng, w, 2*h)
		c, _ := randomTriangle(rng, w, 2*h)

		_, plain := newTestCanvas(t, w, h)
		_, fn := newTestCanvas(t, w, h)
		for _, cv := range []*Canvas{plain, fn} {
			cv.FillTriangle(under[0], under[1], under[2], under[3], under[4], under[5], under[6], under[7], under[8], Palette(9))
		}
		plain.FillTriangle(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], paint)
		fn.FillTriangleFunc(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(_, _, _ float64) Color { return paint })

		if !slices.Equal(plain.pixels, fn.pixels) || !slices.Equal(plain.depth, fn.depth) {
			t.Fatalf("triangle %v over %v: wanted the same pixels and depths as FillTriangle", c, under)
		}
	}
}

// With every corner equally far the weights are the plain barycentric ones. These are
// worked out by hand for the middle of the top-left cell, (0.5, 0.5), of the triangle
// (0,0) (4,0) (0,4): the corners get 1 - 1/8 - 1/8, 1/8 and 1/8.
func TestFillTriangleFuncWeightsAtADistanceAreBarycentric(t *testing.T) {
	fb := newFB(t, 4, 4)
	var first *weighted

	fb.FillTriangleFunc(0, 0, 1, 4, 0, 1, 0, 4, 1, func(b0, b1, b2 float64) Cell {
		if first == nil {
			first = &weighted{b0, b1, b2}
		}
		return Cell{Character: '#'}
	})

	if first == nil {
		t.Fatal("wanted the function called, it was not")
	}
	if want := (weighted{0.75, 0.125, 0.125}); !closeWeights(*first, want, 1e-12) {
		t.Errorf("wanted the first cell's weights %v, got %v", want, *first)
	}
}

func closeWeights(a, b weighted, tol float64) bool {
	return math.Abs(a.b0-b.b0) <= tol && math.Abs(a.b1-b.b1) <= tol && math.Abs(a.b2-b.b2) <= tol
}

// The weights are perspective-correct: blending the corners' places in the scene with
// them gives the point in the scene that the cell shows, which projects back to the
// middle of the cell and is as far away as the depth buffer says. Blending the
// screen-space weights, which a plain fill would use, fails this for any triangle that
// slopes away from the screen.
func TestFillTriangleFuncWeightsAreFromTheScene(t *testing.T) {
	const w, h = 12, 8
	rng := rand.New(rand.NewPCG(11, 12))
	covered := 0

	for range 300 {
		// A scene point (X, Y, Z) is drawn at (X/Z, Y/Z), so a corner drawn at (x, y)
		// at depth z is the scene point (x*z, y*z, z).
		c, ok := randomTriangle(rng, w, h)
		if !ok {
			continue
		}
		var scene [3][3]float64
		for i := range 3 {
			x, y, z := c[3*i], c[3*i+1], c[3*i+2]
			scene[i] = [3]float64{x * z, y * z, z}
		}

		fb := newFB(t, w, h)
		var seen []weighted
		fb.FillTriangleFunc(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(b0, b1, b2 float64) Cell {
			seen = append(seen, weighted{b0, b1, b2})
			return Cell{Character: rune(0x100 + len(seen) - 1)} // which call made this cell
		})

		for y := range h {
			for x := range w {
				ch := fb.At(x, y).Character
				if ch < 0x100 {
					continue
				}
				covered++
				b := seen[ch-0x100]

				if sum := b.b0 + b.b1 + b.b2; math.Abs(sum-1) > 1e-9 {
					t.Fatalf("triangle %v, cell (%d,%d): wanted weights summing to 1, got %v (sum %v)", c, x, y, b, sum)
				}
				var p [3]float64
				for k := range 3 {
					p[k] = b.b0*scene[0][k] + b.b1*scene[1][k] + b.b2*scene[2][k]
				}
				if math.Abs(p[0]/p[2]-(float64(x)+0.5)) > 1e-8 || math.Abs(p[1]/p[2]-(float64(y)+0.5)) > 1e-8 {
					t.Fatalf("triangle %v, cell (%d,%d): wanted the scene point to project to the cell's middle, got (%v,%v)", c, x, y, p[0]/p[2], p[1]/p[2])
				}
				if z := fb.depth[y*w+x]; math.Abs(p[2]-z) > 1e-8*z {
					t.Fatalf("triangle %v, cell (%d,%d): wanted the scene point at the stored depth %v, got %v", c, x, y, z, p[2])
				}
			}
		}
	}
	if covered < 500 {
		t.Fatalf("wanted the random triangles to cover plenty of cells, covered only %d", covered)
	}
}

func TestCanvasFillTriangleFuncWeightsAreFromTheScene(t *testing.T) {
	const w, h = 12, 4 // 12x8 pixels
	rng := rand.New(rand.NewPCG(13, 14))
	covered := 0

	for range 300 {
		c, ok := randomTriangle(rng, w, 2*h)
		if !ok {
			continue
		}
		var scene [3][3]float64
		for i := range 3 {
			x, y, z := c[3*i], c[3*i+1], c[3*i+2]
			scene[i] = [3]float64{x * z, y * z, z}
		}

		_, cv := newTestCanvas(t, w, h)
		var seen []weighted
		cv.FillTriangleFunc(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(b0, b1, b2 float64) Color {
			seen = append(seen, weighted{b0, b1, b2})
			n := len(seen) - 1
			return RGB(uint8(n>>8), uint8(n), 1) // which call made this pixel
		})

		for y := range cv.Height() {
			for x := range w {
				col := cv.At(x, y)
				if col.IsDefault() {
					continue
				}
				covered++
				r, g, _ := col.RGB()
				b := seen[int(r)<<8|int(g)]

				if sum := b.b0 + b.b1 + b.b2; math.Abs(sum-1) > 1e-9 {
					t.Fatalf("triangle %v, pixel (%d,%d): wanted weights summing to 1, got %v", c, x, y, b)
				}
				var p [3]float64
				for k := range 3 {
					p[k] = b.b0*scene[0][k] + b.b1*scene[1][k] + b.b2*scene[2][k]
				}
				if math.Abs(p[0]/p[2]-(float64(x)+0.5)) > 1e-8 || math.Abs(p[1]/p[2]-(float64(y)+0.5)) > 1e-8 {
					t.Fatalf("triangle %v, pixel (%d,%d): wanted the scene point to project to the pixel's middle, got (%v,%v)", c, x, y, p[0]/p[2], p[1]/p[2])
				}
			}
		}
	}
	if covered < 500 {
		t.Fatalf("wanted the random triangles to cover plenty of pixels, covered only %d", covered)
	}
}

// The function is asked only about cells that will be drawn. The half triangle drawn
// first covers the 10 cells of the top-left half of a 4x4 buffer, or all 16 if full.
func TestFillTriangleFuncIsNotAskedAboutHiddenCells(t *testing.T) {
	big := [6]float64{-100, -100, 300, -100, -100, 300}
	tests := []struct {
		name   string
		full   bool    // whether the first fill covers every cell
		behind float64 // its depth
		depth  float64 // depth of the triangle that covers everything
		want   int     // calls
	}{
		{"nearer than all of it", false, 2, 1, 16},
		{"farther: only the uncovered cells", false, 2, 5, 6},
		{"same depth: ties lose", false, 2, 2, 6},
		{"farther than everything", true, 1, 9, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := newFB(t, 4, 4)
			if tt.full {
				fb.FillTriangle(big[0], big[1], tt.behind, big[2], big[3], tt.behind, big[4], big[5], tt.behind, Cell{Character: 'a'})
			} else {
				fb.FillTriangle(0, 0, tt.behind, 4, 0, tt.behind, 0, 4, tt.behind, Cell{Character: 'a'})
			}
			before := slices.Clone(fb.frame)
			calls := 0

			fb.FillTriangleFunc(big[0], big[1], tt.depth, big[2], big[3], tt.depth, big[4], big[5], tt.depth, func(_, _, _ float64) Cell {
				calls++
				return Cell{Character: 'b'}
			})

			if calls != tt.want {
				t.Errorf("wanted %d calls, got %d", tt.want, calls)
			}
			if tt.want == 0 && !slices.Equal(fb.frame, before) {
				t.Errorf("wanted every cell left as it was")
			}
		})
	}
}

func TestCanvasFillTriangleFuncIsNotAskedAboutHiddenPixels(t *testing.T) {
	big := [6]float64{-100, -100, 300, -100, -100, 300}
	// The half triangle drawn first, at depth 2, covers 10 of the 16 pixels.
	tests := []struct {
		name  string
		depth float64 // of the triangle that covers everything
		want  int     // calls
	}{
		{"nearer than all of it", 1, 16},
		{"farther: only the uncovered pixels", 5, 6},
		{"same depth: ties lose", 2, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := newTestCanvas(t, 4, 2) // 4x4 pixels
			c.FillTriangle(0, 0, 2, 4, 0, 2, 0, 4, 2, pix(1))
			calls := 0

			c.FillTriangleFunc(big[0], big[1], tt.depth, big[2], big[3], tt.depth, big[4], big[5], tt.depth, func(_, _, _ float64) Color {
				calls++
				return pix(2)
			})

			if calls != tt.want {
				t.Errorf("wanted %d calls, got %d", tt.want, calls)
			}
			wantCorner := pix(1)
			if tt.depth < 2 {
				wantCorner = pix(2)
			}
			if c.At(0, 0) != wantCorner || c.At(3, 3) != pix(2) {
				t.Errorf("wanted %v at the covered corner and %v at the free one, got %v and %v", wantCorner, pix(2), c.At(0, 0), c.At(3, 3))
			}
		})
	}
}

// What the function returns is drawn with the depth, so a later farther fill loses to it.
func TestFillTriangleFuncDrawsWhatItIsGivenWithItsDepth(t *testing.T) {
	big := [6]float64{-100, -100, 300, -100, -100, 300}
	fb := newFB(t, 3, 2)

	fb.FillTriangleFunc(big[0], big[1], 2, big[2], big[3], 2, big[4], big[5], 2, func(_, _, _ float64) Cell {
		return Cell{Character: 'x', Foreground: Palette(5)}
	})
	fb.FillTriangle(big[0], big[1], 9, big[2], big[3], 9, big[4], big[5], 9, Cell{Character: 'y'})

	for y := range 2 {
		for x := range 3 {
			if got := fb.At(x, y); got != (Cell{Character: 'x', Foreground: Palette(5)}) {
				t.Errorf("wanted the function's cell at (%d,%d), got %+v", x, y, got)
			}
		}
	}
}

// Nothing is drawn, and nothing asked, for what FillTriangle would not draw either.
func TestFillTriangleFuncDrawsNothingForBadTriangles(t *testing.T) {
	nan := math.NaN()
	tests := []struct {
		name string
		c    [9]float64
	}{
		{"zero depth", [9]float64{-100, -100, 0, 300, -100, 1, -100, 300, 1}},
		{"negative depth", [9]float64{-100, -100, 1, 300, -100, -1, -100, 300, 1}},
		{"NaN depth", [9]float64{-100, -100, 1, 300, -100, 1, -100, 300, nan}},
		{"no area", [9]float64{0, 0, 1, 2, 2, 1, 4, 4, 1}},
		{"NaN corner", [9]float64{0, 0, 1, nan, 0, 1, 0, 4, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := newFB(t, 4, 4)
			calls := 0
			c := tt.c

			fb.FillTriangleFunc(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(_, _, _ float64) Cell {
				calls++
				return Cell{Character: '#'}
			})

			if calls != 0 || !slices.Equal(fb.frame, newFB(t, 4, 4).frame) {
				t.Errorf("wanted nothing asked or drawn, got %d calls", calls)
			}
		})
	}
}

func TestFillTriangleFuncWithNoFunctionDrawsNothing(t *testing.T) {
	fb := newFB(t, 4, 4)
	fb.FillTriangleFunc(0, 0, 1, 4, 0, 1, 0, 4, 1, nil)

	_, c := newTestCanvas(t, 4, 2)
	c.FillTriangleFunc(0, 0, 1, 4, 0, 1, 0, 4, 1, nil)

	if !slices.Equal(fb.frame, newFB(t, 4, 4).frame) || c.At(0, 0) != pix(0) {
		t.Errorf("wanted a nil function to draw nothing")
	}
}

// A triangle far larger than the buffer visits only the cells in it.
func TestFillTriangleFuncOnATinyBufferStaysInside(t *testing.T) {
	fb := newFB(t, 1, 1)
	calls := 0

	fb.FillTriangleFunc(-1e9, -1e9, 1, 1e9, -1e9, 1, -1e9, 1e9, 1, func(_, _, _ float64) Cell {
		calls++
		return Cell{Character: '#'}
	})

	if calls != 1 || fb.At(0, 0).Character != '#' {
		t.Errorf("wanted the one cell asked once and drawn, got %d calls and %q", calls, fb.At(0, 0).Character)
	}
}

func BenchmarkFillTriangleFunc(b *testing.B) {
	fb, err := NewFrameBuffer(200, 60)
	if err != nil {
		b.Fatal(err)
	}
	cell := Cell{Character: '#'}
	fn := func(_, _, _ float64) Cell { return cell }

	b.Run("FillTriangle", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			fb.Clear()
			fb.FillTriangle(0, 0, 1, 200, 10, 3, 20, 60, 2, cell)
		}
	})
	b.Run("FillTriangleFunc", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			fb.Clear()
			fb.FillTriangleFunc(0, 0, 1, 200, 10, 3, 20, 60, 2, fn)
		}
	})
}

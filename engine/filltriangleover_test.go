package engine

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// modeOf picks a Fill from the weights, so that one triangle has all three side by side.
func modeOf(b0 float64) Fill { return Fill(int(b0*1000) % 3) }

// Every Fill does what it says, cell by cell, in a triangle that has all three: a cell that was
// skipped is as it was, with its depth; one drawn solid is what FillTriangleFunc would have made
// of it; one drawn over has the same cell and keeps the depth that was there.
func TestFillTriangleOverDoesWhatEachFillSays(t *testing.T) {
	const w, h = 9, 6
	rng := rand.New(rand.NewPCG(51, 1))
	marks := [3]Cell{FillSkip: {Character: 'k'}, FillSolid: {Character: 'S'}, FillOver: {Character: 'O'}}
	under := Cell{Character: '.'}

	for range 300 {
		u, _ := randomTriangle(rng, w, h)
		c, _ := randomTriangle(rng, w, h)
		setup := func() *FrameBuffer {
			fb := newFB(t, w, h)
			fb.FillTriangle(u[0], u[1], u[2], u[3], u[4], u[5], u[6], u[7], u[8], under)
			return fb
		}
		original := setup()

		// What every cell would be if the whole triangle were one Fill.
		whole := [3]*FrameBuffer{}
		for _, mode := range []Fill{FillSolid, FillOver} {
			whole[mode] = setup()
			whole[mode].FillTriangleOver(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(_, _, _ float64, _ Cell) (Cell, Fill) { return marks[mode], mode })
		}
		viaFunc := setup()
		viaFunc.FillTriangleFunc(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(_, _, _ float64) Cell { return marks[FillSolid] })

		mixed := setup()
		mixed.FillTriangleOver(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(b0, _, _ float64, _ Cell) (Cell, Fill) {
			mode := modeOf(b0)
			return marks[mode], mode
		})

		for i := range mixed.frame {
			var want *FrameBuffer
			switch mixed.frame[i] {
			case marks[FillSolid]:
				want = whole[FillSolid]
			case marks[FillOver]:
				want = whole[FillOver]
			default:
				want = original // skipped, or not covered at all
			}
			if mixed.frame[i] != want.frame[i] || mixed.depth[i] != want.depth[i] {
				t.Fatalf("cell %d of triangle %v over %v: wanted %+v at depth %v, got %+v at depth %v", i, c, u, want.frame[i], want.depth[i], mixed.frame[i], mixed.depth[i])
			}
		}
		// All three of them, when the whole is one fill, are the ones named.
		if !slices.Equal(whole[FillSolid].frame, viaFunc.frame) || !slices.Equal(whole[FillSolid].depth, viaFunc.depth) {
			t.Fatalf("triangle %v over %v: wanted FillSolid everywhere to be FillTriangleFunc", c, u)
		}
		if !slices.Equal(whole[FillOver].depth, original.depth) {
			t.Fatalf("triangle %v over %v: wanted FillOver everywhere to leave every depth", c, u)
		}
		for i := range whole[FillOver].frame {
			if (whole[FillOver].frame[i] == marks[FillOver]) != (viaFunc.frame[i] == marks[FillSolid]) {
				t.Fatalf("triangle %v over %v: cell %d: wanted FillOver to cover the cells that FillSolid does", c, u, i)
			}
		}
	}
}

func TestCanvasFillTriangleOverDoesWhatEachFillSays(t *testing.T) {
	const w, h = 9, 4 // 9x8 pixels
	rng := rand.New(rand.NewPCG(51, 2))
	marks := [3]Color{FillSkip: RGB(7, 7, 7), FillSolid: RGB(255, 0, 0), FillOver: RGB(0, 255, 0)}
	under := Palette(9)

	for range 300 {
		u, _ := randomTriangle(rng, w, 2*h)
		c, _ := randomTriangle(rng, w, 2*h)
		setup := func() *Canvas {
			_, cv := newTestCanvas(t, w, h)
			cv.FillTriangle(u[0], u[1], u[2], u[3], u[4], u[5], u[6], u[7], u[8], under)
			return cv
		}
		original := setup()

		whole := [3]*Canvas{}
		for _, mode := range []Fill{FillSolid, FillOver} {
			whole[mode] = setup()
			whole[mode].FillTriangleOver(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(_, _, _ float64, _ Color) (Color, Fill) { return marks[mode], mode })
		}
		viaFunc := setup()
		viaFunc.FillTriangleFunc(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(_, _, _ float64) Color { return marks[FillSolid] })

		mixed := setup()
		mixed.FillTriangleOver(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(b0, _, _ float64, _ Color) (Color, Fill) {
			mode := modeOf(b0)
			return marks[mode], mode
		})

		for i := range mixed.pixels {
			var want *Canvas
			switch mixed.pixels[i] {
			case marks[FillSolid]:
				want = whole[FillSolid]
			case marks[FillOver]:
				want = whole[FillOver]
			default:
				want = original
			}
			if mixed.pixels[i] != want.pixels[i] || mixed.depth[i] != want.depth[i] {
				t.Fatalf("pixel %d of triangle %v over %v: wanted %v at depth %v, got %v at depth %v", i, c, u, want.pixels[i], want.depth[i], mixed.pixels[i], mixed.depth[i])
			}
		}
		if !slices.Equal(whole[FillSolid].pixels, viaFunc.pixels) || !slices.Equal(whole[FillSolid].depth, viaFunc.depth) {
			t.Fatalf("triangle %v over %v: wanted FillSolid everywhere to be FillTriangleFunc", c, u)
		}
		if !slices.Equal(whole[FillOver].depth, original.depth) {
			t.Fatalf("triangle %v over %v: wanted FillOver everywhere to leave every depth", c, u)
		}
		for i := range whole[FillOver].pixels {
			if (whole[FillOver].pixels[i] == marks[FillOver]) != (viaFunc.pixels[i] == marks[FillSolid]) {
				t.Fatalf("triangle %v over %v: pixel %d: wanted FillOver to cover the pixels that FillSolid does", c, u, i)
			}
		}
	}
}

// over is given the cell that is there, an empty one where nothing was drawn.
func TestFillTriangleOverIsGivenWhatIsThere(t *testing.T) {
	fb := newFB(t, 4, 4)
	fb.FillTriangle(0, 0, 5, 2, 0, 5, 0, 2, 5, Cell{Character: 'x', Foreground: Palette(3), Background: RGB(1, 2, 3), Attrs: Bold})
	want := fb.frame[0]

	var seen []Cell
	fb.FillTriangleOver(-10, -10, 1, 20, -10, 1, -10, 20, 1, func(_, _, _ float64, behind Cell) (Cell, Fill) {
		seen = append(seen, behind)
		return Cell{Character: 'o'}, FillOver
	})
	if len(seen) != 16 {
		t.Fatalf("wanted 16 cells asked about, got %d", len(seen))
	}
	if seen[0] != want {
		t.Errorf("wanted the cell that was drawn, %+v, got %+v", want, seen[0])
	}
	if seen[15] != (Cell{Character: ' '}) {
		t.Errorf("wanted a blank cell where nothing was drawn, got %+v", seen[15])
	}

	// Drawn over a second time, a cell is given what the first left, not what was first there.
	var second []Cell
	fb.FillTriangleOver(-10, -10, 2, 20, -10, 2, -10, 20, 2, func(_, _, _ float64, behind Cell) (Cell, Fill) {
		second = append(second, behind)
		return behind, FillSkip
	})
	if len(second) != 16 || second[0] != (Cell{Character: 'o'}) || second[15] != (Cell{Character: 'o'}) {
		t.Errorf("wanted the cells that the first fill drew over, got %+v ... %+v", second[0], second[len(second)-1])
	}
}

func TestCanvasFillTriangleOverIsGivenWhatIsThere(t *testing.T) {
	_, c := newTestCanvas(t, 4, 2) // 4x4 pixels
	c.FillTriangle(0, 0, 5, 2, 0, 5, 0, 2, 5, RGB(9, 8, 7))
	want := c.pixels[0]

	var seen []Color
	c.FillTriangleOver(-10, -10, 1, 20, -10, 1, -10, 20, 1, func(_, _, _ float64, behind Color) (Color, Fill) {
		seen = append(seen, behind)
		return RGB(1, 1, 1), FillOver
	})
	if len(seen) != 16 {
		t.Fatalf("wanted 16 pixels asked about, got %d", len(seen))
	}
	if seen[0] != want || seen[15] != (Color{}) {
		t.Errorf("wanted the drawn color %v then an empty pixel, got %v and %v", want, seen[0], seen[15])
	}
}

// The weights are the ones FillTriangleFunc gives.
func TestFillTriangleOverGivesTheWeightsOfFillTriangleFunc(t *testing.T) {
	const w, h = 9, 6
	rng := rand.New(rand.NewPCG(51, 3))
	for range 200 {
		c, _ := randomTriangle(rng, w, h)
		var viaFunc, viaOver []weighted
		newFB(t, w, h).FillTriangleFunc(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(b0, b1, b2 float64) Cell {
			viaFunc = append(viaFunc, weighted{b0, b1, b2})
			return Cell{}
		})
		newFB(t, w, h).FillTriangleOver(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(b0, b1, b2 float64, _ Cell) (Cell, Fill) {
			viaOver = append(viaOver, weighted{b0, b1, b2})
			return Cell{}, FillSolid
		})
		if !slices.Equal(viaFunc, viaOver) {
			t.Fatalf("triangle %v: wanted the weights of FillTriangleFunc, got %v for %v", c, viaOver, viaFunc)
		}

		// And on a canvas of the same grid.
		var pixelsFunc, pixelsOver []weighted
		_, cv := newTestCanvas(t, w, h/2)
		cv.FillTriangleFunc(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(b0, b1, b2 float64) Color {
			pixelsFunc = append(pixelsFunc, weighted{b0, b1, b2})
			return Color{}
		})
		_, cv = newTestCanvas(t, w, h/2)
		cv.FillTriangleOver(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(b0, b1, b2 float64, _ Color) (Color, Fill) {
			pixelsOver = append(pixelsOver, weighted{b0, b1, b2})
			return Color{}, FillSolid
		})
		if !slices.Equal(pixelsFunc, pixelsOver) {
			t.Fatalf("triangle %v: wanted the weights of Canvas.FillTriangleFunc, got %v for %v", c, pixelsOver, pixelsFunc)
		}
	}
}

// The depth test comes first: over is asked about the cells that are nearer, and no more.
func TestFillTriangleOverIsNotAskedAboutHiddenCells(t *testing.T) {
	big := [6]float64{-100, -100, 300, -100, -100, 300}
	for _, tt := range []struct {
		name   string
		full   bool
		behind float64
		depth  float64
		want   int
	}{
		{"nearer than all of it", false, 2, 1, 16},
		{"farther: only the uncovered cells", false, 2, 5, 6},
		{"same depth: ties lose", false, 2, 2, 6},
		{"farther than everything", true, 1, 9, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			count := func(pixels bool) int {
				calls := 0
				if pixels {
					_, c := newTestCanvas(t, 4, 2)
					if tt.full {
						c.FillTriangle(big[0], big[1], tt.behind, big[2], big[3], tt.behind, big[4], big[5], tt.behind, RGB(1, 1, 1))
					} else {
						c.FillTriangle(0, 0, tt.behind, 4, 0, tt.behind, 0, 4, tt.behind, RGB(1, 1, 1))
					}
					c.FillTriangleOver(big[0], big[1], tt.depth, big[2], big[3], tt.depth, big[4], big[5], tt.depth, func(_, _, _ float64, _ Color) (Color, Fill) {
						calls++
						return Color{}, FillSolid
					})
					return calls
				}
				fb := newFB(t, 4, 4)
				if tt.full {
					fb.FillTriangle(big[0], big[1], tt.behind, big[2], big[3], tt.behind, big[4], big[5], tt.behind, Cell{Character: 'a'})
				} else {
					fb.FillTriangle(0, 0, tt.behind, 4, 0, tt.behind, 0, 4, tt.behind, Cell{Character: 'a'})
				}
				fb.FillTriangleOver(big[0], big[1], tt.depth, big[2], big[3], tt.depth, big[4], big[5], tt.depth, func(_, _, _ float64, _ Cell) (Cell, Fill) {
					calls++
					return Cell{}, FillSolid
				})
				return calls
			}
			if got := count(false); got != tt.want {
				t.Errorf("cells: wanted %d calls, got %d", tt.want, got)
			}
			// The canvas is 4 pixels across and 4 down here: the same grid.
			if got := count(true); got != tt.want {
				t.Errorf("pixels: wanted %d calls, got %d", tt.want, got)
			}
		})
	}
}

// A surface drawn over keeps the depth of what is behind it: a nearer solid drawn later covers
// it, a farther one is hidden by what was behind, and a second pane is given the first's result.
func TestFillTriangleOverLeavesTheDepthSoLaterDrawingDecides(t *testing.T) {
	fb := newFB(t, 4, 4)
	wall, glass, nearer, farther := Cell{Character: 'W'}, Cell{Character: 'G'}, Cell{Character: 'N'}, Cell{Character: 'F'}
	fill := func(z float64, c Cell, how Fill) {
		fb.FillTriangleOver(-10, -10, z, 30, -10, z, -10, 30, z, func(_, _, _ float64, _ Cell) (Cell, Fill) { return c, how })
	}

	fill(5, wall, FillSolid)
	fill(3, glass, FillOver)
	if got := fb.At(1, 1); got != glass {
		t.Fatalf("wanted the glass in front of the wall, got %+v", got)
	}
	if got := fb.depth[5]; got != 5 {
		t.Fatalf("wanted the depth of the wall to stay, got %v", got)
	}

	// Farther than the wall: hidden by it, glass or not.
	fill(6, farther, FillSolid)
	if got := fb.At(1, 1); got != glass {
		t.Errorf("wanted what is behind the wall hidden, got %+v", got)
	}
	// Nearer than the wall, though farther than the glass: the depth of the glass was not stored,
	// so it replaces the glass. That is why a see-through surface is drawn after the solid ones.
	fill(4, nearer, FillSolid)
	if got := fb.At(1, 1); got != nearer {
		t.Errorf("wanted a solid between the wall and the glass to replace the glass, got %+v", got)
	}

	// A second pane is given what the first left, so panes drawn far to near are blended in that order.
	fb = newFB(t, 4, 4)
	fill(5, wall, FillSolid)
	fill(3, glass, FillOver)
	var behind Cell
	fb.FillTriangleOver(-10, -10, 2, 30, -10, 2, -10, 30, 2, func(_, _, _ float64, b Cell) (Cell, Fill) {
		behind = b
		return Cell{Character: 'H'}, FillOver
	})
	if behind != glass {
		t.Errorf("wanted the second pane given the first, got %+v", behind)
	}
}

func TestFillTriangleOverWithNoFunctionOrAnUnknownFillDrawsNothing(t *testing.T) {
	fb := newFB(t, 4, 4)
	fb.FillTriangle(0, 0, 5, 4, 0, 5, 0, 4, 5, Cell{Character: 'a'})
	before, depth := slices.Clone(fb.frame), slices.Clone(fb.depth)

	fb.FillTriangleOver(-10, -10, 1, 30, -10, 1, -10, 30, 1, nil)
	for _, odd := range []Fill{3, 4, 200, 255} {
		fb.FillTriangleOver(-10, -10, 1, 30, -10, 1, -10, 30, 1, func(_, _, _ float64, _ Cell) (Cell, Fill) { return Cell{Character: 'x'}, odd })
	}
	if !slices.Equal(fb.frame, before) || !slices.Equal(fb.depth, depth) {
		t.Errorf("wanted nothing changed")
	}

	_, c := newTestCanvas(t, 4, 2)
	c.FillTriangle(0, 0, 5, 4, 0, 5, 0, 4, 5, RGB(1, 2, 3))
	pixels, cdepth := slices.Clone(c.pixels), slices.Clone(c.depth)
	c.FillTriangleOver(-10, -10, 1, 30, -10, 1, -10, 30, 1, nil)
	for _, odd := range []Fill{3, 200} {
		c.FillTriangleOver(-10, -10, 1, 30, -10, 1, -10, 30, 1, func(_, _, _ float64, _ Color) (Color, Fill) { return RGB(9, 9, 9), odd })
	}
	if !slices.Equal(c.pixels, pixels) || !slices.Equal(c.depth, cdepth) {
		t.Errorf("wanted nothing changed on the canvas")
	}
}

func TestFillTriangleOverDrawsNothingForBadTriangles(t *testing.T) {
	nan := 0.0
	nan /= nan
	for name, c := range map[string][9]float64{
		"no area":            {0, 0, 1, 1, 1, 1, 2, 2, 1},
		"a depth of zero":    {0, 0, 0, 4, 0, 1, 0, 4, 1},
		"a negative depth":   {0, 0, 1, 4, 0, -1, 0, 4, 1},
		"a NaN depth":        {0, 0, 1, 4, 0, nan, 0, 4, 1},
		"a NaN corner":       {0, 0, 1, nan, 0, 1, 0, 4, 1},
		"entirely off sides": {50, 50, 1, 60, 50, 1, 50, 60, 1},
	} {
		fb := newFB(t, 4, 4)
		fb.FillTriangleOver(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(_, _, _ float64, _ Cell) (Cell, Fill) {
			t.Errorf("%s: wanted no call", name)
			return Cell{Character: 'x'}, FillSolid
		})
		_, cv := newTestCanvas(t, 4, 2)
		cv.FillTriangleOver(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(_, _, _ float64, _ Color) (Color, Fill) {
			t.Errorf("%s: wanted no call on the canvas", name)
			return RGB(1, 1, 1), FillSolid
		})
	}
}

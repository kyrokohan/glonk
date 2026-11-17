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

// visits walks the triangles with scan and counts how many times each cell of a w by h grid is visited.
func visits(w, h int, scan func(w, h int, x0, y0, z0, x1, y1, z1, x2, y2, z2 float64, plot func(x, y int, z, p0, p1, p2 float64)), tris [][6]float64) []int {
	n := make([]int, w*h)
	for _, c := range tris {
		scan(w, h, c[0], c[1], 1, c[2], c[3], 1, c[4], c[5], 1, func(x, y int, _, _, _, _ float64) { n[y*w+x]++ })
	}
	return n
}

// fan is the triangles that a polygon is cut into from its first corner, as a mesh cuts a quad,
// and as a mesh of any size is made of triangles that share their edges.
func fan(points ...[2]float64) [][6]float64 {
	var tris [][6]float64
	for i := 1; i+1 < len(points); i++ {
		tris = append(tris, [6]float64{points[0][0], points[0][1], points[i][0], points[i][1], points[i+1][0], points[i+1][1]})
	}
	return tris
}

// Where triangles share an edge, a cell with its centre on the edge is visited once: no more, and
// no less. A cell on the outer edge of a shape is the half-open rule's to decide, in or out by the
// side the edge is on, so it may be left out, and the corner where such edges meet is out. The
// corners are put where centres fall on the diagonals and on the spokes.
func TestScanTriangleOnceVisitsACellOnASharedEdgeOnce(t *testing.T) {
	const w, h = 12, 12
	grid := func(x, y float64) [][6]float64 {
		return fan([2]float64{x, y}, [2]float64{x + 6, y}, [2]float64{x + 6, y + 6}, [2]float64{x, y + 6})
	}
	shapes := map[string][][6]float64{
		"a square, one diagonal":        fan([2]float64{1, 1}, [2]float64{9, 1}, [2]float64{9, 9}, [2]float64{1, 9}),
		"a square, the other":           fan([2]float64{9, 1}, [2]float64{9, 9}, [2]float64{1, 9}, [2]float64{1, 1}),
		"wound the other way":           fan([2]float64{1, 1}, [2]float64{1, 9}, [2]float64{9, 9}, [2]float64{9, 1}),
		"corners on cell centres":       fan([2]float64{1.5, 1.5}, [2]float64{9.5, 1.5}, [2]float64{9.5, 9.5}, [2]float64{1.5, 9.5}),
		"a hexagon of spokes":           fan([2]float64{6, 6}, [2]float64{10, 6}, [2]float64{8, 10}, [2]float64{4, 10}, [2]float64{2, 6}, [2]float64{4, 2}, [2]float64{8, 2}, [2]float64{10, 6}),
		"a slanted quad":                fan([2]float64{1, 2}, [2]float64{10, 1}, [2]float64{11, 9}, [2]float64{2, 10}),
		"a grid of four squares":        slices.Concat(grid(0, 0), grid(6, 0), grid(0, 6), grid(6, 6)),
		"a quad that runs off the grid": fan([2]float64{-3, -3}, [2]float64{15, -3}, [2]float64{15, 15}, [2]float64{-3, 15}),
	}
	ties := 0
	for name, tris := range shapes {
		once := visits(w, h, scanTriangleOnce, tris)
		both := visits(w, h, scanTriangle, tris)
		for i := range once {
			x, y := i%w, i/w
			switch {
			case once[i] > 1:
				t.Errorf("%s: cell (%d, %d) visited %d times, wanted once", name, x, y, once[i])
			case once[i] > 0 && both[i] == 0:
				t.Errorf("%s: cell (%d, %d) visited, but not by scanTriangle", name, x, y)
			}
			if both[i] > 1 {
				ties++
			}
		}
	}
	// The shapes really do have cells on an edge, or this would prove nothing.
	if ties < 20 {
		t.Errorf("wanted shapes with at least 20 cells on a shared edge, got %d", ties)
	}

	// What a quad covers does not depend on how it is cut: the same square cut along either diagonal,
	// and wound either way, is the same cells, once each. A cell on the diagonal that was dropped, or
	// drawn twice, would show against the cut that has it inside a triangle.
	same := visits(w, h, scanTriangleOnce, shapes["a square, one diagonal"])
	for _, name := range []string{"a square, the other", "wound the other way"} {
		if got := visits(w, h, scanTriangleOnce, shapes[name]); !slices.Equal(got, same) {
			t.Errorf("%s: wanted the cells of the same square cut the other way", name)
		}
	}

	// Exact totals: the square on cell centres has its left and top edges in and its right and bottom
	// out, an 8 by 8 block, and a square of four has every cell of the grid once.
	count := func(tris [][6]float64) (n int) {
		for _, v := range visits(w, h, scanTriangleOnce, tris) {
			n += v
		}
		return n
	}
	if got := count(shapes["corners on cell centres"]); got != 64 {
		t.Errorf("wanted 64 cells in the block on cell centres, got %d", got)
	}
	// Which cells: the square's centres on its left column and its top row are in, and those on its right
	// column and its bottom row are out, so it is columns 1 to 8 and rows 1 to 8, not 2 to 9.
	for i, n := range visits(w, h, scanTriangleOnce, shapes["corners on cell centres"]) {
		x, y := i%w, i/w
		if want := x >= 1 && x <= 8 && y >= 1 && y <= 8; (n == 1) != want {
			t.Errorf("block on cell centres: cell (%d, %d) visited %d times, wanted it in the block: %v", x, y, n, want)
		}
	}
	if got := count(shapes["a grid of four squares"]); got != 144 {
		t.Errorf("wanted every one of 144 cells once, got %d", got)
	}
}

// Away from the edges it is scanTriangle: the same cells, with the same depths and weights.
func TestScanTriangleOnceIsScanTriangleWhereNothingIsOnAnEdge(t *testing.T) {
	rng := rand.New(rand.NewPCG(51, 4))
	type visit struct {
		x, y          int
		z, p0, p1, p2 float64
	}
	for range 500 {
		c, _ := randomTriangle(rng, 9, 6)
		var a, b []visit
		scanTriangle(9, 6, c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(x, y int, z, p0, p1, p2 float64) { a = append(a, visit{x, y, z, p0, p1, p2}) })
		scanTriangleOnce(9, 6, c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(x, y int, z, p0, p1, p2 float64) { b = append(b, visit{x, y, z, p0, p1, p2}) })
		if !slices.Equal(a, b) {
			t.Fatalf("triangle %v: wanted the visits of scanTriangle, got %d for %d", c, len(b), len(a))
		}
	}
	// And the same bad triangles draw nothing.
	nan := 0.0
	nan /= nan
	for _, c := range [][9]float64{{0, 0, 1, 1, 1, 1, 2, 2, 1}, {0, 0, 0, 4, 0, 1, 0, 4, 1}, {0, 0, 1, 4, 0, nan, 0, 4, 1}, {0, 0, 1, nan, 0, 1, 0, 4, 1}} {
		scanTriangleOnce(9, 6, c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], func(x, y int, _, _, _, _ float64) { t.Errorf("wanted nothing for %v, got (%d, %d)", c, x, y) })
	}
}

// Through the fill itself: a quad of two triangles, drawn over, asks about each cell once, so a mix is
// made once.
func TestFillTriangleOverAsksAboutACellOnAnEdgeOnce(t *testing.T) {
	fb := newFB(t, 12, 12)
	for _, c := range fan([2]float64{1, 1}, [2]float64{9, 1}, [2]float64{9, 9}, [2]float64{1, 9}) {
		fb.FillTriangleOver(c[0], c[1], 1, c[2], c[3], 1, c[4], c[5], 1, func(_, _, _ float64, behind Cell) (Cell, Fill) {
			behind.Character++ // a cell that is asked twice ends up two on
			return behind, FillOver
		})
	}
	covered := 0
	for i, c := range fb.frame {
		n := int(c.Character) - ' ' // a fresh cell is a space
		if n > 1 {
			t.Fatalf("cell (%d, %d) was asked about %d times", i%12, i/12, n)
		}
		covered += n
	}
	if covered != 64 {
		t.Errorf("wanted the 8 by 8 square covered, got %d cells", covered)
	}

	// On a canvas, with the pixels of a 12 by 12 grid.
	_, c := newTestCanvas(t, 12, 6)
	for _, tri := range fan([2]float64{1, 1}, [2]float64{9, 1}, [2]float64{9, 9}, [2]float64{1, 9}) {
		c.FillTriangleOver(tri[0], tri[1], 1, tri[2], tri[3], 1, tri[4], tri[5], 1, func(_, _, _ float64, behind Color) (Color, Fill) {
			r, _, _ := behind.RGB()
			return RGB(r+1, 0, 0), FillOver // a pixel asked twice ends up two on
		})
	}
	covered = 0
	for i, p := range c.pixels {
		r, _, _ := p.RGB()
		if r > 1 {
			t.Fatalf("pixel (%d, %d) was asked about %d times", i%12, i/12, r)
		}
		covered += int(r)
	}
	if covered != 64 {
		t.Errorf("wanted 64 pixels covered, got %d", covered)
	}
}

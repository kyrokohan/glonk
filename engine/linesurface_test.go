package engine

import (
	"math"
	"math/rand/v2"
	"testing"
)

// planeDepth is the depth at the point (px, py) of the screen of the plane through the three
// corners c (x, y, depth each): the depth that FillTriangle gives a cell, worked out here from the
// weights by the rule that 1/z runs evenly across the screen.
func planeDepth(c [9]float64, px, py float64) float64 {
	area := (c[3]-c[0])*(c[7]-c[1]) - (c[6]-c[0])*(c[4]-c[1])
	w0 := ((c[3]-px)*(c[7]-py) - (c[6]-px)*(c[4]-py)) / area
	w1 := ((c[6]-px)*(c[1]-py) - (c[0]-px)*(c[7]-py)) / area
	w2 := ((c[0]-px)*(c[4]-py) - (c[3]-px)*(c[1]-py)) / area
	return 1 / (w0/c[2] + w1/c[5] + w2/c[8])
}

// A tilted plane over the whole of a w by h grid, as the corners of a triangle that is far bigger.
func bigPlane(rng *rand.Rand, w, h int) [9]float64 {
	return [9]float64{
		-5, -5, 1 + rng.Float64()*30,
		float64(2*w + 20), -5, 1 + rng.Float64()*30,
		-5, float64(2*h + 20), 1 + rng.Float64()*30,
	}
}

// lineOn returns a line that lies on the plane: random points on the screen, floored to cells as a
// mesh floors the ends of an edge, with the depth the plane has at the points themselves.
func lineOn(rng *rand.Rand, plane [9]float64, w, h int) (x0, y0 int, z0 float64, x1, y1 int, z1 float64) {
	px, py := rng.Float64()*float64(w), rng.Float64()*float64(h)
	qx, qy := rng.Float64()*float64(w), rng.Float64()*float64(h)
	return int(math.Floor(px)), int(math.Floor(py)), planeDepth(plane, px, py), int(math.Floor(qx)), int(math.Floor(qy)), planeDepth(plane, qx, qy)
}

// cellsOfLine counts the cells of a line inside a w by h grid.
func cellsOfLine(w, h, x0, y0, x1, y1 int) (n int) {
	walkLine(w, h, x0, y0, x1, y1, func(_, _ int) { n++ })
	return n
}

// A line that lies on a surface is whole on it, cells and pixels, however the surface is tilted
// and however far it is, where DrawLineDepth loses cells to a tie, and an end that is rounded down
// to a cell does not matter.
func TestDrawLineOnSurfaceIsWholeOnATiltedPlane(t *testing.T) {
	const w, h = 40, 12
	rng := rand.New(rand.NewPCG(50, 1))
	mark := Cell{Character: '#'}
	paint := RGB(255, 0, 255)
	lostByDepth := 0

	for range 600 {
		plane := bigPlane(rng, w, h)
		x0, y0, z0, x1, y1, z1 := lineOn(rng, plane, w, h)
		want := cellsOfLine(w, h, x0, y0, x1, y1)

		fb := newFB(t, w, h)
		fb.FillTriangle(plane[0], plane[1], plane[2], plane[3], plane[4], plane[5], plane[6], plane[7], plane[8], Cell{Character: '.'})
		fb.DrawLineOnSurface(x0, y0, z0, x1, y1, z1, mark)
		got := 0
		for _, c := range fb.frame {
			if c == mark {
				got++
			}
		}
		if got != want {
			t.Fatalf("plane %v, line (%d, %d) to (%d, %d): wanted %d cells drawn, got %d", plane, x0, y0, x1, y1, want, got)
		}

		// The same, in pixels: a canvas of w by h/2 cells is w by h pixels.
		_, c := newTestCanvas(t, w, h/2)
		c.FillTriangle(plane[0], plane[1], plane[2], plane[3], plane[4], plane[5], plane[6], plane[7], plane[8], Palette(9))
		c.DrawLineOnSurface(x0, y0, z0, x1, y1, z1, paint)
		got = 0
		for _, p := range c.pixels {
			if p == paint {
				got++
			}
		}
		if got != want {
			t.Fatalf("plane %v, line (%d, %d) to (%d, %d): wanted %d pixels drawn, got %d", plane, x0, y0, x1, y1, want, got)
		}

		// DrawLineDepth is the one that loses cells to a tie.
		tied := newFB(t, w, h)
		tied.FillTriangle(plane[0], plane[1], plane[2], plane[3], plane[4], plane[5], plane[6], plane[7], plane[8], Cell{Character: '.'})
		tied.DrawLineDepth(x0, y0, z0, x1, y1, z1, mark)
		drawn := 0
		for _, c := range tied.frame {
			if c == mark {
				drawn++
			}
		}
		if drawn < want {
			lostByDepth++
		}
	}
	if lostByDepth < 100 {
		t.Errorf("wanted DrawLineDepth to lose cells on at least 100 of 600 lines, or this proves nothing, got %d", lostByDepth)
	}
}

// markedCells counts the cells that hold mark after a line was drawn.
func markedCells(fb *FrameBuffer, mark Cell) (n int) {
	for _, c := range fb.frame {
		if c == mark {
			n++
		}
	}
	return n
}

// A line behind the surface is hidden: by a lot, whatever the tilt, and by a little where the surface is
// flat. In front of it, and on it, it shows.
func TestDrawLineOnSurfaceIsHiddenBehindTheSurface(t *testing.T) {
	const w, h = 40, 12
	rng := rand.New(rand.NewPCG(50, 2))
	mark := Cell{Character: '#'}

	for range 300 {
		plane := bigPlane(rng, w, h)
		x0, y0, z0, x1, y1, z1 := lineOn(rng, plane, w, h)
		for _, tt := range []struct {
			name   string
			scale  float64
			wanted bool
		}{
			{"30% behind", 1.3, false},
			{"twice as far", 2, false},
			{"ten times as far", 10, false},
			{"on it", 1, true},
			{"a tenth in front", 0.9, true},
			{"half as far", 0.5, true},
		} {
			fb := newFB(t, w, h)
			fb.FillTriangle(plane[0], plane[1], plane[2], plane[3], plane[4], plane[5], plane[6], plane[7], plane[8], Cell{Character: '.'})
			fb.DrawLineOnSurface(x0, y0, z0*tt.scale, x1, y1, z1*tt.scale, mark)
			got := markedCells(fb, mark)
			if all := cellsOfLine(w, h, x0, y0, x1, y1); tt.wanted && got != all {
				t.Fatalf("%s: plane %v: wanted all %d cells, got %d", tt.name, plane, all, got)
			} else if !tt.wanted && got != 0 {
				t.Fatalf("%s: plane %v: wanted nothing, got %d cells", tt.name, plane, got)
			}
		}
	}

	// In pixels too.
	_, c := newTestCanvas(t, 20, 6)
	c.FillTriangle(-5, -5, 10, 60, -5, 10, -5, 40, 10, Palette(9))
	c.DrawLineOnSurface(0, 5, 14, 19, 5, 14, RGB(1, 2, 3))
	c.DrawLineOnSurface(0, 7, 10, 19, 7, 10, RGB(4, 5, 6))
	for x := range 20 {
		if got := c.At(x, 5); got != Palette(9) {
			t.Fatalf("pixel (%d, 5): wanted a line 40%% behind a flat wall hidden, got %v", x, got)
		}
		if got := c.At(x, 7); got != RGB(4, 5, 6) {
			t.Fatalf("pixel (%d, 7): wanted the line on the wall, got %v", x, got)
		}
	}
}

// A nearer surface over half the surface hides the line over that half, and the cells beside its
// edge too: the edge is a jump in the depth, which counts as a step, and the cap on what a step can
// allow is what stops it from letting the line through.
func TestDrawLineOnSurfaceStaysHiddenAlongTheEdgeOfAnObject(t *testing.T) {
	const w, h = 40, 12
	mark := Cell{Character: '#'}
	paintScene := func(fb *FrameBuffer) {
		// The far wall at depth 10, and a nearer wall at depth 5 over the left half, x < 20.
		fb.FillTriangle(-5, -5, 10, 100, -5, 10, -5, 60, 10, Cell{Character: '.'})
		fb.FillTriangle(-5, -5, 5, 20, -5, 5, -5, 60, 5, Cell{Character: 'N'})
		fb.FillTriangle(20, -5, 5, 20, 60, 5, -5, 60, 5, Cell{Character: 'N'})
	}
	// The line is on the far wall, so it is 10 away along its whole length.
	fb := newFB(t, w, h)
	paintScene(fb)
	fb.DrawLineOnSurface(0, 5, 10, w-1, 5, 10, mark)
	for x := range w {
		got := fb.At(x, 5)
		switch {
		case x < 20 && got != (Cell{Character: 'N'}):
			t.Errorf("cell (%d, 5): wanted the nearer wall, got %+v", x, got)
		case x >= 20 && got != mark:
			t.Errorf("cell (%d, 5): wanted the line on the far wall, got %+v", x, got)
		}
	}

	// A line only 30% behind the nearer wall is hidden at its edge too, which is what the cap is: 25%.
	near := newFB(t, w, h)
	near.FillTriangle(-5, -5, 5, 20, -5, 5, -5, 60, 5, Cell{Character: 'N'})
	near.FillTriangle(20, -5, 5, 20, 60, 5, -5, 60, 5, Cell{Character: 'N'})
	near.FillTriangle(-5, -5, 6.5, 100, -5, 6.5, -5, 60, 6.5, Cell{Character: '.'}) // farther: loses where the wall is
	near.DrawLineOnSurface(0, 5, 6.5, w-1, 5, 6.5, mark)
	for x := range 20 {
		if got := near.At(x, 5); got != (Cell{Character: 'N'}) {
			t.Errorf("cell (%d, 5): wanted a line 30%% behind the nearer wall hidden, got %+v", x, got)
		}
	}

	// A line that is on the nearer wall shows over it, up to its edge and no further.
	fb = newFB(t, w, h)
	paintScene(fb)
	fb.DrawLineOnSurface(0, 5, 5, w-1, 5, 5, mark)
	for x := range w {
		got := fb.At(x, 5)
		switch {
		case x < 20 && got != mark:
			t.Errorf("cell (%d, 5): wanted the line on the nearer wall, got %+v", x, got)
		case x >= 20 && got != mark && got != (Cell{Character: '.'}):
			t.Errorf("cell (%d, 5): unexpected %+v", x, got)
		}
	}
	// Beyond its edge the line is in front of the far wall, so it is drawn there too, as any nearer line is.
	if got := fb.At(30, 5); got != mark {
		t.Errorf("wanted a line that is nearer than the far wall drawn on it, got %+v", got)
	}
}

// A thing one cell wide in front of a line hides it: the jump is on both sides, and what keeps the
// slope that it would be read as from letting the line through is the cap.
func TestDrawLineOnSurfaceIsHiddenByAThinObject(t *testing.T) {
	const w, h = 40, 12
	mark := Cell{Character: '#'}
	fb := newFB(t, w, h)
	fb.FillTriangle(-5, -5, 10, 100, -5, 10, -5, 60, 10, Cell{Character: '.'})
	// A pole one cell wide, at x = 20, 5 away.
	fb.FillTriangle(20, -5, 5, 21, -5, 5, 20, 60, 5, Cell{Character: 'P'})
	fb.FillTriangle(21, -5, 5, 21, 60, 5, 20, 60, 5, Cell{Character: 'P'})
	fb.DrawLineOnSurface(0, 5, 10, w-1, 5, 10, mark)
	for x := range w {
		got := fb.At(x, 5)
		switch {
		case x == 20 && got != (Cell{Character: 'P'}):
			t.Errorf("cell (%d, 5): wanted the pole to hide the line, got %+v", x, got)
		case x != 20 && got != mark:
			t.Errorf("cell (%d, 5): wanted the line on the wall, got %+v", x, got)
		}
	}
}

// What is drawn keeps the nearer depth, so a line never makes a surface farther, and what is drawn
// later and is farther than the surface is still hidden by it.
func TestDrawLineOnSurfaceKeepsTheNearerDepth(t *testing.T) {
	const w, h = 40, 12
	mark := Cell{Character: '#'}
	rng := rand.New(rand.NewPCG(50, 3))
	for range 100 {
		plane := bigPlane(rng, w, h)
		x0, y0, z0, x1, y1, z1 := lineOn(rng, plane, w, h)
		fb := newFB(t, w, h)
		fb.FillTriangle(plane[0], plane[1], plane[2], plane[3], plane[4], plane[5], plane[6], plane[7], plane[8], Cell{Character: '.'})
		before := append([]float64(nil), fb.depth...)
		fb.DrawLineOnSurface(x0, y0, z0, x1, y1, z1, mark)
		for i, c := range fb.frame {
			// The line lies on the plane, so it leaves the depth that is there exactly as it was: the depths
			// around a cell are what say how steep the surface is, and a line must not blur them.
			if c == mark && fb.depth[i] != before[i] {
				t.Fatalf("cell %d: wanted the depth %v of the surface left as it was, got %v", i, before[i], fb.depth[i])
			}
		}
		// A solid that is a hair farther than the plane everywhere stays hidden behind the line's cells.
		for y := range h {
			for x := range w {
				i := y*w + x
				if fb.frame[i] == mark {
					fb.SetDepth(x, y, before[i]*1.001, Cell{Character: 'F'})
				}
			}
		}
		if got := markedCells(fb, mark); got != cellsOfLine(w, h, x0, y0, x1, y1) {
			t.Fatalf("wanted every cell of the line still there after a solid a hair behind was drawn, got %d", got)
		}
	}

	// A line in front of the surface is stored at its own depth.
	fb := newFB(t, 4, 4)
	fb.FillTriangle(-5, -5, 10, 30, -5, 10, -5, 30, 10, Cell{Character: '.'})
	fb.DrawLineOnSurface(0, 1, 3, 3, 1, 3, mark)
	if got := fb.depth[1*4+2]; got != 3 {
		t.Errorf("wanted a line in front stored at its own depth 3, got %v", got)
	}
}

// Where nothing is stored, or at the edge of the grid, there is no slope to read: a line is drawn
// as DrawLineDepth draws it, on the cells it would, with the depth it has.
func TestDrawLineOnSurfaceOnAnEmptyBufferIsDrawLineDepth(t *testing.T) {
	const w, h = 30, 10
	rng := rand.New(rand.NewPCG(50, 4))
	mark := Cell{Character: '#'}
	for range 300 {
		x0, y0, x1, y1 := rng.IntN(120)-45, rng.IntN(60)-25, rng.IntN(120)-45, rng.IntN(60)-25
		z0, z1 := 0.1+rng.Float64()*50, 0.1+rng.Float64()*50
		a, b := newFB(t, w, h), newFB(t, w, h)
		a.DrawLineDepth(x0, y0, z0, x1, y1, z1, mark)
		b.DrawLineOnSurface(x0, y0, z0, x1, y1, z1, mark)
		for i := range a.frame {
			if a.frame[i] != b.frame[i] || a.depth[i] != b.depth[i] {
				t.Fatalf("line (%d, %d) to (%d, %d): cell %d: wanted what DrawLineDepth draws, %+v at %v, got %+v at %v", x0, y0, x1, y1, i, a.frame[i], a.depth[i], b.frame[i], b.depth[i])
			}
		}
		_, ca := newTestCanvas(t, w, h/2)
		_, cb := newTestCanvas(t, w, h/2)
		ca.DrawLineDepth(x0, y0, z0, x1, y1, z1, RGB(1, 2, 3))
		cb.DrawLineOnSurface(x0, y0, z0, x1, y1, z1, RGB(1, 2, 3))
		for i := range ca.pixels {
			if ca.pixels[i] != cb.pixels[i] || ca.depth[i] != cb.depth[i] {
				t.Fatalf("line (%d, %d) to (%d, %d): pixel %d: wanted what DrawLineDepth draws", x0, y0, x1, y1, i)
			}
		}
	}
}

// What a depth that is not usable does: nothing, as for DrawLineDepth.
func TestDrawLineOnSurfaceWithABadDepthDrawsNothing(t *testing.T) {
	nan := math.NaN()
	for name, z := range map[string][2]float64{
		"zero":     {0, 1},
		"negative": {1, -1},
		"NaN":      {nan, 1},
		"infinite": {math.Inf(1), math.Inf(1)},
	} {
		fb := newFB(t, 10, 4)
		fb.DrawLineOnSurface(0, 1, z[0], 9, 1, z[1], Cell{Character: '#'})
		_, c := newTestCanvas(t, 10, 2)
		c.DrawLineOnSurface(0, 1, z[0], 9, 1, z[1], RGB(1, 2, 3))
		if got := markedCells(fb, Cell{Character: '#'}); got != 0 {
			t.Errorf("%s: wanted no cells, got %d", name, got)
		}
		for _, p := range c.pixels {
			if p != (Color{}) {
				t.Errorf("%s: wanted no pixels", name)
				break
			}
		}
	}
	// A line with one end infinitely far is drawn as far as it is not.
	fb := newFB(t, 10, 4)
	fb.DrawLineOnSurface(0, 1, 5, 9, 1, math.Inf(1), Cell{Character: '#'})
	if got := fb.At(0, 1); got != (Cell{Character: '#'}) {
		t.Errorf("wanted the near end drawn, got %+v", got)
	}
}

// surfaceSlack by hand, on grids whose depths are worked out on paper.
func TestSurfaceSlack(t *testing.T) {
	pinf := math.Inf(1)
	grid := func(w, h int, f func(x, y int) float64) []float64 {
		d := make([]float64, w*h)
		for y := range h {
			for x := range w {
				d[y*w+x] = f(x, y)
			}
		}
		return d
	}
	plane := grid(5, 3, func(x, y int) float64 { return 100 + 2*float64(x) + 3*float64(y) })
	silhouette := grid(5, 1, func(x, _ int) float64 { return []float64{10, 10, 10, 50, 50}[x] })
	pole := grid(5, 1, func(x, _ int) float64 { return []float64{10, 10, 5, 10, 10}[x] })
	empty := grid(3, 1, func(x, _ int) float64 { return []float64{pinf, 10, 10}[x] })
	flat := grid(3, 3, func(_, _ int) float64 { return 10 })

	for _, tt := range []struct {
		name    string
		depth   []float64
		w, h    int
		x, y    int
		want    float64
		comment string
	}{
		{"on a plane: a step of 2 along x and 3 along y", plane, 5, 3, 2, 1, slackPerStep * (2 + 3), ""},
		{"on a plane, at the left edge: only the cell after", plane, 5, 3, 0, 1, slackPerStep * (2 + 3), ""},
		{"on a plane, in a corner", plane, 5, 3, 4, 2, slackPerStep * (2 + 3), ""},
		{"on a flat surface: none", flat, 3, 3, 1, 1, 0, ""},
		{"before the edge of a nearer thing: the step of 40 is held to a quarter of the depth, 10", silhouette, 5, 1, 2, 0, slackCap * 10, ""},
		{"on the nearer thing: the same, a quarter of 50", silhouette, 5, 1, 3, 0, slackCap * 50, ""},
		{"a pole one cell wide: a step of 5 on both sides, held to a quarter of 5", pole, 5, 1, 2, 0, slackCap * 5, ""},
		{"a cell beside the pole: a step of 5 on one side, held to a quarter of 10", pole, 5, 1, 1, 0, slackCap * 10, ""},
		{"away from the pole: flat", pole, 5, 1, 0, 0, 0, ""},
		{"nothing stored: no slope", empty, 3, 1, 0, 0, 0, ""},
		{"next to nothing: only the other side is read", empty, 3, 1, 1, 0, 0, ""},
		{"a grid of one cell has no neighbor", []float64{10}, 1, 1, 0, 0, 0, ""},
	} {
		if got := surfaceSlack(tt.depth, tt.w, tt.h, tt.x, tt.y); math.Abs(got-tt.want) > 1e-12 {
			t.Errorf("%s: wanted %v, got %v", tt.name, tt.want, got)
		}
	}
}

// The slope is read from every side of a cell, the bottom row of the grid included: a surface whose
// depth changes only toward the row below is what lets the line through, and not reading that row, as if
// the grid ended above it, hides it.
func TestDrawLineOnSurfaceReadsTheWholeGrid(t *testing.T) {
	// Four rows, the first three at depth 10 and the last at 12: a line 12 away on row 2 is within the
	// slack of a step of 2 (3, held to a quarter of 10 to 2.5), and only the row below says so.
	fb := newFB(t, 3, 4)
	for y := range 4 {
		for x := range 3 {
			z := 10.0
			if y == 3 {
				z = 12
			}
			fb.SetDepth(x, y, z, Cell{Character: '.'})
		}
	}
	fb.DrawLineOnSurface(0, 2, 12, 2, 2, 12, Cell{Character: '#'})
	if got := markedCells(fb, Cell{Character: '#'}); got != 3 {
		t.Errorf("cells: wanted the 3 cells of the line, got %d", got)
	}

	_, c := newTestCanvas(t, 3, 2) // 3 by 4 pixels
	for y := range 4 {
		for x := range 3 {
			z := 10.0
			if y == 3 {
				z = 12
			}
			c.SetDepth(x, y, z, Palette(9))
		}
	}
	c.DrawLineOnSurface(0, 2, 12, 2, 2, 12, RGB(1, 2, 3))
	n := 0
	for _, p := range c.pixels {
		if p == RGB(1, 2, 3) {
			n++
		}
	}
	if n != 3 {
		t.Errorf("pixels: wanted the 3 pixels of the line, got %d", n)
	}
}

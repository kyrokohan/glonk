package engine

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// canvasOf returns the pixel layer of a new buffer w cells wide and h tall: w pixels
// wide and 2h tall.
func canvasOf(t *testing.T, w, h int) *Canvas {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "") // Seamless depends on the terminal running the tests
	return newFB(t, w, h).Canvas()
}

// drawnPixels says, for every pixel of the canvas in row order, whether it is not empty.
func drawnPixels(c *Canvas) []bool {
	out := make([]bool, len(c.pixels))
	for i, p := range c.pixels {
		out[i] = !p.IsDefault()
	}
	return out
}

// drawnCells says, for every cell of the buffer in row order, whether it holds mark.
func drawnCells(fb *FrameBuffer, mark Cell) []bool {
	out := make([]bool, len(fb.frame))
	for i, c := range fb.frame {
		out[i] = c == mark
	}
	return out
}

var (
	pixelRed  = RGB(220, 40, 40)
	pixelBlue = RGB(40, 60, 220)
	pixelGrn  = RGB(40, 200, 80)
	pixelYlw  = RGB(230, 210, 40)
)

// The pixels are the ones of the step-by-step Bresenham line, the oracle of walkLine,
// for every pair of ends in and around a small canvas: all directions and slopes, ties,
// lines inside, half inside, and wholly outside.
func TestCanvasDrawLineIsTheBresenhamLine(t *testing.T) {
	const w, h = 5, 3 // 5 by 6 pixels
	c := canvasOf(t, w, h)
	ch := c.Height()

	for x0 := -2; x0 < w+2; x0++ {
		for y0 := -2; y0 < ch+2; y0++ {
			for x1 := -2; x1 < w+2; x1++ {
				for y1 := -2; y1 < ch+2; y1++ {
					c.Clear()
					c.DrawLine(x0, y0, x1, y1, pixelRed)

					want := make([]bool, w*ch)
					for _, p := range windowOf(referenceLine(x0, y0, x1, y1), w, ch) {
						want[p.Y*w+p.X] = true
					}
					if got := drawnPixels(c); !slices.Equal(got, want) {
						t.Fatalf("line (%d,%d)-(%d,%d): wanted the Bresenham pixels %v, got %v", x0, y0, x1, y1, want, got)
					}
				}
			}
		}
	}
}

// A canvas line is the line a buffer as big as the canvas draws, whatever the ends:
// reversed, a point, far beyond the canvas, and at the limits of an int.
func TestCanvasDrawLineIsTheLineAFrameBufferDraws(t *testing.T) {
	const w, h = 12, 7
	rng := rand.New(rand.NewPCG(11, 12))
	mark := Cell{Character: '#'}
	coord := func() int {
		switch rng.IntN(8) {
		case 0:
			return rng.IntN(2_000_000_000) - 1_000_000_000
		case 1:
			return math.MaxInt - rng.IntN(5)
		case 2:
			return math.MinInt + rng.IntN(5)
		}
		return rng.IntN(60) - 20
	}

	c := canvasOf(t, w, h)
	for range 3000 {
		x0, y0, x1, y1 := coord(), coord(), coord(), coord()
		c.Clear()
		same := newFB(t, w, 2*h)

		c.DrawLine(x0, y0, x1, y1, pixelRed)
		same.DrawLine(x0, y0, x1, y1, mark)

		if got, want := drawnPixels(c), drawnCells(same, mark); !slices.Equal(got, want) {
			t.Fatalf("line (%d,%d)-(%d,%d): wanted the pixels a buffer of the same size draws\n%s\ngot\n%v", x0, y0, x1, y1, same.String(), got)
		}
	}
}

// Every pixel of a canvas line has the depth of the same cell of the line a buffer of
// the canvas's size draws, over lines with random depths and ends far off the canvas.
func TestCanvasDrawLineDepthStoresWhatAFrameBufferStores(t *testing.T) {
	const w, h = 12, 7
	rng := rand.New(rand.NewPCG(13, 14))
	mark := Cell{Character: '#'}
	coord := func() int {
		if rng.IntN(6) == 0 {
			return rng.IntN(2_000_000_000) - 1_000_000_000
		}
		return rng.IntN(60) - 20
	}
	depth := func() float64 {
		switch rng.IntN(12) {
		case 0:
			return 0
		case 1:
			return math.NaN()
		case 2:
			return -1
		case 3:
			return math.Inf(1)
		}
		return 0.1 + rng.Float64()*50
	}

	c := canvasOf(t, w, h)
	for range 4000 {
		x0, y0, z0 := coord(), coord(), depth()
		x1, y1, z1 := coord(), coord(), depth()
		c.Clear()
		same := newFB(t, w, 2*h)

		c.DrawLineDepth(x0, y0, z0, x1, y1, z1, pixelRed)
		same.DrawLineDepth(x0, y0, z0, x1, y1, z1, mark)

		if !slices.Equal(drawnPixels(c), drawnCells(same, mark)) {
			t.Fatalf("line (%d,%d,%v)-(%d,%d,%v): wanted the pixels a buffer of the same size draws", x0, y0, z0, x1, y1, z1)
		}
		if !slices.Equal(c.depth, same.depth) {
			t.Fatalf("line (%d,%d,%v)-(%d,%d,%v): wanted the depths a buffer of the same size stores", x0, y0, z0, x1, y1, z1)
		}
	}
}

// A line with the same depth at both ends keeps it; with different depths it is
// perspective-correct: halfway from 1 to 3 away is 1.5, because 1/z runs evenly. The ends
// are exact, and a line that is a point is as near as its nearer end.
func TestCanvasDrawLineDepthIsPerspectiveCorrectAndExactAtTheEnds(t *testing.T) {
	c := canvasOf(t, 9, 2)

	c.DrawLineDepth(0, 1, 1, 8, 1, 3, pixelRed)

	if got := c.depth[1*c.width+0]; got != 1 {
		t.Errorf("wanted the first end exactly 1 away, got %v", got)
	}
	if got := c.depth[1*c.width+8]; got != 3 {
		t.Errorf("wanted the last end exactly 3 away, got %v", got)
	}
	if got := c.depth[1*c.width+4]; math.Abs(got-1.5) > 1e-12 {
		t.Errorf("wanted halfway 1.5 away (perspective-correct, not 2), got %v", got)
	}

	c.Clear()
	c.DrawLineDepth(3, 2, 9, 3, 2, 4, pixelBlue)
	if got := c.depth[2*c.width+3]; got != 4 {
		t.Errorf("wanted a point to be as near as its nearer end, 4, got %v", got)
	}
}

// The depth is the depth of the whole line, not of the part that is on the canvas.
func TestCanvasDrawLineDepthKeepsTheDepthOfTheWholeLine(t *testing.T) {
	c := canvasOf(t, 8, 2)

	// From x = -8 to x = 8 the line goes from 1 to 3 away; x = 0 is the halfway point.
	c.DrawLineDepth(-8, 0, 1, 8, 0, 3, pixelRed)

	if got := c.depth[0]; math.Abs(got-1.5) > 1e-12 {
		t.Errorf("wanted the first pixel on the canvas 1.5 away, the depth halfway along the whole line, got %v", got)
	}
}

// A nearer pixel replaces what is there, a farther one does not, and a tie keeps what is
// there. The pixel gets its color and its depth together.
func TestCanvasDrawLineDepthNearerWinsAndATieKeeps(t *testing.T) {
	c := canvasOf(t, 10, 3)
	at := func(x int) Color { return c.At(x, 2) }

	c.DrawLineDepth(0, 2, 5, 9, 2, 5, pixelRed)
	c.DrawLineDepth(0, 2, 5, 9, 2, 5, pixelBlue)
	if at(0) != pixelRed || at(9) != pixelRed {
		t.Errorf("wanted a line at the same depth to leave the red, got %v and %v", at(0), at(9))
	}

	c.DrawLineDepth(0, 2, 4, 9, 2, 4, pixelGrn)
	if at(0) != pixelGrn || at(9) != pixelGrn {
		t.Errorf("wanted a nearer line to replace the red, got %v and %v", at(0), at(9))
	}
	if got := c.depth[2*c.width+4]; got != 4 {
		t.Errorf("wanted the pixel to hold the nearer depth 4, got %v", got)
	}

	c.DrawLineDepth(0, 2, 6, 9, 2, 6, pixelYlw)
	if at(0) != pixelGrn || at(9) != pixelGrn || c.depth[2*c.width+4] != 4 {
		t.Errorf("wanted a farther line to change nothing, got %v and %v", at(0), at(9))
	}

	// A line that crosses another is hidden where it is farther and shows where nearer.
	c.Clear()
	c.DrawLineDepth(0, 0, 4, 9, 0, 4, pixelRed)  // along the top row, 4 away
	c.DrawLineDepth(0, 0, 2, 0, 5, 8, pixelBlue) // down the left column, 2 at the top, 8 at the bottom
	if c.At(0, 0) != pixelBlue {
		t.Errorf("wanted the blue (2 away) over the red (4 away) at the corner, got %v", c.At(0, 0))
	}
}

// A depth that is not positive, or is NaN, draws nothing, and an infinite depth is
// infinitely far: nothing is nearer than it.
func TestCanvasDrawLineDepthWithABadDepthDrawsNothing(t *testing.T) {
	for _, tt := range []struct {
		name   string
		z0, z1 float64
	}{
		{"zero", 0, 2},
		{"negative", 2, -1},
		{"NaN", math.NaN(), 2},
		{"infinite", math.Inf(1), math.Inf(1)},
		{"zero at both", 0, 0},
	} {
		c := canvasOf(t, 8, 2)
		c.DrawLineDepth(0, 0, tt.z0, 7, 3, tt.z1, pixelRed)

		for i, p := range c.pixels {
			if !p.IsDefault() || !math.IsInf(c.depth[i], 1) {
				t.Errorf("%s: wanted nothing drawn, got a pixel at %d with depth %v", tt.name, i, c.depth[i])
				break
			}
		}
	}
}

// DrawLine is Set along a line: it replaces what is there whatever its depth, and it
// neither reads nor writes depth. The zero Color erases.
func TestCanvasDrawLineIgnoresDepthAndTheZeroColorErases(t *testing.T) {
	c := canvasOf(t, 6, 2)

	c.SetDepth(2, 1, 1, pixelRed) // a nearer pixel
	c.DrawLine(0, 1, 5, 1, pixelBlue)
	if c.At(2, 1) != pixelBlue {
		t.Errorf("wanted DrawLine to replace a pixel however near it is, got %v", c.At(2, 1))
	}
	if got := c.depth[1*c.width+2]; got != 1 {
		t.Errorf("wanted DrawLine to leave that pixel's depth 1, got %v", got)
	}
	for x := range c.width {
		if x != 2 && !math.IsInf(c.depth[1*c.width+x], 1) {
			t.Fatalf("wanted DrawLine to write no depth, got %v at %d", c.depth[1*c.width+x], x)
		}
	}

	// So a line with depth, drawn after, still goes over a pixel DrawLine drew on an empty spot.
	c.DrawLineDepth(0, 1, 100, 5, 1, 100, pixelGrn)
	if c.At(0, 1) != pixelGrn {
		t.Errorf("wanted a line with depth to land on a pixel DrawLine drew, got %v", c.At(0, 1))
	}

	c.DrawLine(0, 1, 5, 1, Color{})
	for x := range c.width {
		if !c.At(x, 1).IsDefault() {
			t.Fatalf("wanted the zero Color to erase the line, got %v at %d", c.At(x, 1), x)
		}
	}
}

// After Compose the cells a pixel line fills have the depth of their nearer pixel, so
// depth-tested drawing into the cells is hidden behind the line where it is nearer.
func TestComposeGivesACellItsNearerLinePixelsDepth(t *testing.T) {
	const w, h = 6, 3
	fb := newFB(t, w, h)
	t.Setenv("TERM_PROGRAM", "")
	c := fb.Canvas()

	// Down the column x = 3, from the top pixel (1 away) to the bottom one (5 away).
	c.DrawLineDepth(3, 0, 1, 3, 5, 5, pixelRed)
	c.Compose()

	for row := range h {
		upper := c.depth[(2*row)*c.width+3]
		if got := cellDepth(fb, 3, row); got != upper {
			t.Errorf("row %d: wanted the cell to take its nearer (upper) pixel's depth %v, got %v", row, upper, got)
		}
	}
	if fb.SetDepth(3, 1, 100, Cell{Character: '#'}) {
		t.Errorf("wanted a cell drawn behind the line to be hidden")
	}
	if !fb.SetDepth(3, 1, 0.5, Cell{Character: '#'}) {
		t.Errorf("wanted a cell drawn in front of the line to show")
	}
}

func BenchmarkCanvasDrawLine(b *testing.B) {
	fb, err := NewFrameBuffer(200, 60)
	if err != nil {
		b.Fatal(err)
	}
	c := fb.Canvas()

	b.Run("DrawLine", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.Clear()
			c.DrawLine(0, 0, 199, 119, pixelRed)
			c.DrawLine(10, 100, 190, 6, pixelRed)
			c.DrawLine(-300, 60, 500, 62, pixelRed)
		}
	})
	b.Run("DrawLineDepth", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.Clear()
			c.DrawLineDepth(0, 0, 1, 199, 119, 9, pixelRed)
			c.DrawLineDepth(10, 100, 2, 190, 6, 6, pixelRed)
			c.DrawLineDepth(-300, 60, 1, 500, 62, 20, pixelRed)
		}
	})
}

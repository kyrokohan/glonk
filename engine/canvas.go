package engine

import (
	"math"
	"os"
)

// Block characters for pixel pairs. The upper half block paints the top half of a
// cell in the character color and the bottom half in the background color.
const (
	upperHalfBlock = '▀'
	lowerHalfBlock = '▄'
)

// Canvas is a grid of pixels twice as tall as the FrameBuffer it belongs to: each
// terminal cell holds two stacked pixels, which Compose turns into half-block
// characters. Cells are about twice as tall as wide, so the pixels come out roughly
// square. A pixel is a Color, and the zero Color means empty, so every other color, black
// included, is a pixel.
//
// Get one with FrameBuffer.Canvas. It keeps its own depth per pixel, separate from
// the FrameBuffer's, so solids drawn on a Canvas hide each other, but not the cells
// until Compose: it then gives every cell it fills the depth of its nearer pixel, and
// depth-tested drawing into cells afterwards (FrameBuffer.FillTriangle,
// FrameBuffer.DrawLineDepth, FrameBuffer.SetDepth) is hidden behind the pixels where
// they are nearer.
type Canvas struct {
	// Seamless makes Compose draw solid cells only, with no half-block characters.
	// Some terminals draw those characters a little short of the cell and leave a
	// visible line along every edge; Apple's Terminal does. Solid cells cannot show
	// a line, at the price of silhouettes that are a pixel blockier. It starts out
	// true under Apple's Terminal (TERM_PROGRAM=Apple_Terminal) and false elsewhere;
	// set it yourself to override, for instance inside tmux, which hides that variable.
	Seamless bool

	fb            *FrameBuffer
	width, height int
	pixels        []Color
	depth         []float64
}

// terminalLeavesSeams reports whether the terminal is known to draw half-block
// characters with visible gaps. Only Apple's Terminal is known so far.
func terminalLeavesSeams() bool {
	return os.Getenv("TERM_PROGRAM") == "Apple_Terminal"
}

func newCanvas(fb *FrameBuffer) *Canvas {
	c := &Canvas{
		Seamless: terminalLeavesSeams(),
		fb:       fb,
		width:    fb.width,
		height:   fb.height * 2,
		pixels:   make([]Color, fb.width*fb.height*2),
		depth:    make([]float64, fb.width*fb.height*2),
	}
	c.Clear()
	return c
}

// Canvas returns the pixel layer of this FrameBuffer, creating it on first use.
// It is cleared whenever the FrameBuffer is.
func (fb *FrameBuffer) Canvas() *Canvas {
	if fb.canvas == nil {
		fb.canvas = newCanvas(fb)
	}
	return fb.canvas
}

// Width returns the number of pixel columns, the same as the FrameBuffer's.
func (c *Canvas) Width() int {
	return c.width
}

// Height returns the number of pixel rows, twice the FrameBuffer's.
func (c *Canvas) Height() int {
	return c.height
}

// Clear empties every pixel and resets every depth to infinitely far.
func (c *Canvas) Clear() {
	clear(c.pixels)
	for i := range c.depth {
		c.depth[i] = math.Inf(1)
	}
}

func (c *Canvas) inBounds(x, y int) bool {
	return x >= 0 && x < c.width && y >= 0 && y < c.height
}

// Set stores color at pixel (x, y). Writes outside the canvas are ignored.
func (c *Canvas) Set(x, y int, color Color) {
	if !c.inBounds(x, y) {
		return
	}
	c.pixels[y*c.width+x] = color
}

// At returns the color at pixel (x, y), or the zero Color (empty) outside the canvas.
func (c *Canvas) At(x, y int) Color {
	if !c.inBounds(x, y) {
		return Color{}
	}
	return c.pixels[y*c.width+x]
}

// SetDepth stores color at pixel (x, y) only if z is nearer than what is already
// there, and reports whether it did. Smaller z is nearer; a tie keeps the existing
// pixel. Writes outside the canvas, and a NaN z, are ignored. Set never reads or
// writes depth.
func (c *Canvas) SetDepth(x, y int, z float64, color Color) bool {
	if !c.inBounds(x, y) {
		return false
	}

	i := y*c.width + x
	// Written as !(z < ...) so a NaN fails the test instead of slipping through.
	if !(z < c.depth[i]) {
		return false
	}

	c.depth[i] = z
	c.pixels[i] = color
	return true
}

// DrawLine draws a line of color from pixel (x0, y0) to (x1, y1): the line
// FrameBuffer.DrawLine draws, but into pixels, so twice as fine from top to bottom. Only
// the part inside the canvas is visited, so a line with a far-away end costs no more
// than one that stays on screen, and coordinates beyond +-2^29 are clamped to it.
//
// Like Set it never reads or writes depth: the pixels are replaced whatever is there,
// and a solid or a line drawn afterwards with depth is hidden only by the depth that was
// there before. The zero Color erases the pixels it passes through.
func (c *Canvas) DrawLine(x0, y0, x1, y1 int, color Color) {
	walkLineAt(c.width, c.height, x0, y0, x1, y1, func(x, y, _, _ int) {
		// walkLineAt only visits pixels inside the canvas.
		c.pixels[y*c.width+x] = color
	})
}

// DrawLineDepth draws the same line as DrawLine, but each pixel is stored with
// SetDepth, so a nearer surface hides it and it hides a farther one: a pixel is drawn
// where the line is nearer than what is there, and where it ties, what is there stays.
// z0 and z1 are the distances from the camera of the two ends, as for FillTriangle, and
// must be positive. The depth along the line is the one FrameBuffer.DrawLineDepth gives
// its cells, perspective-correct and exact at the ends; read there what it is and what a
// depth that is not positive, NaN or infinite does.
//
// The depth is the Canvas's own, so a line drawn here is hidden by the solids drawn into
// the Canvas and hides those behind it, in whatever order they were drawn. Once Compose
// has run, the cells it filled have the depth of their nearer pixel.
func (c *Canvas) DrawLineDepth(x0, y0 int, z0 float64, x1, y1 int, z1 float64, color Color) {
	d, ok := newLineDepth(z0, z1)
	if !ok {
		return
	}
	walkLineAt(c.width, c.height, x0, y0, x1, y1, func(x, y, i, n int) {
		c.SetDepth(x, y, d.at(i, n), color)
	})
}

// FillTriangle fills the triangle with corners (x0, y0), (x1, y1), (x2, y2) in
// pixel coordinates, like FrameBuffer.FillTriangle but into pixels. The z values
// are the corners' distances from the camera and must be positive.
func (c *Canvas) FillTriangle(x0, y0, z0, x1, y1, z1, x2, y2, z2 float64, color Color) {
	scanTriangle(c.width, c.height, x0, y0, z0, x1, y1, z1, x2, y2, z2, func(x, y int, z, _, _, _ float64) {
		c.SetDepth(x, y, z, color)
	})
}

// FillTriangleFunc fills the triangle like FillTriangle, but asks color what to draw
// in each covered pixel, with the weights of the three corners there. See
// FrameBuffer.FillTriangleFunc for what the weights are and for the order of the
// depth test and the call: color is never asked about a hidden pixel. A nil color
// draws nothing.
func (c *Canvas) FillTriangleFunc(x0, y0, z0, x1, y1, z1, x2, y2, z2 float64, color func(b0, b1, b2 float64) Color) {
	if color == nil {
		return
	}
	scanTriangle(c.width, c.height, x0, y0, z0, x1, y1, z1, x2, y2, z2, func(x, y int, z, p0, p1, p2 float64) {
		// scanTriangle only visits pixels inside the canvas.
		i := y*c.width + x
		if !(z < c.depth[i]) {
			return
		}
		col := color(p0*z, p1*z, p2*z)
		c.depth[i] = z
		c.pixels[i] = col
	})
}

// Compose writes the pixels into the FrameBuffer's cells as half-block characters,
// or as solid cells if Seamless is set. Cells whose two pixels are both empty are
// left alone; every other cell is replaced, and takes the depth of its nearer pixel
// in place of whatever depth it had. Call it at the point in drawing where the pixels
// should sit: anything drawn into the FrameBuffer afterwards lands on top of them,
// unless it is drawn with a depth (FrameBuffer.FillTriangle, FrameBuffer.DrawLineDepth,
// FrameBuffer.SetDepth) and is farther than the pixels there.
//
// A pixel set with Set has no depth, so it is as far away as can be: depth-tested
// drawing afterwards lands on top of it.
func (c *Canvas) Compose() {
	fb := c.fb
	for row := range fb.height {
		for x := range fb.width {
			topIdx, bottomIdx := (2*row)*c.width+x, (2*row+1)*c.width+x
			top, bottom := c.pixels[topIdx], c.pixels[bottomIdx]
			cellIdx := row*fb.width + x

			// The depth of the cell is its nearer pixel's. A pixel that was erased leaves
			// its old depth behind, so only a pixel that is there counts.
			z := math.Inf(1)
			if !top.IsDefault() {
				z = c.depth[topIdx]
			}
			if !bottom.IsDefault() {
				z = min(z, c.depth[bottomIdx])
			}

			if c.Seamless && (!top.IsDefault() || !bottom.IsDefault()) {
				// One color for the whole cell: the nearer pixel, or the only one set.
				color := top
				if top.IsDefault() || (!bottom.IsDefault() && c.depth[bottomIdx] < c.depth[topIdx]) {
					color = bottom
				}
				fb.frame[cellIdx] = Cell{Character: ' ', Background: color}
				fb.depth[cellIdx] = z
				continue
			}

			var cell Cell
			switch {
			case top.IsDefault() && bottom.IsDefault():
				continue
			case top == bottom:
				// A space on a background is solid however the font draws blocks.
				cell = Cell{Character: ' ', Background: top}
			case !top.IsDefault() && !bottom.IsDefault():
				cell = Cell{Character: upperHalfBlock, Foreground: top, Background: bottom}
			case !top.IsDefault():
				cell = Cell{Character: upperHalfBlock, Foreground: top}
			default:
				cell = Cell{Character: lowerHalfBlock, Foreground: bottom}
			}
			fb.frame[cellIdx] = cell
			fb.depth[cellIdx] = z
		}
	}
}

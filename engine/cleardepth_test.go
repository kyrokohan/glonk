package engine

import (
	"math"
	"slices"
	"testing"
)

// allFar says whether every depth is infinitely far.
func allFar(depth []float64) bool {
	return !slices.ContainsFunc(depth, func(z float64) bool { return !math.IsInf(z, 1) })
}

func TestCanvasClearDepthKeepsThePixels(t *testing.T) {
	c := canvasOf(t, 4, 2)
	c.SetDepth(1, 1, 2, pixelRed)
	c.SetDepth(2, 3, 0.5, pixelBlue)
	c.Set(3, 0, pixelGrn)
	before := slices.Clone(c.pixels)

	c.ClearDepth()

	if !slices.Equal(c.pixels, before) {
		t.Errorf("wanted the pixels kept")
	}
	if !allFar(c.depth) {
		t.Errorf("wanted every depth infinitely far, got %v", c.depth)
	}
	if !c.SetDepth(2, 3, 9, pixelYlw) {
		t.Errorf("wanted a pixel farther than the one there drawn over it")
	}
}

func TestFrameBufferClearDepthKeepsTheCellsAndClearsBothDepths(t *testing.T) {
	fb := newFB(t, 4, 2)
	fb.SetDepth(1, 1, 2, Cell{Character: 'a'})
	fb.Set(3, 0, Cell{Character: 'b'})
	c := fb.Canvas()
	c.SetDepth(0, 3, 1, pixelRed)
	cells, pixels := slices.Clone(fb.frame), slices.Clone(c.pixels)

	fb.ClearDepth()

	if !slices.Equal(fb.frame, cells) || !slices.Equal(c.pixels, pixels) {
		t.Errorf("wanted the cells and the pixels kept")
	}
	if !allFar(fb.depth) || !allFar(c.depth) {
		t.Errorf("wanted every depth of the cells and of the canvas infinitely far")
	}
}

// A frame that has not used its canvas does not get one from ClearDepth.
func TestFrameBufferClearDepthMakesNoCanvas(t *testing.T) {
	fb := newFB(t, 3, 2)
	fb.SetDepth(0, 0, 1, Cell{Character: 'a'})
	fb.ClearDepth()
	if fb.canvas != nil {
		t.Errorf("wanted no canvas made")
	}
	if !allFar(fb.depth) {
		t.Errorf("wanted every depth infinitely far")
	}
}

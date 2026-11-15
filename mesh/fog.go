package mesh

import (
	"math"

	"glonk.dev/engine"
)

// Fog fades what is far away toward one color, so that a world does not end in a hard line
// and things do not appear all at once as the camera comes near. Set it as Camera.Fog:
// DrawShaded and DrawShadedPixels then fade every cell and pixel they draw by how deep it is.
// A program that draws things of its own into the scene calls Apply, to fade them in step.
//
// The distance is a depth, along the camera's view direction (Look): the same number as
// Camera.Far, Canvas.SetDepth, the depth ProjectPixels returns and the t of a Ray. So it does
// not change as the camera turns, but a thing at the side of the view is a little less foggy
// than one the same straight-line distance away in the middle of it.
//
// In cells the color of a mesh fades and the character, which shows its brightness, stays; in
// pixels the shaded color fades. Wireframes are not fogged. To hide where the world ends, give
// the camera a Far, and a Fog that is complete at or before it.
type Fog struct {
	// Color is what everything fades to, read as engine.Mix reads a color.
	Color engine.Color

	// Start is the depth where the fade begins, and End the depth where it is complete.
	// Between them it is a straight line in the color.
	Start, End float64
}

// Apply returns c as it looks in the fog at depth. Where the fog has not begun, depth up to
// Start, it is c itself, so what is nearer is exactly as it is without fog; from End on it is
// the fog's Color itself; and between them c is mixed toward it in a straight line (see
// engine.Mix). A depth that is NaN, a Start or End that is NaN, and a fog that is infinitely
// far have no effect. An End that is not beyond Start makes a hard step: complete beyond Start.
func (f Fog) Apply(c engine.Color, depth float64) engine.Color {
	switch a := f.amount(depth); {
	case a <= 0:
		return c
	case a >= 1:
		return f.Color
	default:
		return engine.Mix(c, f.Color, a)
	}
}

// amount is how much of the way to the fog's color a thing at depth is, from 0 to 1.
func (f Fog) amount(depth float64) float64 {
	switch {
	case math.IsNaN(f.End) || !(depth > f.Start): // written so that a NaN depth or Start fails the test
		return 0
	case depth >= f.End || !(f.End > f.Start):
		return 1
	}
	a := (depth - f.Start) / (f.End - f.Start)
	if math.IsNaN(a) { // an infinite Start or End on both sides
		return 0
	}
	return a
}

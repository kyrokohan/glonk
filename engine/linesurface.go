package engine

import "math"

// How far behind what is stored a line that lies on a surface may be, in steps of the
// surface's own slope: slackPerStep times the depth that the surface changes by from one cell
// to the next, along both axes. A line's cells come from end points that are rounded down to
// cells, so a cell can lie up to a cell off the true line, and across a surface seen at a low
// angle that is a lot of depth. slackCap, a part of the depth stored there, is the most that is
// ever allowed: a line farther behind a surface than that is hidden whatever the slope, which is
// what keeps a step across the edge of a nearer object, or a thin thing, from reading as a slope
// that lets a line behind it through.
const (
	slackPerStep = 1.5
	slackCap     = 0.25
)

// surfaceSlack is how much farther than the depth stored at cell (x, y) of a w by h grid a
// line may be and still count as lying on the surface that is there, worked out from the depths
// stored around it. On each axis it is the larger of the two steps to the cell before and after,
// the ones that are not off the grid or infinite: where two surfaces meet, as a cube and the floor
// it stands on do, the steeper of them is the one that says how far off a cell can be, and the
// other side is flat. A cell with nothing stored in it, or with no neighbor to step to, has no slope: 0.
func surfaceSlack(depth []float64, w, h, x, y int) float64 {
	i := y*w + x
	z := depth[i]
	if math.IsInf(z, 1) {
		return 0
	}
	// step is how far the depth at j is from z, or 0 where there is none: off the grid, or nothing stored.
	step := func(ok bool, j int) float64 {
		if !ok || math.IsInf(depth[j], 1) {
			return 0
		}
		return math.Abs(z - depth[j])
	}
	sx := max(step(x > 0, i-1), step(x < w-1, i+1))
	sy := max(step(y > 0, i-w), step(y < h-1, i+w))
	return min(slackPerStep*(sx+sy), slackCap*z)
}

// laidOn says whether a line cell z away is drawn on the grid of depths, and the depth to keep
// there. It is drawn if it is no farther than what is stored plus the slack of that surface. If it is
// that much nearer than what is stored, it is in front of the surface and keeps its own depth, as a
// line of DrawLineDepth does, so what is drawn later and is behind it is still hidden. If it is not,
// it lies on the surface and keeps the depth that was there: the depths around a pixel are what say how
// steep the surface is, and a line that stored its own would blur that for the cells after it. A line
// that is infinitely far is not drawn: nothing is nearer than it.
func laidOn(depth []float64, w, h, x, y int, z float64) (keep float64, draw bool) {
	if math.IsInf(z, 1) {
		return 0, false
	}
	stored := depth[y*w+x]
	slack := surfaceSlack(depth, w, h, x, y)
	if !(z <= stored+slack) {
		return 0, false
	}
	if z < stored-slack {
		return z, true
	}
	return stored, true
}

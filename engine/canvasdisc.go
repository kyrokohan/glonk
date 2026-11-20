package engine

import "math"

// discSamples is how many samples each side of a pixel has for a disc: a pixel has 64, and its
// share of a disc is in steps of 1/64.
const discSamples = 8

// discOffsets are where the samples sit across and down a pixel, from its left or top edge: the
// middles of an 8 by 8 grid, (2k+1)/16, which are exact in binary.
var discOffsets = [discSamples]float64{1.0 / 16, 3.0 / 16, 5.0 / 16, 7.0 / 16, 9.0 / 16, 11.0 / 16, 13.0 / 16, 15.0 / 16}

// FillDisc fills the disc of radius r around the point (cx, cy) with color, with a smooth edge: a
// ball, a bullet, a planet or a particle, drawn where it is and not on the nearest pixel. Pixel
// (x, y) is the square from (x, y) to (x+1, y+1), so its middle is (x+0.5, y+0.5), and the
// coordinates are those of mesh.Camera.ProjectPixels, so a projected point is a center as it is.
// FillDisc(cx+0.5, cy+0.5, r+0.5) is close to FillCircle(cx, cy, r).
//
// A pixel's share of the disc is how many of 64 points in it, on an 8 by 8 grid, are within r of
// the center. A pixel the disc covers wholly is color, as FillCircle draws it, and the zero Color
// erases it. A pixel on the edge is mixed toward color by its share, as Mix mixes, with the zero
// Color read as black. An empty pixel on the edge has nothing to mix with, since the terminal's own
// background is not known, so it is color where its share is a half or more and stays empty below
// that: a disc on an empty canvas shows its size and place, with a hard edge there. Fill a
// background first for a smooth one.
//
// A radius that is not positive, or NaN, draws nothing, as does a center that is not a finite
// number, and an infinite radius covers the canvas. The shares are worked out the same way on
// every platform, so the same pixels are drawn everywhere. Only the part inside the canvas is
// visited, and nothing is allocated, but each pixel is looked at on its own, so a disc costs
// more than FillCircle: about 1 microsecond at a radius of 2.5 and 60 at 50.5, where
// FillCircle takes 0.03 and 3.5. Like FillCircle it never reads or writes depth.
func (c *Canvas) FillDisc(cx, cy, r float64, color Color) {
	c.disc(cx, cy, r, color, 1, true)
}

// BlendDisc blends color over the disc FillDisc would fill, by the part t from 0 to 1, as
// BlendRect blends over a rectangle: a pixel that is not empty becomes Mix(pixel, color, share*t),
// with its share of the disc as for FillDisc, for a shadow, a glow or a faint copy of a thing. As
// for BlendRect, an empty pixel stays empty, t outside 0 to 1 is clamped and NaN is 0, the zero
// Color is read as black and does not erase, and a pixel the disc touches becomes an exact RGB
// color. The rest is as for FillDisc.
func (c *Canvas) BlendDisc(cx, cy, r float64, color Color, t float64) {
	c.disc(cx, cy, r, color, clamp01(t), false)
}

// disc draws the disc of FillDisc, or of BlendDisc by the part t; fill says whether a pixel the disc
// covers wholly, or an empty one it covers half of, takes the color.
func (c *Canvas) disc(cx, cy, r float64, color Color, t float64, fill bool) {
	if !(r > 0) || math.IsNaN(cx) || math.IsNaN(cy) || math.IsInf(cx, 0) || math.IsInf(cy, 0) {
		return
	}
	r2 := float64(r * r)

	// Clamped as floats before converting, since a far or huge disc has no valid int, and a pixel
	// wider on each side so that the rounding of cx-r and cx+r can never leave one out.
	fw, fh := float64(c.width), float64(c.height)
	left := int(max(0, min(math.Floor(cx-r)-1, fw)))
	right := int(max(-1, min(math.Floor(cx+r)+1, fw-1)))
	top := int(max(0, min(math.Floor(cy-r)-1, fh)))
	bottom := int(max(-1, min(math.Floor(cy+r)+1, fh-1)))

	const whole = discSamples * discSamples
	cr, cg, cb := color.RGB()
	for py := top; py <= bottom; py++ {
		dy := float64(py) - cy
		for px := left; px <= right; px++ {
			n := discShare(float64(px)-cx, dy, r2)
			if n == 0 {
				continue
			}
			i := py*c.width + px
			p := c.pixels[i]
			switch {
			case fill && (n == whole || (p.IsDefault() && 2*n >= whole)):
				c.pixels[i] = color
			case p.IsDefault():
				// Nothing to mix with.
			default:
				s := float64(n) / whole * t
				pr, pg, pb := p.RGB()
				c.pixels[i] = RGB(mixChannel(pr, cr, s), mixChannel(pg, cg, s), mixChannel(pb, cb, s))
			}
		}
	}
}

// discShare counts the samples of a pixel that are within the disc, from 0 to 64: dx and dy are
// how far the pixel's top left corner is from the center, and r2 is the radius squared.
//
// Every square is rounded on its own, by the explicit conversion, which Go may not fuse into a
// multiply-add, so the count is the same on every platform. A pixel whose nearest point is beyond
// the disc, or whose farthest sample is within it, is not sampled one by one: rounding keeps the
// order of distances, so both answers are what counting would give.
func discShare(dx, dy, r2 float64) int {
	nx, ny := max(dx, -(dx+1), 0), max(dy, -(dy+1), 0)
	if float64(nx*nx)+float64(ny*ny) > r2 {
		return 0
	}
	fx := max(math.Abs(dx+discOffsets[0]), math.Abs(dx+discOffsets[discSamples-1]))
	fy := max(math.Abs(dy+discOffsets[0]), math.Abs(dy+discOffsets[discSamples-1]))
	if float64(fx*fx)+float64(fy*fy) <= r2 {
		return discSamples * discSamples
	}

	n := 0
	for _, oy := range discOffsets {
		sy := dy + oy
		sy2 := float64(sy * sy)
		for _, ox := range discOffsets {
			if sx := dx + ox; float64(sx*sx)+sy2 <= r2 {
				n++
			}
		}
	}
	return n
}

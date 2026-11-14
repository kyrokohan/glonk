package mesh

import (
	"math"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// Camera views the world from Position, looking down +z when Yaw and Pitch are 0.
// The fields say which way each angle turns; all angles are in radians.
//
// A FOV, Near or CellAspect left at 0 uses a default (a FOV of pi/2, a quarter turn,
// a Near of 0.1 and a CellAspect of 2), so Camera{Position: p} is a working camera. A
// value that makes no sense, namely a FOV outside 0 to pi (exclusive), a Near that is
// negative, NaN or infinite, or a CellAspect that is negative, NaN or infinite, makes
// the camera see nothing: Draw, DrawShaded and DrawShadedPixels draw nothing and
// Project reports ok=false. So does a Position, Yaw or Pitch that is not a finite
// number.
type Camera struct {
	// Position is where the camera is, in world coordinates.
	Position geometry.Vector3

	// Yaw turns the camera about the vertical axis, in radians. Positive turns it
	// right, toward +x.
	Yaw float64

	// Pitch tilts the camera, in radians. Positive tilts it up. It is not limited;
	// keep it inside +-pi/2 or the view flips over.
	Pitch float64

	// FOV is the vertical field of view in radians, between 0 and pi. 0 means pi/2.
	FOV float64

	// Near is the distance from the camera, along its view direction, of the plane
	// nearer than which everything is cut away. It must be above 0; 0 means 0.1.
	Near float64

	// CellAspect is how many times taller a terminal cell is than it is wide, which
	// is what makes a cube look like a cube and not a slab. Most fonts are close to 2,
	// which is what 0 means. A larger value draws things wider, in more columns, and a
	// smaller one narrower. It must be above 0.
	CellAspect float64
}

// What a zero FOV, Near and CellAspect stand for.
const (
	defaultFOV        = math.Pi / 2
	defaultNear       = 0.1
	defaultCellAspect = 2
)

// withDefaults returns the camera as it is used: a zero FOV, Near or CellAspect
// replaced by its default. ok is false when a value makes no sense and nothing can be
// drawn.
func (c Camera) withDefaults() (cam Camera, ok bool) {
	if c.FOV == 0 {
		c.FOV = defaultFOV
	}
	if c.Near == 0 {
		c.Near = defaultNear
	}
	if c.CellAspect == 0 {
		c.CellAspect = defaultCellAspect
	}
	// Written so that NaN fails every test. An infinite Near passes, and sees nothing.
	ok = c.FOV > 0 && c.FOV < math.Pi && c.Near > 0 && c.CellAspect > 0 && !math.IsInf(c.CellAspect, 0)
	return c, ok
}

// Project returns where p appears on a grid of w x h cells, as fractional cell
// coordinates; the caller is expected to floor them. ok is false when p is nearer
// than Near or behind the camera, has no finite place on the screen, or the camera
// sees nothing at all (see Camera).
func (c Camera) Project(p geometry.Vector3, w, h int) (x, y float64, ok bool) {
	x, y, _, ok = c.projectDepth(p, w, h)
	return x, y, ok
}

// ProjectPixels returns where p appears on the pixels of cv, and how deep it is, placed and
// stored the way DrawShadedPixels, DrawPixels and DrawDepthPixels place and store what they
// draw. It is Project on the grid the canvas belongs to, with y stretched to pixel rows.
//
// x and y are fractional pixel coordinates. Floor them to get the pixel p is in, with
// math.Floor: converting to int rounds a negative number toward zero, which would put a
// point just off the left or top edge on the canvas.
//
// depth is the distance from the camera along Look, which is what those methods give
// Canvas.SetDepth and Canvas.DrawLineDepth. Giving it to them too is what makes a marker,
// a billboard or a particle hidden by the solids in front of it, and hide those behind it,
// in whatever order they are drawn. It is not the straight-line distance: a point to one
// side of the view direction is farther away than its depth.
//
// ok is false when Project's would be: p is nearer than Near or behind the camera, has no
// finite place on the screen, or the camera sees nothing at all (see Camera). It is also
// false for a nil canvas. A point off the canvas is still ok, with coordinates outside
// it; SetDepth ignores a write there.
func (c Camera) ProjectPixels(p geometry.Vector3, cv *engine.Canvas) (x, y, depth float64, ok bool) {
	if cv == nil {
		return 0, 0, 0, false
	}
	x, y, depth, ok = c.projectDepth(p, cv.Width(), cv.Height()/2)
	return x, 2 * y, depth, ok
}

// projectDepth is Project that also returns the depth of p, its view-space z: the distance
// along Look. Project and ProjectPixels both go through it, so they cannot disagree about
// what is ok.
func (c Camera) projectDepth(p geometry.Vector3, w, h int) (x, y, z float64, ok bool) {
	c, usable := c.withDefaults()
	if !usable {
		return 0, 0, 0, false
	}
	v := c.toView(p)

	if v.Z < c.Near {
		return 0, 0, 0, false
	}

	px, py := c.projectView(v, w, h)
	if !finite(px, py, v.Z) { // NaN or infinity: no place on the screen
		return 0, 0, 0, false
	}

	return px, py, v.Z, true
}

// Forward returns the unit vector the camera faces, flat on the xz plane: Pitch
// is ignored, so walking along it never climbs or dives. Use Look for the way
// the camera actually points. With Yaw 0 it is +z, and a positive Yaw turns it
// toward +x.
func (c Camera) Forward() geometry.Vector3 {
	return geometry.Vector3{X: 0, Y: 0, Z: 1}.RotateY(c.Yaw)
}

// Right returns the unit vector to the camera's right, flat on the xz plane.
// Tilting the camera does not move it.
func (c Camera) Right() geometry.Vector3 {
	return geometry.Vector3{X: 1, Y: 0, Z: 0}.RotateY(c.Yaw)
}

// Look returns the unit vector the camera points along, including Pitch. A point
// at Position plus Look times any distance lands in the middle of the screen.
func (c Camera) Look() geometry.Vector3 {
	return geometry.Vector3{X: 0, Y: 0, Z: 1}.RotateX(-c.Pitch).RotateY(c.Yaw)
}

// toView moves a world point into view space: camera at the origin, looking down +z.
// It undoes the camera's turn first and then its tilt, the reverse of how the
// camera was posed (turn about the vertical axis, then tilt about its own). Drawing
// many points does the trigonometry once, with viewTransform.
func (c Camera) toView(p geometry.Vector3) geometry.Vector3 {
	return c.viewTransform().apply(p)
}

// projectView maps a view-space point to fractional cell coordinates.
// The caller must ensure v.Z >= Near; the result is meaningless otherwise. Drawing
// many points does the trigonometry once, with projector.
func (c Camera) projectView(v geometry.Vector3, w, h int) (x, y float64) {
	return c.projector(w, h).project(v)
}

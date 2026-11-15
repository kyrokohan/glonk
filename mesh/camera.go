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
// a Near of 0.1 and a CellAspect of 2), so Camera{Position: p} is a working camera, and
// a Far left at 0 means no limit. A value that makes no sense, namely a FOV outside 0 to
// pi (exclusive), a Near that is negative, NaN or infinite, a Far that is negative, NaN or
// not beyond Near, or a CellAspect that is negative, NaN or infinite, makes the camera
// see nothing: Draw, DrawShaded and DrawShadedPixels draw nothing and Project reports
// ok=false. So does a Position, Yaw or Pitch that is not a finite number.
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

	// Far is the distance from the camera, along its view direction, of the plane farther
	// than which everything is cut away, as Near cuts away what is nearer: a point exactly
	// Far away is still drawn. A face or an edge that crosses it is cut where it crosses,
	// and a mesh wholly beyond it is skipped without being looked at, which saves the time
	// to draw the far end of a scene, or a floor that runs on to the horizon. It must be
	// beyond Near; 0 means no limit, which +Inf does too. Project and ProjectPixels report
	// a point beyond it as not visible, so a program that draws its own things can leave
	// them out too. Ray is not limited by it: a ray has no length.
	Far float64

	// CellAspect is how many times taller a terminal cell is than it is wide, which
	// is what makes a cube look like a cube and not a slab. Most fonts are close to 2,
	// which is what 0 means. A larger value draws things wider, in more columns, and a
	// smaller one narrower. It must be above 0.
	CellAspect float64
}

// What a zero FOV, Near and CellAspect stand for. A zero Far stands for no limit.
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
	if c.Far == 0 {
		c.Far = math.Inf(1)
	}
	// Written so that NaN fails every test. An infinite Near is not below any Far, so it
	// sees nothing.
	ok = c.FOV > 0 && c.FOV < math.Pi && c.Near > 0 && c.Far > c.Near && c.CellAspect > 0 && !math.IsInf(c.CellAspect, 0)
	return c, ok
}

// Project returns where p appears on a grid of w x h cells, as fractional cell
// coordinates; the caller is expected to floor them. ok is false when p is nearer
// than Near or behind the camera, farther than Far, has no finite place on the screen,
// or the camera sees nothing at all (see Camera).
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

// Ray returns the ray into the world through the place (x, y) on a grid of w x h cells, the
// reverse of Project: the ray that Project(p, w, h) puts at (x, y) passes through p. Cell
// coordinates are fractional, as Project returns them, so the middle of cell (i, j) is
// (i+0.5, j+0.5). A place off the grid has a ray too.
//
// The ray starts at the camera, at origin, which is Position. direction is not a unit
// vector. It is one deep along Look: its dot with Look is 1, so the point origin plus t
// times direction is t deep, the depth that Canvas.SetDepth and ProjectPixels use. A program
// that draws by rays can store t as it is. Call Normalize on direction for a unit vector,
// and then t is the straight-line distance.
//
// A ray is not limited by Near or Far: it has no length, and the point origin plus t times
// direction is on the screen for any t that Project would accept.
//
// ok is false when Project's would be for a camera that sees nothing (see Camera), when the
// grid has no height (h is not positive), when x or y is NaN or infinite, and when the ray
// has no finite place, which a Position, Yaw or Pitch that is not a number gives.
func (c Camera) Ray(x, y float64, w, h int) (origin, direction geometry.Vector3, ok bool) {
	c, usable := c.withDefaults()
	if !usable || !(h > 0) {
		return geometry.Vector3{}, geometry.Vector3{}, false
	}
	// Turned the way Look is: up by Pitch, then round by Yaw.
	direction = c.projector(w, h).ray(x, y).RotateX(-c.Pitch).RotateY(c.Yaw)
	if !finite(direction.X, direction.Y, direction.Z, c.Position.X, c.Position.Y, c.Position.Z) {
		return geometry.Vector3{}, geometry.Vector3{}, false
	}
	return c.Position, direction, true
}

// RayPixels is Ray for the pixels of cv, the reverse of ProjectPixels: the ray that
// ProjectPixels puts at (x, y) passes through the point, and with the depth it returns as t
// it reaches it. x and y are fractional pixel coordinates, so the ray through the middle of
// pixel (i, j) is RayPixels(i+0.5, j+0.5, cv). ok is also false for a nil canvas.
func (c Camera) RayPixels(x, y float64, cv *engine.Canvas) (origin, direction geometry.Vector3, ok bool) {
	if cv == nil {
		return geometry.Vector3{}, geometry.Vector3{}, false
	}
	return c.Ray(x, y/2, cv.Width(), cv.Height()/2)
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

	if v.Z < c.Near || v.Z > c.Far {
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

// Up returns the unit vector to the camera's up, perpendicular to Look and Right. Unlike
// Forward and Right, which stay flat on the xz plane, it tilts with Pitch, as Look does.
func (c Camera) Up() geometry.Vector3 {
	return c.Look().Cross(c.Right())
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

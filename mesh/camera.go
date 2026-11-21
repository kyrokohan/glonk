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
// ok=false. So does a Position, Yaw or Pitch that is not a finite number. LookAt points it at
// a place.
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

	// Fog fades shaded meshes toward a color as they get deeper: DrawShaded and
	// DrawShadedPixels apply it to every cell and pixel they draw. nil, the default, is no
	// fog. It is a pointer, so every copy of the Camera shares it. Wireframes are not fogged.
	// Fog.Apply is the same rule for a program that draws things of its own.
	Fog *Fog

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
// pixel (i, j) is RayPixels(i+0.5, j+0.5, cv). ok is also false for a nil canvas. For the rays
// of every pixel, RaysPixels gives them all at once.
func (c Camera) RayPixels(x, y float64, cv *engine.Canvas) (origin, direction geometry.Vector3, ok bool) {
	if cv == nil {
		return geometry.Vector3{}, geometry.Vector3{}, false
	}
	return c.Ray(x, y/2, cv.Width(), cv.Height()/2)
}

// Rays is every ray of a grid of cells, or of the pixels of a canvas, at once: the ray through
// the middle of cell or pixel (i, j) starts at Origin and goes along First + i*Across + j*Down,
// the direction Ray or RayPixels gives for it, to within rounding. Camera.Rays and
// Camera.RaysPixels return it. A program that draws a whole canvas by rays works them out with
// additions, and not with RayPixels for each pixel, which works out the camera's angles every time.
type Rays struct {
	// Origin is where every ray starts: the camera's Position.
	Origin geometry.Vector3

	// First is the direction of the ray through the middle of cell or pixel (0, 0). Like every
	// direction here, it is one deep along Look and not of unit length.
	First geometry.Vector3

	// Across is what a direction gains from one column to the next. It has nothing along Look,
	// so every ray stays one deep.
	Across geometry.Vector3

	// Down is what a direction gains from one row to the next, with nothing along Look either.
	Down geometry.Vector3
}

// Rays returns the rays through the middle of every cell of a grid of w x h cells, the ones Ray
// gives. ok is false where Ray's would be: for a camera that sees nothing (see Camera), a grid
// with no height, and a Position, Yaw or Pitch that is not a number.
func (c Camera) Rays(w, h int) (rays Rays, ok bool) {
	return c.rays(0.5, 0.5, 1, w, h)
}

// RaysPixels is Rays for the pixels of cv: the ray through the middle of pixel (i, j) is the one
// RayPixels(i+0.5, j+0.5, cv) gives. ok is also false for a nil canvas.
func (c Camera) RaysPixels(cv *engine.Canvas) (rays Rays, ok bool) {
	if cv == nil {
		return Rays{}, false
	}
	return c.rays(0.5, 0.25, 0.5, cv.Width(), cv.Height()/2) // a pixel is half a cell tall
}

// rays is Rays on a grid of w x h cells, with the first ray through the place (x, y) and rows dy
// cells apart.
func (c Camera) rays(x, y, dy float64, w, h int) (Rays, bool) {
	c, usable := c.withDefaults()
	if !usable || !(h > 0) {
		return Rays{}, false
	}
	p := c.projector(w, h)
	// The ray of a place is linear in it, so its steps are worked out exactly and turned as Ray
	// turns a direction.
	turn := func(v geometry.Vector3) geometry.Vector3 { return v.RotateX(-c.Pitch).RotateY(c.Yaw) }
	r := Rays{
		Origin: c.Position,
		First:  turn(p.ray(x, y)),
		Across: turn(geometry.Vector3{X: 1 / (p.h / 2 * p.aspect) / p.focal}),
		Down:   turn(geometry.Vector3{Y: -dy / (p.h / 2) / p.focal}),
	}
	if !finite(r.Origin.X, r.Origin.Y, r.Origin.Z, r.First.X, r.First.Y, r.First.Z,
		r.Across.X, r.Across.Y, r.Across.Z, r.Down.X, r.Down.Y, r.Down.Z) {
		return Rays{}, false
	}
	return r, true
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

// LookAt turns the camera to look at target from where it is: it sets Yaw and Pitch so that
// Look points from Position to target, and Project puts target in the middle of the frame. The
// pitch is at most a quarter turn up or down, as Pitch asks. A target straight above or below
// has no direction across to turn to, so the camera keeps its Yaw. A target at the camera's own
// Position, or a target or Position that is not a finite number, leaves the camera as it was.
// It is the one method of Camera that changes it, and so the one that takes a pointer.
func (c *Camera) LookAt(target geometry.Vector3) {
	to := target.Sub(c.Position)
	if !finite(to.X, to.Y, to.Z) || to == (geometry.Vector3{}) {
		return
	}
	// The inverse of Look, which is (cos Pitch sin Yaw, sin Pitch, cos Pitch cos Yaw).
	across := math.Hypot(to.X, to.Z)
	if across > 0 {
		c.Yaw = math.Atan2(to.X, to.Z)
	}
	c.Pitch = math.Atan2(to.Y, across)
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

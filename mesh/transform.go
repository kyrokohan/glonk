package mesh

import "glonk.dev/geometry"

// Transform says where a model sits in the world and how large it is. The zero
// Transform leaves the model as it was built: at the origin, unturned, full size.
//
// All angles are in radians, and they turn the way a Camera's do: Yaw swings the
// front toward +x and Pitch tips it up. A mesh whose Transform has a Position or
// angle that is not a finite number, or a Scale that is negative or not finite, is
// not drawn, and Mesh.Validate reports it.
type Transform struct {
	// Position is where the model's origin sits in the world.
	Position geometry.Vector3

	// Yaw turns the model about the y axis. Positive turns +z toward +x. It is
	// applied after Pitch and Roll.
	Yaw float64

	// Pitch tips the model's front up, as it does for a Camera: positive turns +z
	// toward +y. It is applied after Roll and before Yaw.
	Pitch float64

	// Roll leans the model over about its own z axis, the way it faces: positive
	// turns +y toward +x, so the top leans to the right. It is applied first.
	Roll float64

	// Scale makes the model larger or smaller about its origin, evenly in every
	// direction. 0 means 1, so a Transform that sets only a Position keeps the
	// model's size. It is applied before the turns.
	Scale float64
}

// valid reports whether the transform can be applied: every number finite, and a
// Scale that does not turn the model inside out.
func (t Transform) valid() bool {
	return finite(t.Position.X, t.Position.Y, t.Position.Z, t.Yaw, t.Pitch, t.Roll, t.Scale) && t.Scale >= 0
}

// placement is a Transform worked out once, so that placing a point costs a few
// multiplications and no trigonometry.
type placement struct {
	x, y, z geometry.Vector3 // where the model's axes point, scaled
	origin  geometry.Vector3
}

func (t Transform) placement() placement {
	scale := t.Scale
	if scale == 0 {
		scale = 1
	}
	// RotateX turns +y toward +z, and RotateZ turns +x toward +y. A Pitch that tips
	// +z up and a Roll that leans +y to the right turn the other way.
	turn := func(axis geometry.Vector3) geometry.Vector3 {
		return axis.Scale(scale).RotateZ(-t.Roll).RotateX(-t.Pitch).RotateY(t.Yaw)
	}
	return placement{
		x:      turn(geometry.Vector3{X: 1}),
		y:      turn(geometry.Vector3{Y: 1}),
		z:      turn(geometry.Vector3{Z: 1}),
		origin: t.Position,
	}
}

func (p placement) apply(v geometry.Vector3) geometry.Vector3 {
	return p.origin.Add(p.x.Scale(v.X)).Add(p.y.Scale(v.Y)).Add(p.z.Scale(v.Z))
}

// Apply returns where the model-space point p ends up in the world: scaled, then
// turned by Roll, Pitch and Yaw in that order, then moved to Position. It does not
// check the Transform; a value that is not valid gives a result to match.
func (t Transform) Apply(p geometry.Vector3) geometry.Vector3 {
	return t.placement().apply(p)
}

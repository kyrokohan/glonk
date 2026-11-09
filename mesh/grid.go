package mesh

import (
	"glonk.dev/geometry"
)

// Grid returns a flat grid of lines on the y=0 plane, centered on the origin,
// size wide and deep with divisions cells per side. Each line runs the full
// width, so there are divisions+1 of them in each direction. divisions below 1
// is treated as 1. Lift it to floor height with Mesh.Transform.
func Grid(size float64, divisions int) Mesh {
	m := Mesh{}

	if divisions < 1 {
		divisions = 1
	}

	half := size / 2
	step := size / float64(divisions)

	for i := range divisions + 1 {
		c := -half + float64(i)*step
		base := len(m.Vertices)

		m.Vertices = append(m.Vertices, geometry.Vector3{X: c, Y: 0, Z: -half}, geometry.Vector3{X: c, Y: 0, Z: half})
		m.Vertices = append(m.Vertices, geometry.Vector3{X: -half, Y: 0, Z: c}, geometry.Vector3{X: half, Y: 0, Z: c})
		m.Edges = append(m.Edges, [2]int{base, base + 1}, [2]int{base + 2, base + 3})
	}

	return m
}

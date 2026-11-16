package mesh

import "fmt"

// Validate checks a mesh that was built by hand or by other code, and returns an
// error describing the first problem: a Transform that cannot be applied (a number
// that is not finite, or a negative Scale), a vertex that is not a finite number,
// a vertex outside the bounds that UpdateBounds kept, normals that are not one per vertex or not finite numbers, face colors that are not
// one per face, texture coordinates that are not one triple per face or not finite numbers,
// or an edge or face that names a vertex the mesh does not have.
//
// Drawing skips edges and faces with a bad index instead of failing, so Validate is
// how to find out that a mesh is broken. Meshes from LoadOBJ, Cube and Grid are
// valid.
func (m Mesh) Validate() error {
	n := len(m.Vertices)

	if !m.Transform.valid() {
		return fmt.Errorf("mesh: the transform has a number that is not finite or a negative scale: %+v", m.Transform)
	}
	for i, v := range m.Vertices {
		if !finite(v.X, v.Y, v.Z) {
			return fmt.Errorf("mesh: vertex %d is not a finite number: %v", i, v)
		}
	}
	// A vertex that has moved out of the sphere UpdateBounds kept makes drawing skip the mesh
	// while it is on screen.
	if b := m.currentBounds(); b != nil {
		for i, v := range m.Vertices {
			if v.Sub(b.center).Length() > b.radius {
				return fmt.Errorf("mesh: vertex %d is outside the bounds that UpdateBounds kept: call UpdateBounds again after changing Vertices", i)
			}
		}
	}
	if len(m.Normals) != 0 && len(m.Normals) != n {
		return fmt.Errorf("mesh: %d normals for %d vertices: wanted none, or one for each vertex", len(m.Normals), n)
	}
	for i, v := range m.Normals {
		if !finite(v.X, v.Y, v.Z) {
			return fmt.Errorf("mesh: normal %d is not a finite number: %v", i, v)
		}
	}
	if len(m.FaceColors) != 0 && len(m.FaceColors) != len(m.Faces) {
		return fmt.Errorf("mesh: %d face colors for %d faces: wanted none, or one for each face", len(m.FaceColors), len(m.Faces))
	}
	if len(m.FaceTexCoords) != 0 && len(m.FaceTexCoords) != len(m.Faces) {
		return fmt.Errorf("mesh: %d texture coordinate triples for %d faces: wanted none, or one for each face", len(m.FaceTexCoords), len(m.Faces))
	}
	for i, t := range m.FaceTexCoords {
		if !finite(t[0].X, t[0].Y, t[1].X, t[1].Y, t[2].X, t[2].Y) {
			return fmt.Errorf("mesh: the texture coordinates of face %d are not finite numbers: %v", i, t)
		}
	}
	if len(m.VertexColors) != 0 && len(m.VertexColors) != n {
		return fmt.Errorf("mesh: %d vertex colors for %d vertices: wanted none, or one for each vertex", len(m.VertexColors), n)
	}
	for i, e := range m.Edges {
		for _, v := range e {
			if !validIndex(v, n) {
				return missingVertex("edge", i, v, n)
			}
		}
	}
	for i, f := range m.Faces {
		for _, v := range f {
			if !validIndex(v, n) {
				return missingVertex("face", i, v, n)
			}
		}
	}
	return nil
}

// missingVertex describes edge or face number i naming vertex v in a mesh of n vertices.
func missingVertex(part string, i, v, n int) error {
	if n == 0 {
		return fmt.Errorf("mesh: %s %d uses vertex %d, but the mesh has no vertices", part, i, v)
	}
	return fmt.Errorf("mesh: %s %d uses vertex %d, but only vertices 0 to %d exist", part, i, v, n-1)
}

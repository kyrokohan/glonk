package mesh

import (
	"math"
	"reflect"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// shape is one generated mesh with what is known about it.
type shape struct {
	name                   string
	m                      Mesh
	vertices, faces, edges int
	closed                 bool // every edge is shared by two faces, once each way round
	size                   float64
}

func testShapes() []shape {
	var out []shape
	for _, d := range []struct{ segments, rings int }{{3, 2}, {4, 3}, {7, 5}, {8, 4}, {24, 12}, {3, 40}, {60, 2}} {
		s, r := d.segments, d.rings
		out = append(out, shape{"sphere " + dims(s, r), Sphere(2, s, r), s*(r-1) + 2, 2 * s * (r - 1), s * (2*r - 1), true, 2})
	}
	for _, s := range []int{3, 4, 7, 24, 100} {
		out = append(out,
			shape{"cylinder " + dims(s, 0), Cylinder(1.5, 3, s), 4*s + 2, 4 * s, 3 * s, true, 1.5},
			shape{"cone " + dims(s, 0), Cone(1.5, 3, s), 3*s + 1, 2 * s, 2 * s, true, 1.5})
	}
	out = append(out, shape{"plane", Plane(4, 6), 4, 2, 4, false, 3})
	return out
}

func dims(segments, rings int) string {
	if rings == 0 {
		return string(rune('0'+segments/100%10)) + string(rune('0'+segments/10%10)) + string(rune('0'+segments%10))
	}
	return dims(segments, 0) + "x" + dims(rings, 0)
}

func TestShapesHaveTheCountsTheyPromise(t *testing.T) {
	for _, sh := range testShapes() {
		t.Run(sh.name, func(t *testing.T) {
			m := sh.m
			if len(m.Vertices) != sh.vertices || len(m.Faces) != sh.faces || len(m.Edges) != sh.edges {
				t.Errorf("wanted %d vertices, %d faces and %d edges, got %d, %d and %d",
					sh.vertices, sh.faces, sh.edges, len(m.Vertices), len(m.Faces), len(m.Edges))
			}
			if len(m.Normals) != len(m.Vertices) {
				t.Errorf("wanted a normal for every vertex, got %d for %d", len(m.Normals), len(m.Vertices))
			}
			if err := m.Validate(); err != nil {
				t.Errorf("wanted a valid mesh, got %v", err)
			}
		})
	}
}

func faceNormal(m Mesh, f [3]int) geometry.Vector3 {
	a, b, c := m.Vertices[f[0]], m.Vertices[f[1]], m.Vertices[f[2]]
	return b.Sub(a).Cross(c.Sub(a)).Normalize()
}

// Every face turns its front out of the shape. All but the plane have the origin inside
// them, so a face is wound the right way round if its normal points away from it.
func TestShapesAreWoundOutward(t *testing.T) {
	for _, sh := range testShapes() {
		t.Run(sh.name, func(t *testing.T) {
			for i, f := range sh.m.Faces {
				n := faceNormal(sh.m, f)
				if n == (geometry.Vector3{}) {
					t.Fatalf("face %d %v has no area", i, f)
				}
				a, b, c := sh.m.Vertices[f[0]], sh.m.Vertices[f[1]], sh.m.Vertices[f[2]]
				if sh.name == "plane" {
					if !vecNear(n, v3(0, 1, 0), 1e-12) {
						t.Fatalf("face %d: wanted the plane to face +y, got %v", i, n)
					}
					continue
				}
				if centre := a.Add(b).Add(c).Scale(1.0 / 3); n.Dot(centre) <= 0 {
					t.Fatalf("face %d %v: wanted it to face away from the middle, normal %v at %v", i, f, n, centre)
				}
			}
		})
	}
}

// Normals are unit length and agree with the faces they light: none points into the shape.
func TestShapeNormalsAreUnitAndFaceOutward(t *testing.T) {
	for _, sh := range testShapes() {
		t.Run(sh.name, func(t *testing.T) {
			for i, n := range sh.m.Normals {
				if math.Abs(n.Length()-1) > 1e-12 {
					t.Fatalf("normal %d: wanted a unit vector, got %v", i, n)
				}
			}
			for i, f := range sh.m.Faces {
				fn := faceNormal(sh.m, f)
				for _, v := range f {
					if sh.m.Normals[v].Dot(fn) <= 0 {
						t.Fatalf("face %d: wanted the normal %v of vertex %d to face the way the face (%v) does", i, sh.m.Normals[v], v, fn)
					}
				}
			}
		})
	}
}

func TestSphereNormalsPointOutOfTheMiddle(t *testing.T) {
	m := Sphere(3, 9, 6)
	for i, v := range m.Vertices {
		if !vecNear(m.Normals[i], v.Scale(1.0/3), 1e-12) {
			t.Errorf("vertex %d at %v: wanted the normal %v, got %v", i, v, v.Scale(1.0/3), m.Normals[i])
		}
	}
}

// The side of a cylinder faces straight out and its caps straight up and down, and no
// face mixes the two: the rims are hard.
func TestCylinderHasHardRims(t *testing.T) {
	const segments = 12
	m := Cylinder(2, 5, segments)

	for i := range 2 * segments { // the side
		v, n := m.Vertices[i], m.Normals[i]
		if n.Y != 0 || !vecNear(v3(v.X/2, 0, v.Z/2), n, 1e-12) {
			t.Errorf("side vertex %d at %v: wanted a normal straight out from the axis, got %v", i, v, n)
		}
	}
	for i := 2 * segments; i < len(m.Vertices); i++ { // the caps
		n := m.Normals[i]
		if n.X != 0 || n.Z != 0 || math.Abs(n.Y) != 1 {
			t.Errorf("cap vertex %d: wanted a normal straight up or down, got %v", i, n)
		}
	}
	for i, f := range m.Faces {
		side := 0
		for _, v := range f {
			if v < 2*segments {
				side++
			}
		}
		if side != 0 && side != 3 {
			t.Errorf("face %d %v mixes side and cap vertices", i, f)
		}
	}
}

// The slope of a cone is lit as a slope: every normal on it is square to the surface of
// the true cone at its own angle, which for a tip is the angle halfway between its
// side's two corners, and leans up.
func TestConeNormalsAreSquareToTheSlope(t *testing.T) {
	const segments = 9
	const radius, height = 2.0, 6.0
	m := Cone(radius, height, segments)
	tipVertex := v3(0, 3, 0)

	for s := range segments {
		for i, angle := range map[int]float64{s: float64(s), segments + s: float64(s) + 0.5} {
			phi := 2 * math.Pi * angle / segments
			cos, sin := math.Cos(phi), math.Sin(phi)
			n := m.Normals[i]

			upTheSlope := v3(-radius*cos, height, -radius*sin) // from the rim to the tip, at this angle
			around := v3(-sin, 0, cos)                         // along the rim
			if math.Abs(n.Dot(upTheSlope)) > 1e-12 || math.Abs(n.Dot(around)) > 1e-12 || n.Y <= 0 || n.X*cos+n.Z*sin <= 0 {
				t.Errorf("vertex %d at angle %v: wanted a normal square to the slope, leaning up and out, got %v", i, phi, n)
			}
		}
		if m.Vertices[segments+s] != tipVertex {
			t.Errorf("tip %d: wanted %v, got %v", s, tipVertex, m.Vertices[segments+s])
		}
	}
	for i := 2 * segments; i < len(m.Vertices); i++ {
		if m.Normals[i] != (geometry.Vector3{Y: -1}) {
			t.Errorf("base vertex %d: wanted a normal straight down, got %v", i, m.Normals[i])
		}
	}
	for i, f := range m.Faces {
		down := 0
		for _, v := range f {
			if m.Normals[v].Y == -1 {
				down++
			}
		}
		if down != 0 && down != 3 {
			t.Errorf("face %d %v mixes the base and the slope", i, f)
		}
	}
}

type weldedEdge [2][3]int64

func weld(v geometry.Vector3) [3]int64 {
	q := func(f float64) int64 { return int64(math.Round(f * 1e9)) }
	return [3]int64{q(v.X), q(v.Y), q(v.Z)}
}

// directedEdges counts how often each edge is walked by a face, with vertices that
// coincide counted as one: the hard rims of a cylinder have two vertices at each spot.
func directedEdges(m Mesh) map[weldedEdge]int {
	out := map[weldedEdge]int{}
	for _, f := range m.Faces {
		for i := range 3 {
			out[weldedEdge{weld(m.Vertices[f[i]]), weld(m.Vertices[f[(i+1)%3]])}]++
		}
	}
	return out
}

// A closed shape has no hole and no overlap: every edge is walked once each way round,
// by the two faces on either side of it, which also means they are wound alike.
func TestShapesAreClosedAndConsistentlyWound(t *testing.T) {
	for _, sh := range testShapes() {
		t.Run(sh.name, func(t *testing.T) {
			edges := directedEdges(sh.m)
			boundary := 0
			for e, n := range edges {
				back := edges[weldedEdge{e[1], e[0]}]
				if n != 1 || back > 1 {
					t.Fatalf("wanted every edge walked at most once each way, %v is walked %d times and %d back", e, n, back)
				}
				if back == 0 {
					boundary++
				}
			}
			switch {
			case sh.closed && boundary != 0:
				t.Errorf("wanted a closed shape, %d edges have a face on one side only", boundary)
			case !sh.closed && boundary != 4:
				t.Errorf("wanted the 4 sides of the plane to be its boundary, got %d edges", boundary)
			}
		})
	}
}

func TestShapeBoundsAreWhatTheyShouldBe(t *testing.T) {
	tests := []struct {
		name   string
		m      Mesh
		lo, hi geometry.Vector3
		reachX float64 // how far along +x the first corner of the first ring reaches
	}{
		{"sphere", Sphere(2, 24, 12), v3(-2, -2, -2), v3(2, 2, 2), 2},
		{"sphere with no equator", Sphere(2, 7, 5), v3(-2, -2, -2), v3(2, 2, 2), 2 * math.Sin(2*math.Pi/5)},
		{"cylinder", Cylinder(1.5, 4, 24), v3(-1.5, -2, -1.5), v3(1.5, 2, 1.5), 1.5},
		{"cone", Cone(1.5, 4, 24), v3(-1.5, -2, -1.5), v3(1.5, 2, 1.5), 1.5},
		{"plane", Plane(6, 4), v3(-3, 0, -2), v3(3, 0, 2), 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lo, hi := tt.m.Bounds()

			// Y is exact: the poles, the rims and the plane sit right on it. No vertex goes
			// past a limit on x or z, and the first corner reaches as far as it should.
			if lo.Y != tt.lo.Y || hi.Y != tt.hi.Y {
				t.Errorf("wanted the box to reach y %v to %v, got %v to %v", tt.lo.Y, tt.hi.Y, lo.Y, hi.Y)
			}
			if math.Abs(hi.X-tt.reachX) > 1e-12 {
				t.Errorf("wanted the box to reach x %v, got %v", tt.reachX, hi.X)
			}
			if lo.X < tt.lo.X-1e-12 || lo.Z < tt.lo.Z-1e-12 || hi.X > tt.hi.X+1e-12 || hi.Z > tt.hi.Z+1e-12 {
				t.Errorf("wanted the box inside %v to %v, got %v to %v", tt.lo, tt.hi, lo, hi)
			}
		})
	}
}

// Wireframes follow the form: rings, meridians and rims, never the diagonals the faces
// are cut along, and every line lies along an edge of the surface.
func TestShapeEdgesFollowTheForm(t *testing.T) {
	for _, sh := range testShapes() {
		t.Run(sh.name, func(t *testing.T) {
			m := sh.m
			faceEdges := directedEdges(m)
			seen := map[weldedEdge]bool{}
			for i, e := range m.Edges {
				a, b := m.Vertices[e[0]], m.Vertices[e[1]]
				k := weldedEdge{weld(a), weld(b)}
				if k[0] == k[1] {
					t.Fatalf("edge %d %v has no length", i, e)
				}
				if seen[k] || seen[weldedEdge{k[1], k[0]}] {
					t.Fatalf("edge %d %v is a repeat", i, e)
				}
				seen[k] = true
				if faceEdges[k] == 0 && faceEdges[weldedEdge{k[1], k[0]}] == 0 {
					t.Fatalf("edge %d %v does not lie along an edge of the surface", i, e)
				}

				ring := math.Abs(a.Y-b.Y) < 1e-9             // around the shape
				meridian := math.Abs(a.X*b.Z-a.Z*b.X) < 1e-9 // down the shape: both on one half plane through the axis
				switch {
				case sh.name == "plane":
					if a.X != b.X && a.Z != b.Z {
						t.Fatalf("edge %d %v is a diagonal", i, e)
					}
				case sh.name[:4] == "cone":
					// A rim edge, or a slant from the rim to the tip.
					if !ring && !(b.X == 0 && b.Z == 0) && !(a.X == 0 && a.Z == 0) {
						t.Fatalf("edge %d %v is neither a rim nor a slant", i, e)
					}
				default:
					if !ring && !meridian {
						t.Fatalf("edge %d %v goes along neither a ring nor a meridian: %v to %v", i, e, a, b)
					}
				}
			}
		})
	}
}

func TestShapesBringDetailIntoRange(t *testing.T) {
	tests := []struct {
		name      string
		got, want Mesh
	}{
		{"sphere: segments too few", Sphere(1, 0, 5), Sphere(1, 3, 5)},
		{"sphere: segments negative", Sphere(1, -9, 5), Sphere(1, 3, 5)},
		{"sphere: rings too few", Sphere(1, 6, 1), Sphere(1, 6, 2)},
		{"sphere: rings negative", Sphere(1, 6, -1), Sphere(1, 6, 2)},
		{"sphere: both too few", Sphere(1, 1, 0), Sphere(1, 3, 2)},
		{"cylinder: too few", Cylinder(1, 2, 2), Cylinder(1, 2, 3)},
		{"cylinder: negative", Cylinder(1, 2, -4), Cylinder(1, 2, 3)},
		{"cone: too few", Cone(1, 2, 0), Cone(1, 2, 3)},
		{"cone: negative", Cone(1, 2, math.MinInt), Cone(1, 2, 3)},
		{"sphere: too many", Sphere(1, 513, 513), Sphere(1, 512, 512)},
		{"sphere: absurd", Sphere(1, math.MaxInt, math.MaxInt), Sphere(1, 512, 512)},
		{"cylinder: too many", Cylinder(1, 2, 100000), Cylinder(1, 2, 512)},
		{"cone: absurd", Cone(1, 2, math.MaxInt), Cone(1, 2, 512)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Errorf("wanted the clamped shape (%d vertices), got %d vertices", len(tt.want.Vertices), len(tt.got.Vertices))
			}
		})
	}
	if n := len(Sphere(1, math.MaxInt, math.MaxInt).Vertices); n != 512*511+2 {
		t.Errorf("wanted the biggest sphere to have %d vertices, got %d", 512*511+2, n)
	}
}

// A size that is negative or not a number counts as nothing: the shape keeps its form
// and is valid, but is a point (or a line), and draws nothing.
func TestShapeSizesThatMakeNoSense(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	for _, bad := range []float64{nan, inf, -inf, -1, -math.MaxFloat64, 0} {
		for name, pair := range map[string][2]Mesh{
			"sphere":       {Sphere(bad, 8, 5), Sphere(0, 8, 5)},
			"cylinder, r":  {Cylinder(bad, 2, 8), Cylinder(0, 2, 8)},
			"cylinder, h":  {Cylinder(2, bad, 8), Cylinder(2, 0, 8)},
			"cone, r":      {Cone(bad, 2, 8), Cone(0, 2, 8)},
			"cone, h":      {Cone(2, bad, 8), Cone(2, 0, 8)},
			"plane, width": {Plane(bad, 3), Plane(0, 3)},
			"plane, depth": {Plane(3, bad), Plane(3, 0)},
		} {
			if !reflect.DeepEqual(pair[0], pair[1]) {
				t.Errorf("%s with size %v: wanted the same shape as with size 0", name, bad)
			}
			if err := pair[0].Validate(); err != nil {
				t.Errorf("%s with size %v: wanted a valid mesh, got %v", name, bad, err)
			}
		}
	}
}

func TestShapesOfTheLargestSizeStayFinite(t *testing.T) {
	big := math.MaxFloat64
	for name, m := range map[string]Mesh{
		"sphere": Sphere(big, 8, 5), "cylinder": Cylinder(big, big, 8), "cone": Cone(big, big, 8), "plane": Plane(big, big),
	} {
		if err := m.Validate(); err != nil {
			t.Errorf("%s: wanted a valid mesh, got %v", name, err)
		}
		for i, n := range m.Normals {
			if !finite(n.X, n.Y, n.Z) || n.Length() > 1+1e-12 {
				t.Errorf("%s: normal %d is %v", name, i, n)
			}
		}
	}
}

func TestShapesOfNoSizeKeepUnitNormals(t *testing.T) {
	// A sphere or cylinder knows which way it faces however small. A cone of no radius
	// and no height has no slope to speak of, and says so with a zero normal.
	for _, n := range Sphere(0, 6, 4).Normals {
		if math.Abs(n.Length()-1) > 1e-12 {
			t.Fatalf("wanted unit normals on a sphere of radius 0, got %v", n)
		}
	}
	for _, n := range Cone(0, 0, 6).Normals[:12] {
		if n != (geometry.Vector3{}) {
			t.Fatalf("wanted zero normals on the slope of a cone of no size, got %v", n)
		}
	}
}

// The counts are known up front, so a shape allocates its lists once each and nothing more.
func TestShapesAllocateOnlyTheirLists(t *testing.T) {
	for name, build := range map[string]func() Mesh{
		"sphere":   func() Mesh { return Sphere(1, 64, 32) },
		"cylinder": func() Mesh { return Cylinder(1, 2, 64) },
		"cone":     func() Mesh { return Cone(1, 2, 64) },
		"plane":    func() Mesh { return Plane(2, 2) },
	} {
		if n := testing.AllocsPerRun(20, func() { build() }); n > 6 {
			t.Errorf("%s: wanted the four lists and little else, got %v allocations", name, n)
		}
	}
}

func TestShapesShareNothingBetweenCalls(t *testing.T) {
	for name, make := range map[string]func() Mesh{
		"sphere":   func() Mesh { return Sphere(1, 8, 4) },
		"cylinder": func() Mesh { return Cylinder(1, 2, 8) },
		"cone":     func() Mesh { return Cone(1, 2, 8) },
		"plane":    func() Mesh { return Plane(2, 2) },
	} {
		a := make()
		a.Vertices[0] = v3(9, 9, 9)
		a.Normals[0] = v3(9, 9, 9)
		a.Faces[0] = [3]int{0, 0, 0}
		a.Edges[0] = [2]int{0, 0}

		if !reflect.DeepEqual(make(), make()) {
			t.Errorf("%s: wanted every call to give the same shape", name)
		}
		if reflect.DeepEqual(a, make()) {
			t.Errorf("%s: wanted a fresh shape from each call", name)
		}
	}
}

// Every shape draws, in every mode, and a smooth one shows more shades than its
// facets would.
func TestShapesDrawInEveryMode(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.1)
	cam := Camera{Position: geometry.Vector3{Y: 1.5, Z: -1}, Pitch: -0.25, FOV: 1.3, Near: 0.1}
	place := Transform{Position: geometry.Vector3{Z: 5}, Yaw: 0.4}

	for name, m := range map[string]Mesh{
		"sphere": Sphere(1.5, 16, 8), "cylinder": Cylinder(1, 2.5, 16), "cone": Cone(1.2, 2.5, 16), "plane": Plane(6, 6),
	} {
		t.Run(name, func(t *testing.T) {
			m.Transform = place
			if name == "plane" {
				m.Transform.Position = geometry.Vector3{Y: -1.5, Z: 6}
			}
			wire, glyphs, pixels := newShadeBuffer(t), newShadeBuffer(t), newShadeBuffer(t)
			m.Draw(wire, cam, engine.Cell{Character: '#'})
			m.withColor(engine.Palette(33)).DrawShaded(glyphs, cam, light)
			canvas := pixels.Canvas()
			m.withColor(engine.Palette(33)).DrawShadedPixels(canvas, cam, light)
			canvas.Compose()

			for what, fb := range map[string]*engine.FrameBuffer{"wireframe": wire, "shaded": glyphs, "pixels": pixels} {
				if distinctChars(fb) == 0 {
					t.Errorf("wanted the %s drawing to show something", what)
				}
			}
			if name != "plane" { // a flat surface lit from one direction has one shade
				faceted := m.Clone()
				faceted.Normals = nil
				flat := newShadeBuffer(t)
				faceted.withColor(engine.Palette(33)).DrawShaded(flat, cam, light)
				if s, f := distinctChars(glyphs), distinctChars(flat); s < f {
					t.Errorf("wanted the smooth shape to show at least as many shades as the faceted one's %d, got %d", f, s)
				}
			}
		})
	}
}

// A shape of no size has nothing to light, and as a wireframe is at most a dot.
func TestShapesWithNoSizeShowNothingShaded(t *testing.T) {
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.1)
	for name, m := range map[string]Mesh{
		"sphere": Sphere(0, 8, 4), "cylinder": Cylinder(0, 0, 8), "cone": Cone(0, 0, 8), "plane": Plane(0, 0),
	} {
		m.Transform.Position = geometry.Vector3{Z: 5}
		m.DoubleSided = true
		shaded, wire := newShadeBuffer(t), newShadeBuffer(t)
		m.DrawShaded(shaded, shadeCam(), light)
		m.Draw(wire, shadeCam(), engine.Cell{Character: '#'})

		if got := distinctChars(shaded); got != 0 {
			t.Errorf("%s: wanted nothing shaded, got %d characters\n%s", name, got, shaded.String())
		}
		cells := 0
		for y := range wire.Height() {
			for x := range wire.Width() {
				if wire.At(x, y) != engine.BlankCell {
					cells++
				}
			}
		}
		if cells > 1 {
			t.Errorf("%s: wanted a wireframe of no size to be at most a dot, got %d cells", name, cells)
		}
	}
}

// Arguments of any kind give a valid shape of a size the package promises, in the
// time it promises, with finite vertices and unit (or zero) normals.
func FuzzShapes(f *testing.F) {
	f.Add(1.0, 2.0, 8, 4)
	f.Add(math.NaN(), math.Inf(1), -1, math.MinInt)
	f.Add(-1.0, math.MaxFloat64, math.MaxInt, 0)
	f.Add(math.MaxFloat64, math.MaxFloat64, 512, 512)
	f.Add(5e-324, 5e-324, 3, 2)
	f.Add(0.0, 0.0, math.MaxInt, math.MaxInt)

	f.Fuzz(func(t *testing.T, a, b float64, segments, rings int) {
		for name, m := range map[string]Mesh{
			"sphere": Sphere(a, segments, rings), "cylinder": Cylinder(a, b, segments), "cone": Cone(a, b, segments), "plane": Plane(a, b),
		} {
			if err := m.Validate(); err != nil {
				t.Fatalf("%s(%v, %v, %d, %d): wanted a valid mesh, got %v", name, a, b, segments, rings, err)
			}
			if len(m.Normals) != len(m.Vertices) || len(m.Vertices) > 512*511+2 || len(m.Faces) > 2*512*511 {
				t.Fatalf("%s: wanted one normal per vertex and no more than the limit, got %d normals, %d vertices, %d faces", name, len(m.Normals), len(m.Vertices), len(m.Faces))
			}
			for i, n := range m.Normals {
				if l := n.Length(); l != 0 && math.Abs(l-1) > 1e-9 {
					t.Fatalf("%s: normal %d is %v, of length %v", name, i, n, l)
				}
			}
		}
	})
}

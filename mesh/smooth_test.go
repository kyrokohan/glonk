package mesh

import (
	"math"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// call is one question a light was asked.
type call struct{ point, normal geometry.Vector3 }

// loggingLight remembers every question it is asked and always answers level.
type loggingLight struct {
	calls []call
	level float64
}

func (r *loggingLight) Brightness(point, normal geometry.Vector3) float64 {
	r.calls = append(r.calls, call{point, normal})
	return r.level
}

// barycentric3 is where p sits in the triangle a, b, c in space, as weights of a, b and
// c that sum to 1. It looks only at the triangle, not at any screen.
func barycentric3(p, a, b, c geometry.Vector3) (wa, wb, wc float64) {
	n := b.Sub(a).Cross(c.Sub(a))
	d := n.LengthSq()
	return b.Sub(p).Cross(c.Sub(p)).Dot(n) / d, c.Sub(p).Cross(a.Sub(p)).Dot(n) / d, a.Sub(p).Cross(b.Sub(p)).Dot(n) / d
}

func vecNear(a, b geometry.Vector3, tol float64) bool {
	return a.Sub(b).Length() <= tol
}

// worldDirection is the unit direction a model-space direction points in once the
// model is placed: turned, with the scale and position taken out again.
func worldDirection(tr Transform, d geometry.Vector3) geometry.Vector3 {
	return tr.Apply(d).Sub(tr.Apply(geometry.Vector3{})).Normalize()
}

type smoothScene struct {
	name        string
	tri         [3]geometry.Vector3
	normals     [3]geometry.Vector3
	zero        int // the normal to leave out (zero), or -1
	tr          Transform
	cam         Camera
	doubleSided bool
	back        bool // the camera sees the back of the face
}

var smoothScenes = []smoothScene{
	{
		name:    "slopes away from the camera",
		tri:     [3]geometry.Vector3{{X: -3, Y: -2, Z: 3}, {X: 0, Y: 3, Z: 6}, {X: 3, Y: -2, Z: 9}},
		normals: [3]geometry.Vector3{{Z: -2}, {X: 0.3, Z: -0.3}, {Y: 5, Z: -5}},
		zero:    -1,
		cam:     Camera{Near: 0.1},
	},
	{
		name:    "placed and turned and scaled",
		tri:     [3]geometry.Vector3{{X: -2, Y: -1}, {Y: 2}, {X: 2, Y: -1}},
		normals: [3]geometry.Vector3{{X: -1, Z: -1}, {Y: 1, Z: -0.2}, {X: 3, Z: -3}},
		zero:    -1,
		tr:      Transform{Position: geometry.Vector3{X: 0.5, Z: 6}, Yaw: 0.5, Pitch: -0.3, Roll: 0.4, Scale: 1.5},
		cam:     Camera{Near: 0.1},
	},
	{
		name:    "cut by the near plane",
		tri:     [3]geometry.Vector3{{X: -3, Y: -1, Z: -2}, {Y: 3, Z: 6}, {X: 4, Y: -1, Z: 6}},
		normals: [3]geometry.Vector3{{X: -1, Z: -1}, {Y: 1, Z: -1}, {X: 1, Z: -1}},
		zero:    -1,
		cam:     Camera{Near: 0.5},
	},
	{
		name:    "one normal missing",
		tri:     [3]geometry.Vector3{{X: -3, Y: -2, Z: 3}, {X: 0, Y: 3, Z: 6}, {X: 3, Y: -2, Z: 9}},
		normals: [3]geometry.Vector3{{Z: -2}, {X: 0.3, Z: -0.3}, {Y: 5, Z: -5}},
		zero:    1,
		cam:     Camera{Near: 0.1},
	},
	{
		name:    "scaled, with a normal missing",
		tri:     [3]geometry.Vector3{{X: -2, Y: -1}, {Y: 2}, {X: 2, Y: -1}},
		normals: [3]geometry.Vector3{{X: -1, Z: -1}, {Y: 1, Z: -0.2}, {X: 3, Z: -3}},
		zero:    2,
		tr:      Transform{Position: geometry.Vector3{X: 0.5, Z: 6}, Yaw: 0.5, Scale: 3},
		cam:     Camera{Near: 0.1},
	},
	{
		name:        "seen from behind, double sided",
		tri:         [3]geometry.Vector3{{X: -3, Y: -2, Z: 3}, {X: 0, Y: 3, Z: 6}, {X: 3, Y: -2, Z: 9}},
		normals:     [3]geometry.Vector3{{Z: -2}, {X: 0.3, Z: -0.3}, {Y: 5, Z: -5}},
		zero:        -1,
		cam:         Camera{Position: geometry.Vector3{Z: 12}, Yaw: math.Pi, Near: 0.1},
		doubleSided: true,
		back:        true,
	},
	{
		name:        "seen from behind, with a normal missing",
		tri:         [3]geometry.Vector3{{X: -3, Y: -2, Z: 3}, {X: 0, Y: 3, Z: 6}, {X: 3, Y: -2, Z: 9}},
		normals:     [3]geometry.Vector3{{Z: -2}, {X: 0.3, Z: -0.3}, {Y: 5, Z: -5}},
		zero:        0,
		cam:         Camera{Position: geometry.Vector3{Z: 12}, Yaw: math.Pi, Near: 0.1},
		doubleSided: true,
		back:        true,
	},
}

func (sc smoothScene) mesh() Mesh {
	m := Mesh{
		Vertices:    sc.tri[:],
		Faces:       [][3]int{{0, 1, 2}},
		Normals:     append([]geometry.Vector3(nil), sc.normals[:]...),
		Transform:   sc.tr,
		DoubleSided: sc.doubleSided,
	}
	if sc.zero >= 0 {
		m.Normals[sc.zero] = geometry.Vector3{}
	}
	return m
}

// The light is asked about each cell the face covers, with where that cell is on the
// surface and which way the surface faces there: both worked out from the corners
// as they are in space, perspective and all. The scenes are checked against that
// independent answer.
func TestSmoothShadingAsksTheLightAboutEveryCell(t *testing.T) {
	for _, sc := range smoothScenes {
		t.Run(sc.name, func(t *testing.T) {
			m := sc.mesh()
			var world [3]geometry.Vector3
			for i, v := range sc.tri {
				world[i] = sc.tr.Apply(v)
			}
			faceNormal := world[1].Sub(world[0]).Cross(world[2].Sub(world[0])).Normalize()
			if sc.back {
				faceNormal = faceNormal.Neg()
			}
			var want [3]geometry.Vector3
			for i, n := range sc.normals {
				switch {
				case i == sc.zero:
					want[i] = faceNormal
				case sc.back:
					want[i] = worldDirection(sc.tr, n).Neg()
				default:
					want[i] = worldDirection(sc.tr, n)
				}
			}

			for _, pixels := range []bool{false, true} {
				name := "glyphs"
				if pixels {
					name = "pixels"
				}
				t.Run(name, func(t *testing.T) {
					fb := newShadeBuffer(t)
					light := &loggingLight{level: 0.5}
					if pixels {
						m.DrawShadedPixels(fb.Canvas(), sc.cam, light)
					} else {
						m.DrawShaded(fb, sc.cam, light)
					}

					if len(light.calls) < 30 {
						t.Fatalf("wanted the light asked about plenty of cells, got %d calls", len(light.calls))
					}
					cells := map[[2]int]bool{}
					for _, c := range light.calls {
						// Where it is: on the triangle, at the middle of one cell.
						wa, wb, wc := barycentric3(c.point, world[0], world[1], world[2])
						if wa < -1e-6 || wb < -1e-6 || wc < -1e-6 || math.Abs(wa+wb+wc-1) > 1e-6 {
							t.Fatalf("wanted a point on the triangle, got %v with weights %v %v %v", c.point, wa, wb, wc)
						}
						x, y, ok := sc.cam.Project(c.point, shadeW, shadeH)
						if pixels {
							y *= 2
						}
						if !ok || math.Abs(x-math.Floor(x)-0.5) > 1e-6 || math.Abs(y-math.Floor(y)-0.5) > 1e-6 {
							t.Fatalf("wanted the point to be at the middle of a cell, got %v at (%v, %v)", c.point, x, y)
						}
						cells[[2]int{int(math.Floor(x)), int(math.Floor(y))}] = true

						// Which way it faces: the corners' normals blended by the same weights.
						blend := want[0].Scale(wa).Add(want[1].Scale(wb)).Add(want[2].Scale(wc)).Normalize()
						if math.Abs(c.normal.Length()-1) > 1e-9 || !vecNear(c.normal, blend, 1e-9) {
							t.Fatalf("at %v wanted the unit normal %v, got %v", c.point, blend, c.normal)
						}
					}
					if len(cells) != len(light.calls) {
						t.Errorf("wanted the light asked once about each cell, got %d calls for %d cells", len(light.calls), len(cells))
					}
				})
			}
		})
	}
}

// A mesh with no Normals, or ones that cannot be used, is lit once per face as before,
// with the face's own middle and normal.
func TestFlatShadingAsksTheLightOncePerFace(t *testing.T) {
	sc := smoothScenes[0]
	good := sc.mesh()
	short := good.Clone()
	short.Normals = short.Normals[:2]
	long := good.Clone()
	long.Normals = append(long.Normals, geometry.Vector3{Z: -1})
	nan := good.Clone()
	nan.Normals[1].X = math.NaN()
	inf := good.Clone()
	inf.Normals[2].Y = math.Inf(1)
	none := good.Clone()
	none.Normals = nil

	for name, m := range map[string]Mesh{"none": none, "too few": short, "too many": long, "NaN": nan, "infinite": inf} {
		t.Run(name, func(t *testing.T) {
			light := &loggingLight{level: 0.5}
			m.DrawShaded(newShadeBuffer(t), sc.cam, light)

			if len(light.calls) != 1 {
				t.Fatalf("wanted one question for the one face, got %d", len(light.calls))
			}
			w := m.Vertices
			wantNormal := w[1].Sub(w[0]).Cross(w[2].Sub(w[0])).Normalize()
			wantCentre := w[0].Add(w[1]).Add(w[2]).Scale(1.0 / 3)
			if !vecNear(light.calls[0].normal, wantNormal, 1e-9) || !vecNear(light.calls[0].point, wantCentre, 1e-9) {
				t.Errorf("wanted the face's middle %v and normal %v, got %v", wantCentre, wantNormal, light.calls[0])
			}
		})
	}
}

// A normal of any finite size has a direction, even one too big or too small to measure.
func TestSmoothNormalsOfAnyFiniteSizeKeepTheirDirection(t *testing.T) {
	tests := []struct {
		name   string
		normal geometry.Vector3
		scale  float64 // of the model: this too must not push a normal over the edge
		want   geometry.Vector3
	}{
		{"huge", v3(math.MaxFloat64, math.MaxFloat64, -math.MaxFloat64), 0, v3(1, 1, -1).Normalize()},
		{"huge in a bigger model", v3(math.MaxFloat64, math.MaxFloat64, -math.MaxFloat64), 3, v3(1, 1, -1).Normalize()},
		{"tiny", v3(5e-324, 0, -5e-324), 0, v3(1, 0, -1).Normalize()},
		{"tiny in a smaller model", v3(5e-324, 0, -5e-324), 0.3, v3(1, 0, -1).Normalize()},
		{"tiny and mixed", v3(1e-310, 3e-310, -1e-310), 0, v3(1, 3, -1).Normalize()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := smoothScenes[0].mesh()
			m.Normals = []geometry.Vector3{tt.normal, tt.normal, tt.normal}
			m.Transform.Scale = tt.scale
			light := &loggingLight{level: 0.5}

			m.DrawShaded(newShadeBuffer(t), smoothScenes[0].cam, light)

			if len(light.calls) < 30 {
				t.Fatalf("wanted smooth shading, got %d questions", len(light.calls))
			}
			for _, c := range light.calls {
				if !vecNear(c.normal, tt.want, 1e-9) {
					t.Fatalf("wanted every normal to be %v, got %v", tt.want, c.normal)
				}
			}
		})
	}
}

// Normals of zero all round mean "none here" at every corner, so every cell takes the
// face's own normal and the picture matches flat shading.
func TestZeroNormalsShadeLikeFlat(t *testing.T) {
	cube := Cube(2)
	cube.Transform = Transform{Position: geometry.Vector3{Z: 5}, Yaw: 0.6, Pitch: -0.4}
	cube.Material.Color = engine.Palette(208)
	zeroed := cube.Clone()
	zeroed.Normals = make([]geometry.Vector3, len(cube.Vertices))
	light := goldenSun

	flat, smooth := newShadeBuffer(t), newShadeBuffer(t)
	cube.DrawShaded(flat, shadeCam(), light)
	zeroed.DrawShaded(smooth, shadeCam(), light)

	if picture(flat) != picture(smooth) {
		t.Errorf("wanted zero normals to shade like no normals\n%s\nwanted\n%s", smooth.String(), flat.String())
	}
}

func distinctChars(fb *engine.FrameBuffer) int {
	seen := map[rune]bool{}
	for y := range fb.Height() {
		for x := range fb.Width() {
			if c := fb.At(x, y); c != engine.BlankCell {
				seen[c.Character] = true
			}
		}
	}
	return len(seen)
}

// Across one face the shade changes with the normals, which flat shading cannot show.
func TestSmoothShadingChangesWithinAFace(t *testing.T) {
	tri := Mesh{
		Vertices: []geometry.Vector3{{X: -4, Y: -3, Z: 5}, {Y: 4, Z: 5}, {X: 4, Y: -3, Z: 5}},
		Faces:    [][3]int{{0, 1, 2}},
		// Fanned out to left and right, facing the camera in the middle.
		Normals: []geometry.Vector3{{X: -1, Z: -0.3}, {Z: -1}, {X: 1, Z: -0.3}},
	}
	// Light travelling left: it meets the right-facing side squarely and the left side edge-on.
	light := Directional{Direction: geometry.Vector3{X: -1}}

	smooth, flat := newShadeBuffer(t), newShadeBuffer(t)
	tri.DrawShaded(smooth, shadeCam(), light)
	tri.Normals = nil
	tri.DrawShaded(flat, shadeCam(), light)

	if got := distinctChars(flat); got != 1 {
		t.Errorf("wanted one face lit as one shade, got %d characters", got)
	}
	if got := distinctChars(smooth); got < 5 {
		t.Errorf("wanted the smooth face to show a gradient of at least 5 characters, got %d\n%s", got, smooth.String())
	}
}

// A smooth sphere shows most of the ramp, a faceted one of the same shape only as many
// shades as it has facets that differ.
func TestSmoothSphereUsesMoreShadesThanFaceted(t *testing.T) {
	sphere := Sphere(2, 10, 5)
	sphere.Transform.Position = geometry.Vector3{Z: 5}
	faceted := sphere.Clone()
	faceted.Normals = nil
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.1)

	smoothFB, facetedFB := newShadeBuffer(t), newShadeBuffer(t)
	sphere.DrawShaded(smoothFB, shadeCam(), light)
	faceted.DrawShaded(facetedFB, shadeCam(), light)

	s, f := distinctChars(smoothFB), distinctChars(facetedFB)
	t.Logf("smooth %d shades, faceted %d", s, f)
	if s < 8 || s <= f {
		t.Errorf("wanted the smooth sphere to use at least 8 shades and more than the faceted one's %d, got %d\n%s", f, s, smoothFB.String())
	}
}

// Normals turn with the model, and its scale leaves them unit length.
func TestSmoothNormalsFollowTheTransform(t *testing.T) {
	slab := Mesh{
		Vertices:  []geometry.Vector3{{X: -1, Y: -1}, {Y: 1}, {X: 1, Y: -1}},
		Faces:     [][3]int{{0, 1, 2}},
		Normals:   []geometry.Vector3{{Z: -1}, {Z: -1}, {Z: -1}},
		Transform: Transform{Position: geometry.Vector3{Z: 4}, Yaw: 0.8},
	}
	scaled := slab
	scaled.Transform.Scale = 2.5
	want := worldDirection(slab.Transform, geometry.Vector3{Z: -1})

	for name, m := range map[string]Mesh{"yawed": slab, "yawed and scaled": scaled} {
		t.Run(name, func(t *testing.T) {
			light := &loggingLight{level: 0.5}
			m.DrawShaded(newShadeBuffer(t), shadeCam(), light)

			if len(light.calls) == 0 {
				t.Fatal("wanted the face drawn")
			}
			for _, c := range light.calls {
				if !vecNear(c.normal, want, 1e-9) {
					t.Fatalf("wanted every normal turned with the model to %v, got %v", want, c.normal)
				}
			}
		})
	}
}

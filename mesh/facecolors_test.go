package mesh

import (
	"slices"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

var (
	red   = engine.RGB(230, 60, 50)
	green = engine.RGB(50, 200, 90)
	blue  = engine.RGB(60, 110, 230)
)

// colorsCycling gives each of n faces a color, going round red, green, blue and none (the
// zero Color), which leaves that face to Material.Color.
func colorsCycling(n int) []engine.Color {
	cycle := []engine.Color{red, green, blue, {}}
	out := make([]engine.Color, n)
	for i := range out {
		out[i] = cycle[i%len(cycle)]
	}
	return out
}

// aloneFace is face i of m as a mesh of its own, in color. Everything else is the same, so
// it is placed, lit and clipped as it is in m.
func aloneFace(m Mesh, i int, color engine.Color) Mesh {
	one := m
	one.Faces = [][3]int{m.Faces[i]}
	one.FaceColors = nil
	one.Material.Color = color
	return one
}

// A mesh with FaceColors draws as its faces would drawn one at a time, each in its own
// color (or Material.Color where it has none), in the same order. That holds for glyphs and
// pixels, flat and smooth, from either side, with a face the mesh cannot draw in the middle
// of the list, and with the near plane cutting through.
func TestFaceColorsDrawEachFaceInItsColor(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)

	flat := Cube(2)
	flat.Transform = Transform{Position: geometry.Vector3{Z: 5}, Yaw: 0.6, Pitch: -0.4}
	smooth := Sphere(1.6, 10, 6)
	smooth.Transform = Transform{Position: geometry.Vector3{Z: 4}, Yaw: 0.3}
	both := Cube(2)
	both.Transform = Transform{Position: geometry.Vector3{Z: 5}, Yaw: 2.6, Pitch: 0.3}
	both.DoubleSided = true // seen from behind
	withBadFace := flat.Clone()
	withBadFace.Faces = slices.Insert(withBadFace.Faces, 3, [3]int{0, 1, 99}) // a face that names a vertex it lacks
	cut := Cube(2)
	cut.Transform = Transform{Position: geometry.Vector3{X: 2, Z: 1}, Yaw: 0.1}

	tests := []struct {
		name string
		m    Mesh
		cam  Camera
	}{
		{"flat", flat, shadeCam()},
		{"smooth", smooth, shadeCam()},
		{"seen from behind", both, shadeCam()},
		{"a face that cannot be drawn in the list", withBadFace, shadeCam()},
		{"cut by the near plane", cut, Camera{FOV: 1.4, Near: 0.5}},
	}
	for _, tt := range tests {
		for _, pixels := range []bool{false, true} {
			name := tt.name + ", glyphs"
			if pixels {
				name = tt.name + ", pixels"
			}
			t.Run(name, func(t *testing.T) {
				m := tt.m.withColor(engine.Palette(208))
				m.FaceColors = colorsCycling(len(m.Faces))
				draw := func(m Mesh, fb *engine.FrameBuffer) {
					if pixels {
						canvas := fb.Canvas()
						m.DrawShadedPixels(canvas, tt.cam, light)
						canvas.Compose()
					} else {
						m.DrawShaded(fb, tt.cam, light)
					}
				}

				got, want := newShadeBuffer(t), newShadeBuffer(t)
				draw(m, got)
				for i := range m.Faces {
					color := m.FaceColors[i]
					if color.IsDefault() {
						color = m.Material.Color
					}
					draw(aloneFace(m, i, color), want)
				}

				if picture(got) != picture(want) {
					t.Fatalf("wanted the faces drawn one at a time in their colors\n%s\ngot\n%s", want.String(), got.String())
				}
				seen := map[engine.Color]bool{}
				for y := range got.Height() {
					for x := range got.Width() {
						c := got.At(x, y)
						seen[c.Foreground], seen[c.Background] = true, true
					}
				}
				if len(seen) < 3 { // the blank, and at least two of the face colors
					t.Errorf("wanted several colors on screen, got %d", len(seen))
				}
			})
		}
	}
}

// A face's color does not move to another face when the face before it is skipped, and
// a list of the wrong length colors nothing.
func TestFaceColorsFollowTheFaceNotTheDrawing(t *testing.T) {
	m := Mesh{
		Vertices: []geometry.Vector3{{X: -3, Y: -1, Z: 4}, {X: -1, Y: -1, Z: 4}, {X: -2, Y: 1, Z: 4}, {X: 1, Y: -1, Z: 4}, {X: 3, Y: -1, Z: 4}, {X: 2, Y: 1, Z: 4}},
		Faces:    [][3]int{{0, 2, 1}, {0, 1, 9}, {3, 5, 4}}, // the middle one cannot be drawn
	}
	m.FaceColors = []engine.Color{red, green, blue}

	fb := newShadeBuffer(t)
	m.DrawShaded(fb, shadeCam(), Ambient{Level: 0.5})

	left, right := fb.At(25, 14).Foreground, fb.At(55, 14).Foreground
	if left != red || right != blue {
		t.Errorf("wanted the first face red and the third blue, got %v and %v\n%s", left, right, fb.String())
	}
}

func TestFaceColorsOfTheWrongLengthColorNothing(t *testing.T) {
	cube := solidCube(geometry.Vector3{Z: 5}, 2)
	plain := newShadeBuffer(t)
	cube.DrawShaded(plain, shadeCam(), goldenSun)

	for _, n := range []int{1, len(cube.Faces) - 1, len(cube.Faces) + 1, 100} {
		m := cube.Clone()
		m.FaceColors = colorsCycling(n)
		fb := newShadeBuffer(t)

		m.DrawShaded(fb, shadeCam(), goldenSun)

		if picture(fb) != picture(plain) {
			t.Errorf("%d colors for %d faces: wanted them all ignored", n, len(cube.Faces))
		}
		err := m.Validate()
		if err == nil || !strings.Contains(err.Error(), "face colors") {
			t.Errorf("%d colors for %d faces: wanted Validate to say so, got %v", n, len(cube.Faces), err)
		}
	}
}

func TestValidateAcceptsNoFaceColorsAndOneForEachFace(t *testing.T) {
	cube := Cube(1)
	if err := cube.Validate(); err != nil {
		t.Errorf("wanted none to be fine, got %v", err)
	}
	cube.FaceColors = colorsCycling(len(cube.Faces))
	if err := cube.Validate(); err != nil {
		t.Errorf("wanted one for each face to be fine, got %v", err)
	}
	if err := (Mesh{FaceColors: []engine.Color{red}}).Validate(); err == nil {
		t.Errorf("wanted a color for a mesh with no faces to be reported")
	}
}

func TestCloneFitAndSmoothNormalsKeepFaceColors(t *testing.T) {
	m := Cube(4)
	m.FaceColors = colorsCycling(len(m.Faces))

	clone := m.Clone()
	clone.FaceColors[0] = engine.RGB(1, 2, 3)
	fitted := m.Fit(2)
	smoothed := m.SmoothNormals()

	if m.FaceColors[0] != red {
		t.Errorf("wanted the clone's colors to be its own, but the original changed to %v", m.FaceColors[0])
	}
	if !slices.Equal(fitted.FaceColors, m.FaceColors) || !slices.Equal(smoothed.FaceColors, m.FaceColors) {
		t.Errorf("wanted Fit and SmoothNormals to keep the colors")
	}
	fitted.FaceColors[1] = engine.RGB(4, 5, 6)
	if m.FaceColors[1] == fitted.FaceColors[1] {
		t.Errorf("wanted the fitted copy's colors to be its own")
	}
	if Cube(1).Clone().FaceColors != nil {
		t.Errorf("wanted a mesh without colors to clone without them")
	}
}

// A color is only the color: a wireframe is drawn in the cell it is given.
func TestFaceColorsDoNotChangeAWireframe(t *testing.T) {
	m := solidCube(geometry.Vector3{Z: 5}, 2)
	plain, colored := newShadeBuffer(t), newShadeBuffer(t)
	m.Draw(plain, shadeCam(), wireCell)
	m.FaceColors = colorsCycling(len(m.Faces))
	m.Draw(colored, shadeCam(), wireCell)

	if picture(plain) != picture(colored) {
		t.Errorf("wanted the wireframe to be the same")
	}
}

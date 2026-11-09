package mesh

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// drawEverything draws m with all four ways to draw a mesh into a fresh frame.
func drawEverything(t *testing.T, m Mesh) *engine.FrameBuffer {
	t.Helper()
	return drawWith(t, m, Camera{FOV: math.Pi / 2, Near: 0.1})
}

// drawWith is drawEverything seen through cam.
func drawWith(t *testing.T, m Mesh, cam Camera) *engine.FrameBuffer {
	t.Helper()

	fb, err := engine.NewFrameBuffer(40, 12)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	light := sunAndFill(geometry.Vector3{Y: -1}, 0.2)

	m.Draw(fb, cam, engine.Cell{Character: '#'})
	m.DoubleSided = true
	m.withColor(engine.Palette(1)).DrawShaded(fb, cam, light)
	m.withColor(engine.Palette(2)).DrawShadedPixels(fb.Canvas(), cam, light)
	fb.Canvas().Compose()
	return fb
}

func drawnCells(fb *engine.FrameBuffer) int {
	n := 0
	for y := range fb.Height() {
		for x := range fb.Width() {
			if fb.At(x, y) != engine.BlankCell {
				n++
			}
		}
	}
	return n
}

// triangle is three vertices in front of the camera.
func triangle() []geometry.Vector3 {
	return []geometry.Vector3{{X: -2, Y: -1, Z: 5}, {X: 2, Y: -1, Z: 5}, {X: 0, Y: 2, Z: 5}}
}

func TestDrawSkipsEdgesAndFacesThatNameMissingVertices(t *testing.T) {
	good := Mesh{Vertices: triangle(), Edges: [][2]int{{0, 1}, {1, 2}}, Faces: [][3]int{{0, 1, 2}}}
	want := drawEverything(t, good)
	if drawnCells(want) == 0 {
		t.Fatalf("wanted the good mesh to draw something")
	}

	badEdges := [][2]int{{1, 99}, {-1, 2}, {3, 0}, {0, 3}, {0, math.MaxInt}, {math.MinInt, 1}, {-5, 77}}
	badFaces := [][3]int{{0, 1, 3}, {-1, 1, 2}, {0, 99, 2}, {math.MaxInt, 0, 1}, {0, 1, math.MinInt}, {5, 6, 7}}

	for _, e := range badEdges {
		t.Run(fmt.Sprintf("edge %v", e), func(t *testing.T) {
			m := good
			m.Edges = append(append([][2]int{e}, good.Edges...), e) // first and last
			if got := drawEverything(t, m); !sameFrame(got, want) {
				t.Errorf("wanted the bad edge %v skipped and the rest drawn as before", e)
			}
		})
	}
	for _, f := range badFaces {
		t.Run(fmt.Sprintf("face %v", f), func(t *testing.T) {
			m := good
			m.Faces = append(append([][3]int{f}, good.Faces...), f)
			if got := drawEverything(t, m); !sameFrame(got, want) {
				t.Errorf("wanted the bad face %v skipped and the rest drawn as before", f)
			}
		})
	}
}

func TestDrawWithoutVerticesDrawsNothing(t *testing.T) {
	m := Mesh{Edges: [][2]int{{0, 1}}, Faces: [][3]int{{0, 1, 2}}}
	if n := drawnCells(drawEverything(t, m)); n != 0 {
		t.Errorf("wanted nothing drawn without vertices, got %d cells", n)
	}
}

func TestValidate(t *testing.T) {
	nan := math.NaN()

	tests := []struct {
		name string
		m    Mesh
		want string // "" means valid
	}{
		{"empty mesh", Mesh{}, ""},
		{"cube", Cube(2), ""},
		{"grid", Grid(10, 4), ""},
		{"triangle", Mesh{Vertices: triangle(), Edges: [][2]int{{0, 1}}, Faces: [][3]int{{0, 1, 2}}}, ""},
		{"vertices only", Mesh{Vertices: triangle()}, ""},
		{"last vertex is fine", Mesh{Vertices: triangle(), Edges: [][2]int{{2, 2}}}, ""},

		{"edge past the end", Mesh{Vertices: triangle(), Edges: [][2]int{{0, 1}, {1, 3}}}, "mesh: edge 1 uses vertex 3, but only vertices 0 to 2 exist"},
		{"edge first end", Mesh{Vertices: triangle(), Edges: [][2]int{{9, 1}}}, "mesh: edge 0 uses vertex 9, but only vertices 0 to 2 exist"},
		{"edge negative", Mesh{Vertices: triangle(), Edges: [][2]int{{0, -1}}}, "mesh: edge 0 uses vertex -1, but only vertices 0 to 2 exist"},
		{"face past the end", Mesh{Vertices: triangle(), Faces: [][3]int{{0, 1, 2}, {0, 1, 3}}}, "mesh: face 1 uses vertex 3, but only vertices 0 to 2 exist"},
		{"face first corner", Mesh{Vertices: triangle(), Faces: [][3]int{{3, 1, 2}}}, "mesh: face 0 uses vertex 3, but only vertices 0 to 2 exist"},
		{"face negative", Mesh{Vertices: triangle(), Faces: [][3]int{{0, math.MinInt, 2}}}, fmt.Sprintf("mesh: face 0 uses vertex %d, but only vertices 0 to 2 exist", math.MinInt)},
		{"edges without vertices", Mesh{Edges: [][2]int{{0, 1}}}, "mesh: edge 0 uses vertex 0, but the mesh has no vertices"},
		{"faces without vertices", Mesh{Faces: [][3]int{{0, 1, 2}}}, "mesh: face 0 uses vertex 0, but the mesh has no vertices"},
		{"edge reported before face", Mesh{Vertices: triangle(), Edges: [][2]int{{0, 8}}, Faces: [][3]int{{0, 1, 9}}}, "mesh: edge 0 uses vertex 8, but only vertices 0 to 2 exist"},

		{"NaN vertex", Mesh{Vertices: []geometry.Vector3{{}, {Y: nan}}}, "mesh: vertex 1 is not a finite number: {0 NaN 0}"},
		{"infinite vertex", Mesh{Vertices: []geometry.Vector3{{Z: math.Inf(-1)}}}, "mesh: vertex 0 is not a finite number: {0 0 -Inf}"},
		{"transform with a NaN", Mesh{Transform: Transform{Roll: nan}}, "mesh: the transform has a number that is not finite or a negative scale: " + fmt.Sprintf("%+v", Transform{Roll: nan})},
		{"transform with a negative scale", Mesh{Transform: Transform{Scale: -2}}, "mesh: the transform has a number that is not finite or a negative scale: " + fmt.Sprintf("%+v", Transform{Scale: -2})},
		{"transform reported before vertex", Mesh{Vertices: []geometry.Vector3{{X: nan}}, Transform: Transform{Scale: -1}}, "mesh: the transform has a number that is not finite or a negative scale: " + fmt.Sprintf("%+v", Transform{Scale: -1})},
		{"vertex reported before edge", Mesh{Vertices: []geometry.Vector3{{X: nan}}, Edges: [][2]int{{0, 5}}}, "mesh: vertex 0 is not a finite number: {NaN 0 0}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("wanted a valid mesh, got %v", err)
			case tt.want != "" && err == nil:
				t.Errorf("wanted error %q, got nil", tt.want)
			case tt.want != "" && err.Error() != tt.want:
				t.Errorf("wanted error %q, got %q", tt.want, err)
			}
		})
	}
}

func TestValidateAcceptsWhatLoadOBJReturns(t *testing.T) {
	obj := "v 0 0 0\nv 1 0 0\nv 1 1 0\nv 0 1 0\nf 1 2 3 4\nf -4 -3 -2\nl 1 3\n"
	m, err := LoadOBJ(strings.NewReader(obj))
	if err != nil {
		t.Fatalf("unable to load: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("wanted a loaded model to be valid, got %v", err)
	}
}

func TestClone(t *testing.T) {
	orig := Cube(2)
	orig.Transform.Position = geometry.Vector3{X: 1, Y: 2, Z: 3}
	orig.Transform.Yaw, orig.Transform.Pitch, orig.DoubleSided = 0.5, 0.25, true

	c := orig.Clone()

	if c.Transform.Position != orig.Transform.Position || c.Transform.Yaw != orig.Transform.Yaw || c.Transform.Pitch != orig.Transform.Pitch || c.DoubleSided != orig.DoubleSided {
		t.Errorf("wanted placement and flags copied, got %+v", c)
	}
	if !slices.Equal(c.Vertices, orig.Vertices) || !slices.Equal(c.Edges, orig.Edges) || !slices.Equal(c.Faces, orig.Faces) {
		t.Fatalf("wanted the same vertices, edges and faces")
	}

	wantV, wantE, wantF := orig.Vertices[0], orig.Edges[0], orig.Faces[0]
	c.Vertices[0] = geometry.Vector3{X: 99}
	c.Edges[0] = [2]int{7, 7}
	c.Faces[0] = [3]int{7, 7, 7}
	c.Transform.Position.X = 50

	if orig.Vertices[0] != wantV {
		t.Errorf("wanted the original's vertex untouched, got %v", orig.Vertices[0])
	}
	if orig.Edges[0] != wantE {
		t.Errorf("wanted the original's edge untouched, got %v", orig.Edges[0])
	}
	if orig.Faces[0] != wantF {
		t.Errorf("wanted the original's face untouched, got %v", orig.Faces[0])
	}
	if orig.Transform.Position.X != 1 {
		t.Errorf("wanted the original's position untouched, got %v", orig.Transform.Position)
	}
}

func TestCloneKeepsNilListsNil(t *testing.T) {
	c := Mesh{Vertices: triangle()}.Clone()
	if c.Edges != nil || c.Faces != nil {
		t.Errorf("wanted missing edges and faces to stay nil, got %v and %v", c.Edges, c.Faces)
	}
	if got := (Mesh{}).Clone(); got.Vertices != nil || got.Edges != nil || got.Faces != nil {
		t.Errorf("wanted an empty mesh to clone to an empty mesh, got %+v", got)
	}
}

// Plain assignment shares the lists; Clone is the way out.
func TestAssignmentSharesAndCloneDoesNot(t *testing.T) {
	a := Cube(2)
	b := a
	b.Vertices[0].X = 42
	if a.Vertices[0].X != 42 {
		t.Fatalf("wanted assignment to share the vertex list")
	}

	c := a.Clone()
	c.Vertices[0].X = -1
	if a.Vertices[0].X != 42 {
		t.Errorf("wanted the clone's change to stay out of the original")
	}
}

func cubeAt(z, size float64) Mesh {
	c := Cube(size)
	c.Transform.Position = geometry.Vector3{Z: z}
	return c
}

func TestCameraLeftAtZeroUsesDefaults(t *testing.T) {
	explicit := Camera{FOV: math.Pi / 2, Near: 0.1}
	negativeZero := math.Copysign(0, -1)

	tests := []struct {
		name string
		cam  Camera
		want Camera
	}{
		{"nothing set", Camera{}, explicit},
		{"only a position", Camera{Position: geometry.Vector3{X: 0.5, Y: 0.25}}, Camera{Position: geometry.Vector3{X: 0.5, Y: 0.25}, FOV: math.Pi / 2, Near: 0.1}},
		{"only a turn", Camera{Yaw: 0.3, Pitch: 0.1}, Camera{Yaw: 0.3, Pitch: 0.1, FOV: math.Pi / 2, Near: 0.1}},
		{"FOV set, Near not", Camera{FOV: 1.2}, Camera{FOV: 1.2, Near: 0.1}},
		{"Near set, FOV not", Camera{Near: 0.5}, Camera{FOV: math.Pi / 2, Near: 0.5}},
		{"CellAspect set, the rest not", Camera{CellAspect: 3}, Camera{FOV: math.Pi / 2, Near: 0.1, CellAspect: 3}},
		{"CellAspect 2 is the default", Camera{CellAspect: 2}, explicit},
		{"negative zero counts as zero", Camera{FOV: negativeZero, Near: negativeZero}, explicit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The second cube straddles 0.3 in front of the camera, so which Near is used
			// changes what is cut away; the first only needs a FOV.
			for _, m := range []Mesh{cubeAt(5, 2), cubeAt(0.3, 0.8)} {
				got := drawWith(t, m, tt.cam)
				want := drawWith(t, m, tt.want)
				if drawnCells(got) == 0 {
					t.Fatalf("wanted the camera to see the cube at %v", m.Transform.Position)
				}
				if !sameFrame(got, want) {
					t.Errorf("wanted %+v to draw like %+v for the cube at %v", tt.cam, tt.want, m.Transform.Position)
				}
			}

			p := geometry.Vector3{X: 1, Y: 0.5, Z: 4}
			gx, gy, gok := tt.cam.Project(p, 40, 12)
			wx, wy, wok := tt.want.Project(p, 40, 12)
			if gok != wok || gx != wx || gy != wy {
				t.Errorf("wanted Project to give (%v, %v, %v), got (%v, %v, %v)", wx, wy, wok, gx, gy, gok)
			}
		})
	}
}

func TestCameraWithNonsenseValuesSeesNothing(t *testing.T) {
	inf := math.Inf(1)
	nan := math.NaN()

	tests := []struct {
		name string
		cam  Camera
	}{
		{"negative FOV", Camera{FOV: -1, Near: 0.1}},
		{"FOV of pi", Camera{FOV: math.Pi, Near: 0.1}},
		{"FOV past pi", Camera{FOV: 4, Near: 0.1}},
		{"FOV of a full turn", Camera{FOV: 2 * math.Pi, Near: 0.1}},
		{"NaN FOV", Camera{FOV: nan, Near: 0.1}},
		{"infinite FOV", Camera{FOV: inf, Near: 0.1}},
		{"negative infinite FOV", Camera{FOV: -inf, Near: 0.1}},
		{"negative Near", Camera{FOV: math.Pi / 2, Near: -1}},
		{"NaN Near", Camera{FOV: math.Pi / 2, Near: nan}},
		{"infinite Near", Camera{FOV: math.Pi / 2, Near: inf}},
		{"negative infinite Near", Camera{FOV: math.Pi / 2, Near: -inf}},
		{"negative CellAspect", Camera{CellAspect: -2}},
		{"NaN CellAspect", Camera{CellAspect: nan}},
		{"infinite CellAspect", Camera{CellAspect: inf}},
		{"negative infinite CellAspect", Camera{CellAspect: -inf}},
		{"NaN position", Camera{Position: geometry.Vector3{X: nan}, FOV: math.Pi / 2, Near: 0.1}},
		{"infinite yaw", Camera{Yaw: inf, FOV: math.Pi / 2, Near: 0.1}},
		{"NaN pitch", Camera{Pitch: nan, FOV: math.Pi / 2, Near: 0.1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if n := drawnCells(drawWith(t, cubeAt(5, 2), tt.cam)); n != 0 {
				t.Errorf("wanted nothing drawn, got %d cells", n)
			}
			if _, _, ok := tt.cam.Project(geometry.Vector3{Z: 5}, 40, 12); ok {
				t.Errorf("wanted Project to report ok=false")
			}
		})
	}
}

// A negative Near used to let things behind the camera through, drawn mirrored.
func TestNegativeNearNoLongerDrawsWhatIsBehindTheCamera(t *testing.T) {
	behind := cubeAt(-0.5, 0.2)
	if n := drawnCells(drawWith(t, behind, Camera{FOV: math.Pi / 2, Near: -1})); n != 0 {
		t.Errorf("wanted nothing drawn for a cube behind the camera, got %d cells", n)
	}
}

func TestWithDefaultsAcceptsTheEdgesOfTheRange(t *testing.T) {
	for _, c := range []Camera{
		{FOV: 1e-9, Near: 0.1},
		{FOV: math.Nextafter(math.Pi, 0), Near: 0.1},
		{FOV: math.Pi / 2, Near: 1e-300},
		{FOV: math.Pi / 2, Near: math.MaxFloat64},
		{CellAspect: 1e-300},
		{CellAspect: math.MaxFloat64},
	} {
		if _, ok := c.withDefaults(); !ok {
			t.Errorf("wanted %+v to be usable", c)
		}
	}
}

func TestProjectRejectsPointsWithNoPlaceOnTheScreen(t *testing.T) {
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}
	nan, inf := math.NaN(), math.Inf(1)

	for name, p := range map[string]geometry.Vector3{
		"NaN x":               {X: nan, Z: 5},
		"NaN y":               {Y: nan, Z: 5},
		"NaN z":               {Z: nan},
		"infinite x":          {X: inf, Z: 5},
		"negative infinite y": {Y: -inf, Z: 5},
		"infinitely far":      {Z: inf},
		"infinite in all":     {X: inf, Y: inf, Z: inf},
	} {
		if x, y, ok := cam.Project(p, 40, 12); ok {
			t.Errorf("%s: wanted ok=false, got (%v, %v)", name, x, y)
		}
	}
}

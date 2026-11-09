package mesh

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// These tests pin whole pictures: a handful of scenes drawn through every drawing
// path, kept as text under testdata/golden. A change to the 3D API must leave them
// as they are; a change to how things look rewrites them on purpose, with
//
//	go test ./mesh -run TestGoldenPictures -update
var updateGolden = flag.Bool("update", false, "rewrite the pictures in testdata/golden")

// picture writes a frame as three grids (characters, foreground, background) and a
// legend for the letters in the color grids; '.' is the terminal's default color.
func picture(fb *engine.FrameBuffer) string {
	legend := map[string]rune{}
	var names []string
	letter := func(c engine.Color) rune {
		if c.IsDefault() {
			return '.'
		}
		name := c.String()
		if r, ok := legend[name]; ok {
			return r
		}
		r := rune('a' + len(legend))
		legend[name] = r
		names = append(names, name)
		return r
	}

	var chars, fg, bg strings.Builder
	for y := range fb.Height() {
		for x := range fb.Width() {
			c := fb.At(x, y)
			chars.WriteRune(c.Character)
			fg.WriteRune(letter(c.Foreground))
			bg.WriteRune(letter(c.Background))
		}
		for _, sb := range []*strings.Builder{&chars, &fg, &bg} {
			sb.WriteByte('\n')
		}
	}

	var out strings.Builder
	out.WriteString(chars.String() + "\nforeground\n" + fg.String() + "\nbackground\n" + bg.String() + "\nlegend\n")
	for _, name := range names {
		fmt.Fprintf(&out, "%c %s\n", legend[name], name)
	}
	return out.String()
}

// goldenBuffer returns the blank 48x20 frame the scenes are drawn into.
func goldenBuffer(t *testing.T) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(48, 20)
	if err != nil {
		t.Fatal(err)
	}
	return fb
}

var goldenScenes = []struct {
	name string
	draw func(t *testing.T) *engine.FrameBuffer
}{
	{"wire-cube", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		cube := Cube(2)
		cube.Transform.Position = geometry.Vector3{Z: 3}
		cube.Transform.Yaw = 0.5
		cube.Draw(fb, Camera{}, engine.Cell{Character: '#', Foreground: engine.Palette(46)})
		return fb
	}},
	{"wire-grid-floor", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		floor := Grid(8, 8)
		floor.Transform.Position = geometry.Vector3{Y: -1, Z: 6}
		cam := Camera{Position: geometry.Vector3{Y: 0.5, Z: -2}, Yaw: 0.2, Pitch: -0.3, FOV: 1.2}
		floor.Draw(fb, cam, engine.Cell{Character: '.', Foreground: engine.Palette(240)})
		return fb
	}},
	{"shaded-turned-cube", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		cube := Cube(2)
		cube.Transform.Position = geometry.Vector3{Z: 5}
		cube.Transform.Yaw, cube.Transform.Pitch = 0.6, -0.4
		cube.withColor(engine.Palette(208)).DrawShaded(fb, shadeCam(), goldenSun)
		return fb
	}},
	{"shaded-two-cubes-and-floor", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		cam := Camera{Position: geometry.Vector3{Y: 1, Z: -3}, Pitch: -0.15, FOV: 1.3, Near: 0.1}
		floor := Grid(12, 6)
		floor.Transform.Position = geometry.Vector3{Y: -1, Z: 5}
		near, far := Cube(1.5), Cube(2)
		near.Transform.Position = geometry.Vector3{X: -1.2, Y: -0.2, Z: 3}
		near.Transform.Yaw = 0.7
		far.Transform.Position = geometry.Vector3{X: 1, Z: 7}
		far.Transform.Yaw, far.Transform.Pitch = -0.4, -0.2
		floor.Draw(fb, cam, engine.Cell{Character: ':', Foreground: engine.Palette(240)})
		far.withColor(engine.Palette(33)).DrawShaded(fb, cam, goldenSun)
		near.withColor(engine.Palette(196)).DrawShaded(fb, cam, goldenSun)
		return fb
	}},
	{"pixels-cube", func(t *testing.T) *engine.FrameBuffer {
		t.Setenv("TERM_PROGRAM", "") // Canvas.Seamless depends on the terminal running the tests
		fb := goldenBuffer(t)
		cube := Cube(2)
		cube.Transform.Position = geometry.Vector3{Z: 5}
		cube.Transform.Yaw, cube.Transform.Pitch = 0.6, -0.4
		canvas := fb.Canvas()
		cube.withColor(engine.Palette(208)).DrawShadedPixels(canvas, shadeCam(), goldenSun)
		canvas.Compose()
		return fb
	}},
	{"near-plane-crossing", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		cube := Cube(2)
		cube.Transform.Position = geometry.Vector3{X: 2, Z: 1} // its side face runs from the camera's plane to beyond the near plane
		cube.Transform.Yaw = 0.1
		cube.withColor(engine.Palette(46)).DrawShaded(fb, Camera{FOV: 1.4, Near: 0.5}, goldenSun)
		return fb
	}},
	{"double-sided-quad", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		quad := Mesh{
			Vertices:    []geometry.Vector3{{X: -2, Y: -1}, {X: 2, Y: -1}, {X: 2, Y: 1}, {X: -2, Y: 1}},
			Faces:       [][3]int{{0, 1, 2}, {0, 2, 3}},
			DoubleSided: true,
		}
		quad.Transform.Position = geometry.Vector3{Z: 5}
		quad.Transform.Yaw = 2.6 // seen from behind
		quad.withColor(engine.Palette(135)).DrawShaded(fb, shadeCam(), goldenSun)
		return fb
	}},
	{"wide-cells", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		cube := Cube(2)
		cube.Transform.Position = geometry.Vector3{Z: 6}
		cube.Transform.Yaw = 0.5
		cube.Draw(fb, Camera{CellAspect: 3}, engine.Cell{Character: '#'})
		return fb
	}},
	{"ambient-only", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		cube := Cube(2)
		cube.Transform.Position = geometry.Vector3{Z: 5}
		cube.Transform.Yaw = 0.6
		cube.withColor(engine.Palette(208)).DrawShaded(fb, shadeCam(), goldenAmbientOnly)
		return fb
	}},
	{"smooth-sphere", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		ball := Sphere(2, 24, 12)
		ball.Transform.Position = geometry.Vector3{Z: 5}
		ball.withColor(engine.Palette(33)).DrawShaded(fb, shadeCam(), goldenSun)
		return fb
	}},
	{"faceted-sphere", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		ball := Sphere(2, 24, 12)
		ball.Normals = nil // the same sphere, lit face by face
		ball.Transform.Position = geometry.Vector3{Z: 5}
		ball.withColor(engine.Palette(33)).DrawShaded(fb, shadeCam(), goldenSun)
		return fb
	}},
	{"smooth-sphere-pixels", func(t *testing.T) *engine.FrameBuffer {
		t.Setenv("TERM_PROGRAM", "") // Canvas.Seamless depends on the terminal running the tests
		fb := goldenBuffer(t)
		ball := Sphere(2, 24, 12)
		ball.Transform.Position = geometry.Vector3{Z: 5}
		canvas := fb.Canvas()
		ball.withColor(engine.Palette(33)).DrawShadedPixels(canvas, shadeCam(), goldenSun)
		canvas.Compose()
		return fb
	}},
	{"smooth-sphere-cut-by-near-plane", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		ball := Sphere(2, 24, 12)
		ball.Transform.Position = geometry.Vector3{X: 0.5, Z: 3.2} // its near side reaches in front of the near plane
		ball.withColor(engine.Palette(46)).DrawShaded(fb, Camera{FOV: 1.4, Near: 1.6}, goldenSun)
		return fb
	}},
	{"depth-wire-behind-solid", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		solid := Cube(2)
		solid.Transform = Transform{Position: geometry.Vector3{Z: 5}, Yaw: 0.6, Pitch: -0.4}
		wire := Cube(2)
		wire.Transform = Transform{Position: geometry.Vector3{X: 1.6, Y: 0.4, Z: 7}, Yaw: 0.3}
		// The wireframe is drawn after the solid it is behind: Draw would show it through.
		solid.withColor(engine.Palette(208)).DrawShaded(fb, shadeCam(), goldenSun)
		wire.DrawDepth(fb, shadeCam(), engine.Cell{Character: '#', Foreground: engine.Palette(46)})
		return fb
	}},
	{"depth-wire-in-front-of-solid", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		solid := Cube(2)
		solid.Transform = Transform{Position: geometry.Vector3{Z: 5}, Yaw: 0.6, Pitch: -0.4}
		wire := Cube(1)
		wire.Transform = Transform{Position: geometry.Vector3{X: 0.3, Z: 2.8}, Yaw: 0.6, Pitch: -0.3}
		// The wireframe is drawn before the solid behind it: Draw would cover it.
		wire.DrawDepth(fb, shadeCam(), engine.Cell{Character: '#', Foreground: engine.Palette(46)})
		solid.withColor(engine.Palette(208)).DrawShaded(fb, shadeCam(), goldenSun)
		return fb
	}},
	{"colored-cube", func(t *testing.T) *engine.FrameBuffer {
		fb := goldenBuffer(t)
		cube := coloredCube()
		cube.DrawShaded(fb, shadeCam(), goldenSun)
		return fb
	}},
	{"colored-cube-pixels", func(t *testing.T) *engine.FrameBuffer {
		t.Setenv("TERM_PROGRAM", "") // Canvas.Seamless depends on the terminal running the tests
		fb := goldenBuffer(t)
		canvas := fb.Canvas()
		coloredCube().DrawShadedPixels(canvas, shadeCam(), goldenSun)
		canvas.Compose()
		return fb
	}},
}

func TestGoldenPictures(t *testing.T) {
	for _, tt := range goldenScenes {
		t.Run(tt.name, func(t *testing.T) {
			got := picture(tt.draw(t))
			path := filepath.Join("testdata", "golden", tt.name+".txt")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("no picture to compare with (run with -update to make it): %v", err)
			}
			if got != string(want) {
				t.Errorf("wanted the picture in %s\n%s\ngot\n%s", path, want, got)
			}
		})
	}
}

// What the scenes are lit by. Written as the API of the day expects.
var (
	goldenSun         = sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)
	goldenAmbientOnly = sunAndFill(geometry.Vector3{}, 0.5)
)

// coloredCube is the turned cube of the shaded pictures with a color for each side, the
// -y side left to the mesh's own color.
func coloredCube() Mesh {
	cube := Cube(2)
	cube.Transform = Transform{Position: geometry.Vector3{Z: 5}, Yaw: 0.6, Pitch: -0.4}
	cube.Material.Color = engine.Palette(208)
	sides := []engine.Color{engine.RGB(230, 60, 50), engine.RGB(50, 200, 90), {}, engine.RGB(60, 110, 230), engine.RGB(240, 200, 40), engine.RGB(170, 70, 200)}
	for _, side := range sides { // two triangles for each side, in the order Cube lists them
		cube.FaceColors = append(cube.FaceColors, side, side)
	}
	return cube
}

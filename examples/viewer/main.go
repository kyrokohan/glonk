// Command viewer shows OBJ models in the terminal.
//
//	go run ./examples/viewer                 browse the bundled samples and shapes
//	go run ./examples/viewer model.obj ...   view your own models
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/mesh"
)

//go:embed models/*.obj models/*.mtl
var samples embed.FS

// modelSize is the length of a model's longest side after Fit.
const modelSize = 2

// palette is the colors C cycles through.
var palette = []engine.Color{
	engine.Palette(208), engine.Palette(39), engine.Palette(118),
	engine.Palette(201), engine.Palette(226), engine.Palette(255),
}

// sun lights the model: some ambient fill, and a sun from above, to one side and
// slightly ahead.
var sun = []mesh.Light{
	mesh.Ambient{Level: 0.2},
	mesh.Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}},
}

// How far up and to the left of the camera the lamp is held, and how far it reaches in
// units of how far back the camera is. Far enough aside that the lit side of a round
// shape is clearly its upper left, and a reach long enough that the lamp is not dim.
const (
	lampAside = 2.0
	lampUp    = 2.0
	lampReach = 8.0
)

// lampLights is the ambient fill and a lamp held by the viewer, up and to the left of the
// camera, so that it moves with it. dist is how far back the camera is.
func lampLights(cam mesh.Camera, dist float64) []mesh.Light {
	position := cam.Position.Add(cam.Right().Scale(-lampAside)).Add(geometry.Vector3{Y: lampUp})
	return []mesh.Light{
		mesh.Ambient{Level: 0.2},
		mesh.Point{Position: position, Range: lampReach * dist},
	}
}

type model struct {
	name  string
	mesh  mesh.Mesh // fitted to modelSize; it has Normals if the file or shape came with them
	verts int
	faces int

	// colored is whether the file gave its faces colors of their own (see
	// mesh.LoadOBJFS), which C can show or put aside for the palette.
	colored bool

	// smooth is whether the model is shaded smooth, which F switches. A model that came
	// with normals starts smooth. One that did not starts faceted, as it always has,
	// and smoothing it finds its normals from its faces (SmoothNormals), which is done
	// the first time and kept in smoothed.
	smooth   bool
	smoothed *mesh.Mesh
}

// maxModelBytes bounds the OBJ files the viewer reads. A model larger than this is
// more likely a mistake than something to look at in a terminal, and loading one
// takes up to 35 times as much memory (about 4.5 GB at the limit). 128 MiB lets
// through a model of 800,000 triangles (81 MB), which draws at about 20 frames a
// second. It is a variable so tests can lower it.
var maxModelBytes int64 = 128 << 20

// loadModel reads one OBJ file from fsys, up to maxModelBytes, with the colors of the
// materials in the MTL files beside it, and fits it to modelSize.
func loadModel(fsys fs.FS, name string) (model, error) {
	m, err := mesh.LoadOBJFS(fsys, name, maxModelBytes)
	if err != nil {
		return model{}, err
	}
	return model{
		name: path.Base(name), mesh: m.Fit(modelSize), verts: len(m.Vertices), faces: len(m.Faces),
		colored: len(m.FaceColors) > 0 || len(m.VertexColors) > 0, smooth: len(m.Normals) > 0,
	}, nil
}

// shapeModels are the shapes the mesh package makes, to browse beside the OBJ samples.
// They come with normals, so they start smooth.
func shapeModels() []model {
	var models []model
	for _, s := range []struct {
		name string
		mesh mesh.Mesh
	}{
		{"sphere", mesh.Sphere(modelSize/2, 24, 12)},
		{"cylinder", mesh.Cylinder(0.8, modelSize, 24)},
		{"cone", mesh.Cone(0.9, modelSize, 24)},
	} {
		// Built at modelSize and centered, so there is nothing to fit.
		models = append(models, model{name: s.name, mesh: s.mesh, verts: len(s.mesh.Vertices), faces: len(s.mesh.Faces), smooth: true})
	}
	return models
}

// browseModels is what the viewer shows: the models named in args, or with none the
// bundled samples and the shapes.
func browseModels(args []string) ([]model, error) {
	models, err := loadModels(args)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		models = append(models, shapeModels()...)
	}
	return models, nil
}

// loadModels loads the files named in args, or the bundled samples if there are none.
func loadModels(args []string) ([]model, error) {
	var models []model

	if len(args) == 0 {
		entries, err := fs.ReadDir(samples, "models")
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if path.Ext(e.Name()) != ".obj" { // the .mtl files beside them are not models
				continue
			}
			m, err := loadModel(samples, path.Join("models", e.Name()))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", e.Name(), err)
			}
			models = append(models, m)
		}
		return models, nil
	}

	for _, arg := range args {
		abs, err := filepath.Abs(arg)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", arg, err)
		}
		m, err := loadModel(os.DirFS(filepath.Dir(abs)), filepath.Base(abs))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", arg, err)
		}
		models = append(models, m)
	}
	return models, nil
}

type viewer struct {
	models  []model
	current int

	// The camera orbits the origin, where the model sits: yaw turns it around,
	// pitch raises it (positive pitch looks up, so it is negative to look down on
	// the model), and dist is how far back it stays.
	yaw, pitch, dist float64

	spin        bool
	wire        bool
	halfBlocks  bool
	doubleSided bool
	lamp        bool // lit by a lamp that follows the camera, not the sun
	color       int  // the palette color, for a model without colors of its own and for the wireframe
	ownColors   bool // a model that has colors of its own shows them, not the palette color
	fps         float64

	// flipEdges is set when B is pressed. The next half-block frame then switches the
	// pixel edges between exact and solid. The engine keeps that setting from frame to
	// frame and across resizes. cube3d has the same few lines.
	flipEdges bool
}

func newViewer(models []model) *viewer {
	return &viewer{models: models, pitch: -0.5, dist: 5, spin: true, ownColors: true, fps: 30}
}

func (v *viewer) Update(dt float64, in *engine.Input) error {
	if in.Pressed(engine.KeyEscape) {
		return engine.ErrQuit
	}

	// Shift changes the letter in terminals that cannot report keys exactly.
	pressed := func(lower, upper rune) bool { return in.Pressed(engine.Key(lower)) || in.Pressed(engine.Key(upper)) }
	if pressed('n', 'N') {
		v.current = (v.current + 1) % len(v.models)
	}
	if pressed('r', 'R') {
		v.spin = !v.spin
	}
	if pressed('v', 'V') {
		v.wire = !v.wire
	}
	if pressed('m', 'M') {
		v.halfBlocks = !v.halfBlocks
	}
	if pressed('d', 'D') {
		v.doubleSided = !v.doubleSided
	}
	if pressed('c', 'C') {
		v.nextColor()
	}
	if pressed('f', 'F') {
		v.models[v.current].smooth = !v.models[v.current].smooth
	}
	if pressed('l', 'L') {
		v.lamp = !v.lamp
	}
	if v.halfBlocks && pressed('b', 'B') {
		v.flipEdges = true
	}

	const (
		turnSpeed = 1.5
		spinSpeed = 0.6
		zoomSpeed = 3.0
		maxPitch  = 1.4
	)

	if in.IsDown(engine.KeyLeft) {
		v.yaw -= turnSpeed * dt
	}
	if in.IsDown(engine.KeyRight) {
		v.yaw += turnSpeed * dt
	}
	// Up lifts the camera over the model, which means looking down: negative pitch.
	if in.IsDown(engine.KeyUp) {
		v.pitch -= turnSpeed * dt
	}
	if in.IsDown(engine.KeyDown) {
		v.pitch += turnSpeed * dt
	}
	v.pitch = max(-maxPitch, min(maxPitch, v.pitch))
	if v.spin {
		v.yaw += spinSpeed * dt
	}

	if in.IsDown('+') || in.IsDown('=') || in.IsDown('w') || in.IsDown('W') {
		v.dist -= zoomSpeed * dt
	}
	if in.IsDown('-') || in.IsDown('_') || in.IsDown('s') || in.IsDown('S') {
		v.dist += zoomSpeed * dt
	}
	v.dist = max(2.2, min(14, v.dist))

	v.fps = 0.9*v.fps + 0.1*(1/dt)
	return nil
}

// nextColor is C: the next palette color. A model with colors of its own goes from them
// to the first palette color and, after the last, back to them.
func (v *viewer) nextColor() {
	switch {
	case !v.models[v.current].colored:
		v.color = (v.color + 1) % len(palette)
	case v.ownColors:
		v.ownColors, v.color = false, 0
	case v.color == len(palette)-1:
		v.ownColors, v.color = true, 0
	default:
		v.color++
	}
}

// showsModelColors is whether the current model is drawn in the colors its file gave it.
func (v *viewer) showsModelColors() bool {
	return v.ownColors && v.models[v.current].colored
}

// camera returns the camera for the current orbit: it looks at the origin from dist away.
func (v *viewer) camera() mesh.Camera {
	cam := mesh.Camera{Yaw: v.yaw, Pitch: v.pitch, FOV: 1.2, Near: 0.1}
	cam.Position = cam.Look().Scale(-v.dist)
	return cam
}

// shown returns the current model as it is to be drawn: with normals if it is smooth,
// without if it is faceted.
func (v *viewer) shown() mesh.Mesh {
	cur := &v.models[v.current]
	if cur.smooth {
		if len(cur.mesh.Normals) > 0 {
			return cur.mesh
		}
		if cur.smoothed == nil {
			smoothed := cur.mesh.SmoothNormals()
			cur.smoothed = &smoothed
		}
		return *cur.smoothed
	}
	faceted := cur.mesh
	faceted.Normals = nil
	return faceted
}

// lights returns the lights for a camera: the sun, or the lamp that follows it.
func (v *viewer) lights(cam mesh.Camera) []mesh.Light {
	if v.lamp {
		return lampLights(cam, v.dist)
	}
	return sun
}

func (v *viewer) Draw(fb *engine.FrameBuffer) {
	cam := v.camera()
	lights := v.lights(cam)
	m := v.shown()
	m.DoubleSided = v.doubleSided
	color := palette[v.color]
	// The palette color also covers any face the file gave no color.
	m.Material.Color = color
	if !v.showsModelColors() {
		m.FaceColors, m.VertexColors = nil, nil
	}

	mode := "characters"
	switch {
	case v.halfBlocks:
		canvas := fb.Canvas()
		if v.flipEdges {
			canvas.Seamless = !canvas.Seamless
			v.flipEdges = false
		}
		mode = "half-blocks, exact edges"
		if canvas.Seamless {
			mode = "half-blocks, solid edges"
		}
		if v.wire {
			// Lines of pixels, as fine as the solids, in the palette color.
			mode = "wireframe, " + mode
			m.DrawPixels(canvas, cam, color)
		} else {
			m.DrawShadedPixels(canvas, cam, lights...)
		}
		canvas.Compose()
	case v.wire:
		mode = "wireframe"
		m.Draw(fb, cam, engine.Cell{Character: '#', Foreground: color})
	default:
		m.DrawShaded(fb, cam, lights...)
	}

	if v.doubleSided {
		mode += ", two-sided"
	}
	if !v.wire { // a wireframe has no shading and no light
		shading, light := "faceted", "sun"
		if v.models[v.current].smooth {
			shading = "smooth"
		}
		if v.lamp {
			light = "lamp"
		}
		mode += ", " + shading + ", " + light
		if v.showsModelColors() {
			mode += ", model colors"
		}
	}

	cur := v.models[v.current]
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.DrawText(1, 1, fmt.Sprintf("%s (%d/%d)  %d vertices, %d faces", cur.name, v.current+1, len(v.models), cur.verts, cur.faces), engine.Color{})
	fb.DrawText(1, 2, fmt.Sprintf("%.0f fps  %s", v.fps, mode), engine.Color{})
	fb.DrawText(1, fb.Height()-3, "arrows orbit  +/- zoom  R spin  N next model  Esc quit", engine.Color{})
	fb.DrawText(1, fb.Height()-2, "V wire/solid  M blocks  B edges  D two-sided  C color  F smooth  L lamp", engine.Color{})
}

func main() {
	models, err := browseModels(os.Args[1:])
	if err != nil {
		// Report before the terminal is touched, so the message stays readable.
		fmt.Fprintln(os.Stderr, "viewer:", err)
		os.Exit(1)
	}

	if err := engine.Run(context.Background(), newViewer(models), engine.Config{FPS: 30}); err != nil {
		fmt.Fprintln(os.Stderr, "viewer:", err)
		os.Exit(1)
	}
}

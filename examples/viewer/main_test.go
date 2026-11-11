package main

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/mesh"
)

func loadSamples(t *testing.T) []model {
	t.Helper()
	models, err := loadModels(nil)
	if err != nil {
		t.Fatalf("unable to load the samples: %v", err)
	}
	return models
}

func TestSamplesLoad(t *testing.T) {
	models := loadSamples(t)

	want := []struct {
		name  string
		verts int
		faces int // triangles, after splitting polygons
	}{
		{"house.obj", 30, 44},
		{"icosphere.obj", 42, 80},
		{"octahedron.obj", 6, 8},
		{"torus.obj", 128, 256},
	}
	if len(models) != len(want) {
		t.Fatalf("wanted %d samples, got %d", len(want), len(models))
	}
	for i, w := range want {
		if models[i].name != w.name || models[i].verts != w.verts || models[i].faces != w.faces {
			t.Errorf("sample %d: wanted %s with %d vertices and %d triangles, got %s with %d and %d",
				i, w.name, w.verts, w.faces, models[i].name, models[i].verts, models[i].faces)
		}
	}
}

// Every sample must be a closed solid wound outward everywhere: positive volume, and
// each directed edge used once with its reverse used once by the neighbouring face.
func TestSamplesAreClosedAndWoundOutward(t *testing.T) {
	for _, m := range loadSamples(t) {
		t.Run(m.name, func(t *testing.T) {
			var volume float64
			directed := map[[2]int]int{}
			for _, f := range m.mesh.Faces {
				a, b, c := m.mesh.Vertices[f[0]], m.mesh.Vertices[f[1]], m.mesh.Vertices[f[2]]
				volume += a.Dot(b.Cross(c)) / 6
				for k := range 3 {
					directed[[2]int{f[k], f[(k+1)%3]}]++
				}
			}

			if volume <= 0 {
				t.Errorf("wanted a positive volume (outward winding), got %v", volume)
			}
			for e, n := range directed {
				if n != 1 || directed[[2]int{e[1], e[0]}] != 1 {
					t.Fatalf("wanted every edge shared by two faces wound opposite ways, edge %v used %d times, reverse %d", e, n, directed[[2]int{e[1], e[0]}])
				}
			}
		})
	}
}

func TestSamplesAreFitted(t *testing.T) {
	for _, m := range loadSamples(t) {
		lo, hi := m.mesh.Bounds()
		size, center := hi.Sub(lo), lo.Add(hi).Scale(0.5)
		longest := math.Max(size.X, math.Max(size.Y, size.Z))

		if math.Abs(longest-modelSize) > 1e-9 {
			t.Errorf("%s: wanted the longest side %v, got %v", m.name, float64(modelSize), longest)
		}
		if center.Length() > 1e-9 {
			t.Errorf("%s: wanted the model centered, got center %v", m.name, center)
		}
	}
}

func writeFile(t *testing.T, name, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatalf("unable to write %s: %v", p, err)
	}
	return p
}

func TestLoadModelsFromFiles(t *testing.T) {
	tri := writeFile(t, "tri.obj", "v 0 0 0\nv 4 0 0\nv 0 2 0\nf 1 2 3\n")
	quad := writeFile(t, "quad.obj", "v 0 0 0\nv 1 0 0\nv 1 1 0\nv 0 1 0\nf 1 2 3 4\n")

	models, err := loadModels([]string{tri, quad})
	if err != nil {
		t.Fatalf("unable to load: %v", err)
	}
	if len(models) != 2 || models[0].name != "tri.obj" || models[1].name != "quad.obj" {
		t.Fatalf("wanted tri.obj then quad.obj, got %+v", models)
	}
	if models[0].verts != 3 || models[0].faces != 1 || models[1].faces != 2 {
		t.Errorf("wanted counts 3/1 and 4/2, got %d/%d and %d/%d", models[0].verts, models[0].faces, models[1].verts, models[1].faces)
	}
	if _, size := extents(models[0]); math.Abs(size-modelSize) > 1e-9 {
		t.Errorf("wanted the file fitted to %v, got %v", float64(modelSize), size)
	}
}

func extents(m model) (center geometry.Vector3, longest float64) {
	lo, hi := m.mesh.Bounds()
	size := hi.Sub(lo)
	return lo.Add(hi).Scale(0.5), math.Max(size.X, math.Max(size.Y, size.Z))
}

func TestLoadModelsRelativePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.obj"), []byte("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	models, err := loadModels([]string{"m.obj"})
	if err != nil || len(models) != 1 {
		t.Fatalf("wanted a model from a relative path, got %v, %v", models, err)
	}
}

func TestLoadModelsErrorsNameTheFile(t *testing.T) {
	good := writeFile(t, "good.obj", "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")
	bad := writeFile(t, "bad.obj", "v 0 0 0\nv 1 oops 0\n")
	empty := writeFile(t, "empty.obj", "# nothing\n")
	missing := filepath.Join(t.TempDir(), "missing.obj")

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"missing file", []string{missing}, []string{"missing.obj"}},
		{"malformed file names the line", []string{bad}, []string{"bad.obj", "obj line 2", "oops"}},
		{"file with no geometry", []string{empty}, []string{"empty.obj", "no faces or lines"}},
		{"one bad file among good ones", []string{good, bad}, []string{"bad.obj", "obj line 2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			models, err := loadModels(tt.args)
			if err == nil {
				t.Fatalf("wanted an error, got %d models", len(models))
			}
			for _, frag := range tt.want {
				if !strings.Contains(err.Error(), frag) {
					t.Errorf("wanted the message to contain %q, got %q", frag, err)
				}
			}
		})
	}
}

func drawAll(t *testing.T, v *viewer) *engine.FrameBuffer {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "") // the pixel edge style default depends on the terminal
	fb, err := engine.NewFrameBuffer(80, 24)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	v.Draw(fb)
	return fb
}

// modelCells counts the cells inside the frame border that are not text on the HUD rows.
func modelCells(fb *engine.FrameBuffer) [][2]int {
	var cells [][2]int
	for y := 3; y < fb.Height()-4; y++ {
		for x := 1; x < fb.Width()-1; x++ {
			if fb.At(x, y) != engine.BlankCell {
				cells = append(cells, [2]int{x, y})
			}
		}
	}
	return cells
}

func TestEveryModelDrawsInEveryMode(t *testing.T) {
	modes := []struct {
		name  string
		apply func(v *viewer)
	}{
		{"characters", func(v *viewer) {}},
		{"characters two-sided", func(v *viewer) { v.doubleSided = true }},
		{"wireframe", func(v *viewer) { v.wire = true }},
		{"half-blocks", func(v *viewer) { v.halfBlocks = true }},
		{"half-blocks solid edges", func(v *viewer) { v.halfBlocks, v.flipEdges = true, true }},
		{"half-blocks two-sided", func(v *viewer) { v.halfBlocks, v.doubleSided = true, true }},
		{"wireframe half-blocks", func(v *viewer) { v.wire, v.halfBlocks = true, true }},
		{"wireframe half-blocks solid edges", func(v *viewer) { v.wire, v.halfBlocks, v.flipEdges = true, true, true }},
	}

	models := loadSamples(t)
	for i := range models {
		for _, mode := range modes {
			t.Run(models[i].name+" "+mode.name, func(t *testing.T) {
				v := newViewer(models)
				v.current = i
				mode.apply(v)

				fb := drawAll(t, v)

				if cells := modelCells(fb); len(cells) < 40 {
					t.Errorf("wanted the model on screen, got %d cells", len(cells))
				}
			})
		}
	}
}

func hudText(fb *engine.FrameBuffer, row int) string {
	var sb strings.Builder
	for x := 1; x < fb.Width()-1; x++ {
		sb.WriteRune(fb.At(x, row).Character)
	}
	return sb.String()
}

// modelIndex is where the model called name is in models.
func modelIndex(t *testing.T, models []model, name string) int {
	t.Helper()
	for i, m := range models {
		if m.name == name {
			return i
		}
	}
	t.Fatalf("wanted a model called %s", name)
	return 0
}

func TestHUDNamesTheModelAndMode(t *testing.T) {
	models := loadSamples(t)
	v := newViewer(models)
	v.current = 1

	fb := drawAll(t, v)
	if got := hudText(fb, 1); !strings.Contains(got, "icosphere.obj (2/4)") || !strings.Contains(got, "42 vertices, 80 faces") {
		t.Errorf("wanted the model and its counts on the first line, got %q", got)
	}
	if got := hudText(fb, 2); !strings.Contains(got, "characters") {
		t.Errorf("wanted the mode on the second line, got %q", got)
	}

	v.wire, v.doubleSided = true, true
	fb = drawAll(t, v)
	if got := hudText(fb, 2); !strings.Contains(got, "wireframe, two-sided") {
		t.Errorf("wanted wireframe, two-sided, got %q", got)
	}
}

// The house is lopsided on purpose: the chimney stands on its +X side. Seen from the
// front it must be on the right, and from behind on the left. A mirrored import would
// swap them.
func TestHouseIsNotMirrored(t *testing.T) {
	models := loadSamples(t)
	house := 0
	for i, m := range models {
		if m.name == "house.obj" {
			house = i
		}
	}

	chimneyColumn := func(yaw float64) float64 {
		v := newViewer(models)
		v.current = house
		v.pitch = 0
		v.yaw = yaw
		fb := drawAll(t, v)

		// The chimney is the highest thing, so the topmost row with anything in it.
		cells := modelCells(fb)
		top := cells[0][1]
		for _, c := range cells {
			top = min(top, c[1])
		}
		var sum, n float64
		for _, c := range cells {
			if c[1] == top {
				sum += float64(c[0])
				n++
			}
		}
		return sum / n
	}

	mid := 40.0
	if got := chimneyColumn(0); got <= mid {
		t.Errorf("wanted the chimney right of center from the front, got column %.1f", got)
	}
	if got := chimneyColumn(math.Pi); got >= mid {
		t.Errorf("wanted the chimney left of center from behind, got column %.1f", got)
	}
}

func TestViewerStartsWithAUsefulView(t *testing.T) {
	v := newViewer(loadSamples(t))
	cam := v.camera()

	// The camera is in front of the model (negative Z), looking at the origin.
	if cam.Position.Z >= 0 {
		t.Errorf("wanted the camera in front of the model, got %v", cam.Position)
	}
	toOrigin := cam.Position.Neg().Normalize()
	if got := cam.Look().Dot(toOrigin); math.Abs(got-1) > 1e-9 {
		t.Errorf("wanted the camera to look at the origin, got alignment %v", got)
	}
	if math.Abs(cam.Position.Length()-v.dist) > 1e-9 {
		t.Errorf("wanted the camera %v away, got %v", v.dist, cam.Position.Length())
	}
}

func TestColorAndDrawStyleReachTheScreen(t *testing.T) {
	models := loadSamples(t)

	characters := func(fb *engine.FrameBuffer) map[rune]bool {
		seen := map[rune]bool{}
		for _, c := range modelCells(fb) {
			seen[fb.At(c[0], c[1]).Character] = true
		}
		return seen
	}

	for i, want := range palette {
		v := newViewer(models)
		v.color, v.ownColors = i, false // the house has colors of its own, which would show instead

		fb := drawAll(t, v)
		for _, c := range modelCells(fb) {
			if got := fb.At(c[0], c[1]).Foreground; got != want {
				t.Fatalf("color %d: wanted %v, got %v at %v", i, want, got, c)
			}
		}
	}

	solid := newViewer(models)
	if got := characters(drawAll(t, solid)); len(got) < 2 {
		t.Errorf("wanted shaded drawing to use several characters, got %v", got)
	}

	wire := newViewer(models)
	wire.wire = true
	if got := characters(drawAll(t, wire)); len(got) != 1 || !got['#'] {
		t.Errorf("wanted a wireframe made only of #, got %v", got)
	}
}

// A triangle wound away from the camera is invisible until two-sided drawing is on.
func TestTwoSidedShowsABackwardsModel(t *testing.T) {
	p := writeFile(t, "backwards.obj", "v 0 0 0\nv 0 1 0\nv 1 0 0\nf 1 2 3\n")
	models, err := loadModels([]string{p})
	if err != nil {
		t.Fatalf("unable to load: %v", err)
	}

	for _, halfBlocks := range []bool{false, true} {
		v := newViewer(models)
		v.pitch, v.yaw, v.halfBlocks = 0, 0, halfBlocks
		if cells := modelCells(drawAll(t, v)); len(cells) != 0 {
			t.Errorf("halfBlocks=%v: wanted the backwards triangle culled, got %d cells", halfBlocks, len(cells))
		}

		v.doubleSided = true
		if cells := modelCells(drawAll(t, v)); len(cells) < 10 {
			t.Errorf("halfBlocks=%v: wanted the backwards triangle shown two-sided, got %d cells", halfBlocks, len(cells))
		}
	}
}

func TestColorReachesHalfBlockPixels(t *testing.T) {
	models := loadSamples(t)

	shades := func(color int) map[engine.Color]bool {
		v := newViewer(models)
		v.halfBlocks, v.color, v.ownColors = true, color, false
		fb := drawAll(t, v)

		seen := map[engine.Color]bool{}
		for _, c := range modelCells(fb) {
			cell := fb.At(c[0], c[1])
			seen[cell.Foreground] = true
			seen[cell.Background] = true
		}
		return seen
	}

	first, second := shades(0), shades(1)
	same := len(first) == len(second)
	for c := range first {
		same = same && second[c]
	}
	if same {
		t.Errorf("wanted different palette colors to give different pixels, both gave %v", first)
	}
}

func TestOversizedModelIsRefused(t *testing.T) {
	text := "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n" + strings.Repeat("# padding\n", 200)
	big := writeFile(t, "big.obj", text)
	size := int64(len(text))

	defer func(old int64) { maxModelBytes = old }(maxModelBytes)

	maxModelBytes = size
	if _, err := loadModels([]string{big}); err != nil {
		t.Errorf("wanted a file exactly at the limit to load, got %v", err)
	}

	maxModelBytes = size - 1
	_, err := loadModels([]string{big})
	if !errors.Is(err, mesh.ErrOBJTooLarge) {
		t.Fatalf("wanted ErrOBJTooLarge one byte under, got %v", err)
	}
	for _, frag := range []string{"big.obj", "more than", fmt.Sprint(size - 1)} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("wanted the message to contain %q, got %q", frag, err)
		}
	}
}

func TestDefaultLimitSuitsTheSamplesAndIsGenerous(t *testing.T) {
	if maxModelBytes != 128<<20 {
		t.Errorf("wanted the default limit to be 128 MiB, got %d", maxModelBytes)
	}
	loadSamples(t) // the bundled models are tiny and must load under it
}

// B flips the pixel edges once per press, and the setting then stays across frames.
func TestEdgeKeyFlipsTheEdgeStyleOncePerPress(t *testing.T) {
	for _, wire := range []bool{false, true} {
		t.Run(fmt.Sprintf("wireframe %v", wire), func(t *testing.T) { edgeKeyFlipsTheEdgeStyleOncePerPress(t, wire) })
	}
}

func edgeKeyFlipsTheEdgeStyleOncePerPress(t *testing.T, wire bool) {
	t.Setenv("TERM_PROGRAM", "") // the default style depends on the terminal
	v := newViewer(loadSamples(t))
	v.halfBlocks, v.wire = true, wire
	kb := engine.NewKeyboard()

	fb, err := engine.NewFrameBuffer(80, 24)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	frame := func(pressB bool) string {
		if pressB {
			kb.Press('b')
		}
		if err := step(v, kb, 1.0/30); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		kb.Release('b')
		fb.Clear()
		v.Draw(fb)
		return hudText(fb, 2)
	}

	if got := frame(false); !strings.Contains(got, "exact edges") {
		t.Fatalf("wanted exact edges to start with, got %q", got)
	}
	if got := frame(true); !strings.Contains(got, "solid edges") {
		t.Errorf("wanted solid edges after a press, got %q", got)
	}
	if v.flipEdges {
		t.Errorf("wanted the press used up by the frame that applied it")
	}
	if got := frame(false); !strings.Contains(got, "solid edges") {
		t.Errorf("wanted solid edges kept on the next frame, got %q", got)
	}
	if got := frame(true); !strings.Contains(got, "exact edges") {
		t.Errorf("wanted a second press to switch back, got %q", got)
	}
}

// step runs one Update of dt seconds with the keyboard as it is, then ends the frame
// as Run does.
func step(v *viewer, kb *engine.Keyboard, dt float64) error {
	err := v.Update(dt, kb.Input())
	kb.EndFrame()
	return err
}

// quiet is a viewer that does not spin, so that only the keys move it.
func quiet(t *testing.T) *viewer {
	t.Helper()
	v := newViewer(loadSamples(t))
	v.spin = false
	return v
}

func TestEscapeQuitsTheViewer(t *testing.T) {
	v := quiet(t)
	kb := engine.NewKeyboard()

	if err := step(v, kb, 0.1); err != nil {
		t.Fatalf("wanted no error without Escape, got %v", err)
	}
	kb.Press(engine.KeyEscape)
	if err := step(v, kb, 0.1); !errors.Is(err, engine.ErrQuit) {
		t.Errorf("wanted ErrQuit, got %v", err)
	}
}

func TestToggleKeysSwitchOncePerPressInEitherCase(t *testing.T) {
	tests := []struct {
		lower, upper engine.Key
		get          func(*viewer) bool
		startsOn     bool
	}{
		{'r', 'R', func(v *viewer) bool { return v.spin }, false},
		{'v', 'V', func(v *viewer) bool { return v.wire }, false},
		{'m', 'M', func(v *viewer) bool { return v.halfBlocks }, false},
		{'d', 'D', func(v *viewer) bool { return v.doubleSided }, false},
	}
	for _, tt := range tests {
		for _, key := range []engine.Key{tt.lower, tt.upper} {
			t.Run(fmt.Sprintf("%c", key), func(t *testing.T) {
				v := quiet(t)
				start := tt.get(v)
				kb := engine.NewKeyboard()

				kb.Press(key)
				step(v, kb, 0.1)
				if tt.get(v) == start {
					t.Fatalf("wanted the first press to switch it")
				}
				for range 5 { // held down, no new press
					step(v, kb, 0.1)
				}
				if tt.get(v) == start {
					t.Errorf("wanted holding the key to change nothing")
				}
				kb.Release(key)
				kb.Press(key)
				step(v, kb, 0.1)
				if tt.get(v) != start {
					t.Errorf("wanted a second press to switch it back")
				}
			})
		}
	}
}

func TestNextModelCyclesAndWraps(t *testing.T) {
	v := quiet(t)
	n := len(v.models)
	if n < 2 {
		t.Fatalf("wanted at least two samples to cycle through, got %d", n)
	}
	kb := engine.NewKeyboard()

	for i := 1; i <= n; i++ {
		kb.Press('n')
		step(v, kb, 0.1)
		kb.Release('n')
		if want := i % n; v.current != want {
			t.Fatalf("after %d presses: wanted model %d, got %d", i, want, v.current)
		}
	}
	kb.Press('N')
	step(v, kb, 0.1)
	if v.current != 1 {
		t.Errorf("wanted a capital N to go on too, got model %d", v.current)
	}
}

func TestColorKeyCyclesThroughThePalette(t *testing.T) {
	v := quiet(t)
	v.current = 1 // the icosphere has no colors of its own
	kb := engine.NewKeyboard()

	for i := 1; i <= len(palette)+1; i++ {
		kb.Press('c')
		step(v, kb, 0.1)
		kb.Release('c')
		if want := i % len(palette); v.color != want {
			t.Fatalf("after %d presses: wanted color %d, got %d", i, want, v.color)
		}
		if !v.ownColors {
			t.Fatalf("after %d presses: wanted the setting for models with colors of their own left alone", i)
		}
	}
}

func TestEdgeKeyOnlyWorksOnHalfBlockPixels(t *testing.T) {
	tests := []struct {
		name             string
		halfBlocks, wire bool
		want             bool
	}{
		{"half blocks", true, false, true},
		{"glyphs", false, false, false},
		{"half blocks and wireframe", true, true, true},
		{"neither", false, true, false},
	}
	for _, tt := range tests {
		for _, key := range []engine.Key{'b', 'B'} {
			t.Run(fmt.Sprintf("%s %c", tt.name, key), func(t *testing.T) {
				v := quiet(t)
				v.halfBlocks, v.wire = tt.halfBlocks, tt.wire
				kb := engine.NewKeyboard()
				kb.Press(key)

				step(v, kb, 0.1)

				if v.flipEdges != tt.want {
					t.Errorf("wanted flipEdges %v, got %v", tt.want, v.flipEdges)
				}
			})
		}
	}
}

func TestArrowsOrbitTheModel(t *testing.T) {
	const dt = 0.1
	tests := []struct {
		name   string
		key    engine.Key
		dYaw   float64
		dPitch float64
	}{
		{"left", engine.KeyLeft, -1.5 * dt, 0},
		{"right", engine.KeyRight, 1.5 * dt, 0},
		{"up looks down on the model", engine.KeyUp, 0, -1.5 * dt},
		{"down looks up at it", engine.KeyDown, 0, 1.5 * dt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := quiet(t)
			yaw, pitch := v.yaw, v.pitch
			kb := engine.NewKeyboard()
			kb.Press(tt.key)

			step(v, kb, dt)

			if math.Abs(v.yaw-yaw-tt.dYaw) > 1e-9 || math.Abs(v.pitch-pitch-tt.dPitch) > 1e-9 {
				t.Errorf("wanted yaw %+v and pitch %+v, got %+v and %+v", tt.dYaw, tt.dPitch, v.yaw-yaw, v.pitch-pitch)
			}
		})
	}
}

func TestOrbitingStopsShortOfThePoles(t *testing.T) {
	for _, tt := range []struct {
		key  engine.Key
		want float64
	}{{engine.KeyUp, -1.4}, {engine.KeyDown, 1.4}} {
		v := quiet(t)
		kb := engine.NewKeyboard()
		kb.Press(tt.key)
		for range 100 {
			step(v, kb, 0.1)
		}
		if v.pitch != tt.want {
			t.Errorf("key %v: wanted the pitch held at %v, got %v", tt.key, tt.want, v.pitch)
		}
	}
}

func TestSpinTurnsTheModelWhenNoKeyDoes(t *testing.T) {
	v := newViewer(loadSamples(t))
	yaw := v.yaw

	step(v, engine.NewKeyboard(), 0.1)

	if got := v.yaw - yaw; math.Abs(got-0.6*0.1) > 1e-9 {
		t.Errorf("wanted a spin of 0.06, got %v", got)
	}
}

func TestZoomKeys(t *testing.T) {
	const dt = 0.1
	for _, key := range []engine.Key{'+', '=', 'w', 'W'} {
		v := quiet(t)
		dist := v.dist
		kb := engine.NewKeyboard()
		kb.Press(key)
		step(v, kb, dt)
		if math.Abs(v.dist-(dist-3*dt)) > 1e-9 {
			t.Errorf("key %q: wanted to zoom in to %v, got %v", key, dist-3*dt, v.dist)
		}
	}
	for _, key := range []engine.Key{'-', '_', 's', 'S'} {
		v := quiet(t)
		dist := v.dist
		kb := engine.NewKeyboard()
		kb.Press(key)
		step(v, kb, dt)
		if math.Abs(v.dist-(dist+3*dt)) > 1e-9 {
			t.Errorf("key %q: wanted to zoom out to %v, got %v", key, dist+3*dt, v.dist)
		}
	}
	v := quiet(t)
	dist := v.dist
	kb := engine.NewKeyboard()
	kb.Press('+', '-')
	step(v, kb, dt)
	if v.dist != dist {
		t.Errorf("wanted zooming in and out together to cancel, got %v from %v", v.dist, dist)
	}
}

func TestZoomStaysWithinItsLimits(t *testing.T) {
	for _, tt := range []struct {
		key  engine.Key
		want float64
	}{{'+', 2.2}, {'-', 14}} {
		v := quiet(t)
		kb := engine.NewKeyboard()
		kb.Press(tt.key)
		for range 200 {
			step(v, kb, 0.1)
		}
		if v.dist != tt.want {
			t.Errorf("key %q: wanted the distance held at %v, got %v", tt.key, tt.want, v.dist)
		}
	}
}

// browseAll is the viewer as it starts with no arguments: the samples and the shapes.
func browseAll(t *testing.T) *viewer {
	t.Helper()
	models, err := browseModels(nil)
	if err != nil {
		t.Fatalf("unable to load the models: %v", err)
	}
	v := newViewer(models)
	v.spin = false
	return v
}

func TestBrowsingShowsTheSamplesThenTheShapes(t *testing.T) {
	models, err := browseModels(nil)
	if err != nil {
		t.Fatal(err)
	}

	want := []struct {
		name         string
		verts, faces int
		smooth       bool
	}{
		{"house.obj", 30, 44, false},
		{"icosphere.obj", 42, 80, false},
		{"octahedron.obj", 6, 8, false},
		{"torus.obj", 128, 256, false},
		{"sphere", 266, 528, true},
		{"cylinder", 98, 96, true},
		{"cone", 73, 48, true},
	}
	if len(models) != len(want) {
		t.Fatalf("wanted %d models, got %d", len(want), len(models))
	}
	for i, w := range want {
		m := models[i]
		if m.name != w.name || m.verts != w.verts || m.faces != w.faces || m.smooth != w.smooth {
			t.Errorf("model %d: wanted %s with %d vertices, %d triangles and smooth %v, got %s with %d, %d and %v",
				i, w.name, w.verts, w.faces, w.smooth, m.name, m.verts, m.faces, m.smooth)
		}
	}
}

// Models named on the command line are shown on their own, without the shapes.
func TestNamedModelsAreShownWithoutTheShapes(t *testing.T) {
	p := writeFile(t, "tri.obj", "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")

	models, err := browseModels([]string{p})
	if err != nil || len(models) != 1 || models[0].name != "tri.obj" {
		t.Errorf("wanted just tri.obj, got %d models (err %v)", len(models), err)
	}
	if _, err := browseModels([]string{filepath.Join(t.TempDir(), "missing.obj")}); err == nil || !strings.Contains(err.Error(), "missing.obj") {
		t.Errorf("wanted an error naming the missing file, got %v", err)
	}
}

func TestShapesAreValidFittedAndHaveNormals(t *testing.T) {
	for _, m := range shapeModels() {
		t.Run(m.name, func(t *testing.T) {
			if err := m.mesh.Validate(); err != nil {
				t.Fatalf("wanted a valid mesh, got %v", err)
			}
			if len(m.mesh.Normals) != len(m.mesh.Vertices) || len(m.mesh.Normals) == 0 {
				t.Errorf("wanted a normal for every vertex, got %d for %d", len(m.mesh.Normals), len(m.mesh.Vertices))
			}
			lo, hi := m.mesh.Bounds()
			size, centre := hi.Sub(lo), lo.Add(hi).Scale(0.5)
			if longest := math.Max(size.X, math.Max(size.Y, size.Z)); math.Abs(longest-modelSize) > 1e-9 || centre.Length() > 1e-9 {
				t.Errorf("wanted the shape fitted to %v and centered, got longest side %v centered at %v", float64(modelSize), longest, centre)
			}
		})
	}
}

// A model that came with normals starts smooth. A file that has them is read as such.
func TestModelsWithNormalsStartSmooth(t *testing.T) {
	flat := writeFile(t, "flat.obj", "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")
	round := writeFile(t, "round.obj", "v 0 0 0\nv 1 0 0\nv 0 1 0\nvn 0 0 1\nf 1//1 2//1 3//1\n")

	models, err := loadModels([]string{flat, round})
	if err != nil {
		t.Fatal(err)
	}

	if models[0].smooth || !models[1].smooth {
		t.Errorf("wanted a file without normals to start faceted and one with them smooth, got %v and %v", models[0].smooth, models[1].smooth)
	}
}

func TestShadingKeyTogglesTheCurrentModelOncePerPressInEitherCase(t *testing.T) {
	for _, key := range []engine.Key{'f', 'F'} {
		t.Run(fmt.Sprintf("%c", key), func(t *testing.T) {
			v := browseAll(t)
			kb := engine.NewKeyboard()

			kb.Press(key)
			step(v, kb, 0.1)
			if !v.models[0].smooth {
				t.Fatalf("wanted the first press to smooth the house")
			}
			for range 5 {
				step(v, kb, 0.1)
			}
			if !v.models[0].smooth {
				t.Errorf("wanted holding the key to change nothing")
			}
			kb.Release(key)
			kb.Press(key)
			step(v, kb, 0.1)
			if v.models[0].smooth {
				t.Errorf("wanted a second press to make it faceted again")
			}
		})
	}
}

// Each model keeps its own shading as you move between them.
func TestShadingIsKeptForEachModel(t *testing.T) {
	v := browseAll(t)
	kb := engine.NewKeyboard()
	tap := func(key engine.Key) {
		kb.Press(key)
		step(v, kb, 0.1)
		kb.Release(key)
		step(v, kb, 0.1)
	}

	tap('f') // smooth the house
	tap('n') // the icosphere, which has not been touched
	if v.models[v.current].smooth {
		t.Errorf("wanted the icosphere still faceted")
	}
	for range len(v.models) - 1 { // all the way round to the house
		tap('n')
	}
	if v.current != 0 || !v.models[0].smooth {
		t.Errorf("wanted the house back, still smooth; at model %d smooth %v", v.current, v.models[0].smooth)
	}
	// The shapes started smooth, and F makes one faceted.
	for v.models[v.current].name != "sphere" {
		tap('n')
	}
	tap('f')
	if v.models[v.current].smooth {
		t.Errorf("wanted F to make the smooth sphere faceted")
	}
}

// Smoothing a model that has no normals finds them once, and leaves the model as loaded.
func TestSmoothingFindsNormalsOnceAndKeepsTheOriginal(t *testing.T) {
	v := browseAll(t)
	v.current = 1 // the icosphere
	loaded := v.models[1].mesh

	faceted := v.shown()
	v.models[1].smooth = true
	first, second := v.shown(), v.shown()

	if faceted.Normals != nil {
		t.Errorf("wanted a faceted model drawn without normals")
	}
	if len(first.Normals) != len(first.Vertices) || len(first.Normals) == 0 {
		t.Fatalf("wanted a normal for each of %d vertices, got %d", len(first.Vertices), len(first.Normals))
	}
	if &first.Normals[0] != &second.Normals[0] {
		t.Errorf("wanted the normals found once and kept, but they were found again")
	}
	if loaded.Normals != nil || v.models[1].mesh.Normals != nil {
		t.Errorf("wanted the loaded model left without normals")
	}
	v.models[1].smooth = false
	if v.shown().Normals != nil {
		t.Errorf("wanted faceted again to draw without normals")
	}
}

func TestShapesKeepTheirOwnNormalsWhenFaceted(t *testing.T) {
	v := browseAll(t)
	sphere := modelIndex(t, v.models, "sphere")
	v.current = sphere
	own := v.models[sphere].mesh.Normals

	v.models[sphere].smooth = false
	if v.shown().Normals != nil {
		t.Errorf("wanted a faceted sphere drawn without normals")
	}
	v.models[sphere].smooth = true
	if got := v.shown().Normals; len(got) != len(own) || &got[0] != &own[0] {
		t.Errorf("wanted a smooth sphere drawn with its own normals, not found ones")
	}
}

// modelArea is the part of the frame where the model is drawn, as text: not the border
// and not the lines of text on top and bottom, which name the shading and the light.
func modelArea(fb *engine.FrameBuffer) string {
	var sb strings.Builder
	for y := 3; y < fb.Height()-4; y++ {
		for x := 1; x < fb.Width()-1; x++ {
			c := fb.At(x, y)
			fmt.Fprintf(&sb, "%c%v%v", c.Character, c.Foreground, c.Background)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func shadesIn(fb *engine.FrameBuffer) int {
	seen := map[rune]bool{}
	for _, c := range modelCells(fb) {
		seen[fb.At(c[0], c[1]).Character] = true
	}
	return len(seen)
}

// Smooth and faceted are different pictures, for every model, and smoothing the
// house, whose faces are each one flat shade, adds shades.
func TestSmoothAndFacetedLookDifferent(t *testing.T) {
	v := browseAll(t)
	for i := range v.models {
		v.current = i
		v.models[i].smooth = true
		smooth := drawAll(t, v)
		v.models[i].smooth = false
		faceted := drawAll(t, v)

		if modelArea(smooth) == modelArea(faceted) {
			t.Errorf("%s: wanted smooth and faceted drawings to differ", v.models[i].name)
		}
		if v.models[i].name == "house.obj" && shadesIn(smooth) <= shadesIn(faceted) {
			t.Errorf("house: wanted smoothing to add shades, got %d against %d", shadesIn(smooth), shadesIn(faceted))
		}
	}
}

func TestEveryModelDrawsSmoothOrFacetedInSunOrLampLight(t *testing.T) {
	v := browseAll(t)
	for i := range v.models {
		for _, smooth := range []bool{false, true} {
			for _, lamp := range []bool{false, true} {
				for _, blocks := range []bool{false, true} {
					name := fmt.Sprintf("%s smooth %v lamp %v half-blocks %v", v.models[i].name, smooth, lamp, blocks)
					t.Run(name, func(t *testing.T) {
						v.current, v.lamp, v.halfBlocks = i, lamp, blocks
						v.models[i].smooth = smooth

						fb := drawAll(t, v)

						if cells := modelCells(fb); len(cells) < 40 {
							t.Errorf("wanted the model on screen, got %d cells", len(cells))
						}
					})
				}
			}
		}
	}
}

func TestLampKeySwitchesTheLightOncePerPressInEitherCase(t *testing.T) {
	for _, key := range []engine.Key{'l', 'L'} {
		t.Run(fmt.Sprintf("%c", key), func(t *testing.T) {
			v := browseAll(t)
			kb := engine.NewKeyboard()

			kb.Press(key)
			step(v, kb, 0.1)
			if !v.lamp {
				t.Fatalf("wanted the first press to turn the lamp on")
			}
			for range 5 {
				step(v, kb, 0.1)
			}
			if !v.lamp {
				t.Errorf("wanted holding the key to change nothing")
			}
			kb.Release(key)
			kb.Press(key)
			step(v, kb, 0.1)
			if v.lamp {
				t.Errorf("wanted a second press to bring the sun back")
			}
		})
	}
}

// The sun stays where it is while you orbit. The lamp is held by the viewer, so it moves
// with the camera and stays the same way off from it.
func TestTheLampFollowsTheCameraAndTheSunDoesNot(t *testing.T) {
	v := browseAll(t)
	pointOf := func(lights []mesh.Light) mesh.Point {
		t.Helper()
		for _, l := range lights {
			if p, ok := l.(mesh.Point); ok {
				return p
			}
		}
		t.Fatalf("wanted a point light among %v", lights)
		return mesh.Point{}
	}

	if !slices.Equal(v.lights(v.camera()), sun) {
		t.Fatalf("wanted the sun's lights without the lamp")
	}
	v.lamp = true
	cam1 := v.camera()
	lamp1 := pointOf(v.lights(cam1))
	ambient := false
	for _, l := range v.lights(cam1) {
		a, ok := l.(mesh.Ambient)
		ambient = ambient || (ok && a.Level > 0)
	}
	if !ambient {
		t.Errorf("wanted some ambient light with the lamp, so that the unlit side is not black")
	}
	v.yaw += 1.1
	v.pitch -= 0.2
	cam2 := v.camera()
	lamp2 := pointOf(v.lights(cam2))

	if lamp1.Position.Distance(lamp2.Position) < 1 {
		t.Errorf("wanted the lamp to move with the camera, from %v to %v", lamp1.Position, lamp2.Position)
	}
	// Up and to the left of the camera by the same amounts, whichever way it faces.
	for i, c := range []struct {
		cam  mesh.Camera
		lamp mesh.Point
	}{{cam1, lamp1}, {cam2, lamp2}} {
		off := c.lamp.Position.Sub(c.cam.Position)
		if math.Abs(off.Y-lampUp) > 1e-9 || math.Abs(off.Dot(c.cam.Right())+lampAside) > 1e-9 || math.Abs(off.Dot(c.cam.Forward())) > 1e-9 {
			t.Errorf("view %d: wanted the lamp %v up and %v left of the camera, got %v away", i, lampUp, lampAside, off)
		}
		if want := lampReach * v.dist; math.Abs(c.lamp.Range-want) > 1e-9 {
			t.Errorf("view %d: wanted a reach of %v, got %v", i, want, c.lamp.Range)
		}
	}
	// And it reaches in proportion to how far back the camera is, so zooming keeps it lit.
	v.dist = 10
	if far := pointOf(v.lights(v.camera())); far.Range <= lamp2.Range {
		t.Errorf("wanted a longer reach when zoomed out, got %v after %v", far.Range, lamp2.Range)
	}
	v.lamp = false
	if got := v.lights(v.camera()); !slices.Equal(got, sun) {
		t.Errorf("wanted the sun back")
	}
}

// With the lamp up and to the left of the camera, the lit side of a smooth sphere is its
// upper left. The sun, from above and to one side, lights it from somewhere else.
func TestTheLampLightsTheUpperLeftOfASphere(t *testing.T) {
	v := browseAll(t)
	v.current = modelIndex(t, v.models, "sphere")
	v.lamp = true
	fb := drawAll(t, v)

	cells := modelCells(fb)
	best, ramp := -1, ".:-=+*#%@"
	var bx, by, mx, my float64
	var nBest int
	for _, c := range cells {
		mx, my = mx+float64(c[0]), my+float64(c[1])
		level := strings.IndexRune(ramp, fb.At(c[0], c[1]).Character)
		switch {
		case level > best:
			best, bx, by, nBest = level, float64(c[0]), float64(c[1]), 1
		case level == best:
			bx, by, nBest = bx+float64(c[0]), by+float64(c[1]), nBest+1
		}
	}
	bx, by = bx/float64(nBest), by/float64(nBest)
	mx, my = mx/float64(len(cells)), my/float64(len(cells))

	if bx >= mx || by >= my {
		t.Errorf("wanted the brightest cells (%.1f, %.1f) above and left of the sphere's middle (%.1f, %.1f)\n%s", bx, by, mx, my, fb.String())
	}

	v.lamp = false
	if modelArea(drawAll(t, v)) == modelArea(fb) {
		t.Errorf("wanted the sun and the lamp to light the sphere differently")
	}
}

// The lamp lights the half-block pixels too.
func TestTheLampLightsHalfBlockPixelsToo(t *testing.T) {
	v := browseAll(t)
	v.current, v.halfBlocks = 3, true // the sphere
	sunlit := modelArea(drawAll(t, v))

	v.lamp = true
	lit := modelArea(drawAll(t, v))

	if sunlit == lit {
		t.Errorf("wanted the lamp to light the pixels differently from the sun")
	}
}

func TestHUDNamesTheShadingAndTheLight(t *testing.T) {
	v := browseAll(t)
	line := func() string { return hudText(drawAll(t, v), 2) }

	v.current = 1 // the icosphere, faceted to begin with
	if got := line(); !strings.Contains(got, "characters, faceted, sun") {
		t.Errorf("wanted faceted and sun on the second line, got %q", got)
	}
	v.models[1].smooth, v.lamp = true, true
	if got := line(); !strings.Contains(got, "characters, smooth, lamp") {
		t.Errorf("wanted smooth and lamp, got %q", got)
	}
	v.halfBlocks, v.doubleSided = true, true
	if got := line(); !strings.Contains(got, "two-sided, smooth, lamp") {
		t.Errorf("wanted the shading and light after the mode, got %q", got)
	}
	v.current = modelIndex(t, v.models, "sphere") // it starts smooth
	v.lamp = false
	if got := line(); !strings.Contains(got, "smooth, sun") {
		t.Errorf("wanted the sphere smooth and in the sun, got %q", got)
	}

	// A wireframe has neither.
	v.wire = true
	got := line()
	for _, word := range []string{"smooth", "faceted", "sun", "lamp"} {
		if strings.Contains(got, word) {
			t.Errorf("wanted a wireframe to say nothing of %q, got %q", word, got)
		}
	}
}

func TestHelpMentionsTheShadingAndLightKeys(t *testing.T) {
	fb := drawAll(t, browseAll(t))

	got := hudText(fb, fb.Height()-2)

	// At the end of the line, so the window shows it whole.
	if !strings.HasSuffix(strings.TrimRight(got, " "), "F smooth  L lamp") {
		t.Errorf("wanted the help to end with F and L, whole, got %q", got)
	}
}

// The four materials of house.mtl, as the loader must read them from its Kd lines.
var houseColors = map[string]engine.Color{
	"walls":   engine.RGB(232, 204, 153),
	"roof":    engine.RGB(191, 51, 38),
	"chimney": engine.RGB(140, 140, 148),
	"door":    engine.RGB(102, 64, 31),
}

// foregrounds returns the foreground colors of the model's cells.
func foregrounds(fb *engine.FrameBuffer) map[engine.Color]bool {
	seen := map[engine.Color]bool{}
	for _, c := range modelCells(fb) {
		seen[fb.At(c[0], c[1]).Foreground] = true
	}
	return seen
}

func TestOnlyTheHouseAndTheOctahedronHaveColorsOfTheirOwn(t *testing.T) {
	for _, m := range loadSamples(t) {
		if want := m.name == "house.obj" || m.name == "octahedron.obj"; m.colored != want {
			t.Errorf("%s: wanted colored=%v, got %v", m.name, want, m.colored)
		}
		// The house has a color for each face, from its materials, and the octahedron one
		// for each vertex, from the vertex lines of its file.
		if m.name == "house.obj" && len(m.mesh.FaceColors) != m.faces {
			t.Errorf("%s: wanted a color for each of %d faces, got %d", m.name, m.faces, len(m.mesh.FaceColors))
		}
		if m.name == "octahedron.obj" && (len(m.mesh.VertexColors) != m.verts || len(m.mesh.FaceColors) != 0) {
			t.Errorf("%s: wanted a color for each of %d vertices and none for faces, got %d and %d", m.name, m.verts, len(m.mesh.VertexColors), len(m.mesh.FaceColors))
		}
	}
	for _, m := range shapeModels() {
		if m.colored {
			t.Errorf("%s: wanted a shape to have no colors of its own", m.name)
		}
	}
}

func TestHouseOpensInItsOwnColors(t *testing.T) {
	v := newViewer(loadSamples(t))
	fb := drawAll(t, v)

	// Flat shading shows each face in its color, so the screen holds exactly the four.
	got := foregrounds(fb)
	if len(got) != len(houseColors) {
		t.Errorf("wanted the 4 colors of the materials, got %v", got)
	}
	for name, want := range houseColors {
		if !got[want] {
			t.Errorf("wanted the %s in %v, got %v", name, want, got)
		}
	}
	if got := hudText(fb, 2); !strings.Contains(got, "characters, faceted, sun, model colors") {
		t.Errorf("wanted the HUD to say the model's colors are shown, got %q", got)
	}
}

// The octahedron has a color on each corner, so its faces fade from one to the next: more
// colors on screen than it has corners, and none of them the palette color.
func TestOctahedronOpensInItsVertexColors(t *testing.T) {
	models := loadSamples(t)
	v := newViewer(models)
	v.current = modelIndex(t, models, "octahedron.obj")
	fb := drawAll(t, v)

	got := foregrounds(fb)
	if len(got) <= len(v.models[v.current].mesh.VertexColors) {
		t.Errorf("wanted more colors than the 6 corners, as the faces fade, got %d: %v", len(got), got)
	}
	if got[palette[0]] {
		t.Errorf("wanted the palette color to stay out of it, got %v", got)
	}
	if text := hudText(fb, 2); !strings.Contains(text, "model colors") {
		t.Errorf("wanted the HUD to say the model's colors are shown, got %q", text)
	}
}

func TestVertexColorsReachHalfBlockPixels(t *testing.T) {
	models := loadSamples(t)
	v := newViewer(models)
	v.current = modelIndex(t, models, "octahedron.obj")
	v.halfBlocks = true
	fb := drawAll(t, v)

	shown := map[engine.Color]bool{}
	for _, c := range modelCells(fb) {
		cell := fb.At(c[0], c[1])
		shown[cell.Foreground], shown[cell.Background] = true, true
	}
	if len(shown) < 12 {
		t.Errorf("wanted many shades of the corners' colors among the pixels, got %d: %v", len(shown), shown)
	}
	if got := hudText(fb, 2); !strings.Contains(got, "model colors") {
		t.Errorf("wanted the HUD to say the model's colors are shown, got %q", got)
	}
}

// C puts the vertex colors aside for each palette color in turn and brings them back.
func TestColorKeyCyclesTheOctahedronsVertexColors(t *testing.T) {
	v := quiet(t)
	v.current = modelIndex(t, v.models, "octahedron.obj")
	kb := engine.NewKeyboard()

	for i := 1; i <= len(palette)+1; i++ {
		kb.Press('c')
		step(v, kb, 0.1)
		kb.Release('c')

		got := foregrounds(drawAll(t, v))
		if i <= len(palette) {
			if len(got) != 1 || !got[palette[i-1]] {
				t.Fatalf("after %d presses: wanted only %v, got %v", i, palette[i-1], got)
			}
		} else if len(got) < 7 || !v.ownColors {
			t.Fatalf("after %d presses: wanted the vertex colors back, got %d colors (own=%v)", i, len(got), v.ownColors)
		}
	}
}

func TestModelColorsReachHalfBlockPixels(t *testing.T) {
	v := newViewer(loadSamples(t))
	v.halfBlocks = true
	fb := drawAll(t, v)

	shown := map[engine.Color]bool{}
	for _, c := range modelCells(fb) {
		cell := fb.At(c[0], c[1])
		shown[cell.Foreground], shown[cell.Background] = true, true
	}
	// Each material shows as some shade of its color; shades below a third are too dark
	// to tell the materials apart.
	for name, kd := range houseColors {
		found := false
		for i := 600; i <= 2000 && !found; i++ {
			found = shown[engine.Shade(kd, float64(i)/2000)]
		}
		if !found {
			t.Errorf("wanted a shade of the %s %v among the pixels, got %v", name, kd, shown)
		}
	}
	if got := hudText(fb, 2); !strings.Contains(got, "model colors") {
		t.Errorf("wanted the HUD to say the model's colors are shown, got %q", got)
	}
}

func TestModelsWithoutColorsAreDrawnInThePaletteColor(t *testing.T) {
	models := browseAll(t).models
	for i, m := range models {
		if m.colored {
			continue
		}
		v := newViewer(models)
		v.current = i
		fb := drawAll(t, v)

		if got := foregrounds(fb); len(got) != 1 || !got[palette[0]] {
			t.Errorf("%s: wanted only %v, got %v", m.name, palette[0], got)
		}
		if got := hudText(fb, 2); strings.Contains(got, "model colors") {
			t.Errorf("%s: wanted no mention of model colors, got %q", m.name, got)
		}
	}
}

// C goes from the model's colors through the six palette colors and back to them.
func TestColorKeyCyclesAModelsColorsAndThePalette(t *testing.T) {
	v := quiet(t) // the house
	kb := engine.NewKeyboard()

	for i := 1; i <= 2*(len(palette)+1); i++ {
		kb.Press('c')
		step(v, kb, 0.1)
		kb.Release('c')

		// Press 1 is the first palette color, 6 the last, 7 the model's own again.
		k := (i - 1) % (len(palette) + 1)
		wantOwn, wantColor := k == len(palette), k
		if k == len(palette) {
			wantColor = 0
		}
		if v.ownColors != wantOwn || v.color != wantColor {
			t.Fatalf("after %d presses: wanted own=%v color=%d, got own=%v color=%d", i, wantOwn, wantColor, v.ownColors, v.color)
		}

		fb := drawAll(t, v)
		got := foregrounds(fb)
		if wantOwn {
			if len(got) != len(houseColors) || got[palette[0]] {
				t.Fatalf("after %d presses: wanted the house's own colors, got %v", i, got)
			}
		} else if len(got) != 1 || !got[palette[wantColor]] {
			t.Fatalf("after %d presses: wanted only %v, got %v", i, palette[wantColor], got)
		}
	}

	// The capital letter works too.
	kb.Press('C')
	step(v, kb, 0.1)
	if v.ownColors {
		t.Errorf("wanted a capital C to leave the model's colors too")
	}
}

func TestWireframeKeepsThePaletteColorAndSaysNothingOfModelColors(t *testing.T) {
	v := newViewer(loadSamples(t))
	v.wire = true
	fb := drawAll(t, v)

	if got := foregrounds(fb); len(got) != 1 || !got[palette[0]] {
		t.Errorf("wanted a wireframe in %v, got %v", palette[0], got)
	}
	if got := hudText(fb, 2); strings.Contains(got, "model colors") {
		t.Errorf("wanted no mention of model colors on a wireframe, got %q", got)
	}
}

// The setting is the viewer's, not each model's: leaving the house for the palette and
// coming back to it finds it still in the palette.
func TestModelColorsSettingIsKeptAcrossModels(t *testing.T) {
	v := newViewer(browseAll(t).models)
	kb := engine.NewKeyboard()

	press := func() {
		kb.Press('c')
		step(v, kb, 0.1)
		kb.Release('c')
	}

	press() // the house: palette color 0
	v.current = 1
	press() // the icosphere: palette color 1
	v.current = 0

	if v.ownColors || v.color != 1 {
		t.Errorf("wanted the house to find the palette at color 1, got own=%v color=%d", v.ownColors, v.color)
	}
	if got := foregrounds(drawAll(t, v)); len(got) != 1 || !got[palette[1]] {
		t.Errorf("wanted the house in %v, got %v", palette[1], got)
	}

	// Back to the house's colors and out again starts the palette over at its first color,
	// whatever another model has moved it to.
	for range len(palette) - 1 { // from color 1 round to the house's own colors
		press()
	}
	v.current = 1
	press()
	press() // color 2
	v.current = 0
	press() // the house: the palette from its first color
	if v.ownColors || v.color != 0 {
		t.Errorf("wanted the house to start the palette at color 0, got own=%v color=%d", v.ownColors, v.color)
	}
}

// writeModel writes an OBJ and the files beside it into one folder and returns the OBJ's path.
func writeModel(t *testing.T, obj string, beside map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range beside {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(dir, "m.obj")
	if err := os.WriteFile(p, []byte(obj), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAFileGetsItsColorsFromTheMTLBesideIt(t *testing.T) {
	const obj = "mtllib m.mtl\nv 0 0 0\nv 4 0 0\nv 0 2 0\nusemtl red\nf 1 2 3\n"
	with := writeModel(t, obj, map[string]string{"m.mtl": "newmtl red\nKd 1 0 0\n"})
	without := writeModel(t, obj, nil)

	models, err := loadModels([]string{with, without})
	if err != nil {
		t.Fatal(err)
	}
	if !models[0].colored || !slices.Equal(models[0].mesh.FaceColors, []engine.Color{engine.RGB(255, 0, 0)}) {
		t.Errorf("wanted the file with its MTL colored red, got colored=%v %v", models[0].colored, models[0].mesh.FaceColors)
	}
	if models[1].colored {
		t.Errorf("wanted the file without its MTL uncolored, got %v", models[1].mesh.FaceColors)
	}

	v := newViewer(models[:1])
	v.pitch, v.yaw = 0, 0
	if got := foregrounds(drawAll(t, v)); len(got) != 1 || !got[engine.RGB(255, 0, 0)] {
		t.Errorf("wanted the triangle drawn red, got %v", got)
	}
}

func TestAMTLNamedOutsideTheFilesFolderIsNotRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "secret.mtl"), []byte("newmtl red\nKd 1 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	obj := filepath.Join(sub, "m.obj")
	if err := os.WriteFile(obj, []byte("mtllib ../secret.mtl\nv 0 0 0\nv 4 0 0\nv 0 2 0\nusemtl red\nf 1 2 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	models, err := loadModels([]string{obj})
	if err != nil || models[0].colored {
		t.Errorf("wanted the model without colors, got colored=%v and err %v", models[0].colored, err)
	}
}

// A face the file gave no color takes the palette color, beside the colored ones.
func TestFacesWithoutAColorTakeThePaletteColorBesideColoredOnes(t *testing.T) {
	obj := writeModel(t, "mtllib m.mtl\nv 0 0 0\nv 4 0 0\nv 0 2 0\nv 0 0 3\nusemtl red\nf 1 2 3\nusemtl none\nf 1 3 4\n",
		map[string]string{"m.mtl": "newmtl red\nKd 1 0 0\n"})
	models, err := loadModels([]string{obj})
	if err != nil || !models[0].colored {
		t.Fatalf("wanted a model with some colors, got colored=%v and err %v", models[0].colored, err)
	}

	for _, halfBlocks := range []bool{false, true} {
		v := newViewer(models)
		v.pitch, v.yaw, v.halfBlocks, v.doubleSided = -0.3, 0.7, halfBlocks, true
		fb := drawAll(t, v)

		var palettes, reds int
		for _, c := range modelCells(fb) {
			cell := fb.At(c[0], c[1])
			for _, got := range []engine.Color{cell.Foreground, cell.Background} {
				// Pixels are shades in RGB, where red has no green or blue and the orange of
				// the palette color has some green. Characters have the exact colors.
				r, g, b := rgbOf(got)
				switch {
				case got == palette[0] || (halfBlocks && g > 0):
					palettes++
				case got == engine.RGB(255, 0, 0) || (halfBlocks && r > 0 && g == 0 && b == 0):
					reds++
				}
			}
		}
		if palettes == 0 || reds == 0 {
			t.Errorf("halfBlocks=%v: wanted both the palette color and red on screen, got %d and %d", halfBlocks, palettes, reds)
		}
	}
}

// rgbOf reads the red, green and blue of a color from its "#rrggbb" text, the only
// way the engine shows them.
func rgbOf(c engine.Color) (r, g, b int) {
	if _, err := fmt.Sscanf(c.String(), "#%02x%02x%02x", &r, &g, &b); err != nil {
		return 0, 0, 0 // a default or palette color
	}
	return r, g, b
}

// pixelsOf decodes the half-block cells of a frame back into the pixels they hold, inside
// the frame border and between the lines of text: pixel (x, 2*y) is the top of cell (x, y)
// and (x, 2*y+1) its bottom.
func pixelsOf(fb *engine.FrameBuffer) map[[2]int]engine.Color {
	out := map[[2]int]engine.Color{}
	for y := 3; y < fb.Height()-4; y++ {
		for x := 1; x < fb.Width()-1; x++ {
			c := fb.At(x, y)
			var top, bottom engine.Color
			switch c.Character {
			case '\u2580': // upper half block
				top, bottom = c.Foreground, c.Background
			case '\u2584': // lower half block
				top, bottom = c.Background, c.Foreground
			case ' ': // a solid cell, or nothing
				top, bottom = c.Background, c.Background
			default:
				continue
			}
			if !top.IsDefault() {
				out[[2]int{x, 2 * y}] = top
			}
			if !bottom.IsDefault() {
				out[[2]int{x, 2*y + 1}] = bottom
			}
		}
	}
	return out
}

// With half-blocks on, V draws the wireframe as lines of pixels: no # stands in for them,
// they are in the palette color, and they are exactly the pixels DrawPixels draws for the
// model as the viewer shows it, for every model and shape.
func TestWireframeWithHalfBlocksIsLinesOfPixels(t *testing.T) {
	models := browseAll(t).models
	for i, m := range models {
		t.Run(m.name, func(t *testing.T) {
			v := newViewer(models)
			v.current, v.wire, v.halfBlocks, v.color, v.doubleSided = i, true, true, 2, i%2 == 0
			fb := drawAll(t, v)

			for _, c := range modelCells(fb) {
				if fb.At(c[0], c[1]).Character == '#' {
					t.Fatalf("wanted no # in a pixel wireframe, got one at %v", c)
				}
			}
			got := pixelsOf(fb)
			if len(got) < 40 {
				t.Fatalf("wanted the wireframe on screen as pixels, got %d", len(got))
			}
			for p, color := range got {
				if color != palette[2] {
					t.Fatalf("pixel %v: wanted the palette color %v, got %v", p, palette[2], color)
				}
			}

			ref := newFrame(t, 80, 24)
			canvas := ref.Canvas()
			shown := v.shown()
			shown.DoubleSided = v.doubleSided
			shown.DrawPixels(canvas, v.camera(), palette[2])
			canvas.Compose()
			if want := pixelsOf(ref); !maps.Equal(got, want) {
				t.Fatalf("wanted exactly the pixels DrawPixels draws for the model (%d), got %d", len(want), len(got))
			}
		})
	}
}

func newFrame(t *testing.T, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

func TestHUDNamesAWireframeOfPixels(t *testing.T) {
	v := newViewer(loadSamples(t))
	v.wire, v.halfBlocks = true, true

	if got := hudText(drawAll(t, v), 2); !strings.Contains(got, "wireframe, half-blocks, exact edges") {
		t.Errorf("wanted wireframe, half-blocks, exact edges, got %q", got)
	}
	v.flipEdges, v.doubleSided = true, true
	got := hudText(drawAll(t, v), 2)
	if !strings.Contains(got, "wireframe, half-blocks, solid edges, two-sided") {
		t.Errorf("wanted wireframe, half-blocks, solid edges, two-sided, got %q", got)
	}
	for _, word := range []string{"smooth", "faceted", "sun", "lamp", "model colors"} {
		if strings.Contains(got, word) {
			t.Errorf("wanted a wireframe to say nothing of %q, got %q", word, got)
		}
	}
}

// Each of V and M alone is what it was: # characters for a wireframe, shaded half blocks
// for M. Only the two together are new.
func TestWireframeAndHalfBlocksAloneAreAsTheyWere(t *testing.T) {
	v := newViewer(loadSamples(t))
	v.wire = true
	fb := drawAll(t, v)
	hashes := 0
	for _, c := range modelCells(fb) {
		if fb.At(c[0], c[1]).Character == '#' {
			hashes++
		}
	}
	if hashes < 20 || len(pixelsOf(fb)) != 0 {
		t.Errorf("wanted a wireframe of # alone, got %d # and %d pixels", hashes, len(pixelsOf(fb)))
	}

	v.dist = 2.5 // close, where an area grows faster than a line does
	v.wire, v.halfBlocks = true, true
	lines := len(pixelsOf(drawAll(t, v)))
	v.wire = false
	if solid := len(pixelsOf(drawAll(t, v))); solid < 2*lines || lines < 40 {
		t.Errorf("wanted shaded half blocks alone to fill far more pixels than their wireframe, got %d against %d", solid, lines)
	}
}

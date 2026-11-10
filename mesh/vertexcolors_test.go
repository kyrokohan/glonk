package mesh

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// viewMap says where a point of view space lands on the cell grid, which is what it takes
// to send a ray back out through a cell. It is found by asking the projector about three
// points, so the tests below share no arithmetic with the code that blends the colors.
type viewMap struct{ ox, oy, fx, fy float64 }

func newViewMap(cam Camera, w, h int) viewMap {
	cam, _ = cam.withDefaults()
	proj := cam.projector(w, h)
	x0, y0 := proj.project(geometry.Vector3{Z: 1})
	x1, _ := proj.project(geometry.Vector3{X: 1, Z: 1})
	_, y1 := proj.project(geometry.Vector3{Y: 1, Z: 1})
	return viewMap{ox: x0, oy: y0, fx: x1 - x0, fy: y1 - y0}
}

// hit sends a ray through the point (x, y) of the cell grid and returns the weights of
// the triangle's corners at the place it meets the triangle, and how far along the view
// axis that is. Where a color changes evenly across a triangle in space, this is the
// color a viewer sees there, with perspective and whatever the near plane cut away.
func (vm viewMap) hit(x, y float64, v [3]geometry.Vector3) (b [3]float64, z float64) {
	d := geometry.Vector3{X: (x - vm.ox) / vm.fx, Y: (y - vm.oy) / vm.fy, Z: 1}
	e1, e2 := v[1].Sub(v[0]), v[2].Sub(v[0])
	n := e1.Cross(e2)
	den := d.Dot(n)
	if den == 0 {
		return b, math.Inf(-1)
	}
	s := v[0].Dot(n) / den
	p := d.Scale(s).Sub(v[0])
	nn := n.Dot(n)
	b1 := p.Cross(e2).Dot(n) / nn
	b2 := e1.Cross(p).Dot(n) / nn
	return [3]float64{1 - b1 - b2, b1, b2}, s
}

// blendCam looks down +Z from the origin, where view space and the world are the same.
func blendCam() Camera { return Camera{FOV: 1.4, Near: 0.5} }

// colorOf3 is a color as red, green and blue from 0 to 255, for corners that are not whole
// numbers once a face color has tinted them.
func colorOf3(r, g, b float64) geometry.Vector3 { return geometry.Vector3{X: r, Y: g, Z: b} }

// near1 reports whether got is the color (r, g, b) rounded to whole numbers. A value that
// is within a hair of half-way may have been rounded either way, since the weights it
// came from are worked out another way here.
func near1(got engine.Color, r, g, b float64) bool {
	gr, gg, gb := got.RGB()
	ok := func(have uint8, want float64) bool {
		if math.Abs(math.Abs(want-math.Floor(want))-0.5) < 1e-6 {
			return math.Abs(float64(have)-want) <= 0.5+1e-6
		}
		return float64(have) == math.Round(want)
	}
	return ok(gr, r) && ok(gg, g) && ok(gb, b)
}

// checkBlend draws a one-face mesh and holds every cell (or pixel) well inside the face to
// the blend of the corners' colors at the place a ray through it meets the face. The mesh
// has the three corners at its vertices in order and is seen from the origin.
func checkBlend(t *testing.T, m Mesh, cam Camera, corners [3]geometry.Vector3, pixels bool) {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "")
	fb := newShadeBuffer(t)
	vm := newViewMap(cam, fb.Width(), fb.Height())
	cam, _ = cam.withDefaults()

	var tri [3]geometry.Vector3
	for k, i := range m.Faces[0] {
		tri[k] = m.Vertices[i].Add(m.Transform.Position)
	}

	var compared int
	check := func(px, py int, x, y float64, got engine.Color) {
		b, z := vm.hit(x, y, tri)
		inside := min(b[0], b[1], b[2])
		if z < cam.Near+0.05 || inside < 0.03 {
			if inside < -0.1 && z > cam.Near && !got.IsDefault() {
				t.Errorf("(%d, %d): wanted nothing outside the face, got %v", px, py, got)
			}
			return
		}
		compared++
		sum := corners[0].Scale(b[0]).Add(corners[1].Scale(b[1])).Add(corners[2].Scale(b[2]))
		if got.IsDefault() || !near1(got, sum.X, sum.Y, sum.Z) {
			t.Errorf("(%d, %d): wanted about (%.1f, %.1f, %.1f) from weights %.3f, got %v", px, py, sum.X, sum.Y, sum.Z, b, got)
		}
	}

	light := Ambient{Level: 1}
	if pixels {
		canvas := fb.Canvas()
		m.DrawShadedPixels(canvas, cam, light)
		for py := range canvas.Height() {
			for px := range canvas.Width() {
				check(px, py, float64(px)+0.5, (float64(py)+0.5)/2, canvas.At(px, py))
			}
		}
		if compared < 60 {
			t.Fatalf("wanted at least 60 pixels to compare, got %d", compared)
		}
		return
	}
	m.DrawShaded(fb, cam, light)
	for y := range fb.Height() {
		for x := range fb.Width() {
			check(x, y, float64(x)+0.5, float64(y)+0.5, fb.At(x, y).Foreground)
			// Full light is the last character of the ramp.
			if c := fb.At(x, y); !c.Foreground.IsDefault() && c.Character != '@' {
				t.Errorf("(%d, %d): wanted the brightest character, got %q", x, y, c.Character)
			}
		}
	}
	if compared < 30 {
		t.Fatalf("wanted at least 30 cells to compare, got %d", compared)
	}
}

// blendTriangle is a triangle seen at a slant, its corners at different depths, with
// a color on each corner.
func blendTriangle(colors ...engine.Color) Mesh {
	return Mesh{
		Vertices:     []geometry.Vector3{{X: -2, Y: -1.5, Z: 5}, {X: 2.5, Y: -1, Z: 9}, {X: 0, Y: 2.2, Z: 6}},
		Faces:        [][3]int{{0, 1, 2}},
		VertexColors: colors,
		DoubleSided:  true, // so that the winding does not matter
	}
}

func asVectors(colors ...engine.Color) (out [3]geometry.Vector3) {
	for i, c := range colors {
		out[i] = colorVector(c)
	}
	return out
}

var (
	pureYellow = engine.RGB(250, 240, 0)
	darkGray   = engine.RGB(40, 40, 40)
)

// A face with a color on each corner is drawn in the blend of them at every cell, the one a
// viewer would see at that place on the face: with perspective, from either side, flat or
// smooth, in cells and in pixels, and with the near plane cutting a corner off.
func TestVertexColorsBlendAcrossAFace(t *testing.T) {
	corners := asVectors(pureRed, pureGreen, pureBlue)
	flat := blendTriangle(pureRed, pureGreen, pureBlue)
	smooth := flat
	smooth.Normals = []geometry.Vector3{{X: 0.3, Z: -1}, {Y: 0.4, Z: -1}, {X: -0.5, Y: -0.2, Z: -1}}
	backwards := flat
	backwards.Faces = [][3]int{{0, 2, 1}}
	cut := Mesh{ // a corner behind the camera
		Vertices:     []geometry.Vector3{{X: -2, Y: -1, Z: -1}, {X: 3, Y: -1, Z: 4}, {X: 0, Y: 3, Z: 6}},
		Faces:        [][3]int{{0, 1, 2}},
		VertexColors: []engine.Color{pureRed, pureGreen, pureBlue},
		DoubleSided:  true,
	}
	cutTwo := Mesh{ // two corners behind it
		Vertices:     []geometry.Vector3{{X: -2, Y: -1, Z: -1}, {X: 3, Y: -1, Z: -2}, {X: 0, Y: 3, Z: 7}},
		Faces:        [][3]int{{0, 1, 2}},
		VertexColors: []engine.Color{pureRed, pureGreen, pureBlue},
		DoubleSided:  true,
	}

	viewed := asVectors(pureRed, pureBlue, pureGreen) // the corners of the face {0, 2, 1}, in its order
	behind := func(corner int) Mesh {                 // the one corner that is cut off is not always the first
		m := Mesh{
			Vertices:     []geometry.Vector3{{X: -2, Y: -1, Z: 5}, {X: 3, Y: -1, Z: 4}, {X: 0, Y: 3, Z: 6}},
			Faces:        [][3]int{{0, 1, 2}},
			VertexColors: []engine.Color{pureRed, pureGreen, pureBlue},
			DoubleSided:  true,
		}
		m.Vertices[corner].Z = -1.5
		return m
	}
	for _, tt := range []struct {
		name    string
		m       Mesh
		corners [3]geometry.Vector3
	}{
		{"flat", flat, corners},
		{"smooth", smooth, corners},
		{"seen from behind", backwards, viewed},
		{"one corner cut off", cut, corners},
		{"two corners cut off", cutTwo, corners},
		{"the middle corner cut off", behind(1), corners},
		{"the last corner cut off", behind(2), corners},
	} {
		for _, pixels := range []bool{false, true} {
			name := tt.name + ", cells"
			if pixels {
				name = tt.name + ", pixels"
			}
			t.Run(name, func(t *testing.T) {
				checkBlend(t, tt.m, blendCam(), tt.corners, pixels)
			})
		}
	}
}

func TestVertexColorsAreMultipliedWithTheFacesOwnColor(t *testing.T) {
	face := engine.RGB(200, 100, 50)
	// White leaves the face color as it is, gray darkens it and black blacks it out.
	vertex := []engine.Color{engine.RGB(255, 255, 255), engine.RGB(128, 128, 128), engine.RGB(0, 0, 0)}
	f := colorVector(face)
	corners := [3]geometry.Vector3{
		f,
		colorOf3(128*f.X/255, 128*f.Y/255, 128*f.Z/255),
		colorOf3(0, 0, 0),
	}
	m := blendTriangle(vertex...)
	m.FaceColors = []engine.Color{face}
	m.Material.Color = engine.Palette(208) // not used: the face has a color
	for _, pixels := range []bool{false, true} {
		checkBlend(t, m, blendCam(), corners, pixels)
	}
}

func TestACornerWithoutAColorTakesWhatTheFaceWouldBe(t *testing.T) {
	face := engine.RGB(40, 200, 90)
	tests := []struct {
		name    string
		m       func() Mesh
		corners [3]geometry.Vector3
	}{
		{"Material.Color", func() Mesh {
			m := blendTriangle(pureRed, engine.Color{}, engine.Color{})
			m.Material.Color = pureBlue
			return m
		}, asVectors(pureRed, pureBlue, pureBlue)},
		{"the face color, tinting the others", func() Mesh {
			m := blendTriangle(engine.RGB(255, 128, 0), engine.Color{}, engine.Color{})
			m.FaceColors = []engine.Color{face}
			m.Material.Color = pureBlue
			return m
		}, [3]geometry.Vector3{colorOf3(40, 200*128/255.0, 0), colorVector(face), colorVector(face)}},
		{"black, when there is neither", func() Mesh {
			return blendTriangle(engine.Color{}, pureYellow, engine.Color{})
		}, [3]geometry.Vector3{{}, colorVector(pureYellow), {}}},
		{"Material.Color is not a tint", func() Mesh {
			m := blendTriangle(darkGray, darkGray, darkGray)
			m.Material.Color = pureRed
			return m
		}, asVectors(darkGray, darkGray, darkGray)},
		{"a palette color is read as xterm has it", func() Mesh {
			return blendTriangle(engine.Palette(208), engine.Palette(21), engine.Palette(46))
		}, [3]geometry.Vector3{{X: 255, Y: 135}, {Z: 255}, {Y: 255}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, pixels := range []bool{false, true} {
				checkBlend(t, tt.m(), blendCam(), tt.corners, pixels)
			}
		})
	}
}

// A mesh whose vertices all have one color is drawn exactly as a mesh whose material has
// it, in glyphs and in pixels, flat and smooth, even with the near plane cutting through:
// the blend of three equal colors is that color at every cell.
func TestVertexColorsOfOneColorDrawAsThatColor(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)
	tone := engine.RGB(210, 90, 40)

	flat := Cube(2)
	flat.Transform = Transform{Position: geometry.Vector3{Z: 5}, Yaw: 0.6, Pitch: -0.4}
	smooth := Sphere(1.6, 10, 6)
	smooth.Transform = Transform{Position: geometry.Vector3{Z: 4}, Yaw: 0.3}
	cut := Cube(2)
	cut.Transform = Transform{Position: geometry.Vector3{X: 2, Z: 1}, Yaw: 0.1}
	for _, tt := range []struct {
		name string
		m    Mesh
		cam  Camera
	}{
		{"flat", flat, shadeCam()},
		{"smooth", smooth, shadeCam()},
		{"cut by the near plane", cut, Camera{FOV: 1.4, Near: 0.5}},
	} {
		for _, pixels := range []bool{false, true} {
			name := tt.name + ", glyphs"
			if pixels {
				name = tt.name + ", pixels"
			}
			t.Run(name, func(t *testing.T) {
				plain := tt.m.withColor(tone)
				coloured := tt.m
				coloured.VertexColors = slices.Repeat([]engine.Color{tone}, len(tt.m.Vertices))

				draw := func(m Mesh) *engine.FrameBuffer {
					fb := newShadeBuffer(t)
					if pixels {
						canvas := fb.Canvas()
						m.DrawShadedPixels(canvas, tt.cam, light)
						canvas.Compose()
					} else {
						m.DrawShaded(fb, tt.cam, light)
					}
					return fb
				}
				want, got := draw(plain), draw(coloured)
				if picture(got) != picture(want) {
					t.Fatalf("wanted the picture of a mesh in one color\n%s\ngot\n%s", picture(want), picture(got))
				}
				if strings.HasSuffix(picture(want), "legend\n") {
					t.Fatalf("wanted something drawn in color")
				}
			})
		}
	}
}

// White vertex colors leave a face color as it is.
func TestWhiteVertexColorsLeaveFaceColorsAlone(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)
	m := Sphere(1.6, 10, 6)
	m.Transform = Transform{Position: geometry.Vector3{Z: 4}}
	m.FaceColors = colorsCycling(len(m.Faces))
	white := m.Clone()
	white.VertexColors = slices.Repeat([]engine.Color{engine.RGB(255, 255, 255)}, len(m.Vertices))

	for _, pixels := range []bool{false, true} {
		draw := func(m Mesh) string {
			fb := newShadeBuffer(t)
			if pixels {
				canvas := fb.Canvas()
				m.DrawShadedPixels(canvas, shadeCam(), light)
				canvas.Compose()
			} else {
				m.DrawShaded(fb, shadeCam(), light)
			}
			return picture(fb)
		}
		// A face with no color of its own takes Material.Color, and white vertex colors
		// are drawn as white there, so only faces that have a color are held to the same.
		withBase, plain := m.Clone(), white.Clone()
		for i, c := range withBase.FaceColors {
			if c.IsDefault() {
				withBase.FaceColors[i], plain.FaceColors[i] = pureGreen, pureGreen
			}
		}
		if draw(withBase) != draw(plain) {
			t.Errorf("pixels=%v: wanted white vertex colors to leave the face colors alone", pixels)
		}
	}
}

// A face none of whose corners has a color is drawn as if the mesh had no colors, even
// beside a face that has some.
func TestAFaceWithoutVertexColorsIsDrawnAsBefore(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)
	two := Mesh{
		Vertices: []geometry.Vector3{
			{X: -3, Y: -1, Z: 6}, {X: -1, Y: -1, Z: 6}, {X: -2, Y: 1, Z: 6},
			{X: 1, Y: -1, Z: 6}, {X: 3, Y: -1, Z: 6}, {X: 2, Y: 1, Z: 6},
		},
		Faces:        [][3]int{{0, 2, 1}, {3, 5, 4}},
		VertexColors: []engine.Color{pureRed, pureGreen, pureBlue, {}, {}, {}},
		Material:     Material{Color: engine.RGB(200, 200, 40)},
		DoubleSided:  true,
	}
	for _, pixels := range []bool{false, true} {
		draw := func(m Mesh, fb *engine.FrameBuffer) {
			if pixels {
				canvas := fb.Canvas()
				m.DrawShadedPixels(canvas, shadeCam(), light)
				canvas.Compose()
			} else {
				m.DrawShaded(fb, shadeCam(), light)
			}
		}
		left, right := two, two
		left.Faces, left.VertexColors = two.Faces[:1], two.VertexColors
		right.Faces, right.VertexColors = two.Faces[1:], nil

		got, want := newShadeBuffer(t), newShadeBuffer(t)
		draw(two, got)
		draw(left, want)
		draw(right, want)
		if picture(got) != picture(want) {
			t.Errorf("pixels=%v: wanted the second face as it is without colors\n%s\ngot\n%s", pixels, picture(want), picture(got))
		}
	}
}

func TestVertexColorsOfTheWrongLengthAreNotUsed(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	light := Ambient{Level: 1}
	for _, n := range []int{1, 2, 4} {
		m := blendTriangle(slices.Repeat([]engine.Color{pureRed}, n)...)
		m.Material.Color = pureBlue
		plain := m
		plain.VertexColors = nil

		got, want := newShadeBuffer(t), newShadeBuffer(t)
		m.DrawShaded(got, blendCam(), light)
		plain.DrawShaded(want, blendCam(), light)
		if picture(got) != picture(want) {
			t.Errorf("%d colors for 3 vertices: wanted them not to be used", n)
		}
		err := m.Validate()
		if err == nil || !strings.Contains(err.Error(), "vertex colors") {
			t.Errorf("%d colors for 3 vertices: wanted Validate to report them, got %v", n, err)
		}
	}
	if err := blendTriangle(pureRed, pureGreen, pureBlue).Validate(); err != nil {
		t.Errorf("wanted a valid mesh, got %v", err)
	}
	m := Mesh{Faces: [][3]int{{0, 1, 2}}, VertexColors: []engine.Color{pureRed}}
	if err := m.Validate(); err == nil {
		t.Errorf("wanted an error for a color and no vertices")
	}
	m.DrawShaded(newShadeBuffer(t), blendCam(), light) // and no panic
}

func TestCloneFitAndSmoothNormalsKeepVertexColors(t *testing.T) {
	m := Cube(2)
	m.VertexColors = []engine.Color{pureRed, pureGreen, pureBlue, pureYellow, darkGray, pureRed, pureGreen, pureBlue}

	clone := m.Clone()
	clone.VertexColors[0] = pureBlue
	if m.VertexColors[0] != pureRed {
		t.Errorf("wanted Clone to share nothing with the mesh")
	}
	if got := m.Fit(5).VertexColors; !slices.Equal(got, m.VertexColors) {
		t.Errorf("wanted Fit to keep the colors, got %v", got)
	}
	if got := m.SmoothNormals().VertexColors; !slices.Equal(got, m.VertexColors) {
		t.Errorf("wanted SmoothNormals to keep the colors, got %v", got)
	}
}

func TestWireframesIgnoreVertexColors(t *testing.T) {
	m := Cube(2)
	m.Transform.Position = geometry.Vector3{Z: 5}
	colored := m.Clone()
	colored.VertexColors = slices.Repeat([]engine.Color{pureRed}, len(m.Vertices))
	cell := engine.Cell{Character: '#', Foreground: pureGreen}

	for name, draw := range map[string]func(Mesh, *engine.FrameBuffer){
		"Draw":      func(m Mesh, fb *engine.FrameBuffer) { m.Draw(fb, shadeCam(), cell) },
		"DrawDepth": func(m Mesh, fb *engine.FrameBuffer) { m.DrawDepth(fb, shadeCam(), cell) },
	} {
		got, want := newShadeBuffer(t), newShadeBuffer(t)
		draw(colored, got)
		draw(m, want)
		if picture(got) != picture(want) {
			t.Errorf("%s: wanted a wireframe that ignores vertex colors", name)
		}
	}
}

func TestChannelRoundsAndClamps(t *testing.T) {
	for _, tt := range []struct {
		in   float64
		want uint8
	}{
		{0, 0}, {0.49, 0}, {0.5, 1}, {127.5, 128}, {254.4, 254}, {254.6, 255}, {255, 255},
		{300, 255}, {1e300, 255}, {-0.3, 0}, {-50, 0}, {math.Inf(1), 255}, {math.Inf(-1), 0}, {math.NaN(), 0},
	} {
		if got := channel(tt.in); got != tt.want {
			t.Errorf("channel(%v): wanted %d, got %d", tt.in, tt.want, got)
		}
	}
}

func TestColorAtPointOfATriangleWithNoAreaDoesNotDivideByZero(t *testing.T) {
	p := geometry.Vector3{X: 1, Y: 1, Z: 1}
	corners := [3]geometry.Vector3{{X: 10}, {Y: 20}, {Z: 30}}
	for _, tri := range [][3]geometry.Vector3{{p, p, p}, {p, p.Scale(2), p.Scale(3)}} {
		got := colorAtPoint(corners, tri[0], tri[1], tri[2], p)
		if math.IsNaN(got.X+got.Y+got.Z) || math.IsInf(got.X+got.Y+got.Z, 0) {
			t.Errorf("wanted a number, got %v", got)
		}
	}
}

func TestDrawingWithVertexColorsAllocatesNothingOnceWarm(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector makes sync.Pool drop items, so allocation counts mean nothing")
	}
	t.Setenv("TERM_PROGRAM", "")
	m := Sphere(1.6, 10, 6)
	m.Transform = Transform{Position: geometry.Vector3{Z: 4}}
	m.VertexColors = make([]engine.Color, len(m.Vertices))
	for i := range m.VertexColors {
		m.VertexColors[i] = engine.RGB(uint8(i*7), uint8(255-i*5), uint8(i*3))
	}
	flat := m.Clone()
	flat.Normals = nil
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)
	fb := newShadeBuffer(t)

	for name, draw := range map[string]func(){
		"smooth glyphs": func() { m.DrawShaded(fb, shadeCam(), light) },
		"smooth pixels": func() { m.DrawShadedPixels(fb.Canvas(), shadeCam(), light) },
		"flat glyphs":   func() { flat.DrawShaded(fb, shadeCam(), light) },
		"flat pixels":   func() { flat.DrawShadedPixels(fb.Canvas(), shadeCam(), light) },
	} {
		draw()
		if n := testing.AllocsPerRun(20, draw); n != 0 {
			t.Errorf("%s: wanted no allocation, got %v", name, n)
		}
	}
}

// colorFrom makes a color from the bytes at data[i:]: the default, a palette color or an
// RGB color, whatever the bytes say, and the default when they run out.
func colorFrom(data []byte, i int) engine.Color {
	at := func(k int) byte {
		if k < len(data) {
			return data[k]
		}
		return 0
	}
	b := at(4 * i)
	switch b % 3 {
	case 0:
		return engine.Color{}
	case 1:
		return engine.Palette(at(4*i + 1))
	}
	return engine.RGB(at(4*i+1), at(4*i+2), at(4*i+3))
}

// Whatever the corners, the colors and the camera, a face with vertex colors draws without
// a panic and the same way twice, a list of the wrong length is not used, and a list of
// default colors changes nothing.
func FuzzDrawVertexColors(f *testing.F) {
	tri := floatBytes(-3, -2, 3, 0, 3, 6, 3, -2, 9)
	f.Add(uint8(3), uint8(0), []byte{1, 2, 3, 4, 2, 5, 6, 7, 2, 8, 9, 10}, tri)
	f.Add(uint8(3), uint8(1|2|4), []byte{2, 255, 0, 0, 2, 0, 255, 0, 2, 0, 0, 255}, tri)
	f.Add(uint8(3), uint8(8), []byte{2, 255, 0, 0, 0, 0, 0, 0, 1, 7, 0, 0}, floatBytes(-3, -2, -3, 0, 3, 6, 3, -2, 9)) // cut
	f.Add(uint8(2), uint8(0), []byte{2, 1, 2, 3}, tri)
	f.Add(uint8(5), uint8(0), []byte{}, tri)
	f.Add(uint8(3), uint8(0), []byte{2, 1, 2, 3}, floatBytes(math.NaN(), 0, 0, 1, 1, 1, 0, 0, 0))
	f.Add(uint8(3), uint8(0), []byte{2, 1, 2, 3}, floatBytes(math.Inf(1), 0, 5, 1, 1, 7, 0, 0, 9))
	f.Add(uint8(3), uint8(0), []byte{2, 1, 2, 3}, floatBytes(1e300, 1e300, 1e300, -1e300, 5e-324, 0, 0, 1e-200, 4))

	f.Fuzz(func(t *testing.T, count, flags uint8, colorBytes, vertexData []byte) {
		t.Setenv("TERM_PROGRAM", "")
		fl := floatsFrom(vertexData, 9)
		m := Mesh{
			Vertices:    []geometry.Vector3{{X: fl[0], Y: fl[1], Z: fl[2]}, {X: fl[3], Y: fl[4], Z: fl[5]}, {X: fl[6], Y: fl[7], Z: fl[8]}},
			Faces:       [][3]int{{0, 1, 2}},
			DoubleSided: flags&1 != 0,
		}
		if flags&2 != 0 {
			m.Normals = []geometry.Vector3{{Z: -1}, {X: 1, Z: -1}, {Y: 1, Z: -1}}
		}
		if flags&4 != 0 {
			m.FaceColors = []engine.Color{colorFrom(colorBytes, 7)}
		}
		for i := range int(count % 6) {
			m.VertexColors = append(m.VertexColors, colorFrom(colorBytes, i))
		}
		cam := Camera{FOV: math.Pi / 2, Near: 0.1}
		if flags&8 != 0 {
			cam.Near = 4
		}
		light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)

		draw := func(m Mesh) (cells, pixels string) {
			done := make(chan [2]string, 1)
			go func() {
				a, b := newBufferOf(t), newBufferOf(t)
				m.DrawShaded(a, cam, light)
				canvas := b.Canvas()
				m.DrawShadedPixels(canvas, cam, light)
				canvas.Compose()
				done <- [2]string{picture(a), picture(b)}
			}()
			select {
			case p := <-done:
				return p[0], p[1]
			case <-time.After(3 * time.Second):
				t.Fatalf("wanted drawing to finish for %+v", m)
				return "", ""
			}
		}

		c1, p1 := draw(m)
		if c2, p2 := draw(m); c1 != c2 || p1 != p2 {
			t.Fatalf("wanted the same picture twice")
		}
		without := m
		without.VertexColors = nil
		wc, wp := draw(without)
		switch {
		case len(m.VertexColors) != 3:
			if c1 != wc || p1 != wp {
				t.Fatalf("wanted %d colors for 3 vertices not to be used", len(m.VertexColors))
			}
			if err := m.Validate(); len(m.VertexColors) != 0 && err == nil {
				t.Fatalf("wanted Validate to report %d colors for 3 vertices", len(m.VertexColors))
			}
		case slices.IndexFunc(m.VertexColors, func(c engine.Color) bool { return !c.IsDefault() }) < 0:
			if c1 != wc || p1 != wp {
				t.Fatalf("wanted a list of default colors to change nothing")
			}
		}
	})
}

func newBufferOf(t *testing.T) *engine.FrameBuffer {
	fb, err := engine.NewFrameBuffer(20, 8)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

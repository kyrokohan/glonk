package mesh

import (
	"math"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

func TestFarMakesACameraUsableOrNot(t *testing.T) {
	inf := math.Inf(1)
	tests := []struct {
		name string
		cam  Camera
		want bool
	}{
		{"no Far", Camera{}, true},
		{"an infinite Far is no limit", Camera{Far: inf}, true},
		{"a Far", Camera{Far: 5}, true},
		{"a Far just above the default Near", Camera{Far: 0.2}, true},
		{"a Far above a Near of its own", Camera{Near: 2, Far: 2.5}, true},
		{"a Far equal to the default Near", Camera{Far: 0.1}, false},
		{"a Far below the default Near", Camera{Far: 0.05}, false},
		{"a Far equal to the Near", Camera{Near: 2, Far: 2}, false},
		{"a Far below the Near", Camera{Near: 2, Far: 1}, false},
		{"a negative Far", Camera{Far: -1}, false},
		{"a NaN Far", Camera{Far: math.NaN()}, false},
		{"an infinite Near, which sees nothing", Camera{Near: inf, Far: inf}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := tt.cam.withDefaults(); ok != tt.want {
				t.Errorf("wanted usable=%v, got %v", tt.want, ok)
			}
		})
	}

	// 0 and +Inf are the same: no limit.
	if zero, _ := (Camera{}).withDefaults(); !math.IsInf(zero.Far, 1) {
		t.Errorf("wanted a Far of 0 to mean no limit, got %v", zero.Far)
	}
}

// A camera whose Far makes no sense sees nothing, as for the other fields.
func TestACameraWithANonsenseFarSeesNothing(t *testing.T) {
	wall := farWall(10)
	for _, far := range []float64{-1, math.NaN(), 0.05} {
		cam := Camera{Far: far}
		fb := newShadeBuffer(t)
		wall.DrawShaded(fb, cam, Ambient{Level: 1})
		if got := len(drawn(fb)); got != 0 {
			t.Errorf("Far %v: wanted nothing drawn, got %d cells", far, got)
		}
		if _, _, ok := cam.Project(geometry.Vector3{Z: 5}, 80, 24); ok {
			t.Errorf("Far %v: wanted Project not ok", far)
		}
	}
}

func TestProjectIsNotOkBeyondFar(t *testing.T) {
	cam := Camera{Far: 10}
	cv := pixelCanvas(t, 80, 24)
	for _, tt := range []struct {
		z    float64
		want bool
	}{{5, true}, {10, true}, {10.0001, false}, {50, false}} {
		p := geometry.Vector3{X: 1, Z: tt.z}
		if _, _, ok := cam.Project(p, 80, 24); ok != tt.want {
			t.Errorf("Project at depth %v: wanted ok=%v, got %v", tt.z, tt.want, ok)
		}
		if _, _, _, ok := cam.ProjectPixels(p, cv); ok != tt.want {
			t.Errorf("ProjectPixels at depth %v: wanted ok=%v, got %v", tt.z, tt.want, ok)
		}
	}

	// A ray has no length, so Far does not change it.
	_, with, _ := cam.Ray(40, 12, 80, 24)
	_, without, _ := Camera{}.Ray(40, 12, 80, 24)
	if with != without {
		t.Errorf("wanted the ray to be the same with and without a Far, got %v and %v", with, without)
	}
}

// farWall is a wall square to a camera at the origin looking down +z, dist away.
func farWall(dist float64) Mesh {
	return Mesh{
		Vertices:    []geometry.Vector3{{X: -4, Y: -4, Z: dist}, {X: 4, Y: -4, Z: dist}, {X: 4, Y: 4, Z: dist}, {X: -4, Y: 4, Z: dist}},
		Faces:       [][3]int{{0, 1, 2}, {0, 2, 3}},
		DoubleSided: true,
	}.withColor(engine.Palette(208))
}

// What is exactly Far away is drawn, and what is beyond it is not; a Far beyond the whole scene,
// or infinite, changes nothing.
func TestFarKeepsWhatIsExactlyAtItAndDropsWhatIsBeyond(t *testing.T) {
	wall := farWall(10)
	frames := func(far float64) (cells string, pixels int) {
		cam := Camera{Far: far}
		fb := newShadeBuffer(t)
		wall.DrawShaded(fb, cam, Ambient{Level: 1})
		pix := newShadeBuffer(t).Canvas()
		wall.DrawShadedPixels(pix, cam, Ambient{Level: 1})
		return fb.String(), len(pixelsDrawn(pix))
	}

	noneCells, nonePixels := frames(0)
	if noneCells == newShadeBuffer(t).String() || nonePixels == 0 {
		t.Fatalf("wanted the wall drawn with no Far")
	}
	for _, far := range []float64{10, 10.5, 1e9, math.Inf(1)} {
		if cells, pixels := frames(far); cells != noneCells || pixels != nonePixels {
			t.Errorf("Far %v: wanted the wall drawn as with no Far", far)
		}
	}
	for _, far := range []float64{9.99, 5, 0.2} {
		if cells, pixels := frames(far); cells != newShadeBuffer(t).String() || pixels != 0 {
			t.Errorf("Far %v: wanted a wall beyond it not drawn, got %d pixels", far, pixels)
		}
	}
}

// floorDepth is how deep the floor (y = 0) is along the ray through the place (x, y), or
// +Inf where the ray does not meet the floor ahead of it. The direction of a ray is one deep,
// so the distance along it is the depth.
func floorDepth(ray func(x, y float64) (origin, direction geometry.Vector3, ok bool), x, y float64) float64 {
	origin, direction, ok := ray(x, y)
	if !ok || !(direction.Y < 0) {
		return math.Inf(1)
	}
	return -origin.Y / direction.Y
}

// A face that crosses the far plane is cut where it crosses it. A floor is seen from above, with
// the horizon in view; the ray through the corners of each cell (and pixel) says how deep the
// floor is there. A cell that is wholly nearer than Far is drawn as it is with no Far, whatever
// the shading, the normals or the vertex colors carried to the cut, and one wholly beyond it is
// empty.
func TestAFaceThatCrossesTheFarPlaneIsCutWhereItCrosses(t *testing.T) {
	const far = 12.0
	lights := []Light{
		Ambient{Level: 0.2},
		Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}},
		Point{Position: geometry.Vector3{X: 0, Y: 2, Z: 10}, Range: 40}, // brightness that depends on where
	}
	flat := Plane(400, 400)
	flat.Normals = nil
	smooth := Plane(400, 400)
	colors := []engine.Color{engine.RGB(255, 0, 0), engine.RGB(0, 255, 0), engine.RGB(0, 0, 255), engine.RGB(255, 255, 0)}
	tinted := Plane(400, 400)
	tinted.VertexColors = colors
	// A floor with every corner in front of the camera, so that only the far plane cuts it, and
	// not the near one as well: what carries the cut's colors and normals is then the far cut.
	ahead := Plane(60, 46)
	ahead.Transform.Position = geometry.Vector3{Z: 26}
	ahead.VertexColors = colors
	cases := []struct {
		name  string
		floor Mesh
	}{{"flat", flat}, {"smooth", smooth}, {"tinted", tinted}, {"tinted, wholly in front of the camera", ahead}}

	cam := Camera{Position: geometry.Vector3{Y: 3}, Pitch: -0.6}
	limited := cam
	limited.Far = far

	for _, tc := range cases {
		floor := tc.floor.withColor(engine.Palette(70))

		// Cells.
		whole, cut := newShadeBuffer(t), newShadeBuffer(t)
		floor.DrawShaded(whole, cam, lights...)
		floor.DrawShaded(cut, limited, lights...)
		ray := func(x, y float64) (geometry.Vector3, geometry.Vector3, bool) { return cam.Ray(x, y, shadeW, shadeH) }
		before, beyond, changed := 0, 0, 0
		for y := range shadeH {
			for x := range shadeW {
				depths := [4]float64{
					floorDepth(ray, float64(x), float64(y)), floorDepth(ray, float64(x+1), float64(y)),
					floorDepth(ray, float64(x), float64(y+1)), floorDepth(ray, float64(x+1), float64(y+1)),
				}
				if whole.At(x, y) != cut.At(x, y) {
					changed++
				}
				switch {
				case maxOf(depths) < far:
					before++
					if whole.At(x, y) != cut.At(x, y) {
						t.Fatalf("%s cells: wanted cell (%d, %d), wholly nearer than Far, drawn as with no Far", tc.name, x, y)
					}
				case minOf(depths) > far:
					beyond++
					if cut.At(x, y) != engine.BlankCell {
						t.Fatalf("%s cells: wanted cell (%d, %d), wholly beyond Far, empty, got %v", tc.name, x, y, cut.At(x, y))
					}
				}
			}
		}
		if before < 100 || beyond < 100 || changed < 100 {
			t.Errorf("%s cells: wanted cells before the cut, beyond it, and changed by it, got %d, %d and %d", tc.name, before, beyond, changed)
		}

		// Pixels.
		wholePix, cutPix := newShadeBuffer(t).Canvas(), newShadeBuffer(t).Canvas()
		floor.DrawShadedPixels(wholePix, cam, lights...)
		floor.DrawShadedPixels(cutPix, limited, lights...)
		rayPix := func(x, y float64) (geometry.Vector3, geometry.Vector3, bool) { return cam.RayPixels(x, y, wholePix) }
		before, beyond, changed = 0, 0, 0
		for y := range wholePix.Height() {
			for x := range wholePix.Width() {
				depths := [4]float64{
					floorDepth(rayPix, float64(x), float64(y)), floorDepth(rayPix, float64(x+1), float64(y)),
					floorDepth(rayPix, float64(x), float64(y+1)), floorDepth(rayPix, float64(x+1), float64(y+1)),
				}
				if wholePix.At(x, y) != cutPix.At(x, y) {
					changed++
				}
				switch {
				case maxOf(depths) < far:
					before++
					if wholePix.At(x, y) != cutPix.At(x, y) {
						t.Fatalf("%s pixels: wanted pixel (%d, %d), wholly nearer than Far, drawn as with no Far: %v and %v", tc.name, x, y, cutPix.At(x, y), wholePix.At(x, y))
					}
				case minOf(depths) > far:
					beyond++
					if !cutPix.At(x, y).IsDefault() {
						t.Fatalf("%s pixels: wanted pixel (%d, %d), wholly beyond Far, empty, got %v", tc.name, x, y, cutPix.At(x, y))
					}
				}
			}
		}
		if before < 200 || beyond < 200 || changed < 200 {
			t.Errorf("%s pixels: wanted pixels before the cut, beyond it, and changed by it, got %d, %d and %d", tc.name, before, beyond, changed)
		}
	}
}

func maxOf(vs [4]float64) float64 { return max(vs[0], vs[1], vs[2], vs[3]) }
func minOf(vs [4]float64) float64 { return min(vs[0], vs[1], vs[2], vs[3]) }

// A line out into the distance ends at the cell where the point at depth Far is, in every kind of
// wireframe; one wholly beyond Far is not drawn; one that starts behind the camera is cut at both
// planes.
func TestWireframesEndAtTheFarPlane(t *testing.T) {
	line := func(z0, z1 float64) Mesh {
		return Mesh{Vertices: []geometry.Vector3{{X: 3, Z: z0}, {X: 3, Z: z1}}, Edges: [][2]int{{0, 1}}}
	}
	// The leftmost column drawn, in cells or in pixels, or -1 for nothing.
	leftmost := func(m Mesh, far float64, kind string) int {
		cam := Camera{Far: far}
		fb := newShadeBuffer(t)
		cell := engine.Cell{Character: '#'}
		left := -1
		note := func(x int) {
			if left < 0 || x < left {
				left = x
			}
		}
		switch kind {
		case "Draw":
			m.Draw(fb, cam, cell)
		case "DrawDepth":
			m.DrawDepth(fb, cam, cell)
		case "DrawPixels":
			c := fb.Canvas()
			m.DrawPixels(c, cam, engine.RGB(255, 0, 0))
			for _, p := range pixelsDrawn(c) {
				note(p[0])
			}
			return left
		}
		for _, c := range drawn(fb) {
			note(c[0])
		}
		return left
	}

	atFar, _, _ := Camera{}.Project(geometry.Vector3{X: 3, Z: 20}, shadeW, shadeH)
	// Each line is tried from either end, since the cut is made on whichever end is beyond.
	for _, kind := range []string{"Draw", "DrawDepth", "DrawPixels"} {
		for _, ends := range [][2]float64{{5, 50}, {50, 5}} {
			if got, want := leftmost(line(ends[0], ends[1]), 20, kind), int(math.Floor(atFar)); got != want {
				t.Errorf("%s %v: wanted the line to end in column %d, where the point 20 deep is, got %d", kind, ends, want, got)
			}
			if got, want := leftmost(line(ends[0], ends[1]), 0, kind), int(math.Floor(40+72.0/50)); got != want {
				t.Errorf("%s %v: wanted the line with no Far to run on to column %d, got %d", kind, ends, want, got)
			}
		}
		for _, ends := range [][2]float64{{25, 50}, {50, 25}} {
			if got := leftmost(line(ends[0], ends[1]), 20, kind); got != -1 {
				t.Errorf("%s %v: wanted a line wholly beyond Far not drawn, got column %d", kind, ends, got)
			}
		}
		for _, ends := range [][2]float64{{-5, 50}, {50, -5}} {
			if got, want := leftmost(line(ends[0], ends[1]), 20, kind), int(math.Floor(atFar)); got != want {
				t.Errorf("%s %v: wanted a line from behind the camera cut at both planes, ending in column %d, got %d", kind, ends, want, got)
			}
		}
	}
}

// A mesh wholly beyond the far plane is skipped without being looked at, and drawing it with
// culling off gives the same, empty, picture.
func TestAMeshWhollyBeyondTheFarPlaneIsSkipped(t *testing.T) {
	cube := func(z float64) Mesh {
		c := Cube(2).withColor(engine.Palette(208))
		c.Transform.Position = geometry.Vector3{Z: z}
		return c
	}
	cam := Camera{Far: 10}
	for _, tt := range []struct {
		name string
		z    float64
		cam  Camera
		want bool
	}{
		{"beyond", 30, cam, true},
		{"straddling the far plane", 10.5, cam, false},
		{"in front of it", 5, cam, false},
		{"beyond, with no Far", 30, Camera{}, false},
	} {
		if got := offscreenFor(cube(tt.z), tt.cam, 80, 24); got != tt.want {
			t.Errorf("%s: wanted skipped=%v, got %v", tt.name, tt.want, got)
		}
	}

	draw := func() string {
		fb := newShadeBuffer(t)
		cube(30).DrawShaded(fb, cam, Ambient{Level: 1})
		return fb.String()
	}
	skipping := draw()
	cullingOn = false
	defer func() { cullingOn = true }()
	if without := draw(); without != skipping || without != newShadeBuffer(t).String() {
		t.Errorf("wanted a cube beyond Far to draw nothing with or without skipping")
	}
}

package mesh

import (
	"math"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

type cameraOutput struct {
	x, y float64
	ok   bool
}

func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestCameraProject(t *testing.T) {
	var tests = []struct {
		name  string
		yaw   float64
		point geometry.Vector3
		want  cameraOutput
	}{
		{"straight ahead", 0.00, geometry.Vector3{X: 0, Y: 0, Z: 5}, cameraOutput{40, 12, true}},
		{"behind", 0.00, geometry.Vector3{X: 0, Y: 0, Z: -5}, cameraOutput{0, 0, false}},
		{"right", 0.00, geometry.Vector3{X: 1, Y: 0, Z: 2}, cameraOutput{52, 12, true}},
		{"up", 0.00, geometry.Vector3{X: 0, Y: 1, Z: 2}, cameraOutput{40, 6, true}},
		{"turned", math.Pi / 2, geometry.Vector3{X: 5, Y: 0, Z: 0}, cameraOutput{40, 12, true}},
		{"inside near plane", 0.00, geometry.Vector3{X: 0, Y: 0, Z: 0.05}, cameraOutput{0, 0, false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Camera{
				Position: geometry.Vector3{X: 0, Y: 0, Z: 0},
				FOV:      math.Pi / 2,
				Yaw:      tt.yaw,
				Near:     0.1,
			}

			x, y, ok := c.Project(tt.point, 80, 24)
			if ok != tt.want.ok {
				t.Fatalf("wanted ok=%v, got %v", tt.want.ok, ok)
			}
			if !ok {
				return
			}

			if !floatEquals(x, tt.want.x) {
				t.Errorf("wanted x=%v, got %v", tt.want.x, x)
			}

			if !floatEquals(y, tt.want.y) {
				t.Errorf("wanted y=%v, got %v", tt.want.y, y)
			}
		})
	}
}

func vecEquals(a, b geometry.Vector3) bool {
	return floatEquals(a.X, b.X) && floatEquals(a.Y, b.Y) && floatEquals(a.Z, b.Z)
}

func TestCameraForwardRight(t *testing.T) {
	tests := []struct {
		name           string
		yaw            float64
		forward, right geometry.Vector3
	}{
		{"yaw 0", 0, geometry.Vector3{Z: 1}, geometry.Vector3{X: 1}},
		{"yaw quarter turn", math.Pi / 2, geometry.Vector3{X: 1}, geometry.Vector3{Z: -1}},
		{"yaw half turn", math.Pi, geometry.Vector3{Z: -1}, geometry.Vector3{X: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam := Camera{Yaw: tt.yaw}

			if got := cam.Forward(); !vecEquals(got, tt.forward) {
				t.Errorf("wanted Forward()=%v, got %v", tt.forward, got)
			}
			if got := cam.Right(); !vecEquals(got, tt.right) {
				t.Errorf("wanted Right()=%v, got %v", tt.right, got)
			}
		})
	}
}

// Forward and Right must agree with Project, or WASD moves the wrong way.
func TestCameraForwardRightMatchProject(t *testing.T) {
	const w, h = 80, 24

	for _, yaw := range []float64{0, 0.7, math.Pi / 2, 2.5, -1.2} {
		cam := Camera{
			Position: geometry.Vector3{X: 3, Y: 1, Z: -2},
			Yaw:      yaw,
			FOV:      math.Pi / 2,
			Near:     0.1,
		}
		ahead := cam.Position.Add(cam.Forward().Scale(5))

		x, y, ok := cam.Project(ahead, w, h)
		if !ok || !floatEquals(x, w/2) || !floatEquals(y, h/2) {
			t.Errorf("yaw %v: wanted point ahead at center (%v,%v,true), got (%v,%v,%v)", yaw, w/2, h/2, x, y, ok)
		}

		x, _, ok = cam.Project(ahead.Add(cam.Right()), w, h)
		if !ok || x <= w/2 {
			t.Errorf("yaw %v: wanted point to the right at x>%v, got x=%v ok=%v", yaw, w/2, x, ok)
		}
	}
}

func TestCameraPitchProject(t *testing.T) {
	// A point straight ahead on a camera tilted 45 degrees lands on the edge of the
	// screen: its view-space y over z is exactly -+1, which is a half screen away.
	tests := []struct {
		name  string
		pitch float64
		wantY float64
	}{
		{"level", 0, 12},
		{"tilted up, the point falls to the bottom edge", math.Pi / 4, 24},
		{"tilted down, the point rises to the top edge", -math.Pi / 4, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam := Camera{Pitch: tt.pitch, FOV: math.Pi / 2, Near: 0.1}

			x, y, ok := cam.Project(geometry.Vector3{Z: 5}, 80, 24)
			if !ok {
				t.Fatalf("wanted the point in view")
			}
			if !floatEquals(x, 40) || !floatEquals(y, tt.wantY) {
				t.Errorf("wanted (40,%v), got (%v,%v)", tt.wantY, x, y)
			}
		})
	}
}

// Tilting turns the world about the camera's own sideways axis, so sideways
// distances in view space must not change.
func TestCameraPitchLeavesViewXAlone(t *testing.T) {
	p := geometry.Vector3{X: 2, Y: 1, Z: 4}

	for _, pitch := range []float64{-1.2, -0.4, 0.6, 1.3} {
		level := Camera{}
		tilted := Camera{Pitch: pitch}

		if got, want := tilted.toView(p).X, level.toView(p).X; !floatEquals(got, want) {
			t.Errorf("pitch %v: wanted view x=%v, got %v", pitch, want, got)
		}
		if got, want := tilted.toView(p).Length(), level.toView(p).Length(); !floatEquals(got, want) {
			t.Errorf("pitch %v: wanted tilting to keep distances, got %v vs %v", pitch, got, want)
		}
	}
}

func TestCameraLook(t *testing.T) {
	tests := []struct {
		name       string
		yaw, pitch float64
		want       geometry.Vector3
	}{
		{"level", 0, 0, geometry.Vector3{Z: 1}},
		{"turned right", math.Pi / 2, 0, geometry.Vector3{X: 1}},
		{"straight up", 0, math.Pi / 2, geometry.Vector3{Y: 1}},
		{"straight down", 0, -math.Pi / 2, geometry.Vector3{Y: -1}},
		{"up and turned", math.Pi / 2, math.Pi / 4, geometry.Vector3{X: math.Sqrt2 / 2, Y: math.Sqrt2 / 2}},
		{"down and half turned", math.Pi, -math.Pi / 4, geometry.Vector3{Y: -math.Sqrt2 / 2, Z: -math.Sqrt2 / 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam := Camera{Yaw: tt.yaw, Pitch: tt.pitch}

			got := cam.Look()
			if !vecEquals(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
			if !floatEquals(got.Length(), 1) {
				t.Errorf("wanted a unit vector, got length %v", got.Length())
			}
		})
	}
}

func TestCameraForwardAndRightIgnorePitch(t *testing.T) {
	for _, pitch := range []float64{-1.2, -0.3, 0.5, 1.4} {
		level := Camera{Yaw: 0.9}
		tilted := Camera{Yaw: 0.9, Pitch: pitch}

		if tilted.Forward() != level.Forward() {
			t.Errorf("pitch %v: wanted Forward unchanged, got %v vs %v", pitch, tilted.Forward(), level.Forward())
		}
		if tilted.Right() != level.Right() {
			t.Errorf("pitch %v: wanted Right unchanged, got %v vs %v", pitch, tilted.Right(), level.Right())
		}
		if tilted.Forward().Y != 0 {
			t.Errorf("pitch %v: wanted Forward flat, got y=%v", pitch, tilted.Forward().Y)
		}
	}
}

// Look, Right and Project must agree for every posture, or the world tilts the
// wrong way when the player looks around.
func TestCameraLookMatchesProject(t *testing.T) {
	const w, h = 80, 24

	for _, yaw := range []float64{0, 0.7, math.Pi / 2, 2.5, -1.2} {
		for _, pitch := range []float64{-1.2, -0.5, 0, 0.4, 1.1} {
			cam := Camera{
				Position: geometry.Vector3{X: 3, Y: 1, Z: -2},
				Yaw:      yaw,
				Pitch:    pitch,
				FOV:      math.Pi / 2,
				Near:     0.1,
			}
			ahead := cam.Position.Add(cam.Look().Scale(5))

			x, y, ok := cam.Project(ahead, w, h)
			if !ok || !floatEquals(x, w/2) || !floatEquals(y, h/2) {
				t.Errorf("yaw %v pitch %v: wanted the look point at the center, got (%v,%v,%v)", yaw, pitch, x, y, ok)
			}

			x, _, ok = cam.Project(ahead.Add(cam.Right()), w, h)
			if !ok || x <= w/2 {
				t.Errorf("yaw %v pitch %v: wanted a point to the right at x>%v, got %v", yaw, pitch, w/2, x)
			}

			// World up stays up on screen however the camera is tilted (within +-pi/2).
			_, y, ok = cam.Project(ahead.Add(geometry.Vector3{Y: 0.5}), w, h)
			if !ok || y >= h/2 {
				t.Errorf("yaw %v pitch %v: wanted a point above at y<%v, got %v", yaw, pitch, h/2, y)
			}
		}
	}
}

func TestCellAspectScalesTheWidthOfThePicture(t *testing.T) {
	p := geometry.Vector3{X: 1, Y: 0.5, Z: 5}
	x2, y2, ok2 := Camera{CellAspect: 2}.Project(p, 80, 24)
	x3, y3, ok3 := Camera{CellAspect: 3}.Project(p, 80, 24)
	x1, y1, ok1 := Camera{CellAspect: 1}.Project(p, 80, 24)
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("wanted the point seen, got %v %v %v", ok1, ok2, ok3)
	}

	// The distance from the middle column is in proportion to the aspect, and rows
	// do not depend on it.
	if !floatEquals((x3-40)/(x2-40), 1.5) || !floatEquals((x1-40)/(x2-40), 0.5) {
		t.Errorf("wanted the offsets from the middle to be in proportion to CellAspect, got %v, %v, %v", x1-40, x2-40, x3-40)
	}
	if y1 != y2 || y2 != y3 {
		t.Errorf("wanted the row unchanged by CellAspect, got %v %v %v", y1, y2, y3)
	}
}

func TestCellAspectZeroIsTwo(t *testing.T) {
	p := geometry.Vector3{X: 1.5, Y: -0.5, Z: 4}
	zx, zy, zok := Camera{}.Project(p, 60, 20)
	tx, ty, tok := Camera{CellAspect: 2}.Project(p, 60, 20)

	if zx != tx || zy != ty || zok != tok {
		t.Errorf("wanted a CellAspect of 0 to project like 2, got (%v, %v, %v) and (%v, %v, %v)", zx, zy, zok, tx, ty, tok)
	}
}

// Other checks would catch most of these further along, but the camera itself says
// which values it cannot use.
func TestWithDefaultsRejectsNonsenseCellAspect(t *testing.T) {
	for name, aspect := range map[string]float64{
		"negative":          -2,
		"negative infinity": math.Inf(-1),
		"infinity":          math.Inf(1),
		"NaN":               math.NaN(),
	} {
		if _, ok := (Camera{CellAspect: aspect}).withDefaults(); ok {
			t.Errorf("%s: wanted the camera unusable", name)
		}
	}
}

// pixelCanvas returns the canvas of a w by h frame, which is w pixels wide and 2h tall.
func pixelCanvas(t *testing.T, w, h int) *engine.Canvas {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb.Canvas()
}

// ProjectPixels is Project on the grid the canvas belongs to, with y stretched to pixel rows,
// and its depth is how far p is along Look. The depth is worked out here from the camera's
// own vector, not from the code under test.
func TestProjectPixelsMatchesProjectAndLook(t *testing.T) {
	cams := []Camera{
		{},
		{Position: geometry.Vector3{X: 1, Y: 2, Z: -3}, Yaw: 0.7},
		{Yaw: -2, Pitch: 0.4, FOV: 1},
		{Position: geometry.Vector3{X: -4, Y: 1, Z: 2}, Yaw: 3, Pitch: -0.9, FOV: 2, Near: 0.5, CellAspect: 1.5},
	}
	points := []geometry.Vector3{
		{Z: 5}, {X: 1, Y: -2, Z: 7}, {X: -6, Y: 3, Z: -1}, {X: 3, Y: 0.5, Z: 12}, {Y: 40, Z: 2}, {X: -2, Y: -1, Z: 3},
	}

	for _, cam := range cams {
		for _, size := range [][2]int{{80, 24}, {1, 1}, {30, 11}} {
			cv := pixelCanvas(t, size[0], size[1])
			projected := 0
			for _, p := range points {
				x, y, ok := cam.Project(p, cv.Width(), cv.Height()/2)
				px, py, depth, pixOK := cam.ProjectPixels(p, cv)

				if ok != pixOK {
					t.Errorf("cam %+v, p %v, %dx%d: wanted ok=%v as Project, got %v", cam, p, size[0], size[1], ok, pixOK)
					continue
				}
				if !ok {
					continue
				}
				projected++
				if !floatEquals(px, x) || !floatEquals(py, 2*y) {
					t.Errorf("cam %+v, p %v, %dx%d: wanted (%v, %v) from Project with y doubled, got (%v, %v)", cam, p, size[0], size[1], x, 2*y, px, py)
				}
				if want := p.Sub(cam.Position).Dot(cam.Look()); !floatEquals(depth, want) {
					t.Errorf("cam %+v, p %v: wanted depth %v along Look, got %v", cam, p, want, depth)
				}
			}
			if projected == 0 {
				t.Errorf("cam %+v: wanted some point to be projected, got none", cam)
			}
		}
	}
}

func TestProjectPixelsIsNotOkWhereProjectIsNot(t *testing.T) {
	cv := pixelCanvas(t, 40, 12)
	tests := []struct {
		name string
		cam  Camera
		p    geometry.Vector3
		cv   *engine.Canvas
	}{
		{"behind the camera", Camera{}, geometry.Vector3{Z: -5}, cv},
		{"nearer than Near", Camera{Near: 1}, geometry.Vector3{Z: 0.5}, cv},
		{"a camera that sees nothing", Camera{FOV: 4}, geometry.Vector3{Z: 5}, cv},
		{"a NaN point", Camera{}, geometry.Vector3{X: math.NaN(), Z: 5}, cv},
		{"an infinite point", Camera{}, geometry.Vector3{X: math.Inf(1), Z: 5}, cv},
		{"no canvas", Camera{}, geometry.Vector3{Z: 5}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, depth, ok := tt.cam.ProjectPixels(tt.p, tt.cv)

			if ok || x != 0 || y != 0 || depth != 0 {
				t.Errorf("wanted (0, 0, 0, false), got (%v, %v, %v, %v)", x, y, depth, ok)
			}
		})
	}
}

// A point off the canvas is still projected, as Project does; SetDepth then ignores it.
func TestProjectPixelsKeepsAPointOffTheCanvas(t *testing.T) {
	cv := pixelCanvas(t, 40, 12)

	x, y, _, ok := Camera{}.ProjectPixels(geometry.Vector3{X: 50, Y: 30, Z: 2}, cv)

	if !ok || x < float64(cv.Width()) || y >= 0 {
		t.Errorf("wanted a place off the right and top of the canvas, got (%v, %v, %v)", x, y, ok)
	}
}

// The corners the shaded drawing hands to the fill, in pixels and with their depth, are what
// ProjectPixels says for the mesh's vertices.
func TestProjectPixelsGivesTheCornersTheMeshDraws(t *testing.T) {
	cv := pixelCanvas(t, 40, 12)
	cam := Camera{Position: geometry.Vector3{X: -1, Y: 1}, Yaw: 0.2, Pitch: 0.1}
	m := solidCube(geometry.Vector3{X: 1, Z: 6}, 2)

	type corner struct{ x, y, depth float64 }
	var want []corner
	for _, v := range m.Vertices {
		x, y, depth, ok := cam.ProjectPixels(m.Transform.Apply(v), cv)
		if !ok {
			t.Fatalf("wanted every vertex of the cube in view, got none for %v", v)
		}
		want = append(want, corner{x, y, depth})
	}

	seen := 0
	m.eachShadedTriangle(cam, nil, cv.Width(), cv.Height()/2, func(tri *shadedTriangle) {
		for _, c := range tri.corners {
			seen++
			found := false
			for _, w := range want {
				if floatEquals(c.x, w.x) && floatEquals(2*c.y, w.y) && floatEquals(c.z, w.depth) {
					found = true
				}
			}
			if !found {
				t.Errorf("wanted the corner (%v, %v, %v) to be a vertex ProjectPixels gives, got none", c.x, 2*c.y, c.z)
			}
		}
	})
	if seen == 0 {
		t.Fatalf("wanted the cube to be drawn, got no triangles")
	}
}

// A marker drawn with what ProjectPixels returns lands where the mesh is, and is hidden by it
// or shows in front of it, to the pixel. The wall is square to the view direction, so every
// point of it is the same distance along Look, at any yaw and pitch.
func TestProjectPixelsPlacesAMarkerLikeTheMesh(t *testing.T) {
	cams := []Camera{
		{},
		{Position: geometry.Vector3{X: 3, Y: -2, Z: 5}, Yaw: 0.9},
		{Position: geometry.Vector3{X: -2, Y: 1, Z: -1}, Yaw: -1.1, Pitch: 0.5},
		{Yaw: 2.5, Pitch: -0.7},
	}
	const dist = 6.0
	marker := engine.RGB(0, 255, 0)

	for _, cam := range cams {
		right := geometry.Vector3{X: 1}.RotateX(-cam.Pitch).RotateY(cam.Yaw)
		up := geometry.Vector3{Y: 1}.RotateX(-cam.Pitch).RotateY(cam.Yaw)
		centre := cam.Position.Add(cam.Look().Scale(dist))
		at := func(a, b float64) geometry.Vector3 { return centre.Add(right.Scale(a)).Add(up.Scale(b)) }
		wall := Mesh{
			Vertices:    []geometry.Vector3{at(-2, -2), at(2, -2), at(2, 2), at(-2, 2)},
			Faces:       [][3]int{{0, 1, 2}, {0, 2, 3}},
			DoubleSided: true,
		}.withColor(engine.RGB(200, 100, 50))

		cv := pixelCanvas(t, 40, 12)
		wall.DrawShadedPixels(cv, cam, Ambient{Level: 1})

		pixelOf := func(p geometry.Vector3) (ix, iy int, depth float64) {
			x, y, depth, ok := cam.ProjectPixels(p, cv)
			if !ok {
				t.Fatalf("cam %+v: wanted %v in view", cam, p)
			}
			return int(math.Floor(x)), int(math.Floor(y)), depth
		}

		// On the wall: the pixel is covered, and a marker just behind it is hidden while one
		// just in front is drawn.
		for _, ab := range [][2]float64{{0, 0}, {1.5, 1.5}, {-1.5, -1.5}, {1.5, -1.5}, {0, 1.5}} {
			ix, iy, depth := pixelOf(at(ab[0], ab[1]))
			if cv.At(ix, iy).IsDefault() {
				t.Errorf("cam %+v, point %v: wanted the wall at pixel (%d, %d)", cam, ab, ix, iy)
				continue
			}
			if !floatEquals(depth, dist) {
				t.Errorf("cam %+v, point %v: wanted depth %v, got %v", cam, ab, dist, depth)
			}
			if cv.SetDepth(ix, iy, depth+0.01, marker) {
				t.Errorf("cam %+v, point %v: wanted a marker just behind the wall to be hidden", cam, ab)
			}
			if !cv.SetDepth(ix, iy, depth-0.01, marker) || cv.At(ix, iy) != marker {
				t.Errorf("cam %+v, point %v: wanted a marker just in front of the wall to be drawn", cam, ab)
			}
		}

		// Beside the wall: the pixel is empty.
		for _, ab := range [][2]float64{{3, 0}, {0, 3}, {-3, -3}} {
			ix, iy, _ := pixelOf(at(ab[0], ab[1]))
			if !cv.At(ix, iy).IsDefault() {
				t.Errorf("cam %+v, point %v: wanted no wall at pixel (%d, %d)", cam, ab, ix, iy)
			}
		}
	}
}

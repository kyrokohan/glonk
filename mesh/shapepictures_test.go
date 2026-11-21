package mesh

import (
	"math"
	"slices"
	"testing"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// Every shape has a triple of coordinates for each face, all of them finite numbers.
func TestShapesHaveAPictureCoordinateForEveryFace(t *testing.T) {
	for _, sh := range testShapes() {
		if len(sh.m.FaceTexCoords) != len(sh.m.Faces) {
			t.Errorf("%s: wanted a triple for each of %d faces, got %d", sh.name, len(sh.m.Faces), len(sh.m.FaceTexCoords))
			continue
		}
		for i, tri := range sh.m.FaceTexCoords {
			for _, c := range tri {
				if !finite(c.X, c.Y) {
					t.Fatalf("%s face %d: wanted finite coordinates, got %v", sh.name, i, tri)
				}
			}
		}
	}
}

// pictured returns m with the picture of four quarters on it and a white color for faces without
// a picture.
func pictured(m Mesh) Mesh {
	m.Material.Texture = quarters()
	m.Material.Color = engine.RGB(255, 255, 255)
	return m
}

// sample is a place right and up of the point a camera looks at, and the color wanted there.
type sample struct {
	right, up float64
	color     engine.Color
}

// Seen from in front, every round shape and the plane show the picture the right way up and not
// mirrored: red at the top left, green at the top right, blue and yellow below. The plane is seen
// from above, as a box's top is. The cone is looked at twice: near its tip, which is the top of
// the picture, and near its base, which is the bottom.
func TestShapePicturesAreTheRightWayUp(t *testing.T) {
	front, up := v3(0, 0, 1), v3(0, 1, 0)
	quarters := []sample{{-0.3, 0.3, boxRed}, {0.3, 0.3, boxGreen}, {-0.3, -0.3, boxBlue}, {0.3, -0.3, boxYellow}}
	for _, tt := range []struct {
		name    string
		m       Mesh
		look, n geometry.Vector3 // the point looked at and the way the camera is from it
		yaw     float64
		samples []sample
	}{
		{"sphere", Sphere(1, 24, 12), front, front, 0, quarters},
		{"sphere, 7 segments", Sphere(1, 7, 6), front, front, 0, quarters},
		{"cylinder", Cylinder(1, 2, 24), front, front, 0, quarters},
		// The cone is 1 wide at its base (y = -1) and comes to a point at y = 1, so at y = 0.4 it
		// is 0.3 from the axis, 30% of the way down the picture, and at y = -0.5 it is 0.75, 75%.
		{"cone near the tip", Cone(1, 2, 24), v3(0, 0.4, 0.3), front, 0, []sample{{-0.12, 0, boxRed}, {0.12, 0, boxGreen}}},
		{"cone near the base", Cone(1, 2, 24), v3(0, -0.5, 0.75), front, 0, []sample{{-0.3, 0, boxBlue}, {0.3, 0, boxYellow}}},
		{"plane", Plane(2, 2), v3(0, 0, 0), up, math.Pi, quarters},
	} {
		t.Run(tt.name, func(t *testing.T) {
			at := seenPoint(t, pictured(tt.m), tt.look, tt.n, tt.yaw)
			for _, w := range tt.samples {
				if got := at(w.right, w.up); got != w.color {
					t.Errorf("at %v right and %v up of the point looked at: wanted %v, got %v", w.right, w.up, w.color, got)
				}
			}
		})
	}
}

// seenPoint draws m from a camera 6 away from the point look along n, turned first to yaw and then
// pointed at it, and returns the color at a place a part of the way right and up of it, as the
// camera sees right and up.
func seenPoint(t *testing.T, m Mesh, look, n geometry.Vector3, yaw float64) func(right, up float64) engine.Color {
	t.Helper()
	fb, err := engine.NewFrameBuffer(80, 40)
	if err != nil {
		t.Fatal(err)
	}
	cv := fb.Canvas()
	cam := Camera{Position: look.Add(n.Scale(6)), Yaw: yaw}
	cam.LookAt(look)
	m.DrawShadedPixels(cv, cam, Ambient{Level: 1})
	return func(right, up float64) engine.Color {
		p := look.Add(cam.Right().Scale(right)).Add(cam.Up().Scale(up))
		x, y, _, ok := cam.ProjectPixels(p, cv)
		if !ok {
			t.Fatalf("the point %v is not on the screen", p)
		}
		return cv.At(int(math.Floor(x)), int(math.Floor(y)))
	}
}

// From behind, the picture's two edges meet in the middle of the back with nothing between: its
// left edge, red, to the right, and its right edge, green, to the left, also where the seam falls
// inside a face.
func TestSpherePicturesMeetAtTheBack(t *testing.T) {
	for _, segments := range []int{24, 7} {
		back := v3(0, 0, -1)
		at := seenPoint(t, pictured(Sphere(1, segments, 12)), back, back, 0)
		for _, w := range []struct {
			right float64
			color engine.Color
		}{{0.02, boxRed}, {0.15, boxRed}, {-0.02, boxGreen}, {-0.15, boxGreen}} {
			if got := at(w.right, 0.2); got != w.color {
				t.Errorf("%d segments, %v right of the back: wanted %v, got %v", segments, w.right, w.color, got)
			}
		}
	}
}

// Around a pole, every pixel is the top row of the picture: red or green, never smeared down into
// blue or yellow.
func TestSpherePoleShowsTheTopOfThePicture(t *testing.T) {
	m := pictured(Sphere(1, 24, 12))
	top := v3(0, 1, 0)
	at := seenPoint(t, m, top, top, math.Pi)
	for _, r := range []float64{-0.2, 0, 0.2} {
		for _, u := range []float64{-0.2, 0, 0.2} {
			if got := at(r, u); got != boxRed && got != boxGreen {
				t.Errorf("at %v right and %v up of the pole: wanted the top of the picture, got %v", r, u, got)
			}
		}
	}
}

// The cylinder's caps and the cone's base show no picture: they are drawn in Material.Color.
func TestCapsShowNoPicture(t *testing.T) {
	white := engine.RGB(255, 255, 255)
	for _, tt := range []struct {
		name string
		m    Mesh
		n    geometry.Vector3
	}{
		{"cylinder top", Cylinder(1, 2, 24), v3(0, 1, 0)},
		{"cylinder bottom", Cylinder(1, 2, 24), v3(0, -1, 0)},
		{"cone base", Cone(1, 2, 24), v3(0, -1, 0)},
	} {
		at := seenPoint(t, pictured(tt.m), tt.n, tt.n, 0)
		if got := at(0.2, 0.2); got != white {
			t.Errorf("%s: wanted the color of the material, %v, got %v", tt.name, white, got)
		}
	}
}

// A shape with no picture is drawn as it was before it had coordinates.
func TestShapesWithoutAPictureDrawAsBefore(t *testing.T) {
	for _, sh := range testShapes() {
		draw := func(m Mesh) ([]engine.Color, string) {
			fb, err := engine.NewFrameBuffer(40, 20)
			if err != nil {
				t.Fatal(err)
			}
			m.Transform = Transform{Position: v3(0, 0, 8), Yaw: 0.7, Pitch: 0.4}
			m.Material.Color = engine.RGB(200, 120, 40)
			m.DrawShaded(fb, Camera{}, Ambient{Level: 0.3}, Directional{Direction: v3(1, -1, 1), Intensity: 0.8})
			cv := fb.Canvas()
			m.DrawShadedPixels(cv, Camera{}, Ambient{Level: 0.3}, Directional{Direction: v3(1, -1, 1), Intensity: 0.8})
			var pixels []engine.Color
			for y := range cv.Height() {
				for x := range cv.Width() {
					pixels = append(pixels, cv.At(x, y))
				}
			}
			return pixels, fb.String()
		}
		without := sh.m.Clone()
		without.FaceTexCoords = nil
		p1, c1 := draw(sh.m)
		p2, c2 := draw(without)
		if !slices.Equal(p1, p2) || c1 != c2 {
			t.Errorf("%s: wanted the same drawing with and without coordinates when there is no picture", sh.name)
		}
	}
}

// Just off a pole of the sphere, and just under the tip of the cone, a point in the middle of a
// slice shows the middle of that slice's part of the picture, so the picture does not fan out round
// them. The picture has four columns for each slice, red at the slice's edges and blue in its
// middle: a pole or a tip at the edge of each slice would show red there.
func TestPolesAndTheTipTakeTheMiddleOfEachSlice(t *testing.T) {
	const segments = 24
	columns := bitmap.New(4*segments, 1)
	for x := range 4 * segments {
		c := boxRed
		if x%4 == 1 || x%4 == 2 {
			c = boxBlue
		}
		columns.Set(x, 0, c)
	}
	for _, tt := range []struct {
		name  string
		m     Mesh
		point func(theta float64) geometry.Vector3 // a point on the surface near the pole or the tip
		from  geometry.Vector3
	}{
		{"sphere's top pole", Sphere(1, segments, 12), func(th float64) geometry.Vector3 {
			s, c := math.Sincos(math.Pi / 48)
			return v3(s*math.Cos(th), c, s*math.Sin(th))
		}, v3(0, 6, 0)},
		{"sphere's bottom pole", Sphere(1, segments, 12), func(th float64) geometry.Vector3 {
			s, c := math.Sincos(math.Pi / 48)
			return v3(s*math.Cos(th), -c, s*math.Sin(th))
		}, v3(0, -6, 0)},
		{"cone's tip", Cone(1, 2, segments), func(th float64) geometry.Vector3 {
			return v3(0.05*math.Cos(th), 0.9, 0.05*math.Sin(th)) // 5% of the way down from the tip
		}, v3(0, 4, 3)},
	} {
		m := tt.m
		m.Material.Texture = columns
		fb, err := engine.NewFrameBuffer(200, 100)
		if err != nil {
			t.Fatal(err)
		}
		cv := fb.Canvas()
		cam := Camera{Position: tt.from, FOV: 0.15}
		cam.LookAt(geometry.Vector3{Y: tt.point(0).Y}) // close in, so a slice near the pole is many pixels wide
		m.DrawShadedPixels(cv, cam, Ambient{Level: 1})
		checked := 0
		for s := range segments {
			p := tt.point(2 * math.Pi * (float64(s) + 0.5) / segments)
			if p.Sub(tt.from).Dot(p.Sub(v3(0, p.Y, 0))) > 0 && tt.name == "cone's tip" {
				continue // the far side of the cone, hidden
			}
			x, y, _, ok := cam.ProjectPixels(p, cv)
			if !ok {
				continue
			}
			checked++
			if got := cv.At(int(math.Floor(x)), int(math.Floor(y))); got != boxBlue {
				t.Errorf("%s, slice %d: wanted the middle of its part, blue, got %v", tt.name, s, got)
			}
		}
		if checked < 4 {
			t.Errorf("%s: wanted several slices checked, got %d", tt.name, checked)
		}
	}
}

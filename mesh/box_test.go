package mesh

import (
	"math"
	"reflect"
	"testing"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// boxFaceWays are the faces of a box in the order BoxFaces makes them, each with the way it faces
// and the yaw a camera is turned to before it looks at it: for the top and the bottom, which are
// seen straight down and straight up, that yaw is what puts the box's front (+z) at the bottom of
// the screen from above, and at its top from below.
var boxFaceWays = []struct {
	name string
	n    geometry.Vector3
	yaw  float64
}{
	{"front", v3(0, 0, 1), 0},
	{"back", v3(0, 0, -1), 0},
	{"left", v3(-1, 0, 0), 0},
	{"right", v3(1, 0, 0), 0},
	{"top", v3(0, 1, 0), math.Pi},
	{"bottom", v3(0, -1, 0), math.Pi},
}

var (
	boxRed    = engine.RGB(255, 0, 0)
	boxGreen  = engine.RGB(0, 255, 0)
	boxBlue   = engine.RGB(0, 0, 255)
	boxYellow = engine.RGB(255, 255, 0)
)

// quarters is a picture of 2 by 2 pixels: red and green over blue and yellow.
func quarters() *bitmap.Bitmap {
	p := bitmap.New(2, 2)
	p.Set(0, 0, boxRed)
	p.Set(1, 0, boxGreen)
	p.Set(0, 1, boxBlue)
	p.Set(1, 1, boxYellow)
	return p
}

// seenFace draws m, a box 2 on each side, from a camera outside the face that faces n, turned
// first to yaw and then pointed at the middle of the face. It returns the color of the face at the
// point a part of the way to the right of the middle and up, as the camera sees right and up.
func seenFace(t *testing.T, m Mesh, n geometry.Vector3, yaw float64) func(right, up float64) engine.Color {
	t.Helper()
	fb, err := engine.NewFrameBuffer(60, 30)
	if err != nil {
		t.Fatal(err)
	}
	cv := fb.Canvas()
	cam := Camera{Position: n.Scale(6), Yaw: yaw}
	cam.LookAt(n) // the middle of the face
	m.DrawShadedPixels(cv, cam, Ambient{Level: 1})
	return func(right, up float64) engine.Color {
		p := n.Add(cam.Right().Scale(right)).Add(cam.Up().Scale(up))
		x, y, _, ok := cam.ProjectPixels(p, cv)
		if !ok {
			t.Fatalf("the point %v is not on the screen", p)
		}
		return cv.At(int(math.Floor(x)), int(math.Floor(y)))
	}
}

// Seen from outside, through a camera, every face shows the picture the right way up and not
// mirrored: red at its top left, green at its top right, blue and yellow below. The top's picture
// has its bottom edge along the front, and the bottom's its top edge.
func TestBoxShowsThePictureTheRightWayUpOnEveryFace(t *testing.T) {
	m := Box(2, 2, 2)
	m.Material.Texture = quarters()
	for _, f := range boxFaceWays {
		t.Run(f.name, func(t *testing.T) {
			at := seenFace(t, m, f.n, f.yaw)
			for _, q := range []struct {
				right, up float64
				want      engine.Color
			}{{-0.5, 0.5, boxRed}, {0.5, 0.5, boxGreen}, {-0.5, -0.5, boxBlue}, {0.5, -0.5, boxYellow}} {
				if got := at(q.right, q.up); got != q.want {
					t.Errorf("at %v right and %v up of the middle: wanted %v, got %v", q.right, q.up, q.want, got)
				}
			}
		})
	}
}

// Each face shows its own part of an atlas of six colors, a part that is turned inside out shows
// the picture mirrored, and parts left zero are the whole picture.
func TestBoxFacesShowTheirParts(t *testing.T) {
	atlas := bitmap.New(3, 2)
	colors := []engine.Color{boxRed, boxGreen, boxBlue, boxYellow, engine.RGB(255, 0, 255), engine.RGB(0, 255, 255)}
	part := func(i int) geometry.Rect { return geometry.NewRect(float64(i%3)/3, float64(i/3)/2, 1.0/3, 0.5) }
	for i, c := range colors {
		atlas.Set(i%3, i/3, c)
	}
	m := BoxFaces(2, 2, 2, BoxParts{Front: part(0), Back: part(1), Left: part(2), Right: part(3), Top: part(4), Bottom: part(5)})
	m.Material.Texture = atlas
	for i, f := range boxFaceWays {
		if got := seenFace(t, m, f.n, f.yaw)(0, 0); got != colors[i] {
			t.Errorf("%s: wanted its part, %v, got %v", f.name, colors[i], got)
		}
	}

	mirrored := BoxFaces(2, 2, 2, BoxParts{Back: geometry.Rect{Min: geometry.Vector2{X: 1}, Max: geometry.Vector2{Y: 1}}})
	mirrored.Material.Texture = quarters()
	if got := seenFace(t, mirrored, v3(0, 0, -1), 0)(-0.5, 0.5); got != boxGreen {
		t.Errorf("wanted a part turned inside out to show the picture mirrored, green at the top left, got %v", got)
	}

	if !reflect.DeepEqual(BoxFaces(2, 3, 4, BoxParts{}), Box(2, 3, 4)) {
		t.Errorf("wanted parts left zero to make the box Box makes")
	}
}

// Face i is Vertices[4i:4i+4] and Faces[2i] and Faces[2i+1], in the order Front, Back, Left,
// Right, Top, Bottom, its corners bottom left, top left, top right and bottom right from outside.
func TestBoxLayoutIsTheDocumentedOne(t *testing.T) {
	m := Box(2, 4, 6)
	half := v3(1, 2, 3)
	for i, f := range boxFaceWays {
		if m.Faces[2*i] != [3]int{4 * i, 4*i + 1, 4*i + 2} || m.Faces[2*i+1] != [3]int{4 * i, 4*i + 2, 4*i + 3} {
			t.Errorf("%s: wanted the triangles of vertices %d to %d, got %v and %v", f.name, 4*i, 4*i+3, m.Faces[2*i], m.Faces[2*i+1])
		}
		for k := range 4 {
			v := m.Vertices[4*i+k]
			if m.Normals[4*i+k] != f.n {
				t.Errorf("%s corner %d: wanted the normal %v, got %v", f.name, k, f.n, m.Normals[4*i+k])
			}
			if got, want := v.Dot(f.n), math.Abs(half.Dot(f.n)); got != want {
				t.Errorf("%s corner %d at %v: wanted it on the face, %v out", f.name, k, v, want)
			}
		}
		if err := m.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

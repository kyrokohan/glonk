package mesh

import (
	"math"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

func TestFogApply(t *testing.T) {
	haze := engine.RGB(40, 40, 60)
	red := engine.RGB(200, 0, 0)
	inf := math.Inf(1)
	tests := []struct {
		name  string
		fog   Fog
		color engine.Color
		depth float64
		want  engine.Color
	}{
		{"before Start, nothing changes", Fog{haze, 10, 30}, red, 5, red},
		{"at Start, nothing changes", Fog{haze, 10, 30}, red, 10, red},
		{"halfway, an even mix", Fog{haze, 10, 30}, red, 20, engine.RGB(120, 20, 30)},
		{"a quarter of the way", Fog{haze, 10, 30}, red, 15, engine.RGB(160, 10, 15)},
		{"at End it is the fog's color", Fog{haze, 10, 30}, red, 30, haze},
		{"beyond End it is the fog's color", Fog{haze, 10, 30}, red, 500, haze},
		{"a negative depth is before Start", Fog{haze, 10, 30}, red, -4, red},
		{"a palette color before Start stays a palette color", Fog{haze, 10, 30}, engine.Palette(208), 5, engine.Palette(208)},
		{"a palette fog color is exactly itself at End", Fog{engine.Palette(236), 10, 30}, red, 40, engine.Palette(236)},
		{"a depth that is not a number does nothing", Fog{haze, 10, 30}, red, math.NaN(), red},
		{"an infinite depth is complete fog", Fog{haze, 10, 30}, red, inf, haze},
		{"a negative infinite depth does nothing", Fog{haze, 10, 30}, red, -inf, red},
		{"a Start that is not a number does nothing", Fog{haze, math.NaN(), 30}, red, 20, red},
		{"an End that is not a number does nothing", Fog{haze, 10, math.NaN()}, red, 20, red},
		{"an infinite End never gets going", Fog{haze, 10, inf}, red, 1e9, red},
		{"an infinite Start does nothing before End", Fog{haze, -inf, 30}, red, 20, red},
		{"an End equal to Start is a hard step: at it, nothing", Fog{haze, 10, 10}, red, 10, red},
		{"an End equal to Start is a hard step: beyond it, all", Fog{haze, 10, 10}, red, 10.001, haze},
		{"an End before Start is a hard step at Start: before", Fog{haze, 10, 5}, red, 7, red},
		{"an End before Start is a hard step at Start: beyond", Fog{haze, 10, 5}, red, 11, haze},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fog.Apply(tt.color, tt.depth); got != tt.want {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

// colorsNear says whether two colors are the same to a few levels of each channel.
func colorsNear(a, b engine.Color, tol int) bool {
	ar, ag, ab := a.RGB()
	br, bg, bb := b.RGB()
	near := func(x, y uint8) bool { return math.Abs(float64(x)-float64(y)) <= float64(tol) }
	return near(ar, br) && near(ag, bg) && near(ab, bb)
}

// fogFloors are the floors the fog is drawn on, as for the far plane: flat, smooth with a light
// that varies from place to place, tinted, and tinted with every corner in front of the camera.
func fogFloors() []struct {
	name  string
	floor Mesh
} {
	flat := Plane(400, 400)
	flat.Normals = nil
	colors := []engine.Color{engine.RGB(255, 0, 0), engine.RGB(0, 255, 0), engine.RGB(0, 0, 255), engine.RGB(255, 255, 0)}
	tinted := Plane(400, 400)
	tinted.VertexColors = colors
	ahead := Plane(60, 46)
	ahead.Transform.Position = geometry.Vector3{Z: 26}
	ahead.VertexColors = colors
	return []struct {
		name  string
		floor Mesh
	}{{"flat", flat}, {"smooth", Plane(400, 400)}, {"tinted", tinted}, {"tinted, wholly in front of the camera", ahead}}
}

var fogLights = []Light{
	Ambient{Level: 0.2},
	Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}},
	Point{Position: geometry.Vector3{X: 0, Y: 2, Z: 10}, Range: 40},
}

// The fog fades each cell and pixel by how deep it is, flat or smooth, tinted or not. A floor is
// seen from above at a pitch, so that one face runs from near to the horizon, and the ray through
// the middle of every cell (and pixel) says how deep the floor is there. What is nearer than
// Start is exactly as it is with no fog. What is farther is the no-fog frame's color faded by that
// depth, with the character of the cell (or the brightness of the pixel) as it was, and from End on
// it is the fog's color. Where nothing was drawn, nothing is.
func TestFogFadesEveryCellAndPixelByItsDepth(t *testing.T) {
	haze := engine.RGB(30, 30, 60)
	fog := &Fog{Color: haze, Start: 8, End: 24}
	cam := Camera{Position: geometry.Vector3{Y: 3}, Pitch: -0.6}
	foggy := cam
	foggy.Fog = fog

	for _, tc := range fogFloors() {
		floor := tc.floor.withColor(engine.Palette(70))

		// Cells.
		whole, fogged := newShadeBuffer(t), newShadeBuffer(t)
		floor.DrawShaded(whole, cam, fogLights...)
		floor.DrawShaded(fogged, foggy, fogLights...)
		ray := func(x, y float64) (geometry.Vector3, geometry.Vector3, bool) { return cam.Ray(x, y, shadeW, shadeH) }
		near, mid, far := 0, 0, 0
		for y := range shadeH {
			for x := range shadeW {
				was, got := whole.At(x, y), fogged.At(x, y)
				if was == engine.BlankCell {
					if got != engine.BlankCell {
						t.Fatalf("%s cells: wanted fog to leave cell (%d, %d), where nothing is drawn, empty, got %v", tc.name, x, y, got)
					}
					continue
				}
				depth := floorDepth(ray, float64(x)+0.5, float64(y)+0.5)
				if got.Character != was.Character {
					t.Fatalf("%s cells: wanted cell (%d, %d) to keep its character %q, got %q", tc.name, x, y, was.Character, got.Character)
				}
				if want := fog.Apply(was.Foreground, depth); !colorsNear(got.Foreground, want, 1) {
					t.Fatalf("%s cells: wanted cell (%d, %d), %.2f deep, in %v, got %v", tc.name, x, y, depth, want, got.Foreground)
				}
				switch {
				case depth < fog.Start-1e-6:
					near++
					if got != was {
						t.Fatalf("%s cells: wanted cell (%d, %d), %.2f deep and before the fog, as it is without it, got %v and %v", tc.name, x, y, depth, got, was)
					}
				case depth > fog.End+1e-6:
					far++
					if got.Foreground != haze {
						t.Fatalf("%s cells: wanted cell (%d, %d), %.2f deep and beyond the fog, in the fog's color, got %v", tc.name, x, y, depth, got.Foreground)
					}
				default:
					mid++
				}
			}
		}
		if near < 100 || mid < 100 || far < 50 {
			t.Errorf("%s cells: wanted cells before, in and beyond the fog, got %d, %d and %d", tc.name, near, mid, far)
		}

		// Pixels.
		wholePix, foggedPix := newShadeBuffer(t).Canvas(), newShadeBuffer(t).Canvas()
		floor.DrawShadedPixels(wholePix, cam, fogLights...)
		floor.DrawShadedPixels(foggedPix, foggy, fogLights...)
		rayPix := func(x, y float64) (geometry.Vector3, geometry.Vector3, bool) { return cam.RayPixels(x, y, wholePix) }
		near, mid, far = 0, 0, 0
		for y := range wholePix.Height() {
			for x := range wholePix.Width() {
				was, got := wholePix.At(x, y), foggedPix.At(x, y)
				if was.IsDefault() {
					if !got.IsDefault() {
						t.Fatalf("%s pixels: wanted fog to leave pixel (%d, %d), where nothing is drawn, empty, got %v", tc.name, x, y, got)
					}
					continue
				}
				depth := floorDepth(rayPix, float64(x)+0.5, float64(y)+0.5)
				if want := fog.Apply(was, depth); !colorsNear(got, want, 1) {
					t.Fatalf("%s pixels: wanted pixel (%d, %d), %.2f deep, in %v, got %v", tc.name, x, y, depth, want, got)
				}
				switch {
				case depth < fog.Start-1e-6:
					near++
					if got != was {
						t.Fatalf("%s pixels: wanted pixel (%d, %d), %.2f deep and before the fog, as it is without it, got %v and %v", tc.name, x, y, depth, got, was)
					}
				case depth > fog.End+1e-6:
					far++
					if got != haze {
						t.Fatalf("%s pixels: wanted pixel (%d, %d), %.2f deep and beyond the fog, in the fog's color, got %v", tc.name, x, y, depth, got)
					}
				default:
					mid++
				}
			}
		}
		if near < 200 || mid < 200 || far < 40 {
			t.Errorf("%s pixels: wanted pixels before, in and beyond the fog, got %d, %d and %d", tc.name, near, mid, far)
		}
	}
}

// With no Fog, or one that starts beyond the whole scene, the frame is the one there was.
func TestNoFogAndFarFogDrawTheSameFrame(t *testing.T) {
	cam := Camera{Position: geometry.Vector3{Y: 3}, Pitch: -0.6}
	far := cam
	far.Fog = &Fog{Color: engine.RGB(30, 30, 60), Start: 1e9, End: 2e9}
	nothing := cam
	nothing.Fog = nil

	for _, tc := range fogFloors() {
		floor := tc.floor.withColor(engine.Palette(70))
		whole := newShadeBuffer(t)
		floor.DrawShaded(whole, cam, fogLights...)
		wholePix := newShadeBuffer(t).Canvas()
		floor.DrawShadedPixels(wholePix, cam, fogLights...)
		for name, c := range map[string]Camera{"a nil Fog": nothing, "a Fog beyond the scene": far} {
			cells := newShadeBuffer(t)
			floor.DrawShaded(cells, c, fogLights...)
			for y := range shadeH {
				for x := range shadeW {
					if cells.At(x, y) != whole.At(x, y) {
						t.Fatalf("%s, %s: wanted cell (%d, %d) as it is without fog, got %v and %v", tc.name, name, x, y, cells.At(x, y), whole.At(x, y))
					}
				}
			}
			pixels := newShadeBuffer(t).Canvas()
			floor.DrawShadedPixels(pixels, c, fogLights...)
			for y := range pixels.Height() {
				for x := range pixels.Width() {
					if pixels.At(x, y) != wholePix.At(x, y) {
						t.Fatalf("%s, %s: wanted pixel (%d, %d) as it is without fog, got %v and %v", tc.name, name, x, y, pixels.At(x, y), wholePix.At(x, y))
					}
				}
			}
		}
	}
}

// Wireframes are not fogged, and the copies of a Camera share the Fog they point to.
func TestFogLeavesWireframesAloneAndIsSharedByCopies(t *testing.T) {
	line := Mesh{Vertices: []geometry.Vector3{{X: 3, Z: 5}, {X: 3, Z: 50}}, Edges: [][2]int{{0, 1}}}
	cam := Camera{}
	foggy := Camera{Fog: &Fog{Color: engine.RGB(30, 30, 60), Start: 1, End: 2}}
	cell := engine.Cell{Character: '#', Foreground: engine.RGB(200, 0, 0)}

	plain, fogged := newShadeBuffer(t), newShadeBuffer(t)
	line.Draw(plain, cam, cell)
	line.Draw(fogged, foggy, cell)
	for y := range shadeH {
		for x := range shadeW {
			if plain.At(x, y) != fogged.At(x, y) {
				t.Fatalf("wanted the wireframe unfogged, cell (%d, %d) is %v and was %v", x, y, fogged.At(x, y), plain.At(x, y))
			}
		}
	}

	a := foggy
	b := a
	b.Fog.End = 99
	if a.Fog.End != 99 {
		t.Errorf("wanted copies of a Camera to share their Fog, got an End of %v", a.Fog.End)
	}
}

// A fog and a far plane work together: with the fog complete at the far plane, every cell before the
// plane is exactly what the fog alone draws, so that the cut makes no step in the color, and there
// is nothing beyond it.
func TestFogAndTheFarPlaneWorkTogether(t *testing.T) {
	const far = 24.0
	flat := Plane(400, 400)
	flat.Normals = nil
	floor := flat.withColor(engine.Palette(70))
	foggy := Camera{Position: geometry.Vector3{Y: 3}, Pitch: -0.6, Fog: &Fog{Color: engine.RGB(30, 30, 60), Start: 8, End: far}}
	cut := foggy
	cut.Far = far
	ray := func(x, y float64) (geometry.Vector3, geometry.Vector3, bool) { return foggy.Ray(x, y, shadeW, shadeH) }

	fogOnly, both := newShadeBuffer(t), newShadeBuffer(t)
	floor.DrawShaded(fogOnly, foggy, fogLights...)
	floor.DrawShaded(both, cut, fogLights...)
	before, beyond := 0, 0
	for y := range shadeH {
		for x := range shadeW {
			depth := floorDepth(ray, float64(x)+0.5, float64(y)+0.5)
			switch {
			case depth > far+1e-6:
				beyond++
				if both.At(x, y) != engine.BlankCell {
					t.Fatalf("wanted nothing beyond the far plane, got %v in cell (%d, %d), %.2f deep", both.At(x, y), x, y, depth)
				}
			case depth < far-1e-6:
				before++
				if both.At(x, y) != fogOnly.At(x, y) {
					t.Fatalf("wanted cell (%d, %d), %.2f deep, as the fog alone draws it, got %v and %v", x, y, depth, both.At(x, y), fogOnly.At(x, y))
				}
			}
		}
	}
	if before < 200 || beyond < 100 {
		t.Errorf("wanted cells before the far plane and beyond it, got %d and %d", before, beyond)
	}
}

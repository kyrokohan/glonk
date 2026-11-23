package mesh

import (
	"math"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

var magenta = engine.RGB(255, 0, 255)

// outlineSquare is the outline of a 1 by 1 square lift above the floor, away from the camera:
// the square the issue measured.
func outlineSquare(away, lift float64) Mesh {
	m := Mesh{
		Vertices: []geometry.Vector3{
			{X: -0.5, Y: lift, Z: away - 0.5}, {X: 0.5, Y: lift, Z: away - 0.5},
			{X: 0.5, Y: lift, Z: away + 0.5}, {X: -0.5, Y: lift, Z: away + 0.5},
		},
		Edges: [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}},
	}
	m.UpdateBounds()
	return m
}

// floorCam is the camera of the issue: 1.6 above the floor, looking at the square.
func floorCam(away float64) Camera {
	return Camera{Position: geometry.Vector3{Y: 1.6}, Pitch: -math.Atan2(1.6, away), FOV: math.Pi / 3}
}

// countColor counts the pixels of a canvas that are color.
func countPixels(c *engine.Canvas, color engine.Color) (n int) {
	for y := range c.Height() {
		for x := range c.Width() {
			if c.At(x, y) == color {
				n++
			}
		}
	}
	return n
}

// shareOfOutline is the share of an outline's pixels, as DrawPixels makes them on an empty canvas, that
// are drawn by draw over a floor: the issue's measure.
func shareOfOutline(t *testing.T, away, lift float64, draw func(m Mesh, c *engine.Canvas, cam Camera)) float64 {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "")
	square, cam := outlineSquare(away, lift), floorCam(away)

	empty, err := engine.NewFrameBuffer(160, 45)
	if err != nil {
		t.Fatal(err)
	}
	square.DrawPixels(empty.Canvas(), cam, magenta)
	want := countPixels(empty.Canvas(), magenta)
	if want == 0 {
		t.Fatalf("away %v: wanted an outline to measure", away)
	}

	fb, _ := engine.NewFrameBuffer(160, 45)
	c := fb.Canvas()
	Plane(40, 40).DrawShadedPixels(c, cam, Ambient{Level: 1})
	draw(square, c, cam)
	return float64(countPixels(c, magenta)) / float64(want)
}

// The issue's table: an outline that lies on a floor, drawn after it, at the five distances it
// measured. DrawDepthPixels loses half of it or all of it at every one of them without a lift, and
// the outline that lies on the surface is whole at each, with no lift at all.
func TestAnOutlineOnAFloorIsWholeAtEveryDistance(t *testing.T) {
	for _, away := range []float64{1.5, 3, 5, 8, 12} {
		depth := shareOfOutline(t, away, 0, func(m Mesh, c *engine.Canvas, cam Camera) { m.DrawDepthPixels(c, cam, magenta) })
		surface := shareOfOutline(t, away, 0, func(m Mesh, c *engine.Canvas, cam Camera) { m.DrawOnSurfacePixels(c, cam, magenta) })
		if depth > 0.6 {
			t.Errorf("%v away: wanted DrawDepthPixels to lose a good part of the outline with no lift, or this proves nothing, got %.0f%%", away, 100*depth)
		}
		if surface != 1 {
			t.Errorf("%v away: wanted the whole outline, got %.0f%%", away, 100*surface)
		}
	}
}

// And in cells: the floor is cells, and so is the outline, lying on it.
func TestAnOutlineOnAFloorIsWholeInCells(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	mark := engine.Cell{Character: 'X', Foreground: magenta}
	count := func(fb *engine.FrameBuffer) (n int) {
		for y := range fb.Height() {
			for x := range fb.Width() {
				if fb.At(x, y) == mark {
					n++
				}
			}
		}
		return n
	}
	for _, away := range []float64{1.5, 3, 5, 8, 12} {
		square, cam := outlineSquare(away, 0), floorCam(away)
		empty, _ := engine.NewFrameBuffer(80, 24)
		square.Draw(empty, cam, mark)
		want := count(empty)

		depth, _ := engine.NewFrameBuffer(80, 24)
		Plane(40, 40).DrawShaded(depth, cam, Ambient{Level: 1})
		square.DrawDepth(depth, cam, mark)

		fb, _ := engine.NewFrameBuffer(80, 24)
		Plane(40, 40).DrawShaded(fb, cam, Ambient{Level: 1})
		square.DrawOnSurface(fb, cam, mark)
		if got := count(fb); got != want {
			t.Errorf("%v away: wanted all %d cells of the outline, got %d (DrawDepth got %d)", away, want, got, count(depth))
		}
	}
}

// The other side of it: an outline that is under the floor is hidden. 0.3 under it is a third of
// the square, which no distance of the five makes ambiguous.
func TestAnOutlineUnderTheFloorIsHidden(t *testing.T) {
	for _, away := range []float64{1.5, 3, 5, 8, 12} {
		got := shareOfOutline(t, away, -0.3, func(m Mesh, c *engine.Canvas, cam Camera) { m.DrawOnSurfacePixels(c, cam, magenta) })
		if got != 0 {
			t.Errorf("%v away: wanted an outline 0.3 under the floor hidden, got %.0f%%", away, 100*got)
		}
		above := shareOfOutline(t, away, 0.05, func(m Mesh, c *engine.Canvas, cam Camera) { m.DrawOnSurfacePixels(c, cam, magenta) })
		if above != 1 {
			t.Errorf("%v away: wanted an outline above the floor whole, got %.0f%%", away, 100*above)
		}
	}
}

// cubeOnAFloor is a solid cube standing on a floor, seen from a low angle, and the camera.
func cubeOnAFloor() (floor, cube Mesh, cam Camera) {
	floor = Plane(30, 30)
	floor.Material.Color = engine.RGB(70, 90, 70)
	cube = Cube(1)
	cube.Material.Color = engine.RGB(200, 140, 60)
	cube.Transform = Transform{Position: geometry.Vector3{Y: 0.5, Z: 4}, Yaw: 0.6}
	cam = Camera{Position: geometry.Vector3{Y: 0.5}, Pitch: -0.03, FOV: math.Pi / 3, Near: 0.1}
	return floor, cube, cam
}

// visibleEdges are the edges of the cube that have a face turned to the camera: for a solid that is
// convex, the edges that can be seen.
func visibleEdges(m Mesh, cam Camera) [][2]int {
	place := m.Transform.placement()
	var front [][3]int
	for _, f := range m.Faces {
		a, b, c := place.apply(m.Vertices[f[0]]), place.apply(m.Vertices[f[1]]), place.apply(m.Vertices[f[2]])
		if b.Sub(a).Cross(c.Sub(a)).Dot(cam.Position.Sub(a)) > 0 {
			front = append(front, f)
		}
	}
	var seen [][2]int
	for _, e := range m.Edges {
		for _, f := range front {
			if (f[0] == e[0] || f[1] == e[0] || f[2] == e[0]) && (f[0] == e[1] || f[1] == e[1] || f[2] == e[1]) {
				seen = append(seen, e)
				break
			}
		}
	}
	return seen
}

// near reports whether a pixel of want is within one pixel of (x, y).
func nearPixel(want map[[2]int]bool, x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if want[[2]int{x + dx, y + dy}] {
				return true
			}
		}
	}
	return false
}

// The outline of a cube that stands on a floor, seen from a low angle, is unbroken where it can be
// seen, and not there where it cannot: every pixel of the edges that have a face turned to the camera is
// drawn, and the others, behind the cube, draw nothing but where they meet one that is drawn.
func TestTheOutlineOfACubeOnAFloorIsUnbroken(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	floor, cube, cam := cubeOnAFloor()
	seen := visibleEdges(cube, cam)
	if len(seen) < 5 || len(seen) > 9 {
		t.Fatalf("wanted 5 to 9 of the 12 edges to be seen from here, got %d", len(seen))
	}

	// What every pixel of the edges that are seen is, with no depth.
	only := cube
	only.Edges = seen
	emptyFB, _ := engine.NewFrameBuffer(160, 45)
	only.DrawPixels(emptyFB.Canvas(), cam, magenta)
	want := map[[2]int]bool{}
	for y := range emptyFB.Canvas().Height() {
		for x := range emptyFB.Canvas().Width() {
			if emptyFB.Canvas().At(x, y) == magenta {
				want[[2]int{x, y}] = true
			}
		}
	}
	if len(want) < 100 {
		t.Fatalf("wanted an outline of at least 100 pixels, got %d", len(want))
	}

	fb, _ := engine.NewFrameBuffer(160, 45)
	c := fb.Canvas()
	light := Ambient{Level: 1}
	floor.DrawShadedPixels(c, cam, light)
	cube.DrawShadedPixels(c, cam, light)
	cube.DrawOnSurfacePixels(c, cam, magenta)

	got := 0
	for y := range c.Height() {
		for x := range c.Width() {
			drawn := c.At(x, y) == magenta
			switch {
			case want[[2]int{x, y}] && !drawn:
				t.Errorf("pixel (%d, %d) of an edge that is seen was not drawn", x, y)
			case drawn && !want[[2]int{x, y}] && !nearPixel(want, x, y):
				t.Errorf("pixel (%d, %d) is drawn, and no edge that is seen is near it: an edge behind the cube shows", x, y)
			}
			if drawn {
				got++
			}
		}
	}
	if got < len(want) {
		t.Errorf("wanted at least the %d pixels of the edges that are seen, got %d", len(want), got)
	}

	// In cells, the same.
	mark := engine.Cell{Character: 'X', Foreground: magenta}
	onlyCells, _ := engine.NewFrameBuffer(80, 24)
	only.Draw(onlyCells, cam, mark)
	cells, _ := engine.NewFrameBuffer(80, 24)
	floor.DrawShaded(cells, cam, light)
	cube.DrawShaded(cells, cam, light)
	cube.DrawOnSurface(cells, cam, mark)
	wantCells := map[[2]int]bool{}
	for y := range onlyCells.Height() {
		for x := range onlyCells.Width() {
			if onlyCells.At(x, y) == mark {
				wantCells[[2]int{x, y}] = true
			}
		}
	}
	for y := range cells.Height() {
		for x := range cells.Width() {
			drawn := cells.At(x, y) == mark
			switch {
			case wantCells[[2]int{x, y}] && !drawn:
				t.Errorf("cell (%d, %d) of an edge that is seen was not drawn", x, y)
			case drawn && !wantCells[[2]int{x, y}] && !nearPixel(wantCells, x, y):
				t.Errorf("cell (%d, %d) is drawn, and no edge that is seen is near it: an edge behind the cube shows", x, y)
			}
		}
	}
}

// A solid that is nearer hides the outline, wherever in the order it is drawn, and where it is not the
// outline is as it was.
func TestANearerSolidHidesTheOutline(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	floor, cube, cam := cubeOnAFloor()
	light := Ambient{Level: 1}
	wall := wallAt(2.5, 1.2, 1.2, engine.RGB(40, 200, 220)) // in front of the cube, over its middle
	wall.Transform.Position = geometry.Vector3{Y: 0.5}

	render := func(wallFirst bool) (*engine.Canvas, *engine.Canvas) {
		fb, _ := engine.NewFrameBuffer(160, 45)
		c := fb.Canvas()
		floor.DrawShadedPixels(c, cam, light)
		cube.DrawShadedPixels(c, cam, light)
		if wallFirst {
			wall.DrawShadedPixels(c, cam, light)
		}
		cube.DrawOnSurfacePixels(c, cam, magenta)
		if !wallFirst {
			wall.DrawShadedPixels(c, cam, light)
		}

		bare, _ := engine.NewFrameBuffer(160, 45)
		b := bare.Canvas()
		floor.DrawShadedPixels(b, cam, light)
		cube.DrawShadedPixels(b, cam, light)
		cube.DrawOnSurfacePixels(b, cam, magenta)
		return c, b
	}
	before, bare := render(true)
	after, _ := render(false)

	// Where the wall is.
	alone, _ := engine.NewFrameBuffer(160, 45)
	wall.DrawShadedPixels(alone.Canvas(), cam, light)
	covered, outside := 0, 0
	for y := range before.Height() {
		for x := range before.Width() {
			if before.At(x, y) != after.At(x, y) {
				t.Fatalf("pixel (%d, %d): wanted the same picture whichever order the wall is drawn in, got %v and %v", x, y, before.At(x, y), after.At(x, y))
			}
			if !alone.Canvas().At(x, y).IsDefault() {
				if before.At(x, y) == magenta {
					t.Fatalf("pixel (%d, %d): wanted the outline hidden by the wall", x, y)
				}
				if bare.At(x, y) == magenta {
					covered++ // the outline was there and the wall took it
				}
			} else {
				outside++
				if before.At(x, y) != bare.At(x, y) {
					t.Fatalf("pixel (%d, %d): wanted what is outside the wall as it was without it", x, y)
				}
			}
		}
	}
	if covered < 10 {
		t.Errorf("wanted the wall to cover at least 10 pixels of the outline, or this proves nothing, got %d", covered)
	}
	_ = outside
}

// The case the manual shows: a grid on a floor. It is a different mesh from the floor, and is whole on it,
// and a box that is nearer hides it.
func TestAGridOnAFloorIsWholeAndHiddenByANearerBox(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	floor := Plane(40, 40)
	floor.Material.Color = engine.RGB(70, 90, 70)
	grid := Grid(8, 8)
	grid.Transform.Position = geometry.Vector3{Z: 7}
	cam := floorCam(7)
	light := Ambient{Level: 1}

	emptyFB, _ := engine.NewFrameBuffer(160, 45)
	grid.DrawPixels(emptyFB.Canvas(), cam, magenta)
	want := countPixels(emptyFB.Canvas(), magenta)

	fb, _ := engine.NewFrameBuffer(160, 45)
	c := fb.Canvas()
	floor.DrawShadedPixels(c, cam, light)
	grid.DrawOnSurfacePixels(c, cam, magenta)
	if got := countPixels(c, magenta); got != want {
		t.Errorf("wanted all %d pixels of the grid on the floor, got %d", want, got)
	}

	// A box in front of it.
	box := Cube(2)
	box.Material.Color = engine.RGB(200, 140, 60)
	box.Transform.Position = geometry.Vector3{Y: 1, Z: 4}
	fb2, _ := engine.NewFrameBuffer(160, 45)
	c2 := fb2.Canvas()
	floor.DrawShadedPixels(c2, cam, light)
	box.DrawShadedPixels(c2, cam, light)
	boxOnly, _ := engine.NewFrameBuffer(160, 45)
	box.DrawShadedPixels(boxOnly.Canvas(), cam, light)
	grid.DrawOnSurfacePixels(c2, cam, magenta)
	hidden := 0
	for y := range c2.Height() {
		for x := range c2.Width() {
			if !boxOnly.Canvas().At(x, y).IsDefault() {
				if c2.At(x, y) == magenta {
					t.Fatalf("pixel (%d, %d): wanted the grid hidden by the box", x, y)
				}
				if c.At(x, y) == magenta {
					hidden++
				}
			}
		}
	}
	if hidden < 20 {
		t.Errorf("wanted the box to hide at least 20 pixels of the grid, or this proves nothing, got %d", hidden)
	}
}

// With nothing drawn there is no surface for a line to lie on, and DrawOnSurface is DrawDepth: the same
// cells and pixels, for edges cut by the near plane or off to the side.
func TestDrawOnSurfaceOnAnEmptyFrameIsDrawDepth(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	mark := engine.Cell{Character: 'X', Foreground: magenta}
	sphere := Sphere(1.5, 8, 5)
	meshes := map[string]Mesh{"a cube": Cube(2), "a sphere": sphere, "a grid": Grid(10, 10), "a square": outlineSquare(3, 0)}
	for name, m := range meshes {
		for _, pos := range []geometry.Vector3{{Z: 5}, {Z: 0.5}, {Z: -2}, {X: 4, Z: 3}, {X: 0.3, Y: 0.2, Z: 0.05}} {
			m.Transform = Transform{Position: pos, Yaw: 0.4, Pitch: 0.2}
			for _, cam := range []Camera{{FOV: 1.2, Near: 0.1}, {FOV: 1.2, Near: 0.1, Far: 6}, {Position: geometry.Vector3{Y: 1.6}, Pitch: -0.4, FOV: 1.0}} {
				a, _ := engine.NewFrameBuffer(80, 24)
				b, _ := engine.NewFrameBuffer(80, 24)
				m.DrawDepth(a, cam, mark)
				m.DrawOnSurface(b, cam, mark)
				for y := range a.Height() {
					for x := range a.Width() {
						if a.At(x, y) != b.At(x, y) {
							t.Fatalf("%s at %v: cell (%d, %d): wanted what DrawDepth draws, got %+v and %+v", name, pos, x, y, a.At(x, y), b.At(x, y))
						}
					}
				}
				pa, _ := engine.NewFrameBuffer(80, 24)
				pb, _ := engine.NewFrameBuffer(80, 24)
				m.DrawDepthPixels(pa.Canvas(), cam, magenta)
				m.DrawOnSurfacePixels(pb.Canvas(), cam, magenta)
				for y := range pa.Canvas().Height() {
					for x := range pa.Canvas().Width() {
						if pa.Canvas().At(x, y) != pb.Canvas().At(x, y) {
							t.Fatalf("%s at %v: pixel (%d, %d): wanted what DrawDepthPixels draws", name, pos, x, y)
						}
					}
				}
			}
		}
	}
}

func TestDrawingOnASurfaceAllocatesNothingOnceWarm(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector makes sync.Pool drop items, so allocation counts mean nothing")
	}
	t.Setenv("TERM_PROGRAM", "")
	floor, cube, cam := cubeOnAFloor()
	fb, _ := engine.NewFrameBuffer(160, 45)
	floor.DrawShaded(fb, cam, Ambient{Level: 1})
	cube.DrawShaded(fb, cam, Ambient{Level: 1})
	mark := engine.Cell{Character: 'X'}
	for name, draw := range map[string]func(){
		"cells":  func() { cube.DrawOnSurface(fb, cam, mark) },
		"pixels": func() { cube.DrawOnSurfacePixels(fb.Canvas(), cam, magenta) },
	} {
		draw()
		if n := testing.AllocsPerRun(20, draw); n != 0 {
			t.Errorf("%s: wanted no allocation, got %v", name, n)
		}
	}
}

package mesh

import (
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// pixelWire is the color of the lines in these tests, one that no shade of the solids
// drawn beside them can come out as.
var pixelWire = engine.RGB(1, 254, 1)

// shadeCanvas returns the pixel layer of a new shade buffer: 80 pixels wide, 48 tall.
func shadeCanvas(t *testing.T) *engine.Canvas {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "") // Canvas.Seamless depends on the terminal running the tests
	return newShadeBuffer(t).Canvas()
}

// pixelsOf is the set of pixels a canvas has drawn in, with their colors.
func pixelsOf(c *engine.Canvas) map[[2]int]engine.Color {
	out := map[[2]int]engine.Color{}
	for y := range c.Height() {
		for x := range c.Width() {
			if p := c.At(x, y); !p.IsDefault() {
				out[[2]int{x, y}] = p
			}
		}
	}
	return out
}

// A wireframe of pixels is the line between the ends of each edge, as projected by
// Camera.Project and stretched to pixel rows, drawn with Canvas.DrawLine: two other
// pieces of code, not the loop under test, say where the pixels are.
func TestDrawPixelsDrawsEachEdgeBetweenItsProjectedEnds(t *testing.T) {
	rng := rand.New(rand.NewPCG(51, 52))
	cam := Camera{FOV: math.Pi / 2, Near: 0.1}
	const w, h = shadeW, shadeH

	point := func() geometry.Vector3 {
		z := 1 + 9*rng.Float64()
		return geometry.Vector3{X: (rng.Float64()*2 - 1) * z * 0.8, Y: (rng.Float64()*2 - 1) * z * 0.3, Z: z}
	}

	tested := 0
	for range 800 {
		a, b := point(), point()
		ax, ay, okA := cam.Project(a, w, h)
		bx, by, okB := cam.Project(b, w, h)
		if !okA || !okB {
			continue
		}

		want := shadeCanvas(t)
		want.DrawLine(int(math.Floor(ax)), int(math.Floor(2*ay)), int(math.Floor(bx)), int(math.Floor(2*by)), pixelWire)
		got := shadeCanvas(t)
		edgeMesh(a, b).DrawPixels(got, cam, pixelWire)

		if !maps.Equal(pixelsOf(got), pixelsOf(want)) {
			t.Fatalf("edge %v to %v: wanted the line between (%v,%v) and (%v,%v) in pixels, got %d pixels, wanted %d", a, b, ax, 2*ay, bx, 2*by, len(pixelsOf(got)), len(pixelsOf(want)))
		}
		tested++
	}
	if tested < 600 {
		t.Fatalf("wanted at least 600 edges tested, got %d", tested)
	}
}

// Seen through cells, a pixel wireframe is the shape of the cell one: every cell Draw
// draws holds a pixel of the lines, give or take one cell where Bresenham's choices at
// two resolutions differ. This catches a wireframe drawn at the wrong scale or offset.
func TestDrawPixelsLandsOnTheCellsDrawDraws(t *testing.T) {
	m := Cube(2)
	m.Transform = Transform{Position: geometry.Vector3{X: 0.3, Y: -0.2, Z: 5}, Yaw: 0.6, Pitch: -0.4}

	cells := newShadeBuffer(t)
	m.Draw(cells, shadeCam(), wireCell)
	px := shadeCanvas(t)
	m.DrawPixels(px, shadeCam(), pixelWire)

	pixelCells := map[[2]int]bool{}
	for p := range pixelsOf(px) {
		pixelCells[[2]int{p[0], p[1] / 2}] = true
	}
	withinOne := func(set map[[2]int]bool, c [2]int) bool {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if set[[2]int{c[0] + dx, c[1] + dy}] {
					return true
				}
			}
		}
		return false
	}
	drawn := cellsOf(cells)
	if len(drawn) < 30 || len(pixelCells) < 30 {
		t.Fatalf("wanted a wireframe on screen, got %d cells and %d cells of pixels", len(drawn), len(pixelCells))
	}
	for c := range drawn {
		if !withinOne(pixelCells, c) {
			t.Fatalf("wanted a pixel of the lines within one cell of cell %v, which Draw draws", c)
		}
	}
	for c := range pixelCells {
		if !withinOne(drawn, c) {
			t.Fatalf("wanted the pixels in cell %v to lie within one cell of a cell Draw draws", c)
		}
	}
}

// A wireframe behind a solid is hidden exactly where the solid covers it; one in front of
// it is drawn whole. Both are drawn after the solid, where DrawPixels would have put
// them on top either way.
func TestDrawDepthPixelsIsHiddenByNearerSolidsAndHidesFartherOnes(t *testing.T) {
	solid := solidCube(geometry.Vector3{Z: 5}, 2)
	tests := []struct {
		name    string
		wire    Mesh
		inFront bool
	}{
		{"behind", solidCube(geometry.Vector3{X: 1.2, Y: 0.3, Z: 9}, 2), false},
		{"in front", solidCube(geometry.Vector3{X: 0.3, Z: 2.8}, 1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alone, wireAlone, both := shadeCanvas(t), shadeCanvas(t), shadeCanvas(t)
			solid.DrawShadedPixels(alone, shadeCam(), goldenSun)
			tt.wire.DrawPixels(wireAlone, shadeCam(), pixelWire)
			solid.DrawShadedPixels(both, shadeCam(), goldenSun)
			tt.wire.DrawDepthPixels(both, shadeCam(), pixelWire)
			s, w := pixelsOf(alone), pixelsOf(wireAlone)

			overlap, outside := 0, 0
			for p := range w {
				if _, ok := s[p]; ok {
					overlap++
				} else {
					outside++
				}
			}
			if overlap < 3 || outside < 3 {
				t.Fatalf("wanted the wire to overlap the solid and stick out of it, got %d and %d pixels", overlap, outside)
			}
			for y := range both.Height() {
				for x := range both.Width() {
					p := [2]int{x, y}
					var want engine.Color
					if _, isWire := w[p]; isWire && (tt.inFront || s[p].IsDefault()) {
						want = pixelWire // nothing nearer covers it
					} else {
						want = s[p]
					}
					if got := both.At(x, y); got != want {
						t.Fatalf("pixel (%d,%d): wanted %v, got %v", x, y, want, got)
					}
				}
			}
		})
	}
}

// Whatever order a scene is drawn in, it comes out the same: that is what depth is for.
func TestDrawDepthPixelsDoesNotDependOnDrawingOrder(t *testing.T) {
	rng := rand.New(rand.NewPCG(53, 54))
	for scene := range 300 {
		type item struct {
			m    Mesh
			wire bool
		}
		var items []item
		for range 3 + rng.IntN(4) {
			m := Cube(0.5 + 2.5*rng.Float64())
			m.Transform = Transform{
				Position: geometry.Vector3{X: rng.Float64()*8 - 4, Y: rng.Float64()*4 - 2, Z: 3 + 11*rng.Float64()},
				Yaw:      rng.Float64() * 6, Pitch: rng.Float64() * 3,
			}
			m.Material.Color = engine.Palette(uint8(16 + rng.IntN(200)))
			items = append(items, item{m, rng.IntN(2) == 0})
		}
		draw := func(order []int) map[[2]int]engine.Color {
			c := shadeCanvas(t)
			for _, i := range order {
				it := items[i]
				if it.wire {
					it.m.DrawDepthPixels(c, shadeCam(), it.m.Material.Color)
				} else {
					it.m.DrawShadedPixels(c, shadeCam(), goldenSun)
				}
			}
			return pixelsOf(c)
		}

		first := draw(rng.Perm(len(items)))
		for range 2 {
			if again := draw(rng.Perm(len(items))); !maps.Equal(again, first) {
				t.Fatalf("scene %d: wanted the same pixels in any order of drawing", scene)
			}
		}
	}
}

// On a blank canvas DrawDepthPixels draws the pixels DrawPixels does, for anything: cubes,
// grids and spheres, near and far, across the near plane and with edges far off the screen.
func TestDrawDepthPixelsDrawsThePixelsDrawPixelsDoes(t *testing.T) {
	rng := rand.New(rand.NewPCG(55, 56))
	for scene := range 400 {
		var m Mesh
		switch scene % 3 {
		case 0:
			m = Cube(0.5 + 3*rng.Float64())
		case 1:
			m = Grid(4+10*rng.Float64(), 1+rng.IntN(8))
		default:
			m = Sphere(1+rng.Float64(), 3+rng.IntN(10), 2+rng.IntN(6))
		}
		m.Transform = Transform{
			Position: geometry.Vector3{X: rng.Float64()*20 - 10, Y: rng.Float64()*10 - 5, Z: rng.Float64()*16 - 4},
			Yaw:      rng.Float64() * 6, Pitch: rng.Float64() * 3, Roll: rng.Float64() * 3,
		}
		cam := Camera{Position: geometry.Vector3{X: rng.Float64()*4 - 2, Z: -rng.Float64() * 3}, Yaw: rng.Float64()*2 - 1, FOV: 0.5 + 2*rng.Float64(), Near: 0.05 + 0.5*rng.Float64()}

		plain, deep := shadeCanvas(t), shadeCanvas(t)
		m.DrawPixels(plain, cam, pixelWire)
		m.DrawDepthPixels(deep, cam, pixelWire)

		if !maps.Equal(pixelsOf(plain), pixelsOf(deep)) {
			t.Fatalf("scene %d: wanted DrawDepthPixels to draw the pixels DrawPixels does, got %d and wanted %d", scene, len(pixelsOf(deep)), len(pixelsOf(plain)))
		}
	}
}

// The two ways of drawing differ in what they leave behind. DrawPixels paints over a
// nearer solid and puts nothing in the depth, so a farther solid drawn after it covers it.
// DrawDepthPixels stores the depth, so a farther solid cannot.
func TestDrawPixelsIgnoresDepthAndDrawDepthPixelsKeepsIt(t *testing.T) {
	front := solidCube(geometry.Vector3{X: 0.3, Z: 2.8}, 1)
	far := solidCube(geometry.Vector3{X: 1.2, Y: 0.3, Z: 9}, 2)
	wire := func(m Mesh, depth bool) func(*engine.Canvas) {
		return func(c *engine.Canvas) {
			if depth {
				m.DrawDepthPixels(c, shadeCam(), pixelWire)
			} else {
				m.DrawPixels(c, shadeCam(), pixelWire)
			}
		}
	}
	solidPixels := func(m Mesh) map[[2]int]engine.Color {
		c := shadeCanvas(t)
		m.DrawShadedPixels(c, shadeCam(), goldenSun)
		return pixelsOf(c)
	}

	// A wireframe behind a solid, drawn after it: DrawPixels puts it on top.
	for _, depth := range []bool{false, true} {
		c := shadeCanvas(t)
		front.DrawShadedPixels(c, shadeCam(), goldenSun)
		wire(far, depth)(c)
		over := 0
		for p := range solidPixels(front) {
			if c.At(p[0], p[1]) == pixelWire {
				over++
			}
		}
		if (over > 0) == depth {
			t.Errorf("depth %v: wanted the wire to show over the solid only for DrawPixels, got %d pixels", depth, over)
		}
	}

	// A wireframe in front, then a solid behind it: DrawPixels' lines are covered, DrawDepthPixels' are not.
	for _, depth := range []bool{false, true} {
		c := shadeCanvas(t)
		wire(front, depth)(c)
		far.DrawShadedPixels(c, shadeCam(), goldenSun)
		kept := 0
		for p := range solidPixels(far) {
			if c.At(p[0], p[1]) == pixelWire {
				kept++
			}
		}
		if (kept > 0) != depth {
			t.Errorf("depth %v: wanted the wire kept in front of the farther solid only for DrawDepthPixels, got %d pixels", depth, kept)
		}
	}
}

// pixelDepthOf finds the depth that drawing leaves at pixel (x, y), by asking on fresh
// canvases whether something at a given distance would be drawn there: it is, exactly
// when it is nearer than what is there. It gives +Inf for a pixel with nothing in it.
func pixelDepthOf(t *testing.T, draw func(c *engine.Canvas), x, y int) float64 {
	t.Helper()
	drawn := func(z float64) bool {
		c := shadeCanvas(t)
		draw(c)
		return c.SetDepth(x, y, z, engine.RGB(9, 9, 9))
	}
	lo, hi := 0.0, 1000.0
	if drawn(hi) {
		return math.Inf(1)
	}
	for range 60 {
		if mid := (lo + hi) / 2; drawn(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi
}

// The depth stored along an edge is its distance from the camera at the place the pixel
// is, perspective and all: 1/z runs evenly along the line, and the ends are exact.
func TestDrawDepthPixelsStoresTheDepthAlongTheEdge(t *testing.T) {
	cam := Camera{FOV: math.Pi / 2, Near: 0.5}
	// A level edge from 2 to 6 away, from the left of the middle to the right of it.
	m := edgeMesh(geometry.Vector3{X: -1, Z: 2}, geometry.Vector3{X: 1, Z: 6})
	draw := func(c *engine.Canvas) { m.DrawDepthPixels(c, cam, pixelWire) }

	line := shadeCanvas(t)
	m.DrawPixels(line, cam, pixelWire)
	var cols []int
	row := 0
	for p := range pixelsOf(line) {
		cols = append(cols, p[0])
		row = p[1]
	}
	first, last := cols[0], cols[0]
	for _, x := range cols {
		first, last = min(first, x), max(last, x)
	}
	if last-first < 12 || len(cols) != last-first+1 {
		t.Fatalf("wanted a level line of at least 13 pixels, got %d pixels from %d to %d", len(cols), first, last)
	}

	for _, x := range []int{first, first + 3, (first + last) / 2, last - 2, last} {
		tt := float64(x-first) / float64(last-first)
		want := 1 / ((1-tt)/2 + tt/6)
		if got := pixelDepthOf(t, draw, x, row); math.Abs(got-want) > 1e-6 {
			t.Errorf("column %d (%.2f along): wanted depth %v, got %v", x, tt, want, got)
		}
	}
}

// Where an edge runs far off the screen, the part that is drawn still has the depth it
// has on the whole edge: 3 where it crosses the middle of the screen.
func TestDrawDepthPixelsKeepsDepthWhenAnEdgeIsFarOffScreen(t *testing.T) {
	for name, m := range map[string]Mesh{
		"sideways": edgeMesh(geometry.Vector3{X: -1e9, Z: 1}, geometry.Vector3{X: 1e9, Z: 5}),
		"upwards":  edgeMesh(geometry.Vector3{Y: -1e9, Z: 1}, geometry.Vector3{Y: 1e9, Z: 5}),
	} {
		t.Run(name, func(t *testing.T) {
			draw := func(c *engine.Canvas) { m.DrawDepthPixels(c, shadeCam(), pixelWire) }
			plain := shadeCanvas(t)
			m.DrawPixels(plain, shadeCam(), pixelWire)
			pixels := pixelsOf(plain)
			if len(pixels) < 40 {
				t.Fatalf("wanted a line across the screen, got %d pixels", len(pixels))
			}
			n := 0
			for p := range pixels {
				if n++; n%9 != 0 {
					continue // each probe redraws the canvas many times
				}
				if got := pixelDepthOf(t, draw, p[0], p[1]); math.Abs(got-3) > 1e-6 {
					t.Fatalf("pixel %v: wanted depth 3 where the edge crosses the middle, got %v", p, got)
				}
			}
		})
	}
}

// An edge that runs almost into the camera projects absurdly far away, so it is cut back
// to the screen, and the depth at the cut must be the depth the edge really has there,
// not that of an end. This edge goes from 5 away to a hair from the camera, and the part
// on the screen ends where it leaves it, nearer than 5. Where a column of pixels is on the
// edge is found independently, by asking Camera.Project along the edge.
func TestDrawDepthPixelsHasTheRealDepthAtACutEnd(t *testing.T) {
	cam := Camera{FOV: math.Pi / 2, Near: 1e-12}
	a, b := geometry.Vector3{Z: 5}, geometry.Vector3{X: 10, Z: 1e-9}
	at := func(l float64) geometry.Vector3 { return a.Add(b.Sub(a).Scale(l)) }
	const w, h = shadeW, shadeH

	// Once with the end that is cut last and once first.
	for name, m := range map[string]Mesh{"cut end last": edgeMesh(a, b), "cut end first": edgeMesh(b, a)} {
		t.Run(name, func(t *testing.T) {
			draw := func(c *engine.Canvas) { m.DrawDepthPixels(c, cam, pixelWire) }
			line := shadeCanvas(t)
			m.DrawPixels(line, cam, pixelWire)
			last, row := 0, 0
			for p := range pixelsOf(line) {
				if p[0] > last {
					last, row = p[0], p[1]
				}
			}
			if last < 60 {
				t.Fatalf("wanted the edge to run to the right of the screen, got its last pixel at column %d", last)
			}

			for _, col := range []int{50, 60, last - 2, last} {
				// Screen x grows with how far along the edge a point is: bisect for the column's middle.
				lo, hi := 0.0, 1.0
				for range 200 {
					mid := (lo + hi) / 2
					if x, _, ok := cam.Project(at(mid), w, h); ok && x < float64(col)+0.5 {
						lo = mid
					} else {
						hi = mid
					}
				}
				want := at(lo).Z
				if got := pixelDepthOf(t, draw, col, row); math.Abs(got-want) > 0.06*want {
					t.Errorf("column %d: wanted about the depth %v the edge has there, got %v", col, want, got)
				}
			}
		})
	}
}

// An edge from behind the camera to in front of it is cut at the near plane and the
// part in front is drawn, a level line across the middle of the screen here.
func TestDrawPixelsCutsAnEdgeAtTheNearPlane(t *testing.T) {
	m := edgeMesh(geometry.Vector3{X: -3, Z: -2}, geometry.Vector3{X: 3, Z: 6})
	for _, depth := range []bool{false, true} {
		c := shadeCanvas(t)
		if depth {
			m.DrawDepthPixels(c, shadeCam(), pixelWire)
		} else {
			m.DrawPixels(c, shadeCam(), pixelWire)
		}

		pixels := pixelsOf(c)
		if len(pixels) < 20 {
			t.Fatalf("depth %v: wanted the part in front of the camera drawn, got %d pixels", depth, len(pixels))
		}
		for p := range pixels {
			if p[1] != c.Height()/2 {
				t.Fatalf("depth %v: wanted a level line across the middle row %d, got a pixel at %v", depth, c.Height()/2, p)
			}
		}
	}
}

// A mesh that cannot be drawn is not drawn, as for Draw.
func TestPixelWireframesOfBadMeshesDrawNothing(t *testing.T) {
	bad := edgeMesh(geometry.Vector3{Z: 4}, geometry.Vector3{X: 1, Z: 4})
	bad.Edges = append(bad.Edges, [2]int{0, 9}, [2]int{-1, 1})
	badTransform := bad.Clone()
	badTransform.Transform.Scale = -1
	nan := edgeMesh(geometry.Vector3{X: math.NaN(), Z: 4}, geometry.Vector3{Z: 4})
	behind := edgeMesh(geometry.Vector3{Z: -4}, geometry.Vector3{X: 1, Z: -2})

	for _, depth := range []bool{false, true} {
		draw := func(m Mesh, c *engine.Canvas) {
			if depth {
				m.DrawDepthPixels(c, shadeCam(), pixelWire)
			} else {
				m.DrawPixels(c, shadeCam(), pixelWire)
			}
		}
		for name, m := range map[string]Mesh{"transform": badTransform, "NaN vertex": nan, "behind the camera": behind} {
			c := shadeCanvas(t)
			draw(m, c)
			if n := len(pixelsOf(c)); n != 0 {
				t.Errorf("depth %v, %s: wanted nothing drawn, got %d pixels", depth, name, n)
			}
		}
		c := shadeCanvas(t)
		draw(bad, c) // edges with missing vertices are skipped, the good edge is drawn
		if n := len(pixelsOf(c)); n < 3 {
			t.Errorf("depth %v: wanted the good edge drawn, got %d pixels", depth, n)
		}
	}
}

// BenchmarkDrawWire measures the four ways of drawing a wireframe on the cube3d scene.
// The pixel ones have a buffer of their own: clearing a buffer clears its canvas too, which
// would add to the cost of the others.
func BenchmarkDrawWire(b *testing.B) {
	floor, cubes, cam, _ := benchScene()
	for _, size := range []struct{ w, h int }{{80, 24}, {200, 60}} {
		cells, err := engine.NewFrameBuffer(size.w, size.h)
		if err != nil {
			b.Fatal(err)
		}
		pixels, err := engine.NewFrameBuffer(size.w, size.h)
		if err != nil {
			b.Fatal(err)
		}
		canvas := pixels.Canvas()
		cell, color := engine.Cell{Character: '#'}, engine.RGB(10, 250, 10)
		for _, bench := range []struct {
			name string
			fb   *engine.FrameBuffer
			draw func()
		}{
			{"Draw", cells, func() {
				floor.Draw(cells, cam, cell)
				for _, c := range cubes {
					c.Draw(cells, cam, cell)
				}
			}},
			{"DrawDepth", cells, func() {
				floor.DrawDepth(cells, cam, cell)
				for _, c := range cubes {
					c.DrawDepth(cells, cam, cell)
				}
			}},
			{"DrawPixels", pixels, func() {
				floor.DrawPixels(canvas, cam, color)
				for _, c := range cubes {
					c.DrawPixels(canvas, cam, color)
				}
			}},
			{"DrawDepthPixels", pixels, func() {
				floor.DrawDepthPixels(canvas, cam, color)
				for _, c := range cubes {
					c.DrawDepthPixels(canvas, cam, color)
				}
			}},
		} {
			b.Run(fmt.Sprintf("%s %dx%d", bench.name, size.w, size.h), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					bench.fb.Clear()
					bench.draw()
				}
			})
		}
	}
}

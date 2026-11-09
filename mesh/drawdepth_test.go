package mesh

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

var wireCell = engine.Cell{Character: '#', Foreground: engine.Palette(46)}

// cellsOf is the set of cells a frame has drawn in.
func cellsOf(fb *engine.FrameBuffer) map[[2]int]bool {
	out := map[[2]int]bool{}
	for y := range fb.Height() {
		for x := range fb.Width() {
			if fb.At(x, y) != engine.BlankCell {
				out[[2]int{x, y}] = true
			}
		}
	}
	return out
}

func solidCube(pos geometry.Vector3, size float64) Mesh {
	m := Cube(size)
	m.Transform = Transform{Position: pos, Yaw: 0.6, Pitch: -0.3}
	return m.withColor(engine.Palette(208))
}

// A wireframe behind a solid is hidden exactly where the solid covers it; one in front
// of it is drawn whole. Both are drawn after the solid, where Draw would have put them
// on top either way.
func TestDrawDepthIsHiddenByNearerSolidsAndHidesFartherOnes(t *testing.T) {
	solid := solidCube(geometry.Vector3{Z: 5}, 2)
	tests := []struct {
		name     string
		wire     Mesh
		inFront  bool
		wantBoth bool // the wire and the solid must overlap, or the test shows nothing
	}{
		{"behind", solidCube(geometry.Vector3{X: 1.2, Y: 0.3, Z: 9}, 2), false, true},
		{"in front", solidCube(geometry.Vector3{X: 0.3, Z: 2.8}, 1), true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alone, wireAlone, both := newShadeBuffer(t), newShadeBuffer(t), newShadeBuffer(t)
			solid.DrawShaded(alone, shadeCam(), goldenSun)
			tt.wire.Draw(wireAlone, shadeCam(), wireCell)
			solid.DrawShaded(both, shadeCam(), goldenSun)
			tt.wire.DrawDepth(both, shadeCam(), wireCell)
			s, w := cellsOf(alone), cellsOf(wireAlone)

			overlap, outside := 0, 0
			for c := range w {
				if s[c] {
					overlap++
				} else {
					outside++
				}
			}
			if overlap < 3 || outside < 3 {
				t.Fatalf("wanted the wire to overlap the solid and stick out of it, got %d and %d cells", overlap, outside)
			}
			for y := range both.Height() {
				for x := range both.Width() {
					c := [2]int{x, y}
					want := engine.BlankCell
					switch {
					case w[c] && (tt.inFront || !s[c]):
						want = wireCell // nothing nearer covers it
					case s[c]:
						want = alone.At(x, y)
					}
					if got := both.At(x, y); got != want {
						t.Fatalf("cell (%d,%d): wanted %+v, got %+v\n%s", x, y, want, got, both.String())
					}
				}
			}
		})
	}
}

// Whatever order a scene is drawn in, it comes out the same: that is what depth is for.
func TestDrawDepthDoesNotDependOnDrawingOrder(t *testing.T) {
	rng := rand.New(rand.NewPCG(41, 42))
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
		draw := func(order []int) string {
			fb := newShadeBuffer(t)
			for _, i := range order {
				it := items[i]
				if it.wire {
					cell := engine.Cell{Character: '#', Foreground: it.m.Material.Color}
					it.m.DrawDepth(fb, shadeCam(), cell)
				} else {
					it.m.DrawShaded(fb, shadeCam(), goldenSun)
				}
			}
			return picture(fb)
		}

		order := rng.Perm(len(items))
		first := draw(order)
		for range 2 {
			if again := draw(rng.Perm(len(items))); again != first {
				t.Fatalf("scene %d: wanted the same picture in any order, got\n%s\nand\n%s", scene, first, again)
			}
		}
	}
}

// On a blank frame DrawDepth draws the cells Draw does, for anything: cubes, grids and
// spheres, near and far, across the near plane and with edges far off the screen.
func TestDrawDepthDrawsTheCellsDrawDoes(t *testing.T) {
	rng := rand.New(rand.NewPCG(43, 44))
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

		plain, deep := newShadeBuffer(t), newShadeBuffer(t)
		m.Draw(plain, cam, wireCell)
		m.DrawDepth(deep, cam, wireCell)

		if picture(plain) != picture(deep) {
			t.Fatalf("scene %d: wanted DrawDepth to draw what Draw does\n%s\nwanted\n%s", scene, deep.String(), plain.String())
		}
	}
}

// depthOf finds the depth that drawing leaves at cell (x, y), by asking on fresh frames
// whether something at a given distance would be drawn there: it is, exactly when it is
// nearer than what is there. It gives +Inf for a cell with nothing in it.
func depthOf(t *testing.T, draw func(fb *engine.FrameBuffer), x, y int) float64 {
	t.Helper()
	drawn := func(z float64) bool {
		fb := newShadeBuffer(t)
		draw(fb)
		return fb.SetDepth(x, y, z, engine.Cell{Character: '?'})
	}
	lo, hi := 0.0, 1000.0
	if drawn(hi) {
		return math.Inf(1)
	}
	for range 60 {
		if mid := (lo + hi) / 2; drawn(mid) {
			lo = mid // there is something farther than mid
		} else {
			hi = mid
		}
	}
	return hi
}

// edgeMesh is a mesh of one edge between two points.
func edgeMesh(a, b geometry.Vector3) Mesh {
	return Mesh{Vertices: []geometry.Vector3{a, b}, Edges: [][2]int{{0, 1}}}
}

// The two ways of drawing differ in what they leave behind. Draw paints over a nearer
// solid and puts nothing in the depth buffer, so a farther solid drawn after it covers
// it. DrawDepth stores the depth, so a farther solid cannot.
func TestDrawIgnoresDepthAndDrawDepthKeepsIt(t *testing.T) {
	front := solidCube(geometry.Vector3{X: 0.3, Z: 2.8}, 1)
	far := solidCube(geometry.Vector3{X: 1.2, Y: 0.3, Z: 9}, 2)
	wire := func(m Mesh, depth bool) func(*engine.FrameBuffer) {
		return func(fb *engine.FrameBuffer) {
			if depth {
				m.DrawDepth(fb, shadeCam(), wireCell)
			} else {
				m.Draw(fb, shadeCam(), wireCell)
			}
		}
	}

	// A wireframe behind a solid, drawn after it: Draw puts it on top.
	for _, depth := range []bool{false, true} {
		fb := newShadeBuffer(t)
		front.DrawShaded(fb, shadeCam(), goldenSun)
		wire(far, depth)(fb)
		over := 0
		for y := range shadeH {
			for x := range shadeW {
				if fb.At(x, y) == wireCell && cellsOf(solidOnly(t, front))[[2]int{x, y}] {
					over++
				}
			}
		}
		if (over > 0) == depth {
			t.Errorf("depth %v: wanted the wire to show over the solid only for Draw, got %d cells", depth, over)
		}
	}

	// A wireframe in front, then a solid behind it: Draw's lines are covered, DrawDepth's are not.
	for _, depth := range []bool{false, true} {
		fb := newShadeBuffer(t)
		wire(front, depth)(fb)
		far.DrawShaded(fb, shadeCam(), goldenSun)
		kept := 0
		for c := range cellsOf(solidOnly(t, far)) {
			if fb.At(c[0], c[1]) == wireCell {
				kept++
			}
		}
		if (kept > 0) != depth {
			t.Errorf("depth %v: wanted the wire kept in front of the farther solid only for DrawDepth, got %d cells", depth, kept)
		}
	}
}

func solidOnly(t *testing.T, m Mesh) *engine.FrameBuffer {
	fb := newShadeBuffer(t)
	m.DrawShaded(fb, shadeCam(), goldenSun)
	return fb
}

// The depth stored along an edge is its distance from the camera along the view, at the
// place the cell is, perspective and all.
func TestDrawDepthStoresTheDepthAlongTheEdge(t *testing.T) {
	cam := Camera{FOV: math.Pi / 2, Near: 0.5}
	edge := func(m Mesh) func(fb *engine.FrameBuffer) {
		return func(fb *engine.FrameBuffer) { m.DrawDepth(fb, cam, wireCell) }
	}
	drawnCells := func(m Mesh) [][2]int {
		fb := newShadeBuffer(t)
		m.Draw(fb, cam, wireCell)
		var out [][2]int
		for c := range cellsOf(fb) {
			out = append(out, c)
		}
		slices.SortFunc(out, func(a, b [2]int) int { return a[0] - b[0] })
		return out
	}

	t.Run("across the view, every cell is as far as the edge", func(t *testing.T) {
		m := edgeMesh(geometry.Vector3{X: -2, Z: 4}, geometry.Vector3{X: 2, Z: 4})
		cells := drawnCells(m)
		if len(cells) < 10 {
			t.Fatalf("wanted a line of cells, got %d", len(cells))
		}
		for _, c := range cells {
			if got := depthOf(t, edge(m), c[0], c[1]); math.Abs(got-4) > 1e-9 {
				t.Fatalf("cell %v: wanted depth 4, got %v", c, got)
			}
		}
	})

	t.Run("straight away from the camera, a single cell at the nearer end", func(t *testing.T) {
		m := edgeMesh(geometry.Vector3{Z: 10}, geometry.Vector3{Z: 2})
		cells := drawnCells(m)
		if len(cells) != 1 {
			t.Fatalf("wanted a single cell, got %v", cells)
		}
		if got := depthOf(t, edge(m), cells[0][0], cells[0][1]); math.Abs(got-2) > 1e-9 {
			t.Errorf("wanted the nearer end's depth, 2, got %v", got)
		}
	})

	t.Run("cut at the near plane, the cut end is as far as the plane", func(t *testing.T) {
		// From behind the camera to 5 away: the part in front starts 0.5 away.
		m := edgeMesh(geometry.Vector3{X: 0.5, Z: -1}, geometry.Vector3{X: 0.5, Z: 5})
		near, far := math.Inf(1), 0.0
		for _, c := range drawnCells(m) {
			d := depthOf(t, edge(m), c[0], c[1])
			near, far = math.Min(near, d), math.Max(far, d)
		}
		if math.Abs(near-0.5) > 1e-9 || math.Abs(far-5) > 1e-9 {
			t.Errorf("wanted depths from 0.5 to 5, got %v to %v", near, far)
		}
	})

	t.Run("slanting away, the middle is nearer than halfway", func(t *testing.T) {
		// Perspective: 1/z runs evenly on the screen, so halfway across a line from 2 to 6
		// away it is 3 away, not 4.
		m := edgeMesh(geometry.Vector3{X: -3, Z: 2}, geometry.Vector3{X: 3, Z: 6})
		cells := drawnCells(m)
		first := depthOf(t, edge(m), cells[0][0], cells[0][1])
		last := depthOf(t, edge(m), cells[len(cells)-1][0], cells[len(cells)-1][1])
		mid := depthOf(t, edge(m), cells[len(cells)/2][0], cells[len(cells)/2][1])
		if math.Abs(first-2) > 1e-9 || math.Abs(last-6) > 1e-9 {
			t.Errorf("wanted the ends at 2 and 6, got %v and %v", first, last)
		}
		if mid < 2.6 || mid > 3.6 {
			t.Errorf("wanted the middle near 3 (perspective), not 4 (even), got %v", mid)
		}
	})
}

// An edge with an end so far off the screen that it is trimmed keeps the depth it has at
// the cells that are left, not the depth of its far ends.
func TestDrawDepthKeepsDepthWhenAnEdgeIsFarOffScreen(t *testing.T) {
	for name, m := range map[string]Mesh{
		// X (or Y) is 0 halfway along, where the edge is 3 from the camera, and it crosses the middle of the screen.
		"sideways": edgeMesh(geometry.Vector3{X: -1e9, Z: 1}, geometry.Vector3{X: 1e9, Z: 5}),
		"upwards":  edgeMesh(geometry.Vector3{Y: -1e9, Z: 1}, geometry.Vector3{Y: 1e9, Z: 5}),
	} {
		t.Run(name, func(t *testing.T) {
			draw := func(fb *engine.FrameBuffer) { m.DrawDepth(fb, shadeCam(), wireCell) }
			plain := newShadeBuffer(t)
			m.Draw(plain, shadeCam(), wireCell)
			cells := cellsOf(plain)
			if len(cells) < 20 {
				t.Fatalf("wanted a line across the screen, got %d cells", len(cells))
			}
			n := 0
			for c := range cells {
				if n++; n%5 != 0 {
					continue // every fifth cell is enough, and each probe redraws the frame
				}
				if got := depthOf(t, draw, c[0], c[1]); math.Abs(got-3) > 1e-6 {
					t.Fatalf("cell %v: wanted depth 3 where the edge crosses the middle, got %v", c, got)
				}
			}
		})
	}
}

// Pixel solids hide a wireframe once they are composed, and a wireframe in front of them is drawn.
func TestDrawDepthSeesPixelSolids(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	solid := solidCube(geometry.Vector3{Z: 5}, 2)
	pixels := func(fb *engine.FrameBuffer) {
		canvas := fb.Canvas()
		solid.DrawShadedPixels(canvas, shadeCam(), goldenSun)
		canvas.Compose()
	}
	alone := newShadeBuffer(t)
	pixels(alone)
	s := cellsOf(alone)

	for _, tt := range []struct {
		name    string
		wire    Mesh
		inFront bool
	}{
		{"behind", solidCube(geometry.Vector3{X: 1.2, Y: 0.3, Z: 9}, 2), false},
		{"in front", solidCube(geometry.Vector3{X: 0.3, Z: 2.8}, 1), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wireAlone, both := newShadeBuffer(t), newShadeBuffer(t)
			tt.wire.Draw(wireAlone, shadeCam(), wireCell)
			pixels(both)
			tt.wire.DrawDepth(both, shadeCam(), wireCell)

			covered := 0
			for c := range cellsOf(wireAlone) {
				if s[c] {
					covered++
				}
				want := both.At(c[0], c[1]) == wireCell
				if wantDrawn := tt.inFront || !s[c]; want != wantDrawn {
					t.Fatalf("cell %v: wanted the wire drawn %v, got %v\n%s", c, wantDrawn, want, both.String())
				}
			}
			if covered < 3 {
				t.Fatalf("wanted the wire to overlap the pixels, got %d cells", covered)
			}
		})
	}
}

// A mesh that cannot be drawn is not drawn, as for Draw.
func TestDrawDepthOfBadMeshesDrawsNothing(t *testing.T) {
	bad := edgeMesh(geometry.Vector3{Z: 4}, geometry.Vector3{X: 1, Z: 4})
	bad.Edges = append(bad.Edges, [2]int{0, 9}, [2]int{-1, 1})
	badTransform := bad.Clone()
	badTransform.Transform.Scale = -1
	nan := edgeMesh(geometry.Vector3{X: math.NaN(), Z: 4}, geometry.Vector3{Z: 4})
	behind := edgeMesh(geometry.Vector3{Z: -4}, geometry.Vector3{X: 1, Z: -2})

	for name, m := range map[string]Mesh{"transform": badTransform, "NaN vertex": nan, "behind the camera": behind} {
		fb := newShadeBuffer(t)
		m.DrawDepth(fb, shadeCam(), wireCell)
		if len(cellsOf(fb)) != 0 {
			t.Errorf("%s: wanted nothing drawn, got %d cells", name, len(cellsOf(fb)))
		}
	}
	fb := newShadeBuffer(t)
	bad.DrawDepth(fb, shadeCam(), wireCell) // edges with missing vertices are skipped, the good edge is drawn
	if len(cellsOf(fb)) < 3 {
		t.Errorf("wanted the good edge drawn, got %d cells", len(cellsOf(fb)))
	}
}

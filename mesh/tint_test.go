package mesh

import (
	"math"
	"testing"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// mix8 is part t of the way from a to b on one channel, rounded: written here, not taken from
// the engine, so that the two cannot agree by being the same code.
func mix8(a, b uint8, t float64) uint8 {
	return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t))
}

// mixed is the color part t of the way from behind to color. A color that is not set is read as
// black, as the engine reads it.
func mixed(behind, color engine.Color, t float64) engine.Color {
	br, bg, bb := behind.RGB()
	cr, cg, cb := color.RGB()
	return engine.RGB(mix8(br, cr, t), mix8(bg, cg, t), mix8(bb, cb, t))
}

var glass = engine.RGB(0, 100, 240)

// glassPane is a pane of glass facing the camera at depth 3, 2 by 2, that shows 0.25 of its color.
func glassPane() Mesh {
	m := wallAt(3, 2, 2, glass)
	m.Material.Opacity = 0.25
	return m
}

// gridPane is a pane of glass like glassPane, cut into n by m squares of two triangles each, which
// is how a floor or a sheet of water is made: a great many edges that triangles share.
func gridPane(n, m int) Mesh {
	pane := Mesh{Material: Material{Color: glass, Opacity: 0.25}, DoubleSided: true}
	for j := 0; j <= m; j++ {
		for i := 0; i <= n; i++ {
			pane.Vertices = append(pane.Vertices, geometry.Vector3{X: -1 + 2*float64(i)/float64(n), Y: -1 + 2*float64(j)/float64(m), Z: 3})
		}
	}
	for j := range m {
		for i := range n {
			a := j*(n+1) + i
			pane.Faces = append(pane.Faces, [3]int{a, a + n + 1, a + n + 2}, [3]int{a, a + n + 2, a + 1})
		}
	}
	return pane
}

// twoColored is a backdrop at depth 6 whose two triangles are different colors.
func twoColored() Mesh {
	m := wallAt(6, 9, 6, pureBlue)
	m.FaceColors = []engine.Color{engine.RGB(200, 100, 40), engine.RGB(30, 220, 90)}
	return m
}

// checkGlass draws the backdrop, then the pane, and holds every cell and pixel to what a pane of
// glass makes of what was behind it. The pane's footprint is where it makes a mark on an empty
// frame; there each cell is the cell that was behind, its character and attributes kept, its
// foreground and background each mixed with the pane's color by its opacity, and a pixel is mixed
// the same way. Outside it nothing changed. surface is the color the pane has by then, after the
// light and fog.
func checkGlass(t *testing.T, cam Camera, light Light, backdrop []Mesh, pane Mesh, surface engine.Color, opacity float64) {
	t.Helper()
	behindCells, behindPixels := drawLit(t, cam, light, backdrop...)
	aloneCells, alonePixels := drawLit(t, cam, light, pane)
	bothCells, bothPixels := drawLit(t, cam, light, append(append([]Mesh{}, backdrop...), pane)...)

	covered := 0
	for i := range bothCells {
		want := behindCells[i]
		if aloneCells[i].Foreground != (engine.Color{}) {
			covered++
			want.Foreground = mixed(behindCells[i].Foreground, surface, opacity)
			want.Background = mixed(behindCells[i].Background, surface, opacity)
		}
		if bothCells[i] != want {
			t.Fatalf("cell (%d, %d): behind it %+v: wanted %+v, got %+v", i%shadeW, i/shadeW, behindCells[i], want, bothCells[i])
		}
	}
	if covered < 60 {
		t.Fatalf("wanted a pane of at least 60 cells, got %d", covered)
	}

	covered = 0
	for i := range bothPixels {
		want := behindPixels[i]
		if alonePixels[i] != (engine.Color{}) {
			covered++
			want = mixed(behindPixels[i], surface, opacity)
		}
		if bothPixels[i] != want {
			t.Fatalf("pixel (%d, %d): behind it %v: wanted %v, got %v", i%shadeW, i/shadeW, behindPixels[i], want, bothPixels[i])
		}
	}
	if covered < 120 {
		t.Fatalf("wanted a pane of at least 120 pixels, got %d", covered)
	}
}

// A face that can be seen through is mixed over what is behind it, cell by cell and pixel by pixel:
// over a backdrop of one color, one of two, and over nothing, where an empty cell or pixel is read as
// black; flat and smooth, lit, and in fog.
func TestGlassIsMixedOverWhatIsBehindIt(t *testing.T) {
	plain := wallAt(6, 9, 6, engine.RGB(200, 100, 40))
	smooth := glassPane()
	smooth.Normals = []geometry.Vector3{{Z: -1}, {Z: -1}, {Z: -1}, {Z: -1}}
	half := Ambient{Level: 0.5}
	foggy := shadeCam()
	foggy.Fog = &Fog{Color: pureRed, Start: 1, End: 5}

	for _, tt := range []struct {
		name     string
		cam      Camera
		light    Light
		backdrop []Mesh
		pane     Mesh
		surface  engine.Color
	}{
		{"over one color", shadeCam(), Ambient{Level: 1}, []Mesh{plain}, glassPane(), glass},
		{"over two colors", shadeCam(), Ambient{Level: 1}, []Mesh{twoColored()}, glassPane(), glass},
		{"over nothing", shadeCam(), Ambient{Level: 1}, nil, glassPane(), glass},
		{"a pane of 70 triangles", shadeCam(), Ambient{Level: 1}, []Mesh{twoColored()}, gridPane(7, 5), glass},
		{"a pane of 128 triangles over nothing", shadeCam(), Ambient{Level: 1}, nil, gridPane(8, 8), glass},
		{"lit", shadeCam(), half, []Mesh{twoColored()}, glassPane(), engine.Shade(glass, 0.5)},
		{"smooth and lit", shadeCam(), half, []Mesh{twoColored()}, smooth, engine.Shade(glass, 0.5)},
		// The pane is 3 away, in fog that is a quarter of the way gone.
		{"in fog", foggy, half, []Mesh{twoColored()}, glassPane(), foggy.Fog.Apply(engine.Shade(glass, 0.5), 3)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checkGlass(t, tt.cam, tt.light, tt.backdrop, tt.pane, tt.surface, 0.25)
		})
	}

	// Over nothing, what is behind reads as black: a quarter of the color, as worked out by hand.
	cells, pixels := drawMeshes(t, shadeCam(), glassPane())
	quarter := engine.RGB(0, 25, 60)
	found := false
	for i, c := range cells {
		if c.Foreground != (engine.Color{}) {
			found = true
			if c.Foreground != quarter || c.Background != quarter {
				t.Fatalf("cell %d: wanted a block of %v, got %+v", i, quarter, c)
			}
		}
	}
	for _, p := range pixels {
		if p != (engine.Color{}) && p != quarter {
			t.Fatalf("wanted pixels of %v over nothing, got %v", quarter, p)
		}
	}
	if !found {
		t.Fatal("wanted the pane to be seen over nothing")
	}
}

// Glass over a drawn character keeps it, its attributes too: only the colors are mixed.
func TestGlassKeepsTheCharacterBehindIt(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	fb := newShadeBuffer(t)
	for y := range fb.Height() {
		for x := range fb.Width() {
			fb.Set(x, y, engine.Cell{Character: 'x', Foreground: engine.RGB(200, 100, 40), Background: engine.RGB(10, 20, 30), Attrs: engine.Bold})
		}
	}
	glassPane().DrawShaded(fb, shadeCam(), Ambient{Level: 1})
	want := engine.Cell{Character: 'x', Foreground: mixed(engine.RGB(200, 100, 40), glass, 0.25), Background: mixed(engine.RGB(10, 20, 30), glass, 0.25), Attrs: engine.Bold}
	changed := 0
	for y := range fb.Height() {
		for x := range fb.Width() {
			if c := fb.At(x, y); c != (engine.Cell{Character: 'x', Foreground: engine.RGB(200, 100, 40), Background: engine.RGB(10, 20, 30), Attrs: engine.Bold}) {
				changed++
				if c != want {
					t.Fatalf("(%d, %d): wanted %+v, got %+v", x, y, want, c)
				}
			}
		}
	}
	if changed < 60 {
		t.Errorf("wanted a pane of at least 60 cells, got %d", changed)
	}
}

// A see-through face is tested against depth and does not store it. A solid in front of the glass
// hides it, and glass behind a solid is not seen; but a solid between the glass and what is behind
// it replaces the glass, since its depth was never stored, which is why the solids are drawn first.
func TestGlassIsTestedAgainstDepthButDoesNotStoreIt(t *testing.T) {
	wall := wallAt(6, 9, 6, engine.RGB(200, 100, 40))
	pane := glassPane() // 3 away
	solidIn := func(z float64) Mesh { return wallAt(z, 20, 20, engine.RGB(30, 220, 90)) }

	footprint, _ := drawMeshes(t, shadeCam(), pane)
	tintedCells, tintedPixels := drawMeshes(t, shadeCam(), wall, pane)
	wallCells, wallPixels := drawMeshes(t, shadeCam(), wall)

	// The glass behind a solid wall is hidden by it, whichever order they are drawn in.
	nearWallCells, nearWallPixels := drawMeshes(t, shadeCam(), solidIn(2), pane)
	soloCells, soloPixels := drawMeshes(t, shadeCam(), solidIn(2))
	if !slicesEqual(nearWallCells, soloCells) || !slicesEqual(nearWallPixels, soloPixels) {
		t.Errorf("wanted glass behind a nearer solid to be hidden")
	}
	// In front of the wall it is mixed in (checked cell by cell above), and so it differs from the wall.
	if slicesEqual(tintedCells, wallCells) || slicesEqual(tintedPixels, wallPixels) {
		t.Errorf("wanted the glass to show in front of the wall")
	}

	// A solid drawn after the glass, farther than it, but nearer than the wall: replaces the glass.
	between, betweenPixels := drawMeshes(t, shadeCam(), wall, pane, solidIn(4))
	alone, alonePixels := drawMeshes(t, shadeCam(), wall, solidIn(4))
	if !slicesEqual(between, alone) || !slicesEqual(betweenPixels, alonePixels) {
		t.Errorf("wanted a solid between the glass and the wall to replace the glass")
	}
	// A solid drawn after the glass, nearer than it, hides it too.
	nearer, nearerPixels := drawMeshes(t, shadeCam(), wall, pane, solidIn(2))
	nearerAlone, nearerAlonePixels := drawMeshes(t, shadeCam(), wall, solidIn(2))
	if !slicesEqual(nearer, nearerAlone) || !slicesEqual(nearerPixels, nearerAlonePixels) {
		t.Errorf("wanted a solid in front of the glass to hide it")
	}
	if len(footprint) == 0 {
		t.Fatal("wanted a frame")
	}
}

// Two panes are mixed in the order they are drawn: one over the other, and the other way round
// is not the same. Drawn far to near, the nearer shows over the farther.
func TestPanesAreMixedInTheOrderTheyAreDrawn(t *testing.T) {
	wall := wallAt(8, 12, 8, engine.RGB(200, 100, 40))
	far, near := glassPane(), glassPane()
	far.Transform.Position = geometry.Vector3{Z: 1} // 4 away
	far.Material.Color = pureRed
	near.Material.Color = pureGreen

	farFirst, farFirstPixels := drawMeshes(t, shadeCam(), wall, far, near)
	nearFirst, nearFirstPixels := drawMeshes(t, shadeCam(), wall, near, far)
	if slicesEqual(farFirst, nearFirst) || slicesEqual(farFirstPixels, nearFirstPixels) {
		t.Fatalf("wanted the order of two panes to change what is drawn")
	}

	// Far to near, at a pixel where both are: the wall, then red over it, then green over that.
	behind := engine.RGB(200, 100, 40)
	want := mixed(mixed(behind, pureRed, 0.25), pureGreen, 0.25)
	middle := (shadeH)*shadeW + shadeW/2 // the middle pixel row of the canvas, at the middle column
	if got := farFirstPixels[middle]; got != want {
		t.Errorf("wanted the near pane over the far one, %v, got %v", want, got)
	}
}

// An Opacity that is not between 0 and 1 is a solid mesh, as 0 is, and one that is draws something else.
func TestOpacityOutsideZeroToOneIsSolid(t *testing.T) {
	solid := wallAt(3, 2, 2, glass)
	wantCells, wantPixels := drawMeshes(t, shadeCam(), twoColored(), solid)
	for _, o := range []float64{0, 1, 1.5, 100, -0.5, math.NaN(), math.Inf(1), math.Inf(-1)} {
		m := solid
		m.Material.Opacity = o
		cells, pixels := drawMeshes(t, shadeCam(), twoColored(), m)
		if !slicesEqual(cells, wantCells) || !slicesEqual(pixels, wantPixels) {
			t.Errorf("Opacity %v: wanted what a solid mesh draws", o)
		}
	}
	for _, o := range []float64{0.01, 0.5, 0.99} {
		m := solid
		m.Material.Opacity = o
		cells, pixels := drawMeshes(t, shadeCam(), twoColored(), m)
		if slicesEqual(cells, wantCells) || slicesEqual(pixels, wantPixels) {
			t.Errorf("Opacity %v: wanted something other than a solid mesh", o)
		}
	}
}

// A see-through face is skipped from behind, as a solid's is, and drawn when the mesh is DoubleSided,
// lit as the side the camera sees.
func TestGlassSeenFromBehind(t *testing.T) {
	back := glassPane()
	back.DoubleSided = false
	back.Faces = [][3]int{{0, 2, 1}, {0, 3, 2}} // wound the other way: facing away from the camera
	wall := wallAt(6, 9, 6, engine.RGB(200, 100, 40))

	wallCells, wallPixels := drawMeshes(t, shadeCam(), wall)
	cells, pixels := drawMeshes(t, shadeCam(), wall, back)
	if !slicesEqual(cells, wallCells) || !slicesEqual(pixels, wallPixels) {
		t.Errorf("wanted a pane that faces away to be skipped")
	}

	back.DoubleSided = true
	cells, pixels = drawMeshes(t, shadeCam(), wall, back)
	if slicesEqual(cells, wallCells) || slicesEqual(pixels, wallPixels) {
		t.Errorf("wanted a double-sided pane that faces away to be seen")
	}
	// Seen from the front or the back, the same pane makes the same picture.
	front := glassPane()
	frontCells, frontPixels := drawMeshes(t, shadeCam(), wall, front)
	if !slicesEqual(cells, frontCells) || !slicesEqual(pixels, frontPixels) {
		t.Errorf("wanted the same picture from either side")
	}
}

// A hole stays a hole in a see-through face: the pixels of the picture are mixed over what is behind,
// and a pixel that is not there leaves it alone.
func TestHolesInGlassLeaveWhatIsBehindAlone(t *testing.T) {
	pic := bitmap.New(2, 1)
	pic.Set(0, 0, pureRed) // the right pixel is a hole
	pane := glassPane()
	pane.FaceTexCoords = [][3]geometry.Vector2{{uv(0, 1), uv(0, 0), uv(1, 0)}, {uv(0, 1), uv(1, 0), uv(1, 1)}}
	pane.Material.Texture = pic
	wall := twoColored()

	wallCells, wallPixels := drawMeshes(t, shadeCam(), wall)
	cells, pixels := drawMeshes(t, shadeCam(), wall, pane)
	whole, _ := drawMeshes(t, shadeCam(), glassPane()) // where the pane is
	x0, x1 := shadeW, -1
	for i, c := range whole {
		if c.Foreground != (engine.Color{}) {
			x0, x1 = min(x0, i%shadeW), max(x1, i%shadeW)
		}
	}
	middle := (x0 + x1) / 2
	pictured := 0
	for i := range cells {
		x, y := i%shadeW, i/shadeW
		pixel := 2*y*shadeW + x
		switch {
		case whole[i].Foreground == (engine.Color{}) || (x >= middle-1 && x <= middle+1):
			// Outside the pane, or too near the edge of the pixel to say which it is.
			if whole[i].Foreground == (engine.Color{}) && (cells[i] != wallCells[i] || pixels[pixel] != wallPixels[pixel]) {
				t.Fatalf("cell %d: wanted what is outside the pane left alone", i)
			}
		case x > middle+1: // the hole
			if cells[i] != wallCells[i] || pixels[pixel] != wallPixels[pixel] {
				t.Fatalf("cell %d: wanted the hole to leave what is behind alone, got %+v", i, cells[i])
			}
		default: // the red pixel of the picture, mixed over the wall
			pictured++
			if cells[i].Foreground != mixed(wallCells[i].Foreground, pureRed, 0.25) {
				t.Fatalf("cell %d: wanted the red pixel mixed over %v, got %v", i, wallCells[i].Foreground, cells[i].Foreground)
			}
		}
	}
	if pictured < 20 {
		t.Errorf("wanted at least 20 cells of the picture, got %d", pictured)
	}
}

func TestDrawingGlassAllocatesNothingOnceWarm(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector makes sync.Pool drop items, so allocation counts mean nothing")
	}
	t.Setenv("TERM_PROGRAM", "")
	m := Sphere(1.6, 10, 6)
	m.Transform = Transform{Position: geometry.Vector3{Z: 4}}
	m.Material.Opacity = 0.4
	flat := m.Clone()
	flat.Normals = nil
	light := sunAndFill(geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}, 0.2)
	fb := newShadeBuffer(t)
	cam := shadeCam()
	foggy := cam
	foggy.Fog = &Fog{Color: pureBlue, Start: 1, End: 8}
	for name, draw := range map[string]func(){
		"smooth cells":  func() { m.DrawShaded(fb, cam, light) },
		"smooth pixels": func() { m.DrawShadedPixels(fb.Canvas(), cam, light) },
		"flat cells":    func() { flat.DrawShaded(fb, cam, light) },
		"flat pixels":   func() { flat.DrawShadedPixels(fb.Canvas(), cam, light) },
		"fog cells":     func() { m.DrawShaded(fb, foggy, light) },
		"fog pixels":    func() { m.DrawShadedPixels(fb.Canvas(), foggy, light) },
	} {
		draw()
		if n := testing.AllocsPerRun(20, draw); n != 0 {
			t.Errorf("%s: wanted no allocation, got %v", name, n)
		}
	}
}

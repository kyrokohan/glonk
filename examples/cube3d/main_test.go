package main

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/mesh"
)

func newFrame(t *testing.T, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

// row returns the text of row y.
func row(fb *engine.FrameBuffer, y int) string {
	return strings.Split(fb.String(), "\n")[y]
}

// B flips the pixel edges once per press, and the setting then stays across frames.
func TestEdgeKeyFlipsTheEdgeStyleOncePerPress(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "") // the default style depends on the terminal
	g := newCubeGame()
	g.halfBlocks = true
	kb := engine.NewKeyboard()

	fb := newFrame(t, 80, 24)
	frame := func(pressB bool) string {
		if pressB {
			kb.Press('b')
		}
		if err := g.Update(1.0/30, kb.Input()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		kb.EndFrame()
		kb.Release('b')
		fb.Clear()
		g.Draw(fb)
		return row(fb, 1)
	}

	if got := frame(false); !strings.Contains(got, "exact edges") {
		t.Fatalf("wanted exact edges to start with, got %q", got)
	}
	if got := frame(true); !strings.Contains(got, "solid edges") {
		t.Errorf("wanted solid edges after a press, got %q", got)
	}
	if g.flipEdges {
		t.Errorf("wanted the press used up by the frame that applied it")
	}
	if got := frame(false); !strings.Contains(got, "solid edges") {
		t.Errorf("wanted solid edges kept on the next frame, got %q", got)
	}
	if got := frame(true); !strings.Contains(got, "exact edges") {
		t.Errorf("wanted a second press to switch back, got %q", got)
	}
}

func TestSceneDrawsAndStartsWithGlyphs(t *testing.T) {
	g := newCubeGame()
	fb := newFrame(t, 80, 24)
	g.Draw(fb)

	if got := row(fb, 1); !strings.Contains(got, "glyphs") {
		t.Errorf("wanted glyph shading to start with, got %q", got)
	}
	drawn := 0
	for y := 3; y < fb.Height()-4; y++ {
		for x := 1; x < fb.Width()-1; x++ {
			if fb.At(x, y) != engine.BlankCell {
				drawn++
			}
		}
	}
	if drawn == 0 {
		t.Errorf("wanted the floor and cubes on screen, got an empty scene")
	}
}

func TestKeyDownTakesEitherCase(t *testing.T) {
	tests := []struct {
		name   string
		down   []engine.Key
		letter rune
		want   bool
	}{
		{"lower case", []engine.Key{'w'}, 'w', true},
		{"upper case", []engine.Key{'W'}, 'w', true},
		{"both cases", []engine.Key{'w', 'W'}, 'w', true},
		{"nothing down", nil, 'w', false},
		{"another letter", []engine.Key{'s'}, 'w', false},
		{"another letter in upper case", []engine.Key{'S'}, 'w', false},
		{"an arrow is not a letter", []engine.Key{engine.KeyUp}, 'w', false},
		{"q", []engine.Key{'Q'}, 'q', true},
		{"e", []engine.Key{'E'}, 'e', true},
		{"a digit has no other case", []engine.Key{'1'}, '1', true},
		{"a digit that is not down", []engine.Key{'2'}, '1', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := engine.NewKeyboard()
			kb.Press(tt.down...)
			if got := keyDown(kb.Input().IsDown, tt.letter); got != tt.want {
				t.Errorf("wanted keyDown(%v, %q) = %v, got %v", tt.down, tt.letter, tt.want, got)
			}
		})
	}
}

func TestHelpListsEveryKeyAndFitsANarrowWindow(t *testing.T) {
	for _, width := range []int{80, 40, 30} {
		fb := newFrame(t, width, 24)
		newCubeGame().Draw(fb)

		first, second := row(fb, fb.Height()-3), row(fb, fb.Height()-2)
		if width >= 40 {
			for _, want := range []string{"WASD move", "Q/E turn", "arrows look"} {
				if !strings.Contains(first, want) {
					t.Errorf("width %d: wanted %q in %q", width, want, first)
				}
			}
		}
		for _, want := range []string{"M mode", "B edges", "Esc quit"} {
			if !strings.Contains(second, want) {
				t.Errorf("width %d: wanted %q in %q", width, want, second)
			}
		}
	}
}

// With no keys held nothing moves, however long the frame.
func TestUpdateWithoutKeysStandsStill(t *testing.T) {
	g := newCubeGame()
	before := g.cam

	if err := g.Update(0.1, engine.NewKeyboard().Input()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.cam != before {
		t.Errorf("wanted the camera unmoved, got %+v from %+v", g.cam, before)
	}
}

// frame holds keys for one Update of dt seconds and ends the frame, as Run does.
func frame(t *testing.T, g *cubeGame, kb *engine.Keyboard, dt float64) error {
	t.Helper()
	err := g.Update(dt, kb.Input())
	kb.EndFrame()
	return err
}

func TestWalkingMovesAlongWhereTheCameraFaces(t *testing.T) {
	const dt = 0.1
	const distance = 3.0 * dt // moveSpeed * dt

	for _, yaw := range []float64{0, 0.7, -2} {
		for _, tt := range []struct {
			name string
			keys []engine.Key
			dir  func(mesh.Camera) geometry.Vector3
		}{
			{"W forward", []engine.Key{'w'}, func(c mesh.Camera) geometry.Vector3 { return c.Forward() }},
			{"S back", []engine.Key{'s'}, func(c mesh.Camera) geometry.Vector3 { return c.Forward().Neg() }},
			{"D right", []engine.Key{'d'}, func(c mesh.Camera) geometry.Vector3 { return c.Right() }},
			{"A left", []engine.Key{'a'}, func(c mesh.Camera) geometry.Vector3 { return c.Right().Neg() }},
			{"capital W", []engine.Key{'W'}, func(c mesh.Camera) geometry.Vector3 { return c.Forward() }},
			{"W and D together go diagonally, no faster", []engine.Key{'w', 'd'}, func(c mesh.Camera) geometry.Vector3 { return c.Forward().Add(c.Right()).Normalize() }},
			{"W and S cancel", []engine.Key{'w', 's'}, func(mesh.Camera) geometry.Vector3 { return geometry.Vector3{} }},
		} {
			t.Run(fmt.Sprintf("%s at yaw %v", tt.name, yaw), func(t *testing.T) {
				g := newCubeGame()
				g.cam.Yaw = yaw
				start := g.cam.Position
				kb := engine.NewKeyboard()
				kb.Press(tt.keys...)

				if err := frame(t, g, kb, dt); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				want := start.Add(tt.dir(g.cam).Scale(distance))
				if got := g.cam.Position; got.Sub(want).Length() > 1e-9 {
					t.Errorf("wanted the camera at %v, got %v", want, got)
				}
			})
		}
	}
}

func TestWalkingStopsWhenTheKeyIsReleased(t *testing.T) {
	g := newCubeGame()
	kb := engine.NewKeyboard()
	kb.Press('w')
	frame(t, g, kb, 0.1)
	moved := g.cam.Position

	frame(t, g, kb, 0.1) // still held: keeps going
	if g.cam.Position == moved {
		t.Fatalf("wanted the camera to keep moving while W is held")
	}
	kb.Release('w')
	stopped := g.cam.Position
	frame(t, g, kb, 0.1)

	if g.cam.Position != stopped {
		t.Errorf("wanted the camera to stay put after W is released, got %v from %v", g.cam.Position, stopped)
	}
}

func TestWalkingStopsAtTheInvisibleWalls(t *testing.T) {
	for _, tt := range []struct {
		name  string
		key   engine.Key
		yaw   float64
		check func(geometry.Vector3) bool
	}{
		{"north", 'w', 0, func(p geometry.Vector3) bool { return p.Z == walkLimit }},
		{"south", 's', 0, func(p geometry.Vector3) bool { return p.Z == -walkLimit }},
		{"east", 'd', 0, func(p geometry.Vector3) bool { return p.X == walkLimit }},
		{"west", 'a', 0, func(p geometry.Vector3) bool { return p.X == -walkLimit }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := newCubeGame()
			kb := engine.NewKeyboard()
			kb.Press(tt.key)
			for range 200 {
				frame(t, g, kb, 0.25)
			}
			if !tt.check(g.cam.Position) {
				t.Errorf("wanted the camera stopped at the wall, got %v", g.cam.Position)
			}
		})
	}
}

func TestPitchDoesNotLiftTheWalker(t *testing.T) {
	g := newCubeGame()
	g.cam.Pitch = 1
	y := g.cam.Position.Y
	kb := engine.NewKeyboard()
	kb.Press('w')

	frame(t, g, kb, 0.1)

	if g.cam.Position.Y != y {
		t.Errorf("wanted walking to stay level however the camera looks, got y %v from %v", g.cam.Position.Y, y)
	}
}

func TestTurningAndLooking(t *testing.T) {
	const dt = 0.1
	tests := []struct {
		name       string
		keys       []engine.Key
		dYaw, dPit float64
	}{
		{"E turns right", []engine.Key{'e'}, 1.5 * dt, 0},
		{"capital E turns right", []engine.Key{'E'}, 1.5 * dt, 0},
		{"right arrow turns right", []engine.Key{engine.KeyRight}, 1.5 * dt, 0},
		{"Q turns left", []engine.Key{'q'}, -1.5 * dt, 0},
		{"left arrow turns left", []engine.Key{engine.KeyLeft}, -1.5 * dt, 0},
		{"Q and E cancel", []engine.Key{'q', 'e'}, 0, 0},
		{"up arrow looks up", []engine.Key{engine.KeyUp}, 0, 1.0 * dt},
		{"down arrow looks down", []engine.Key{engine.KeyDown}, 0, -1.0 * dt},
		{"up and down cancel", []engine.Key{engine.KeyUp, engine.KeyDown}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newCubeGame()
			kb := engine.NewKeyboard()
			kb.Press(tt.keys...)

			frame(t, g, kb, dt)

			if math.Abs(g.cam.Yaw-tt.dYaw) > 1e-9 || math.Abs(g.cam.Pitch-tt.dPit) > 1e-9 {
				t.Errorf("wanted yaw %v and pitch %v, got %v and %v", tt.dYaw, tt.dPit, g.cam.Yaw, g.cam.Pitch)
			}
		})
	}
}

func TestLookingStopsJustShortOfStraightUpOrDown(t *testing.T) {
	for _, tt := range []struct {
		key  engine.Key
		want float64
	}{{engine.KeyUp, 1.5}, {engine.KeyDown, -1.5}} {
		g := newCubeGame()
		kb := engine.NewKeyboard()
		kb.Press(tt.key)
		for range 100 {
			frame(t, g, kb, 0.1)
		}
		if g.cam.Pitch != tt.want {
			t.Errorf("key %v: wanted the pitch held at %v, got %v", tt.key, tt.want, g.cam.Pitch)
		}
	}
}

func TestEscapeQuits(t *testing.T) {
	g := newCubeGame()
	kb := engine.NewKeyboard()
	start := g.cam

	if err := frame(t, g, kb, 0.1); err != nil {
		t.Fatalf("wanted no error without Escape, got %v", err)
	}
	kb.Press(engine.KeyEscape)
	if err := frame(t, g, kb, 0.1); !errors.Is(err, engine.ErrQuit) {
		t.Errorf("wanted ErrQuit, got %v", err)
	}
	if g.cam != start {
		t.Errorf("wanted nothing to move on the quitting frame, got %+v", g.cam)
	}
}

// M is a press, not a hold: holding it down must not flicker the mode.
func TestModeKeySwitchesOncePerPress(t *testing.T) {
	for _, key := range []engine.Key{'m', 'M'} {
		g := newCubeGame()
		kb := engine.NewKeyboard()

		kb.Press(key)
		frame(t, g, kb, 0.1)
		if !g.halfBlocks {
			t.Fatalf("key %q: wanted half blocks after the first press", key)
		}
		for range 5 { // held down, no new press
			frame(t, g, kb, 0.1)
		}
		if !g.halfBlocks {
			t.Errorf("key %q: wanted holding the key to change nothing", key)
		}
		kb.Release(key)
		kb.Press(key)
		frame(t, g, kb, 0.1)
		if g.halfBlocks {
			t.Errorf("key %q: wanted a second press to switch back to glyphs", key)
		}
	}
}

func TestEdgeKeyDoesNothingInGlyphMode(t *testing.T) {
	g := newCubeGame()
	kb := engine.NewKeyboard()
	kb.Press('b')

	frame(t, g, kb, 0.1)

	if g.flipEdges {
		t.Errorf("wanted B ignored while drawing glyphs")
	}
}

// ghostColor is the color nothing else in the scene is drawn in, so a cell in it is the ghost.
var ghostColor = engine.Palette(51)

// cellsOf is the set of cells inside the frame border and between the lines of text where
// keep says the cell counts.
func cellsOf(fb *engine.FrameBuffer, keep func(engine.Cell) bool) map[[2]int]bool {
	out := map[[2]int]bool{}
	for y := 3; y < fb.Height()-4; y++ {
		for x := 1; x < fb.Width()-1; x++ {
			if c := fb.At(x, y); c != engine.BlankCell && keep(c) {
				out[[2]int{x, y}] = true
			}
		}
	}
	return out
}

func anyCell(engine.Cell) bool { return true }

func isGhost(c engine.Cell) bool { return c.Foreground == ghostColor }

// sceneAt is a game with the camera at a place, in glyph or half-block mode.
func sceneAt(t *testing.T, x, z, yaw float64, halfBlocks bool) *cubeGame {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "") // the pixel edge style depends on the terminal
	g := newCubeGame()
	g.cam.Position = geometry.Vector3{X: x, Z: z}
	g.cam.Yaw = yaw
	g.halfBlocks = halfBlocks
	return g
}

func TestTheGhostStandsOnTheFloorAndIsDrawnInItsOwnColor(t *testing.T) {
	g := newCubeGame()
	lo, _ := g.ghost.Bounds()

	if got := g.ghost.Transform.Position.Y + lo.Y; math.Abs(got-floorY) > 1e-9 {
		t.Errorf("wanted the ghost standing on the floor at %v, got its base at %v", floorY, got)
	}
	if g.ghost.Material.Color != ghostColor || len(g.ghost.Edges) != 12 {
		t.Errorf("wanted a cube in the ghost color, got %v with %d edges", g.ghost.Material.Color, len(g.ghost.Edges))
	}
	gl, gh := g.ghost.Bounds()
	gp := g.ghost.Transform.Position
	for _, c := range g.cubes {
		if c.Material.Color == ghostColor {
			t.Errorf("wanted no solid cube in the ghost's color")
		}
		// Standing clear of every solid, so that walking around it is not walking through one.
		cl, ch := c.Bounds()
		cp := c.Transform.Position
		if gp.X+gl.X < cp.X+ch.X && gp.X+gh.X > cp.X+cl.X && gp.Z+gl.Z < cp.Z+ch.Z && gp.Z+gh.Z > cp.Z+cl.Z {
			t.Errorf("wanted the ghost clear of the cube at %v, but their footprints overlap", cp)
		}
	}
}

// The ghost adds its own cells and changes nothing else: take it out and the scene is what
// it was. In half-block mode too, where the cells it lands on are pixels.
func TestTheGhostChangesOnlyItsOwnCells(t *testing.T) {
	for _, halfBlocks := range []bool{false, true} {
		for _, pose := range []struct{ x, z, yaw float64 }{{0, -4, 0}, {3, 2, 0.5}, {-2, 5, -0.4}, {0, 14, math.Pi}, {5, -3, -0.7}} {
			g := sceneAt(t, pose.x, pose.z, pose.yaw, halfBlocks)
			with, without, alone := newFrame(t, 80, 24), newFrame(t, 80, 24), newFrame(t, 80, 24)
			g.Draw(with)
			g.ghost.Draw(alone, g.cam, engine.Cell{Character: '#', Foreground: ghostColor})
			ghostCells := cellsOf(alone, anyCell)
			g.ghost = mesh.Mesh{}
			g.Draw(without)

			for y := range 24 {
				for x := range 80 {
					if with.At(x, y) != without.At(x, y) && !ghostCells[[2]int{x, y}] {
						t.Fatalf("blocks %v, pose %v: cell (%d,%d) differs but is not one of the ghost's", halfBlocks, pose, x, y)
					}
				}
			}
		}
	}
}

// From the start the orange cube is in front of the ghost's left side: the ghost is
// drawn after the cubes, and the cube hides the part of its edges that it covers, while
// the rest shows. The same in half-block mode, where the cube is pixels.
func TestTheGhostIsHiddenBehindANearerCube(t *testing.T) {
	for _, halfBlocks := range []bool{false, true} {
		name := "glyphs"
		if halfBlocks {
			name = "half blocks"
		}
		t.Run(name, func(t *testing.T) {
			g := sceneAt(t, 0, -4, 0, halfBlocks)
			full, solids, ghostAlone, floorAlone := newFrame(t, 80, 24), newFrame(t, 80, 24), newFrame(t, 80, 24), newFrame(t, 80, 24)
			g.Draw(full)

			noGhost := *g
			noGhost.ghost, noGhost.floor = mesh.Mesh{}, mesh.Mesh{}
			noGhost.Draw(solids)
			g.ghost.Draw(ghostAlone, g.cam, engine.Cell{Character: '#', Foreground: ghostColor})
			g.floor.Draw(floorAlone, g.cam, engine.Cell{Character: '.'})
			covered, edges, floor := cellsOf(solids, anyCell), cellsOf(ghostAlone, anyCell), cellsOf(floorAlone, anyCell)

			hidden, shown := 0, 0
			for c := range edges {
				if floor[c] {
					continue // a floor line may sit in the same cell
				}
				drawn := isGhost(full.At(c[0], c[1]))
				switch {
				case covered[c] && drawn:
					t.Fatalf("cell %v: wanted the ghost hidden behind the cube, but it is drawn", c)
				case !covered[c] && !drawn:
					t.Fatalf("cell %v: wanted the ghost where nothing covers it, but it is missing", c)
				case covered[c]:
					hidden++
				default:
					shown++
				}
			}
			if hidden < 3 || shown < 10 {
				t.Errorf("wanted the ghost partly hidden and partly shown from the start, got %d hidden and %d shown", hidden, shown)
			}
		})
	}
}

// A solid farther away than the ghost does not hide it, whichever way the solid is drawn:
// as characters or, composed after the pixels are in, as half blocks.
func TestTheGhostIsWholeWhereItIsNearerThanASolid(t *testing.T) {
	for _, halfBlocks := range []bool{false, true} {
		name := "glyphs"
		if halfBlocks {
			name = "half blocks"
		}
		t.Run(name, func(t *testing.T) {
			g := sceneAt(t, 0, -4, 0, halfBlocks)
			g.cubes = []mesh.Mesh{newCube(5, 1.6, 14, engine.Palette(208))} // a big cube behind the ghost
			full, solid, alone, floorAlone := newFrame(t, 80, 24), newFrame(t, 80, 24), newFrame(t, 80, 24), newFrame(t, 80, 24)
			g.Draw(full)
			noGhost := *g
			noGhost.ghost, noGhost.floor = mesh.Mesh{}, mesh.Mesh{}
			noGhost.Draw(solid)
			g.ghost.Draw(alone, g.cam, engine.Cell{Character: '#', Foreground: ghostColor})
			g.floor.Draw(floorAlone, g.cam, engine.Cell{Character: '.'})
			covered, edges, floor := cellsOf(solid, anyCell), cellsOf(alone, anyCell), cellsOf(floorAlone, anyCell)

			over := 0
			for c := range edges {
				if floor[c] {
					continue
				}
				if covered[c] {
					over++
				}
				if !isGhost(full.At(c[0], c[1])) {
					t.Fatalf("cell %v: wanted the ghost drawn in front of the cube behind it", c)
				}
			}
			if over < 5 {
				t.Errorf("wanted the ghost drawn over the cube in at least 5 cells, got %d", over)
			}
		})
	}
}

// The floor is drawn first and ignores depth, so the cubes cover its lines where they
// stand: no cell a solid cube covers shows the floor.
func TestCubesCoverTheFloorLines(t *testing.T) {
	floorColor := engine.Palette(240)
	for _, halfBlocks := range []bool{false, true} {
		for _, pose := range []struct{ x, z, yaw float64 }{{0, -4, 0}, {3, 2, 0.5}, {-2, 5, -0.4}, {0, 14, math.Pi}} {
			g := sceneAt(t, pose.x, pose.z, pose.yaw, halfBlocks)
			full, solids := newFrame(t, 80, 24), newFrame(t, 80, 24)
			g.Draw(full)
			noFloor := *g
			noFloor.floor, noFloor.ghost = mesh.Mesh{}, mesh.Mesh{}
			noFloor.Draw(solids)

			covered := cellsOf(solids, anyCell)
			for c := range covered {
				if full.At(c[0], c[1]).Foreground == floorColor {
					t.Fatalf("blocks %v, pose %v: wanted the cube to cover the floor at %v", halfBlocks, pose, c)
				}
			}
			if len(covered) < 10 {
				t.Fatalf("blocks %v, pose %v: wanted cubes in view, got %d cells", halfBlocks, pose, len(covered))
			}
		}
	}
}

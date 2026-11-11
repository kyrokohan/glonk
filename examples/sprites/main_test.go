package main

import (
	"errors"
	"math"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

func newFrame(t *testing.T, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

// newSizedGame returns a game that has been told its frame is w x h, as Run does
// before the first Update.
func newSizedGame(w, h int) *spriteGame {
	g := newSpriteGame()
	g.Resize(w, h)
	return g
}

// hold returns an Input with keys down.
func hold(keys ...engine.Key) *engine.Input {
	kb := engine.NewKeyboard()
	kb.Press(keys...)
	return kb.Input()
}

func TestSceneDrawsAndUpdatesAtAnySizeWithoutPanic(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {2, 2}, {3, 3}, {5, 5}, {20, 8}, {80, 24}, {200, 60}} {
		g := newSizedGame(size[0], size[1])
		fb := newFrame(t, size[0], size[1])
		for range 200 {
			if err := g.Update(0.1, hold(engine.KeyRight, engine.KeyDown)); err != nil {
				t.Fatalf("%v: unexpected error: %v", size, err)
			}
			g.Draw(fb)
		}
	}
}

func TestArrowsSteerTheShipAtTheSameSpeedInEveryDirection(t *testing.T) {
	tests := []struct {
		name           string
		keys           []engine.Key
		wantDX, wantDY float64
	}{
		{"left", []engine.Key{engine.KeyLeft}, -1.2, 0},
		{"right", []engine.Key{engine.KeyRight}, 1.2, 0},
		{"up", []engine.Key{engine.KeyUp}, 0, -1.2},
		{"down", []engine.Key{engine.KeyDown}, 0, 1.2},
		{"diagonal is no faster", []engine.Key{engine.KeyRight, engine.KeyDown}, 1.2 / math.Sqrt2, 1.2 / math.Sqrt2},
		{"opposite keys cancel", []engine.Key{engine.KeyLeft, engine.KeyRight}, 0, 0},
		{"other keys do nothing", []engine.Key{'w', engine.KeyEnter}, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newSizedGame(80, 40)
			g.ship.Position.X, g.ship.Position.Y = 10, 12 // well away from the rock
			if err := g.Update(0.05, hold(tt.keys...)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			dx, dy := g.ship.Position.X-10, g.ship.Position.Y-12
			if math.Abs(dx-tt.wantDX) > 1e-9 || math.Abs(dy-tt.wantDY) > 1e-9 {
				t.Errorf("wanted the ship to move by (%v, %v), got (%v, %v)", tt.wantDX, tt.wantDY, dx, dy)
			}
		})
	}
}

func TestShipStaysOnTheSeaInsideTheBorder(t *testing.T) {
	g := newSizedGame(40, 20)
	w, h := g.ship.Frame.Width(), g.ship.Frame.Height()

	// Long enough to cross the whole frame twice over in each direction.
	for _, key := range []engine.Key{engine.KeyLeft, engine.KeyUp, engine.KeyRight, engine.KeyDown} {
		for range 40 {
			if err := g.Update(0.1, hold(key)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		x, y := g.ship.Position.X, g.ship.Position.Y
		if x < 1 || x+float64(w) > 39 || y < 1 || y+float64(h) > float64(20-sandRows-1) {
			t.Errorf("after %v: wanted the ship inside columns 1 to 38 and above the sand, got (%v, %v)", key, x, y)
		}
	}
}

func TestShipInAFrameTooSmallForItStaysPutAndDoesNotPanic(t *testing.T) {
	g := newSizedGame(4, 4)
	if err := g.Update(0.1, hold(engine.KeyRight, engine.KeyDown)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.ship.Position.X != 1 || g.ship.Position.Y != 1 {
		t.Errorf("wanted the ship held at (1, 1), got (%v, %v)", g.ship.Position.X, g.ship.Position.Y)
	}
}

func TestWalkerRunsThroughItsFourFramesAndRepeats(t *testing.T) {
	g := newSizedGame(80, 24)
	g.walker.Position.X = 20
	kb := engine.NewKeyboard()

	// 0.13 s at 8 frames a second steps a little over one frame each time.
	for i, want := range []byte{'/', ' ', '\\', ' ', '/'} {
		if i > 0 {
			if err := g.Update(0.13, kb.Input()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			g.walker.Position.X = 20 // keep it away from the edges so the legs line up below
		}
		fb := newFrame(t, 80, 24)
		g.Draw(fb)
		y := int(g.walker.Position.Y) + 2
		// The first leg is '/', then a hole, then '\', then a hole; on the sand a hole shows
		// whatever the floor has there, so a hole is only checked not to be a leg.
		got := fb.At(20, y).Character
		if want == ' ' && (got == '/' || got == '\\') || want != ' ' && got != rune(want) {
			t.Errorf("step %d: wanted %q for the first leg, got %q", i, want, got)
		}
	}
}

func TestWalkerMovesRightAndComesBackInFromTheLeft(t *testing.T) {
	g := newSizedGame(40, 24)
	x := g.walker.Position.X
	if err := g.Update(0.1, &engine.Input{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := g.walker.Position.X - x; math.Abs(got-0.9) > 1e-9 {
		t.Errorf("wanted the walker to move 0.9, got %v", got)
	}

	g.walker.Position.X = 38.95
	if err := g.Update(0.01, &engine.Input{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.walker.Position.X != -3 {
		t.Errorf("wanted the walker back at -3, just off the left edge, got %v", g.walker.Position.X)
	}
}

func TestWalkerIsHiddenBehindTheBorderAtEitherEdge(t *testing.T) {
	g := newSizedGame(40, 24)
	for _, x := range []float64{-2, -1, 0, 37, 38, 38.9} {
		g.walker.Position.X = x
		fb := newFrame(t, 40, 24)
		g.Draw(fb)
		for y := range 24 {
			if c := fb.At(0, y).Character; y > 0 && y < 23 && c != '│' {
				t.Fatalf("x=%v: wanted the left border whole at row %d, got %q", x, y, c)
			}
			if c := fb.At(39, y).Character; y > 0 && y < 23 && c != '│' {
				t.Fatalf("x=%v: wanted the right border whole at row %d, got %q", x, y, c)
			}
		}
	}
}

func TestGhostShowsTheSeaThroughItsHolesAndKeepsItBehindItsLetters(t *testing.T) {
	g := newSizedGame(60, 20)
	g.ghost.Position.X, g.ghost.Position.Y = 30, 4

	with := newFrame(t, 60, 20)
	g.Draw(with)
	g.layer.Remove(g.ghost)
	without := newFrame(t, 60, 20)
	g.Draw(without)

	art := g.blink.Frame()
	holes, letters := 0, 0
	for y := range art.Height() {
		for x := range art.Width() {
			got, floor, piece := with.At(30+x, 4+y), without.At(30+x, 4+y), art.At(x, y)
			if piece.Character == ' ' {
				holes++
				if got != floor {
					t.Errorf("(%d, %d): wanted the sea through the hole, got %+v over %+v", x, y, got, floor)
				}
				continue
			}
			letters++
			if got.Character != piece.Character || got.Foreground != piece.Foreground || got.Background != floor.Background {
				t.Errorf("(%d, %d): wanted %q in the ghost's color on the sea's background, got %+v over %+v", x, y, piece.Character, got, floor)
			}
		}
	}
	if holes == 0 || letters == 0 {
		t.Fatalf("wanted a ghost with holes and letters, got %d and %d", holes, letters)
	}
}

func TestGhostBlinksAndFloatsAcrossTheSea(t *testing.T) {
	g := newSizedGame(80, 24)
	kb := engine.NewKeyboard()
	if err := g.Update(0.01, kb.Input()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	open, x0, y0 := g.blink.Index(), g.ghost.Position.X, g.ghost.Position.Y

	// 0.7 s is a little more than one frame at 1.5 a second.
	if err := g.Update(0.7, kb.Input()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.blink.Index() == open {
		t.Errorf("wanted the ghost's frame to change, still %d", open)
	}
	if g.ghost.Position.X == x0 && g.ghost.Position.Y == y0 {
		t.Errorf("wanted the ghost to have floated from (%v, %v)", x0, y0)
	}

	// The two frames take 1.33 s, so it opens its eyes again instead of staying shut.
	if err := g.Update(0.7, kb.Input()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.blink.Index() != open {
		t.Errorf("wanted the blink to loop back to frame %d, got %d", open, g.blink.Index())
	}
}

func TestShipIsSeeThroughAtTheCornersOfItsArt(t *testing.T) {
	g := newSizedGame(60, 20)
	g.ship.Position.X, g.ship.Position.Y = 10, 5

	fb := newFrame(t, 60, 20)
	g.Draw(fb)
	// The top left of the art is a space: the sea shows there, with its own background.
	if got := fb.At(10, 5); got.Background != engine.Palette(17) || got.Character == '|' {
		t.Errorf("wanted the sea through the ship's top left, got %+v", got)
	}
}

func TestDrawShowsTheBorderTheShipAndTheHint(t *testing.T) {
	g := newSizedGame(60, 20)
	fb := newFrame(t, 60, 20)
	g.Draw(fb)

	if fb.At(0, 0).Character != '┌' || fb.At(59, 0).Character != '┐' || fb.At(0, 19).Character != '└' || fb.At(59, 19).Character != '┘' {
		t.Errorf("wanted a box around the whole frame")
	}
	if got := strings.Split(fb.String(), "\n")[19]; !strings.Contains(got, hint) {
		t.Errorf("wanted the key hint on the bottom border, got %q", got)
	}
	sx, sy := int(g.ship.Position.X), int(g.ship.Position.Y)
	if got := fb.At(sx+3, sy).Character; got != '|' {
		t.Errorf("wanted the ship's mast at (%d, %d), got %q", sx+3, sy, got)
	}
	if got := fb.At(sx, sy+2).Foreground; got != engine.Palette(208) {
		t.Errorf("wanted the ship drawn in its color, got %v", got)
	}
}

func TestHintNeedsRoomForTheCorners(t *testing.T) {
	for _, tt := range []struct {
		width int
		want  bool
	}{{len(hint) + 3, false}, {len(hint) + 4, true}} {
		g := newSizedGame(tt.width, 12)
		fb := newFrame(t, tt.width, 12)
		g.Draw(fb)
		if got := strings.Contains(strings.Split(fb.String(), "\n")[11], hint); got != tt.want {
			t.Errorf("width %d: hint shown is %v, wanted %v", tt.width, got, tt.want)
		}
	}
}

func TestEscapeQuitsAndOtherKeysDoNot(t *testing.T) {
	g := newSizedGame(64, 24)
	kb := engine.NewKeyboard()

	kb.Press('q', engine.KeyEnter)
	if err := g.Update(0.1, kb.Input()); err != nil {
		t.Fatalf("wanted other keys to be ignored, got %v", err)
	}
	kb.EndFrame()

	kb.Press(engine.KeyEscape)
	if err := g.Update(0.1, kb.Input()); !errors.Is(err, engine.ErrQuit) {
		t.Errorf("wanted ErrQuit on Escape, got %v", err)
	}
}

func TestFirstFrameIsLaidOutBeforeAnyUpdate(t *testing.T) {
	g := newSizedGame(80, 24)
	if y := g.walker.Position.Y; y != float64(24-sandRows-1) {
		t.Errorf("wanted the walker on the sand at row %d, got %v", 24-sandRows-1, y)
	}
	if y := g.ghost.Position.Y; y < 1 || y >= float64(24-sandRows-1) {
		t.Errorf("wanted the ghost over the sea, got row %v", y)
	}
	if x := g.ghost.Position.X; x < 1 || x >= 79 {
		t.Errorf("wanted the ghost inside the frame, got column %v", x)
	}
}

// The rock sits in the middle of the sea at 80 by 24: columns 36 to 43, rows 9 to 11.
func newRockGame(t *testing.T) *spriteGame {
	t.Helper()
	g := newSizedGame(80, 24)
	if r := g.rock.Bounds(); r != geometry.NewRect(36, 9, 8, 3) {
		t.Fatalf("wanted the rock at columns 36 to 43 and rows 9 to 11, got %v", r)
	}
	return g
}

// hairOf is how far two boxes overlap in the narrower direction, which rounding in a push
// leaves at nothing worth drawing.
func hairOf(a, b geometry.Rect) float64 {
	i := a.Intersect(b)
	return min(i.Width(), i.Height())
}

func TestShipIsPushedOutOfTheRockAndStopsAgainstIt(t *testing.T) {
	g := newRockGame(t)
	g.ship.Position = geometry.Vector2{X: 20, Y: 8.5} // level with the rock, to its left
	rock := g.rock.Bounds()

	for i := range 60 {
		if err := g.Update(0.05, hold(engine.KeyRight)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hair := hairOf(g.ship.Bounds(), rock); hair > 1e-9 {
			t.Fatalf("frame %d: wanted the ship out of the rock, overlapping by %v at %v", i, hair, g.ship.Position)
		}
		if g.ship.Position.Y != 8.5 {
			t.Fatalf("frame %d: wanted the ship to go straight at the rock's face, got Y %v", i, g.ship.Position.Y)
		}
	}
	// Stopped with its right edge against the rock's left edge, and drawn in the column
	// next to it, whichever way the sum was rounded.
	if got := g.ship.Position.X; math.Abs(got-28) > 1e-9 {
		t.Errorf("wanted the ship stopped at column 28, got %v", got)
	}
	fb := newFrame(t, 80, 24)
	g.Draw(fb)
	if got := fb.At(35, 10).Character; got != '\\' && got != '_' && got != '/' && got != '|' {
		t.Errorf("wanted the ship's side drawn in the column just before the rock, got %q", got)
	}
	if got := fb.At(36, 10).Character; got != '#' {
		t.Errorf("wanted the rock's first column right after the ship, got %q", got)
	}
}

// A push leaves the ship against the rock to within rounding, and the ship is drawn in the
// column its position is floored to, so a position a hair short of 28 would rest a column
// short, for some frame rates and not others.
func TestShipRestsExactlyAgainstTheRockAtAnyFrameRate(t *testing.T) {
	for _, dt := range []float64{0.05, 1.0 / 30, 0.033, 0.0167, 0.02, 0.01} {
		for _, y := range []float64{8, 8.5, 9.3, 10.123} {
			g := newRockGame(t)
			g.ship.Position = geometry.Vector2{X: 20.37, Y: y}
			for range 150 {
				if err := g.Update(dt, hold(engine.KeyRight)); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			if g.ship.Position.X != 28 {
				t.Errorf("dt %v, Y %v: wanted the ship to rest at exactly column 28, got %v", dt, y, g.ship.Position.X)
			}
		}
	}
}

// A push does not know where the ship came from, so a frame long enough to carry it deep into
// the rock could push it out the far side. The step is capped, and it stays in front.
func TestALongFrameCannotCarryTheShipThroughTheRock(t *testing.T) {
	for _, dt := range []float64{0.1, 0.5, 2, 60, math.Inf(1)} {
		g := newRockGame(t)
		g.ship.Position = geometry.Vector2{X: 27, Y: 9.3}
		if err := g.Update(dt, hold(engine.KeyRight)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := g.ship.Position; got.X > 28 {
			t.Errorf("dt %v: wanted the ship held in front of the rock, got %v", dt, got)
		}
	}
}

func TestShipSlidesAlongTheRockWhenSteeredIntoItAtAnAngle(t *testing.T) {
	g := newRockGame(t)
	g.ship.Position = geometry.Vector2{X: 20, Y: 5}
	rock := g.rock.Bounds()
	var slid float64

	for i := range 40 {
		before := g.ship.Position
		if err := g.Update(0.05, hold(engine.KeyRight, engine.KeyDown)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hair := hairOf(g.ship.Bounds(), rock); hair > 1e-9 {
			t.Fatalf("frame %d: wanted the ship out of the rock, overlapping by %v", i, hair)
		}
		if g.ship.Position.X >= 27.5 && g.ship.Position.Y > before.Y {
			slid += g.ship.Position.Y - before.Y // moved down while against the face
		}
	}
	if slid < 2 {
		t.Errorf("wanted the ship to slide down the rock's face, slid %v", slid)
	}
}

func TestGhostFloatsThroughTheRock(t *testing.T) {
	g := newRockGame(t)
	rock := g.rock.Bounds()

	var through int
	for step := range 400 {
		g.t = float64(step) * 0.1
		g.place()
		if g.ghost.Bounds().Overlaps(rock) {
			through++
		}
	}
	if through == 0 {
		t.Errorf("wanted the ghost's path to cross the rock")
	}
	// It is drawn over the rock, not behind it: the rock was added first.
	g.ghost.Position = geometry.Vector2{X: 37, Y: 9}
	fb := newFrame(t, 80, 24)
	g.Draw(fb)
	if got := fb.At(37, 10).Character; got != '(' {
		t.Errorf("wanted the ghost's face drawn over the rock, got %q", got)
	}
	// Nothing about the ghost moves the rock or is pushed by it, whatever the frame count.
	for range 100 {
		if err := g.Update(0.1, hold()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if g.rock.Bounds() != rock {
		t.Errorf("wanted the rock to stay where it was, got %v", g.rock.Bounds())
	}
}

// What you see is what the ship runs into: every cell of the rock's box is drawn as the rock.
func TestRockIsDrawnAsAWholeBlockOfItsBox(t *testing.T) {
	g := newRockGame(t)
	g.ghost.Position = geometry.Vector2{X: 60, Y: 2} // out of the way
	fb := newFrame(t, 80, 24)
	g.Draw(fb)
	for y := 9; y < 12; y++ {
		for x := 36; x < 44; x++ {
			if got := fb.At(x, y).Character; got != '#' {
				t.Fatalf("(%d, %d): wanted the rock, got %q", x, y, got)
			}
		}
	}
	if fb.At(35, 10).Character == '#' || fb.At(44, 10).Character == '#' || fb.At(40, 8).Character == '#' || fb.At(40, 12).Character == '#' {
		t.Errorf("wanted the rock to stop at its box")
	}
}

func TestRockStaysInTheSeaAtEverySizeItFits(t *testing.T) {
	for _, size := range [][2]int{{20, 12}, {40, 12}, {80, 24}, {200, 60}} {
		g := newSizedGame(size[0], size[1])
		r := g.rock.Bounds()
		if r.Min.X < 1 || r.Max.X > float64(size[0]-1) || r.Min.Y < 1 || r.Max.Y > float64(size[1]-sandRows-1) {
			t.Errorf("%v: wanted the rock inside the border and above the sand, got %v", size, r)
		}
		if r.Min.X != math.Floor(r.Min.X) || r.Min.Y != math.Floor(r.Min.Y) {
			t.Errorf("%v: wanted the rock on whole cells, got %v", size, r)
		}
	}
}

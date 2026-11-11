package main

import (
	"errors"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/sprite"
)

func newFrame(t testing.TB, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

// newSizedGame returns a game that has been told its frame is w x h, as Run does before the
// first Update.
func newSizedGame(w, h int) *worldGame {
	g := newWorldGame()
	g.Resize(w, h)
	return g
}

// hold returns an Input with keys down.
func hold(keys ...engine.Key) *engine.Input {
	kb := engine.NewKeyboard()
	kb.Press(keys...)
	return kb.Input()
}

// overlapsSolid reports whether the player's box covers a solid cell, asking every cell it
// could cover, which is independent of how Move finds the edge.
func overlapsSolid(g *worldGame, box geometry.Rect) bool {
	for y := int(math.Floor(box.Min.Y)); y < int(math.Ceil(box.Max.Y)); y++ {
		for x := int(math.Floor(box.Min.X)); x < int(math.Ceil(box.Max.X)); x++ {
			if g.level.Solid(x, y) {
				return true
			}
		}
	}
	return false
}

func TestTheLevelIsBiggerThanTheScreenAndWalledIn(t *testing.T) {
	g := newSizedGame(80, 24)
	if g.level.Width() != 120 || g.level.Height() != 36 {
		t.Fatalf("wanted a level of 120 by 36, got %d by %d", g.level.Width(), g.level.Height())
	}
	if g.level.Width() <= 80 || g.level.Height() <= 24 {
		t.Errorf("wanted a level bigger than the screen, so that there is something to scroll")
	}
	for x := range g.level.Width() {
		if !g.level.Solid(x, 0) || !g.level.Solid(x, g.level.Height()-1) {
			t.Fatalf("wanted a wall along the top and bottom at column %d", x)
		}
	}
	for y := range g.level.Height() {
		if !g.level.Solid(0, y) || !g.level.Solid(g.level.Width()-1, y) {
			t.Fatalf("wanted a wall along both sides at row %d", y)
		}
	}
	if g.total != 36 || g.coins.Len() != 36 {
		t.Errorf("wanted 36 coins, got %d (%d in the layer)", g.total, g.coins.Len())
	}
	if overlapsSolid(g, g.player.Bounds()) {
		t.Errorf("wanted the player to start clear of the walls, at %v", g.player.Position)
	}
	if g.player.Position == (geometry.Vector2{}) {
		t.Errorf("wanted the player placed at the @")
	}
}

// A player is three cells wide, so what it can reach is found by walking a box of that size
// one cell at a time through the cells that are not solid.
func TestEveryCoinCanBeReached(t *testing.T) {
	g := newSizedGame(80, 24)
	w, h := g.level.Width(), g.level.Height()
	free := func(x, y int) bool {
		for dx := range 3 {
			if g.level.Solid(x+dx, y) {
				return false
			}
		}
		return true
	}

	start := [2]int{int(g.player.Position.X), int(g.player.Position.Y)}
	seen := map[[2]int]bool{start: true}
	queue := [][2]int{start}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := [2]int{p[0] + d[0], p[1] + d[1]}
			if n[0] >= 0 && n[1] >= 0 && n[0] < w && n[1] < h && !seen[n] && free(n[0], n[1]) {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}

	for coin := range g.coins.All() {
		cx, cy := int(coin.Position.X), int(coin.Position.Y)
		reached := false
		for dx := range 3 {
			reached = reached || seen[[2]int{cx - dx, cy}]
		}
		if !reached {
			t.Errorf("wanted the coin at (%d, %d) to be reachable", cx, cy)
		}
	}
}

func TestCameraFollowsThePlayerAndStopsAtTheEdgesOfTheLevel(t *testing.T) {
	g := newSizedGame(80, 24)
	level := g.level.Bounds()

	// In a corner of the level the view is held at the corner.
	g.player.Position = geometry.Vector2{X: 3, Y: 3}
	g.follow()
	if g.view.Min != (geometry.Vector2{}) {
		t.Errorf("wanted the view at the level's corner, got %v", g.view)
	}
	g.player.Position = geometry.Vector2{X: 117, Y: 34}
	g.follow()
	if g.view.Max != level.Max {
		t.Errorf("wanted the view against the far corner, got %v", g.view)
	}

	// In the middle it is centered on the player.
	g.player.Position = geometry.Vector2{X: 59, Y: 17}
	g.follow()
	if c, want := g.view.Center(), g.player.Bounds().Center(); c != want {
		t.Errorf("wanted the view centered on the player at %v, got %v", want, c)
	}
	if g.view.Size() != (geometry.Vector2{X: 80, Y: 24}) {
		t.Errorf("wanted the view to be the size of the screen, got %v", g.view.Size())
	}

	// A screen bigger than the level shows the whole of it, centered.
	g.Resize(200, 60)
	if g.view.Min != (geometry.Vector2{X: -40, Y: -12}) {
		t.Errorf("wanted the level centered on a bigger screen, got %v", g.view)
	}
	// A new size keeps the view on the player.
	g.Resize(40, 12)
	if c := g.view.Center(); c != g.player.Bounds().Center() {
		t.Errorf("wanted the view on the player after a resize, got %v", c)
	}
}

func TestThePlayerNeverEntersAWallWhateverTheKeysAndTheFrameTimes(t *testing.T) {
	g := newSizedGame(80, 24)
	rng := rand.New(rand.NewPCG(5, 6))
	keys := []engine.Key{engine.KeyLeft, engine.KeyRight, engine.KeyUp, engine.KeyDown}
	var moved int
	for i := range 6000 {
		var down []engine.Key
		for _, k := range keys {
			if rng.IntN(3) == 0 {
				down = append(down, k)
			}
		}
		// Mostly frames of a thirtieth of a second, sometimes much longer.
		dt := 1.0 / 30
		switch rng.IntN(20) {
		case 0:
			dt = rng.Float64() * 3
		case 1:
			dt = 100
		}
		before := g.player.Position
		if err := g.Update(dt, hold(down...)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if overlapsSolid(g, g.player.Bounds()) {
			t.Fatalf("frame %d: wanted the player out of the walls, at %v after %v with %v", i, g.player.Position, before, down)
		}
		if g.player.Position != before {
			moved++
		}
	}
	if moved < 3000 {
		t.Errorf("wanted the player to move in most of 6000 frames, moved in %d", moved)
	}
}

func TestThePlayerRestsExactlyAgainstAWallAndSlidesAlongIt(t *testing.T) {
	g := newSizedGame(80, 24)
	// The first room, a few rows down from its top, so that there is room above and below.
	for range 200 {
		g.Update(1.0/30, hold(engine.KeyLeft))
	}
	if x := g.player.Position.X; x != math.Floor(x) || !g.level.Solid(int(x)-1, int(g.player.Position.Y)) {
		t.Fatalf("wanted the player to rest on a whole column right against a wall, got %v", g.player.Position)
	}
	rest := g.player.Position

	// Pressing into the wall changes nothing, at any frame rate.
	for _, dt := range []float64{1.0 / 60, 1.0 / 30, 0.0167, 0.033, 0.1, 5} {
		g.Update(dt, hold(engine.KeyLeft))
		if g.player.Position != rest {
			t.Fatalf("dt %v: wanted the player to stay at %v pressed into the wall, got %v", dt, rest, g.player.Position)
		}
	}

	// Left and down together slides down the wall, staying against it.
	for range 15 {
		g.Update(1.0/30, hold(engine.KeyLeft, engine.KeyDown))
	}
	if g.player.Position.Y <= rest.Y || g.player.Position.X != rest.X {
		t.Errorf("wanted the player to slide down the wall from %v, got %v", rest, g.player.Position)
	}
}

func TestOneLongFrameMovesThePlayerAsFarAsManyShortOnes(t *testing.T) {
	// In the open, away from every wall, so that the only difference is how time is cut up.
	long, short := newSizedGame(80, 24), newSizedGame(80, 24)
	long.Update(0.3, hold(engine.KeyRight))
	for range 30 {
		short.Update(0.01, hold(engine.KeyRight))
	}
	if d := math.Abs(long.player.Position.X - short.player.Position.X); d > 1e-9 {
		t.Errorf("wanted the same distance in 0.3 s however it is cut up, got %v and %v", long.player.Position, short.player.Position)
	}
}

func TestALongFrameDoesNotCarryThePlayerThroughAWall(t *testing.T) {
	g := newSizedGame(80, 24)
	y := int(g.player.Position.Y)
	var wall int // the first wall to the right of the player on its row
	for x := int(g.player.Position.X) + 1; ; x++ {
		if g.level.Solid(x, y) {
			wall = x
			break
		}
	}
	g.Update(60, hold(engine.KeyRight))
	if got := g.player.Position.X; got != float64(wall-3) {
		t.Errorf("wanted a minute-long frame to stop the player against the wall at column %d, got column %v", wall, got)
	}
}

func TestCoinsAreCollectedWhenTouchedAndOnlyOnce(t *testing.T) {
	g := newSizedGame(80, 24)
	coin := firstOf(g)
	g.player.Position = geometry.Vector2{X: coin.Position.X - 1, Y: coin.Position.Y} // the coin is under the player's middle
	before := g.coins.Len()
	if err := g.Update(0, hold()); err != nil {
		t.Fatal(err)
	}
	if g.score != 1 || g.coins.Len() != before-1 {
		t.Errorf("wanted one coin collected, got a score of %d and %d coins left of %d", g.score, g.coins.Len(), before)
	}
	for range 5 {
		g.Update(0.1, hold())
	}
	if g.score != 1 {
		t.Errorf("wanted the coin to count once, got a score of %d", g.score)
	}
}

func TestCollectingEveryCoinSaysSo(t *testing.T) {
	g := newSizedGame(80, 24)
	var spots []geometry.Vector2
	for c := range g.coins.All() {
		spots = append(spots, c.Position)
	}
	for _, p := range spots {
		g.player.Position = geometry.Vector2{X: p.X - 1, Y: p.Y}
		g.Update(0, hold())
	}
	if g.score != g.total || g.coins.Len() != 0 {
		t.Fatalf("wanted every coin collected, got %d of %d with %d left", g.score, g.total, g.coins.Len())
	}
	fb := newFrame(t, 80, 24)
	g.Draw(fb)
	if got := strings.Split(fb.String(), "\n")[0]; !strings.Contains(got, "all 36 coins found!") {
		t.Errorf("wanted the screen to say so, got %q", got)
	}
}

func TestDrawShowsThePlayerAndCoinsOnTheTilesTheyStandOn(t *testing.T) {
	g := newSizedGame(80, 24)
	// Somewhere in the middle of the level and off a whole cell, so that the view is too.
	g.player.Position = geometry.Vector2{X: 57.6, Y: 20.4}
	for g.coins.Len() > 0 {
		g.coins.Remove(firstOf(g))
	}
	g.follow()
	fb := newFrame(t, 80, 24)
	g.Draw(fb)

	ox, oy := int(math.Floor(g.view.Min.X)), int(math.Floor(g.view.Min.Y))
	if g.view.Min.X == math.Floor(g.view.Min.X) && g.view.Min.Y == math.Floor(g.view.Min.Y) {
		t.Fatalf("wanted a view off a whole cell, got %v", g.view.Min)
	}
	px, py := int(math.Floor(g.player.Position.X))-ox, int(math.Floor(g.player.Position.Y))-oy
	if got := fb.String(); !strings.Contains(strings.Split(got, "\n")[py], "(@)") {
		t.Errorf("wanted the player on row %d, got\n%s", py, got)
	}
	if got := fb.At(px+1, py).Character; got != '@' {
		t.Errorf("wanted the player's middle at column %d, got %q", px+1, got)
	}
	// Every other cell of the screen below the panel is the tile of the level under it.
	for y := hudRows; y < 23; y++ {
		for x := range 80 {
			if y == py && x >= px && x < px+3 {
				continue
			}
			if got, want := fb.At(x, y).Character, g.level.At(x+ox, y+oy).Cell.Character; got != want {
				t.Fatalf("(%d, %d): wanted the level's %q, got %q", x, y, want, got)
			}
		}
	}
}

func firstOf(g *worldGame) *sprite.Sprite {
	for c := range g.coins.All() {
		return c
	}
	return nil
}

func TestDrawAtEverySize(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {2, 2}, {3, 3}, {20, 8}, {80, 24}, {121, 37}, {200, 60}, {400, 100}} {
		g := newSizedGame(size[0], size[1])
		fb := newFrame(t, size[0], size[1])
		for range 30 {
			if err := g.Update(0.1, hold(engine.KeyRight, engine.KeyDown)); err != nil {
				t.Fatalf("%v: unexpected error: %v", size, err)
			}
			g.Draw(fb)
		}
		g.paused = true
		g.Draw(fb)
	}
}

func TestTheHUDShowsTheCoinsAndTheKeys(t *testing.T) {
	g := newSizedGame(80, 24)
	fb := newFrame(t, 80, 24)
	g.Draw(fb)
	lines := strings.Split(fb.String(), "\n")
	if !strings.Contains(lines[0], "┌─ coins ─") || !strings.Contains(lines[1], "│  0/36 ░░░░░░░░░░░░░░░░ │") || !strings.HasPrefix(lines[2], "└──") {
		t.Errorf("wanted the panel with the count and an empty bar on top, got\n%s\n%s\n%s", lines[0], lines[1], lines[2])
	}
	if !strings.Contains(lines[23], "arrows walk  Esc pauses") {
		t.Errorf("wanted the keys at the bottom, got %q", lines[23])
	}
	small := newFrame(t, 12, 4)
	newSizedGame(12, 4).Draw(small)
	if strings.Contains(small.String(), "Esc") {
		t.Errorf("wanted the keys left out where they do not fit, got\n%s", small)
	}
}

func TestTheBarFillsAsCoinsAreCollected(t *testing.T) {
	g := newSizedGame(80, 24)
	spots := make([]geometry.Vector2, 0, g.total)
	for c := range g.coins.All() {
		spots = append(spots, c.Position)
	}
	for _, p := range spots[:18] { // half of them
		g.player.Position = geometry.Vector2{X: p.X - 1, Y: p.Y}
		g.Update(0, hold())
	}
	fb := newFrame(t, 80, 24)
	g.Draw(fb)
	if got := strings.Split(fb.String(), "\n")[1]; !strings.Contains(got, "│ 18/36 ████████░░░░░░░░ │") {
		t.Errorf("wanted half the bar filled at 18 of 36, got %q", got)
	}
	if got := fb.At(8, 1); got.Character != '█' || got.Foreground != engine.Palette(226) || got.Background != hudStyle.Background {
		t.Errorf("wanted the bar's first cell yellow on the panel's background, got %+v", got)
	}
}

func TestEscapeOpensThePauseMenuAndOtherKeysDoNot(t *testing.T) {
	g := newSizedGame(80, 24)
	kb := engine.NewKeyboard()
	kb.Press('q', engine.KeyEnter)
	if err := g.Update(0.1, kb.Input()); err != nil || g.paused {
		t.Fatalf("wanted other keys to be ignored, got %v and paused %v", err, g.paused)
	}
	kb.EndFrame()
	kb.Press(engine.KeyEscape)
	if err := g.Update(0.1, kb.Input()); err != nil || !g.paused {
		t.Errorf("wanted Escape to pause without quitting, got %v and paused %v", err, g.paused)
	}
}

func TestTheWorldStandsStillWhilePaused(t *testing.T) {
	g := newSizedGame(80, 24)
	g.Update(0, hold(engine.KeyEscape))
	before, view := g.player.Position, g.view
	for range 30 {
		g.Update(0.1, hold(engine.KeyRight, engine.KeyDown))
	}
	if g.player.Position != before || g.view != view || !g.paused {
		t.Errorf("wanted the player to stay at %v while paused, got %v (paused %v)", before, g.player.Position, g.paused)
	}
}

func TestThePauseMenuResumesAndQuits(t *testing.T) {
	pause := func() *worldGame {
		g := newSizedGame(80, 24)
		g.Update(0, hold(engine.KeyEscape))
		return g
	}

	// Enter on Resume, the first item, goes back to the game.
	g := pause()
	if err := g.Update(0, hold(engine.KeyEnter)); err != nil || g.paused {
		t.Errorf("wanted Resume to unpause, got %v and paused %v", err, g.paused)
	}
	// And the player walks again.
	before := g.player.Position
	g.Update(0.1, hold(engine.KeyRight))
	if g.player.Position == before {
		t.Errorf("wanted the player to walk after resuming, still at %v", before)
	}

	// Escape again does the same.
	g = pause()
	if err := g.Update(0, hold(engine.KeyEscape)); err != nil || g.paused {
		t.Errorf("wanted Escape to unpause, got %v and paused %v", err, g.paused)
	}

	// Down to Quit and Enter ends the game.
	g = pause()
	if err := g.Update(0, hold(engine.KeyDown)); err != nil || !g.paused {
		t.Fatalf("wanted Down to stay in the menu, got %v and paused %v", err, g.paused)
	}
	if err := g.Update(0, hold(engine.KeyEnter)); !errors.Is(err, engine.ErrQuit) {
		t.Errorf("wanted ErrQuit from Quit, got %v", err)
	}

	// The menu opens on Resume every time, not where it was left.
	g = pause()
	g.Update(0, hold(engine.KeyDown))
	g.Update(0, hold(engine.KeyEscape))
	g.Update(0, hold(engine.KeyEscape))
	if !g.paused || g.menu.Selected != resumeItem {
		t.Errorf("wanted the menu reopened on Resume, got paused %v and item %d", g.paused, g.menu.Selected)
	}
}

func TestThePauseMenuIsDrawnOverTheWorldInTheMiddle(t *testing.T) {
	g := newSizedGame(80, 24)
	g.Update(0, hold(engine.KeyEscape))
	fb := newFrame(t, 80, 24)
	g.Draw(fb)

	lines := strings.Split(fb.String(), "\n")
	row := func(text string) int {
		for y, l := range lines {
			if strings.Contains(l, text) {
				return y
			}
		}
		t.Fatalf("wanted %q on the screen, got\n%s", text, fb)
		return -1
	}
	title, resume, quit := row("paused"), row("Resume"), row("Quit")
	if resume != title+1 || quit != title+2 || title != 10 {
		t.Errorf("wanted the panel in the middle, title row 10 and the items under it, got rows %d, %d and %d", title, resume, quit)
	}
	if !strings.Contains(lines[23], "Esc resumes") {
		t.Errorf("wanted the keys for the menu at the bottom, got %q", lines[23])
	}

	// The selected item is reversed, the other is not, and the world is still around the panel.
	x := strings.Index(lines[resume], "Resume")
	x = len([]rune(lines[resume][:x]))
	if !fb.At(x, resume).Attrs.Has(engine.Reverse) || fb.At(x, quit).Attrs.Has(engine.Reverse) {
		t.Errorf("wanted Resume reversed and Quit not, got %+v and %+v", fb.At(x, resume), fb.At(x, quit))
	}
	if fb.At(0, 12).Character != g.level.At(int(g.view.Min.X), 12+int(g.view.Min.Y)).Cell.Character {
		t.Errorf("wanted the world to show to the left of the panel")
	}
}

// A coin is drawn in the cell of its tile and animated, with the floor around it, wherever the
// view is.
func TestACoinIsDrawnInTheCellOfItsTile(t *testing.T) {
	g := newSizedGame(80, 24)
	g.player.Position = geometry.Vector2{X: 40.5, Y: 20.5}
	g.follow()
	fb := newFrame(t, 80, 24)
	g.Draw(fb)
	ox, oy := int(math.Floor(g.view.Min.X)), int(math.Floor(g.view.Min.Y))

	isCoin := map[[2]int]bool{}
	for coin := range g.coins.All() {
		isCoin[[2]int{int(coin.Position.X), int(coin.Position.Y)}] = true
	}

	var checked int
	for coin := range g.coins.All() {
		cx, cy := int(coin.Position.X)-ox, int(coin.Position.Y)-oy
		if cx < 1 || cx > 78 || cy < 1 || cy > 22 {
			continue
		}
		if got := fb.At(cx, cy).Character; got != 'o' && got != 'O' && got != '0' {
			t.Errorf("coin at screen (%d, %d): wanted the coin, got %q", cx, cy, got)
		}
		if !isCoin[[2]int{cx - 1 + ox, cy + oy}] {
			if got, want := fb.At(cx-1, cy).Character, g.level.At(cx-1+ox, cy+oy).Cell.Character; got != want {
				t.Errorf("beside the coin at (%d, %d): wanted the level's %q, got %q", cx, cy, want, got)
			}
		}
		if bg := fb.At(cx, cy).Background; bg != legend['.'].Cell.Background {
			t.Errorf("coin at (%d, %d): wanted the floor behind it, got %v", cx, cy, bg)
		}
		checked++
	}
	if checked < 2 {
		t.Errorf("wanted at least two coins in view to check, got %d", checked)
	}
}

// Holding Right and Down along the bottom of the first room slides the player along the wall
// until the opening of the corridor that leads down from it, and then down the corridor: the
// slide is what finds the way in, at any frame rate, without the player lining up with it.
func TestSlidingAlongAWallFindsTheWayDownTheCorridor(t *testing.T) {
	for _, dt := range []float64{1.0 / 60, 1.0 / 30, 1.0 / 24, 0.04, 0.05} {
		g := newSizedGame(80, 24)
		for range 90 {
			g.Update(dt, hold(engine.KeyLeft)) // to the west wall
		}
		for range 90 {
			g.Update(dt, hold(engine.KeyDown)) // to the bottom of the room
		}
		bottom := g.player.Position.Y
		for range 300 {
			g.Update(dt, hold(engine.KeyRight, engine.KeyDown))
		}
		if g.player.Position.Y < bottom+8 {
			t.Errorf("dt %v: wanted the player to slide to the corridor and down it from %v, got %v", dt, bottom, g.player.Position)
		}
	}
}

package tilemap

import (
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"glonk.dev/geometry"
)

// open reports whether (x, y) is a tile of the map that is not solid.
func open(m *Map, x, y int) bool { return !m.Solid(x, y) }

// edge says whether a mover can step from the tile a to the tile b, which are neighbors: both
// are open, and for a diagonal step so are both tiles beside it. It is the rule written out for
// the reference, and shares nothing with the search.
func edge(m *Map, ax, ay, bx, by int) bool {
	if !open(m, ax, ay) || !open(m, bx, by) {
		return false
	}
	if ax != bx && ay != by {
		return open(m, bx, ay) && open(m, ax, by)
	}
	return true
}

// referenceSteps is the steps from every tile of the map to the nearest target, by Floyd and
// Warshall's shortest paths over the graph of edges, with -1 for a tile with no way to one, in
// row order. It is slow and sure, for small maps.
func referenceSteps(m *Map, targets []geometry.Vector2) []int {
	w, h := m.Width(), m.Height()
	n := w * h
	// No path is anywhere near this long, and two of them added still fit an int of 32 bits.
	const far = 1 << 20
	dist := make([][]int, n)
	for i := range dist {
		dist[i] = make([]int, n)
		for j := range dist[i] {
			dist[i][j] = far
		}
		dist[i][i] = 0
	}
	for y := range h {
		for x := range w {
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if (dx != 0 || dy != 0) && edge(m, x, y, x+dx, y+dy) {
						dist[y*w+x][(y+dy)*w+x+dx] = 1
					}
				}
			}
		}
	}
	for k := range n {
		for i := range n {
			for j := range n {
				if dist[i][k]+dist[k][j] < dist[i][j] {
					dist[i][j] = dist[i][k] + dist[k][j]
				}
			}
		}
	}
	out := make([]int, n)
	for i := range out {
		out[i] = -1
		if m.Solid(i%w, i/w) {
			continue
		}
		for _, t := range targets {
			if math.IsNaN(t.X) || math.IsNaN(t.Y) || math.IsInf(t.X, 0) || math.IsInf(t.Y, 0) {
				continue
			}
			tx, ty := int(math.Floor(t.X)), int(math.Floor(t.Y))
			if m.Solid(tx, ty) {
				continue
			}
			if d := dist[ty*w+tx][i]; d < far && (out[i] == -1 || d < out[i]) {
				out[i] = d
			}
		}
	}
	return out
}

// center is the middle of the tile (x, y).
func center(x, y int) geometry.Vector2 { return pt(float64(x)+0.5, float64(y)+0.5) }

// route searches m for targets and returns the Route.
func route(m *Map, targets ...geometry.Vector2) *Route {
	var r Route
	r.To(m, targets...)
	return &r
}

// randomTargets picks one to three targets, in tiles or anywhere near them, some in walls,
// outside the map, or the same as another.
func randomTargets(rng *rand.Rand, w, h int) []geometry.Vector2 {
	ts := make([]geometry.Vector2, 1+rng.IntN(3))
	for i := range ts {
		ts[i] = pt(float64(rng.IntN(w+4))-2+float64(rng.IntN(4))/4, float64(rng.IntN(h+4))-2+float64(rng.IntN(4))/4)
	}
	if rng.IntN(5) == 0 {
		ts = append(ts, ts[0])
	}
	return ts
}

// The steps of every tile are the reference's, for hundreds of random small maps with walls,
// with one to three targets that can be in walls or outside, and every point of every tile.
func TestRouteStepsAreTheShortestPathsOfTheReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(51, 52))
	var reachable, blocked int
	for range 400 {
		w, h := 3+rng.IntN(5), 3+rng.IntN(4)
		m := randomLevel(rng, w, h)
		targets := randomTargets(rng, w, h)
		r := route(m, targets...)
		want := referenceSteps(m, targets)
		for y := range h {
			for x := range w {
				for _, off := range []geometry.Vector2{{X: 0, Y: 0}, {X: 0.5, Y: 0.5}, {X: 0.999, Y: 0.001}} {
					got, ok := r.Steps(pt(float64(x)+off.X, float64(y)+off.Y))
					if w := want[y*w+x]; ok != (w >= 0) || (ok && got != w) {
						t.Fatalf("Steps at (%v, %v) from %v was %d %v, wanted %d, in\n%s", float64(x)+off.X, float64(y)+off.Y, targets, got, ok, w, show(m))
					}
				}
				if want[y*w+x] >= 0 {
					reachable++
				} else {
					blocked++
				}
			}
		}
	}
	if reachable < 2000 || blocked < 2000 {
		t.Errorf("wanted both kinds of tile to be tried, got %d reachable and %d not", reachable, blocked)
	}
}

// From every tile with a way to a target, Next is the center of a neighbor that a mover can step
// to along an edge, one step nearer, a straight one before a diagonal one, and following it
// arrives, in as many moves as there are steps.
func TestRouteNextIsAStepNearerAlongAnEdge(t *testing.T) {
	rng := rand.New(rand.NewPCG(53, 54))
	var walked int
	for range 300 {
		w, h := 3+rng.IntN(6), 3+rng.IntN(5)
		m := randomLevel(rng, w, h)
		targets := randomTargets(rng, w, h)
		r := route(m, targets...)
		want := referenceSteps(m, targets)
		for y := range h {
			for x := range w {
				here := want[y*w+x]
				from := pt(float64(x)+0.25, float64(y)+0.75)
				next, ok := r.Next(from)
				if here <= 0 {
					if ok {
						t.Fatalf("Next from (%d, %d), %d steps away, was %v, wanted none", x, y, here, next)
					}
					continue
				}
				if !ok {
					t.Fatalf("Next from (%d, %d), %d steps away, was none, in\n%s", x, y, here, show(m))
				}
				nx, ny := int(math.Floor(next.X)), int(math.Floor(next.Y))
				if next != center(nx, ny) {
					t.Fatalf("Next from (%d, %d) was %v, wanted the center of a tile", x, y, next)
				}
				if (nx != x || ny != y) && !edge(m, x, y, nx, ny) || abs(nx-x) > 1 || abs(ny-y) > 1 || (nx == x && ny == y) {
					t.Fatalf("Next from (%d, %d) was tile (%d, %d), which is not a step, in\n%s", x, y, nx, ny, show(m))
				}
				if want[ny*w+nx] != here-1 {
					t.Fatalf("Next from (%d, %d), %d steps away, went to a tile %d steps away", x, y, here, want[ny*w+nx])
				}
				// A straight step before a diagonal one.
				if nx != x && ny != y {
					for _, d := range [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
						if sx, sy := x+d[0], y+d[1]; edge(m, x, y, sx, sy) && want[sy*w+sx] == here-1 {
							t.Fatalf("Next from (%d, %d) went diagonally though the straight step to (%d, %d) is as near", x, y, sx, sy)
						}
					}
				}
				// Following it arrives.
				p, moves := center(x, y), 0
				for {
					n, ok := r.Next(p)
					if !ok {
						break
					}
					p = n
					moves++
					if moves > here {
						t.Fatalf("from (%d, %d), %d steps away, still walking after %d moves", x, y, here, moves)
					}
				}
				if moves != here {
					t.Fatalf("from (%d, %d), %d steps away, arrived in %d moves", x, y, here, moves)
				}
				walked++
			}
		}
	}
	if walked < 1500 {
		t.Errorf("wanted many walks to be tried, got %d", walked)
	}
}

func abs(v int) int { return max(v, -v) }

func TestRouteInAnOpenRoomIsTheKingsDistance(t *testing.T) {
	m := New(7, 5, Tile{})
	r := route(m, center(3, 2))
	for y := range 5 {
		for x := range 7 {
			if got, ok := r.Steps(center(x, y)); !ok || got != max(abs(x-3), abs(y-2)) {
				t.Errorf("(%d, %d): wanted %d steps, got %d %v", x, y, max(abs(x-3), abs(y-2)), got, ok)
			}
		}
	}
}

func TestARouteGoesRoundAWallThroughItsDoorway(t *testing.T) {
	m := level(
		".....#.....",
		".....#.....",
		".....#.....",
		"...........",
		".....#.....",
	)
	r := route(m, center(9, 1))
	if got, ok := r.Steps(center(1, 1)); !ok || got != 8 { // down to the doorway at row 3, across, up
		t.Errorf("wanted 8 steps round the wall, got %d %v", got, ok)
	}
	// From the left of the wall, the first step goes toward the doorway, not at the wall.
	next, ok := r.Next(center(4, 0))
	if !ok || next.Y < 0.5+0.9 {
		t.Errorf("wanted a step down toward the doorway, got %v %v", next, ok)
	}
	if got, ok := r.Steps(center(5, 1)); ok {
		t.Errorf("wanted no steps in a wall, got %d", got)
	}
}

func TestARouteDoesNotCutTheCornerOfAWall(t *testing.T) {
	// The only way from the left tile to the right one is the diagonal between two walls that
	// touch at a corner: a mover would clip them, so it is no way.
	crack := level(".#", "#.")
	if _, ok := route(crack, center(1, 1)).Steps(center(0, 0)); ok {
		t.Errorf("wanted no way through a diagonal gap between two walls")
	}
	// With one side solid the diagonal is a corner too.
	one := level("..", ".#")
	r := route(one, center(1, 0))
	if got, ok := r.Steps(center(0, 1)); !ok || got != 2 {
		t.Errorf("wanted 2 steps, round the open side, got %d %v", got, ok)
	}
	if next, ok := r.Next(center(0, 1)); !ok || next != center(0, 0) {
		t.Errorf("wanted the step to be up the open side, to the center of (0, 0), got %v %v", next, ok)
	}
	// With both sides open it is a step.
	free := level("..", "..")
	if got, ok := route(free, center(1, 1)).Steps(center(0, 0)); !ok || got != 1 {
		t.Errorf("wanted a diagonal step to count as 1, got %d %v", got, ok)
	}
}

// Next never takes a diagonal step through a corner, even when the tile it goes to is as near as the
// one it should take, and is met first: either tile beside the step can be the wall.
func TestNextDoesNotTakeAnEqualDiagonalThroughACorner(t *testing.T) {
	for name, m := range map[string]*Map{
		"the wall below the step":  level("...", "...", ".#."),
		"the wall beside the step": level("...", "..#", "..."),
	} {
		// Targets at the top left and the bottom right: from the middle both are a diagonal step
		// away as far as the tiles go, but the way to the bottom right is through a corner.
		r := route(m, center(0, 0), center(2, 2))
		if got, ok := r.Steps(center(1, 1)); !ok || got != 1 {
			t.Fatalf("%s: wanted 1 step, got %d %v", name, got, ok)
		}
		if next, ok := r.Next(center(1, 1)); !ok || next != center(0, 0) {
			t.Errorf("%s: wanted the step to be to (0, 0), not through the corner to (2, 2), got %v %v", name, next, ok)
		}
	}
}

func TestARouteWithSeveralTargetsGoesToTheNearest(t *testing.T) {
	m := New(11, 1, Tile{})
	r := route(m, center(0, 0), center(10, 0))
	for x, want := range []int{0, 1, 2, 3, 4, 5, 4, 3, 2, 1, 0} {
		if got, ok := r.Steps(center(x, 0)); !ok || got != want {
			t.Errorf("tile %d: wanted %d steps, got %d %v", x, want, got, ok)
		}
	}
	if next, ok := r.Next(center(7, 0)); !ok || next != center(8, 0) {
		t.Errorf("wanted the step from 7 to be to 8, the nearer target, got %v %v", next, ok)
	}
	// A target twice is a target once.
	twice := route(m, center(5, 0), pt(5.9, 0.1))
	if got, _ := twice.Steps(center(0, 0)); got != 5 {
		t.Errorf("wanted 5 steps to a target given twice, got %d", got)
	}
}

func TestARouteIgnoresTargetsThatAreNotInOpenTiles(t *testing.T) {
	m := level("...#.", "...#.")
	for name, target := range map[string]geometry.Vector2{
		"in a wall":     pt(3.5, 0.5),
		"outside":       pt(-0.5, 0.5),
		"far outside":   pt(1e300, -1e300),
		"NaN":           pt(math.NaN(), 0.5),
		"infinity":      pt(0.5, math.Inf(1)),
		"below the map": pt(1, 2),
	} {
		r := route(m, target)
		if got, ok := r.Steps(center(0, 0)); ok {
			t.Errorf("%s: wanted nothing to be reached from a target that is left out, got %d", name, got)
		}
	}
	// Another target that is fine still works.
	r := route(m, pt(3.5, 0.5), center(1, 1))
	if got, ok := r.Steps(center(0, 0)); !ok || got != 1 {
		t.Errorf("wanted the good target to count, got %d %v", got, ok)
	}
	// No targets at all.
	if _, ok := route(m).Steps(center(0, 0)); ok {
		t.Errorf("wanted nothing to be reached with no target")
	}
}

func TestATileThatNoTargetCanBeReachedFromHasNoSteps(t *testing.T) {
	m := level(
		"..#..",
		"..#..",
		"..#..",
	)
	r := route(m, center(0, 0))
	if _, ok := r.Steps(center(4, 1)); ok {
		t.Errorf("wanted the far side of a full wall not to be reached")
	}
	if _, ok := r.Next(center(4, 1)); ok {
		t.Errorf("wanted no step from a tile that cannot reach the target")
	}
	if got, ok := r.Steps(center(1, 2)); !ok || got != 2 {
		t.Errorf("wanted the near side to be reached, got %d %v", got, ok)
	}
}

func TestEveryPointOfATileGivesTheAnswerOfTheTile(t *testing.T) {
	m := level("....", ".##.", "....")
	r := route(m, center(3, 1))
	want, _ := r.Steps(center(0, 1))
	wantNext, _ := r.Next(center(0, 1))
	for _, p := range []geometry.Vector2{pt(0, 1), pt(0.999999, 1), pt(0, 1.999999), pt(0.5, 1.5), pt(0.0000001, 1.9999999)} {
		if got, ok := r.Steps(p); !ok || got != want {
			t.Errorf("Steps at %v was %d %v, wanted %d", p, got, ok, want)
		}
		if got, ok := r.Next(p); !ok || got != wantNext {
			t.Errorf("Next at %v was %v %v, wanted %v", p, got, ok, wantNext)
		}
	}
	// The edge between tiles is in the tile to its right or below it.
	if got, _ := r.Steps(pt(1, 1)); got == want {
		if _, ok := r.Steps(pt(1.5, 1.5)); ok {
			t.Errorf("wanted (1, 1) to be in the wall at tile (1, 1)")
		}
	}
	if _, ok := r.Steps(pt(1, 1)); ok {
		t.Errorf("wanted the point (1, 1) to be in the wall below and to its right")
	}
}

func TestAnArrivedMoverHasNoStep(t *testing.T) {
	m := New(3, 3, Tile{})
	r := route(m, center(1, 1))
	if got, ok := r.Steps(pt(1.9, 1.1)); !ok || got != 0 {
		t.Errorf("wanted 0 steps in the target's tile, got %d %v", got, ok)
	}
	if next, ok := r.Next(pt(1.9, 1.1)); ok {
		t.Errorf("wanted no step in the target's tile, got %v", next)
	}
}

func TestAZeroNilOrEmptyRouteHasNothing(t *testing.T) {
	var zero Route
	if _, ok := zero.Steps(center(0, 0)); ok {
		t.Errorf("wanted the zero Route to have no steps")
	}
	if _, ok := zero.Next(center(0, 0)); ok {
		t.Errorf("wanted the zero Route to have no step")
	}
	for name, m := range map[string]*Map{"nil": nil, "empty": New(0, 0, Tile{})} {
		r := route(m, center(0, 0))
		if _, ok := r.Steps(center(0, 0)); ok {
			t.Errorf("%s: wanted nothing", name)
		}
	}
	// A map with one tile: it is a target and nowhere to go.
	one := route(New(1, 1, Tile{}), center(0, 0))
	if got, ok := one.Steps(center(0, 0)); !ok || got != 0 {
		t.Errorf("wanted 0 steps on a map of one tile, got %d %v", got, ok)
	}
}

func TestNaNInfinityAndFarPointsHaveNoSteps(t *testing.T) {
	r := route(New(3, 3, Tile{}), center(1, 1))
	for _, p := range []geometry.Vector2{
		pt(math.NaN(), 1), pt(1, math.NaN()), pt(math.Inf(1), 1), pt(1, math.Inf(-1)),
		pt(1e300, 1), pt(1, -1e300), pt(-0.5, 1), pt(3, 1), pt(1, 3),
	} {
		if _, ok := r.Steps(p); ok {
			t.Errorf("wanted no steps at %v", p)
		}
		if _, ok := r.Next(p); ok {
			t.Errorf("wanted no step at %v", p)
		}
	}
}

// A search on a bigger map, a smaller one and one of the same size each gives the right answer,
// with nothing left over from the one before.
func TestARouteSearchedAgainHasNothingLeftOver(t *testing.T) {
	rng := rand.New(rand.NewPCG(55, 56))
	var r Route
	for _, size := range [][2]int{{5, 4}, {9, 7}, {3, 3}, {9, 7}, {6, 6}, {1, 1}, {7, 3}} {
		m := randomLevel(rng, size[0], size[1])
		targets := randomTargets(rng, size[0], size[1])
		r.To(m, targets...)
		want := referenceSteps(m, targets)
		for y := range size[1] {
			for x := range size[0] {
				got, ok := r.Steps(center(x, y))
				if w := want[y*size[0]+x]; ok != (w >= 0) || (ok && got != w) {
					t.Fatalf("%d by %d after another search, tile (%d, %d): wanted %d, got %d %v", size[0], size[1], x, y, w, got, ok)
				}
			}
		}
		if _, ok := r.Steps(center(size[0], 0)); ok {
			t.Fatalf("wanted the tile past the map's width to have no steps")
		}
	}
}

func TestRouteDoesNotAllocateOnceItHasSearched(t *testing.T) {
	m := level(
		"..........",
		"....#.....",
		"..........",
	)
	var r Route
	r.To(m, center(0, 0))
	if n := testing.AllocsPerRun(100, func() {
		r.To(m, center(9, 2))
		r.To(m, center(0, 0), center(9, 2))
		r.Steps(center(5, 1))
		r.Next(center(5, 0))
	}); n != 0 {
		t.Errorf("wanted no allocation, got %v", n)
	}
}

// walk steers a box of half a tile to the target with Next and Move, a step of at most speed a
// frame, and reports where it ended up and whether it got to the target's tile in time.
func walk(m *Map, r *Route, from geometry.Vector2, target geometry.Vector2, frames int, speed float64, straight bool) (end geometry.Vector2, arrived bool) {
	b := geometry.NewRect(from.X-0.25, from.Y-0.25, 0.5, 0.5)
	tx, ty := int(math.Floor(target.X)), int(math.Floor(target.Y))
	for range frames {
		c := b.Center()
		if int(math.Floor(c.X)) == tx && int(math.Floor(c.Y)) == ty {
			return c, true
		}
		aim := target
		if !straight {
			next, ok := r.Next(c)
			if !ok {
				return c, false
			}
			aim = next
		}
		d := geometry.Vector2{X: aim.X - c.X, Y: aim.Y - c.Y}
		if n := math.Hypot(d.X, d.Y); n > speed {
			d = geometry.Vector2{X: d.X / n * speed, Y: d.Y / n * speed}
		}
		b, _, _ = m.Move(b, d)
	}
	return b.Center(), false
}

// The enemy that walks straight at the player stops at the first wall between them, and stays
// there, which is the reason for the route; the one that follows Next gets there, round every
// corner, without Move ever holding it against a wall that the route went too near.
func TestAMoverFollowingTheRouteArrivesWhereOneGoingStraightIsStuck(t *testing.T) {
	mazes := map[string]struct {
		m        *Map
		from, to geometry.Vector2
	}{
		"a wall with a doorway": {level(
			"............",
			".....#......",
			".....#......",
			".....#......",
			".....#......",
			"............",
		), center(1, 2), center(10, 2)},
		"a winding corridor": {level(
			"#########",
			"#...#...#",
			"#.#.#.#.#",
			"#.#...#.#",
			"#.#####.#",
			"#.......#",
			"#########",
		), center(1, 1), center(7, 1)},
		"a corridor with diagonal corners": {level(
			"#######",
			"#.....#",
			"#.###.#",
			"#.#...#",
			"#.###.#",
			"#.....#",
			"#######",
		), center(1, 1), center(3, 3)},
		"a U-shaped trap": {level(
			".........",
			".#######.",
			".#.....#.",
			".#.###.#.",
			".#.#.#.#.",
			".#.#.#.#.",
			"...#.....",
		), center(4, 5), center(4, 0)},
	}
	for name, tt := range mazes {
		t.Run(name, func(t *testing.T) {
			r := route(tt.m, tt.to)
			if _, ok := r.Steps(tt.from); !ok {
				t.Fatalf("wanted the start to have a way to the target, in\n%s", show(tt.m))
			}
			if end, arrived := walk(tt.m, r, tt.from, tt.to, 4000, 0.2, true); arrived {
				t.Errorf("wanted a mover going straight at the target to be stuck, but it arrived at %v", end)
			}
			if end, arrived := walk(tt.m, r, tt.from, tt.to, 4000, 0.2, false); !arrived {
				t.Errorf("wanted a mover following the route to arrive, but it ended at %v, in\n%s", end, show(tt.m))
			}
		})
	}
}

// Any walls, targets and points: no panic, every reachable tile's step is legal and nearer, and the
// steps agree with a walk of the route.
func FuzzRoute(f *testing.F) {
	f.Add(5, 4, "..#....#.#..##.....", 0.5, 0.5, 3.5, 2.5)
	f.Add(1, 1, "", 0.0, 0.0, 0.0, 0.0)
	f.Add(8, 8, strings.Repeat("#.", 40), math.NaN(), 1.0, 1e300, -1.0)
	f.Add(3, 3, "#########", 1.0, 1.0, 1.0, 1.0)

	f.Fuzz(func(t *testing.T, w, h int, walls string, tx, ty, px, py float64) {
		w, h = 1+int(uint(w)%9), 1+int(uint(h)%9) // abs of the smallest int is negative
		m := New(w, h, Tile{})
		for i := range w * h {
			if i < len(walls) && walls[i] == '#' {
				m.Set(i%w, i/w, wall)
			}
		}
		var r Route
		r.To(m, pt(tx, ty), pt(px, py))
		for y := range h {
			for x := range w {
				here, ok := r.Steps(center(x, y))
				next, nok := r.Next(center(x, y))
				if ok != (here >= 0 && !m.Solid(x, y)) && ok {
					t.Fatalf("(%d, %d): a solid tile has steps", x, y)
				}
				if nok != (ok && here > 0) {
					t.Fatalf("(%d, %d): Next is %v for %d steps %v", x, y, nok, here, ok)
				}
				if nok {
					nx, ny := int(math.Floor(next.X)), int(math.Floor(next.Y))
					if !edge(m, x, y, nx, ny) {
						t.Fatalf("(%d, %d): Next went to (%d, %d), which is not a step", x, y, nx, ny)
					}
					if got, _ := r.Steps(next); got != here-1 {
						t.Fatalf("(%d, %d): Next went to a tile %d steps away, wanted %d", x, y, got, here-1)
					}
				}
			}
		}
	})
}

func BenchmarkRoute(b *testing.B) {
	rng := rand.New(rand.NewPCG(57, 58))
	open := New(400, 200, Tile{})
	walls := New(400, 200, Tile{})
	for y := range 200 {
		for x := range 400 {
			if rng.IntN(100) < 20 {
				walls.Set(x, y, wall)
			}
		}
	}
	walls.Set(200, 100, Tile{}) // the target is in the open
	var r Route
	b.Run("To, across an open 400 by 200 map", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			r.To(open, center(200, 100))
		}
	})
	b.Run("To, across a 400 by 200 map with a fifth of it wall", func(b *testing.B) {
		r.To(walls, center(200, 100))
		reached := 0
		for _, s := range r.steps {
			if s != unreached {
				reached++
			}
		}
		if reached < 40_000 {
			b.Fatalf("wanted the search to cross most of the map, it reached %d tiles", reached)
		}
		b.ReportAllocs()
		for b.Loop() {
			r.To(walls, center(200, 100))
		}
	})
	b.Run("To, from three targets in a small map", func(b *testing.B) {
		small := New(40, 20, Tile{})
		var s Route
		b.ReportAllocs()
		for b.Loop() {
			s.To(small, center(1, 1), center(38, 18), center(20, 10))
		}
	})
	r.To(open, center(200, 100))
	b.Run("Next", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			r.Next(pt(37.5, 12.5))
		}
	})
	b.Run("Steps", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			r.Steps(pt(37.5, 12.5))
		}
	})
}

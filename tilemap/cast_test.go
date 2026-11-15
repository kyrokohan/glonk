package tilemap

import (
	"math"
	"math/big"
	"math/rand/v2"
	"strings"
	"testing"

	"glonk.dev/geometry"
)

// level makes a map from text art in which # is a wall and anything else is floor.
func level(lines ...string) *Map {
	return FromLines(map[rune]Tile{'#': wall}, lines...)
}

func pt(x, y float64) geometry.Vector2 { return geometry.Vector2{X: x, Y: y} }

// interval is a range of t, from lo to hi, each end in the range or open; a nil end has no limit.
type interval struct {
	lo, hi         *big.Rat
	loOpen, hiOpen bool
}

func (iv *interval) raiseLo(v *big.Rat, open bool) {
	if iv.lo == nil || v.Cmp(iv.lo) > 0 || (v.Cmp(iv.lo) == 0 && open) {
		iv.lo, iv.loOpen = v, open
	}
}

func (iv *interval) lowerHi(v *big.Rat, open bool) {
	if iv.hi == nil || v.Cmp(iv.hi) < 0 || (v.Cmp(iv.hi) == 0 && open) {
		iv.hi, iv.hiOpen = v, open
	}
}

func (iv *interval) empty() bool {
	if iv.lo == nil || iv.hi == nil {
		return false
	}
	c := iv.lo.Cmp(iv.hi)
	return c > 0 || (c == 0 && (iv.loOpen || iv.hiOpen))
}

// inCell narrows the range to the t for which o + t*d is in the cell [c, c+1) on this axis: at
// or past its first edge and before its last, which is the floor rule.
func (iv *interval) inCell(o, d *big.Rat, c int) {
	first := new(big.Rat).Sub(big.NewRat(int64(c), 1), o)
	last := new(big.Rat).Sub(big.NewRat(int64(c)+1, 1), o)
	if d.Sign() == 0 {
		if first.Sign() > 0 || last.Sign() <= 0 { // never in it
			iv.raiseLo(big.NewRat(1, 1), false)
			iv.lowerHi(big.NewRat(0, 1), false)
		}
		return
	}
	t0, t1 := new(big.Rat).Quo(first, d), new(big.Rat).Quo(last, d)
	if d.Sign() > 0 {
		iv.raiseLo(t0, false)
		iv.lowerHi(t1, true)
	} else {
		iv.raiseLo(t1, true)
		iv.lowerHi(t0, false)
	}
}

func rat(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }

// oracleSees decides Sees with exact arithmetic and no walk: it asks, of every tile the line
// could be in, whether the line has a point in it, by solving for the range of t.
func oracleSees(m *Map, a, b geometry.Vector2) bool {
	ax, ay, dx, dy := rat(a.X), rat(a.Y), rat(b.X-a.X), rat(b.Y-a.Y)
	for cy := int(math.Floor(min(a.Y, b.Y))); cy <= int(math.Floor(max(a.Y, b.Y))); cy++ {
		for cx := int(math.Floor(min(a.X, b.X))); cx <= int(math.Floor(max(a.X, b.X))); cx++ {
			if !m.Solid(cx, cy) {
				continue
			}
			iv := interval{}
			iv.raiseLo(big.NewRat(0, 1), false)
			iv.lowerHi(big.NewRat(1, 1), false)
			iv.inCell(ax, dx, cx)
			iv.inCell(ay, dy, cy)
			if !iv.empty() {
				return false
			}
		}
	}
	return true
}

// oracleCast decides Cast the same way: the solid tile whose range of t, from 0, starts first,
// with a range that starts at a point in the tile before one that starts just after it.
func oracleCast(m *Map, o, d geometry.Vector2) (Hit, bool) {
	ox, oy, dx, dy := rat(o.X), rat(o.Y), rat(d.X), rat(d.Y)
	var best *interval
	var bx, by int
	try := func(cx, cy int) {
		if !m.Solid(cx, cy) {
			return
		}
		iv := &interval{}
		iv.raiseLo(big.NewRat(0, 1), false)
		iv.inCell(ox, dx, cx)
		iv.inCell(oy, dy, cy)
		if iv.empty() {
			return
		}
		if best == nil || iv.lo.Cmp(best.lo) < 0 || (iv.lo.Cmp(best.lo) == 0 && !iv.loOpen && best.loOpen) {
			best, bx, by = iv, cx, cy
		}
	}
	// Every tile of the map and the ring round it, which a ray from inside must reach, and the
	// tile the ray starts in, for one that starts outside.
	for cy := -1; cy <= m.Height(); cy++ {
		for cx := -1; cx <= m.Width(); cx++ {
			try(cx, cy)
		}
	}
	try(int(math.Floor(o.X)), int(math.Floor(o.Y)))
	if best == nil {
		return Hit{}, false
	}
	t, _ := best.lo.Float64()
	return Hit{Point: pt(o.X+t*d.X, o.Y+t*d.Y), X: bx, Y: by, Distance: t * math.Hypot(d.X, d.Y)}, true
}

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-9*max(1, math.Abs(a), math.Abs(b)) }

func sameHit(got, want Hit) bool {
	return got.X == want.X && got.Y == want.Y && near(got.Point.X, want.Point.X) && near(got.Point.Y, want.Point.Y) && near(got.Distance, want.Distance)
}

// randomLevel makes a map of w by h with about a quarter of its tiles walls.
func randomLevel(rng *rand.Rand, w, h int) *Map {
	m := New(w, h, Tile{})
	for y := range h {
		for x := range w {
			if rng.IntN(100) < 28 {
				m.Set(x, y, wall)
			}
		}
	}
	return m
}

// The answer is the one of the exact oracle for every pair of points in halves, in and around a
// small map, and for random pairs in sixteenths in bigger ones, with walls at random.
func TestSeesMatchesTheExactOracle(t *testing.T) {
	rng := rand.New(rand.NewPCG(41, 42))
	var seen, blocked int
	check := func(m *Map, a, b geometry.Vector2) {
		t.Helper()
		got, want := m.Sees(a, b), oracleSees(m, a, b)
		if got != want {
			t.Fatalf("Sees(%v, %v) was %v, wanted %v, in\n%s", a, b, got, want, show(m))
		}
		if m.Sees(b, a) != got {
			t.Fatalf("Sees(%v, %v) is not the same both ways", a, b)
		}
		if got {
			seen++
		} else {
			blocked++
		}
	}
	for range 2 {
		m := randomLevel(rng, 4, 3)
		var points []geometry.Vector2
		for x := -2; x <= 2*4+2; x++ {
			for y := -2; y <= 2*3+2; y++ {
				points = append(points, pt(float64(x)/2, float64(y)/2))
			}
		}
		for _, a := range points {
			for _, b := range points {
				check(m, a, b)
			}
		}
	}
	for range 3000 {
		w, h := 3+rng.IntN(8), 3+rng.IntN(6)
		m := randomLevel(rng, w, h)
		p := func() geometry.Vector2 {
			return pt(float64(rng.IntN((w+4)*16))/16-2, float64(rng.IntN((h+4)*16))/16-2)
		}
		check(m, p(), p())
	}
	if seen < 500 || blocked < 500 {
		t.Errorf("wanted both answers to be tried often, got %d seen and %d blocked", seen, blocked)
	}
}

// show draws a map as text, for a failing test.
func show(m *Map) string {
	var sb strings.Builder
	for y := range m.Height() {
		for x := range m.Width() {
			if m.Solid(x, y) {
				sb.WriteByte('#')
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// The same for Cast: the tile, the point and the distance of the oracle, for every start in
// halves in and around a small map and every direction of whole numbers up to 3, and for random
// ones in bigger maps, from sixteenths, with directions in sixteenths.
func TestCastMatchesTheExactOracle(t *testing.T) {
	rng := rand.New(rand.NewPCG(43, 44))
	var corners int
	check := func(m *Map, o, d geometry.Vector2) {
		t.Helper()
		got, ok := m.Cast(o, d)
		want, wantOK := oracleCast(m, o, d)
		if ok != wantOK || (ok && !sameHit(got, want)) {
			t.Fatalf("Cast(%v, %v) was %+v %v, wanted %+v %v, in\n%s", o, d, got, ok, want, wantOK, show(m))
		}
		if math.Mod(got.Point.X, 1) == 0 && math.Mod(got.Point.Y, 1) == 0 {
			corners++
		}
	}
	for range 1 {
		m := randomLevel(rng, 5, 4)
		for x := -1; x <= 2*5+1; x++ {
			for y := -1; y <= 2*4+1; y++ {
				for dx := -3; dx <= 3; dx++ {
					for dy := -3; dy <= 3; dy++ {
						if dx != 0 || dy != 0 {
							check(m, pt(float64(x)/2, float64(y)/2), pt(float64(dx), float64(dy)))
						}
					}
				}
			}
		}
	}
	for range 1000 {
		w, h := 3+rng.IntN(8), 3+rng.IntN(6)
		m := randomLevel(rng, w, h)
		o := pt(float64(rng.IntN((w+2)*16))/16-1, float64(rng.IntN((h+2)*16))/16-1)
		d := pt(float64(rng.IntN(129)-64)/16, float64(rng.IntN(129)-64)/16)
		if d == (geometry.Vector2{}) {
			continue
		}
		check(m, o, d)
	}
	if corners < 100 {
		t.Errorf("wanted rays to meet corners often, got %d", corners)
	}
}

// A ray through the corner of four tiles is in the tile that holds the corner, the one to the
// right of the line between columns and below the line between rows, and passes the others.
func TestARayThroughACornerIsStoppedOnlyByTheTileThatHoldsIt(t *testing.T) {
	tests := []struct {
		name string
		m    *Map
		a, b geometry.Vector2
		sees bool
	}{
		// Down and right: the corner (1, 1) is in the tile it goes to, so the two at the side
		// do not matter, and it passes between two walls that touch only there.
		{"down and right between two walls", level(".#", "#."), pt(0.5, 0.5), pt(1.5, 1.5), true},
		{"down and right into a wall", level("..", ".#"), pt(0.5, 0.5), pt(1.5, 1.5), false},
		{"down and right, the corner tile is floor", level(".#", "#."), pt(0.5, 0.5), pt(1.5, 1.5), true},
		// Up and right: the corner is in the tile below and to the right of it, at the side.
		{"up and right, the corner tile is a wall", level("..", ".#"), pt(0.5, 1.5), pt(1.5, 0.5), false},
		{"up and right, the others are walls", level("#.", ".."), pt(0.5, 1.5), pt(1.5, 0.5), true},
		{"up and right between two walls", level("#.", ".#"), pt(0.5, 1.5), pt(1.5, 0.5), false},
		// The same lines the other way round.
		{"down and right, reversed", level(".#", "#."), pt(1.5, 1.5), pt(0.5, 0.5), true},
		{"up and right, reversed", level("..", ".#"), pt(1.5, 0.5), pt(0.5, 1.5), false},
		// The far end is exactly on the corner: the tile that holds it counts, the others do not.
		{"to a corner held by a wall", level("..", ".#"), pt(0.5, 0.5), pt(1, 1), false},
		{"to a corner held by floor", level(".#", "#."), pt(0.5, 0.5), pt(1, 1), true},
		{"left and up to a corner, not held by the wall behind it", level(".#", "#."), pt(1.5, 1.5), pt(1, 1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.Sees(tt.a, tt.b); got != tt.sees {
				t.Errorf("Sees(%v, %v) was %v, wanted %v, in\n%s", tt.a, tt.b, got, tt.sees, show(tt.m))
			}
			if got := tt.m.Sees(tt.b, tt.a); got != tt.sees {
				t.Errorf("Sees(%v, %v) was %v, wanted %v", tt.b, tt.a, got, tt.sees)
			}
			if want := oracleSees(tt.m, tt.a, tt.b); want != tt.sees {
				t.Errorf("wanted the oracle to say %v for the test's own answer, got %v", tt.sees, want)
			}
		})
	}
}

// A point at whole numbers is the top left corner of its tile and is in that tile alone, as a
// sprite at (4, 3) stands on tile (4, 3), so sight from tile-aligned positions next to a wall
// works, and a point on a wall's far face is not in the wall.
func TestPointsAtWholeNumbersAreInTheirOwnTile(t *testing.T) {
	m := level(
		"#....",
		"#....",
		"#....",
	)
	if !m.Sees(pt(1, 1), pt(4, 1)) {
		t.Errorf("wanted the line along row 1 from the tile next to the wall to be clear")
	}
	if !m.Sees(pt(1, 0), pt(1, 2)) {
		t.Errorf("wanted the line down the far face of the wall, which is in column 1, to be clear")
	}
	if m.Sees(pt(0, 1), pt(4, 1)) {
		t.Errorf("wanted a point at (0, 1), in the wall, not to see")
	}
	if !m.Sees(pt(3, 2), pt(3, 2)) || m.Sees(pt(0, 2), pt(0, 2)) {
		t.Errorf("wanted a point alone to be seen unless it is in a wall")
	}
	// The near face of a wall is in the wall: a line down it is stopped.
	right := level("..#..", "..#..", "..#..")
	if right.Sees(pt(2, 0), pt(2, 2)) {
		t.Errorf("wanted the line down the left face of a wall, which is in the wall's column, to be stopped")
	}
	if !right.Sees(pt(3, 0), pt(3, 2)) {
		t.Errorf("wanted the line down the right face, which is in the next column, to be clear")
	}
	// And along the top and the bottom of a row of wall.
	flat := level(".....", "#####", ".....")
	if flat.Sees(pt(0, 1), pt(4.5, 1)) {
		t.Errorf("wanted the line along the top face of a row of wall, which is in the row, to be stopped")
	}
	if !flat.Sees(pt(0, 2), pt(4.5, 2)) {
		t.Errorf("wanted the line along its bottom face, which is in the next row, to be clear")
	}
}

// A wall that the line only clips at a tip, for a length that a step of a tenth of a tile
// along the line steps over, is not missed, which is what the issue says a stepping loop can do.
func TestTheTipOfAWallIsNotMissed(t *testing.T) {
	// The line is in the row of the wall (3, 1) only from x of 3.99 to 4.
	slope := 1 / 3.99
	a, b := pt(0, 0), pt(9.9, 9.9*slope)
	clipped := level("..........", "...#......", "..........", "..........")

	// The stepping loop of the issue, a tenth of the line at a time, never lands in the wall.
	length := math.Hypot(b.X-a.X, b.Y-a.Y)
	for n := 0.0; n <= length; n += 0.1 {
		x, y := a.X+(b.X-a.X)*n/length, a.Y+(b.Y-a.Y)*n/length
		if clipped.Solid(int(math.Floor(x)), int(math.Floor(y))) {
			t.Fatalf("wanted the stepping loop to miss the wall, which this test is about, but it landed in it at %v, %v", x, y)
		}
	}

	if clipped.Sees(a, b) {
		t.Errorf("wanted the wall clipped at its corner to stop the line")
	}
	h, ok := clipped.Cast(a, pt(1, slope))
	if !ok || h.X != 3 || h.Y != 1 || h.Point.Y != 1 || !near(h.Point.X, 3.99) {
		t.Errorf("wanted the hit on tile (3, 1) at (3.99, 1), got %+v %v", h, ok)
	}
	// With the wall a row lower the line is clear.
	lower := level("..........", "..........", "...#......", "..........")
	if !lower.Sees(a, b) {
		t.Errorf("wanted the line to be clear when the wall is a row lower")
	}
}

func TestCastAlongTheAxesHitsOnTheEdgeItCrosses(t *testing.T) {
	m := level(
		"#####",
		"#...#",
		"#####",
	)
	from := pt(2.5, 1.5)
	tests := []struct {
		name string
		d    geometry.Vector2
		want Hit
	}{
		{"right", pt(1, 0), Hit{Point: pt(4, 1.5), X: 4, Y: 1, Distance: 1.5}},
		{"left", pt(-1, 0), Hit{Point: pt(1, 1.5), X: 0, Y: 1, Distance: 1.5}},
		{"down", pt(0, 1), Hit{Point: pt(2.5, 2), X: 2, Y: 2, Distance: 0.5}},
		{"up", pt(0, -1), Hit{Point: pt(2.5, 1), X: 2, Y: 0, Distance: 0.5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := m.Cast(from, tt.d)
			if !ok || got != tt.want {
				t.Errorf("wanted %+v, got %+v %v", tt.want, got, ok)
			}
		})
	}
}

// For any numbers, not only ones a computer holds exactly, the coordinate of the edge a ray enters
// a tile by is exact: a whole number, the tile's own edge, and not the rounded result of a sum.
func TestTheHitPointIsExactlyOnTheEdgeTheRayCrosses(t *testing.T) {
	rng := rand.New(rand.NewPCG(49, 50))
	m := randomLevel(rng, 12, 9)
	for range 3000 {
		o := pt(rng.Float64()*12, rng.Float64()*9)
		d := pt(rng.NormFloat64(), rng.NormFloat64())
		h, ok := m.Cast(o, d)
		if !ok {
			t.Fatalf("wanted a hit from %v along %v", o, d)
		}
		if h.Distance == 0 {
			continue // it started in a wall
		}
		onX := h.Point.X == float64(h.X) || h.Point.X == float64(h.X+1)
		onY := h.Point.Y == float64(h.Y) || h.Point.Y == float64(h.Y+1)
		if !onX && !onY {
			t.Fatalf("Cast(%v, %v) hit %+v, wanted a coordinate to be exactly on the tile's edge", o, d, h)
		}
	}
}

// A ray through a corner enters the tile at the corner, exactly, for starts a computer does not
// hold exactly too, since the corner's coordinates are whole numbers.
func TestARayThatMeetsACornerHitsItExactly(t *testing.T) {
	m := level(".....", ".....", ".....", "...#.", ".....")
	for _, v := range []float64{0.1, 0.2, 0.3, 0.7, 1.1, 0.123456789} {
		for _, k := range []float64{1, 3, 0.1} {
			h, ok := m.Cast(pt(v, v), pt(k, k))
			if !ok || h.X != 3 || h.Y != 3 || h.Point != pt(3, 3) {
				t.Errorf("from (%v, %v) along (%v, %v): wanted the corner (3, 3) of tile (3, 3), got %+v %v", v, v, k, k, h, ok)
			}
		}
	}
}

func TestCastThroughTheMapReachesTheEdgeOfTheWorld(t *testing.T) {
	m := New(4, 3, Tile{})
	for _, tt := range []struct {
		d    geometry.Vector2
		want Hit
	}{
		{pt(1, 0), Hit{Point: pt(4, 1.5), X: 4, Y: 1, Distance: 2.5}},
		{pt(-1, 0), Hit{Point: pt(0, 1.5), X: -1, Y: 1, Distance: 1.5}},
		{pt(0, -1), Hit{Point: pt(1.5, 0), X: 1, Y: -1, Distance: 1.5}},
		{pt(0, 1), Hit{Point: pt(1.5, 3), X: 1, Y: 3, Distance: 1.5}},
	} {
		got, ok := m.Cast(pt(1.5, 1.5), tt.d)
		if !ok || got != tt.want {
			t.Errorf("direction %v: wanted the edge of the map, %+v, got %+v %v", tt.d, tt.want, got, ok)
		}
	}
}

func TestTheLengthOfTheDirectionDoesNotMatter(t *testing.T) {
	m := level(
		".........",
		"....#....",
		".........",
	)
	from := pt(0.25, 1.5)
	want, ok := m.Cast(from, pt(1, 0))
	if !ok || want.X != 4 {
		t.Fatalf("wanted a hit on the wall, got %+v %v", want, ok)
	}
	for _, d := range []geometry.Vector2{pt(2, 0), pt(0.001, 0), pt(1e300, 0), pt(1e-300, 0), pt(5e-324, 0), pt(math.MaxFloat64, 0)} {
		if got, ok := m.Cast(from, d); !ok || got != want {
			t.Errorf("direction %v: wanted %+v, got %+v %v", d, want, got, ok)
		}
	}
	slanted, _ := m.Cast(pt(0.5, 0.5), pt(3, 1))
	for _, k := range []float64{2, 0.5, 1e200, 1e-200} {
		if got, ok := m.Cast(pt(0.5, 0.5), pt(3*k, 1*k)); !ok || !sameHit(got, slanted) {
			t.Errorf("direction (3, 1) times %v: wanted %+v, got %+v %v", k, slanted, got, ok)
		}
	}
}

func TestACastFromInsideAWallOrOutsideTheMapHitsAtOnce(t *testing.T) {
	m := level(
		"...",
		".#.",
		"...",
	)
	for _, tt := range []struct {
		name string
		from geometry.Vector2
		want Hit
	}{
		{"in a wall", pt(1.25, 1.75), Hit{Point: pt(1.25, 1.75), X: 1, Y: 1}},
		{"outside", pt(-2.5, 1.5), Hit{Point: pt(-2.5, 1.5), X: -3, Y: 1}},
		{"far outside", pt(1e300, -1e300), Hit{Point: pt(1e300, -1e300), X: farCell, Y: -farCell}},
		{"just outside", pt(3, 1), Hit{Point: pt(3, 1), X: 3, Y: 1}},
	} {
		got, ok := m.Cast(tt.from, pt(1, 0.5))
		if !ok || got != tt.want {
			t.Errorf("%s: wanted %+v, got %+v %v", tt.name, tt.want, got, ok)
		}
		if m.Sees(tt.from, pt(0.5, 0.5)) {
			t.Errorf("%s: wanted a point that is solid not to see", tt.name)
		}
	}
}

func TestANilOrEmptyMapIsSolidEverywhere(t *testing.T) {
	for name, m := range map[string]*Map{"nil": nil, "empty": New(0, 0, Tile{})} {
		if m.Sees(pt(0.5, 0.5), pt(0.5, 0.5)) || m.Sees(pt(0, 0), pt(1, 1)) {
			t.Errorf("%s: wanted nothing to be seen", name)
		}
		got, ok := m.Cast(pt(2.5, 3.5), pt(1, 1))
		if !ok || got != (Hit{Point: pt(2.5, 3.5), X: 2, Y: 3}) {
			t.Errorf("%s: wanted a hit at the start, got %+v %v", name, got, ok)
		}
	}
}

func TestNaNInfinityAndNoDirectionHaveNoHit(t *testing.T) {
	m := level("...", "...", "...")
	bad := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, v := range bad {
		for _, p := range []geometry.Vector2{pt(v, 1), pt(1, v)} {
			if m.Sees(p, pt(1, 1)) || m.Sees(pt(1, 1), p) {
				t.Errorf("wanted %v not to be seen", p)
			}
			if _, ok := m.Cast(p, pt(1, 0)); ok {
				t.Errorf("wanted no hit from %v", p)
			}
			if _, ok := m.Cast(pt(1, 1), p); ok {
				t.Errorf("wanted no hit in the direction %v", p)
			}
		}
	}
	if _, ok := m.Cast(pt(1, 1), pt(0, 0)); ok {
		t.Errorf("wanted no hit in no direction")
	}
	if _, ok := m.Cast(pt(1, 1), pt(math.Copysign(0, -1), 0)); ok {
		t.Errorf("wanted no hit in a direction of negative zero")
	}
	// Far apart enough that the line between overflows: both are outside.
	if m.Sees(pt(-math.MaxFloat64, 0), pt(math.MaxFloat64, 0)) {
		t.Errorf("wanted points far outside not to see")
	}
	// Close enough that a division overflows: the line is a point.
	if !m.Sees(pt(1.5, 1.5), pt(1.5+1e-320, 1.5)) {
		t.Errorf("wanted a line shorter than anything to be clear inside a tile")
	}
}

func TestSeesAndCastDoNotAllocate(t *testing.T) {
	m := level(
		"..........",
		"....#.....",
		"..........",
	)
	if n := testing.AllocsPerRun(100, func() {
		m.Sees(pt(0.5, 0.5), pt(9.5, 1.5))
		m.Sees(pt(0.5, 0.5), pt(9.5, 0.5))
		m.Cast(pt(0.5, 1.5), pt(1, 0))
		m.Cast(pt(5.5, 1.5), pt(-3, 1))
	}); n != 0 {
		t.Errorf("wanted no allocation, got %v", n)
	}
}

// Any start, end and direction: no panic, and what comes back is consistent.
func FuzzSeesAndCast(f *testing.F) {
	f.Add(0.5, 0.5, 3.5, 1.5, 1.0, 0.5)
	f.Add(1.0, 1.0, 1.0, 1.0, 0.0, 1.0)
	f.Add(-1.0, 2.0, 4.0, -1.0, -1.0, -1.0)
	f.Add(0.0, 0.0, 5.0, 5.0, 1.0, 1.0)
	f.Add(1e300, -1e300, 1e-300, 5e-324, 1e300, 1e-300)
	f.Add(math.NaN(), 0.0, math.Inf(1), 0.0, 0.0, 0.0)
	f.Add(2.0, 2.0, 2.0000000000000004, 2.0, 5e-324, 0.0)

	m := level(
		"#######",
		"#..#..#",
		"#.....#",
		"#..#..#",
		"#######",
	)
	f.Fuzz(func(t *testing.T, ax, ay, bx, by, dx, dy float64) {
		a, b, d := pt(ax, ay), pt(bx, by), pt(dx, dy)
		sees := m.Sees(a, b)
		if a == b && !math.IsNaN(ax) && !math.IsNaN(ay) && !math.IsInf(ax, 0) && !math.IsInf(ay, 0) {
			if want := !m.Solid(cellOf(ax), cellOf(ay)); sees != want {
				t.Fatalf("Sees(%v, %v) was %v, wanted %v", a, a, sees, want)
			}
		}
		h, ok := m.Cast(a, d)
		if !ok {
			return
		}
		if !m.Solid(h.X, h.Y) {
			t.Fatalf("Cast(%v, %v) hit %+v, which is not solid", a, d, h)
		}
		if h.Distance < 0 || math.IsNaN(h.Distance) || math.IsInf(h.Distance, 0) {
			t.Fatalf("Cast(%v, %v) went %v", a, d, h.Distance)
		}
		if math.IsNaN(h.Point.X) || math.IsNaN(h.Point.Y) {
			t.Fatalf("Cast(%v, %v) hit at %v", a, d, h.Point)
		}
		// Inside the map, the point is on the tile's edge, or in it.
		if a.X >= 0 && a.X < 7 && a.Y >= 0 && a.Y < 5 {
			if h.Point.X < float64(h.X)-1e-6 || h.Point.X > float64(h.X)+1+1e-6 || h.Point.Y < float64(h.Y)-1e-6 || h.Point.Y > float64(h.Y)+1+1e-6 {
				t.Fatalf("Cast(%v, %v) hit %+v, whose point is not in the tile", a, d, h)
			}
		}
	})
}

func BenchmarkSeesAndCast(b *testing.B) {
	m := New(400, 200, Tile{})
	rng := rand.New(rand.NewPCG(47, 48))
	for range 1500 {
		m.Set(rng.IntN(400), rng.IntN(200), wall)
	}
	open := New(400, 200, Tile{})
	b.Run("Sees across an open map", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			open.Sees(pt(0.5, 0.5), pt(399.5, 199.5))
		}
	})
	b.Run("Sees, a short line", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			open.Sees(pt(100.5, 100.5), pt(108.5, 103.5))
		}
	})
	b.Run("Sees, stopped early by a wall", func(b *testing.B) {
		b.ReportAllocs()
		w := New(400, 200, Tile{})
		w.Set(50, 25, wall)
		for b.Loop() {
			w.Sees(pt(0.5, 0.25), pt(399.5, 199.5))
		}
	})
	b.Run("Cast across an open map", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			open.Cast(pt(0.5, 0.5), pt(2, 1))
		}
	})
	b.Run("Cast through a scattering of walls", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			m.Cast(pt(200.5, 100.5), pt(1.3, 0.7))
		}
	})
}

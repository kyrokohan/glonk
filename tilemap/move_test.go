package tilemap

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"glonk.dev/geometry"
)

// moveRoom is 10 by 5: a wall all round, one in the middle of row 2, and a gap in the floor.
//
//	##########
//	#........#
//	#..#.....#
//	#........#
//	####.#####
func moveRoom() *Map {
	return FromLines(legend(),
		"##########",
		"#........#",
		"#..#.....#",
		"#........#",
		"####.#####",
	)
}

func box(x, y, w, h float64) geometry.Rect { return geometry.NewRect(x, y, w, h) }

func TestMoveStopsAtSolidTilesAndSlides(t *testing.T) {
	inf, nan := math.Inf(1), math.NaN()
	tests := []struct {
		name       string
		box        geometry.Rect
		delta      geometry.Vector2
		want       geometry.Rect
		hitX, hitY bool
	}{
		{"free, then stopped below by the wall in the middle", box(1, 1, 1, 1), geometry.Vector2{X: 2, Y: 1}, box(3, 1, 1, 1), false, true},
		{"into a wall sideways, slides on", box(7, 1, 1, 1), geometry.Vector2{X: 5, Y: 1}, box(8, 2, 1, 1), true, false},
		{"into a corner", box(8, 3, 1, 1), geometry.Vector2{X: 5, Y: 5}, box(8, 3, 1, 1), true, true},
		{"down through the gap and stopped by the edge of the world", box(4, 3, 1, 1), geometry.Vector2{Y: 3}, box(4, 4, 1, 1), false, true},
		{"too wide for the gap", box(3.5, 3, 2, 1), geometry.Vector2{Y: 3}, box(3.5, 3, 2, 1), false, true},
		{"touching a wall and moving along it", box(1, 1, 1, 1), geometry.Vector2{X: -5, Y: 2}, box(1, 3, 1, 1), true, false},
		{"arriving exactly at a wall is not being stopped", box(1, 1, 1, 1), geometry.Vector2{X: 7}, box(8, 1, 1, 1), false, false},
		{"the next move is", box(8, 1, 1, 1), geometry.Vector2{X: 1}, box(8, 1, 1, 1), true, false},
		{"a thin wall at speed", box(1, 2, 1, 1), geometry.Vector2{X: 50}, box(2, 2, 1, 1), true, false},
		{"a fraction of a cell, into the corner", box(1.25, 1.25, 0.5, 0.5), geometry.Vector2{X: 10, Y: 10}, box(8.5, 3.5, 0.5, 0.5), true, true},
		{"standing on the floor and running along it", box(2, 3, 1, 1), geometry.Vector2{X: 5}, box(7, 3, 1, 1), false, false},
		{"standing on the floor and pressing into it", box(7, 3, 1, 1), geometry.Vector2{Y: 1}, box(7, 3, 1, 1), false, true},
		{"a huge move stops at the wall in bounded time", box(1, 1, 1, 1), geometry.Vector2{X: 1e300}, box(8, 1, 1, 1), true, false},
		{"a huge move the other way", box(8, 3, 1, 1), geometry.Vector2{X: -1e300, Y: -1e300}, box(1, 1, 1, 1), true, true},
		{"no move", box(2, 2, 1, 1), geometry.Vector2{}, box(2, 2, 1, 1), false, false},
		{"up to the roof", box(2, 3, 1, 1), geometry.Vector2{Y: -9}, box(2, 1, 1, 1), false, true},
		{"left to the wall at the edge", box(5, 3, 2, 1), geometry.Vector2{X: -9}, box(1, 3, 2, 1), true, false},
		{"a box as tall as the room's rows stops at the roof and the wall", box(2, 1, 1, 3), geometry.Vector2{X: 9, Y: -3}, box(2, 1, 1, 3), true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, hitX, hitY := moveRoom().Move(tt.box, tt.delta)
			if got != tt.want || hitX != tt.hitX || hitY != tt.hitY {
				t.Errorf("%v by %v: wanted %v (%v, %v), got %v (%v, %v)", tt.box, tt.delta, tt.want, tt.hitX, tt.hitY, got, hitX, hitY)
			}
		})
	}

	// Numbers that cannot be moved by or moved.
	for name, c := range map[string]struct {
		box   geometry.Rect
		delta geometry.Vector2
	}{
		"a NaN delta":      {box(2, 2, 1, 1), geometry.Vector2{X: nan}},
		"a NaN delta in Y": {box(2, 2, 1, 1), geometry.Vector2{Y: nan}},
		"an infinite X":    {box(2, 2, 1, 1), geometry.Vector2{X: inf}},
		"an infinite Y":    {box(2, 2, 1, 1), geometry.Vector2{X: 1, Y: -inf}},
		"a NaN box":        {geometry.Rect{Min: geometry.Vector2{X: nan, Y: 2}, Max: geometry.Vector2{X: 3, Y: 3}}, geometry.Vector2{X: 1}},
		"an infinite box":  {geometry.Rect{Min: geometry.Vector2{X: -inf, Y: 2}, Max: geometry.Vector2{X: inf, Y: 3}}, geometry.Vector2{X: 1}},
		"an empty box":     {box(2, 2, 0, 1), geometry.Vector2{X: 1}},
		"the zero box":     {geometry.Rect{}, geometry.Vector2{X: 1, Y: 1}},
	} {
		got, hx, hy := moveRoom().Move(c.box, c.delta)
		if hx || hy || !sameRect(got, c.box) {
			t.Errorf("%s: wanted the box as it was and not stopped, got %v (%v, %v)", name, got, hx, hy)
		}
	}
}

func sameRect(a, b geometry.Rect) bool {
	same := func(x, y float64) bool { return math.Float64bits(x) == math.Float64bits(y) }
	return same(a.Min.X, b.Min.X) && same(a.Min.Y, b.Min.Y) && same(a.Max.X, b.Max.X) && same(a.Max.Y, b.Max.Y)
}

func TestMoveFromInsideASolidTileIsNotStoppedByIt(t *testing.T) {
	room := moveRoom()
	// (3, 2) is the wall in the middle of the room. A box on it can move on, and is stopped only
	// by what it moves into.
	for _, c := range []struct {
		name  string
		delta geometry.Vector2
		want  geometry.Rect
		hitX  bool
	}{
		{"right", geometry.Vector2{X: 1}, box(4, 2, 1, 1), false},
		{"left", geometry.Vector2{X: -1}, box(2, 2, 1, 1), false},
		{"down", geometry.Vector2{Y: 1}, box(3, 3, 1, 1), false},
		{"up", geometry.Vector2{Y: -1}, box(3, 1, 1, 1), false},
		{"right into the next wall", geometry.Vector2{X: 9}, box(8, 2, 1, 1), true},
	} {
		got, hx, _ := room.Move(box(3, 2, 1, 1), c.delta)
		if got != c.want || hx != c.hitX {
			t.Errorf("%s: wanted %v (%v), got %v (%v)", c.name, c.want, c.hitX, got, hx)
		}
	}
	// A box half in the wall and half out is stopped by walls ahead, not by the one it is in.
	got, hx, _ := room.Move(box(2.5, 2, 1, 1), geometry.Vector2{X: 3})
	if got != box(5.5, 2, 1, 1) || hx {
		t.Errorf("wanted a box half in the wall to move on, got %v (%v)", got, hx)
	}
}

func TestMoveFromOutsideTheMapCannotMove(t *testing.T) {
	room := moveRoom()
	for _, b := range []geometry.Rect{box(20, 1, 1, 1), box(-5, 1, 1, 1), box(2, 30, 1, 1), box(2, -30, 1, 1), box(1e9, 1e9, 1, 1)} {
		for _, d := range []geometry.Vector2{{X: 3}, {X: -3}, {Y: 3}, {Y: -3}} {
			got, _, _ := room.Move(b, d)
			if got != b {
				t.Errorf("%v by %v: wanted a box outside the map to stay, got %v", b, d, got)
			}
		}
	}
	// Reaching past the edge in the other direction is the same: it is at the wall.
	if got, hx, _ := room.Move(box(2, -0.5, 1, 1), geometry.Vector2{X: 1}); got != box(2, -0.5, 1, 1) || !hx {
		t.Errorf("wanted a box overlapping the outside to be stopped, got %v (%v)", got, hx)
	}
	// A box partly outside along its own path is fine, if the rows it covers are inside.
	if got, hx, _ := room.Move(box(-0.5, 1, 1, 1), geometry.Vector2{X: 1}); got != box(0.5, 1, 1, 1) || hx {
		t.Errorf("wanted a box half outside to move along inside rows, got %v (%v)", got, hx)
	}
	if got, hx, hy := (*Map)(nil).Move(box(0, 0, 1, 1), geometry.Vector2{X: 1, Y: 1}); got != box(0, 0, 1, 1) || !hx || !hy {
		t.Errorf("wanted a nil map to be a wall on every side, got %v (%v, %v)", got, hx, hy)
	}
}

// boxHitsSolid is whether r overlaps any solid cell, found by asking every cell it could touch,
// the world outside the map included.
func boxHitsSolid(m *Map, r geometry.Rect) bool {
	for cy := int(math.Floor(r.Min.Y)); cy < int(math.Ceil(r.Max.Y)); cy++ {
		for cx := int(math.Floor(r.Min.X)); cx < int(math.Ceil(r.Max.X)); cx++ {
			if m.Solid(cx, cy) && r.Overlaps(geometry.NewRect(float64(cx), float64(cy), 1, 1)) {
				return true
			}
		}
	}
	return false
}

// referenceMove moves the box a sixty-fourth of a cell at a time, along X and then along Y,
// stopping at the step that would overlap a solid cell. It needs a box that starts clear and
// numbers in sixty-fourths, so that every step and edge is exact, and it shares nothing with
// Move's way of finding the edge.
func referenceMove(m *Map, b geometry.Rect, delta geometry.Vector2) (geometry.Rect, bool, bool) {
	const step = 1.0 / 64
	run := func(d float64, along geometry.Vector2) (hit bool) {
		for n := math.Abs(d) / step; n > 0; n-- {
			next := b.Translate(along.Scale(math.Copysign(step, d)))
			if boxHitsSolid(m, next) {
				return true
			}
			b = next
		}
		return false
	}
	hitX := run(delta.X, geometry.Vector2{X: 1})
	hitY := run(delta.Y, geometry.Vector2{Y: 1})
	return b, hitX, hitY
}

func sixtyFourths(rng *rand.Rand, lo, hi int) float64 { return float64(lo+rng.IntN(hi-lo)) / 64 }

func TestMoveMatchesSteppingASixtyFourthAtATime(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 32))
	var moved, stoppedX, stoppedY, slid, both, clear int
	for range 4000 {
		mw, mh := 3+rng.IntN(10), 3+rng.IntN(6)
		m := New(mw, mh, Tile{})
		for y := range mh {
			for x := range mw {
				if rng.IntN(100) < 25 {
					m.Set(x, y, wall)
				}
			}
		}
		// A box that starts inside the map and clear of the walls, whose edges are sixty-fourths.
		var b geometry.Rect
		var found bool
		for range 40 {
			b = box(sixtyFourths(rng, 0, mw*64), sixtyFourths(rng, 0, mh*64), sixtyFourths(rng, 4, 3*64), sixtyFourths(rng, 4, 3*64))
			if !boxHitsSolid(m, b) {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		clear++
		var d geometry.Vector2
		if rng.IntN(4) > 0 {
			d.X = sixtyFourths(rng, -12*64, 12*64)
		}
		if rng.IntN(4) > 0 {
			d.Y = sixtyFourths(rng, -8*64, 8*64)
		}

		want, wantX, wantY := referenceMove(m, b, d)
		got, hitX, hitY := m.Move(b, d)
		if got != want || hitX != wantX || hitY != wantY {
			t.Fatalf("map %dx%d, box %v by %v: stepping says %v (%v, %v), got %v (%v, %v)", mw, mh, b, d, want, wantX, wantY, got, hitX, hitY)
		}
		if got != b {
			moved++
		}
		if hitX {
			stoppedX++
		}
		if hitY {
			stoppedY++
		}
		if hitX && !hitY && got.Min.Y != b.Min.Y {
			slid++ // stopped sideways and moved on along Y
		}
		if hitX && hitY {
			both++
		}
	}
	// Guard: plenty of moves, of each kind of stop, and of sliding.
	if clear < 3000 || moved < 2000 || stoppedX < 600 || stoppedY < 600 || slid < 200 || both < 100 {
		t.Errorf("wanted at least 3000 boxes, 2000 moves, 600 stops on each axis, 200 slides and 100 corners, got %d, %d, %d, %d, %d and %d", clear, moved, stoppedX, stoppedY, slid, both)
	}
}

// A move far longer than the map ends where one that just reaches the far side does, because
// the edge of the world is a wall.
func TestMoveOfAnyLengthEndsWhereOneToTheEdgeDoes(t *testing.T) {
	rng := rand.New(rand.NewPCG(41, 42))
	for range 2000 {
		mw, mh := 3+rng.IntN(10), 3+rng.IntN(6)
		m := New(mw, mh, Tile{})
		for y := range mh {
			for x := range mw {
				if rng.IntN(100) < 25 {
					m.Set(x, y, wall)
				}
			}
		}
		b := box(sixtyFourths(rng, 0, mw*64), sixtyFourths(rng, 0, mh*64), sixtyFourths(rng, 4, 128), sixtyFourths(rng, 4, 128))
		if boxHitsSolid(m, b) {
			continue
		}
		reach := float64(mw + mh + 8)
		for _, sign := range [][2]float64{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
			far := geometry.Vector2{X: sign[0] * 1e9, Y: sign[1] * 1e9}
			near := geometry.Vector2{X: sign[0] * reach, Y: sign[1] * reach}
			g1, x1, y1 := m.Move(b, far)
			g2, x2, y2 := m.Move(b, near)
			if g1 != g2 || x1 != x2 || y1 != y2 {
				t.Fatalf("box %v by %v: wanted the same end as a move of %v, got %v and %v", b, far, near, g1, g2)
			}
		}
	}
}

func TestMoveDoesNotAllocateAndDoesBoundedWork(t *testing.T) {
	m := New(200, 60, Tile{})
	for x := range 200 {
		m.Set(x, 0, wall)
		m.Set(x, 59, wall)
	}
	b := box(10, 10, 2, 2)
	if n := testing.AllocsPerRun(100, func() { m.Move(b, geometry.Vector2{X: 3.5, Y: -2.5}) }); n != 0 {
		t.Errorf("wanted no allocation, got %v", n)
	}

	huge := New(2048, 2048, Tile{})
	start := time.Now()
	for range 100 {
		huge.Move(box(1, 1, 1, 1), geometry.Vector2{X: 1e300, Y: 1e300})
		huge.Move(box(1e300, 1, 1, 1), geometry.Vector2{X: -1e300})
		huge.Move(box(-1e300, -1e300, 1e300, 1), geometry.Vector2{X: 1e300, Y: 5})
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("wanted huge moves to be bounded by the map, 300 took %v", took)
	}
}

func FuzzMapMove(f *testing.F) {
	inf, nan := math.Inf(1), math.NaN()
	f.Add(uint64(0x00ff00ff), uint8(8), uint8(6), 1.0, 1.0, 1.0, 1.0, 3.5, 2.5)
	f.Add(uint64(0), uint8(5), uint8(5), 4.5, 4.5, 2.0, 2.0, 10.0, 10.0)
	f.Add(uint64(0xffffffff), uint8(8), uint8(6), 1.0, 1.0, 1.0, 1.0, -3.0, 4.0)
	f.Add(uint64(1), uint8(4), uint8(4), 1.0, 1.0, 1.0, 1.0, nan, 0.0)
	f.Add(uint64(1), uint8(4), uint8(4), 1.0, 1.0, 1.0, 1.0, inf, -inf)
	f.Add(uint64(1), uint8(4), uint8(4), nan, 1.0, 1.0, 1.0, 1.0, 1.0)
	f.Add(uint64(2), uint8(4), uint8(4), 1e300, -1e300, 1e299, 1.0, -1e300, 1e300)
	f.Add(uint64(3), uint8(16), uint8(16), -2.5, -2.5, 3.0, 40.0, 100.0, -100.0)
	f.Add(uint64(0xdeadbeef), uint8(6), uint8(6), 2.5, 2.5, 0.5, 0.5, 0.0, 0.0)

	f.Fuzz(func(t *testing.T, bits uint64, mw, mh uint8, x, y, w, h, dx, dy float64) {
		m := New(int(mw%17), int(mh%17), Tile{})
		for cy := range m.Height() {
			for cx := range m.Width() {
				if bits>>(uint(cx*3+cy*5)%64)&1 == 1 {
					m.Set(cx, cy, wall)
				}
			}
		}
		b := geometry.Rect{Min: geometry.Vector2{X: x, Y: y}, Max: geometry.Vector2{X: x + w, Y: y + h}}
		d := geometry.Vector2{X: dx, Y: dy}

		done := make(chan struct{})
		var got geometry.Rect
		var hx, hy bool
		go func() {
			defer close(done)
			got, hx, hy = m.Move(b, d)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatalf("wanted Move to finish for %v by %v", b, d)
		}

		again, ax, ay := m.Move(b, d)
		if !sameRect(again, got) || ax != hx || ay != hy {
			t.Fatalf("wanted the same result twice")
		}
		if !finite(b.Min.X, b.Min.Y, b.Max.X, b.Max.Y, dx, dy) || b.Empty() {
			if !sameRect(got, b) || hx || hy {
				t.Fatalf("wanted %v as it was, got %v (%v, %v)", b, got, hx, hy)
			}
			return
		}
		scale := max(1, math.Abs(b.Min.X), math.Abs(b.Max.X), math.Abs(b.Min.Y), math.Abs(b.Max.Y), math.Abs(dx), math.Abs(dy))
		if scale > 1e6 {
			return // sums of numbers this large lose the fractions the checks below look at
		}
		tol := 1e-9 * scale

		// The same size, and never farther along an axis than it was told, nor the other way.
		if math.Abs(got.Width()-b.Width()) > tol || math.Abs(got.Height()-b.Height()) > tol {
			t.Fatalf("wanted the size of %v kept, got %v", b, got)
		}
		for _, a := range [][3]float64{{got.Min.X - b.Min.X, dx, 0}, {got.Min.Y - b.Min.Y, dy, 1}} {
			moved, asked := a[0], a[1]
			if math.Abs(moved) > math.Abs(asked)+tol || (asked >= 0 && moved < -tol) || (asked <= 0 && moved > tol) {
				t.Fatalf("box %v by %v: moved %v along an axis it was asked to move %v", b, d, moved, asked)
			}
		}
		// A box that starts clear of the walls ends clear of them, whatever it runs into.
		if !boxHitsSolid(m, b) && boxHitsSolid(m, got) && !touchingOnly(m, got) {
			t.Fatalf("box %v by %v ended in a wall at %v", b, d, got)
		}
	})
}

// touchingOnly reports whether r overlaps a solid cell by no more than rounding.
func touchingOnly(m *Map, r geometry.Rect) bool {
	for cy := int(math.Floor(r.Min.Y)); cy < int(math.Ceil(r.Max.Y)); cy++ {
		for cx := int(math.Floor(r.Min.X)); cx < int(math.Ceil(r.Max.X)); cx++ {
			if m.Solid(cx, cy) {
				i := r.Intersect(geometry.NewRect(float64(cx), float64(cy), 1, 1))
				if min(i.Width(), i.Height()) > 1e-9 {
					return false
				}
			}
		}
	}
	return true
}

func BenchmarkMapMove(b *testing.B) {
	rng := rand.New(rand.NewPCG(5, 6))
	m := New(200, 60, Tile{})
	for range 1500 {
		m.Set(rng.IntN(200), rng.IntN(60), wall)
	}
	start := box(100.5, 30.5, 2, 2)
	b.Run("a box of 2 by 2 moving 3.5 and 2.5", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			m.Move(start, geometry.Vector2{X: 3.5, Y: 2.5})
		}
	})
}

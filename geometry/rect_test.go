package geometry

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestNewRectWidthHeightAndSize(t *testing.T) {
	r := NewRect(2, 3, 5, 7)
	if r.Min != (Vector2{X: 2, Y: 3}) || r.Max != (Vector2{X: 7, Y: 10}) {
		t.Errorf("wanted (2, 3) to (7, 10), got %v", r)
	}
	if r.Width() != 5 || r.Height() != 7 || r.Size() != (Vector2{X: 5, Y: 7}) {
		t.Errorf("wanted 5 by 7, got %v by %v (%v)", r.Width(), r.Height(), r.Size())
	}
	if r.Empty() {
		t.Errorf("wanted a rectangle with area not to be empty")
	}
}

func TestEmpty(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	tests := []struct {
		name string
		r    Rect
		want bool
	}{
		{"the zero Rect", Rect{}, true},
		{"unit", NewRect(0, 0, 1, 1), false},
		{"no width", NewRect(0, 0, 0, 5), true},
		{"no height", NewRect(0, 0, 5, 0), true},
		{"negative width", NewRect(5, 0, -2, 5), true},
		{"negative height", NewRect(0, 5, 5, -2), true},
		{"inside out on both axes", Rect{Min: Vector2{X: 3, Y: 3}, Max: Vector2{X: 1, Y: 1}}, true},
		{"a NaN corner", Rect{Min: Vector2{X: nan}, Max: Vector2{X: 1, Y: 1}}, true},
		{"a NaN size", NewRect(0, 0, nan, 1), true},
		{"infinite", Rect{Min: Vector2{X: -inf, Y: -inf}, Max: Vector2{X: inf, Y: inf}}, false},
		{"a huge one", NewRect(-1e300, -1e300, 2e300, 2e300), false},
		{"a tiny one", NewRect(0, 0, 5e-324, 5e-324), false},
	}
	for _, tt := range tests {
		if got := tt.r.Empty(); got != tt.want {
			t.Errorf("%s: wanted Empty %v, got %v", tt.name, tt.want, got)
		}
	}
}

func TestContainsIsHalfOpen(t *testing.T) {
	r := NewRect(2, 3, 4, 2) // x from 2 to 6, y from 3 to 5
	nan := math.NaN()
	tests := []struct {
		p    Vector2
		want bool
	}{
		{Vector2{X: 2, Y: 3}, true}, // the top left corner is in
		{Vector2{X: 5.999, Y: 4.999}, true},
		{Vector2{X: 4, Y: 4}, true},
		{Vector2{X: 6, Y: 4}, false}, // Max is not
		{Vector2{X: 4, Y: 5}, false},
		{Vector2{X: 6, Y: 5}, false},
		{Vector2{X: 1.999, Y: 4}, false},
		{Vector2{X: 4, Y: 2.999}, false},
		{Vector2{X: nan, Y: 4}, false},
		{Vector2{X: 4, Y: nan}, false},
		{Vector2{X: math.Inf(1), Y: 4}, false},
	}
	for _, tt := range tests {
		if got := r.Contains(tt.p); got != tt.want {
			t.Errorf("Contains(%v): wanted %v, got %v", tt.p, tt.want, got)
		}
	}
	if (Rect{}).Contains(Vector2{}) || NewRect(0, 0, 0, 5).Contains(Vector2{}) {
		t.Errorf("wanted an empty rectangle to contain nothing")
	}
}

func TestOverlapsAndIntersectOnEdgeCases(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	base := NewRect(0, 0, 4, 4)
	tests := []struct {
		name string
		o    Rect
		want Rect // the intersection; the zero Rect when they do not overlap
	}{
		{"identical", base, base},
		{"inside", NewRect(1, 1, 2, 2), NewRect(1, 1, 2, 2)},
		{"around", NewRect(-1, -1, 6, 6), base},
		{"partly", NewRect(3, 2, 4, 4), Rect{Min: Vector2{X: 3, Y: 2}, Max: Vector2{X: 4, Y: 4}}},
		{"touching on the right", NewRect(4, 0, 4, 4), Rect{}},
		{"touching below", NewRect(0, 4, 4, 4), Rect{}},
		{"touching on the left", NewRect(-4, 0, 4, 4), Rect{}},
		{"touching above", NewRect(0, -4, 4, 4), Rect{}},
		{"touching at a corner", NewRect(4, 4, 4, 4), Rect{}},
		{"a hair over the edge", NewRect(math.Nextafter(4, 0), 0, 4, 4), Rect{Min: Vector2{X: math.Nextafter(4, 0)}, Max: Vector2{X: 4, Y: 4}}},
		{"overlapping on x only", NewRect(2, 10, 4, 4), Rect{}},
		{"overlapping on y only", NewRect(10, 2, 4, 4), Rect{}},
		{"an empty one inside", NewRect(2, 2, 0, 0), Rect{}},
		{"an empty one with a width inside", NewRect(2, 2, 3, 0), Rect{}},
		{"turned inside out", Rect{Min: Vector2{X: 3, Y: 3}, Max: Vector2{X: 1, Y: 1}}, Rect{}},
		{"a NaN corner", Rect{Min: Vector2{X: nan, Y: 0}, Max: Vector2{X: 3, Y: 3}}, Rect{}},
		{"a NaN size", NewRect(1, 1, nan, 2), Rect{}},
		{"infinite", Rect{Min: Vector2{X: -inf, Y: -inf}, Max: Vector2{X: inf, Y: inf}}, base},
		{"an infinite half plane", Rect{Min: Vector2{X: 2, Y: -inf}, Max: Vector2{X: inf, Y: inf}}, Rect{Min: Vector2{X: 2}, Max: Vector2{X: 4, Y: 4}}},
		{"far away", NewRect(1e300, 1e300, 1e299, 1e299), Rect{}},
		{"huge and around", NewRect(-1e300, -1e300, 2e300, 2e300), base},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantOverlap := tt.want != (Rect{})
			for _, pair := range [][2]Rect{{base, tt.o}, {tt.o, base}} {
				if got := pair[0].Overlaps(pair[1]); got != wantOverlap {
					t.Errorf("%v.Overlaps(%v): wanted %v, got %v", pair[0], pair[1], wantOverlap, got)
				}
				if got := pair[0].Intersect(pair[1]); got != tt.want {
					t.Errorf("%v.Intersect(%v): wanted %v, got %v", pair[0], pair[1], tt.want, got)
				}
			}
		})
	}
}

func TestTranslate(t *testing.T) {
	r := NewRect(1, 2, 3, 4).Translate(Vector2{X: 10, Y: -5})
	if want := NewRect(11, -3, 3, 4); r != want {
		t.Errorf("wanted %v, got %v", want, r)
	}
	if r := NewRect(0, 0, 2, 2).Translate(Vector2{X: math.NaN()}); !r.Empty() {
		t.Errorf("wanted a NaN move to give an empty rectangle, got %v", r)
	}
}

// halves picks the coordinates of the rectangles the oracle below uses: multiples of a
// half, with plenty of repeats, so that edges often line up.
func halves(rng *rand.Rand) float64 {
	return float64(rng.IntN(9)-4) / 2
}

func randomHalfRect(rng *rand.Rand) Rect {
	// A start and a size, which is mostly positive but sometimes flat or negative, so that
	// some rectangles have no points.
	x, y := halves(rng), halves(rng)
	w, h := float64(rng.IntN(7)-1)/2, float64(rng.IntN(7)-1)/2
	return Rect{Min: Vector2{X: x, Y: y}, Max: Vector2{X: x + w, Y: y + h}}
}

// With every coordinate a multiple of a half, a point sampled at quarters is in the
// overlap of two rectangles if there is any overlap at all, and the overlap's corners are
// sampled points too. So Overlaps and Intersect must agree with sampling, which decides
// them without comparing a single edge.
func TestOverlapsAndIntersectMatchSampling(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	var overlapped, touched int
	for range 100_000 {
		a, b := randomHalfRect(rng), randomHalfRect(rng)

		first, last := Vector2{}, Vector2{}
		found := false
		for x := -2.5; x <= 5; x += 0.25 { // the rectangles lie within -2 and 4.5
			for y := -2.5; y <= 5; y += 0.25 {
				p := Vector2{X: x, Y: y}
				if !a.Contains(p) || !b.Contains(p) {
					continue
				}
				if !found {
					first, found = p, true
				}
				last = p
			}
		}

		if got := a.Overlaps(b); got != found {
			t.Fatalf("%v.Overlaps(%v): sampling says %v, got %v", a, b, found, got)
		}
		if b.Overlaps(a) != found {
			t.Fatalf("wanted Overlaps to be symmetric for %v and %v", a, b)
		}
		want := Rect{}
		if found {
			want = Rect{Min: first, Max: Vector2{X: last.X + 0.25, Y: last.Y + 0.25}}
			overlapped++
		}
		if got := a.Intersect(b); got != want {
			t.Fatalf("%v.Intersect(%v): sampling says %v, got %v", a, b, want, got)
		}
		if !found && !a.Empty() && !b.Empty() && (a.Min.X == b.Max.X || a.Max.X == b.Min.X || a.Min.Y == b.Max.Y || a.Max.Y == b.Min.Y) {
			touched++
		}
	}
	// Guard: the draw reaches overlaps and, among rectangles that do not, many that touch.
	if overlapped < 4_000 || touched < 3_000 {
		t.Errorf("wanted at least 4000 overlaps and 3000 touching pairs, got %d and %d", overlapped, touched)
	}
}

func FuzzRect(f *testing.F) {
	nan, inf := math.NaN(), math.Inf(1)
	f.Add(0.0, 0.0, 4.0, 4.0, 2.0, 2.0, 6.0, 6.0)
	f.Add(0.0, 0.0, 4.0, 4.0, 4.0, 0.0, 8.0, 4.0)
	f.Add(0.0, 0.0, 0.0, 0.0, -1.0, -1.0, 1.0, 1.0)
	f.Add(nan, 0.0, 1.0, 1.0, 0.0, 0.0, 1.0, 1.0)
	f.Add(-inf, -inf, inf, inf, 0.0, 0.0, 1.0, 1.0)
	f.Add(1e308, 1e308, -1e308, -1e308, 5e-324, 5e-324, 0.0, 1.0)
	f.Add(3.0, 3.0, 1.0, 1.0, 0.0, 0.0, 5.0, 5.0)

	f.Fuzz(func(t *testing.T, ax0, ay0, ax1, ay1, bx0, by0, bx1, by1 float64) {
		a := Rect{Min: Vector2{X: ax0, Y: ay0}, Max: Vector2{X: ax1, Y: ay1}}
		b := Rect{Min: Vector2{X: bx0, Y: by0}, Max: Vector2{X: bx1, Y: by1}}

		overlap := a.Overlaps(b)
		if overlap != b.Overlaps(a) {
			t.Fatalf("wanted Overlaps to be symmetric for %v and %v", a, b)
		}
		if overlap && (a.Empty() || b.Empty()) {
			t.Fatalf("wanted an empty rectangle to overlap nothing: %v, %v", a, b)
		}
		i := a.Intersect(b)
		if i != b.Intersect(a) {
			t.Fatalf("wanted Intersect to be symmetric for %v and %v", a, b)
		}
		if !overlap {
			if i != (Rect{}) {
				t.Fatalf("wanted the zero Rect for rectangles that do not overlap, got %v", i)
			}
			return
		}
		if i.Empty() || !a.Contains(i.Min) || !b.Contains(i.Min) {
			t.Fatalf("wanted a non-empty intersection that starts inside both, got %v from %v and %v", i, a, b)
		}
		if i.Max.X > a.Max.X || i.Max.X > b.Max.X || i.Max.Y > a.Max.Y || i.Max.Y > b.Max.Y {
			t.Fatalf("wanted the intersection to end inside both, got %v from %v and %v", i, a, b)
		}
		if !a.Overlaps(i) || !b.Overlaps(i) {
			t.Fatalf("wanted the intersection to overlap both, got %v from %v and %v", i, a, b)
		}
	})
}

func TestPushOutOf(t *testing.T) {
	inf, nan := math.Inf(1), math.NaN()
	tests := []struct {
		name string
		r, o Rect
		want Vector2
	}{
		{"out of a wall on the left, by the shorter way", NewRect(0, 0, 4, 4), NewRect(3, -5, 7, 10), Vector2{X: -1}},
		{"out of a wall on the right", NewRect(8, 0, 4, 4), NewRect(0, -5, 10, 10), Vector2{X: 2}},
		{"out of a floor, upward", NewRect(0, -6, 4, 5), NewRect(-5, -2, 15, 5), Vector2{Y: -1}},
		{"out of a ceiling, downward", NewRect(0, 2, 4, 4), NewRect(-5, 0, 15, 3), Vector2{Y: 1}},
		{"fully inside: the shortest way out", NewRect(4, 4, 1, 1), NewRect(0, 0, 10, 10), Vector2{X: -5}},
		{"fully inside, nearer the bottom", NewRect(4, 7, 1, 1), NewRect(0, 0, 10, 10), Vector2{Y: 3}},
		{"equal both ways goes toward positive", NewRect(3, 3, 2, 2), NewRect(0, 0, 8, 8), Vector2{X: 5}},
		{"equal on both axes goes to X", NewRect(0, 0, 2, 2), NewRect(1, 1, 9, 9), Vector2{X: -1}},
		{"equal on both axes, and X goes toward positive", NewRect(0, 0, 3, 3), NewRect(-5, -5, 7, 7), Vector2{X: 2}},
		{"touching", NewRect(0, 0, 4, 4), NewRect(4, 0, 4, 4), Vector2{}},
		{"touching at a corner", NewRect(0, 0, 4, 4), NewRect(4, 4, 4, 4), Vector2{}},
		{"apart", NewRect(0, 0, 1, 1), NewRect(5, 5, 1, 1), Vector2{}},
		{"the mover is empty", NewRect(2, 2, 0, 3), NewRect(0, 0, 10, 10), Vector2{}},
		{"the wall is empty", NewRect(0, 0, 4, 4), NewRect(2, 2, 0, 0), Vector2{}},
		{"the mover is a NaN", Rect{Min: Vector2{X: nan}, Max: Vector2{X: 4, Y: 4}}, NewRect(0, 0, 3, 3), Vector2{}},
		{"the wall is a NaN", NewRect(0, 0, 4, 4), NewRect(1, 1, nan, 3), Vector2{}},
		{"a floor with no end sideways is left by the other axis", NewRect(0, 2, 2, 2), Rect{Min: Vector2{X: -inf, Y: 3}, Max: Vector2{X: inf, Y: 10}}, Vector2{Y: -1}},
		{"a wall with no end both ways is not left at all", NewRect(0, 0, 2, 2), Rect{Min: Vector2{X: -inf, Y: -inf}, Max: Vector2{X: inf, Y: inf}}, Vector2{}},
		{"a mover with no end sideways leaves by the other axis", Rect{Min: Vector2{X: -inf, Y: 0}, Max: Vector2{X: inf, Y: 2}}, NewRect(0, 1, 3, 4), Vector2{Y: -1}},
		{"huge and finite: the short axis, a tie toward positive", Rect{Min: Vector2{X: -1e308}, Max: Vector2{X: 1e308, Y: 1}}, NewRect(0, 0, 1, 1), Vector2{Y: 1}},
		{"a hair of overlap", NewRect(0, 0, 4, 4), NewRect(math.Nextafter(4, 0), 0, 4, 4), Vector2{X: math.Nextafter(4, 0) - 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.PushOutOf(tt.o); got != tt.want {
				t.Errorf("%v.PushOutOf(%v): wanted %v, got %v", tt.r, tt.o, tt.want, got)
			}
		})
	}
}

// clearsAt is whether r moved by (x, y) no longer overlaps o.
func clearsAt(r, o Rect, x, y float64) bool {
	return !r.Translate(Vector2{X: x, Y: y}).Overlaps(o)
}

// referencePush finds the smallest single-axis move that clears o by trying them in order,
// half a unit at a time, which is all there is to try when every coordinate is a multiple of
// a half. It shares no arithmetic with PushOutOf. Ties are as the doc says: toward positive
// on an axis, and X before Y.
func referencePush(r, o Rect) Vector2 {
	if !r.Overlaps(o) {
		return Vector2{}
	}
	steps := func(dir Vector2) float64 {
		for n := 0.5; n < 40; n += 0.5 {
			if clearsAt(r, o, n*dir.X, n*dir.Y) {
				return n
			}
		}
		return math.Inf(1)
	}
	// The shorter of the two ways on an axis as a move; a tie goes to positive.
	shorter := func(positive, negative float64) float64 {
		if negative < positive {
			return -negative
		}
		return positive
	}
	dx := shorter(steps(Vector2{X: 1}), steps(Vector2{X: -1}))
	dy := shorter(steps(Vector2{Y: 1}), steps(Vector2{Y: -1}))
	if math.Abs(dy) < math.Abs(dx) {
		return Vector2{Y: dy}
	}
	return Vector2{X: dx}
}

// Rectangles with coordinates in halves, drawn so that many overlap: the shortest way out
// is a multiple of a half, so trying them in turn finds it, and PushOutOf must agree on every
// pair, ties included.
func TestPushOutOfMatchesTryingEveryMove(t *testing.T) {
	rng := rand.New(rand.NewPCG(21, 22))
	var pushed, ties, inside int
	for range 50_000 {
		r, o := randomHalfRect(rng), randomHalfRect(rng)
		want, got := referencePush(r, o), r.PushOutOf(o)
		if got != want {
			t.Fatalf("%v.PushOutOf(%v): trying every move says %v, got %v", r, o, want, got)
		}
		if got == (Vector2{}) {
			continue
		}
		pushed++
		if !clearsAt(r, o, got.X, got.Y) {
			t.Fatalf("%v.PushOutOf(%v) = %v: wanted them clear afterwards", r, o, got)
		}
		if got.X != 0 && got.Y != 0 {
			t.Fatalf("wanted a move along one axis, got %v", got)
		}
		// How many of these push on a tie, and how many have the mover inside the wall.
		if math.Abs(r.Max.X-o.Min.X) == math.Abs(o.Max.X-r.Min.X) || math.Abs(r.Max.Y-o.Min.Y) == math.Abs(o.Max.Y-r.Min.Y) {
			ties++
		}
		if r.Min.X >= o.Min.X && r.Max.X <= o.Max.X && r.Min.Y >= o.Min.Y && r.Max.Y <= o.Max.Y {
			inside++
		}
	}
	// Guard: plenty of pushes, ties and movers inside a wall.
	if pushed < 2_000 || ties < 300 || inside < 200 {
		t.Errorf("wanted at least 2000 pushes, 300 ties and 200 movers inside, got %d, %d and %d", pushed, ties, inside)
	}
}

// With numbers that are not exact, the push clears to within rounding, and a smaller one
// does not clear at all.
func TestPushOutOfIsTheSmallestMoveAndClearsToWithinRounding(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 32))
	var pushed int
	for range 50_000 {
		r := NewRect(rng.Float64()*20-10, rng.Float64()*20-10, rng.Float64()*6+0.01, rng.Float64()*6+0.01)
		o := NewRect(rng.Float64()*20-10, rng.Float64()*20-10, rng.Float64()*6+0.01, rng.Float64()*6+0.01)
		move := r.PushOutOf(o)
		if move == (Vector2{}) {
			if r.Overlaps(o) {
				t.Fatalf("wanted a push for %v and %v, which overlap", r, o)
			}
			continue
		}
		pushed++

		left := r.Translate(move).Intersect(o)
		if left.Width() > 1e-9 && left.Height() > 1e-9 {
			t.Fatalf("%v pushed by %v out of %v still overlaps by %v", r, move, o, left)
		}
		if math.Abs(move.X)+math.Abs(move.Y) > 1e-6 && !r.Translate(move.Scale(0.99)).Overlaps(o) {
			t.Fatalf("wanted %v to be the smallest move for %v and %v, a shorter one clears", move, r, o)
		}
	}
	if pushed < 2_000 {
		t.Errorf("wanted at least 2000 pushes, got %d", pushed)
	}
}

func FuzzPushOutOf(f *testing.F) {
	nan, inf := math.NaN(), math.Inf(1)
	f.Add(0.0, 0.0, 4.0, 4.0, 3.0, -5.0, 10.0, 5.0)
	f.Add(4.0, 4.0, 5.0, 5.0, 0.0, 0.0, 10.0, 10.0)
	f.Add(0.0, 0.0, 2.0, 2.0, -inf, -inf, inf, inf)
	f.Add(0.0, 0.0, 2.0, 2.0, -inf, 1.0, inf, 5.0)
	f.Add(nan, 0.0, 1.0, 1.0, 0.0, 0.0, 1.0, 1.0)
	f.Add(-1e308, 0.0, 1e308, 1.0, 0.0, 0.0, 1.0, 1.0)
	f.Add(-1e308, -1e308, 1e308, 1e308, 0.0, 0.0, 1e308, 1e308)
	f.Add(0.0, 0.0, 5e-324, 5e-324, 0.0, 0.0, 1.0, 1.0)
	f.Add(0.0, 0.0, 4.0, 4.0, 4.0, 0.0, 8.0, 4.0)

	f.Fuzz(func(t *testing.T, ax0, ay0, ax1, ay1, bx0, by0, bx1, by1 float64) {
		r := Rect{Min: Vector2{X: ax0, Y: ay0}, Max: Vector2{X: ax1, Y: ay1}}
		o := Rect{Min: Vector2{X: bx0, Y: by0}, Max: Vector2{X: bx1, Y: by1}}

		move := r.PushOutOf(o)
		if move == (Vector2{}) {
			return
		}
		if !r.Overlaps(o) {
			t.Fatalf("wanted no push for %v and %v, which do not overlap, got %v", r, o, move)
		}
		if move.X != 0 && move.Y != 0 || math.IsNaN(move.X+move.Y) || math.IsInf(move.X+move.Y, 0) {
			t.Fatalf("wanted a finite move along one axis, got %v", move)
		}

		// Cleared to within the rounding of the sums, which is relative to the numbers.
		after := r.Translate(move)
		if math.IsInf(after.Min.X+after.Max.X+after.Min.Y+after.Max.Y, 0) {
			return
		}
		scale := max(1, math.Abs(r.Min.X), math.Abs(r.Max.X), math.Abs(o.Min.X), math.Abs(o.Max.X),
			math.Abs(r.Min.Y), math.Abs(r.Max.Y), math.Abs(o.Min.Y), math.Abs(o.Max.Y))
		if scale > 1e300 { // the sums themselves overflow in places
			return
		}
		inter := after.Intersect(o)
		if inter.Width() > 1e-12*scale && inter.Height() > 1e-12*scale {
			t.Fatalf("%v pushed by %v out of %v still overlaps by %v", r, move, o, inter)
		}
	})
}

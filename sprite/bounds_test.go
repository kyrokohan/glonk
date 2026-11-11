package sprite

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

func boxFrame(w, h int) *Frame { return NewFrame(w, h) }

func TestBoundsAreWhatIsDrawn(t *testing.T) {
	at := geometry.Vector2{X: 2.5, Y: -1.25}
	grow := NewAnimation(1, boxFrame(3, 1), boxFrame(5, 2))
	grow.Loop = true
	nilFrame := NewAnimation(1, nil)
	nan, inf := math.NaN(), math.Inf(1)

	tests := []struct {
		name string
		s    *Sprite
		want geometry.Rect
	}{
		{"a cell is 1 by 1", &Sprite{Position: at}, geometry.NewRect(2.5, -1.25, 1, 1)},
		{"a blank cell is too", &Sprite{Position: at, Cell: engine.BlankCell}, geometry.NewRect(2.5, -1.25, 1, 1)},
		{"a frame is its size", &Sprite{Position: at, Frame: boxFrame(4, 3)}, geometry.NewRect(2.5, -1.25, 4, 3)},
		{"a frame with holes is still whole", &Sprite{Position: at, Frame: FromLines(engine.Cell{}, "  x", "   ")}, geometry.NewRect(2.5, -1.25, 3, 2)},
		{"an animation is its current frame", &Sprite{Position: at, Animation: grow}, geometry.NewRect(2.5, -1.25, 3, 1)},
		{"an animation takes the place of the frame", &Sprite{Position: at, Frame: boxFrame(9, 9), Animation: grow}, geometry.NewRect(2.5, -1.25, 3, 1)},
		{"a frame takes the place of the cell", &Sprite{Position: at, Cell: engine.Cell{Character: 'x'}, Frame: boxFrame(2, 2)}, geometry.NewRect(2.5, -1.25, 2, 2)},
		{"an empty frame has an empty box, not the cell's", &Sprite{Position: at, Frame: boxFrame(0, 0)}, geometry.Rect{Min: at, Max: at}},
		{"a frame with no height", &Sprite{Position: at, Frame: &Frame{}}, geometry.Rect{Min: at, Max: at}},
		{"an animation with no frames draws nothing, not the frame", &Sprite{Position: at, Frame: boxFrame(9, 9), Animation: NewAnimation(1)}, geometry.Rect{Min: at, Max: at}},
		{"an animation whose frame is nil", &Sprite{Position: at, Animation: nilFrame}, geometry.Rect{Min: at, Max: at}},
		{"overlay changes nothing", &Sprite{Position: at, Frame: boxFrame(2, 2), Overlay: true}, geometry.NewRect(2.5, -1.25, 2, 2)},
		{"a nil sprite", nil, geometry.Rect{}},
		{"the zero sprite is a cell at the origin", &Sprite{}, geometry.NewRect(0, 0, 1, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Bounds(); got != tt.want {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
			if tt.want.Empty() != tt.s.Bounds().Empty() {
				t.Errorf("wanted the box to be empty: %v", tt.want.Empty())
			}
		})
	}

	// A position that is not a number, or too big to add a size to, has no box.
	for _, x := range []float64{nan, inf, -inf, 1e300} {
		for _, s := range []*Sprite{{Position: geometry.Vector2{X: x}}, {Position: geometry.Vector2{X: x}, Frame: boxFrame(4, 4)}} {
			if got := s.Bounds(); !got.Empty() {
				t.Errorf("X=%v: wanted an empty box, got %v", x, got)
			}
		}
	}
	nanY := &Sprite{Position: geometry.Vector2{Y: nan}}
	if !nanY.Bounds().Empty() {
		t.Errorf("wanted a NaN Y to have no box")
	}
}

func TestBoundsFollowTheAnimation(t *testing.T) {
	a := NewAnimation(1, boxFrame(3, 1), boxFrame(5, 2))
	a.Loop = true
	s := &Sprite{Position: geometry.Vector2{X: 1, Y: 1}, Animation: a}

	for i, want := range [][2]float64{{3, 1}, {5, 2}, {3, 1}, {5, 2}} {
		if got := s.Bounds().Size(); got != (geometry.Vector2{X: want[0], Y: want[1]}) {
			t.Fatalf("second %d: wanted %v, got %v", i, want, got)
		}
		a.Advance(1)
	}
	a.Reset()
	if got := s.Bounds().Size(); got != (geometry.Vector2{X: 3, Y: 1}) {
		t.Errorf("wanted Reset to bring back the first frame's size, got %v", got)
	}
}

func TestSpritesOverlapWhenTheirBoxesShareArea(t *testing.T) {
	frame := func(x, y float64, w, h int) *Sprite {
		return &Sprite{Position: geometry.Vector2{X: x, Y: y}, Frame: boxFrame(w, h)}
	}
	cell := func(x, y float64) *Sprite { return &Sprite{Position: geometry.Vector2{X: x, Y: y}} }

	tests := []struct {
		name string
		a, b *Sprite
		want bool
	}{
		{"cell inside a frame", cell(3, 3), frame(2, 2, 4, 4), true},
		{"cell on the frame's last cell", cell(5, 5), frame(2, 2, 4, 4), true},
		{"cell just past the frame", cell(6, 3), frame(2, 2, 4, 4), false},
		{"cell on the frame's first cell", cell(2, 2), frame(2, 2, 4, 4), true},
		{"cell just before the frame", cell(1, 2), frame(2, 2, 4, 4), false},
		{"two cells in one place", cell(3, 3), cell(3, 3), true},
		{"two cells side by side", cell(3, 3), cell(4, 3), false},
		{"half a cell apart", cell(3, 3), cell(3.5, 3), true},
		{"frames sharing a corner cell", frame(0, 0, 3, 3), frame(2, 2, 3, 3), true},
		{"frames touching at a corner", frame(0, 0, 3, 3), frame(3, 3, 3, 3), false},
		{"an empty frame inside another", frame(3, 3, 0, 0), frame(2, 2, 4, 4), false},
		{"a nil sprite", nil, cell(0, 0), false},
		{"both nil", nil, nil, false},
		{"a NaN position", cell(math.NaN(), 0), cell(0, 0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Overlaps(tt.b); got != tt.want {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
			if got := tt.b.Overlaps(tt.a); got != tt.want {
				t.Errorf("reversed: wanted %v, got %v", tt.want, got)
			}
		})
	}
}

func TestLayerAllIsDrawOrder(t *testing.T) {
	a, b, c := plain(0, 'a'), plain(1, 'b'), plain(2, 'c')
	l := NewLayer()
	if n := len(slices.Collect(l.All())); n != 0 {
		t.Errorf("wanted nothing from an empty layer, got %d", n)
	}
	l.Add(a)
	l.Add(b)
	l.Add(a) // twice
	l.Add(c)
	if got := slices.Collect(l.All()); !slices.Equal(got, []*Sprite{a, b, a, c}) {
		t.Errorf("wanted the sprites in the order they were added, twice for one, got %v", got)
	}

	var seen int
	for range l.All() {
		seen++
		if seen == 2 {
			break
		}
	}
	if seen != 2 {
		t.Errorf("wanted the loop to stop where it was told to, got %d", seen)
	}

	l.Remove(a)
	if got := slices.Collect(l.All()); !slices.Equal(got, []*Sprite{b, c}) {
		t.Errorf("wanted the sprites left after Remove, got %v", got)
	}
	l.Clear()
	if n := len(slices.Collect(l.All())); n != 0 {
		t.Errorf("wanted nothing after Clear, got %d", n)
	}
}

func TestLayerHits(t *testing.T) {
	player := &Sprite{Position: geometry.Vector2{X: 5, Y: 2}, Frame: boxFrame(2, 2)} // 5 up to 7, 2 up to 4
	inside := plain(6, 'i')
	inside.Position.Y = 3
	touching := plain(7, 't') // starts where the player ends
	touching.Position.Y = 2
	corner := &Sprite{Position: geometry.Vector2{X: 4, Y: 1}, Frame: boxFrame(2, 2)} // shares the corner cell
	empty := &Sprite{Position: geometry.Vector2{X: 5, Y: 2}, Frame: boxFrame(0, 0)}
	far := plain(40, 'f')
	nan := &Sprite{Position: geometry.Vector2{X: math.NaN(), Y: 2}}

	l := NewLayer()
	for _, s := range []*Sprite{far, corner, empty, player, touching, nan, inside, corner} {
		l.Add(s)
	}

	got := slices.Collect(l.Hits(player.Bounds()))
	if want := []*Sprite{corner, player, inside, corner}; !slices.Equal(got, want) {
		t.Errorf("wanted the sprites that overlap the player's box in draw order, itself and a twice-added one included, got %d: %v", len(got), got)
	}

	for name, r := range map[string]geometry.Rect{
		"the zero Rect":   {},
		"a flat one":      geometry.NewRect(5, 2, 2, 0),
		"inside out":      {Min: geometry.Vector2{X: 7, Y: 4}, Max: geometry.Vector2{X: 5, Y: 2}},
		"a NaN":           geometry.NewRect(math.NaN(), 2, 2, 2),
		"far from all":    geometry.NewRect(-30, -30, 2, 2),
		"past the others": geometry.NewRect(41, 2, 2, 2),
	} {
		if got := slices.Collect(l.Hits(r)); len(got) != 0 {
			t.Errorf("%s: wanted nothing, got %v", name, got)
		}
	}
	// A box that only touches the player's still takes in the one that starts there.
	if got := slices.Collect(l.Hits(geometry.NewRect(7, 2, 3, 2))); !slices.Equal(got, []*Sprite{touching}) {
		t.Errorf("wanted only the sprite inside that box, got %v", got)
	}

	var first []*Sprite
	for sp := range l.Hits(player.Bounds()) {
		first = append(first, sp)
		break
	}
	if len(first) != 1 || first[0] != corner {
		t.Errorf("wanted the loop to stop at the first hit, got %v", first)
	}

	l.Remove(corner)
	if got := slices.Collect(l.Hits(player.Bounds())); !slices.Equal(got, []*Sprite{player, inside}) {
		t.Errorf("wanted what is left after Remove, got %v", got)
	}
	l.Clear()
	if got := slices.Collect(l.Hits(player.Bounds())); len(got) != 0 {
		t.Errorf("wanted nothing after Clear, got %v", got)
	}
}

// boxSpec is a sprite the way the oracle knows it: its position, and the size of what it
// draws, in quarters of a cell so that everything below is whole numbers.
type boxSpec struct {
	s          *Sprite
	x, y, w, h int  // quarters
	nanPos     bool // a position that is not a number: no box
	sizeless   bool // nothing to draw: no box
}

func randomBoxScene(rng *rand.Rand, n int) []boxSpec {
	specs := make([]boxSpec, 0, n)
	for range n {
		x, y := rng.IntN(36)-12, rng.IntN(36)-12 // quarters, so -3 to 6 cells
		spec := boxSpec{x: x, y: y, w: 4, h: 4}
		pos := geometry.Vector2{X: float64(x) / 4, Y: float64(y) / 4}
		s := &Sprite{Position: pos}
		switch rng.IntN(6) {
		case 0: // a cell
		case 1, 2: // a frame
			fw, fh := 1+rng.IntN(4), 1+rng.IntN(3)
			s.Frame = boxFrame(fw, fh)
			spec.w, spec.h = 4*fw, 4*fh
		case 3: // an animation, with frames of different sizes, at a known place in its cycle
			sizes := [][2]int{{1 + rng.IntN(3), 1 + rng.IntN(3)}, {1 + rng.IntN(4), 1 + rng.IntN(2)}, {2, 1 + rng.IntN(3)}}
			a := NewAnimation(1, boxFrame(sizes[0][0], sizes[0][1]), boxFrame(sizes[1][0], sizes[1][1]), boxFrame(sizes[2][0], sizes[2][1]))
			a.Loop = true
			at := rng.IntN(7)
			for range at {
				a.Advance(1)
			}
			s.Animation = a
			spec.w, spec.h = 4*sizes[at%3][0], 4*sizes[at%3][1]
		case 4: // nothing to draw
			s.Frame = boxFrame(0, 0)
			spec.sizeless = true
		case 5: // a position that is not a number
			s.Position.X = math.NaN()
			spec.nanPos = true
		}
		spec.s = s
		specs = append(specs, spec)
	}
	return specs
}

// Hits against a plain walk over the scene with the sizes the scene was built from, which
// the sprites are never asked for, in whole quarters.
func TestLayerHitsMatchesAWalkOverTheScene(t *testing.T) {
	rng := rand.New(rand.NewPCG(41, 42))
	var queries, hitQueries, hits int
	for range 300 {
		specs := randomBoxScene(rng, 1+rng.IntN(12))
		l := NewLayer()
		for _, sp := range specs {
			l.Add(sp.s)
		}

		for range 20 {
			rx, ry := rng.IntN(36)-12, rng.IntN(36)-12
			rw, rh := rng.IntN(17)-2, rng.IntN(17)-2 // quarters, sometimes none or negative
			r := geometry.NewRect(float64(rx)/4, float64(ry)/4, float64(rw)/4, float64(rh)/4)
			if rng.IntN(25) == 0 {
				r.Min.X = math.NaN()
			}

			var want []*Sprite
			if !math.IsNaN(r.Min.X) && rw > 0 && rh > 0 {
				for _, sp := range specs {
					if sp.nanPos || sp.sizeless {
						continue
					}
					if sp.x < rx+rw && rx < sp.x+sp.w && sp.y < ry+rh && ry < sp.y+sp.h {
						want = append(want, sp.s)
					}
				}
			}
			got := slices.Collect(l.Hits(r))
			if !slices.Equal(got, want) {
				t.Fatalf("query %v: wanted %d hits, got %d", r, len(want), len(got))
			}
			queries++
			if len(want) > 0 {
				hitQueries++
				hits += len(want)
			}
		}
	}
	// Guard: plenty of queries hit something, and some of those hit several.
	if hitQueries < queries/4 || hits < hitQueries+500 {
		t.Errorf("wanted at least a quarter of %d queries to hit and some to hit several, got %d queries and %d hits", queries, hitQueries, hits)
	}
}

func TestBoundsAndQueriesAllocateNothing(t *testing.T) {
	l := NewLayer()
	for i := range 200 {
		l.Add(&Sprite{Position: geometry.Vector2{X: float64(i % 40), Y: float64(i / 40)}, Frame: boxFrame(1+i%3, 1+i%2)})
	}
	r := geometry.NewRect(10, 1, 6, 2)
	player := l.sprites[0]

	var n int
	tests := map[string]func(){
		"Bounds":   func() { n += int(player.Bounds().Width()) },
		"Overlaps": func() { _ = player.Overlaps(l.sprites[1]) },
		"All": func() {
			n += 1
			for range l.All() {
				n++
			}
		},
		"Hits": func() {
			for sp := range l.Hits(r) {
				if sp != player {
					n++
				}
			}
		},
	}
	for name, f := range tests {
		f()
		if got := testing.AllocsPerRun(50, f); got != 0 {
			t.Errorf("%s: wanted no allocation, got %v", name, got)
		}
	}
	_ = n
}

func FuzzLayerHits(f *testing.F) {
	f.Add([]byte{0, 8, 8, 0, 0, 1, 9, 9, 2, 2, 2, 7, 7, 3, 3, 8, 8, 4, 4})
	f.Add([]byte{255, 255, 255, 255, 255, 254, 254, 254, 254, 254, 0, 0, 0, 0, 0})
	f.Add([]byte{})
	f.Add([]byte{3, 1, 1, 1, 1, 3, 200, 200, 5, 5, 3, 128, 128, 1, 1, 0, 0, 40, 40})

	f.Fuzz(func(t *testing.T, data []byte) {
		next := func() byte {
			if len(data) == 0 {
				return 0
			}
			b := data[0]
			data = data[1:]
			return b
		}
		coord := func() float64 { // quarters, with the odd NaN and infinity
			switch b := next(); b {
			case 255:
				return math.NaN()
			case 254:
				return math.Inf(1)
			default:
				return float64(int(b)-128) / 4
			}
		}

		l := NewLayer()
		for range int(next() % 12) {
			s := &Sprite{Position: geometry.Vector2{X: coord(), Y: coord()}}
			switch next() % 4 {
			case 1:
				s.Frame = boxFrame(int(next()%5), int(next()%4))
			case 2:
				a := NewAnimation(float64(next()%4), boxFrame(int(next()%4), int(next()%3)), nil)
				a.Loop = next()%2 == 0
				a.Advance(float64(next()%5) / 2)
				s.Animation = a
			case 3:
				s.Overlay = true
			}
			l.Add(s)
		}
		r := geometry.Rect{Min: geometry.Vector2{X: coord(), Y: coord()}, Max: geometry.Vector2{X: coord(), Y: coord()}}

		got := slices.Collect(l.Hits(r))
		var want []*Sprite
		for sp := range l.All() {
			if r.Overlaps(sp.Bounds()) {
				want = append(want, sp)
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("wanted the sprites whose boxes overlap %v, got %d for %d", r, len(got), len(want))
		}
		if r.Empty() && len(got) != 0 {
			t.Fatalf("wanted nothing from an empty box, got %d", len(got))
		}
	})
}

func BenchmarkLayerHits(b *testing.B) {
	rng := rand.New(rand.NewPCG(51, 52))
	l := NewLayer()
	for range 1000 {
		s := &Sprite{Position: geometry.Vector2{X: rng.Float64() * 220, Y: rng.Float64() * 60}}
		if rng.IntN(3) == 0 {
			s.Frame = boxFrame(1+rng.IntN(4), 1+rng.IntN(3))
		}
		l.Add(s)
	}
	player := &Sprite{Position: geometry.Vector2{X: 100, Y: 30}, Frame: boxFrame(3, 2)}
	box := player.Bounds()

	b.Run("1000 sprites, a few hits", func(b *testing.B) {
		b.ReportAllocs()
		var n int
		for b.Loop() {
			for sp := range l.Hits(box) {
				if sp != player {
					n++
				}
			}
		}
		_ = n
	})
}

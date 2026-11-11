package sprite

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// viewAt is a view whose top left corner is (x, y), on a screen that is 12 by 6.
func viewAt(x, y float64) geometry.Rect { return geometry.NewRect(x, y, 12, 6) }

// cellsOf is the places in fb that hold a character, as "x,y" with the character.
func inkOf(fb *engine.FrameBuffer) map[[2]int]rune {
	out := map[[2]int]rune{}
	for y := range fb.Height() {
		for x := range fb.Width() {
			if c := fb.At(x, y); c.Character != ' ' && c.Character != 0 {
				out[[2]int{x, y}] = c.Character
			}
		}
	}
	return out
}

func TestDrawViewDrawsInTheCellMinusTheViewsCell(t *testing.T) {
	// Each sprite lands in floor(position) - floor(view.Min), two floors and a difference,
	// not the floor of a difference: 5.1 - 2.9 floors to 2, but the cells are 5 and 2.
	tests := []struct {
		name string
		at   geometry.Vector2
		view geometry.Vector2
		want [2]int
	}{
		{"no view", geometry.Vector2{X: 5, Y: 2}, geometry.Vector2{}, [2]int{5, 2}},
		{"a whole view", geometry.Vector2{X: 5, Y: 2}, geometry.Vector2{X: 2, Y: 1}, [2]int{3, 1}},
		{"a fraction in the sprite", geometry.Vector2{X: 5.9, Y: 2.9}, geometry.Vector2{X: 2, Y: 1}, [2]int{3, 1}},
		{"a fraction in the view", geometry.Vector2{X: 5, Y: 2}, geometry.Vector2{X: 2.9, Y: 1.9}, [2]int{3, 1}},
		{"fractions in both: the floors differ from the floor of the difference", geometry.Vector2{X: 5.1, Y: 2.1}, geometry.Vector2{X: 2.9, Y: 1.9}, [2]int{3, 1}},
		{"a view left of the origin shifts the sprite right", geometry.Vector2{X: 1, Y: 1}, geometry.Vector2{X: -3, Y: -2}, [2]int{4, 3}},
		{"a view just under a whole cell left of the origin", geometry.Vector2{X: 1, Y: 1}, geometry.Vector2{X: -0.1, Y: -0.1}, [2]int{2, 2}},
		{"a negative sprite and view", geometry.Vector2{X: -4.5, Y: -2.5}, geometry.Vector2{X: -7.2, Y: -5.2}, [2]int{3, 3}},
		{"a sprite left of the view", geometry.Vector2{X: 1, Y: 3}, geometry.Vector2{X: 2, Y: 0}, [2]int{-1, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := mustBuffer(t, 12, 6)
			l := NewLayer()
			l.Add(&Sprite{Position: tt.at, Cell: engine.Cell{Character: 'x'}})
			l.DrawView(fb, geometry.Rect{Min: tt.view})

			want := map[[2]int]rune{}
			if tt.want[0] >= 0 && tt.want[1] >= 0 {
				want[tt.want] = 'x'
			}
			if got := inkOf(fb); !equalInk(got, want) {
				t.Errorf("wanted %v, got %v", want, got)
			}
		})
	}
}

func equalInk(a, b map[[2]int]rune) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// A sprite and a tile are floored in the world before the view is taken off, so a sprite that
// is on a tile stays on it however the view moves, which a floor of the difference would not.
func TestDrawViewKeepsASpriteOnTheCellItStandsOn(t *testing.T) {
	rng := rand.New(rand.NewPCG(71, 72))
	for range 500 {
		view := geometry.Vector2{X: rng.Float64()*40 - 10, Y: rng.Float64()*20 - 5}
		cx, cy := float64(rng.IntN(30)-5), float64(rng.IntN(15)-3)

		whole := NewLayer()
		whole.Add(&Sprite{Position: geometry.Vector2{X: cx, Y: cy}, Cell: engine.Cell{Character: 'a'}})
		fraction := NewLayer()
		fraction.Add(&Sprite{Position: geometry.Vector2{X: cx + 0.99, Y: cy + 0.99}, Cell: engine.Cell{Character: 'a'}})

		a, b := mustBuffer(t, 30, 14), mustBuffer(t, 30, 14)
		whole.DrawView(a, geometry.Rect{Min: view})
		fraction.DrawView(b, geometry.Rect{Min: view})
		if !equalInk(inkOf(a), inkOf(b)) {
			t.Fatalf("view %v, cell (%v, %v): wanted a sprite anywhere in the cell drawn in the same place, got %v and %v", view, cx, cy, inkOf(a), inkOf(b))
		}
	}
}

func TestDrawViewOnlyUsesTheViewsCorner(t *testing.T) {
	l := NewLayer()
	l.Add(&Sprite{Position: geometry.Vector2{X: 5, Y: 3}, Cell: engine.Cell{Character: 'x'}})
	var pictures []map[[2]int]rune
	for _, view := range []geometry.Rect{
		{Min: geometry.Vector2{X: 2, Y: 1}},
		geometry.NewRect(2, 1, 1, 1),
		geometry.NewRect(2, 1, 1000, 1000),
		{Min: geometry.Vector2{X: 2, Y: 1}, Max: geometry.Vector2{X: -5, Y: -5}},
		{Min: geometry.Vector2{X: 2, Y: 1}, Max: geometry.Vector2{X: math.NaN(), Y: math.Inf(1)}},
	} {
		fb := mustBuffer(t, 12, 6)
		l.DrawView(fb, view)
		pictures = append(pictures, inkOf(fb))
	}
	for i, p := range pictures {
		if !equalInk(p, pictures[0]) || len(p) != 1 {
			t.Errorf("view %d: wanted the same single sprite in every case, got %v", i, p)
		}
	}
}

func TestDrawViewDrawsNothingForAViewThatIsNotANumber(t *testing.T) {
	l := NewLayer()
	l.Add(&Sprite{Position: geometry.Vector2{X: 5, Y: 3}, Cell: engine.Cell{Character: 'x'}})
	l.Add(&Sprite{Position: geometry.Vector2{X: 2, Y: 2}, Frame: NewFrame(2, 2), Overlay: true})
	nan, inf := math.NaN(), math.Inf(1)
	for _, min := range []geometry.Vector2{{X: nan}, {Y: nan}, {X: inf}, {X: -inf}, {Y: inf}, {Y: -inf}, {X: nan, Y: inf}} {
		fb := mustBuffer(t, 12, 6)
		l.DrawView(fb, geometry.Rect{Min: min})
		if got := inkOf(fb); len(got) != 0 {
			t.Errorf("view at %v: wanted nothing drawn, got %v", min, got)
		}
		if !slices.Equal(gridOf(fb), gridOf(mustBuffer(t, 12, 6))) {
			t.Errorf("view at %v: wanted the buffer untouched", min)
		}
	}

	// A view far away, finite, shows nothing either, and nothing breaks.
	for _, far := range []float64{1e9, -1e9, 1e300, -1e300, math.MaxFloat64} {
		fb := mustBuffer(t, 12, 6)
		l.DrawView(fb, geometry.Rect{Min: geometry.Vector2{X: far, Y: far}})
		if got := inkOf(fb); len(got) != 0 {
			t.Errorf("view at %v: wanted nothing drawn, got %v", far, got)
		}
	}
}

func TestDrawViewStillSkipsSpritesThatHaveNoPosition(t *testing.T) {
	l := NewLayer()
	l.Add(&Sprite{Position: geometry.Vector2{X: math.NaN(), Y: 1}, Cell: engine.Cell{Character: 'a'}})
	l.Add(&Sprite{Position: geometry.Vector2{X: 1, Y: math.Inf(1)}, Cell: engine.Cell{Character: 'b'}})
	l.Add(&Sprite{Position: geometry.Vector2{X: math.Inf(-1), Y: 1}, Frame: NewFrame(2, 2)})
	l.Add(&Sprite{Position: geometry.Vector2{X: 3, Y: 3}, Cell: engine.Cell{Character: 'c'}})
	fb := mustBuffer(t, 12, 6)
	l.DrawView(fb, viewAt(1, 1))
	if got, want := inkOf(fb), (map[[2]int]rune{{2, 2}: 'c'}); !equalInk(got, want) {
		t.Errorf("wanted %v, got %v", want, got)
	}
}

func TestDrawViewKeepsTheDrawOrder(t *testing.T) {
	l := NewLayer()
	l.Add(&Sprite{Position: geometry.Vector2{X: 5, Y: 3}, Cell: engine.Cell{Character: 'a'}})
	l.Add(&Sprite{Position: geometry.Vector2{X: 5.5, Y: 3.5}, Cell: engine.Cell{Character: 'b'}})
	fb := mustBuffer(t, 12, 6)
	l.DrawView(fb, viewAt(2, 1))
	if got := fb.At(3, 2).Character; got != 'b' {
		t.Errorf("wanted the later sprite on top, got %q", got)
	}
}

// randomViewScene is a backdrop and sprites of every kind at positions around a screen of
// the size, in whole and fractional cells.
func randomViewScene(rng *rand.Rand, w, h int, around geometry.Vector2) (backdrop *engine.FrameBuffer, sprites []*Sprite) {
	fb, _ := engine.NewFrameBuffer(w, h)
	for y := range h {
		for x := range w {
			fb.Set(x, y, randomCell(rng))
		}
	}
	randomFrame := func() *Frame {
		f := NewFrame(rng.IntN(6), rng.IntN(5))
		for y := range f.Height() {
			for x := range f.Width() {
				f.Set(x, y, randomCell(rng))
			}
		}
		return f
	}
	for range 1 + rng.IntN(5) {
		sp := &Sprite{
			Position: geometry.Vector2{X: around.X + rng.Float64()*26 - 6, Y: around.Y + rng.Float64()*14 - 4},
			Cell:     randomCell(rng),
			Overlay:  rng.IntN(2) == 0,
		}
		switch rng.IntN(5) {
		case 0:
		case 1:
			a := NewAnimation(1+rng.Float64()*4, randomFrame(), nil, randomFrame())
			a.Loop = rng.IntN(2) == 0
			a.Advance(rng.Float64() * 2)
			sp.Animation = a
		default:
			sp.Frame = randomFrame()
		}
		sprites = append(sprites, sp)
	}
	return fb, sprites
}

// Drawing through a view is drawing the same sprites moved by the view's cell, a whole
// number of cells, which the reference renderer knows how to do.
func TestDrawViewMatchesTheReferenceWithTheSpritesMoved(t *testing.T) {
	rng := rand.New(rand.NewPCG(81, 82))
	var shifted int
	for range 3000 {
		w, h := 1+rng.IntN(14), 1+rng.IntN(8)
		view := geometry.Vector2{X: rng.Float64()*40 - 15, Y: rng.Float64()*24 - 9}
		fb, sprites := randomViewScene(rng, w, h, view) // around the view, so most are on screen
		backdrop := gridOf(fb)
		if rng.IntN(4) == 0 {
			view = geometry.Vector2{X: math.Floor(view.X), Y: math.Floor(view.Y)} // exactly on a cell
		}

		l := NewLayer()
		var moved []*Sprite
		for _, sp := range sprites {
			l.Add(sp)
			cp := *sp
			cp.Position.X -= math.Floor(view.X)
			cp.Position.Y -= math.Floor(view.Y)
			moved = append(moved, &cp)
		}
		l.DrawView(fb, geometry.Rect{Min: view})

		want := reference(w, h, backdrop, moved)
		if got := gridOf(fb); !slices.Equal(got, want) {
			t.Fatalf("buffer %dx%d, view %v: wanted what the reference draws with the sprites moved", w, h, view)
		}
		if !slices.Equal(want, backdrop) {
			shifted++
		}
	}
	if shifted < 500 {
		t.Errorf("wanted at least 500 of the scenes to draw something, got %d", shifted)
	}
}

// With the view at the origin, DrawView is Draw.
func TestDrawViewAtTheOriginIsDraw(t *testing.T) {
	rng := rand.New(rand.NewPCG(91, 92))
	for range 1000 {
		w, h := 1+rng.IntN(14), 1+rng.IntN(8)
		a, sprites := randomViewScene(rng, w, h, geometry.Vector2{})
		b := mustBuffer(t, w, h)
		for y := range h {
			for x := range w {
				b.Set(x, y, a.At(x, y))
			}
		}
		l := NewLayer()
		for _, sp := range sprites {
			l.Add(sp)
		}
		l.Draw(a)
		l.DrawView(b, geometry.Rect{})
		if !slices.Equal(gridOf(a), gridOf(b)) {
			t.Fatalf("buffer %dx%d: wanted a view at the origin to draw what Draw does", w, h)
		}
	}
}

func TestDrawViewAllocatesNothing(t *testing.T) {
	l := NewLayer()
	for i := range 100 {
		l.Add(&Sprite{Position: geometry.Vector2{X: float64(i % 20), Y: float64(i / 20)}, Cell: engine.Cell{Character: 'x'}})
		l.Add(&Sprite{Position: geometry.Vector2{X: float64(i % 20), Y: float64(i / 20)}, Frame: NewFrame(2, 2), Overlay: true})
	}
	fb := mustBuffer(t, 40, 12)
	view := viewAt(3.5, 1.5)
	if n := testing.AllocsPerRun(50, func() { l.DrawView(fb, view) }); n != 0 {
		t.Errorf("wanted no allocation, got %v", n)
	}
}

func FuzzLayerDrawView(f *testing.F) {
	f.Add([]byte{3, 10, 10, 1, 2, 3, 4, 5, 6, 7, 8}, 2.5, 1.5)
	f.Add([]byte{1, 255, 255, 0, 0}, math.NaN(), 0.0)
	f.Add([]byte{5, 128, 128, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}, 1e300, -1e300)
	f.Add([]byte{}, math.Inf(1), math.Inf(-1))
	f.Add([]byte{2, 0, 0, 1, 1, 200, 200, 3, 3}, -0.5, -0.5)

	f.Fuzz(func(t *testing.T, data []byte, vx, vy float64) {
		next := func() byte {
			if len(data) == 0 {
				return 0
			}
			b := data[0]
			data = data[1:]
			return b
		}
		coord := func() float64 { return (float64(next()) - 100) / 4 } // quarters, -25 up to 38

		l := NewLayer()
		var moved []*Sprite
		fx, fy := math.Floor(vx), math.Floor(vy)
		finite := !math.IsNaN(fx+fy) && math.Abs(fx) < 1e6 && math.Abs(fy) < 1e6
		for range int(next() % 8) {
			sp := &Sprite{Position: geometry.Vector2{X: coord(), Y: coord()}, Cell: engine.Cell{Character: 'a' + rune(next()%26)}, Overlay: next()%2 == 0}
			if next()%2 == 0 {
				sp.Frame = NewFrame(int(next()%5), int(next()%4))
			}
			l.Add(sp)
			cp := *sp
			cp.Position.X -= fx
			cp.Position.Y -= fy
			moved = append(moved, &cp)
		}

		fb := mustBuffer(t, 12, 6)
		backdrop := gridOf(fb)
		l.DrawView(fb, geometry.Rect{Min: geometry.Vector2{X: vx, Y: vy}})

		switch {
		case math.IsNaN(vx+vy) || math.IsInf(vx, 0) || math.IsInf(vy, 0):
			if !slices.Equal(gridOf(fb), backdrop) {
				t.Fatalf("wanted nothing drawn for a view at (%v, %v)", vx, vy)
			}
		case finite:
			if want := reference(12, 6, backdrop, moved); !slices.Equal(gridOf(fb), want) {
				t.Fatalf("view (%v, %v): wanted what the reference draws with the sprites moved", vx, vy)
			}
		}
	})
}

func BenchmarkLayerDrawView(b *testing.B) {
	fb := mustBuffer(b, 200, 60)
	rng := rand.New(rand.NewPCG(71, 72))
	cells := NewLayer()
	for range 1000 {
		cells.Add(&Sprite{Position: geometry.Vector2{X: rng.Float64() * 400, Y: rng.Float64() * 120}, Cell: engine.Cell{Character: 'o'}})
	}
	view := geometry.NewRect(37.5, 12.5, 200, 60)
	b.Run("1000 single cells through a view", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			cells.DrawView(fb, view)
		}
	})
}

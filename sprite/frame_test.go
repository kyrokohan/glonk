package sprite

import (
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

var (
	red   = engine.Palette(196)
	green = engine.Palette(46)
)

// cellsOfFrame lists the cells of a frame row by row.
func cellsOfFrame(f *Frame) []engine.Cell {
	var out []engine.Cell
	for y := range f.Height() {
		for x := range f.Width() {
			out = append(out, f.At(x, y))
		}
	}
	return out
}

func TestFromLinesMakesAGridOfTheTextArt(t *testing.T) {
	style := engine.Cell{Foreground: red, Background: green, Attrs: engine.Bold}
	letter := func(r rune) engine.Cell {
		return engine.Cell{Character: r, Foreground: red, Background: green, Attrs: engine.Bold}
	}
	blank := engine.BlankCell

	tests := []struct {
		name   string
		lines  []string
		w, h   int
		wanted []engine.Cell // row by row
	}{
		{"two rows", []string{" /\\ ", "/__\\"}, 4, 2, []engine.Cell{
			blank, letter('/'), letter('\\'), blank,
			letter('/'), letter('_'), letter('_'), letter('\\'),
		}},
		{"ragged lines are padded on the right", []string{"ab", "c", ""}, 2, 3, []engine.Cell{
			letter('a'), letter('b'),
			letter('c'), blank,
			blank, blank,
		}},
		{"one cell for each character, not each byte", []string{"é中"}, 2, 1, []engine.Cell{letter('é'), letter('中')}},
		{"spaces in the middle are holes", []string{"a b"}, 3, 1, []engine.Cell{letter('a'), blank, letter('b')}},
		{"no lines", nil, 0, 0, nil},
		{"only empty lines", []string{"", ""}, 0, 0, nil},
		{"one empty line among others", []string{"", "x"}, 1, 2, []engine.Cell{blank, letter('x')}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FromLines(style, tt.lines...)

			if f.Width() != tt.w || f.Height() != tt.h {
				t.Fatalf("wanted %d by %d, got %d by %d", tt.w, tt.h, f.Width(), f.Height())
			}
			if got := cellsOfFrame(f); !slices.Equal(got, tt.wanted) {
				t.Errorf("wanted cells %+v, got %+v", tt.wanted, got)
			}
		})
	}
}

// The style's colors and attributes go on the letters only, and its own Character is not used.
func TestFromLinesStylesLettersNotHoles(t *testing.T) {
	f := FromLines(engine.Cell{Character: 'Z', Background: green}, "a b")

	if got := f.At(0, 0); got.Background != green || got.Character != 'a' {
		t.Errorf("wanted the letter to carry the background and keep its own character, got %+v", got)
	}
	if got := f.At(1, 0); got != engine.BlankCell {
		t.Errorf("wanted the hole to stay a plain blank, got %+v", got)
	}
}

func TestNewFrameMakesBlankCellsAndRefusesBadSizes(t *testing.T) {
	f := NewFrame(3, 2)
	if f.Width() != 3 || f.Height() != 2 {
		t.Fatalf("wanted 3 by 2, got %d by %d", f.Width(), f.Height())
	}
	for _, c := range cellsOfFrame(f) {
		if c != engine.BlankCell {
			t.Fatalf("wanted blank cells, got %+v", c)
		}
	}

	for _, tt := range []struct {
		name string
		w, h int
	}{
		{"zero columns", 0, 5}, {"zero rows", 5, 0}, {"negative width", -1, 3}, {"negative height", 3, -1},
		{"too many cells", 1<<20 + 1, 1}, {"too many in two directions", 1 << 11, 1 << 11},
		{"overflows an int", math.MaxInt, 2}, {"overflows both", 1 << 30, 1 << 30}, {"minimum int", math.MinInt, math.MinInt},
	} {
		if f := NewFrame(tt.w, tt.h); f.Width() != 0 || f.Height() != 0 {
			t.Errorf("%s: wanted an empty frame, got %d by %d", tt.name, f.Width(), f.Height())
		}
	}
	if f := NewFrame(1<<20, 1); f.Width() != 1<<20 {
		t.Errorf("wanted a frame of exactly the most cells to be made, got %d wide", f.Width())
	}
}

func TestFromLinesRefusesTooMuchText(t *testing.T) {
	// A thousand lines of a thousand characters is the limit; one more cell is not made.
	line := strings.Repeat("x", 1<<10)
	ok := make([]string, 1<<10)
	for i := range ok {
		ok[i] = line
	}
	if f := FromLines(engine.Cell{}, ok...); f.Width() != 1<<10 || f.Height() != 1<<10 {
		t.Errorf("wanted the full frame, got %d by %d", f.Width(), f.Height())
	}
	if f := FromLines(engine.Cell{}, append(ok, "x")...); f.Width() != 0 || f.Height() != 0 {
		t.Errorf("wanted an empty frame for one cell too many, got %d by %d", f.Width(), f.Height())
	}
	// Many short lines and one long one: the grid is far bigger than the text.
	spiky := make([]string, 1<<11)
	spiky[0] = strings.Repeat("x", 1<<11)
	if f := FromLines(engine.Cell{}, spiky...); f.Width() != 0 {
		t.Errorf("wanted an empty frame when the padded grid is too big, got %d wide", f.Width())
	}
}

func TestFrameAtSetAndClone(t *testing.T) {
	f := NewFrame(3, 2)
	cell := engine.Cell{Character: 'q', Foreground: red}
	f.Set(2, 1, cell)
	f.Set(3, 1, engine.Cell{Character: 'x'}) // outside: ignored
	f.Set(-1, 0, engine.Cell{Character: 'x'})
	f.Set(0, 2, engine.Cell{Character: 'x'})

	if got := f.At(2, 1); got != cell {
		t.Errorf("wanted %+v, got %+v", cell, got)
	}
	for _, p := range [][2]int{{3, 1}, {-1, 0}, {0, 2}, {0, -1}, {99, 99}} {
		if got := f.At(p[0], p[1]); got != engine.BlankCell {
			t.Errorf("At(%d,%d): wanted a blank outside the frame, got %+v", p[0], p[1], got)
		}
	}
	if n := len(slices.DeleteFunc(cellsOfFrame(f), func(c engine.Cell) bool { return c == engine.BlankCell })); n != 1 {
		t.Errorf("wanted one cell set and writes outside ignored, got %d", n)
	}

	c := f.Clone()
	c.Set(0, 0, engine.Cell{Character: 'z'})
	if f.At(0, 0) != engine.BlankCell {
		t.Errorf("wanted a clone to share nothing with the original")
	}
	if c.At(2, 1) != cell || c.Width() != 3 || c.Height() != 2 {
		t.Errorf("wanted the clone to hold what the original did")
	}
}

func TestNilFrameIsAFrameOfNoCells(t *testing.T) {
	var f *Frame
	f.Set(0, 0, engine.Cell{Character: 'x'})
	if f.Width() != 0 || f.Height() != 0 || f.At(0, 0) != engine.BlankCell {
		t.Errorf("wanted a nil frame to be empty")
	}
	if c := f.Clone(); c.Width() != 0 || c.Height() != 0 {
		t.Errorf("wanted a clone of nil to be empty")
	}
}

// reference draws sprites the plain way, to compare Layer.Draw with: the cell at column
// col of row row of a frame goes to (floor(x)+col, floor(y)+row), worked out in float64
// and kept if it is on the buffer. Nothing here shares clipping arithmetic with the
// code under test.
func reference(w, h int, grid []engine.Cell, sprites []*Sprite) []engine.Cell {
	out := slices.Clone(grid)
	put := func(tx, ty float64, cell engine.Cell) {
		if tx >= 0 && tx < float64(w) && ty >= 0 && ty < float64(h) { // false for NaN
			out[int(ty)*w+int(tx)] = cell
		}
	}
	for _, sp := range sprites {
		fx, fy := math.Floor(sp.Position.X), math.Floor(sp.Position.Y)
		if sp.Frame == nil {
			put(fx, fy, sp.Cell)
			continue
		}
		for row := range sp.Frame.Height() {
			for col := range sp.Frame.Width() {
				put(fx+float64(col), fy+float64(row), sp.Frame.At(col, row))
			}
		}
	}
	return out
}

func randomCell(rng *rand.Rand) engine.Cell {
	return engine.Cell{
		Character:  rune("ab#/ .xyz"[rng.IntN(9)]),
		Foreground: engine.Palette(uint8(rng.IntN(4))),
		Background: engine.Palette(uint8(rng.IntN(3))),
		Attrs:      engine.Attr(rng.IntN(3)),
	}
}

func gridOf(fb *engine.FrameBuffer) []engine.Cell {
	var out []engine.Cell
	for y := range fb.Height() {
		for x := range fb.Width() {
			out = append(out, fb.At(x, y))
		}
	}
	return out
}

// Layer.Draw puts every cell of every frame where the plain reference puts it, over a
// colored backdrop, for frames of every size, positions with fractions, negative ones,
// past each edge, beyond what an int holds, NaN and infinite, and sprites that overlap.
func TestLayerDrawsFramesLikeTheReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(61, 62))
	pos := func() float64 {
		switch rng.IntN(14) {
		case 0:
			return math.NaN()
		case 1:
			return math.Inf(1 - 2*rng.IntN(2))
		case 2:
			return (rng.Float64()*2 - 1) * 1e18
		case 3:
			return math.MaxFloat64
		}
		return rng.Float64()*20 - 6
	}

	partial := 0
	for range 3000 {
		w, h := 1+rng.IntN(12), 1+rng.IntN(7)
		fb, err := engine.NewFrameBuffer(w, h)
		if err != nil {
			t.Fatal(err)
		}
		for y := range h {
			for x := range w {
				fb.Set(x, y, randomCell(rng))
			}
		}
		backdrop := gridOf(fb)

		var sprites []*Sprite
		layer := NewLayer()
		for range 1 + rng.IntN(4) {
			sp := &Sprite{Position: geometry.Vector2{X: pos(), Y: pos()}, Cell: randomCell(rng)}
			if rng.IntN(5) != 0 { // most have a frame
				f := NewFrame(rng.IntN(6), rng.IntN(5))
				for y := range f.Height() {
					for x := range f.Width() {
						f.Set(x, y, randomCell(rng))
					}
				}
				sp.Frame = f
			}
			sprites = append(sprites, sp)
			layer.Add(sp)
		}

		layer.Draw(fb)

		want := reference(w, h, backdrop, sprites)
		if got := gridOf(fb); !slices.Equal(got, want) {
			t.Fatalf("buffer %dx%d: wanted what the reference draws", w, h)
		}
		for _, sp := range sprites {
			if sp.Frame != nil && sp.Frame.Width() > 1 && sp.Position.X > -float64(sp.Frame.Width()) && sp.Position.X < 0 {
				partial++ // sticking out of the left edge, to be sure that case is exercised
			}
		}
	}
	if partial < 100 {
		t.Errorf("wanted at least 100 frames sticking out of the left edge among the cases, got %d", partial)
	}
}

// A frame of one cell is a single sprite: it draws the cell where the sprite would.
func TestFrameOfOneCellDrawsLikeASingleSprite(t *testing.T) {
	rng := rand.New(rand.NewPCG(63, 64))
	for range 1000 {
		x, y := rng.Float64()*14-3, rng.Float64()*8-2
		cell := randomCell(rng)
		single, framed := &Sprite{Position: geometry.Vector2{X: x, Y: y}, Cell: cell}, &Sprite{Position: geometry.Vector2{X: x, Y: y}}
		framed.Frame = NewFrame(1, 1)
		framed.Frame.Set(0, 0, cell)

		a, b := mustBuffer(t, 10, 5), mustBuffer(t, 10, 5)
		la, lb := NewLayer(), NewLayer()
		la.Add(single)
		lb.Add(framed)
		la.Draw(a)
		lb.Draw(b)

		if !slices.Equal(gridOf(a), gridOf(b)) {
			t.Fatalf("sprite at (%v, %v): wanted a frame of one cell to draw the same cell", x, y)
		}
	}
}

func mustBuffer(t testing.TB, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatal(err)
	}
	return fb
}

// Drawn by hand: the picture lands with its top left cell at the floored position, and
// what is off any edge is cut off cell by cell. A negative fraction floors down, so -0.1
// is the column before the first.
func TestFramesAreClippedCellByCell(t *testing.T) {
	art := FromLines(engine.Cell{}, "abc", "def")
	draw := func(x, y float64) string {
		fb := mustBuffer(t, 5, 3)
		l := NewLayer()
		l.Add(&Sprite{Position: geometry.Vector2{X: x, Y: y}, Frame: art})
		l.Draw(fb)
		return fb.String()
	}

	tests := []struct {
		name string
		x, y float64
		want string
	}{
		{"inside", 1, 1, "\n abc\n def"},
		{"inside, whole", 1, 0, " abc\n def\n"},
		{"fractions floor", 1.9, 0.9, " abc\n def\n"},
		{"off the left edge", -1, 0, "bc\nef\n"},
		{"a negative fraction floors down", -0.1, 0, "bc\nef\n"},
		{"off the top edge", 0, -1, "def\n\n"},
		{"off the right edge", 3, 0, "   ab\n   de\n"},
		{"off the bottom edge", 0, 2, "\n\nabc"},
		{"the corner only", 4, 2, "\n\n    a"},
		{"just off the left", -3, 0, "\n\n"},
		{"just off the right", 5, 0, "\n\n"},
		{"just off the top", 0, -2, "\n\n"},
		{"just off the bottom", 0, 3, "\n\n"},
		{"the last column only", -2, 0, "c\nf\n"},
	}
	for _, tt := range tests {
		if got := draw(tt.x, tt.y); got != tt.want {
			t.Errorf("%s: at (%v, %v) wanted %q, got %q", tt.name, tt.x, tt.y, tt.want, got)
		}
	}
}

// A frame replaces the cells under it, blanks of the art included, and a later sprite
// draws over an earlier one.
func TestFramesReplaceWhatIsUnderThemAndLayerInOrder(t *testing.T) {
	fb := mustBuffer(t, 4, 1)
	for x := range 4 {
		fb.Set(x, 0, engine.Cell{Character: '.', Background: green})
	}
	l := NewLayer()
	l.Add(&Sprite{Position: geometry.Vector2{X: 0}, Frame: FromLines(engine.Cell{}, "a b")})
	l.Add(&Sprite{Position: geometry.Vector2{X: 2}, Frame: FromLines(engine.Cell{}, "cd")})

	l.Draw(fb)

	if got := fb.String(); got != "a cd" {
		t.Errorf("wanted the blank of the first frame to cover the dot, and the second frame over its last letter, got %q", got)
	}
	if got := fb.At(1, 0); got != engine.BlankCell {
		t.Errorf("wanted the hole to replace the cell whole, background included, got %+v", got)
	}
}

// A Frame is drawn in place of Cell, an empty Frame draws nothing and does not fall back
// to Cell, and no Frame is the single sprite it always was.
func TestFrameTakesThePlaceOfCell(t *testing.T) {
	cell := engine.Cell{Character: 'C'}
	tests := []struct {
		name  string
		frame *Frame
		want  string
	}{
		{"no frame draws the cell", nil, "C"},
		{"a frame replaces the cell", FromLines(engine.Cell{}, "ab"), "ab"},
		{"an empty frame draws nothing", NewFrame(0, 0), ""},
		{"a nil-sized frame from no text draws nothing", FromLines(engine.Cell{}), ""},
	}
	for _, tt := range tests {
		fb := mustBuffer(t, 4, 1)
		l := NewLayer()
		l.Add(&Sprite{Cell: cell, Frame: tt.frame})
		l.Draw(fb)
		if got := fb.String(); got != tt.want {
			t.Errorf("%s: wanted %q, got %q", tt.name, tt.want, got)
		}
	}
}

// Drawing frames must not panic or draw outside the buffer, wherever the sprite is.
func FuzzFrameDraw(f *testing.F) {
	f.Add(1.9, 2.1, uint8(3), uint8(2))
	f.Add(-0.1, -0.1, uint8(5), uint8(4))
	f.Add(7.999, 3.999, uint8(2), uint8(2))
	f.Add(math.NaN(), 1.0, uint8(3), uint8(3))
	f.Add(1.0, math.Inf(-1), uint8(3), uint8(3))
	f.Add(1e300, -1e300, uint8(255), uint8(255))
	f.Add(-8.0, 0.0, uint8(8), uint8(1))
	f.Add(math.MaxFloat64, math.SmallestNonzeroFloat64, uint8(1), uint8(1))

	f.Fuzz(func(t *testing.T, x, y float64, fw, fh uint8) {
		const w, h = 8, 4
		fb, err := engine.NewFrameBuffer(w, h)
		if err != nil {
			t.Fatal(err)
		}
		frame := NewFrame(int(fw), int(fh))
		for cy := range frame.Height() {
			for cx := range frame.Width() {
				frame.Set(cx, cy, engine.Cell{Character: rune('a' + (cx+cy)%26), Foreground: red})
			}
		}
		sp := &Sprite{Position: geometry.Vector2{X: x, Y: y}, Frame: frame}
		l := NewLayer()
		l.Add(sp)

		l.Draw(fb)

		blank := make([]engine.Cell, w*h)
		for i := range blank {
			blank[i] = engine.BlankCell
		}
		want := reference(w, h, blank, []*Sprite{sp})
		if got := gridOf(fb); !slices.Equal(got, want) {
			t.Fatalf("frame %dx%d at (%v, %v): wanted what the reference draws", fw, fh, x, y)
		}
	})
}

func BenchmarkLayerDraw(b *testing.B) {
	fb, err := engine.NewFrameBuffer(200, 60)
	if err != nil {
		b.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(71, 72))

	cells := NewLayer()
	for range 1000 {
		cells.Add(&Sprite{Position: geometry.Vector2{X: rng.Float64() * 220, Y: rng.Float64() * 70}, Cell: engine.Cell{Character: 'o', Foreground: red}})
	}
	b.Run("1000 single cells", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			cells.Draw(fb)
		}
	})

	art := FromLines(engine.Cell{Foreground: green}, " /\\ ", "/__\\")
	frames := NewLayer()
	for range 200 {
		frames.Add(&Sprite{Position: geometry.Vector2{X: rng.Float64() * 220, Y: rng.Float64() * 70}, Frame: art})
	}
	b.Run("200 frames of 4 by 2", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			frames.Draw(fb)
		}
	})
}

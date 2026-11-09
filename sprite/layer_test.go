package sprite

import (
	"math"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

const (
	frameW = 10
	frameH = 5
)

func newFrame(t *testing.T) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(frameW, frameH)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

// drawn lists the cells that are not blank, as "x,y".
func drawn(fb *engine.FrameBuffer) map[[2]int]rune {
	got := map[[2]int]rune{}
	for y := range fb.Height() {
		for x := range fb.Width() {
			if c := fb.At(x, y); c != engine.BlankCell {
				got[[2]int{x, y}] = c.Character
			}
		}
	}
	return got
}

func TestLayerDrawFloorsThePosition(t *testing.T) {
	tests := []struct {
		name string
		x, y float64
		want map[[2]int]rune // nil: nothing drawn
	}{
		{"origin", 0, 0, map[[2]int]rune{{0, 0}: 'X'}},
		{"fraction floors down", 2.9, 1.1, map[[2]int]rune{{2, 1}: 'X'}},
		{"last cell", frameW - 0.1, frameH - 0.1, map[[2]int]rune{{frameW - 1, frameH - 1}: 'X'}},
		{"just left of the frame", -0.1, 0, nil},
		{"just above the frame", 0, -0.1, nil},
		{"one past the right edge", frameW, 0, nil},
		{"one past the bottom edge", 0, frameH, nil},
		{"far away", 1000, -1000, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLayer()
			l.Add(New(geometry.Vector2{X: tt.x, Y: tt.y}, 'X'))
			fb := newFrame(t)
			l.Draw(fb)

			got := drawn(fb)
			if len(got) != len(tt.want) {
				t.Fatalf("wanted %v drawn, got %v", tt.want, got)
			}
			for pos, r := range tt.want {
				if got[pos] != r {
					t.Errorf("wanted %q at %v, got %v", r, pos, got)
				}
			}
		})
	}
}

// What int(NaN) and int(1e300) give differs between CPUs: on some the result is
// cell 0, so a sprite with no real position must be stopped before converting.
func TestLayerDrawSkipsSpritesWithoutARealPosition(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)

	tests := []struct {
		name string
		x, y float64
	}{
		{"NaN x", nan, 1},
		{"NaN y", 1, nan},
		{"NaN both", nan, nan},
		{"+Inf x", inf, 1},
		{"-Inf x", -inf, 1},
		{"+Inf y", 1, inf},
		{"-Inf y", 1, -inf},
		{"1e300", 1e300, 1e300},
		{"-1e300", -1e300, 1},
		{"largest float", math.MaxFloat64, 0},
		{"just past int64", 1 << 63, 0},
		{"smallest int64", -(1 << 63), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLayer()
			l.Add(New(geometry.Vector2{X: tt.x, Y: tt.y}, 'X'))
			fb := newFrame(t)
			l.Draw(fb)

			if got := drawn(fb); len(got) != 0 {
				t.Errorf("wanted nothing drawn, got %v", got)
			}
		})
	}
}

func TestLayerIgnoresNilSprites(t *testing.T) {
	l := NewLayer()
	l.Add(New(geometry.Vector2{X: 1, Y: 1}, 'A'))
	l.Add(nil)
	l.Add(New(geometry.Vector2{X: 2, Y: 1}, 'B'))

	fb := newFrame(t)
	l.Draw(fb)

	want := map[[2]int]rune{{1, 1}: 'A', {2, 1}: 'B'}
	got := drawn(fb)
	if len(got) != len(want) || got[[2]int{1, 1}] != 'A' || got[[2]int{2, 1}] != 'B' {
		t.Errorf("wanted %v, got %v", want, got)
	}
}

func TestLayerDrawsAfterABadSprite(t *testing.T) {
	l := NewLayer()
	l.Add(New(geometry.Vector2{X: math.NaN(), Y: 0}, 'N'))
	l.Add(New(geometry.Vector2{X: 3, Y: 2}, 'G'))

	fb := newFrame(t)
	l.Draw(fb)

	got := drawn(fb)
	if len(got) != 1 || got[[2]int{3, 2}] != 'G' {
		t.Errorf("wanted only G at (3, 2), got %v", got)
	}
}

func TestLayerLaterSpritesDrawOverEarlierOnes(t *testing.T) {
	l := NewLayer()
	l.Add(New(geometry.Vector2{X: 4, Y: 4}, 'a'))
	l.Add(New(geometry.Vector2{X: 4.5, Y: 4.5}, 'b'))

	fb := newFrame(t)
	l.Draw(fb)

	if got := fb.At(4, 4).Character; got != 'b' {
		t.Errorf("wanted b on top, got %q", got)
	}
}

func TestSpriteCellArrivesWhole(t *testing.T) {
	cell := engine.Cell{
		Character:  'x',
		Foreground: engine.Palette(208),
		Background: engine.RGB(10, 20, 30),
		Attrs:      engine.Bold | engine.Underline,
	}
	l := NewLayer()
	l.Add(&Sprite{Position: geometry.Vector2{X: 3, Y: 2}, Cell: cell})
	fb := newFrame(t)

	l.Draw(fb)

	if got := fb.At(3, 2); got != cell {
		t.Errorf("wanted %+v, got %+v", cell, got)
	}
}

func TestNewMakesAPlainCharacter(t *testing.T) {
	s := New(geometry.Vector2{X: 1, Y: 2}, 'q')

	if s.Cell != (engine.Cell{Character: 'q'}) || s.Position != (geometry.Vector2{X: 1, Y: 2}) {
		t.Errorf("wanted a q in the default colors at (1, 2), got %+v", *s)
	}
}

// A sprite is not see-through: the cell under it, background and attributes
// included, is replaced by the sprite's.
func TestSpriteReplacesTheWholeCellUnderIt(t *testing.T) {
	fb := newFrame(t)
	fb.Set(2, 2, engine.Cell{Character: '#', Foreground: engine.Palette(1), Background: engine.Palette(2), Attrs: engine.Reverse})
	l := NewLayer()
	l.Add(New(geometry.Vector2{X: 2, Y: 2}, 'x'))

	l.Draw(fb)

	if got, want := fb.At(2, 2), (engine.Cell{Character: 'x'}); got != want {
		t.Errorf("wanted %+v, got %+v", want, got)
	}
}

func TestSpriteWithOnlyABackgroundDrawsAColoredBlank(t *testing.T) {
	blank := engine.Cell{Background: engine.Palette(4)}
	l := NewLayer()
	l.Add(&Sprite{Position: geometry.Vector2{X: 1, Y: 1}, Cell: blank})
	l.Add(New(geometry.Vector2{X: 2, Y: 1}, 'a'))
	fb := newFrame(t)

	l.Draw(fb)

	if got := fb.At(1, 1); got != blank {
		t.Errorf("wanted %+v, got %+v", blank, got)
	}
	if got := fb.String(); got != "\n  a\n\n\n" {
		t.Errorf("wanted the blank to show as a space before the a, got %q", got)
	}
}

func TestLaterSpriteReplacesAnEarlierOneWhole(t *testing.T) {
	l := NewLayer()
	l.Add(&Sprite{Position: geometry.Vector2{X: 4, Y: 4}, Cell: engine.Cell{Character: 'a', Foreground: engine.Palette(1), Background: engine.Palette(2), Attrs: engine.Bold}})
	l.Add(New(geometry.Vector2{X: 4, Y: 4}, 'b'))
	fb := newFrame(t)

	l.Draw(fb)

	if got, want := fb.At(4, 4), (engine.Cell{Character: 'b'}); got != want {
		t.Errorf("wanted %+v, got %+v", want, got)
	}
}

// The zero Sprite is a blank cell at the top left, as it always was.
func TestZeroSpriteIsABlankAtTheOrigin(t *testing.T) {
	fb := newFrame(t)
	fb.Set(0, 0, engine.Cell{Character: '#'})
	l := NewLayer()
	l.Add(&Sprite{})

	l.Draw(fb)

	if got := fb.At(0, 0); got != (engine.Cell{}) {
		t.Errorf("wanted the zero Cell at the origin, got %+v", got)
	}
}

// plain returns a sprite showing r at (x, 0).
func plain(x float64, r rune) *Sprite {
	return New(geometry.Vector2{X: x}, r)
}

// rowZero is the first line of the drawn frame.
func rowZero(l *Layer, t *testing.T) string {
	fb := newFrame(t)
	l.Draw(fb)
	line, _, _ := strings.Cut(fb.String(), "\n")
	return line
}

func TestLayerLen(t *testing.T) {
	l := NewLayer()
	if l.Len() != 0 {
		t.Fatalf("wanted a new layer to be empty, got %d", l.Len())
	}
	a := plain(0, 'a')
	l.Add(a)
	l.Add(nil)
	l.Add(plain(1, 'b'))
	l.Add(a) // the same sprite again

	if got := l.Len(); got != 3 {
		t.Errorf("wanted 3 (nil ignored, a duplicate counted), got %d", got)
	}
}

func TestLayerRemove(t *testing.T) {
	a, b, c := plain(0, 'a'), plain(1, 'b'), plain(2, 'c')

	tests := []struct {
		name      string
		remove    *Sprite
		wantFound bool
		wantRow   string
	}{
		{"the first", a, true, " bc"},
		{"one in the middle", b, true, "a c"},
		{"the last", c, true, "ab"},
		{"one that is not in the layer", plain(0, 'z'), false, "abc"},
		{"nil", nil, false, "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLayer()
			l.Add(a)
			l.Add(b)
			l.Add(c)

			found := l.Remove(tt.remove)

			if found != tt.wantFound {
				t.Errorf("wanted Remove to report %v, got %v", tt.wantFound, found)
			}
			if got := rowZero(l, t); got != tt.wantRow {
				t.Errorf("wanted the row %q, got %q", tt.wantRow, got)
			}
		})
	}
}

func TestLayerRemoveTakesOutEveryTimeASpriteWasAdded(t *testing.T) {
	l := NewLayer()
	a := plain(0, 'a')
	l.Add(a)
	l.Add(plain(1, 'b'))
	l.Add(a)

	if !l.Remove(a) || l.Len() != 1 {
		t.Fatalf("wanted both a's gone and b left, got %d sprites", l.Len())
	}
	if l.Remove(a) {
		t.Errorf("wanted a second Remove to find nothing")
	}
}

// Removing one sprite must not change which of the others is on top.
func TestLayerRemoveKeepsTheDrawOrder(t *testing.T) {
	l := NewLayer()
	under, middle, over := plain(0, 'u'), plain(0, 'm'), plain(0, 'o')
	l.Add(under)
	l.Add(middle)
	l.Add(over)

	l.Remove(middle)
	if got := rowZero(l, t); got != "o" {
		t.Errorf("wanted the later sprite still on top, got %q", got)
	}
	l.Remove(over)
	if got := rowZero(l, t); got != "u" {
		t.Errorf("wanted the first sprite to show once the others are gone, got %q", got)
	}
}

func TestRemovedSpriteCanBeAddedBack(t *testing.T) {
	l := NewLayer()
	a := plain(3, 'a')
	l.Add(a)
	l.Remove(a)
	if got := rowZero(l, t); got != "" {
		t.Fatalf("wanted nothing drawn once removed, got %q", got)
	}

	l.Add(a)

	if got := rowZero(l, t); got != "   a" {
		t.Errorf("wanted a drawn again, got %q", got)
	}
}

func TestLayerClear(t *testing.T) {
	l := NewLayer()
	l.Add(plain(0, 'a'))
	l.Add(plain(1, 'b'))

	l.Clear()

	if l.Len() != 0 || rowZero(l, t) != "" {
		t.Errorf("wanted an empty layer that draws nothing, got %d sprites and %q", l.Len(), rowZero(l, t))
	}
	l.Add(plain(2, 'c'))
	if got := rowZero(l, t); got != "  c" {
		t.Errorf("wanted the layer usable again, got %q", got)
	}
	NewLayer().Clear() // an empty layer clears without trouble
}

// A removed sprite must not stay reachable through the layer's spare capacity.
func TestRemoveAndClearLetGoOfTheSprites(t *testing.T) {
	spare := func(l *Layer) []*Sprite { return l.sprites[len(l.sprites):cap(l.sprites)] }

	l := NewLayer()
	a, b, c := plain(0, 'a'), plain(1, 'b'), plain(2, 'c')
	l.Add(a)
	l.Add(b)
	l.Add(c)

	l.Remove(a)
	for i, sp := range spare(l) {
		if sp != nil {
			t.Errorf("after Remove: wanted spare slot %d empty, got %+v", i, *sp)
		}
	}
	l.Clear()
	for i, sp := range l.sprites[:cap(l.sprites)] {
		if sp != nil {
			t.Errorf("after Clear: wanted slot %d empty, got %+v", i, *sp)
		}
	}
}

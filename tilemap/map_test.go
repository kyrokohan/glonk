package tilemap

import (
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// The tiles the tests use. Each has the Kind of the rune it stands for in legend, which
// FromLines would fill in and keeps where it is already there, so that a tile it makes is
// equal to the fixture.
var (
	wall    = Tile{Cell: engine.Cell{Character: '#', Foreground: engine.Palette(1)}, Solid: true, Kind: '#'}
	floor   = Tile{Cell: engine.Cell{Character: '.', Foreground: engine.Palette(2), Background: engine.Palette(17)}, Kind: '.'}
	water   = Tile{Cell: engine.Cell{Character: '~', Foreground: engine.Palette(4)}, Solid: true, Kind: '~'}
	grass   = Tile{Cell: engine.Cell{Character: '"', Foreground: engine.Palette(10)}, Kind: '"'}
	window  = Tile{Cell: engine.Cell{Character: ' ', Attrs: engine.Reverse}, Kind: 'w'}         // a blank that shows: not a hole
	shade   = Tile{Cell: engine.Cell{Character: ' ', Background: engine.Palette(3)}, Kind: 's'} // another
	blank   = Tile{Cell: engine.Cell{Character: ' ', Foreground: engine.Palette(5)}, Kind: 'o'} // a space whose foreground is not seen: a hole
	nothing = Tile{}
)

func legend() map[rune]Tile {
	return map[rune]Tile{'#': wall, '.': floor, '~': water, '"': grass, 'w': window, 's': shade, 'o': blank, '_': nothing}
}

func newBuffer(t testing.TB, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatal(err)
	}
	return fb
}

func TestFromLinesMakesAGridOfTheTextArt(t *testing.T) {
	m := FromLines(legend(),
		"#####",
		"#.~.#",
		"#####",
	)
	if m.Width() != 5 || m.Height() != 3 || m.Bounds() != geometry.NewRect(0, 0, 5, 3) {
		t.Fatalf("wanted 5 by 3, got %d by %d (%v)", m.Width(), m.Height(), m.Bounds())
	}
	want := []string{"#####", "#.~.#", "#####"}
	for y, line := range want {
		for x, r := range line {
			if got := m.At(x, y); got != legend()[r] {
				t.Errorf("(%d, %d): wanted the tile of %q, got %+v", x, y, r, got)
			}
		}
	}
	if !m.Solid(0, 0) || m.Solid(1, 1) || !m.Solid(2, 1) {
		t.Errorf("wanted walls and water solid and floor not")
	}
}

func TestFromLinesPadsShortLinesAndTakesUnknownRunesAsTheEmptyTile(t *testing.T) {
	m := FromLines(legend(), "#.#.", "#", "", " ?x.", "é#日.")
	if m.Width() != 4 || m.Height() != 5 {
		t.Fatalf("wanted 4 by 5, got %d by %d", m.Width(), m.Height())
	}
	for _, c := range [][2]int{{1, 1}, {3, 1}, {0, 2}, {3, 2}, {0, 3}, {1, 3}, {2, 3}, {0, 4}, {2, 4}} {
		if got := m.At(c[0], c[1]); got != nothing {
			t.Errorf("(%d, %d): wanted the empty tile, got %+v", c[0], c[1], got)
		}
		if m.Solid(c[0], c[1]) {
			t.Errorf("(%d, %d): wanted the empty tile not to be solid", c[0], c[1])
		}
	}
	// The width is the longest line, not the first.
	if longer := FromLines(legend(), "#", "#.#."); longer.Width() != 4 || longer.At(3, 1) != floor || longer.At(3, 0) != nothing {
		t.Errorf("wanted a width of 4 from the longest line, got %d", longer.Width())
	}
	// Runes of any width are one cell each.
	withWide := FromLines(map[rune]Tile{'日': wall, 'é': floor}, "é日é")
	if withWide.Width() != 3 || withWide.At(1, 0) != wall || withWide.At(2, 0) != floor {
		t.Errorf("wanted a rune to be a cell however many bytes it takes, got %d wide, %+v", withWide.Width(), withWide.At(1, 0))
	}
	if m.At(3, 3) != floor || m.At(3, 4) != floor || m.At(1, 4) != wall {
		t.Errorf("wanted the legend's tiles where the text says")
	}
}

// bareLegend is a legend as a program writes it, with no Kind on any tile. X looks like a wall
// and is not one, and P is a marker with no look at all.
func bareLegend() map[rune]Tile {
	return map[rune]Tile{
		'#': {Cell: engine.Cell{Character: '#'}, Solid: true},
		'X': {Cell: engine.Cell{Character: '#'}, Solid: true},
		'.': {Cell: engine.Cell{Character: '.'}},
		'P': {},
	}
}

func TestFromLinesGivesEveryTileTheRuneThatMadeIt(t *testing.T) {
	lg := bareLegend()
	lines := []string{"#X.P", ".X#P", "PP.#"}
	m := FromLines(lg, lines...)
	for y, line := range lines {
		for x, r := range line {
			got := m.At(x, y)
			want := lg[r]
			want.Kind = r
			if got != want {
				t.Errorf("(%d, %d): wanted the tile of %q with its Kind, %+v, got %+v", x, y, r, want, got)
			}
		}
	}
}

func TestFromLinesLeavesWhatTheLegendDoesNotListWithNoKind(t *testing.T) {
	m := FromLines(bareLegend(), "#?.", "", " é#日", "P")
	for _, c := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {2, 1}, {0, 2}, {1, 2}, {3, 2}, {1, 3}, {2, 3}, {3, 3}} {
		if got := m.At(c[0], c[1]); got != nothing || got.Kind != 0 {
			t.Errorf("(%d, %d): wanted the zero Tile, got %+v", c[0], c[1], got)
		}
	}
	// A marker that is listed is not the zero Tile, though nothing about it is seen or solid.
	if got := m.At(0, 3); got == nothing || got.Kind != 'P' || got.Solid || got.Cell != (engine.Cell{}) {
		t.Errorf("wanted the listed marker P to be a see-through tile of kind P, got %+v", got)
	}
}

func TestFromLinesKeepsAKindThatTheLegendHas(t *testing.T) {
	crate := Tile{Cell: engine.Cell{Character: 'x'}, Solid: true, Kind: 'W'}
	m := FromLines(map[rune]Tile{'a': crate, 'b': crate, 'c': {Cell: engine.Cell{Character: 'x'}, Solid: true}}, "abc")
	if m.At(0, 0).Kind != 'W' || m.At(1, 0).Kind != 'W' {
		t.Errorf("wanted the Kind of the legend kept, got %q and %q", m.At(0, 0).Kind, m.At(1, 0).Kind)
	}
	if m.At(2, 0).Kind != 'c' {
		t.Errorf("wanted a tile with no Kind to get its rune, got %q", m.At(2, 0).Kind)
	}
	// Two runes that say the same Kind and look the same are one tile, as before: the empty
	// tile, the crate, and the third.
	if len(m.tiles) != 3 {
		t.Errorf("wanted 3 different tiles, got %d", len(m.tiles))
	}
}

// Runes whose tiles look alike are different tiles now, since the map can say which made them.
func TestRunesWithTilesThatLookAlikeAreDifferentTiles(t *testing.T) {
	m := FromLines(bareLegend(), "#X#X")
	if len(m.tiles) != 3 { // the empty tile, #, X
		t.Errorf("wanted 3 different tiles, got %d", len(m.tiles))
	}
	a, b := m.At(0, 0), m.At(1, 0)
	if a == b || a.Cell != b.Cell || a.Solid != b.Solid {
		t.Errorf("wanted tiles that look alike and differ in Kind, got %+v and %+v", a, b)
	}
	if m.At(2, 0) != a || m.At(3, 0) != b {
		t.Errorf("wanted every # to be one tile and every X another")
	}
}

func TestTilesThatDifferOnlyInKindAreDifferentTiles(t *testing.T) {
	look := engine.Cell{Character: '#'}
	a, b := Tile{Cell: look, Kind: 'a'}, Tile{Cell: look, Kind: 'b'}
	m := New(4, 1, nothing)
	m.Set(0, 0, a)
	m.Set(1, 0, b)
	m.Set(2, 0, a)
	if m.At(0, 0) != a || m.At(1, 0) != b || m.At(2, 0) != a || m.At(3, 0) != nothing {
		t.Errorf("wanted each cell to give back the tile Set in it, got %+v %+v %+v %+v", m.At(0, 0), m.At(1, 0), m.At(2, 0), m.At(3, 0))
	}
	if len(m.tiles) != 3 { // nothing, a, b: the second a is the first
		t.Errorf("wanted 3 different tiles, got %d", len(m.tiles))
	}
}

// Kind costs a cell nothing: a cell is still a two byte number naming a tile, and a map of a
// million cells of three kinds holds four tiles.
func TestACellStaysTwoBytesWhateverItsKind(t *testing.T) {
	row := strings.Repeat("#X.", 334)[:1000]
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = row
	}
	m := FromLines(bareLegend(), lines...)
	if size := reflect.TypeOf(m.ids).Elem().Size(); size != 2 {
		t.Errorf("wanted a cell to be 2 bytes, got %d", size)
	}
	if len(m.ids) != 1_000_000 || len(m.tiles) != 4 {
		t.Errorf("wanted 1,000,000 cells of 4 different tiles, got %d and %d", len(m.ids), len(m.tiles))
	}
}

func TestFromLinesOfNothingIsAnEmptyMap(t *testing.T) {
	for name, m := range map[string]*Map{
		"no lines":        FromLines(legend()),
		"empty lines":     FromLines(legend(), "", "", ""),
		"a nil legend":    FromLines(nil, "###"),
		"an empty legend": FromLines(map[rune]Tile{}, "###"),
	} {
		if name == "a nil legend" || name == "an empty legend" {
			if m.Width() != 3 || m.Height() != 1 || m.At(0, 0) != nothing {
				t.Errorf("%s: wanted 3 by 1 of the empty tile, got %d by %d", name, m.Width(), m.Height())
			}
			continue
		}
		if m.Width() != 0 || m.Height() != 0 || m.At(0, 0) != nothing || !m.Solid(0, 0) {
			t.Errorf("%s: wanted an empty map, got %d by %d", name, m.Width(), m.Height())
		}
	}
}

func TestFromLinesRefusesTooManyCells(t *testing.T) {
	row := strings.Repeat("#", 1000)
	lines := make([]string, 4200) // 4,200,000 cells: more than 4,194,304
	for i := range lines {
		lines[i] = row
	}
	if m := FromLines(legend(), lines...); m.Width() != 0 || m.Height() != 0 {
		t.Errorf("wanted an empty map for too many cells, got %d by %d", m.Width(), m.Height())
	}
	if m := FromLines(legend(), lines[:4000]...); m.Width() != 1000 || m.Height() != 4000 {
		t.Errorf("wanted 4,000,000 cells to be allowed, got %d by %d", m.Width(), m.Height())
	}
}

func TestFromLinesSharesOneTileAmongRunesThatHaveIt(t *testing.T) {
	same := map[rune]Tile{'a': wall, 'b': wall, 'c': floor}
	m := FromLines(same, "abcab")
	if len(m.tiles) != 3 { // the empty tile, the wall, the floor
		t.Errorf("wanted 3 different tiles, got %d", len(m.tiles))
	}
	if m.At(0, 0) != wall || m.At(1, 0) != wall || m.At(2, 0) != floor {
		t.Errorf("wanted runes with one tile to give it")
	}
}

func TestFromLinesPastTheMostTilesGivesTheEmptyTile(t *testing.T) {
	legend := map[rune]Tile{}
	var sb strings.Builder
	for i := range 70_000 {
		r := rune(0x2000 + i)
		legend[r] = Tile{Cell: engine.Cell{Character: r}}
		sb.WriteRune(r)
	}
	m := FromLines(legend, sb.String())
	if m.Width() != 70_000 {
		t.Fatalf("wanted 70000 columns, got %d", m.Width())
	}
	if len(m.tiles) != maxTiles {
		t.Errorf("wanted %d different tiles, got %d", maxTiles, len(m.tiles))
	}
	if got := m.At(100, 0); got.Cell.Character != 0x2000+100 {
		t.Errorf("wanted an early tile kept, got %+v", got)
	}
	if got := m.At(69_999, 0); got != nothing {
		t.Errorf("wanted a tile past the most to come out empty, got %+v", got)
	}
}

func TestNew(t *testing.T) {
	m := New(4, 3, floor)
	if m.Width() != 4 || m.Height() != 3 {
		t.Fatalf("wanted 4 by 3, got %d by %d", m.Width(), m.Height())
	}
	for y := range 3 {
		for x := range 4 {
			if m.At(x, y) != floor {
				t.Fatalf("(%d, %d): wanted every cell to be the fill", x, y)
			}
		}
	}

	big := 1 << 11
	for name, m := range map[string]*Map{
		"no columns":        New(0, 5, floor),
		"no rows":           New(5, 0, floor),
		"negative":          New(-3, 4, floor),
		"negative height":   New(3, -4, floor),
		"too many cells":    New(big, big+1, floor),
		"the largest sizes": New(math.MaxInt, math.MaxInt, floor),
		"one large side":    New(math.MaxInt, 2, floor),
	} {
		if m.Width() != 0 || m.Height() != 0 {
			t.Errorf("%s: wanted an empty map, got %d by %d", name, m.Width(), m.Height())
		}
	}
	if m := New(big, big, floor); m.Width() != big || m.Height() != big {
		t.Errorf("wanted the most cells to be allowed, got %d by %d", m.Width(), m.Height())
	}
}

func TestSetAtAndSolid(t *testing.T) {
	m := New(3, 2, floor)
	m.Set(1, 1, wall)
	m.Set(0, 0, water)
	if m.At(1, 1) != wall || m.At(0, 0) != water || m.At(2, 1) != floor {
		t.Errorf("wanted Set to change only its cell")
	}
	if !m.Solid(1, 1) || !m.Solid(0, 0) || m.Solid(2, 1) {
		t.Errorf("wanted solidity to follow the tile")
	}

	// A tile already in the map, the one it started as included, is reused, a new one is added.
	before := len(m.tiles)
	m.Set(2, 1, floor)
	if len(m.tiles) != before {
		t.Errorf("wanted the fill to be reused, tiles went from %d to %d", before, len(m.tiles))
	}
	m.Set(2, 0, wall)
	if len(m.tiles) != before {
		t.Errorf("wanted a known tile to be reused, tiles went from %d to %d", before, len(m.tiles))
	}
	m.Set(2, 0, grass)
	if len(m.tiles) != before+1 || m.At(2, 0) != grass {
		t.Errorf("wanted a new tile to be added, got %d tiles and %+v", len(m.tiles), m.At(2, 0))
	}

	// Outside is left alone, and is solid and the zero tile.
	for _, c := range [][2]int{{-1, 0}, {0, -1}, {3, 0}, {0, 2}, {math.MaxInt, 0}, {0, math.MinInt}, {math.MinInt, math.MinInt}} {
		m.Set(c[0], c[1], wall)
		if m.At(c[0], c[1]) != nothing || !m.Solid(c[0], c[1]) {
			t.Errorf("%v: wanted the zero tile and a wall outside the map", c)
		}
	}
	if m.At(0, 1) != floor || m.At(1, 0) != floor {
		t.Errorf("wanted nothing outside the map to reach inside it")
	}
}

func TestSetStopsAddingTilesAtTheMost(t *testing.T) {
	m := New(2, 1, floor)
	last := Tile{}
	for i := range 70_000 {
		tile := Tile{Cell: engine.Cell{Character: rune(0x2000 + i)}}
		m.Set(0, 0, tile)
		if len(m.tiles) < maxTiles || m.At(0, 0) == tile {
			last = tile
		}
	}
	if len(m.tiles) != maxTiles {
		t.Errorf("wanted %d different tiles, got %d", maxTiles, len(m.tiles))
	}
	if m.At(0, 0) != last {
		t.Errorf("wanted the cell to keep the last tile that fit, got %+v", m.At(0, 0))
	}
	m.Set(1, 0, wall) // no room for another kind: nothing happens
	if m.At(1, 0) != floor {
		t.Errorf("wanted a new tile with no room to leave the cell alone, got %+v", m.At(1, 0))
	}
	m.Set(1, 0, floor) // but a known one is fine
	if m.At(1, 0) != floor {
		t.Errorf("wanted a known tile to be set")
	}
}

func TestANilMapIsAMapOfNoCells(t *testing.T) {
	var m *Map
	fb := newBuffer(t, 4, 3)
	m.Draw(fb, geometry.Rect{})
	m.Set(0, 0, wall)
	if m.Width() != 0 || m.Height() != 0 || m.At(0, 0) != nothing || !m.Solid(0, 0) || m.Bounds() != (geometry.Rect{}) {
		t.Errorf("wanted a nil map to be empty and solid")
	}
	if got := fb.String(); strings.Trim(got, " \n") != "" {
		t.Errorf("wanted nothing drawn, got %q", got)
	}
}

func TestDrawShowsTheTilesInView(t *testing.T) {
	m := FromLines(legend(),
		"#####",
		"#.~.#",
		"#####",
	)
	tests := []struct {
		name string
		view geometry.Vector2
		want string // the 7 by 4 frame
	}{
		{"at the origin", geometry.Vector2{}, "#####  \n#.~.#  \n#####  \n       "},
		{"moved right by two", geometry.Vector2{X: 2}, "###    \n~.#    \n###    \n       "},
		{"moved down by one", geometry.Vector2{Y: 1}, "#.~.#  \n#####  \n       \n       "},
		{"a fraction does not move it", geometry.Vector2{X: 0.9, Y: 0.9}, "#####  \n#.~.#  \n#####  \n       "},
		{"left of the map: tiles to the right", geometry.Vector2{X: -2}, "  #####\n  #.~.#\n  #####\n       "},
		{"above the map", geometry.Vector2{Y: -1}, "       \n#####  \n#.~.#  \n#####  "},
		{"a little left: floored to the cell before", geometry.Vector2{X: -0.1}, " #####\n #.~.#\n #####\n       "},
		{"past the right edge", geometry.Vector2{X: 5}, "       \n       \n       \n       "},
		{"just inside the right edge", geometry.Vector2{X: 4}, "#      \n#      \n#      \n       "},
		{"far away", geometry.Vector2{X: 1e9, Y: -1e9}, "       \n       \n       \n       "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := newBuffer(t, 7, 4)
			m.Draw(fb, geometry.Rect{Min: tt.view})
			if got := fb.String(); stripTrailing(got) != stripTrailing(tt.want) {
				t.Errorf("wanted\n%s\ngot\n%s", tt.want, got)
			}
		})
	}
}

func stripTrailing(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

func TestDrawDoesNotDrawHolesAndKeepsWhatIsUnderThem(t *testing.T) {
	m := FromLines(legend(), "#_ows")
	fb := newBuffer(t, 5, 1)
	for x := range 5 {
		fb.Set(x, 0, engine.Cell{Character: 'x', Foreground: engine.Palette(5)})
	}
	m.Draw(fb, geometry.Rect{})

	// The two holes, the zero tile and a space, show what was there. A blank that shows, by
	// an attribute or a background, replaces it.
	for _, x := range []int{1, 2} {
		if got := fb.At(x, 0); got.Character != 'x' || got.Foreground != engine.Palette(5) {
			t.Errorf("(%d, 0): wanted a hole to leave the cell as it was, got %+v", x, got)
		}
	}
	if got := fb.At(3, 0); got != window.Cell {
		t.Errorf("wanted a blank with an attribute to be drawn, got %+v", got)
	}
	if got := fb.At(4, 0); got != shade.Cell {
		t.Errorf("wanted a blank with a background to be drawn, got %+v", got)
	}
	if got := fb.At(0, 0); got != wall.Cell {
		t.Errorf("wanted the wall drawn, got %+v", got)
	}
}

func TestOneMapDrawnOverAnother(t *testing.T) {
	ground := FromLines(legend(), ".....", ".....")
	roof := FromLines(legend(), "_#_#_", "_____")
	fb := newBuffer(t, 5, 2)
	ground.Draw(fb, geometry.Rect{})
	roof.Draw(fb, geometry.Rect{})
	if got := stripTrailing(fb.String()); got != ".#.#.\n....." {
		t.Errorf("wanted the roof over the ground, with the ground showing through its holes, got\n%s", got)
	}
}

// A reference that asks the map about every screen cell in turn, which is slow and shares
// nothing with the clipping Draw does.
func drawReference(m *Map, w, h int, backdrop []engine.Cell, view geometry.Vector2) []engine.Cell {
	out := append([]engine.Cell(nil), backdrop...)
	vx, vy := int(math.Floor(view.X)), int(math.Floor(view.Y))
	for sy := range h {
		for sx := range w {
			x, y := sx+vx, sy+vy
			if x < 0 || x >= m.Width() || y < 0 || y >= m.Height() {
				continue
			}
			if c := m.At(x, y).Cell; !isHole(c) {
				out[sy*w+sx] = c
			}
		}
	}
	return out
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

func randomCell(rng *rand.Rand) engine.Cell {
	return engine.Cell{Character: rune("ab#/ .xyz\x00"[rng.IntN(10)]), Foreground: engine.Palette(uint8(rng.IntN(4)))}
}

func TestDrawMatchesAskingTheMapAboutEveryCell(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	var drawn, partly int
	for range 3000 {
		mw, mh := 1+rng.IntN(12), 1+rng.IntN(8)
		m := New(mw, mh, Tile{})
		kinds := []Tile{{}, {Cell: randomCell(rng)}, {Cell: randomCell(rng), Solid: true}, window, wall, floor}
		for y := range mh {
			for x := range mw {
				m.Set(x, y, kinds[rng.IntN(len(kinds))])
			}
		}
		w, h := 1+rng.IntN(14), 1+rng.IntN(8)
		fb := newBuffer(t, w, h)
		for y := range h {
			for x := range w {
				fb.Set(x, y, randomCell(rng))
			}
		}
		backdrop := gridOf(fb)

		// Mostly overlapping the map, sometimes well clear of it.
		view := geometry.Vector2{X: rng.Float64()*float64(mw+w) - float64(w) + 1, Y: rng.Float64()*float64(mh+h) - float64(h) + 1}
		if rng.IntN(10) == 0 {
			view = geometry.Vector2{X: rng.Float64()*60 - 25, Y: rng.Float64()*40 - 15}
		}
		if rng.IntN(3) == 0 {
			view = geometry.Vector2{X: math.Floor(view.X), Y: math.Floor(view.Y)}
		}
		m.Draw(fb, geometry.Rect{Min: view})

		want := drawReference(m, w, h, backdrop, view)
		got := gridOf(fb)
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("map %dx%d, frame %dx%d, view %v: cell %d: wanted %+v, got %+v", mw, mh, w, h, view, i, want[i], got[i])
			}
		}
		for i := range want {
			if want[i] != backdrop[i] {
				drawn++
				break
			}
		}
		if view.X < 0 || view.Y < 0 || view.X+float64(w) > float64(mw) || view.Y+float64(h) > float64(mh) {
			partly++
		}
	}
	if drawn < 1500 || partly < 1500 {
		t.Errorf("wanted at least 1500 scenes that draw and 1500 that reach past the map's edge, got %d and %d", drawn, partly)
	}
}

func TestDrawLeavesEverythingForAViewThatIsNotANumber(t *testing.T) {
	m := FromLines(legend(), "#####", "#...#")
	nan, inf := math.NaN(), math.Inf(1)
	for _, v := range []geometry.Vector2{{X: nan}, {Y: nan}, {X: inf}, {X: -inf}, {Y: inf}, {Y: -inf}, {X: nan, Y: inf}, {X: math.MaxFloat64}, {Y: -math.MaxFloat64}, {X: 1e300, Y: 1e300}} {
		fb := newBuffer(t, 7, 4)
		before := gridOf(fb)
		m.Draw(fb, geometry.Rect{Min: v})
		for i, c := range gridOf(fb) {
			if c != before[i] {
				t.Fatalf("view %v: wanted nothing drawn", v)
			}
		}
	}
}

func TestDrawAllocatesNothing(t *testing.T) {
	m := New(60, 30, floor)
	for x := range 60 {
		m.Set(x, 3, wall)
	}
	fb := newBuffer(t, 40, 12)
	view := geometry.Rect{Min: geometry.Vector2{X: 7.5, Y: 2.5}}
	if n := testing.AllocsPerRun(50, func() { m.Draw(fb, view) }); n != 0 {
		t.Errorf("wanted no allocation, got %v", n)
	}
}

func FuzzFromLines(f *testing.F) {
	f.Add("#####\n#...#\n#####", "#.")
	f.Add("", "")
	f.Add("日本語\n\n é", "日é")
	f.Add(strings.Repeat("#", 5000), "#")
	f.Add("\x00\xff\xfe\n", "\x00")

	f.Fuzz(func(t *testing.T, text, runes string) {
		lg := map[rune]Tile{}
		for i, r := range runes {
			lg[r] = Tile{Cell: engine.Cell{Character: r}, Solid: i%2 == 0}
		}
		m := FromLines(lg, strings.Split(text, "\n")...)
		if m.Width() < 0 || m.Height() < 0 || m.Width()*m.Height() > maxCells {
			t.Fatalf("wanted a sane size, got %d by %d", m.Width(), m.Height())
		}
		if len(m.tiles) > maxTiles || len(m.ids) != m.Width()*m.Height() {
			t.Fatalf("wanted %d ids and at most %d tiles, got %d and %d", m.Width()*m.Height(), maxTiles, len(m.ids), len(m.tiles))
		}
		for y := range m.Height() {
			for x := range m.Width() {
				if m.At(x, y).Solid != m.Solid(x, y) {
					t.Fatalf("(%d, %d): wanted At and Solid to agree", x, y)
				}
			}
		}
		// Every cell the text lists is the legend's tile with its rune as Kind, and every other is
		// the zero Tile, unless the map ran out of room for tiles.
		if len(m.tiles) < maxTiles {
			for y, line := range strings.Split(text, "\n") {
				x := 0
				for _, r := range line {
					want, listed := lg[r]
					if listed {
						want.Kind = r
					}
					if got := m.At(x, y); got != want {
						t.Fatalf("(%d, %d) of %q: wanted %+v, got %+v", x, y, r, want, got)
					}
					x++
				}
			}
		}
		fb := newBuffer(t, 10, 4)
		m.Draw(fb, geometry.Rect{})
	})
}

func FuzzMapDraw(f *testing.F) {
	f.Add(uint8(5), uint8(3), 0.0, 0.0, uint8(10), uint8(4))
	f.Add(uint8(0), uint8(0), 1.5, -2.5, uint8(10), uint8(4))
	f.Add(uint8(9), uint8(9), math.NaN(), 0.0, uint8(3), uint8(3))
	f.Add(uint8(9), uint8(9), math.Inf(1), math.Inf(-1), uint8(3), uint8(3))
	f.Add(uint8(9), uint8(9), -4.0, 100.0, uint8(20), uint8(20))
	f.Add(uint8(200), uint8(200), 1e300, -1e300, uint8(1), uint8(1))

	f.Fuzz(func(t *testing.T, mw, mh uint8, vx, vy float64, fw, fh uint8) {
		w, h := 1+int(fw%24), 1+int(fh%12)
		m := New(int(mw%40), int(mh%20), Tile{})
		kinds := []Tile{{}, wall, floor, window}
		for y := range m.Height() {
			for x := range m.Width() {
				m.Set(x, y, kinds[(x*7+y*3)%len(kinds)])
			}
		}
		fb := newBuffer(t, w, h)
		before := gridOf(fb)
		m.Draw(fb, geometry.Rect{Min: geometry.Vector2{X: vx, Y: vy}})

		fx, fy := math.Floor(vx), math.Floor(vy)
		if !math.IsNaN(fx+fy) && math.Abs(fx) < 1e6 && math.Abs(fy) < 1e6 {
			want := drawReference(m, w, h, before, geometry.Vector2{X: vx, Y: vy})
			for i, c := range gridOf(fb) {
				if c != want[i] {
					t.Fatalf("view (%v, %v): cell %d: wanted %+v, got %+v", vx, vy, i, want[i], c)
				}
			}
		}
	})
}

func BenchmarkFromLines(b *testing.B) {
	rng := rand.New(rand.NewPCG(7, 8))
	lines := make([]string, 400)
	for i := range lines {
		row := make([]byte, 1000)
		for j := range row {
			row[j] = "#X."[rng.IntN(3)]
		}
		lines[i] = string(row)
	}
	lg := bareLegend()
	b.Run("1000 by 400 of three tiles", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			FromLines(lg, lines...)
		}
	})
}

func BenchmarkMapAt(b *testing.B) {
	m := FromLines(bareLegend(), strings.Repeat("#X.", 100), strings.Repeat(".X#", 100))
	b.Run("At and Solid", func(b *testing.B) {
		b.ReportAllocs()
		n := 0
		for b.Loop() {
			for x := range 300 {
				if m.At(x, 1).Solid || m.Solid(x, 0) {
					n++
				}
			}
		}
		_ = n
	})
}

func BenchmarkMapDraw(b *testing.B) {
	rng := rand.New(rand.NewPCG(5, 6))
	m := New(400, 200, floor)
	for range 8000 {
		m.Set(rng.IntN(400), rng.IntN(200), wall)
	}
	fb := newBuffer(b, 200, 60)
	view := geometry.Rect{Min: geometry.Vector2{X: 37.5, Y: 12.5}}
	b.Run("200 by 60 of a 400 by 200 map", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			m.Draw(fb, view)
		}
	})
}

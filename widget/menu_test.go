package widget

import (
	"math"
	"slices"
	"strings"
	"testing"

	"glonk.dev/engine"
)

// press runs one frame of Update with keys pressed, as a game would.
func press(m *Menu, keys ...engine.Key) (int, bool) {
	kb := engine.NewKeyboard()
	kb.Press(keys...)
	return m.Update(kb.Input())
}

// reversedRows returns the rows of fb whose cell in column x is reversed.
func reversedRows(fb *engine.FrameBuffer, x int) []int {
	var rows []int
	for y := range fb.Height() {
		if fb.At(x, y).Attrs.Has(engine.Reverse) {
			rows = append(rows, y)
		}
	}
	return rows
}

func TestMenuMovesAndWraps(t *testing.T) {
	tests := []struct {
		name  string
		start int
		keys  []engine.Key
		want  int
	}{
		{"down", 0, []engine.Key{engine.KeyDown}, 1},
		{"down wraps from the last item to the first", 2, []engine.Key{engine.KeyDown}, 0},
		{"up", 2, []engine.Key{engine.KeyUp}, 1},
		{"up wraps from the first item to the last", 0, []engine.Key{engine.KeyUp}, 2},
		{"up and down in one frame cancel", 1, []engine.Key{engine.KeyUp, engine.KeyDown}, 1},
		{"no key", 1, nil, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Menu{Items: []string{"a", "b", "c"}, Selected: tt.start}
			choice, chosen := press(&m, tt.keys...)
			if m.Selected != tt.want {
				t.Errorf("wanted Selected to be %d, got %d", tt.want, m.Selected)
			}
			if choice != -1 || chosen {
				t.Errorf("wanted nothing chosen, got (%d, %v)", choice, chosen)
			}
		})
	}
}

func TestMenuEnterChooses(t *testing.T) {
	m := Menu{Items: []string{"a", "b", "c"}, Selected: 2}
	choice, chosen := press(&m, engine.KeyEnter)
	if choice != 2 || !chosen {
		t.Errorf("wanted (2, true), got (%d, %v)", choice, chosen)
	}
	if m.Selected != 2 {
		t.Errorf("wanted choosing not to move the selection, got %d", m.Selected)
	}

	// The moves come before Enter, so Down and Enter in one frame choose the next item.
	m = Menu{Items: []string{"a", "b", "c"}, Selected: 0}
	choice, chosen = press(&m, engine.KeyDown, engine.KeyEnter)
	if choice != 1 || !chosen || m.Selected != 1 {
		t.Errorf("wanted (1, true) with Selected 1, got (%d, %v) with Selected %d", choice, chosen, m.Selected)
	}
}

func TestMenuAKeyHeldDownMovesOnce(t *testing.T) {
	m := Menu{Items: []string{"a", "b", "c"}}
	kb := engine.NewKeyboard()

	kb.Press(engine.KeyDown)
	m.Update(kb.Input())
	kb.EndFrame()
	// The key is still down, but it was not pressed again.
	m.Update(kb.Input())
	kb.EndFrame()

	if m.Selected != 1 {
		t.Errorf("wanted one move for one press, got Selected %d", m.Selected)
	}
}

func TestMenuLeavesOtherKeysToTheGame(t *testing.T) {
	m := Menu{Items: []string{"a", "b", "c"}, Selected: 1}
	keys := []engine.Key{engine.KeyLeft, engine.KeyRight, engine.KeyEscape, engine.KeyTab, engine.KeyBackspace, 'w', 's', ' ', 'k', 'j'}
	choice, chosen := press(&m, keys...)
	if m.Selected != 1 || choice != -1 || chosen {
		t.Errorf("wanted nothing to happen, got Selected %d and (%d, %v)", m.Selected, choice, chosen)
	}
}

func TestMenuWithNoItems(t *testing.T) {
	for _, items := range [][]string{nil, {}} {
		m := Menu{Items: items, Selected: 5}
		for _, key := range []engine.Key{engine.KeyDown, engine.KeyUp, engine.KeyEnter} {
			if choice, chosen := press(&m, key); choice != -1 || chosen {
				t.Errorf("wanted nothing chosen from no items, got (%d, %v)", choice, chosen)
			}
		}
		if m.Selected != 5 {
			t.Errorf("wanted Selected left as it was, got %d", m.Selected)
		}

		fb := frame(t, 6, 3)
		m.Draw(fb, 0, 0, 6, 3)
		sameLines(t, fb, "", "", "")
	}
}

func TestMenuSelectedThatIsOutOfRangeIsZero(t *testing.T) {
	for _, bad := range []int{-1, -3, 3, 99, math.MinInt, math.MaxInt} {
		tests := []struct {
			name string
			keys []engine.Key
			want int
		}{
			{"no key", nil, 0},
			{"down", []engine.Key{engine.KeyDown}, 1},
			{"up", []engine.Key{engine.KeyUp}, 2},
		}
		for _, tt := range tests {
			m := Menu{Items: []string{"a", "b", "c"}, Selected: bad}
			press(&m, tt.keys...)
			if m.Selected != tt.want {
				t.Errorf("wanted Selected %d after %s from %d, got %d", tt.want, tt.name, bad, m.Selected)
			}
		}

		m := Menu{Items: []string{"a", "b", "c"}, Selected: bad}
		if choice, chosen := press(&m, engine.KeyEnter); choice != 0 || !chosen {
			t.Errorf("wanted Enter from %d to choose item 0, got (%d, %v)", bad, choice, chosen)
		}
	}
}

func TestMenuDraw(t *testing.T) {
	m := Menu{Items: []string{"Resume", "Options", "Quit"}, Selected: 1}
	fb := frame(t, 10, 4)
	m.Draw(fb, 1, 0, 8, 3)
	sameLines(t, fb, " Resume", " Options", " Quit", "")

	// The selected row is reversed across the whole width, and only that.
	if got := reversedRows(fb, 1); !slices.Equal(got, []int{1}) {
		t.Errorf("wanted only row 1 reversed at its first column, got %v", got)
	}
	if got := reversedRows(fb, 8); !slices.Equal(got, []int{1}) {
		t.Errorf("wanted only row 1 reversed at its last column, got %v", got)
	}
	for _, x := range []int{0, 9} {
		if got := reversedRows(fb, x); got != nil {
			t.Errorf("wanted nothing outside the width reversed at column %d, got rows %v", x, got)
		}
	}
}

func TestMenuStyle(t *testing.T) {
	style := engine.Cell{Character: 'Z', Foreground: engine.Palette(2), Background: engine.Palette(4), Attrs: engine.Bold}
	m := Menu{Items: []string{"ab", "cd"}, Selected: 1, Style: style}
	fb := frame(t, 6, 2)
	m.Draw(fb, 0, 0, 4, 2)

	cell := func(r rune, attrs engine.Attr) engine.Cell {
		return engine.Cell{Character: r, Foreground: style.Foreground, Background: style.Background, Attrs: attrs}
	}
	checks := []struct {
		name string
		x, y int
		want engine.Cell
	}{
		{"unselected text", 0, 0, cell('a', engine.Bold)},
		{"unselected text, second cell", 1, 0, cell('b', engine.Bold)},
		{"unselected blank is left alone", 2, 0, engine.BlankCell},
		{"selected text keeps its attributes and adds Reverse", 0, 1, cell('c', engine.Bold|engine.Reverse)},
		{"selected blank is filled in the style", 2, 1, cell(' ', engine.Bold|engine.Reverse)},
		{"selected, last cell of the width", 3, 1, cell(' ', engine.Bold|engine.Reverse)},
		{"beyond the width", 4, 1, engine.BlankCell},
	}
	for _, c := range checks {
		if got := fb.At(c.x, c.y); got != c.want {
			t.Errorf("wanted %s, (%d, %d), to be %+v, got %+v", c.name, c.x, c.y, c.want, got)
		}
	}
}

func TestMenuWithAStyleThatIsAlreadyReversed(t *testing.T) {
	m := Menu{Items: []string{"a", "b"}, Selected: 1, Style: engine.Cell{Attrs: engine.Reverse}}
	fb := frame(t, 4, 2)
	m.Draw(fb, 0, 0, 4, 2)

	// The selected item is the one that is not reversed.
	if got := reversedRows(fb, 0); !slices.Equal(got, []int{0}) {
		t.Errorf("wanted only the unselected row reversed, got %v", got)
	}
	if got := fb.At(3, 1); got != (engine.Cell{Character: ' '}) {
		t.Errorf("wanted the selected blank to be plain, got %+v", got)
	}
}

func TestMenuCutsItemsToTheWidth(t *testing.T) {
	m := Menu{Items: []string{"abcdefghij", "xy"}}
	fb := frame(t, 10, 2)
	m.Draw(fb, 0, 0, 4, 2)
	sameLines(t, fb, "abcd", "xy")
}

func TestMenuScrollsToKeepTheSelectionInView(t *testing.T) {
	items := strings.Fields("0 1 2 3 4 5 6 7 8 9")
	tests := []struct {
		name     string
		h        int
		selected int
		want     []string
		row      int // which row is selected
	}{
		{"first item", 3, 0, []string{"0", "1", "2"}, 0},
		{"second item, still at the top", 3, 1, []string{"0", "1", "2"}, 1},
		{"third item, the middle row from here", 3, 2, []string{"1", "2", "3"}, 1},
		{"in the middle", 3, 5, []string{"4", "5", "6"}, 1},
		{"second to last, the list ends", 3, 8, []string{"7", "8", "9"}, 1},
		{"last item", 3, 9, []string{"7", "8", "9"}, 2},
		{"one row shows only the selection", 1, 4, []string{"4"}, 0},
		{"two rows, at the start", 2, 0, []string{"0", "1"}, 0},
		{"two rows, in the middle", 2, 3, []string{"2", "3"}, 1},
		{"two rows, at the end", 2, 9, []string{"8", "9"}, 1},
		{"as many rows as items", 10, 7, items, 7},
		{"more rows than items", 14, 7, items, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Menu{Items: items, Selected: tt.selected}
			fb := frame(t, 5, 12)
			m.Draw(fb, 0, 0, 5, tt.h)
			sameLines(t, fb, append(slices.Clone(tt.want), make([]string, 12-len(tt.want))...)...)
			if got := reversedRows(fb, 0); !slices.Equal(got, []int{tt.row}) {
				t.Errorf("wanted row %d selected, got %v", tt.row, got)
			}
		})
	}
}

func TestMenuDrawDoesNotChangeTheMenu(t *testing.T) {
	m := Menu{Items: []string{"a", "b", "c"}, Selected: 99}
	fb := frame(t, 4, 3)
	m.Draw(fb, 0, 0, 4, 3)

	if m.Selected != 99 || len(m.Items) != 3 {
		t.Errorf("wanted the menu as it was, got Selected %d and %d items", m.Selected, len(m.Items))
	}
	// An out of range selection is drawn as 0.
	if got := reversedRows(fb, 0); !slices.Equal(got, []int{0}) {
		t.Errorf("wanted item 0 drawn selected, got rows %v", got)
	}
}

func TestMenuOffTheFrame(t *testing.T) {
	tests := []struct {
		name       string
		x, y, w, h int
		want       []string
		reversed   []int // rows reversed in the first column of the menu that is on the frame
	}{
		{"cut at the left", -2, 0, 6, 3, []string{"cdef", "ijkl", "opqr"}, []int{0}},
		{"cut at the top", 0, -1, 6, 3, []string{"ghijkl", "mnopqr", ""}, nil},
		{"cut at the bottom", 0, 2, 6, 3, []string{"", "", "abcdef"}, []int{2}},
		{"cut at the right", 3, 0, 6, 3, []string{"   abc", "   ghi", "   mno"}, []int{0}},
		{"left of the frame", -6, 0, 6, 3, []string{"", "", ""}, nil},
		{"above the frame", 0, -3, 6, 3, []string{"", "", ""}, nil},
		{"below the frame", 0, 3, 6, 3, []string{"", "", ""}, nil},
		{"no width", 0, 0, 0, 3, []string{"", "", ""}, nil},
		{"no height", 0, 0, 6, 0, []string{"", "", ""}, nil},
		{"negative size", 0, 0, -6, -3, []string{"", "", ""}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Menu{Items: []string{"abcdef", "ghijkl", "mnopqr"}}
			fb := frame(t, 6, 3)
			m.Draw(fb, tt.x, tt.y, tt.w, tt.h)
			sameLines(t, fb, tt.want...)
			if got := reversedRows(fb, max(tt.x, 0)); !slices.Equal(got, tt.reversed) {
				t.Errorf("wanted rows %v reversed, got %v", tt.reversed, got)
			}
		})
	}
}

func TestMenuWithHugeNumbers(t *testing.T) {
	big := []int{math.MinInt, math.MinInt + 1, -1, 0, 1, math.MaxInt - 1, math.MaxInt}
	items := strings.Fields("a bb ccc dddd eeeee")
	for _, x := range big {
		for _, y := range big {
			for _, w := range big {
				for _, h := range big {
					for sel := range 5 {
						m := Menu{Items: items, Selected: sel}
						m.Draw(frame(t, 4, 3), x, y, w, h)
					}
				}
			}
		}
	}

	// A list that starts on the frame and has more room than any frame fits as it is.
	m := Menu{Items: items}
	fb := frame(t, 6, 6)
	m.Draw(fb, 0, 0, math.MaxInt, math.MaxInt)
	sameLines(t, fb, "a", "bb", "ccc", "dddd", "eeeee", "")
}

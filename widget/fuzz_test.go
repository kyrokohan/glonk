package widget

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"glonk.dev/engine"
)

// The fuzz targets draw on a frame of this size, every cell of which starts as sentinel, a
// cell that no widget draws, so that a cell a widget touched outside its area shows.
const fuzzW, fuzzH = 12, 5

var sentinel = engine.Cell{Character: '.', Foreground: engine.Palette(7)}

// sentinelFrame returns a frame of sentinel cells.
func sentinelFrame(t testing.TB) *engine.FrameBuffer {
	fb := frame(t, fuzzW, fuzzH)
	fill(fb, sentinel)
	return fb
}

// untouchedOutside fails unless every cell of fb for which inside is false is still sentinel.
func untouchedOutside(t testing.TB, what string, fb *engine.FrameBuffer, inside func(cx, cy int) bool) {
	t.Helper()
	for cy := range fuzzH {
		for cx := range fuzzW {
			if !inside(cx, cy) && fb.At(cx, cy) != sentinel {
				t.Fatalf("wanted %s to leave (%d, %d), outside its area, untouched, got %+v", what, cx, cy, fb.At(cx, cy))
			}
		}
	}
}

// within reports whether c is one of the n cells from start, without overflowing: the
// distance from start to c is worked out as an unsigned number, which cannot wrap.
func within(c, start, n int) bool {
	return n > 0 && c >= start && uint64(c)-uint64(start) < uint64(n)
}

// FuzzDraw draws each widget at any position and size, with any text and value. It must not
// panic or run long, and must leave every cell outside the widget's area exactly as it was.
func FuzzDraw(f *testing.F) {
	f.Add(0, 0, 10, 4, 0.5, "Title", uint8(1), '.')
	f.Add(-3, -1, 6, 3, 0.25, "Status", uint8(2), ' ')
	f.Add(2, 1, 7, 3, 1.0, "abc", uint8(0), '#')
	f.Add(0, 0, math.MaxInt, math.MaxInt, 0.5, "Title", uint8(1), '.')
	f.Add(math.MinInt, math.MinInt, math.MaxInt, math.MaxInt, math.NaN(), "e\u0301", uint8(2), rune(0))
	f.Add(math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt, math.Inf(1), "中", uint8(9), rune(0x1b))
	f.Add(-1, -1, 2, 2, -1.0, "", uint8(1), '.')
	f.Add(11, 4, math.MaxInt/2, math.MaxInt/2, 1-1e-16, "\xff\u200d", uint8(2), '.')

	f.Fuzz(func(t *testing.T, x, y, w, h int, value float64, text string, align uint8, fill rune) {
		where := fmt.Sprintf("x=%d y=%d w=%d h=%d", x, y, w, h)
		style := engine.Cell{Character: 'Z', Foreground: engine.Palette(1), Background: engine.Palette(2), Attrs: engine.Bold}
		inRow := func(cx, cy int) bool { return cy == y && within(cx, x, w) }

		fb := sentinelFrame(t)
		Label{Text: text, Align: Align(align), Style: style}.Draw(fb, x, y, w)
		untouchedOutside(t, "a label at "+where, fb, inRow)

		fb = sentinelFrame(t)
		Panel{Title: text, Style: style, Fill: engine.Cell{Character: fill}}.Draw(fb, x, y, w, h)
		untouchedOutside(t, "a panel at "+where, fb, func(cx, cy int) bool { return within(cx, x, w) && within(cy, y, h) })

		fb = sentinelFrame(t)
		Bar{Value: value, Full: engine.Cell{Character: '#'}, Empty: engine.Cell{Character: '-'}}.Draw(fb, x, y, w)
		untouchedOutside(t, "a bar at "+where, fb, inRow)

		// A bar fills from the left: no filled cell after an empty one.
		if y >= 0 && y < fuzzH {
			seenEmpty := false
			for cx := range fuzzW {
				switch fb.At(cx, y).Character {
				case '-':
					seenEmpty = true
				case '#':
					if seenEmpty {
						t.Fatalf("wanted a bar filled from the left, got a filled cell at %d after an empty one (value=%v %s)", cx, value, where)
					}
				}
			}
		}
	})
}

// FuzzMenu runs a menu on any items, selection, keys, position and size. The selection must
// stay an item, a choice must come only from Enter, Draw must leave the menu and every cell
// outside its area as they were, and nothing may panic or run long.
func FuzzMenu(f *testing.F) {
	f.Add("Resume\nOptions\nQuit", 1, 1, 0, 8, 3, uint8(2))
	f.Add("a\nb\nc\nd\ne\nf\ng", 3, -2, -1, 6, 4, uint8(4))
	f.Add("", 0, 0, 0, 6, 3, uint8(7))
	f.Add("only", -9, 0, 0, 6, 1, uint8(1))
	f.Add("one\ntwo", math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt, uint8(3))
	f.Add("one\ntwo", math.MinInt, math.MinInt, math.MinInt, math.MaxInt, math.MaxInt, uint8(6))
	f.Add("x\u0301\n中\n\xff", 2, 3, 2, 4, 5, uint8(2))

	f.Fuzz(func(t *testing.T, text string, selected, x, y, w, h int, keys uint8) {
		var items []string
		if text != "" {
			items = strings.Split(text, "\n")
		}
		m := Menu{Items: items, Selected: selected, Style: engine.Cell{Foreground: engine.Palette(1), Background: engine.Palette(2)}}

		kb := engine.NewKeyboard()
		for bit, key := range []engine.Key{engine.KeyUp, engine.KeyDown, engine.KeyEnter} {
			if keys&(1<<bit) != 0 {
				kb.Press(key)
			}
		}
		choice, chosen := m.Update(kb.Input())
		enter := keys&4 != 0
		switch {
		case len(items) == 0:
			if chosen || choice != -1 {
				t.Fatalf("wanted nothing chosen from no items, got (%d, %v)", choice, chosen)
			}
		case m.Selected < 0 || m.Selected >= len(items):
			t.Fatalf("wanted Selected to be one of %d items, got %d", len(items), m.Selected)
		case chosen != enter:
			t.Fatalf("wanted a choice exactly when Enter was pressed (%v), got chosen %v", enter, chosen)
		case chosen && choice != m.Selected, !chosen && choice != -1:
			t.Fatalf("wanted the choice to be the selected item, or -1, got (%d, %v) with Selected %d", choice, chosen, m.Selected)
		}

		before := m.Selected
		where := fmt.Sprintf("x=%d y=%d w=%d h=%d", x, y, w, h)
		fb := sentinelFrame(t)
		m.Draw(fb, x, y, w, h)
		untouchedOutside(t, "a menu at "+where, fb, func(cx, cy int) bool { return within(cx, x, w) && within(cy, y, h) })
		if m.Selected != before || len(m.Items) != len(items) {
			t.Fatalf("wanted Draw not to change the menu, Selected went from %d to %d", before, m.Selected)
		}
	})
}

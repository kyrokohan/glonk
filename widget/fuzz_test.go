package widget

import (
	"math"
	"testing"

	"glonk.dev/engine"
)

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
		const fw, fh = 12, 5
		sentinel := engine.Cell{Character: '.', Foreground: engine.Palette(7)}
		fresh := func() *engine.FrameBuffer {
			fb := frame(t, fw, fh)
			for cy := range fh {
				for cx := range fw {
					fb.Set(cx, cy, sentinel)
				}
			}
			return fb
		}
		untouchedOutside := func(name string, fb *engine.FrameBuffer, inside func(cx, cy int) bool) {
			t.Helper()
			for cy := range fh {
				for cx := range fw {
					if !inside(cx, cy) && fb.At(cx, cy) != sentinel {
						t.Fatalf("wanted %s to leave (%d, %d), outside its area, untouched, got %+v (x=%d y=%d w=%d h=%d)", name, cx, cy, fb.At(cx, cy), x, y, w, h)
					}
				}
			}
		}
		style := engine.Cell{Character: 'Z', Foreground: engine.Palette(1), Background: engine.Palette(2), Attrs: engine.Bold}
		inRow := func(cx, cy int) bool { return cy == y && within(cx, x, w) }

		fb := fresh()
		Label{Text: text, Align: Align(align), Style: style}.Draw(fb, x, y, w)
		untouchedOutside("a label", fb, inRow)

		fb = fresh()
		Panel{Title: text, Style: style, Fill: engine.Cell{Character: fill}}.Draw(fb, x, y, w, h)
		untouchedOutside("a panel", fb, func(cx, cy int) bool { return within(cx, x, w) && within(cy, y, h) })

		fb = fresh()
		Bar{Value: value, Full: engine.Cell{Character: '#'}, Empty: engine.Cell{Character: '-'}}.Draw(fb, x, y, w)
		untouchedOutside("a bar", fb, inRow)

		// A bar fills from the left: no filled cell after an empty one.
		if y >= 0 && y < fh {
			seenEmpty := false
			for cx := range fw {
				switch fb.At(cx, y).Character {
				case '-':
					seenEmpty = true
				case '#':
					if seenEmpty {
						t.Fatalf("wanted a bar filled from the left, got a filled cell at %d after an empty one (value=%v x=%d w=%d)", cx, value, x, w)
					}
				}
			}
		}
	})
}

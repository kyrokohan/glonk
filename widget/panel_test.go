package widget

import (
	"math"
	"testing"

	"glonk.dev/engine"
)

func TestPanelBorderAndTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		x     int
		w, h  int
		want  []string
	}{
		{"with a title", "Menu", 1, 10, 4, []string{
			" ┌─ Menu ─┐",
			" │        │",
			" │        │",
			" └────────┘",
		}},
		{"a long title is cut to fit", "Long title", 0, 10, 3, []string{
			"┌─ Long ─┐",
			"│        │",
			"└────────┘",
		}},
		{"the smallest panel with a title", "abc", 0, 7, 2, []string{
			"┌─ a ─┐",
			"└─────┘",
		}},
		{"no room for a title", "abc", 0, 6, 2, []string{
			"┌────┐",
			"└────┘",
		}},
		{"no title", "", 0, 8, 2, []string{
			"┌──────┐",
			"└──────┘",
		}},
		{"a title with nothing to show", "\u0301\u200b", 0, 8, 2, []string{
			"┌──────┐",
			"└──────┘",
		}},
		{"the smallest panel", "x", 0, 2, 2, []string{
			"┌┐",
			"└┘",
		}},
		{"two wide", "", 0, 2, 4, []string{
			"┌┐",
			"││",
			"││",
			"└┘",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := frame(t, 12, tt.h)
			Panel{Title: tt.title}.Draw(fb, tt.x, 0, tt.w, tt.h)
			sameLines(t, fb, tt.want...)
		})
	}
}

func TestPanelSmallerThanTwoByTwoDrawsNothing(t *testing.T) {
	for _, size := range [][2]int{{1, 5}, {5, 1}, {0, 0}, {-3, 4}, {4, -3}, {1, 1}} {
		fb := frame(t, 6, 6)
		fill(fb, engine.Cell{Character: 'x'})
		Panel{Title: "t", Fill: engine.Cell{Character: '.'}}.Draw(fb, 1, 1, size[0], size[1])
		if got := fb.At(2, 2); got.Character != 'x' {
			t.Errorf("wanted a %d by %d panel to draw nothing, got %+v at (2, 2)", size[0], size[1], got)
		}
	}
}

func TestPanelFill(t *testing.T) {
	tests := []struct {
		name string
		fill engine.Cell
		want []string
	}{
		{"a hole leaves the inside alone", engine.Cell{}, []string{"┌───┐", "│xxx│", "└───┘"}},
		{"a blank with a foreground is still a hole", engine.Cell{Character: ' ', Foreground: engine.Palette(2)}, []string{"┌───┐", "│xxx│", "└───┘"}},
		{"a character", engine.Cell{Character: '.'}, []string{"┌───┐", "│...│", "└───┘"}},
		{"a blank with a background is drawn", engine.Cell{Character: ' ', Background: engine.Palette(3)}, []string{"┌───┐", "│   │", "└───┘"}},
		{"a blank with an attribute is drawn", engine.Cell{Attrs: engine.Reverse}, []string{"┌───┐", "│   │", "└───┘"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := frame(t, 5, 3)
			fill(fb, engine.Cell{Character: 'x'})
			Panel{Fill: tt.fill}.Draw(fb, 0, 0, 5, 3)
			sameLines(t, fb, tt.want...)
		})
	}

	fb := frame(t, 5, 3)
	Panel{Fill: engine.Cell{Character: ' ', Background: engine.Palette(3)}}.Draw(fb, 0, 0, 5, 3)
	for x := 1; x <= 3; x++ {
		if got := fb.At(x, 1).Background; got != engine.Palette(3) {
			t.Errorf("wanted the background of (%d, 1) to be the fill's, got %+v", x, got)
		}
	}
	if got := fb.At(0, 1).Background; !got.IsDefault() {
		t.Errorf("wanted the border not to take the fill's background, got %+v", got)
	}
}

func TestPanelStyleIsTheBorderAndTheTitle(t *testing.T) {
	style := engine.Cell{Character: 'Z', Foreground: engine.Palette(2), Background: engine.Palette(4), Attrs: engine.Bold}
	fb := frame(t, 10, 3)
	Panel{Title: "ab", Style: style, Fill: engine.Cell{Character: '.'}}.Draw(fb, 0, 0, 10, 3)

	with := func(r rune) engine.Cell {
		c := style
		c.Character = r
		return c
	}
	checks := []struct {
		x, y int
		want engine.Cell
	}{
		{0, 0, with('┌')}, {9, 0, with('┐')}, {0, 2, with('└')}, {9, 2, with('┘')},
		{2, 0, with(' ')}, {3, 0, with('a')}, {4, 0, with('b')}, {5, 0, with(' ')}, {6, 0, with('─')},
		{0, 1, with('│')}, {9, 1, with('│')}, {3, 2, with('─')},
		{4, 1, engine.Cell{Character: '.'}},
	}
	for _, c := range checks {
		if got := fb.At(c.x, c.y); got != c.want {
			t.Errorf("wanted (%d, %d) to be %+v, got %+v", c.x, c.y, c.want, got)
		}
	}
}

func TestPanelOffTheFrame(t *testing.T) {
	tests := []struct {
		name       string
		x, y, w, h int
		want       []string
	}{
		{"cut at the left and the top", -2, -1, 6, 3, []string{"   │", "───┘", ""}},
		{"cut at the right and the bottom", 3, 1, 6, 4, []string{"", "   ┌──", "   │"}},
		{"cut at the left only", -3, 0, 6, 3, []string{"──┐", "  │", "──┘"}},
		{"only the corner is on the frame", 5, 2, 4, 4, []string{"", "", "     ┌"}},
		{"only the far corner is on the frame", -3, -3, 4, 4, []string{"┘", "", ""}},
		{"left of the frame", -6, 0, 6, 3, []string{"", "", ""}},
		{"right of the frame", 6, 0, 6, 3, []string{"", "", ""}},
		{"above the frame", 0, -3, 6, 3, []string{"", "", ""}},
		{"below the frame", 0, 3, 6, 3, []string{"", "", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := frame(t, 6, 3)
			Panel{}.Draw(fb, tt.x, tt.y, tt.w, tt.h)
			sameLines(t, fb, tt.want...)
		})
	}
}

func TestPanelTitleCutByTheFrame(t *testing.T) {
	// The title starts 3 cells in, at -2, so its first two letters are off the frame.
	fb := frame(t, 8, 2)
	Panel{Title: "Status"}.Draw(fb, -5, 0, 14, 2)
	sameLines(t, fb, "atus ───", "────────")
}

func TestPanelWithHugeNumbers(t *testing.T) {
	big := []int{math.MinInt, math.MinInt + 1, -1, 0, 1, 3, math.MaxInt - 1, math.MaxInt}
	for _, x := range big {
		for _, y := range big {
			for _, w := range big {
				for _, h := range big {
					fb := frame(t, 4, 3)
					Panel{Title: "title", Fill: engine.Cell{Character: '.'}}.Draw(fb, x, y, w, h)
				}
			}
		}
	}

	// A panel that starts on the frame and goes on forever draws its top and left edges, and
	// the inside, and nothing of the far edges.
	fb := frame(t, 4, 3)
	Panel{Fill: engine.Cell{Character: '.'}}.Draw(fb, 0, 0, math.MaxInt, math.MaxInt)
	sameLines(t, fb, "┌───", "│...", "│...")

	// Starting a little in: the far corner's sum, x+w-1, would wrap around to a negative
	// number if it were added plainly, and the panel would vanish.
	fb = frame(t, 4, 3)
	Panel{Fill: engine.Cell{Character: '.'}}.Draw(fb, 2, 1, math.MaxInt, math.MaxInt)
	sameLines(t, fb, "", "  ┌─", "  │.")

	// One that starts far off to the left and ends before the frame does not touch it.
	fb = frame(t, 4, 3)
	Panel{Fill: engine.Cell{Character: '.'}}.Draw(fb, math.MinInt, 0, math.MaxInt, 3)
	sameLines(t, fb, "", "", "")

	// Wrapping around must not bring one back: far right, far below.
	fb = frame(t, 4, 3)
	Panel{}.Draw(fb, math.MaxInt-1, math.MaxInt-1, math.MaxInt, math.MaxInt)
	sameLines(t, fb, "", "", "")
}

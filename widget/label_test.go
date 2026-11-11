package widget

import (
	"math"
	"testing"

	"glonk.dev/engine"
)

func TestLabelAlignment(t *testing.T) {
	tests := []struct {
		name  string
		align Align
		text  string
		want  string
	}{
		{"left", Left, "ab", "ab"},
		{"center", Center, "ab", "    ab"},
		{"center puts the extra cell of an odd gap on the right", Center, "abc", "   abc"},
		{"right", Right, "ab", "        ab"},
		{"a number that is not an alignment is left", Align(9), "ab", "ab"},
		{"exact fit", Center, "abcdefghij", "abcdefghij"},
		{"cut to the width, left", Left, "abcdefghijkl", "abcdefghij"},
		{"cut to the width, center", Center, "abcdefghijkl", "abcdefghij"},
		{"cut to the width, right", Right, "abcdefghijkl", "abcdefghij"},
		{"empty", Center, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := frame(t, 10, 1)
			Label{Text: tt.text, Align: tt.align}.Draw(fb, 0, 0, 10)
			sameLines(t, fb, tt.want)
		})
	}
}

func TestLabelStaysInItsWidth(t *testing.T) {
	fb := frame(t, 12, 3)
	Label{Text: "ab", Align: Right}.Draw(fb, 2, 1, 6)
	sameLines(t, fb, "", "      ab", "")

	fb = frame(t, 12, 3)
	Label{Text: "abcdefghij", Align: Left}.Draw(fb, 2, 1, 4)
	sameLines(t, fb, "", "  abcd", "")
}

func TestLabelCountsCellsLikeDrawText(t *testing.T) {
	// An accent that follows its letter has no cell: one cell wide, so it fits in 1.
	fb := frame(t, 4, 1)
	Label{Text: "e\u0301x"}.Draw(fb, 0, 0, 1)
	sameLines(t, fb, "e")

	// A wide character is one cell drawn as '?'.
	fb = frame(t, 4, 1)
	Label{Text: "中中"}.Draw(fb, 0, 0, 1)
	sameLines(t, fb, "?")

	// Centering uses the cells, not the runes: "cafe" and an accent is 4 cells in 10.
	fb = frame(t, 10, 1)
	Label{Text: "cafe\u0301", Align: Center}.Draw(fb, 0, 0, 10)
	sameLines(t, fb, "   cafe")
}

func TestLabelStyle(t *testing.T) {
	style := engine.Cell{Character: 'Z', Foreground: engine.Palette(2), Background: engine.Palette(4), Attrs: engine.Bold | engine.Underline}
	fb := frame(t, 6, 3)
	Label{Text: "hi", Style: style}.Draw(fb, 1, 1, 4)

	for i, r := range "hi" {
		want := engine.Cell{Character: r, Foreground: style.Foreground, Background: style.Background, Attrs: style.Attrs}
		if got := fb.At(1+i, 1); got != want {
			t.Errorf("wanted cell %d of the label to be %+v, got %+v", i, want, got)
		}
	}
	for _, p := range [][2]int{{0, 1}, {3, 1}, {5, 1}, {1, 0}, {1, 2}} {
		if got := fb.At(p[0], p[1]); got != engine.BlankCell {
			t.Errorf("wanted (%d, %d), outside the text, to be untouched, got %+v", p[0], p[1], got)
		}
	}
}

func TestLabelWithOnlyAForegroundDrawsOnlyThat(t *testing.T) {
	fb := frame(t, 4, 1)
	fb.Set(0, 0, engine.Cell{Character: 'x', Background: engine.Palette(5), Attrs: engine.Dim})
	Label{Text: "a", Style: engine.Cell{Foreground: engine.Palette(3)}}.Draw(fb, 0, 0, 4)

	want := engine.Cell{Character: 'a', Foreground: engine.Palette(3)}
	if got := fb.At(0, 0); got != want {
		t.Errorf("wanted the label to replace the cell as DrawText does, %+v, got %+v", want, got)
	}
}

func TestLabelOffTheFrame(t *testing.T) {
	tests := []struct {
		name  string
		fw    int
		x, w  int
		text  string
		align Align
		want  string
	}{
		{"cut at the left", 6, -3, 8, "hello", Left, "lo"},
		{"cut at the right", 6, 4, 8, "hello", Left, "    he"},
		{"centered, then cut at the left", 6, -2, 10, "abcd", Center, " abcd"},
		{"entirely left of the frame", 6, -10, 5, "hello", Left, ""},
		{"entirely right of the frame", 6, 6, 5, "hello", Left, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := frame(t, tt.fw, 1)
			Label{Text: tt.text, Align: tt.align}.Draw(fb, tt.x, 0, tt.w)
			sameLines(t, fb, tt.want)
		})
	}

	fb := frame(t, 6, 2)
	Label{Text: "hello"}.Draw(fb, 0, -1, 5)
	Label{Text: "hello"}.Draw(fb, 0, 2, 5)
	sameLines(t, fb, "", "")
}

func TestLabelWidthThatIsNotPositiveDrawsNothing(t *testing.T) {
	for _, w := range []int{0, -1, math.MinInt} {
		fb := frame(t, 6, 1)
		Label{Text: "hello", Style: engine.Cell{Background: engine.Palette(1)}}.Draw(fb, 0, 0, w)
		if got := fb.String(); got != "" {
			t.Errorf("wanted nothing drawn for a width of %d, got %q", w, got)
		}
		if got := fb.At(0, 0); got != engine.BlankCell {
			t.Errorf("wanted the cell untouched for a width of %d, got %+v", w, got)
		}
	}
}

func TestLabelWithHugeNumbers(t *testing.T) {
	big := []int{math.MinInt, math.MinInt + 1, -1, 0, 1, math.MaxInt - 1, math.MaxInt}
	for _, align := range []Align{Left, Center, Right} {
		for _, x := range big {
			for _, w := range big {
				fb := frame(t, 6, 1)
				Label{Text: "hello", Align: align, Style: engine.Cell{Background: engine.Palette(1)}}.Draw(fb, x, 0, w)
			}
		}
	}

	// A label that starts on the frame and has more room than any frame draws as left
	// aligned; centered or right aligned, it is far off the frame to the right.
	fb := frame(t, 6, 1)
	Label{Text: "hi"}.Draw(fb, 0, 0, math.MaxInt)
	sameLines(t, fb, "hi")

	fb = frame(t, 6, 1)
	Label{Text: "hi", Align: Center}.Draw(fb, 0, 0, math.MaxInt)
	Label{Text: "hi", Align: Right}.Draw(fb, 0, 0, math.MaxInt)
	sameLines(t, fb, "")
}

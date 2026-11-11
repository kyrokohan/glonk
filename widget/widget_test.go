package widget

import (
	"math"
	"strings"
	"testing"

	"glonk.dev/engine"
)

// frame returns a blank w by h frame.
func frame(t testing.TB, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatal(err)
	}
	return fb
}

// fill sets every cell of fb to c.
func fill(fb *engine.FrameBuffer, c engine.Cell) {
	for y := range fb.Height() {
		for x := range fb.Width() {
			fb.Set(x, y, c)
		}
	}
}

// sameLines fails unless fb reads as lines, which have no trailing spaces, as String has none.
func sameLines(t *testing.T, fb *engine.FrameBuffer, lines ...string) {
	t.Helper()
	if got, want := fb.String(), strings.Join(lines, "\n"); got != want {
		t.Errorf("wanted\n%s\ngot\n%s", want, got)
	}
}

func TestClip(t *testing.T) {
	tests := []struct {
		name string
		text string
		w    int
		want string
	}{
		{"fits", "abc", 5, "abc"},
		{"exact", "abc", 3, "abc"},
		{"cut", "abcdef", 3, "abc"},
		{"nothing fits", "abc", 0, ""},
		{"empty", "", 4, ""},
		{"a mark stays with its character", "e\u0301x", 1, "e\u0301"},
		{"a mark after a full line stays too", "ab\u0301c", 2, "ab\u0301"},
		{"a wide character is one cell", "中中中", 2, "中中"},
		{"invalid bytes are a cell each", "\xff\xfe\xfd", 2, "\xff\xfe"},
		{"only marks", "\u0301\u0301", 1, "\u0301\u0301"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clip(tt.text, tt.w); got != tt.want {
				t.Errorf("wanted clip(%q, %d) = %q, got %q", tt.text, tt.w, tt.want, got)
			}
		})
	}
}

func TestReachHoldsAtTheLargestInt(t *testing.T) {
	tests := []struct {
		name     string
		start, n int
		want     int
	}{
		{"small", 3, 4, 7},
		{"negative start", -10, 4, -6},
		{"zero", 5, 0, 5},
		{"most negative start", math.MinInt, math.MaxInt, -1},
		{"would overflow", math.MaxInt - 1, 5, math.MaxInt},
		{"exactly the largest", math.MaxInt - 5, 5, math.MaxInt},
		{"both largest", math.MaxInt, math.MaxInt, math.MaxInt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reach(tt.start, tt.n); got != tt.want {
				t.Errorf("wanted reach(%d, %d) = %d, got %d", tt.start, tt.n, tt.want, got)
			}
		})
	}
}

func TestIsHole(t *testing.T) {
	tests := []struct {
		name string
		cell engine.Cell
		want bool
	}{
		{"zero cell", engine.Cell{}, true},
		{"space", engine.Cell{Character: ' '}, true},
		{"blank with a foreground", engine.Cell{Character: ' ', Foreground: engine.Palette(1)}, true},
		{"blank with a background", engine.Cell{Character: ' ', Background: engine.Palette(1)}, false},
		{"blank with an attribute", engine.Cell{Attrs: engine.Reverse}, false},
		{"a character", engine.Cell{Character: '.'}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHole(tt.cell); got != tt.want {
				t.Errorf("wanted isHole(%+v) = %v, got %v", tt.cell, tt.want, got)
			}
		})
	}
}

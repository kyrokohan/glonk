package widget

import (
	"math"
	"testing"

	"glonk.dev/engine"
)

func TestBarFill(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		w     int
		want  string
	}{
		{"empty", 0, 8, "░░░░░░░░"},
		{"quarter", 0.25, 8, "██░░░░░░"},
		{"half", 0.5, 8, "████░░░░"},
		{"full", 1, 8, "████████"},
		{"rounds down", 0.34, 10, "███░░░░░░░"},
		{"rounds up", 0.36, 10, "████░░░░░░"},
		{"a little is nothing", 0.04, 10, "░░░░░░░░░░"},
		{"almost full is full", 0.96, 10, "██████████"},
		{"below zero", -3, 8, "░░░░░░░░"},
		{"above one", 7, 8, "████████"},
		{"negative infinity", math.Inf(-1), 8, "░░░░░░░░"},
		{"infinity", math.Inf(1), 8, "████████"},
		{"not a number", math.NaN(), 8, "░░░░░░░░"},
		{"negative zero", math.Copysign(0, -1), 8, "░░░░░░░░"},
		{"one cell, half", 0.5, 1, "█"},
		{"one cell, less than half", 0.49, 1, "░"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := frame(t, 12, 1)
			Bar{Value: tt.value}.Draw(fb, 0, 0, tt.w)
			sameLines(t, fb, tt.want)
		})
	}
}

func TestBarCells(t *testing.T) {
	full := engine.Cell{Character: '#', Foreground: engine.Palette(2)}
	empty := engine.Cell{Character: ' ', Background: engine.Palette(8)}

	fb := frame(t, 6, 1)
	Bar{Value: 0.5, Full: full, Empty: empty}.Draw(fb, 0, 0, 6)
	for x := range 6 {
		want := empty
		if x < 3 {
			want = full
		}
		if got := fb.At(x, 0); got != want {
			t.Errorf("wanted cell %d to be %+v, got %+v", x, want, got)
		}
	}

	// Only one of the two set: the other is still the default.
	fb = frame(t, 4, 1)
	Bar{Value: 0.5, Full: full}.Draw(fb, 0, 0, 4)
	if got := fb.At(3, 0); got != (engine.Cell{Character: '░'}) {
		t.Errorf("wanted the default empty cell, got %+v", got)
	}
	fb = frame(t, 4, 1)
	Bar{Value: 0.5, Empty: empty}.Draw(fb, 0, 0, 4)
	if got := fb.At(0, 0); got != (engine.Cell{Character: '█'}) {
		t.Errorf("wanted the default full cell, got %+v", got)
	}
}

func TestBarDrawsEveryCellAsGiven(t *testing.T) {
	// Unlike a panel's fill, a blank empty cell is not a hole: it replaces what is under it.
	fb := frame(t, 4, 1)
	fill(fb, engine.Cell{Character: 'x'})
	Bar{Value: 0.5, Full: engine.Cell{Character: '#'}, Empty: engine.Cell{Character: ' ', Foreground: engine.Palette(1)}}.Draw(fb, 0, 0, 4)
	sameLines(t, fb, "##")
}

func TestBarStaysInItsWidth(t *testing.T) {
	fb := frame(t, 10, 3)
	Bar{Value: 1}.Draw(fb, 2, 1, 5)
	sameLines(t, fb, "", "  █████", "")
}

func TestBarOffTheFrame(t *testing.T) {
	tests := []struct {
		name  string
		fw    int
		x, w  int
		value float64
		want  string
	}{
		{"cut at the left, in the filled part", 6, -3, 8, 0.75, "███░░"},
		{"cut at the left, past the filled part", 6, -6, 8, 0.25, "░░"},
		{"cut at the right", 6, 4, 8, 0.75, "    ██"},
		{"entirely left", 6, -8, 8, 1, ""},
		{"entirely right", 6, 6, 8, 1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := frame(t, tt.fw, 1)
			Bar{Value: tt.value}.Draw(fb, tt.x, 0, tt.w)
			sameLines(t, fb, tt.want)
		})
	}

	fb := frame(t, 6, 2)
	Bar{Value: 1}.Draw(fb, 0, -1, 6)
	Bar{Value: 1}.Draw(fb, 0, 2, 6)
	sameLines(t, fb, "", "")
}

func TestBarWidthThatIsNotPositiveDrawsNothing(t *testing.T) {
	for _, w := range []int{0, -1, math.MinInt} {
		fb := frame(t, 4, 1)
		Bar{Value: 1}.Draw(fb, 0, 0, w)
		sameLines(t, fb, "")
	}
}

func TestBarWithHugeNumbers(t *testing.T) {
	big := []int{math.MinInt, math.MinInt + 1, -1, 0, 1, math.MaxInt - 1, math.MaxInt}
	for _, x := range big {
		for _, w := range big {
			for _, v := range []float64{0, 0.5, 1 - 1e-16, 1} {
				fb := frame(t, 4, 1)
				Bar{Value: v}.Draw(fb, x, 0, w)
			}
		}
	}

	// More room than any frame: the filled part is half of that, which is off the frame.
	fb := frame(t, 4, 1)
	Bar{Value: 0.5}.Draw(fb, 0, 0, math.MaxInt)
	sameLines(t, fb, "████")

	fb = frame(t, 4, 1)
	Bar{Value: 0.0000001}.Draw(fb, 0, 0, math.MaxInt)
	sameLines(t, fb, "████")

	// Starting a little in: x+w would wrap around to a negative number if it were added
	// plainly, and the bar would vanish.
	fb = frame(t, 4, 1)
	Bar{Value: 0.5}.Draw(fb, 2, 0, math.MaxInt)
	sameLines(t, fb, "  ██")

	// Starting far to the left, the bar ends before the frame begins.
	fb = frame(t, 4, 1)
	Bar{Value: 1}.Draw(fb, math.MinInt, 0, math.MaxInt)
	sameLines(t, fb, "")
}

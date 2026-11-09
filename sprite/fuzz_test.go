package sprite

import (
	"math"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// Drawing a sprite anywhere must not panic, and must touch exactly the cell the
// position names, and nothing when the position is not on the frame.
func FuzzLayerDraw(f *testing.F) {
	f.Add(1.9, 2.1, 'x', uint8(1), uint8(2), uint16(3))
	f.Add(-0.1, 0.0, 'x', uint8(0), uint8(0), uint16(0))
	f.Add(7.999, 3.999, rune(0), uint8(255), uint8(255), uint16(0xffff))
	f.Add(8.0, 4.0, 'x', uint8(1), uint8(1), uint16(1))
	f.Add(math.NaN(), 1.0, 'x', uint8(1), uint8(1), uint16(1))
	f.Add(1.0, math.Inf(1), 'x', uint8(1), uint8(1), uint16(1))
	f.Add(1e300, -1e300, rune(0x1b), uint8(1), uint8(1), uint16(1))
	f.Add(math.MaxFloat64, math.SmallestNonzeroFloat64, '中', uint8(9), uint8(9), uint16(9))

	f.Fuzz(func(t *testing.T, x, y float64, ch rune, fg, bg uint8, attrs uint16) {
		const w, h = 8, 4
		fb, err := engine.NewFrameBuffer(w, h)
		if err != nil {
			t.Fatal(err)
		}
		cell := engine.Cell{Character: ch, Foreground: engine.Palette(fg), Background: engine.Palette(bg), Attrs: engine.Attr(attrs)}
		l := NewLayer()
		l.Add(&Sprite{Position: geometry.Vector2{X: x, Y: y}, Cell: cell})

		l.Draw(fb)

		fx, fy := math.Floor(x), math.Floor(y)
		onFrame := fx >= 0 && fx < w && fy >= 0 && fy < h // false for NaN
		for cy := range h {
			for cx := range w {
				want := engine.BlankCell
				if onFrame && float64(cx) == fx && float64(cy) == fy {
					want = cell
				}
				if got := fb.At(cx, cy); got != want {
					t.Fatalf("sprite at (%v, %v): wanted %+v at (%d, %d), got %+v", x, y, want, cx, cy, got)
				}
			}
		}
	})
}

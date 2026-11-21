package bitmap

import (
	"math"
	"math/rand/v2"
	"sync"
	"testing"

	"glonk.dev/engine"
)

// halvedReference is the copy of pic at half its size, worked out apart from the code under test:
// each pixel from the up to four under it, with floats, a hole where more than half are holes.
func halvedReference(pic *Bitmap) [][]engine.Color {
	w, h := pic.Width(), pic.Height()
	out := make([][]engine.Color, (h+1)/2)
	for y := range out {
		out[y] = make([]engine.Color, (w+1)/2)
		for x := range out[y] {
			var sum [3]float64
			n, holes := 0, 0
			for _, d := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				sx, sy := 2*x+d[0], 2*y+d[1]
				if sx >= w || sy >= h {
					continue
				}
				c := pic.Shown(sx, sy)
				if c.IsDefault() {
					holes++
					continue
				}
				r, g, b := c.RGB()
				sum[0], sum[1], sum[2] = sum[0]+float64(r), sum[1]+float64(g), sum[2]+float64(b)
				n++
			}
			if float64(holes) > float64(holes+n)/2 {
				continue
			}
			mean := func(s float64) uint8 { return uint8(math.Floor(s/float64(n) + 0.5)) }
			out[y][x] = engine.RGB(mean(sum[0]), mean(sum[1]), mean(sum[2]))
		}
	}
	return out
}

// randomPicture is a w by h picture of RGB and palette colors with holes in it.
func randomPicture(rng *rand.Rand, w, h int) *Bitmap {
	pic := New(w, h)
	for y := range h {
		for x := range w {
			switch rng.IntN(4) {
			case 0: // a hole
			case 1:
				pic.Set(x, y, engine.Palette(uint8(rng.IntN(256))))
			default:
				pic.Set(x, y, engine.RGB(uint8(rng.IntN(256)), uint8(rng.IntN(256)), uint8(rng.IntN(256))))
			}
		}
	}
	return pic
}

// Every copy is the reference halving of the one before, for pictures of every shape, odd and
// even, through views with a look, mirrored, and a part, down to 1 by 1.
func TestHalvedIsTheReferenceAtEveryLevel(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	for _, size := range [][2]int{{1, 1}, {2, 1}, {1, 2}, {2, 2}, {3, 3}, {5, 2}, {7, 1}, {1, 9}, {8, 8}, {13, 6}, {16, 16}} {
		base := randomPicture(rng, size[0]+2, size[1]+2)
		for name, pic := range map[string]*Bitmap{
			"a picture": base,
			"a part":    base.Sub(1, 1, size[0], size[1]),
			"mirrored":  base.Mirrored(),
			"shaded":    base.Shaded(0.4),
			"tinted":    base.Sub(1, 1, size[0], size[1]).Tinted(engine.RGB(255, 255, 255), 0.5),
		} {
			if pic.Halved(0) != pic || pic.Halved(-2) != pic {
				t.Fatalf("%s %v: wanted Halved(0) and below to be the picture", name, size)
			}
			if pic.Width() == 1 && pic.Height() == 1 {
				if pic.Halved(1) != pic {
					t.Fatalf("%s %v: wanted a picture of 1 by 1 to be its own smallest copy", name, size)
				}
				continue
			}
			prev := pic
			for n := 1; ; n++ {
				got := pic.Halved(n)
				want := halvedReference(prev)
				if got.Width() != len(want[0]) || got.Height() != len(want) {
					t.Fatalf("%s %v, level %d: wanted %d by %d, got %d by %d", name, size, n, len(want[0]), len(want), got.Width(), got.Height())
				}
				for y := range want {
					for x := range want[y] {
						if got.At(x, y) != want[y][x] {
							t.Fatalf("%s %v, level %d, pixel (%d, %d): wanted %v, got %v", name, size, n, x, y, want[y][x], got.At(x, y))
						}
					}
				}
				if got.Width() == 1 && got.Height() == 1 {
					if pic.Halved(n+5) != got {
						t.Fatalf("%s %v: wanted a level past the smallest to be the smallest", name, size)
					}
					break
				}
				prev = got
			}
		}
	}
}

// The rule written out: a hole where most are holes, kept at two of four, and the mean of what is
// there, palette colors as their RGB.
func TestHalvedRule(t *testing.T) {
	red, blue := engine.RGB(200, 0, 0), engine.RGB(0, 0, 100)
	pic := func(w int, colors ...engine.Color) *Bitmap {
		p := New(w, (len(colors)+w-1)/w)
		for i, c := range colors {
			if !c.IsDefault() {
				p.Set(i%w, i/w, c)
			}
		}
		return p
	}
	hole := engine.Color{}
	for _, tt := range []struct {
		name string
		pic  *Bitmap
		want engine.Color
	}{
		{"four", pic(2, red, red, blue, blue), engine.RGB(100, 0, 50)},
		{"two holes of four are kept", pic(2, red, hole, hole, blue), engine.RGB(100, 0, 50)},
		{"three holes of four are a hole", pic(2, red, hole, hole, hole), hole},
		{"four holes", pic(2, hole, hole, hole, hole), hole},
		{"one hole of two, at an edge, is kept", pic(1, red, hole), red},
		{"both of two are a hole", pic(1, hole, hole), hole},
		{"palette colors are their RGB", pic(2, engine.Palette(196), engine.Palette(21)), engine.RGB(128, 0, 128)},
		{"a mean is rounded to the nearest", pic(2, engine.RGB(1, 0, 0), engine.RGB(2, 0, 0)), engine.RGB(2, 0, 0)},
	} {
		if got := tt.pic.Halved(1).At(0, 0); got != tt.want {
			t.Errorf("%s: wanted %v, got %v", tt.name, tt.want, got)
		}
	}
}

// Set on a picture, or on any view of it, makes the copies of every view again; Set on a copy does
// nothing.
func TestHalvedCopiesFollowSet(t *testing.T) {
	red, green := engine.RGB(200, 0, 0), engine.RGB(0, 200, 0)
	pic := New(4, 4)
	for y := range 4 {
		for x := range 4 {
			pic.Set(x, y, red)
		}
	}
	part := pic.Sub(0, 0, 2, 2)
	if pic.Halved(1).At(0, 0) != red || part.Halved(1).At(0, 0) != red {
		t.Fatalf("wanted red copies to start with")
	}
	for _, p := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		part.Set(p[0], p[1], green) // through the view
	}
	if got := pic.Halved(1).At(0, 0); got != green {
		t.Errorf("wanted the picture's copy made again after a view was set, got %v", got)
	}
	if got := part.Halved(1).At(0, 0); got != green {
		t.Errorf("wanted the view's copy made again, got %v", got)
	}
	copied := pic.Halved(1)
	copied.Set(0, 0, red)
	if got := copied.At(0, 0); got != green {
		t.Errorf("wanted Set to do nothing on a copy, got %v", got)
	}
	if copied.Halved(1) != copied {
		t.Errorf("wanted a copy to have no copies of its own")
	}
	var none *Bitmap
	if none.Halved(1) != nil || New(0, 0).Halved(1).Width() != 0 {
		t.Errorf("wanted nothing to halve in a nil or an empty picture")
	}
}

// Many goroutines asking for the copies at once get the same ones, with no race between them.
func TestHalvedFromManyGoroutines(t *testing.T) {
	pic := randomPicture(rand.New(rand.NewPCG(1, 2)), 64, 48)
	view := pic.Tinted(engine.RGB(0, 0, 255), 0.3)
	var wg sync.WaitGroup
	got := make([][2]*Bitmap, 16)
	for i := range got {
		wg.Go(func() {
			got[i] = [2]*Bitmap{pic.Halved(3), view.Halved(2)}
		})
	}
	wg.Wait()
	for i := range got {
		for k := range 2 {
			a, b := got[i][k], got[0][k]
			if a.Width() != b.Width() || a.Height() != b.Height() {
				t.Fatalf("goroutine %d: wanted the same copy", i)
			}
			for y := range a.Height() {
				for x := range a.Width() {
					if a.At(x, y) != b.At(x, y) {
						t.Fatalf("goroutine %d: wanted the same copy, pixel (%d, %d) differs", i, x, y)
					}
				}
			}
		}
	}
}

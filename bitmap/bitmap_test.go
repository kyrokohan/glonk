package bitmap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"math/big"
	"slices"
	"testing"

	"glonk.dev/engine"
)

func encodePNG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("unable to encode a PNG: %v", err)
	}
	return buf.Bytes()
}

func encodeJPEG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatalf("unable to encode a JPEG: %v", err)
	}
	return buf.Bytes()
}

// pngHeader is the start of a PNG that says it is w by h pixels of 8-bit RGBA and has
// nothing else: all that is needed to say how big the picture is.
func pngHeader(w, h uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6 // bit depth, color type RGBA

	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	binary.Write(&buf, binary.BigEndian, uint32(len(ihdr)))
	body := append([]byte("IHDR"), ihdr...)
	buf.Write(body)
	binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(body))
	return buf.Bytes()
}

// The picture the drawing tests use, 3 by 2, as colors: five opaque ones, black among them,
// and one see-through pixel at (1, 0).
var (
	red    = engine.RGB(220, 40, 40)
	green  = engine.RGB(40, 200, 80)
	blue   = engine.RGB(40, 60, 220)
	yellow = engine.RGB(230, 210, 40)
	black  = engine.RGB(0, 0, 0)
	grey   = engine.RGB(90, 90, 90) // what the drawing tests put on the canvas first

	testColors = [2][3]engine.Color{{red, {}, green}, {blue, yellow, black}}
)

// testPicture returns testColors decoded from a PNG.
func testPicture(t testing.TB) *Bitmap {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for y := range 2 {
		for x := range 3 {
			r, g, b := testColors[y][x].RGB()
			a := uint8(255)
			if testColors[y][x].IsDefault() {
				r, g, b, a = 77, 88, 99, 0 // a see-through pixel can hold any color; it is not used
			}
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: a})
		}
	}
	pic, err := Decode(bytes.NewReader(encodePNG(t, img)))
	if err != nil {
		t.Fatalf("unable to decode the test picture: %v", err)
	}
	return pic
}

// canvasOf returns the pixel layer of a new frame w cells wide and h tall, w pixels wide and
// 2h tall, every pixel the color bg.
func canvasOf(t testing.TB, w, h int, bg engine.Color) *engine.Canvas {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatal(err)
	}
	c := fb.Canvas()
	c.FillRect(0, 0, c.Width(), c.Height(), bg)
	return c
}

// pixelsOf returns every pixel of c in row order.
func pixelsOf(c *engine.Canvas) []engine.Color {
	out := make([]engine.Color, 0, c.Width()*c.Height())
	for y := range c.Height() {
		for x := range c.Width() {
			out = append(out, c.At(x, y))
		}
	}
	return out
}

func TestDecodeKeepsEveryPixelOfAPNG(t *testing.T) {
	pic := testPicture(t)
	if pic.Width() != 3 || pic.Height() != 2 {
		t.Fatalf("wanted a picture of 3 by 2, got %d by %d", pic.Width(), pic.Height())
	}
	for y := range 2 {
		for x := range 3 {
			if got := pic.At(x, y); got != testColors[y][x] {
				t.Errorf("pixel (%d, %d): wanted %v, got %v", x, y, testColors[y][x], got)
			}
		}
	}
	// Black is a color, not the lack of one.
	if got := pic.At(2, 1); got.IsDefault() || got != engine.RGB(0, 0, 0) {
		t.Errorf("wanted black to be a pixel, got %v", got)
	}
}

func TestAlphaOfHalfOrMoreIsDrawnAsAnOpaqueColor(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 6, 1))
	for x, a := range []uint8{0, 1, 127, 128, 200, 255} {
		img.SetNRGBA(x, 0, color.NRGBA{R: 50, G: 100, B: 150, A: a})
	}
	pic, err := Decode(bytes.NewReader(encodePNG(t, img)))
	if err != nil {
		t.Fatal(err)
	}
	for x, drawn := range []bool{false, false, false, true, true, true} {
		got := pic.At(x, 0)
		if drawn && got != engine.RGB(50, 100, 150) {
			t.Errorf("alpha %d: wanted the color, as it is, got %v", []uint8{0, 1, 127, 128, 200, 255}[x], got)
		}
		if !drawn && !got.IsDefault() {
			t.Errorf("alpha %d: wanted nothing, got %v", []uint8{0, 1, 127, 128, 200, 255}[x], got)
		}
	}
}

// Every kind of PNG the standard library writes, with colors worked out by hand.
func TestDecodeReadsEveryKindOfPNG(t *testing.T) {
	gray := image.NewGray(image.Rect(0, 0, 2, 1))
	gray.SetGray(0, 0, color.Gray{Y: 0})
	gray.SetGray(1, 0, color.Gray{Y: 200})

	rgba := image.NewRGBA(image.Rect(0, 0, 2, 1)) // premultiplied
	rgba.SetRGBA(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	rgba.SetRGBA(1, 0, color.RGBA{R: 0, G: 0, B: 0, A: 0})

	pal := image.NewPaletted(image.Rect(0, 0, 3, 1), color.Palette{color.NRGBA{R: 1, G: 2, B: 3, A: 255}, color.NRGBA{A: 0}, color.NRGBA{R: 9, G: 9, B: 9, A: 100}})
	pal.SetColorIndex(0, 0, 0)
	pal.SetColorIndex(1, 0, 1)
	pal.SetColorIndex(2, 0, 2)

	nrgba64 := image.NewNRGBA64(image.Rect(0, 0, 2, 1))
	nrgba64.SetNRGBA64(0, 0, color.NRGBA64{R: 0xFFFF, G: 0x8000, B: 0, A: 0xFFFF})
	nrgba64.SetNRGBA64(1, 0, color.NRGBA64{R: 0xFFFF, G: 0xFFFF, B: 0xFFFF, A: 0x1000})

	rgba64 := image.NewRGBA64(image.Rect(0, 0, 1, 1))
	rgba64.SetRGBA64(0, 0, color.RGBA64{R: 0x4000, G: 0x8000, B: 0xC000, A: 0xFFFF})

	gray16 := image.NewGray16(image.Rect(0, 0, 2, 1))
	gray16.SetGray16(0, 0, color.Gray16{Y: 0x8000})
	gray16.SetGray16(1, 0, color.Gray16{Y: 0xFFFF})

	tests := []struct {
		name string
		img  image.Image
		want []engine.Color
	}{
		{"gray", gray, []engine.Color{engine.RGB(0, 0, 0), engine.RGB(200, 200, 200)}},
		{"rgba, premultiplied", rgba, []engine.Color{engine.RGB(10, 20, 30), {}}},
		{"paletted, with transparency", pal, []engine.Color{engine.RGB(1, 2, 3), {}, {}}},
		{"nrgba64", nrgba64, []engine.Color{engine.RGB(255, 128, 0), {}}},
		{"rgba64", rgba64, []engine.Color{engine.RGB(64, 128, 192)}},
		{"gray16", gray16, []engine.Color{engine.RGB(128, 128, 128), engine.RGB(255, 255, 255)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pic, err := Decode(bytes.NewReader(encodePNG(t, tt.img)))
			if err != nil {
				t.Fatal(err)
			}
			var got []engine.Color
			for x := range pic.Width() {
				got = append(got, pic.At(x, 0))
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

func TestDecodeReadsAJPEGWithEveryPixelDrawn(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 8))
	for y := range 8 {
		for x := range 16 {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 60, B: 20, A: 255})
		}
	}
	pic, err := Decode(bytes.NewReader(encodeJPEG(t, img)))
	if err != nil {
		t.Fatal(err)
	}
	if pic.Width() != 16 || pic.Height() != 8 {
		t.Fatalf("wanted 16 by 8, got %d by %d", pic.Width(), pic.Height())
	}
	for y := range 8 {
		for x := range 16 {
			p := pic.At(x, y)
			if p.IsDefault() {
				t.Fatalf("wanted every pixel of a JPEG drawn, (%d, %d) is not", x, y)
			}
			// A JPEG is lossy: the color comes back close, not exact.
			r, g, b := p.RGB()
			if abs(int(r)-200) > 6 || abs(int(g)-60) > 6 || abs(int(b)-20) > 6 {
				t.Fatalf("wanted (%d, %d) close to 200, 60, 20, got %d, %d, %d", x, y, r, g, b)
			}
		}
	}
}

// intsFrom returns those of vs that an int can hold, so that a list of sizes around 2^31 and
// beyond can be written once and be only what fits on a platform where an int is 32 bits.
func intsFrom(vs ...int64) []int {
	var out []int
	for _, v := range vs {
		if v >= math.MinInt && v <= math.MaxInt {
			out = append(out, int(v))
		}
	}
	return out
}

// aboveInt32 is a size over 2^31 where an int can be that large, and the largest int where not.
const aboveInt32 = int(min(int64(math.MaxInt), 1<<31+5))

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestDecodeRefusesWhatIsNotAPicture(t *testing.T) {
	good := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 8, 8)))
	tests := map[string][]byte{
		"empty":               {},
		"text":                []byte("not a picture"),
		"GIF":                 []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"BMP":                 []byte("BM\x1a\x00\x00\x00\x00\x00\x00\x00\x1a\x00\x00\x00"),
		"PNG signature alone": []byte("\x89PNG\r\n\x1a\n"),
		"JPEG marker alone":   {0xFF, 0xD8},
		"PNG, no pixels wide": pngHeader(0, 5),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			pic, err := Decode(bytes.NewReader(data))
			if !errors.Is(err, ErrFormat) || pic != nil {
				t.Errorf("wanted ErrFormat and no picture, got %v and %v", err, pic)
			}
		})
	}

	// A picture whose header is sound and the rest is not has another error, and is refused.
	corrupt := bytes.Clone(good)
	corrupt[len(corrupt)/2] ^= 0xFF
	for name, data := range map[string][]byte{
		"PNG cut after IHDR": good[:33],
		"PNG cut short":      good[:len(good)-20],
		"PNG, damaged":       corrupt,
		"header, no picture": pngHeader(8, 8),
	} {
		t.Run(name, func(t *testing.T) {
			pic, err := Decode(bytes.NewReader(data))
			if err == nil || pic != nil || errors.Is(err, ErrFormat) || errors.Is(err, ErrTooLarge) {
				t.Errorf("wanted a decoding error and no picture, got %v and %v", err, pic)
			}
		})
	}
}

type failingReader struct{}

var errBroken = errors.New("the disk is on fire")

func (failingReader) Read([]byte) (int, error) { return 0, errBroken }

func TestDecodePassesOnAFailureOfTheReader(t *testing.T) {
	if _, err := Decode(failingReader{}); !errors.Is(err, errBroken) {
		t.Errorf("wanted the reader's error, got %v", err)
	}
}

// zeros never ends, and counts what is taken from it.
type zeros struct{ n int64 }

func (z *zeros) Read(p []byte) (int, error) {
	clear(p)
	z.n += int64(len(p))
	return len(p), nil
}

func TestDecodeLimitStopsReadingAFileThatNeverEnds(t *testing.T) {
	src := &zeros{}
	if _, err := DecodeLimit(src, 1000, 1<<20); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("wanted ErrTooLarge, got %v", err)
	}
	if src.n > 1000+1+4096 { // a reader may ask for a little more than it keeps
		t.Errorf("wanted about 1,000 bytes taken from a file that never ends, got %d", src.n)
	}
}

func TestDecodeLimitByteLimitIsExact(t *testing.T) {
	data := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 4, 4)))
	n := int64(len(data))
	if _, err := DecodeLimit(bytes.NewReader(data), n, 1<<20); err != nil {
		t.Errorf("wanted a file of exactly the limit to be read, got %v", err)
	}
	if _, err := DecodeLimit(bytes.NewReader(data), n-1, 1<<20); !errors.Is(err, ErrTooLarge) {
		t.Errorf("wanted a file one byte over the limit refused, got %v", err)
	}
	// The largest limit does not overflow when one is added to it.
	if _, err := DecodeLimit(bytes.NewReader(data), math.MaxInt64, math.MaxInt64); err != nil {
		t.Errorf("wanted the largest limits to work, got %v", err)
	}
}

func TestDecodeLimitPixelLimitIsExactAndIsCheckedBeforeDecoding(t *testing.T) {
	data := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 64, 64))) // 4,096 pixels
	if _, err := DecodeLimit(bytes.NewReader(data), 1<<20, 4096); err != nil {
		t.Errorf("wanted a picture of exactly the limit to be read, got %v", err)
	}
	if _, err := DecodeLimit(bytes.NewReader(data), 1<<20, 4095); !errors.Is(err, ErrTooLarge) {
		t.Errorf("wanted a picture one pixel over the limit refused, got %v", err)
	}

	// These headers claim pictures of up to 40 GB and have no pixels after them, so they
	// could only be refused from the header: decoding one would fail, not be refused.
	for name, data := range map[string][]byte{
		"100,000 by 100,000": pngHeader(100_000, 100_000),
		"2^31-1 by 1":        pngHeader(1<<31-1, 1),
		"1 by 2^31-1":        pngHeader(1, 1<<31-1),
		"2,049 by 2,048":     pngHeader(2049, 2048),
	} {
		if pic, err := Decode(bytes.NewReader(data)); !errors.Is(err, ErrTooLarge) || pic != nil {
			t.Errorf("%s: wanted ErrTooLarge from the header, got %v", name, err)
		}
	}
	// And one that fits the default is read as far as it goes, which is not far.
	if _, err := Decode(bytes.NewReader(pngHeader(2048, 2048))); errors.Is(err, ErrTooLarge) || errors.Is(err, ErrFormat) {
		t.Errorf("wanted 2,048 by 2,048 to be within the default limit, got %v", err)
	}
}

// A limit that is not positive refuses everything, a picture or not, and says it is too large
// rather than that the data is not a picture, because it never looks at it.
func TestDecodeLimitThatIsNotPositiveRefusesEverything(t *testing.T) {
	inputs := map[string][]byte{
		"a picture":     encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 2, 2))),
		"not a picture": []byte("not a picture"),
		"nothing":       {},
	}
	for name, data := range inputs {
		for _, lim := range [][2]int64{{0, 100}, {100_000, 0}, {-1, 100}, {100_000, -1}, {0, 0}, {math.MinInt64, math.MinInt64}} {
			if _, err := DecodeLimit(bytes.NewReader(data), lim[0], lim[1]); !errors.Is(err, ErrTooLarge) {
				t.Errorf("%s, limits %v: wanted ErrTooLarge, got %v", name, lim, err)
			}
		}
	}
}

func TestANilBitmapIsAPictureOfNoPixels(t *testing.T) {
	var b *Bitmap
	c := canvasOf(t, 4, 2, grey)
	b.Draw(c, 0, 0)
	b.DrawScaled(c, 0, 0, 10, 10)
	if b.Width() != 0 || b.Height() != 0 || !b.At(0, 0).IsDefault() {
		t.Errorf("wanted a nil bitmap to be empty")
	}
	for _, p := range pixelsOf(c) {
		if p != grey {
			t.Fatalf("wanted a nil bitmap to draw nothing")
		}
	}
}

func TestAtIsEmptyOutsideThePicture(t *testing.T) {
	pic := testPicture(t)
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {3, 0}, {0, 2}, {math.MinInt, 0}, {0, math.MaxInt}} {
		if got := pic.At(p[0], p[1]); !got.IsDefault() {
			t.Errorf("wanted nothing at %v, got %v", p, got)
		}
	}
}

// nearestByInterval is which of the n pixels of a picture pixel i, of the m it is stretched
// to, takes its color from, found a different way from the code: the pixel s covers the
// stretched pixels from s*m/n up to (s+1)*m/n, and the one whose range holds the middle of
// pixel i, i+1/2, is it. Written without a division, so there is no rounding to disagree.
func nearestByInterval(i, m, n int) int {
	for s := range n {
		if 2*s*m <= (2*i+1)*n && (2*i+1)*n < 2*(s+1)*m {
			return s
		}
	}
	return -1
}

func TestNearestIsThePixelUnderTheMiddle(t *testing.T) {
	for n := 1; n <= 9; n++ {
		for m := 1; m <= 40; m++ {
			for i := range m {
				if got, want := nearest(i, m, n), nearestByInterval(i, m, n); got != want {
					t.Fatalf("stretching %d pixels to %d, pixel %d: wanted %d, got %d", n, m, i, want, got)
				}
			}
		}
	}
	// At its own size every pixel is itself.
	for _, n := range slices.Concat([]int{1, 2, 3, 100, 4096, 1<<31 - 1, math.MaxInt}, intsFrom(1<<31, 1<<31+1)) {
		for _, i := range []int{0, 1, n / 2, n - 1} {
			if got := nearest(i, n, n); got != i {
				t.Errorf("at its own size, %d pixels, pixel %d: wanted %d, got %d", n, i, i, got)
			}
		}
	}
	// Never past the picture, at the ends of the largest sizes.
	for _, m := range slices.Concat([]int{1, 1<<31 - 1, math.MaxInt - 1, math.MaxInt}, intsFrom(1<<31, 1<<31+1)) {
		for _, n := range []int{1, 2, 5, 1 << 22} {
			for _, i := range []int{0, 1, m / 2, m - 2, m - 1} {
				if i < 0 || i >= m {
					continue
				}
				if got := nearest(i, m, n); got < 0 || got >= n {
					t.Errorf("stretching %d pixels to %d, pixel %d: wanted 0 to %d, got %d", n, m, i, n-1, got)
				}
			}
			// When no pixel of the picture is skipped, the ends are the ends.
			if m >= n && (nearest(0, m, n) != 0 || nearest(m-1, m, n) != n-1) {
				t.Errorf("stretching %d pixels to %d: wanted the first pixel to the first and the last to the last", n, m)
			}
		}
	}
}

// referenceDraw says what every pixel of a w by h canvas should be after a picture is drawn
// on a canvas of bg, stretched to rw by rh at (x, y): each pixel in the rectangle takes the
// color of the picture's pixel found by nearestByInterval, and a see-through one leaves bg.
func referenceDraw(pic *Bitmap, w, h int, bg engine.Color, x, y, rw, rh int) []engine.Color {
	out := make([]engine.Color, w*h)
	for py := range h {
		for px := range w {
			out[py*w+px] = bg
			i, j := px-x, py-y
			if i < 0 || j < 0 || i >= rw || j >= rh {
				continue
			}
			if p := pic.At(nearestByInterval(i, rw, pic.Width()), nearestByInterval(j, rh, pic.Height())); !p.IsDefault() {
				out[py*w+px] = p
			}
		}
	}
	return out
}

// Draw puts the picture where it says and leaves the rest of the canvas, and the see-through
// pixel's place, as they were.
func TestDrawPutsThePictureAtItsOwnSize(t *testing.T) {
	const w, h = 5, 3 // 5 by 6 pixels
	pic := testPicture(t)
	for x := -4; x < w+4; x++ {
		for y := -4; y < 2*h+4; y++ {
			c := canvasOf(t, w, h, grey)
			pic.Draw(c, x, y)
			if got, want := pixelsOf(c), referenceDraw(pic, w, 2*h, grey, x, y, pic.Width(), pic.Height()); !slices.Equal(got, want) {
				t.Fatalf("at (%d, %d): wanted %v, got %v", x, y, want, got)
			}
		}
	}
}

func TestDrawScaledIsTheReferenceForEverySizeAndPlace(t *testing.T) {
	const w, h = 5, 3
	pic := testPicture(t)
	for x := -3; x < w+3; x++ {
		for y := -3; y < 2*h+3; y++ {
			for rw := -1; rw < w+4; rw++ {
				for rh := -1; rh < 2*h+4; rh++ {
					c := canvasOf(t, w, h, grey)
					pic.DrawScaled(c, x, y, rw, rh)
					if got, want := pixelsOf(c), referenceDraw(pic, w, 2*h, grey, x, y, rw, rh); !slices.Equal(got, want) {
						t.Fatalf("stretched to %d by %d at (%d, %d): wanted %v, got %v", rw, rh, x, y, want, got)
					}
				}
			}
		}
	}
}

func TestDrawScaledAtItsOwnSizeIsDraw(t *testing.T) {
	pic := testPicture(t)
	for _, at := range [][2]int{{0, 0}, {-1, 1}, {3, 4}, {-5, -5}} {
		a, b := canvasOf(t, 5, 3, grey), canvasOf(t, 5, 3, grey)
		pic.Draw(a, at[0], at[1])
		pic.DrawScaled(b, at[0], at[1], 3, 2)
		if !slices.Equal(pixelsOf(a), pixelsOf(b)) {
			t.Errorf("at %v: wanted DrawScaled at the picture's size to be Draw", at)
		}
	}
}

func TestDrawScaledShowsThePixelsAsBlocksAndShrinksByTakingOne(t *testing.T) {
	pic := testPicture(t)
	c := canvasOf(t, 6, 3, grey)
	pic.DrawScaled(c, 0, 0, 6, 4) // twice as wide and tall
	want := [][]engine.Color{
		{red, red, grey, grey, green, green},
		{red, red, grey, grey, green, green},
		{blue, blue, yellow, yellow, black, black},
		{blue, blue, yellow, yellow, black, black},
	}
	for y, row := range want {
		for x, w := range row {
			if got := c.At(x, y); got != w {
				t.Errorf("doubled, pixel (%d, %d): wanted %v, got %v", x, y, w, got)
			}
		}
	}

	// Shrunk to a pixel it is the one under its middle: the second of three across, the
	// second of two down, which is yellow.
	c = canvasOf(t, 2, 1, grey)
	pic.DrawScaled(c, 0, 0, 1, 1)
	if got := c.At(0, 0); got != blue && got != yellow {
		t.Errorf("wanted a one pixel picture to be one of the pixels of the picture, got %v", got)
	}
}

func TestDrawTheColorsOfTheCanvasUnderAnythingSeeThroughStay(t *testing.T) {
	pic := testPicture(t)
	c := canvasOf(t, 5, 3, grey)
	c.FillRect(0, 0, 5, 6, engine.RGB(1, 2, 3))
	pic.Draw(c, 1, 1)
	if got := c.At(2, 1); got != engine.RGB(1, 2, 3) { // the see-through pixel (1, 0)
		t.Errorf("wanted what was under the see-through pixel left, got %v", got)
	}
	if got := c.At(1, 1); got != red {
		t.Errorf("wanted red over what was there, got %v", got)
	}
}

func TestDrawNeverReadsOrWritesDepth(t *testing.T) {
	pic := testPicture(t)
	c := canvasOf(t, 5, 3, grey)
	c.SetDepth(1, 1, 0.5, engine.RGB(9, 9, 9)) // nearer than anything: depth-tested drawing would lose to it
	pic.Draw(c, 1, 1)
	if got := c.At(1, 1); got != red {
		t.Errorf("wanted the picture to replace a pixel whatever its depth, got %v", got)
	}
	// Depth was not written. Where there was some it is as it was, 0.5, and where there was
	// none it is still infinitely far, so a pixel at any distance wins there.
	if c.SetDepth(1, 1, 0.6, blue) || !c.SetDepth(1, 1, 0.4, blue) {
		t.Errorf("wanted the depth at (1, 1) to be as it was, 0.5")
	}
	if !c.SetDepth(3, 2, 1e9, blue) {
		t.Errorf("wanted the depth under the picture to be as it was, infinitely far")
	}
}

// nearestBig is nearest for any sizes, with big integers, for the extremes: the floor of
// (2i+1)n / 2m.
func nearestBig(i *big.Int, m *big.Int, n int) int {
	num := new(big.Int).Mul(i, big.NewInt(2))
	num.Add(num, big.NewInt(1))
	num.Mul(num, big.NewInt(int64(n)))
	den := new(big.Int).Mul(m, big.NewInt(2))
	return int(num.Quo(num, den).Int64())
}

// referenceDrawBig is referenceDraw for any position and size, worked out with big integers
// so that nothing can overflow.
func referenceDrawBig(pic *Bitmap, w, h int, bg engine.Color, x, y, rw, rh int) []engine.Color {
	out := make([]engine.Color, w*h)
	bigW, bigH := big.NewInt(int64(rw)), big.NewInt(int64(rh))
	for py := range h {
		for px := range w {
			out[py*w+px] = bg
			i := new(big.Int).Sub(big.NewInt(int64(px)), big.NewInt(int64(x)))
			j := new(big.Int).Sub(big.NewInt(int64(py)), big.NewInt(int64(y)))
			if i.Sign() < 0 || j.Sign() < 0 || i.Cmp(bigW) >= 0 || j.Cmp(bigH) >= 0 {
				continue
			}
			if p := pic.At(nearestBig(i, bigW, pic.Width()), nearestBig(j, bigH, pic.Height())); !p.IsDefault() {
				out[py*w+px] = p
			}
		}
	}
	return out
}

// Positions and sizes as large as an int goes must not wrap around onto the canvas, loop for
// long or panic, and must draw what the exact reference says.
func TestDrawWithHugeNumbers(t *testing.T) {
	const w, h = 5, 3
	pic := testPicture(t)
	places := slices.Concat([]int{math.MinInt, math.MinInt + 1, -3, 0, 2, math.MaxInt - 1, math.MaxInt}, intsFrom(-(1 << 40)))
	sizes := slices.Concat([]int{math.MinInt, -1, 0, 1, 3, math.MaxInt/2 + 1, math.MaxInt - 1, math.MaxInt}, intsFrom(1<<31, 1<<31+1))
	for _, x := range places {
		for _, y := range places {
			c := canvasOf(t, w, h, grey)
			pic.Draw(c, x, y)
			if got, want := pixelsOf(c), referenceDrawBig(pic, w, 2*h, grey, x, y, pic.Width(), pic.Height()); !slices.Equal(got, want) {
				t.Fatalf("Draw at (%d, %d): wanted %v, got %v", x, y, want, got)
			}
			for _, rw := range sizes {
				for _, rh := range sizes {
					c := canvasOf(t, w, h, grey)
					pic.DrawScaled(c, x, y, rw, rh)
					if got, want := pixelsOf(c), referenceDrawBig(pic, w, 2*h, grey, x, y, rw, rh); !slices.Equal(got, want) {
						t.Fatalf("DrawScaled to %d by %d at (%d, %d): wanted %v, got %v", rw, rh, x, y, want, got)
					}
				}
			}
		}
	}
}

func TestDrawingAllocatesNothing(t *testing.T) {
	pic := testPicture(t)
	c := canvasOf(t, 80, 24, grey)
	draws := map[string]func(){
		"Draw":                    func() { pic.Draw(c, 10, 10) },
		"Draw, mostly off":        func() { pic.Draw(c, -1, 47) },
		"DrawScaled":              func() { pic.DrawScaled(c, 5, 5, 60, 30) },
		"DrawScaled, huge":        func() { pic.DrawScaled(c, -(math.MaxInt / 2), 0, math.MaxInt, math.MaxInt) },
		"DrawScaled, big numbers": func() { pic.DrawScaled(c, 0, 0, aboveInt32, aboveInt32) },
	}
	for name, draw := range draws {
		if n := testing.AllocsPerRun(50, draw); n != 0 {
			t.Errorf("wanted %s to allocate nothing, got %v allocations", name, n)
		}
	}
}

// Whatever the bytes are, DecodeLimit does not panic, and a picture it returns is within the
// limits, has pixels that can be read, and can be drawn.
func FuzzDecode(f *testing.F) {
	img := image.NewNRGBA(image.Rect(0, 0, 5, 4))
	for i := range img.Pix {
		img.Pix[i] = uint8(i * 7)
	}
	var p, j bytes.Buffer
	png.Encode(&p, img)
	jpeg.Encode(&j, img, nil)
	for _, seed := range [][]byte{p.Bytes(), j.Bytes(), pngHeader(100_000, 100_000), pngHeader(0, 1), pngHeader(64, 64), {0xFF, 0xD8}, []byte("\x89PNG\r\n\x1a\n"), nil} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		const maxPixels = 1 << 12
		pic, err := DecodeLimit(bytes.NewReader(data), 1<<16, maxPixels)
		if err != nil {
			if pic != nil {
				t.Fatalf("wanted no picture with an error, got one with %v", err)
			}
			return
		}
		if pic.Width() <= 0 || pic.Height() <= 0 || pic.Width()*pic.Height() > maxPixels {
			t.Fatalf("wanted a picture of 1 to %d pixels, got %d by %d", maxPixels, pic.Width(), pic.Height())
		}
		pic.At(pic.Width()-1, pic.Height()-1)
		c := canvasOf(t, 4, 2, grey)
		pic.Draw(c, -1, -1)
		pic.DrawScaled(c, 0, 0, 7, 3)
	})
}

// Any picture, place and size, as large as an int goes, draws what the exact reference
// draws.
func FuzzDrawScaled(f *testing.F) {
	f.Add(0, 0, 3, 2)
	f.Add(-2, -1, 7, 5)
	f.Add(1, 1, 1, 1)
	f.Add(0, 0, 0, 3)
	f.Add(math.MinInt, math.MinInt, math.MaxInt, math.MaxInt)
	f.Add(math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt)
	f.Add(-(math.MaxInt / 2), 2, math.MaxInt, 4)
	f.Add(2, 1, aboveInt32, aboveInt32)

	pic := testPicture(f)
	f.Fuzz(func(t *testing.T, x, y, rw, rh int) {
		const w, h = 4, 2
		c := canvasOf(t, w, h, grey)
		pic.DrawScaled(c, x, y, rw, rh)
		if got, want := pixelsOf(c), referenceDrawBig(pic, w, 2*h, grey, x, y, rw, rh); !slices.Equal(got, want) {
			t.Fatalf("stretched to %d by %d at (%d, %d): wanted %v, got %v", rw, rh, x, y, want, got)
		}
	})
}

func BenchmarkBitmap(b *testing.B) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = uint8(i * 13)
	}
	pic, err := Decode(bytes.NewReader(encodePNG(b, img)))
	if err != nil {
		b.Fatal(err)
	}
	fb, err := engine.NewFrameBuffer(200, 60)
	if err != nil {
		b.Fatal(err)
	}
	c := fb.Canvas()

	b.Run("Draw, 64 by 64", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			pic.Draw(c, 10, 10)
		}
	})
	b.Run("DrawScaled, 64 by 64 to 128 by 96", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			pic.DrawScaled(c, 10, 10, 128, 96)
		}
	})
	b.Run("DrawScaled, to the whole canvas", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			pic.DrawScaled(c, 0, 0, 200, 120)
		}
	})
	b.Run("DrawScaled, far off", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			pic.DrawScaled(c, 1_000_000, 0, 5_000_000, 5_000_000)
		}
	})
	b.Run("Decode, 64 by 64 PNG", func(b *testing.B) {
		data := encodePNG(b, img)
		b.ReportAllocs()
		for b.Loop() {
			if _, err := Decode(bytes.NewReader(data)); err != nil {
				b.Fatal(err)
			}
		}
	})
}

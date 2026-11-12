package bitmap

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"math/bits"

	"glonk.dev/engine"
	"glonk.dev/internal/imgdata"
)

// What Decode accepts: a file of up to 32 MiB that says it holds up to 2,048 by 2,048 pixels.
const (
	defaultMaxBytes  = 32 << 20
	defaultMaxPixels = 1 << 22
)

// ErrFormat is returned for data that is not a JPEG or a PNG, or that holds no pixels.
var ErrFormat = errors.New("bitmap: not a JPEG or PNG picture")

// ErrTooLarge is returned for a picture whose file is longer than the limit on bytes, or
// whose header says it has more pixels than the limit on pixels. It is returned before the
// picture is decoded.
var ErrTooLarge = errors.New("bitmap: picture too large")

// Bitmap is a picture, as the pixels of a Canvas. Every method is safe on a nil *Bitmap,
// which is a picture of no pixels.
type Bitmap struct {
	w, h   int
	pixels []engine.Color // row by row; the zero Color is a pixel that is not drawn
}

// Decode reads a JPEG or PNG from r and returns it as a Bitmap, refusing one that is over
// 32 MiB, or whose header says it has more than 4,194,304 pixels (2,048 by 2,048), as
// DecodeLimit does with those limits. The error is ErrFormat when the data is not a picture,
// ErrTooLarge when it is over a limit, and otherwise says why the picture could not be read:
// a failure of r, or a picture that is cut short or damaged. Where an int has 32 bits, the
// standard library itself refuses a PNG header of 2^28 pixels or more, and that is ErrFormat.
func Decode(r io.Reader) (*Bitmap, error) {
	return DecodeLimit(r, defaultMaxBytes, defaultMaxPixels)
}

// DecodeLimit is Decode with limits of your own: no more than maxBytes bytes are read from
// r, and a picture whose header says it has more than maxPixels pixels is not decoded. A
// limit that is not positive refuses every picture. The kind of picture is found by what
// the data holds, and its size is read from the header before the picture is decoded, so
// the most that a file can make the program spend is in proportion to the limits, not to
// what the file claims.
func DecodeLimit(r io.Reader, maxBytes, maxPixels int64) (*Bitmap, error) {
	if maxBytes <= 0 || maxPixels <= 0 {
		return nil, ErrTooLarge
	}
	// One byte more than allowed: getting it is how a file that is too long is told from one
	// that is exactly as long as the limit.
	limit := maxBytes
	if limit < math.MaxInt64 {
		limit++
	}
	data, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return nil, fmt.Errorf("bitmap: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrTooLarge
	}

	width, height, decode, ok := imgdata.Open(data)
	if !ok {
		return nil, ErrFormat
	}
	// In 64 bits, where two sizes of up to 2^31 cannot overflow.
	if int64(width)*int64(height) > maxPixels {
		return nil, ErrTooLarge
	}
	img, err := decode()
	if err != nil {
		return nil, fmt.Errorf("bitmap: %w", err)
	}
	b := img.Bounds()
	if b.Empty() {
		return nil, ErrFormat
	}
	return fromImage(img), nil
}

// fromImage converts a picture to the colors of a canvas. A pixel that is half transparent
// or more becomes the zero Color, which is not drawn.
func fromImage(img image.Image) *Bitmap {
	b := img.Bounds()
	bm := &Bitmap{w: b.Dx(), h: b.Dy(), pixels: make([]engine.Color, b.Dx()*b.Dy())}
	for y := range bm.h {
		for x := range bm.w {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			if c.A >= 128 {
				bm.pixels[y*bm.w+x] = engine.RGB(c.R, c.G, c.B)
			}
		}
	}
	return bm
}

// Width returns the width of the picture in pixels.
func (b *Bitmap) Width() int {
	if b == nil {
		return 0
	}
	return b.w
}

// Height returns the height of the picture in pixels.
func (b *Bitmap) Height() int {
	if b == nil {
		return 0
	}
	return b.h
}

// At returns the color of pixel (x, y), counted from the top left, or the zero Color, which
// is none, for a pixel that is see-through or outside the picture.
func (b *Bitmap) At(x, y int) engine.Color {
	if b == nil || x < 0 || x >= b.w || y < 0 || y >= b.h {
		return engine.Color{}
	}
	return b.pixels[y*b.w+x]
}

// Draw draws the picture at its own size on c, with its top left pixel at pixel (x, y).
// Pixels that are see-through are left as they are, and the others replace what is there,
// whatever its depth, which Draw does not read or write, as Canvas.Set does. Only the pixels
// that are on the canvas are visited, so no position can make it slow, or overflow.
func (b *Bitmap) Draw(c *engine.Canvas, x, y int) {
	if b == nil {
		return
	}
	x0, x1 := visible(x, b.w, c.Width())
	y0, y1 := visible(y, b.h, c.Height())
	for py := y0; py < y1; py++ {
		row := b.pixels[(py-y)*b.w:]
		for px := x0; px < x1; px++ {
			if p := row[px-x]; !p.IsDefault() {
				c.Set(px, py, p)
			}
		}
	}
}

// DrawScaled draws the picture stretched or squeezed to w pixels wide and h tall on c, with
// its top left pixel at pixel (x, y). Each pixel drawn takes the one pixel of the picture
// that is under its middle, with no blending, so a picture drawn at its own size is
// drawn as Draw draws it. A width or height that is not positive draws nothing. Everything
// else is as for Draw.
func (b *Bitmap) DrawScaled(c *engine.Canvas, x, y, w, h int) {
	if b == nil {
		return
	}
	x0, x1 := visible(x, w, c.Width())
	y0, y1 := visible(y, h, c.Height())
	for py := y0; py < y1; py++ {
		row := b.pixels[nearest(py-y, h, b.h)*b.w:]
		for px := x0; px < x1; px++ {
			if p := row[nearest(px-x, w, b.w)]; !p.IsDefault() {
				c.Set(px, py, p)
			}
		}
	}
}

// visible returns the part of the n pixels from start that is inside [0, limit), as the first
// and one past the last, empty (lo == hi) when there is none. It cannot overflow.
func visible(start, n, limit int) (lo, hi int) {
	if n <= 0 || start >= limit {
		return 0, 0
	}
	end := math.MaxInt
	if start <= math.MaxInt-n {
		end = start + n
	}
	lo, hi = max(start, 0), min(end, limit)
	if lo >= hi {
		return 0, 0
	}
	return lo, hi
}

// nearest returns which of the n pixels of a picture the pixel i, of the m it is stretched
// to, takes its color from: the one the middle of the pixel falls in, which is the floor of
// (2i+1)n / 2m. It needs 0 <= i < m and n >= 1, and gives 0 <= s < n.
func nearest(i, m, n int) int {
	// Two sizes of up to 2^31 and a position under 2^32 cannot overflow 63 bits. (Compared as
	// int64, so that this compiles where an int has only 32 bits, and is then always true.)
	if int64(m) <= 1<<31 && int64(n) <= 1<<31 {
		return int((2*int64(i) + 1) * int64(n) / (2 * int64(m)))
	}
	hi, lo := bits.Mul64(2*uint64(i)+1, uint64(n))
	q, _ := bits.Div64(hi, lo, 2*uint64(m))
	return int(q)
}

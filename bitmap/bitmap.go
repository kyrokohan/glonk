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
// which is a picture of no pixels, and on an empty one, which New, FromImage and Sub return
// for a size, an image or a rectangle that has none.
//
// A Bitmap can be a view of part of another, which shares its pixels: see Sub.
type Bitmap struct {
	w, h   int            // both 0, or both positive
	stride int            // the distance from a pixel to the one below it in pixels, at least w
	pixels []engine.Color // row by row from the top left of the view; the zero Color is a pixel that is not drawn
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
	if img.Bounds().Empty() {
		return nil, ErrFormat
	}
	return FromImage(img), nil
}

// New returns a picture of w by h pixels, none of them drawn yet: every pixel is the zero
// Color, which is see-through, until [Bitmap.Set] gives it a color. A size that is not
// positive, or has more than 4,194,304 pixels (2,048 by 2,048), which is what Decode allows,
// gives an empty picture of 0 by 0, so that a size read from a file cannot ask for gigabytes.
func New(w, h int) *Bitmap {
	// Divided, not multiplied, so that no pair of sizes overflows.
	if w <= 0 || h <= 0 || h > defaultMaxPixels/w {
		return &Bitmap{}
	}
	return &Bitmap{w: w, h: h, stride: w, pixels: make([]engine.Color, w*h)}
}

// FromImage returns a copy of img as a Bitmap, with the rule Decode has for alpha: a pixel
// with an alpha of 128 or more is drawn, as an opaque color, and one with less becomes the
// zero Color, which is not drawn. The top left corner of img.Bounds() is pixel (0, 0). A nil
// img, or one with no pixels, gives an empty picture of 0 by 0.
//
// Unlike Decode, it has no limits: the pixels are already in memory, and made by the program.
// A picture that is bigger than that is the program's to refuse. Changing img afterwards does
// not change the Bitmap.
func FromImage(img image.Image) *Bitmap {
	if img == nil {
		return &Bitmap{}
	}
	b := img.Bounds()
	if b.Empty() {
		return &Bitmap{}
	}
	bm := &Bitmap{w: b.Dx(), h: b.Dy(), stride: b.Dx(), pixels: make([]engine.Color, b.Dx()*b.Dy())}
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
	return b.pixels[y*b.stride+x]
}

// Set gives pixel (x, y), counted from the top left, the color c. The zero Color makes the
// pixel see-through, as At returns it. A pixel outside the picture is left out, as for
// [engine.Canvas.Set]. It changes the picture for everything that draws it, so a Bitmap that
// is shared and drawn while it is set needs the program's own lock.
func (b *Bitmap) Set(x, y int, c engine.Color) {
	if b == nil || x < 0 || x >= b.w || y < 0 || y >= b.h {
		return
	}
	b.pixels[y*b.stride+x] = c
}

// Sub returns the part of the picture that is w pixels wide and h tall with its top left
// pixel at (x, y), as a Bitmap of its own that shares the pixels of this one, so that a strip
// of animation frames in one picture is a frame at a time to every call here: draw it, stretch
// it, ask for a pixel of it. Set on either changes the other. Nothing is copied, so a view keeps
// the whole picture in memory for as long as it is used.
//
// A rectangle that runs off the picture is cut to it, and one with nothing left, or a width or
// height that is not positive, gives an empty picture of 0 by 0. The rectangle is counted in
// the pixels of the picture it is taken from, so a Sub of a Sub is relative to the first. No
// position or size can overflow.
func (b *Bitmap) Sub(x, y, w, h int) *Bitmap {
	if b == nil {
		return &Bitmap{}
	}
	x0, x1 := visible(x, w, b.w)
	y0, y1 := visible(y, h, b.h)
	if x0 == x1 || y0 == y1 {
		return &Bitmap{}
	}
	return &Bitmap{w: x1 - x0, h: y1 - y0, stride: b.stride, pixels: b.pixels[y0*b.stride+x0:]}
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
		row := b.pixels[(py-y)*b.stride:]
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
	if b == nil || b.w == 0 {
		return
	}
	x0, x1 := visible(x, w, c.Width())
	y0, y1 := visible(y, h, c.Height())
	bw, bh, stride := b.w, b.h, b.stride // in locals: c.Set could alias b for all the compiler knows
	for py := y0; py < y1; py++ {
		row := b.pixels[nearest(py-y, h, bh)*stride:]
		for px := x0; px < x1; px++ {
			if p := row[nearest(px-x, w, bw)]; !p.IsDefault() {
				c.Set(px, py, p)
			}
		}
	}
}

// DrawScaledDepth is DrawScaled at a depth, for a picture of a thing that stands in a 3D
// scene. Each pixel it draws is stored with [engine.Canvas.SetDepth], so a nearer surface
// hides it, it hides a farther one, and the order of drawing stops mattering. A tie keeps what
// is there, as for Canvas.DrawLineDepth. A see-through pixel is left alone, color and depth.
//
// depth is the same for the whole picture, which suits a thing that faces the camera. In a
// scene drawn through a mesh.Camera it is the distance along the camera's view direction, and
// mesh.Camera.ProjectPixels gives it with the pixel to draw at. A depth that is not positive,
// or is NaN, draws nothing, and an infinite depth is infinitely far: nothing is nearer than
// it, so nothing is drawn. Everything else is as for DrawScaled.
func (b *Bitmap) DrawScaledDepth(c *engine.Canvas, x, y, w, h int, depth float64) {
	if b == nil || b.w == 0 || !(depth > 0) {
		return
	}
	x0, x1 := visible(x, w, c.Width())
	y0, y1 := visible(y, h, c.Height())
	bw, bh, stride := b.w, b.h, b.stride
	for py := y0; py < y1; py++ {
		row := b.pixels[nearest(py-y, h, bh)*stride:]
		for px := x0; px < x1; px++ {
			if p := row[nearest(px-x, w, bw)]; !p.IsDefault() {
				c.SetDepth(px, py, depth, p)
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

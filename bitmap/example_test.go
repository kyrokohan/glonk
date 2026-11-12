package bitmap_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
)

// logo is a 4 by 3 PNG made here so that the example needs no file: a yellow picture with a
// see-through pixel at its top left corner and a black one at the bottom right.
func logo() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	for y := range 3 {
		for x := range 4 {
			img.SetNRGBA(x, y, color.NRGBA{R: 230, G: 210, B: 40, A: 255})
		}
	}
	img.SetNRGBA(0, 0, color.NRGBA{})
	img.SetNRGBA(3, 2, color.NRGBA{A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// show prints the canvas, one character for each pixel: . for the gray behind the picture,
// Y for yellow, # for black.
func show(canvas *engine.Canvas) {
	for y := range canvas.Height() {
		var row strings.Builder
		for x := range canvas.Width() {
			switch canvas.At(x, y) {
			case engine.RGB(230, 210, 40):
				row.WriteByte('Y')
			case engine.RGB(0, 0, 0):
				row.WriteByte('#')
			default:
				row.WriteByte('.')
			}
		}
		fmt.Println(row.String())
	}
}

// A picture is decoded once and drawn as often as you like. A see-through pixel leaves what
// is on the canvas under it, here a gray background, and black is a color like any other.
func ExampleDecode() {
	pic, err := bitmap.Decode(bytes.NewReader(logo()))
	if err != nil {
		panic(err)
	}

	fb, err := engine.NewFrameBuffer(7, 2) // 7 by 4 pixels
	if err != nil {
		panic(err)
	}
	canvas := fb.Canvas()
	canvas.FillRect(0, 0, canvas.Width(), canvas.Height(), engine.RGB(90, 90, 90))

	pic.Draw(canvas, 2, 1)
	show(canvas)
	// Output:
	// .......
	// ...YYY.
	// ..YYYY.
	// ..YYY#.
}

// DrawScaled stretches or squeezes a picture to any size, taking the pixel under the middle
// of each one it draws, so a picture that is made bigger shows its pixels as blocks.
func ExampleBitmap_DrawScaled() {
	pic, err := bitmap.Decode(bytes.NewReader(logo()))
	if err != nil {
		panic(err)
	}

	fb, err := engine.NewFrameBuffer(8, 3) // 8 by 6 pixels
	if err != nil {
		panic(err)
	}
	canvas := fb.Canvas()
	canvas.FillRect(0, 0, canvas.Width(), canvas.Height(), engine.RGB(90, 90, 90))

	pic.DrawScaled(canvas, 0, 0, 8, 6) // twice as wide and tall
	show(canvas)
	// Output:
	// ..YYYYYY
	// ..YYYYYY
	// YYYYYYYY
	// YYYYYYYY
	// YYYYYY##
	// YYYYYY##
}

// DecodeLimit takes limits of its own. A file that is longer, or whose header says it has more
// pixels, is refused before it is decoded.
func ExampleDecodeLimit() {
	_, err := bitmap.DecodeLimit(bytes.NewReader(logo()), 1<<20, 8) // 8 pixels at most
	fmt.Println(err)
	_, err = bitmap.DecodeLimit(strings.NewReader("not a picture"), 1<<20, 1<<20)
	fmt.Println(err)
	// Output:
	// bitmap: picture too large
	// bitmap: not a JPEG or PNG picture
}

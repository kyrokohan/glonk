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

// New makes a picture in code, with no file: every pixel starts see-through and Set gives it a
// color. This is the picture of ExampleDecode, so it is drawn the same way.
func ExampleNew() {
	yellow := engine.RGB(230, 210, 40)
	pic := bitmap.New(4, 3)
	for y := range pic.Height() {
		for x := range pic.Width() {
			pic.Set(x, y, yellow)
		}
	}
	pic.Set(0, 0, engine.Color{})      // the zero Color is see-through again
	pic.Set(3, 2, engine.RGB(0, 0, 0)) // black is a color like any other
	pic.Set(9, 9, yellow)              // outside the picture: left out

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

// FromImage copies any image.Image, with the alpha rule of Decode: a pixel of 128 or more is
// drawn and a fainter one is not. The top left corner of the image, here (10, 20), becomes pixel
// (0, 0) of the picture.
func ExampleFromImage() {
	img := image.NewNRGBA(image.Rect(10, 20, 14, 23)) // 4 by 3
	for y := 20; y < 23; y++ {
		for x := 10; x < 14; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 230, G: 210, B: 40, A: 255})
		}
	}
	img.SetNRGBA(10, 20, color.NRGBA{R: 230, G: 210, B: 40, A: 127}) // too faint to draw
	img.SetNRGBA(13, 22, color.NRGBA{A: 255})                        // black

	pic := bitmap.FromImage(img)

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

// Sub is a view of part of a picture, which shares its pixels: the frames of an animation can
// be side by side in one picture and drawn a frame at a time. Here a strip of three frames of 2 by
// 2 pixels is drawn with a gap between them.
func ExampleBitmap_Sub() {
	strip := bitmap.New(6, 2)
	for y, row := range []string{"YYY###", "YY#Y##"} {
		for x, r := range row {
			switch r {
			case 'Y':
				strip.Set(x, y, engine.RGB(230, 210, 40))
			case '#':
				strip.Set(x, y, engine.RGB(0, 0, 0))
			}
		}
	}

	fb, err := engine.NewFrameBuffer(8, 1) // 8 by 2 pixels
	if err != nil {
		panic(err)
	}
	canvas := fb.Canvas()
	for i := range 3 {
		strip.Sub(2*i, 0, 2, 2).Draw(canvas, 3*i, 0) // frame i
	}
	show(canvas)
	// Output:
	// YY.Y#.##
	// YY.#Y.##
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

// DrawScaledDepth draws a picture that a nearer surface hides and that hides a farther one,
// in whatever order they are drawn. Here a wall of gray pixels 2 away covers the left half of
// the canvas: a picture 5 away is hidden where the wall is, and one 1 away is drawn over it,
// with its see-through corner showing the wall.
func ExampleBitmap_DrawScaledDepth() {
	pic, err := bitmap.Decode(bytes.NewReader(logo()))
	if err != nil {
		panic(err)
	}

	for _, z := range []float64{5, 1} {
		fb, err := engine.NewFrameBuffer(8, 2) // 8 by 4 pixels
		if err != nil {
			panic(err)
		}
		canvas := fb.Canvas()
		for y := range canvas.Height() {
			for x := range 4 {
				canvas.SetDepth(x, y, 2, engine.RGB(90, 90, 90))
			}
		}

		pic.DrawScaledDepth(canvas, 2, 0, 4, 3, z)
		fmt.Printf("depth %.0f:\n", z)
		show(canvas)
	}
	// Output:
	// depth 5:
	// ....YY..
	// ....YY..
	// ....Y#..
	// ........
	// depth 1:
	// ...YYY..
	// ..YYYY..
	// ..YYY#..
	// ........
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

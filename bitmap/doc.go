// Package bitmap reads a JPEG or PNG picture and draws it on the pixel canvas of the engine
// package, at its own size or stretched.
//
// [Decode] reads a picture once and turns it into the colors of the canvas, so drawing it
// every frame is a copy to an [engine.Canvas] that allocates nothing. The pictures come from
// files and downloads that cannot be trusted, and a small file can claim a picture of
// billions of pixels, so Decode reads the size from the picture's header and refuses one that
// is over its limits before it spends memory on it. The kind is found by what the data holds,
// not by what a file is called.
//
//	logo, err := bitmap.Decode(bytes.NewReader(logoPNG)) // a //go:embed []byte, a file, a download
//	if err != nil {
//		return err
//	}
//	logo.Draw(canvas, 2, 1)                // at its own size, top left pixel at (2, 1)
//	logo.DrawScaled(canvas, 40, 1, 24, 12) // or stretched to 24 by 12 pixels
//
// # Transparency
//
// A pixel of a PNG with an alpha of 128 or more is drawn, as an opaque color, and one with
// less is left alone, so what is on the canvas under it shows. A canvas pixel is one color
// with no alpha, and the terminal's own background is not known, so there is no blending.
// A JPEG has no alpha, so all its pixels are drawn. Black is a color like any other and is
// drawn.
//
// # Scaling
//
// [Bitmap.DrawScaled] takes, for each pixel it draws, the one pixel of the picture under
// the middle of it, with no blending or smoothing: a picture drawn at its own size comes out
// exactly as it is, and one that is stretched shows its pixels as blocks. The pixels of a
// canvas are about square, so a picture keeps its shape when its width and height are
// scaled by the same amount.
//
// # Limits
//
// [DecodeLimit] sets the most bytes and the most pixels a picture may have; Decode allows
// 32 MiB and 4,194,304 pixels, which is 2,048 by 2,048. The memory spent on a picture is in
// proportion to its pixels, so lower the limits where memory is short. Drawing only visits
// the pixels that are on the canvas, so a picture drawn far off the screen, or stretched to
// a huge size, costs no more than one drawn on it.
package bitmap

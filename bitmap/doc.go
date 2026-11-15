// Package bitmap reads a JPEG or PNG picture, or makes one in code, and draws it on the pixel
// canvas of the engine package, at its own size or stretched.
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
// # Pictures made in code
//
// [New] makes a blank picture, every pixel see-through, and [Bitmap.Set] gives a pixel its
// color: a generated texture, a sprite in other colors, a small map of a level. [FromImage]
// copies an image.Image, one that a program drew with the image package or got from another
// library, so that nothing has to be written out as a PNG and read back.
//
//	tile := bitmap.New(8, 8)
//	tile.Set(3, 4, engine.RGB(230, 210, 40)) // the other 63 pixels stay see-through
//	pic := bitmap.FromImage(img)             // any image.Image, with the alpha rule below
//
// # Transparency
//
// A pixel of a PNG with an alpha of 128 or more is drawn, as an opaque color, and one with
// less is left alone, so what is on the canvas under it shows. A canvas pixel is one color
// with no alpha, and the terminal's own background is not known, so there is no blending.
// A JPEG has no alpha, so all its pixels are drawn. Black is a color like any other and is
// drawn. [FromImage] has the same rule for any image, and in a picture made with [New] the
// zero Color is the see-through pixel.
//
// # Scaling
//
// [Bitmap.DrawScaled] takes, for each pixel it draws, the one pixel of the picture under
// the middle of it, with no blending or smoothing: a picture drawn at its own size comes out
// exactly as it is, and one that is stretched shows its pixels as blocks. The pixels of a
// canvas are about square, so a picture keeps its shape when its width and height are
// scaled by the same amount.
//
// # Depth
//
// [Bitmap.Draw] and [Bitmap.DrawScaled] never read or write the depth of the canvas, which is
// right for a logo or a HUD: they replace what is there. A picture of a thing that stands in a
// 3D scene, an enemy, a pickup or a tree, should be hidden by the walls in front of it and
// hide those behind it. [Bitmap.DrawScaledDepth] does that, with one depth for the whole
// picture, and a tie keeps what is there. The depth and the pixel to draw at come from
// mesh.Camera.ProjectPixels, from the pixels of the foot and the top of the thing.
//
// # Limits
//
// [DecodeLimit] sets the most bytes and the most pixels a picture may have; Decode allows
// 32 MiB and 4,194,304 pixels, which is 2,048 by 2,048. The memory spent on a picture is in
// proportion to its pixels, so lower the limits where memory is short. Drawing only visits
// the pixels that are on the canvas, so a picture drawn far off the screen, or stretched to
// a huge size, costs no more than one drawn on it. [FromImage] has no limits, since the
// pixels of its image are already in memory and made by the program, but [New] gives an empty
// picture for a size of more than the same 4,194,304 pixels, as a size can come from a file.
package bitmap

package engine

import (
	"image"
	"image/color"
)

// maxImagePixels is the most pixels Image makes a picture of, the limit Decode has for a picture
// it reads, so that a scale read from somewhere cannot ask for gigabytes.
const maxImagePixels = 1 << 22

// ImageOptions says how Image draws a frame. The zero value draws it at its own size in exact
// colors.
type ImageOptions struct {
	// Scale is how many pixels of the picture make one pixel of a Canvas, across and down. A
	// Scale below 1 counts as 1.
	Scale int

	// Colors says whether an RGB color is drawn exactly or as the nearest of the palette colors 16
	// to 255, as a terminal without true color shows it. Colors256 does the second; ColorsAuto and
	// ColorsTrue draw it exactly, since a picture has no terminal to ask. A palette color is
	// always drawn as xterm defines it.
	Colors ColorMode

	// Foreground and Background are what the default colors of a cell look like, which a terminal
	// decides and a program cannot know. The zero Color gives light gray for the foreground and
	// black for the background.
	Foreground, Background Color

	// Text draws each printable ASCII character in its cell as a letter of the font that
	// Canvas.DrawText draws with, in the foreground over the background, and not as a mark. A
	// letter is drawn at the largest whole size that leaves a blank column beside it, Scale/6, so
	// letters appear from a Scale of 6; below that, and for any other character, a cell is drawn
	// as without it.
	Text bool
}

// Image draws the frame as a picture, for a test to compare with one it has stored or for a
// person to look at, where there is no terminal. It shows the cells, which a Canvas's pixels are
// once Compose has put them there.
//
// The picture is Width*Scale pixels across and 2*Height*Scale down: a cell is one pixel of a
// Canvas across and two down, so a frame made with a Canvas comes out pixel for pixel at Scale 1, and
// a half-block cell is its two pixels. A space, or a character that is not set, is the cell's
// background, a full block (█) its foreground, and ▀ and ▄ the foreground over the background and
// the other way up. Any other character is a mark: a rectangle in the foreground over the middle
// half of the cell, which shows where text is and in what color but not what it says; use
// [FrameBuffer.String] for that. Reverse swaps a cell's colors; Bold, Dim and Underline are not
// drawn. Every pixel is opaque.
//
// With Text, from a Scale of 6, a printable ASCII character is its letter instead, Scale/6 times the
// font's 5 by 7 pixels, in the middle of the cell, rounded up and to the left. A wide or control
// character is drawn as ?, as the terminal is sent one, and box lines, shades and every other
// character outside ASCII stay marks. At the usual Scale of 4 there is no room for a letter, and
// nothing changes.
//
// A picture of more than 4,194,304 pixels is not made: the result is empty, 0 by 0. The picture is
// a copy, and the frame can be drawn into again.
//
//	f, _ := os.Create("frame.png")
//	png.Encode(f, fb.Image(engine.ImageOptions{Scale: 4}))
func (fb *FrameBuffer) Image(opts ImageOptions) *image.RGBA {
	scale := max(opts.Scale, 1)
	// A picture is at least 2*scale*scale pixels, so a bigger scale is over the limit, and then no
	// size here can overflow, even in 32 bits.
	if scale > 1<<11 {
		return image.NewRGBA(image.Rectangle{})
	}
	w, h := int64(fb.width)*int64(scale), 2*int64(fb.height)*int64(scale)
	if h > maxImagePixels/w { // divided, so that no pair of sizes overflows
		return image.NewRGBA(image.Rectangle{})
	}

	pen := newImagePen(opts)
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	// The mark: the middle half of the cell across and down, and the whole cell where there is
	// no room for less.
	markX0, markX1 := scale/4, scale-scale/4
	markY0, markY1 := scale/2, 2*scale-scale/2
	// The size of a letter, 0 for none, and where its top left is in the cell.
	size := 0
	if opts.Text {
		size = scale / fontAdvance
	}
	textX, textY := (scale-fontWidth*size)/2, (2*scale-fontHeight*size)/2

	for cy := range fb.height {
		for cx := range fb.width {
			cell := fb.frame[cy*fb.width+cx]
			fg, bg := pen.rgba(cell.Foreground, pen.fg), pen.rgba(cell.Background, pen.bg)
			if cell.Attrs.Has(Reverse) {
				fg, bg = bg, fg
			}
			x0, y0 := cx*scale, 2*cy*scale
			switch r := safeRune(cell.Character); {
			case r == ' ':
				fillRect(img, x0, y0, x0+scale, y0+2*scale, bg)
			case r == upperHalfBlock:
				fillRect(img, x0, y0, x0+scale, y0+scale, fg)
				fillRect(img, x0, y0+scale, x0+scale, y0+2*scale, bg)
			case r == lowerHalfBlock:
				fillRect(img, x0, y0, x0+scale, y0+scale, bg)
				fillRect(img, x0, y0+scale, x0+scale, y0+2*scale, fg)
			case r == '█':
				fillRect(img, x0, y0, x0+scale, y0+2*scale, fg)
			case size > 0 && r > ' ' && r <= '~':
				fillRect(img, x0, y0, x0+scale, y0+2*scale, bg)
				gx, gy := x0+textX, y0+textY
				for row, bits := range font[r-' '] {
					for col := range fontWidth {
						if bits&(1<<(fontWidth-1-col)) != 0 {
							px, py := gx+col*size, gy+row*size
							fillRect(img, px, py, px+size, py+size, fg)
						}
					}
				}
			default:
				fillRect(img, x0, y0, x0+scale, y0+2*scale, bg)
				fillRect(img, x0+markX0, y0+markY0, x0+markX1, y0+markY1, fg)
			}
		}
	}
	return img
}

// fillRect sets the pixels from (x0, y0) up to, not including, (x1, y1) to c.
func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		row := img.Pix[y*img.Stride+4*x0 : y*img.Stride+4*x1]
		for i := 0; i < len(row); i += 4 {
			row[i], row[i+1], row[i+2], row[i+3] = c.R, c.G, c.B, c.A
		}
	}
}

// imagePen turns the Color of a cell into what a picture shows.
type imagePen struct {
	fg, bg  color.RGBA // the default colors
	palette bool       // an RGB color is shown as the nearest palette color
}

func newImagePen(opts ImageOptions) imagePen {
	p := imagePen{palette: opts.Colors == Colors256}
	p.fg = exactRGBA(opts.Foreground, color.RGBA{R: 229, G: 229, B: 229, A: 255})
	p.bg = exactRGBA(opts.Background, color.RGBA{A: 255})
	return p
}

// exactRGBA is c as the red, green and blue it stands for, or def where c is the default color.
func exactRGBA(c Color, def color.RGBA) color.RGBA {
	if c.IsDefault() {
		return def
	}
	r, g, b := c.RGB()
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// rgba is what c looks like, def where c is the default color.
func (p imagePen) rgba(c Color, def color.RGBA) color.RGBA {
	if p.palette && c.kind() == colorRGB {
		r, g, b := c.RGB()
		c = Palette(nearestPalette(float64(r), float64(g), float64(b)))
	}
	return exactRGBA(c, def)
}

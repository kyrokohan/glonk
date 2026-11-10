package mesh

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"

	"glonk.dev/engine"
)

// The most an OBJ can ask of the images its materials name. They come from the same
// untrusted place as the model, and a small file can claim a huge picture.
const (
	maxTextureBytes  = 64 << 20 // per file, or the OBJ's own limit if that is smaller
	maxTexturePixels = 1 << 24  // decoded pixels, all of an OBJ's images together
	maxTextureFiles  = 8        // image names tried per OBJ
)

// texture is a decoded image to look colors up in.
type texture struct {
	img  image.Image // from a decoder, so its top left is (0, 0)
	w, h int
}

// texture returns the image called name, or nil if there is none to be had: the name is
// not a plain path inside the OBJ's directory, the file is missing, over the limit,
// neither a JPEG nor a PNG (found by what is in it, not by its name), broken or cut
// short, or more than the budget allows. The same name is only tried once, and no more
// than maxTextureFiles names are.
//
// The size is read from the header and checked before the picture is decoded. What is
// spent from the budget is the size of a picture that is decoded, whether or not it
// turns out to be whole, so a file that lies cannot be tried again and again.
func (l *materialLibrary) texture(name string) *texture {
	if t, tried := l.textures[name]; tried {
		return t
	}
	if name == "" || len(l.textures) >= maxTextureFiles {
		return nil
	}
	t := l.loadTexture(name)
	l.textures[name] = t
	return t
}

func (l *materialLibrary) loadTexture(name string) *texture {
	data, ok := l.readFile(name, l.textureLimit)
	if !ok {
		return nil
	}

	var (
		config func(io.Reader) (image.Config, error)
		decode func(io.Reader) (image.Image, error)
	)
	switch {
	case bytes.HasPrefix(data, []byte("\xff\xd8")):
		config, decode = jpeg.DecodeConfig, jpeg.Decode
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		config, decode = png.DecodeConfig, png.Decode
	default:
		return nil
	}

	cfg, err := config(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil
	}
	// In 64 bits, where two sizes of up to 2^31 cannot overflow.
	pixels := int64(cfg.Width) * int64(cfg.Height)
	if pixels > maxTexturePixels-l.pixels {
		return nil
	}
	l.pixels += pixels

	img, err := decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	b := img.Bounds()
	if b.Empty() {
		return nil
	}
	return &texture{img: img, w: b.Dx(), h: b.Dy()}
}

// at returns the color of the texel at texture coordinates (u, v), as an OBJ gives them:
// (0, 0) is the bottom left of the picture and (1, 1) the top right, and the picture
// repeats beyond that. The nearest texel is used, with no blending. The picture's own
// transparency is ignored and its colors are used as they are, with no gamma conversion.
// A coordinate that is NaN or infinite gives the zero Color, meaning none.
func (t *texture) at(u, v float64) engine.Color {
	if math.IsNaN(u) || math.IsInf(u, 0) || math.IsNaN(v) || math.IsInf(v, 0) {
		return engine.Color{}
	}
	u -= math.Floor(u)
	v -= math.Floor(v)
	// u is in [0, 1), but u*w can round up to w, and v of 0 is the bottom row, which is
	// the last one.
	x := min(int(u*float64(t.w)), t.w-1)
	y := min(int((1-v)*float64(t.h)), t.h-1)

	c := color.NRGBAModel.Convert(t.img.At(x, y)).(color.NRGBA)
	return engine.RGB(c.R, c.G, c.B)
}

// Package imgdata finds out what picture some untrusted bytes hold, and how big they say it
// is, before anything is decoded. A small file can claim a huge picture, so whoever reads one
// asks here first, compares the size with what it is willing to spend, and decodes only then.
//
// It is shared by the packages that read pictures, and is not part of glonk's API.
package imgdata

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
)

// Open recognizes a JPEG or a PNG by how data starts, whatever the file was called, and reads
// the size its header claims without decoding the picture. It returns that size, and decode,
// which decodes the whole picture and is where the memory is spent. ok is false for any other
// kind of data, and for a header that cannot be read or claims no pixels.
//
// That the header is sound does not mean the picture is: decode can still fail, for a
// picture that is cut short or damaged, and what it returns can be smaller than the header
// claimed.
func Open(data []byte) (width, height int, decode func() (image.Image, error), ok bool) {
	var (
		config func(io.Reader) (image.Config, error)
		read   func(io.Reader) (image.Image, error)
	)
	switch {
	case bytes.HasPrefix(data, []byte("\xff\xd8")):
		config, read = jpeg.DecodeConfig, jpeg.Decode
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		config, read = png.DecodeConfig, png.Decode
	default:
		return 0, 0, nil, false
	}

	cfg, err := config(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, nil, false
	}
	return cfg.Width, cfg.Height, func() (image.Image, error) { return read(bytes.NewReader(data)) }, true
}

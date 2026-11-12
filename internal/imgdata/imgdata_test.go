package imgdata

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"runtime"
	"testing"
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
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("unable to encode a JPEG: %v", err)
	}
	return buf.Bytes()
}

// picture is a w by h picture whose pixels differ from one another.
func picture(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(10 * x), G: uint8(20 * y), B: uint8(x + y), A: 255})
		}
	}
	return img
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

func TestOpenFindsAPictureByItsContentAndReadsItsSize(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		w, h int
	}{
		{"PNG", encodePNG(t, picture(7, 3)), 7, 3},
		{"PNG, one pixel", encodePNG(t, picture(1, 1)), 1, 1},
		{"JPEG", encodeJPEG(t, picture(16, 8)), 16, 8},
		{"JPEG, one pixel", encodeJPEG(t, picture(1, 1)), 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h, decode, ok := Open(tt.data)
			if !ok || w != tt.w || h != tt.h {
				t.Fatalf("wanted (%d, %d, true), got (%d, %d, %v)", tt.w, tt.h, w, h, ok)
			}
			img, err := decode()
			if err != nil {
				t.Fatalf("wanted the picture to decode, got %v", err)
			}
			if b := img.Bounds(); b.Dx() != tt.w || b.Dy() != tt.h {
				t.Errorf("wanted a decoded picture of %d by %d, got %v", tt.w, tt.h, b)
			}
		})
	}
}

func TestOpenDecodesAPNGExactly(t *testing.T) {
	want := picture(5, 4)
	_, _, decode, ok := Open(encodePNG(t, want))
	if !ok {
		t.Fatal("wanted a PNG to open")
	}
	img, err := decode()
	if err != nil {
		t.Fatal(err)
	}
	for y := range 4 {
		for x := range 5 {
			if got := color.NRGBAModel.Convert(img.At(x, y)); got != want.NRGBAAt(x, y) {
				t.Fatalf("pixel (%d, %d): wanted %v, got %v", x, y, want.NRGBAAt(x, y), got)
			}
		}
	}
}

func TestOpenCanBeCalledAndDecodedMoreThanOnce(t *testing.T) {
	data := encodePNG(t, picture(3, 3))
	_, _, decode, _ := Open(data)
	for range 2 {
		if _, err := decode(); err != nil {
			t.Errorf("wanted every call to decode to work, got %v", err)
		}
	}
}

func TestOpenRefusesWhatIsNotAPicture(t *testing.T) {
	good := encodePNG(t, picture(8, 8))
	tests := map[string][]byte{
		"nil":                 nil,
		"empty":               {},
		"text":                []byte("not a picture"),
		"GIF":                 []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"BMP":                 []byte("BM\x1a\x00\x00\x00\x00\x00\x00\x00\x1a\x00\x00\x00"),
		"PNG signature alone": []byte("\x89PNG\r\n\x1a\n"),
		"JPEG marker alone":   {0xFF, 0xD8},
		"PNG, no pixels wide": pngHeader(0, 5),
		"PNG, no pixels tall": pngHeader(5, 0),
		"PNG cut inside IHDR": good[:20],
		"JPEG marker, text":   []byte("\xff\xd8 and then some text"),
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if w, h, decode, ok := Open(data); ok || w != 0 || h != 0 || decode != nil {
				t.Errorf("wanted nothing opened, got (%d, %d, %v, %v)", w, h, decode != nil, ok)
			}
		})
	}
}

// The size comes from the header alone, so a file can say how big it is without being
// decoded, and a caller can refuse it before a byte of it is spent.
//
// Where an int has 32 bits the standard library refuses a PNG header of 2^28 pixels or more
// itself ("dimension overflow"), so there Open says nothing of those, and they are only
// tried where an int has 64.
func TestOpenReadsTheSizeWithoutDecoding(t *testing.T) {
	sizes := [][2]uint32{{16_000, 16_000}} // 256 million pixels, 1 GB as 8-bit RGBA
	if math.MaxInt > math.MaxInt32 {
		sizes = append(sizes, [2]uint32{100_000, 100_000}, [2]uint32{1<<31 - 1, 1}) // 40 GB, 2 GB
	}

	for _, size := range sizes {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		w, h, decode, ok := Open(pngHeader(size[0], size[1]))
		runtime.ReadMemStats(&after)

		if !ok || uint32(w) != size[0] || uint32(h) != size[1] {
			t.Fatalf("wanted (%d, %d, true), got (%d, %d, %v)", size[0], size[1], w, h, ok)
		}
		if spent := after.TotalAlloc - before.TotalAlloc; spent > 1<<20 {
			t.Errorf("%d by %d: wanted opening to allocate nothing like the size of the picture, got %d bytes", size[0], size[1], spent)
		}
		// The header is all there is, so the picture itself does not decode.
		if _, err := decode(); err == nil {
			t.Errorf("%d by %d: wanted a header with no picture after it not to decode", size[0], size[1])
		}
	}
}

func TestOpenSaysNothingOfWhetherThePictureIsWhole(t *testing.T) {
	good := encodePNG(t, picture(32, 32))
	goodJPEG := encodeJPEG(t, picture(32, 32))
	corrupt := bytes.Clone(good)
	corrupt[len(corrupt)/2] ^= 0xFF

	tests := map[string][]byte{
		"PNG cut after IHDR":  good[:33],
		"PNG cut short":       good[:len(good)-20],
		"PNG, damaged":        corrupt,
		"JPEG cut short":      goodJPEG[:len(goodJPEG)/2],
		"JPEG cut after SOF":  goodJPEG[:len(goodJPEG)/4],
		"PNG with no image":   pngHeader(32, 32),
		"PNG header and junk": append(pngHeader(32, 32), "junk"...),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, decode, ok := Open(data)
			if !ok {
				return // refused already, which is also fine
			}
			if img, err := decode(); err == nil {
				t.Errorf("wanted a picture that is not whole not to decode, got %v", img.Bounds())
			}
		})
	}
}

// Whatever the bytes are, Open does not panic, a picture it opens has a size, and decoding
// one that is small enough to be worth it either fails or gives the size the header gave.
func FuzzOpen(f *testing.F) {
	img := picture(4, 3)
	var p, j bytes.Buffer
	png.Encode(&p, img)
	jpeg.Encode(&j, img, nil)
	for _, seed := range [][]byte{p.Bytes(), j.Bytes(), pngHeader(100_000, 100_000), pngHeader(0, 1), pngHeader(1<<31-1, 1), {0xFF, 0xD8}, []byte("\x89PNG\r\n\x1a\n"), nil} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		w, h, decode, ok := Open(data)
		if !ok {
			if w != 0 || h != 0 || decode != nil {
				t.Fatalf("wanted nothing when not ok, got (%d, %d, %v)", w, h, decode != nil)
			}
			return
		}
		if w <= 0 || h <= 0 || decode == nil {
			t.Fatalf("wanted a size and a decoder when ok, got (%d, %d, %v)", w, h, decode != nil)
		}
		if int64(w)*int64(h) > 1<<16 {
			return // too large to decode in a test, as a real caller would also refuse
		}
		if img, err := decode(); err == nil && (img.Bounds().Dx() != w || img.Bounds().Dy() != h) {
			t.Fatalf("wanted a decoded picture of %d by %d as the header said, got %v", w, h, img.Bounds())
		}
	})
}

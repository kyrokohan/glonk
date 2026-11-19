package engine

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
)

var updateImage = flag.Bool("update", false, "rewrite the pictures in testdata")

// px is the pixel at (x, y) of img.
func px(img *image.RGBA, x, y int) color.RGBA { return img.RGBAAt(x, y) }

// rgba is the opaque color with these channels.
func rgba(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 255} }

var (
	lightGray = rgba(229, 229, 229)
	black     = rgba(0, 0, 0)
)

// oneCell returns a 1 by 1 frame holding cell.
func oneCell(t *testing.T, cell Cell) *FrameBuffer {
	t.Helper()
	fb := mustFrame(t, 1, 1)
	fb.Set(0, 0, cell)
	return fb
}

func mustFrame(t testing.TB, w, h int) *FrameBuffer {
	t.Helper()
	fb, err := NewFrameBuffer(w, h)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

// art reads a picture drawn in characters: '.' is bg, 'F' is fg, from the pixels of img.
func art(img *image.RGBA, fg, bg color.RGBA) string {
	var rows []string
	for y := range img.Bounds().Dy() {
		var sb strings.Builder
		for x := range img.Bounds().Dx() {
			switch p := px(img, x, y); p {
			case fg:
				sb.WriteByte('F')
			case bg:
				sb.WriteByte('.')
			default:
				sb.WriteByte('?')
			}
		}
		rows = append(rows, sb.String())
	}
	return strings.Join(rows, "\n")
}

// A frame made from a Canvas comes out pixel for pixel: at Scale 1 the picture is the canvas, and
// at a larger scale each pixel is a block of one color. With Seamless, a cell is one color,
// whichever of its two pixels is nearer.
func TestImageOfAComposedCanvasIsTheCanvas(t *testing.T) {
	rng := rand.New(rand.NewPCG(81, 82))
	pick := func() Color {
		switch rng.IntN(4) {
		case 0:
			return Color{} // empty
		case 1:
			return Palette(uint8(rng.IntN(256)))
		}
		return RGB(uint8(rng.IntN(256)), uint8(rng.IntN(256)), uint8(rng.IntN(256)))
	}
	want := func(c Color) color.RGBA {
		if c.IsDefault() {
			return black
		}
		r, g, b := c.RGB()
		return rgba(r, g, b)
	}

	for trial := range 60 {
		w, h := 1+rng.IntN(7), 1+rng.IntN(5)
		fb := mustFrame(t, w, h)
		canvas := fb.Canvas()
		canvas.Seamless = trial%2 == 1
		for y := range canvas.Height() {
			for x := range canvas.Width() {
				canvas.Set(x, y, pick())
			}
		}
		canvas.Compose()

		for _, scale := range []int{1, 3} {
			img := fb.Image(ImageOptions{Scale: scale})
			if img.Bounds().Dx() != w*scale || img.Bounds().Dy() != 2*h*scale {
				t.Fatalf("wanted %d by %d pixels, got %v", w*scale, 2*h*scale, img.Bounds())
			}
			for y := range 2 * h {
				for x := range w {
					if !canvas.Seamless {
						// Each pixel is its own, and an empty one is the background.
						pixel := canvas.At(x, y)
						for dy := range scale {
							for dx := range scale {
								if got := px(img, x*scale+dx, y*scale+dy); got != want(pixel) {
									t.Fatalf("trial %d scale %d: wanted pixel (%d,%d) %v, got %v", trial, scale, x, y, want(pixel), got)
								}
							}
						}
						continue
					}
					// Seamless: both pixels of the cell show one color, which is one of the two.
					top, bottom := canvas.At(x, y&^1), canvas.At(x, y|1)
					got := px(img, x*scale, y*scale)
					if top.IsDefault() && bottom.IsDefault() {
						if got != black {
							t.Fatalf("trial %d: wanted an empty cell to be the background, got %v", trial, got)
						}
					} else if got != want(top) && got != want(bottom) {
						t.Fatalf("trial %d: wanted a seamless cell to be one of its pixels, got %v of %v and %v", trial, got, want(top), want(bottom))
					}
					if other := px(img, x*scale, (y^1)*scale); other != got {
						t.Fatalf("trial %d: wanted both halves of a seamless cell the same, got %v and %v", trial, got, other)
					}
				}
			}
		}
	}
}

// Each kind of character, with a foreground F and a background B, as a cell of 1, 2, 3, 4 and 8.
func TestImageDrawsEachKindOfCell(t *testing.T) {
	fgc, bgc := RGB(200, 30, 60), RGB(10, 20, 90)
	f, b := rgba(200, 30, 60), rgba(10, 20, 90)
	cell := func(r rune) Cell { return Cell{Character: r, Foreground: fgc, Background: bgc} }

	// The mark, written out: the middle half of the cell across and down.
	marks := map[int]string{
		1: "F\nF",
		2: "..\nFF\nFF\n..",
		3: "...\nFFF\nFFF\nFFF\nFFF\n...",
		4: "....\n....\n.FF.\n.FF.\n.FF.\n.FF.\n....\n....",
		8: strings.Repeat("........\n", 4) + strings.Repeat("..FFFF..\n", 8) + strings.TrimSuffix(strings.Repeat("........\n", 4), "\n"),
	}
	solid := func(top, bottom string, s int) string {
		return strings.TrimSuffix(strings.Repeat(strings.Repeat(top, s)+"\n", s)+strings.Repeat(strings.Repeat(bottom, s)+"\n", s), "\n")
	}

	for _, s := range []int{1, 2, 3, 4, 8} {
		for name, tt := range map[string]struct {
			r    rune
			want string
		}{
			"space":            {' ', solid(".", ".", s)},
			"nul":              {0, solid(".", ".", s)},
			"upper half":       {'▀', solid("F", ".", s)},
			"lower half":       {'▄', solid(".", "F", s)},
			"full block":       {'█', solid("F", "F", s)},
			"a letter":         {'H', marks[s]},
			"a digit":          {'7', marks[s]},
			"a box line":       {'─', marks[s]},
			"a shade":          {'▒', marks[s]},
			"a control":        {'\n', marks[s]}, // shown as ?
			"a wide character": {'世', marks[s]},  // shown as ?
		} {
			img := oneCell(t, cell(tt.r)).Image(ImageOptions{Scale: s})
			if got := art(img, f, b); got != tt.want {
				t.Errorf("%s at scale %d: wanted\n%s\ngot\n%s", name, s, tt.want, got)
			}
		}
	}
}

// letterArt is the picture of one cell of the given scale holding r with Text, drawn out from the
// font's own source in glyphArt: the letter at scale/6 times its size, its top left at half the room
// that is left across and down, rounded down, on the background.
func letterArt(r rune, scale int) string {
	size := scale / 6
	glyph := strings.Split(strings.TrimPrefix(glyphArt[r-' '], "\n"), "\n")
	left, top := (scale-5*size)/2, (2*scale-7*size)/2
	rows := make([]string, 2*scale)
	for y := range rows {
		row := []byte(strings.Repeat(".", scale))
		if gy := y - top; gy >= 0 && gy < 7*size {
			for x := range row {
				if gx := x - left; gx >= 0 && gx < 5*size && glyph[gy/size][gx/size] == '#' {
					row[x] = 'F'
				}
			}
		}
		rows[y] = string(row)
	}
	return strings.Join(rows, "\n")
}

// markArt is the mark of one cell of the given scale: the middle half across and down.
func markArt(scale int) string {
	rows := make([]string, 2*scale)
	for y := range rows {
		row := []byte(strings.Repeat(".", scale))
		if y >= scale/2 && y < 2*scale-scale/2 {
			for x := scale / 4; x < scale-scale/4; x++ {
				row[x] = 'F'
			}
		}
		rows[y] = string(row)
	}
	return strings.Join(rows, "\n")
}

// With Text, a printable ASCII character is its letter from Scale 6 up, a wide or control character
// is a question mark, and anything else outside ASCII, or any character below Scale 6, is a mark.
func TestImageTextDrawsTheLetters(t *testing.T) {
	fgc, bgc := RGB(200, 30, 60), RGB(10, 20, 90)
	f, b := rgba(200, 30, 60), rgba(10, 20, 90)
	draw := func(r rune, scale int, attrs Attr) *image.RGBA {
		return oneCell(t, Cell{Character: r, Foreground: fgc, Background: bgc, Attrs: attrs}).Image(ImageOptions{Scale: scale, Text: true})
	}

	for _, scale := range []int{6, 7, 12, 13, 18} {
		for _, r := range "Hg7!~" {
			if got, want := art(draw(r, scale, 0), f, b), letterArt(r, scale); got != want {
				t.Errorf("%q at scale %d: wanted\n%s\ngot\n%s", r, scale, want, got)
			}
		}
		for _, r := range "\n世" { // a control and a wide character, which the terminal is sent as ?
			if got, want := art(draw(r, scale, 0), f, b), letterArt('?', scale); got != want {
				t.Errorf("%q at scale %d: wanted a question mark\n%s\ngot\n%s", r, scale, want, got)
			}
		}
		for _, r := range "é─▒" {
			if got, want := art(draw(r, scale, 0), f, b), markArt(scale); got != want {
				t.Errorf("%q at scale %d: wanted a mark\n%s\ngot\n%s", r, scale, want, got)
			}
		}
		// Reverse swaps the colors of the letter too.
		if got, want := art(draw('H', scale, Reverse), b, f), letterArt('H', scale); got != want {
			t.Errorf("reversed at scale %d: wanted the letter in the background color\n%s\ngot\n%s", scale, want, got)
		}
	}
	for _, scale := range []int{1, 4, 5} {
		if got, want := art(draw('H', scale, 0), f, b), markArt(scale); got != want {
			t.Errorf("scale %d: wanted a mark, with no room for a letter\n%s\ngot\n%s", scale, want, got)
		}
	}
}

func TestImageReverseSwapsTheColorsAndOtherAttributesDoNothing(t *testing.T) {
	cell := Cell{Character: '▀', Foreground: RGB(200, 30, 60), Background: RGB(10, 20, 90)}
	plain := oneCell(t, cell).Image(ImageOptions{Scale: 2})

	reversed := cell
	reversed.Attrs = Reverse
	got := oneCell(t, reversed).Image(ImageOptions{Scale: 2})
	if px(got, 0, 0) != rgba(10, 20, 90) || px(got, 0, 2) != rgba(200, 30, 60) {
		t.Errorf("wanted the colors swapped, got %v over %v", px(got, 0, 0), px(got, 0, 2))
	}

	for _, a := range []Attr{Bold, Dim, Underline, Bold | Dim | Underline} {
		withAttr := cell
		withAttr.Attrs = a
		if img := oneCell(t, withAttr).Image(ImageOptions{Scale: 2}); !bytes.Equal(img.Pix, plain.Pix) {
			t.Errorf("wanted attribute %v not to change the picture", a)
		}
	}

	// Reverse with the default colors swaps what they look like.
	d := oneCell(t, Cell{Character: ' ', Attrs: Reverse}).Image(ImageOptions{})
	if px(d, 0, 0) != lightGray {
		t.Errorf("wanted a reversed blank to show the default foreground, got %v", px(d, 0, 0))
	}
}

func TestImageDefaultColorsAndTheirOverrides(t *testing.T) {
	blankWithText := func(fg, bg Color) Cell { return Cell{Character: '█', Foreground: fg, Background: bg} }
	for name, tt := range map[string]struct {
		opts     ImageOptions
		cell     Cell
		wantAt00 color.RGBA // a pixel of the full block, which is the foreground
		wantBg   color.RGBA // a pixel of a blank in the same colors
	}{
		"defaults":                             {ImageOptions{}, blankWithText(Color{}, Color{}), lightGray, black},
		"explicit colors are not the defaults": {ImageOptions{Foreground: RGB(1, 2, 3), Background: RGB(4, 5, 6)}, blankWithText(RGB(9, 8, 7), RGB(7, 8, 9)), rgba(9, 8, 7), rgba(7, 8, 9)},
		"an RGB default":                       {ImageOptions{Foreground: RGB(1, 2, 3), Background: RGB(4, 5, 6)}, blankWithText(Color{}, Color{}), rgba(1, 2, 3), rgba(4, 5, 6)},
		"a palette default":                    {ImageOptions{Foreground: Palette(196), Background: Palette(21)}, blankWithText(Color{}, Color{}), rgba(255, 0, 0), rgba(0, 0, 255)},
		"a default of default":                 {ImageOptions{Foreground: Color{}, Background: Color{}}, blankWithText(Color{}, Color{}), lightGray, black},
		"black is not the default":             {ImageOptions{Foreground: RGB(1, 2, 3)}, blankWithText(Palette(16), Palette(16)), black, black},
	} {
		full := oneCell(t, tt.cell).Image(tt.opts)
		blank := tt.cell
		blank.Character = ' '
		empty := oneCell(t, blank).Image(tt.opts)
		if px(full, 0, 0) != tt.wantAt00 || px(empty, 0, 0) != tt.wantBg {
			t.Errorf("%s: wanted foreground %v and background %v, got %v and %v", name, tt.wantAt00, tt.wantBg, px(full, 0, 0), px(empty, 0, 0))
		}
	}
}

func TestImageColorModes(t *testing.T) {
	rng := rand.New(rand.NewPCG(83, 84))
	for range 300 {
		r, g, b := uint8(rng.IntN(256)), uint8(rng.IntN(256)), uint8(rng.IntN(256))
		cell := Cell{Character: ' ', Background: RGB(r, g, b)}
		exact := rgba(r, g, b)
		for _, mode := range []ColorMode{ColorsAuto, ColorsTrue} {
			if got := px(oneCell(t, cell).Image(ImageOptions{Colors: mode}), 0, 0); got != exact {
				t.Fatalf("mode %d: wanted %v exactly, got %v", mode, exact, got)
			}
		}
		// The nearest of the palette colors 16 to 255: one as near as the scan finds.
		got := px(oneCell(t, cell).Image(ImageOptions{Colors: Colors256}), 0, 0)
		scan := nearestPaletteScan(float64(r), float64(g), float64(b))
		wantRGB := rgba(uint8(paletteRGB[scan][0]), uint8(paletteRGB[scan][1]), uint8(paletteRGB[scan][2]))
		dist := func(c color.RGBA) float64 {
			dr, dg, db := float64(c.R)-float64(r), float64(c.G)-float64(g), float64(c.B)-float64(b)
			return dr*dr + dg*dg + db*db
		}
		if dist(got) != dist(wantRGB) {
			t.Fatalf("wanted the palette color nearest (%d,%d,%d), %v, got %v", r, g, b, wantRGB, got)
		}
		inPalette := false
		for i := 16; i < 256; i++ {
			inPalette = inPalette || got == rgba(uint8(paletteRGB[i][0]), uint8(paletteRGB[i][1]), uint8(paletteRGB[i][2]))
		}
		if !inPalette {
			t.Fatalf("wanted a color from the palette, got %v", got)
		}
	}

	// A palette color is the same in every mode, and the default colors are not sent, so they are not rounded.
	for _, mode := range []ColorMode{ColorsAuto, Colors256, ColorsTrue} {
		img := oneCell(t, Cell{Character: '█', Foreground: Palette(208), Background: Palette(5)}).Image(ImageOptions{Colors: mode})
		if px(img, 0, 0) != rgba(255, 135, 0) {
			t.Errorf("mode %d: wanted palette 208 as xterm has it, got %v", mode, px(img, 0, 0))
		}
		// A palette color from 0 to 15 is xterm's, not rounded to a color from 16 up.
		if got := px(oneCell(t, Cell{Character: ' ', Background: Palette(5)}).Image(ImageOptions{Colors: mode}), 0, 0); got != rgba(205, 0, 205) {
			t.Errorf("mode %d: wanted palette 5 as xterm has it, got %v", mode, got)
		}
		def := oneCell(t, Cell{Character: '█'}).Image(ImageOptions{Colors: mode, Foreground: RGB(1, 2, 3)})
		if px(def, 0, 0) != rgba(1, 2, 3) {
			t.Errorf("mode %d: wanted the default foreground as it is, got %v", mode, px(def, 0, 0))
		}
	}
}

func TestImageSize(t *testing.T) {
	for _, tt := range []struct{ w, h, scale, wantW, wantH int }{
		{1, 1, 0, 1, 2},
		{1, 1, -5, 1, 2},
		{1, 1, 1, 1, 2},
		{3, 2, 1, 3, 4},
		{3, 2, 4, 12, 16},
		{100, 30, 2, 200, 120},
		{2048, 1, 1, 2048, 2},
		{1, 1, 1448, 1448, 2896}, // 4,193,408 pixels: under the limit
		{1, 1, 1449, 0, 0},       // 4,199,202: over it
		{100, 30, 2048, 0, 0},
		{1, 1, 1 << 11, 0, 0},
		{1, 1, 1<<11 + 1, 0, 0},
		{1, 1, 1 << 30, 0, 0},
		{1, 1, int(^uint(0) >> 1), 0, 0},
	} {
		img := mustFrame(t, tt.w, tt.h).Image(ImageOptions{Scale: tt.scale})
		if img == nil || img.Bounds().Dx() != tt.wantW || img.Bounds().Dy() != tt.wantH {
			t.Errorf("%dx%d at scale %d: wanted %dx%d pixels, got %v", tt.w, tt.h, tt.scale, tt.wantW, tt.wantH, img.Bounds())
		}
	}
	// A picture that is not made allocates nothing like its size.
	if n := testing.AllocsPerRun(5, func() { mustFrame(t, 1, 1).Image(ImageOptions{Scale: 1 << 30}) }); n > 10 {
		t.Errorf("wanted a picture over the limit to cost nothing, got %v allocations", n)
	}
}

func TestImageIsOpaqueACopyAndOnlyTheCells(t *testing.T) {
	fb := mustFrame(t, 5, 3)
	fb.DrawText(0, 1, "hello", RGB(9, 9, 9))
	img := fb.Image(ImageOptions{Scale: 2})
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 255 {
			t.Fatalf("wanted every pixel opaque, got alpha %d at byte %d", img.Pix[i], i)
		}
	}

	before := bytes.Clone(img.Pix)
	fb.Clear()
	fb.DrawText(0, 0, "other", RGB(1, 1, 1))
	if !bytes.Equal(img.Pix, before) {
		t.Errorf("wanted the picture to be a copy of the frame as it was")
	}

	// Pixels set on the canvas and not composed are not in the cells, so not in the picture.
	fb.Clear()
	fb.Canvas().Set(1, 1, RGB(255, 0, 0))
	if got := px(fb.Image(ImageOptions{}), 1, 1); got != black {
		t.Errorf("wanted a canvas pixel that is not composed not to be drawn, got %v", got)
	}
	fb.Canvas().Compose()
	if got := px(fb.Image(ImageOptions{}), 1, 1); got != rgba(255, 0, 0) {
		t.Errorf("wanted a composed canvas pixel, got %v", got)
	}
}

// goldenFrame is a small frame that has everything in it: a border, text, a menu line that is
// reversed, and pixels drawn with a Canvas.
func goldenFrame(t *testing.T) *FrameBuffer {
	t.Helper()
	fb := mustFrame(t, 18, 8)
	canvas := fb.Canvas()
	canvas.Seamless = false
	canvas.FillCircle(9, 7, 5, RGB(240, 140, 30))
	canvas.FillRect(2, 8, 6, 4, Palette(39))
	canvas.FillRect(12, 3, 4, 8, RGB(80, 200, 120))
	canvas.Compose()
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), Palette(244))
	fb.DrawText(2, 0, "glonk", Color{})
	fb.DrawText(1, 6, " reversed line  ", Color{})
	for x := 1; x < 17; x++ {
		cell := fb.At(x, 6)
		cell.Attrs = Reverse
		fb.Set(x, 6, cell)
	}
	return fb
}

// A picture of a frame is what is stored in testdata/frame.png, which shows the feature to anyone who
// opens it. Run with -update to write it again. The pictures are compared pixel by pixel, not as bytes,
// since the PNG encoder is the standard library's to change.
func TestImageMatchesTheStoredPicture(t *testing.T) {
	for path, opts := range map[string]ImageOptions{
		"testdata/frame.png":      {Scale: 4},
		"testdata/frame-text.png": {Scale: 12, Text: true},
	} {
		matchStoredPicture(t, path, goldenFrame(t).Image(opts))
	}
}

// matchStoredPicture checks img against the picture stored at path, after writing it there when the
// test is run with -update.
func matchStoredPicture(t *testing.T, path string, img *image.RGBA) {
	t.Helper()
	if *updateImage {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("wanted the stored picture (run with -update to write it): %v", err)
	}
	defer f.Close()
	stored, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Bounds() != img.Bounds() {
		t.Fatalf("%s: wanted a picture of %v, got %v", path, stored.Bounds(), img.Bounds())
	}
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			r, g, b, a := stored.At(x, y).RGBA()
			want := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
			if got := px(img, x, y); got != want {
				t.Fatalf("%s: wanted pixel (%d,%d) to be %v, got %v", path, x, y, want, got)
			}
		}
	}
}

// BenchmarkImage is what a screenshot costs: a 100 by 30 frame of half blocks.
func BenchmarkImage(b *testing.B) {
	fb := mustFrame(b, 100, 30)
	canvas := fb.Canvas()
	canvas.FillCircle(50, 30, 25, RGB(240, 140, 30))
	canvas.Compose()
	fb.DrawText(2, 2, "a line of text over the picture", Color{})
	for _, scale := range []int{1, 4} {
		b.Run(map[int]string{1: "scale 1", 4: "scale 4"}[scale], func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				fb.Image(ImageOptions{Scale: scale})
			}
		})
	}
}

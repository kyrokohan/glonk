// Package brand holds glonk's logo as source, and the tests that the logo files in the
// repository are what that source draws. It has no code of its own, as internal/sitecheck
// has none. BRAND.md says how the logo is used; this says what it is.
//
//	go test ./internal/brand -update
//
// writes the files again (make brand does the same).
package brand

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/mesh"
)

var update = flag.Bool("update", false, "write the logo files again from this source")

// name is the word glonk as the typeface Departure Mono draws it at its own size, one
// character a pixel: two rows for the tops of the l and the k, six for the letters, and two
// for the tail of the g.
var name = []string{
	".......###..................#....",
	".........#..................#....",
	".##.#....#.....###...#.##...#...#",
	"#..##....#....#...#..##..#..#..#.",
	"#...#....#....#...#..#...#..#.#..",
	"#...#....#....#...#..#...#..###..",
	"#..##....#....#...#..#...#..#..#.",
	".##.#..#####...###...#...#..#...#",
	"....#............................",
	".###.............................",
}

// The lockup, in pixels: the mark, a gap, then the name. The mark stands on the line the
// letters stand on and is exactly as tall as they are.
const (
	markSize = 6 // across and down: half of it lit, half of it front
	markTop  = 2 // the row where the letters g, o, n start
	gap      = 3
	clear    = 3 // what is kept empty around the lockup: half the mark
)

// oLeft and oRight are the columns of name that the letter o takes.
const oLeft, oRight = 14, 18

// A ground is one of the two things glonk is shown on, with the colors used there.
type ground struct {
	title      string
	page       engine.Color // the ground itself
	word       engine.Color // the name
	ink, quiet engine.Color // text, and text that matters less
	top, front engine.Color // the mark: its lit half over its front
}

// The mark's colors are terminal colors on both grounds, so a terminal can print it. Night is a
// terminal throughout. Paper's ground and inks are for pages, which no terminal has to show.
var (
	night = ground{"night", engine.Palette(16), engine.Palette(231), engine.Palette(254), engine.Palette(245), engine.Palette(220), engine.Palette(136)}
	paper = ground{"paper", engine.RGB(0xe7, 0xec, 0xf0), engine.RGB(0x14, 0x21, 0x3d), engine.RGB(0x14, 0x21, 0x3d), engine.RGB(0x46, 0x56, 0x7a), engine.Palette(178), engine.Palette(94)}
)

// hex is a color as a page writes it.
func hex(c engine.Color) string {
	r, g, b := c.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// frame returns a blank frame whose canvas is w by h pixels; h must be even.
func frame(t *testing.T, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h/2)
	if err != nil {
		t.Fatal(err)
	}
	return fb
}

// markFrame draws the mark as small as it goes: one row of two cells, each lit above and
// front below. Every picture of the mark alone is this frame, made larger.
func markFrame(t *testing.T, g ground) *engine.FrameBuffer {
	fb := frame(t, 2, 2)
	c := fb.Canvas()
	c.FillRect(0, 0, 2, 1, g.top)
	c.FillRect(0, 1, 2, 1, g.front)
	c.Compose()
	return fb
}

// lockupFrame draws the mark and the name on the ground, with pad pixels of it around them.
func lockupFrame(t *testing.T, g ground, pad int) *engine.FrameBuffer {
	w, h := markSize+gap+len(name[0])+2*pad, len(name)+2*pad
	fb := frame(t, w, h)
	c := fb.Canvas()
	c.FillRect(0, 0, w, h, g.page)
	c.FillRect(pad, pad+markTop, markSize, markSize/2, g.top)
	c.FillRect(pad, pad+markTop+markSize/2, markSize, markSize/2, g.front)
	for y, row := range name {
		for x, r := range row {
			if r == '#' {
				c.Set(pad+markSize+gap+x, pad+y, g.word)
			}
		}
	}
	c.Compose()
	return fb
}

// cardLines is what the card says glonk is, on two lines, as Departure Mono draws them at its
// own size: one character a pixel, ten rows a line.
var cardLines = [][]string{
	{
		"...#............#............................#..................###....................................#..................#................................................#...........................###........................###................",
		"..#.#...........#.................................................#....................................#..............................................................................................#..........................#...#...............",
		".#...#.........#####...###...##.##..##.#...###....#.##....###.....#...........##.##...###...#.##....##.#...###...##.##..###....#.##....##.#..........###...#.##....##.#..###....#.##....###..........#####...###...##.##.........#.......###.....#...",
		".#...#..........#.....#...#...##....#.#.#....#....##..#......#....#............##....#...#..##..#..#..##..#...#...##......#....##..#..#..##.........#...#..##..#..#..##....#....##..#..#...#..........#.....#...#...##...........#......#...#....#...",
		".#####..........#.....#####...#.....#.#.#....#....#...#...####....#............#.....#####..#...#..#...#..#####...#.......#....#...#..#...#.........#####..#...#..#...#....#....#...#..#####..........#.....#...#...#............#..##..#...#........",
		".#...#..........#.....#.......#.....#.#.#....#....#...#..#...#....#............#.....#......#...#..#...#..#.......#.......#....#...#..#...#.........#......#...#..#...#....#....#...#..#..............#.....#...#...#............#...#..#...#........",
		".#...#..........#..#..#...#...#.....#.#.#....#....#...#..#..##....#............#.....#...#..#...#..#..##..#...#...#.......#....#...#..#..##.........#...#..#...#..#..##....#....#...#..#...#..........#.....#...#...#............#...#..#...#....#...",
		".#...#...........##....###...####...#.#.#..#####..#...#...##.#..#####.........####....###...#...#...##.#...###...####...#####..#...#...##.#..........###...#...#...##.#..#####..#...#...###..........####....###...####...........###....###.....#...",
		"..........................................................................................................................................#...........................#..............................................................................",
		".......................................................................................................................................###.........................###...............................................................................",
	},
	{
		"..###...####..................###...####......................................#..............#.....#...............#................................................###....................................",
		".#...#..#...#................#...#..#...#.....................................#..............#.....#...............................................................#.......................................",
		".....#..#...#....................#..#...#.................###...##.##.........#.##....###...#####..#.##..........###....#.##...........###...#.##....###..........#####..##.##...###...##.#....###.........",
		"....#...#...#..................##...#...#................#...#...##...........##..#..#...#...#.....##..#...........#....##..#.........#...#..##..#..#...#..........#......##........#..#.#.#..#...#........",
		"...#....#...#....................#..#...#................#...#...#............#...#..#...#...#.....#...#...........#....#...#.........#...#..#...#..#####..........#......#......####..#.#.#..#####........",
		"..#.....#...#....................#..#...#................#...#...#............#...#..#...#...#.....#...#...........#....#...#.........#...#..#...#..#..............#......#.....#...#..#.#.#..#............",
		".#......#...#....#...........#...#..#...#....#...........#...#...#............##..#..#...#...#..#..#...#...........#....#...#.........#...#..#...#..#...#..........#......#.....#..##..#.#.#..#...#....#...",
		".#####..####.....#............###...####.....#............###...####..........#.##....###.....##...#...#.........#####..#...#..........###...#...#...###..........####...####....##.#..#.#.#...###.....#...",
		"................#...........................#..............................................................................................................................................................",
		"...........................................................................................................................................................................................................",
	},
}

// linesFrame draws one line of text, given as rows of pixels, on the ground.
func linesFrame(t *testing.T, g ground, rows []string) *engine.FrameBuffer {
	fb := frame(t, len(rows[0]), len(rows))
	c := fb.Canvas()
	c.FillRect(0, 0, len(rows[0]), len(rows), g.page)
	for y, row := range rows {
		for x, r := range row {
			if r == '#' {
				c.Set(x, y, g.ink)
			}
		}
	}
	c.Compose()
	return fb
}

// The card is the picture shown beside a link to glonk: the logo, what glonk is, and something
// it drew. It is 1280 by 640, the size GitHub asks for, with 80 pixels kept clear at the sides.
const (
	cardW, cardH = 1280, 640
	cardMargin   = 80
	sceneScale   = 8 // a pixel of the scene is this many of the card, so that it reads as a pixel
)

// blockScene draws glonk's own work for the card and for the first step of the site: a gold block
// lit from above on a floor of lines, with grey blocks behind it that fade into the night. Every
// game starts with a block. It is w by h pixels, seen through a camera with that field of view.
func blockScene(t *testing.T, w, h int, fov float64) *engine.FrameBuffer {
	fb := frame(t, w, h)
	c := fb.Canvas()
	cam := mesh.Camera{
		Position: geometry.Vector3{Y: 2.1, Z: -7.6}, Pitch: -0.2, FOV: fov,
		Fog: &mesh.Fog{Color: night.page, Start: 7, End: 20},
	}
	lights := []mesh.Light{mesh.Ambient{Level: 0.44}, mesh.Directional{Direction: geometry.Vector3{X: 0.25, Y: -1, Z: 0.5}}}
	block := func(x, z float64, color engine.Color) {
		m := mesh.Cube(2)
		m.Transform.Position = geometry.Vector3{X: x, Z: z}
		m.Material.Color = color
		m.DrawShadedPixels(c, cam, lights...)
	}
	floor := mesh.Grid(48, 24)
	floor.Transform.Position.Y = -1
	floor.DrawDepthPixels(c, cam, engine.Palette(236))
	for _, at := range [][2]float64{{-5, 5}, {4.4, 3.4}, {-2.6, 9.5}, {7.5, 11}, {1.5, 14}, {-9, 12}} {
		block(at[0], at[1], engine.Palette(242))
	}
	block(0, 0, night.top)
	c.Compose()
	return fb
}

// card puts the three together, each made larger by a whole number.
func card(t *testing.T) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, cardW, cardH))
	r, g, b := night.page.RGB()
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{R: r, G: g, B: b, A: 255}), image.Point{}, draw.Src)
	place := func(fb *engine.FrameBuffer, scale, x, y int) {
		part := fb.Image(engine.ImageOptions{Scale: scale})
		draw.Draw(img, part.Bounds().Add(image.Pt(x, y)), part, image.Point{}, draw.Src)
	}
	scene := blockScene(t, cardW/sceneScale, 40, 0.56)
	place(scene, sceneScale, 0, cardH-2*scene.Height()*sceneScale)
	place(lockupFrame(t, night, 0), 12, cardMargin, 64)
	place(linesFrame(t, night, cardLines[0]), 4, cardMargin, 216)
	place(linesFrame(t, night, cardLines[1]), 4, cardMargin, 264)
	return img
}

// picture is a frame as a PNG, scale pixels of the picture to one of the frame.
func picture(t *testing.T, fb *engine.FrameBuffer, scale int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, fb.Image(engine.ImageOptions{Scale: scale})); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// rect is one rectangle of an SVG path, closed.
func rect(x, y, w, h int) string { return fmt.Sprintf("M%d %dh%dv%dh%dz", x, y, w, h, -w) }

// namePath is the name as one SVG path, moved right by dx: each run of pixels in a row is a
// rectangle. One path and not many shapes, so that no renderer leaves a hairline between them.
func namePath(dx int) string {
	var d strings.Builder
	for y, row := range name {
		for x := 0; x < len(row); x++ {
			if row[x] != '#' {
				continue
			}
			start := x
			for x < len(row) && row[x] == '#' {
				x++
			}
			d.WriteString(rect(dx+start, y, x-start, 1))
		}
	}
	return d.String()
}

// lockupSVG is the mark and the name with nothing behind them, for a page of that ground.
func lockupSVG(g ground) []byte {
	w, h := markSize+gap+len(name[0]), len(name)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-label="glonk" shape-rendering="crispEdges">`+"\n", w, h, 8*w, 8*h)
	b.WriteString("<title>glonk</title>\n")
	fmt.Fprintf(&b, `<path fill="%s" d="%s"/>`+"\n", hex(g.top), rect(0, markTop, markSize, markSize/2))
	fmt.Fprintf(&b, `<path fill="%s" d="%s"/>`+"\n", hex(g.front), rect(0, markTop+markSize/2, markSize, markSize/2))
	fmt.Fprintf(&b, `<path fill="%s" d="%s"/>`+"\n", hex(g.word), namePath(markSize+gap))
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// markSVG is the mark alone in one ground's colors.
func markSVG(g ground) []byte {
	return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2 2" width="64" height="64" role="img" aria-label="glonk" shape-rendering="crispEdges">
<title>glonk</title>
<path fill="%s" d="%s"/>
<path fill="%s" d="%s"/>
</svg>
`, hex(g.top), rect(0, 0, 2, 1), hex(g.front), rect(0, 1, 2, 1))
}

// iconSVG is the mark for a browser tab: paper's colors, and night's where the visitor's
// system is dark.
func iconSVG() []byte {
	return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2 2" shape-rendering="crispEdges">
<title>glonk</title>
<style>.t{fill:%s}.f{fill:%s}@media (prefers-color-scheme:dark){.t{fill:%s}.f{fill:%s}}</style>
<path class="t" d="%s"/>
<path class="f" d="%s"/>
</svg>
`, hex(paper.top), hex(paper.front), hex(night.top), hex(night.front), rect(0, 0, 2, 1), rect(0, 1, 2, 1))
}

// icoSizes are the pictures favicon.ico holds, for browsers that do not take an SVG.
var icoSizes = []int{16, 32}

// ico packs PNG pictures, each size pixels square, into one .ico file.
func ico(sizes []int, pngs [][]byte) []byte {
	var b bytes.Buffer
	le := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	le([3]uint16{0, 1, uint16(len(sizes))}) // reserved, 1 for an icon, how many pictures
	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		le([4]uint8{uint8(size), uint8(size), 0, 0}) // width, height, no palette, reserved
		le([2]uint16{1, 32})                         // color planes, bits per pixel
		le([2]uint32{uint32(len(pngs[i])), uint32(offset)})
		offset += len(pngs[i])
	}
	for _, p := range pngs {
		b.Write(p)
	}
	return b.Bytes()
}

// icoPictures reads a .ico file back as the pictures in it.
func icoPictures(t *testing.T, data []byte) []image.Image {
	t.Helper()
	if len(data) < 6 {
		t.Fatalf("wanted an .ico header of 6 bytes, got %d bytes in all", len(data))
	}
	if kind := binary.LittleEndian.Uint16(data[2:]); kind != 1 {
		t.Fatalf("wanted type 1, an icon, got %d", kind)
	}
	n := int(binary.LittleEndian.Uint16(data[4:]))
	var out []image.Image
	for i := range n {
		e := 6 + 16*i
		if len(data) < e+16 {
			t.Fatalf("wanted an entry for picture %d, the file ends before it", i)
		}
		size, at := int(binary.LittleEndian.Uint32(data[e+8:])), int(binary.LittleEndian.Uint32(data[e+12:]))
		if at < 0 || size < 0 || at+size > len(data) {
			t.Fatalf("picture %d is said to be %d bytes at %d, in a file of %d", i, size, at, len(data))
		}
		img, err := png.Decode(bytes.NewReader(data[at : at+size]))
		if err != nil {
			t.Fatalf("picture %d: %v", i, err)
		}
		if w, h := int(data[e]), int(data[e+1]); img.Bounds().Dx() != w || img.Bounds().Dy() != h {
			t.Fatalf("picture %d: its entry says %d by %d, the picture is %v", i, w, h, img.Bounds())
		}
		out = append(out, img)
	}
	return out
}

// A file of the repository that this source draws: where it is, from the root, and its bytes.
type file struct {
	path string
	draw func(t *testing.T) []byte
}

var files = []file{
	{"brand/glonk-night.svg", func(*testing.T) []byte { return lockupSVG(night) }},
	{"brand/glonk-paper.svg", func(*testing.T) []byte { return lockupSVG(paper) }},
	{"brand/mark.svg", func(*testing.T) []byte { return markSVG(night) }},
	{"brand/glonk-night.png", func(t *testing.T) []byte { return picture(t, lockupFrame(t, night, clear), 16) }},
	{"brand/glonk-paper.png", func(t *testing.T) []byte { return picture(t, lockupFrame(t, paper, clear), 16) }},
	{"brand/mark.png", func(t *testing.T) []byte { return picture(t, markFrame(t, night), 256) }},
	{"site/favicon.svg", func(*testing.T) []byte { return iconSVG() }},
	// 140 by 56 pixels at 6 times: shown at half that on the site, where a pixel is then 3 across
	// and the picture 30 cells by 6 rows.
	{"site/block.png", func(t *testing.T) []byte { return picture(t, blockScene(t, 140, 56, 0.72), 6) }},
	{"site/card.png", func(t *testing.T) []byte {
		var buf bytes.Buffer
		if err := png.Encode(&buf, card(t)); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}},
	{"site/apple-touch-icon.png", func(t *testing.T) []byte { return picture(t, markFrame(t, night), 90) }},
	{"site/favicon.ico", func(t *testing.T) []byte {
		var pngs [][]byte
		for _, size := range icoSizes {
			pngs = append(pngs, picture(t, markFrame(t, night), size/2))
		}
		return ico(icoSizes, pngs)
	}},
}

// root is the path of a file of the repository, given from its root.
func root(path string) string { return filepath.Join("..", "..", filepath.FromSlash(path)) }

// samePicture fails unless two pictures have the same size and the same pixels.
func samePicture(t *testing.T, what string, got, want image.Image) {
	t.Helper()
	if got.Bounds() != want.Bounds() {
		t.Fatalf("%s: wanted a picture of %v, got %v", what, want.Bounds(), got.Bounds())
	}
	b := want.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gr, gg, gb, ga := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb || ga != wa {
				t.Fatalf("%s: wanted pixel (%d,%d) to be %v, got %v", what, x, y, want.At(x, y), got.At(x, y))
			}
		}
	}
}

// decode reads a PNG.
func decode(t *testing.T, what string, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	return img
}

// The logo files are what this source draws. Text is compared byte for byte. Pictures are
// compared pixel by pixel and not as bytes, since the PNG encoder is the standard library's to
// change. Run with -update to write them again.
func TestTheFilesAreWhatTheSourceDraws(t *testing.T) {
	for _, f := range files {
		want := f.draw(t)
		if *update {
			if err := os.MkdirAll(filepath.Dir(root(f.path)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(root(f.path), want, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got, err := os.ReadFile(root(f.path))
		if err != nil {
			t.Fatalf("wanted %s (run with -update to write it): %v", f.path, err)
		}
		switch filepath.Ext(f.path) {
		case ".png":
			samePicture(t, f.path, decode(t, f.path, got), decode(t, f.path, want))
		case ".ico":
			gotPictures, wantPictures := icoPictures(t, got), icoPictures(t, want)
			if len(gotPictures) != len(wantPictures) {
				t.Fatalf("%s: wanted %d pictures, got %d", f.path, len(wantPictures), len(gotPictures))
			}
			for i := range wantPictures {
				samePicture(t, fmt.Sprintf("%s, picture %d", f.path, i), gotPictures[i], wantPictures[i])
			}
		default:
			// A checkout on Windows may have turned the ends of the lines into CRLF.
			got = bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n"))
			if !bytes.Equal(got, want) {
				t.Errorf("%s is not what the source draws (run with -update to write it):\ngot  %q\nwant %q", f.path, got, want)
			}
		}
	}
}

// The mark is two cells of the upper half block, lit above and front below: what a terminal
// prints for it, and the reason it can be the logo of a terminal engine.
func TestTheMarkIsTwoUpperHalfBlocks(t *testing.T) {
	fb := markFrame(t, night)
	if got := fb.String(); got != "▀▀" {
		t.Fatalf("wanted the mark to be two upper half blocks, got %q", got)
	}
	for x := range 2 {
		c := fb.At(x, 0)
		if c.Foreground != night.top || c.Background != night.front {
			t.Errorf("cell %d: wanted the lit color over the front color, got %v over %v", x, c.Foreground, c.Background)
		}
	}
}

// In the lockup the mark is exactly as tall as the letters that have no stem, and stands
// where they stand.
func TestTheMarkIsAsTallAsTheLetters(t *testing.T) {
	first, last := -1, -1
	for y, row := range name {
		if strings.Contains(row[oLeft:oRight+1], "#") {
			if first < 0 {
				first = y
			}
			last = y
		}
	}
	if first != markTop || last != markTop+markSize-1 {
		t.Errorf("wanted the mark on rows %d to %d, where the o is; the o is on rows %d to %d", markTop, markTop+markSize-1, first, last)
	}
	if markSize%2 != 0 {
		t.Errorf("wanted a mark that splits into two equal halves, got one %d tall", markSize)
	}
	for y, row := range name {
		if len(row) != len(name[0]) {
			t.Errorf("row %d of the name is %d wide, wanted %d", y, len(row), len(name[0]))
		}
	}
}

// luminance is how light a color is, from 0 to 1, as WCAG 2 defines it.
func luminance(c engine.Color) float64 {
	r, g, b := c.RGB()
	channel := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

// contrast is the WCAG 2 contrast of two colors, from 1 to 21.
func contrast(a, b engine.Color) float64 {
	hi, lo := luminance(a), luminance(b)
	if hi < lo {
		hi, lo = lo, hi
	}
	return (hi + 0.05) / (lo + 0.05)
}

// Text has to be readable on both grounds (4.5 to 1), and the mark has to be found on them: its
// front half against the ground (3 to 1, as for any picture that means something), with the lit
// half the lighter of the two.
func TestEveryColorCanBeSeenOnItsGround(t *testing.T) {
	for _, g := range []ground{night, paper} {
		for what, c := range map[string]engine.Color{"the name": g.word, "text": g.ink, "quiet text": g.quiet} {
			if got := contrast(c, g.page); got < 4.5 {
				t.Errorf("%s: %s is %s on %s, a contrast of %.2f; wanted 4.5 or more", g.title, what, hex(c), hex(g.page), got)
			}
		}
		if got := contrast(g.front, g.page); got < 3 {
			t.Errorf("%s: the mark's front is %s on %s, a contrast of %.2f; wanted 3 or more", g.title, hex(g.front), hex(g.page), got)
		}
		if luminance(g.top) <= luminance(g.front) {
			t.Errorf("%s: wanted the lit half %s lighter than the front %s", g.title, hex(g.top), hex(g.front))
		}
	}
}

// BRAND.md gives each terminal color as its number and its hex value. The engine knows what
// every number is, so the page cannot say a color that is not that number's.
func TestBrandMdNamesTheColorsTheSourceUses(t *testing.T) {
	page, err := os.ReadFile(root("BRAND.md"))
	if err != nil {
		t.Fatal(err)
	}
	pairs := regexp.MustCompile("\\b(\\d{1,3}) `(#[0-9a-f]{6})`").FindAllStringSubmatch(string(page), -1)
	said := map[string]bool{}
	for _, p := range pairs {
		n, _ := strconv.Atoi(p[1])
		if n > 255 {
			t.Errorf("BRAND.md: %s is not a terminal color number", p[1])
			continue
		}
		if want := hex(engine.Palette(uint8(n))); p[2] != want {
			t.Errorf("BRAND.md says terminal color %d is %s; it is %s", n, p[2], want)
		}
		said[p[2]] = true
	}
	for _, g := range []ground{night, paper} {
		for what, c := range map[string]engine.Color{"ground": g.page, "name": g.word, "text": g.ink, "quiet text": g.quiet, "mark, lit": g.top, "mark, front": g.front} {
			if !said[hex(c)] && !strings.Contains(string(page), "`"+hex(c)+"`") {
				t.Errorf("BRAND.md does not give %s's %s, %s", g.title, what, hex(c))
			}
		}
	}
}

// The site's stylesheet is in the two grounds' colors: the same values, so that the page and the
// logo on it cannot drift apart.
func TestTheSiteWearsBothGrounds(t *testing.T) {
	css, err := os.ReadFile(root("site/style.css"))
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []ground{night, paper} {
		for what, c := range map[string]engine.Color{"ground": g.page, "text": g.ink, "quiet text": g.quiet} {
			if !strings.Contains(string(css), hex(c)) {
				t.Errorf("site/style.css does not use %s's %s, %s", g.title, what, hex(c))
			}
		}
	}
}

// The card is the size a link preview wants and light enough to load at once, and its text is
// whole: every row of a line is as wide as the others.
func TestTheCardIsWhatALinkPreviewWants(t *testing.T) {
	data, err := os.ReadFile(root("site/card.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) >= 1_000_000 {
		t.Errorf("wanted the card under 1 MB, it is %d bytes", len(data))
	}
	if b := decode(t, "site/card.png", data).Bounds(); b.Dx() != cardW || b.Dy() != cardH {
		t.Errorf("wanted a card of %d by %d, got %v", cardW, cardH, b)
	}
	for i, rows := range cardLines {
		if len(rows) != len(name) {
			t.Errorf("line %d: wanted %d rows, as the name has, got %d", i, len(name), len(rows))
		}
		for y, row := range rows {
			if len(row) != len(rows[0]) {
				t.Errorf("line %d: row %d is %d wide, wanted %d", i, y, len(row), len(rows[0]))
			}
		}
		if w := cardMargin + 4*len(rows[0]); w > cardW-cardMargin {
			t.Errorf("line %d ends at %d, past the %d pixels kept clear at the right", i, w, cardMargin)
		}
	}
}

// favicon.ico holds one picture of each size, and each is the mark.
func TestTheIconHoldsTheMarkAtEachSize(t *testing.T) {
	data, err := os.ReadFile(root("site/favicon.ico"))
	if err != nil {
		t.Fatal(err)
	}
	pictures := icoPictures(t, data)
	if len(pictures) != len(icoSizes) {
		t.Fatalf("wanted %d pictures, got %d", len(icoSizes), len(pictures))
	}
	for i, size := range icoSizes {
		samePicture(t, fmt.Sprintf("the picture of %d pixels", size), pictures[i], markFrame(t, night).Image(engine.ImageOptions{Scale: size / 2}))
	}
}

// Every SVG is well-formed XML with a viewBox, so that it scales, and says what it is to someone
// who cannot see it.
func TestTheSVGsAreWellFormed(t *testing.T) {
	for _, f := range files {
		if filepath.Ext(f.path) != ".svg" {
			continue
		}
		data, err := os.ReadFile(root(f.path))
		if err != nil {
			t.Fatal(err)
		}
		dec := xml.NewDecoder(bytes.NewReader(data))
		var sawRoot, sawTitle bool
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s: %v", f.path, err)
			}
			start, ok := tok.(xml.StartElement)
			if !ok {
				continue
			}
			switch start.Name.Local {
			case "svg":
				sawRoot = true
				var viewBox string
				for _, a := range start.Attr {
					if a.Name.Local == "viewBox" {
						viewBox = a.Value
					}
				}
				if viewBox == "" {
					t.Errorf("%s: wanted a viewBox", f.path)
				}
			case "title":
				sawTitle = true
			}
		}
		if !sawRoot || !sawTitle {
			t.Errorf("%s: wanted an svg element with a title, got svg %v and title %v", f.path, sawRoot, sawTitle)
		}
	}
}

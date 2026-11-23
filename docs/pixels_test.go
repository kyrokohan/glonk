package docs

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/widget"
)

// show prints a pixel canvas, a # for every pixel that is set and a dot for every empty one.
func show(canvas *engine.Canvas) {
	for y := range canvas.Height() {
		row := make([]byte, canvas.Width())
		for x := range row {
			row[x] = '.'
			if !canvas.At(x, y).IsDefault() {
				row[x] = '#'
			}
		}
		fmt.Println(string(row))
	}
}

// The pixel canvas has two pixels to a cell, one above the other. Compose turns them into cells.
func Example_pixelCanvas() {
	fb, _ := engine.NewFrameBuffer(20, 5)

	canvas := fb.Canvas() // 20 pixels across and 10 down
	canvas.FillRect(1, 1, 6, 7, engine.RGB(40, 60, 220))
	canvas.FillCircle(14, 5, 4, engine.RGB(230, 210, 40))
	canvas.Compose() // the pixels become cells of the frame

	show(canvas)
	// Output:
	// ....................
	// .######.......#.....
	// .######.....#####...
	// .######....#######..
	// .######....#######..
	// .######...#########.
	// .######....#######..
	// .######....#######..
	// ............#####...
	// ..............#.....
}

// Text on the pixel canvas is drawn in a font of 5 by 7 pixels, at any whole number of times that.
func Example_pixelText() {
	fb, _ := engine.NewFrameBuffer(32, 4)
	canvas := fb.Canvas()

	title, white := "GLONK", engine.RGB(255, 255, 255)
	canvas.DrawText((canvas.Width()-engine.PixelTextWidth(title, 1))/2, 0, title, white)
	canvas.Compose()

	show(canvas)
	// Output:
	// ..###..#......###..#...#.#...#..
	// .#...#.#.....#...#.##..#.#..#...
	// .#.....#.....#...#.##..#.#.#....
	// .#.###.#.....#...#.#.#.#.##.....
	// .#...#.#.....#...#.#..##.#.#....
	// .#...#.#.....#...#.#..##.#..#...
	// ..###..#####..###..#...#.#...#..
	// ................................
}

// A JPEG or a PNG becomes a picture once, at start-up: from a //go:embed []byte, a file or a
// download. Here a PNG made in memory stands in for the file.
func Example_decode() {
	var file bytes.Buffer
	png.Encode(&file, image.NewRGBA(image.Rect(0, 0, 8, 6)))

	pic, err := bitmap.Decode(&file)
	if err != nil {
		fmt.Println("no picture:", err)
		return
	}

	fmt.Println(pic.Width(), "by", pic.Height(), "pixels")
	// Output:
	// 8 by 6 pixels
}

// coin is a picture made in code: a see-through square with a gold disc in it.
func coin() *bitmap.Bitmap {
	gold := engine.RGB(255, 200, 0)
	pic := bitmap.New(5, 5)
	for y := range 5 {
		for x := range 5 {
			if (x-2)*(x-2)+(y-2)*(y-2) <= 5 {
				pic.Set(x, y, gold)
			}
		}
	}
	return pic
}

// A picture is drawn at its own size or stretched, and a view of it shares its pixels.
func Example_pictures() {
	fb, _ := engine.NewFrameBuffer(24, 5)
	canvas := fb.Canvas()

	pic := coin()
	pic.Draw(canvas, 1, 1)                  // at its own size
	pic.DrawScaled(canvas, 8, 0, 10, 10)    // stretched to 10 by 10 pixels
	pic.Sub(0, 0, 3, 5).Draw(canvas, 20, 1) // its left three columns only
	canvas.Compose()

	show(canvas)
	// Output:
	// ..........######........
	// ..###.....######.....##.
	// .#####..##########..###.
	// .#####..##########..###.
	// .#####..##########..###.
	// ..###...##########...##.
	// ........##########......
	// ........##########......
	// ..........######........
	// ..........######........
}

// A picture goes over cells that are already in the frame, such as a filled panel, with
// ComposeRect: draw the cells, draw the pixels, then put that rectangle down.
func Example_pictureOverCells() {
	fb, _ := engine.NewFrameBuffer(14, 5)

	widget.Panel{Title: "coins", Fill: engine.Cell{Background: engine.Palette(236)}}.Draw(fb, 0, 0, 14, 5)
	fb.DrawText(8, 2, "x 3", engine.Color{})

	canvas, pic := fb.Canvas(), coin()
	pic.Draw(canvas, 2, 2) // pixel (2, 2) is in cell (2, 1): a cell is one pixel across and two down
	canvas.ComposeRect(2, 2, pic.Width(), pic.Height())

	middle := fb.At(4, 2) // the cell that holds pixels (4, 4) and (4, 5)
	fmt.Println("the middle of the coin is gold:", middle.Background == engine.RGB(255, 200, 0))
	fmt.Printf("the label is still there: %c%c%c\n", fb.At(8, 2).Character, fb.At(9, 2).Character, fb.At(10, 2).Character)
	// Output:
	// the middle of the coin is gold: true
	// the label is still there: x 3
}

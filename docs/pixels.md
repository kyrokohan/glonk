# Pixels, shapes and pictures

A terminal cell is about twice as tall as it is wide. The pixel canvas cuts every cell into two square pixels, one above the other, so a frame can hold shapes and pictures finer than characters.

## The canvas

```go
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
```

Each `#` here is a pixel that was set, printed for this guide. On screen they are colors.

- **`fb.Canvas()`** is the frame's canvas. It is as wide as the frame, and twice as tall.
- **`Compose()`** turns the pixels into cells of the frame. Nothing drawn on the canvas shows without it.
- **A cell whose two pixels differ** becomes the upper half block, `▀`, in two colors. One whose pixels are alike becomes a solid cell.
- **Draw the cells of a HUD after `Compose`.** Text written then replaces the cells it lands on.
- **Apple's Terminal shows a faint line through half blocks.** glonk detects it, and `Compose` draws solid cells there: no line, and a blockier picture. `Canvas.Seamless` overrides the choice.

## Shapes

| Call | Draws |
|---|---|
| `FillRect(x, y, w, h, color)` | A filled rectangle |
| `DrawRect(x, y, w, h, color)` | Its border, one pixel thick, inside the same rectangle |
| `FillCircle(cx, cy, r, color)` | A filled circle, `2r+1` pixels across |
| `DrawCircle(cx, cy, r, color)` | Its outline, one pixel thick |
| `FillDisc(cx, cy, r, color)` | A disc whose center and radius can be fractions of a pixel, with a smooth edge |
| `DrawLine(x0, y0, x1, y1, color)` | A line, one pixel thick |
| `BlendRect(x, y, w, h, color, t)` | A color mixed over a rectangle, by the part `t` from 0 to 1 |
| `BlendDisc(cx, cy, r, color, t)` | A color mixed over a disc: a shadow or a glow |

- **Shapes clip.** Only the pixels on the canvas are visited, so a shape far off screen costs nothing.
- **The zero `Color` erases.**
- **Shapes ignore depth.** They replace the pixels under them.
- **None of them allocates.**
- **Circles and discs draw the same pixels on every platform.**
- **`FillDisc` is for a small round thing that moves smoothly.** A pixel on its edge is mixed toward the color by how much of it the disc covers.
- **An empty pixel has nothing to mix with.** On a disc's edge it takes the color from half covered, and stays empty below that. Fill a background first for a smooth edge everywhere.
- **`BlendRect` and `BlendDisc` leave empty pixels empty,** since the terminal's own background is not known. Fill first where the tint should show.
- **`BlendRect` over the whole canvas** is a dim behind a menu, a fade to black or a red flash, in one call.

## Text in pixels

```go
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
```

- **`Canvas.DrawText`** draws the printable ASCII characters in a font that comes with glonk: letters 5 pixels wide and 7 tall, 6 apart.
- **`DrawTextScaled(x, y, text, color, scale)`** draws them at 2 or 3 times the size, or more.
- **`engine.PixelTextWidth(text, scale)`** is how wide the text is, so a title centers. Its height is `7*scale`.
- **Only the pixels of the letters are written,** so what is behind them shows between the strokes. `FillRect` a background first to have one.
- **A character with no letter is drawn as `?`,** and there is no wrapping.
- `FrameBuffer.DrawText` still draws cells. Use it where text is part of the grid, and this where it is part of a picture.

## Pictures

The `bitmap` package reads a JPEG or a PNG once, at start-up: from a `//go:embed` byte slice, a file or a download.

```go
	pic, err := bitmap.Decode(&file)
	if err != nil {
		fmt.Println("no picture:", err)
		return
	}
```

- **`Decode` finds the format by what the file holds,** not by its name.
- **A pixel with an alpha of 128 or more is drawn as an opaque color.** A lighter one is a hole: what is under it shows. There is no blending.
- **Black is a color** and is drawn.

A picture can also be made in code:

```go
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
```

- **`bitmap.New(w, h)`** makes a picture of holes, and `Set` colors one pixel.
- **`bitmap.FromImage(img)`** copies any `image.Image`, with the same rule for alpha.

Then draw it:

```go
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
```

- **`Draw`** copies the picture at its own size, and **`DrawScaled`** stretches it. Neither allocates.
- **`DrawScaled` takes the nearest pixel,** with no smoothing, so a picture at its own size comes out exactly.
- **`Sub(x, y, w, h)`** is a view of one rectangle of a picture. It shares the pixels, so a strip of animation frames is one picture, drawn a frame at a time.
- **`Mirrored()`** is a view with its columns reversed, for a thing that faces the other way.
- **`Shaded(brightness)`** is a view drawn darker, and **`Tinted(color, part)`** one mixed toward a color. Drawing either costs about five times a plain draw.
- **Make the views once,** at start-up. Nothing is copied.

## Files that cannot be trusted

- **A small file can claim a picture of billions of pixels.** `Decode` reads the size from the header first.
- **It refuses a file over 32 MiB, or a picture of more than 4,194,304 pixels,** which is 2,048 by 2,048, before decoding it.
- **`DecodeLimit`** takes limits of your own. A source that never ends is read only up to the limit.
- **The decoders add about 220 KB to a program,** and only to one that imports `bitmap`.

## Over cells

The canvas is under every cell, so a filled panel drawn after a picture covers it. `ComposeRect` puts one rectangle of the canvas down over cells that are already in the frame:

```go
	widget.Panel{Title: "coins", Fill: engine.Cell{Background: engine.Palette(236)}}.Draw(fb, 0, 0, 14, 5)
	fb.DrawText(8, 2, "x 3", engine.Color{})

	canvas, pic := fb.Canvas(), coin()
	pic.Draw(canvas, 2, 2) // pixel (2, 2) is in cell (2, 1): a cell is one pixel across and two down
	canvas.ComposeRect(2, 2, pic.Width(), pic.Height())
```

- **The order is:** draw the cells, draw the pixels, then `ComposeRect` that rectangle, in pixels.
- **A cell that holds a letter loses it** when a pixel is put down in it, since a half block cannot hold one.
- **A cell whose two pixels are both empty is not touched,** and keeps its letter.
- **Where the canvas also holds a world** that `Compose` has put down, `Clear` it first and draw the picture on the empty canvas.
- `examples/world` has a coin in its panel, drawn this way.

## In a 3D scene

- **`Draw` and `DrawScaled` never read or write depth.** That is right for a logo or a HUD.
- **`DrawScaledDepth(canvas, x, y, w, h, depth)`** is for a picture of a thing that stands in the scene: an enemy, a pickup, a tree. A nearer surface hides it, and it hides a farther one.
- **The depth is one number for the whole picture.** `Camera.ProjectPixels` gives it, with the pixel to draw at. It costs 4 to 5 percent more than `DrawScaled`.
- **`Compose` gives each cell the depth of its nearer pixel,** so what is drawn with a depth afterwards is hidden behind nearer pixels.

## Not included

Smooth scaling, anti-aliasing other than the edge of a disc, animated formats, ellipses, rounded rectangles, lines thicker than a pixel, and blending by a pixel's own alpha.

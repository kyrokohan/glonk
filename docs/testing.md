# Testing a game

`Update` and `Draw` are plain methods, so a game is tested by calling them. No terminal is needed: `engine.Keyboard` stands in for the keys, and a frame prints as text or draws as a picture.

## Press a key

This tests the game of [Your first game](first-game.md):

```go
func TestEscapeQuits(t *testing.T) {
	kb := engine.NewKeyboard()
	kb.Press(engine.KeyEscape)

	err := (&game{}).Update(1.0/30, kb.Input())

	if !errors.Is(err, engine.ErrQuit) {
		t.Errorf("wanted ErrQuit, got %v", err)
	}
}
```

- **`NewKeyboard`** acts like a terminal with key releases. A key is down from `Press` until `Release`.
- **`kb.Input()`** is what `Update` takes.
- **`EndFrame`** ends a frame as `Run` does. `Pressed` and `Started` are true for one frame only.
- **A `Press` of a key that is already down is a repeat.** `Pressed` is true again and `Started` is not, so a test can hold a key.
- **`PressWith(mods, keys...)`** presses a chord, **`Type(text)`** types, and **`Paste(text)`** pastes.
- **`MouseMove`, `MouseDown`, `MouseUp` and `Wheel`** move and click the pointer.
- None of it needs a clock, so the tests are exact.

`NewLegacyKeyboard` acts like a [terminal without key releases](input.md#terminals-without-key-releases), for a game that asks `in.Exact()`. A key stays held for about 150 ms after its last `Press`, `Release` does nothing, and `Advance` moves the clock.

## Read a frame as text

```go
func TestTheAtWalksAcrossTheBox(t *testing.T) {
	g := &game{}
	g.Update(0.25, engine.NewKeyboard().Input()) // a quarter of a second at 20 cells a second

	fb, _ := engine.NewFrameBuffer(12, 3)
	g.Draw(fb)

	want := "┌──────────┐\n" +
		"│    @     │\n" +
		"└──────────┘"
	if got := fb.String(); got != want {
		t.Errorf("wanted\n%s\ngot\n%s", want, got)
	}
}
```

- **`engine.NewFrameBuffer(w, h)`** makes a frame of any size.
- **`fb.String()`** is the frame as text, one line per row.
- It leaves off the trailing spaces of each line, and shows characters only, not colors.

## Read a frame as a picture

`fb.Image` draws what `String` cannot show: colors, and the two pixels of every cell of the pixel canvas.

```go
	canvas := fb.Canvas()
	canvas.FillRect(0, 0, 4, 5, engine.RGB(240, 140, 30)) // pixels: four across, five down
	canvas.Compose()
	fb.DrawText(5, 1, "score", engine.Palette(46))

	img := fb.Image(engine.ImageOptions{Scale: 4})
```

- **The picture is `Width*Scale` by `2*Height*Scale` pixels.** At `Scale: 1`, a frame drawn on the pixel canvas comes out pixel for pixel.
- **A character is drawn as a mark:** a rectangle in its color, which shows where text is and not what it says.
- **With `Text: true` and a `Scale` of 6 or more,** printable ASCII is drawn as letters, so a screenshot can be read.
- **Pixels set on the pixel canvas are in the picture only after `Compose`.**
- **A picture of more than 4,194,304 pixels is not made.**

A test can write the picture to a file when it is asked to, for a person to look at:

```go
	if path := os.Getenv("GLONK_SCREENSHOT"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			panic(err)
		}
	}
```

That is `GLONK_SCREENSHOT=scene.png go test`. `ExampleFrameBuffer_Image` in the `engine` package is all of it, and the documentation of [Image](https://pkg.go.dev/glonk.dev/engine#FrameBuffer.Image) has the rest of its rules.

## A whole run

To test a whole run, with key bytes in and frames out, give `Config.Input` and `Config.Output` a reader and a writer that are not a terminal:

```go
	var out bytes.Buffer
	err := engine.Run(context.Background(), &greeter{}, engine.Config{
		Width: 8, Height: 1, FPS: 100,
		Input:  strings.NewReader(""), // key presses would be read from here
		Output: &out,
	})
```

- **`Run` then has no terminal to take over.** It needs a `Width` and a `Height`, and writes nothing but the frames.
- **Every frame is sent whole,** so `Stats.Bytes` is the size of the frame.
- **A key is held from its press to its release,** and `in.Exact()` is true. A plain letter has no release, so it stays held.
- **`Config.LegacyKeys`** makes it act like a terminal without key releases.
- **Ctrl+C still ends the run.** Ctrl+Z does nothing, and quit signals are left to your program.
- **The end of the input does not end the run.**

The pictures on glonk.dev are made this way: each example plays a script of key presses with no terminal.

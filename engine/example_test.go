package engine_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"glonk.dev/engine"
)

// walker is a game: a dot that crosses the screen until Escape is pressed.
type walker struct{ x float64 }

func (w *walker) Update(dt float64, in *engine.Input) error {
	if in.Pressed(engine.KeyEscape) {
		return engine.ErrQuit
	}
	w.x += 20 * dt // cells per second
	return nil
}

func (w *walker) Draw(fb *engine.FrameBuffer) {
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.Set(int(w.x)%fb.Width(), fb.Height()/2, engine.Cell{Character: '@', Foreground: engine.Palette(208)})
}

// Run takes over the terminal until the game quits, the user presses Ctrl-C, or the
// program is told to stop; the terminal is restored when it returns.
func ExampleRun() {
	// Width and Height of 0 fill the terminal and follow resizes.
	err := engine.Run(context.Background(), &walker{}, engine.Config{FPS: 30})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// greeter draws a greeting and quits on its third Update.
type greeter struct{ updates int }

func (g *greeter) Update(float64, *engine.Input) error {
	if g.updates++; g.updates == 3 {
		return engine.ErrQuit
	}
	return nil
}

func (g *greeter) Draw(fb *engine.FrameBuffer) {
	fb.DrawText(0, 0, "hello", engine.Color{})
}

// Without a terminal, Run needs a size and writes only the frames, and reads keys
// from Input, so a game can be run and checked in a test.
func ExampleRun_headless() {
	var out bytes.Buffer
	err := engine.Run(context.Background(), &greeter{}, engine.Config{
		Width: 8, Height: 1, FPS: 100,
		Input:  strings.NewReader(""), // key presses would be read from here
		Output: &out,
	})

	fmt.Println("error:", err)
	fmt.Println("frames written:", strings.Count(out.String(), "hello"))
	// Output:
	// error: <nil>
	// frames written: 2
}

func ExampleFrameBuffer_DrawText() {
	fb, err := engine.NewFrameBuffer(16, 3)
	if err != nil {
		panic(err)
	}

	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.DrawText(2, 1, "hello, glonk", engine.Color{})

	fmt.Println(fb)
	// Output:
	// ┌──────────────┐
	// │ hello, glonk │
	// └──────────────┘
}

// TextWidth says how many cells a string fills, so that it can be centered.
func ExampleTextWidth() {
	fb, err := engine.NewFrameBuffer(16, 3)
	if err != nil {
		panic(err)
	}

	title := "cafe\u0301" // five runes, but the combining accent has no cell: four cells
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.DrawText((fb.Width()-engine.TextWidth(title))/2, 1, title, engine.Color{})

	fmt.Println(engine.TextWidth(title))
	fmt.Println(fb)
	// Output:
	// 4
	// ┌──────────────┐
	// │     cafe     │
	// └──────────────┘
}

// A Keyboard stands in for the terminal's keyboard in a test: hand its Input to Update.
func ExampleKeyboard() {
	kb := engine.NewKeyboard()
	game := &walker{}

	kb.Press('d')
	fmt.Println(kb.Input().IsDown('d'), kb.Input().Pressed('d')) // down, and new this frame
	kb.EndFrame()
	fmt.Println(kb.Input().IsDown('d'), kb.Input().Pressed('d')) // still down, no longer new
	kb.Release('d')
	fmt.Println(kb.Input().IsDown('d'))

	fmt.Println(game.Update(0.1, kb.Input()))
	kb.Press(engine.KeyEscape)
	fmt.Println(game.Update(0.1, kb.Input()))
	// Output:
	// true true
	// true false
	// false
	// <nil>
	// quit
}

// A FrameBuffer prints as text, one line per row, which makes a whole screen easy to
// check in a test.
func ExampleFrameBuffer_String() {
	fb, err := engine.NewFrameBuffer(10, 3)
	if err != nil {
		panic(err)
	}

	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.DrawText(2, 1, "hi", engine.Color{})

	fmt.Println(fb)
	// Output:
	// ┌────────┐
	// │ hi     │
	// └────────┘
}

// A Canvas holds two pixels per cell, one above the other, and Compose turns them
// into half-block characters.
func ExampleCanvas() {
	fb, err := engine.NewFrameBuffer(8, 2)
	if err != nil {
		panic(err)
	}

	canvas := fb.Canvas()
	for x := range canvas.Width() {
		canvas.Set(x, x%canvas.Height(), engine.Palette(196)) // a zigzag in palette color 196
	}
	canvas.Compose()

	fmt.Println(fb)
	// Output:
	// ▀▄  ▀▄
	//   ▀▄  ▀▄
}

// A shade is an exact RGB color, so brightness runs smoothly on a terminal with true
// color and is rounded to the palette on one without.
func ExampleShade() {
	red := engine.Palette(196)
	fmt.Println(engine.Shade(red, 1), engine.Shade(red, 0.5), engine.Shade(red, 0))
	// Output:
	// #ff0000 #800000 #000000
}

func ExamplePalette() {
	orange := engine.Palette(208)
	black := engine.Palette(0) // a real color; the default is the zero Color

	fmt.Println(orange, black)
	fmt.Println(black.IsDefault(), engine.Color{}.IsDefault())
	// Output:
	// palette(208) palette(0)
	// false true
}

// An RGB color is shown as the nearest palette color until true color output is added,
// but it keeps its exact value, and Shade scales it exactly.
func ExampleRGB() {
	c := engine.RGB(255, 135, 0)

	fmt.Println(c)
	fmt.Println(engine.Shade(c, 0.5))
	// Output:
	// #ff8700
	// #804400
}

func ExampleCell() {
	cell := engine.Cell{
		Character:  'x',
		Foreground: engine.RGB(255, 135, 0),
		Background: engine.Palette(0),
		Attrs:      engine.Bold | engine.Underline,
	}

	fmt.Println(cell.Foreground, cell.Background)
	fmt.Println(cell.Attrs.Has(engine.Bold), cell.Attrs.Has(engine.Reverse))
	// Output:
	// #ff8700 palette(0)
	// true false
}

// A color can say its red, green and blue, for mixing it with another. A palette color is
// read as xterm defines it, and the default color as black.
func ExampleColor_RGB() {
	fmt.Println(engine.Palette(208).RGB())
	fmt.Println(engine.RGB(12, 34, 56).RGB())
	fmt.Println(engine.Color{}.RGB())
	// Output:
	// 255 135 0
	// 12 34 56
	// 0 0 0
}

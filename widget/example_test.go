package widget_test

import (
	"fmt"

	"glonk.dev/engine"
	"glonk.dev/widget"
)

// A label sits at the left, middle or right of the width it is given.
func ExampleLabel() {
	fb, err := engine.NewFrameBuffer(20, 3)
	if err != nil {
		panic(err)
	}

	widget.Label{Text: "score"}.Draw(fb, 0, 0, 20)
	widget.Label{Text: "score", Align: widget.Center}.Draw(fb, 0, 1, 20)
	widget.Label{Text: "score", Align: widget.Right}.Draw(fb, 0, 2, 20)

	fmt.Println(fb)
	// Output:
	// score
	//        score
	//                score
}

// A panel has a border and a title; a label and a bar go inside it.
func ExamplePanel() {
	fb, err := engine.NewFrameBuffer(16, 4)
	if err != nil {
		panic(err)
	}

	widget.Panel{Title: "Status"}.Draw(fb, 0, 0, 16, 4)
	widget.Label{Text: "coins 3/12"}.Draw(fb, 2, 1, 12)
	widget.Bar{Value: 0.25}.Draw(fb, 2, 2, 12)

	fmt.Println(fb)
	// Output:
	// ┌─ Status ─────┐
	// │ coins 3/12   │
	// │ ███░░░░░░░░░ │
	// └──────────────┘
}

// A menu is moved and chosen from Update, with the keyboard, and drawn from Draw. A Keyboard
// stands in for the terminal here.
func ExampleMenu() {
	menu := widget.Menu{Items: []string{"Resume", "Options", "Quit"}}
	kb := engine.NewKeyboard()

	kb.Press(engine.KeyDown)
	menu.Update(kb.Input())
	kb.EndFrame()

	kb.Press(engine.KeyEnter)
	choice, chosen := menu.Update(kb.Input())
	kb.EndFrame()
	fmt.Println(choice, chosen, menu.Items[choice])

	fb, err := engine.NewFrameBuffer(10, 3)
	if err != nil {
		panic(err)
	}
	menu.Draw(fb, 1, 0, 8, 3) // the selected row is drawn in reverse, which a String does not show
	fmt.Println(fb)
	// Output:
	// 1 true Options
	//  Resume
	//  Options
	//  Quit
}

// A bar is filled from the left, by the value, and its cells can be anything.
func ExampleBar() {
	fb, err := engine.NewFrameBuffer(10, 3)
	if err != nil {
		panic(err)
	}

	bar := widget.Bar{
		Full:  engine.Cell{Character: '#', Foreground: engine.Palette(2)},
		Empty: engine.Cell{Character: '-'},
	}
	for row, value := range []float64{0, 0.5, 1} {
		bar.Value = value
		bar.Draw(fb, 0, row, 10)
	}

	fmt.Println(fb)
	// Output:
	// ----------
	// #####-----
	// ##########
}

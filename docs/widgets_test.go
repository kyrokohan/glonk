package docs

import (
	"fmt"
	"unicode"

	"glonk.dev/engine"
	"glonk.dev/widget"
)

// A widget is a value with a Draw method that takes a place in cells.
func Example_hud() {
	fb, _ := engine.NewFrameBuffer(22, 4)
	coins, total := 3, 12

	widget.Panel{Title: "Status"}.Draw(fb, 0, 0, 22, 4)
	widget.Label{Text: fmt.Sprintf("coins %d/%d", coins, total)}.Draw(fb, 2, 1, 18)
	widget.Bar{Value: float64(coins) / float64(total)}.Draw(fb, 2, 2, 18)

	fmt.Println(fb)
	// Output:
	// ┌─ Status ───────────┐
	// │ coins 3/12         │
	// │ █████░░░░░░░░░░░░░ │
	// └────────────────────┘
}

// pause is a game's pause screen: a menu keeps its selection, so the game keeps the menu.
type pause struct {
	menu widget.Menu
}

func (p *pause) Update(dt float64, in *engine.Input) error {
	if choice, chosen := p.menu.Update(in); chosen && choice == 1 {
		return engine.ErrQuit
	}
	return nil
}

func (p *pause) Draw(fb *engine.FrameBuffer) {
	widget.Panel{Title: "Paused"}.Draw(fb, 0, 0, 14, 4)
	p.menu.Draw(fb, 2, 1, 10, 2)
}

// Down moves a menu's selection and Enter chooses the item.
func Example_menu() {
	p, kb := &pause{menu: widget.Menu{Items: []string{"Resume", "Quit"}}}, engine.NewKeyboard()

	kb.Press(engine.KeyDown)
	fmt.Println(p.Update(1.0/30, kb.Input()), p.menu.Selected)
	kb.EndFrame()

	kb.Press(engine.KeyEnter)
	fmt.Println(p.Update(1.0/30, kb.Input()))

	fb, _ := engine.NewFrameBuffer(14, 4)
	p.Draw(fb)
	fmt.Println(fb)
	// Output:
	// <nil> 1
	// quit
	// ┌─ Paused ───┐
	// │ Resume     │
	// │ Quit       │
	// └────────────┘
}

// A text field is one line to type into. Accept says which characters it takes.
func Example_textField() {
	field, kb := widget.TextField{MaxLength: 4, Accept: unicode.IsDigit}, engine.NewKeyboard()

	kb.Type("12ab345")
	kb.Press(engine.KeyEnter)
	entered := field.Update(kb.Input())
	fmt.Printf("%q entered: %v\n", field.Text, entered)

	fb, _ := engine.NewFrameBuffer(12, 1)
	fb.DrawText(0, 0, "Score:", engine.Color{})
	field.Draw(fb, 7, 0, 5)
	fmt.Println(fb)
	// Output:
	// "1234" entered: true
	// Score: 1234
}

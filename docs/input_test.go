package docs

import (
	"fmt"
	"time"

	"glonk.dev/engine"
)

// player reads the keyboard the three ways a game asks about a key.
type player struct {
	x, jumps, row int
}

func (p *player) Update(dt float64, in *engine.Input) error {
	if in.IsDown('d') || in.IsDown(engine.KeyRight) {
		p.x++ // held: every frame while the key is down
	}
	if in.Started(' ') {
		p.jumps++ // went down: once, however long it is held
	}
	if in.Pressed(engine.KeyDown) {
		p.row++ // went down or repeated: a held key moves a menu on
	}
	return nil
}

// IsDown is for a key that keeps something going, Started for one that does something once, and
// Pressed for one whose repeat should act again.
func Example_keys() {
	p, kb := &player{}, engine.NewKeyboard()

	kb.Press('d', ' ', engine.KeyDown)
	for range 3 { // three frames with all three keys held, and the terminal repeating them
		p.Update(1.0/30, kb.Input())
		kb.EndFrame()
		kb.Press(' ', engine.KeyDown)
	}

	fmt.Println("moved", p.x, "jumped", p.jumps, "rows", p.row)
	// Output:
	// moved 3 jumped 1 rows 3
}

// editor reads what a text field and a shortcut need: the characters typed, a paste, and the keys
// with their modifiers.
type editor struct {
	text  string
	saved bool
}

func (e *editor) Update(dt float64, in *engine.Input) error {
	e.text += in.Text() + in.Paste()

	for _, k := range in.KeyEvents() {
		if k.Key == 's' && k.Mods.Has(engine.ModCtrl) && k.Action == engine.ActionPress {
			e.saved = true
		}
	}
	return nil
}

// Text is what was typed and Paste what was pasted. A chord such as Ctrl+S is in KeyEvents, and
// IsDown, Pressed and Started never see it.
func Example_textAndChords() {
	e, kb := &editor{}, engine.NewKeyboard()

	kb.Type("glo")
	kb.Paste("nk\n")
	kb.PressWith(engine.ModCtrl, 's')
	e.Update(1.0/30, kb.Input())

	fmt.Printf("%q saved %v, s down %v\n", e.text, e.saved, kb.Input().IsDown('s'))
	// Output:
	// "glonk\n" saved true, s down false
}

// pointer reads the mouse, which a game gets with Config.Mouse.
type pointer struct {
	x, y   int
	clicks int
	scroll int
}

func (p *pointer) Update(dt float64, in *engine.Input) error {
	if x, y, ok := in.MousePos(); ok {
		p.x, p.y = x, y
	}
	if in.MouseStarted(engine.MouseLeft) {
		p.clicks++
	}
	_, turned := in.Wheel()
	p.scroll += turned
	return nil
}

// The pointer is in a cell, a button starts once for each click, and the wheel says how far it
// turned in the frame.
func Example_mouse() {
	p, kb := &pointer{}, engine.NewKeyboard()

	kb.MouseMove(12, 4)
	kb.MouseDown(engine.MouseLeft)
	kb.MouseUp(engine.MouseLeft) // a click shorter than a frame is not lost
	kb.Wheel(0, 3)
	p.Update(1.0/30, kb.Input())

	fmt.Println("at", p.x, p.y, "clicks", p.clicks, "scroll", p.scroll)
	// Output:
	// at 12 4 clicks 1 scroll 3
}

// A terminal that reports no key releases lets a key go about 150 ms after its last press or
// repeat. NewLegacyKeyboard acts like one, and Advance moves its clock.
func Example_withoutKeyReleases() {
	kb := engine.NewLegacyKeyboard()

	kb.Press('w')
	fmt.Println("exact:", kb.Input().Exact(), "held:", kb.Input().IsDown('w'))

	kb.Advance(200 * time.Millisecond) // no repeat came
	fmt.Println("held:", kb.Input().IsDown('w'))
	// Output:
	// exact: false held: true
	// held: false
}

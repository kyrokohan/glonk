# Widgets

The `widget` package draws the pieces of a HUD and a menu: a label, a panel, a bar, a menu and a text field. Each is a value with a `Draw` method that takes a place in cells, as `DrawBox` does.

## A HUD

```go
	widget.Panel{Title: "Status"}.Draw(fb, 0, 0, 22, 4)
	widget.Label{Text: fmt.Sprintf("coins %d/%d", coins, total)}.Draw(fb, 2, 1, 18)
	widget.Bar{Value: float64(coins) / float64(total)}.Draw(fb, 2, 2, 18)

	fmt.Println(fb)
	// Output:
	// ┌─ Status ───────────┐
	// │ coins 3/12         │
	// │ █████░░░░░░░░░░░░░ │
	// └────────────────────┘
```

- **Draw widgets after the world.** They are placed on the screen, not in the level.
- **A style is an `engine.Cell`.** Its colors and attributes are used, and its character is not.
- **A `Label`** is a line of text, cut off at its width. `Align` is `Left`, `Center` or `Right`.
- **A `Bar`** shows a `Value` from 0 to 1 in the cells `Full` and `Empty`, which are `█` and `░` unless you say.
- **A `Panel`** is a border with a title, which shows from 7 cells wide.
- **A blank `Fill` is a hole.** The panel then leaves its inside alone and shows what is under it.

## Panels that share an edge

A border drawn over a light box line that is already there meets it as `├ ┤ ┬ ┴ ┼`. So a list of rows, a table or a view split in two is panels drawn edge on edge.

- **`fb.SetJoined(x, y, cell)`** draws one cell that way, for lines of your own.
- **Double, heavy and rounded lines are drawn over,** not joined.

## A menu

A menu keeps its selection, so the game keeps the menu:

```go
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
```

- **Up and Down move the selection,** and wrap around. Enter chooses, and `Update` returns the index of the item.
- **Escape and every other key are left to the game,** which can use Escape to open and close the menu.
- **The selected row is drawn reversed.** A list longer than its rows scrolls, keeping the selection in the middle row.
- **With `Config.Mouse` set it takes the mouse too.** A pointer that moves onto an item selects it, a left click chooses it, and the wheel steps the selection.
- **A menu not drawn since its last `Update` ignores the mouse,** as on the frame it opens.

## A text field

```go
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
```

- **A `TextField`** is one line to type into: a name, a number, a search.
- **Call `Update` for the field that has the keyboard.** It reports Enter.
- **Typing and a paste go in at the cursor.** A paste's line breaks go in as spaces.
- **Left, Right, Home and End move the cursor,** and Backspace and Delete delete.
- **Only characters one column wide go in,** none past `MaxLength`, and only those `Accept` takes.
- **Text longer than the field scrolls** to keep the cursor in view. The cursor is drawn reversed.
- **Escape, Tab and focus are the game's.**

## Always safe

- What runs off the frame is cut off.
- A size that is not positive draws nothing.
- Nothing allocates, except a text field's new text.

## Not included

Several lines of text, selecting text, layouts, and focus or nesting: a widget is drawn where you say. `examples/world` has a coin panel and a pause menu, and `examples/input` a text field.

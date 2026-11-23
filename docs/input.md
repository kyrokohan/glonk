# Keys, text and the mouse

`Update` is handed an `*engine.Input` every frame. It says which keys are down, what was typed or pasted, and where the mouse is.

## Three ways to ask about a key

```go
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
```

- **`IsDown`** is true while the key is held. Use it for what keeps going, such as walking.
- **`Started`** is true in the frame the key went down. A repeat does not set it off again. Use it for what happens once, such as a jump.
- **`Pressed`** is true when the key went down, and again each time it repeats. Use it where a held key should act again, such as moving through a menu.

A key is its character, such as `'d'` or `' '`, or one of these constants:

| Keys | Constants |
|---|---|
| The arrows | `KeyUp`, `KeyDown`, `KeyLeft`, `KeyRight` |
| Editing | `KeyEnter`, `KeyBackspace`, `KeyTab`, `KeyEscape`, `KeyInsert`, `KeyDelete` |
| Moving | `KeyHome`, `KeyEnd`, `KeyPageUp`, `KeyPageDown` |
| Function keys | `KeyF1` to `KeyF12` |

## Terminals without key releases

Most terminals say when a key goes down and when it repeats, and never when it comes up. Some say that too, through the [kitty keyboard protocol](https://sw.kovidgoyal.net/kitty/keyboard-protocol/): Ghostty, kitty, WezTerm, foot, iTerm2 and Alacritty. Only there can several keys be held at once. glonk asks the terminal at startup, and uses key releases where it has them.

Without key releases:

- **A held key is a timing guess.** It counts as held until about 150 ms pass with no repeat.
- **Only one key stays held.** The OS repeats just the last key pressed, so holding W and then pressing E lets W go. Apple's Terminal is one of these.
- **`Started` can fire twice.** The OS waits longer than 150 ms before its first repeat, so a key held past that starts again.
- **A letter has the case Shift gives it:** `'W'` with Shift held. With key releases a letter is always lower case. The examples ask for both.

```go
	kb := engine.NewLegacyKeyboard()

	kb.Press('w')
	fmt.Println("exact:", kb.Input().Exact(), "held:", kb.Input().IsDown('w'))

	kb.Advance(200 * time.Millisecond) // no repeat came
	fmt.Println("held:", kb.Input().IsDown('w'))
	// Output:
	// exact: false held: true
	// held: false
```

- **`in.Exact()`** says which kind this run has. A game can warn the player, or change to controls that work one key at a time.
- **`Config.LegacyKeys`** turns key releases off, for a terminal that misbehaves with them.
- **`engine.Probe(cfg)`** asks before a game starts. It takes a quarter of a second at most, and leaves the terminal as it found it.
- **`KeyReleases`** in its answer is what `Exact` will say. A launcher or a first screen can tell the player, and name a better terminal.

`examples/input` opens by printing what `Probe` found:

```go
// probeLine is what engine.Probe found, in a line, or why it could not ask.
func probeLine(t engine.Terminal, err error) string {
	switch {
	case err != nil:
		return "could not ask the terminal: " + err.Error()
	case !t.IsTerminal:
		return "not a terminal: input or output is redirected"
	}
	yes := map[bool]string{true: "yes", false: "no"}
	return fmt.Sprintf("this terminal: key releases %s, true color %s, %d by %d cells", yes[t.KeyReleases], yes[t.TrueColor], t.Width, t.Height)
}
```

```go
	fmt.Println(probeLine(engine.Probe(engine.Config{})))
```

## Shortcuts

```go
	for _, k := range in.KeyEvents() {
		if k.Key == 's' && k.Mods.Has(engine.ModCtrl) && k.Action == engine.ActionPress {
			e.saved = true
		}
	}
```

- **`KeyEvents`** is the frame's key events, in order. Each has its key, the modifiers held with it, and whether it was a press, a repeat or a release.
- **The modifiers** are `ModShift`, `ModAlt`, `ModCtrl` and `ModSuper`.
- **A chord is only there.** A character key with Ctrl, Alt or Super held is never in `IsDown`, `Pressed` or `Started`, so Ctrl+W does not walk a player who walks on W.
- **An arrow or a function key with a modifier is still that key:** Ctrl+PageDown is `KeyPageDown` to `IsDown`, `Pressed` and `Started`.
- **Hyper and Meta chords are not read.**

Without key releases a terminal says less about modifiers:

- **Escape counts after 50 ms.** An Escape and the start of an arrow key are the same byte, so Escape waits to see that nothing follows.
- **Alt+key arrives as Escape, then the key.** glonk reads it as Alt+key, and so it reads an Escape and a key that arrive together.
- **Ctrl+letter is a control byte.** That is why Ctrl+I is Tab, Ctrl+M is Enter and Ctrl+H is Backspace.
- **Shift is the case of a letter,** not a modifier of it.
- **The arrows, Home, End, Page Up, Page Down, Insert, Delete and F1 to F12 carry their modifiers.**

## Typing and a paste

```go
	e.text += in.Text() + in.Paste()
```

- **`Text`** is the characters typed this frame, for a text field such as `widget.TextField`.
- **`Paste`** is what was pasted, as one string. A newline in it does not press Enter, and a Ctrl+C in it does not end the run.
- **A paste needs bracketed paste.** glonk asks the terminal for it, and turns it off again when the run ends and across Ctrl+Z.
- **Where a terminal lacks it,** a paste arrives as the key presses it is, a newline as Enter, and `Paste` is empty.
- **A paste is cut at 1 MiB.** Its text is valid UTF-8, and its line breaks are newlines.

## The mouse

Set `Config.Mouse`, and the terminal reports the pointer:

```go
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
```

- **`MousePos`** is the cell the pointer is in, counted from 0 at the top left of the terminal. `ok` is false until the terminal has said where it is.
- **`MouseDown`, `MouseStarted` and `MouseReleased`** ask about `MouseLeft`, `MouseMiddle` and `MouseRight`. A click shorter than a frame is not lost.
- **`Wheel`** is how far the wheel turned in the frame, and **`MouseMoved`** whether the pointer went to another cell.
- **`MousePixels`** is where the pointer is on the pixel canvas. A cell is two pixels tall, so the top pixel of the cell (x, y) is (x, 2y).
- **`MouseFine`** says the terminal reports the pointer in pixels. `MousePixels` then has fractions of a pixel. Elsewhere it is the middle of the cell.

What to know before turning it on:

- **A terminal that reports the mouse no longer selects text with it.** Shift with the mouse still does, in most terminals. That is why it is off unless you ask.
- **glonk turns it off again** when the run ends, and across Ctrl+Z.
- **Only the three buttons and the wheel are reported,** with no modifiers.
- **A terminal without the decimal mouse mode, 1006, sends nothing glonk reads,** and `ok` stays false.
- **The cell can be outside the frame,** when the frame is smaller than the terminal.

`go run glonk.dev/examples/input@latest` shows what your terminal sends, as glonk reads it.

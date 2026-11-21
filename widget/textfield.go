package widget

import (
	"math"
	"strings"
	"unicode/utf8"

	"glonk.dev/engine"
)

// TextField is one line the user types into, with a cursor, that scrolls when the text is longer
// than the field: a name, a number, a search or a chat line. Call Update from the game's Update for
// the field that has the keyboard, and Draw from its Draw.
type TextField struct {
	// Text is what is in the field. Typing and pasting put in only characters one column wide, as
	// [engine.RuneWidth] says; text the game sets is drawn a character to a cell, as
	// [engine.FrameBuffer.DrawText] draws it, so a wide one shows as '?'.
	Text string

	// Cursor is where the next character goes, counted in characters: 0 is before the first, and
	// the number of them after the last. A number outside that is taken as the nearer end.
	Cursor int

	// MaxLength is the most characters typing and pasting fill the field to; 0 is no limit. Text
	// the game sets longer is left as it is, and nothing more goes in.
	MaxLength int

	// Accept says which characters may be typed or pasted, such as only digits for a number. Nil
	// takes every one that is one column wide, and one Accept takes that is not is dropped still.
	Accept func(rune) bool

	// Style is the colors and attributes of the field, across its whole width. The cursor is drawn
	// with Reverse toggled, as [Menu] draws its selection.
	Style engine.Cell

	scroll int // the first character Draw showed, kept so that the text moves only to keep the cursor in view
}

// Update edits the field with what was typed, pasted and pressed since the last frame, and reports
// whether Enter was pressed: a new press, as [Menu] takes it, so a held Enter says so once. The
// text stays as it is; the game clears it if it wants.
//
// The typed text ([engine.Input.Text]) and then the pasted text ([engine.Input.Paste]) go in at the
// cursor, which moves past them. A paste's line breaks become spaces and its other control
// characters are dropped. Only characters one column wide that Accept takes go in, and none past
// MaxLength. Then the keys, each repeating while held: Backspace and Delete delete the character
// before and after the cursor, Left and Right move it a character, and Home and End to the start
// and the end. Text goes in before the keys because Input keeps no order between them within a
// frame: a and Backspace in one frame leave nothing, and Backspace and b in one frame delete the
// character before the b. Escape, Tab and every other key are left to the game.
//
// A frame that changes Text allocates the new text; any other allocates nothing.
func (f *TextField) Update(in *engine.Input) (entered bool) {
	f.Cursor = min(max(f.Cursor, 0), utf8.RuneCountInString(f.Text))
	if typed := in.Text(); typed != "" {
		f.insert(typed, false)
	}
	if pasted := in.Paste(); pasted != "" {
		f.insert(pasted, true)
	}

	if in.Pressed(engine.KeyBackspace) && f.Cursor > 0 {
		f.Cursor--
		f.deleteAt(f.Cursor)
	}
	if in.Pressed(engine.KeyDelete) && f.Cursor < utf8.RuneCountInString(f.Text) {
		f.deleteAt(f.Cursor)
	}
	// Counted again: deleting from text the game set that is not UTF-8 can join two bytes into one character.
	n := utf8.RuneCountInString(f.Text)
	if in.Pressed(engine.KeyLeft) {
		f.Cursor = max(f.Cursor-1, 0)
	}
	if in.Pressed(engine.KeyRight) {
		f.Cursor = min(f.Cursor+1, n)
	}
	if in.Pressed(engine.KeyHome) {
		f.Cursor = 0
	}
	if in.Pressed(engine.KeyEnd) {
		f.Cursor = n
	}
	return in.Started(engine.KeyEnter)
}

// insert puts the characters of s that the field takes in at the cursor, and moves the cursor past
// them. A paste's line breaks become spaces.
func (f *TextField) insert(s string, paste bool) {
	room := math.MaxInt
	if f.MaxLength > 0 {
		room = f.MaxLength - utf8.RuneCountInString(f.Text)
	}
	var kept strings.Builder
	added := 0
	for _, r := range s {
		if added >= room {
			break
		}
		if paste && r == '\n' {
			r = ' '
		}
		if engine.RuneWidth(r) != 1 || (f.Accept != nil && !f.Accept(r)) {
			continue
		}
		kept.WriteRune(r)
		added++
	}
	if added == 0 {
		return
	}
	at := byteAt(f.Text, f.Cursor)
	f.Text = f.Text[:at] + kept.String() + f.Text[at:]
	f.Cursor += added
}

// deleteAt takes out the character at index i, which is in the text.
func (f *TextField) deleteAt(i int) {
	at := byteAt(f.Text, i)
	_, size := utf8.DecodeRuneInString(f.Text[at:])
	f.Text = f.Text[:at] + f.Text[at+size:]
}

// byteAt returns where character i of s starts, or len(s) for i at or past the end.
func byteAt(s string, i int) int {
	for at := range s {
		if i == 0 {
			return at
		}
		i--
	}
	return len(s)
}

// Draw draws the field in the w cells from (x, y) to the right: Style across all of them, the text
// a character to a cell, and the cursor with Reverse toggled, a blank one after the last character.
// Text wider than the field scrolls to keep the cursor in view, by no more than it must: Draw keeps
// where it scrolled to in the field, so the text stays put while the cursor moves within it, and
// comes back as the text gets shorter, so the field stays full. Draw changes nothing else.
func (f *TextField) Draw(fb *engine.FrameBuffer, x, y, w int) {
	if w <= 0 {
		return
	}
	n := utf8.RuneCountInString(f.Text)
	cursor := min(max(f.Cursor, 0), n)
	// The cursor after the last character takes a cell of its own, so the text and it are n+1 cells.
	f.scroll = min(max(f.scroll, cursor-w+1, 0), cursor, max(n+1-w, 0))

	left, right := max(x, 0), min(reach(x, w), fb.Width())
	if left >= right || y < 0 || y >= fb.Height() {
		return
	}
	first := left - x // the first cell of the field that is on the frame
	at := byteAt(f.Text, f.scroll+min(first, n))
	blank := styled(f.Style, ' ')
	for cx := left; cx < right; cx++ {
		c := first + (cx - left)
		cell := blank
		if at < len(f.Text) {
			r, size := utf8.DecodeRuneInString(f.Text[at:])
			cell, at = styled(f.Style, r), at+size
		}
		if c == cursor-f.scroll {
			cell.Attrs ^= engine.Reverse
		}
		fb.Set(cx, y, cell)
	}
}

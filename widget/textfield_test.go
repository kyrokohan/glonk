package widget

import (
	"math"
	"strings"
	"testing"
	"unicode"

	"glonk.dev/engine"
)

// edit runs one frame of Update on a field: text typed, then pasted, with keys pressed.
func edit(f *TextField, typed, pasted string, keys ...engine.Key) bool {
	kb := engine.NewKeyboard()
	if typed != "" {
		kb.Type(typed)
	}
	if pasted != "" {
		kb.Paste(pasted)
	}
	kb.Press(keys...)
	return f.Update(kb.Input())
}

// fieldRow returns the characters of row y of fb from x, w of them, and the columns among them
// that are reversed.
func fieldRow(fb *engine.FrameBuffer, x, y, w int) (string, []int) {
	var text []rune
	var reversed []int
	for c := range w {
		cell := fb.At(x+c, y)
		text = append(text, cell.Character)
		if cell.Attrs.Has(engine.Reverse) {
			reversed = append(reversed, c)
		}
	}
	return string(text), reversed
}

func TestTextFieldTypesAtTheCursor(t *testing.T) {
	for _, tt := range []struct {
		name, text  string
		cursor      int
		typed, want string
		wantCursor  int
	}{
		{"into an empty field", "", 0, "abc", "abc", 3},
		{"at the end", "ab", 2, "cd", "abcd", 4},
		{"at the start", "cd", 0, "ab", "abcd", 2},
		{"in the middle", "ad", 1, "bc", "abcd", 3},
		{"after characters of several bytes", "éé", 1, "x", "éxé", 2},
		{"wide, combining and control characters dropped", "", 0, "a中é\tb", "aeb", 3},
		{"a cursor past the end taken as the end", "ab", 9, "c", "abc", 3},
		{"a cursor before the start taken as the start", "bc", -4, "a", "abc", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := TextField{Text: tt.text, Cursor: tt.cursor}
			if entered := edit(&f, tt.typed, ""); entered {
				t.Errorf("wanted typing not to enter")
			}
			if f.Text != tt.want || f.Cursor != tt.wantCursor {
				t.Errorf("wanted %q with the cursor at %d, got %q at %d", tt.want, tt.wantCursor, f.Text, f.Cursor)
			}
		})
	}
}

func TestTextFieldPastesOneLine(t *testing.T) {
	f := TextField{Text: "[]", Cursor: 1}
	edit(&f, "", "one\ntwo\tthree\x1b\n")
	if want := "[one twothree ]"; f.Text != want || f.Cursor != 14 {
		t.Errorf("wanted %q with the cursor before ], got %q at %d", want, f.Text, f.Cursor)
	}
	// Typing comes before the paste in a frame.
	f = TextField{}
	edit(&f, "ab", "cd")
	if f.Text != "abcd" {
		t.Errorf("wanted the typed text and then the paste, got %q", f.Text)
	}
}

func TestTextFieldMaxLengthAndAccept(t *testing.T) {
	f := TextField{MaxLength: 5}
	edit(&f, "abc", "")
	edit(&f, "", "défgh")
	if f.Text != "abcdé" || f.Cursor != 5 {
		t.Errorf("wanted a paste cut at 5 characters, got %q at %d", f.Text, f.Cursor)
	}
	edit(&f, "z", "", engine.KeyHome)
	if f.Text != "abcdé" {
		t.Errorf("wanted nothing typed into a full field, got %q", f.Text)
	}
	long := TextField{Text: "toolong", Cursor: 7, MaxLength: 3}
	edit(&long, "x", "", engine.KeyBackspace)
	if long.Text != "toolon" {
		t.Errorf("wanted text set longer than MaxLength left alone, and nothing more in, got %q", long.Text)
	}

	digits := TextField{Accept: unicode.IsDigit}
	edit(&digits, "a1b2", "3 c4\n")
	if digits.Text != "1234" {
		t.Errorf("wanted only digits, got %q", digits.Text)
	}
	wide := TextField{Accept: func(rune) bool { return true }}
	edit(&wide, "中a", "")
	if wide.Text != "a" {
		t.Errorf("wanted a wide character dropped even when Accept takes it, got %q", wide.Text)
	}
}

func TestTextFieldKeysEditByCharacter(t *testing.T) {
	for _, tt := range []struct {
		name       string
		text       string
		cursor     int
		keys       []engine.Key
		want       string
		wantCursor int
	}{
		{"Backspace deletes before the cursor", "aéc", 2, []engine.Key{engine.KeyBackspace}, "ac", 1},
		{"Backspace at the start does nothing", "ab", 0, []engine.Key{engine.KeyBackspace}, "ab", 0},
		{"Delete deletes after the cursor", "aéc", 1, []engine.Key{engine.KeyDelete}, "ac", 1},
		{"Delete at the end does nothing", "ab", 2, []engine.Key{engine.KeyDelete}, "ab", 2},
		{"Delete at the start", "ab", 0, []engine.Key{engine.KeyDelete}, "b", 0},
		{"Left", "ab", 2, []engine.Key{engine.KeyLeft}, "ab", 1},
		{"Left at the start stays", "ab", 0, []engine.Key{engine.KeyLeft}, "ab", 0},
		{"Right", "ab", 0, []engine.Key{engine.KeyRight}, "ab", 1},
		{"Right at the end stays", "ab", 2, []engine.Key{engine.KeyRight}, "ab", 2},
		{"Home", "abc", 2, []engine.Key{engine.KeyHome}, "abc", 0},
		{"End", "aéc", 0, []engine.Key{engine.KeyEnd}, "aéc", 3},
		{"Delete at the end of a field set too short a cursor", "ab", 7, []engine.Key{engine.KeyDelete}, "ab", 2},
		{"Delete joining bytes that are not UTF-8, then End", "0\xc50\xad", 2, []engine.Key{engine.KeyDelete, engine.KeyEnd}, "0\xc5\xad", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := TextField{Text: tt.text, Cursor: tt.cursor}
			edit(&f, "", "", tt.keys...)
			if f.Text != tt.want || f.Cursor != tt.wantCursor {
				t.Errorf("wanted %q with the cursor at %d, got %q at %d", tt.want, tt.wantCursor, f.Text, f.Cursor)
			}
		})
	}
}

// Text goes in before the keys of the same frame.
func TestTextFieldTypesBeforeTheKeys(t *testing.T) {
	f := TextField{}
	edit(&f, "a", "", engine.KeyBackspace)
	if f.Text != "" || f.Cursor != 0 {
		t.Errorf("wanted a and Backspace in one frame to leave nothing, got %q at %d", f.Text, f.Cursor)
	}
	f = TextField{Text: "xy", Cursor: 2}
	edit(&f, "b", "", engine.KeyBackspace)
	if f.Text != "xy" {
		t.Errorf("wanted Backspace to delete the b typed in its frame, got %q", f.Text)
	}
}

// Enter says so on a new press, once, and leaves the text.
func TestTextFieldEnter(t *testing.T) {
	f := TextField{Text: "name", Cursor: 4}
	kb := engine.NewKeyboard()
	kb.Press(engine.KeyEnter)
	if !f.Update(kb.Input()) || f.Text != "name" {
		t.Fatalf("wanted Enter to enter and keep the text, got %q", f.Text)
	}
	kb.EndFrame()
	kb.Press(engine.KeyEnter) // still held: a repeat
	if f.Update(kb.Input()) {
		t.Errorf("wanted a held Enter to enter once")
	}
	if edit(&f, "", "", engine.KeyLeft) {
		t.Errorf("wanted no Enter without one")
	}
}

func TestTextFieldDraw(t *testing.T) {
	style := engine.Cell{Foreground: engine.Palette(3), Background: engine.Palette(4)}
	fb := frame(t, 20, 3)
	f := TextField{Text: "héllo", Cursor: 1, Style: style}
	f.Draw(fb, 2, 1, 8)
	text, reversed := fieldRow(fb, 2, 1, 8)
	if text != "héllo   " || len(reversed) != 1 || reversed[0] != 1 {
		t.Errorf("wanted %q with column 1 reversed, got %q with %v", "héllo   ", text, reversed)
	}
	if c := fb.At(9, 1); c.Background != style.Background || c.Foreground != style.Foreground {
		t.Errorf("wanted the whole width in the style, got %+v at the last cell", c)
	}
	if c := fb.At(10, 1); c.Background == style.Background {
		t.Errorf("wanted nothing drawn past the width, got %+v", c)
	}

	fb = frame(t, 20, 3)
	f.Cursor = 5
	f.Draw(fb, 2, 1, 8)
	if _, reversed := fieldRow(fb, 2, 1, 8); len(reversed) != 1 || reversed[0] != 5 {
		t.Errorf("wanted a reversed blank after the last character, got %v", reversed)
	}
	if c := fb.At(7, 1); c.Character != ' ' {
		t.Errorf("wanted the cursor after the text to be a blank, got %q", c.Character)
	}

	empty := TextField{}
	fb = frame(t, 20, 3)
	empty.Draw(fb, 0, 0, 4)
	if _, reversed := fieldRow(fb, 0, 0, 4); len(reversed) != 1 || reversed[0] != 0 {
		t.Errorf("wanted an empty field to show its cursor in the first cell, got %v", reversed)
	}
}

// Text wider than the field scrolls as little as keeps the cursor in view, stays put while the
// cursor moves within the view, and comes back as the text gets shorter.
func TestTextFieldScrolls(t *testing.T) {
	f := TextField{Text: "abcdefghij", Cursor: 10}
	draw := func() (string, []int) {
		fb := frame(t, 20, 1)
		f.Draw(fb, 0, 0, 5)
		return fieldRow(fb, 0, 0, 5)
	}
	if text, rev := draw(); text != "ghij " || rev[0] != 4 {
		t.Fatalf("wanted the end in view with the cursor after it, got %q, %v", text, rev)
	}
	f.Cursor = 7
	if text, rev := draw(); text != "ghij " || rev[0] != 1 {
		t.Errorf("wanted the text to stay put while the cursor is in view, got %q, %v", text, rev)
	}
	f.Cursor = 3
	if text, rev := draw(); text != "defgh" || rev[0] != 0 {
		t.Errorf("wanted the text to scroll back just to the cursor, got %q, %v", text, rev)
	}
	f.Cursor = 9
	if text, rev := draw(); text != "fghij" || rev[0] != 4 {
		t.Errorf("wanted the text to scroll on just to the cursor, got %q, %v", text, rev)
	}
	f.Text, f.Cursor = "abcdef", 6
	if text, rev := draw(); text != "cdef " || rev[0] != 4 {
		t.Errorf("wanted shorter text to come back and fill the field, got %q, %v", text, rev)
	}
	f.Text, f.Cursor = "ab", 2
	if text, _ := draw(); text != "ab   " {
		t.Errorf("wanted text that fits shown from the start, got %q", text)
	}
}

func TestTextFieldOffTheFrame(t *testing.T) {
	for _, tt := range []struct {
		name    string
		x, y, w int
	}{
		{"left of the frame", -3, 1, 6},
		{"right of the frame", 17, 1, 6},
		{"above the frame", 2, -1, 6},
		{"below the frame", 2, 3, 6},
		{"no width", 2, 1, 0},
		{"huge", math.MaxInt, math.MaxInt, math.MaxInt},
		{"huge the other way", math.MinInt, 1, math.MaxInt},
		{"from far left onto the frame", math.MinInt + 5, 1, math.MaxInt},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fb := sentinelFrame(t)
			f := TextField{Text: strings.Repeat("ab", 40), Cursor: 3, Style: engine.Cell{Background: engine.Palette(5)}}
			f.Draw(fb, tt.x, tt.y, tt.w)
			untouchedOutside(t, tt.name, fb, func(cx, cy int) bool { return within(cx, tt.x, tt.w) && cy == tt.y })
		})
	}
	// Part of a field on the frame shows that part of it.
	fb := frame(t, 20, 3)
	f := TextField{Text: "abcdef", Cursor: 6}
	f.Draw(fb, -2, 0, 8)
	if text, rev := fieldRow(fb, 0, 0, 6); text != "cdef  " || len(rev) != 1 || rev[0] != 4 {
		t.Errorf("wanted the field from its third cell, got %q, %v", text, rev)
	}
}

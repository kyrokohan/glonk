package web

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The modifier bits a page passes with a key, as the kitty keyboard protocol counts them: Shift, Alt
// (Option on a Mac), Ctrl and Super (Command on a Mac, the Windows key elsewhere).
const (
	modShift = 1 << iota
	modAlt
	modCtrl
	modSuper
)

// The actions a page passes with a key, as the protocol numbers them.
const (
	actionPress   = 1
	actionRepeat  = 2
	actionRelease = 3
)

// namedKeys are the keys a page names by a word, as KeyboardEvent.key does, and the protocol's form
// for each: the number and the final byte of CSI number ; modifiers:action final. F3 is CSI 13 ~
// rather than CSI 1 R, which can be read as a cursor position report.
var namedKeys = map[string]struct {
	n     int
	final byte
}{
	"Escape": {27, 'u'}, "Enter": {13, 'u'}, "Tab": {9, 'u'}, "Backspace": {127, 'u'},
	"ArrowUp": {1, 'A'}, "ArrowDown": {1, 'B'}, "ArrowRight": {1, 'C'}, "ArrowLeft": {1, 'D'},
	"Home": {1, 'H'}, "End": {1, 'F'},
	"F1": {1, 'P'}, "F2": {1, 'Q'}, "F3": {13, '~'}, "F4": {1, 'S'},
	"F5": {15, '~'}, "F6": {17, '~'}, "F7": {18, '~'}, "F8": {19, '~'}, "F9": {20, '~'}, "F10": {21, '~'},
	"F11": {23, '~'}, "F12": {24, '~'},
	"Insert": {2, '~'}, "Delete": {3, '~'}, "PageUp": {5, '~'}, "PageDown": {6, '~'},
}

// unshifted is the character of a key the page names by its place, KeyboardEvent.code, on a US layout,
// for the keys that are neither a letter nor a digit: the protocol names a key by the character it
// types without Shift.
var unshifted = map[string]rune{
	"Space": ' ', "Minus": '-', "Equal": '=', "BracketLeft": '[', "BracketRight": ']', "Backslash": '\\',
	"Semicolon": ';', "Quote": '\'', "Backquote": '`', "Comma": ',', "Period": '.', "Slash": '/',
}

// keyBytes is what a terminal with the kitty keyboard protocol sends for a key a page reports: key and
// code are KeyboardEvent.key and .code, mods the modifier bits and action a press, a repeat or a release.
// A key glonk does not read gives nothing: a modifier alone, a dead key, a key the page does not name,
// and any key with Super, which is the browser's and the system's.
func keyBytes(key, code string, mods, action int) []byte {
	if mods&modSuper != 0 || action < actionPress || action > actionRelease {
		return nil
	}
	m := 1 + mods&(modShift|modAlt|modCtrl)
	if k, ok := namedKeys[key]; ok {
		return fmt.Appendf(nil, "\x1b[%d;%d:%d%c", k.n, m, action, k.final)
	}
	if !typed(key) {
		return nil
	}
	base := baseKey(key, code)
	// A key types its text when it goes down or repeats, and not with Ctrl, which makes it a shortcut.
	if action == actionRelease || mods&modCtrl != 0 {
		return fmt.Appendf(nil, "\x1b[%d;%d:%du", base, m, action)
	}
	points := make([]string, 0, utf8.RuneCountInString(key))
	for _, r := range key {
		points = append(points, fmt.Sprint(r))
	}
	return fmt.Appendf(nil, "\x1b[%d;%d:%d;%su", base, m, action, strings.Join(points, ":"))
}

// typed reports whether key is the text a key types, as KeyboardEvent.key gives it, and not the name of
// a key: one character, or more where they are not all ASCII letters and digits, as a word such as
// Shift, Dead or CapsLock is. Every character must be printable.
func typed(key string) bool {
	if key == "" {
		return false
	}
	word := utf8.RuneCountInString(key) > 1
	for _, r := range key {
		if !unicode.IsGraphic(r) {
			return false
		}
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			word = false
		}
	}
	return !word
}

// baseKey is the code the protocol names a key by: the character it types without Shift. A letter or a
// digit, or one of the keys in unshifted, comes from where the key is, so Shift and 1 is still the 1
// key; any other key is its own text, lowered.
func baseKey(key, code string) rune {
	switch {
	case len(code) == 4 && strings.HasPrefix(code, "Key") && code[3] >= 'A' && code[3] <= 'Z':
		return rune(code[3] - 'A' + 'a')
	case len(code) == 6 && strings.HasPrefix(code, "Digit") && code[5] >= '0' && code[5] <= '9':
		return rune(code[5])
	}
	if r, ok := unshifted[code]; ok {
		return r
	}
	r, _ := utf8.DecodeRuneInString(key)
	return unicode.ToLower(r)
}

// pasteBytes is what a terminal with bracketed paste sends for text pasted into the page. An escape in
// it is taken out, so that the paste can neither end itself early nor carry a sequence.
func pasteBytes(text string) []byte {
	return []byte("\x1b[200~" + strings.ReplaceAll(text, "\x1b", "") + "\x1b[201~")
}

// pointerBytes is the decimal mouse report (SGR, mode 1006) for what the pointer did in a cell, counted
// from 0 at the top left: kind is "move", "down", "up" or "wheel". button is 0 for the left, 1 the
// middle and 2 the right, and for a move the one held down, or -1 for none; for the wheel it is 0 up,
// 1 down, 2 left and 3 right. Anything else gives nothing.
func pointerBytes(kind string, button, col, row int) []byte {
	col, row = max(col, 0)+1, max(row, 0)+1
	final, cb := byte('M'), button
	switch {
	case kind == "move" && button == -1:
		cb = 35
	case kind == "move" && button >= 0 && button <= 2:
		cb = 32 + button
	case (kind == "down" || kind == "up") && button >= 0 && button <= 2:
		if kind == "up" {
			final = 'm'
		}
	case kind == "wheel" && button >= 0 && button <= 3:
		cb = 64 + button
	default:
		return nil
	}
	return fmt.Appendf(nil, "\x1b[<%d;%d;%d%c", cb, col, row, final)
}

// focusBytes is what a terminal sends when its window gains or loses focus: losing it lets every key go,
// since their releases will not come.
func focusBytes(in bool) []byte {
	if in {
		return []byte("\x1b[I")
	}
	return []byte("\x1b[O")
}

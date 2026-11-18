package engine

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Key is a keypress: printable keys are their own rune, special keys are negative.
//
// The function keys and Home, End, Page Up, Page Down, Insert and Delete are read in the forms
// that xterm, rxvt and the kitty keyboard protocol send them in. A terminal that sends one in
// another form sends nothing that glonk reads. A modifier held with one of them, as with an
// arrow, is not looked at by IsDown, Pressed and Started: Ctrl+PageDown is PageDown there, and
// Input.KeyEvents says Ctrl was held.
type Key rune

// Special keys. Negative values so they can never collide with a real rune.
const (
	// KeyUnknown stands for no particular key. Input never reports it as pressed.
	KeyUnknown Key = -1 - iota
	// KeyUp is the up arrow.
	KeyUp
	// KeyDown is the down arrow.
	KeyDown
	// KeyLeft is the left arrow.
	KeyLeft
	// KeyRight is the right arrow.
	KeyRight
	// KeyEscape is the Escape key. Where the terminal cannot tell it from the start
	// of an arrow key, it counts after 50 ms with nothing following.
	KeyEscape
	// KeyEnter is the Enter (Return) key.
	KeyEnter
	// KeyBackspace is the Backspace key.
	KeyBackspace
	// KeyTab is the Tab key.
	KeyTab
	// KeyCtrlC is Ctrl-C. Raw mode turns off the terminal's own handling of it, so
	// it arrives as a key; Run ends the program when it is pressed, so Update never
	// sees it.
	KeyCtrlC
	// keyCtrlZ asks Run to suspend. Like Ctrl-C it is Run's, never the game's.
	keyCtrlZ

	// KeyHome is the Home key.
	KeyHome
	// KeyEnd is the End key.
	KeyEnd
	// KeyPageUp is the Page Up key.
	KeyPageUp
	// KeyPageDown is the Page Down key.
	KeyPageDown
	// KeyInsert is the Insert key.
	KeyInsert
	// KeyDelete is the Delete key, the one that deletes forward. Backspace is KeyBackspace.
	KeyDelete
	// KeyF1 is the F1 function key.
	KeyF1
	// KeyF2 is the F2 function key.
	KeyF2
	// KeyF3 is the F3 function key.
	KeyF3
	// KeyF4 is the F4 function key.
	KeyF4
	// KeyF5 is the F5 function key.
	KeyF5
	// KeyF6 is the F6 function key.
	KeyF6
	// KeyF7 is the F7 function key.
	KeyF7
	// KeyF8 is the F8 function key.
	KeyF8
	// KeyF9 is the F9 function key.
	KeyF9
	// KeyF10 is the F10 function key.
	KeyF10
	// KeyF11 is the F11 function key.
	KeyF11
	// KeyF12 is the F12 function key.
	KeyF12
)

// keyKind says what happened to a key. Only terminals speaking the kitty
// keyboard protocol report repeats and releases; everything else sends presses.
// The focus kinds are not about a key: they carry KeyUnknown and tell Input the
// window gained or lost focus, which we only ask for alongside the protocol.
// kindReset also carries KeyUnknown: part of the input was thrown away, so any
// release in it may have been lost and Input must forget which keys are held.
// kindPaste carries KeyUnknown and the pasted text, and kindPasteStart is the parser's
// own: the marker that opens a paste was read, and the keyParser, which never hands
// it on, collects what follows. The mouse kinds are not about a key either: key holds the cell the
// pointer was in (see cellKey), btn the button or the wheel direction (see mouse.go), and a
// kindMouseMove has no button.
type keyKind uint8

const (
	kindPress keyKind = iota
	kindRepeat
	kindRelease
	kindFocusGained
	kindFocusLost
	kindReset
	kindPaste
	kindPasteStart
	kindMouseMove
	kindMouseDown
	kindMouseUp
	kindWheel
)

// keyEvent is one decoded key event.
type keyEvent struct {
	key  Key
	kind keyKind

	// mods are the modifiers the terminal said were held, and chord is whether this is a shortcut (see
	// chordOf), which only Input.KeyEvents sees. text is what a terminal with the kitty keyboard protocol said
	// the key typed; where it is empty, Input works it out from the key (see textOf).
	mods  Mods
	chord bool
	btn   uint8 // which button or wheel direction a mouse event is about: it fits where the struct has padding
	text  string
}

// Kitty keyboard protocol modifier bits that are not read, after subtracting the protocol's +1 offset: a
// chord with Hyper or Meta is dropped, as every chord was before there was an API for them. Shift, Alt, Ctrl
// and Super are Mods, and the lock keys are not modifiers of a shortcut.
const (
	modHyper = 1 << 4
	modMeta  = 1 << 5
)

// privateUseStart is where the protocol puts keys with no character, such as a
// lone Shift or a keypad key. We have no use for them.
const privateUseStart = 57344

// arrow maps the final byte of an arrow-key escape sequence to its Key.
func arrow(c byte) Key {
	switch c {
	case 'A':
		return KeyUp
	case 'B':
		return KeyDown
	case 'C':
		return KeyRight
	case 'D':
		return KeyLeft
	default:
		return KeyUnknown
	}
}

// functionKeys is F1 to F12 in order, so that a number from a sequence finds its key.
var functionKeys = [...]Key{KeyF1, KeyF2, KeyF3, KeyF4, KeyF5, KeyF6, KeyF7, KeyF8, KeyF9, KeyF10, KeyF11, KeyF12}

// ss3Key maps the final byte of an SS3 sequence (ESC O x) to its key: the four arrows
// (cursor keys in application mode), Home and End, and F1 to F4, which xterm sends this way.
func ss3Key(c byte) Key {
	switch {
	case c >= 'A' && c <= 'D':
		return arrow(c)
	case c == 'H':
		return KeyHome
	case c == 'F':
		return KeyEnd
	case c >= 'P' && c <= 'S':
		return functionKeys[c-'P']
	}
	return KeyUnknown
}

// tildeKey maps the number of a CSI n ~ sequence to its key: the editing keys, and the
// function keys, which skip 16 and 22. Home and End have two numbers each, 1 and 7 and 4 and 8,
// because xterm and rxvt differ. Any other number is not a key we read.
func tildeKey(n int) Key {
	switch n {
	case 1, 7:
		return KeyHome
	case 2:
		return KeyInsert
	case 3:
		return KeyDelete
	case 4, 8:
		return KeyEnd
	case 5:
		return KeyPageUp
	case 6:
		return KeyPageDown
	case 11, 12, 13, 14, 15:
		return functionKeys[n-11] // F1 to F5: 11 to 15
	case 17, 18, 19, 20, 21:
		return functionKeys[n-17+5] // F6 to F10: 17 to 21
	case 23, 24:
		return functionKeys[n-23+10] // F11 and F12: 23 and 24
	}
	return KeyUnknown
}

// maxPendingInput bounds how many bytes of an unfinished sequence keyParser holds.
// Real key sequences are a few bytes long; more than this is garbage.
const maxPendingInput = 128

// parseEvents decodes input that is known to be complete into key events. For
// input that arrives in pieces use a keyParser.
func parseEvents(b []byte) []keyEvent {
	var p keyParser
	return append(p.Feed(b), p.Flush()...)
}

// parse decodes raw terminal input into key events and reports how many bytes it
// used.
//
// A sequence cut off at the end of b is not used when final is false: the caller
// keeps those bytes and tries again once more have arrived. When final is true
// nothing more is coming, so a lone ESC is the Escape key, an unfinished UTF-8
// rune is dropped, and an unfinished CSI sequence is dropped with a reset event,
// since the release it may have carried is lost.
//
// ESC followed by a key is an Alt chord (Alt+x arrives as ESC x). Games do not get
// chords, and reading them as Escape then x would make Alt+anything press Escape,
// so they are dropped, as the kitty protocol path already does.
func parse(b []byte, final bool) (events []keyEvent, consumed int) {
	press := func(k Key) { events = append(events, keyEvent{key: k, kind: kindPress}) }

	i := 0
	for i < len(b) {
		c := b[i]
		switch {
		case c == 0x1b:
			if i+1 >= len(b) {
				if !final {
					return events, i
				}
				press(KeyEscape)
				i++
				continue
			}

			switch next := b[i+1]; {
			case next == '[':
				j := i + 2
				// Skipping parameters lets modified arrows (ESC [1;5C) read as
				// plain arrows and unknown keys (ESC [16~) drop cleanly.
				for j < len(b) && b[j] >= 0x20 && b[j] <= 0x3f {
					j++
				}
				if j >= len(b) {
					if !final {
						return events, i
					}
					events = append(events, keyEvent{key: KeyUnknown, kind: kindReset})
					i = len(b)
					continue
				}
				if b[j] == '~' && string(b[i+2:j]) == "200" {
					// A paste is text, not keys: what follows is the keyParser's to collect.
					return append(events, keyEvent{key: KeyUnknown, kind: kindPasteStart}), j + 1
				}
				if b[j] == 'M' && j == i+2 {
					// The old mouse report, ESC [ M and a byte each for button, column and row, which a terminal
					// sends where it has the mouse and not the decimal form glonk asks for. No key is this, so it
					// is dropped with its three bytes, which would otherwise be read as typed characters.
					if j+3 >= len(b) {
						if !final {
							return events, i
						}
						events = append(events, keyEvent{key: KeyUnknown, kind: kindReset})
						i = len(b)
						continue
					}
					i = j + 4
					continue
				}
				events = appendCSI(events, b[i+2:j], b[j])
				i = j + 1

			case next == 'O':
				// Some terminals send arrows, Home, End and F1 to F4 as SS3 (ESC O A) instead of CSI.
				if i+2 >= len(b) {
					if !final {
						return events, i
					}
					i += 2 // Alt+O
					continue
				}
				switch f := b[i+2]; {
				case ss3Key(f) != KeyUnknown:
					press(ss3Key(f))
					i += 3
				case f >= 0x40 && f <= 0x7e:
					i += 3 // keypad and the rest: not keys we read
				default:
					i += 2 // Alt+O; f is read on its own
				}

			case next == 0x1b:
				press(KeyEscape) // the first of two; the second is read next
				i++

			default: // Alt chord: ESC and a key
				r, size := rune(next), 1
				if next >= utf8.RuneSelf {
					if !final && !utf8.FullRune(b[i+1:]) {
						return events, i
					}
					r, size = utf8.DecodeRune(b[i+1:])
				}
				switch {
				case next == 0x7f:
					events = append(events, keyEvent{key: KeyBackspace, kind: kindPress, mods: ModAlt, chord: true})
				case r >= ' ' && !(r == utf8.RuneError && size == 1):
					events = append(events, keyEvent{key: Key(r), kind: kindPress, mods: ModAlt, chord: true})
				}
				i += 1 + size
			}
		case c == 0x03:
			press(KeyCtrlC)
			i++
		case c == 0x1a:
			press(keyCtrlZ)
			i++
		case c == '\r':
			press(KeyEnter)
			i++
		case c == 0x7f || c == 0x08:
			press(KeyBackspace)
			i++
		case c == '\t':
			press(KeyTab)
			i++
		case c >= 0x01 && c <= 0x1a:
			// Ctrl and a letter arrive as the control byte, 1 for a to 26 for z. The ones that are
			// also a key of their own, Ctrl-C and Ctrl-Z above, Ctrl-H, Ctrl-I and Ctrl-M, are read
			// as that key; Ctrl-J, a newline, is Ctrl and j.
			events = append(events, keyEvent{key: Key('a' + c - 1), kind: kindPress, mods: ModCtrl, chord: true})
			i++
		case c < 0x20:
			i++
		default:
			if !final && !utf8.FullRune(b[i:]) {
				return events, i
			}
			r, size := utf8.DecodeRune(b[i:])
			if !(r == utf8.RuneError && size == 1) {
				press(Key(r))
			}
			i += size
		}
	}
	return events, i
}

// keyParser decodes input that arrives in pieces. Terminals send a key as a short
// sequence of bytes, and a read can end in the middle of one; the parser keeps the
// unfinished tail and joins it to the next piece.
//
// A tail that is a lone ESC cannot be told from the Escape key until a moment has
// passed without more bytes: the caller checks Pending after each Feed, waits, and
// calls Flush if nothing came.
//
// A paste is text between two markers, and can be many reads long, so it is kept apart
// from the unfinished sequence: see paste.go. While one is being collected Pending is
// true, and Flush ends it as it stands.
type keyParser struct {
	pending []byte

	inPaste bool   // the marker that opens a paste was read and its end not yet
	paste   []byte // what has been collected of it, up to maxPaste
	hold    []byte // the last bytes of it, which may be the start of the end marker
	scan    []byte // room to look at hold and the next read together
}

// Feed decodes the next piece of input after whatever was left over from the last.
func (p *keyParser) Feed(b []byte) []keyEvent {
	data := b
	if len(p.pending) > 0 {
		data = append(p.pending, b...)
		p.pending = p.pending[:0] // data has it now
	}

	var events []keyEvent
	for {
		if p.inPaste {
			n, done := p.collect(data)
			data = data[n:]
			if !done {
				return events // all of it was the paste's
			}
			events = append(events, keyEvent{key: KeyUnknown, kind: kindPaste, text: p.take()})
		}

		keys, n := parse(data, false)
		if k := len(keys); k > 0 && keys[k-1].kind == kindPasteStart {
			events = append(events, keys[:k-1]...)
			p.inPaste = true
			data = data[n:]
			continue
		}
		if events == nil {
			events = keys // nothing before it was a paste, so no copy
		} else {
			events = append(events, keys...)
		}
		rest := data[n:]
		if len(rest) > maxPendingInput {
			// No key sequence is this long, so this is garbage that would only grow.
			events = append(events, keyEvent{key: KeyUnknown, kind: kindReset})
			rest = nil
		}
		p.pending = append(p.pending[:0], rest...)
		return events
	}
}

// Pending reports whether an unfinished sequence, or a paste, is being held.
func (p *keyParser) Pending() bool {
	return len(p.pending) > 0 || p.inPaste
}

// Pasting reports whether a paste is being collected.
func (p *keyParser) Pasting() bool {
	return p.inPaste
}

// Flush gives up waiting: it decodes the held bytes as the end of the input, and delivers a
// paste that has not ended as it stands.
func (p *keyParser) Flush() []keyEvent {
	var events []keyEvent
	if p.inPaste {
		p.addPaste(p.hold) // what might have begun the end marker is text after all
		events = append(events, keyEvent{key: KeyUnknown, kind: kindPaste, text: p.take()})
	}
	keys, _ := parse(p.pending, true)
	p.pending = p.pending[:0]
	if events == nil {
		return keys
	}
	return append(events, keys...)
}

// appendCSI decodes a CSI sequence (parameter bytes plus final byte) and appends
// its key event, if it is one we understand. Anything else is dropped.
func appendCSI(events []keyEvent, params []byte, final byte) []keyEvent {
	// '<' to '?' as the first parameter byte marks a private sequence, such as a
	// protocol reply, not a key. CSI < ... M and m are the one kind we read: a mouse report.
	if len(params) > 0 && params[0] >= '<' && params[0] <= '?' {
		if params[0] == '<' && (final == 'M' || final == 'm') {
			return appendMouse(events, params[1:], final)
		}
		return events
	}

	// Focus reports are a bare CSI I or CSI O.
	if len(params) == 0 {
		switch final {
		case 'I':
			return append(events, keyEvent{key: KeyUnknown, kind: kindFocusGained})
		case 'O':
			return append(events, keyEvent{key: KeyUnknown, kind: kindFocusLost})
		}
	}

	fields := csiParams(params)

	switch {
	case final >= 'A' && final <= 'D':
		// The modifiers are kept for Input.KeyEvents and are not looked at by IsDown, Pressed and
		// Started, so ctrl+right still turns right, and so do the keys below.
		return appendCSIKey(events, arrow(final), fields)
	case final == 'H' || final == 'F' || (final >= 'P' && final <= 'S'):
		// Home, End and F1 to F4 are CSI 1 ; modifiers X, or a bare CSI X. Another first
		// parameter is not one of them: CSI row ; column R, for one, is a cursor position
		// report, which glonk never asks for but which this keeps from being read as F3.
		if len(params) > 0 && param(fields, 0, 0) != 1 {
			return events
		}
		return appendCSIKey(events, ss3Key(final), fields)
	case final == '~':
		if key := tildeKey(param(fields, 0, 0)); key != KeyUnknown {
			return appendCSIKey(events, key, fields)
		}
	case final == 'u':
		return appendKitty(events, fields)
	}
	return events
}

// appendCSIKey appends the key a CSI sequence named, with the modifiers its second parameter says:
// 1 plus the bits, as in the kitty protocol and in xterm (CSI 1 ; 5 C is Ctrl and the right arrow).
func appendCSIKey(events []keyEvent, key Key, fields [][]int) []keyEvent {
	mods := modsOf(max(param(fields, 1, 0), 1) - 1)
	return append(events, keyEvent{key: key, kind: eventKind(fields), mods: mods, chord: chordOf(key, mods)})
}

// modsOf is the Mods of the protocol's modifier bits: shift, alt, ctrl and super, with the lock keys and
// anything else left out. A number that is not a plausible set, from a damaged sequence, is no modifiers.
func modsOf(bits int) Mods {
	if bits < 0 || bits > 0xff {
		return 0
	}
	return Mods(bits) & modsMask
}

// appendKitty decodes a kitty keyboard protocol key event: CSI code ; mods:event ; text u.
func appendKitty(events []keyEvent, fields [][]int) []keyEvent {
	code := param(fields, 0, 0)
	// The protocol sends modifiers as 1 plus their bits; absent means none.
	raw := max(param(fields, 1, 0), 1) - 1
	mods := modsOf(raw)

	var key Key
	switch {
	case mods&chordMods == ModCtrl && code == 'c' && raw&(modHyper|modMeta) == 0:
		key = KeyCtrlC
	case mods&chordMods == ModCtrl && code == 'z' && raw&(modHyper|modMeta) == 0:
		key = keyCtrlZ
	case raw&(modHyper|modMeta) != 0:
		return events // a Hyper or Meta chord is not read
	case code == 27:
		key = KeyEscape
	case code == 13:
		key = KeyEnter
	case code == 9:
		key = KeyTab
	case code == 127:
		key = KeyBackspace
	case code >= 32 && code < privateUseStart && utf8.ValidRune(rune(code)):
		key = Key(code)
	default:
		return events
	}

	if key == KeyCtrlC || key == keyCtrlZ {
		// The key is the chord: Run's, with no modifiers to report.
		return append(events, keyEvent{key: key, kind: eventKind(fields)})
	}
	e := keyEvent{key: key, kind: eventKind(fields), mods: mods, chord: chordOf(key, mods)}
	if e.kind != kindRelease {
		e.text = kittyText(fields)
	}
	return append(events, e)
}

// kittyText is the text a key event carries in its third field, which the terminal sends when it was
// asked for it (flag 16): the code points of the characters, joined by colons. Anything that is not a
// character to show, a control character or a code that is not one, is left out.
func kittyText(fields [][]int) string {
	if len(fields) < 3 {
		return ""
	}
	var text []rune
	for _, cp := range fields[2] {
		// Bounded before it is a rune: a number past the last one would wrap into a valid one.
		if cp >= ' ' && cp <= utf8.MaxRune && !(cp >= 0x7f && cp < 0xa0) && utf8.ValidRune(rune(cp)) && len(text) < 16 {
			text = append(text, rune(cp))
		}
	}
	return string(text)
}

// eventKind reads the event type from the second sub-value of the modifiers field.
func eventKind(fields [][]int) keyKind {
	switch param(fields, 1, 1) {
	case 2:
		return kindRepeat
	case 3:
		return kindRelease
	default:
		return kindPress
	}
}

// csiParams splits CSI parameter bytes such as "97;5:3" into fields of sub-values.
// Empty or unreadable values come back as 0, which every caller treats as "not given".
func csiParams(p []byte) [][]int {
	var fields [][]int
	for _, f := range strings.Split(string(p), ";") {
		var subs []int
		for _, s := range strings.Split(f, ":") {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 {
				n = 0
			}
			subs = append(subs, n)
		}
		fields = append(fields, subs)
	}
	return fields
}

// param returns sub-value s of field f, or 0 if it is absent.
func param(fields [][]int, f, s int) int {
	if f >= len(fields) || s >= len(fields[f]) {
		return 0
	}
	return fields[f][s]
}

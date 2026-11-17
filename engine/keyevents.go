package engine

import (
	"slices"
	"unicode/utf8"
)

// Mods is a set of modifier keys held with a key. A terminal says which are down only in some
// cases, see [Input.KeyEvents]. The bits are the ones the kitty keyboard protocol and xterm use,
// so that a modifier a terminal sends is the set.
type Mods uint8

// The modifiers.
const (
	// ModShift is the Shift key.
	ModShift Mods = 1 << iota
	// ModAlt is the Alt key, which is Option on a Mac.
	ModAlt
	// ModCtrl is the Ctrl key.
	ModCtrl
	// ModSuper is the Super key: the Windows key, or Command on a Mac.
	ModSuper
)

// Has reports whether every modifier in want is in m.
func (m Mods) Has(want Mods) bool {
	return m&want == want
}

// KeyAction is what happened to a key.
type KeyAction uint8

// The things that happen to a key.
const (
	// ActionPress is a key going down. Without the kitty keyboard protocol it is also every auto-repeat.
	ActionPress KeyAction = iota
	// ActionRepeat is a held key sending another press. Only terminals with the kitty keyboard
	// protocol say so.
	ActionRepeat
	// ActionRelease is a key coming up. Only terminals with the kitty keyboard protocol say so.
	ActionRelease
)

// KeyEvent is one thing that happened to a key, with the modifiers held at the time.
type KeyEvent struct {
	// Key is the key, as [Input] reports it: a letter is lowercase on a terminal with the kitty keyboard
	// protocol and has the case Shift gives it on one without, and a special key is one of the Key constants.
	Key Key
	// Mods are the modifiers held with it, as far as the terminal said.
	Mods Mods
	// Action is a press, a repeat or a release.
	Action KeyAction
}

const (
	// modsMask is every modifier that Mods can hold: the shift, alt, ctrl and super bits of the protocol.
	modsMask = ModShift | ModAlt | ModCtrl | ModSuper

	// chordMods are the modifiers that make a character key a shortcut and not the key itself.
	// Shift does not: it only changes which character the key is.
	chordMods = ModAlt | ModCtrl | ModSuper

	// maxFrameEvents and maxFrameText bound what one frame of Input keeps of the key events and the
	// typed text, so that input nobody is reading cannot grow without end. A frame is a thirtieth of a
	// second or so of typing; the limits are far above what a person does in it.
	maxFrameEvents = 256
	maxFrameText   = 4096
)

// modifiersIgnored reports whether the modifiers held with k are not looked at by IsDown, Pressed and
// Started: the arrows, the navigation keys and the function keys. Ctrl+Right is Right there, so a game
// that turns right on the arrow turns right when Ctrl is down too.
func modifiersIgnored(k Key) bool {
	switch k {
	case KeyUp, KeyDown, KeyLeft, KeyRight, KeyHome, KeyEnd, KeyPageUp, KeyPageDown, KeyInsert, KeyDelete:
		return true
	}
	return slices.Contains(functionKeys[:], k)
}

// chordOf reports whether k with mods is a chord: a shortcut, such as Ctrl+W, which is not the plain key.
// Only [Input.KeyEvents] sees a chord; IsDown, Pressed and Started do not, so a game that walks on W is not
// walked by Ctrl+W, which a terminal may use to close a tab.
func chordOf(k Key, mods Mods) bool {
	return mods&chordMods != 0 && !modifiersIgnored(k)
}

// textOf is the characters that a key event types, when the terminal did not say: the key's own character,
// with Shift applied to the ASCII letters, since the layout is not known. A chord, a special key and a
// release type nothing.
func textOf(e keyEvent) string {
	r := rune(e.key)
	if e.kind == kindRelease || e.chord || r < ' ' || (r >= 0x7f && r < 0xa0) || !utf8.ValidRune(r) {
		return ""
	}
	if e.mods&ModShift != 0 && r >= 'a' && r <= 'z' {
		r -= 'a' - 'A'
	}
	return string(r)
}

// actionOf is the KeyAction of a key event kind: a press, a repeat or a release.
func actionOf(kind keyKind) KeyAction {
	switch kind {
	case kindRepeat:
		return ActionRepeat
	case kindRelease:
		return ActionRelease
	}
	return ActionPress
}

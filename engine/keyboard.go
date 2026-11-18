package engine

import (
	"time"
	"unicode/utf8"
)

// Keyboard simulates a keyboard for testing a Game's Update. NewKeyboard behaves like a
// terminal that reports key releases: a key is down from Press until Release, and the
// Input it hands over says so exactly, with no clock involved. NewLegacyKeyboard behaves
// like one that does not, which most terminals are, so that a game can test both. PressWith
// presses a chord such as Ctrl+S, Type types text and Paste pastes it, for a game that reads
// Input.KeyEvents, Input.Text and Input.Paste.
//
//	kb := engine.NewKeyboard()
//	kb.Press(engine.KeyEscape)
//	err := game.Update(1.0/30, kb.Input()) // engine.ErrQuit, if the game quits on Escape
//	kb.EndFrame()
//
// Ctrl-C and Ctrl-Z never reach a game's Input, because Run handles them, so a
// Keyboard has no business pressing them.
type Keyboard struct {
	in  *Input
	now time.Time // the clock of a keyboard without releases; the other kind needs none
}

// NewKeyboard returns a Keyboard with no keys down, that acts like a terminal that reports
// key releases: its Input.Exact is true.
func NewKeyboard() *Keyboard {
	return &Keyboard{in: newInput(true)}
}

// NewLegacyKeyboard returns a Keyboard with no keys down, that acts like a terminal that
// reports no key releases: its Input.Exact is false, and a key counts as held for about
// 150 ms after the last Press, as it does in Run. No release ever arrives, so Release does
// nothing: a key is let go by moving the clock on with Advance, with no Press. A key held
// past the delay before the OS repeats it starts twice (see Input.Started), which Advance
// lets a test see.
func NewLegacyKeyboard() *Keyboard {
	return &Keyboard{in: newInput(false)}
}

// Input returns the Input to give Update. It is the same Input every time, and it
// follows the keyboard.
func (k *Keyboard) Input() *Input {
	return k.in
}

// Press puts keys down. They count as pressed (Input.Pressed) until EndFrame, and
// stay down (Input.IsDown) until Release. Pressing a key that is already down is
// a key repeat: it is pressed again, but it has not started (Input.Started), and in
// Input.KeyEvents it is an ActionRepeat. On a NewLegacyKeyboard a Press is a press or a
// repeat arriving, which it cannot tell apart (an ActionPress either way), and the key stays
// down for about 150 ms after it. A printable key also types its character (Input.Text).
func (k *Keyboard) Press(keys ...Key) {
	for _, key := range keys {
		kind := kindPress
		if k.in.exact && k.in.held[key] {
			kind = kindRepeat
		}
		k.in.event(keyEvent{key: key, kind: kind}, k.now)
	}
}

// PressWith puts keys down with modifiers held, as a terminal with the kitty keyboard protocol reports
// them. With Ctrl, Alt or Super held a character key is a chord, such as Ctrl+S: it is in
// Input.KeyEvents and nowhere else, and being no more than an event it is not held, so there is nothing
// to release. Special keys keep their modifiers in KeyEvents and are pressed as with Press. With only
// Shift, a letter types its capital, as on a terminal that says what a key types, and Text is right.
func (k *Keyboard) PressWith(mods Mods, keys ...Key) {
	mods &= modsMask
	for _, key := range keys {
		k.in.event(keyEvent{key: key, kind: kindPress, mods: mods, chord: chordOf(key, mods)}, k.now)
	}
}

// Type types text, a key at a time, as a person does: each character is pressed and let go, so it
// is in Input.Text and Input.KeyEvents and no key is left held. A newline is Enter and a tab is Tab,
// which type nothing; other control characters and invalid UTF-8 are left out. On a NewLegacyKeyboard
// each key counts as held for about 150 ms after, as after any Press.
func (k *Keyboard) Type(text string) {
	for _, r := range text {
		key := Key(r)
		switch {
		case r == '\n':
			key = KeyEnter
		case r == '\t':
			key = KeyTab
		case r < ' ' || r == 0x7f || r == utf8.RuneError:
			continue
		}
		k.Press(key)
		k.Release(key)
	}
}

// Paste pastes text, as a terminal with bracketed paste sends it: it is in Input.Paste and nowhere
// else, no key is pressed and nothing is typed. The text is what a real paste would be: valid UTF-8,
// line breaks as newlines, at most 1 MiB. Several in a frame are joined.
func (k *Keyboard) Paste(text string) {
	k.in.event(keyEvent{key: KeyUnknown, kind: kindPaste, text: pasteText(text)}, k.now)
}

// Release lets keys go. A key pressed and released before the frame ends is still
// seen as down by that frame's Update, as a quick tap is on a real terminal.
// Releasing a key that is not down does nothing, and neither does releasing one on a
// NewLegacyKeyboard, where no release is ever reported.
func (k *Keyboard) Release(keys ...Key) {
	for _, key := range keys {
		k.in.event(keyEvent{key: key, kind: kindRelease}, k.now)
	}
}

// Advance moves the clock of the Keyboard on by d, as the time between two frames passes.
// On a NewLegacyKeyboard it is what lets a key go, about 150 ms after its last Press, and
// what puts the delay before the OS starts repeating a key between two Presses. A Keyboard
// from NewKeyboard needs no clock, so for it nothing changes that a test can see. A d that
// is not positive does nothing.
func (k *Keyboard) Advance(d time.Duration) {
	if d <= 0 {
		return
	}
	k.now = k.now.Add(d)
	k.in.beginFrame(k.now)
}

// EndFrame does what Run does after each Update: the presses of that frame are
// forgotten, so Input.Pressed is false again, and keys still held stay down.
func (k *Keyboard) EndFrame() {
	k.in.endFrame()
}

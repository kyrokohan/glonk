package engine

import "time"

// Keyboard simulates a keyboard for testing a Game's Update. NewKeyboard behaves like a
// terminal that reports key releases: a key is down from Press until Release, and the
// Input it hands over says so exactly, with no clock involved. NewLegacyKeyboard behaves
// like one that does not, which most terminals are, so that a game can test both.
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
// a key repeat: it is pressed again, but it has not started (Input.Started). On a
// NewLegacyKeyboard a Press is a press or a repeat arriving, and the key stays down for
// about 150 ms after it.
func (k *Keyboard) Press(keys ...Key) {
	for _, key := range keys {
		k.in.event(keyEvent{key, kindPress}, k.now)
	}
}

// Release lets keys go. A key pressed and released before the frame ends is still
// seen as down by that frame's Update, as a quick tap is on a real terminal.
// Releasing a key that is not down does nothing, and neither does releasing one on a
// NewLegacyKeyboard, where no release is ever reported.
func (k *Keyboard) Release(keys ...Key) {
	for _, key := range keys {
		k.in.event(keyEvent{key, kindRelease}, k.now)
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

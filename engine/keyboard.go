package engine

import "time"

// Keyboard simulates a keyboard for testing a Game's Update. It behaves like a
// terminal that reports key releases: a key is down from Press until Release, and
// the Input it hands over says so exactly, with no clock involved.
//
//	kb := engine.NewKeyboard()
//	kb.Press(engine.KeyEscape)
//	err := game.Update(1.0/30, kb.Input()) // engine.ErrQuit, if the game quits on Escape
//	kb.EndFrame()
//
// Ctrl-C and Ctrl-Z never reach a game's Input, because Run handles them, so a
// Keyboard has no business pressing them.
type Keyboard struct {
	in *Input
}

// NewKeyboard returns a Keyboard with no keys down.
func NewKeyboard() *Keyboard {
	return &Keyboard{in: newInput(true)}
}

// Input returns the Input to give Update. It is the same Input every time, and it
// follows the keyboard.
func (k *Keyboard) Input() *Input {
	return k.in
}

// Press puts keys down. They count as pressed (Input.Pressed) until EndFrame, and
// stay down (Input.IsDown) until Release. Pressing a key that is already down is
// a key repeat: it is pressed again, but it has not started (Input.Started).
func (k *Keyboard) Press(keys ...Key) {
	for _, key := range keys {
		k.in.event(keyEvent{key, kindPress}, time.Time{})
	}
}

// Release lets keys go. A key pressed and released before the frame ends is still
// seen as down by that frame's Update, as a quick tap is on a real terminal.
// Releasing a key that is not down does nothing.
func (k *Keyboard) Release(keys ...Key) {
	for _, key := range keys {
		k.in.event(keyEvent{key, kindRelease}, time.Time{})
	}
}

// EndFrame does what Run does after each Update: the presses of that frame are
// forgotten, so Input.Pressed is false again, and keys still held stay down.
func (k *Keyboard) EndFrame() {
	k.in.endFrame()
}

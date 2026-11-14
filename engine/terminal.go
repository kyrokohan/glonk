package engine

import (
	"errors"
	"io"
	"os"

	"golang.org/x/term"
)

// keyboardFlags is what we ask of the kitty keyboard protocol: 1 disambiguate
// escape codes, 2 report press/repeat/release, 8 report all keys as escape codes
// (without it letters send no release events).
const keyboardFlags = 1 | 2 | 8

type terminal struct {
	fd       int
	sizeFd   int // where Size asks
	state    *term.State
	out      io.Writer
	keyboard bool // whether we pushed protocol flags that Teardown must pop

	// A headless terminal is no terminal at all: a fixed size, and nothing written
	// to out but the frames.
	headless bool
	w, h     int
}

func newTerminal(in *os.File, out io.Writer) *terminal {
	t := &terminal{
		fd:  int(in.Fd()),
		out: out,
	}
	t.sizeFd = t.fd
	if f, ok := out.(*os.File); ok {
		t.sizeFd = int(f.Fd())
	}
	return t
}

// newHeadless returns a terminal that is a w x h window onto out, with no raw mode,
// no mode sequences and nothing to restore.
func newHeadless(out io.Writer, w, h int) *terminal {
	return &terminal{out: out, headless: true, w: w, h: h}
}

// canSuspend says whether Ctrl-Z may stop the process: only where the system has
// job control and the program owns a terminal that could be handed back.
func (t *terminal) canSuspend() bool {
	return suspendSupported && !t.headless
}

// enterSequence is what Setup writes: the alternate screen first, then what is set
// on it. Autowrap is off so that a row too long for the window, or a character
// wider than expected in the last column, is cut off rather than wrapped onto the
// next line, which would scroll the whole screen.
func (t *terminal) enterSequence() *sequenceBuilder {
	sb := newSequenceBuilder()
	sb.SwitchToAlternateScreen()
	sb.DisableAutowrap()
	sb.EraseInDisplay(2)
	sb.HideCursor()
	return sb
}

func (t *terminal) Setup() error {
	if t.headless {
		return nil
	}
	var err error
	t.state, err = term.MakeRaw(t.fd)
	if err != nil {
		return err
	}
	err = t.enterSequence().Flush(t.out)
	if err != nil {
		// ignoring the restore's error because we care about the flush error more
		term.Restore(t.fd, t.state)
		t.state = nil
		return err
	}
	return nil
}

// EnableKeyboardProtocol turns on press/repeat/release reporting, plus focus
// reporting: a release that happens while the window is unfocused never arrives,
// so Input needs to know when to forget every held key. Call it only after a
// probe says the terminal supports the protocol.
func (t *terminal) EnableKeyboardProtocol() error {
	if t.headless {
		return nil
	}
	// Set first: if the write fails halfway, popping an unpushed flag is harmless
	// (the protocol treats it as a reset) but leaving one pushed would not be.
	t.keyboard = true
	sb := newSequenceBuilder()
	sb.PushKeyboardFlags(keyboardFlags)
	sb.EnableFocusReporting()
	return sb.Flush(t.out)
}

// exitSequence is what Teardown writes: flags are popped first because they were
// pushed on the alternate screen, which has its own stack. Autowrap goes back on
// before the screen is left, where it was turned off.
func (t *terminal) exitSequence() *sequenceBuilder {
	sb := newSequenceBuilder()
	if t.keyboard {
		sb.PopKeyboardFlags()
		sb.DisableFocusReporting()
	}
	sb.ShowCursor()
	sb.EnableAutowrap()
	sb.SwitchToMainScreen()
	return sb
}

// Teardown undoes Setup and anything enabled after it. It does nothing if the
// terminal is not set up, so it is safe to call twice or after a Setup that failed.
func (t *terminal) Teardown() error {
	if t.state == nil {
		return nil
	}
	flushErr := t.exitSequence().Flush(t.out)
	restoreErr := term.Restore(t.fd, t.state)
	t.state, t.keyboard = nil, false
	return errors.Join(flushErr, restoreErr)
}

// Size returns the terminal's width and height in cells.
func (t *terminal) Size() (int, int, error) {
	if t.headless {
		return t.w, t.h, nil
	}
	return term.GetSize(t.sizeFd)
}

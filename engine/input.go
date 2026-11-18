package engine

import (
	"bytes"
	"io"
	"os"
	"sync"
	"time"
)

// Without release events a key counts as held until this long passes
// without another repeat.
const holdTimeout = 150 * time.Millisecond

// Input is the keyboard state handed to Game.Update each frame.
//
// Most terminals report presses and auto-repeats but never releases, and the OS
// repeats only the last key pressed, so holding two keys at once does not work
// there. Terminals that implement the kitty keyboard protocol report releases;
// there Input tracks exactly which keys are down. Letters keep their case in
// the first kind of terminal (shift+w arrives as 'W'), not in the second. [Input.Exact]
// says which kind this is.
//
// IsDown is for keys held to keep something going, Started for keys that switch something
// once, and Pressed for keys whose auto-repeat should act again.
//
// [Input.Paste] is what was pasted, which is no key at all. [Input.KeyEvents] and [Input.Text] are for what
// IsDown, Pressed and Started do not say: which
// modifiers were held with a key, the keys of a frame in the order they came, and the characters
// that were typed, for a text field or a shortcut. IsDown, Pressed and Started never see a chord,
// a character key with Ctrl, Alt or Super held, such as Ctrl+W: it is not the key itself, and a
// terminal may mean it for something else.
//
// To test a game's Update, build an Input with a [Keyboard].
type Input struct {
	now      time.Time
	exact    bool // true when the terminal reports key releases
	lastSeen map[Key]time.Time
	held     map[Key]bool
	pressed  map[Key]bool
	started  map[Key]bool // the pressed keys that were not already down

	events []KeyEvent // this frame's key events, in order
	text   []byte     // this frame's typed text
	paste  string     // this frame's pasted text
}

// newInput returns an Input with no keys held. exact says whether the terminal
// reports key releases.
func newInput(exact bool) *Input {
	return &Input{
		exact:    exact,
		lastSeen: make(map[Key]time.Time),
		held:     make(map[Key]bool),
		pressed:  make(map[Key]bool),
		started:  make(map[Key]bool),
	}
}

// event records a key event at the given time.
func (in *Input) event(e keyEvent, at time.Time) {
	switch e.kind {
	case kindFocusLost, kindReset:
		in.releaseAll()
		return
	case kindFocusGained:
		return // nothing to do: keys held across a focus change send fresh repeats
	case kindPaste:
		in.addPaste(e.text)
		return // text, not keys: it is in no list of keys and in no map
	}

	in.list(e)
	if e.chord {
		return // not the plain key: only the list above sees it, release included
	}
	if e.kind == kindRelease {
		delete(in.held, e.key)
		return
	}
	if in.beginsHold(e, at) {
		in.started[e.key] = true
	}
	in.lastSeen[e.key] = at
	in.held[e.key] = true
	in.pressed[e.key] = true
}

// list adds a key event to this frame's KeyEvents, and the characters it typed to its Text. Both
// stop growing at their limits, so input nobody reads cannot fill the memory.
func (in *Input) list(e keyEvent) {
	if e.key == KeyCtrlC {
		return // Run's, never the game's: a press ends the run, and a release of it comes here
	}
	if e.kind == kindRelease && !in.exact {
		return // no release is ever reported there: a Keyboard's Release does nothing either
	}
	if len(in.events) < maxFrameEvents {
		in.events = append(in.events, KeyEvent{Key: e.key, Mods: e.mods, Action: actionOf(e.kind)})
	}
	text := e.text
	if text == "" {
		text = textOf(e)
	}
	if len(in.text)+len(text) <= maxFrameText {
		in.text = append(in.text, text...)
	}
}

// beginsHold reports whether e puts its key down, as opposed to repeating a key that is
// already down. It must be asked before the event is recorded.
//
// shortcut: without release events a key unseen for holdTimeout counts as new, and the OS
// waits longer than that before it repeats, so a longer hold starts twice. Only a terminal
// that reports releases can be exact.
func (in *Input) beginsHold(e keyEvent, at time.Time) bool {
	if in.exact {
		return e.kind == kindPress && !in.held[e.key]
	}
	t, ok := in.lastSeen[e.key]
	return !ok || at.Sub(t) >= holdTimeout
}

// releaseAll marks every key as released, for when the terminal loses focus
// and the real releases will never arrive.
func (in *Input) releaseAll() {
	clear(in.held)
}

// beginFrame sets the time IsDown measures against.
// Passed in rather than read from the clock so tests can use exact times.
func (in *Input) beginFrame(now time.Time) {
	in.now = now
}

// endFrame forgets this frame's presses, so Pressed and Started are false again until the
// next key event.
func (in *Input) endFrame() {
	clear(in.pressed)
	clear(in.started)
	in.events = in.events[:0]
	in.text = in.text[:0]
	in.paste = ""
}

// KeyEvents returns what happened to the keys since the previous frame, in the order it happened: each
// press, repeat and release, with the modifiers the terminal said were held. A text field, a shortcut
// such as Ctrl+S, or Shift+Tab reads it; IsDown, Pressed and Started are for keys held to play.
//
// Chords are here and nowhere else (see [Input]). Special keys carry their modifiers too, and are also
// in IsDown, Pressed and Started whatever those are. Ctrl-C and Ctrl-Z are Run's and are never here.
// Where the terminal reports no releases there are no ActionRelease and no ActionRepeat, only presses.
//
// What a terminal says of the modifiers depends on it. With the kitty keyboard protocol they are exact.
// Without it, Shift is the case of a letter and not a modifier of it, Alt arrives as Escape and the key
// (so a quick Escape then a key reads as Alt+key), Ctrl and a letter arrive as a control byte (so Ctrl+I
// is Tab, Ctrl+M is Enter and Ctrl+H is Backspace), and the arrows, Home, End, Page Up and Down, Insert,
// Delete and F1 to F12 carry their modifiers in the sequence. Hyper and Meta chords are not read.
//
// At most 256 events are kept for a frame; the rest are dropped. The slice is valid until Update returns:
// copy what is to be kept.
func (in *Input) KeyEvents() []KeyEvent {
	return in.events
}

// Paste returns the text pasted since the previous frame, or the empty string if there was none: one
// string for each paste, joined in order if there were several. A terminal that supports bracketed paste,
// which glonk asks for, marks a paste, so that it arrives whole and is not typing: it is in no key event,
// in no map of keys, and not in Text, so a newline in it does not press Enter, an escape sequence in it
// is not read as keys, and a Ctrl-C in it does not end the run. A terminal without it sends a paste as
// the key presses it is, and Paste is empty.
//
// The text is valid UTF-8, with U+FFFD for what is not, and every line break is a newline, since a
// terminal sends them as carriage returns. At most 1 MiB is kept, in a paste and in a frame: the rest is
// dropped. A paste that stalls for half a second before its end is delivered as it stands.
func (in *Input) Paste() string {
	return in.paste
}

// Text returns the characters typed since the previous frame, in order, as a text field wants them: the
// characters the keys type, not the keys. A special key such as Enter or Backspace, a release and a
// paste type nothing here, and neither does a chord, unless the terminal says it typed something (Option
// and a is a with a ring on a Mac, and the kitty protocol sends the text of it).
//
// A terminal with the kitty keyboard protocol sends the text of each key, so Shift, the layout and
// compose keys are right. Without it, or with it but not its text flag, the text is the printable
// characters as they arrived, which is right on a terminal that sends the character itself, or the key
// with Shift applied to the ASCII letters only. At most 4,096 bytes are kept for a frame.
func (in *Input) Text() string {
	return string(in.text)
}

// IsDown reports whether k is held. Where the terminal reports releases this is
// exact. Elsewhere a new hold reads up briefly before the OS starts
// auto-repeating, and a released key stays down for up to about 150 ms.
func (in *Input) IsDown(k Key) bool {
	if in.exact {
		// pressed keeps a tap shorter than one frame visible.
		return in.held[k] || in.pressed[k]
	}
	t, ok := in.lastSeen[k]
	return ok && in.now.Sub(t) < holdTimeout
}

// Exact reports whether the terminal reports key releases, so that IsDown and Started are
// exact. It is true on a terminal with the kitty keyboard protocol, and in a headless Run
// unless [Config.LegacyKeys] is set. Elsewhere it is false: a held key is a timing guess and
// only one key stays held at a time. A game that needs two at once, to walk and turn, can
// use this to warn the player, or to change to controls that work one key at a time. It is
// settled before the first Update and does not change while Run runs.
func (in *Input) Exact() bool {
	return in.exact
}

// Pressed reports whether k was pressed or repeated since the previous frame. It is true
// again for every auto-repeat of a held key, which is right for moving through a menu and
// wrong for a key that switches something: use [Input.Started] for that.
func (in *Input) Pressed(k Key) bool {
	return in.pressed[k]
}

// Started reports whether k went down since the previous frame. Unlike [Input.Pressed] it is
// false for the auto-repeats of a key that is already down, so a key that switches something
// switches it once however long it is held.
//
// Where the terminal reports releases this is exact. Elsewhere a key not seen for about 150 ms
// counts as new, and the OS waits longer than that before it starts repeating, so a key held
// past that delay starts twice: when it is pressed, and when its first repeat arrives. The
// repeats after that come close enough together not to start again.
func (in *Input) Started(k Key) bool {
	return in.started[k]
}

// escapeTimeout is how long an unfinished escape sequence is waited for. The Escape
// key and the start of an arrow key are both a lone ESC byte, so after seeing one
// the input is only taken as the Escape key once this long has passed with nothing
// more. Terminals send a whole key at once, so any value well above the time a read
// takes works; it is the delay a real Escape press waits.
const escapeTimeout = 50 * time.Millisecond

// readBuffer is how many bytes are read from the terminal at once. Keys arrive a few
// bytes at a time, so a large buffer is only filled by pasting or a backlog.
const readBuffer = 4096

// inputAction is what Run must do once input has been handled.
type inputAction uint8

const (
	actionNone    inputAction = iota
	actionQuit                // Ctrl-C
	actionSuspend             // Ctrl-Z
)

// inputFeed turns the raw bytes read from the terminal into Input updates. It holds
// a keyParser, and a timer that gives up waiting for the rest of an unfinished
// sequence.
type inputFeed struct {
	parser     keyParser
	in         *Input
	wait       time.Duration
	pasteWait  time.Duration // how long a paste may stall, where wait is for an unfinished sequence
	timer      *time.Timer
	canSuspend bool // whether Ctrl-Z may stop the program
	refresh    bool // the window came back to the front since refreshNeeded was last asked
}

func newInputFeed(in *Input) *inputFeed {
	f := &inputFeed{in: in, wait: escapeTimeout, pasteWait: pasteTimeout, canSuspend: suspendSupported}
	f.timer = time.NewTimer(f.wait)
	f.timer.Stop()
	return f
}

// feed handles one chunk read from the terminal and says whether it held Ctrl-C
// or Ctrl-Z. What follows that key in the chunk is dropped.
func (f *inputFeed) feed(chunk []byte, now time.Time) inputAction {
	action := f.apply(f.parser.Feed(chunk), now)

	f.stop()
	if f.parser.Pending() {
		wait := f.wait
		if f.parser.Pasting() {
			wait = f.pasteWait
		}
		f.timer.Reset(wait)
	}
	return action
}

// expired is the channel that fires when the wait for the rest of a sequence is over.
func (f *inputFeed) expired() <-chan time.Time {
	return f.timer.C
}

// expire gives up waiting: whatever is held is taken as it stands, so a lone ESC
// becomes the Escape key. It says whether that produced Ctrl-C or Ctrl-Z.
func (f *inputFeed) expire(now time.Time) inputAction {
	return f.apply(f.parser.Flush(), now)
}

// reset forgets every held key and any half-read sequence, for when input picks up
// after a gap in which releases and the rest of a sequence were lost.
func (f *inputFeed) reset() {
	f.stop()
	f.parser = keyParser{}
	f.in.releaseAll()
}

// stop cancels the wait, draining the channel so a tick that was already on its way
// cannot be mistaken for a later wait ending (needed before Go 1.23 timer semantics).
func (f *inputFeed) stop() {
	if !f.timer.Stop() {
		select {
		case <-f.timer.C:
		default:
		}
	}
}

// refreshNeeded reports whether the terminal said its window came back to the front
// since the last time it was asked. What the window showed may have been disturbed
// meanwhile, so the screen is worth drawing again in full.
func (f *inputFeed) refreshNeeded() bool {
	r := f.refresh
	f.refresh = false
	return r
}

func (f *inputFeed) apply(events []keyEvent, now time.Time) inputAction {
	for _, e := range events {
		if e.kind == kindFocusGained {
			f.refresh = true
		}
		switch {
		case e.key == KeyCtrlC && e.kind != kindRelease:
			return actionQuit
		case e.key == keyCtrlZ:
			// Only the press suspends: a repeat would suspend again the moment the
			// program resumes with the key still down.
			if e.kind == kindPress && f.canSuspend {
				return actionSuspend
			}
		default:
			f.in.event(e, now)
		}
	}
	return actionNone
}

// readFrom streams what r yields in chunks. A file gets the platform reader, which
// can be stopped cleanly; anything else can only be read with blocking reads.
func readFrom(r io.Reader) (chunks <-chan []byte, stop func()) {
	if f, ok := r.(*os.File); ok {
		return readInput(f)
	}
	return readBlocking(r)
}

// readBlocking streams chunks read from r until it errors, then closes the channel.
// Its goroutine sits in a blocking Read, which nothing can interrupt, so stop only
// lets go of a chunk nobody is receiving: a goroutine waiting in Read stays behind
// and takes the next input. readInput avoids that where it can.
func readBlocking(r io.Reader) (chunks <-chan []byte, stop func()) {
	ch := make(chan []byte)
	done := make(chan struct{})
	go func() {
		defer close(ch)
		buf := make([]byte, readBuffer)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				select {
				case ch <- bytes.Clone(buf[:n]):
				case <-done:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	var once sync.Once
	return ch, func() { once.Do(func() { close(done) }) }
}

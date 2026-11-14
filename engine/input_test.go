package engine

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func TestReadBlocking(t *testing.T) {
	r := iotest.DataErrReader(iotest.OneByteReader(strings.NewReader("ab\x03")))
	var chunks [][]byte
	input, _ := readBlocking(r)
	for chunk := range input {
		chunks = append(chunks, chunk)
	}
	got := bytes.Join(chunks, nil)

	if !bytes.Equal([]byte("ab\x03"), got) {
		t.Errorf("wanted %q, got %q", "ab\x03", got)
	}
}

func TestInputIsDown(t *testing.T) {
	t0 := time.Unix(0, 0)
	ms := time.Millisecond

	tests := []struct {
		name    string
		presses []time.Duration
		frameAt time.Duration
		want    bool
	}{
		{"never pressed", nil, 0, false},
		{"just pressed", []time.Duration{0}, 0, true},
		{"inside window", []time.Duration{0}, 149 * ms, true},
		{"at timeout", []time.Duration{0}, holdTimeout, false},
		{"repeat refreshes", []time.Duration{0, 100 * ms}, 200 * ms, true},
		{"repeat expires", []time.Duration{0, 100 * ms}, 250 * ms, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := newInput(false)
			for _, at := range tt.presses {
				in.event(keyEvent{'w', kindPress}, t0.Add(at))
			}
			in.beginFrame(t0.Add(tt.frameAt))

			if got := in.IsDown('w'); got != tt.want {
				t.Errorf("wanted IsDown('w')=%v, got %v", tt.want, got)
			}
		})
	}
}

func TestInputIsDownPerKey(t *testing.T) {
	t0 := time.Unix(0, 0)
	in := newInput(false)
	in.event(keyEvent{'a', kindPress}, t0)
	in.beginFrame(t0)

	if !in.IsDown('a') {
		t.Errorf("wanted IsDown('a')=true, got false")
	}
	if in.IsDown('b') {
		t.Errorf("wanted IsDown('b')=false, got true")
	}
}

func TestInputIsDownBeforeFirstFrame(t *testing.T) {
	// now and an unseen key are both the zero time here, so without the
	// lookup check every key would read as held.
	in := newInput(false)

	if in.IsDown('w') {
		t.Errorf("wanted IsDown('w')=false before any frame, got true")
	}
}

func TestInputPressed(t *testing.T) {
	t0 := time.Unix(0, 0)
	in := newInput(false)
	in.event(keyEvent{'q', kindPress}, t0)
	in.beginFrame(t0)

	if !in.Pressed('q') {
		t.Fatalf("wanted Pressed('q')=true before endFrame, got false")
	}
	if in.Pressed('x') {
		t.Errorf("wanted Pressed('x')=false, got true")
	}

	in.endFrame()

	if in.Pressed('q') {
		t.Errorf("wanted Pressed('q')=false after endFrame, got true")
	}
	// Pressed is per frame, IsDown is time based; clearing one must not touch the other.
	if !in.IsDown('q') {
		t.Errorf("wanted IsDown('q')=true after endFrame, got false")
	}
}

// Where releases are reported only a key that was not down starts a hold: a repeat does not,
// and neither does a press of a key that is down, which is what Keyboard.Press of a held key is.
func TestInputStartedWhereReleasesAreReported(t *testing.T) {
	press, repeat, release := keyEvent{'m', kindPress}, keyEvent{'m', kindRepeat}, keyEvent{'m', kindRelease}
	frames := []struct {
		name   string
		events []keyEvent
		want   bool
	}{
		{"a press starts", []keyEvent{press}, true},
		{"a repeat does not", []keyEvent{repeat}, false},
		{"a press of a key that is down is a repeat", []keyEvent{press}, false},
		{"a quiet frame starts nothing", nil, false},
		{"a press after a release starts", []keyEvent{release, press}, true},
		{"a tap inside one frame starts", []keyEvent{release, press, release}, true},
		{"a repeat of a key that is not down does not", []keyEvent{repeat}, false},
	}

	in := newInput(true)
	at := time.Unix(0, 0)
	for _, f := range frames {
		t.Run(f.name, func(t *testing.T) {
			for _, e := range f.events {
				in.event(e, at)
			}
			in.beginFrame(at)

			if got := in.Started('m'); got != f.want {
				t.Errorf("wanted Started('m')=%v, got %v", f.want, got)
			}
			if in.Started('m') && !in.Pressed('m') {
				t.Errorf("wanted a started key to be pressed too")
			}
			if in.Started('x') {
				t.Errorf("wanted Started('x')=false, got true")
			}
			in.endFrame()
			if in.Started('m') {
				t.Errorf("wanted Started('m')=false after endFrame, got true")
			}
			at = at.Add(time.Second)
		})
	}
}

// Without release events a key unseen for the hold timeout is new. The OS waits longer than
// that before it repeats, so the first repeat of a long hold starts a second time: the docs of
// Started say so, and this keeps the two the same.
func TestInputStartedWithoutReleases(t *testing.T) {
	t0 := time.Unix(0, 0)
	ms := time.Millisecond

	tests := []struct {
		name    string
		presses []time.Duration
		want    bool
	}{
		{"the first press", []time.Duration{0}, true},
		{"a repeat soon after", []time.Duration{0, 30 * ms}, false},
		{"a repeat inside the window", []time.Duration{0, 149 * ms}, false},
		{"a repeat at the timeout", []time.Duration{0, holdTimeout}, true},
		{"the first repeat after the OS delay", []time.Duration{0, 500 * ms}, true},
		{"the repeats after that", []time.Duration{0, 500 * ms, 530 * ms, 560 * ms}, false},
		{"a press after a long gap", []time.Duration{0, 10 * time.Second}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := newInput(false)
			for i, at := range tt.presses {
				in.event(keyEvent{'m', kindPress}, t0.Add(at))
				in.beginFrame(t0.Add(at))
				if i < len(tt.presses)-1 {
					in.endFrame()
				}
			}

			if got := in.Started('m'); got != tt.want {
				t.Errorf("wanted Started('m')=%v for the last press, got %v", tt.want, got)
			}
		})
	}
}

func TestInputSaysWhetherItIsExact(t *testing.T) {
	if !newInput(true).Exact() {
		t.Errorf("wanted Exact()=true where releases are reported, got false")
	}
	if newInput(false).Exact() {
		t.Errorf("wanted Exact()=false where releases are not reported, got true")
	}
}

func TestInputExactHoldsTwoKeys(t *testing.T) {
	t0 := time.Unix(0, 0)
	in := newInput(true)

	// W goes down, then E: with exact state W stays held although only E repeats.
	in.event(keyEvent{'w', kindPress}, t0)
	in.event(keyEvent{'e', kindPress}, t0.Add(100*time.Millisecond))
	in.beginFrame(t0.Add(120 * time.Millisecond))
	in.endFrame()

	// Ten seconds later, with no further events, both are still down.
	in.beginFrame(t0.Add(10 * time.Second))
	if !in.IsDown('w') || !in.IsDown('e') {
		t.Errorf("wanted w and e both down, got w=%v e=%v", in.IsDown('w'), in.IsDown('e'))
	}
	if in.IsDown('x') {
		t.Errorf("wanted IsDown('x')=false, got true")
	}
}

func TestInputExactRelease(t *testing.T) {
	t0 := time.Unix(0, 0)
	in := newInput(true)

	in.event(keyEvent{'w', kindPress}, t0)
	in.event(keyEvent{'e', kindPress}, t0)
	in.beginFrame(t0)
	in.endFrame()

	in.event(keyEvent{'w', kindRelease}, t0.Add(time.Second))
	in.beginFrame(t0.Add(time.Second))

	if in.IsDown('w') {
		t.Errorf("wanted IsDown('w')=false after release, got true")
	}
	if !in.IsDown('e') {
		t.Errorf("wanted IsDown('e')=true while still held, got false")
	}
}

func TestInputExactRepeatKeepsHeld(t *testing.T) {
	t0 := time.Unix(0, 0)
	in := newInput(true)

	in.event(keyEvent{'w', kindPress}, t0)
	in.beginFrame(t0)
	in.endFrame()

	in.event(keyEvent{'w', kindRepeat}, t0.Add(500*time.Millisecond))
	in.beginFrame(t0.Add(500 * time.Millisecond))

	if !in.IsDown('w') {
		t.Errorf("wanted IsDown('w')=true on repeat, got false")
	}
	if !in.Pressed('w') {
		t.Errorf("wanted Pressed('w')=true on repeat, got false")
	}
}

// A tap can start and end inside one frame; Update must still see the key down once.
func TestInputExactSameFrameTap(t *testing.T) {
	t0 := time.Unix(0, 0)
	in := newInput(true)

	in.event(keyEvent{'w', kindPress}, t0)
	in.event(keyEvent{'w', kindRelease}, t0.Add(10*time.Millisecond))
	in.beginFrame(t0.Add(16 * time.Millisecond))

	if !in.IsDown('w') {
		t.Errorf("wanted IsDown('w')=true for a tap inside the frame, got false")
	}

	in.endFrame()
	in.beginFrame(t0.Add(33 * time.Millisecond))

	if in.IsDown('w') {
		t.Errorf("wanted IsDown('w')=false the frame after the tap, got true")
	}
}

func TestInputReleaseAll(t *testing.T) {
	t0 := time.Unix(0, 0)
	in := newInput(true)

	in.event(keyEvent{'w', kindPress}, t0)
	in.event(keyEvent{'e', kindPress}, t0)
	in.beginFrame(t0)
	in.endFrame()

	in.releaseAll()
	in.beginFrame(t0.Add(time.Second))

	if in.IsDown('w') || in.IsDown('e') {
		t.Errorf("wanted nothing down after releaseAll, got w=%v e=%v", in.IsDown('w'), in.IsDown('e'))
	}
}

// Without release events the old timing rule must stay in force.
func TestInputLegacyIgnoresHeld(t *testing.T) {
	t0 := time.Unix(0, 0)
	in := newInput(false)

	in.event(keyEvent{'w', kindPress}, t0)
	in.beginFrame(t0.Add(10 * time.Second))

	if in.IsDown('w') {
		t.Errorf("wanted IsDown('w')=false long after a legacy press, got true")
	}
}

func TestInputFocusLostReleasesHeldKeys(t *testing.T) {
	t0 := time.Unix(0, 0)
	in := newInput(true)

	in.event(keyEvent{'w', kindPress}, t0)
	in.event(keyEvent{'e', kindPress}, t0)
	in.beginFrame(t0)
	in.endFrame()

	in.event(keyEvent{KeyUnknown, kindFocusLost}, t0.Add(time.Second))
	in.beginFrame(t0.Add(time.Second))

	if in.IsDown('w') || in.IsDown('e') {
		t.Errorf("wanted nothing down after focus loss, got w=%v e=%v", in.IsDown('w'), in.IsDown('e'))
	}
}

func TestInputFocusGainedKeepsState(t *testing.T) {
	t0 := time.Unix(0, 0)
	in := newInput(true)

	in.event(keyEvent{'w', kindPress}, t0)
	in.beginFrame(t0)
	in.endFrame()

	in.event(keyEvent{KeyUnknown, kindFocusGained}, t0.Add(time.Second))
	in.beginFrame(t0.Add(time.Second))

	if !in.IsDown('w') {
		t.Errorf("wanted w still down after focus gain, got false")
	}
	if in.Pressed(KeyUnknown) || in.IsDown(KeyUnknown) {
		t.Errorf("wanted a focus event to leave no key state, got pressed=%v down=%v", in.Pressed(KeyUnknown), in.IsDown(KeyUnknown))
	}
}

// ---- the input feed ----

func newTestFeed(exact bool) (*inputFeed, *Input) {
	in := newInput(exact)
	f := newInputFeed(in)
	f.wait = 5 * time.Millisecond // short, so the tests can wait for it
	return f, in
}

// The other tests shorten the wait, so this is the only one that sees the real default.
func TestInputFeedWaitsLongEnoughForARealSequence(t *testing.T) {
	f := newInputFeed(newInput(false))
	if f.wait != escapeTimeout {
		t.Errorf("wanted new feed to wait escapeTimeout (%v), got %v", escapeTimeout, f.wait)
	}
	if escapeTimeout < 20*time.Millisecond {
		t.Errorf("wanted a wait that outlasts a slow link's gap between reads, got %v", escapeTimeout)
	}
}

func fired(f *inputFeed, within time.Duration) bool {
	select {
	case <-f.expired():
		return true
	case <-time.After(within):
		return false
	}
}

func TestInputFeedLoneEscapeBecomesEscapeAfterTheWait(t *testing.T) {
	f, in := newTestFeed(false)
	t0 := time.Unix(0, 0)

	if f.feed([]byte("\x1b"), t0) != actionNone {
		t.Fatalf("wanted a lone ESC not to quit")
	}
	if in.Pressed(KeyEscape) {
		t.Fatalf("wanted Escape not pressed before the wait is over")
	}
	if !fired(f, time.Second) {
		t.Fatalf("wanted the wait to end")
	}

	if f.expire(t0.Add(50*time.Millisecond)) != actionNone {
		t.Fatalf("wanted Escape not to quit")
	}
	if !in.Pressed(KeyEscape) {
		t.Errorf("wanted Escape pressed once the wait is over")
	}
}

func TestInputFeedEscapeSequenceInTwoReadsIsNotEscape(t *testing.T) {
	f, in := newTestFeed(false)
	t0 := time.Unix(0, 0)

	f.feed([]byte("\x1b"), t0)
	f.feed([]byte("[A"), t0.Add(2*time.Millisecond))

	if !in.Pressed(KeyUp) {
		t.Errorf("wanted Up pressed")
	}
	if in.Pressed(KeyEscape) {
		t.Errorf("wanted Escape not pressed when the sequence completed")
	}
	if fired(f, 40*time.Millisecond) {
		t.Errorf("wanted the wait cancelled once the sequence completed")
	}
}

func TestInputFeedTimerOnlyRunsWhileSomethingIsHeld(t *testing.T) {
	f, _ := newTestFeed(false)
	t0 := time.Unix(0, 0)

	f.feed([]byte("wasd"), t0)
	if fired(f, 40*time.Millisecond) {
		t.Errorf("wanted no wait for complete input")
	}

	f.feed([]byte("\x1b[1"), t0)
	if !fired(f, time.Second) {
		t.Errorf("wanted a wait for an unfinished sequence")
	}
	f.expire(t0) // the wait ended, so the held bytes are given up on

	// Each read that leaves something held restarts the wait.
	f.feed([]byte("\x1b"), t0)
	f.feed([]byte("["), t0)
	if !fired(f, time.Second) {
		t.Errorf("wanted the wait to end after the last read")
	}
}

// A tick left over from an earlier wait must not end a later one early. Timers built
// for Go 1.23 or later cannot leave one behind; stop drains the channel for programs
// built with an older go.mod, which this toolchain can no longer emulate, so this
// guards the behavior rather than the drain.
func TestInputFeedStaleTickDoesNotEndALaterWait(t *testing.T) {
	f, _ := newTestFeed(false)
	f.wait = time.Millisecond
	t0 := time.Unix(0, 0)

	f.feed([]byte("\x1b"), t0)
	time.Sleep(20 * time.Millisecond) // the first wait ends and its tick sits in the channel

	f.wait = time.Hour
	f.feed([]byte("\x1b"), t0) // a new wait starts
	if fired(f, 30*time.Millisecond) {
		t.Errorf("wanted the old tick gone, but it ended the new wait")
	}
}

func TestInputFeedSplitKitlyReleaseIsNotStuck(t *testing.T) {
	f, in := newTestFeed(true)
	t0 := time.Unix(0, 0)

	f.feed([]byte("\x1b[119u"), t0)
	in.beginFrame(t0)
	if !in.IsDown('w') {
		t.Fatalf("wanted w held after the press")
	}
	in.endFrame()

	f.feed([]byte("\x1b[119;1"), t0.Add(time.Second))
	f.feed([]byte(":3u"), t0.Add(time.Second))
	in.beginFrame(t0.Add(time.Second))
	if in.IsDown('w') {
		t.Errorf("wanted w released by the release that arrived in two reads")
	}
}

// The kitty protocol says a press from a repeat, so a held key starts once however long it is held.
func TestInputFeedHeldKittyKeyStartsOnce(t *testing.T) {
	f, in := newTestFeed(true)
	t0 := time.Unix(0, 0)

	var started []bool
	frame := func(chunk string, at time.Duration) {
		f.feed([]byte(chunk), t0.Add(at))
		in.beginFrame(t0.Add(at))
		started = append(started, in.Started('m'))
		in.endFrame()
	}
	frame("\x1b[109u", 0)
	frame("\x1b[109;1:2u", 500*time.Millisecond)
	frame("\x1b[109;1:2u", 530*time.Millisecond)
	frame("\x1b[109;1:3u", 600*time.Millisecond)
	frame("\x1b[109u", 700*time.Millisecond)

	if want := []bool{true, false, false, false, true}; !slices.Equal(started, want) {
		t.Errorf("wanted Started per frame %v, got %v", want, started)
	}
}

func TestInputFeedDamagedStreamForgetsHeldKeys(t *testing.T) {
	f, in := newTestFeed(true)
	t0 := time.Unix(0, 0)

	f.feed([]byte("\x1b[119u\x1b[101u"), t0)
	in.beginFrame(t0)
	in.endFrame()

	// A release is cut off and its tail never comes.
	f.feed([]byte("\x1b[119;1"), t0)
	f.expire(t0.Add(50 * time.Millisecond))

	in.beginFrame(t0.Add(50 * time.Millisecond))
	if in.IsDown('w') || in.IsDown('e') {
		t.Errorf("wanted every key forgotten after a damaged stream, got w=%v e=%v", in.IsDown('w'), in.IsDown('e'))
	}
}

func TestInputFeedCtrlC(t *testing.T) {
	tests := []struct {
		name  string
		chunk string
		want  inputAction
	}{
		{"legacy ctrl-c", "\x03", actionQuit},
		{"legacy ctrl-c among keys", "ab\x03cd", actionQuit},
		{"kitty ctrl-c press", "\x1b[99;5u", actionQuit},
		{"kitty ctrl-c repeat", "\x1b[99;5:2u", actionQuit},
		{"kitty ctrl-c release", "\x1b[99;5:3u", actionNone},
		{"plain c", "c", actionNone},
		{"ctrl-w", "\x1b[119;5u", actionNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := newTestFeed(true)
			if got := f.feed([]byte(tt.chunk), time.Unix(0, 0)); got != tt.want {
				t.Errorf("wanted action %v, got %v", tt.want, got)
			}
		})
	}

	// Ctrl-C split across reads still quits.
	f, _ := newTestFeed(true)
	if f.feed([]byte("\x1b[99;"), time.Unix(0, 0)) != actionNone {
		t.Fatalf("wanted no quit on half a sequence")
	}
	if f.feed([]byte("5u"), time.Unix(0, 0)) != actionQuit {
		t.Errorf("wanted quit once the sequence completed")
	}
}

func TestInputFeedEventsKeepTheirReadTime(t *testing.T) {
	f, in := newTestFeed(false)
	t0 := time.Unix(100, 0)

	f.feed([]byte("w"), t0)
	in.beginFrame(t0.Add(100 * time.Millisecond))
	if !in.IsDown('w') {
		t.Errorf("wanted w down 100 ms after it arrived")
	}
	in.beginFrame(t0.Add(holdTimeout))
	if in.IsDown('w') {
		t.Errorf("wanted w no longer down once the hold timeout passed")
	}
}

func TestReadBlockingReadsBigChunksAtOnce(t *testing.T) {
	data := strings.Repeat("w", 1000)

	var chunks []int
	input, _ := readBlocking(strings.NewReader(data))
	for chunk := range input {
		chunks = append(chunks, len(chunk))
	}
	if len(chunks) != 1 || chunks[0] != 1000 {
		t.Errorf("wanted one chunk of 1000 bytes, got %v", chunks)
	}

	big := strings.Repeat("w", 3*readBuffer+17)
	total := 0
	input, _ = readBlocking(strings.NewReader(big))
	for chunk := range input {
		if len(chunk) > readBuffer {
			t.Errorf("wanted chunks of at most %d bytes, got %d", readBuffer, len(chunk))
		}
		total += len(chunk)
	}
	if total != len(big) {
		t.Errorf("wanted %d bytes in all, got %d", len(big), total)
	}
}

func TestReadBlockingStopFreesAChunkNobodyReceives(t *testing.T) {
	r, w := io.Pipe()
	input, stop := readBlocking(r)

	go w.Write([]byte("x")) // read, then stuck handing it over
	time.Sleep(50 * time.Millisecond)
	stop()
	stop() // safe to repeat

	select {
	case _, ok := <-input:
		if ok {
			t.Errorf("wanted the dropped chunk not to be delivered after stop")
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("wanted the channel closed once stop released the pending chunk")
	}
	w.Close()
}

func TestInputFeedCtrlZ(t *testing.T) {
	tests := []struct {
		name  string
		chunk string
		want  inputAction
	}{
		{"legacy ctrl-z", "\x1a", actionSuspend},
		{"kitty ctrl-z press", "\x1b[122;5u", actionSuspend},
		{"kitty ctrl-z with caps lock", "\x1b[122;69u", actionSuspend},
		{"kitty ctrl-z repeat", "\x1b[122;5:2u", actionNone},
		{"kitty ctrl-z release", "\x1b[122;5:3u", actionNone},
		{"plain z", "z", actionNone},
		{"alt-z", "\x1bz", actionNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, in := newTestFeed(true)
			f.canSuspend = true
			if got := f.feed([]byte(tt.chunk), time.Unix(0, 0)); got != tt.want {
				t.Errorf("wanted action %v, got %v", tt.want, got)
			}
			if in.Pressed(keyCtrlZ) || in.IsDown(keyCtrlZ) {
				t.Errorf("wanted Ctrl-Z to stay out of the game's input")
			}
		})
	}

	// Ctrl-Z split across reads still suspends.
	f, _ := newTestFeed(true)
	f.canSuspend = true
	if f.feed([]byte("\x1b[122;"), time.Unix(0, 0)) != actionNone {
		t.Fatalf("wanted nothing on half a sequence")
	}
	if f.feed([]byte("5u"), time.Unix(0, 0)) != actionSuspend {
		t.Errorf("wanted a suspend once the sequence completed")
	}
}

func TestInputFeedCtrlZIsIgnoredWhereSuspendingIsNotSupported(t *testing.T) {
	f, in := newTestFeed(false)
	f.canSuspend = false

	if got := f.feed([]byte("a\x1ab"), time.Unix(0, 0)); got != actionNone {
		t.Errorf("wanted no action, got %v", got)
	}
	if !in.Pressed('a') || !in.Pressed('b') {
		t.Errorf("wanted the keys around Ctrl-Z to arrive, got a=%v b=%v", in.Pressed('a'), in.Pressed('b'))
	}
	if in.Pressed(keyCtrlZ) {
		t.Errorf("wanted Ctrl-Z kept from the game")
	}
}

func TestInputFeedDropsWhatFollowsCtrlZ(t *testing.T) {
	f, in := newTestFeed(false)
	f.canSuspend = true

	if got := f.feed([]byte("a\x1ab"), time.Unix(0, 0)); got != actionSuspend {
		t.Fatalf("wanted a suspend, got %v", got)
	}
	if !in.Pressed('a') {
		t.Errorf("wanted the key before Ctrl-Z to arrive")
	}
	if in.Pressed('b') {
		t.Errorf("wanted the key after Ctrl-Z dropped, it is stale once the program resumes")
	}
}

func TestInputFeedResetForgetsHeldKeysAndHalfSequences(t *testing.T) {
	f, in := newTestFeed(true)
	t0 := time.Unix(0, 0)

	f.feed([]byte("\x1b[119u"), t0) // w down, its release will never come
	in.endFrame()                   // so the press itself no longer counts
	f.feed([]byte("\x1b[1"), t0)    // half a sequence, with the timer running
	in.beginFrame(t0)
	if !in.IsDown('w') {
		t.Fatalf("wanted w held before the reset")
	}
	if !f.parser.Pending() {
		t.Fatalf("wanted half a sequence held before the reset")
	}

	f.reset()

	if in.IsDown('w') {
		t.Errorf("wanted w no longer held after a reset")
	}
	if f.parser.Pending() {
		t.Errorf("wanted nothing held back after a reset")
	}
	if fired(f, 4*f.wait) {
		t.Errorf("wanted the wait cancelled by a reset")
	}

	// What arrives next stands on its own instead of finishing the old sequence.
	in.endFrame()
	f.feed([]byte("A"), t0)
	if !in.Pressed('A') || in.Pressed(KeyUp) {
		t.Errorf("wanted a plain A after the reset, got A=%v up=%v", in.Pressed('A'), in.Pressed(KeyUp))
	}
}

// Keys typed before the keyboard protocol was on arrive with no release; Run feeds
// them in and releases at once, so they are taps and nothing stays held.
func TestTypedAheadKeysAreTapsNotHolds(t *testing.T) {
	f, in := newTestFeed(true)
	t0 := time.Unix(0, 0)

	if got := f.feed([]byte("wa"), t0); got != actionNone {
		t.Fatalf("wanted no action, got %v", got)
	}
	in.releaseAll()

	in.beginFrame(t0)
	if !in.Pressed('w') || !in.Pressed('a') {
		t.Errorf("wanted both keys seen as pressed in the first frame")
	}
	in.endFrame()

	in.beginFrame(t0.Add(time.Second))
	if in.IsDown('w') || in.IsDown('a') {
		t.Errorf("wanted the keys released after that frame, got w=%v a=%v", in.IsDown('w'), in.IsDown('a'))
	}
}

func TestTypedAheadCtrlCStillQuits(t *testing.T) {
	f, _ := newTestFeed(true)
	if got := f.feed([]byte("a\x03"), time.Unix(0, 0)); got != actionQuit {
		t.Errorf("wanted a quit, got %v", got)
	}
}

func TestInputFeedSaysWhenTheWindowCameBackToTheFront(t *testing.T) {
	f, _ := newTestFeed(true)

	if f.refreshNeeded() {
		t.Fatalf("wanted no refresh before anything happened")
	}
	f.feed([]byte("\x1b[119u"), time.Unix(0, 0)) // a key press is not the window coming back
	if f.refreshNeeded() {
		t.Errorf("wanted no refresh for a key press")
	}

	f.feed([]byte("\x1b[I"), time.Unix(0, 0))
	if !f.refreshNeeded() {
		t.Fatalf("wanted a refresh once the terminal reports focus gained")
	}
	if f.refreshNeeded() {
		t.Errorf("wanted the refresh reported once, not again")
	}

	f.feed([]byte("\x1b[O"), time.Unix(0, 0)) // focus lost: nothing to redraw
	if f.refreshNeeded() {
		t.Errorf("wanted no refresh when focus is lost")
	}
}

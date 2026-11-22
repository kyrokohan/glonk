package engine

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"
	"unsafe"
)

// sgr is a mouse report as a terminal with mode 1006 sends it: the button number, then the column and the row,
// which it counts from 1, and M for a press, a movement or a wheel step and m for a release.
func sgr(cb, col, row int, final byte) string {
	return fmt.Sprintf("\x1b[<%d;%d;%d%c", cb, col, row, final)
}

// mouseEv is the event of a mouse report in a cell, which is counted from 0.
func mouseEv(kind keyKind, btn uint8, x, y int) keyEvent {
	return keyEvent{key: cellKey(x, y), kind: kind, btn: btn}
}

// mouseKeyCases are mouse reports, as the parser reads them, and what must not be read as one.
var mouseKeyCases = []keyCase{
	{"left press", sgr(0, 1, 1, 'M'), []keyEvent{mouseEv(kindMouseDown, 0, 0, 0)}},
	{"middle press", sgr(1, 5, 7, 'M'), []keyEvent{mouseEv(kindMouseDown, 1, 4, 6)}},
	{"right press", sgr(2, 120, 40, 'M'), []keyEvent{mouseEv(kindMouseDown, 2, 119, 39)}},
	{"left release", sgr(0, 3, 4, 'm'), []keyEvent{mouseEv(kindMouseUp, 0, 2, 3)}},
	{"middle release", sgr(1, 3, 4, 'm'), []keyEvent{mouseEv(kindMouseUp, 1, 2, 3)}},
	{"right release", sgr(2, 3, 4, 'm'), []keyEvent{mouseEv(kindMouseUp, 2, 2, 3)}},
	{"movement with no button", sgr(35, 10, 2, 'M'), []keyEvent{mouseEv(kindMouseMove, 0, 9, 1)}},
	{"drag with the left button", sgr(32, 10, 2, 'M'), []keyEvent{mouseEv(kindMouseMove, 0, 9, 1)}},
	{"drag with the middle button", sgr(33, 10, 2, 'M'), []keyEvent{mouseEv(kindMouseMove, 0, 9, 1)}},
	{"drag with the right button", sgr(34, 10, 2, 'M'), []keyEvent{mouseEv(kindMouseMove, 0, 9, 1)}},
	{"wheel up", sgr(64, 2, 2, 'M'), []keyEvent{mouseEv(kindWheel, wheelUp, 1, 1)}},
	{"wheel down", sgr(65, 2, 2, 'M'), []keyEvent{mouseEv(kindWheel, wheelDown, 1, 1)}},
	{"wheel left", sgr(66, 2, 2, 'M'), []keyEvent{mouseEv(kindWheel, wheelLeft, 1, 1)}},
	{"wheel right", sgr(67, 2, 2, 'M'), []keyEvent{mouseEv(kindWheel, wheelRight, 1, 1)}},
	{"shift, meta and ctrl are not read: a press", sgr(4+8+16, 1, 1, 'M'), []keyEvent{mouseEv(kindMouseDown, 0, 0, 0)}},
	{"ctrl with a drag", sgr(16+32, 1, 1, 'M'), []keyEvent{mouseEv(kindMouseMove, 0, 0, 0)}},
	{"ctrl with the wheel", sgr(16+65, 1, 1, 'M'), []keyEvent{mouseEv(kindWheel, wheelDown, 0, 0)}},
	{"a button past the seventh is dropped", sgr(128, 1, 1, 'M'), nil},
	{"and so is its release and its drag", sgr(129, 1, 1, 'm') + sgr(128+32, 1, 1, 'M'), nil},
	{"a wheel is never let go", sgr(65, 1, 1, 'm'), nil},
	{"a wheel is not moved", sgr(64+32, 1, 1, 'M'), nil},
	{"no button and no movement is where it is", sgr(3, 6, 6, 'M'), []keyEvent{mouseEv(kindMouseMove, 0, 5, 5)}},
	{"column and row 0 are the first cell", sgr(0, 0, 0, 'M'), []keyEvent{mouseEv(kindMouseDown, 0, 0, 0)}},
	{"cells that no terminal has are brought in", sgr(0, 99999, 99999, 'M'), []keyEvent{mouseEv(kindMouseDown, 0, maxMouseCell, maxMouseCell)}},
	{"numbers that grow without end are brought in", "\x1b[<0;99999999999999999999;99999999999999999999M", []keyEvent{mouseEv(kindMouseDown, 0, maxMouseCell, maxMouseCell)}},
	{"the last cell there can be", sgr(0, maxMouseCell+1, maxMouseCell+1, 'M'), []keyEvent{mouseEv(kindMouseDown, 0, maxMouseCell, maxMouseCell)}},
	{"a button number no terminal sends", sgr(256, 1, 1, 'M'), nil},
	{"a huge button number", "\x1b[<99999999999999999999;1;1M", nil},
	{"no numbers", "\x1b[<M", nil},
	{"one number", "\x1b[<0M", nil},
	{"two numbers", "\x1b[<0;1M", nil},
	{"four numbers", "\x1b[<0;1;2;3M", nil},
	{"an empty first number", "\x1b[<;1;2M", nil},
	{"an empty second number", "\x1b[<0;;2M", nil},
	{"an empty third number", "\x1b[<0;1;M", nil},
	{"a separator at the end", "\x1b[<0;1;2;M", nil},
	{"a question mark in a number", "\x1b[<0;1?;2M", nil},
	{"a colon in a number", "\x1b[<0:1;1;2M", nil},
	{"a minus in a number", "\x1b[<0;-1;2M", nil},
	{"another final byte", "\x1b[<0;1;2H", nil},
	{"another private marker", "\x1b[?0;1;2M", nil},
	{"no marker is no report", "\x1b[0;1;2M", nil},
	{"keys either side of a report", "a" + sgr(0, 2, 2, 'M') + "b", []keyEvent{{key: 'a', kind: kindPress}, mouseEv(kindMouseDown, 0, 1, 1), {key: 'b', kind: kindPress}}},
	{"reports one after another", sgr(35, 1, 1, 'M') + sgr(35, 2, 1, 'M') + sgr(0, 2, 1, 'M'), []keyEvent{mouseEv(kindMouseMove, 0, 0, 0), mouseEv(kindMouseMove, 0, 1, 0), mouseEv(kindMouseDown, 0, 1, 0)}},
	{"a report then a paste then a key", sgr(35, 1, 1, 'M') + "\x1b[200~x\x1b[201~k", []keyEvent{mouseEv(kindMouseMove, 0, 0, 0), pasted("x"), {key: 'k', kind: kindPress}}},
	{"a report then a kitty key", sgr(0, 1, 1, 'M') + "\x1b[97u", []keyEvent{mouseEv(kindMouseDown, 0, 0, 0), {key: 'a', kind: kindPress}}},
	{"a truncated report is reported as a reset", "\x1b[<0;1;", []keyEvent{{key: KeyUnknown, kind: kindReset}}},

	// The old form, which a terminal sends where it has the mouse and not the decimal form: nothing is read from it,
	// and its three bytes are not typed.
	{"the old form is dropped with its bytes", "a\x1b[M #$b", presses('a', 'b')},
	{"the old form with bytes that are no characters", "\x1b[M\xe0\xff\x80k", presses('k')},
	{"two of the old form", "\x1b[M #$\x1b[M!%&", nil},
	{"the old form cut short is reported as a reset", "\x1b[M #", []keyEvent{{key: KeyUnknown, kind: kindReset}}},
	{"the old form with nothing after it is reported as a reset", "\x1b[M", []keyEvent{{key: KeyUnknown, kind: kindReset}}},
	{"parameters before an M make it another sequence, with no bytes taken", "\x1b[1;2Mabc", presses('a', 'b', 'c')},
}

func TestParseEventsMouse(t *testing.T) {
	for _, tt := range mouseKeyCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseEvents([]byte(tt.in)); !slices.Equal(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

// A mouse event is the size of a key event was without it, so the key parser costs what it did: btn fits where the
// struct had padding. This is that struct, without btn, on any word size. A key event that grows costs the key
// parser time (#93), so if this fails, measure it with make benchcmp before changing it here.
type keyEventBeforeTheMouse struct {
	_ Key
	_ keyKind
	_ Mods
	_ bool
	_ string
}

func TestMouseEventsDoNotGrowKeyEvents(t *testing.T) {
	if got, want := unsafe.Sizeof(keyEvent{}), unsafe.Sizeof(keyEventBeforeTheMouse{}); got != want {
		t.Errorf("wanted a keyEvent of %d bytes, as before the mouse, got %d", want, got)
	}
}

func TestCellKeyRoundTrips(t *testing.T) {
	for _, c := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {119, 39}, {maxMouseCell, maxMouseCell}, {maxMouseCell, 0}, {0, maxMouseCell}} {
		x, y := mouseEv(kindMouseMove, 0, c[0], c[1]).cell()
		if x != c[0] || y != c[1] {
			t.Errorf("cell %v: got %d, %d", c, x, y)
		}
		if k := cellKey(c[0], c[1]); k < 0 {
			t.Errorf("cell %v: wanted a key that is not negative, got %d", c, k)
		}
	}
	// Out of range is brought in, not wrapped.
	for _, c := range []struct{ in, want [2]int }{
		{[2]int{-1, -5}, [2]int{0, 0}},
		{[2]int{maxMouseCell + 1, maxMouseCell + 9}, [2]int{maxMouseCell, maxMouseCell}},
		{[2]int{math.MaxInt, math.MinInt}, [2]int{maxMouseCell, 0}},
	} {
		x, y := mouseEv(kindMouseMove, 0, c.in[0], c.in[1]).cell()
		if x != c.want[0] || y != c.want[1] {
			t.Errorf("cell %v: wanted %v, got %d, %d", c.in, c.want, x, y)
		}
	}
}

func TestInputMousePosIsUnknownUntilReported(t *testing.T) {
	in := newInput(true)
	if x, y, ok := in.MousePos(); ok || x != 0 || y != 0 {
		t.Errorf("wanted no position before a report, got %d, %d, %v", x, y, ok)
	}
	// Every kind of report says where the pointer is.
	for _, tt := range []struct {
		name string
		in   string
	}{
		{"movement", sgr(35, 11, 6, 'M')},
		{"press", sgr(0, 11, 6, 'M')},
		{"release", sgr(0, 11, 6, 'm')},
		{"wheel", sgr(64, 11, 6, 'M')},
		{"drag", sgr(32, 11, 6, 'M')},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := fed(t, true, tt.in)
			if x, y, ok := in.MousePos(); !ok || x != 10 || y != 5 {
				t.Errorf("wanted 10, 5 and known, got %d, %d, %v", x, y, ok)
			}
		})
	}
	// A report that is dropped does not say.
	if _, _, ok := fed(t, true, sgr(128, 3, 3, 'M')).MousePos(); ok {
		t.Errorf("wanted a button that is not reported not to say where the pointer is")
	}
}

func TestInputMousePosStaysAcrossFramesAndFollowsTheLastReport(t *testing.T) {
	in := fed(t, true, sgr(35, 2, 2, 'M'), sgr(35, 30, 9, 'M'))
	in.endFrame()
	if x, y, ok := in.MousePos(); !ok || x != 29 || y != 8 {
		t.Errorf("wanted the last report to stay after the frame, got %d, %d, %v", x, y, ok)
	}
}

func TestInputMouseButtonsGoDownAndUp(t *testing.T) {
	in := newInput(true)
	feed := newInputFeed(in)
	now := time.Unix(0, 0)
	send := func(s string) { feed.feed([]byte(s), now) }
	state := func() [9]bool {
		var s [9]bool
		for b := range MouseButton(3) {
			s[b*3], s[b*3+1], s[b*3+2] = in.MouseDown(b), in.MouseStarted(b), in.MouseReleased(b)
		}
		return s
	}

	if state() != [9]bool{} {
		t.Fatalf("wanted nothing down to begin with, got %v", state())
	}
	send(sgr(0, 1, 1, 'M'))
	if want := [9]bool{true, true, false}; state() != want {
		t.Errorf("after the left press, wanted %v, got %v", want, state())
	}
	in.endFrame()
	if want := [9]bool{true, false, false}; state() != want {
		t.Errorf("a frame later, wanted it still down and not started, got %v", state())
	}
	send(sgr(2, 1, 1, 'M')) // another button, on its own
	if want := [9]bool{true, false, false, false, false, false, true, true, false}; state() != want {
		t.Errorf("with the right button down too, wanted %v, got %v", want, state())
	}
	in.endFrame()
	send(sgr(0, 1, 1, 'm'))
	if want := [9]bool{false, false, true, false, false, false, true, false, false}; state() != want {
		t.Errorf("after the left release, wanted %v, got %v", want, state())
	}
	in.endFrame()
	if want := [9]bool{false, false, false, false, false, false, true, false, false}; state() != want {
		t.Errorf("a frame later, wanted only the right button down, got %v", want)
	}
	send(sgr(2, 1, 1, 'm'))
	in.endFrame()
	if state() != [9]bool{} {
		t.Errorf("wanted everything let go, got %v", state())
	}
}

// A click shorter than a frame is not lost: it was started and released in the frame, and is not down at the end.
func TestInputMouseClickInOneFrameIsStartedAndReleased(t *testing.T) {
	in := fed(t, true, sgr(0, 4, 4, 'M'), sgr(0, 4, 4, 'm'))
	if in.MouseDown(MouseLeft) || !in.MouseStarted(MouseLeft) || !in.MouseReleased(MouseLeft) {
		t.Errorf("wanted a click to be started and released and not down, got down=%v started=%v released=%v",
			in.MouseDown(MouseLeft), in.MouseStarted(MouseLeft), in.MouseReleased(MouseLeft))
	}
}

// A press of a button that is down is no start, as a repeat of a key is none; a release of a button that is up
// is no release.
func TestInputMouseRepeatedPressAndStrayRelease(t *testing.T) {
	in := fed(t, true, sgr(0, 1, 1, 'M'))
	in.endFrame()
	in.event(mouseEv(kindMouseDown, 0, 1, 1), time.Unix(0, 0))
	if !in.MouseDown(MouseLeft) || in.MouseStarted(MouseLeft) {
		t.Errorf("wanted a second press to leave it down and not started again")
	}

	in = fed(t, true, sgr(0, 1, 1, 'm'))
	if in.MouseDown(MouseLeft) || in.MouseReleased(MouseLeft) {
		t.Errorf("wanted a release of a button that was not down to be nothing")
	}
}

func TestInputMouseButtonsOutOfRangeAreNeverDown(t *testing.T) {
	in := newInput(true)
	for _, b := range []MouseButton{3, 4, 127, 255} {
		in.event(mouseEv(kindMouseDown, uint8(b), 5, 5), time.Unix(0, 0))
		if in.MouseDown(b) || in.MouseStarted(b) || in.MouseReleased(b) {
			t.Errorf("button %d: wanted nothing", b)
		}
		// And no real button moves for it: a button number that wrapped would press one.
		for _, real := range []MouseButton{MouseLeft, MouseMiddle, MouseRight} {
			if in.MouseDown(real) || in.MouseStarted(real) {
				t.Errorf("button %d: wanted button %d untouched", b, real)
			}
		}
		in.event(mouseEv(kindMouseUp, uint8(b), 5, 5), time.Unix(0, 0))
	}

	// With every button down, asking about one that does not exist is still no, not the one it would wrap to.
	in.event(mouseEv(kindMouseDown, 0, 5, 5), time.Unix(0, 0))
	in.event(mouseEv(kindMouseDown, 1, 5, 5), time.Unix(0, 0))
	in.event(mouseEv(kindMouseDown, 2, 5, 5), time.Unix(0, 0))
	for _, b := range []MouseButton{3, 4, 5, 6, 255} {
		if in.MouseDown(b) || in.MouseStarted(b) || in.MouseReleased(b) {
			t.Errorf("button %d, with the others down: wanted no", b)
		}
	}
	in.event(keyEvent{key: KeyUnknown, kind: kindFocusLost}, time.Unix(0, 0))
	for _, b := range []MouseButton{3, 4, 5, 6, 255} {
		if in.MouseReleased(b) {
			t.Errorf("button %d, after the others were let go: wanted no release", b)
		}
	}
	if x, y, ok := in.MousePos(); !ok || x != 5 || y != 5 {
		t.Errorf("wanted the position of those reports kept all the same, got %d, %d, %v", x, y, ok)
	}
}

func TestInputWheelAddsUpTheFrame(t *testing.T) {
	in := fed(t, true, sgr(65, 1, 1, 'M'), sgr(65, 1, 1, 'M'), sgr(64, 1, 1, 'M'), sgr(67, 1, 1, 'M'), sgr(67, 1, 1, 'M'), sgr(66, 1, 1, 'M'), sgr(65, 1, 1, 'M'))
	if dx, dy := in.Wheel(); dx != 1 || dy != 2 {
		t.Errorf("wanted 1 right and 2 down, got %d, %d", dx, dy)
	}
	in.endFrame()
	if dx, dy := in.Wheel(); dx != 0 || dy != 0 {
		t.Errorf("wanted a new frame to start at 0, 0, got %d, %d", dx, dy)
	}
	for name, tt := range map[string]struct {
		in     string
		dx, dy int
	}{"up": {sgr(64, 1, 1, 'M'), 0, -1}, "down": {sgr(65, 1, 1, 'M'), 0, 1}, "left": {sgr(66, 1, 1, 'M'), -1, 0}, "right": {sgr(67, 1, 1, 'M'), 1, 0}} {
		if dx, dy := fed(t, true, tt.in).Wheel(); dx != tt.dx || dy != tt.dy {
			t.Errorf("wheel %s: wanted %d, %d, got %d, %d", name, tt.dx, tt.dy, dx, dy)
		}
	}
}

func TestInputWheelIsBoundedPerFrame(t *testing.T) {
	in := newInput(true)
	for range maxFrameWheel + 500 {
		in.event(mouseEv(kindWheel, wheelDown, 0, 0), time.Unix(0, 0))
		in.event(mouseEv(kindWheel, wheelLeft, 0, 0), time.Unix(0, 0))
	}
	if dx, dy := in.Wheel(); dx != -maxFrameWheel || dy != maxFrameWheel {
		t.Errorf("wanted %d, %d, got %d, %d", -maxFrameWheel, maxFrameWheel, dx, dy)
	}
	// The simulator can be given any number, and overflows nothing.
	kb := NewKeyboard()
	kb.Wheel(math.MaxInt, math.MinInt)
	kb.Wheel(math.MaxInt, math.MinInt)
	if dx, dy := kb.Input().Wheel(); dx != maxFrameWheel || dy != -maxFrameWheel {
		t.Errorf("wanted %d, %d, got %d, %d", maxFrameWheel, -maxFrameWheel, dx, dy)
	}
}

// The terminal says nothing of a release while the window is not focused, so losing focus, or input thrown
// away, lets the buttons go, and the game sees each as released, so that a drag ends.
func TestInputFocusLostAndResetReleaseTheButtons(t *testing.T) {
	for name, kind := range map[string]keyKind{"focus lost": kindFocusLost, "reset": kindReset} {
		t.Run(name, func(t *testing.T) {
			in := fed(t, true, sgr(0, 1, 1, 'M'), sgr(2, 1, 1, 'M'))
			in.endFrame()
			in.event(keyEvent{key: KeyUnknown, kind: kind}, time.Unix(0, 0))
			for _, b := range []MouseButton{MouseLeft, MouseRight} {
				if in.MouseDown(b) || !in.MouseReleased(b) {
					t.Errorf("button %d: wanted it up and released, got down=%v released=%v", b, in.MouseDown(b), in.MouseReleased(b))
				}
			}
			if in.MouseReleased(MouseMiddle) {
				t.Errorf("wanted the middle button, never down, not to be released")
			}
			if _, _, ok := in.MousePos(); !ok {
				t.Errorf("wanted the pointer to stay where it was")
			}
		})
	}

	in := fed(t, true, sgr(0, 1, 1, 'M'))
	in.event(keyEvent{key: KeyUnknown, kind: kindFocusGained}, time.Unix(0, 0))
	if !in.MouseDown(MouseLeft) {
		t.Errorf("wanted gaining focus to let nothing go")
	}
}

// A mouse report is no key and no text: nothing in a game that reads keys changes.
func TestMouseIsNotAKey(t *testing.T) {
	in := fed(t, true, sgr(0, 3, 3, 'M'), sgr(35, 9, 9, 'M'), sgr(64, 1, 1, 'M'), sgr(0, 3, 3, 'm'))
	if len(in.KeyEvents()) != 0 || in.Text() != "" || in.Paste() != "" {
		t.Errorf("wanted no key event, text or paste, got %v, %q, %q", in.KeyEvents(), in.Text(), in.Paste())
	}
	for _, k := range []Key{0, 'a', 'h', ' ', KeyEnter, KeyUp, KeyEscape, Key(3 << 16)} {
		if in.IsDown(k) || in.Pressed(k) || in.Started(k) {
			t.Errorf("key %v: wanted a mouse report not to press it", k)
		}
	}
}

func TestKeyboardMouseMethods(t *testing.T) {
	kb := NewKeyboard()
	in := kb.Input()
	if _, _, ok := in.MousePos(); ok {
		t.Fatalf("wanted no position of a new keyboard")
	}
	kb.MouseDown(MouseLeft) // before any move: the first cell
	if x, y, ok := in.MousePos(); !ok || x != 0 || y != 0 || !in.MouseDown(MouseLeft) || !in.MouseStarted(MouseLeft) {
		t.Errorf("wanted a press at 0, 0, got %d, %d, %v", x, y, ok)
	}
	kb.MouseMove(7, 3)
	kb.MouseUp(MouseLeft)
	if x, y, _ := in.MousePos(); x != 7 || y != 3 || in.MouseDown(MouseLeft) || !in.MouseReleased(MouseLeft) {
		t.Errorf("wanted the release at 7, 3, got %d, %d", x, y)
	}
	kb.MouseUp(MouseRight) // not down: nothing
	if in.MouseReleased(MouseRight) {
		t.Errorf("wanted the release of a button that is up to do nothing")
	}
	kb.MouseDown(MouseButton(9)) // no such button: nothing, and the pointer is where it was
	if x, y, _ := in.MousePos(); x != 7 || y != 3 || in.MouseDown(MouseButton(9)) {
		t.Errorf("wanted a button that does not exist to do nothing")
	}
	kb.MouseMove(-4, math.MaxInt)
	if x, y, _ := in.MousePos(); x != 0 || y != maxMouseCell {
		t.Errorf("wanted a cell brought in, got %d, %d", x, y)
	}
	kb.EndFrame()
	if in.MouseStarted(MouseLeft) || in.MouseReleased(MouseLeft) {
		t.Errorf("wanted a new frame to have no starts or releases")
	}
}

// The simulator and a real run agree on where the pointer is, which buttons are down, started and released, and
// what the wheel did, step by step.
func TestKeyboardMatchesTheMouseOfARealRun(t *testing.T) {
	type step struct {
		do    func(*Keyboard)
		bytes string // "" is the end of a frame
	}
	move := func(x, y int) step {
		return step{func(kb *Keyboard) { kb.MouseMove(x, y) }, sgr(35, x+1, y+1, 'M')}
	}
	down := func(b MouseButton, x, y int) step {
		return step{func(kb *Keyboard) { kb.MouseDown(b) }, sgr(int(b), x+1, y+1, 'M')}
	}
	up := func(b MouseButton, x, y int) step {
		return step{func(kb *Keyboard) { kb.MouseUp(b) }, sgr(int(b), x+1, y+1, 'm')}
	}
	wheel := func(cb, dx, dy, x, y int) step {
		return step{func(kb *Keyboard) { kb.Wheel(dx, dy) }, sgr(cb, x+1, y+1, 'M')}
	}
	frame := step{}

	scripts := map[string][]step{
		"a click":                          {move(3, 4), down(MouseLeft, 3, 4), frame, up(MouseLeft, 3, 4), frame, frame},
		"a click inside one frame":         {move(1, 1), down(MouseLeft, 1, 1), up(MouseLeft, 1, 1), frame, frame},
		"a drag":                           {move(0, 0), down(MouseLeft, 0, 0), frame, move(5, 5), frame, move(9, 2), frame, up(MouseLeft, 9, 2), frame},
		"three buttons":                    {down(MouseLeft, 0, 0), down(MouseMiddle, 0, 0), down(MouseRight, 0, 0), frame, up(MouseMiddle, 0, 0), frame, up(MouseLeft, 0, 0), up(MouseRight, 0, 0), frame},
		"a press of a button that is down": {down(MouseLeft, 0, 0), frame, down(MouseLeft, 0, 0), frame},
		"a release of a button that is up": {up(MouseRight, 0, 0), frame},
		"the wheel":                        {wheel(65, 0, 1, 0, 0), wheel(65, 0, 1, 0, 0), wheel(64, 0, -1, 0, 0), frame, wheel(66, -1, 0, 0, 0), wheel(67, 1, 0, 0, 0), wheel(67, 1, 0, 0, 0), frame, frame},
		"the wheel and a click":            {move(2, 2), wheel(65, 0, 1, 2, 2), down(MouseLeft, 2, 2), frame},
	}

	summary := func(in *Input) string {
		x, y, ok := in.MousePos()
		dx, dy := in.Wheel()
		s := fmt.Sprintf("pos %d,%d,%v wheel %d,%d", x, y, ok, dx, dy)
		for b := MouseButton(0); b < 3; b++ {
			s += fmt.Sprintf(" b%d:%v/%v/%v", b, in.MouseDown(b), in.MouseStarted(b), in.MouseReleased(b))
		}
		return s
	}
	for name, steps := range scripts {
		t.Run(name, func(t *testing.T) {
			kb := NewKeyboard()
			real := newInput(true)
			feed := newInputFeed(real)
			now := time.Unix(0, 0)
			for i, s := range steps {
				if s.bytes == "" {
					if got, want := summary(kb.Input()), summary(real); got != want {
						t.Fatalf("step %d, at the end of a frame: keyboard %s, real %s", i, got, want)
					}
					kb.EndFrame()
					real.endFrame()
					continue
				}
				s.do(kb)
				feed.feed([]byte(s.bytes), now)
				if got, want := summary(kb.Input()), summary(real); got != want {
					t.Fatalf("step %d (%q): keyboard %s, real %s", i, s.bytes, got, want)
				}
			}
		})
	}
}

// MouseMoved is a move to another cell since the previous frame: the first report counts, and a report in
// the same cell, a frame with none, a turn of the wheel or a click in place, and a trip away and back within
// the frame do not.
func TestInputMouseMovedIsAnotherCellSinceThePreviousFrame(t *testing.T) {
	in := newInput(true)
	feed := newInputFeed(in)
	now := time.Unix(0, 0)
	frame := func(reports ...string) bool {
		t.Helper()
		in.endFrame()
		for _, r := range reports {
			feed.feed([]byte(r), now)
		}
		in.beginFrame(now)
		return in.MouseMoved()
	}
	if in.MouseMoved() {
		t.Fatalf("wanted no move before a report")
	}
	for _, tt := range []struct {
		name    string
		reports []string
		want    bool
	}{
		{"the first report", []string{sgr(35, 5, 5, 'M')}, true},
		{"a frame with none", nil, false},
		{"the same cell", []string{sgr(35, 5, 5, 'M')}, false},
		{"another cell", []string{sgr(35, 6, 5, 'M')}, true},
		{"another row", []string{sgr(35, 6, 9, 'M')}, true},
		{"the wheel in place", []string{sgr(65, 6, 9, 'M')}, false},
		{"a click in place", []string{sgr(0, 6, 9, 'M'), sgr(0, 6, 9, 'm')}, false},
		{"away and back", []string{sgr(35, 20, 2, 'M'), sgr(35, 6, 9, 'M')}, false},
		{"a drag", []string{sgr(0, 6, 9, 'M'), sgr(32, 7, 9, 'M')}, true},
	} {
		if got := frame(tt.reports...); got != tt.want {
			t.Errorf("%s: wanted MouseMoved %v, got %v", tt.name, tt.want, got)
		}
	}

	// A Keyboard moves it as a terminal does, and its wheel reports the cell it turns in.
	kb := NewKeyboard()
	kb.MouseMove(3, 4)
	if !kb.Input().MouseMoved() {
		t.Errorf("wanted a Keyboard's first move to count")
	}
	kb.EndFrame()
	kb.Wheel(0, 1)
	kb.MouseDown(MouseLeft)
	if kb.Input().MouseMoved() {
		t.Errorf("wanted a Keyboard's wheel and click in place not to be a move")
	}
	kb.EndFrame()
	kb.MouseMove(3, 5)
	if !kb.Input().MouseMoved() {
		t.Errorf("wanted a Keyboard's move to another cell to count")
	}
}

// In pixel mode a report is a pixel of the terminal, counted from 0 at the grid's top left, and less than 0
// over the padding; the answer to how large a cell is reads as one, and neither is ever a key.
func TestParsePixelReports(t *testing.T) {
	parse := func(in string, pixels bool) []keyEvent {
		p := keyParser{pixels: pixels}
		return append(p.Feed([]byte(in)), p.Flush()...)
	}
	for _, tt := range []struct {
		name   string
		in     string
		pixels bool
		want   []keyEvent
	}{
		{"a pixel", sgr(35, 40, 70, 'M'), true, []keyEvent{mouseEv(kindMouseMove, 0, 40, 70)}},
		{"pixel 0 is the first", sgr(0, 0, 0, 'M'), true, []keyEvent{mouseEv(kindMouseDown, 0, 0, 0)}},
		{"over the padding", "\x1b[<35;-4;-3M", true, []keyEvent{mouseEv(kindMouseMove, 0, 0, 0)}},
		{"the same report in cells", sgr(35, 40, 70, 'M'), false, []keyEvent{mouseEv(kindMouseMove, 0, 39, 69)}},
		{"a minus in a cell report", "\x1b[<35;-4;3M", false, nil},
		{"a minus on the button", "\x1b[<-225;4;3M", true, nil}, // its low byte reads as a move, so only the sign stops it
		{"a minus after a digit", "\x1b[<35;4-2;3M", true, nil},
		{"two minuses", "\x1b[<35;--4;3M", true, nil},
		{"the size of a cell", "\x1b[6;34;16t", false, []keyEvent{{key: cellKey(16, 34), kind: kindCellSize}}},
		{"a size of 0", "\x1b[6;0;16t", false, nil},
		{"the text area is no cell size", "\x1b[4;1938;1712t", false, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := parse(tt.in, tt.pixels); !slices.Equal(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

// With pixel reports a cell is the pixel divided by the size of one, and MousePixels says where in it the
// pointer is; without them MousePixels is the middle of the cell.
func TestInputMousePixels(t *testing.T) {
	kb := NewKeyboard()
	if _, _, ok := kb.Input().MousePixels(); ok || kb.Input().MouseFine() {
		t.Fatalf("wanted no position and no pixels before a report")
	}
	kb.MouseMove(3, 2)
	if x, y, ok := kb.Input().MousePixels(); !ok || x != 3.5 || y != 5 || kb.Input().MouseFine() {
		t.Errorf("wanted the middle of cell 3, 2 in cells, got %v, %v, %v", x, y, ok)
	}

	kb = NewKeyboard()
	kb.CellSize(16, 34)
	if !kb.Input().MouseFine() {
		t.Fatalf("wanted pixel reports on after CellSize")
	}
	for _, tt := range []struct {
		px, py         int
		cellX, cellY   int
		pixelX, pixelY float64
	}{
		{0, 0, 0, 0, 0.5 / 16, 1.0 / 34},
		{15, 33, 0, 0, 15.5 / 16, 2 * 33.5 / 34},
		{16, 34, 1, 1, 16.5 / 16, 2 * 34.5 / 34},
		{40, 50, 2, 1, 40.5 / 16, 2 * 50.5 / 34},
	} {
		kb.MouseMovePixels(tt.px, tt.py)
		in := kb.Input()
		if x, y, _ := in.MousePos(); x != tt.cellX || y != tt.cellY {
			t.Errorf("pixel %d, %d: wanted cell %d, %d, got %d, %d", tt.px, tt.py, tt.cellX, tt.cellY, x, y)
		}
		if x, y, ok := in.MousePixels(); !ok || x != tt.pixelX || y != tt.pixelY {
			t.Errorf("pixel %d, %d: wanted canvas %v, %v, got %v, %v", tt.px, tt.py, tt.pixelX, tt.pixelY, x, y)
		}
	}

	// MouseMove goes to the top left pixel of a cell, and a click and the wheel stay where the pointer is.
	kb.MouseMove(5, 3)
	kb.MouseDown(MouseLeft)
	kb.Wheel(0, 1)
	in := kb.Input()
	if x, y, _ := in.MousePos(); x != 5 || y != 3 || !in.MouseStarted(MouseLeft) {
		t.Errorf("wanted a click in cell 5, 3, got %d, %d", x, y)
	}
	if x, y, _ := in.MousePixels(); x != 80.5/16 || y != 2*102.5/34 {
		t.Errorf("wanted the top left pixel of cell 5, 3, got %v, %v", x, y)
	}

	// A new cell size moves the pointer into the cell it is in at that size; a size of 0 is none.
	in.setCellSize(8, 17)
	if x, y, _ := in.MousePos(); x != 10 || y != 6 {
		t.Errorf("wanted cell 10, 6 at half the size, got %d, %d", x, y)
	}
	in.setCellSize(0, 17)
	if x, y, _ := in.MousePos(); x != 10 || y != 6 || in.mouse.cellW != 8 {
		t.Errorf("wanted a size of 0 to leave the last, got %d, %d and a cell %d wide", x, y, in.mouse.cellW)
	}
	kb.CellSize(0, 9)
	if in.mouse.cellH != 17 {
		t.Errorf("wanted CellSize of 0 to do nothing, got a cell %d high", in.mouse.cellH)
	}
}

// MouseMoved stays a move to another cell: a move within a cell is not one.
func TestInputMouseMovedInPixels(t *testing.T) {
	kb := NewKeyboard()
	kb.CellSize(16, 34)
	kb.MouseMovePixels(20, 40)
	kb.EndFrame()
	kb.MouseMovePixels(30, 60)
	if kb.Input().MouseMoved() {
		t.Errorf("wanted a move within cell 1, 1 not to be a move")
	}
	kb.EndFrame()
	kb.MouseMovePixels(33, 60)
	if !kb.Input().MouseMoved() {
		t.Errorf("wanted a move into cell 2, 1 to be one")
	}
}

// The answer to how large a cell is reaches Input through the feed, as a terminal sends it.
func TestInputFeedCellSizeAnswer(t *testing.T) {
	in := newInput(true)
	f := newInputFeed(in)
	f.usePixels(16, 34)
	now := time.Unix(0, 0)
	f.feed([]byte(sgr(35, 64, 68, 'M')), now)
	if x, y, _ := in.MousePos(); x != 4 || y != 2 {
		t.Fatalf("wanted cell 4, 2, got %d, %d", x, y)
	}
	f.feed([]byte("\x1b[6;17;8t"), now)
	if x, y, _ := in.MousePos(); x != 8 || y != 4 || len(in.KeyEvents()) != 0 {
		t.Errorf("wanted the zoomed cell 8, 4 and no key, got %d, %d and %v", x, y, in.KeyEvents())
	}
	f.reset()
	f.feed([]byte(sgr(35, 64, 68, 'M')), now)
	if x, y, _ := in.MousePos(); x != 8 || y != 4 {
		t.Errorf("wanted reports still read in pixels after a reset, got %d, %d", x, y)
	}
}

// Pixel reports turned on after a report in cells keep the pointer in its cell, at the middle of it.
func TestInputPixelsAfterACellReport(t *testing.T) {
	kb := NewKeyboard()
	kb.MouseMove(4, 2)
	kb.CellSize(16, 34)
	in := kb.Input()
	if x, y, _ := in.MousePos(); x != 4 || y != 2 {
		t.Errorf("wanted the pointer still in cell 4, 2, got %d, %d", x, y)
	}
	if x, y, _ := in.MousePixels(); x != 72.5/16 || y != 2*85.5/34 {
		t.Errorf("wanted the middle of cell 4, 2 in pixels, got %v, %v", x, y)
	}
}

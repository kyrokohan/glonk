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

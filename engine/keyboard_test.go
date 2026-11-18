package engine

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestKeyboardStartsWithNothingDown(t *testing.T) {
	in := NewKeyboard().Input()

	for _, k := range []Key{'w', KeyEscape, KeyUp} {
		if in.IsDown(k) || in.Pressed(k) {
			t.Errorf("wanted %v neither down nor pressed", k)
		}
	}
}

func TestKeyboardPressAndEndFrame(t *testing.T) {
	kb := NewKeyboard()
	kb.Press('w')

	if !kb.Input().IsDown('w') || !kb.Input().Pressed('w') {
		t.Fatalf("wanted w down and pressed after Press, got down=%v pressed=%v", kb.Input().IsDown('w'), kb.Input().Pressed('w'))
	}
	if kb.Input().IsDown('e') || kb.Input().Pressed('e') {
		t.Errorf("wanted other keys untouched")
	}

	kb.EndFrame()
	if !kb.Input().IsDown('w') {
		t.Errorf("wanted w still down after EndFrame")
	}
	if kb.Input().Pressed('w') {
		t.Errorf("wanted w no longer pressed after EndFrame")
	}
}

func TestKeyboardRelease(t *testing.T) {
	kb := NewKeyboard()
	kb.Press('w', 'e')
	kb.EndFrame()

	kb.Release('w')

	if kb.Input().IsDown('w') {
		t.Errorf("wanted w up after Release")
	}
	if !kb.Input().IsDown('e') {
		t.Errorf("wanted e still down")
	}
}

func TestKeyboardTapIsSeenByTheFrameItHappenedIn(t *testing.T) {
	kb := NewKeyboard()
	kb.Press('w')
	kb.Release('w')

	if !kb.Input().IsDown('w') || !kb.Input().Pressed('w') {
		t.Errorf("wanted a tap within one frame to be seen")
	}
	kb.EndFrame()
	if kb.Input().IsDown('w') || kb.Input().Pressed('w') {
		t.Errorf("wanted the tap gone by the next frame")
	}
}

func TestKeyboardPressingSeveralKeysAtOnce(t *testing.T) {
	kb := NewKeyboard()
	kb.Press('w', 'd', KeyUp)

	for _, k := range []Key{'w', 'd', KeyUp} {
		if !kb.Input().IsDown(k) || !kb.Input().Pressed(k) {
			t.Errorf("wanted %v down and pressed", k)
		}
	}
	kb.EndFrame()
	kb.Release('w', 'd')
	if kb.Input().IsDown('w') || kb.Input().IsDown('d') || !kb.Input().IsDown(KeyUp) {
		t.Errorf("wanted only the up arrow left down")
	}
}

func TestKeyboardRepeatIsPressedAgain(t *testing.T) {
	kb := NewKeyboard()
	kb.Press('w')
	if !kb.Input().Started('w') {
		t.Errorf("wanted the first press to start the key")
	}
	kb.EndFrame()

	kb.Press('w')

	if !kb.Input().Pressed('w') || !kb.Input().IsDown('w') {
		t.Errorf("wanted a repeat to count as pressed again")
	}
	if kb.Input().Started('w') {
		t.Errorf("wanted a repeat not to start the key again")
	}

	kb.EndFrame()
	kb.Release('w')
	kb.Press('w')

	if !kb.Input().Started('w') {
		t.Errorf("wanted a press after a release to start the key")
	}
}

func TestKeyboardSaysWhetherItReportsReleases(t *testing.T) {
	if !NewKeyboard().Input().Exact() {
		t.Errorf("wanted a Keyboard to act like a terminal that reports releases")
	}
	if NewLegacyKeyboard().Input().Exact() {
		t.Errorf("wanted a legacy Keyboard to act like one that does not")
	}
}

// Without releases a key is held while presses and repeats keep arriving, and lapses
// about 150 ms after the last one.
func TestLegacyKeyboardKeyLapsesWhenNoRepeatArrives(t *testing.T) {
	kb := NewLegacyKeyboard()
	in := kb.Input()

	kb.Press('w')
	if !in.IsDown('w') {
		t.Fatalf("wanted w down after a press")
	}
	kb.Advance(holdTimeout - time.Millisecond)
	if !in.IsDown('w') {
		t.Errorf("wanted w still down just inside the timeout")
	}

	kb.Press('w') // a repeat arrives and holds it for another timeout
	kb.Advance(holdTimeout - time.Millisecond)
	if !in.IsDown('w') {
		t.Errorf("wanted a repeat to hold w for another timeout")
	}
	kb.Advance(time.Millisecond)
	if in.IsDown('w') {
		t.Errorf("wanted w let go once the timeout passed without a repeat")
	}
}

func TestLegacyKeyboardNeverHearsOfARelease(t *testing.T) {
	kb := NewLegacyKeyboard()

	kb.Press('w')
	kb.Release('w')

	if !kb.Input().IsDown('w') {
		t.Errorf("wanted Release to do nothing, as no release is reported")
	}
}

// The first repeat of a long hold comes after the OS delay, when the key no longer counts
// as held, so it starts a second time. The repeats after it do not.
func TestLegacyKeyboardLongHoldStartsTwice(t *testing.T) {
	kb := NewLegacyKeyboard()
	in := kb.Input()
	started := func() bool {
		s := in.Started('w')
		kb.EndFrame()
		return s
	}

	kb.Press('w')
	if !started() {
		t.Errorf("wanted the press to start the key")
	}
	kb.Advance(30 * time.Millisecond)
	kb.Press('w')
	if started() {
		t.Errorf("wanted a repeat inside the timeout not to start the key")
	}
	kb.Advance(500 * time.Millisecond)
	kb.Press('w')
	if !started() {
		t.Errorf("wanted the first repeat after the OS delay to start the key again")
	}
	kb.Advance(30 * time.Millisecond)
	kb.Press('w')
	if started() {
		t.Errorf("wanted the repeats after that not to start it")
	}
}

func TestKeyboardAdvance(t *testing.T) {
	// Exact keys need no clock.
	kb := NewKeyboard()
	kb.Press('w')
	kb.Advance(time.Hour)
	if !kb.Input().IsDown('w') {
		t.Errorf("wanted Advance to change nothing on a Keyboard that reports releases")
	}

	// A time that is not positive does nothing.
	for _, d := range []time.Duration{0, -time.Hour} {
		kb := NewLegacyKeyboard()
		kb.Press('w')
		kb.Advance(holdTimeout - time.Millisecond)
		kb.Advance(d) // the clock stays where it is, so the next millisecond is the timeout
		kb.Advance(time.Millisecond)
		if kb.Input().IsDown('w') {
			t.Errorf("Advance(%v): wanted the clock not to move, and w let go at the timeout", d)
		}
	}
}

// What the legacy Keyboard offers must be what Run's Input shows in a terminal that reports
// no releases. Each script is played twice: on a legacy Keyboard, and as the plain bytes a
// terminal sends into the parser Run uses. ">100ms" lets that long pass, and after every
// frame ('|') both must show the same keys down, pressed and started.
func TestLegacyKeyboardMatchesTheInputOfARealRun(t *testing.T) {
	scripts := []string{
		"+w | >100ms | >100ms |",
		"+w >100ms +w | >149ms | >1ms |",
		"+w | >30ms +w | >500ms +w | >30ms +w |",
		"+w +e | >200ms | +e | >100ms |",
	}
	bytesOf := map[string]string{"w": "w", "e": "e"}

	for _, script := range scripts {
		t.Run(script, func(t *testing.T) {
			kb := NewLegacyKeyboard()
			real := newInput(false)
			feed := newInputFeed(real)
			now := time.Unix(0, 0)

			for step, op := range strings.Fields(script) {
				switch {
				case op == "|":
					real.beginFrame(now)
					for _, k := range []Key{'w', 'e'} {
						if got, want := kb.Input().IsDown(k), real.IsDown(k); got != want {
							t.Fatalf("step %d: IsDown(%v): keyboard %v, real %v", step, k, got, want)
						}
						if got, want := kb.Input().Pressed(k), real.Pressed(k); got != want {
							t.Fatalf("step %d: Pressed(%v): keyboard %v, real %v", step, k, got, want)
						}
						if got, want := kb.Input().Started(k), real.Started(k); got != want {
							t.Fatalf("step %d: Started(%v): keyboard %v, real %v", step, k, got, want)
						}
					}
					kb.EndFrame()
					real.endFrame()
				case op[0] == '>':
					d, err := time.ParseDuration(op[1:])
					if err != nil {
						t.Fatalf("step %d: bad time %q: %v", step, op, err)
					}
					kb.Advance(d)
					now = now.Add(d)
				default: // "+w"
					kb.Press(Key(op[1]))
					feed.feed([]byte(bytesOf[op[1:]]), now)
				}
			}
		})
	}
}

func TestKeyboardReleasingAKeyThatIsNotDown(t *testing.T) {
	kb := NewKeyboard()
	kb.Release('w')

	if kb.Input().IsDown('w') || kb.Input().Pressed('w') {
		t.Errorf("wanted nothing to change")
	}
}

func TestKeyboardInputIsTheSameEveryTime(t *testing.T) {
	kb := NewKeyboard()
	in := kb.Input()

	kb.Press('w')

	if kb.Input() != in || !in.IsDown('w') {
		t.Errorf("wanted the one Input to follow the keyboard")
	}
}

// What the Keyboard offers must be what Run's Input shows in a terminal that reports
// releases. Each script is played twice: on a Keyboard, and as the key bytes of the
// kitty keyboard protocol into the parser Run uses. After every frame ('|') both must
// show the same keys down and pressed.
func TestKeyboardMatchesTheInputOfARealRun(t *testing.T) {
	bytesOf := map[Key]string{'w': "119", 'e': "101", KeyEscape: "27"}
	scripts := []string{
		"+w | | -w | |",
		"+w -w | |",
		"+w +e | -w | +w | -e | -w |",
		"+w +w | | +w | -w |",
		"+Esc | |",
		"-w | +w -w +w | -w |",
		"+w | +e | -w -e |",
		"+w | +w | +w | -w | +w |",
	}
	keyOf := map[string]Key{"w": 'w', "e": 'e', "Esc": KeyEscape}

	for _, script := range scripts {
		t.Run(script, func(t *testing.T) {
			kb := NewKeyboard()
			real := newInput(true)
			feed := newInputFeed(real)
			now := time.Unix(0, 0)

			for step, op := range strings.Fields(script) {
				if op == "|" {
					real.beginFrame(now)
					for _, k := range []Key{'w', 'e', KeyEscape} {
						if got, want := kb.Input().IsDown(k), real.IsDown(k); got != want {
							t.Fatalf("step %d: IsDown(%v): keyboard %v, real %v", step, k, got, want)
						}
						if got, want := kb.Input().Pressed(k), real.Pressed(k); got != want {
							t.Fatalf("step %d: Pressed(%v): keyboard %v, real %v", step, k, got, want)
						}
						if got, want := kb.Input().Started(k), real.Started(k); got != want {
							t.Fatalf("step %d: Started(%v): keyboard %v, real %v", step, k, got, want)
						}
					}
					kb.EndFrame()
					real.endFrame()
					continue
				}

				key := keyOf[op[1:]]
				if op[0] == '+' {
					kb.Press(key)
					feed.feed([]byte("\x1b["+bytesOf[key]+"u"), now)
				} else {
					kb.Release(key)
					feed.feed([]byte("\x1b["+bytesOf[key]+";1:3u"), now)
				}
			}
		})
	}
}

// The function and navigation keys are keys like any other: pressed, started and down.
func TestKeyboardPressesFunctionAndNavigationKeys(t *testing.T) {
	keys := append([]Key{KeyHome, KeyEnd, KeyPageUp, KeyPageDown, KeyInsert, KeyDelete}, functionKeys[:]...)
	for name, kb := range map[string]*Keyboard{"releases reported": NewKeyboard(), "no releases": NewLegacyKeyboard()} {
		kb.Press(keys...)
		in := kb.Input()
		for _, k := range keys {
			if !in.Pressed(k) || !in.Started(k) || !in.IsDown(k) {
				t.Errorf("%s: key %d: wanted it pressed, started and down", name, k)
			}
		}
		kb.EndFrame()
		for _, k := range keys {
			if in.Pressed(k) || in.Started(k) {
				t.Errorf("%s: key %d: wanted a new frame to forget the press", name, k)
			}
		}
	}
}

func TestKeyboardPressWith(t *testing.T) {
	kb := NewKeyboard()
	kb.PressWith(ModCtrl, 's')
	kb.PressWith(ModShift, 'a')
	kb.PressWith(ModCtrl|ModAlt, KeyRight)
	in := kb.Input()

	want := []KeyEvent{
		{Key: 's', Mods: ModCtrl, Action: ActionPress},
		{Key: 'a', Mods: ModShift, Action: ActionPress},
		{Key: KeyRight, Mods: ModCtrl | ModAlt, Action: ActionPress},
	}
	if got := in.KeyEvents(); !slices.Equal(got, want) {
		t.Errorf("wanted %v, got %v", want, got)
	}
	if got := in.Text(); got != "A" {
		t.Errorf("wanted Shift and a to type A, and a chord and an arrow to type nothing, got %q", got)
	}
	if in.IsDown('s') || in.Pressed('s') || in.Started('s') {
		t.Errorf("wanted Ctrl+S, a chord, not to be the key s")
	}
	if !in.IsDown('a') || !in.IsDown(KeyRight) || !in.Started(KeyRight) {
		t.Errorf("wanted Shift and a to be a, and Ctrl+Alt and the arrow still to be the arrow")
	}

	kb.EndFrame()
	if len(in.KeyEvents()) != 0 || in.Text() != "" {
		t.Errorf("wanted a new frame to forget the events and the text")
	}
}

func TestKeyboardPressWithKeepsOnlyTheModifiersThereAre(t *testing.T) {
	kb := NewKeyboard()
	kb.PressWith(Mods(0xff), 'x')
	if got := kb.Input().KeyEvents(); len(got) != 1 || got[0].Mods != modsMask {
		t.Errorf("wanted shift, alt, ctrl and super and nothing else, got %v", got)
	}
}

func TestKeyboardType(t *testing.T) {
	kb := NewKeyboard()
	kb.Type("Hi!\n\tok\x01\x7f\xffé")
	in := kb.Input()

	if got, want := in.Text(), "Hi!oké"; got != want {
		t.Errorf("wanted %q, got %q: Enter and Tab and the control characters type nothing", want, got)
	}
	var keys []Key
	pairs := 0
	for _, e := range in.KeyEvents() {
		if e.Action == ActionPress {
			keys = append(keys, e.Key)
		} else {
			pairs++
		}
	}
	if want := []Key{'H', 'i', '!', KeyEnter, KeyTab, 'o', 'k', 'é'}; !slices.Equal(keys, want) {
		t.Errorf("wanted the keys %v, got %v", want, keys)
	}
	if pairs != len(keys) {
		t.Errorf("wanted every key let go, %d releases for %d presses", pairs, len(keys))
	}
	for _, k := range keys {
		if in.held[k] {
			t.Errorf("wanted key %v not to be held after being typed", k)
		}
	}
}

func TestKeyboardTypeOnALegacyKeyboard(t *testing.T) {
	kb := NewLegacyKeyboard()
	kb.Type("ab")
	in := kb.Input()
	if in.Text() != "ab" {
		t.Errorf("wanted ab typed, got %q", in.Text())
	}
	if got := in.KeyEvents(); len(got) != 2 || got[0].Action != ActionPress || got[1].Action != ActionPress {
		t.Errorf("wanted two presses and no release, got %v", got)
	}
	kb.EndFrame()
	kb.Advance(100 * time.Millisecond)
	if !in.IsDown('a') {
		t.Errorf("wanted a typed key to count as held for about 150 ms, as after any Press")
	}
	kb.Advance(100 * time.Millisecond)
	if in.IsDown('a') {
		t.Errorf("wanted it let go after that")
	}
}

// The simulator and a real run agree on what the frame's events and text are, for the same typing.
func TestKeyboardMatchesTheEventsAndTextOfARealRun(t *testing.T) {
	type step struct {
		name  string
		press func(*Keyboard)
		bytes string
	}
	typed := func(s string) step {
		var b strings.Builder
		for _, r := range s {
			fmt.Fprintf(&b, "\x1b[%d;;%du\x1b[%d;1:3u", r, r, r)
		}
		return step{"type " + s, func(kb *Keyboard) { kb.Type(s) }, b.String()}
	}
	steps := []step{
		typed("hi"),
		typed("é"),
		{"ctrl+w", func(kb *Keyboard) { kb.PressWith(ModCtrl, 'w') }, "\x1b[119;5u"},
		{"alt+x", func(kb *Keyboard) { kb.PressWith(ModAlt, 'x') }, "\x1b[120;3u"},
		{"super+k", func(kb *Keyboard) { kb.PressWith(ModSuper, 'k') }, "\x1b[107;9u"},
		{"shift+a", func(kb *Keyboard) { kb.PressWith(ModShift, 'a') }, "\x1b[97;2;65u"},
		{"ctrl+shift+z", func(kb *Keyboard) { kb.PressWith(ModCtrl|ModShift, 'q') }, "\x1b[113;6u"},
		{"ctrl+right", func(kb *Keyboard) { kb.PressWith(ModCtrl, KeyRight) }, "\x1b[1;5C"},
		{"shift+f5", func(kb *Keyboard) { kb.PressWith(ModShift, KeyF5) }, "\x1b[15;2~"},
		{"alt+page down", func(kb *Keyboard) { kb.PressWith(ModAlt, KeyPageDown) }, "\x1b[6;3~"},
		{"f1", func(kb *Keyboard) { kb.Press(KeyF1) }, "\x1b[1P"},
		{"enter", func(kb *Keyboard) { kb.Press(KeyEnter) }, "\x1b[13u"},
		{"release", func(kb *Keyboard) { kb.Release('z') }, "\x1b[122;1:3u"},
		{"paste", func(kb *Keyboard) { kb.Paste("one\r\ntwo") }, "\x1b[200~one\r\ntwo\x1b[201~"},
		{"empty paste", func(kb *Keyboard) { kb.Paste("") }, "\x1b[200~\x1b[201~"},
		{"paste with bytes that are not text", func(kb *Keyboard) { kb.Paste("a\xffb\rc") }, "\x1b[200~a\xffb\rc\x1b[201~"},
		{"paste of escape sequences", func(kb *Keyboard) { kb.Paste("\x1b[A\x03") }, "\x1b[200~\x1b[A\x03\x1b[201~"},
	}

	// Each step alone in a frame, and all of them in one.
	check := func(t *testing.T, chosen []step) {
		t.Helper()
		kb := NewKeyboard()
		real := newInput(true)
		feed := newInputFeed(real)
		for _, s := range chosen {
			s.press(kb)
			feed.feed([]byte(s.bytes), time.Unix(0, 0))
		}
		if got, want := kb.Input().KeyEvents(), real.KeyEvents(); !slices.Equal(got, want) {
			t.Errorf("events: keyboard %v, real %v", got, want)
		}
		if got, want := kb.Input().Text(), real.Text(); got != want {
			t.Errorf("text: keyboard %q, real %q", got, want)
		}
		if got, want := kb.Input().Paste(), real.Paste(); got != want {
			t.Errorf("paste: keyboard %q, real %q", got, want)
		}
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) { check(t, []step{s}) })
	}
	t.Run("all in one frame", func(t *testing.T) { check(t, steps) })
}

// Pressing a key that is down is a repeat where releases are reported, as the kitty protocol says it, and
// still not a new start.
func TestKeyboardPressOfAHeldKeyIsARepeatInKeyEvents(t *testing.T) {
	kb := NewKeyboard()
	kb.Press('a')
	kb.EndFrame()
	kb.Press('a')
	kb.Press('b')
	got := kb.Input().KeyEvents()
	want := []KeyEvent{{Key: 'a', Action: ActionRepeat}, {Key: 'b', Action: ActionPress}}
	if !slices.Equal(got, want) {
		t.Errorf("wanted %v, got %v", want, got)
	}
	if kb.Input().Started('a') || !kb.Input().Pressed('a') {
		t.Errorf("wanted the repeat pressed and not started")
	}

	legacy := NewLegacyKeyboard()
	legacy.Press('a')
	legacy.Press('a')
	if got := legacy.Input().KeyEvents(); len(got) != 2 || got[1].Action != ActionPress {
		t.Errorf("wanted two presses where repeats cannot be told, got %v", got)
	}
}

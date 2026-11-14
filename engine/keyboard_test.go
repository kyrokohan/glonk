package engine

import (
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

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

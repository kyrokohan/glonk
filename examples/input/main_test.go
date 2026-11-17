package main

import (
	"strings"
	"testing"

	"glonk.dev/engine"
)

func newFrame(t *testing.T, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

// frame draws the monitor after one Update with kb's input and returns the text of the frame.
func frame(t *testing.T, m *monitor, kb *engine.Keyboard) string {
	t.Helper()
	if err := m.Update(1.0/30, kb.Input()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kb.EndFrame()
	fb := newFrame(t, 80, 24)
	m.Draw(fb)
	return fb.String()
}

func TestAKeyThatWentDownIsLoggedOnce(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	kb.Press(engine.KeyF5)
	if got := frame(t, m, kb); !strings.Contains(got, "down    F5") {
		t.Errorf("wanted F5 logged as going down, got\n%s", got)
	}
	if got := frame(t, m, kb); strings.Count(got, "F5") != 2 { // the log line and the held list
		t.Errorf("wanted nothing new logged on the next frame, got\n%s", got)
	}
}

func TestARepeatIsLoggedAsOne(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	kb.Press('a')
	frame(t, m, kb)
	kb.Press('a') // a key that is already down, pressed again
	if got := frame(t, m, kb); !strings.Contains(got, "repeat  'a'") {
		t.Errorf("wanted the repeat of a logged, got\n%s", got)
	}
}

func TestEveryKeyIsNamedAndNoTwoShareAName(t *testing.T) {
	seen := map[string]engine.Key{}
	for _, k := range probeKeys {
		name := keyName(k)
		if name == "" {
			t.Errorf("key %d: wanted a name", k)
		}
		if other, dup := seen[name]; dup {
			t.Errorf("keys %d and %d are both called %q", k, other, name)
		}
		seen[name] = k
	}
	if len(probeKeys) != len(specialKeys)+('~'-' '+1) {
		t.Errorf("wanted the special keys and every printable ASCII character, got %d keys", len(probeKeys))
	}
}

func TestHeldKeysAreListedUntilTheyAreReleased(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	kb.Press(engine.KeyHome, engine.KeyF12)
	if got := frame(t, m, kb); !strings.Contains(got, "held: Home F12") {
		t.Errorf("wanted Home and F12 held, got\n%s", got)
	}
	kb.Release(engine.KeyHome)
	if got := frame(t, m, kb); !strings.Contains(got, "held: F12") || strings.Contains(got, "held: Home") {
		t.Errorf("wanted only F12 held, got\n%s", got)
	}
}

func TestItSaysWhetherTheTerminalReportsReleases(t *testing.T) {
	if got := frame(t, newMonitor(), engine.NewKeyboard()); !strings.Contains(got, "releases: yes") {
		t.Errorf("wanted a terminal with releases, got\n%s", got)
	}
	if got := frame(t, newMonitor(), engine.NewLegacyKeyboard()); !strings.Contains(got, "releases: no") {
		t.Errorf("wanted a terminal without, got\n%s", got)
	}
}

func TestTheLogKeepsTheNewestLines(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	for i := range maxLog + 50 {
		key := engine.Key('!' + i%90)
		kb.Press(key)
		frame(t, m, kb)
		kb.Release(key)
	}
	if len(m.log) != maxLog {
		t.Errorf("wanted %d lines kept, got %d", maxLog, len(m.log))
	}
}

func TestDrawAtEverySize(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	kb.Press(engine.KeyPageDown, 'x')
	m.Update(1.0/30, kb.Input())
	for _, size := range [][2]int{{1, 1}, {2, 2}, {20, 8}, {80, 24}, {200, 60}} {
		m.Draw(newFrame(t, size[0], size[1]))
	}
}

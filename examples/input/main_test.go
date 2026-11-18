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
	if got := frame(t, m, kb); !strings.Contains(got, "press   F5") {
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

func TestAChordIsLoggedWithItsModifiers(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	kb.PressWith(engine.ModCtrl|engine.ModShift, 'w')
	kb.PressWith(engine.ModAlt, engine.KeyRight)
	got := frame(t, m, kb)
	for _, want := range []string{"press   Ctrl+Shift+'w'", "press   Alt+Right"} {
		if !strings.Contains(got, want) {
			t.Errorf("wanted %q logged, got\n%s", want, got)
		}
	}
	if strings.Contains(got, "held: 'w'") {
		t.Errorf("wanted a chord not to be held, got\n%s", got)
	}
}

func TestAReleaseIsLogged(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	kb.Press('a')
	frame(t, m, kb)
	kb.Release('a')
	if got := frame(t, m, kb); !strings.Contains(got, "release 'a'") || strings.Contains(got, "held: 'a'") {
		t.Errorf("wanted the release logged and the key no longer held, got\n%s", got)
	}
}

func TestTypedTextIsShownAndKeepsTheLastCharacters(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	kb.Type("hello ")
	kb.PressWith(engine.ModShift, 'w')
	kb.Type("orld")
	if got := frame(t, m, kb); !strings.Contains(got, "typed: hello World") {
		t.Errorf("wanted the typed text shown, got\n%s", got)
	}
	kb.Type(strings.Repeat("x", 100))
	frame(t, m, kb)
	if len(m.typed) != maxTyped {
		t.Errorf("wanted the last %d characters kept, got %d", maxTyped, len(m.typed))
	}
}

func TestModsAreNamedInOrder(t *testing.T) {
	for mods, want := range map[engine.Mods]string{
		0: "", engine.ModCtrl: "Ctrl+", engine.ModAlt | engine.ModCtrl: "Ctrl+Alt+",
		engine.ModShift | engine.ModAlt | engine.ModCtrl | engine.ModSuper: "Ctrl+Alt+Shift+Super+",
	} {
		if got := modsName(mods); got != want {
			t.Errorf("wanted %q, got %q", want, got)
		}
	}
}

func TestAPasteIsLoggedAndShown(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	kb.Paste("hello\nworld")
	got := frame(t, m, kb)
	for _, want := range []string{"paste   11 bytes, 2 lines", `pasted: "hello\nworld"`} {
		if !strings.Contains(got, want) {
			t.Errorf("wanted %q, got\n%s", want, got)
		}
	}
	if strings.Contains(got, "typed: h") || strings.Contains(got, "held:") && strings.Contains(got, "held: 'h'") {
		t.Errorf("wanted a paste not to be typing, got\n%s", got)
	}
}

// What is pasted is shown, never carried out: an escape sequence is quoted, and a long paste is cut.
func TestAPastedEscapeSequenceIsQuotedAndALongPasteIsCut(t *testing.T) {
	if got := preview("\x1b[2J\x07"); got != `"\x1b[2J\a"` {
		t.Errorf("wanted the escape sequence quoted, got %s", got)
	}
	long := preview(strings.Repeat("a", 500))
	if !strings.HasSuffix(long, `"...`) || len(long) > maxPasted+6 {
		t.Errorf("wanted a long paste cut at %d characters, got %d bytes: %s", maxPasted, len(long), long)
	}
	if got := preview("日本語"); got != `"日本語"` {
		t.Errorf("wanted text kept as it is, got %s", got)
	}
}

func TestTheMouseIsUnknownUntilReported(t *testing.T) {
	m := newMonitor()
	got := frame(t, m, engine.NewKeyboard())
	if !strings.Contains(got, "mouse: no report yet") {
		t.Errorf("wanted it to say that no report has come, got\n%s", got)
	}
}

func TestTheMouseIsShownAndItsButtonsLogged(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	kb.MouseMove(12, 5)
	kb.MouseDown(engine.MouseLeft)
	got := frame(t, m, kb)
	for _, want := range []string{"press   mouse Left at 12,5", "mouse: cell 12,5, down: [Left]"} {
		if !strings.Contains(got, want) {
			t.Errorf("wanted %q, got\n%s", want, got)
		}
	}

	kb.MouseDown(engine.MouseRight)
	kb.MouseMove(13, 5)
	got = frame(t, m, kb)
	if !strings.Contains(got, "down: [Left Right]") || strings.Count(got, "press   mouse Left") != 1 {
		t.Errorf("wanted both buttons down and the left press logged once, got\n%s", got)
	}

	kb.MouseUp(engine.MouseLeft)
	got = frame(t, m, kb)
	if !strings.Contains(got, "release mouse Left at 13,5") || !strings.Contains(got, "down: [Right]") {
		t.Errorf("wanted the release logged and the right button still down, got\n%s", got)
	}
}

func TestTheWheelIsLoggedAndAddedUp(t *testing.T) {
	m := newMonitor()
	kb := engine.NewKeyboard()
	kb.Wheel(0, 2)
	frame(t, m, kb)
	kb.Wheel(-1, -3)
	got := frame(t, m, kb)
	for _, want := range []string{"wheel   right -1, down -3", "wheel total: right -1, down -1"} {
		if !strings.Contains(got, want) {
			t.Errorf("wanted %q, got\n%s", want, got)
		}
	}
}

package engine

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// fed returns an Input that has been given the bytes a terminal sent, as Run gives them, with releases
// reported or not.
func fed(t *testing.T, exact bool, chunks ...string) *Input {
	t.Helper()
	in := newInput(exact)
	feed := newInputFeed(in)
	now := time.Unix(0, 0)
	for _, c := range chunks {
		if action := feed.feed([]byte(c), now); action != actionNone {
			t.Fatalf("%q: wanted no quit or suspend, got %v", c, action)
		}
	}
	in.beginFrame(now)
	return in
}

// The key events of a frame are in the order they came, each with its modifiers and what happened to it.
func TestInputKeyEventsAreKeptInOrderWithTheirModifiers(t *testing.T) {
	in := fed(t, true,
		"\x1b[97;;97u",  // a pressed
		"\x1b[97;1:3u",  // a let go
		"\x1b[1;5C",     // Ctrl and the right arrow
		"\x1b[119;5u",   // Ctrl+W, a chord
		"\x1b[100;3:2u", // Alt+D repeating
		"\x1b[15;2:3~",  // Shift and F5 let go
	)
	want := []KeyEvent{
		{Key: 'a', Action: ActionPress},
		{Key: 'a', Action: ActionRelease},
		{Key: KeyRight, Mods: ModCtrl, Action: ActionPress},
		{Key: 'w', Mods: ModCtrl, Action: ActionPress},
		{Key: 'd', Mods: ModAlt, Action: ActionRepeat},
		{Key: KeyF5, Mods: ModShift, Action: ActionRelease},
	}
	if got := in.KeyEvents(); !slices.Equal(got, want) {
		t.Errorf("wanted %v, got %v", want, got)
	}
}

// A chord is not the key itself: only KeyEvents sees it, and it neither starts nor lets go of the plain key.
func TestInputChordsAreOnlyInKeyEvents(t *testing.T) {
	in := fed(t, true, "\x1b[119u", "\x1b[119;5u", "\x1b[119;5:3u", "\x1b[119;3u", "\x1b[119;9u")
	if got := len(in.KeyEvents()); got != 5 {
		t.Errorf("wanted the press and the four chord events, got %d", got)
	}
	if in.Started('w') != true || in.Pressed('w') != true {
		t.Errorf("wanted the one plain press, got started=%v pressed=%v", in.Started('w'), in.Pressed('w'))
	}
	in.endFrame() // what is left is held: a chord's release must not have let the key go
	if !in.IsDown('w') {
		t.Errorf("wanted w still down: a chord let go of it, or the plain press was lost")
	}

	chordsOnly := fed(t, true, "\x1b[119;5u", "\x1b[119;3u", "\x1b[13;5u", "\x1b[27;3u", "\x1b[8;5u")
	for _, k := range []Key{'w', KeyEnter, KeyEscape, KeyBackspace} {
		if chordsOnly.IsDown(k) || chordsOnly.Pressed(k) || chordsOnly.Started(k) {
			t.Errorf("key %v: wanted a chord not to count as the key", k)
		}
	}

	legacy := fed(t, false, "\x17", "\x1bx", "\x1b\x7f")
	if legacy.IsDown('w') || legacy.IsDown('x') || legacy.IsDown(KeyBackspace) {
		t.Errorf("wanted Ctrl+W, Alt+X and Alt+Backspace to be chords without the protocol too")
	}
	if got := len(legacy.KeyEvents()); got != 3 {
		t.Errorf("wanted the three chords in KeyEvents, got %v", legacy.KeyEvents())
	}
}

// The arrows, navigation and function keys are the key whatever is held with them, as the arrows always were.
func TestInputSpecialKeysWithModifiersAreStillTheKey(t *testing.T) {
	for _, exact := range []bool{true, false} {
		in := fed(t, exact, "\x1b[1;5C", "\x1b[5;3~", "\x1b[15;7~", "\x1b[1;2H", "\x1b[1;9P")
		for _, k := range []Key{KeyRight, KeyPageUp, KeyF5, KeyHome, KeyF1} {
			if !in.Pressed(k) || !in.Started(k) || !in.IsDown(k) {
				t.Errorf("exact=%v key %v: wanted it pressed, started and down", exact, k)
			}
		}
		if got := len(in.KeyEvents()); got != 5 {
			t.Errorf("exact=%v: wanted 5 events, got %v", exact, in.KeyEvents())
		}
	}
}

func TestInputTextIsWhatWasTyped(t *testing.T) {
	tests := []struct {
		name   string
		exact  bool
		chunks []string
		want   string
	}{
		{"legacy letters", false, []string{"hello"}, "hello"},
		{"legacy keeps the case it was sent in", false, []string{"Hello, World"}, "Hello, World"},
		{"legacy non-ascii", false, []string{"héllo ", "日本"}, "héllo 日本"},
		{"kitty text of each key", true, []string{"\x1b[97;;97u", "\x1b[98;2;66u", "\x1b[233;;233u"}, "aBé"},
		{"kitty text of a repeat types again", true, []string{"\x1b[97;;97u", "\x1b[97;1:2;97u"}, "aa"},
		{"kitty several code points", true, []string{"\x1b[97;;97:98:99u"}, "abc"},
		{"kitty text beats the key", true, []string{"\x1b[49;2;33u"}, "!"},
		{"kitty without text falls back to the key", true, []string{"\x1b[97u", "\x1b[98;2u"}, "aB"},
		{"the fallback shifts ASCII letters only", true, []string{"\x1b[49;2u", "\x1b[233;2u"}, "1é"},
		{"a space is text", false, []string{"a b"}, "a b"},
		{"split reads", false, []string{"h", "i", " there"}, "hi there"},
		{"nothing typed", false, nil, ""},
	}
	for _, tt := range tests {
		if got := fed(t, tt.exact, tt.chunks...).Text(); got != tt.want {
			t.Errorf("%s: wanted %q, got %q", tt.name, tt.want, got)
		}
	}
}

// Text is characters: not keys that type nothing, not chords, not releases, not a paste of control codes.
func TestInputTextLeavesOutWhatTypesNothing(t *testing.T) {
	tests := []struct {
		name   string
		exact  bool
		chunks []string
	}{
		{"enter tab backspace", false, []string{"\r\t\x7f"}},
		{"arrows and function keys", false, []string{"\x1b[A\x1b[15~\x1bOP"}},
		{"ctrl letters", false, []string{"\x01\x02\x17"}},
		{"alt letters", false, []string{"\x1bx\x1bé"}},
		{"kitty chords", true, []string{"\x1b[119;5u", "\x1b[119;3u", "\x1b[119;9u"}},
		{"kitty enter with text", true, []string{"\x1b[13;;13u", "\x1b[9;;9u", "\x1b[127u"}},
		{"kitty releases", true, []string{"\x1b[97;1:3;97u", "\x1b[98;2:3u"}},
	}
	for _, tt := range tests {
		if got := fed(t, tt.exact, tt.chunks...).Text(); got != "" {
			t.Errorf("%s: wanted no text, got %q", tt.name, got)
		}
	}
}

// A terminal that says a chord typed something is believed: Option and a is a with a ring on a Mac. The chord
// is still not the key, and still an event with its modifier.
func TestInputTextOfAChordTheTerminalSaidTyped(t *testing.T) {
	in := fed(t, true, "\x1b[97;3;229u", "\x1b[98;5u")
	if got := in.Text(); got != "å" {
		t.Errorf("wanted the å the terminal said, and nothing for Ctrl+B, got %q", got)
	}
	if in.IsDown('a') || in.IsDown('b') {
		t.Errorf("wanted neither chord to be a key")
	}
	if got := in.KeyEvents(); len(got) != 2 || got[0].Mods != ModAlt || got[1].Mods != ModCtrl {
		t.Errorf("wanted both chords as events with their modifiers, got %v", got)
	}
}

func TestInputFrameForgetsItsEventsAndText(t *testing.T) {
	in := fed(t, true, "\x1b[97;;97u", "\x1b[119;5u")
	if len(in.KeyEvents()) != 2 || in.Text() != "a" {
		t.Fatalf("wanted two events and the text a, got %v and %q", in.KeyEvents(), in.Text())
	}
	in.endFrame()
	if len(in.KeyEvents()) != 0 || in.Text() != "" {
		t.Errorf("wanted the next frame to start with nothing, got %v and %q", in.KeyEvents(), in.Text())
	}
}

func TestInputKeyEventsLeaveOutWhatIsRuns(t *testing.T) {
	// Ctrl-C and Ctrl-Z end or suspend Run: they are never the game's, a release of them included.
	in := newInput(true)
	feed := newInputFeed(in)
	feed.canSuspend = true
	if got := feed.feed([]byte("\x1b[99;5:3u"), time.Unix(0, 0)); got != actionNone {
		t.Fatalf("wanted a release of Ctrl-C to do nothing, got %v", got)
	}
	if len(in.KeyEvents()) != 0 {
		t.Errorf("wanted no event for a release of Ctrl-C, got %v", in.KeyEvents())
	}
	if got := feed.feed([]byte("\x1b[99;5u"), time.Unix(0, 0)); got != actionQuit {
		t.Errorf("wanted Ctrl-C to quit, got %v", got)
	}
	if len(in.KeyEvents()) != 0 {
		t.Errorf("wanted Ctrl-C not to be an event, got %v", in.KeyEvents())
	}
}

func TestInputFocusAndResetAreNotKeyEvents(t *testing.T) {
	in := fed(t, true, "\x1b[I", "\x1b[O", "\x1b[119u", "\x1b[11")
	if got := in.KeyEvents(); len(got) != 1 || got[0].Key != 'w' {
		t.Errorf("wanted only the press of w, got %v", got)
	}
}

// Without the protocol no release or repeat is ever reported, so none is listed, as a Keyboard's Release says.
func TestInputWithoutReleasesListsOnlyPresses(t *testing.T) {
	in := newInput(false)
	in.event(keyEvent{key: 'w', kind: kindPress}, time.Unix(0, 0))
	in.event(keyEvent{key: 'w', kind: kindRelease}, time.Unix(0, 0))
	if got := in.KeyEvents(); len(got) != 1 || got[0].Action != ActionPress {
		t.Errorf("wanted the press only, got %v", got)
	}
}

// What one frame keeps is bounded, and the maps still know every key.
func TestInputFrameLimits(t *testing.T) {
	in := fed(t, false, strings.Repeat("a", maxFrameEvents+100))
	if got := len(in.KeyEvents()); got != maxFrameEvents {
		t.Errorf("wanted the events cut at %d, got %d", maxFrameEvents, got)
	}
	if !in.Pressed('a') {
		t.Errorf("wanted the key to count for Pressed after the list was full")
	}
	if got := len(in.Text()); got != maxFrameText && got != maxFrameEvents+100 {
		t.Errorf("wanted the text kept up to its own limit, got %d bytes", got)
	}

	long := fed(t, false, strings.Repeat("é", maxFrameText))
	if got := long.Text(); len(got) > maxFrameText || !utf8.ValidString(got) {
		t.Errorf("wanted at most %d bytes of valid text, got %d bytes (valid: %v)", maxFrameText, len(got), utf8.ValidString(got))
	}

	huge := newInput(true)
	for range 10 * maxFrameText {
		huge.event(keyEvent{key: 'a', kind: kindPress}, time.Unix(0, 0))
	}
	if got := len(huge.Text()); got != maxFrameText {
		t.Errorf("wanted the text cut at %d bytes, got %d", maxFrameText, got)
	}
	huge.endFrame()
	if huge.Text() != "" || len(huge.KeyEvents()) != 0 {
		t.Errorf("wanted a new frame to be empty")
	}
}

func TestInputKeyEventsDoNotAllocateOnceWarm(t *testing.T) {
	in := newInput(true)
	feed := newInputFeed(in)
	now := time.Unix(0, 0)
	chunk := []byte("hello")
	frame := func() {
		feed.feed(chunk, now)
		_ = in.KeyEvents()
		in.endFrame()
	}
	frame() // grows the lists
	// feed allocates for the parser's events, so measure the Input side alone.
	events := []keyEvent{{key: 'h', kind: kindPress}, {key: 'i', kind: kindPress}, {key: 'x', kind: kindPress, mods: ModCtrl, chord: true}}
	in.endFrame()
	if n := testing.AllocsPerRun(50, func() {
		for _, e := range events {
			in.event(e, now)
		}
		in.endFrame()
	}); n > 2 { // the two typed characters: string(rune) of a letter does not allocate, a longer one may
		t.Errorf("wanted at most 2 allocations a frame for 3 events once the lists are grown, got %v", n)
	}
}

func TestModsHas(t *testing.T) {
	m := ModCtrl | ModShift
	if !m.Has(ModCtrl) || !m.Has(ModShift) || !m.Has(ModCtrl|ModShift) {
		t.Errorf("wanted Ctrl and Shift in %v", m)
	}
	if m.Has(ModAlt) || m.Has(ModCtrl|ModAlt) || m.Has(ModSuper) {
		t.Errorf("wanted Alt and Super not in %v", m)
	}
	if !m.Has(0) || !Mods(0).Has(0) {
		t.Errorf("wanted every set to hold the empty set")
	}
}

// The modifier bits are the protocol's, so that a number a terminal sends is the set.
func TestModsAreTheProtocolsBits(t *testing.T) {
	if ModShift != 1 || ModAlt != 2 || ModCtrl != 4 || ModSuper != 8 {
		t.Errorf("wanted shift 1, alt 2, ctrl 4, super 8, got %d %d %d %d", ModShift, ModAlt, ModCtrl, ModSuper)
	}
}

func TestModsOf(t *testing.T) {
	for bits, want := range map[int]Mods{
		0: 0, 1: ModShift, 2: ModAlt, 4: ModCtrl, 8: ModSuper, 15: modsMask,
		64: 0, 128: 0, 65: ModShift, 69: ModCtrl | ModShift, // caps lock and num lock are not modifiers
		16: 0, 32: 0, // hyper and meta are not read as a modifier here
		-1: 0, 256: 0, 260: 0, 1 << 30: 0, 1<<30 + 4: 0, 255: modsMask,
	} {
		if got := modsOf(bits); got != want {
			t.Errorf("modsOf(%d): wanted %v, got %v", bits, want, got)
		}
	}
}

func TestChordOf(t *testing.T) {
	for _, tt := range []struct {
		k    Key
		mods Mods
		want bool
	}{
		{'w', 0, false}, {'w', ModShift, false}, {'w', ModCtrl, true}, {'w', ModAlt, true}, {'w', ModSuper, true},
		{'w', ModShift | ModCtrl, true}, {KeyEnter, ModCtrl, true}, {KeyEscape, ModAlt, true}, {KeyTab, ModShift, false},
		{KeyRight, ModCtrl, false}, {KeyHome, ModAlt, false}, {KeyF5, ModCtrl | ModAlt, false}, {KeyPageDown, ModSuper, false},
	} {
		if got := chordOf(tt.k, tt.mods); got != tt.want {
			t.Errorf("chordOf(%v, %v): wanted %v, got %v", tt.k, tt.mods, tt.want, got)
		}
	}
}

func TestTextOf(t *testing.T) {
	for _, tt := range []struct {
		name string
		e    keyEvent
		want string
	}{
		{"a letter", keyEvent{key: 'a'}, "a"},
		{"a shifted letter", keyEvent{key: 'a', mods: ModShift}, "A"},
		{"a shifted digit", keyEvent{key: '1', mods: ModShift}, "1"},
		{"a space", keyEvent{key: ' '}, " "},
		{"a repeat", keyEvent{key: 'a', kind: kindRepeat}, "a"},
		{"a release", keyEvent{key: 'a', kind: kindRelease}, ""},
		{"a chord", keyEvent{key: 'a', mods: ModCtrl, chord: true}, ""},
		{"a special key", keyEvent{key: KeyEnter}, ""},
		{"a control code", keyEvent{key: 7}, ""},
		{"delete", keyEvent{key: 0x7f}, ""},
		{"a c1 control", keyEvent{key: 0x85}, ""},
		{"the last c1 control", keyEvent{key: 0x9f}, ""},
		{"the first character after them", keyEvent{key: 0xa0}, "\u00a0"},
		{"a shifted z", keyEvent{key: 'z', mods: ModShift}, "Z"},
		{"a shifted letter before a", keyEvent{key: '`', mods: ModShift}, "`"},
		{"a shifted letter after z", keyEvent{key: '{', mods: ModShift}, "{"},
		{"a surrogate", keyEvent{key: 0xd800}, ""},
		{"past the last rune", keyEvent{key: 0x110000}, ""},
		{"non-ascii", keyEvent{key: 'é', mods: ModShift}, "é"},
	} {
		if got := textOf(tt.e); got != tt.want {
			t.Errorf("%s: wanted %q, got %q", tt.name, tt.want, got)
		}
	}
}

func TestKittyText(t *testing.T) {
	for _, tt := range []struct {
		name   string
		fields [][]int
		want   string
	}{
		{"no third field", [][]int{{97}, {1}}, ""},
		{"empty", [][]int{{97}, {0}, {0}}, ""},
		{"one", [][]int{{97}, {0}, {97}}, "a"},
		{"several", [][]int{{97}, {0}, {104, 105}}, "hi"},
		{"controls and delete left out", [][]int{{97}, {0}, {7, 65, 127, 66, 155}}, "AB"},
		{"the last c1 control is left out and the next character is kept", [][]int{{97}, {0}, {0x9f, 0xa0, 0x7e, 0x7f}}, "\u00a0~"},
		{"a space is kept", [][]int{{97}, {0}, {32}}, " "},
		{"not a rune left out", [][]int{{97}, {0}, {55296, 65, 1114112}}, "A"},
		{"cut at sixteen", [][]int{{97}, {0}, slices.Repeat([]int{97}, 40)}, strings.Repeat("a", 16)},
	} {
		if got := kittyText(tt.fields); got != tt.want {
			t.Errorf("%s: wanted %q, got %q", tt.name, tt.want, got)
		}
	}
}

func TestActionOf(t *testing.T) {
	for kind, want := range map[keyKind]KeyAction{kindPress: ActionPress, kindRepeat: ActionRepeat, kindRelease: ActionRelease} {
		if got := actionOf(kind); got != want {
			t.Errorf("kind %d: wanted action %d, got %d", kind, want, got)
		}
	}
}

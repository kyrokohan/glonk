package engine

import (
	"slices"
	"strings"
	"testing"
)

func presses(keys ...Key) []keyEvent {
	events := make([]keyEvent, len(keys))
	for i, k := range keys {
		events[i] = keyEvent{key: k, kind: kindPress}
	}
	return events
}

// chord is a character key with modifiers held that make it a shortcut, which only KeyEvents sees.
func chord(k Key, mods Mods) keyEvent {
	return keyEvent{key: k, kind: kindPress, mods: mods, chord: true}
}

// modded is a special key with modifiers held, which IsDown, Pressed and Started see whatever they are.
func modded(k Key, kind keyKind, mods Mods) keyEvent {
	return keyEvent{key: k, kind: kind, mods: mods}
}

type keyCase struct {
	name string
	in   string
	want []keyEvent
}

// legacyKeyCases are what terminals without the kitty keyboard protocol send, for
// input that is complete: parseEvents must give exactly this, and a keyParser fed in
// pieces must agree.
var legacyKeyCases = []keyCase{
	{"printable", "wa", presses('w', 'a')},
	{"arrows", "\x1b[A\x1b[D", presses(KeyUp, KeyLeft)},
	{"ss3 arrow", "\x1bOB", presses(KeyDown)},
	{"ss3 all four arrows", "\x1bOA\x1bOB\x1bOC\x1bOD", presses(KeyUp, KeyDown, KeyRight, KeyLeft)},
	{"ss3 just past the arrows", "\x1bOE", nil},
	{"modified arrow keeps its modifier and is no chord", "\x1b[1;5C", []keyEvent{modded(KeyRight, kindPress, ModCtrl)}},
	{"unknown csi skipped", "\x1b[16~x", presses('x')},
	{"lone escape", "\x1b", presses(KeyEscape)},
	{"alt and a key, not escape then key", "\x1bx", []keyEvent{chord('x', ModAlt)}},
	{"truncated csi is dropped and reported as a reset", "\x1b[", []keyEvent{{key: KeyUnknown, kind: kindReset}}},
	{"controls", "\x03\r\x7f\t", presses(KeyCtrlC, KeyEnter, KeyBackspace, KeyTab)},
	{"ctrl-z", "\x1a", presses(keyCtrlZ)},
	{"ctrl-z among keys", "a\x1ab", presses('a', keyCtrlZ, 'b')},
	{"ctrl and a letter", "\x01\x17\x0a", []keyEvent{chord('a', ModCtrl), chord('w', ModCtrl), chord('j', ModCtrl)}},
	{"other controls dropped", "\x00\x1c\x1d\x1e\x1f", nil},
	{"ctrl-h ctrl-i ctrl-m are backspace, tab, enter", "\x08\x09\x0d", presses(KeyBackspace, KeyTab, KeyEnter)},
	{"utf-8", "é", presses('é')},
	{"invalid byte dropped", "\xff", nil},
	{"empty", "", nil},
	{"alt+o at the end is dropped", "\x1bO", nil},
	{"ss3 f1 to f4", "\x1bOP\x1bOQ\x1bOR\x1bOS", presses(KeyF1, KeyF2, KeyF3, KeyF4)},
	{"ss3 home and end", "\x1bOH\x1bOF", presses(KeyHome, KeyEnd)},
	{"ss3 keypad is dropped", "\x1bOM\x1bOj", nil},
	{"csi home and end", "\x1b[H\x1b[F", presses(KeyHome, KeyEnd)},
	{"csi bare f1 to f4", "\x1b[P\x1b[Q\x1b[R\x1b[S", presses(KeyF1, KeyF2, KeyF3, KeyF4)},
	{"tilde editing keys", "\x1b[1~\x1b[2~\x1b[3~\x1b[4~\x1b[5~\x1b[6~\x1b[7~\x1b[8~",
		presses(KeyHome, KeyInsert, KeyDelete, KeyEnd, KeyPageUp, KeyPageDown, KeyHome, KeyEnd)},
	{"tilde function keys", "\x1b[11~\x1b[12~\x1b[13~\x1b[14~\x1b[15~\x1b[17~\x1b[18~\x1b[19~\x1b[20~\x1b[21~\x1b[23~\x1b[24~",
		presses(KeyF1, KeyF2, KeyF3, KeyF4, KeyF5, KeyF6, KeyF7, KeyF8, KeyF9, KeyF10, KeyF11, KeyF12)},
	{"csi f1 to f4 with a modifier", "\x1b[1;2P\x1b[1;5Q\x1b[1;3R\x1b[1;6S",
		[]keyEvent{modded(KeyF1, kindPress, ModShift), modded(KeyF2, kindPress, ModCtrl), modded(KeyF3, kindPress, ModAlt), modded(KeyF4, kindPress, ModCtrl|ModShift)}},
	{"modified home and end", "\x1b[1;5H\x1b[1;2F", []keyEvent{modded(KeyHome, kindPress, ModCtrl), modded(KeyEnd, kindPress, ModShift)}},
	{"modified tilde keys", "\x1b[5;5~\x1b[15;2~\x1b[3;3~",
		[]keyEvent{modded(KeyPageUp, kindPress, ModCtrl), modded(KeyF5, kindPress, ModShift), modded(KeyDelete, kindPress, ModAlt)}},
	{"every modifier of a legacy sequence", "\x1b[1;2A\x1b[1;3A\x1b[1;4A\x1b[1;5A\x1b[1;8A\x1b[1;9A\x1b[1;16A",
		[]keyEvent{modded(KeyUp, kindPress, ModShift), modded(KeyUp, kindPress, ModAlt), modded(KeyUp, kindPress, ModAlt|ModShift),
			modded(KeyUp, kindPress, ModCtrl), modded(KeyUp, kindPress, ModCtrl|ModAlt|ModShift), modded(KeyUp, kindPress, ModSuper),
			modded(KeyUp, kindPress, ModSuper|ModCtrl|ModAlt|ModShift)}},
	{"a modifier that is not a number is none", "\x1b[1;;A\x1b[1;0A\x1b[1;99999999999999999999A", presses(KeyUp, KeyUp, KeyUp)},
	{"tilde numbers that are no key", "\x1b[0~\x1b[9~\x1b[10~\x1b[16~\x1b[22~\x1b[25~\x1b[99~", nil},
	{"cursor position report is not f3", "\x1b[12;40R\x1b[24;80R", nil},
	{"cursor position is not home", "\x1b[5;10H\x1b[2;7F", nil},
	{"function key between letters", "a\x1b[15~b", presses('a', KeyF5, 'b')},
	{"two function keys in one read", "\x1bOP\x1b[24~", presses(KeyF1, KeyF12)},
	{"alt+o then a plain key", "\x1bO1", presses('1')},
	{"double escape", "\x1b\x1b", presses(KeyEscape, KeyEscape)},
	{"escape then an arrow", "\x1b\x1b[A", presses(KeyEscape, KeyUp)},
	{"alt chord with a non-ascii key", "\x1bé", []keyEvent{chord('é', ModAlt)}},
	{"alt+enter", "\x1b\r", nil},
	{"alt+backspace", "\x1b\x7f", []keyEvent{chord(KeyBackspace, ModAlt)}},
	{"alt chord then a key", "\x1bxw", []keyEvent{chord('x', ModAlt), {key: 'w', kind: kindPress}}},
	{"key then alt chord", "w\x1bx", []keyEvent{{key: 'w', kind: kindPress}, chord('x', ModAlt)}},
	{"alt and a space", "\x1b ", []keyEvent{chord(' ', ModAlt)}},
	{"alt and a control byte is dropped", "\x1b\x01\x1b\x00", nil},
	{"alt chord with an invalid byte", "\x1b\xff", nil},
	{"backspace ctrl-h", "\x08", presses(KeyBackspace)},
}

func TestParseEvents(t *testing.T) {
	for _, tt := range legacyKeyCases {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEvents([]byte(tt.in))

			if !slices.Equal(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

// kittyKeyCases are sequences a terminal sends once the kitty keyboard protocol is on
// (flags 1|2|8), plus things that must be ignored.
var kittyKeyCases = []keyCase{
	{"letter press", "\x1b[119u", presses('w')},
	{"letter press explicit", "\x1b[119;1:1u", presses('w')},
	{"letter repeat", "\x1b[119;1:2u", []keyEvent{{key: 'w', kind: kindRepeat}}},
	{"letter release", "\x1b[119;1:3u", []keyEvent{{key: 'w', kind: kindRelease}}},
	{"shift is a modifier and the key stays w", "\x1b[119;2u", []keyEvent{{key: 'w', kind: kindPress, mods: ModShift}}},
	{"shift release", "\x1b[119;2:3u", []keyEvent{{key: 'w', kind: kindRelease, mods: ModShift}}},
	{"caps lock ignored", "\x1b[119;65u", presses('w')},
	{"num lock ignored", "\x1b[119;129u", presses('w')},
	{"shift and caps lock", "\x1b[119;66u", []keyEvent{{key: 'w', kind: kindPress, mods: ModShift}}},
	{"two events in one read", "\x1b[119;1:3u\x1b[101u", []keyEvent{{key: 'w', kind: kindRelease}, {key: 'e', kind: kindPress}}},
	{"escape", "\x1b[27u", presses(KeyEscape)},
	{"enter", "\x1b[13u", presses(KeyEnter)},
	{"tab", "\x1b[9u", presses(KeyTab)},
	{"backspace", "\x1b[127u", presses(KeyBackspace)},
	{"enter release", "\x1b[13;1:3u", []keyEvent{{key: KeyEnter, kind: kindRelease}}},
	{"ctrl-c", "\x1b[99;5u", presses(KeyCtrlC)},
	{"ctrl-c with caps lock", "\x1b[99;69u", presses(KeyCtrlC)},
	{"ctrl-c release", "\x1b[99;5:3u", []keyEvent{{key: KeyCtrlC, kind: kindRelease}}},
	{"ctrl-z", "\x1b[122;5u", presses(keyCtrlZ)},
	{"ctrl-z with caps lock", "\x1b[122;69u", presses(keyCtrlZ)},
	{"ctrl-z repeat", "\x1b[122;5:2u", []keyEvent{{key: keyCtrlZ, kind: kindRepeat}}},
	{"ctrl-z release", "\x1b[122;5:3u", []keyEvent{{key: keyCtrlZ, kind: kindRelease}}},
	{"plain z", "\x1b[122u", presses('z')},
	{"alt-z is a chord", "\x1b[122;3u", []keyEvent{chord('z', ModAlt)}},
	{"ctrl-w is a chord", "\x1b[119;5u", []keyEvent{chord('w', ModCtrl)}},
	{"alt-w is a chord", "\x1b[119;3u", []keyEvent{chord('w', ModAlt)}},
	{"super-w is a chord", "\x1b[119;9u", []keyEvent{chord('w', ModSuper)}},
	{"ctrl-alt-c is a chord, not ctrl-c", "\x1b[99;7u", []keyEvent{chord('c', ModCtrl|ModAlt)}},
	{"ctrl-shift-w with caps and num lock", "\x1b[119;198u", []keyEvent{chord('w', ModCtrl|ModShift)}},
	{"hyper chord dropped", "\x1b[119;17u", nil},
	{"meta chord dropped", "\x1b[119;33u", nil},
	{"hyper with ctrl-c dropped", "\x1b[99;21u", nil},
	{"ctrl-w release is a chord release", "\x1b[119;5:3u", []keyEvent{{key: 'w', kind: kindRelease, mods: ModCtrl, chord: true}}},
	{"ctrl-enter is a chord", "\x1b[13;5u", []keyEvent{chord(KeyEnter, ModCtrl)}},
	{"alt-escape is a chord", "\x1b[27;3u", []keyEvent{chord(KeyEscape, ModAlt)}},
	{"shift-tab is no chord", "\x1b[9;2u", []keyEvent{{key: KeyTab, kind: kindPress, mods: ModShift}}},
	{"text of a plain key", "\x1b[97;;97u", []keyEvent{{key: 'a', kind: kindPress, text: "a"}}},
	{"text of a space", "\x1b[32;;32u", []keyEvent{{key: ' ', kind: kindPress, text: " "}}},
	{"text of an alt chord is believed", "\x1b[97;3;229u", []keyEvent{{key: 'a', kind: kindPress, mods: ModAlt, chord: true, text: "å"}}},
	{"text of a shifted key", "\x1b[97;2;65u", []keyEvent{{key: 'a', kind: kindPress, mods: ModShift, text: "A"}}},
	{"text of a repeat", "\x1b[97;1:2;97u", []keyEvent{{key: 'a', kind: kindRepeat, text: "a"}}},
	{"no text on a release", "\x1b[97;1:3;97u", []keyEvent{{key: 'a', kind: kindRelease}}},
	{"text of several code points", "\x1b[97;;97:98u", []keyEvent{{key: 'a', kind: kindPress, text: "ab"}}},
	{"text of a non-ascii key", "\x1b[233;;233u", []keyEvent{{key: 'é', kind: kindPress, text: "é"}}},
	{"control characters are not text", "\x1b[97;;7:97:127:155u", []keyEvent{{key: 'a', kind: kindPress, text: "a"}}},
	{"text that is not a character is dropped", "\x1b[97;;55296:1114112u", []keyEvent{{key: 'a', kind: kindPress}}},
	{"a code point that would wrap into a rune is dropped", "\x1b[97;;4294967361:4294967361u", []keyEvent{{key: 'a', kind: kindPress}}},
	{"a code point past int32 beside a good one", "\x1b[97;;65:4294967361:66u", []keyEvent{{key: 'a', kind: kindPress, text: "AB"}}},
	{"enter has no text to type", "\x1b[13;;13u", []keyEvent{{key: KeyEnter, kind: kindPress}}},
	{"unicode letter", "\x1b[233u", presses('é')},
	{"arrow press", "\x1b[1;1A", presses(KeyUp)},
	{"arrow repeat", "\x1b[1;1:2B", []keyEvent{{key: KeyDown, kind: kindRepeat}}},
	{"arrow release", "\x1b[1;1:3C", []keyEvent{{key: KeyRight, kind: kindRelease}}},
	{"arrow release with shift", "\x1b[1;2:3D", []keyEvent{modded(KeyLeft, kindRelease, ModShift)}},
	{"bare arrow", "\x1b[A", presses(KeyUp)},
	{"f5 press repeat release", "\x1b[15~\x1b[15;1:2~\x1b[15;1:3~", []keyEvent{{key: KeyF5, kind: kindPress}, {key: KeyF5, kind: kindRepeat}, {key: KeyF5, kind: kindRelease}}},
	{"f5 release with ctrl", "\x1b[15;5:3~", []keyEvent{modded(KeyF5, kindRelease, ModCtrl)}},
	{"f1 release", "\x1b[1;1:3P", []keyEvent{{key: KeyF1, kind: kindRelease}}},
	{"f4 repeat", "\x1b[1;1:2S", []keyEvent{{key: KeyF4, kind: kindRepeat}}},
	{"home release", "\x1b[1;1:3H", []keyEvent{{key: KeyHome, kind: kindRelease}}},
	{"end repeat", "\x1b[1;1:2F", []keyEvent{{key: KeyEnd, kind: kindRepeat}}},
	{"page up release", "\x1b[5;1:3~", []keyEvent{{key: KeyPageUp, kind: kindRelease}}},
	{"insert and delete press", "\x1b[2~\x1b[3;1:1~", presses(KeyInsert, KeyDelete)},
	{"f12 with ctrl and alt", "\x1b[24;7~", []keyEvent{modded(KeyF12, kindPress, ModCtrl|ModAlt)}},
	{"navigation key between letter events", "\x1b[119u\x1b[6;1:3~\x1b[119;1:3u", []keyEvent{{key: 'w', kind: kindPress}, {key: KeyPageDown, kind: kindRelease}, {key: 'w', kind: kindRelease}}},
	{"lone modifier key dropped", "\x1b[57441;1u", nil},
	{"keypad key dropped", "\x1b[57399u", nil},
	{"first private use code dropped", "\x1b[57344u", nil},
	{"space key", "\x1b[32u", presses(' ')},
	{"just below space dropped", "\x1b[31u", nil},
	{"private marker lt dropped", "\x1b[<1u", nil},
	{"private marker gt dropped", "\x1b[>11u", nil},
	{"private arrow lookalike dropped", "\x1b[?1A", nil},
	{"private lt arrow lookalike dropped", "\x1b[<1A", nil},
	{"private reply dropped", "\x1b[?11u", nil},
	{"device attributes reply dropped", "\x1b[?62;4c", nil},
	{"missing code dropped", "\x1b[u", nil},
	{"empty code with mods dropped", "\x1b[;5u", nil},
	{"only separators dropped", "\x1b[1;u", nil},
	{"control code dropped", "\x1b[5u", nil},
	{"surrogate dropped", "\x1b[55296u", nil},
	{"past max rune dropped", "\x1b[1114112u", nil},
	{"huge number dropped", "\x1b[99999999999999999999u", nil},
	{"negative code dropped", "\x1b[-5u", nil},
	{"unknown event type is press", "\x1b[119;1:9u", presses('w')},
	{"event type without modifiers value", "\x1b[119;:3u", []keyEvent{{key: 'w', kind: kindRelease}}},
	{"unknown final dropped", "\x1b[119;1:3z", nil},
	{"focus gained", "\x1b[I", []keyEvent{{key: KeyUnknown, kind: kindFocusGained}}},
	{"focus lost", "\x1b[O", []keyEvent{{key: KeyUnknown, kind: kindFocusLost}}},
	{"focus lost between keys", "\x1b[119u\x1b[O\x1b[119;1:3u", []keyEvent{{key: 'w', kind: kindPress}, {key: KeyUnknown, kind: kindFocusLost}, {key: 'w', kind: kindRelease}}},
	{"focus with parameters dropped", "\x1b[1I", nil},
	{"junk then key", "\x1b[?11u\x1b[119u", presses('w')},
	{"key then truncated is reported as a reset", "\x1b[119u\x1b[11", []keyEvent{{key: 'w', kind: kindPress}, {key: KeyUnknown, kind: kindReset}}},
}

func TestParseEventsKitty(t *testing.T) {
	for _, tt := range kittyKeyCases {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEvents([]byte(tt.in))

			if !slices.Equal(got, tt.want) {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
		})
	}
}

// feedAll sends the pieces to a fresh parser one after another, then flushes, the way
// Run does when nothing more arrives, and returns every event in order.
func feedAll(pieces ...string) []keyEvent {
	var p keyParser
	var events []keyEvent
	for _, piece := range pieces {
		events = append(events, p.Feed([]byte(piece))...)
	}
	return append(events, p.Flush()...)
}

// allKeyInputsForFuzz is every input of the tables above, as seeds for a fuzzer.
func allKeyInputsForFuzz() []string { return allKeyInputs() }

func allKeyInputs() []string {
	var inputs []string
	for _, c := range legacyKeyCases {
		inputs = append(inputs, c.in)
	}
	for _, c := range kittyKeyCases {
		inputs = append(inputs, c.in)
	}
	return inputs
}

// The point of a keyParser: however a stream is cut into reads, the keys that come
// out are the keys of the whole stream.
func TestKeyParserSplitAnywhereMatchesTheWhole(t *testing.T) {
	inputs := allKeyInputs()

	check := func(whole string, cuts ...int) {
		t.Helper()
		var pieces []string
		prev := 0
		for _, c := range cuts {
			pieces = append(pieces, whole[prev:c])
			prev = c
		}
		pieces = append(pieces, whole[prev:])

		want := parseEvents([]byte(whole))
		if got := feedAll(pieces...); !slices.Equal(got, want) {
			t.Fatalf("%q cut as %q: wanted %v, got %v", whole, pieces, want, got)
		}
	}

	for _, whole := range inputs {
		// Every single cut, every two cuts, and one byte at a time.
		for a := 0; a <= len(whole); a++ {
			check(whole, a)
			for b := a; b <= len(whole); b++ {
				check(whole, a, b)
			}
		}
		var each []int
		for k := 1; k < len(whole); k++ {
			each = append(each, k)
		}
		check(whole, each...)
	}

	// Two inputs back to back, so sequences meet the start of the next one.
	for _, first := range inputs {
		for _, second := range inputs {
			whole := first + second
			for k := 0; k <= len(whole); k++ {
				check(whole, k)
			}
		}
	}
}

func TestKeyParserLoneEscapeWaitsForTheRest(t *testing.T) {
	var p keyParser

	if got := p.Feed([]byte("\x1b")); len(got) != 0 || !p.Pending() {
		t.Fatalf("wanted a lone ESC to be held, got events %v pending=%v", got, p.Pending())
	}
	if got := p.Feed([]byte("[A")); !slices.Equal(got, presses(KeyUp)) || p.Pending() {
		t.Errorf("wanted the arrow to complete as Up, got %v pending=%v", got, p.Pending())
	}

	// If nothing follows, flushing says it was the Escape key.
	if got := p.Feed([]byte("\x1b")); len(got) != 0 {
		t.Fatalf("wanted a lone ESC held again, got %v", got)
	}
	if got := p.Flush(); !slices.Equal(got, presses(KeyEscape)) || p.Pending() {
		t.Errorf("wanted Escape after the wait, got %v pending=%v", got, p.Pending())
	}

	// The key typed after the wait is its own key, not an Alt chord with the Escape.
	if got := p.Feed([]byte("x")); !slices.Equal(got, presses('x')) {
		t.Errorf("wanted x on its own, got %v", got)
	}
}

func TestKeyParserSplitReleaseIsNotLost(t *testing.T) {
	press := []keyEvent{{key: 'w', kind: kindPress}}
	release := []keyEvent{{key: 'w', kind: kindRelease}}

	var p keyParser
	var got []keyEvent
	for _, piece := range []string{"\x1b[119u", "\x1b[119;1", ":3u"} {
		got = append(got, p.Feed([]byte(piece))...)
	}
	if want := append(append([]keyEvent{}, press...), release...); !slices.Equal(got, want) {
		t.Errorf("wanted a press then a release, got %v", got)
	}
	if p.Pending() {
		t.Errorf("wanted nothing held, got pending bytes")
	}
}

func TestKeyParserSplitRunesDecodeOnce(t *testing.T) {
	for _, r := range []string{"é", "€", "😀"} {
		for k := 1; k < len(r); k++ {
			var p keyParser
			got := p.Feed([]byte(r[:k]))
			if len(got) != 0 || !p.Pending() {
				t.Fatalf("%q cut at %d: wanted the first part held, got %v pending=%v", r, k, got, p.Pending())
			}
			got = p.Feed([]byte(r[k:]))
			if want := presses(Key([]rune(r)[0])); !slices.Equal(got, want) {
				t.Errorf("%q cut at %d: wanted %v, got %v", r, k, want, got)
			}
		}
	}

	// A rune that never finishes is dropped when the wait ends, with no key.
	var p keyParser
	p.Feed([]byte("é")[:1])
	if got := p.Flush(); len(got) != 0 {
		t.Errorf("wanted an unfinished rune dropped, got %v", got)
	}
}

func TestKeyParserAltChordIsNotEscape(t *testing.T) {
	want := []keyEvent{chord('x', ModAlt)}
	if got := feedAll("\x1bx"); !slices.Equal(got, want) {
		t.Errorf("wanted Alt+x, not Escape, in one piece, got %v", got)
	}
	if got := feedAll("\x1b", "x"); !slices.Equal(got, want) {
		t.Errorf("wanted Alt+x, not Escape, across two pieces, got %v", got)
	}
}

func TestKeyParserDiscardingAnUnfinishedSequenceReportsAReset(t *testing.T) {
	var p keyParser
	p.Feed([]byte("\x1b[119;1"))

	got := p.Flush()
	if want := []keyEvent{{key: KeyUnknown, kind: kindReset}}; !slices.Equal(got, want) {
		t.Errorf("wanted a reset when the wait ends, got %v", got)
	}
	if p.Pending() {
		t.Errorf("wanted nothing held after the flush")
	}
}

func TestKeyParserDropsGarbageThatNeverEnds(t *testing.T) {
	var p keyParser

	got := p.Feed([]byte("\x1b[" + strings.Repeat("1", 500)))
	if want := []keyEvent{{key: KeyUnknown, kind: kindReset}}; !slices.Equal(got, want) {
		t.Errorf("wanted a reset for a sequence longer than any key, got %v", got)
	}
	if p.Pending() {
		t.Errorf("wanted the garbage discarded, still holding %d bytes", len(p.pending))
	}

	// And the parser is fine afterwards.
	if got := p.Feed([]byte("w")); !slices.Equal(got, presses('w')) {
		t.Errorf("wanted the parser to carry on, got %v", got)
	}

	// Growing it a few bytes at a time hits the same limit, never more.
	for range 200 {
		p.Feed([]byte("\x1b[1"))
		p.Feed([]byte("1111"))
		if len(p.pending) > maxPendingInput {
			t.Fatalf("wanted at most %d bytes held, got %d", maxPendingInput, len(p.pending))
		}
	}
}

func TestKeyParserFlushWithNothingPending(t *testing.T) {
	var p keyParser
	if got := p.Flush(); len(got) != 0 || p.Pending() {
		t.Errorf("wanted an empty flush to do nothing, got %v pending=%v", got, p.Pending())
	}
	p.Feed([]byte("abc"))
	if p.Pending() {
		t.Errorf("wanted complete input never held")
	}
}

// Held bytes must not alias the caller's slice, which the caller is free to reuse.
func TestKeyParserCopiesWhatItHolds(t *testing.T) {
	var p keyParser
	buf := []byte("\x1b[")
	p.Feed(buf)
	buf[0], buf[1] = 'x', 'y'

	if got := p.Feed([]byte("A")); !slices.Equal(got, presses(KeyUp)) {
		t.Errorf("wanted the held bytes unaffected by the caller reusing its buffer, got %v", got)
	}
}

// BenchmarkKeyParserFeed is what each read from the terminal costs: a line of typed text, a burst of
// arrow keys, and function and navigation keys. The events it returns are most of the allocation.
func BenchmarkKeyParserFeed(b *testing.B) {
	for _, tt := range []struct{ name, in string }{
		{"typed text of 80 letters", strings.Repeat("glonk ", 13) + "ok"},
		{"20 arrow keys", strings.Repeat("\x1b[A\x1b[1;5C", 10)},
		{"function and navigation keys", "\x1b[15~\x1b[6~\x1bOH\x1bOP\x1b[1;2F\x1b[24~\x1b[3~\x1b[5;5~"},
		{"kitty releases", strings.Repeat("\x1b[119;1:3u\x1b[15;1:3~", 5)},
		{"kitty text of 20 keys", strings.Repeat("\x1b[104;;104u\x1b[105;2;73u", 10)},
		{"ctrl and alt chords", strings.Repeat("\x01\x17\x1bx\x1b[119;5u\x1b[1;5C", 4)},
	} {
		b.Run(tt.name, func(b *testing.B) {
			var p keyParser
			chunk := []byte(tt.in)
			b.ReportAllocs()
			for b.Loop() {
				p.Feed(chunk)
			}
		})
	}
}

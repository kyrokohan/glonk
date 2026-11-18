package engine

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// feedChunks gives a keyParser the input in reads of n bytes, and flushes at the end.
func feedChunks(in string, n int) []keyEvent {
	var p keyParser
	var events []keyEvent
	for b := []byte(in); len(b) > 0; {
		k := min(n, len(b))
		events = append(events, p.Feed(b[:k])...)
		b = b[k:]
	}
	return append(events, p.Flush()...)
}

// A paste is one event however the reads cut it, and the keys around it are the same.
func TestKeyParserPasteIsTheSameInAnyReads(t *testing.T) {
	in := "ab\x1b[200~hello, world\r\nline two\x1b[201~cd\x1b[200~\x1b[A\x1b[201~e"
	want := []keyEvent{
		{key: 'a', kind: kindPress}, {key: 'b', kind: kindPress},
		pasted("hello, world\nline two"),
		{key: 'c', kind: kindPress}, {key: 'd', kind: kindPress},
		pasted("\x1b[A"),
		{key: 'e', kind: kindPress},
	}
	for n := 1; n <= len(in); n++ {
		if got := feedChunks(in, n); !slices.Equal(got, want) {
			t.Fatalf("in reads of %d: wanted %v, got %v", n, want, got)
		}
	}
}

// The end marker is found wherever it is split, whatever bytes come before it.
func TestKeyParserPasteEndMarkerSplitAtEveryByte(t *testing.T) {
	for _, body := range []string{"", "x", "\x1b", "\x1b[", "\x1b[2", "\x1b[20", "\x1b[201", "\x1b[2011~", "ab\x1b[20"} {
		whole := "\x1b[200~" + body + "\x1b[201~k"
		want := []keyEvent{pasted(body), {key: 'k', kind: kindPress}}
		for cut := 0; cut <= len(whole); cut++ {
			var p keyParser
			got := append(p.Feed([]byte(whole[:cut])), p.Feed([]byte(whole[cut:]))...)
			got = append(got, p.Flush()...)
			if !slices.Equal(got, want) {
				t.Fatalf("%q cut at %d: wanted %v, got %v", whole, cut, want, got)
			}
		}
	}
}

func TestKeyParserPasteIsPendingUntilItEnds(t *testing.T) {
	var p keyParser
	if got := p.Feed([]byte("\x1b[200~par")); len(got) != 0 {
		t.Fatalf("wanted no event before the end of the paste, got %v", got)
	}
	if !p.Pending() || !p.Pasting() {
		t.Errorf("wanted a paste in progress to be pending")
	}
	if got := p.Feed([]byte("t\x1b[201~")); !slices.Equal(got, []keyEvent{pasted("part")}) {
		t.Errorf("wanted the paste, got %v", got)
	}
	if p.Pending() || p.Pasting() {
		t.Errorf("wanted nothing pending after the paste")
	}
}

// A paste that stalls is delivered as it stands, what might have been its end marker included, and keys after it
// are keys, not more of the paste.
func TestKeyParserFlushEndsAPasteAsItStands(t *testing.T) {
	var p keyParser
	p.Feed([]byte("\x1b[200~text\x1b[2"))
	got := p.Flush()
	if want := []keyEvent{pasted("text\x1b[2")}; !slices.Equal(got, want) {
		t.Fatalf("wanted %v, got %v", want, got)
	}
	if p.Pending() {
		t.Errorf("wanted nothing pending after the flush")
	}
	// The rest of the paste turns up late: its end marker is no key, and what is after is typed.
	if got := p.Feed([]byte("01~ok")); !slices.Equal(got, presses('0', '1', '~', 'o', 'k')) {
		t.Errorf("wanted the late bytes read as keys, got %v", got)
	}
}

func TestKeyParserPasteIsCutAtTheLimit(t *testing.T) {
	var p keyParser
	chunk := bytes.Repeat([]byte("a"), 4096)
	var events []keyEvent
	events = append(events, p.Feed([]byte("\x1b[200~"))...)
	for range maxPaste/len(chunk) + 3 {
		events = append(events, p.Feed(chunk)...)
	}
	events = append(events, p.Feed([]byte("\x1b[201~z"))...)
	if len(events) != 2 || events[0].kind != kindPaste || events[1].key != 'z' {
		t.Fatalf("wanted the paste and then z, got %d events", len(events))
	}
	if got := len(events[0].text); got != maxPaste {
		t.Errorf("wanted %d bytes kept, got %d", maxPaste, got)
	}
	if cap(p.paste) > 1<<16 {
		t.Errorf("wanted a big paste to leave no memory behind, got a capacity of %d", cap(p.paste))
	}

	// What is held while it is collected is bounded too, not only what is delivered.
	var q keyParser
	q.Feed([]byte("\x1b[200~"))
	for range maxPaste/len(chunk) + 50 {
		q.Feed(chunk)
	}
	if got := len(q.paste); got != maxPaste {
		t.Errorf("wanted %d bytes held of a paste past its limit, got %d", maxPaste, got)
	}
}

// addPaste keeps exactly maxPaste bytes, whether the last piece is short of the room, exactly it, or one over.
func TestKeyParserAddPasteEdgesOfTheLimit(t *testing.T) {
	for _, over := range []int{-3, -1, 0, 1, 2, 100} {
		var p keyParser
		p.paste = make([]byte, maxPaste-5)
		p.addPaste(bytes.Repeat([]byte("a"), 5+over))
		if want := min(maxPaste, maxPaste+over); len(p.paste) != want {
			t.Errorf("room for 5, adding %d: wanted %d bytes held, got %d", 5+over, want, len(p.paste))
		}
	}
}

// A paste whose invalid bytes make the text one character too long is cut where a character ends, which can be
// exactly at the limit.
func TestPasteTextCutExactlyOnACharacterBoundary(t *testing.T) {
	// Each "a" and invalid byte becomes "a" and U+FFFD, four bytes for two: 262144 of them are exactly maxPaste
	// bytes, and one more is over.
	in := strings.Repeat("a\xff", 262145)
	if got := pasteText(in); len(got) != maxPaste || !utf8.ValidString(got) {
		t.Errorf("wanted a valid text of exactly %d bytes, got %d (valid: %v)", maxPaste, len(got), utf8.ValidString(got))
	}
}

// A paste waits longer than an Escape key does, and a feed starts with both.
func TestPasteWaitsLongerThanAnEscape(t *testing.T) {
	if pasteTimeout <= escapeTimeout {
		t.Errorf("wanted a paste to be given more time than an escape, got %v and %v", pasteTimeout, escapeTimeout)
	}
	feed := newInputFeed(newInput(true))
	if feed.pasteWait != pasteTimeout || feed.wait != escapeTimeout {
		t.Errorf("wanted a feed to wait %v for an escape and %v for a paste, got %v and %v", escapeTimeout, pasteTimeout, feed.wait, feed.pasteWait)
	}
}

// A character cut in two by the limit leaves valid text.
func TestKeyParserPasteCutInTheMiddleOfACharacterIsValid(t *testing.T) {
	body := strings.Repeat("a", maxPaste-1) + "日本"
	got := feedChunks("\x1b[200~"+body+"\x1b[201~", 4096)
	if len(got) != 1 || !utf8.ValidString(got[0].text) || len(got[0].text) > maxPaste {
		t.Errorf("wanted one valid paste of at most %d bytes, got %d events", maxPaste, len(got))
	}
}

func TestPasteText(t *testing.T) {
	for _, tt := range []struct{ name, in, want string }{
		{"plain", "hello", "hello"},
		{"carriage returns", "a\rb", "a\nb"},
		{"crlf", "a\r\nb", "a\nb"},
		{"lf", "a\nb", "a\nb"},
		{"many breaks", "\r\r\n\n\r", "\n\n\n\n"},
		{"invalid utf-8", "a\xffb", "a\ufffdb"},
		{"cut character", "\xe6\x97", "\ufffd"},
		{"empty", "", ""},
		{"keeps controls", "\x1b[A\x03\t", "\x1b[A\x03\t"},
	} {
		if got := pasteText(tt.in); got != tt.want {
			t.Errorf("%s: wanted %q, got %q", tt.name, tt.want, got)
		}
	}
	if got := len(pasteText(strings.Repeat("a", maxPaste+500))); got != maxPaste {
		t.Errorf("wanted a text cut at %d bytes, got %d", maxPaste, got)
	}
	// Bytes that are not text become three each: the bound still holds, and no character is cut in two.
	for _, in := range []string{strings.Repeat("\xff", maxPaste), strings.Repeat("a", maxPaste-1) + "\xff\xff", strings.Repeat("日", maxPaste/3+9), strings.Repeat("a", maxPaste-2) + "日"} {
		got := pasteText(in)
		if len(got) > maxPaste || !utf8.ValidString(got) {
			t.Errorf("wanted at most %d bytes of valid text, got %d bytes (valid: %v)", maxPaste, len(got), utf8.ValidString(got))
		}
	}
}

func TestInputPaste(t *testing.T) {
	in := fed(t, true, "\x1b[200~hi there\x1b[201~")
	if got := in.Paste(); got != "hi there" {
		t.Errorf("wanted the pasted text, got %q", got)
	}
	if in.Text() != "" || len(in.KeyEvents()) != 0 {
		t.Errorf("wanted a paste in neither Text nor KeyEvents, got %q and %v", in.Text(), in.KeyEvents())
	}
	for _, k := range []Key{'h', 'i', ' ', KeyEnter} {
		if in.IsDown(k) || in.Pressed(k) || in.Started(k) {
			t.Errorf("key %v: wanted a paste not to press it", k)
		}
	}
	in.endFrame()
	if in.Paste() != "" {
		t.Errorf("wanted the next frame to start with no paste, got %q", in.Paste())
	}
}

func TestInputPastesInAFrameAreJoinedInOrder(t *testing.T) {
	in := fed(t, false, "\x1b[200~one\x1b[201~x\x1b[200~two\x1b[201~", "\x1b[200~\x1b[201~\x1b[200~three\x1b[201~")
	if got := in.Paste(); got != "onetwothree" {
		t.Errorf("wanted the pastes joined, got %q", got)
	}
	if in.Text() != "x" {
		t.Errorf("wanted the x typed between them, got %q", in.Text())
	}
}

func TestInputPastesInAFrameAreCutAtTheLimit(t *testing.T) {
	in := newInput(true)
	half := strings.Repeat("a", maxPaste/2+10)
	for range 3 {
		in.event(keyEvent{key: KeyUnknown, kind: kindPaste, text: half}, time.Unix(0, 0))
	}
	if got := len(in.Paste()); got != maxPaste {
		t.Errorf("wanted %d bytes kept of a frame's pastes, got %d", maxPaste, got)
	}

	// Not in the middle of a character.
	in = newInput(true)
	in.event(keyEvent{key: KeyUnknown, kind: kindPaste, text: strings.Repeat("a", maxPaste-1)}, time.Unix(0, 0))
	in.event(keyEvent{key: KeyUnknown, kind: kindPaste, text: "日"}, time.Unix(0, 0))
	if got := in.Paste(); !utf8.ValidString(got) || len(got) != maxPaste-1 {
		t.Errorf("wanted the character that did not fit left out whole, got %d bytes (valid: %v)", len(got), utf8.ValidString(got))
	}

	// What comes before the character that does not fit is kept, down to the byte: x fits, and the two bytes of
	// the character after it do not.
	in = newInput(true)
	in.event(keyEvent{key: KeyUnknown, kind: kindPaste, text: strings.Repeat("a", maxPaste-2)}, time.Unix(0, 0))
	in.event(keyEvent{key: KeyUnknown, kind: kindPaste, text: "xé"}, time.Unix(0, 0))
	if got := in.Paste(); len(got) != maxPaste-1 || !strings.HasSuffix(got, "ax") {
		t.Errorf("wanted the x kept and the é left out, got %d bytes ending %q", len(got), got[len(got)-3:])
	}
}

// What a paste holds is not read as keys: it cannot quit, suspend, or press anything.
func TestInputFeedPasteCannotQuitSuspendOrPress(t *testing.T) {
	in := newInput(true)
	feed := newInputFeed(in)
	feed.canSuspend = true
	now := time.Unix(0, 0)
	for _, c := range []string{"\x1b[200~\x03\x1a\r\x1b[A", "x\x1b[99;5u\x1b[201~"} {
		if got := feed.feed([]byte(c), now); got != actionNone {
			t.Fatalf("%q: wanted nothing to happen, got %v", c, got)
		}
	}
	if got := in.Paste(); got != "\x03\x1a\n\x1b[Ax\x1b[99;5u" {
		t.Errorf("wanted every byte of it as text, got %q", got)
	}
	if len(in.KeyEvents()) != 0 || in.IsDown(KeyEnter) || in.IsDown(KeyUp) {
		t.Errorf("wanted no key from a paste")
	}
	// And a Ctrl-C after it does quit.
	if got := feed.feed([]byte("\x03"), now); got != actionQuit {
		t.Errorf("wanted Ctrl-C after the paste to quit, got %v", got)
	}
}

// While a paste is being collected the wait is the paste's, a stall delivers it as it stands, and what comes
// after is read as keys.
func TestInputFeedPasteStallDeliversIt(t *testing.T) {
	in := newInput(true)
	feed := newInputFeed(in)
	feed.wait = time.Hour // an escape's wait must not be what ends a paste
	feed.pasteWait = 10 * time.Millisecond
	now := time.Unix(0, 0)
	feed.feed([]byte("\x1b[200~part"), now)
	if in.Paste() != "" {
		t.Fatalf("wanted nothing delivered before the paste ends")
	}
	select {
	case <-feed.expired():
	case <-time.After(5 * time.Second):
		t.Fatalf("wanted the wait for a stalled paste to end")
	}
	if got := feed.expire(now); got != actionNone {
		t.Fatalf("wanted nothing to quit, got %v", got)
	}
	if got := in.Paste(); got != "part" {
		t.Errorf("wanted the paste as it stood, got %q", got)
	}
	feed.feed([]byte("a"), now)
	if !in.Pressed('a') {
		t.Errorf("wanted the key after a stalled paste to be a key")
	}
}

// An escape's wait, not the paste's, is for a lone escape: pending a paste does not shorten or lengthen it.
func TestInputFeedEscapeStillWaitsItsOwnTime(t *testing.T) {
	feed := newInputFeed(newInput(true))
	feed.wait = 10 * time.Millisecond
	feed.pasteWait = time.Hour
	feed.feed([]byte("\x1b"), time.Unix(0, 0))
	select {
	case <-feed.expired():
	case <-time.After(5 * time.Second):
		t.Fatalf("wanted a lone escape to be waited for by the escape's wait, not the paste's")
	}
}

func TestKeyboardPaste(t *testing.T) {
	kb := NewKeyboard()
	kb.Paste("a\r\nb\xff")
	kb.Paste("")
	kb.Paste("c")
	in := kb.Input()
	if got := in.Paste(); got != "a\nb\ufffdc" {
		t.Errorf("wanted the pastes joined as a real one is, got %q", got)
	}
	if in.Text() != "" || len(in.KeyEvents()) != 0 || in.IsDown('a') {
		t.Errorf("wanted no key and no typing from a paste")
	}
	kb.EndFrame()
	if in.Paste() != "" {
		t.Errorf("wanted a new frame to have no paste")
	}
}

func TestKeyParserSmallPastesReuseTheirBuffer(t *testing.T) {
	var p keyParser
	chunk := []byte("\x1b[200~short paste\x1b[201~")
	p.Feed(chunk)
	if n := testing.AllocsPerRun(50, func() { p.Feed(chunk) }); n > 3 {
		t.Errorf("wanted three allocations for a small paste (the events, its text, its copy), and the collector's buffer reused, got %v", n)
	}
}

// BenchmarkKeyParserPaste is what a paste costs the parser: the markers, the copy of the text and the
// search for the end marker across reads.
func BenchmarkKeyParserPaste(b *testing.B) {
	for _, tt := range []struct {
		name       string
		size, read int
	}{{"4 KB in one read", 4096, 8192}, {"64 KB in 4 KB reads", 1 << 16, 4096}, {"1 MiB in 4 KB reads", 1 << 20, 4096}} {
		body := strings.Repeat("some pasted text, with words and a line\r\n", tt.size/40+1)[:tt.size]
		in := []byte("\x1b[200~" + body + "\x1b[201~")
		b.Run(tt.name, func(b *testing.B) {
			var p keyParser
			b.SetBytes(int64(len(in)))
			b.ReportAllocs()
			for b.Loop() {
				for rest := in; len(rest) > 0; {
					n := min(tt.read, len(rest))
					p.Feed(rest[:n])
					rest = rest[n:]
				}
			}
		})
	}
}

// A suspend in the middle of a paste drops what was collected, and what comes after is typed.
func TestInputFeedResetDropsAPasteInProgress(t *testing.T) {
	in := newInput(true)
	feed := newInputFeed(in)
	now := time.Unix(0, 0)
	feed.feed([]byte("\x1b[200~half"), now)
	feed.reset()
	feed.feed([]byte("a"), now)
	if in.Paste() != "" || !in.Pressed('a') {
		t.Errorf("wanted no paste and a typed a after a reset, got %q and pressed=%v", in.Paste(), in.Pressed('a'))
	}
}

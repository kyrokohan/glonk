package engine

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

func TestParseProbeReply(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantKitty bool
		wantDone  bool
	}{
		{"empty", "", false, false},
		{"protocol reply only", "\x1b[?0u", true, false},
		{"protocol reply with flags", "\x1b[?11u", true, false},
		{"both replies", "\x1b[?0u\x1b[?62;4c", true, true},
		{"attributes only", "\x1b[?62;4c", false, true},
		{"attributes without question mark", "\x1b[62;4c", false, true},
		{"junk before the replies", "ab\x1b[?0u\x1b[?6c", true, true},
		{"keystroke before the reply", "w\x1b[?6c", false, true},
		{"key event is not a protocol reply", "\x1b[119u\x1b[?6c", false, true},
		{"other private mode reply", "\x1b[?25h", false, false},
		{"truncated protocol reply", "\x1b[?0", false, false},
		{"truncated attributes reply", "\x1b[?62;4", false, false},
		{"bare escape", "\x1b", false, false},
		{"escape then bracket", "\x1b[", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kitty, done := parseProbeReply([]byte(tt.in))

			if kitty != tt.wantKitty || done != tt.wantDone {
				t.Errorf("wanted kitty=%v done=%v, got kitty=%v done=%v", tt.wantKitty, tt.wantDone, kitty, done)
			}
		})
	}
}

// parseProbeReply is readProbeAnswers as the kitty tests read it: whether the protocol was answered, and the wait is over.
func parseProbeReply(b []byte) (kitty, done bool) {
	a, done := readProbeAnswers(b)
	return a.kitty, done
}

// probeKitty is probe asking the kitty question alone.
func probeKitty(in <-chan []byte, w io.Writer, timeout time.Duration) (bool, []byte, error) {
	a, typed, err := probe(in, w, timeout, true, false)
	return a.kitty, typed, err
}

func feed(chunks ...string) <-chan []byte {
	ch := make(chan []byte, len(chunks))
	for _, c := range chunks {
		ch <- []byte(c)
	}
	return ch
}

func TestProbeKitty(t *testing.T) {
	const wantQueries = "\x1b[?u\x1b[c"

	tests := []struct {
		name   string
		chunks []string
		want   bool
	}{
		{"supported", []string{"\x1b[?0u", "\x1b[?6c"}, true},
		{"supported in one read", []string{"\x1b[?0u\x1b[?6c"}, true},
		{"unsupported", []string{"\x1b[?6c"}, false},
		{"reply split across reads", []string{"\x1b[?", "0u\x1b[?6", "c"}, true},
		{"junk first", []string{"w", "\x1b[?0u\x1b[?6c"}, true},
		{"keystroke between replies", []string{"\x1b[?0u", "w", "\x1b[?6c"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer

			got, _, err := probeKitty(feed(tt.chunks...), &out, time.Second)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
			if out.String() != wantQueries {
				t.Errorf("wanted queries %q, got %q", wantQueries, out.String())
			}
		})
	}
}

func TestProbeKittyTimesOut(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   bool
	}{
		{"never answers", nil, false},
		{"protocol reply but no attributes reply", []string{"\x1b[?0u"}, true},
		{"only a truncated reply", []string{"\x1b[?0"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			got, _, err := probeKitty(feed(tt.chunks...), &bytes.Buffer{}, 30*time.Millisecond)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("wanted %v, got %v", tt.want, got)
			}
			if time.Since(start) < 25*time.Millisecond {
				t.Errorf("wanted the probe to wait out its timeout, returned after %v", time.Since(start))
			}
		})
	}
}

// A normal terminal answers in milliseconds; the probe must not sit out the timeout.
func TestProbeKittyReturnsOnAttributesReply(t *testing.T) {
	ch := make(chan []byte)
	go func() {
		time.Sleep(20 * time.Millisecond)
		ch <- []byte("\x1b[?0u")
		ch <- []byte("\x1b[?6c")
	}()

	start := time.Now()
	got, _, err := probeKitty(ch, &bytes.Buffer{}, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Errorf("wanted supported, got unsupported")
	}
	if time.Since(start) > time.Second {
		t.Errorf("wanted the probe to return when the reply arrived, took %v", time.Since(start))
	}
}

func TestProbeKittyClosedInput(t *testing.T) {
	ch := make(chan []byte)
	close(ch)

	start := time.Now()
	got, _, err := probeKitty(ch, &bytes.Buffer{}, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Errorf("wanted unsupported on closed input, got supported")
	}
	// A closed channel is always ready; looping on it would burn CPU until the timeout.
	if time.Since(start) > time.Second {
		t.Errorf("wanted an immediate return on closed input, took %v", time.Since(start))
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestProbeKittyWriteError(t *testing.T) {
	got, typed, err := probeKitty(feed(), failingWriter{}, time.Second)

	if err == nil {
		t.Errorf("wanted the write error, got nil")
	}
	if got {
		t.Errorf("wanted unsupported on error, got supported")
	}
	if typed != nil {
		t.Errorf("wanted nothing typed on error, got %q", typed)
	}
}

func TestStripProbeReplies(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"protocol reply", "\x1b[?0u", ""},
		{"protocol reply with flags", "\x1b[?11u", ""},
		{"attributes reply", "\x1b[?62;4c", ""},
		{"attributes reply without question mark", "\x1b[62;4c", ""},
		{"both replies", "\x1b[?0u\x1b[?62;4c", ""},
		{"keys before, between and after", "w\x1b[?0u\x1b[?62;4ce", "we"},
		{"key before the replies", "q\x1b[?6c", "q"},
		{"key after the replies", "\x1b[?0u\x1b[?6cx", "x"},
		{"several keys", "ab\x1b[?0ucd\x1b[?6cef", "abcdef"},
		{"escape key after a reply", "\x1b[?0u\x1b", "\x1b"},
		{"alt chord", "\x1bx", "\x1bx"},
		{"control character", "\x03", "\x03"},
		{"multi-byte rune", "é\x1b[?6c→", "é→"},
		{"kitty key event is not a reply", "\x1b[119u", "\x1b[119u"},
		{"kitty release is not a reply", "\x1b[119;1:3u\x1b[?6c", "\x1b[119;1:3u"},
		{"arrow key", "\x1b[A\x1b[?6c", "\x1b[A"},
		{"other private mode", "\x1b[?25h", "\x1b[?25h"},
		{"cut off protocol reply", "\x1b[?0", "\x1b[?0"},
		{"cut off after a key", "ab\x1b[", "ab\x1b["},
		{"lone escape", "\x1b", "\x1b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(stripProbeReplies([]byte(tt.in))); got != tt.want {
				t.Errorf("wanted %q, got %q", tt.want, got)
			}
		})
	}
}

func TestProbeKittyReturnsWhatWasTyped(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   string
	}{
		{"typed before the replies", []string{"w", "\x1b[?0u\x1b[?6c"}, "w"},
		{"typed between the replies", []string{"\x1b[?0u", "w", "\x1b[?6c"}, "w"},
		{"typed in the same read as the replies", []string{"q\x1b[?0u\x1b[?6cz"}, "qz"},
		{"typed after the attributes reply, same read", []string{"\x1b[?6cx"}, "x"},
		{"replies split across reads", []string{"a\x1b[?", "0u\x1b[?6", "cb"}, "ab"},
		{"nothing typed", []string{"\x1b[?0u\x1b[?6c"}, ""},
		{"escape key pressed during the wait", []string{"\x1b", "\x1b[?6c"}, "\x1b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, typed, err := probeKitty(feed(tt.chunks...), &bytes.Buffer{}, time.Second)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(typed) != tt.want {
				t.Errorf("wanted %q, got %q", tt.want, typed)
			}
		})
	}
}

// A terminal that never answers costs the whole wait, and the keys typed in that
// time must survive it.
func TestProbeKittyKeepsKeysWhenNothingAnswers(t *testing.T) {
	_, typed, err := probeKitty(feed("ab", "c"), &bytes.Buffer{}, 30*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(typed) != "abc" {
		t.Errorf("wanted %q, got %q", "abc", typed)
	}
}

func TestProbeKittyKeepsKeysOnClosedInput(t *testing.T) {
	ch := make(chan []byte, 1)
	ch <- []byte("hi")
	close(ch)

	_, typed, err := probeKitty(ch, &bytes.Buffer{}, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(typed) != "hi" {
		t.Errorf("wanted %q, got %q", "hi", typed)
	}
}

// The answers about the mouse: DECRPM for 1016, where 1 to 3 mean the terminal has it, and the size of a cell.
func TestReadProbeAnswersAboutTheMouse(t *testing.T) {
	for _, tt := range []struct {
		name         string
		in           string
		pixels       bool
		cellW, cellH int
		done         bool
	}{
		{"set", "\x1b[?1016;1$y\x1b[?62c", true, 0, 0, true},
		{"reset", "\x1b[?1016;2$y\x1b[?62c", true, 0, 0, true},
		{"permanently set", "\x1b[?1016;3$y\x1b[?62c", true, 0, 0, true},
		{"not recognized", "\x1b[?1016;0$y\x1b[?62c", false, 0, 0, true},
		{"permanently reset", "\x1b[?1016;4$y\x1b[?62c", false, 0, 0, true},
		{"another mode", "\x1b[?1006;1$y\x1b[?62c", false, 0, 0, true},
		{"the cell size", "\x1b[6;34;16t\x1b[?62c", false, 16, 34, true},
		{"a cell size of 0", "\x1b[6;0;16t\x1b[?62c", false, 0, 0, true},
		{"the text area is not the cell", "\x1b[4;1938;1712t\x1b[?62c", false, 0, 0, true},
		{"all of them, as Ghostty answers", "\x1b[?0u\x1b[?1016;2$y\x1b[6;34;16t\x1b[?62;22;52c", true, 16, 34, true},
		{"cut off before the end", "\x1b[?1016;2$y\x1b[6;34;1", true, 0, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, done := readProbeAnswers([]byte(tt.in))
			if a.pixels != tt.pixels || a.cellW != tt.cellW || a.cellH != tt.cellH || done != tt.done {
				t.Errorf("wanted pixels %v, a cell %d x %d and done %v, got %+v and %v", tt.pixels, tt.cellW, tt.cellH, tt.done, a, done)
			}
		})
	}
	if got := string(stripProbeReplies([]byte("a\x1b[?1016;2$yb\x1b[6;34;16tc\x1b[?62cd"))); got != "abcd" {
		t.Errorf("wanted the mouse answers taken out of what was typed, got %q", got)
	}
}

// The probe asks only what it is asked to, and reads the mouse answers split across reads.
func TestProbeAsksWhatItIsAsked(t *testing.T) {
	for _, tt := range []struct {
		name            string
		askKitty, mouse bool
		want            string
	}{
		{"the keyboard", true, false, "\x1b[?u\x1b[c"},
		{"the mouse", false, true, "\x1b[?1016$p\x1b[16t\x1b[c"},
		{"both", true, true, "\x1b[?u\x1b[?1016$p\x1b[16t\x1b[c"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if _, _, err := probe(feed("\x1b[?6c"), &out, time.Second, tt.askKitty, tt.mouse); err != nil {
				t.Fatal(err)
			}
			if out.String() != tt.want {
				t.Errorf("wanted %q asked, got %q", tt.want, out.String())
			}
		})
	}
	a, typed, err := probe(feed("x\x1b[?101", "6;2$y\x1b[6;3", "4;16t\x1b[?62c"), &bytes.Buffer{}, time.Second, false, true)
	if err != nil || !a.pixels || a.cellW != 16 || a.cellH != 34 || string(typed) != "x" {
		t.Errorf("wanted the split answers read and x kept, got %+v, %q, %v", a, typed, err)
	}
}

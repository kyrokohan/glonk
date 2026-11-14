package engine

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"golang.org/x/term"
)

func TestKeyboardSequences(t *testing.T) {
	tests := []struct {
		name  string
		build func(sb *sequenceBuilder)
		want  string
	}{
		{"query protocol", func(sb *sequenceBuilder) { sb.QueryKeyboardProtocol() }, "\x1b[?u"},
		{"query attributes", func(sb *sequenceBuilder) { sb.QueryDeviceAttributes() }, "\x1b[c"},
		{"push flags", func(sb *sequenceBuilder) { sb.PushKeyboardFlags(11) }, "\x1b[>11u"},
		{"push other flags", func(sb *sequenceBuilder) { sb.PushKeyboardFlags(1) }, "\x1b[>1u"},
		{"pop flags", func(sb *sequenceBuilder) { sb.PopKeyboardFlags() }, "\x1b[<u"},
		{"reset style", func(sb *sequenceBuilder) { sb.ResetStyle() }, "\x1b[0m"},
		{"enable focus reporting", func(sb *sequenceBuilder) { sb.EnableFocusReporting() }, "\x1b[?1004h"},
		{"disable focus reporting", func(sb *sequenceBuilder) { sb.DisableFocusReporting() }, "\x1b[?1004l"},
		{"disable autowrap", func(sb *sequenceBuilder) { sb.DisableAutowrap() }, "\x1b[?7l"},
		{"enable autowrap", func(sb *sequenceBuilder) { sb.EnableAutowrap() }, "\x1b[?7h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := newSequenceBuilder()
			tt.build(sb)

			if got := sb.String(); got != tt.want {
				t.Errorf("wanted %q, got %q", tt.want, got)
			}
		})
	}
}

func TestEnableKeyboardProtocol(t *testing.T) {
	var out bytes.Buffer
	tty := newTerminal(os.Stdin, &out)

	if err := tty.EnableKeyboardProtocol(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := "\x1b[>11u\x1b[?1004h"; out.String() != want {
		t.Errorf("wanted %q, got %q", want, out.String())
	}
	if !tty.keyboard {
		t.Errorf("wanted the terminal to remember it must pop the flags, got false")
	}
}

// Even a failed write must be remembered, or the flags could be left pushed.
func TestEnableKeyboardProtocolFailedWrite(t *testing.T) {
	tty := newTerminal(os.Stdin, failingWriter{})

	if err := tty.EnableKeyboardProtocol(); err == nil {
		t.Errorf("wanted the write error, got nil")
	}
	if !tty.keyboard {
		t.Errorf("wanted the terminal to remember it must pop the flags, got false")
	}
}

func TestExitSequence(t *testing.T) {
	tests := []struct {
		name     string
		keyboard bool
		want     string
	}{
		// Autowrap comes back before the alternate screen is left, where it was turned off.
		{"without the protocol", false, "\x1b[?25h\x1b[?7h\x1b[?1049l"},
		// Popped before leaving the alternate screen, where the flags were pushed.
		{"with the protocol", true, "\x1b[<u\x1b[?1004l\x1b[?25h\x1b[?7h\x1b[?1049l"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tty := newTerminal(os.Stdin, &bytes.Buffer{})
			tty.keyboard = tt.keyboard

			if got := tty.exitSequence().String(); got != tt.want {
				t.Errorf("wanted %q, got %q", tt.want, got)
			}
		})
	}
}

func TestTeardownBeforeSetupDoesNothing(t *testing.T) {
	r, _, err := os.Pipe() // not a terminal, so touching it would fail loudly
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	var out bytes.Buffer
	tty := newTerminal(r, &out)
	if err := tty.Teardown(); err != nil || out.Len() != 0 {
		t.Errorf("wanted no error and no output, got err=%v out=%q", err, out.String())
	}
}

func TestTeardownTwiceWritesOnce(t *testing.T) {
	r, _, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	var out bytes.Buffer
	tty := newTerminal(r, &out)
	tty.state = &term.State{} // as if Setup had worked
	tty.keyboard = true

	tty.Teardown() // the restore fails on a pipe; what matters is it is over
	first := out.String()
	if first == "" {
		t.Fatalf("wanted the exit sequence written")
	}
	if tty.state != nil || tty.keyboard {
		t.Errorf("wanted the terminal marked as torn down, got state=%v keyboard=%v", tty.state, tty.keyboard)
	}

	if err := tty.Teardown(); err != nil || out.String() != first {
		t.Errorf("wanted a second Teardown to do nothing, got err=%v and %q more", err, strings.TrimPrefix(out.String(), first))
	}
}

func TestEnterSequence(t *testing.T) {
	tty := newTerminal(os.Stdin, &bytes.Buffer{})

	// The alternate screen first, so autowrap is turned off on it and back on there.
	want := "\x1b[?1049h\x1b[?7l\x1b[2J\x1b[?25l"
	if got := tty.enterSequence().String(); got != want {
		t.Errorf("wanted %q, got %q", want, got)
	}
}

func TestHeadlessTerminalWritesNothingAndKeepsItsSize(t *testing.T) {
	var out bytes.Buffer
	tty := newHeadless(&out, 7, 3)

	for name, err := range map[string]error{
		"Setup":                  tty.Setup(),
		"EnableKeyboardProtocol": tty.EnableKeyboardProtocol(),
		"Teardown":               tty.Teardown(),
	} {
		if err != nil {
			t.Errorf("%s: wanted nil, got %v", name, err)
		}
	}
	if out.Len() != 0 {
		t.Errorf("wanted nothing written, got %q", out.String())
	}
	if w, h, err := tty.Size(); w != 7 || h != 3 || err != nil {
		t.Errorf("wanted 7x3, got %dx%d (err %v)", w, h, err)
	}
}

func TestOnlyARealTerminalCanSuspend(t *testing.T) {
	if newHeadless(io.Discard, 1, 1).canSuspend() {
		t.Errorf("wanted a headless terminal not to suspend")
	}
	if got := newTerminal(os.Stdin, io.Discard).canSuspend(); got != suspendSupported {
		t.Errorf("wanted a terminal to suspend exactly where the system can, got %v", got)
	}
}

func TestSequenceBuilderWritesNumbersInDecimal(t *testing.T) {
	tests := []struct {
		name  string
		build func(sb *sequenceBuilder)
		want  string
	}{
		{"move to the corner", func(sb *sequenceBuilder) { sb.MoveTo(1, 1) }, "\x1b[1;1H"},
		{"move to a normal spot", func(sb *sequenceBuilder) { sb.MoveTo(24, 80) }, "\x1b[24;80H"},
		{"move to a big spot", func(sb *sequenceBuilder) { sb.MoveTo(1000, 20000) }, "\x1b[1000;20000H"},
		{"erase the display", func(sb *sequenceBuilder) { sb.EraseInDisplay(2) }, "\x1b[2J"},
		{"keyboard flags", func(sb *sequenceBuilder) { sb.PushKeyboardFlags(11) }, "\x1b[>11u"},
		{"text and an invalid rune", func(sb *sequenceBuilder) { sb.Print("a"); sb.PrintRune('é'); sb.PrintRune(-1) }, "aé�"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := newSequenceBuilder()
			tt.build(sb)
			if got := sb.String(); got != tt.want {
				t.Errorf("wanted %q, got %q", tt.want, got)
			}
		})
	}
}

func TestSequenceBuilderResetKeepsItsBufferAndStartsOver(t *testing.T) {
	sb := newSequenceBuilder()
	sb.Grow(64)
	sb.MoveTo(10, 10)
	sb.Print("first")
	first := cap(sb.buf)

	sb.reset()
	if sb.Len() != 0 || sb.String() != "" {
		t.Fatalf("wanted an empty builder after reset, got %q", sb.String())
	}
	sb.MoveTo(1, 1)

	if got := sb.String(); got != "\x1b[1;1H" {
		t.Errorf("wanted only what was built after the reset, got %q", got)
	}
	if cap(sb.buf) != first {
		t.Errorf("wanted the buffer kept, capacity went from %d to %d", first, cap(sb.buf))
	}
}

func TestSequenceBuilderFlushIsOneWriteAndLeavesTheBuilderAlone(t *testing.T) {
	sb := newSequenceBuilder()
	sb.Print("hello")
	var out countingWriter

	if err := sb.Flush(&out); err != nil {
		t.Fatal(err)
	}

	if out.writes != 1 || out.bytes != 5 || sb.String() != "hello" {
		t.Errorf("wanted one write of 5 bytes and the builder unchanged, got %d writes of %d bytes and %q", out.writes, out.bytes, sb.String())
	}
}

// tempFile returns an open temporary file, closed when the test ends.
func tempFile(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestSizeAsksTheOutputWhenItIsAFile(t *testing.T) {
	in, out := tempFile(t), tempFile(t)
	if got := newTerminal(in, out).sizeFd; got != int(out.Fd()) {
		t.Errorf("wanted fd %d, got %d", out.Fd(), got)
	}
	if got := newTerminal(in, &bytes.Buffer{}).sizeFd; got != int(in.Fd()) {
		t.Errorf("wanted the input's fd, got %d", got)
	}
}

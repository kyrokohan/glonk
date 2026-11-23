//go:build linux || darwin

package engine

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// probeOn runs Probe on a pseudo-terminal 100 by 30 whose other end sends answer once the question has
// been asked, and returns what Probe said and everything it wrote.
func probeOn(t *testing.T, answer string) (Terminal, string) {
	t.Helper()
	master, slave := openPty(t)
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 100, Row: 30}); err != nil {
		t.Fatalf("giving the terminal its size: %v", err)
	}
	written := collect(master)
	type result struct {
		t   Terminal
		err error
	}
	done := make(chan result, 1)
	go func() {
		got, err := Probe(Config{Input: slave, Output: slave})
		done <- result{got, err}
	}()
	waitFor(t, "the question", func() bool { return strings.Contains(written(), "\x1b[c") })
	if answer != "" {
		if _, err := master.WriteString(answer); err != nil {
			t.Fatalf("answering: %v", err)
		}
	}
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("wanted no error, got %v", r.err)
		}
		return r.t, written()
	case <-time.After(5 * time.Second):
		t.Fatalf("wanted Probe to return within 5 s")
	}
	return Terminal{}, ""
}

// Probe asks the kitty question and device attributes and nothing else, and says what came back.
func TestProbeSaysWhatTheTerminalCanDo(t *testing.T) {
	t.Setenv("COLORTERM", "truecolor")
	got, written := probeOn(t, "\x1b[?0u\x1b[?62c")
	want := Terminal{IsTerminal: true, Width: 100, Height: 30, KeyReleases: true, TrueColor: true}
	if got != want {
		t.Errorf("wanted %+v, got %+v", want, got)
	}
	if written != "\x1b[?u\x1b[c" {
		t.Errorf("wanted only the two questions written, nothing drawn and no alternate screen, got %q", written)
	}
}

// A terminal without the protocol answers device attributes only; one that answers nothing is given up on
// after the probe's wait, which is no error.
func TestProbeWithoutTheProtocol(t *testing.T) {
	t.Setenv("COLORTERM", "")
	t.Setenv("TERM", "xterm-256color")
	for _, tt := range []struct {
		name, answer string
	}{
		{"device attributes only", "\x1b[?62c"},
		{"nothing at all", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := probeOn(t, tt.answer)
			want := Terminal{IsTerminal: true, Width: 100, Height: 30}
			if got != want {
				t.Errorf("wanted %+v, got %+v", want, got)
			}
		})
	}
}

// Probe leaves the terminal's settings as it found them, and a key pressed while it asks is dropped,
// while what is typed after it returns reaches the next reader whole.
func TestProbeLeavesTheTerminalAsItFoundIt(t *testing.T) {
	master, slave := openPty(t)
	before, err := unix.IoctlGetTermios(int(slave.Fd()), getTermios)
	if err != nil {
		t.Fatalf("reading the terminal's settings: %v", err)
	}
	written := collect(master)
	done := make(chan error, 1)
	go func() {
		_, err := Probe(Config{Input: slave, Output: slave})
		done <- err
	}()
	waitFor(t, "the question", func() bool { return strings.Contains(written(), "\x1b[c") })
	if _, err := master.WriteString("x\x1b[?62c"); err != nil {
		t.Fatalf("answering: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("wanted no error, got %v", err)
	}
	// Its reader is gone, so nothing reads the terminal after it. stop returns once the reader's loop is
	// over, a moment before its goroutine leaves, so wait for it to leave.
	waitFor(t, "Probe's reader to leave", func() bool {
		stacks := make([]byte, 1<<20)
		return !strings.Contains(string(stacks[:runtime.Stack(stacks, true)]), "engine.readInput")
	})
	after, err := unix.IoctlGetTermios(int(slave.Fd()), getTermios)
	if err != nil {
		t.Fatalf("reading the terminal's settings again: %v", err)
	}
	if *after != *before {
		t.Errorf("wanted the terminal's settings as they were, got %+v, was %+v", *after, *before)
	}

	// The terminal is back in its line mode, so the next line arrives whole, and the x is not in it.
	if _, err := master.WriteString("y\n"); err != nil {
		t.Fatalf("typing: %v", err)
	}
	chunks, stop := readInput(slave)
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	select {
	case b := <-chunks:
		if string(b) != "y\n" {
			t.Errorf("wanted only the line typed after Probe, got %q", b)
		}
	case <-ctx.Done():
		t.Fatalf("wanted the line typed after Probe to reach the next reader")
	}
}

// Where the input or the output is not a terminal, Probe says so, asks nothing and gives no error.
func TestProbeOfWhatIsNoTerminal(t *testing.T) {
	_, slave := openPty(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	t.Setenv("COLORTERM", "truecolor")
	for _, tt := range []struct {
		name string
		cfg  Config
	}{
		{"input a pipe", Config{Input: r, Output: slave}},
		{"output a pipe", Config{Input: slave, Output: w}},
		{"neither a file", Config{Input: strings.NewReader(""), Output: &strings.Builder{}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Probe(tt.cfg)
			if err != nil || got != (Terminal{}) {
				t.Errorf("wanted a zero Terminal and no error, got %+v and %v", got, err)
			}
		})
	}
}

// The size is the output's, as Run reads it, where the input is another terminal.
func TestProbeTakesTheSizeOfTheOutput(t *testing.T) {
	inMaster, inSlave := openPty(t)
	outMaster, outSlave := openPty(t)
	if err := unix.IoctlSetWinsize(int(outSlave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 100, Row: 30}); err != nil {
		t.Fatalf("giving the output its size: %v", err)
	}
	written := collect(outMaster)
	done := make(chan Terminal, 1)
	go func() {
		got, _ := Probe(Config{Input: inSlave, Output: outSlave})
		done <- got
	}()
	waitFor(t, "the question", func() bool { return strings.Contains(written(), "\x1b[c") })
	if _, err := inMaster.WriteString("\x1b[?62c"); err != nil {
		t.Fatalf("answering: %v", err)
	}
	if got := <-done; got.Width != 100 || got.Height != 30 {
		t.Errorf("wanted the output's 100 by 30, got %d by %d", got.Width, got.Height)
	}
}

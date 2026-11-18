//go:build linux || darwin

package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// sizeGame is a Game that passes on each size Run tells it.
type sizeGame struct{ sizes chan [2]int }

func (g sizeGame) Update(float64, *Input) error { return nil }
func (g sizeGame) Draw(*FrameBuffer)            {}

// Resize drops a size nobody is waiting for, so that it never holds Run up.
func (g sizeGame) Resize(w, h int) {
	select {
	case g.sizes <- [2]int{w, h}:
	default:
	}
}

// A pseudo-terminal that nobody gave a size reports 0 by 0, as under script started from a
// CI step. Run must start on it, with the size it falls back to, and follow the real size
// once there is one.
func TestRunOnATerminalThatReportsNoSize(t *testing.T) {
	master, slave := openPty(t)
	go io.Copy(io.Discard, master) // the frames and the probe: nobody reads them, and a full buffer would stop Run

	if w, h, err := term.GetSize(int(slave.Fd())); err != nil || w != 0 || h != 0 {
		t.Fatalf("wanted a terminal that reports 0 by 0, got %d by %d (%v)", w, h, err)
	}

	game := sizeGame{sizes: make(chan [2]int, 16)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, game, Config{FPS: 60, Input: slave, Output: slave}) }()

	next := func() [2]int {
		t.Helper()
		select {
		case size := <-game.sizes:
			return size
		case err := <-done:
			t.Fatalf("wanted Run to tell the game its size, but it ended with %v", err)
		case <-time.After(5 * time.Second):
			t.Fatalf("wanted Run to tell the game its size within 5 s")
		}
		return [2]int{}
	}

	if got := next(); got != [2]int{fallbackWidth, fallbackHeight} {
		t.Fatalf("wanted the fallback size %d by %d, got %d by %d", fallbackWidth, fallbackHeight, got[0], got[1])
	}

	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 40, Col: 120}); err != nil {
		t.Fatalf("giving the terminal a size: %v", err)
	}
	if got := next(); got != [2]int{120, 40} {
		t.Errorf("wanted the terminal's own size, 120 by 40, once it reports one, got %d by %d", got[0], got[1])
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("wanted Run to end with context.Canceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("wanted Run to end within 5 s of its context being canceled")
	}
}

// On a real terminal Run asks for bracketed paste as it takes the terminal over, reads a paste as one
// string, and turns the mode off again before it gives the terminal back.
func TestRunAsksForBracketedPasteAndReadsIt(t *testing.T) {
	master, slave := openPty(t)
	var mu sync.Mutex
	var out bytes.Buffer
	go func() { // what Run writes: the modes and the frames
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			mu.Lock()
			out.Write(buf[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	written := func() string {
		mu.Lock()
		defer mu.Unlock()
		return out.String()
	}

	var got atomic.Value
	game := scriptedGame{update: func(_ float64, in *Input) error {
		if p := in.Paste(); p != "" {
			got.Store(p + "|" + fmt.Sprint(len(in.KeyEvents())))
		}
		return nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, game, Config{FPS: 60, Width: 20, Height: 5, Input: slave, Output: slave}) }()

	waitFor(t, "the terminal to be asked for bracketed paste", func() bool { return strings.Contains(written(), "\x1b[?2004h") })
	if _, err := master.WriteString("\x1b[200~pasted\rtext\x1b[201~"); err != nil {
		t.Fatalf("pasting: %v", err)
	}
	waitFor(t, "the paste to reach the game", func() bool { return got.Load() != nil })
	if v := got.Load().(string); v != "pasted\ntext|0" {
		t.Errorf("wanted the paste as one string and no key event, got %q", v)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("wanted Run to end within 5 s of its context being canceled")
	}
	// Run has written it all, but the goroutine above may not have read it yet: the alternate screen is the last
	// thing the exit sequence leaves, so once it is read, what comes before it is too.
	waitFor(t, "the alternate screen to be left", func() bool { return strings.Contains(written(), "\x1b[?1049l") })
	all := written()
	on, off := strings.LastIndex(all, "\x1b[?2004h"), strings.LastIndex(all, "\x1b[?2004l")
	if off < on || off < 0 {
		t.Errorf("wanted bracketed paste turned off after it was turned on, got on at %d and off at %d", on, off)
	}
	if main := strings.LastIndex(all, "\x1b[?1049l"); off > main {
		t.Errorf("wanted it turned off before the alternate screen is left")
	}
}

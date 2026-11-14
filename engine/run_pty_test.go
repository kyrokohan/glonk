//go:build linux || darwin

package engine

import (
	"context"
	"errors"
	"io"
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

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

// sizeGame is a Game that passes on each size Run tells it, and draws "glonk" ending in the last
// cell of the frame, which reaches the terminal only if the whole frame is sent.
type sizeGame struct{ sizes chan [2]int }

func (g sizeGame) Update(float64, *Input) error { return nil }
func (g sizeGame) Draw(fb *FrameBuffer)         { fb.DrawText(fb.Width()-5, fb.Height()-1, "glonk", Color{}) }

// Resize drops a size nobody is waiting for, so that it never holds Run up.
func (g sizeGame) Resize(w, h int) {
	select {
	case g.sizes <- [2]int{w, h}:
	default:
	}
}

// A pseudo-terminal that nobody gave a size reports 0 by 0, as under script started from a
// CI step. Run must start on it with the size it falls back to on each axis the terminal
// reports as 0, or the one Config chose, send the whole frame, and follow the real size once
// there is one.
func TestRunOnATerminalThatReportsNoSize(t *testing.T) {
	tests := []struct {
		name         string
		cols, rows   uint16 // what the terminal reports
		width        int    // Config.Width
		want, resize [2]int // what the game is told, at the start and at 120 by 40
	}{
		{"no size", 0, 0, 0, [2]int{fallbackWidth, fallbackHeight}, [2]int{120, 40}},
		{"no columns", 0, 30, 0, [2]int{fallbackWidth, 30}, [2]int{120, 40}},
		{"no rows", 100, 0, 0, [2]int{100, fallbackHeight}, [2]int{120, 40}},
		{"no size and a width chosen", 0, 0, 100, [2]int{100, fallbackHeight}, [2]int{100, 40}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			master, slave := openPty(t)
			if tt.cols != 0 || tt.rows != 0 {
				if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: tt.rows, Col: tt.cols}); err != nil {
					t.Fatalf("giving the terminal its size: %v", err)
				}
			}
			written := collect(master) // a full buffer would stop Run
			if w, h, err := term.GetSize(int(slave.Fd())); err != nil || w != int(tt.cols) || h != int(tt.rows) {
				t.Fatalf("wanted a terminal that reports %d by %d, got %d by %d (%v)", tt.cols, tt.rows, w, h, err)
			}

			game := sizeGame{sizes: make(chan [2]int, 16)}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- Run(ctx, game, Config{FPS: 60, Width: tt.width, Input: slave, Output: slave}) }()

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

			if got := next(); got != tt.want {
				t.Fatalf("wanted the game told %d by %d, got %d by %d", tt.want[0], tt.want[1], got[0], got[1])
			}
			waitFor(t, "the last cell of the frame to be sent", func() bool { return strings.Contains(written(), "glonk") })

			if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 40, Col: 120}); err != nil {
				t.Fatalf("giving the terminal a size: %v", err)
			}
			if got := next(); got != tt.resize {
				t.Errorf("wanted %d by %d once the terminal reports 120 by 40, got %d by %d", tt.resize[0], tt.resize[1], got[0], got[1])
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
		})
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

// collect reads everything written to a pty, on its own goroutine, and returns what has been read so far.
func collect(r io.Reader) func() string {
	var mu sync.Mutex
	var out bytes.Buffer
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			mu.Lock()
			out.Write(buf[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		return out.String()
	}
}

// Run asks for the mouse only when the program asked, reads the reports a terminal sends, and turns the mouse
// off before it gives the terminal back.
func TestRunAsksForTheMouseOnlyWhenConfiguredAndReadsIt(t *testing.T) {
	for _, mouse := range []bool{false, true} {
		t.Run(fmt.Sprintf("mouse=%v", mouse), func(t *testing.T) {
			master, slave := openPty(t)
			written := collect(master)

			var last atomic.Value
			var wheel atomic.Int64
			game := scriptedGame{update: func(_ float64, in *Input) error {
				if x, y, ok := in.MousePos(); ok && in.MouseStarted(MouseLeft) {
					last.Store(fmt.Sprintf("%d,%d", x, y))
				}
				_, dy := in.Wheel()
				wheel.Add(int64(dy))
				return nil
			}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- Run(ctx, game, Config{FPS: 60, Width: 20, Height: 5, Input: slave, Output: slave, Mouse: mouse})
			}()

			waitFor(t, "the terminal to be taken over", func() bool { return strings.Contains(written(), "\x1b[?2004h") })
			// The reports are read whether or not they were asked for.
			if _, err := master.WriteString(sgr(35, 2, 2, 'M') + sgr(0, 5, 3, 'M') + sgr(65, 5, 3, 'M')); err != nil {
				t.Fatalf("sending the reports: %v", err)
			}
			waitFor(t, "the click and the wheel to reach the game", func() bool { return last.Load() != nil && wheel.Load() == 1 })
			if got := last.Load().(string); got != "4,2" {
				t.Errorf("wanted the click at 4,2, got %q", got)
			}

			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatalf("wanted Run to end within 5 s of its context being canceled")
			}
			waitFor(t, "the alternate screen to be left", func() bool { return strings.Contains(written(), "\x1b[?1049l") })
			all := written()

			if got := strings.Contains(all, "\x1b[?1003h\x1b[?1006h"); got != mouse {
				t.Errorf("wanted the mouse asked for: %v, got %v", mouse, got)
			}
			if got := strings.Contains(all, "\x1b[?1004h"); got != mouse {
				t.Errorf("wanted focus reported with the mouse and not without it (there is no keyboard protocol here): %v, got %v", mouse, got)
			}
			// Turned off in the reverse order, before the alternate screen is left.
			off := "\x1b[?1004l\x1b[?1006l\x1b[?1003l\x1b[?2004l"
			if got := strings.Contains(all, off); got != mouse {
				t.Errorf("wanted the mouse turned off in reverse order (%q): %v, got %v", off, mouse, got)
			}
			if mouse && strings.LastIndex(all, off) > strings.LastIndex(all, "\x1b[?1049l") {
				t.Errorf("wanted the mouse turned off before the alternate screen is left")
			}
		})
	}
}

package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// scriptedGame is a Game made of two functions, for driving Run in tests.
type scriptedGame struct {
	update func(dt float64, in *Input) error
	draw   func(fb *FrameBuffer)
}

func (g scriptedGame) Update(dt float64, in *Input) error {
	if g.update == nil {
		return nil
	}
	return g.update(dt, in)
}

func (g scriptedGame) Draw(fb *FrameBuffer) {
	if g.draw != nil {
		g.draw(fb)
	}
}

// startHeadless runs game on a 3x2 frame at the highest frame rate unless cfg says
// otherwise, and delivers Run's result on the returned channel.
func startHeadless(ctx context.Context, game Game, cfg Config) <-chan error {
	if cfg.FPS == 0 {
		cfg.FPS = maxFPS
	}
	if cfg.Width == 0 && cfg.Height == 0 {
		cfg.Width, cfg.Height = 3, 2
	}
	done := make(chan error, 1)
	go func() { done <- Run(ctx, game, cfg) }()
	return done
}

// finished waits for a run to end, failing the test instead of hanging when it does not.
func finished(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatalf("wanted Run to return, but it was still going after 3s")
		return nil
	}
}

// runHeadless runs game to its end and returns what Run returned.
func runHeadless(t *testing.T, game Game, cfg Config) error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return finished(t, startHeadless(ctx, game, cfg))
}

// waitFor polls until cond holds, failing the test if it does not within two seconds.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("wanted %s, but it never happened", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// frameOf is the bytes Run writes for one frame of the given size after draw.
func frameOf(t *testing.T, w, h int, draw func(*FrameBuffer)) string {
	t.Helper()
	fb, err := NewFrameBuffer(w, h)
	if err != nil {
		t.Fatal(err)
	}
	draw(fb)
	var b bytes.Buffer
	if err := fb.presentClipped(&b, w, h); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestRunHeadlessWritesNothingButFrames(t *testing.T) {
	draw := func(fb *FrameBuffer) { fb.Set(1, 0, Cell{Character: 'X', Foreground: Palette(208)}) }
	updates, draws := 0, 0
	game := scriptedGame{
		update: func(float64, *Input) error {
			updates++
			if updates == 4 {
				return ErrQuit
			}
			return nil
		},
		draw: func(fb *FrameBuffer) { draws++; draw(fb) },
	}

	var out bytes.Buffer
	err := runHeadless(t, game, Config{Input: strings.NewReader(""), Output: &out})

	if err != nil {
		t.Fatalf("wanted nil, got %v", err)
	}
	if draws != 3 {
		t.Fatalf("wanted 3 frames drawn, got %d", draws)
	}
	// Equal to the frames alone: no screen or mode sequences, no keyboard probe, no clear.
	want := strings.Repeat(frameOf(t, 3, 2, draw), draws)
	if out.String() != want {
		t.Errorf("wanted only the frames\n%q\ngot\n%q", want, out.String())
	}
}

func TestRunHeadlessNeedsASize(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		wantNoTerm    bool
	}{
		{"neither", 0, 0, true},
		{"width only", 5, 0, true},
		{"height only", 0, 5, true},
		{"negative width is the frame's error", -1, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			game := scriptedGame{update: func(float64, *Input) error { called = true; return ErrQuit }}
			var out bytes.Buffer
			cfg := Config{FPS: 30, Width: tt.width, Height: tt.height, Input: strings.NewReader(""), Output: &out}

			err := Run(context.Background(), game, cfg)

			if err == nil {
				t.Fatalf("wanted an error, got nil")
			}
			if got := errors.Is(err, errNoTerminal); got != tt.wantNoTerm {
				t.Errorf("wanted errors.Is(err, errNoTerminal) = %v, got %v: %v", tt.wantNoTerm, got, err)
			}
			if called || out.Len() != 0 {
				t.Errorf("wanted the game and the output left alone, got called=%v out=%q", called, out.String())
			}
		})
	}
}

func TestRunHeadlessEnds(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		input   string
		update  func(float64, *Input) error
		wantErr error
	}{
		{"update asks to quit", "", func(float64, *Input) error { return ErrQuit }, nil},
		{"ctrl-c in the input", "\x03", func(float64, *Input) error { return nil }, nil},
		{"kitty ctrl-c in the input", "\x1b[99;5u", func(float64, *Input) error { return nil }, nil},
		{"update fails", "", func(float64, *Input) error { return boom }, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := runHeadless(t, scriptedGame{update: tt.update}, Config{Input: strings.NewReader(tt.input), Output: &out})

			if !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Errorf("wanted %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestRunHeadlessReturnsAFailedWrite(t *testing.T) {
	boom := errors.New("disk full")

	err := runHeadless(t, scriptedGame{}, Config{Input: strings.NewReader(""), Output: errWriter{boom}})

	if !errors.Is(err, boom) {
		t.Errorf("wanted %v, got %v", boom, err)
	}
}

func TestRunHeadlessKeepsGoingAtTheEndOfInput(t *testing.T) {
	updates := 0
	game := scriptedGame{update: func(float64, *Input) error {
		updates++
		if updates == 20 {
			return ErrQuit
		}
		return nil
	}}

	err := runHeadless(t, game, Config{Input: strings.NewReader("a"), Output: io.Discard})

	if err != nil || updates != 20 {
		t.Errorf("wanted the game to run to its own end, got err=%v after %d updates", err, updates)
	}
}

func TestRunHeadlessReadsExactKeys(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	var down, lapsed atomic.Bool
	seen := false
	game := scriptedGame{update: func(_ float64, in *Input) error {
		d := in.IsDown('w')
		seen = seen || d
		if seen && !d {
			lapsed.Store(true)
		}
		down.Store(d)
		return nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startHeadless(ctx, game, Config{Input: pr, Output: io.Discard})

	pw.Write([]byte("\x1b[119u"))
	waitFor(t, "w to be down", down.Load)
	// A timing guess would let go of w after about 150ms with no repeat.
	time.Sleep(2 * holdTimeout)
	if lapsed.Load() {
		t.Errorf("wanted w held until its release arrives")
	}
	pw.Write([]byte("\x1b[119;1:3u"))
	waitFor(t, "w to be up after its release", func() bool { return !down.Load() })

	cancel()
	finished(t, done)
}

// A headless run reads keys as the protocol reports them, so its Input is exact, unless
// LegacyKeys asks for the timing guess.
func TestRunHeadlessSaysWhetherKeysAreExact(t *testing.T) {
	tests := []struct {
		name       string
		legacyKeys bool
		want       bool
	}{
		{"by default", false, true},
		{"with LegacyKeys", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			game := scriptedGame{update: func(_ float64, in *Input) error {
				got = in.Exact()
				return ErrQuit
			}}

			err := runHeadless(t, game, Config{LegacyKeys: tt.legacyKeys, Input: strings.NewReader(""), Output: io.Discard})

			if err != nil || got != tt.want {
				t.Errorf("wanted Exact()=%v and no error, got %v and %v", tt.want, got, err)
			}
		})
	}
}

func TestRunHeadlessLegacyKeysTimeOut(t *testing.T) {
	seen := false
	game := scriptedGame{update: func(_ float64, in *Input) error {
		d := in.IsDown('w')
		seen = seen || d
		if seen && !d {
			return ErrQuit
		}
		return nil
	}}

	// No release ever comes for a plain letter; only the timing lets go of it.
	err := runHeadless(t, game, Config{LegacyKeys: true, Input: strings.NewReader("w"), Output: io.Discard})

	if err != nil || !seen {
		t.Errorf("wanted w down and then up again, got err=%v seen=%v", err, seen)
	}
}

func TestRunHeadlessDeliversAPlainKeyAsAPress(t *testing.T) {
	pressed := false
	game := scriptedGame{update: func(_ float64, in *Input) error {
		if in.Pressed('w') {
			pressed = true
			return ErrQuit
		}
		return nil
	}}

	err := runHeadless(t, game, Config{Input: strings.NewReader("w"), Output: io.Discard})

	if err != nil || !pressed {
		t.Errorf("wanted a press of w, got err=%v pressed=%v", err, pressed)
	}
}

// sizedGame records the calls Run makes, in order.
type sizedGame struct {
	scriptedGame
	calls []string
}

func (g *sizedGame) Resize(w, h int) { g.calls = append(g.calls, fmt.Sprintf("resize %dx%d", w, h)) }

func TestRunTellsTheGameItsSizeBeforeTheFirstUpdate(t *testing.T) {
	g := &sizedGame{}
	updates := 0
	g.update = func(float64, *Input) error {
		g.calls = append(g.calls, "update")
		updates++
		if updates == 3 {
			return ErrQuit
		}
		return nil
	}
	g.draw = func(*FrameBuffer) { g.calls = append(g.calls, "draw") }

	err := runHeadless(t, g, Config{Width: 5, Height: 4, Input: strings.NewReader(""), Output: io.Discard})

	if err != nil {
		t.Fatalf("wanted nil, got %v", err)
	}
	// One Resize, because a fixed frame never changes size, and before everything else.
	want := "resize 5x4,update,draw,update,draw,update"
	if got := strings.Join(g.calls, ","); got != want {
		t.Errorf("wanted calls\n%s\ngot\n%s", want, got)
	}
}

func TestRunReturnsTheContextErrorThatEndedIt(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		updates := 0
		game := scriptedGame{update: func(float64, *Input) error {
			if updates++; updates == 3 {
				cancel()
			}
			return nil
		}}

		err := finished(t, startHeadless(ctx, game, Config{Input: strings.NewReader(""), Output: io.Discard}))

		if err != context.Canceled {
			t.Errorf("wanted context.Canceled, got %v", err)
		}
	})

	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()

		err := finished(t, startHeadless(ctx, scriptedGame{}, Config{Input: strings.NewReader(""), Output: io.Discard}))

		if err != context.DeadlineExceeded {
			t.Errorf("wanted context.DeadlineExceeded, got %v", err)
		}
	})

	t.Run("the game quitting is not an error even with a context that is still live", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		game := scriptedGame{update: func(float64, *Input) error { return ErrQuit }}

		err := finished(t, startHeadless(ctx, game, Config{Input: strings.NewReader(""), Output: io.Discard}))

		if err != nil {
			t.Errorf("wanted nil, got %v", err)
		}
	})
}

func TestRunWithAContextAlreadyDoneDoesNothing(t *testing.T) {
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"cancelled", cancelled, context.Canceled},
		{"past its deadline", expired, context.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &sizedGame{}
			g.update = func(float64, *Input) error { g.calls = append(g.calls, "update"); return nil }
			g.draw = func(*FrameBuffer) { g.calls = append(g.calls, "draw") }
			var out bytes.Buffer

			err := Run(tt.ctx, g, Config{FPS: 30, Width: 3, Height: 2, Input: strings.NewReader(""), Output: &out})

			if err != tt.want {
				t.Errorf("wanted %v, got %v", tt.want, err)
			}
			if len(g.calls) != 0 || out.Len() != 0 {
				t.Errorf("wanted the game and the output left alone, got calls=%v out=%q", g.calls, out.String())
			}
		})
	}
}

func TestRunStartsNoFrameOnceTheContextEnds(t *testing.T) {
	// With a tick already waiting when the context ends, select chooses between
	// them at random, so run it enough times that a missing check cannot hide.
	for range 20 {
		ctx, cancel := context.WithCancel(context.Background())
		updates, draws := 0, 0
		game := scriptedGame{
			update: func(float64, *Input) error {
				if updates++; updates == 3 {
					cancel()
					time.Sleep(3 * time.Millisecond) // longer than a frame at maxFPS
				}
				return nil
			},
			draw: func(*FrameBuffer) { draws++ },
		}

		err := finished(t, startHeadless(ctx, game, Config{Input: strings.NewReader(""), Output: io.Discard}))
		cancel()

		if err != context.Canceled {
			t.Fatalf("wanted context.Canceled, got %v", err)
		}
		// The frame under way finishes, and no other starts.
		if updates != 3 || draws != 3 {
			t.Fatalf("wanted 3 updates and 3 draws, got %d and %d", updates, draws)
		}
	}
}

func TestRunNeverCallsTheGameOnceItHasReturned(t *testing.T) {
	var calls atomic.Int64
	game := &sizedGame{}
	game.update = func(float64, *Input) error {
		if calls.Add(1) == 5 {
			return ErrQuit
		}
		return nil
	}
	game.draw = func(*FrameBuffer) { calls.Add(1) }

	err := runHeadless(t, game, Config{Input: strings.NewReader(""), Output: io.Discard})
	atReturn := calls.Load()
	time.Sleep(30 * time.Millisecond) // thirty frames at maxFPS

	if err != nil || calls.Load() != atReturn {
		t.Errorf("wanted nil and no calls after Run returned, got err=%v and %d more calls", err, calls.Load()-atReturn)
	}
}

// Without a terminal what is written depends on Config and nothing else: not on the
// machine's COLORTERM.
func TestRunHeadlessColors(t *testing.T) {
	orange := RGB(255, 128, 0)
	game := scriptedGame{
		update: func(float64, *Input) error { return nil },
		draw:   func(fb *FrameBuffer) { fb.Set(0, 0, Cell{Character: 'x', Foreground: orange}) },
	}
	tests := []struct {
		name      string
		mode      ColorMode
		colorterm string
		want      string
		notWant   string
	}{
		{"auto ignores COLORTERM", ColorsAuto, "truecolor", "\x1b[38;5;208m", "38;2"},
		{"auto without COLORTERM", ColorsAuto, "", "\x1b[38;5;208m", "38;2"},
		{"256 colors", Colors256, "truecolor", "\x1b[38;5;208m", "38;2"},
		{"true color without COLORTERM", ColorsTrue, "", "\x1b[38;2;255;128;0m", "38;5"},
		{"true color with COLORTERM", ColorsTrue, "truecolor", "\x1b[38;2;255;128;0m", "38;5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COLORTERM", tt.colorterm)
			var out bytes.Buffer
			count := 0
			g := scriptedGame{update: func(float64, *Input) error {
				if count++; count == 2 {
					return ErrQuit
				}
				return nil
			}, draw: game.draw}

			err := runHeadless(t, g, Config{Colors: tt.mode, Input: strings.NewReader(""), Output: &out})

			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tt.want) || strings.Contains(out.String(), tt.notWant) {
				t.Errorf("wanted %q and no %q, got %q", tt.want, tt.notWant, out.String())
			}
		})
	}
}

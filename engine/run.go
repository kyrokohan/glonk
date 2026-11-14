package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/term"
)

// maxDT caps the time step Update sees, in seconds. After a stall (the machine
// sleeping, or the process stopped from another shell) the real gap can be
// huge, and one giant step would fling everything across the world.
const maxDT = 0.25

// probeTimeout is how long Run waits for a terminal to answer the keyboard
// protocol probe. Terminals answer in milliseconds; this only matters when one never does.
const probeTimeout = 250 * time.Millisecond

// maxFPS is the highest frame rate Run accepts. A terminal cannot show more, and the
// frame timer needs an interval of at least a nanosecond.
const maxFPS = 1000

// Config sets up Run. A Width or Height of 0 fills the terminal on that axis
// and follows resizes; a nonzero value fixes that axis, and where the terminal is
// smaller than the frame only the part that fits is shown, from the top left. A
// terminal that reports no size, as a pseudo-terminal nobody gave one does, counts as
// 80 columns by 24 rows until it reports one. FPS must be between 1 and 1000.
//
// Where the terminal supports the kitty keyboard protocol, Run uses it so Input
// can tell exactly which keys are down. LegacyKeys turns that off and uses the
// timing-based handling every terminal supports, as an escape hatch for a
// terminal that misbehaves.
type Config struct {
	// Width and Height are the size of the frame in cells. 0 fills the terminal on
	// that axis.
	Width, Height int

	// FPS is how many times a second Run calls Update and Draw, from 1 to 1000.
	FPS int

	// LegacyKeys turns off the kitty keyboard protocol even where the terminal has
	// it, and uses the timing-based key handling that every terminal supports.
	LegacyKeys bool

	// Colors says whether an RGB color is sent exactly or as the nearest palette
	// color. The zero value, ColorsAuto, sends it exactly when the environment says the
	// terminal can show 24-bit colors (COLORTERM=truecolor, which terminals that can
	// set themselves), and rounded otherwise. Set ColorsTrue where the terminal can
	// but the environment does not say so, inside tmux or over ssh, say. A Palette
	// color is always sent as the palette color, so it follows the user's theme.
	Colors ColorMode

	// Input is where keys are read from; nil means os.Stdin.
	//
	// When Input and Output are both terminals, Run takes them over: raw mode, the
	// alternate screen, the keyboard protocol, quit signals and Ctrl-Z. Opening
	// /dev/tty for both lets a program draw on the terminal while its stdin is a
	// pipe:
	//
	//	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	//	if err != nil {
	//		return err
	//	}
	//	defer tty.Close()
	//	err = engine.Run(ctx, game, engine.Config{FPS: 30, Input: tty, Output: tty})
	//
	// When either is not a terminal, Run runs headless: see Output.
	Input io.Reader

	// Output is where frames are written; nil means os.Stdout.
	//
	// Run runs headless when Input or Output is not a terminal. It then needs a
	// nonzero Width and Height, writes nothing but the frames (no screen or mode
	// sequences), and leaves terminal settings and signals alone. Input is still
	// read as keys; a key is held from its press until its release, as the kitty
	// keyboard protocol reports them, unless LegacyKeys asks for the timing-based
	// handling. Ctrl-C ends Run as usual, Ctrl-Z does nothing, and the end of Input
	// does not end Run. That makes Run testable with a string of key bytes in and a
	// buffer out.
	Output io.Writer
}

// checkConfig rejects settings Run cannot honor.
func checkConfig(cfg Config) error {
	if cfg.FPS < 1 || cfg.FPS > maxFPS {
		return fmt.Errorf("fps must be between 1 and %d, got %d", maxFPS, cfg.FPS)
	}
	if cfg.Colors > ColorsTrue {
		return fmt.Errorf("colors must be ColorsAuto, Colors256 or ColorsTrue, got %d", cfg.Colors)
	}
	return nil
}

// errNoTerminal is what Run returns when input and output are not both terminals
// and the frame size was not given.
var errNoTerminal = errors.New("input and output are not both terminals: set Width and Height to run without one")

// openTerminal picks how Run reaches the terminal: the real one when input and
// output are both terminals, otherwise a headless one of the configured size. It
// returns the reader to take keys from.
func openTerminal(cfg Config) (*terminal, io.Reader, error) {
	var in io.Reader = os.Stdin
	if cfg.Input != nil {
		in = cfg.Input
	}
	var out io.Writer = os.Stdout
	if cfg.Output != nil {
		out = cfg.Output
	}

	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if inOK && outOK && term.IsTerminal(int(inFile.Fd())) && term.IsTerminal(int(outFile.Fd())) {
		return newTerminal(inFile, out), in, nil
	}
	if cfg.Width == 0 || cfg.Height == 0 {
		return nil, nil, errNoTerminal
	}
	return newHeadless(out, cfg.Width, cfg.Height), in, nil
}

// frameDT returns the seconds between two frames, capped at maxDT.
func frameDT(now, last time.Time) float64 {
	dt := now.Sub(last).Seconds()
	if dt > maxDT {
		dt = maxDT
	}
	return dt
}

// A terminal that reports no size is taken to be this big until it reports one. A
// pseudo-terminal that nobody gave a size reports 0 by 0, as under script started from
// a CI step or in some container consoles, and 80 by 24 is what such a terminal is
// conventionally assumed to be.
const (
	fallbackWidth  = 80
	fallbackHeight = 24
)

// resolveSize picks the frame size: a nonzero config value wins on its axis, otherwise
// the terminal's, or the fallback where the terminal reports 0. Negatives pass through
// so NewFrameBuffer rejects them.
func resolveSize(cfgW, cfgH, termW, termH int) (int, int) {
	resW := termW
	if cfgW != 0 {
		resW = cfgW
	} else if termW == 0 {
		resW = fallbackWidth
	}

	resH := termH
	if cfgH != 0 {
		resH = cfgH
	} else if termH == 0 {
		resH = fallbackHeight
	}

	return resW, resH
}

// finish adds a failure to restore the terminal to err, the result of the run.
func finish(err error, tty *terminal) error {
	if restoreErr := tty.Teardown(); restoreErr != nil {
		return errors.Join(err, fmt.Errorf("restoring the terminal: %w", restoreErr))
	}
	return err
}

// handle carries out what the input asked for. done says Run should now return err.
func handle(action inputAction, suspend func() error) (done bool, err error) {
	switch action {
	case actionQuit:
		return true, nil
	case actionSuspend:
		if err := suspend(); err != nil {
			return true, err
		}
	}
	return false, nil
}

// Run takes over the terminal and calls game.Update and game.Draw cfg.FPS times
// a second, printing each frame. The terminal is restored when Run returns, and
// nothing reads it afterwards, so the program can read its own input once Run is
// over.
//
// # Calls to the game
//
// Resize (if the game has one), Update and Draw are called from the goroutine that
// called Run, one at a time, and never after Run has returned. Run first calls
// Resize with the frame's size, if the game has one; each frame then calls Resize
// if the size changed, then Update, then Draw. Once ctx is done or a quit signal
// arrives, no new frame starts; a frame already under way finishes, because Run
// never interrupts a call into the game.
//
// # What is sent
//
// To a terminal, Run sends only the cells that changed since the previous frame, as
// one synchronized update, which terminals that know the mode show all at once and
// the rest ignore. A screen that does not change sends nothing. Everything is drawn
// again after the window changes size, after Ctrl-Z, after a failed write, and when
// the terminal reports that its window came back to the front.
//
// Because only changes are sent, text that something else writes to the terminal
// while Run is running (a Println, a log line) is not wiped by the next frame: it
// stays until those cells change or the screen is drawn again. Log to a file instead.
// Without a terminal, see below, every frame is written whole.
//
// # How Run ends
//
// Run returns nil when the player ends it: Update returns ErrQuit, the user
// presses Ctrl-C, or the process receives SIGINT, SIGTERM or SIGHUP. It returns
// ctx.Err(), which is context.Canceled or context.DeadlineExceeded, when ctx ends
// it; if ctx is done before Run starts, it returns that at once and neither
// touches the terminal nor calls the game. Any other Update error is returned as it
// is. A panic in the game restores the terminal first and then carries on up the
// stack. If the terminal cannot be restored, Run returns that error too, joined to
// the one that ended the run, if there was one.
//
// # Signals
//
// Those signals need no handler of your own. After the first one Run stops
// catching them, so a second takes Go's default action and ends a game stuck in
// Update. Catching them yourself (signal.Notify, signal.NotifyContext) keeps that
// from working. A signal the process was started ignoring (nohup does this to
// SIGHUP) stays ignored.
//
// # Ctrl-Z
//
// Ctrl-Z suspends the program: the terminal is restored, the process stops until
// the shell continues it (fg), and Run takes the terminal over again. A stop sent
// from outside (kill -TSTP) is not caught, so it leaves the terminal as it is.
// Windows has no suspend.
//
// # Without a terminal
//
// With a Config.Input or Config.Output that is not a terminal, Run runs headless:
// the frame size must be given, and what is said here about the terminal, signals
// and Ctrl-Z does not apply. See Config.Output.
//
// # What cannot be restored
//
// Run cannot restore the terminal if the process ends without returning from it:
// os.Exit or log.Fatal called by the game, a panic in a goroutine of your own,
// SIGKILL, or SIGQUIT (which keeps Go's stack dump). Typing reset repairs the
// terminal.
func Run(ctx context.Context, game Game, cfg Config) (err error) {
	if err := checkConfig(cfg); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	tty, src, err := openTerminal(cfg)
	if err != nil {
		return err
	}
	termW, termH, err := tty.Size()
	if err != nil {
		return err
	}

	resW, resH := resolveSize(cfg.Width, cfg.Height, termW, termH)

	nfb, err := NewFrameBuffer(resW, resH)
	if err != nil {
		return err
	}

	// From here a quit signal must reach the deferred Teardown, not kill the process
	// with the terminal still taken over.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if !tty.headless { // signals belong to the process, which a headless Run does not own
		defer notifyQuit(cancel)()
	}

	err = tty.Setup()
	if err != nil {
		return err
	}
	defer func() { err = finish(err, tty) }()
	input, stopInput := readFrom(src)
	defer stopInput() // before Teardown: nothing may read the terminal once Run is over

	// A headless input is read as the protocol reports keys: there is no terminal to ask.
	exact := tty.headless && !cfg.LegacyKeys
	var typed []byte // keys pressed while the probe waited
	if !tty.headless && !cfg.LegacyKeys {
		kitty, rest, err := probeKitty(input, tty.out, probeTimeout)
		if err != nil {
			return err
		}
		typed = rest
		if kitty {
			if err := tty.EnableKeyboardProtocol(); err != nil {
				return err
			}
			exact = true
		}
	}

	ticker := time.NewTicker(time.Second / time.Duration(cfg.FPS))
	defer ticker.Stop()

	last := time.Now()

	in := newInput(exact)
	keys := newInputFeed(in)
	keys.canSuspend = tty.canSuspend()
	defer keys.stop()

	// Only the cells that changed are sent to a terminal; to anything else every
	// frame goes whole.
	pres := newPresenter(!tty.headless, cfg.Colors.useTrueColor(tty.headless, os.Getenv))

	// suspend hands the terminal back, stops the process until the shell continues
	// it, then takes the terminal over again.
	suspend := func() error {
		keys.reset()
		if err := tty.Teardown(); err != nil {
			return err
		}
		stopProcess()
		if err := tty.Setup(); err != nil {
			return err
		}
		if exact {
			if err := tty.EnableKeyboardProtocol(); err != nil {
				return err
			}
		}
		pres.invalidate() // the screen was left and set up again, so it is blank
		last = time.Now() // so the time spent stopped is not a frame
		return nil
	}

	// The terminal sent those keys before the keyboard protocol was on, so they come
	// with no release. They count as taps: held, they would never be let go.
	if len(typed) > 0 {
		action := keys.feed(typed, time.Now())
		in.releaseAll()
		if done, err := handle(action, suspend); done {
			return err
		}
	}

	resizer, _ := game.(Resizer)
	if resizer != nil {
		resizer.Resize(nfb.Width(), nfb.Height())
	}

	// The window size the screen was last cleared for.
	shownW, shownH := termW, termH

	dynamic := cfg.Width == 0 || cfg.Height == 0
	for {
		select {
		case <-runCtx.Done():
			return ctx.Err() // nil when only a quit signal ended it
		case now := <-ticker.C:
			// select picks at random among what is ready, so a tick can win over a
			// run that has ended.
			if runCtx.Err() != nil {
				return ctx.Err()
			}
			dt := frameDT(now, last)
			last = now

			curW, curH, err := tty.Size()
			if err != nil {
				return err
			}
			// Some terminals briefly report 0x0; keep what is on screen then.
			if curW > 0 && curH > 0 {
				if dynamic {
					resW, resH := resolveSize(cfg.Width, cfg.Height, curW, curH)
					if resW != nfb.Width() || resH != nfb.Height() {
						nfb, err = nfb.resized(resW, resH)
						if err != nil {
							return err
						}
						if resizer != nil {
							resizer.Resize(resW, resH)
						}
					}
				}
				// Whatever the window held before no longer matches it, and what is left
				// outside a smaller frame or past a shrunk window would stay on screen:
				// the next frame erases the display and is sent whole.
				if curW != shownW || curH != shownH {
					pres.invalidate()
					shownW, shownH = curW, curH
				}
			}

			in.beginFrame(now)
			if err := game.Update(dt, in); err != nil {
				if errors.Is(err, ErrQuit) {
					return nil
				}
				return err
			}
			in.endFrame()
			nfb.Clear()
			game.Draw(nfb)
			// A fixed-size frame can be larger than the window: show the part that fits.
			if err := pres.present(tty.out, nfb, shownW, shownH); err != nil {
				return err
			}
		case b, ok := <-input:
			if !ok {
				input = nil
				continue
			}
			action := keys.feed(b, time.Now())
			if keys.refreshNeeded() {
				pres.invalidate()
			}
			if done, err := handle(action, suspend); done {
				return err
			}
		case <-keys.expired():
			if done, err := handle(keys.expire(time.Now()), suspend); done {
				return err
			}
		}
	}
}

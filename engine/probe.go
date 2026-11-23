package engine

import (
	"bytes"
	"errors"
	"io"
	"os"
	"time"

	"golang.org/x/term"
)

// Terminal is what Probe found out about the terminal a program runs in.
type Terminal struct {
	// IsTerminal says the input and the output are both a terminal. Where they are not, the
	// other fields are zero: there is nothing to ask.
	IsTerminal bool

	// Width and Height are its size in cells.
	Width, Height int

	// KeyReleases says it speaks the kitty keyboard protocol, so that it reports a key let
	// go: Input.Exact will be true, and several keys can be held at once. Without it, a held
	// key is a timing guess and only one stays held (see the guide docs/input.md).
	KeyReleases bool

	// TrueColor says the environment says it shows 24-bit colors, so that ColorsAuto sends
	// RGB colors exactly; otherwise they are sent as the nearest of the 256-color palette.
	TrueColor bool
}

// Probe asks the terminal Run would take over what it can do, without starting a game: whether
// it reports key releases, whether it shows 24-bit colors, and its size, for a launcher or a
// game's first screen to tell the player before the game begins. It reads Config.Input and
// Config.Output, nil meaning stdin and stdout, and no other field of cfg.
//
// Where the input or the output is not a terminal, Probe says so and asks nothing. Otherwise it
// puts the terminal in raw mode for as long as the question takes, at most a quarter second and
// much less where the terminal answers, writes the question and nothing else, and puts it back as
// it found it. A key pressed meanwhile, Ctrl-C included, is read along with the answer and
// dropped: a terminal cannot be given input back. Call it before Run, not while Run has the
// terminal.
//
// The error says the terminal could not be put in raw mode, written to or put back.
func Probe(cfg Config) (_ Terminal, err error) {
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
	if !inOK || !outOK || !term.IsTerminal(int(inFile.Fd())) || !term.IsTerminal(int(outFile.Fd())) {
		return Terminal{}, nil
	}

	w, h, err := newTerminal(inFile, out).Size()
	if err != nil {
		return Terminal{}, err
	}
	state, err := term.MakeRaw(int(inFile.Fd()))
	if err != nil {
		return Terminal{}, err
	}
	defer func() { err = errors.Join(err, term.Restore(int(inFile.Fd()), state)) }()
	chunks, stop := readInput(inFile)
	defer stop() // before the restore: nothing may read the terminal once Probe is over

	answers, _, err := probe(chunks, out, probeTimeout, true, false)
	if err != nil {
		return Terminal{}, err
	}
	return Terminal{
		IsTerminal:  true,
		Width:       w,
		Height:      h,
		KeyReleases: answers.kitty,
		TrueColor:   detectTrueColor(os.Getenv),
	}, nil
}

// probeAnswers is what a terminal said to the probe.
type probeAnswers struct {
	kitty        bool // it speaks the kitty keyboard protocol
	pixels       bool // it has pointer reports in pixels, mode 1016
	cellW, cellH int  // the size of a cell in pixels, or 0 where it did not say
}

// isProbeReply reports whether a CSI sequence is an answer to one of the probe's questions: the kitty protocol's
// flags (ESC [ ? flags u), DECRPM (ESC [ ? mode ; Ps $ y), the size of a cell (ESC [ 6 ; h ; w t), or device
// attributes (ESC [ ... c). No key is any of them.
func isProbeReply(params []byte, final byte) bool {
	private := len(params) > 0 && params[0] == '?'
	return (private && (final == 'u' || final == 'y')) || final == 'c' || final == 't'
}

// readProbeAnswers scans b for the answers to the probe's questions. done is true if it holds the
// device-attributes reply (a CSI sequence ending in c), which every terminal sends after the others, if any,
// and so marks the end of the wait.
func readProbeAnswers(b []byte) (a probeAnswers, done bool) {
	for i := 0; i+1 < len(b); i++ {
		if b[i] != 0x1b || b[i+1] != '[' {
			continue
		}
		j := i + 2
		for j < len(b) && b[j] >= 0x20 && b[j] <= 0x3f {
			j++
		}
		if j >= len(b) {
			break // cut off; the rest may arrive in the next read
		}
		params, final := b[i+2:j], b[j]
		private := len(params) > 0 && params[0] == '?'
		switch {
		case private && final == 'u':
			a.kitty = true
		case private && final == 'y':
			// DECRPM for 1016: 1 set and 2 reset both mean the terminal has the mode, and 3 that it is set for
			// good; 0 is a mode it does not know, and 4 one it cannot set.
			fields := csiParams(bytes.TrimSuffix(params[1:], []byte("$"))) // the $ is the reply's, not the number's
			if param(fields, 0, 0) == 1016 {
				if ps := param(fields, 1, 0); ps >= 1 && ps <= 3 {
					a.pixels = true
				}
			}
		case final == 't':
			fields := csiParams(params)
			if h, w := param(fields, 1, 0), param(fields, 2, 0); param(fields, 0, 0) == 6 && h > 0 && w > 0 {
				a.cellW, a.cellH = w, h
			}
		case final == 'c':
			done = true
		}
		i = j
	}
	return a, done
}

// stripProbeReplies returns b without the probe's replies, so what is left is what
// was typed. It follows the same rules as readProbeAnswers, and a reply cut off at
// the end of b is kept, since it cannot be told apart from the start of a key.
func stripProbeReplies(b []byte) []byte {
	var typed []byte
	for i := 0; i < len(b); {
		if b[i] == 0x1b && i+1 < len(b) && b[i+1] == '[' {
			j := i + 2
			for j < len(b) && b[j] >= 0x20 && b[j] <= 0x3f {
				j++
			}
			if j < len(b) && isProbeReply(b[i+2:j], b[j]) {
				i = j + 1
				continue
			}
		}
		typed = append(typed, b[i])
		i++
	}
	return typed
}

// probe asks the terminal what it has: with askKitty whether it speaks the kitty keyboard protocol, and with
// askMouse whether it has pointer reports in pixels and how large a cell is in pixels. It writes those questions
// and then a device-attributes query to w, and reads the answers from in. The wait ends when the attributes
// reply arrives, so a terminal that has none of it answers almost at once; one that answers nothing costs the
// full timeout.
//
// Keys typed during the wait are read along with the replies. They are returned in
// typed, so that they are not lost.
func probe(in <-chan []byte, w io.Writer, timeout time.Duration, askKitty, askMouse bool) (a probeAnswers, typed []byte, err error) {
	sb := newSequenceBuilder()
	if askKitty {
		sb.QueryKeyboardProtocol()
	}
	if askMouse {
		sb.Print(queryMousePixelsSeq)
		sb.Print(queryCellSizeSeq)
	}
	sb.QueryDeviceAttributes()
	if err := sb.Flush(w); err != nil {
		return probeAnswers{}, nil, err
	}

	// Replies can be split across reads, so keep everything and rescan it.
	var got []byte
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case b, ok := <-in:
			if !ok {
				a, _ := readProbeAnswers(got)
				return a, stripProbeReplies(got), nil
			}
			got = append(got, b...)
			if a, done := readProbeAnswers(got); done {
				return a, stripProbeReplies(got), nil
			}
		case <-timer.C:
			a, _ := readProbeAnswers(got)
			return a, stripProbeReplies(got), nil
		}
	}
}

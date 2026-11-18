// Package record plays a game from a script of key presses, with no terminal, and keeps the
// frames it drew as an asciicast, the text format of terminal recordings. The pictures of the
// examples in site/media are drawn from these.
//
// It is for the examples' tests. Each has a TestRecord that calls Save with its script, and
//
//	go test ./examples/cube3d -run TestRecord -record /tmp/casts
//
// writes cube3d.cast there (make record does all of them). Without the flag those tests skip.
package record

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"glonk.dev/engine"
)

var dir = flag.String("record", "", "write the recording, an asciicast, into this directory")

// A Step is something done to the keyboard just before one frame is played.
type Step struct {
	Frame int                    // the frame, counted from 0
	Do    func(*engine.Keyboard) // a press, a release, a move of the mouse
}

// A Tape says what to play and how large: the size in cells, how many frames make a second,
// how many frames there are in all, and what happens to the keyboard on the way.
type Tape struct {
	Width, Height int
	FPS           int
	Frames        int
	Steps         []Step
}

// Hold is the two steps of a key held from one frame up to, and not including, another.
func Hold(key engine.Key, from, to int) []Step {
	return []Step{
		{Frame: from, Do: func(kb *engine.Keyboard) { kb.Press(key) }},
		{Frame: to, Do: func(kb *engine.Keyboard) { kb.Release(key) }},
	}
}

// Tap is a key pressed on one frame and let go on the next.
func Tap(key engine.Key, frame int) []Step { return Hold(key, frame, frame+1) }

// Steps joins several lists of steps into one.
func Steps(lists ...[]Step) []Step {
	var out []Step
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

// frameStart is how every frame of a Run without a terminal begins: the cursor moved home.
const frameStart = "\x1b[1;1H"

// player is the game as Run sees it. It gives the real game the same time step every frame
// and the script's keys, whatever the clock and the input of the run are, so that a tape plays
// the same on any machine.
type player struct {
	game  engine.Game
	kb    *engine.Keyboard
	tape  Tape
	frame int
}

func (p *player) Update(float64, *engine.Input) error {
	if p.frame >= p.tape.Frames {
		return engine.ErrQuit
	}
	for _, s := range p.tape.Steps {
		if s.Frame == p.frame {
			s.Do(p.kb)
		}
	}
	p.frame++
	err := p.game.Update(1/float64(p.tape.FPS), p.kb.Input())
	p.kb.EndFrame()
	return err
}

func (p *player) Draw(fb *engine.FrameBuffer) { p.game.Draw(fb) }

func (p *player) Resize(w, h int) {
	if r, ok := p.game.(engine.Resizer); ok {
		r.Resize(w, h)
	}
}

func (p *player) Stats(s engine.Stats) {
	if r, ok := p.game.(engine.StatsReceiver); ok {
		r.Stats(s)
	}
}

// frames keeps what a run writes, one entry a frame.
type frames [][]byte

func (f *frames) Write(b []byte) (int, error) {
	// Run writes a frame in one piece. Should it ever not, the rest belongs to the same frame.
	if len(*f) > 0 && !bytes.HasPrefix(b, []byte(frameStart)) {
		last := len(*f) - 1
		(*f)[last] = append((*f)[last], b...)
		return len(b), nil
	}
	*f = append(*f, bytes.Clone(b))
	return len(b), nil
}

// Play runs game through tape and returns what it drew, a frame at a time. It takes as long as
// the tape is, since a game may show how fast it runs. A game that quits ends the tape early.
func Play(game engine.Game, tape Tape) ([][]byte, error) {
	// Run refuses a tape with no size or no rate; only the length is the tape's own.
	if tape.Frames <= 0 {
		return nil, fmt.Errorf("record: a tape needs a length, got %d frames", tape.Frames)
	}
	var out frames
	err := engine.Run(context.Background(), &player{game: game, kb: engine.NewKeyboard(), tape: tape}, engine.Config{
		Width:  tape.Width,
		Height: tape.Height,
		FPS:    tape.FPS,
		Colors: engine.ColorsTrue,
		Input:  strings.NewReader(""),
		Output: &out,
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Cast writes frames as an asciicast, version 2: a line that says the size, then a line for
// each frame with the second it is shown at.
func Cast(w io.Writer, tape Tape, frames [][]byte) error {
	if tape.FPS <= 0 {
		return errors.New("record: a cast needs the tape's frames a second")
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(map[string]int{"version": 2, "width": tape.Width, "height": tape.Height}); err != nil {
		return err
	}
	for i, f := range frames {
		if err := enc.Encode([]any{float64(i) / float64(tape.FPS), "o", string(f)}); err != nil {
			return err
		}
	}
	return nil
}

// Save plays game through tape and writes name.cast into the directory the -record flag names.
// Without the flag it skips the test: playing takes as long as the tape is.
func Save(t *testing.T, name string, game engine.Game, tape Tape) {
	t.Helper()
	if *dir == "" {
		t.Skip("run with -record and a directory to write the recording (make record)")
	}
	if err := save(*dir, name, game, tape); err != nil {
		t.Fatal(err)
	}
}

// save is Save without the test: it plays the tape and writes the cast into dir.
func save(dir, name string, game engine.Game, tape Tape) error {
	played, err := Play(game, tape)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := Cast(&buf, tape, played); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".cast"), buf.Bytes(), 0o644)
}

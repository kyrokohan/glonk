// Command input shows what a terminal sends, as glonk reads it: each key as it goes down, whether
// the terminal reports key releases, and which keys are held. Run it in every terminal you care
// about, and press what a game would use: the function keys, Home and End, Page Up and Page Down,
// the arrows, letters.
//
//	go run ./examples/input
//
// Ctrl-C quits, so that Escape and every other key can be tried.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"glonk.dev/engine"
)

// maxLog is how many lines of what was pressed are kept, newest first.
const maxLog = 200

// specialKeys are the keys that are not a character, in the order the monitor asks about them.
var specialKeys = []engine.Key{
	engine.KeyUp, engine.KeyDown, engine.KeyLeft, engine.KeyRight,
	engine.KeyEscape, engine.KeyEnter, engine.KeyBackspace, engine.KeyTab,
	engine.KeyHome, engine.KeyEnd, engine.KeyPageUp, engine.KeyPageDown, engine.KeyInsert, engine.KeyDelete,
	engine.KeyF1, engine.KeyF2, engine.KeyF3, engine.KeyF4, engine.KeyF5, engine.KeyF6,
	engine.KeyF7, engine.KeyF8, engine.KeyF9, engine.KeyF10, engine.KeyF11, engine.KeyF12,
}

// names are the special keys by name. Ctrl-C and Ctrl-Z are Run's, so Input never reports them.
var names = map[engine.Key]string{
	engine.KeyUp: "Up", engine.KeyDown: "Down", engine.KeyLeft: "Left", engine.KeyRight: "Right",
	engine.KeyEscape: "Escape", engine.KeyEnter: "Enter", engine.KeyBackspace: "Backspace", engine.KeyTab: "Tab",
	engine.KeyHome: "Home", engine.KeyEnd: "End", engine.KeyPageUp: "PageUp", engine.KeyPageDown: "PageDown",
	engine.KeyInsert: "Insert", engine.KeyDelete: "Delete",
	engine.KeyF1: "F1", engine.KeyF2: "F2", engine.KeyF3: "F3", engine.KeyF4: "F4", engine.KeyF5: "F5", engine.KeyF6: "F6",
	engine.KeyF7: "F7", engine.KeyF8: "F8", engine.KeyF9: "F9", engine.KeyF10: "F10", engine.KeyF11: "F11", engine.KeyF12: "F12",
}

// keyName is how the monitor writes a key: a special key by its name, a character quoted.
func keyName(k engine.Key) string {
	if name, ok := names[k]; ok {
		return name
	}
	return strconv.QuoteRune(rune(k))
}

// probeKeys is every key the monitor asks Input about each frame: the special keys, and the
// printable ASCII characters, space to tilde.
var probeKeys = func() []engine.Key {
	keys := append([]engine.Key(nil), specialKeys...)
	for r := ' '; r <= '~'; r++ {
		keys = append(keys, engine.Key(r))
	}
	return keys
}()

type monitor struct {
	exact bool     // whether the terminal reports key releases
	held  []string // the keys that are down now
	log   []string // what was pressed, newest first
}

func newMonitor() *monitor { return &monitor{} }

// Update notes every key that went down this frame and which are held.
func (m *monitor) Update(_ float64, in *engine.Input) error {
	m.exact = in.Exact()
	m.held = m.held[:0]
	for _, k := range probeKeys {
		switch {
		case in.Started(k):
			m.add("down    " + keyName(k))
		case in.Pressed(k):
			m.add("repeat  " + keyName(k))
		}
		if in.IsDown(k) {
			m.held = append(m.held, keyName(k))
		}
	}
	return nil
}

// add puts a line at the top of the log and drops the oldest past maxLog.
func (m *monitor) add(line string) {
	m.log = append([]string{line}, m.log...)
	if len(m.log) > maxLog {
		m.log = m.log[:maxLog]
	}
}

func (m *monitor) Draw(fb *engine.FrameBuffer) {
	fb.DrawText(1, 0, "glonk input monitor: press keys. Ctrl-C quits.", engine.Palette(252))

	releases := "no: a held key is a timing guess, and only one stays held"
	if m.exact {
		releases = "yes (the kitty keyboard protocol): held keys are exact"
	}
	fb.DrawText(1, 1, "terminal reports key releases: "+releases, engine.Palette(245))
	fb.DrawText(1, 3, "held: "+strings.Join(m.held, " "), engine.Palette(226))

	for i, line := range m.log {
		y := 5 + i
		if y >= fb.Height() {
			break
		}
		color := engine.Palette(250)
		if strings.HasPrefix(line, "repeat") {
			color = engine.Palette(243)
		}
		fb.DrawText(1, y, line, color)
	}
}

func main() {
	// Width and Height of 0 fill the terminal and follow resizes.
	err := engine.Run(context.Background(), newMonitor(), engine.Config{FPS: 30})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Command input shows what a terminal sends, as glonk reads it: every key event with the
// modifiers held, the text typed, what is pasted, where the mouse is and what its buttons and wheel
// do, whether the terminal reports key releases, and which keys are held. Run it in every terminal
// you care about, and press what a game would use: the function keys, Home and End, Page Up and
// Page Down, the arrows, letters, Ctrl, Alt and Shift with them, paste some text with line breaks,
// and move the mouse over the window, click, drag and turn the wheel. The mouse line says the canvas pixel
// too, and "fine" where the terminal reports the pointer in pixels. The field line is a
// widget.TextField to type into and edit, with Left, Right, Home, End, Backspace and Delete; Enter
// logs its text and empties it.
//
//	go run ./examples/input
//
// Ctrl-C quits, so that Escape and every other key can be tried. Before it starts, it prints what
// engine.Probe says of the terminal, which is there again once it quits.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"glonk.dev/engine"
	"glonk.dev/widget"
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

// maxTyped is how many characters of the typed text the monitor shows, and maxPasted how many of
// the last paste.
const (
	maxTyped  = 60
	maxPasted = 60
)

type monitor struct {
	exact  bool     // whether the terminal reports key releases
	held   []string // the keys that are down now
	typed  []rune   // the last characters typed
	pasted string   // the start of the last paste, quoted
	log    []string // what happened to the keys and the mouse, newest first

	pos            string   // where the pointer was last reported, or "" before a report
	buttons        []string // the mouse buttons that are down now
	wheelX, wheelY int      // what the wheel has added up to since the monitor began

	pixel string           // where the pointer is in canvas pixels, and whether finer than a cell, or "" before a report
	field widget.TextField // a line to type into and edit, which Enter logs and empties
}

// mouseButtons are the buttons Input reports, by name.
var mouseButtons = []struct {
	b    engine.MouseButton
	name string
}{{engine.MouseLeft, "Left"}, {engine.MouseMiddle, "Middle"}, {engine.MouseRight, "Right"}}

// preview is the start of a paste, quoted so that what it holds, a line break or an escape sequence,
// is shown and not carried out.
func preview(paste string) string {
	r := []rune(paste)
	more := ""
	if len(r) > maxPasted {
		r, more = r[:maxPasted], "..."
	}
	return strconv.Quote(string(r)) + more
}

// modsName writes modifiers as Ctrl+Alt+Shift+Super+, in that order, or nothing for none.
func modsName(m engine.Mods) string {
	var sb strings.Builder
	for _, mod := range []struct {
		bit  engine.Mods
		name string
	}{{engine.ModCtrl, "Ctrl+"}, {engine.ModAlt, "Alt+"}, {engine.ModShift, "Shift+"}, {engine.ModSuper, "Super+"}} {
		if m.Has(mod.bit) {
			sb.WriteString(mod.name)
		}
	}
	return sb.String()
}

// actionName writes what happened to a key.
func actionName(a engine.KeyAction) string {
	switch a {
	case engine.ActionRepeat:
		return "repeat"
	case engine.ActionRelease:
		return "release"
	}
	return "press"
}

func newMonitor() *monitor {
	return &monitor{field: widget.TextField{Style: engine.Cell{Foreground: engine.Palette(117), Background: engine.Palette(236)}}}
}

// Update notes every key event of the frame, the text typed, and which keys are held.
func (m *monitor) Update(_ float64, in *engine.Input) error {
	m.exact = in.Exact()
	for _, e := range in.KeyEvents() {
		m.add(fmt.Sprintf("%-8s%s%s", actionName(e.Action), modsName(e.Mods), keyName(e.Key)))
	}
	if paste := in.Paste(); paste != "" {
		m.add(fmt.Sprintf("%-8s%d bytes, %d lines", "paste", len(paste), strings.Count(paste, "\n")+1))
		m.pasted = preview(paste)
	}
	m.mouse(in)
	if m.field.Update(in) {
		m.add(fmt.Sprintf("%-8s%q", "entered", m.field.Text))
		m.field.Text, m.field.Cursor = "", 0
	}
	m.typed = append(m.typed, []rune(in.Text())...)
	if len(m.typed) > maxTyped {
		m.typed = m.typed[len(m.typed)-maxTyped:]
	}
	m.held = m.held[:0]
	for _, k := range probeKeys {
		if in.IsDown(k) {
			m.held = append(m.held, keyName(k))
		}
	}
	return nil
}

// mouse notes where the pointer is, what its buttons did in the frame and which are down, and the wheel.
func (m *monitor) mouse(in *engine.Input) {
	x, y, ok := in.MousePos()
	if ok {
		m.pos = fmt.Sprintf("%d,%d", x, y)
		// The canvas pixel too, with fractions where the terminal reports pixels, and the middle of the cell where not.
		px, py, _ := in.MousePixels()
		how := "by cell: the terminal reports no pixels"
		if in.MouseFine() {
			how = "fine: the terminal reports pixels"
		}
		m.pixel = fmt.Sprintf("canvas pixel: %.1f,%.1f, %s", px, py, how)
	}
	m.buttons = m.buttons[:0]
	for _, mb := range mouseButtons {
		if in.MouseStarted(mb.b) {
			m.add(fmt.Sprintf("%-8smouse %s at %d,%d", "press", mb.name, x, y))
		}
		if in.MouseReleased(mb.b) {
			m.add(fmt.Sprintf("%-8smouse %s at %d,%d", "release", mb.name, x, y))
		}
		if in.MouseDown(mb.b) {
			m.buttons = append(m.buttons, mb.name)
		}
	}
	if dx, dy := in.Wheel(); dx != 0 || dy != 0 {
		m.wheelX, m.wheelY = m.wheelX+dx, m.wheelY+dy
		m.add(fmt.Sprintf("%-8sright %+d, down %+d", "wheel", dx, dy))
	}
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

	releases := "no: every auto-repeat is a press, a held key is a timing guess, and only one stays held"
	if m.exact {
		releases = "yes (the kitty keyboard protocol): repeats, releases and modifiers are exact"
	}
	fb.DrawText(1, 1, "terminal reports key releases: "+releases, engine.Palette(245))
	fb.DrawText(1, 3, "held: "+strings.Join(m.held, " "), engine.Palette(226))
	fb.DrawText(1, 4, "typed: "+string(m.typed), engine.Palette(117))
	fb.DrawText(1, 5, "pasted: "+m.pasted, engine.Palette(183))
	fb.DrawText(1, 6, m.mouseLine(), engine.Palette(121))
	fb.DrawText(1, 7, m.pixel, engine.Palette(121))
	fb.DrawText(1, 8, "field:", engine.Palette(117))
	m.field.Draw(fb, 8, 8, 40)

	for i, line := range m.log {
		y := 10 + i
		if y >= fb.Height() {
			break
		}
		color := engine.Palette(250)
		if strings.HasPrefix(line, "repeat") || strings.HasPrefix(line, "release") {
			color = engine.Palette(243)
		}
		fb.DrawText(1, y, line, color)
	}
}

// mouseLine is the state of the mouse in one line: where it is, which buttons are down and the wheel, or that no
// terminal report has come, with the two things that stop one.
func (m *monitor) mouseLine() string {
	if m.pos == "" {
		return "mouse: no report yet: move the pointer over the window (a terminal without SGR mouse mode sends none)"
	}
	return fmt.Sprintf("mouse: cell %s, down: [%s], wheel total: right %+d, down %+d", m.pos, strings.Join(m.buttons, " "), m.wheelX, m.wheelY)
}

// probeLine is what engine.Probe found, in a line, or why it could not ask.
func probeLine(t engine.Terminal, err error) string {
	switch {
	case err != nil:
		return "could not ask the terminal: " + err.Error()
	case !t.IsTerminal:
		return "not a terminal: input or output is redirected"
	}
	yes := map[bool]string{true: "yes", false: "no"}
	return fmt.Sprintf("this terminal: key releases %s, true color %s, %d by %d cells", yes[t.KeyReleases], yes[t.TrueColor], t.Width, t.Height)
}

func main() {
	// Asked before the game, so the answer is on the screen again when Ctrl-C leaves the game's.
	fmt.Println(probeLine(engine.Probe(engine.Config{})))

	// Width and Height of 0 fill the terminal and follow resizes.
	err := engine.Run(context.Background(), newMonitor(), engine.Config{FPS: 30, Mouse: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

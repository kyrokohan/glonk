package web

import (
	"bytes"
	"context"
	"io"
	"slices"
	"testing"

	"glonk.dev/engine"
)

// seen is what a game's Input said over a run fed some bytes.
type seen struct {
	events  []engine.KeyEvent
	text    string
	paste   string
	held    []engine.Key // the keys down at the end
	pos     [3]int       // x, y and 1 for known, at the end
	started []engine.MouseButton
	ended   []engine.MouseButton
	down    []engine.MouseButton // the buttons down at the end
	wheelX  int
	wheelY  int
}

// stall is an input that never ends, so that Run keeps reading until the game quits.
type stall struct{}

func (stall) Read([]byte) (int, error) { select {} }

// feed runs a headless game on b, as a page's Run does, for a few frames, and says what Input saw.
func feed(t *testing.T, b []byte) seen {
	t.Helper()
	var s seen
	frames := 0
	game := game{update: func(in *engine.Input) error {
		s.events = append(s.events, in.KeyEvents()...)
		s.text += in.Text()
		s.paste += in.Paste()
		for _, mb := range []engine.MouseButton{engine.MouseLeft, engine.MouseMiddle, engine.MouseRight} {
			if in.MouseStarted(mb) {
				s.started = append(s.started, mb)
			}
			if in.MouseReleased(mb) {
				s.ended = append(s.ended, mb)
			}
		}
		dx, dy := in.Wheel()
		s.wheelX, s.wheelY = s.wheelX+dx, s.wheelY+dy
		frames++
		if frames < 8 {
			return nil
		}
		s.held = nil
		for _, k := range []engine.Key{'a', 'w', engine.KeyUp, engine.KeyF5} {
			if in.IsDown(k) {
				s.held = append(s.held, k)
			}
		}
		s.down = nil
		for _, mb := range []engine.MouseButton{engine.MouseLeft, engine.MouseMiddle, engine.MouseRight} {
			if in.MouseDown(mb) {
				s.down = append(s.down, mb)
			}
		}
		if x, y, ok := in.MousePos(); ok {
			s.pos = [3]int{x, y, 1}
		}
		return engine.ErrQuit
	}}
	err := engine.Run(context.Background(), game, engine.Config{
		Width: 20, Height: 5, FPS: 200,
		Input:  io.MultiReader(bytes.NewReader(b), stall{}),
		Output: io.Discard,
	})
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	return s
}

// game is a Game of one Update function and an empty Draw.
type game struct{ update func(*engine.Input) error }

func (g game) Update(_ float64, in *engine.Input) error { return g.update(in) }
func (g game) Draw(*engine.FrameBuffer)                 {}

// keys joins what keyBytes gives for each of a run of keys.
func keys(ks ...[]byte) []byte { return bytes.Join(ks, nil) }

func TestKeysReachTheGameAsTheyWerePressed(t *testing.T) {
	for _, tt := range []struct {
		name   string
		in     []byte
		events []engine.KeyEvent
		text   string
	}{
		{"a letter, down and up", keys(keyBytes("a", "KeyA", 0, actionPress), keyBytes("a", "KeyA", 0, actionRelease)),
			[]engine.KeyEvent{{Key: 'a', Action: engine.ActionPress}, {Key: 'a', Action: engine.ActionRelease}}, "a"},
		{"Shift and a letter types the capital", keyBytes("A", "KeyA", modShift, actionPress),
			[]engine.KeyEvent{{Key: 'a', Mods: engine.ModShift, Action: engine.ActionPress}}, "A"},
		{"Shift and 1 is the 1 key and types !", keyBytes("!", "Digit1", modShift, actionPress),
			[]engine.KeyEvent{{Key: '1', Mods: engine.ModShift, Action: engine.ActionPress}}, "!"},
		{"a repeat", keyBytes("w", "KeyW", 0, actionRepeat),
			[]engine.KeyEvent{{Key: 'w', Action: engine.ActionRepeat}}, "w"},
		{"Ctrl and a letter types nothing", keyBytes("s", "KeyS", modCtrl, actionPress),
			[]engine.KeyEvent{{Key: 's', Mods: engine.ModCtrl, Action: engine.ActionPress}}, ""},
		{"Option and a types å", keyBytes("å", "KeyA", modAlt, actionPress),
			[]engine.KeyEvent{{Key: 'a', Mods: engine.ModAlt, Action: engine.ActionPress}}, "å"},
		{"letters beyond ASCII", keys(keyBytes("é", "", 0, actionPress), keyBytes("ß", "", 0, actionPress)),
			[]engine.KeyEvent{{Key: 'é', Action: engine.ActionPress}, {Key: 'ß', Action: engine.ActionPress}}, "éß"},
		{"the space bar", keyBytes(" ", "Space", 0, actionPress),
			[]engine.KeyEvent{{Key: ' ', Action: engine.ActionPress}}, " "},
		{"Escape, Enter, Tab and Backspace", keys(keyBytes("Escape", "Escape", 0, actionPress), keyBytes("Enter", "Enter", 0, actionPress), keyBytes("Tab", "Tab", modShift, actionPress), keyBytes("Backspace", "Backspace", 0, actionPress)),
			[]engine.KeyEvent{{Key: engine.KeyEscape, Action: engine.ActionPress}, {Key: engine.KeyEnter, Action: engine.ActionPress}, {Key: engine.KeyTab, Mods: engine.ModShift, Action: engine.ActionPress}, {Key: engine.KeyBackspace, Action: engine.ActionPress}}, ""},
		{"arrows, Home and End with modifiers", keys(keyBytes("ArrowUp", "ArrowUp", 0, actionPress), keyBytes("ArrowLeft", "ArrowLeft", modCtrl|modShift, actionRelease), keyBytes("Home", "Home", modAlt, actionPress), keyBytes("End", "End", 0, actionRepeat)),
			[]engine.KeyEvent{{Key: engine.KeyUp, Action: engine.ActionPress}, {Key: engine.KeyLeft, Mods: engine.ModCtrl | engine.ModShift, Action: engine.ActionRelease}, {Key: engine.KeyHome, Mods: engine.ModAlt, Action: engine.ActionPress}, {Key: engine.KeyEnd, Action: engine.ActionRepeat}}, ""},
		{"Insert, Delete, Page Up and Page Down", keys(keyBytes("Insert", "Insert", 0, actionPress), keyBytes("Delete", "Delete", modShift, actionPress), keyBytes("PageUp", "PageUp", 0, actionPress), keyBytes("PageDown", "PageDown", 0, actionRelease)),
			[]engine.KeyEvent{{Key: engine.KeyInsert, Action: engine.ActionPress}, {Key: engine.KeyDelete, Mods: engine.ModShift, Action: engine.ActionPress}, {Key: engine.KeyPageUp, Action: engine.ActionPress}, {Key: engine.KeyPageDown, Action: engine.ActionRelease}}, ""},
		{"what is not a key glonk reads", keys(keyBytes("Shift", "ShiftLeft", modShift, actionPress), keyBytes("Dead", "Quote", 0, actionPress), keyBytes("CapsLock", "CapsLock", 0, actionPress), keyBytes("Unidentified", "", 0, actionPress), keyBytes("c", "KeyC", modSuper, actionPress), keyBytes("a", "KeyA", 0, 4)),
			nil, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := feed(t, tt.in)
			if !slices.Equal(s.events, tt.events) || s.text != tt.text {
				t.Errorf("wanted %v and text %q, got %v and %q", tt.events, tt.text, s.events, s.text)
			}
		})
	}
}

// Every function key arrives as itself, F3 included, which a cursor report could be taken for.
func TestFunctionKeys(t *testing.T) {
	var in []byte
	var want []engine.KeyEvent
	for i, k := range []engine.Key{engine.KeyF1, engine.KeyF2, engine.KeyF3, engine.KeyF4, engine.KeyF5, engine.KeyF6, engine.KeyF7, engine.KeyF8, engine.KeyF9, engine.KeyF10, engine.KeyF11, engine.KeyF12} {
		name := "F" + string(rune('0'+(i+1)%10))
		if i >= 9 {
			name = "F1" + string(rune('0'+(i+1)%10))
		}
		in = append(in, keyBytes(name, name, modShift, actionPress)...)
		want = append(want, engine.KeyEvent{Key: k, Mods: engine.ModShift, Action: engine.ActionPress})
	}
	if s := feed(t, in); !slices.Equal(s.events, want) {
		t.Errorf("wanted %v, got %v", want, s.events)
	}
}

// Keys held when the page loses focus are let go, since their releases will not come.
func TestFocusOutLetsTheKeysGo(t *testing.T) {
	s := feed(t, keys(keyBytes("w", "KeyW", 0, actionPress), keyBytes("ArrowUp", "ArrowUp", 0, actionPress)))
	if !slices.Equal(s.held, []engine.Key{'w', engine.KeyUp}) {
		t.Fatalf("wanted w and Up held, got %v", s.held)
	}
	if s := feed(t, keys(keyBytes("w", "KeyW", 0, actionPress), focusBytes(false))); len(s.held) != 0 {
		t.Errorf("wanted nothing held after the focus went, got %v", s.held)
	}
	if s := feed(t, keys(focusBytes(false), focusBytes(true), keyBytes("w", "KeyW", 0, actionPress))); !slices.Equal(s.held, []engine.Key{'w'}) {
		t.Errorf("wanted a key pressed after the focus came back held, got %v", s.held)
	}
}

// A paste arrives whole, as one, with its line break, and an escape in it neither ends it nor is read.
func TestPasteArrivesWhole(t *testing.T) {
	s := feed(t, pasteBytes("one\ntwo\x1b[201~three\x1b[A"))
	if s.paste != "one\ntwo[201~three[A" || len(s.events) != 0 || s.text != "" {
		t.Errorf("wanted the paste whole, with no key and no text, got %q, %v, %q", s.paste, s.events, s.text)
	}
}

// The pointer arrives in the cell the page says, with its buttons and the wheel.
func TestThePointer(t *testing.T) {
	s := feed(t, keys(
		pointerBytes("move", -1, 3, 2),
		pointerBytes("down", 0, 3, 2),
		pointerBytes("move", 0, 7, 4), // a drag
		pointerBytes("up", 0, 7, 4),
		pointerBytes("down", 2, 7, 4),
		pointerBytes("wheel", 1, 7, 4),
		pointerBytes("wheel", 1, 7, 4),
		pointerBytes("wheel", 0, 7, 4),
		pointerBytes("wheel", 3, 7, 4),
		pointerBytes("move", -1, -5, -1), // off the left and the top
	))
	if s.pos != [3]int{0, 0, 1} {
		t.Errorf("wanted the pointer held to cell 0, 0 off the edge, got %v", s.pos)
	}
	if !slices.Equal(s.started, []engine.MouseButton{engine.MouseLeft, engine.MouseRight}) || !slices.Equal(s.ended, []engine.MouseButton{engine.MouseLeft}) {
		t.Errorf("wanted left pressed and let go and right pressed, got started %v, ended %v", s.started, s.ended)
	}
	if !slices.Equal(s.down, []engine.MouseButton{engine.MouseRight}) {
		t.Errorf("wanted the right button still down, got %v", s.down)
	}
	if s.wheelX != 1 || s.wheelY != 1 {
		t.Errorf("wanted the wheel 1 right and 1 down in all, got %d, %d", s.wheelX, s.wheelY)
	}
	if b := pointerBytes("drag", 0, 1, 1); b != nil {
		t.Errorf("wanted nothing for a kind that is none, got %q", b)
	}
	if b := pointerBytes("down", 3, 1, 1); b != nil {
		t.Errorf("wanted nothing for a button that is none, got %q", b)
	}
}

// The drag moves the pointer to the cell it ends in.
func TestThePointerFollowsADrag(t *testing.T) {
	s := feed(t, keys(pointerBytes("down", 0, 3, 2), pointerBytes("move", 0, 7, 4)))
	if s.pos != [3]int{7, 4, 1} || !slices.Equal(s.down, []engine.MouseButton{engine.MouseLeft}) {
		t.Errorf("wanted the pointer dragged to 7, 4 with the left button down, got %v and %v", s.pos, s.down)
	}
}

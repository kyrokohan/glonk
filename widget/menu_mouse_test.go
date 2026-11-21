package widget

import (
	"slices"
	"testing"

	"glonk.dev/engine"
)

// pointed is a menu drawn on a frame of its own, run a frame at a time with a Keyboard, as a game
// with Config.Mouse set runs it.
type pointed struct {
	m          Menu
	kb         *engine.Keyboard
	fb         *engine.FrameBuffer
	x, y, w, h int
}

// newPointed is a menu of items at (x, y), w by h, on a frame of 20 by 8, with sel selected.
func newPointed(t *testing.T, items []string, sel, x, y, w, h int) *pointed {
	return &pointed{m: Menu{Items: items, Selected: sel}, kb: engine.NewKeyboard(), fb: frame(t, 20, 8), x: x, y: y, w: w, h: h}
}

// draw draws the menu on a cleared frame, as the game's Draw does.
func (p *pointed) draw() {
	p.fb.Clear()
	p.m.Draw(p.fb, p.x, p.y, p.w, p.h)
}

// update runs Update on what do does to the Keyboard, then ends the frame and lets the keys go.
func (p *pointed) update(do func(kb *engine.Keyboard), keys ...engine.Key) (int, bool) {
	if do != nil {
		do(p.kb)
	}
	p.kb.Press(keys...)
	choice, chosen := p.m.Update(p.kb.Input())
	p.kb.EndFrame()
	p.kb.Release(keys...)
	return choice, chosen
}

// moveTo moves the pointer to a cell.
func moveTo(x, y int) func(*engine.Keyboard) {
	return func(kb *engine.Keyboard) { kb.MouseMove(x, y) }
}

// clickAt moves the pointer to a cell and clicks the left button there, down and up in one frame.
func clickAt(x, y int) func(*engine.Keyboard) {
	return func(kb *engine.Keyboard) {
		kb.MouseMove(x, y)
		kb.MouseDown(engine.MouseLeft)
		kb.MouseUp(engine.MouseLeft)
	}
}

// A pointer that moves onto an item selects it; one that then sits still leaves the selection to the keys.
func TestMenuPointerSelectsOnAMoveAndLeavesTheKeysAlone(t *testing.T) {
	p := newPointed(t, []string{"a", "b", "c", "d"}, 0, 2, 1, 10, 4)
	p.draw()
	if _, chosen := p.update(moveTo(5, 3)); chosen || p.m.Selected != 2 {
		t.Fatalf("wanted the move onto row 3 to select item 2 and choose nothing, got Selected %d, chosen %v", p.m.Selected, chosen)
	}
	p.draw()
	p.update(nil, engine.KeyDown)
	if p.m.Selected != 3 {
		t.Fatalf("wanted Down under a still pointer to select item 3, got %d", p.m.Selected)
	}
	p.draw()
	p.update(nil)
	if p.m.Selected != 3 {
		t.Errorf("wanted a still pointer not to take the selection back, got %d", p.m.Selected)
	}
	p.draw()
	p.update(moveTo(9, 3)) // along the same item
	if p.m.Selected != 2 {
		t.Errorf("wanted a move within an item to select it, got %d", p.m.Selected)
	}
	if got := reversedRows(p.fb, 5); !slices.Equal(got, []int{4}) {
		t.Errorf("wanted the last Draw to show item 3 in row 4, got reversed rows %v", got)
	}
}

// A press of the left button on an item chooses it at once, however short; another button does not.
func TestMenuClickChoosesTheItemUnderThePointer(t *testing.T) {
	p := newPointed(t, []string{"a", "b", "c"}, 0, 2, 1, 10, 3)
	p.draw()
	if choice, chosen := p.update(clickAt(4, 2)); !chosen || choice != 1 || p.m.Selected != 1 {
		t.Fatalf("wanted a click on row 2 to choose item 1, got (%d, %v), Selected %d", choice, chosen, p.m.Selected)
	}
	p.draw()
	if choice, chosen := p.update(func(kb *engine.Keyboard) {
		kb.MouseMove(4, 3)
		kb.MouseDown(engine.MouseRight)
		kb.MouseUp(engine.MouseRight)
	}); chosen || choice != -1 {
		t.Errorf("wanted the right button to choose nothing, got (%d, %v)", choice, chosen)
	}
	p.draw()
	if choice, chosen := p.update(clickAt(4, 1), engine.KeyEnter); !chosen || choice != 0 {
		t.Errorf("wanted a click to come before Enter, got (%d, %v)", choice, chosen)
	}
	p.draw()
	p.update(func(kb *engine.Keyboard) { kb.MouseMove(4, 3); kb.MouseDown(engine.MouseLeft) })
	p.draw()
	if _, chosen := p.update(func(kb *engine.Keyboard) { kb.MouseUp(engine.MouseLeft) }); chosen {
		t.Errorf("wanted letting the button go not to choose again")
	}
}

// The pointer off the items does nothing: beside them, below the last of a short list, above, or past
// the edge of the frame.
func TestMenuIgnoresThePointerOffItsItems(t *testing.T) {
	for _, tt := range []struct {
		name string
		x, y int
	}{
		{"left of it", 1, 2},
		{"right of it", 12, 2},
		{"above it", 4, 0},
		{"below the last item, in its rows", 4, 4},
		{"past the edge of the frame", 21, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := newPointed(t, []string{"a", "b", "c"}, 1, 2, 1, 10, 5)
			if tt.name == "past the edge of the frame" {
				p.x, p.w = 15, 10 // reaches past the 20 columns of the frame
				tt.x = 21
			}
			p.draw()
			choice, chosen := p.update(func(kb *engine.Keyboard) {
				kb.MouseMove(tt.x, tt.y)
				kb.MouseDown(engine.MouseLeft)
				kb.MouseUp(engine.MouseLeft)
				kb.Wheel(0, 1)
			})
			if chosen || choice != -1 || p.m.Selected != 1 {
				t.Errorf("wanted nothing to happen, got (%d, %v), Selected %d", choice, chosen, p.m.Selected)
			}
		})
	}
}

// The pointer is read against the menu's last Draw, once: not before it is drawn, and not on a frame
// after one with no Draw, as when it opens again.
func TestMenuReadsThePointerOnlyAfterADraw(t *testing.T) {
	p := newPointed(t, []string{"a", "b", "c"}, 0, 2, 1, 10, 3)
	if _, chosen := p.update(clickAt(4, 2)); chosen || p.m.Selected != 0 {
		t.Fatalf("wanted a menu never drawn to ignore a click, got chosen %v, Selected %d", chosen, p.m.Selected)
	}
	p.draw()
	p.update(nil)
	if _, chosen := p.update(clickAt(4, 3)); chosen || p.m.Selected != 0 {
		t.Errorf("wanted a menu not drawn since its last Update to ignore a click, got chosen %v, Selected %d", chosen, p.m.Selected)
	}
	p.draw()
	if choice, chosen := p.update(clickAt(4, 3)); !chosen || choice != 2 {
		t.Errorf("wanted the click read again once drawn, got (%d, %v)", choice, chosen)
	}
	p.m.Draw(p.fb, 2, 1, 0, 3) // a Draw that draws nothing leaves nothing to point at
	if _, chosen := p.update(clickAt(4, 2)); chosen {
		t.Errorf("wanted a menu drawn with no width to ignore a click")
	}
}

// The wheel over an item moves the selection an item a step, wrapping as the keys do; off the items it
// does nothing.
func TestMenuWheelStepsAndWraps(t *testing.T) {
	for _, tt := range []struct {
		name      string
		start, dy int
		x, want   int
	}{
		{"down a step", 1, 1, 4, 2},
		{"down past the last wraps", 3, 1, 4, 0},
		{"up past the first wraps", 0, -1, 4, 3},
		{"three steps", 0, 3, 4, 3},
		{"many steps", 1, 9, 4, 2},
		{"off the items", 1, 1, 15, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := newPointed(t, []string{"a", "b", "c", "d"}, 0, 2, 1, 10, 4)
			p.draw()
			p.update(moveTo(tt.x, 1)) // over item 0, or beside it, before the wheel turns
			p.m.Selected = tt.start
			p.draw()
			if _, chosen := p.update(func(kb *engine.Keyboard) { kb.Wheel(0, tt.dy) }); chosen || p.m.Selected != tt.want {
				t.Errorf("wanted Selected %d and nothing chosen, got %d, chosen %v", tt.want, p.m.Selected, chosen)
			}
		})
	}
}

// A list longer than its rows holds still while the pointer picks among the items it shows, so that it
// does not run away under a pointer moving along its top row, and centers again after a key.
func TestMenuListHoldsStillUnderThePointer(t *testing.T) {
	items := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	p := newPointed(t, items, 5, 0, 0, 10, 3)
	p.draw() // items 4, 5 and 6
	for i := range 4 {
		p.update(moveTo(i, 0))
		p.draw()
		if p.m.Selected != 4 {
			t.Fatalf("move %d along the top row: wanted item 4 selected, got %d", i, p.m.Selected)
		}
		if got := reversedRows(p.fb, 9); !slices.Equal(got, []int{0}) {
			t.Fatalf("move %d along the top row: wanted the list to hold still with item 4 in row 0, got reversed rows %v", i, got)
		}
	}
	p.update(moveTo(3, 2))
	p.draw()
	if p.m.Selected != 6 || !slices.Equal(reversedRows(p.fb, 9), []int{2}) {
		t.Fatalf("wanted the bottom row to pick item 6 where it shows, got Selected %d, reversed rows %v", p.m.Selected, reversedRows(p.fb, 9))
	}
	p.update(nil, engine.KeyDown)
	p.draw() // a key centers it again: items 6, 7 and 8
	if p.m.Selected != 7 || !slices.Equal(reversedRows(p.fb, 9), []int{1}) {
		t.Errorf("wanted Down to center item 7, got Selected %d, reversed rows %v", p.m.Selected, reversedRows(p.fb, 9))
	}
	p.update(moveTo(0, 0))
	p.m.Selected = 0 // the game moves it off the rows that showed
	p.draw()
	if !slices.Equal(reversedRows(p.fb, 9), []int{0}) {
		t.Errorf("wanted an item off the rows that showed to be scrolled to, got reversed rows %v", reversedRows(p.fb, 9))
	}
}

// A key that moves the selection within the items showing centers it again, as before the pointer
// picked one; rows past the frame, and items taken away since Draw, are not there to point at.
func TestMenuKeysCenterAndFewerItemsAreNotThere(t *testing.T) {
	items := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	p := newPointed(t, items, 5, 0, 0, 10, 4)
	p.draw() // items 3 to 6
	p.update(moveTo(0, 0))
	p.draw()
	if p.m.Selected != 3 || !slices.Equal(reversedRows(p.fb, 9), []int{0}) {
		t.Fatalf("wanted the top row to pick item 3 where it shows, got Selected %d, reversed rows %v", p.m.Selected, reversedRows(p.fb, 9))
	}
	p.update(nil, engine.KeyDown)
	p.draw() // item 4, still showing, centered: items 2 to 5
	if p.m.Selected != 4 || !slices.Equal(reversedRows(p.fb, 9), []int{2}) {
		t.Errorf("wanted Down to center item 4, got Selected %d, reversed rows %v", p.m.Selected, reversedRows(p.fb, 9))
	}

	p = newPointed(t, items, 5, 0, 0, 10, 4)
	p.draw() // items 3 to 6
	p.update(moveTo(0, 1))
	p.draw()
	p.update(nil, engine.KeyUp)
	p.draw() // item 3, still showing, centered: items 1 to 4
	if p.m.Selected != 3 || !slices.Equal(reversedRows(p.fb, 9), []int{2}) {
		t.Errorf("wanted Up to center item 3, got Selected %d, reversed rows %v", p.m.Selected, reversedRows(p.fb, 9))
	}

	// So does the wheel.
	p = newPointed(t, items, 5, 0, 0, 10, 4)
	p.draw() // items 3 to 6
	p.update(moveTo(0, 1))
	p.draw()
	p.update(func(kb *engine.Keyboard) { kb.Wheel(0, -1) })
	p.draw() // item 3, centered: items 1 to 4
	if p.m.Selected != 3 || !slices.Equal(reversedRows(p.fb, 9), []int{2}) {
		t.Errorf("wanted the wheel to center item 3, got Selected %d, reversed rows %v", p.m.Selected, reversedRows(p.fb, 9))
	}

	// A menu drawn in another number of rows centers again.
	p = newPointed(t, items, 5, 0, 0, 10, 4)
	p.draw() // items 3 to 6
	p.update(moveTo(0, 0))
	p.h = 2
	p.draw() // item 3, centered in two rows: items 2 and 3
	if p.m.Selected != 3 || !slices.Equal(reversedRows(p.fb, 9), []int{1}) {
		t.Errorf("wanted a menu drawn in 2 rows to center item 3, got Selected %d, reversed rows %v", p.m.Selected, reversedRows(p.fb, 9))
	}

	// Rows past the bottom of the frame show nothing, though the pointer can be there in a larger terminal.
	low := newPointed(t, []string{"a", "b", "c", "d"}, 0, 2, 6, 10, 4)
	low.draw()
	if choice, chosen := low.update(clickAt(4, 8)); chosen || choice != -1 || low.m.Selected != 0 {
		t.Errorf("wanted a click below the frame to do nothing, got (%d, %v), Selected %d", choice, chosen, low.m.Selected)
	}

	q := newPointed(t, []string{"a", "b", "c", "d"}, 0, 2, 1, 10, 4)
	q.draw()
	q.m.Items = q.m.Items[:2]
	if choice, chosen := q.update(clickAt(4, 4)); chosen || choice != -1 || q.m.Selected != 0 {
		t.Errorf("wanted a click where a removed item was drawn to do nothing, got (%d, %v), Selected %d", choice, chosen, q.m.Selected)
	}
}

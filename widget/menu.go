package widget

import "glonk.dev/engine"

// Menu is a list of items with one of them selected, which the arrow keys move and Enter
// chooses, and, with [engine.Config.Mouse] set, the pointer moves and a click chooses. Call
// Update from the game's Update, and Draw from its Draw.
type Menu struct {
	// Items are the lines of the menu, in order. Each is written as a [Label] is, cut off
	// at the width.
	Items []string

	// Selected is the index of the selected item. A number that is not the index of an item
	// is taken as 0.
	Selected int

	// Style is the colors and attributes of the items. The selected one is drawn with
	// Reverse toggled, across the whole width, so that it stands out.
	Style engine.Cell

	// Where Draw last drew the menu, for Update to read the mouse against: the columns of the
	// frame from left up to right and the rows from first up to last, the item top being in row
	// y. Rows past the last item are no item. shown says it was drawn since the last Update.
	shown          bool
	left, right    int
	first, last, y int
	top, rows      int  // the first item drawn, and the rows Draw was given
	held           bool // the pointer picked Selected, so Draw keeps the items it showed
}

// Update moves the selection from the keys pressed since the last frame: Up and Down move it
// to the item before or after, wrapping around at the ends, and keep moving it while the key
// repeats, and Enter chooses the selected item once for each press, so a held Enter does not
// choose again (see [engine.Input.Started] for where it still can). The moves are made before
// Enter is read, so Down and Enter in one frame choose the next item. It returns the index
// of the chosen item and true on Enter, and -1 and false otherwise, and always when there
// are no items. Other keys, Escape included, are left to the game.
//
// With [engine.Config.Mouse] set it reads the pointer too, against where Draw last drew the
// menu: a pointer that moves onto an item selects it, a turn of the wheel over an item moves
// the selection an item a step, wrapping as the keys do, and a press of the left button on an
// item chooses it, at once, as Enter does. These come before the keys, and a click before
// Enter. A pointer that has not moved (see [engine.Input.MouseMoved]) leaves the selection to
// the keys, and one off the items, beside them or below the last, does nothing. Where Draw drew
// is read once: a menu that was not drawn since the last Update, before it is first drawn or on
// the frame it opens again, ignores the mouse, so a pointer where an old menu was picks nothing.
func (m *Menu) Update(in *engine.Input) (choice int, chosen bool) {
	shown := m.shown
	m.shown = false
	n := len(m.Items)
	if n == 0 {
		return -1, false
	}
	if m.Selected < 0 || m.Selected >= n {
		m.Selected = 0
	}

	line := -1 // the item under the pointer
	if shown {
		line = m.itemAt(in, n)
	}
	if line >= 0 && in.MouseMoved() {
		m.Selected, m.held = line, true
	}
	if _, dy := in.Wheel(); line >= 0 && dy != 0 {
		m.Selected, m.held = ((m.Selected+dy)%n+n)%n, false
	}
	if in.Pressed(engine.KeyUp) {
		m.Selected, m.held = (m.Selected+n-1)%n, false
	}
	if in.Pressed(engine.KeyDown) {
		m.Selected, m.held = (m.Selected+1)%n, false
	}
	if line >= 0 && in.MouseStarted(engine.MouseLeft) {
		m.Selected, m.held = line, true
		return line, true
	}
	// Started, not Pressed: a menu that opens another must not choose in it at once, from
	// the repeats of the Enter that chose here.
	if in.Started(engine.KeyEnter) {
		return m.Selected, true
	}
	return -1, false
}

// itemAt returns the item of n that Draw drew under the pointer, or -1 when the pointer is not
// on one.
func (m *Menu) itemAt(in *engine.Input, n int) int {
	x, y, ok := in.MousePos()
	if !ok || x < m.left || x >= m.right || y < m.first || y >= m.last {
		return -1
	}
	if i := m.top + (y - m.y); i < n { // below the last item, or the items are fewer since Draw
		return i
	}
	return -1
}

// Draw draws the menu in the w cells from (x, y) to the right and the h rows down, one item
// to a row. A menu with more items than rows scrolls to keep the selected item in the middle
// row, or as near the middle as the ends of the list allow, except that an item the pointer
// picked keeps the items that were showing while it is among them, so that the list does not
// move under the pointer. Draw does not change Items or Selected; it keeps where it drew, for
// the next Update to read the mouse against.
func (m *Menu) Draw(fb *engine.FrameBuffer, x, y, w, h int) {
	m.shown = false
	n := len(m.Items)
	if n == 0 || w <= 0 || h <= 0 {
		return
	}
	selected := m.Selected
	if selected < 0 || selected >= n {
		selected = 0
	}
	top := 0
	if n > h {
		top = min(max(selected-h/2, 0), n-h)
		if m.held && m.rows == h && m.top <= n-h && selected >= m.top && selected-m.top < h {
			top = m.top
		}
	}
	m.top, m.rows = top, h
	m.left, m.right = max(x, 0), min(reach(x, w), fb.Width())
	m.first, m.last, m.y = max(y, 0), min(reach(y, h), fb.Height()), y
	m.shown = m.left < m.right && m.first < m.last

	for i := 0; i < h && top+i < n; i++ {
		row := reach(y, i)
		if row < 0 {
			continue
		}
		if row >= fb.Height() {
			break
		}

		style := m.Style
		if top+i == selected {
			style.Attrs ^= engine.Reverse
			blank := styled(style, ' ')
			for cx := max(x, 0); cx < min(reach(x, w), fb.Width()); cx++ {
				fb.Set(cx, row, blank)
			}
		}
		Label{Text: m.Items[top+i], Style: style}.Draw(fb, x, row, w)
	}
}

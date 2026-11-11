package widget

import "glonk.dev/engine"

// Menu is a list of items with one of them selected, which the arrow keys move and Enter
// chooses. Call Update from the game's Update, and Draw from its Draw.
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
}

// Update moves the selection from the keys pressed since the last frame: Up and Down move it
// to the item before or after, wrapping around at the ends, and Enter chooses the selected
// item. The moves are made before Enter is read, so Down and Enter in one frame choose the
// next item. It returns the index of the chosen item and true on Enter, and -1 and false
// otherwise, and always when there are no items. Other keys, Escape included, are left to
// the game.
func (m *Menu) Update(in *engine.Input) (choice int, chosen bool) {
	n := len(m.Items)
	if n == 0 {
		return -1, false
	}
	if m.Selected < 0 || m.Selected >= n {
		m.Selected = 0
	}

	if in.Pressed(engine.KeyUp) {
		m.Selected = (m.Selected + n - 1) % n
	}
	if in.Pressed(engine.KeyDown) {
		m.Selected = (m.Selected + 1) % n
	}
	if in.Pressed(engine.KeyEnter) {
		return m.Selected, true
	}
	return -1, false
}

// Draw draws the menu in the w cells from (x, y) to the right and the h rows down, one item
// to a row. A menu with more items than rows scrolls to keep the selected item in the middle
// row, or as near the middle as the ends of the list allow. Draw does not change the menu.
func (m *Menu) Draw(fb *engine.FrameBuffer, x, y, w, h int) {
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
	}

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

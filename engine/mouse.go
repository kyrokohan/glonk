package engine

// MouseButton is a mouse button.
type MouseButton uint8

// The mouse buttons Input reports. The others a mouse has are not.
const (
	// MouseLeft is the main button.
	MouseLeft MouseButton = iota
	// MouseMiddle is the wheel pressed in.
	MouseMiddle
	// MouseRight is the secondary button.
	MouseRight
)

const (
	mouseButtons = 3 // how many buttons there are

	// maxMouseCell bounds a coordinate, so that a report that makes one up cannot hold a number a cell can
	// not have: no terminal is nearly 32,767 cells across, and the cell is packed in a key by cellKey.
	maxMouseCell = 1<<15 - 1

	// maxFrameWheel bounds what the wheel adds up to in a frame, on each axis, as maxFrameText bounds text.
	maxFrameWheel = 4096
)

// The wheel directions a terminal reports, as the btn of a kindWheel: the SGR button numbers 64 to 67 less 64.
const (
	wheelUp = iota
	wheelDown
	wheelLeft
	wheelRight
)

// cellKey packs a cell into the key of a mouse event, which has no key to hold. Both are 0 to maxMouseCell.
func cellKey(x, y int) Key {
	return Key(clampCell(x))<<16 | Key(clampCell(y))
}

// cell is the cell a mouse event happened in.
func (e keyEvent) cell() (x, y int) {
	return int(e.key >> 16), int(e.key & 0xffff)
}

// clampCell keeps a coordinate within 0 and maxMouseCell.
func clampCell(n int) int {
	return min(max(n, 0), maxMouseCell)
}

// maxReportNumber stops a number in a mouse report from growing past what any of them can mean.
const maxReportNumber = 1 << 20

// appendMouse decodes the parameters of an SGR mouse report, the part of CSI < button ; column ; row M or m
// after the "<", and appends the event it is. M is a press, a wheel step or a movement and m is a release. A
// report that is not three numbers, or that is about a button glonk does not report, is dropped.
//
// The button number is the xterm one: the low two bits are the button (3 for none), 4, 8 and 16 are Shift, Meta
// and Ctrl, which are not read, 32 is a movement, 64 is the wheel and 128 is a button past the seventh.
//
// With pixels, the column and the row are a pixel of the terminal, counted from 0 at the top left of the grid
// (mode 1016). A terminal reports the pointer over the padding round the grid too, where they can be less than 0,
// which is taken as 0, as the edge of the grid. A cell report has no minus sign.
func appendMouse(events []keyEvent, params []byte, final byte, pixels bool) []keyEvent {
	var n [3]int
	var negative uint8    // bit i: number i had a minus sign
	field, digits := 0, 0 // which number, and how many digits it has so far
	for _, c := range params {
		switch {
		case c >= '0' && c <= '9':
			n[field] = min(n[field]*10+int(c-'0'), maxReportNumber)
			digits++
		case c == ';' && digits > 0 && field < len(n)-1:
			field, digits = field+1, 0
		case c == '-' && pixels && digits == 0 && field > 0 && negative&(1<<field) == 0:
			negative |= 1 << field
		default:
			return events
		}
	}
	if negative != 0 {
		for i := range n {
			if negative&(1<<i) != 0 {
				n[i] = -n[i]
			}
		}
	}
	if field != len(n)-1 || digits == 0 {
		return events
	}

	cb := n[0]
	if cb > 0xff || cb&128 != 0 {
		return events
	}
	key := cellKey(n[1]-1, n[2]-1) // the terminal counts cells from 1
	if pixels {
		key = cellKey(n[1], n[2])
	}
	code := uint8(cb & 3)
	ev := keyEvent{key: key}
	switch {
	case cb&64 != 0:
		if final != 'M' || cb&32 != 0 {
			return events // a wheel is a step, not held and not let go
		}
		ev.kind, ev.btn = kindWheel, code
	case cb&32 != 0 || code == 3:
		ev.kind = kindMouseMove // dragging or not, it is where the pointer is
	case final == 'm':
		ev.kind, ev.btn = kindMouseUp, code
	default:
		ev.kind, ev.btn = kindMouseDown, code
	}
	return append(events, ev)
}

// mouseState is what Input knows of the mouse.
type mouseState struct {
	x, y     int
	ok       bool // whether a report has said where the pointer is
	down     [mouseButtons]bool
	started  [mouseButtons]bool // went down since the previous frame
	released [mouseButtons]bool // went up since the previous frame
	wheelX   int                // what the wheel added up to this frame
	wheelY   int

	// fromX, fromY and fromOK are where the pointer was at the end of the previous frame, for MouseMoved.
	fromX, fromY int
	fromOK       bool

	// pixels says the terminal reports the pointer in its pixels (mode 1016), a cell being cellW by cellH of
	// them; rawX and rawY are where the last report said it is, in pixels or in cells.
	pixels       bool
	cellW, cellH int
	rawX, rawY   int
}

// mouseEvent records a mouse report. Every one says where the pointer is.
func (in *Input) mouseEvent(e keyEvent) {
	m := &in.mouse
	m.rawX, m.rawY = e.cell()
	m.x, m.y = m.cellOf()
	m.ok = true
	switch b := e.btn; e.kind {
	case kindMouseDown:
		if b < mouseButtons && !m.down[b] {
			m.down[b], m.started[b] = true, true
		}
	case kindMouseUp:
		if b < mouseButtons && m.down[b] {
			m.down[b], m.released[b] = false, true
		}
	case kindWheel:
		switch b {
		case wheelUp:
			in.addWheel(0, -1)
		case wheelDown:
			in.addWheel(0, 1)
		case wheelLeft:
			in.addWheel(-1, 0)
		case wheelRight:
			in.addWheel(1, 0)
		}
	}
}

// cellOf is the cell the last report put the pointer in.
func (m *mouseState) cellOf() (x, y int) {
	if m.pixels {
		return m.rawX / m.cellW, m.rawY / m.cellH
	}
	return m.rawX, m.rawY
}

// usePixels turns pixel reports on, a cell being w by h pixels, which must be more than 0. A report read before
// was in cells, so the pointer it put somewhere is taken to the middle of that cell, in pixels.
func (in *Input) usePixels(w, h int) {
	m := &in.mouse
	if !m.pixels && m.ok {
		m.rawX, m.rawY = m.x*w+w/2, m.y*h+h/2
	}
	m.pixels = true
	in.setCellSize(w, h)
}

// setCellSize records the size of a cell in pixels, which the terminal said, and puts the pointer in the cell it
// is in at that size. A size that is not more than 0 is not one, and leaves the last.
func (in *Input) setCellSize(w, h int) {
	m := &in.mouse
	if w <= 0 || h <= 0 {
		return
	}
	m.cellW, m.cellH = w, h
	if m.ok {
		m.x, m.y = m.cellOf()
	}
}

// addWheel adds to what the wheel has done this frame, up to maxFrameWheel either way.
func (in *Input) addWheel(dx, dy int) {
	m := &in.mouse
	m.wheelX = min(max(m.wheelX+min(max(dx, -maxFrameWheel), maxFrameWheel), -maxFrameWheel), maxFrameWheel)
	m.wheelY = min(max(m.wheelY+min(max(dy, -maxFrameWheel), maxFrameWheel), -maxFrameWheel), maxFrameWheel)
}

// releaseMouse lets every button go, as releaseAll lets the keys go: the window lost focus, or input was
// thrown away, and a release may never arrive. A game sees each as released, so that a drag ends.
func (in *Input) releaseMouse() {
	m := &in.mouse
	for b := range m.down {
		if m.down[b] {
			m.down[b], m.released[b] = false, true
		}
	}
}

// endMouseFrame forgets what only this frame saw: the buttons that went down or up, and the wheel. Where the
// pointer is and which buttons are down stay, and where it is becomes where the next frame's MouseMoved starts.
func (in *Input) endMouseFrame() {
	m := &in.mouse
	m.started, m.released = [mouseButtons]bool{}, [mouseButtons]bool{}
	m.wheelX, m.wheelY = 0, 0
	m.fromX, m.fromY, m.fromOK = m.x, m.y, m.ok
}

// MousePos is the cell the pointer was last reported in, counted from 0 at the top left of the terminal, which
// is where a frame starts: the cell is also the frame's when the frame fills the terminal. ok is false until a
// report has said, which is always so when Config.Mouse is not set or the terminal sends none. The cell can be
// outside the frame, when the frame is smaller than the terminal. A canvas has two pixels to a cell, one above
// the other, so the pixel at the top of the cell is (x, 2*y); MousePixels says which of them, where the terminal
// reports pixels. In pixels a pointer over the padding round the grid is in the nearest cell at the top and the
// left, and past the last at the bottom and the right, as the terminal puts it.
//
// The pointer is where it last was when it leaves the window: a terminal says nothing outside it.
func (in *Input) MousePos() (x, y int, ok bool) {
	return in.mouse.x, in.mouse.y, in.mouse.ok
}

// MousePixels is where the pointer is in the pixels of a [Canvas], with fractions: a cell is one pixel across and
// two down, so (2.5, 7.25) is in the middle of the third column, a quarter of the way into pixel row 7. Where the
// terminal reports the pointer in its own pixels (see MouseFine), it is the middle of the terminal pixel the
// pointer is on; elsewhere it is the middle of the cell, (x+0.5, 2y+1) for MousePos x and y, so it is coarser but
// still right. ok is as MousePos says it. Like MousePos it counts from the top left of the terminal, and the canvas
// row of a pointer over the cell at row 3 is between 6 and 8.
func (in *Input) MousePixels() (x, y float64, ok bool) {
	m := &in.mouse
	if !m.ok {
		return 0, 0, false
	}
	if m.pixels {
		return (float64(m.rawX) + 0.5) / float64(m.cellW), 2 * (float64(m.rawY) + 0.5) / float64(m.cellH), true
	}
	return float64(m.x) + 0.5, 2*float64(m.y) + 1, true
}

// MouseFine reports whether the terminal reports the pointer in its own pixels, finer than a cell, so that
// MousePixels moves smoothly, as Exact says the keys are exact. Run asks for that with Config.Mouse where the
// terminal has it (mode 1016) and says how large a cell is; elsewhere the pointer moves a cell at a time.
func (in *Input) MouseFine() bool {
	return in.mouse.pixels
}

// MouseMoved reports whether the pointer is in another cell than at the end of the previous frame, so that a
// pointer that rests is told from one that moves: a menu follows a move, and leaves a selection the keys made
// under a pointer that sits still. The first report counts, since before it the pointer was nowhere. A click
// or a turn of the wheel where the pointer is does not move it, and nor does leaving a cell and coming back
// within one frame.
func (in *Input) MouseMoved() bool {
	m := &in.mouse
	return m.ok && (!m.fromOK || m.x != m.fromX || m.y != m.fromY)
}

// MouseDown reports whether b is down: pressed, and not yet let go. A drag is MouseDown with a MousePos that
// moves. A button is let go when the window loses focus, because the terminal then does not say.
func (in *Input) MouseDown(b MouseButton) bool {
	return b < mouseButtons && in.mouse.down[b]
}

// MouseStarted reports whether b went down since the previous frame, so that a click is one frame whatever
// time it was held for, and a click shorter than a frame is not lost: it is Started and Released in that frame.
func (in *Input) MouseStarted(b MouseButton) bool {
	return b < mouseButtons && in.mouse.started[b]
}

// MouseReleased reports whether b went up since the previous frame. A button let go because the window lost
// focus counts.
func (in *Input) MouseReleased(b MouseButton) bool {
	return b < mouseButtons && in.mouse.released[b]
}

// Wheel is how far the wheel turned since the previous frame, a step at a time: dy is positive for down, toward
// the user, and dx for right, as screen coordinates are. It is 0 and 0 without a wheel. A frame adds up to
// 4,096 steps on each axis, and no more.
func (in *Input) Wheel() (dx, dy int) {
	return in.mouse.wheelX, in.mouse.wheelY
}

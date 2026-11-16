package engine

import (
	"io"
	"slices"
	"strconv"
	"unicode/utf8"
)

// presenter sends frames to the terminal.
//
// In changes-only mode it remembers the frame it sent last and writes just the cells
// that differ, so a screen that does not change costs no bytes and a moving ball costs
// a few. That is only right while the terminal still shows what was sent, so anything
// that could have changed the screen behind its back must call invalidate: the next
// frame is then sent in full, after erasing the display.
//
// A frame that changes anything is sent as one synchronized update, in one write. In
// full mode every frame is sent whole and nothing else is written, which is what a
// program reading the output of a headless Run expects.
//
// One presenter belongs to one run; it is not safe for use by several goroutines.
type presenter struct {
	changesOnly bool
	trueColor   bool // send RGB colors exactly, not as the nearest palette color

	// prev is the part of the last frame that was sent, row by row, prevW cells wide.
	// It is a copy: the frame it came from is drawn into again straight away.
	prev         []Cell
	prevW, prevH int
	valid        bool // prev is what the terminal shows

	sb sequenceBuilder // kept between frames, so it stops allocating once it has grown

	// bytes is how many bytes the last call of present sent.
	bytes int
}

func newPresenter(changesOnly, trueColor bool) *presenter {
	return &presenter{changesOnly: changesOnly, trueColor: trueColor}
}

// invalidate says the terminal may no longer show the last frame, because the window
// changed size, the program was suspended, a write failed or the window came back to
// the front. The next frame is erased and sent in full.
func (p *presenter) invalidate() {
	p.valid = false
}

// present sends the top left maxW x maxH cells of fb. A fixed-size frame can be larger
// than the window, and only the part that fits is shown.
func (p *presenter) present(w io.Writer, fb *FrameBuffer, maxW, maxH int) error {
	width := min(fb.width, max(maxW, 0))
	height := min(fb.height, max(maxH, 0))

	p.sb.reset()
	p.sb.trueColor = p.trueColor
	p.bytes = 0
	if !p.changesOnly {
		fb.appendFrame(&p.sb, width, height)
		p.bytes = p.sb.Len()
		return p.sb.Flush(w)
	}

	// The whole frame, the erase of a restart included, is one synchronized update
	// in one write, so the terminal never shows it half done.
	p.sb.BeginSynchronizedUpdate()
	begun := p.sb.Len()

	// A different region leaves cells of the old one on screen that nothing would
	// overwrite, so it is a reason to start over as well.
	full := !p.valid || width != p.prevW || height != p.prevH
	if full {
		p.sb.EraseInDisplay(2)
	}
	pen := p.appendChanges(fb, width, height, full)
	if p.sb.Len() == begun {
		return nil // nothing changed: nothing to send
	}
	if pen != (style{}) {
		p.sb.ResetStyle() // leave nothing set, for the shell and for whatever draws next
	}
	p.sb.EndSynchronizedUpdate()
	p.bytes = p.sb.Len()

	if err := p.sb.Flush(w); err != nil {
		p.valid = false // some of it may have arrived
		return err
	}
	p.remember(fb, width, height)
	return nil
}

// remember keeps a copy of the part of fb that was just sent.
func (p *presenter) remember(fb *FrameBuffer, width, height int) {
	p.prev = slices.Grow(p.prev[:0], width*height)
	for y := range height {
		p.prev = append(p.prev, fb.frame[y*fb.width:y*fb.width+width]...)
	}
	p.prevW, p.prevH, p.valid = width, height, true
}

// appendChanges builds the sequence that turns what the terminal shows into the part
// of fb that is width x height, and returns the style it leaves set. With full, every
// cell is written; otherwise only the cells that differ from prev, plus the unchanged
// ones between two changes when writing them costs less than skipping over them.
func (p *presenter) appendChanges(fb *FrameBuffer, width, height int, full bool) (pen style) {
	sb := &p.sb
	curX, curY := -1, -1 // where the cursor is, or -1 when that is not known

	for y := range height {
		row := fb.frame[y*fb.width : y*fb.width+width]
		var old []Cell
		if !full {
			old = p.prev[y*width : (y+1)*width]
			if slices.Equal(row, old) {
				continue
			}
		}
		changed := func(x int) bool { return full || row[x] != old[x] }

		for x := 0; x < width; {
			if !changed(x) {
				x++
				continue
			}

			// A run starts here: get the cursor to it.
			switch {
			case curY == y && curX == x:
			case curY == y && curX >= 0 && x > curX:
				sb.appendCSI(x-curX, 'C')
			default:
				sb.MoveTo(y+1, x+1)
			}

			for x < width {
				if !changed(x) {
					// Write the unchanged cells up to the next change if that is cheaper
					// than skipping them, and only if that needs no style change.
					end := x
					for end < width && !changed(end) {
						end++
					}
					if end == width || !gapCheaperToWrite(row[x:end], pen, end-x) {
						break
					}
				}
				c := row[x]
				if st := c.style(); st != pen {
					sb.SetStyle(pen, st)
					pen = st
				}
				r := c.Character
				if r < 0x20 || r >= 0x7f {
					r = safeRune(r)
				}
				sb.PrintRune(r)
				x++
			}

			// The cursor is now at x, which is only used by a later run on this row; a
			// run that ended in the last column has none, as the row is done.
			curX, curY = x, y
		}
	}
	return pen
}

// gapCheaperToWrite reports whether writing the unchanged cells of gap costs no more
// bytes than a cursor move over them would, and needs no change of style.
func gapCheaperToWrite(gap []Cell, pen style, n int) bool {
	jump := len(escapeSeq) + len(strconv.Itoa(n)) + 1 // ESC [ n C
	cost := 0
	for _, c := range gap {
		if c.style() != pen {
			return false
		}
		r := c.Character
		if r < 0x20 || r >= 0x7f {
			r = safeRune(r)
		}
		cost += utf8.RuneLen(r)
		if cost > jump {
			return false
		}
	}
	return true
}

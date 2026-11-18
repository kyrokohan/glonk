package engine

import (
	"io"
	"slices"
	"strconv"
	"unicode/utf8"
)

const (
	escapeSeq          = "\x1b["
	hideCursorSeq      = "\x1b[?25l"
	showCursorSeq      = "\x1b[?25h"
	alternateScreenSeq = "\x1b[?1049h"
	mainScreenSeq      = "\x1b[?1049l"
	disableWrapSeq     = "\x1b[?7l"
	enableWrapSeq      = "\x1b[?7h"
	resetSeq           = "\x1b[0m"

	// Synchronized output: the terminal holds what it is sent between the two codes
	// and shows it all at once. A terminal that does not know the mode ignores them.
	syncBeginSeq = "\x1b[?2026h"
	syncEndSeq   = "\x1b[?2026l"

	// Kitty keyboard protocol: ask for the current flags (a supporting terminal
	// answers ESC [ ? flags u), ask for device attributes (every terminal answers),
	// and pop the flags we pushed.
	queryKeyboardSeq   = "\x1b[?u"
	queryAttributesSeq = "\x1b[c"
	popKeyboardSeq     = "\x1b[<u"

	// Focus reporting: the terminal sends ESC [ I when the window gains focus and
	// ESC [ O when it loses it.
	enableFocusSeq  = "\x1b[?1004h"
	disableFocusSeq = "\x1b[?1004l"

	// Bracketed paste: the terminal wraps what is pasted in ESC [ 200 ~ and ESC [ 201 ~.
	// Mouse reporting: 1003 is every movement, with or without a button, and 1006 the decimal
	// form of a report that has no limit on the column. Turned off in the reverse order.
	enableMouseSeq  = "\x1b[?1003h\x1b[?1006h"
	disableMouseSeq = "\x1b[?1006l\x1b[?1003l"

	enablePasteSeq  = "\x1b[?2004h"
	disablePasteSeq = "\x1b[?2004l"
)

// sequenceBuilder collects the bytes of one write to the terminal. It keeps its
// buffer between uses, so a builder that is reset and filled again each frame
// allocates only until it has grown to the size of a frame.
type sequenceBuilder struct {
	buf []byte

	// trueColor makes SetStyle write an RGB color exactly. Off, it writes the nearest
	// palette color.
	trueColor bool
}

func newSequenceBuilder() *sequenceBuilder {
	return &sequenceBuilder{}
}

// reset empties the builder and keeps its buffer.
func (sb *sequenceBuilder) reset() {
	sb.buf = sb.buf[:0]
}

// String returns what has been built so far.
func (sb *sequenceBuilder) String() string {
	return string(sb.buf)
}

// Len returns how many bytes have been built so far.
func (sb *sequenceBuilder) Len() int {
	return len(sb.buf)
}

// Flush writes everything built so far to w in a single Write. The builder is left
// as it is; reset it to build the next one.
func (sb *sequenceBuilder) Flush(w io.Writer) error {
	_, err := w.Write(sb.buf)
	return err
}

func (sb *sequenceBuilder) Print(s string) {
	sb.buf = append(sb.buf, s...)
}

// PrintRune appends r as UTF-8. A value that is not a valid character comes out as
// U+FFFD, the same as string(r).
func (sb *sequenceBuilder) PrintRune(r rune) {
	sb.buf = utf8.AppendRune(sb.buf, r)
}

// appendCSI starts a control sequence with one number and ends it with final.
func (sb *sequenceBuilder) appendCSI(n int, final byte) {
	sb.buf = append(sb.buf, escapeSeq...)
	sb.buf = strconv.AppendInt(sb.buf, int64(n), 10)
	sb.buf = append(sb.buf, final)
}

func (sb *sequenceBuilder) MoveTo(n, m int) {
	sb.buf = append(sb.buf, escapeSeq...)
	sb.buf = strconv.AppendInt(sb.buf, int64(n), 10)
	sb.buf = append(sb.buf, ';')
	sb.buf = strconv.AppendInt(sb.buf, int64(m), 10)
	sb.buf = append(sb.buf, 'H')
}

func (sb *sequenceBuilder) EraseInDisplay(n int) {
	sb.appendCSI(n, 'J')
}

func (sb *sequenceBuilder) SwitchToAlternateScreen() {
	sb.Print(alternateScreenSeq)
}

func (sb *sequenceBuilder) SwitchToMainScreen() {
	sb.Print(mainScreenSeq)
}

// DisableAutowrap makes a character written in the last column stay there, so a row
// that is too long is cut off at the edge instead of continuing on the next line.
func (sb *sequenceBuilder) DisableAutowrap() {
	sb.Print(disableWrapSeq)
}

// EnableAutowrap turns line wrapping back on, as the shell expects it.
func (sb *sequenceBuilder) EnableAutowrap() {
	sb.Print(enableWrapSeq)
}

func (sb *sequenceBuilder) HideCursor() {
	sb.Print(hideCursorSeq)
}

func (sb *sequenceBuilder) ShowCursor() {
	sb.Print(showCursorSeq)
}

// BeginSynchronizedUpdate asks the terminal to hold back what follows until
// EndSynchronizedUpdate, so that a frame is never seen half drawn.
func (sb *sequenceBuilder) BeginSynchronizedUpdate() {
	sb.Print(syncBeginSeq)
}

// EndSynchronizedUpdate lets the terminal show everything held back since
// BeginSynchronizedUpdate.
func (sb *sequenceBuilder) EndSynchronizedUpdate() {
	sb.Print(syncEndSeq)
}

func (sb *sequenceBuilder) ResetStyle() {
	sb.Print(resetSeq)
}

// Grow reserves room for n more bytes so a frame is built in one allocation
// instead of the builder doubling its way up.
func (sb *sequenceBuilder) Grow(n int) {
	sb.buf = slices.Grow(sb.buf, n)
}

// QueryKeyboardProtocol asks whether the terminal speaks the kitty keyboard protocol.
func (sb *sequenceBuilder) QueryKeyboardProtocol() {
	sb.Print(queryKeyboardSeq)
}

// QueryDeviceAttributes asks for the terminal's device attributes. Sent right after
// the protocol query: every terminal answers it, so its reply marks the end of the wait.
func (sb *sequenceBuilder) QueryDeviceAttributes() {
	sb.Print(queryAttributesSeq)
}

// PushKeyboardFlags turns on the given kitty keyboard protocol flags until popped.
func (sb *sequenceBuilder) PushKeyboardFlags(flags int) {
	sb.buf = append(sb.buf, escapeSeq...)
	sb.buf = append(sb.buf, '>')
	sb.buf = strconv.AppendInt(sb.buf, int64(flags), 10)
	sb.buf = append(sb.buf, 'u')
}

// PopKeyboardFlags restores the keyboard mode that was active before the last push.
func (sb *sequenceBuilder) PopKeyboardFlags() {
	sb.Print(popKeyboardSeq)
}

// EnableFocusReporting asks the terminal to report when its window gains or loses focus.
func (sb *sequenceBuilder) EnableFocusReporting() {
	sb.Print(enableFocusSeq)
}

// EnableBracketedPaste asks the terminal to mark what is pasted, so that it is not read as typing.
func (sb *sequenceBuilder) EnableBracketedPaste() {
	sb.Print(enablePasteSeq)
}

// DisableBracketedPaste turns bracketed paste back off.
func (sb *sequenceBuilder) DisableBracketedPaste() {
	sb.Print(disablePasteSeq)
}

// EnableMouse asks the terminal to report the pointer: where it is when it moves, a button pressed
// and let go, and the wheel.
func (sb *sequenceBuilder) EnableMouse() {
	sb.Print(enableMouseSeq)
}

// DisableMouse turns mouse reporting back off.
func (sb *sequenceBuilder) DisableMouse() {
	sb.Print(disableMouseSeq)
}

// DisableFocusReporting turns focus reporting back off.
func (sb *sequenceBuilder) DisableFocusReporting() {
	sb.Print(disableFocusSeq)
}

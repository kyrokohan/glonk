package engine

import (
	"bytes"
	"strings"
	"time"
	"unicode/utf8"
)

// Bracketed paste: a terminal that has been asked wraps what is pasted in ESC [ 200 ~ and ESC [ 201 ~,
// so that a program can tell a paste from typing. glonk asks for it as it takes the terminal over, and
// reads it whether it asked or not.
const (
	// maxPaste is the most of a paste, or of the pastes in a frame, that is kept, in bytes. What is
	// beyond it is dropped, so that input nobody reads cannot fill the memory.
	maxPaste = 1 << 20

	// pasteTimeout is how long a paste may stall, with its end marker not yet read, before it is
	// delivered as it stands. A paste arrives in reads as fast as the terminal sends them, and a slow
	// link can pause; a longer wait than an Escape key's is right, and a paste that never ends must not
	// swallow every key after it, Ctrl-C included.
	pasteTimeout = 500 * time.Millisecond
)

// pasteEndMarker closes a paste.
var pasteEndMarker = []byte("\x1b[201~")

// collect takes the part of data that belongs to the paste being collected, and reports how many
// bytes that was and whether the end marker was among them. The marker can be split between reads,
// so the last bytes of what is collected are held back until the next read shows what they are.
func (p *keyParser) collect(data []byte) (n int, done bool) {
	p.scan = append(append(p.scan[:0], p.hold...), data...)
	if i := bytes.Index(p.scan, pasteEndMarker); i >= 0 {
		p.addPaste(p.scan[:i])
		n = i + len(pasteEndMarker) - len(p.hold) // the bytes of data, not of what was held
		return n, true
	}
	keep := min(len(pasteEndMarker)-1, len(p.scan))
	p.addPaste(p.scan[:len(p.scan)-keep])
	p.hold = append(p.hold[:0], p.scan[len(p.scan)-keep:]...)
	return len(data), false
}

// addPaste adds to what has been collected, up to maxPaste.
func (p *keyParser) addPaste(b []byte) {
	if room := maxPaste - len(p.paste); len(b) > room {
		b = b[:max(room, 0)]
	}
	p.paste = append(p.paste, b...)
}

// take ends the paste and returns its text.
func (p *keyParser) take() string {
	text := pasteText(string(p.paste))
	p.inPaste = false
	p.hold = p.hold[:0]
	if cap(p.paste) > 1<<16 {
		p.paste = nil // a big paste does not leave its memory behind
	} else {
		p.paste = p.paste[:0]
	}
	return text
}

// pasteText is the text of a paste as Input.Paste gives it: no more than maxPaste bytes, valid
// UTF-8 with U+FFFD for what is not, and every line break a newline, since a terminal sends a
// paste's line breaks as carriage returns or as carriage returns and newlines.
func pasteText(raw string) string {
	if len(raw) > maxPaste {
		raw = raw[:maxPaste]
	}
	// A run of bytes that are not UTF-8 becomes three, which is more than it was where they are single
	// bytes between characters, so the text can grow, and it is cut again, at a character.
	s := strings.ToValidUTF8(raw, "\ufffd")
	if len(s) > maxPaste {
		s = s[:maxPaste]
		for len(s) > 0 {
			if r, size := utf8.DecodeLastRuneInString(s); r != utf8.RuneError || size != 1 {
				break
			}
			s = s[:len(s)-1]
		}
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// addPaste adds the text of a paste to this frame's, up to maxPaste.
func (in *Input) addPaste(text string) {
	if in.paste == "" {
		in.paste = text
		return
	}
	cut := min(maxPaste-len(in.paste), len(text))
	for cut > 0 && cut < len(text) && !utf8.RuneStart(text[cut]) {
		cut-- // not in the middle of a character
	}
	if cut > 0 {
		in.paste += text[:cut]
	}
}

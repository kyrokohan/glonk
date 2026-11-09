package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
	"testing"
)

// change replaces n random cells of fb with random ones.
func change(rng *rand.Rand, fb *FrameBuffer, n int) {
	for range n {
		fb.Set(rng.IntN(fb.Width()), rng.IntN(fb.Height()), randomCell(rng))
	}
}

// scribble writes junk straight onto the terminal, as something other than glonk might.
func scribble(rng *rand.Rand, v *vterm) {
	for range 1 + rng.IntN(6) {
		v.grid[rng.IntN(len(v.grid))] = vcell{r: 'J', st: style{fg: Palette(uint8(1 + rng.IntN(200)))}}
	}
}

func sizedFrame(t testing.TB, w, h int) *FrameBuffer {
	t.Helper()
	fb, err := NewFrameBuffer(w, h)
	if err != nil {
		t.Fatal(err)
	}
	return fb
}

// The main check on the presenter: however a run goes, after every frame the terminal
// shows exactly the part of the frame that was meant to be shown and nothing else. A
// run mixes small changes, whole new frames, windows of other sizes, frames larger than
// the window, junk written behind its back, and invalidation.
func TestPresenterTerminalAlwaysShowsTheFrame(t *testing.T) {
	for _, trueColor := range []bool{false, true} {
		t.Run(fmt.Sprintf("true color %v", trueColor), func(t *testing.T) {
			presenterRuns(t, trueColor)
		})
	}
}

func presenterRuns(t *testing.T, trueColor bool) {
	rng := rand.New(rand.NewPCG(1, 2))
	for run := range 600 {
		termW, termH := 1+rng.IntN(20), 1+rng.IntN(8)
		v := newVterm(t, termW, termH)
		v.trueColor = trueColor
		p := newPresenter(true, trueColor)

		fb := randomFrame(t, rng, 1+rng.IntN(termW+3), 1+rng.IntN(termH+3))
		clipW, clipH := termW, termH

		for step := range 1 + rng.IntN(12) {
			what := rng.IntN(10)
			switch {
			case what <= 4:
				change(rng, fb, 1+rng.IntN(4))
			case what == 5:
				fb = randomFrame(t, rng, fb.Width(), fb.Height())
			case what == 6:
				clipW, clipH = 1+rng.IntN(termW), 1+rng.IntN(termH)
			case what == 7:
				scribble(rng, v)
				p.invalidate()
			case what == 8:
				fb = randomFrame(t, rng, 1+rng.IntN(termW+3), 1+rng.IntN(termH+3))
			default:
				// the same frame again
			}

			if err := p.present(v, fb, clipW, clipH); err != nil {
				t.Fatal(err)
			}

			if d := v.screenDiff(fb, clipW, clipH); d != "" {
				t.Fatalf("run %d step %d (%d change kind, terminal %dx%d, frame %dx%d, clip %dx%d): %s", run, step, what, termW, termH, fb.Width(), fb.Height(), clipW, clipH, d)
			}
			if v.inSync {
				t.Fatalf("run %d step %d: wanted no synchronized update left open", run, step)
			}
		}
	}
}

func TestPresenterSendsNothingForAFrameThatDidNotChange(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	fb := randomFrame(t, rng, 30, 10)
	p := newPresenter(true, false)
	var out countingWriter
	if err := p.present(&out, fb, 30, 10); err != nil {
		t.Fatal(err)
	}
	first := out.bytes

	for range 3 {
		if err := p.present(&out, fb, 30, 10); err != nil {
			t.Fatal(err)
		}
	}

	if out.bytes != first || out.writes != 1 {
		t.Errorf("wanted only the first frame written, got %d bytes in %d writes after a first frame of %d", out.bytes, out.writes, first)
	}
}

func TestPresenterOneChangedCellCostsAFewBytes(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	fb := randomFrame(t, rng, 80, 24)
	p := newPresenter(true, false)
	if err := p.present(&countingWriter{}, fb, 80, 24); err != nil {
		t.Fatal(err)
	}

	fb.Set(40, 12, Cell{Character: 'X', Foreground: Palette(208), Background: Palette(4), Attrs: Bold})
	var out countingWriter
	if err := p.present(&out, fb, 80, 24); err != nil {
		t.Fatal(err)
	}

	if out.bytes == 0 || out.bytes > 60 || out.writes != 1 { // the cell, its style and cursor move, and the update codes
		t.Errorf("wanted one small write for one cell, got %d bytes in %d writes", out.bytes, out.writes)
	}
}

func TestPresenterFirstFrameErasesTheDisplayAndSendsEverything(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	fb := randomFrame(t, rng, 12, 5)
	var out bytes.Buffer

	if err := newPresenter(true, false).present(&out, fb, 12, 5); err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(out.String(), "\x1b[?2026h\x1b[2J") {
		t.Errorf("wanted the display erased first, got %q", out.String()[:min(out.Len(), 12)])
	}
	v := newVterm(t, 12, 5)
	scribble(rng, v)
	v.Write(out.Bytes())
	if d := v.screenDiff(fb, 12, 5); d != "" {
		t.Errorf("wanted the whole frame, got %s", d)
	}
}

func TestPresenterInvalidateDrawsTheSameFrameAgainInFull(t *testing.T) {
	rng := rand.New(rand.NewPCG(9, 10))
	fb := randomFrame(t, rng, 12, 5)
	p := newPresenter(true, false)
	v := newVterm(t, 12, 5)
	if err := p.present(v, fb, 12, 5); err != nil {
		t.Fatal(err)
	}
	scribble(rng, v)

	p.invalidate()
	var out bytes.Buffer
	if err := p.present(io2(v, &out), fb, 12, 5); err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(out.String(), "\x1b[?2026h\x1b[2J") {
		t.Errorf("wanted an erase first, got %q", out.String()[:min(out.Len(), 12)])
	}
	if d := v.screenDiff(fb, 12, 5); d != "" {
		t.Errorf("wanted the junk gone, got %s", d)
	}
}

// io2 writes to both w1 and w2.
func io2(w1 *vterm, w2 *bytes.Buffer) *teeWriter {
	return &teeWriter{v: w1, b: w2}
}

type teeWriter struct {
	v *vterm
	b *bytes.Buffer
}

func (t *teeWriter) Write(p []byte) (int, error) {
	t.b.Write(p)
	return t.v.Write(p)
}

// flakyWriter fails its first Write and passes the rest on.
type flakyWriter struct {
	next   *vterm
	failed bool
}

func (f *flakyWriter) Write(p []byte) (int, error) {
	if !f.failed {
		f.failed = true
		return 0, errors.New("disk full")
	}
	return f.next.Write(p)
}

func TestPresenterAFailedWriteMakesTheNextFrameWhole(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	fb := randomFrame(t, rng, 12, 5)
	v := newVterm(t, 12, 5)
	p := newPresenter(true, false)
	if err := p.present(v, fb, 12, 5); err != nil {
		t.Fatal(err)
	}

	change(rng, fb, 3)
	w := &flakyWriter{next: v}
	if err := p.present(w, fb, 12, 5); err == nil {
		t.Fatalf("wanted the write error back")
	}
	// Part of a failed write may have reached the terminal; make that so.
	scribble(rng, v)
	if err := p.present(w, fb, 12, 5); err != nil {
		t.Fatal(err)
	}

	if d := v.screenDiff(fb, 12, 5); d != "" {
		t.Errorf("wanted the frame whole after the failure, got %s", d)
	}
}

func TestFullPresenterSendsEveryFrameWholeAndNothingElse(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 14))
	fb := randomFrame(t, rng, 16, 6)
	var want bytes.Buffer
	if err := fb.presentClipped(&want, 10, 4); err != nil {
		t.Fatal(err)
	}
	p := newPresenter(false, false)

	for i := range 3 {
		var got bytes.Buffer
		if err := p.present(&got, fb, 10, 4); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Bytes(), want.Bytes()) {
			t.Fatalf("frame %d: wanted exactly what presentClipped writes,\n%q\ngot\n%q", i, want.String(), got.String())
		}
		p.invalidate() // makes no difference to a presenter that always sends everything
	}
	for _, other := range []string{"\x1b[2J", "\x1b[?2026"} {
		if strings.Contains(want.String(), other) {
			t.Errorf("wanted a full frame to hold no %q", other)
		}
	}
}

// A cursor jump does not change the colors, so a run of changes in one style sets it once.
func TestPresenterCarriesTheStyleAcrossJumps(t *testing.T) {
	fb := sizedFrame(t, 30, 6)
	p := newPresenter(true, false)
	if err := p.present(&countingWriter{}, fb, 30, 6); err != nil {
		t.Fatal(err)
	}
	st := Cell{Character: 'x', Foreground: Palette(208), Attrs: Bold}
	fb.Set(2, 0, st)
	fb.Set(20, 3, st)
	fb.Set(28, 5, st)
	var out bytes.Buffer

	if err := p.present(&out, fb, 30, 6); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if n := strings.Count(got, "m"); n != 2 { // bold and the color in one sequence, and the final reset
		t.Errorf("wanted the style set once and reset once, got %d sequences in %q", n, got)
	}
	if !strings.HasSuffix(got, "\x1b[0m\x1b[?2026l") {
		t.Errorf("wanted the style reset at the end of the frame, inside the update, got %q", got)
	}
}

func TestPresenterJoinsShortGapsAndJumpsOverLongOnes(t *testing.T) {
	tests := []struct {
		name     string
		second   int
		wantJump string // "" when the gap is written out
	}{
		{"a gap of one", 4, ""},
		{"a gap of four costs the same as a jump", 7, ""},
		{"a gap of five is jumped", 8, "\x1b[5C"},
		{"a gap of nine is jumped", 12, "\x1b[9C"},
		{"a gap of ten is jumped", 13, "\x1b[10C"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := sizedFrame(t, 30, 1)
			for x := range 30 {
				fb.Set(x, 0, Cell{Character: '.'})
			}
			p := newPresenter(true, false)
			if err := p.present(&countingWriter{}, fb, 30, 1); err != nil {
				t.Fatal(err)
			}
			fb.Set(2, 0, Cell{Character: 'A'})
			fb.Set(tt.second, 0, Cell{Character: 'B'})
			var out bytes.Buffer

			if err := p.present(&out, fb, 30, 1); err != nil {
				t.Fatal(err)
			}

			got := out.String()
			if tt.wantJump == "" && strings.Contains(got, "C") {
				t.Errorf("wanted the gap written out with no jump, got %q", got)
			}
			if tt.wantJump != "" && !strings.Contains(got, tt.wantJump) {
				t.Errorf("wanted the jump %q, got %q", tt.wantJump, got)
			}
			v := newVterm(t, 30, 1)
			base := sizedFrame(t, 30, 1)
			for x := range 30 {
				base.Set(x, 0, Cell{Character: '.'})
			}
			base.present(v)
			v.Write(out.Bytes())
			if d := v.screenDiff(fb, 30, 1); d != "" {
				t.Errorf("wanted the frame, got %s", d)
			}
		})
	}
}

// A gap whose cells have another style cannot be written without changing it; that costs
// more than the jump, so the jump is used however short the gap.
func TestPresenterDoesNotWriteAGapInAnotherStyle(t *testing.T) {
	fb := sizedFrame(t, 12, 1)
	for x := range 12 {
		fb.Set(x, 0, Cell{Character: '.', Foreground: Palette(2)})
	}
	p := newPresenter(true, false)
	if err := p.present(&countingWriter{}, fb, 12, 1); err != nil {
		t.Fatal(err)
	}
	fb.Set(2, 0, Cell{Character: 'A'})
	fb.Set(4, 0, Cell{Character: 'B'})
	var out bytes.Buffer

	if err := p.present(&out, fb, 12, 1); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "\x1b[1C") {
		t.Errorf("wanted a jump over the one cell in another style, got %q", out.String())
	}
}

// The cursor stays in the last column when something is written there, so what comes next
// must be placed by position, not by a move forward.
func TestPresenterPlacesTheCellAfterTheLastColumnByPosition(t *testing.T) {
	rng := rand.New(rand.NewPCG(15, 16))
	fb := randomFrame(t, rng, 6, 3)
	v := newVterm(t, 6, 3)
	p := newPresenter(true, false)
	if err := p.present(v, fb, 6, 3); err != nil {
		t.Fatal(err)
	}

	fb.Set(5, 0, Cell{Character: 'L'})
	fb.Set(5, 1, Cell{Character: 'M'})
	fb.Set(0, 2, Cell{Character: 'N'})
	fb.Set(1, 2, Cell{Character: 'O'})
	if err := p.present(v, fb, 6, 3); err != nil {
		t.Fatal(err)
	}

	if d := v.screenDiff(fb, 6, 3); d != "" {
		t.Error(d)
	}
}

// Run draws into one FrameBuffer over and over. If the presenter kept that frame instead
// of a copy, it would always find nothing changed.
func TestPresenterKeepsACopyOfTheFrame(t *testing.T) {
	fb := sizedFrame(t, 10, 2)
	p := newPresenter(true, false)
	v := newVterm(t, 10, 2)
	fb.Set(1, 1, Cell{Character: 'a'})
	if err := p.present(v, fb, 10, 2); err != nil {
		t.Fatal(err)
	}

	fb.Clear()
	fb.Set(7, 0, Cell{Character: 'b'})
	if err := p.present(v, fb, 10, 2); err != nil {
		t.Fatal(err)
	}

	if d := v.screenDiff(fb, 10, 2); d != "" {
		t.Error(d)
	}
}

func TestPresenterShowsOnlyThePartOfALargeFrameThatFits(t *testing.T) {
	rng := rand.New(rand.NewPCG(17, 18))
	fb := randomFrame(t, rng, 12, 6)
	v := newVterm(t, 7, 3)
	p := newPresenter(true, false)

	if err := p.present(v, fb, 7, 3); err != nil {
		t.Fatal(err)
	}
	change(rng, fb, 5)
	if err := p.present(v, fb, 7, 3); err != nil {
		t.Fatal(err)
	}

	if d := v.screenDiff(fb, 7, 3); d != "" {
		t.Error(d)
	}
}

func FuzzPresenterChanges(f *testing.F) {
	seed := func(runes ...rune) {
		var b []byte
		for _, r := range runes {
			b = binary.LittleEndian.AppendUint32(b, uint32(r))
		}
		f.Add(b, uint8(2), uint8(1), uint8(5), false)
		f.Add(b, uint8(2), uint8(1), uint8(5), true)
	}
	seed('a', 'b', 0, 0x1b, '\n', 0x9b, '中', '😀', 0x301, 0xd800, -1, '─', '▀', '⣿')
	seed('x', 'x', 'x', 'y', 'y', 'x', 'x', 'x', 'x', 'x', 'y', 'x', 'x', 'x', 'x', 'x')

	f.Fuzz(func(t *testing.T, data []byte, clipW, clipH, mask uint8, trueColor bool) {
		const w, h = 8, 2
		fb := sizedFrame(t, w, h)
		fill := func(offset int) {
			for i := range w * h {
				r := 'x'
				var fg, bg, attrs byte
				if k := (4*i + offset) % max(len(data), 1); len(data) >= 4 && k+4 <= len(data) {
					r = rune(int32(binary.LittleEndian.Uint32(data[k:])))
					fg, bg, attrs = data[k], data[(k+1)%len(data)], data[(k+2)%len(data)]
				}
				fb.Set(i%w, i/w, Cell{Character: r, Foreground: pix(fg), Background: RGB(bg, fg, attrs), Attrs: Attr(attrs & 15)})
			}
		}
		v := newVterm(t, w, h)
		v.trueColor = trueColor
		p := newPresenter(true, trueColor)
		cw, ch := 1+int(clipW)%w, 1+int(clipH)%h

		fill(0)
		if err := p.present(v, fb, cw, ch); err != nil {
			t.Fatal(err)
		}
		if d := v.screenDiff(fb, cw, ch); d != "" {
			t.Fatal("first frame: " + d)
		}

		// Change only some of the cells: those whose bit is set in mask.
		old := append([]Cell(nil), fb.frame...)
		fill(7)
		for i := range w * h {
			if mask&(1<<(i%8)) == 0 {
				fb.frame[i] = old[i]
			}
		}
		if err := p.present(v, fb, cw, ch); err != nil {
			t.Fatal(err)
		}
		if d := v.screenDiff(fb, cw, ch); d != "" {
			t.Fatalf("second frame (mask %08b): %s", mask, d)
		}
	})
}

// A cell that differs from the last frame in one thing only, whichever it is, is sent.
func TestPresenterNoticesEveryKindOfChange(t *testing.T) {
	base := Cell{Character: 'a', Foreground: Palette(1), Background: Palette(2), Attrs: Bold}
	tests := []struct {
		name string
		to   Cell
	}{
		{"character", Cell{Character: 'b', Foreground: Palette(1), Background: Palette(2), Attrs: Bold}},
		{"foreground", Cell{Character: 'a', Foreground: Palette(3), Background: Palette(2), Attrs: Bold}},
		{"foreground to default", Cell{Character: 'a', Background: Palette(2), Attrs: Bold}},
		{"background", Cell{Character: 'a', Foreground: Palette(1), Background: Palette(3), Attrs: Bold}},
		{"background to default", Cell{Character: 'a', Foreground: Palette(1), Attrs: Bold}},
		{"attribute added", Cell{Character: 'a', Foreground: Palette(1), Background: Palette(2), Attrs: Bold | Underline}},
		{"attribute removed", Cell{Character: 'a', Foreground: Palette(1), Background: Palette(2)}},
		{"RGB for palette", Cell{Character: 'a', Foreground: RGB(255, 0, 0), Background: Palette(2), Attrs: Bold}},
	}
	for _, tt := range tests {
		for _, trueColor := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, true color %v", tt.name, trueColor), func(t *testing.T) {
				changeIsSent(t, base, tt.to, trueColor)
			})
		}
	}
}

func changeIsSent(t *testing.T, base, to Cell, trueColor bool) {
	fb := sizedFrame(t, 5, 2)
	for i := range 10 {
		fb.Set(i%5, i/5, base)
	}
	v := newVterm(t, 5, 2)
	v.trueColor = trueColor
	p := newPresenter(true, trueColor)
	if err := p.present(v, fb, 5, 2); err != nil {
		t.Fatal(err)
	}

	fb.Set(2, 1, to)
	var out countingWriter
	if err := p.present(io.MultiWriter(v, &out), fb, 5, 2); err != nil {
		t.Fatal(err)
	}

	if out.bytes == 0 {
		t.Errorf("wanted the change sent")
	}
	if d := v.screenDiff(fb, 5, 2); d != "" {
		t.Error(d)
	}
}

func TestPresenterWrapsAFrameInOneSynchronizedUpdate(t *testing.T) {
	rng := rand.New(rand.NewPCG(19, 20))
	fb := randomFrame(t, rng, 20, 6)
	p := newPresenter(true, false)
	var out bytes.Buffer
	count := &countingWriter{}
	w := io.MultiWriter(&out, count)

	for _, step := range []struct {
		name    string
		prepare func()
	}{
		{"the first frame", func() {}},
		{"a small change", func() { change(rng, fb, 2) }},
		{"after invalidate", func() { p.invalidate() }},
	} {
		name := step.name
		out.Reset()
		*count = countingWriter{}
		step.prepare()

		if err := p.present(w, fb, 20, 6); err != nil {
			t.Fatal(err)
		}

		got := out.String()
		if !strings.HasPrefix(got, "\x1b[?2026h") || !strings.HasSuffix(got, "\x1b[?2026l") {
			t.Errorf("%s: wanted the update to begin and end the write, got %q", name, got)
		}
		if strings.Count(got, "\x1b[?2026h") != 1 || strings.Count(got, "\x1b[?2026l") != 1 || count.writes != 1 {
			t.Errorf("%s: wanted one update in one write, got %d begins, %d ends, %d writes", name, strings.Count(got, "\x1b[?2026h"), strings.Count(got, "\x1b[?2026l"), count.writes)
		}
		if i := strings.Index(got, "\x1b[2J"); i >= 0 && !(i > strings.Index(got, "\x1b[?2026h") && i < strings.LastIndex(got, "\x1b[?2026l")) {
			t.Errorf("%s: wanted the erase inside the update, got %q", name, got)
		}
	}

	*count = countingWriter{}
	if err := p.present(count, fb, 20, 6); err != nil || count.writes != 0 {
		t.Errorf("wanted nothing written, not even an empty update, for a frame that did not change; got %d writes (%v)", count.writes, err)
	}
}

func TestFullPresenterNeverSynchronizes(t *testing.T) {
	rng := rand.New(rand.NewPCG(21, 22))
	fb := randomFrame(t, rng, 10, 4)
	var out bytes.Buffer

	if err := newPresenter(false, false).present(&out, fb, 10, 4); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), "\x1b[?2026") {
		t.Errorf("wanted a headless frame to hold no synchronized update, got %q", out.String())
	}
}

package record

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"glonk.dev/engine"
)

// probe is a game that notes what it is given and draws the number of the frame.
type probe struct {
	dts     []float64
	down    []bool // whether x was down, frame by frame
	pressed []bool // whether x was newly pressed, frame by frame
	quitAt  int    // the update that returns failWith or quits, counted from 1; 0 for never
	fail    error
	size    [2]int
	stats   int
}

func (p *probe) Update(dt float64, in *engine.Input) error {
	p.dts = append(p.dts, dt)
	p.down = append(p.down, in.IsDown('x'))
	p.pressed = append(p.pressed, in.Pressed('x'))
	if p.quitAt != 0 && len(p.dts) == p.quitAt {
		if p.fail != nil {
			return p.fail
		}
		return engine.ErrQuit
	}
	return nil
}

func (p *probe) Draw(fb *engine.FrameBuffer) {
	fb.DrawText(0, 0, fmt.Sprintf("frame %d", len(p.dts)), engine.Color{})
}

func (p *probe) Resize(w, h int)    { p.size = [2]int{w, h} }
func (p *probe) Stats(engine.Stats) { p.stats++ }

// quick is a tape short enough for a test: 6 frames in 30 ms.
func quick(steps ...[]Step) Tape {
	return Tape{Width: 12, Height: 2, FPS: 200, Frames: 6, Steps: Steps(steps...)}
}

func TestPlayGivesOneFrameForEachFrameOfTheTape(t *testing.T) {
	g := &probe{}
	got, err := Play(g, quick())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 || len(g.dts) != 6 {
		t.Fatalf("wanted 6 frames from 6 updates, got %d frames from %d updates", len(got), len(g.dts))
	}
	for i, f := range got {
		if !bytes.HasPrefix(f, []byte(frameStart)) {
			t.Errorf("frame %d: wanted it to start by moving the cursor home, got %q", i, f)
		}
		if want := fmt.Sprintf("frame %d", i+1); !bytes.Contains(f, []byte(want)) {
			t.Errorf("frame %d: wanted it to show %q, got %q", i, want, f)
		}
	}
}

// The game is given the tape's time step, not the clock's, so that it moves the same on any machine.
func TestPlayGivesTheGameTheSameTimeStepEveryFrame(t *testing.T) {
	g := &probe{}
	if _, err := Play(g, quick()); err != nil {
		t.Fatal(err)
	}
	for i, dt := range g.dts {
		if dt != 1.0/200 {
			t.Errorf("update %d: wanted a time step of 1/200 s, got %v", i, dt)
		}
	}
}

func TestStepsReachTheGameOnTheFrameTheyName(t *testing.T) {
	g := &probe{}
	if _, err := Play(g, quick(Hold('x', 2, 4))); err != nil {
		t.Fatal(err)
	}
	if want := []bool{false, false, true, true, false, false}; !reflect.DeepEqual(g.down, want) {
		t.Errorf("wanted x down on frames 2 and 3, got %v", g.down)
	}
	if want := []bool{false, false, true, false, false, false}; !reflect.DeepEqual(g.pressed, want) {
		t.Errorf("wanted x newly pressed on frame 2 only, got %v", g.pressed)
	}
}

func TestTapIsDownForOneFrame(t *testing.T) {
	g := &probe{}
	if _, err := Play(g, quick(Tap('x', 0), Tap('x', 5))); err != nil {
		t.Fatal(err)
	}
	if want := []bool{true, false, false, false, false, true}; !reflect.DeepEqual(g.down, want) {
		t.Errorf("wanted x down on frames 0 and 5 only, got %v", g.down)
	}
}

func TestTheSameTapeGivesTheSameFrames(t *testing.T) {
	first, err := Play(&probe{}, quick(Hold('x', 1, 3)))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Play(&probe{}, quick(Hold('x', 1, 3)))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("wanted the same frames from the same tape, got\n%q\nand\n%q", first, second)
	}
}

func TestAGameThatQuitsEndsTheTapeEarly(t *testing.T) {
	g := &probe{quitAt: 3}
	got, err := Play(g, quick())
	if err != nil {
		t.Fatalf("wanted quitting to end the tape without an error, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("wanted the 2 frames drawn before the game quit, got %d", len(got))
	}
}

func TestAnErrorOfTheGameIsReturned(t *testing.T) {
	broken := errors.New("broken")
	if _, err := Play(&probe{quitAt: 2, fail: broken}, quick()); !errors.Is(err, broken) {
		t.Errorf("wanted the game's error, got %v", err)
	}
}

func TestTheGameLearnsItsSizeAndHowItRuns(t *testing.T) {
	g := &probe{}
	if _, err := Play(g, quick()); err != nil {
		t.Fatal(err)
	}
	if g.size != [2]int{12, 2} {
		t.Errorf("wanted the game told a size of 12 by 2, got %v", g.size)
	}
	if g.stats != 6 {
		t.Errorf("wanted the game told how each of 6 frames went, got %d", g.stats)
	}
}

// tinted is a game that draws one color no terminal palette has.
type tinted struct{}

func (tinted) Update(float64, *engine.Input) error { return nil }
func (tinted) Draw(fb *engine.FrameBuffer) {
	fb.DrawText(0, 0, "x", engine.RGB(1, 2, 3))
}

// A recording keeps colors as the game gave them, not as the nearest of the 256.
func TestPlayKeepsExactColors(t *testing.T) {
	got, err := Play(tinted{}, quick())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got[0], []byte("38;2;1;2;3")) {
		t.Errorf("wanted the color sent as red 1, green 2, blue 3, got %q", got[0])
	}
}

// A tape is played at its own speed, since a game may show how fast it runs: 6 frames at 200 a
// second cannot be over before 5 of the gaps between them have passed.
func TestPlayTakesAsLongAsTheTape(t *testing.T) {
	start := time.Now()
	if _, err := Play(&probe{}, quick()); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 25*time.Millisecond {
		t.Errorf("wanted the tape to take 25 ms or more, it took %v", took)
	}
}

// A game that is neither a Resizer nor a StatsReceiver must still play.
type plain struct{ updates int }

func (p *plain) Update(float64, *engine.Input) error { p.updates++; return nil }
func (p *plain) Draw(*engine.FrameBuffer)            {}

func TestAGameWithoutTheOptionalMethodsPlays(t *testing.T) {
	g := &plain{}
	got, err := Play(g, quick())
	if err != nil || len(got) != 6 || g.updates != 6 {
		t.Errorf("wanted 6 frames and no error, got %d frames from %d updates and %v", len(got), g.updates, err)
	}
}

func TestATapeNeedsASizeARateAndALength(t *testing.T) {
	for name, tape := range map[string]Tape{
		"no width":   {Height: 2, FPS: 200, Frames: 6},
		"no height":  {Width: 12, FPS: 200, Frames: 6},
		"no rate":    {Width: 12, Height: 2, Frames: 6},
		"no frames":  {Width: 12, Height: 2, FPS: 200},
		"minus rate": {Width: 12, Height: 2, FPS: -1, Frames: 6},
	} {
		g := &probe{}
		if _, err := Play(g, tape); err == nil || len(g.dts) != 0 {
			t.Errorf("%s: wanted an error and no update, got %v after %d updates", name, err, len(g.dts))
		}
	}
}

func TestFramesKeepsAFrameWrittenInPieces(t *testing.T) {
	var f frames
	for _, piece := range []string{frameStart + "one", frameStart + "two", " more", frameStart + "three"} {
		if n, err := f.Write([]byte(piece)); n != len(piece) || err != nil {
			t.Fatalf("wanted the whole piece taken, got %d of %d and %v", n, len(piece), err)
		}
	}
	want := [][]byte{[]byte(frameStart + "one"), []byte(frameStart + "two more"), []byte(frameStart + "three")}
	if !reflect.DeepEqual([][]byte(f), want) {
		t.Errorf("wanted three frames with the loose piece on the second, got %q", [][]byte(f))
	}
}

// What frames keeps must not change when the writer uses its buffer again.
func TestFramesCopiesWhatItIsGiven(t *testing.T) {
	var f frames
	buf := []byte(frameStart + "a")
	f.Write(buf)
	buf[len(buf)-1] = 'b'
	if got := string(f[0]); got != frameStart+"a" {
		t.Errorf("wanted the frame as it was written, got %q", got)
	}
}

// castLines reads a cast back: its header, and its events.
func castLines(t *testing.T, cast []byte) (map[string]int, [][]any) {
	t.Helper()
	sc := bufio.NewScanner(bytes.NewReader(cast))
	sc.Buffer(nil, 1<<24)
	var header map[string]int
	var events [][]any
	for sc.Scan() {
		if header == nil {
			if err := json.Unmarshal(sc.Bytes(), &header); err != nil {
				t.Fatalf("header: %v", err)
			}
			continue
		}
		var e []any
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("event %d: %v", len(events), err)
		}
		events = append(events, e)
	}
	return header, events
}

func TestCastIsAnAsciicastOfTheFrames(t *testing.T) {
	tape := Tape{Width: 12, Height: 2, FPS: 4, Frames: 3}
	played := [][]byte{[]byte(frameStart + "a\x1b[0m"), []byte(frameStart + "b"), []byte(frameStart + "▀")}
	var buf bytes.Buffer
	if err := Cast(&buf, tape, played); err != nil {
		t.Fatal(err)
	}
	header, events := castLines(t, buf.Bytes())
	if want := map[string]int{"version": 2, "width": 12, "height": 2}; !reflect.DeepEqual(header, want) {
		t.Errorf("wanted the header %v, got %v", want, header)
	}
	if len(events) != 3 {
		t.Fatalf("wanted 3 events, got %d", len(events))
	}
	for i, e := range events {
		want := []any{float64(i) / 4, "o", string(played[i])}
		if !reflect.DeepEqual(e, want) {
			t.Errorf("event %d: wanted %q, got %q", i, want, e)
		}
	}
	if strings.Count(buf.String(), "\n") != 4 {
		t.Errorf("wanted 4 lines, one for the header and one for each frame, got %q", buf.String())
	}
}

func TestCastNeedsTheRate(t *testing.T) {
	if err := Cast(&bytes.Buffer{}, Tape{Width: 12, Height: 2}, nil); err == nil {
		t.Error("wanted an error for a tape with no frames a second: the times cannot be worked out")
	}
}

func TestSaveWritesTheCastWhereItIsTold(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "casts", "today")
	if err := save(dir, "probe", &probe{}, quick()); err != nil {
		t.Fatal(err)
	}
	cast, err := os.ReadFile(filepath.Join(dir, "probe.cast"))
	if err != nil {
		t.Fatal(err)
	}
	header, events := castLines(t, cast)
	if header["width"] != 12 || header["height"] != 2 || len(events) != 6 {
		t.Errorf("wanted a cast of 12 by 2 with 6 frames, got %v with %d", header, len(events))
	}
}

func TestSaveReturnsWhatWentWrong(t *testing.T) {
	broken := errors.New("broken")
	if err := save(t.TempDir(), "probe", &probe{quitAt: 1, fail: broken}, quick()); !errors.Is(err, broken) {
		t.Errorf("wanted the game's error, got %v", err)
	}
	file := filepath.Join(t.TempDir(), "a file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := save(file, "probe", &probe{}, quick()); err == nil {
		t.Error("wanted an error for a directory that is a file")
	}
}

// Without the flag Save skips, so that make check does not spend the seconds a tape lasts.
func TestSaveSkipsWithoutTheFlag(t *testing.T) {
	if *dir != "" {
		t.Skip("the flag is set")
	}
	g := &probe{}
	t.Run("skipped", func(t *testing.T) {
		Save(t, "probe", g, quick())
		t.Error("wanted Save to skip the test")
	})
	if len(g.dts) != 0 {
		t.Errorf("wanted nothing played, got %d updates", len(g.dts))
	}
}

package engine

import (
	"bytes"
	"context"
	"io"
	"math"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// statsGame is a Game that keeps what Run tells it about each frame, and can be slowed down in
// each part of a frame. It quits on the update after the last frame it wants.
type statsGame struct {
	frames                    int // frames to run
	updateWait, drawWait      time.Duration
	out                       *slowWriter
	updates                   int
	got                       []Stats
	sizeAtStats               []int // how much had been written when Stats was called
	statsAfterQuit, quitCalls atomic.Int32
	calls                     atomic.Int32 // Stats calls, for a test that waits for a frame while the run is going
}

func (g *statsGame) Update(float64, *Input) error {
	time.Sleep(g.updateWait)
	if g.updates++; g.updates > g.frames {
		g.quitCalls.Add(1)
		return ErrQuit
	}
	return nil
}

func (g *statsGame) Draw(fb *FrameBuffer) {
	time.Sleep(g.drawWait)
	fb.Set(1, 0, Cell{Character: 'X', Foreground: Palette(208)})
}

func (g *statsGame) Stats(s Stats) {
	if g.quitCalls.Load() > 0 {
		g.statsAfterQuit.Add(1)
	}
	g.got = append(g.got, s)
	g.calls.Add(1)
	g.sizeAtStats = append(g.sizeAtStats, g.out.n)
}

// plainGame is the same game without a Stats method.
type plainGame struct{ *statsGame }

func (g plainGame) Stats() {} // a method of that name that is not Stats(Stats) is not asked for

// slowWriter counts what is written to it, and takes its time about it if asked.
type slowWriter struct {
	n    int
	wait time.Duration
	buf  bytes.Buffer
}

func (w *slowWriter) Write(p []byte) (int, error) {
	time.Sleep(w.wait)
	w.n += len(p)
	return w.buf.Write(p)
}

func runStats(t *testing.T, g Game, out io.Writer, fps int) {
	t.Helper()
	if err := runHeadless(t, g, Config{Input: strings.NewReader(""), Output: out, FPS: fps}); err != nil {
		t.Fatalf("wanted nil, got %v", err)
	}
}

// Stats is called once for each frame, after the frame has been written, and Bytes is exactly what
// was written for it.
func TestStatsAreGivenAfterEveryFrameIsWritten(t *testing.T) {
	out := &slowWriter{}
	g := &statsGame{frames: 5, out: out}
	runStats(t, g, out, 0)

	if len(g.got) != 5 {
		t.Fatalf("wanted 5 calls, got %d", len(g.got))
	}
	frame := len(frameOf(t, 3, 2, func(fb *FrameBuffer) { fb.Set(1, 0, Cell{Character: 'X', Foreground: Palette(208)}) }))
	for i, s := range g.got {
		if s.Bytes != frame {
			t.Errorf("frame %d: wanted %d bytes, got %d", i, frame, s.Bytes)
		}
		if want := (i + 1) * frame; g.sizeAtStats[i] != want {
			t.Errorf("frame %d: wanted %d bytes written when Stats was called, got %d", i, want, g.sizeAtStats[i])
		}
		if s.FPS <= 0 || s.Update < 0 || s.Draw < 0 || s.Write < 0 {
			t.Errorf("frame %d: wanted numbers that are not negative and a rate, got %+v", i, s)
		}
		if s.Update > 2*time.Second || s.Draw > 2*time.Second || s.Write > 2*time.Second {
			t.Errorf("frame %d: wanted times of the frame and not of the run, got %+v", i, s)
		}
	}
	if out.n != 5*frame {
		t.Errorf("wanted %d bytes in all, got %d", 5*frame, out.n)
	}
}

// Each time is the time of its own part of the frame.
func TestStatsTimeEachPartOfTheFrame(t *testing.T) {
	const slow = 20 * time.Millisecond
	fastest := func(g *statsGame, pick func(Stats) time.Duration) time.Duration {
		best := time.Hour
		for _, s := range g.got {
			best = min(best, pick(s))
		}
		return best
	}
	for name, tt := range map[string]struct {
		update, draw, write time.Duration
		pick                func(Stats) time.Duration
		others              []func(Stats) time.Duration
	}{
		"Update": {update: slow, pick: func(s Stats) time.Duration { return s.Update }, others: []func(Stats) time.Duration{func(s Stats) time.Duration { return s.Draw }, func(s Stats) time.Duration { return s.Write }}},
		"Draw":   {draw: slow, pick: func(s Stats) time.Duration { return s.Draw }, others: []func(Stats) time.Duration{func(s Stats) time.Duration { return s.Update }, func(s Stats) time.Duration { return s.Write }}},
		"Write":  {write: slow, pick: func(s Stats) time.Duration { return s.Write }, others: []func(Stats) time.Duration{func(s Stats) time.Duration { return s.Update }, func(s Stats) time.Duration { return s.Draw }}},
	} {
		out := &slowWriter{wait: tt.write}
		g := &statsGame{frames: 6, out: out, updateWait: tt.update, drawWait: tt.draw}
		runStats(t, g, out, 0)
		if len(g.got) != 6 {
			t.Fatalf("%s: wanted 6 calls, got %d", name, len(g.got))
		}
		for i, s := range g.got {
			if tt.pick(s) < slow {
				t.Errorf("%s, frame %d: wanted at least %v, got %v", name, i, slow, tt.pick(s))
			}
		}
		// What is not slow is quick in at least one frame, so a time is not being put in the wrong place.
		for _, other := range tt.others {
			if best := fastest(g, other); best >= slow {
				t.Errorf("%s: wanted the other times to stay short, got %v at best", name, best)
			}
		}
	}
}

// The rate is the dt of the frames, averaged: the first frame sets it and each later one moves it a
// tenth of the way.
func TestMeterAveragesTheRate(t *testing.T) {
	var m meter
	for _, tt := range []struct{ dt, want float64 }{
		{0.01, 100},
		{0.02, 95},       // 0.9*100 + 0.1*50
		{0, 95},          // no time passed: no news
		{-1, 95},         // not a time
		{math.NaN(), 95}, // not a time
		{0.02, 90.5},     // 0.9*95 + 0.1*50
		{0.25, 81.85},    // 0.9*90.5 + 0.1*4
	} {
		if got := m.rate(tt.dt); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("after dt %v: wanted %v, got %v", tt.dt, tt.want, got)
		}
	}
	var stall meter
	if got := stall.rate(0.25); got != 4 {
		t.Errorf("wanted a first frame of a quarter of a second to read 4, got %v", got)
	}
}

// Run at a given rate reads about that rate.
func TestStatsReadTheRateOfTheRun(t *testing.T) {
	out := &slowWriter{}
	g := &statsGame{frames: 15, out: out}
	runStats(t, g, out, 50)
	last := g.got[len(g.got)-1].FPS
	if last < 30 || last > 55 {
		t.Errorf("wanted about 50 frames a second, got %v", last)
	}
}

// A meter that is off reads no clock and gives nothing, which is what a game with no Stats method has.
func TestAMeterThatIsOffReadsNoClock(t *testing.T) {
	var m meter
	m.start()
	if !m.mark.IsZero() || m.lap() != 0 || !m.mark.IsZero() {
		t.Errorf("wanted a meter that is off to read no clock, got %v", m.mark)
	}
	on := meter{on: true}
	on.start()
	if on.mark.IsZero() {
		t.Errorf("wanted a meter that is on to read the clock")
	}
	time.Sleep(2 * time.Millisecond)
	if d := on.lap(); d < 2*time.Millisecond {
		t.Errorf("wanted a lap of at least 2ms, got %v", d)
	}
}

// A game that does not ask is never timed: Run reads no clock for it, and four times a frame for a
// game that does (the start, and the end of Update, of Draw and of the write), and once more at
// the start of the frame whose Update quits.
func TestAGameThatDoesNotAskIsNeverTimed(t *testing.T) {
	defer func(f func() time.Time) { clock = f }(clock)
	reads := 0
	clock = func() time.Time { reads++; return time.Now() }

	g := &statsGame{frames: 5, out: &slowWriter{}}
	runStats(t, plainGame{g}, g.out, 0)
	if reads != 0 {
		t.Errorf("wanted no clock read for a game that does not ask, got %d", reads)
	}
	g = &statsGame{frames: 5, out: &slowWriter{}}
	runStats(t, g, g.out, 0)
	if reads != 5*4+1 {
		t.Errorf("wanted 4 clock reads for each of 5 frames and one for the frame that quits, got %d", reads)
	}
}

// A game with a method that is not Stats(Stats) is not given the numbers, and what Run writes does
// not depend on whether the game asks.
func TestAGameThatDoesNotAskIsNotTold(t *testing.T) {
	asked := &slowWriter{}
	ga := &statsGame{frames: 4, out: asked}
	runStats(t, ga, asked, 0)

	plain := &slowWriter{}
	gp := &statsGame{frames: 4, out: plain}
	runStats(t, plainGame{gp}, plain, 0)

	if len(gp.got) != 0 {
		t.Errorf("wanted no calls for a game with no Stats method, got %d", len(gp.got))
	}
	if asked.buf.String() != plain.buf.String() || asked.n == 0 {
		t.Errorf("wanted the same output with and without the method, got %q and %q", asked.buf.String(), plain.buf.String())
	}
}

// The frame whose Update ends the run is not a frame, and Stats is not called after Run returns.
func TestStatsAreNotGivenForTheFrameThatQuits(t *testing.T) {
	out := &slowWriter{}
	g := &statsGame{frames: 3, out: out}
	runStats(t, g, out, 0)
	if len(g.got) != 3 || g.statsAfterQuit.Load() != 0 {
		t.Errorf("wanted 3 calls and none after the quit, got %d and %d", len(g.got), g.statsAfterQuit.Load())
	}
	time.Sleep(20 * time.Millisecond)
	if len(g.got) != 3 {
		t.Errorf("wanted no call after Run returned, got %d", len(g.got))
	}

	// A run that is cancelled between frames gives no more either.
	ctx, cancel := context.WithCancel(context.Background())
	g = &statsGame{frames: 1 << 30, out: &slowWriter{}}
	done := startHeadless(ctx, g, Config{Input: strings.NewReader(""), Output: g.out})
	waitFor(t, "a frame", func() bool { return g.calls.Load() > 0 })
	cancel()
	finished(t, done)
	n := len(g.got)
	time.Sleep(20 * time.Millisecond)
	if len(g.got) != n {
		t.Errorf("wanted no call after Run returned, got %d more", len(g.got)-n)
	}
}

// Asking costs no allocation in the frame loop: a run of many frames allocates no more than a run of few.
func TestStatsAllocateNothingPerFrame(t *testing.T) {
	run := func(frames int, ask bool) float64 {
		return testing.AllocsPerRun(5, func() {
			out := &slowWriter{}
			g := &statsGame{frames: frames, out: out}
			g.got = make([]Stats, 0, frames)
			g.sizeAtStats = make([]int, 0, frames)
			var game Game = g
			if !ask {
				game = plainGame{g}
			}
			if err := Run(context.Background(), game, Config{Width: 3, Height: 2, FPS: maxFPS, Input: strings.NewReader(""), Output: io.Discard}); err != nil {
				t.Error(err)
			}
		})
	}
	for _, ask := range []bool{true, false} {
		few, many := run(10, ask), run(110, ask)
		if many-few > 2 { // a run allocates a few times at the start, and nothing for a frame
			t.Errorf("asking %v: wanted no allocation per frame, got %v for 10 frames and %v for 110", ask, few, many)
		}
	}
}

// The presenter counts what it sends: all of a full frame, 0 for a frame that changes nothing, and what
// a change takes.
func TestPresenterCountsTheBytesItSends(t *testing.T) {
	frame := func(fb *FrameBuffer, c rune) {
		fb.Clear()
		fb.DrawText(0, 0, "hello", Color{})
		fb.Set(2, 1, Cell{Character: c, Foreground: Palette(208)})
	}
	fb, err := NewFrameBuffer(8, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, changesOnly := range []bool{true, false} {
		p := newPresenter(changesOnly, false)
		var out bytes.Buffer
		frame(fb, 'a')
		if err := p.present(&out, fb, 8, 3); err != nil {
			t.Fatal(err)
		}
		first := out.Len()
		if first == 0 || p.bytes != first {
			t.Errorf("changesOnly %v: wanted the first frame to count %d bytes, got %d", changesOnly, first, p.bytes)
		}

		out.Reset()
		if err := p.present(&out, fb, 8, 3); err != nil {
			t.Fatal(err)
		}
		if changesOnly && (p.bytes != 0 || out.Len() != 0) {
			t.Errorf("wanted a frame that changes nothing to send 0 bytes, got %d counted and %d written", p.bytes, out.Len())
		}
		if !changesOnly && p.bytes != first {
			t.Errorf("wanted a whole frame again, got %d", p.bytes)
		}

		out.Reset()
		frame(fb, 'b')
		if err := p.present(&out, fb, 8, 3); err != nil {
			t.Fatal(err)
		}
		if p.bytes != out.Len() || p.bytes == 0 || (changesOnly && p.bytes >= first) {
			t.Errorf("changesOnly %v: wanted the one change to count %d bytes and be smaller than %d, got %d", changesOnly, out.Len(), first, p.bytes)
		}
	}
}

// BenchmarkMeterFrame is the cost of the numbers of one frame: four clock reads and the average
// for a game that asks, and nothing for one that does not.
func BenchmarkMeterFrame(b *testing.B) {
	for _, on := range []bool{true, false} {
		name := "asked"
		if !on {
			name = "not asked"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			m := meter{on: on}
			for b.Loop() {
				m.start()
				m.lap()
				m.lap()
				m.lap()
				m.rate(0.02)
			}
		})
	}
}

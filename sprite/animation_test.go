package sprite

import (
	"math"
	"math/rand/v2"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// letters is a frame for each of the given characters, one cell each.
func letters(chars string) []*Frame {
	var out []*Frame
	for _, c := range chars {
		out = append(out, FromLines(engine.Cell{}, string(c)))
	}
	return out
}

// shows is the characters of the frames an animation went through, one for each time it
// was advanced by dt, starting with the one it shows first.
func shows(a *Animation, dt float64, steps int) string {
	var out []rune
	char := func() rune {
		if f := a.Frame(); f != nil {
			return f.At(0, 0).Character
		}
		return '-'
	}
	out = append(out, char())
	for range steps {
		a.Advance(dt)
		out = append(out, char())
	}
	return string(out)
}

// Four frames at 10 a second, advanced by 0.05: each frame lasts two steps, and the
// animation then plays once and stops on the last, or starts again.
func TestAnimationPlaysAtItsSpeedOnceOrInALoop(t *testing.T) {
	// Started 0.01 s in, so that no step lands exactly on the change of a frame, where
	// the rounding of a sum decides which side it falls.
	once := NewAnimation(10, letters("abcd")...)
	once.Advance(0.01)
	if got := shows(once, 0.05, 12); got != "aabbccddddddd" {
		t.Errorf("playing once: wanted aabbccddddddd, got %s", got)
	}

	loop := NewAnimation(10, letters("abcd")...)
	loop.Loop = true
	loop.Advance(0.01)
	if got := shows(loop, 0.05, 12); got != "aabbccddaabbc" {
		t.Errorf("looping: wanted aabbccddaabbc, got %s", got)
	}
}

// Time is what counts: the same total time gives the same frame however it is cut up.
func TestAnimationDoesNotDependOnHowTimeIsCutUp(t *testing.T) {
	for _, dts := range [][]float64{{0.55}, {0.05, 0.5}, {0.25, 0.25, 0.05}, {0.01, 0.54}, {0.5, 0.05}} {
		a := NewAnimation(10, letters("abcdefgh")...)
		for _, dt := range dts {
			a.Advance(dt)
		}
		if got := a.Index(); got != 5 {
			t.Errorf("after %v: wanted frame 5 (0.55 s at 10 a second), got %d", dts, got)
		}
	}
}

func TestAnimationDoneTimingAndReset(t *testing.T) {
	a := NewAnimation(4, letters("xyz")...) // 3 frames at 4 a second: 0.75 s
	a.Advance(0.74)
	if a.Done() || a.Index() != 2 {
		t.Errorf("wanted the last frame shown but not done at 0.74, got index %d done %v", a.Index(), a.Done())
	}
	a.Advance(0.01)
	if !a.Done() || a.Index() != 2 {
		t.Errorf("wanted done at 0.75 on the last frame, got index %d done %v", a.Index(), a.Done())
	}
	a.Advance(100)
	if !a.Done() || a.Index() != 2 {
		t.Errorf("wanted it to stay done on the last frame, got index %d done %v", a.Index(), a.Done())
	}

	a.Reset()
	if a.Done() || a.Index() != 0 {
		t.Errorf("wanted Reset to start again, got index %d done %v", a.Index(), a.Done())
	}
	a.Advance(0.3)
	if a.Index() != 1 {
		t.Errorf("wanted the animation to play again after Reset, got index %d", a.Index())
	}

	loop := NewAnimation(4, letters("xyz")...)
	loop.Loop = true
	loop.Advance(1000)
	if loop.Done() {
		t.Errorf("wanted a looping animation never to be done")
	}
}

// Turning Loop on after the end wraps from there, and turning it off in the middle of a
// loop stops on the last frame when it gets there.
func TestAnimationLoopCanBeChangedWhilePlaying(t *testing.T) {
	a := NewAnimation(10, letters("abc")...)
	a.Advance(5) // done
	a.Loop = true
	if got := a.Index(); got != 0 {
		t.Errorf("wanted the finished animation to wrap to frame 0 once it loops, got %d", got)
	}
	a.Advance(0.15)
	if got := a.Index(); got != 1 {
		t.Errorf("wanted it to run on from the start, got %d", got)
	}

	a.Loop = false
	a.Advance(5)
	if got, done := a.Index(), a.Done(); got != 2 || !done {
		t.Errorf("wanted it to stop on the last frame without Loop, got index %d done %v", got, done)
	}
}

// A time that cannot be added is ignored: zero, negative, NaN and infinite.
func TestAnimationIgnoresTimeThatCannotBeAdded(t *testing.T) {
	for _, dt := range []float64{0, -0.1, -1e300, math.NaN(), math.Inf(1), math.Inf(-1)} {
		a := NewAnimation(10, letters("abc")...)
		a.Advance(0.15)
		a.Advance(dt)
		if got := a.Index(); got != 1 || a.Done() {
			t.Errorf("dt %v: wanted the animation left on frame 1, got %d", dt, got)
		}
	}
}

// With no usable speed the animation does not run: it shows its first frame and never ends.
func TestAnimationWithoutASpeedDoesNotRun(t *testing.T) {
	for _, fps := range []float64{0, -5, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, loop := range []bool{false, true} {
			a := NewAnimation(fps, letters("abc")...)
			a.Loop = loop
			a.Advance(10)
			if a.Index() != 0 || a.Done() {
				t.Errorf("fps %v loop %v: wanted frame 0 and not done, got %d and %v", fps, loop, a.Index(), a.Done())
			}
			if f := a.Frame(); f == nil || f.At(0, 0).Character != 'a' {
				t.Errorf("fps %v loop %v: wanted the first frame shown", fps, loop)
			}
		}
	}
}

func TestAnimationWithNoFramesShowsNothing(t *testing.T) {
	a := NewAnimation(10)
	a.Advance(1)
	if a.Frame() != nil || a.Index() != 0 || !a.Done() {
		t.Errorf("wanted nothing to show and done, got %v %d %v", a.Frame(), a.Index(), a.Done())
	}
	a.Loop = true
	if a.Done() {
		t.Errorf("wanted an empty looping animation not to be done")
	}

	var none *Animation
	none.Advance(1)
	none.Reset()
	if none.Frame() != nil || none.Index() != 0 || !none.Done() {
		t.Errorf("wanted a nil animation to have no frames")
	}
}

func TestAnimationOfOneFrameAndNilFrames(t *testing.T) {
	one := NewAnimation(2, letters("q")...)
	one.Advance(0.4)
	if one.Done() || one.Index() != 0 {
		t.Errorf("wanted a single frame to last its half second, got done %v", one.Done())
	}
	one.Advance(0.1)
	if !one.Done() || one.Index() != 0 {
		t.Errorf("wanted a single frame done after its half second")
	}

	a := NewAnimation(10, letters("a")[0], nil)
	a.Advance(0.15)
	if a.Frame() != nil || a.Index() != 1 {
		t.Errorf("wanted the nil second frame to be shown as nothing, got %v at %d", a.Frame(), a.Index())
	}
}

func TestNewAnimationKeepsItsOwnList(t *testing.T) {
	frames := letters("ab")
	a := NewAnimation(1, frames...)
	frames[0] = nil
	if a.Frames[0] == nil {
		t.Errorf("wanted the animation to keep the frames it was given")
	}
}

// Very long times: the index stays in range, the elapsed time does not grow without bound
// when looping, and the answer after a huge step is what the arithmetic says.
func TestAnimationSurvivesHugeAndTinyTimes(t *testing.T) {
	a := NewAnimation(8, letters("abcd")...) // half a second a play
	a.Loop = true
	a.Advance(1e9) // exactly 2e9 plays
	if got := a.Index(); got != 0 {
		t.Errorf("wanted frame 0 after a whole number of plays, got %d", got)
	}
	a.Advance(0.3) // 2.4 frames in
	if got := a.Index(); got != 2 {
		t.Errorf("wanted frame 2 after 0.3 s more, got %d", got)
	}
	a.Advance(math.MaxFloat64)
	a.Advance(math.MaxFloat64)
	if got := a.Index(); got < 0 || got > 3 {
		t.Errorf("wanted an index in range after the largest times, got %d", got)
	}
	if a.elapsed < 0 || a.elapsed >= 0.5 {
		t.Errorf("wanted the elapsed time kept inside one play, got %v", a.elapsed)
	}

	// A step of a billion seconds must not cost the time already played its precision: the
	// plays are taken off the step first, then the rest is added.
	precise := NewAnimation(8, letters("abcd")...)
	precise.Loop = true
	precise.Advance(0.2)
	precise.Advance(1e9)
	if math.Abs(precise.elapsed-0.2) > 1e-12 {
		t.Errorf("wanted the elapsed time to stay 0.2 after whole plays, got %v", precise.elapsed)
	}

	slow := NewAnimation(5e-324, letters("ab")...) // a play that is longer than a float holds
	slow.Loop = true
	slow.Advance(1e300)
	if got := slow.Index(); got != 0 {
		t.Errorf("wanted an animation that is too slow to move to stay on frame 0, got %d", got)
	}

	fast := NewAnimation(math.MaxFloat64, letters("abc")...)
	fast.Advance(1)
	if got, done := fast.Index(), fast.Done(); got != 2 || !done {
		t.Errorf("wanted an absurdly fast animation to be at its end, got %d %v", got, done)
	}
}

// A play too long for a float (a very low FPS) never ends and never turns the time into
// NaN, however much is added.
func TestAnimationOfAnEndlessPlayNeverOverflows(t *testing.T) {
	for _, loop := range []bool{false, true} {
		a := NewAnimation(5e-324, letters("abc")...)
		a.Loop = loop
		a.Advance(math.MaxFloat64)
		a.Advance(math.MaxFloat64)
		a.Advance(1)

		if math.IsNaN(a.elapsed) || math.IsInf(a.elapsed, 0) {
			t.Errorf("loop %v: wanted a finite elapsed time, got %v", loop, a.elapsed)
		}
		if a.Index() != 0 || a.Done() {
			t.Errorf("loop %v: wanted frame 0 and not done, got %d and %v", loop, a.Index(), a.Done())
		}
		a.FPS = 10 // and it is usable once it has a real speed
		a.Reset()
		a.Advance(0.15)
		if a.Index() != 1 {
			t.Errorf("loop %v: wanted the animation to run after Reset at a real speed, got %d", loop, a.Index())
		}
	}
}

// Time passes only while the animation can run: it is not saved up while FPS is zero.
func TestAnimationDoesNotSaveUpTimeItCouldNotRun(t *testing.T) {
	a := NewAnimation(0, letters("abcd")...)
	a.Advance(10)
	a.FPS = 10
	if a.Index() != 0 {
		t.Errorf("wanted the animation to start from the beginning once it has a speed, got %d", a.Index())
	}
	a.Advance(0.15)
	if a.Index() != 1 {
		t.Errorf("wanted frame 1 after 0.15 s at 10 a second, got %d", a.Index())
	}

	none := NewAnimation(10)
	none.Advance(5)
	none.Frames = letters("abcd")
	if none.Index() != 0 {
		t.Errorf("wanted time not to be saved up while there were no frames, got %d", none.Index())
	}
}

// Changing FPS or the list after some time has played is safe, whatever it does to the
// position: the index stays in range.
func TestAnimationIndexStaysInRangeWhenItsSettingsChange(t *testing.T) {
	a := NewAnimation(1, letters("abcdef")...)
	a.Advance(5.5)
	a.Frames = a.Frames[:2]
	for _, fps := range []float64{1e300, math.MaxFloat64} { // the second makes the position infinite
		a.FPS = fps
		for _, loop := range []bool{false, true} {
			a.Loop = loop
			if got := a.Index(); got < 0 || got > 1 {
				t.Errorf("fps %v, loop %v: wanted an index in range after the speed and list changed, got %d", fps, loop, got)
			}
		}
	}

	// 5.5 s in at 1 a second, then 2.4 a second on 4 frames: 13.2 frames in, which is
	// frame 1 of the 4 when looping and the last when not.
	b := NewAnimation(1, letters("abcdef")...)
	b.Advance(5.5)
	b.FPS, b.Frames = 2.4, b.Frames[:4]
	if got := b.Index(); got != 3 {
		t.Errorf("wanted the last frame when not looping, got %d", got)
	}
	b.Loop = true
	if got := b.Index(); got != 1 {
		t.Errorf("wanted the position to wrap to frame 1 when looping, got %d", got)
	}
	a.Frames = nil
	if a.Index() != 0 || a.Frame() != nil {
		t.Errorf("wanted no frame once the list is emptied")
	}
}

// Against a reference that adds the times up plainly: the same frame, looping or not, for
// random speeds and times, skipping the instant a frame changes where rounding may differ.
func TestAnimationMatchesAPlainReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(81, 82))
	checked := 0
	for range 2000 {
		n := 1 + rng.IntN(6)
		fps := []float64{1, 2, 4, 8, 10, 12.5, 24, 30}[rng.IntN(8)]
		a := NewAnimation(fps, letters("abcdefgh"[:n])...)
		a.Loop = rng.IntN(2) == 0

		total := 0.0
		for range 1 + rng.IntN(8) {
			dt := rng.Float64() * 0.4
			a.Advance(dt)
			total += dt

			pos := total * fps
			if math.Abs(pos-math.Round(pos)) < 1e-9 {
				continue // right on a change of frame
			}
			want := int(pos)
			wantDone := false
			if a.Loop {
				want %= n
			} else if want >= n {
				want, wantDone = n-1, true
			}
			if got := a.Index(); got != want || a.Done() != wantDone {
				t.Fatalf("%d frames at %v a second, %v s in, loop %v: wanted index %d done %v, got %d %v", n, fps, total, a.Loop, want, wantDone, a.Index(), a.Done())
			}
			checked++
		}
	}
	if checked < 5000 {
		t.Errorf("wanted at least 5000 comparisons, got %d", checked)
	}
}

// What a sprite draws: an Animation takes the place of a Frame, and a Frame of the Cell;
// an Animation with nothing to show draws nothing, and does not fall back.
func TestSpriteDrawsItsAnimationInPlaceOfItsFrameAndCell(t *testing.T) {
	anim := NewAnimation(10, letters("12")...)
	sp := &Sprite{Cell: engine.Cell{Character: 'C'}, Frame: FromLines(engine.Cell{}, "F"), Animation: anim}
	l := NewLayer()
	l.Add(sp)
	at := func() string {
		fb := mustBuffer(t, 3, 1)
		l.Draw(fb)
		return fb.String()
	}

	if got := at(); got != "1" {
		t.Errorf("wanted the first frame of the animation, got %q", got)
	}
	anim.Advance(0.1)
	if got := at(); got != "2" {
		t.Errorf("wanted the second frame after 0.1 s, got %q", got)
	}

	anim.Frames = nil
	if got := at(); got != "" {
		t.Errorf("wanted an empty animation to draw nothing, not the frame or the cell, got %q", got)
	}
	anim.Frames = []*Frame{nil}
	if got := at(); got != "" {
		t.Errorf("wanted a nil frame to draw nothing, got %q", got)
	}

	sp.Animation = nil
	if got := at(); got != "F" {
		t.Errorf("wanted the frame once there is no animation, got %q", got)
	}
	sp.Frame = nil
	if got := at(); got != "C" {
		t.Errorf("wanted the cell once there is no frame either, got %q", got)
	}
}

// An animation moves with its sprite, is clipped like a frame, and is see-through when the
// sprite is an Overlay.
func TestAnimatedSpriteMovesClipsAndOverlays(t *testing.T) {
	anim := NewAnimation(1, FromLines(engine.Cell{Foreground: red}, "a b"), FromLines(engine.Cell{Foreground: red}, "xyz"))
	sp := &Sprite{Position: geometry.Vector2{X: 1}, Animation: anim, Overlay: true}
	fb := floor(t, 4)
	l := NewLayer()
	l.Add(sp)

	l.Draw(fb)
	if got := fb.String(); got != ".a.b" {
		t.Errorf("wanted the first frame with a hole showing the floor, got %q", got)
	}

	anim.Advance(1)
	sp.Position.X = 2.5
	fb = floor(t, 4)
	l.Draw(fb) // "xyz" at column 2: the z is off the edge
	if got := fb.String(); got != "..xy" {
		t.Errorf("wanted the second frame moved and clipped, got %q", got)
	}
	if got := fb.At(2, 0); got.Background != green {
		t.Errorf("wanted the letters to keep the floor's background, got %+v", got)
	}
}

// Advancing and asking never panic or give an index out of range, whatever the numbers.
func FuzzAnimation(f *testing.F) {
	nan, inf := math.NaN(), math.Inf(1)
	f.Add(10.0, 0.05, 0.2, 7.0, uint8(4), true)
	f.Add(10.0, 0.05, 0.2, 7.0, uint8(4), false)
	f.Add(0.0, 1.0, 1.0, 1.0, uint8(3), true)
	f.Add(nan, 1.0, 1.0, 1.0, uint8(3), false)
	f.Add(inf, 1.0, 1.0, 1.0, uint8(3), true)
	f.Add(5e-324, 1e300, 1e300, 1e300, uint8(2), true)
	f.Add(math.MaxFloat64, math.MaxFloat64, math.MaxFloat64, 1.0, uint8(255), false)
	f.Add(10.0, -1.0, nan, inf, uint8(1), true)
	f.Add(10.0, 1.0, 1.0, 1.0, uint8(0), true)

	f.Fuzz(func(t *testing.T, fps, d1, d2, d3 float64, n uint8, loop bool) {
		frames := make([]*Frame, n)
		for i := range frames {
			frames[i] = NewFrame(1, 1)
		}
		a := NewAnimation(fps, frames...)
		a.Loop = loop

		last := 0
		for i, dt := range []float64{d1, d2, d3} {
			a.Advance(dt)

			got := a.Index()
			if got < 0 || (n > 0 && got >= int(n)) || (n == 0 && got != 0) {
				t.Fatalf("%d frames at %v, step %d (dt %v): index %d out of range", n, fps, i, dt, got)
			}
			if (a.Frame() == nil) != (n == 0) {
				t.Fatalf("%d frames: wanted a frame exactly when there are some", n)
			}
			if loop && a.Done() {
				t.Fatalf("wanted a looping animation never to be done")
			}
			if !loop && got < last {
				t.Fatalf("wanted an animation that plays once never to go back, from %d to %d", last, got)
			}
			if a.Done() && n > 0 && got != int(n)-1 {
				t.Fatalf("wanted a finished animation on its last frame, got %d of %d", got, n)
			}
			last = got
		}
	})
}

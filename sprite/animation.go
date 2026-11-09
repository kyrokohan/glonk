package sprite

import (
	"math"
	"slices"
)

// Animation is a list of frames shown one after another at a steady speed. Give it to a
// sprite through Sprite.Animation and call Advance from the game's Update with the time
// that passed; the sprite then shows the current frame. It runs in time, not in calls, so
// it plays at the same speed at any frame rate.
//
// Frames and FPS may be changed at any time; the time played so far is kept in seconds.
// An animation is not safe for use by several goroutines at once. One Animation can be
// shared by several sprites, which then all show the same frame, and it must be advanced
// once for each tick, not once for each sprite.
//
// Every method is safe on a nil *Animation, which has no frames.
type Animation struct {
	// Frames are the pictures, shown in order. A nil frame is a picture of nothing.
	Frames []*Frame

	// FPS is how many frames are shown in a second. An FPS that is not positive and
	// finite does not run: the animation stays on its first frame.
	FPS float64

	// Loop makes the animation start again after the last frame. If it is false the
	// animation stops on its last frame and Done reports true.
	Loop bool

	elapsed float64 // seconds played: wrapped to the length of one play when looping, and no more than that when not
}

// NewAnimation returns an animation of frames at fps frames a second. It plays once;
// set Loop to repeat it.
func NewAnimation(fps float64, frames ...*Frame) *Animation {
	return &Animation{Frames: slices.Clone(frames), FPS: fps}
}

// running reports whether the animation can move: it has frames and a speed.
func (a *Animation) running() bool {
	return len(a.Frames) > 0 && a.FPS > 0 && !math.IsInf(a.FPS, 1)
}

// length is the time one play takes, in seconds. It is only worked out for a running
// animation; a very low FPS can make it infinite, which is fine.
func (a *Animation) length() float64 {
	return float64(len(a.Frames)) / a.FPS
}

// Advance moves the animation on by dt seconds. A dt that is zero, negative, NaN or infinite
// is ignored, and so is any dt for an animation that has no frames or no speed. A
// looping animation starts again after its last frame; one that does not stops on it.
func (a *Animation) Advance(dt float64) {
	if a == nil || !(dt > 0) || math.IsInf(dt, 1) || !a.running() {
		return
	}

	total := a.length()
	if a.Loop && !math.IsInf(total, 1) {
		// Both parts are less than total, so their sum cannot overflow however large dt is.
		a.elapsed = math.Mod(a.elapsed+math.Mod(dt, total), total)
		return
	}
	// Playing once, or a play too long for a float to hold (a very low FPS): the time is
	// capped, so that adding to it can never overflow.
	a.elapsed = min(a.elapsed+dt, total, math.MaxFloat64)
}

// Index returns the number of the current frame, from 0. It is 0 for an animation that
// has no frames or does not run.
func (a *Animation) Index() int {
	if a == nil || !a.running() {
		return 0
	}

	n := float64(len(a.Frames))
	pos := a.elapsed * a.FPS // the frame, with a fraction, the animation is at
	switch {
	case pos < n: // the usual case, and false for NaN
	case a.Loop: // Loop was set after the last play ended, or FPS has changed since
		pos = math.Mod(pos, n)
	default:
		return len(a.Frames) - 1
	}
	if !(pos >= 0) { // NaN, from an infinite elapsed time and an FPS that changed
		return 0
	}
	return min(int(pos), len(a.Frames)-1)
}

// Frame returns the current frame, or nil if there are no frames, or if the current one is
// nil.
func (a *Animation) Frame() *Frame {
	if a == nil || len(a.Frames) == 0 {
		return nil
	}
	return a.Frames[a.Index()]
}

// Done reports whether an animation that does not loop has played to the end: its last
// frame has been shown for its whole time. It is false for a looping animation, and for one
// that has frames but does not run, which never gets there. An animation with no frames is
// done unless it loops, as it has nothing left to show.
func (a *Animation) Done() bool {
	switch {
	case a == nil:
		return true
	case a.Loop:
		return false
	case len(a.Frames) == 0:
		return true
	case !a.running():
		return false
	}
	return a.elapsed >= a.length()
}

// Reset starts the animation again from its first frame.
func (a *Animation) Reset() {
	if a != nil {
		a.elapsed = 0
	}
}

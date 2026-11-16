package engine

import (
	"errors"
	"time"
)

// ErrQuit, returned from Update, ends Run with a nil error, as the player quitting
// does.
var ErrQuit = errors.New("quit")

// Game is what Run drives: Update advances state, Draw renders it. Run calls them
// one at a time from its own goroutine, never after it has returned; see Run.
// An error returned from Update ends Run with that error, except ErrQuit, which
// ends it cleanly. A game that needs to know the size of the frame can also have a
// Resize method; see Resizer.
type Game interface {
	// Update advances the game by dt seconds, the time since the previous frame (at
	// most a quarter of a second, so a stall does not fling things across the world).
	// in is the keyboard state for this frame. Return ErrQuit to end Run cleanly.
	Update(dt float64, in *Input) error

	// Draw renders the current state into fb, which is blank at the start of every
	// call.
	Draw(fb *FrameBuffer)
}

// Resizer is an optional part of a Game. A game that has a Resize method is told
// the size of its frame, which Update and Draw need to know and Update is not
// otherwise given.
type Resizer interface {
	// Resize is called with the frame's size in cells before the first Update, and
	// again whenever that size changes, always before the Update and Draw of the
	// frame that has the new size. The size is that of the FrameBuffer Draw
	// receives: Config.Width and Config.Height where they are set, the terminal's
	// size on an axis left at 0.
	Resize(width, height int)
}

// Stats is how a frame went, as Run measured it. A Game that has a Stats method is given
// one after every frame; see StatsReceiver.
type Stats struct {
	// FPS is the frames a second Run is managing, averaged over about the last ten
	// frames. It follows the dt that Update is given, so a stall of more than a quarter of
	// a second counts as a quarter of a second.
	FPS float64

	// Update is how long this frame's Update took.
	Update time.Duration

	// Draw is how long this frame took to clear the FrameBuffer and run Draw.
	Draw time.Duration

	// Write is how long it took to get this frame to the terminal: working out which
	// cells changed, and writing them. A terminal that cannot keep up makes the write
	// block, so this is where a slow link shows.
	Write time.Duration

	// Bytes is how many bytes were sent for this frame: everything written, the
	// sequences that move and set colors included. It is 0 when nothing on the
	// screen changed. A headless Run sends every frame whole, so there it is the size
	// of the frame.
	Bytes int
}

// StatsReceiver is an optional part of a Game. A game that has a Stats method is told how
// each frame went, and a game that does not is not timed at all, so asking is the only way
// to pay for it.
type StatsReceiver interface {
	// Stats is called once after every frame has been written to the terminal, with that
	// frame's numbers, from the goroutine that calls Update and Draw. A game keeps them
	// and shows them in the next Draw. It is not called for a frame whose Update ends the
	// run, or after Run has returned.
	Stats(s Stats)
}

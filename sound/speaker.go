package sound

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ebitengine/oto/v3"
)

// buffered is how much of the mix the sound device holds: long enough not to run dry, short
// enough that a sound starts soon after Play.
const buffered = 40 * time.Millisecond

// opened is set by the first Open: a program has one sound device, and the library under it
// cannot open a second or try again.
var opened atomic.Bool

// kept holds the one speaker, so that it plays on in a program that dropped it: oto closes a
// player nothing refers to.
var kept *Speaker

// A Speaker plays a mixer on the machine's sound device.
type Speaker struct {
	ctx    *oto.Context
	player *oto.Player
	once   sync.Once
}

// Open plays m on the sound device, until Close. A program has one speaker: a second Open is an
// error, also after the first one failed.
//
// Where there is no sound device, or the system will not open it, Open returns an error that a
// program can show while it plays on in silence. On Windows a missing device is silence and no
// error. In a browser the sound starts only after the player presses a key or clicks.
func Open(m *Mixer) (*Speaker, error) {
	if !opened.CompareAndSwap(false, true) {
		return nil, errors.New("sound: Open was called before, and a program has one speaker")
	}
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   Rate,
		ChannelCount: 2,
		Format:       oto.FormatFloat32LE,
		BufferSize:   buffered,
	})
	if err != nil {
		return nil, fmt.Errorf("sound: %w", err)
	}
	// A browser starts the device only once the player has pressed a key or clicked, which
	// cannot happen while Open waits.
	if runtime.GOOS != "js" {
		select {
		case <-ready:
		case <-time.After(3 * time.Second):
			return nil, errors.New("sound: the sound device did not start within 3 seconds")
		}
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("sound: no sound device: %w", err)
		}
	}
	player := ctx.NewPlayer(&mixReader{m: m})
	player.SetBufferSize(frameBytes * int(buffered*Rate/time.Second))
	player.Play()
	kept = &Speaker{ctx: ctx, player: player}
	return kept, nil
}

// Close stops the speaker. Sound does not play again in this program.
func (s *Speaker) Close() error {
	var err error
	s.once.Do(func() {
		s.player.PauseAndStopReading()
		err = s.ctx.Suspend()
	})
	return err
}

// frameBytes is the size of one frame of the mix as the device takes it: two float32 samples.
const frameBytes = 8

// A mixReader is a mixer as the bytes a sound device reads: frames of two little-endian float32
// samples.
type mixReader struct {
	m      *Mixer
	frames []float32
}

// Read fills p with whole frames of the mix. It never ends.
func (r *mixReader) Read(p []byte) (int, error) {
	n := len(p) / frameBytes * 2
	if cap(r.frames) < n {
		r.frames = make([]float32, n)
	}
	r.frames = r.frames[:n]
	r.m.Read(r.frames)
	for i, s := range r.frames {
		binary.LittleEndian.PutUint32(p[4*i:], math.Float32bits(s))
	}
	return 4 * n, nil
}

package sound

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"glonk.dev/geometry"
)

// The bytes a sound device reads are whole frames of the mix, two little-endian float32 samples
// each, and the reader never ends. No test opens a device: a machine that runs tests may have none.
func TestMixReader(t *testing.T) {
	m := NewMixer(0)
	m.Play(steady(0.5, 100), geometry.Vector3{X: 3}, Sound{Near: 5, Range: 10}) // the right ear only
	r := &mixReader{m: m}
	p := make([]byte, 2*frameBytes+5) // two frames, and three bytes too few for a third
	n, err := r.Read(p)
	if n != 2*frameBytes || err != nil {
		t.Fatalf("wanted two whole frames and no error, got %d bytes and %v", n, err)
	}
	left := math.Float32frombits(binary.LittleEndian.Uint32(p[0:]))
	right := math.Float32frombits(binary.LittleEndian.Uint32(p[4:]))
	if !near(left, 0) || !near(right, 0.5) {
		t.Errorf("wanted the first frame silent on the left and 0.5 on the right, got %v and %v", left, right)
	}
	if n, err := r.Read(make([]byte, 1000*frameBytes)); n != 1000*frameBytes || err != nil {
		t.Errorf("wanted the mix to go on after the sound ended, got %d bytes and %v", n, err)
	}
}

// Reading the mix for the device allocates nothing once its buffer is the size the device asks for.
func TestMixReaderAllocatesNothing(t *testing.T) {
	r := &mixReader{m: NewMixer(0)}
	p := make([]byte, 40*frameBytes)
	r.Read(p)
	if a := testing.AllocsPerRun(100, func() { r.Read(p) }); a != 0 {
		t.Errorf("wanted no allocations, got %v", a)
	}
}

// A program has one speaker: Open after an earlier Open, which worked or not, is an error, and
// does not reach the device.
func TestOpenOnce(t *testing.T) {
	was := opened.Swap(true)
	defer opened.Store(was)
	// The reason is checked: on a machine with no sound device, Open fails anyway.
	if s, err := Open(NewMixer(0)); s != nil || err == nil || !strings.Contains(err.Error(), "called before") {
		t.Errorf("wanted a second Open refused, got %v and %v", s, err)
	}
}

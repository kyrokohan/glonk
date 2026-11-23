package sound

import (
	"math"
	"sync"
	"testing"
	"time"

	"glonk.dev/geometry"
)

// near reports whether two samples are within a millionth.
func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-6 }

func TestFalloff(t *testing.T) {
	for _, c := range []struct {
		d, near, rng, want float64
	}{
		{0, 1, 9, 1}, {1, 1, 9, 1}, {5, 1, 9, 0.25}, {7, 1, 9, 0.0625}, {9, 1, 9, 0}, {12, 1, 9, 0},
		{0.5, 0, 0, 1}, {32, 0, 0, 0}, {16.5, 0, 0, 0.25}, // the defaults: near 1, range 32
		{3, 4, 2, 1}, {4.5, 4, 2, 0}, // a range below near counts as near
	} {
		if got := falloff(c.d, c.near, c.rng); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("falloff(%v, near %v, range %v): wanted %v, got %v", c.d, c.near, c.rng, c.want, got)
		}
	}
}

// A sound on the listener's right is in the right ear only, one on the left in the left only, and
// one in front in both, each at -3 dB. A listener turned the other way hears them the other way.
func TestPan(t *testing.T) {
	half := float32(0.5 * math.Sqrt2 / 2)
	for _, c := range []struct {
		name           string
		right, at      geometry.Vector3
		left, rightEar float32
	}{
		{"on the right", geometry.Vector3{X: 1}, geometry.Vector3{X: 3}, 0, 0.5},
		{"on the left", geometry.Vector3{X: 1}, geometry.Vector3{X: -3}, 0.5, 0},
		{"in front", geometry.Vector3{X: 1}, geometry.Vector3{Z: 3}, half, half},
		{"behind", geometry.Vector3{X: 1}, geometry.Vector3{Z: -3}, half, half},
		{"on the right of one turned round", geometry.Vector3{X: -1}, geometry.Vector3{X: 3}, 0.5, 0},
		{"at the listener", geometry.Vector3{X: 1}, geometry.Vector3{}, half, half},
	} {
		m := NewMixer(0)
		m.Listen(geometry.Vector3{}, c.right)
		m.Play(steady(0.5, 100), c.at, Sound{Near: 5, Range: 10})
		l, r := frames(m, 1)
		if !near(l[0], c.left) || !near(r[0], c.rightEar) {
			t.Errorf("%s: wanted %v and %v, got %v and %v", c.name, c.left, c.rightEar, l[0], r[0])
		}
	}
}

// When the listener turns, a sound glides to its new ears over 20 ms, a little each frame, and
// never jumps.
func TestGlide(t *testing.T) {
	m := NewMixer(0)
	m.Play(steady(0.5, Rate), geometry.Vector3{X: 3}, Sound{Near: 5, Range: 10})
	frames(m, 1)
	m.Listen(geometry.Vector3{}, geometry.Vector3{X: -1}) // turned round: the sound is now on the left
	l, r := frames(m, glide+10)
	most := float32(0.5/glide) * 1.0001
	for i := 1; i < len(r); i++ {
		if math.Abs(float64(r[i]-r[i-1])) > float64(most) || math.Abs(float64(l[i]-l[i-1])) > float64(most) {
			t.Fatalf("frame %d: a step of %v and %v, more than the glide's %v", i, l[i]-l[i-1], r[i]-r[i-1], most)
		}
	}
	if !near(l[len(l)-1], 0.5) || !near(r[len(r)-1], 0) {
		t.Errorf("wanted the sound in the left ear after the glide, got %v and %v", l[len(l)-1], r[len(r)-1])
	}
	if near(l[glide/2], 0.5) {
		t.Errorf("wanted the sound half way at half the glide, got %v already", l[glide/2])
	}
}

// When every voice is in use, the sound nearest its end gives way to a new one, and a loop never
// does.
func TestVoiceLimit(t *testing.T) {
	m := NewMixer(2)
	long := m.Play(steady(0.1, 1000), geometry.Vector3{}, Sound{})
	short := m.Play(steady(0.1, 10), geometry.Vector3{}, Sound{})
	third := m.Play(steady(0.1, 500), geometry.Vector3{}, Sound{})
	if long.done || !short.done || third == nil || third.done {
		t.Errorf("wanted the short sound to give way, got long %v, short %v, third %v", long.done, short.done, third)
	}

	m = NewMixer(2)
	m.Loop(steady(0.1, 10), geometry.Vector3{}, Sound{})
	m.Loop(steady(0.1, 10), geometry.Vector3{}, Sound{})
	if v := m.Play(steady(0.1, 10), geometry.Vector3{}, Sound{}); v != nil {
		t.Errorf("wanted no voice when every one is a loop, got one")
	}
	if v := m.Play(&Clip{}, geometry.Vector3{}, Sound{}); v != nil {
		t.Errorf("wanted no voice for an empty clip")
	}
}

// A loop meets its end to its start with no gap, and a sound played once ends and leaves the mix.
func TestLoopsAndEnds(t *testing.T) {
	m := NewMixer(0)
	m.Loop(NewClip([]float32{0.1, 0.2, 0.3}), geometry.Vector3{}, Sound{})
	l, _ := frames(m, 7)
	g := float32(math.Sqrt2 / 2)
	for i, want := range []float32{0.1, 0.2, 0.3, 0.1, 0.2, 0.3, 0.1} {
		if !near(l[i], want*g) {
			t.Fatalf("wanted the loop end to start, got %v", l)
		}
	}

	m = NewMixer(0)
	m.PlayHere(NewClip([]float32{0.2, 0.2}), 1)
	l, _ = frames(m, 4)
	if l[1] == 0 || l[2] != 0 || len(m.voices) != 0 {
		t.Errorf("wanted the sound to end after its two samples and leave the mix, got %v and %d voices", l, len(m.voices))
	}
}

// Many loud sounds at once are bent below 1, not cut off, and a quiet mix is left as it is.
func TestLimiter(t *testing.T) {
	m := NewMixer(0)
	for range 8 {
		m.PlayHere(steady(0.5, 100), 1)
	}
	l, _ := frames(m, 1)
	if !(l[0] > knee && l[0] <= 1) {
		t.Errorf("wanted eight loud sounds bent between the knee and 1, got %v", l[0])
	}
	for _, s := range []float32{0, 0.5, knee, -knee} {
		if limit(s) != s {
			t.Errorf("wanted %v left as it is, got %v", s, limit(s))
		}
	}
	for a, b := float32(0.81), float32(0.9); b < 50; a, b = b, b*2 {
		if limit(a) > limit(b) || limit(b) > 1 || limit(-b) != -limit(b) {
			t.Fatalf("wanted the limiter to rise toward 1 and never past it, alike both ways: %v at %v, %v at %v", limit(a), a, limit(b), b)
		}
	}
	if !(limit(0.9) < limit(1.2)) {
		t.Errorf("wanted louder to stay louder just above the knee, got %v and %v", limit(0.9), limit(1.2))
	}
}

// Stop fades a sound out over 10 ms with no jump, then ends it; Fade takes a loop to a level.
func TestStopAndFade(t *testing.T) {
	m := NewMixer(0)
	v := m.PlayHere(steady(0.5, Rate), 1)
	frames(m, 1)
	v.Stop()
	l, _ := frames(m, Rate/100+5)
	for i := 1; i < len(l); i++ {
		if l[i] > l[i-1] {
			t.Fatalf("wanted the stop to fade down, got a rise at frame %d", i)
		}
	}
	if l[len(l)-1] != 0 || !v.done {
		t.Errorf("wanted the sound gone after 10 ms, got %v", l[len(l)-1])
	}
	v.Move(geometry.Vector3{X: 1}) // does nothing once a voice has ended
	v.Fade(1, 0)

	m = NewMixer(0)
	loop := m.Loop(steady(0.5, 100), geometry.Vector3{}, Sound{})
	loop.Fade(0.5, 100*time.Millisecond)
	l, _ = frames(m, Rate/10)
	if !near(l[len(l)-1], 0.5*0.5*math.Sqrt2/2) {
		t.Errorf("wanted the loop at half its level after the fade, got %v", l[len(l)-1])
	}
	var none *Voice
	none.Stop() // a nil voice, from a Play that found no room, does nothing
	none.Move(geometry.Vector3{})
	none.Fade(0, 0)
}

func TestVolume(t *testing.T) {
	m := NewMixer(0)
	m.PlayHere(steady(0.5, Rate), 1)
	m.SetVolume(0.5)
	l, _ := frames(m, glide+1)
	if !near(l[len(l)-1], float32(0.5*math.Sqrt2/2*0.5)) {
		t.Errorf("wanted half the volume after the glide, got %v", l[len(l)-1])
	}
	if l[0] <= l[len(l)-1] {
		t.Errorf("wanted the volume to glide down, got %v then %v", l[0], l[len(l)-1])
	}
}

// Play, Move, Listen and Read can run at once, from different goroutines.
func TestConcurrentUse(t *testing.T) {
	m := NewMixer(8)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		buf := make([]float32, 512)
		for range 200 {
			m.Read(buf)
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 200 {
			v := m.Play(steady(0.1, 300), geometry.Vector3{X: float64(i % 5)}, Sound{})
			v.Move(geometry.Vector3{Z: 1})
			m.Listen(geometry.Vector3{X: float64(i)}, geometry.Vector3{X: 1})
			if i%7 == 0 {
				v.Stop()
			}
		}
	}()
	wg.Wait()
}

func TestNewClipAndDuration(t *testing.T) {
	c := NewClip([]float32{2, -2, float32(math.NaN()), 0.5})
	if c.samples[0] != 1 || c.samples[1] != -1 || c.samples[2] != 0 || c.samples[3] != 0.5 {
		t.Errorf("wanted the samples cut to -1 to 1 and NaN as 0, got %v", c.samples)
	}
	if d := steady(0, Rate/2).Duration(); d != 500*time.Millisecond {
		t.Errorf("wanted half a second, got %v", d)
	}
}

func TestResample(t *testing.T) {
	got := resample([]float32{0, 1, 0}, Rate/2)
	want := []float32{0, 0.5, 1, 0.5, 0, 0}
	if len(got) != len(want) {
		t.Fatalf("wanted %d samples at twice the rate, got %v", len(want), got)
	}
	for i := range want {
		if !near(got[i], want[i]) {
			t.Fatalf("wanted %v, got %v", want, got)
		}
	}
}

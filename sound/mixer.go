package sound

import (
	"math"
	"sync"
	"time"

	"glonk.dev/geometry"
)

const (
	// defaultVoices is how many sounds a mixer plays at once when NewMixer is given 0.
	defaultVoices = 32
	// glide is how many samples a gain takes to reach a new value when a sound or the listener
	// moves: 20 ms, long enough not to click and short enough not to be heard as a slide.
	glide = Rate / 50
	// stopTime is how long Stop takes to fade a voice out, so that it does not click.
	stopTime = 10 * time.Millisecond
	// knee is the level above which the limiter bends the mix, so that a crowd of sounds does
	// not crack.
	knee = 0.8
)

// A Sound says how a clip played at a place is heard: how loud it is, and over what distance.
type Sound struct {
	// Gain is how loud the sound is at its loudest, from 0 up; 0 means 1. A gain that is not a
	// number, or below 0, is silent, and one above 16 is 16.
	Gain float64
	// Near is the distance within which the sound is heard at its full gain; 0 means 1.
	Near float64
	// Range is the distance from which it is no longer heard; 0 means 32. Between Near and Range
	// its gain falls as (1 - (d-Near)/(Range-Near))², so it fades out and reaches silence at
	// Range. A Range below Near counts as Near.
	Range float64
}

// A Mixer plays clips and mixes them into stereo at Rate, each heard from where it is. Make one
// with NewMixer. Its methods can be called from any goroutine.
type Mixer struct {
	mu        sync.Mutex
	voices    []*Voice
	max       int
	listener  geometry.Vector3
	right     geometry.Vector3
	volume    float64
	volTarget float64
	volStep   float64
	volLeft   int
}

// NewMixer makes a mixer that plays up to voices sounds at once; 0 means 32. The listener is at
// the origin with +x to the right, and the volume is 1.
func NewMixer(voices int) *Mixer {
	if voices <= 0 {
		voices = defaultVoices
	}
	return &Mixer{max: voices, right: geometry.Vector3{X: 1}, volume: 1, volTarget: 1}
}

// Play plays c once at a place, heard as s says. It returns the Voice it plays in, to move, fade
// or stop, or nil if every voice is a loop and none can give way.
//
// When every voice is in use, the one nearest its end gives way. A loop never does.
func (m *Mixer) Play(c *Clip, at geometry.Vector3, s Sound) *Voice {
	return m.start(&Voice{clip: c.samples, pos: at, sound: s})
}

// PlayHere plays c once at the listener, in both ears alike, at gain, however the listener
// moves: a sound of the player's own, such as a step or a menu's click. A gain of 0 means 1.
func (m *Mixer) PlayHere(c *Clip, gain float64) *Voice {
	return m.start(&Voice{clip: c.samples, here: true, sound: Sound{Gain: gain}})
}

// Loop plays c over and over at a place, end to start with no gap, until it is stopped. Fade
// changes its level, as for wind that rises and falls.
func (m *Mixer) Loop(c *Clip, at geometry.Vector3, s Sound) *Voice {
	return m.start(&Voice{clip: c.samples, pos: at, sound: s, loop: true})
}

// start adds a voice to the mix, at its gains with no glide, and makes room for it if it must.
func (m *Mixer) start(v *Voice) *Voice {
	if len(v.clip) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.voices) == m.max {
		least := -1
		for i, o := range m.voices {
			if !o.loop && (least < 0 || len(o.clip)-o.at < len(m.voices[least].clip)-m.voices[least].at) {
				least = i
			}
		}
		if least < 0 {
			return nil
		}
		m.voices[least].done = true
		m.voices = append(m.voices[:least], m.voices[least+1:]...)
	}
	v.m, v.level, v.levelTarget = m, 1, 1
	v.left, v.right = m.ears(v)
	v.leftTarget, v.rightTarget = v.left, v.right
	m.voices = append(m.voices, v)
	return v
}

// Listen says where the listener is and which way is their right, which with a camera of
// glonk.dev/mesh is cam.Position and cam.Right(). Every sound glides to where it is now heard. A
// right of zero length is +x.
func (m *Mixer) Listen(position, right geometry.Vector3) {
	if right.LengthSq() == 0 {
		right = geometry.Vector3{X: 1}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listener, m.right = position, right.Normalize()
	for _, v := range m.voices {
		m.aim(v)
	}
}

// SetVolume sets the loudness of the whole mix, from 0, silent, to 1, as it is. It glides there
// over 20 ms.
func (m *Mixer) SetVolume(v float64) {
	if !(v > 0) {
		v = 0
	}
	v = min(v, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.volTarget, m.volStep, m.volLeft = v, (v-m.volume)/glide, glide
}

// Read mixes the next len(stereo)/2 frames into stereo, left and right in turn, from -1 to 1, and
// returns how many samples it wrote: len(stereo), less one if that is odd. It never runs out: with
// nothing playing, it writes silence. A test reads it with no sound device.
func (m *Mixer) Read(stereo []float32) int {
	n := len(stereo) / 2 * 2
	clear(stereo[:n])
	m.mu.Lock()
	defer m.mu.Unlock()
	live := m.voices[:0]
	for _, v := range m.voices {
		v.mix(stereo[:n])
		if !v.done {
			live = append(live, v)
		}
	}
	clear(m.voices[len(live):])
	m.voices = live
	for i := 0; i < n; i += 2 {
		if m.volLeft > 0 {
			m.volume += m.volStep
			if m.volLeft--; m.volLeft == 0 {
				m.volume = m.volTarget
			}
		}
		stereo[i] = limit(stereo[i] * float32(m.volume))
		stereo[i+1] = limit(stereo[i+1] * float32(m.volume))
	}
	return n
}

// ears is how loud v is in the left ear and in the right, from where it is.
func (m *Mixer) ears(v *Voice) (left, right float64) {
	gain := v.gain()
	if v.here {
		return gain * math.Sqrt2 / 2, gain * math.Sqrt2 / 2
	}
	to := v.pos.Sub(m.listener)
	d := to.Length()
	gain *= falloff(d, v.sound.Near, v.sound.Range)
	side := 0.0
	if d > 0 {
		side = max(-1, min(1, to.Dot(m.right)/d))
	}
	angle := (side + 1) * math.Pi / 4 // equal power: -3 dB in each ear in the middle
	return gain * math.Cos(angle), gain * math.Sin(angle)
}

// aim sets v gliding to how loud it is now heard in each ear.
func (m *Mixer) aim(v *Voice) {
	v.leftTarget, v.rightTarget = m.ears(v)
	v.leftStep = (v.leftTarget - v.left) / glide
	v.rightStep = (v.rightTarget - v.right) / glide
	v.earsLeft = glide
}

// falloff is the part of its gain a sound keeps at distance d: all of it within near, none from
// rng on, and (1 - (d-near)/(rng-near))² between.
func falloff(d, near, rng float64) float64 {
	if near <= 0 {
		near = 1
	}
	if rng <= 0 {
		rng = 32
	}
	switch {
	case d <= near:
		return 1
	case d >= rng:
		return 0
	}
	x := 1 - (d-near)/(rng-near)
	return x * x
}

// limit bends a sample above the knee toward 1, never past it, so that sounds that add up past 1
// are made quieter and not cut off.
func limit(s float32) float32 {
	a := math.Abs(float64(s))
	if a <= knee {
		return s
	}
	return float32(math.Copysign(knee+(1-knee)*math.Tanh((a-knee)/(1-knee)), float64(s)))
}

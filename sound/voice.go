package sound

import (
	"time"

	"glonk.dev/geometry"
)

// A Voice is one clip playing in a mixer, from Play, PlayHere or Loop. Its methods can be called
// from any goroutine, also after it has ended, when they do nothing.
type Voice struct {
	m     *Mixer
	clip  []float32
	at    int
	loop  bool
	here  bool
	done  bool
	pos   geometry.Vector3
	sound Sound

	level, levelTarget, levelStep float64
	levelLeft                     int
	stopping                      bool

	left, right, leftTarget, rightTarget, leftStep, rightStep float64
	earsLeft                                                  int
}

// Move puts the voice at another place, and it glides to how it is heard from there over 20 ms.
// A voice played with PlayHere stays at the listener.
func (v *Voice) Move(to geometry.Vector3) {
	if v == nil {
		return
	}
	v.m.mu.Lock()
	defer v.m.mu.Unlock()
	if v.done || v.here {
		return
	}
	v.pos = to
	v.m.aim(v)
}

// Fade takes the voice's level to a part of its gain, from 0 to 1, over a time: a loop that rises
// and falls, or a sound that dies away. A time of 0 or less is at once. A voice faded to 0 keeps
// playing, silent; Stop ends it.
func (v *Voice) Fade(to float64, over time.Duration) {
	if v == nil {
		return
	}
	if !(to > 0) {
		to = 0
	}
	to = min(to, 1)
	v.m.mu.Lock()
	defer v.m.mu.Unlock()
	if v.done || v.stopping {
		return
	}
	v.fade(to, over)
}

// Stop fades the voice out over 10 ms, so that it does not click, and ends it.
func (v *Voice) Stop() {
	if v == nil {
		return
	}
	v.m.mu.Lock()
	defer v.m.mu.Unlock()
	if v.done {
		return
	}
	v.stopping = true
	v.fade(0, stopTime)
}

// fade sets the level gliding to to over a time.
func (v *Voice) fade(to float64, over time.Duration) {
	steps := int(over * Rate / time.Second)
	if steps <= 0 {
		v.level, v.levelTarget, v.levelLeft = to, to, 0
		if to == 0 && v.stopping {
			v.done = true
		}
		return
	}
	v.levelTarget, v.levelStep, v.levelLeft = to, (to-v.level)/float64(steps), steps
}

// gain is the voice's own gain, as its Sound says.
func (v *Voice) gain() float64 {
	g := v.sound.Gain
	switch {
	case g == 0:
		return 1
	case !(g > 0):
		return 0
	}
	return min(g, 16)
}

// mix adds the voice to the frames of stereo, and marks it done at the end of its clip or of a
// stop.
func (v *Voice) mix(stereo []float32) {
	for i := 0; i < len(stereo) && !v.done; i += 2 {
		if v.earsLeft > 0 {
			v.left += v.leftStep
			v.right += v.rightStep
			if v.earsLeft--; v.earsLeft == 0 {
				v.left, v.right = v.leftTarget, v.rightTarget
			}
		}
		if v.levelLeft > 0 {
			v.level += v.levelStep
			if v.levelLeft--; v.levelLeft == 0 {
				v.level = v.levelTarget
				if v.stopping {
					v.done = true
				}
			}
		}
		s := float64(v.clip[v.at]) * v.level
		stereo[i] += float32(s * v.left)
		stereo[i+1] += float32(s * v.right)
		if v.at++; v.at == len(v.clip) {
			if !v.loop {
				v.done = true
			}
			v.at = 0
		}
	}
}

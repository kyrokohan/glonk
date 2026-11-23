package main

import (
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/sound"
)

// No beep clicks, as one that stopped mid-wave would by jumping to silence: heard at full
// gain, no sample is further from the one before than the steepest of the tones goes.
func TestNoBeepClicks(t *testing.T) {
	for _, s := range sources {
		mixer := sound.NewMixer(0)
		mixer.Loop(s.clip, geometry.Vector3{}, sound.Sound{}) // at the listener: full gain
		stereo := make([]float32, 2*(sound.Rate+1))           // a second and a frame, across the seam
		mixer.Read(stereo)
		for i := 2; i < len(stereo); i += 2 {
			if d := stereo[i] - stereo[i-2]; d > 0.02 || d < -0.02 {
				t.Fatalf("%c: a jump of %v at sample %d, which would click", s.letter, d, i/2)
			}
		}
	}
}

// The arrows walk the listener, and it stays on the floor.
func TestTheArrowsWalk(t *testing.T) {
	g := newGame(sound.NewMixer(0))
	kb := engine.NewKeyboard()
	kb.Press(engine.KeyRight)
	start := g.at.X
	g.Update(0.5, kb.Input())
	if g.at.X != start+speed*0.5 {
		t.Errorf("wanted the listener %v to the right, got %v", speed*0.5, g.at.X-start)
	}
	for range 100 {
		g.Update(0.5, kb.Input())
	}
	if g.at.X != width-2 {
		t.Errorf("wanted the listener stopped at the wall, got %v", g.at.X)
	}
}

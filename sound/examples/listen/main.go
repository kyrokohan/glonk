// Command listen plays three sounds at places on a floor, and lets you walk among them: each is
// louder as you near it, and in the ear nearer to it.
//
//	go run ./examples/listen
//
// The arrow keys walk, and Esc quits. Where there is no sound device it says so, and you can
// still walk.
package main

import (
	"context"
	"fmt"
	"math"
	"os"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/sound"
)

// A source is a sound at a place on the floor, drawn as its letter.
type source struct {
	letter rune
	at     geometry.Vector3
	color  engine.Color
	clip   *sound.Clip
}

// Three beeps, each its own pitch and rhythm, so that the ear can tell them apart.
var sources = []source{
	{'A', geometry.Vector3{X: 8, Z: 4}, engine.Palette(208), tone(400, 0.5, 0.3)},
	{'B', geometry.Vector3{X: 52, Z: 4}, engine.Palette(45), tone(300, 0.25, 0.4)},
	{'C', geometry.Vector3{X: 30, Z: 17}, engine.Palette(171), tone(200, 1, 0.6)},
}

// The floor is 60 by 20 cells, one unit of the world to a cell: x to the right and z down.
const width, height, speed = 60, 20, 12.0

// tone is a second of beeps at hz, on for the first part of every period. Each beep holds a whole
// number of waves, so it starts and ends at rest and does not click, nor does the loop.
func tone(hz, period, part float64) *sound.Clip {
	samples := make([]float32, sound.Rate)
	for i := range samples {
		t := float64(i) / sound.Rate
		if math.Mod(t, period) < part*period {
			samples[i] = float32(0.3 * math.Sin(2*math.Pi*hz*t))
		}
	}
	return sound.NewClip(samples)
}

type game struct {
	mixer   *sound.Mixer
	at      geometry.Vector3
	trouble string
}

// newGame loops the sources in mixer, with the listener in the middle of the floor.
func newGame(mixer *sound.Mixer) *game {
	g := &game{mixer: mixer, at: geometry.Vector3{X: width / 2, Z: height / 2}}
	for _, s := range sources {
		mixer.Loop(s.clip, s.at, sound.Sound{Near: 2, Range: 30})
	}
	mixer.Listen(g.at, geometry.Vector3{X: 1})
	return g
}

func (g *game) Update(dt float64, in *engine.Input) error {
	if in.Pressed(engine.KeyEscape) {
		return engine.ErrQuit
	}
	var step geometry.Vector3
	if in.IsDown(engine.KeyLeft) {
		step.X--
	}
	if in.IsDown(engine.KeyRight) {
		step.X++
	}
	if in.IsDown(engine.KeyUp) {
		step.Z--
	}
	if in.IsDown(engine.KeyDown) {
		step.Z++
	}
	g.at = g.at.Add(step.Scale(speed * dt))
	g.at.X = max(1, min(width-2, g.at.X))
	g.at.Z = max(1, min(height-2, g.at.Z))
	g.mixer.Listen(g.at, geometry.Vector3{X: 1}) // facing up the screen: +x is to the right
	return nil
}

func (g *game) Draw(fb *engine.FrameBuffer) {
	fb.DrawBox(0, 0, width, height, engine.Color{})
	fb.DrawText(2, 0, " arrows walk, Esc quits ", engine.Color{})
	for _, s := range sources {
		fb.Set(int(s.at.X), int(s.at.Z), engine.Cell{Character: s.letter, Foreground: s.color, Attrs: engine.Bold})
	}
	fb.Set(int(g.at.X), int(g.at.Z), engine.Cell{Character: '@'})
	line := ""
	for _, s := range sources {
		line += fmt.Sprintf("%c %2.0f away   ", s.letter, s.at.Distance(g.at))
	}
	fb.DrawText(1, height, line, engine.Color{})
	if g.trouble != "" {
		fb.DrawText(1, height+1, g.trouble, engine.Palette(203))
	}
}

func main() {
	mixer := sound.NewMixer(0)
	g := newGame(mixer)
	speaker, err := sound.Open(mixer)
	if err != nil {
		g.trouble = "no sound: " + err.Error()
	} else {
		defer speaker.Close()
	}

	err = engine.Run(context.Background(), g, engine.Config{FPS: 30, Width: width, Height: height + 2})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

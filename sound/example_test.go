package sound_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"glonk.dev/geometry"
	"glonk.dev/sound"
)

// tone is a clip of a steady level, a stand-in for a sound read with LoadWAV.
func tone(level float32, d time.Duration) *sound.Clip {
	samples := make([]float32, int(d*sound.Rate/time.Second))
	for i := range samples {
		samples[i] = level
	}
	return sound.NewClip(samples)
}

// A sound is heard from where it is: louder in the nearer ear.
func ExampleMixer() {
	mixer := sound.NewMixer(0)
	mixer.Listen(geometry.Vector3{}, geometry.Vector3{X: 1}) // at the origin, +x to the right

	bell := tone(0.5, time.Second)
	mixer.Play(bell, geometry.Vector3{X: 2, Z: 2}, sound.Sound{Near: 4}) // ahead, to the right

	stereo := make([]float32, 2) // one frame: left, right
	mixer.Read(stereo)
	fmt.Printf("left %.2f, right %.2f\n", stereo[0], stereo[1])
	// Output:
	// left 0.11, right 0.49
}

// A sound fades with distance from its Near, and is silent from its Range.
func ExampleSound() {
	for _, d := range []float64{1, 4, 7, 10} {
		mixer := sound.NewMixer(0)
		mixer.Play(tone(0.5, time.Second), geometry.Vector3{Z: d}, sound.Sound{Near: 1, Range: 7})
		stereo := make([]float32, 2)
		mixer.Read(stereo)
		fmt.Printf("%2.0f away: %.3f\n", d, stereo[0]+stereo[1])
	}
	// Output:
	//  1 away: 0.707
	//  4 away: 0.177
	//  7 away: 0.000
	// 10 away: 0.000
}

// A loop plays until it is stopped, and Fade takes it to a level over a time.
func ExampleVoice_Fade() {
	mixer := sound.NewMixer(0)
	wind := mixer.Loop(tone(0.5, time.Second), geometry.Vector3{}, sound.Sound{})
	wind.Fade(0.2, 50*time.Millisecond)

	stereo := make([]float32, 2*sound.Rate/10) // a tenth of a second
	mixer.Read(stereo)
	fmt.Printf("after the fade: %.3f\n", stereo[len(stereo)-1])
	// Output:
	// after the fade: 0.071
}

// A WAV file of 16-bit PCM becomes a clip at the mixer's rate, here from 24 kHz.
func ExampleLoadWAV() {
	format := struct {
		Size                    uint32
		Tag, Channels           uint16
		Rate, BytesPerSecond    uint32
		BytesPerFrame, BitsEach uint16
	}{16, 1, 1, 24000, 48000, 2, 16}
	var file bytes.Buffer
	file.WriteString("RIFF\x00\x00\x00\x00WAVEfmt ")
	binary.Write(&file, binary.LittleEndian, format)
	file.WriteString("data")
	binary.Write(&file, binary.LittleEndian, uint32(2*2400)) // a tenth of a second, in 16-bit samples
	file.Write(make([]byte, 2*2400))

	clip, err := sound.LoadWAV(&file)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(clip.Duration().Round(time.Millisecond))
	// Output:
	// 100ms
}

// The listener follows a camera: its position, and the way to its right.
func ExampleMixer_Listen() {
	mixer := sound.NewMixer(0)
	position, right := geometry.Vector3{X: 10}, geometry.Vector3{Z: -1} // a camera turned left
	mixer.Listen(position, right)

	mixer.Play(tone(0.5, time.Second), geometry.Vector3{X: 10, Z: -2}, sound.Sound{Near: 4})
	stereo := make([]float32, 2)
	mixer.Read(stereo)
	fmt.Printf("left %.2f, right %.2f\n", stereo[0], math.Round(float64(stereo[1])*100)/100)
	// Output:
	// left 0.00, right 0.50
}

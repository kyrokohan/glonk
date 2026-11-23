package sound

import (
	"bytes"
	"fmt"
	"testing"

	"glonk.dev/geometry"
)

// Mixing a buffer of 40 ms, as a sound device asks for, with voices playing at places round the
// listener.
func BenchmarkRead(b *testing.B) {
	for _, n := range []int{1, 8, 32} {
		b.Run(fmt.Sprintf("%d voices", n), func(b *testing.B) {
			m := NewMixer(n)
			clip := steady(0.1, Rate)
			for i := range n {
				m.Loop(clip, geometry.Vector3{X: float64(i%7 - 3), Z: float64(i % 5)}, Sound{Range: 10})
			}
			buf := make([]float32, 2*Rate/25)
			b.ReportAllocs()
			for b.Loop() {
				m.Read(buf)
			}
		})
	}
}

// Reading a second of stereo sound at 44.1 kHz, which is mixed to mono and resampled.
func BenchmarkLoadWAV(b *testing.B) {
	file := wavFile(pcmFormat(2, 44100), pcmData(make([]int16, 2*44100)...))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := LoadWAV(bytes.NewReader(file)); err != nil {
			b.Fatal(err)
		}
	}
}

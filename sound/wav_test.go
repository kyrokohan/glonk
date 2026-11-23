package sound

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestLoadWAV(t *testing.T) {
	odd := chunk{id: "junk", body: []byte{1, 2, 3}} // an odd size, padded to four
	extensible := func(guidByte byte) chunk {
		b := make([]byte, 40)
		copy(b, pcmFormat(1, Rate).body)
		binary.LittleEndian.PutUint16(b[0:], 0xFFFE)
		binary.LittleEndian.PutUint16(b[16:], 22)
		binary.LittleEndian.PutUint16(b[24:], 1)
		copy(b[26:], pcmGUID)
		b[39] = guidByte
		return chunk{id: "fmt ", body: b}
	}
	for _, c := range []struct {
		name string
		file []byte
		want []float32
	}{
		{"mono", wavFile(pcmFormat(1, Rate), pcmData(16384, -16384)), []float32{0.5, -0.5}},
		{"stereo, mixed to mono", wavFile(pcmFormat(2, Rate), pcmData(16384, 0, -32768, 0)), []float32{0.25, -0.5}},
		{"data before fmt, and a LIST chunk", wavFile(chunk{id: "LIST", body: []byte("INFOxxxx")}, pcmData(16384), pcmFormat(1, Rate)), []float32{0.5}},
		{"a chunk of an odd size", wavFile(pcmFormat(1, Rate), odd, pcmData(16384)), []float32{0.5}},
		{"extensible PCM", wavFile(extensible(0x71), pcmData(16384)), []float32{0.5}},
		{"a data size of 0xFFFFFFFF", wavFile(pcmFormat(1, Rate), chunk{id: "data", body: pcmData(16384, 16384).body, size: 0xFFFFFFFF}), []float32{0.5, 0.5}},
		{"a data size larger than the file", wavFile(pcmFormat(1, Rate), chunk{id: "data", body: pcmData(16384).body, size: 1000}), []float32{0.5}},
		{"a last frame cut off", wavFile(pcmFormat(2, Rate), chunk{id: "data", body: append(pcmData(16384, 16384).body, 1, 2)}), []float32{0.5}},
		{"half the rate, resampled", wavFile(pcmFormat(1, Rate/2), pcmData(0, 16384)), []float32{0, 0.25, 0.5, 0.5}},
	} {
		clip, err := LoadWAV(bytes.NewReader(c.file))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if len(clip.samples) != len(c.want) {
			t.Errorf("%s: wanted %v, got %v", c.name, c.want, clip.samples)
			continue
		}
		for i := range c.want {
			if !near(clip.samples[i], c.want[i]) {
				t.Errorf("%s: wanted %v, got %v", c.name, c.want, clip.samples)
				break
			}
		}
	}
}

func TestLoadWAVRefuses(t *testing.T) {
	bits8 := pcmFormat(1, Rate)
	binary.LittleEndian.PutUint16(bits8.body[14:], 8)
	float := pcmFormat(1, Rate)
	binary.LittleEndian.PutUint16(float.body[0:], 3)
	badAlign := pcmFormat(2, Rate)
	binary.LittleEndian.PutUint16(badAlign.body[12:], 2)
	noRate := pcmFormat(1, 0)
	notPCM := make([]byte, 40)
	copy(notPCM, pcmFormat(1, Rate).body)
	binary.LittleEndian.PutUint16(notPCM[0:], 0xFFFE)
	binary.LittleEndian.PutUint16(notPCM[24:], 3) // float, in an extensible header
	copy(notPCM[26:], pcmGUID)
	for _, c := range []struct {
		name, says string
		file       []byte
	}{
		{"not a WAV file", "not a WAV file", []byte("ID3 something else entirely")},
		{"no fmt chunk", "no fmt chunk", wavFile(pcmData(1))},
		{"no data chunk", "no data chunk", wavFile(pcmFormat(1, Rate))},
		{"8-bit samples", "8-bit", wavFile(bits8, pcmData(1))},
		{"float samples", "format 3", wavFile(float, pcmData(1))},
		{"an extensible header of floats", "other than PCM", wavFile(chunk{id: "fmt ", body: notPCM}, pcmData(1))},
		{"frames of the wrong size", "frames are 2 bytes", wavFile(badAlign, pcmData(1))},
		{"a rate of 0", "0 samples a second", wavFile(noRate, pcmData(1))},
		{"a short fmt chunk", "where 16 is the least", wavFile(chunk{id: "fmt ", body: []byte{1, 0}}, pcmData(1))},
	} {
		_, err := LoadWAV(bytes.NewReader(c.file))
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: wanted an error that says %q, got %v", c.name, c.says, err)
		}
	}
	if _, err := LoadWAV(bytes.NewReader(make([]byte, maxWAV+1))); err == nil || !strings.Contains(err.Error(), "32 MiB") {
		t.Errorf("wanted a file over 32 MiB refused, got %v", err)
	}
}

// Whatever a file holds, LoadWAV returns a clip of samples from -1 to 1, or an error, and never
// panics.
func FuzzLoadWAV(f *testing.F) {
	f.Add(wavFile(pcmFormat(1, Rate), pcmData(16384, -16384)))
	f.Add(wavFile(pcmFormat(2, 22050), pcmData(1, 2, 3, 4)))
	f.Add(wavFile(chunk{id: "LIST", body: []byte{1}}, pcmFormat(1, 8000), chunk{id: "data", body: []byte{1, 2}, size: 0xFFFFFFFF}))
	f.Fuzz(func(t *testing.T, file []byte) {
		clip, err := LoadWAV(bytes.NewReader(file))
		if err != nil {
			return
		}
		for _, s := range clip.samples {
			if math.IsNaN(float64(s)) || s < -1 || s > 1 {
				t.Fatalf("a sample of %v", s)
			}
		}
	})
}

package sound

import (
	"bytes"
	"encoding/binary"
)

// steady is a clip of n samples, every one level.
func steady(level float32, n int) *Clip {
	s := make([]float32, n)
	for i := range s {
		s[i] = level
	}
	return NewClip(s)
}

// frames reads n frames of the mix and returns them as left and right.
func frames(m *Mixer, n int) (left, right []float32) {
	stereo := make([]float32, 2*n)
	m.Read(stereo)
	for i := 0; i < len(stereo); i += 2 {
		left = append(left, stereo[i])
		right = append(right, stereo[i+1])
	}
	return left, right
}

// A chunk of a WAV file.
type chunk struct {
	id   string
	body []byte
	size uint32 // what the header says; 0 means len(body)
}

// wavFile makes a WAV file of its chunks, each padded to an even length.
func wavFile(chunks ...chunk) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(0)) // writers often leave it wrong; it is not read
	b.WriteString("WAVE")
	for _, c := range chunks {
		size := c.size
		if size == 0 {
			size = uint32(len(c.body))
		}
		b.WriteString(c.id)
		binary.Write(&b, binary.LittleEndian, size)
		b.Write(c.body)
		if len(c.body)%2 == 1 && c.size == 0 {
			b.WriteByte(0)
		}
	}
	return b.Bytes()
}

// pcmFormat is a fmt chunk of 16-bit PCM.
func pcmFormat(channels, rate int) chunk {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint16(b[0:], 1)
	binary.LittleEndian.PutUint16(b[2:], uint16(channels))
	binary.LittleEndian.PutUint32(b[4:], uint32(rate))
	binary.LittleEndian.PutUint32(b[8:], uint32(rate*2*channels))
	binary.LittleEndian.PutUint16(b[12:], uint16(2*channels))
	binary.LittleEndian.PutUint16(b[14:], 16)
	return chunk{id: "fmt ", body: b}
}

// pcmData is a data chunk of 16-bit samples.
func pcmData(samples ...int16) chunk {
	b := make([]byte, 2*len(samples))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(s))
	}
	return chunk{id: "data", body: b}
}

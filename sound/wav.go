package sound

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// maxWAV is the largest WAV file LoadWAV reads: almost six minutes of mono 16-bit sound at 48 kHz.
const maxWAV = 32 << 20

// The GUID that marks PCM in a WAVE_FORMAT_EXTENSIBLE header, after its first two bytes, which
// are the format tag 1.
var pcmGUID = []byte{0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}

// LoadWAV reads a WAV file of 16-bit PCM into a clip. A file of another rate is resampled to Rate
// and one of more than one channel is mixed down to mono, since a clip is heard from where it is
// played.
//
// The file may come from anywhere: it is read up to 32 MiB, and a larger one is refused before
// it is decoded. Its chunks may come in any order, and the ones it does not need are skipped. A
// data chunk whose size is 0 or 0xFFFFFFFF, as a writer that could not seek back leaves it, or
// is larger than the file, runs to the end of the file. Anything else that is wrong is an error
// that says what.
func LoadWAV(r io.Reader) (*Clip, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxWAV+1))
	if err != nil {
		return nil, fmt.Errorf("sound: reading a WAV file: %w", err)
	}
	if len(data) > maxWAV {
		return nil, errors.New("sound: a WAV file of more than 32 MiB")
	}
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("sound: not a WAV file: it does not start with RIFF and WAVE")
	}

	var (
		format      []byte
		samples     []byte
		haveSamples bool
	)
	for at := 12; at+8 <= len(data); {
		id := string(data[at : at+4])
		size := int64(binary.LittleEndian.Uint32(data[at+4 : at+8]))
		body := at + 8
		end := body + int(min(size, int64(len(data)-body)))
		switch id {
		case "fmt ":
			format = data[body:end]
		case "data":
			if size == 0 || size == 0xFFFFFFFF {
				end = len(data)
			}
			samples, haveSamples = data[body:end], true
		}
		if size == 0xFFFFFFFF || int64(end) == int64(len(data)) {
			break
		}
		at = end + int(size&1) // a chunk of an odd size is padded to an even one
	}
	if format == nil {
		return nil, errors.New("sound: a WAV file with no fmt chunk")
	}
	if !haveSamples {
		return nil, errors.New("sound: a WAV file with no data chunk")
	}
	channels, rate, err := readFormat(format)
	if err != nil {
		return nil, err
	}

	frames := len(samples) / (2 * channels) // a cut-off last frame is dropped
	mono := make([]float32, frames)
	for i := range mono {
		var sum int32
		for c := range channels {
			sum += int32(int16(binary.LittleEndian.Uint16(samples[2*(i*channels+c):])))
		}
		mono[i] = float32(sum) / float32(channels) / 32768
	}
	return &Clip{samples: resample(mono, rate)}, nil
}

// readFormat reads a fmt chunk, and says why where it is not 16-bit PCM that LoadWAV can play.
func readFormat(f []byte) (channels, rate int, err error) {
	if len(f) < 16 {
		return 0, 0, fmt.Errorf("sound: a WAV fmt chunk of %d bytes, where 16 is the least", len(f))
	}
	tag := binary.LittleEndian.Uint16(f[0:2])
	channels = int(binary.LittleEndian.Uint16(f[2:4]))
	rate = int(binary.LittleEndian.Uint32(f[4:8]))
	align := int(binary.LittleEndian.Uint16(f[12:14]))
	bits := int(binary.LittleEndian.Uint16(f[14:16]))
	if tag == 0xFFFE {
		if len(f) < 40 || binary.LittleEndian.Uint16(f[24:26]) != 1 || !bytes.Equal(f[26:40], pcmGUID) {
			return 0, 0, errors.New("sound: a WAV file of a kind other than PCM")
		}
		tag = 1
	}
	switch {
	case tag != 1:
		return 0, 0, fmt.Errorf("sound: a WAV file of format %d, where only PCM, format 1, is read", tag)
	case bits != 16:
		return 0, 0, fmt.Errorf("sound: a WAV file of %d-bit samples, where only 16-bit ones are read", bits)
	case channels < 1:
		return 0, 0, errors.New("sound: a WAV file with no channels")
	case align != 2*channels:
		return 0, 0, fmt.Errorf("sound: a WAV file whose frames are %d bytes, where %d channels of 16 bits take %d", align, channels, 2*channels)
	case rate < 1 || rate > 384000:
		return 0, 0, fmt.Errorf("sound: a WAV file at %d samples a second, outside 1 to 384,000", rate)
	}
	return channels, rate, nil
}

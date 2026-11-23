package sound

import (
	"math"
	"time"
)

// Rate is the samples a second of every clip and of the mix, in each channel.
const Rate = 48000

// A Clip is a sound to play: mono samples at Rate, from -1 to 1. A clip does not change once it
// is made, so one clip can play in many voices at once.
type Clip struct {
	samples []float32
}

// NewClip makes a clip of samples at Rate, from -1 to 1: a sound made in code. The samples are
// copied, a sample outside -1 to 1 is cut to it, and one that is not a number is 0.
func NewClip(samples []float32) *Clip {
	c := &Clip{samples: make([]float32, len(samples))}
	for i, s := range samples {
		switch {
		case s != s: // NaN
		case s > 1:
			c.samples[i] = 1
		case s < -1:
			c.samples[i] = -1
		default:
			c.samples[i] = s
		}
	}
	return c
}

// Duration is how long the clip plays.
func (c *Clip) Duration() time.Duration {
	return time.Duration(len(c.samples)) * time.Second / Rate
}

// resample turns samples at the rate from into samples at Rate, joining neighbors with straight
// lines.
func resample(samples []float32, from int) []float32 {
	if from == Rate || len(samples) == 0 {
		return samples
	}
	n := int(math.Ceil(float64(len(samples)) * Rate / float64(from)))
	out := make([]float32, n)
	step := float64(from) / Rate
	last := len(samples) - 1
	for i := range out {
		pos := float64(i) * step
		j := int(pos)
		if j >= last {
			out[i] = samples[last]
			continue
		}
		part := float32(pos - float64(j))
		out[i] = samples[j]*(1-part) + samples[j+1]*part
	}
	return out
}

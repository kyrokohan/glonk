package engine

import "time"

// clock is where a meter reads the time, which tests count the reads of.
var clock = time.Now

// meter takes the numbers of Stats for Run. A meter that is off reads no clock.
type meter struct {
	on   bool
	fps  float64
	mark time.Time // the end of the last lap
}

// start begins a frame.
func (m *meter) start() {
	if m.on {
		m.mark = clock()
	}
}

// lap returns the time since the last lap, or since start, and begins the next one.
func (m *meter) lap() time.Duration {
	if !m.on {
		return 0
	}
	now := clock()
	d := now.Sub(m.mark)
	m.mark = now
	return d
}

// rate folds the dt of a frame into the average and returns it. The first frame sets it, and
// a dt that is not a positive number leaves it as it was.
func (m *meter) rate(dt float64) float64 {
	switch {
	case !(dt > 0):
	case m.fps == 0:
		m.fps = 1 / dt
	default:
		m.fps = 0.9*m.fps + 0.1/dt
	}
	return m.fps
}

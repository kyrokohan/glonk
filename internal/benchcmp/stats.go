package main

import (
	"math"
	"slices"
)

// median returns the middle of the values, or the mean of the middle two, and NaN for none.
func median(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := slices.Clone(v)
	slices.Sort(s)
	if n := len(s); n%2 == 0 {
		return (s[n/2-1] + s[n/2]) / 2
	}
	return s[len(s)/2]
}

// A kind says what to make of a benchmark's change.
type kind int

const (
	same   kind = iota // no change that is clearly more than the noise
	slower             // clearly slower than the base
	faster             // clearly faster
	noisy              // two runs of the base differ by more than the threshold, so a smaller change cannot be told
)

func (k kind) String() string {
	return [...]string{"same", "slower", "faster", "noisy"}[k]
}

// A series is what one benchmark did in every round: the base, the base again as a control, and
// the head, in ns/op, and the allocations of the base and the head.
type series struct {
	base, control, head    []float64
	baseAllocs, headAllocs []float64
}

// An outcome is the comparison of one benchmark.
type outcome struct {
	base, head  float64 // the best run of each, the smallest ns/op
	delta       float64 // head over base by the best runs, less 1, as a fraction
	typical     float64 // the same by the median runs
	noise       float64 // how far the control's best run is from the base's, as a fraction
	kind        kind
	allocsKnown bool // both sides counted allocations

	baseAllocs, headAllocs float64
}

// allocsUp reports whether the head allocates more than the base. It is exact where the count is
// small, which is where it matters, since several benchmarks promise none: any rise counts. The
// count of a benchmark that allocates hundreds of thousands of times varies by one or two from
// the runtime's own bookkeeping, so a rise of no more than a thousandth is not one.
func (o outcome) allocsUp() bool {
	return o.allocsKnown && o.headAllocs-o.baseAllocs > o.baseAllocs/1000
}

// allocsChanged reports whether the head allocates a different number of times, by more than that
// rounding of the runtime, so that a benchmark is not listed for a count that moved by one in four
// hundred thousand.
func (o outcome) allocsChanged() bool {
	return o.allocsKnown && math.Abs(o.headAllocs-o.baseAllocs) > min(o.headAllocs, o.baseAllocs)/1000
}

// regressed reports whether the benchmark counts against the head.
func (o outcome) regressed() bool { return o.kind == slower || o.allocsUp() }

// judge compares the head of a benchmark with its base, and says whether it could: it needs a
// base and a head.
//
// The change is read two ways, and both must agree. By the best run of each, the smallest, which a
// computer that is busy with something else for a moment cannot make look faster, so that it is
// steady where the median is not; and by the median run, the typical one, which a single lucky
// round cannot make look different. The noise is how far the control, the same code again, is from
// the base by the best run. A benchmark is slower, or faster, if both changes are beyond the threshold, a
// fraction, and the best run's is also beyond twice the noise. Where it is not, and the noise is
// itself beyond the threshold, the benchmark is noisy: a change as big as the threshold could be
// hiding in it. Run on identical code, this found nothing in 151 benchmarks, and found a change of 5
// percent in 147 of them.
func judge(s series, threshold float64) (outcome, bool) {
	if len(s.base) == 0 || len(s.head) == 0 {
		return outcome{}, false
	}
	o := outcome{base: slices.Min(s.base), head: slices.Min(s.head)}
	if o.base > 0 {
		o.delta = o.head/o.base - 1
		if mb := median(s.base); mb > 0 {
			o.typical = median(s.head)/mb - 1
		}
		if len(s.control) > 0 {
			o.noise = math.Abs(slices.Min(s.control)/o.base - 1)
		}
	}
	limit := max(threshold, 2*o.noise)
	switch {
	case o.delta > limit && o.typical > threshold:
		o.kind = slower
	case o.delta < -limit && o.typical < -threshold:
		o.kind = faster
	case o.noise > threshold:
		o.kind = noisy
	}
	if len(s.baseAllocs) > 0 && len(s.headAllocs) > 0 {
		o.allocsKnown = true
		o.baseAllocs, o.headAllocs = median(s.baseAllocs), median(s.headAllocs)
	}
	return o, true
}

// percentile returns the p-th fraction of the values, by the nearest rank, and NaN for none.
func percentile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := slices.Clone(v)
	slices.Sort(s)
	return s[max(int(math.Ceil(p*float64(len(s))))-1, 0)]
}

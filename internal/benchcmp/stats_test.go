package main

import (
	"math"
	"testing"
)

func TestMedian(t *testing.T) {
	for name, tt := range map[string]struct {
		in   []float64
		want float64
	}{
		"one":              {[]float64{7}, 7},
		"odd":              {[]float64{5, 1, 3}, 3},
		"even":             {[]float64{4, 1, 3, 2}, 2.5},
		"not changed":      {[]float64{9, 9, 1, 9, 9}, 9},
		"two of one value": {[]float64{2, 2}, 2},
		"negative":         {[]float64{-3, -1, -2}, -2},
	} {
		in := append([]float64(nil), tt.in...)
		if got := median(tt.in); got != tt.want {
			t.Errorf("%s: wanted %v, got %v", name, tt.want, got)
		}
		for i := range in {
			if in[i] != tt.in[i] {
				t.Errorf("%s: wanted the values left in their order", name)
			}
		}
	}
	if got := median(nil); !math.IsNaN(got) {
		t.Errorf("wanted NaN for no values, got %v", got)
	}
}

func TestPercentile(t *testing.T) {
	v := []float64{10, 1, 5, 2, 9, 3, 8, 4, 7, 6}
	for p, want := range map[float64]float64{0.5: 5, 0.9: 9, 1: 10, 0.1: 1, 0: 1, 0.95: 10} {
		if got := percentile(v, p); got != want {
			t.Errorf("%v: wanted %v, got %v", p, want, got)
		}
	}
	if got := percentile(nil, 0.9); !math.IsNaN(got) {
		t.Errorf("wanted NaN for no values, got %v", got)
	}
}

// rounds makes the values of every round the same, which is what a run with no spread gives.
func rounds(v float64) []float64 { return []float64{v, v, v, v, v} }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestJudge(t *testing.T) {
	const threshold = 0.03
	tests := []struct {
		name                 string
		base, control, head  float64
		want                 kind
		wantDelta, wantNoise float64
	}{
		{"nothing changed", 100, 100, 100, same, 0, 0},
		{"a little slower, under the threshold", 100, 100, 102, same, 0.02, 0},
		{"slower, over the threshold", 100, 100, 104, slower, 0.04, 0},
		{"faster, over the threshold", 100, 100, 90, faster, -0.10, 0},
		{"a little faster, under the threshold", 100, 100, 98, same, -0.02, 0},
		{"slower, but within twice the noise", 100, 102, 103.5, same, 0.035, 0.02},
		{"slower, over twice the noise", 100, 102, 106, slower, 0.06, 0.02},
		{"faster, over twice the noise", 100, 102, 94, faster, -0.06, 0.02},
		{"the control faster is noise as well", 100, 98, 106, slower, 0.06, 0.02},
		{"noisy, and not changed", 100, 105, 100, noisy, 0, 0.05},
		{"noisy, and a small change", 100, 105, 108, noisy, 0.08, 0.05},
		{"noisy, and a change that is clear", 100, 105, 112, slower, 0.12, 0.05},
		{"noisy, and clearly faster", 100, 105, 70, faster, -0.30, 0.05},
		{"a base of nothing", 0, 0, 5, same, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := series{base: rounds(tt.base), control: rounds(tt.control), head: rounds(tt.head)}
			got, ok := judge(s, threshold)
			if !ok {
				t.Fatal("wanted a comparison")
			}
			if got.kind != tt.want || !near(got.delta, tt.wantDelta) || !near(got.noise, tt.wantNoise) {
				t.Errorf("wanted %v, change %v and noise %v, got %v, %v and %v", tt.want, tt.wantDelta, tt.wantNoise, got.kind, got.delta, got.noise)
			}
		})
	}
}

// The best run and the typical run must agree, since either alone can be fooled: a round in which
// the machine was busy makes the typical run look slower, and a round in which it was idle makes
// the best look faster.
func TestJudgeNeedsTheBestRunAndTheTypicalRunToAgree(t *testing.T) {
	steady := rounds(100)
	for name, tt := range map[string]struct {
		base, head []float64
		want       kind
	}{
		"both slower":                            {steady, []float64{110, 110, 110, 110, 110}, slower},
		"the best is slower, the typical is not": {[]float64{100, 100, 100, 100, 100}, []float64{105, 101, 101, 101, 101}, same},
		"the typical is slower, the best is not": {[]float64{100, 100, 100, 100, 100}, []float64{100, 110, 110, 110, 110}, same},
		"both faster":                            {steady, []float64{90, 90, 90, 90, 90}, faster},
		"one lucky fast run":                     {steady, []float64{60, 110, 110, 110, 110}, same},
		"one busy slow run in the head":          {steady, []float64{100, 100, 100, 100, 900}, same},
		"one busy slow run in the base":          {[]float64{100, 100, 100, 100, 900}, []float64{100, 100, 100, 100, 100}, same},
		"a base that is mostly slow":             {[]float64{100, 130, 130, 130, 130}, rounds(110), same},
	} {
		got, _ := judge(series{base: tt.base, control: steady, head: tt.head}, 0.03)
		if got.kind != tt.want {
			t.Errorf("%s: wanted %v, got %v (best %v, typical %v)", name, tt.want, got.kind, got.delta, got.typical)
		}
	}
}

// Each of the best run, the typical run and the control has to be read on its own terms.
func TestJudgeReadsTheBestTheTypicalAndTheControlEachOnItsOwn(t *testing.T) {
	// The best is under the threshold and the typical is over it: not slower.
	got, _ := judge(series{base: rounds(100), control: rounds(100), head: []float64{102, 105, 105, 105, 105}}, 0.03)
	if got.kind != same || !near(got.delta, 0.02) || !near(got.typical, 0.05) {
		t.Errorf("wanted same for a best of +2%% and a typical of +5%%, got %+v", got)
	}
	// The control's best run is the base's, though its typical run is not: no noise.
	got, _ = judge(series{base: rounds(100), control: []float64{100, 110, 110, 110, 110}, head: rounds(104)}, 0.03)
	if got.kind != slower || got.noise != 0 {
		t.Errorf("wanted slower with no noise, since the control's best run is the base's, got %+v", got)
	}
	// The typical change exactly at the threshold is not beyond it, though the best one is.
	base := []float64{100, 120, 120, 120, 120}
	if got, _ = judge(series{base: base, control: base, head: rounds(180)}, 0.5); got.kind != same || !near(got.typical, 0.5) {
		t.Errorf("wanted same for a typical change of exactly the threshold, got %+v", got)
	}
	if got, _ = judge(series{base: base, control: base, head: rounds(181)}, 0.5); got.kind != slower {
		t.Errorf("wanted slower for a typical change just over the threshold, got %+v", got)
	}
}

func TestJudgeShowsBothChanges(t *testing.T) {
	s := series{base: []float64{100, 120, 130, 130, 130}, control: []float64{100, 100, 100, 100, 100}, head: []float64{110, 140, 150, 150, 150}}
	got, _ := judge(s, 0.03)
	if got.base != 100 || got.head != 110 || !near(got.delta, 0.10) || !near(got.typical, 150.0/130-1) {
		t.Errorf("wanted the best runs 100 and 110, a change of 10%% and a typical change of 15.4%%, got %+v", got)
	}
}

func TestJudgeNeedsABaseAndAHead(t *testing.T) {
	if _, ok := judge(series{head: rounds(1)}, 0.03); ok {
		t.Errorf("wanted nothing to compare a head with, without a base")
	}
	if _, ok := judge(series{base: rounds(1), control: rounds(1)}, 0.03); ok {
		t.Errorf("wanted nothing to compare a base with, without a head")
	}
	// Without a control there is no noise to speak of.
	if got, ok := judge(series{base: rounds(100), head: rounds(110)}, 0.03); !ok || got.kind != slower || got.noise != 0 {
		t.Errorf("wanted slower with no noise, got %+v %v", got, ok)
	}
}

// A change exactly as big as the threshold is not beyond it, nor is one exactly twice the noise, and
// noise exactly as big as the threshold is not beyond it either.
func TestJudgeAtTheEdges(t *testing.T) {
	for name, tt := range map[string]struct {
		control, head float64
		threshold     float64
		want          kind
	}{
		"exactly the threshold":                   {100, 150, 0.5, same},
		"just over the threshold":                 {100, 151, 0.5, slower},
		"exactly twice the noise":                 {125, 150, 0.1, noisy}, // not slower, and the noise is more than the threshold
		"just over twice the noise":               {125, 151, 0.1, slower},
		"the noise exactly the threshold":         {150, 100, 0.5, same},
		"the noise just over the threshold":       {151, 100, 0.5, noisy},
		"exactly faster than the threshold":       {100, 50, 0.5, same},
		"just faster than the threshold":          {100, 49, 0.5, faster},
		"exactly as far below as twice the noise": {75, 50, 0.1, noisy},
		"just beyond twice the noise below":       {75, 49, 0.1, faster},
	} {
		s := series{base: rounds(100), control: rounds(tt.control), head: rounds(tt.head)}
		got, _ := judge(s, tt.threshold)
		if got.kind != tt.want {
			t.Errorf("%s: wanted %v, got %v (change %v, noise %v)", name, tt.want, got.kind, got.delta, got.noise)
		}
	}
}

func TestAllocationsRiseCountsExceptByARoundingOfTheRuntime(t *testing.T) {
	for name, tt := range map[string]struct {
		base, head         []float64
		known, up, changed bool
	}{
		"the same":                             {rounds(2), rounds(2), true, false, false},
		"one more, from none":                  {rounds(0), rounds(1), true, true, true},
		"one more, from many":                  {rounds(150), rounds(151), true, true, true},
		"one fewer":                            {rounds(3), rounds(2), true, false, true},
		"one fewer of hundreds of thousands":   {rounds(401833), rounds(401832), true, false, false},
		"a real fall of hundreds of thousands": {rounds(402500), rounds(401832), true, false, true},
		"a fall just over a thousandth":        {rounds(1_000_000), rounds(999_000), true, false, true},
		"a rise just over a thousandth":        {rounds(999_000), rounds(1_000_000), true, true, true},
		"not counted":                          {nil, rounds(1), false, false, false},
		"a median of counts":                   {[]float64{0, 0, 7, 0, 0}, rounds(0), true, false, false},
		"one more of hundreds of thousands":    {rounds(401832), rounds(401833), true, false, false},
		"a thousandth more":                    {rounds(1000), rounds(1001), true, false, false},
		"a little over a thousandth more":      {rounds(1000), rounds(1002), true, true, true},
		"a real rise of hundreds of thousands": {rounds(401832), rounds(402500), true, true, true},
	} {
		s := series{base: rounds(100), control: rounds(100), head: rounds(100), baseAllocs: tt.base, headAllocs: tt.head}
		got, _ := judge(s, 0.03)
		if got.allocsKnown != tt.known || got.allocsUp() != tt.up || got.allocsChanged() != tt.changed {
			t.Errorf("%s: wanted known %v, more %v and changed %v, got %+v", name, tt.known, tt.up, tt.changed, got)
		}
		// Time is the same, so only allocations can make it a regression.
		if got.regressed() != tt.up {
			t.Errorf("%s: wanted regressed to be %v", name, tt.up)
		}
	}
}

func TestOnlySlowerAndMoreAllocationsAreRegressions(t *testing.T) {
	for k, want := range map[kind]bool{same: false, faster: false, noisy: false, slower: true} {
		if got := (outcome{kind: k}).regressed(); got != want {
			t.Errorf("%v: wanted %v, got %v", k, want, got)
		}
	}
	// Faster, and allocating more: still a regression.
	if !(outcome{kind: faster, allocsKnown: true, baseAllocs: 0, headAllocs: 1}).regressed() {
		t.Errorf("wanted more allocations to count even when it is faster")
	}
}

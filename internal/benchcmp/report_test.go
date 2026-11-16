package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatTime(t *testing.T) {
	for ns, want := range map[float64]string{
		0: "0.0ns", 15.18: "15.2ns", 999.4: "999.4ns", 1000: "1.00us", 3584: "3.58us",
		1_598_179: "1.60ms", 2_778_387_000: "2.78s", 1e9: "1.00s",
	} {
		if got := formatTime(ns); got != want {
			t.Errorf("%v: wanted %q, got %q", ns, want, got)
		}
	}
}

func TestPercent(t *testing.T) {
	for f, want := range map[float64]string{0: "+0.0%", 0.034: "+3.4%", -0.1: "-10.0%", 1.5: "+150.0%"} {
		if got := percent(f); got != want {
			t.Errorf("%v: wanted %q, got %q", f, want, got)
		}
	}
}

func TestReportListsWhatChangedAndCountsTheRest(t *testing.T) {
	entries := []entry{
		{"bitmap", "Draw", outcome{base: 3600, head: 3560, delta: -0.011, typical: -0.009, noise: 0.004, kind: same}},
		{"mesh", "DrawFog", outcome{base: 58_600, head: 63_900, delta: 0.09, typical: 0.081, noise: 0.006, kind: slower}},
		{"tilemap", "Route/To", outcome{base: 1_600_000, head: 1_500_000, delta: -0.0625, typical: -0.07, noise: 0.01, kind: faster}},
		{"engine", "Compose", outcome{base: 900, head: 905, delta: 0.0055, typical: 0.02, noise: 0.05, kind: noisy}},
		{"sprite", "Draw", outcome{base: 100, head: 101, delta: 0.01, typical: 0.012, noise: 0.002, kind: same, allocsKnown: true, baseAllocs: 0, headAllocs: 1}},
	}
	var buf bytes.Buffer
	got := report(&buf, header{head: "the working tree", base: "abc123 (main)", rounds: 5, benchtime: "200ms", threshold: 0.03},
		entries, []string{"mesh NewThing"}, nil, false)
	want := `head: the working tree
base: abc123 (main)
5 rounds of 200ms, each running the base, the base again and the head
noise, the base's best run against its own again, over 5 benchmarks: median 0.6%, 90% within 5.0%, worst 5.0% (engine Compose)
reported: a change of more than 3.0% by the best and by the typical run, and of more than twice the noise of that benchmark by the best, or more allocations

verdict  package  benchmark  base     head     change  typical  noise  allocs
slower   mesh     DrawFog    58.60us  63.90us  +9.0%   +8.1%    0.6%   
same     sprite   Draw       100.0ns  101.0ns  +1.0%   +1.2%    0.2%   0 -> 1
noisy    engine   Compose    900.0ns  905.0ns  +0.5%   +2.0%    5.0%   
faster   tilemap  Route/To   1.60ms   1.50ms   -6.2%   -7.0%    1.0%   

5 benchmarks compared: 1 slower, 1 faster, 1 noisy, 2 the same; 1 allocate more
only in the head, not compared (1):
  mesh NewThing
`
	if buf.String() != want {
		t.Errorf("wanted\n%s\ngot\n%s", want, buf.String())
	}
	if got != 2 { // the one that is slower, and the one that allocates more
		t.Errorf("wanted 2 regressions, got %d", got)
	}
}

func TestReportWithAllListsEverythingAndNothingChangedIsShort(t *testing.T) {
	entries := []entry{
		{"a", "X", outcome{base: 10, head: 10, kind: same}},
		{"a", "Y", outcome{base: 20, head: 20, kind: same}},
	}
	var short, long bytes.Buffer
	h := header{head: "h", base: "b", rounds: 3, benchtime: "1s", threshold: 0.03}
	if n := report(&short, h, entries, nil, nil, false); n != 0 {
		t.Errorf("wanted no regressions, got %d", n)
	}
	report(&long, h, entries, nil, nil, true)
	if strings.Contains(short.String(), "verdict") || !strings.Contains(short.String(), "2 benchmarks compared: 0 slower, 0 faster, 0 noisy, 2 the same; 0 allocate more") {
		t.Errorf("wanted only the count, got\n%s", short.String())
	}
	if !strings.Contains(long.String(), "verdict") || strings.Count(long.String(), "\nsame ") != 2 {
		t.Errorf("wanted both rows, got\n%s", long.String())
	}
	// With nothing compared there is no noise to say.
	var none bytes.Buffer
	report(&none, h, nil, nil, []string{"a X"}, false)
	if strings.Contains(none.String(), "noise, the base") || !strings.Contains(none.String(), "only in the base, not compared (1):\n  a X") {
		t.Errorf("wanted no noise line and the one in the base, got\n%s", none.String())
	}
}

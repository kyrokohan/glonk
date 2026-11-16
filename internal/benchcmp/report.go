package main

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"
)

// An entry is a benchmark and how it compared.
type entry struct {
	pkg, name string
	o         outcome
}

// A header says what was compared, and how.
type header struct {
	head, base string
	rounds     int
	benchtime  string
	threshold  float64  // a fraction
	notRun     []string // the packages whose test binary is the base's, each with how many benchmarks it has
}

// formatTime writes a time in nanoseconds as ns, us, ms or s, with the digits that matter.
func formatTime(ns float64) string {
	switch {
	case ns >= 1e9:
		return fmt.Sprintf("%.2fs", ns/1e9)
	case ns >= 1e6:
		return fmt.Sprintf("%.2fms", ns/1e6)
	case ns >= 1e3:
		return fmt.Sprintf("%.2fus", ns/1e3)
	}
	return fmt.Sprintf("%.1fns", ns)
}

// percent writes a fraction as a signed percentage.
func percent(f float64) string { return fmt.Sprintf("%+.1f%%", f*100) }

// report writes what changed and returns how many benchmarks regressed. It lists the ones that
// are not the same, the slowest first, or all of them, and then counts the rest.
func report(w io.Writer, h header, entries []entry, onlyHead, onlyBase []string, all bool) int {
	fmt.Fprintf(w, "head: %s\nbase: %s\n", h.head, h.base)
	if h.rounds > 0 {
		fmt.Fprintf(w, "%d rounds of %s, each running the base, the base again and the head\n", h.rounds, h.benchtime)
	}

	if len(h.notRun) > 0 {
		fmt.Fprintf(w, "not run, since their test binary is the same file as the base's, so it is the same code: %s\n", strings.Join(h.notRun, ", "))
	}

	noises := make([]float64, len(entries))
	worst := 0
	for i, e := range entries {
		noises[i] = e.o.noise
		if e.o.noise > entries[worst].o.noise {
			worst = i
		}
	}
	if len(entries) > 0 {
		fmt.Fprintf(w, "noise, the base's best run against its own again, over %d benchmarks: median %s, 90%% within %s, worst %s (%s %s)\n",
			len(entries), percent(median(noises))[1:], percent(percentile(noises, 0.9))[1:], percent(entries[worst].o.noise)[1:],
			entries[worst].pkg, entries[worst].name)
	}
	fmt.Fprintf(w, "reported: a change of more than %s by the best and by the typical run, and of more than twice the noise of that benchmark by the best, or more allocations\n\n", percent(h.threshold)[1:])

	shown := slices.Clone(entries)
	shown = slices.DeleteFunc(shown, func(e entry) bool {
		return !all && e.o.kind == same && !e.o.allocsChanged()
	})
	slices.SortStableFunc(shown, func(a, b entry) int {
		switch {
		case a.o.delta > b.o.delta:
			return -1
		case a.o.delta < b.o.delta:
			return 1
		}
		return strings.Compare(a.pkg+a.name, b.pkg+b.name)
	})
	if len(shown) > 0 {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "verdict\tpackage\tbenchmark\tbase\thead\tchange\ttypical\tnoise\tallocs")
		for _, e := range shown {
			allocs := ""
			if e.o.allocsChanged() {
				allocs = fmt.Sprintf("%g -> %g", e.o.baseAllocs, e.o.headAllocs)
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", e.o.kind, e.pkg, e.name,
				formatTime(e.o.base), formatTime(e.o.head), percent(e.o.delta), percent(e.o.typical), percent(e.o.noise)[1:], allocs)
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	var counts [4]int
	regressions, moreAllocs := 0, 0
	for _, e := range entries {
		counts[e.o.kind]++
		if e.o.regressed() {
			regressions++
		}
		if e.o.allocsUp() {
			moreAllocs++
		}
	}
	fmt.Fprintf(w, "%d benchmarks compared: %d slower, %d faster, %d noisy, %d the same; %d allocate more\n",
		len(entries), counts[slower], counts[faster], counts[noisy], counts[same], moreAllocs)
	for _, list := range []struct {
		side  string
		names []string
	}{{"head", onlyHead}, {"base", onlyBase}} {
		if len(list.names) > 0 {
			fmt.Fprintf(w, "only in the %s, not compared (%d):\n  %s\n", list.side, len(list.names), strings.Join(list.names, "\n  "))
		}
	}
	return regressions
}

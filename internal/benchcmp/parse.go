package main

import (
	"strconv"
	"strings"
)

// A line is one benchmark's result: its name, without the -N that tells how many processors it
// ran on, and the numbers this tool compares.
type line struct {
	name      string
	ns        float64 // time for each operation
	allocs    float64 // allocations for each operation, where the run counted them
	hasAllocs bool
}

// parse returns the results in the output of a test binary run with -test.bench. Every other
// line, such as ok, PASS and anything a benchmark prints, is skipped, and so is a result that has
// no ns/op.
func parse(out string) []line {
	var lines []line
	for _, text := range strings.Split(out, "\n") {
		f := strings.Fields(text)
		if len(f) < 4 || !strings.HasPrefix(f[0], "Benchmark") {
			continue
		}
		if _, err := strconv.ParseInt(f[1], 10, 64); err != nil { // how many times it ran
			continue
		}
		l := line{name: trimProcs(f[0])}
		hasNS := false
		// The rest is pairs of a number and a unit.
		for i := 2; i+1 < len(f); i += 2 {
			v, err := strconv.ParseFloat(f[i], 64)
			if err != nil {
				break
			}
			switch f[i+1] {
			case "ns/op":
				l.ns, hasNS = v, true
			case "allocs/op":
				l.allocs, l.hasAllocs = v, true
			}
		}
		if hasNS {
			lines = append(lines, l)
		}
	}
	return lines
}

// trimProcs removes the -N that a benchmark's name ends with when it ran on more than one
// processor.
func trimProcs(name string) string {
	i := strings.LastIndexByte(name, '-')
	if i < 0 || i == len(name)-1 {
		return name
	}
	if _, err := strconv.ParseUint(name[i+1:], 10, 32); err != nil {
		return name
	}
	return name[:i]
}

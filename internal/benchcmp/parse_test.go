package main

import (
	"slices"
	"testing"
)

func TestParseReadsTheNumbersOfEveryResult(t *testing.T) {
	out := `goos: darwin
goarch: arm64
pkg: glonk.dev/bitmap
cpu: Apple M1 Pro
BenchmarkBitmap/Draw,_64_by_64-10         	  307018	      3584 ns/op	       0 B/op	       0 allocs/op
BenchmarkBitmap/Sub-10                    	78415201	        15.18 ns/op	      56 B/op	       1 allocs/op
BenchmarkNoMemory-10                      	 1000000	      1.5 ns/op
BenchmarkWithRate-10                      	 1000000	      2.5 ns/op	 120.5 MB/s	       3 allocs/op
BenchmarkOneProcessor                     	      10	    100 ns/op
BenchmarkSize-10-4                        	     100	     50 ns/op
PASS
ok  	glonk.dev/bitmap	3.2s
`
	want := []line{
		{"BenchmarkBitmap/Draw,_64_by_64", 3584, 0, true},
		{"BenchmarkBitmap/Sub", 15.18, 1, true},
		{"BenchmarkNoMemory", 1.5, 0, false},
		{"BenchmarkWithRate", 2.5, 3, true},
		{"BenchmarkOneProcessor", 100, 0, false},
		{"BenchmarkSize-10", 50, 0, false},
	}
	if got := parse(out); !slices.Equal(got, want) {
		t.Errorf("wanted %+v, got %+v", want, got)
	}
}

func TestParseSkipsWhatIsNotAResult(t *testing.T) {
	for name, out := range map[string]string{
		"empty":               "",
		"a failed benchmark":  "--- FAIL: BenchmarkX\n    x_test.go:10: it broke\nFAIL\n",
		"a name and no count": "BenchmarkX-10 ns/op\n",
		"a log line":          "BenchmarkX: starting up\n",
		"no ns/op":            "BenchmarkX-10 100 5 B/op\n",
		"a bad number":        "BenchmarkX-10 100 abc ns/op\n",
		"too short":           "BenchmarkX-10 100\n",
		"another prefix":      "TestX 100 5 ns/op\n",
	} {
		if got := parse(out); len(got) != 0 {
			t.Errorf("%s: wanted nothing, got %+v", name, got)
		}
	}
}

func TestTrimProcsRemovesOnlyTheProcessorCount(t *testing.T) {
	for name, want := range map[string]string{
		"BenchmarkX-10":      "BenchmarkX",
		"BenchmarkX-1":       "BenchmarkX",
		"BenchmarkX":         "BenchmarkX",
		"BenchmarkX/a-b-8":   "BenchmarkX/a-b",
		"BenchmarkX/a-b":     "BenchmarkX/a-b",
		"BenchmarkX-":        "BenchmarkX-",
		"BenchmarkX-1e3":     "BenchmarkX-1e3",
		"BenchmarkX-100-100": "BenchmarkX-100",
		"":                   "",
	} {
		if got := trimProcs(name); got != want {
			t.Errorf("%q: wanted %q, got %q", name, want, got)
		}
	}
}

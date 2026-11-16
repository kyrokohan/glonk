// Command benchcmp compares the benchmarks of the working tree with those of an earlier commit,
// for a change that might have made drawing, loading or searching slower.
//
//	go run ./internal/benchcmp -base origin/main
//
// It builds the test binaries of both, and runs them in rounds. In every round it runs the
// base, the base again and the head, one after the other, in an order that rotates, so that a
// computer that slows down or warms up over the minutes affects all three alike. The base again
// is a control: the same code again shows how much a result varies by chance, and a change is
// reported only if it is clearly more than that, by the best run and by the typical run. Allocations
// hardly vary, so any rise in them is reported. A package whose test binary is the same file for both
// is not run: built without file paths, the same code is the same binary, so there is nothing to
// compare and any difference would be noise. See Makefile (make benchcmp) and CONTRIBUTING.md.
//
// The exit status is 0 if nothing regressed, 1 if something did, and 2 if the comparison could
// not be made.
package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// A config is what a run does.
type config struct {
	dir       string  // the tree to measure: the working tree of a module
	base      string  // the commit, tag or branch to compare with
	bench     string  // a regular expression for the benchmarks to run
	pkgs      string  // the packages, as for go test
	rounds    int     // how many times each of the three runs
	benchtime string  // how long each benchmark runs, as for go test
	threshold float64 // the smallest change to report, a fraction
	all       bool    // list every benchmark
	identical bool    // run the packages whose test binary is the same too, to measure the noise
}

func main() {
	c, err := parseFlags(os.Args[1:], os.Stderr)
	if err != nil {
		os.Exit(2)
	}
	os.Exit(run(c, os.Stdout, os.Stderr))
}

// parseFlags reads the command line, and says what is wrong with it on w. The threshold is given in
// percent and kept as a fraction.
func parseFlags(args []string, w io.Writer) (config, error) {
	c := config{dir: "."}
	var percent float64
	fs := flag.NewFlagSet("benchcmp", flag.ContinueOnError)
	fs.SetOutput(w)
	fs.StringVar(&c.base, "base", "origin/main", "the commit, tag or branch to compare with")
	fs.StringVar(&c.bench, "bench", ".", "run only the benchmarks that match this regular expression")
	fs.StringVar(&c.pkgs, "pkgs", "./...", "the packages to run, as for go test")
	fs.IntVar(&c.rounds, "rounds", 5, "how many rounds to run")
	fs.StringVar(&c.benchtime, "benchtime", "200ms", "how long each benchmark runs, as for go test")
	fs.Float64Var(&percent, "threshold", 3, "the smallest change to report, in percent")
	fs.BoolVar(&c.all, "all", false, "list every benchmark, not only the ones that changed")
	fs.BoolVar(&c.identical, "identical", false, "run the packages whose test binary is the same as the base's too, to measure the noise")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	c.threshold = percent / 100
	return c, nil
}

// A unit is a package whose benchmarks are run.
type unit struct {
	same       bool   // the test binary of the base is the same file as the head's: the same code
	benchmarks int    // how many benchmarks the head has
	short      string // for the report: the path without the module's
	headBin    string
	headDir    string
	baseBin    string // empty where the base has no such package
	baseDir    string
}

// run compares and reports, and returns the exit status.
func run(c config, stdout, stderr io.Writer) int {
	if c.rounds < 1 || c.threshold < 0 {
		fmt.Fprintln(stderr, "benchcmp: rounds must be at least 1 and the threshold not negative")
		return 2
	}
	data, h, err := measure(c, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "benchcmp:", err)
		return 2
	}

	var entries []entry
	var onlyHead, onlyBase []string
	for _, k := range sortedKeys(data) {
		o, ok := judge(*data[k], c.threshold)
		switch {
		case ok:
			entries = append(entries, entry{pkg: k.pkg, name: k.name, o: o})
		case len(data[k].head) > 0:
			onlyHead = append(onlyHead, k.String())
		default:
			onlyBase = append(onlyBase, k.String())
		}
	}

	h.threshold = c.threshold
	regressions := report(stdout, h, entries, onlyHead, onlyBase, c.all)
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		for _, e := range entries {
			if e.o.regressed() {
				fmt.Fprintf(stdout, "::warning title=benchmark regression::%s %s is %s", e.pkg, e.name, percent(e.o.delta))
				if e.o.allocsChanged() {
					fmt.Fprintf(stdout, " (allocations %g -> %g)", e.o.baseAllocs, e.o.headAllocs)
				}
				fmt.Fprintln(stdout)
			}
		}
	}
	if regressions > 0 {
		return 1
	}
	return 0
}

// A key names a benchmark.
type key struct{ pkg, name string }

func (k key) String() string { return k.pkg + " " + k.name }

func sortedKeys(m map[key]*series) []key {
	keys := make([]key, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b key) int { return strings.Compare(a.String(), b.String()) })
	return keys
}

// measure builds and runs everything, and returns what every benchmark did in each round.
func measure(c config, stderr io.Writer) (map[key]*series, header, error) {
	var h header
	root, err := output(c.dir, "go", "list", "-m", "-f", "{{.Dir}}")
	if err != nil {
		return nil, h, err
	}
	// Both without links, since git says where the repository is by its real path and go may not
	// (/var is /private/var on a Mac), and they must agree to find where the module is in it.
	root, err = filepath.EvalSymlinks(strings.TrimSpace(root))
	if err != nil {
		return nil, h, err
	}
	top, err := output(root, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, h, fmt.Errorf("the base is read from git, and %s is not in a repository: %w", root, err)
	}
	top, err = filepath.EvalSymlinks(strings.TrimSpace(top))
	if err != nil {
		return nil, h, err
	}
	rel, err := filepath.Rel(top, root)
	if err != nil {
		return nil, h, err
	}

	tmp, err := os.MkdirTemp("", "benchcmp-")
	if err != nil {
		return nil, h, err
	}
	defer os.RemoveAll(tmp)

	sha, err := extractBase(root, c.base, filepath.Join(tmp, "base"))
	if err != nil {
		return nil, h, err
	}
	baseRoot := filepath.Join(tmp, "base", rel)
	if _, err := os.Stat(filepath.Join(baseRoot, "go.mod")); err != nil {
		return nil, h, fmt.Errorf("the module is not in the base commit %s", sha[:min(len(sha), 12)])
	}
	h = header{head: "the working tree", base: fmt.Sprintf("%s (%s)", sha[:min(len(sha), 12)], c.base), rounds: c.rounds, benchtime: c.benchtime}

	units, err := buildUnits(c, root, baseRoot, tmp, stderr)
	if err != nil {
		return nil, h, err
	}
	for _, u := range units {
		if u.same {
			h.notRun = append(h.notRun, fmt.Sprintf("%s (%d)", u.short, u.benchmarks))
		}
	}

	data := map[key]*series{}
	at := func(pkg, name string) *series {
		k := key{pkg, name}
		if data[k] == nil {
			data[k] = &series{}
		}
		return data[k]
	}
	if !slices.ContainsFunc(units, func(u unit) bool { return !u.same }) {
		h.rounds = 0        // none were run
		return data, h, nil // the same code everywhere: nothing to run
	}
	for r := range c.rounds {
		fmt.Fprintf(stderr, "round %d of %d\n", r+1, c.rounds)
		for i, u := range units {
			if u.same {
				continue
			}
			for _, role := range order(r, i) {
				bin, dir := u.baseBin, u.baseDir
				if role == "head" {
					bin, dir = u.headBin, u.headDir
				}
				if bin == "" {
					continue
				}
				out, err := runBench(bin, dir, c)
				if err != nil {
					return nil, h, fmt.Errorf("%s (%s): %w", u.short, role, err)
				}
				for _, l := range parse(out) {
					at(u.short, l.name).add(role, l)
				}
			}
		}
	}
	return data, h, nil
}

// add records a result of a run in the role it ran in: the base and the head count allocations, and
// the control, which is the base again, only its time.
func (s *series) add(role string, l line) {
	switch role {
	case "base":
		s.base = append(s.base, l.ns)
		if l.hasAllocs {
			s.baseAllocs = append(s.baseAllocs, l.allocs)
		}
	case "control":
		s.control = append(s.control, l.ns)
	case "head":
		s.head = append(s.head, l.ns)
		if l.hasAllocs {
			s.headAllocs = append(s.headAllocs, l.allocs)
		}
	}
}

// order says in which order the base, the control and the head run for a package in a round. It
// turns from round to round and from package to package, so that none of them is always first,
// where a computer that warms up or tires would favor it.
func order(round, pkg int) [3]string {
	roles := [3]string{"base", "control", "head"}
	n := (round + pkg) % 3
	return [3]string{roles[n], roles[(n+1)%3], roles[(n+2)%3]}
}

// buildUnits lists the packages with tests and builds their test binaries, for the head and for the
// base where it has the package.
func buildUnits(c config, root, baseRoot, tmp string, stderr io.Writer) ([]unit, error) {
	out, err := output(root, "go", "list", "-f", "{{.ImportPath}}\t{{.Dir}}\t{{len .TestGoFiles}}\t{{len .XTestGoFiles}}", c.pkgs)
	if err != nil {
		return nil, err
	}
	mod, err := output(root, "go", "list", "-m")
	if err != nil {
		return nil, err
	}
	mod = strings.TrimSpace(mod)

	var units []unit
	for _, row := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Split(row, "\t")
		if len(f) != 4 || (f[2] == "0" && f[3] == "0") {
			continue // no tests
		}
		dir, err := filepath.EvalSymlinks(f[1])
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, err
		}
		u := unit{short: strings.TrimPrefix(strings.TrimPrefix(f[0], mod), "/"), headDir: dir, baseDir: filepath.Join(baseRoot, rel)}
		if u.short == "" {
			u.short = "."
		}
		fmt.Fprintf(stderr, "building %s\n", u.short)
		u.headBin = filepath.Join(tmp, fmt.Sprintf("head-%d.test", len(units)))
		if err := build(root, f[0], u.headBin); err != nil {
			return nil, err
		}
		u.benchmarks = benchmarks(u.headBin, u.headDir)
		if u.benchmarks == 0 {
			continue
		}
		if _, err := os.Stat(u.baseDir); err == nil {
			u.baseBin = filepath.Join(tmp, fmt.Sprintf("base-%d.test", len(units)))
			if err := build(baseRoot, f[0], u.baseBin); err != nil {
				return nil, fmt.Errorf("the base: %w", err)
			}
			if !c.identical {
				if u.same, err = sameFile(u.baseBin, u.headBin); err != nil {
					return nil, err
				}
			}
		}
		units = append(units, u)
	}
	return units, nil
}

// build compiles the tests of the package at the import path, in the module at dir.
func build(dir, path, out string) error {
	// Without file paths in the binary, two builds of the same code are the same binary, so that
	// only a change of code can change how fast it runs: where the binaries differ only by the
	// length of a path, data lands on other cache lines and a benchmark can differ by 4 percent.
	cmd := exec.Command("go", "test", "-c", "-trimpath", "-o", out, path)
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go test -c %s: %w\n%s", path, err, bytes.TrimSpace(b))
	}
	return nil
}

// benchmarks returns how many benchmarks the test binary has, not counting their sub-benchmarks.
func benchmarks(bin, dir string) int {
	cmd := exec.Command(bin, "-test.run=^$", "-test.list=^Benchmark")
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Benchmark") {
			n++
		}
	}
	return n
}

// sameFile reports whether two files have the same bytes.
func sameFile(a, b string) (bool, error) {
	sum := func(name string) ([sha256.Size]byte, error) {
		var out [sha256.Size]byte
		f, err := os.Open(name)
		if err != nil {
			return out, err
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return out, err
		}
		copy(out[:], h.Sum(nil))
		return out, nil
	}
	x, err := sum(a)
	if err != nil {
		return false, err
	}
	y, err := sum(b)
	if err != nil {
		return false, err
	}
	return x == y, nil
}

// runBench runs the benchmarks of a test binary once, in the package's directory.
func runBench(bin, dir string, c config) (string, error) {
	cmd := exec.Command(bin, "-test.run=^$", "-test.bench="+c.bench, "-test.benchtime="+c.benchtime, "-test.benchmem")
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w\n%s", err, bytes.TrimSpace(b))
	}
	return string(b), nil
}

// output runs a command in a directory and returns what it printed.
func output(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, bytes.TrimSpace(stderr.Bytes()))
	}
	return string(b), nil
}

// extractBase writes the files of a commit into dest, from git archive, and returns its hash.
func extractBase(dir, ref, dest string) (string, error) {
	top, err := output(dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top = strings.TrimSpace(top)
	sha, err := output(top, "git", "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("there is no commit, tag or branch %q to compare with", ref)
	}
	sha = strings.TrimSpace(sha)

	cmd := exec.Command("git", "archive", "--format=tar", sha)
	cmd.Dir = top
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	tr := tar.NewReader(pipe)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		name := filepath.FromSlash(h.Name)
		if !filepath.IsLocal(name) {
			return "", fmt.Errorf("git archive gave the path %q, which is outside the directory", h.Name)
		}
		target := filepath.Join(dest, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, h.FileInfo().Mode().Perm())
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return "", err
			}
			if err := f.Close(); err != nil {
				return "", err
			}
		}
		// Links and the rest are not needed to build.
	}
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("git archive: %w", err)
	}
	return sha, nil
}

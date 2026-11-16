package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The module that the end to end tests compare: one package with one benchmark of a loop whose length is
// work, and an allocation if alloc.
func workSource(work int, alloc bool) string {
	body := ""
	if alloc {
		body = "\tscratch = make([]int, 8)\n"
	}
	return fmt.Sprintf(`package benchmod

var sink int
var scratch []int

// Work does the same amount of work every time.
func Work() int {
%s	s := 0
	for i := 0; i < %d; i++ {
		s += i*i ^ (s >> 3)
	}
	return s
}
`, body, work)
}

const workTest = `package benchmod

import "testing"

func BenchmarkWork(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = Work()
	}
}
`

const extraTest = `
func BenchmarkExtra(b *testing.B) {
	for b.Loop() {
		sink = Work()
	}
}
`

// repo makes a git repository holding the module with the base's work, committed, and returns
// its directory. The tests then change the working tree and compare it with that commit.
func repo(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds and runs test binaries")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}
	dir := t.TempDir()
	write(t, dir, "go.mod", "module benchmod\n\ngo 1.26\n")
	write(t, dir, "work.go", workSource(2000, false))
	write(t, dir, "work_test.go", workTest)
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "base"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func write(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// compare runs the tool on dir against its commit, with a big threshold so that only a big change
// counts and the noise of a busy machine cannot, and returns the status and what it wrote.
func compare(t *testing.T, dir string) (int, string, string) {
	t.Helper()
	return compareWith(t, config{dir: dir, base: "HEAD", bench: ".", pkgs: "./...", rounds: 3, benchtime: "10ms", threshold: 0.4})
}

func compareWith(t *testing.T, c config) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(c, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// A package whose test binary is the base's, since the code is the same, is not run: there is
// nothing to compare, and a difference would be only noise.
func TestAnUnchangedTreeIsNotRun(t *testing.T) {
	dir := repo(t)
	code, out, errs := compare(t, dir)
	if code != 0 || !strings.Contains(out, "not run, since their test binary is the same file as the base's, so it is the same code: . (1)") ||
		!strings.Contains(out, "0 benchmarks compared") || strings.Contains(errs, "round 1") || strings.Contains(out, "rounds of") {
		t.Errorf("wanted status 0, the package not run and no round, got %d\n%s\n%s", code, out, errs)
	}
}

func TestAnUnchangedTreeRunWhenAskedRegressesNothing(t *testing.T) {
	dir := repo(t)
	code, out, errs := compareWith(t, config{dir: dir, base: "HEAD", bench: ".", pkgs: "./...", rounds: 3, benchtime: "10ms", threshold: 0.4, identical: true})
	if code != 0 || !strings.Contains(out, "1 benchmarks compared: 0 slower, 0 faster, ") || !strings.Contains(out, "; 0 allocate more") || strings.Contains(out, "not run") {
		t.Errorf("wanted status 0 and one benchmark neither slower nor faster, got %d\n%s\n%s", code, out, errs)
	}
	if !strings.Contains(out, "base: ") || !strings.Contains(out, "3 rounds of 10ms") || !strings.Contains(errs, "round 3 of 3") {
		t.Errorf("wanted the header and the rounds, got\n%s\n%s", out, errs)
	}
}

func TestSameFile(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a", "same bytes")
	write(t, dir, "b", "same bytes")
	write(t, dir, "c", "same byteS")
	write(t, dir, "d", "same bytes, and more")
	for _, tt := range []struct {
		a, b string
		want bool
	}{{"a", "b", true}, {"a", "a", true}, {"a", "c", false}, {"a", "d", false}} {
		if got, err := sameFile(filepath.Join(dir, tt.a), filepath.Join(dir, tt.b)); err != nil || got != tt.want {
			t.Errorf("%s and %s: wanted %v, got %v %v", tt.a, tt.b, tt.want, got, err)
		}
	}
	if _, err := sameFile(filepath.Join(dir, "a"), filepath.Join(dir, "missing")); err == nil {
		t.Errorf("wanted an error for a file that is not there")
	}
	if _, err := sameFile(filepath.Join(dir, "missing"), filepath.Join(dir, "a")); err == nil {
		t.Errorf("wanted an error for a file that is not there")
	}
}

func TestATreeThatDoesMoreWorkIsSlower(t *testing.T) {
	dir := repo(t)
	t.Setenv("GITHUB_ACTIONS", "true")
	write(t, dir, "work.go", workSource(8000, false)) // four times as much, not committed
	code, out, errs := compare(t, dir)
	if code != 1 || !strings.Contains(out, "slower   .        BenchmarkWork") || !strings.Contains(out, "1 slower") {
		t.Errorf("wanted status 1 and a slower benchmark, got %d\n%s\n%s", code, out, errs)
	}
	// In GitHub Actions a regression is also a warning, with no allocations to say where there are none.
	if !strings.Contains(out, "::warning title=benchmark regression::. BenchmarkWork is +") || strings.Contains(out, "(allocations") {
		t.Errorf("wanted a warning annotation without allocations, got\n%s", out)
	}
}

func TestATreeThatDoesLessWorkIsFaster(t *testing.T) {
	dir := repo(t)
	write(t, dir, "work.go", workSource(500, false))
	code, out, errs := compare(t, dir)
	if code != 0 || !strings.Contains(out, "faster   .        BenchmarkWork") {
		t.Errorf("wanted status 0 and a faster benchmark, got %d\n%s\n%s", code, out, errs)
	}
}

func TestATreeThatAllocatesMoreRegressesWhateverTheTime(t *testing.T) {
	dir := repo(t)
	t.Setenv("GITHUB_ACTIONS", "") // as on a developer's machine, not in the checks, where it is set
	write(t, dir, "work.go", workSource(2000, true))
	code, out, errs := compare(t, dir)
	if code != 1 || !strings.Contains(out, "0 -> 1") || !strings.Contains(out, "1 allocate more") {
		t.Errorf("wanted status 1 and an allocation, got %d\n%s\n%s", code, out, errs)
	}
	if strings.Contains(out, "::warning") {
		t.Errorf("wanted no annotations outside GitHub Actions, got\n%s", out)
	}
}

func TestABenchmarkOnOneSideIsListedAndNotCompared(t *testing.T) {
	dir := repo(t)
	write(t, dir, "work_test.go", workTest+extraTest)
	code, out, errs := compare(t, dir)
	if code != 0 || !strings.Contains(out, "only in the head, not compared (1):\n  . BenchmarkExtra") || !strings.Contains(out, "1 benchmarks compared") {
		t.Errorf("wanted status 0 and the new benchmark listed, got %d\n%s\n%s", code, out, errs)
	}
}

func TestAProblemIsStatus2(t *testing.T) {
	dir := repo(t)
	var stdout, stderr bytes.Buffer
	c := config{dir: dir, base: "no-such-branch", bench: ".", pkgs: "./...", rounds: 1, benchtime: "10ms", threshold: 0.03}
	if code := run(c, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), `no commit, tag or branch "no-such-branch"`) {
		t.Errorf("wanted status 2 and a word about the base, got %d\n%s", code, stderr.String())
	}

	write(t, dir, "work.go", "package benchmod\n\nthis is not Go\n")
	if code, _, errs := compare(t, dir); code != 2 || !strings.Contains(errs, "benchcmp: ") {
		t.Errorf("wanted status 2 for a tree that does not build, got %d\n%s", code, errs)
	}

	for _, rounds := range []int{0, -1} {
		c := config{dir: dir, base: "HEAD", bench: ".", pkgs: "./...", rounds: rounds, benchtime: "10ms", threshold: 0.03}
		if code := run(c, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "rounds must be at least 1") {
			t.Errorf("wanted status 2 for %d rounds, got %d\n%s", rounds, code, stderr.String())
		}
	}
	c = config{dir: dir, base: "HEAD", bench: ".", pkgs: "./...", rounds: 1, benchtime: "10ms", threshold: -0.01}
	if code := run(c, &stdout, &stderr); code != 2 {
		t.Errorf("wanted status 2 for a threshold below 0, got %d", code)
	}
}

func TestOutsideAGitRepositoryIsStatus2(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go and git")
	}
	dir := t.TempDir()
	write(t, dir, "go.mod", "module benchmod\n\ngo 1.26\n")
	var stdout, stderr bytes.Buffer
	c := config{dir: dir, base: "HEAD", bench: ".", pkgs: "./...", rounds: 1, benchtime: "10ms", threshold: 0.03}
	if code := run(c, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "benchcmp: ") {
		t.Errorf("wanted status 2, got %d\n%s", code, stderr.String())
	}
}

// Over any three rounds, and for any package, each of the three goes first once and each is run
// once a round, and the next package starts with the next one.
func TestOrderTurns(t *testing.T) {
	for pkg := range 4 {
		firsts := map[string]int{}
		for round := range 3 {
			o := order(round, pkg)
			seen := map[string]bool{}
			for _, role := range o {
				seen[role] = true
			}
			if len(seen) != 3 || !seen["base"] || !seen["control"] || !seen["head"] {
				t.Fatalf("round %d, package %d: wanted each once, got %v", round, pkg, o)
			}
			firsts[o[0]]++
		}
		if len(firsts) != 3 {
			t.Errorf("package %d: wanted each of the three to go first once in three rounds, got %v", pkg, firsts)
		}
	}
	if order(0, 0) == order(0, 1) {
		t.Errorf("wanted the next package to start with a different one")
	}
}

func TestParseFlags(t *testing.T) {
	c, err := parseFlags(nil, os.Stderr)
	want := config{dir: ".", base: "origin/main", bench: ".", pkgs: "./...", rounds: 5, benchtime: "200ms", threshold: 0.03}
	if err != nil || c != want {
		t.Errorf("wanted the defaults %+v, got %+v %v", want, c, err)
	}
	c, err = parseFlags([]string{"-base", "v0.8.0", "-bench", "Draw", "-pkgs", "./mesh", "-rounds", "9", "-benchtime", "1s", "-threshold", "2.5", "-all", "-identical"}, os.Stderr)
	want = config{dir: ".", base: "v0.8.0", bench: "Draw", pkgs: "./mesh", rounds: 9, benchtime: "1s", threshold: 0.025, all: true, identical: true}
	if err != nil || c != want {
		t.Errorf("wanted %+v, got %+v %v", want, c, err)
	}
	var w bytes.Buffer
	if _, err := parseFlags([]string{"-rounds", "many"}, &w); err == nil || !strings.Contains(w.String(), "rounds") {
		t.Errorf("wanted an error that says what is wrong, got %v %q", err, w.String())
	}
}

func TestAddKeepsEachRoleApart(t *testing.T) {
	var s series
	s.add("base", line{ns: 1, allocs: 10, hasAllocs: true})
	s.add("control", line{ns: 2, allocs: 20, hasAllocs: true})
	s.add("head", line{ns: 3, allocs: 30, hasAllocs: true})
	s.add("head", line{ns: 4})
	s.add("nobody", line{ns: 5, allocs: 50, hasAllocs: true})
	want := series{base: []float64{1}, control: []float64{2}, head: []float64{3, 4}, baseAllocs: []float64{10}, headAllocs: []float64{30}}
	if !slices.Equal(s.base, want.base) || !slices.Equal(s.control, want.control) || !slices.Equal(s.head, want.head) ||
		!slices.Equal(s.baseAllocs, want.baseAllocs) || !slices.Equal(s.headAllocs, want.headAllocs) {
		t.Errorf("wanted %+v, got %+v", want, s)
	}
}

func TestAnAnnotationSaysWhichAllocationsChanged(t *testing.T) {
	dir := repo(t)
	t.Setenv("GITHUB_ACTIONS", "true")
	write(t, dir, "work.go", workSource(2000, true))
	if _, out, _ := compare(t, dir); !strings.Contains(out, "::warning title=benchmark regression::. BenchmarkWork is ") || !strings.Contains(out, "(allocations 0 -> 1)") {
		t.Errorf("wanted an annotation with the allocations, got\n%s", out)
	}
}

// git runs git in dir for a test.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// With two packages and one changed, only that one is run and compared, and the other is said not to
// have been.
func TestOnlyAChangedPackageIsRun(t *testing.T) {
	dir := repo(t)
	if err := os.Mkdir(filepath.Join(dir, "other"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "other/other.go", "package other\n\nvar sink int\n\nfunc Spin() int {\n\ts := 0\n\tfor i := 0; i < 3000; i++ {\n\t\ts += i ^ (s >> 2)\n\t}\n\treturn s\n}\n")
	write(t, dir, "other/other_test.go", "package other\n\nimport \"testing\"\n\nfunc BenchmarkSpin(b *testing.B) {\n\tfor b.Loop() {\n\t\tsink = Spin()\n\t}\n}\n\nfunc BenchmarkSpinTwice(b *testing.B) {\n\tfor b.Loop() {\n\t\tsink = Spin() + Spin()\n\t}\n}\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "a second package")

	write(t, dir, "work.go", workSource(8000, false)) // only the first package changes
	code, out, errs := compare(t, dir)
	if code != 1 || !strings.Contains(out, "1 benchmarks compared: 1 slower") || !strings.Contains(out, "same code: other (2)") {
		t.Errorf("wanted the changed package compared and the other not run, got %d\n%s\n%s", code, out, errs)
	}
	if strings.Contains(out, "Spin") {
		t.Errorf("wanted nothing of the package that was not run in the table, got\n%s", out)
	}
}

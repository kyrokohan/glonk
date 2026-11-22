.DEFAULT_GOAL := help
.PHONY: help fmt fmtcheck test vet lint vuln fuzz bench benchcmp brand record site play run build check

help:
	@echo "Available targets:"
	@echo "  fmt      - Format Go code"
	@echo "  fmtcheck - Fail if any Go file is not formatted"
	@echo "  test     - Run tests with the race detector (RACE= to turn it off)"
	@echo "  vet      - Check for suspicious code with go vet"
	@echo "  lint     - Find more mistakes with staticcheck (downloads it the first time)"
	@echo "  vuln     - List known vulnerabilities in Go and our dependencies with govulncheck"
	@echo "  fuzz     - Fuzz every fuzz test for FUZZTIME each (default 10s)"
	@echo "  bench    - Run the benchmarks once, with allocations (BENCH= picks some)"
	@echo "  benchcmp - Compare the benchmarks with those of BASE (default origin/main), the packages that differ"
	@echo "  brand    - Write the logo files and the site's icons again from internal/brand"
	@echo "  record   - Play the examples' scripts and write the recordings, as asciicasts, into CASTS (default casts)"
	@echo "  site     - Write the manual and the changelog of glonk.dev again from README.md and CHANGELOG.md"
	@echo "  play     - Build the game on glonk.dev's cover for the browser, into site/media/play (Cloudflare runs it)"
	@echo "  run      - Run an example (default bounce2d; EXAMPLE=./examples/cube3d; ARGS= passes arguments)"
	@echo "  build    - Compile every package"
	@echo "  check    - Check formatting, run tests and vet"

fmt:
	@echo "Formatting..."
	go fmt ./...

# Lists the files and fails if any is not gofmt-formatted; CI runs this too.
fmtcheck:
	@echo "Checking format..."
	@test -z "$$(gofmt -l .)" || { echo "not formatted:"; gofmt -l .; exit 1; }

# The race detector needs cgo; run `make test RACE=` where there is no C compiler.
RACE ?= -race
test:
	@echo "Testing..."
	go test $(RACE) ./...

vet:
	@echo "Vetting..."
	go vet ./...

# Pinned, so that a new release cannot turn CI red by itself. Dependabot does not read
# this file: bump it by hand now and then. It is not part of check because it needs
# the network the first time.
STATICCHECK ?= honnef.co/go/tools/cmd/staticcheck@2026.2.2
lint:
	@echo "Linting..."
	go run $(STATICCHECK) ./...

# Pinned and bumped by hand, like STATICCHECK. A new report can appear against code that
# has not changed, so CI also runs this every week.
GOVULNCHECK ?= golang.org/x/vuln/cmd/govulncheck@v1.9.0
vuln:
	@echo "Checking for known vulnerabilities..."
	go run $(GOVULNCHECK) ./...

# Go fuzzes one target per run, so loop over every Fuzz test in every package.
# A failing input is saved under testdata/fuzz/ and then runs with plain `make test`.
FUZZTIME ?= 10s
fuzz:
	@for pkg in $$(go list ./...); do \
		for t in $$(go test -list '^Fuzz' $$pkg | grep '^Fuzz'); do \
			echo "Fuzzing $$pkg $$t..."; \
			go test $$pkg -run '^$$' -fuzz "^$$t$$" -fuzztime $(FUZZTIME) || exit 1; \
		done; \
	done

# Run once, with allocations, on the working tree; BENCH is a regular expression for the names.
BENCH ?= .
bench:
	@echo "Benchmarking..."
	go test -run '^$$' -bench '$(BENCH)' -benchmem ./...

# The benchmarks of the working tree against those of BASE (a commit, tag or branch), in
# interleaved rounds with the base run twice as a control for noise: see internal/benchcmp. It
# exits 1 for a change that is clearly slower or allocates more. It runs only the packages whose
# code differs from the base's, which can take minutes for a change to a package that others use;
# BENCH=Draw ROUNDS=9 narrows it and looks closer, and FLAGS=-identical runs the rest too.
BASE ?= origin/main
FLAGS ?=
ROUNDS ?= 5
BENCHTIME ?= 200ms
THRESHOLD ?= 3
benchcmp:
	@echo "Comparing the benchmarks with $(BASE)..."
	go run ./internal/benchcmp -base '$(BASE)' -bench '$(BENCH)' -rounds $(ROUNDS) -benchtime $(BENCHTIME) -threshold $(THRESHOLD) $(FLAGS)

# The logo is source: the pixels and the colors are in internal/brand, whose test also fails,
# under make check, if a file is not what they draw. See BRAND.md.
brand:
	@echo "Drawing the logo files..."
	go test ./internal/brand -run TestTheFilesAreWhatTheSourceDraws -update

# The recordings in site/media are drawn from what these four examples play: each has a script of
# key presses in its record_test.go, and internal/record plays it with no terminal. A tape takes
# as long as it lasts, which is why make check skips them.
CASTS ?= casts
record:
	@echo "Recording the examples into $(CASTS)..."
	go test ./examples/cube3d ./examples/viewer ./examples/sprites ./examples/world -run '^TestRecord$$' -record '$(abspath $(CASTS))'

# The manual and the changelog on glonk.dev are README.md and CHANGELOG.md as pages. The test in
# internal/sitepages makes them, and fails under make check when one of the two files has changed
# and its page has not.
site:
	@echo "Writing the manual and the changelog of glonk.dev..."
	go test ./internal/sitepages -run TestThePagesAreWhatTheRepositorySays -update

# The game on glonk.dev's cover is cube3d built for the browser (see glonk.dev/web), gzipped,
# since Cloudflare does not compress application/wasm and web/glonk.js unpacks it, beside the
# wasm_exec.js of the Go that built it, which must match, and glonk.js itself, which site/play.js
# loads. Cloudflare runs this before every deploy, as the build command of the Worker; the files
# are not committed.
PLAY := site/media/play
play:
	@echo "Building the game of glonk.dev..."
	mkdir -p $(PLAY)
	GOOS=js GOARCH=wasm go build -trimpath -ldflags=-s -o $(PLAY)/cube3d.wasm ./examples/cube3d
	gzip -9nf $(PLAY)/cube3d.wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" $(PLAY)/
	cp web/glonk.js $(PLAY)/

EXAMPLE ?= ./examples/bounce2d
ARGS ?=
run:
	@echo "Running..."
	go run $(EXAMPLE) $(ARGS)

build:
	@echo "Building..."
	go build ./...

check: fmtcheck test vet
	@echo "All checks passed!"

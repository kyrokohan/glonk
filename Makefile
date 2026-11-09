.DEFAULT_GOAL := help
.PHONY: help fmt fmtcheck test vet lint vuln fuzz run build check

help:
	@echo "Available targets:"
	@echo "  fmt      - Format Go code"
	@echo "  fmtcheck - Fail if any Go file is not formatted"
	@echo "  test     - Run tests with the race detector (RACE= to turn it off)"
	@echo "  vet      - Check for suspicious code with go vet"
	@echo "  lint     - Find more mistakes with staticcheck (downloads it the first time)"
	@echo "  vuln     - List known vulnerabilities in Go and our dependencies with govulncheck"
	@echo "  fuzz     - Fuzz every fuzz test for FUZZTIME each (default 10s)"
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

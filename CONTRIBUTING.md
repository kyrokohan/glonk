# Contributing

Issues and pull requests are welcome. glonk is a one-person project, so replies can take a while. Everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md).

## Before you start

- For a bug, a typo or a small improvement, go ahead and open a pull request.
- For anything larger, open an issue first. The API is pre-1.0 and may still change, so a big change is best agreed before you write it.
- Planned work and ideas are [issues](https://github.com/usrgames/glonk/issues). `help wanted` and `good first issue` mark places to start; comment on an issue before starting a large one.
- To report a security problem, do not use an issue. See [SECURITY.md](SECURITY.md).

## Setting up

You need Go 1.26 or newer. The race detector needs cgo, so a C compiler must be installed.

```
make check    # formatting, go vet, and all tests with the race detector
make lint     # staticcheck, which finds mistakes that go vet does not
make vuln     # govulncheck, which lists known vulnerabilities in Go and our dependencies
make fuzz     # fuzz every fuzz test for FUZZTIME each (default 10s)
make bench    # run the benchmarks once, with allocations
make benchcmp # compare the benchmarks with those of a base, BASE=origin/main unless said
make run      # try an example; EXAMPLE=./examples/cube3d picks another
```

`make check` is what CI runs, on Linux and macOS with Go 1.26 and 1.27. CI also vets for Windows, FreeBSD and 32-bit ARM, builds for Windows, runs the tests as a 32-bit program, and runs `make lint`. `make vuln` runs in CI too, on every pull request and every week, but does not block a merge. `lint` and `vuln` download their tool the first time, at the version pinned in the `Makefile`; Dependabot does not read that file, so bump the versions by hand now and then.

## Sending a change

Every change, the maintainer's included, goes in through a pull request; nobody pushes to `main`.

1. Start from an up-to-date `main` on a branch named `type/short-subject`, with the type from the commit messages below, as in `git switch -c fix/obj-crash`.
2. Commit, run `make check`, and push the branch (to your fork, or to the repository if you can).
3. Open a pull request titled `type: subject`. The template asks what and why, the issue it closes, and four ticks.
4. The checks must pass: the tests, the cross-system builds and `staticcheck`.
5. The owner reviews and merges it. A change of several commits is merged by rebase, so each commit stays with its message; a small fix may be squashed into one commit with the pull request title. The branch is deleted afterwards.

## Making a change

- Keep a change to one thing. Commit messages are `type: subject` with the type one of `feat`, `fix`, `refactor`, `docs`, `test` or `chore`, and a lowercase subject in the imperative, under about 70 characters.
- Behavior changes come with tests. For a bug, write the test first and see it fail.
- Exported names need a doc comment, and a doc comment must not name an unexported identifier, because readers of the documentation cannot look it up. Examples in `example_test.go` files need an `// Output:` comment so that `go test` runs them. `engine/docs_test.go` checks all of this.
- The first sentence of a package comment is shown as written in pkg.go.dev lists, so it must not contain `[Link]` brackets.
- What `Run` writes to the terminal is tested by replaying the bytes onto a small virtual terminal (`engine/vterm_test.go`) and requiring that it shows exactly the frame; do the same for a change to how frames are drawn.
- Text and cell contents reach the terminal only through the frame buffer, which checks every cell. Do not send them by another route. Control sequences are built in one place, `engine/escape.go`.
- Comments say why, not what.
- The website at glonk.dev is the plain HTML in `site/`, deployed from `main` by Cloudflare as Workers static assets, configured in `wrangler.jsonc`, whose 404 page is what answers a request for any other path with the `go-import` tags. Its install line must match the README's and the module in `go.mod`, and both pages must keep their `go-import` tags at the top of the head, because the import path depends on them. `internal/sitecheck` checks all of that.
- The logo files in `brand/` and the site's icons are drawn by the test in `internal/brand`, from the pixels and the terminal color numbers written there. Change them there and run `make brand`; do not edit the files. `BRAND.md` says how the mark, the colors and the type are used.
- The recordings in `site/media/` show what four of the examples play from the scripts in their `record_test.go`. `make record` plays them again and writes the frames as asciicasts; the pictures are those frames as a terminal shows them. Re-record an example when what it draws changes. The folder has a `go.mod` of its own so that Go leaves it out of the module: do not put anything there that a package needs.
- The manual and the changelog on glonk.dev (`site/manual/` and `site/changelog/`) are `README.md` and `CHANGELOG.md` as pages, made by the test in `internal/sitepages`. After a change to either file run `make site`, or `make check` fails; do not edit the pages. The test knows the Markdown the two files use (headings, paragraphs, flat lists, tables, code, bold, links and pictures) and refuses anything else until it is taught it.
- The showcase on glonk.dev (`site/showcase/`) lists projects made with glonk from `internal/sitepages/showcase.json`. A project asks to be listed with the "Show your project" issue form, which states the rule. To list one, the maintainer adds its entry to that file and its picture, a PNG and if there is a recording a GIF of the same name, to `site/media/showcase/`, and runs `make site`, which checks the entry.
- The game on glonk.dev's cover is `examples/cube3d` built for the browser: the `web` package runs it in the page, `web/glonk.js` starts it and draws it with xterm.js, which is in `site/media/xterm/`, and `site/play.js` is the cover's button around it. `make play` builds it into `site/media/play/`, which is not committed: Cloudflare runs `make play` as the Worker's build command before every deploy. The landing page loads `play.js`, and no page loads any other script; `internal/sitecheck` holds that. On glonk.dev Cloudflare also adds its Web Analytics beacon to every page, at the edge: it is a setting of the Cloudflare account, kept on purpose, and in no file of the repository.

## Measuring speed

glonk keeps its drawing, loading and searching fast, and a change that can slow a hot path should show that it did not. `make benchcmp` runs every benchmark on the working tree (uncommitted changes included) and on `BASE`, a commit, tag or branch, `origin/main` unless said, and prints what changed:

```
make benchcmp                       # against origin/main
make benchcmp BASE=v0.8.0           # against a release
make benchcmp BENCH=Draw ROUNDS=9   # fewer benchmarks, a closer look
```

A package whose test binary is the same file as the base's has the same code, so it is not run and is listed as not run: that is exact, since the binaries are built without file paths (below), and it makes a change to one package quick, since only the packages that can see the change differ. The benchmarks of every package together take about two minutes a round, ten at the default 5 rounds; `BENCH` narrows them, and `make benchcmp BASE=HEAD FLAGS=-identical` runs the packages that are the same too, which is how to measure the noise of the machine. A change to nothing but a file that a benchmark reads is not seen, since the binary is the same. Each round runs the base, the base again and the head, one after the other in an order that turns from round to round, so a computer that warms up or gets busy affects all three alike, and the test binaries are built without file paths (`-trimpath`), so that identical code is an identical binary: two builds that differed only in the length of a path differed by 4 percent on whole groups of benchmarks. The base run twice is a control. The noise is how far the control's best run is from the base's, which is about 0.3 percent for most benchmarks and over 1 percent for the few that are short and busy. A benchmark is reported as slower or faster only if its change is beyond `THRESHOLD` percent (3) both by the best run and by the typical run, the median, since either alone can be fooled by one round in which the machine was idle or busy, and the best run's is also beyond twice the noise. A benchmark whose noise alone is beyond the threshold is marked noisy, since a change that size could hide in it. Allocations do not vary where there are few of them, so any rise in `allocs/op` is a regression, whatever the time; a rise of a thousandth or less in a benchmark that allocates hundreds of thousands of times is the runtime's own bookkeeping and is not. It exits 1 for a regression, which is what to look at before sending the change; `make bench` runs the benchmarks once without comparing.

Run on identical code with `-identical`, it flagged nothing in 151 benchmarks, and made to see a change of 5 percent it found it in 147 of them; it cannot tell 1 or 2 percent from noise, which is why the threshold is 3. Use more rounds and a narrower `BENCH` for a closer look, and close other programs. Nothing is published or kept between runs. A workflow, `bench`, runs it in GitHub Actions when started by hand with a base; it is not part of the checks on a pull request, since shared runners are noisy, it takes minutes, and nothing should wait for it.

## Making a release

The maintainer makes releases from `main` once CI there is green. While glonk is below 1.0, a release that adds or changes API is a new minor version (0.6.0) and one with only fixes is a patch (0.6.1).

1. On a branch `docs/release-vX.Y.Z`, one commit, `docs: prepare the vX.Y.Z release`:
   - In `CHANGELOG.md`, `## [Unreleased]` becomes `## [X.Y.Z] - YYYY-MM-DD`, opened by one sentence on what the release adds and whether anything was removed, renamed or changed. An empty `## [Unreleased]` goes above it.
   - The links at the bottom: `[Unreleased]` compares `vX.Y.Z...HEAD`, and a new `[X.Y.Z]` compares the previous tag with `vX.Y.Z`.
   - `SECURITY.md` supports `vX.Y.x`.
   - `make site`, which writes the changelog page of glonk.dev again.

   The date is the day the tag is pushed. If the tag goes out on a later day, change the date first.
2. Merge the pull request once its checks pass (by rebase if it has other commits, so the prep commit stays last and keeps its message), then wait for CI on `main`.
3. Tag the merged commit and push the tag. The `release tags` ruleset does not let a tag be moved or deleted, so check the commit first:
   ```
   git switch main && git pull
   git log -1 --format='%h %s'     # must be: docs: prepare the vX.Y.Z release
   git tag -a vX.Y.Z -m "glonk vX.Y.Z"
   git push origin vX.Y.Z
   ```
4. Publish the GitHub release, `gh release create vX.Y.Z --verify-tag --title "glonk vX.Y.Z" --notes-file notes.md`, with notes in this order:
   - one sentence on what it adds, and whether anything was removed, renamed or changed;
   - Highlights, one bullet for each feature, naming the API;
   - Check before upgrading: what can break or behave differently, or "Nothing to change: this release only adds.";
   - the pre-1.0 line: the API may still change, what is untested, and the Go version from `go.mod`;
   - "Full list of changes: CHANGELOG.md".
5. Check it:
   - `git rev-parse vX.Y.Z^{commit}` is the commit from step 3.
   - `gh release view vX.Y.Z` shows the notes, and the release is Latest.
   - `curl -s https://proxy.golang.org/glonk.dev/@v/vX.Y.Z.info` returns the version. This needs the `go-import` tags on glonk.dev.
   - https://pkg.go.dev/glonk.dev@vX.Y.Z shows it; if it doesn't, the page offers to fetch it.
6. Close the release's milestone.

## License

By contributing you agree that your work is released under the [MIT License](LICENSE) like the rest of the project.

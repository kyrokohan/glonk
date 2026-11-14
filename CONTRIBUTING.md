# Contributing

Issues and pull requests are welcome. glonk is a one-person project, so replies can take a while. Everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md).

## Before you start

- For a bug, a typo or a small improvement, go ahead and open a pull request.
- For anything larger, open an issue first. The API is pre-1.0 and may still change, so a big change is best agreed before you write it.
- Planned work and ideas are [issues](https://github.com/kyrokohan/glonk/issues). `help wanted` and `good first issue` mark places to start; comment on an issue before starting a large one.
- To report a security problem, do not use an issue. See [SECURITY.md](SECURITY.md).

## Setting up

You need Go 1.26 or newer. The race detector needs cgo, so a C compiler must be installed.

```
make check    # formatting, go vet, and all tests with the race detector
make lint     # staticcheck, which finds mistakes that go vet does not
make vuln     # govulncheck, which lists known vulnerabilities in Go and our dependencies
make fuzz     # fuzz every fuzz test for FUZZTIME each (default 10s)
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

## Making a release

The maintainer makes releases from `main` once CI there is green. While glonk is below 1.0, a release that adds or changes API is a new minor version (0.6.0) and one with only fixes is a patch (0.6.1).

1. On a branch `docs/release-vX.Y.Z`, one commit, `docs: prepare the vX.Y.Z release`:
   - In `CHANGELOG.md`, `## [Unreleased]` becomes `## [X.Y.Z] - YYYY-MM-DD`, opened by one sentence on what the release adds and whether anything was removed, renamed or changed. An empty `## [Unreleased]` goes above it.
   - The links at the bottom: `[Unreleased]` compares `vX.Y.Z...HEAD`, and a new `[X.Y.Z]` compares the previous tag with `vX.Y.Z`.
   - `SECURITY.md` supports `vX.Y.x`.

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

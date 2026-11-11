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

## License

By contributing you agree that your work is released under the [MIT License](LICENSE) like the rest of the project.

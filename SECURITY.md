# Security policy

## Supported versions

glonk is pre-1.0. Only the latest release (v0.4.x) and the latest commit on `main` are supported. A fix lands on `main` first and ships in a new patch release; older releases are not patched.

## Reporting a vulnerability

Please report it privately, not in a public issue or pull request. On GitHub, open the repository's **Security** tab and choose **Report a vulnerability**. That opens a private thread with the maintainer.

Include what you can of:

- what the problem is and what an attacker gains from it;
- the smallest input or program that shows it (an OBJ file, a byte sequence on stdin, a few lines of Go);
- the glonk commit or version, your Go version and operating system, and the terminal you used if it matters.

glonk is maintained by one person, so replies are best effort. A report that comes with a reproduction is the easiest to act on. You will be told when a fix is on `main`, and credited in the changelog unless you would rather not be.

## What counts

glonk reads data it does not control and writes to a person's terminal, so these matter most:

- **OBJ loading** (`mesh.LoadOBJ`, `mesh.LoadOBJLimit` and `mesh.LoadOBJFS`): a file that crashes the loader, makes it run for far longer than its size explains, or uses far more memory than documented. `LoadOBJ` has no size limit on purpose; reading an endless source with it is expected, and `LoadOBJLimit` is the safe call for untrusted input.
- **Material files** (`mesh.LoadOBJFS`): an OBJ that makes it open or read a file outside the OBJ's own folder, or an MTL file that crashes the loader or costs far more time or memory than its size and the documented limits explain.
- **Terminal output**: text, cell contents or a file name that makes glonk send the terminal something other than the characters it was given, such as an escape sequence that runs.
- **Keyboard input handling**: a byte sequence from the terminal that crashes the program, hangs it, or leaves keys stuck.
- **Leaving the terminal in a bad state** after a crash, signal or panic that glonk claims to handle (see "Compatibility and limits" in the README for the cases it does not).

Bugs that are only wrong drawing, or are listed as known limits in the README, are ordinary issues.

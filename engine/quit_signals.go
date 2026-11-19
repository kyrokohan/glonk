//go:build !js

package engine

import (
	"os"
	"syscall"
)

// quitSignals are the signals that end Run cleanly: Ctrl-C sent from outside the
// terminal, a plain kill, and the terminal window closing.
var quitSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

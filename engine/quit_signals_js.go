package engine

import (
	"os"
	"syscall"
)

// quitSignals has no SIGHUP under js, which does not define one. A browser has no
// terminal, so every Run there is headless and never listens for these anyway.
var quitSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

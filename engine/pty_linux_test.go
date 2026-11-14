//go:build linux

package engine

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// openPty returns both ends of a new pseudo-terminal, closed with the test. Nobody gives it
// a window size, so the slave reports 0 by 0.
func openPty(t *testing.T) (master, slave *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pseudo-terminals here: %v", err)
	}
	t.Cleanup(func() { master.Close() })

	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil { // unlock the slave
		t.Fatalf("unlocking the pseudo-terminal: %v", err)
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("finding the pseudo-terminal's number: %v", err)
	}
	slave, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("opening the pseudo-terminal's slave: %v", err)
	}
	// Closed first, so that the read of the master ends with it.
	t.Cleanup(func() { slave.Close() })
	return master, slave
}

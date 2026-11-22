//go:build darwin

package engine

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// getTermios is the request that reads a terminal's settings.
const getTermios = unix.TIOCGETA

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
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
		t.Fatalf("granting the pseudo-terminal's slave: %v", err)
	}
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
		t.Fatalf("unlocking the pseudo-terminal's slave: %v", err)
	}
	// The slave is /dev/ttysNNN, with NNN the minor number of the master's device, which is
	// what ptsname answers. x/sys has no wrapper for the ioctl that gives the name, and a
	// direct system call is deprecated on macOS.
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		t.Fatalf("finding the pseudo-terminal's number: %v", err)
	}
	slave, err = os.OpenFile(fmt.Sprintf("/dev/ttys%03d", unix.Minor(uint64(st.Rdev))), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("opening the pseudo-terminal's slave: %v", err)
	}
	// Closed first, so that the read of the master ends with it.
	t.Cleanup(func() { slave.Close() })
	return master, slave
}

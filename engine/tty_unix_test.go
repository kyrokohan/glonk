//go:build unix

package engine

import (
	"bytes"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// newPipe returns a pipe whose ends close with the test.
func newPipe(t *testing.T) (r, w *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	return r, w
}

// receive waits for the next chunk, failing the test instead of hanging.
func receive(t *testing.T, ch <-chan []byte) ([]byte, bool) {
	t.Helper()
	select {
	case b, ok := <-ch:
		return b, ok
	case <-time.After(3 * time.Second):
		t.Fatalf("wanted a chunk or the end of input, got neither in 3 s")
		return nil, false
	}
}

// returnsWithin runs f and fails if it takes longer than d.
func returnsWithin(t *testing.T, what string, d time.Duration, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { f(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("wanted %s to return within %v", what, d)
	}
}

func TestReadInputLeavesLaterInputAlone(t *testing.T) {
	r, w := newPipe(t)
	input, stop := readInput(r)

	w.WriteString("ab")
	if b, _ := receive(t, input); string(b) != "ab" {
		t.Fatalf("wanted %q, got %q", "ab", b)
	}
	returnsWithin(t, "stop", 3*time.Second, stop)

	// What is typed after Run is over belongs to the program, not to the reader
	// that used to run inside Run. Wait long enough for a reader that was left
	// behind to take it, then check it is still there.
	w.WriteString("later\n")
	time.Sleep(3 * pollInterval)
	if ready, _ := waitReadable(int(r.Fd())); !ready {
		t.Fatalf("wanted %q still in the pipe after stop, but something read it", "later\n")
	}
	buf := make([]byte, 16)
	n, err := r.Read(buf)
	if err != nil || string(buf[:n]) != "later\n" {
		t.Errorf("wanted %q left in the pipe, got %q (err %v)", "later\n", buf[:n], err)
	}
}

func TestSelectReady(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		err     error
		ready   bool
		wantErr bool
	}{
		{"input waiting", 1, nil, true, false},
		{"timed out", 0, nil, false, false},
		{"interrupted by a signal", -1, unix.EINTR, false, false},
		{"bad descriptor", -1, unix.EBADF, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready, err := selectReady(tt.n, tt.err)
			if ready != tt.ready || (err != nil) != tt.wantErr {
				t.Errorf("wanted ready=%v err=%v, got ready=%v err=%v", tt.ready, tt.wantErr, ready, err)
			}
		})
	}
}

func TestRetryable(t *testing.T) {
	for err, want := range map[error]bool{
		nil:         false,
		unix.EINTR:  true,
		unix.EAGAIN: true,
		unix.EBADF:  false,
		unix.EIO:    false,
	} {
		if got := retryable(err); got != want {
			t.Errorf("wanted retryable(%v) = %v, got %v", err, want, got)
		}
	}
}

func TestReadInputStopIsPromptWhenIdle(t *testing.T) {
	r, _ := newPipe(t)
	_, stop := readInput(r)

	start := time.Now()
	returnsWithin(t, "stop on an idle input", 3*time.Second, stop)
	returnsWithin(t, "a second stop", time.Second, stop)
	// Well past a poll, which a busy runner can make late (#155); a stop that waits for input fails returnsWithin above.
	if d, bound := time.Since(start), 50*pollInterval; d > bound {
		t.Errorf("wanted stop within %v (a poll is %v), took %v", bound, pollInterval, d)
	}
}

func TestReadInputStopDoesNotNeedAReceiver(t *testing.T) {
	r, w := newPipe(t)
	_, stop := readInput(r)

	w.WriteString("x") // read, then held because nobody receives
	time.Sleep(3 * pollInterval)
	returnsWithin(t, "stop with a chunk waiting to be received", 3*time.Second, stop)
}

func TestReadInputEndsWithTheInput(t *testing.T) {
	r, w := newPipe(t)
	input, stop := readInput(r)
	defer stop()

	w.WriteString("ab")
	w.Close()

	var got []byte
	for {
		b, ok := receive(t, input)
		if !ok {
			break
		}
		got = append(got, b...)
	}
	if string(got) != "ab" {
		t.Errorf("wanted %q before the end, got %q", "ab", got)
	}
}

func TestReadInputSurvivesSignals(t *testing.T) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR1)
	defer signal.Stop(sigs)

	r, w := newPipe(t)
	input, stop := readInput(r)
	defer stop()

	// Signals landing while the reader waits must not end the stream or eat input.
	quit := make(chan struct{})
	defer close(quit)
	go func() {
		for {
			select {
			case <-quit:
				return
			case <-time.After(time.Millisecond):
				syscall.Kill(os.Getpid(), syscall.SIGUSR1)
			}
		}
	}()
	go func() {
		for range sigs {
		}
	}()

	time.Sleep(10 * pollInterval)
	w.WriteString("still here")
	if b, ok := receive(t, input); !ok || string(b) != "still here" {
		t.Errorf("wanted %q after a storm of signals, got %q (open: %v)", "still here", b, ok)
	}
}

func TestReadInputReadsBigChunksAtOnce(t *testing.T) {
	r, w := newPipe(t)
	input, stop := readInput(r)
	defer stop()

	w.WriteString(strings.Repeat("w", 1000))
	if b, _ := receive(t, input); len(b) != 1000 {
		t.Errorf("wanted one chunk of 1000 bytes, got %d", len(b))
	}

	big := strings.Repeat("w", 3*readBuffer+17)
	go func() { w.WriteString(big); w.Close() }()
	total := 0
	for {
		b, ok := receive(t, input)
		if !ok {
			break
		}
		if len(b) > readBuffer {
			t.Errorf("wanted chunks of at most %d bytes, got %d", readBuffer, len(b))
		}
		total += len(b)
	}
	if total != len(big) {
		t.Errorf("wanted %d bytes in all, got %d", len(big), total)
	}
}

func TestReadInputCopiesEachChunk(t *testing.T) {
	r, w := newPipe(t)
	input, stop := readInput(r)
	defer stop()

	w.WriteString("one")
	first, _ := receive(t, input)
	w.WriteString("two")
	receive(t, input)
	if !bytes.Equal(first, []byte("one")) {
		t.Errorf("wanted the first chunk untouched by later reads, got %q", first)
	}
}

func TestReadInputFallsBackAboveTheSelectLimit(t *testing.T) {
	r, w := newPipe(t)
	high, err := unix.FcntlInt(r.Fd(), unix.F_DUPFD, selectLimit+100)
	if err != nil {
		t.Skipf("cannot make a descriptor above %d: %v", selectLimit, err)
	}
	f := os.NewFile(uintptr(high), "high")
	defer f.Close()

	input, stop := readInput(f) // select would panic on this descriptor
	defer stop()

	w.WriteString("hi")
	if b, _ := receive(t, input); string(b) != "hi" {
		t.Errorf("wanted %q, got %q", "hi", b)
	}
}

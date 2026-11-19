package engine

import (
	"context"
	"os"
	"os/signal"
	"sync"
)

// notifyQuit calls cancel on the first quit signal. It then lets go of the signals,
// so a second one takes the default action and ends a program that is stuck. A
// SIGHUP or SIGINT the process was started ignoring (nohup does this to SIGHUP)
// stays ignored. Go reports no other signal as ignored, so SIGTERM is always
// watched. stop undoes the registration.
func notifyQuit(cancel context.CancelFunc) (stop func()) {
	var watched []os.Signal
	for _, s := range quitSignals {
		if !signal.Ignored(s) {
			watched = append(watched, s)
		}
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, watched...)

	done := make(chan struct{})
	go func() {
		select {
		case <-ch:
			signal.Stop(ch)
			cancel()
		case <-done:
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			signal.Stop(ch)
			close(done)
		})
	}
}

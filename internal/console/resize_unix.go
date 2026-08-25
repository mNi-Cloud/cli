//go:build !windows

package console

import (
	"os"
	"os/signal"
	"syscall"
)

// Resized reports every change the user makes to the size of the terminal. A
// terminal tells the process it belongs to about a resize with SIGWINCH, and a
// command running on the far end only learns about it if this side passes it
// on.
func (f *File) Resized() (<-chan struct{}, func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)

	changed := make(chan struct{}, 1)
	done := make(chan struct{})

	go func() {
		defer close(changed)
		for {
			select {
			case <-signals:
				// A size that is already waiting to be sent is the size the
				// terminal has now, so a resize during a resize is dropped
				// rather than queued.
				select {
				case changed <- struct{}{}:
				default:
				}
			case <-done:
				return
			}
		}
	}()

	return changed, func() {
		signal.Stop(signals)
		close(done)
	}
}

//go:build windows

package console

// Resized reports every change the user makes to the size of the terminal.
// Windows sends no signal for a resize, so a command started from here keeps
// the size the terminal had when it was started.
func (f *File) Resized() (<-chan struct{}, func()) {
	return nil, func() {}
}

// Package console offers a console that a server streams over a connection to
// something local: the terminal of the user for a serial console, and a local
// port for a graphical one.
package console

import (
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/term"
)

const (
	// escapeKey leaves a console. It is Ctrl-], the key telnet and virtctl leave
	// a console with.
	escapeKey = 0x1d
	// EscapeName is escapeKey as it is written for the user.
	EscapeName = "Ctrl-]"
)

// errEscaped says that the user left the console, which is how a console is
// meant to end rather than a failure.
var errEscaped = errors.New("the escape key was typed")

// Terminal is the terminal a console is attached to. Raw hands back what puts
// the terminal back the way the user had it.
type Terminal interface {
	io.Reader
	Raw() (restore func() error, err error)
}

// File is the terminal of this process. It keeps the handle it reads from and
// the handle it is sized from apart, because a terminal reads from one and is
// sized from the other: on Windows the input handle answers no size query, so
// the output handle is used for that.
type File struct {
	In  *os.File
	Out *os.File
}

// NewFile builds the terminal of this process from its input and output
// handles. The output handle may be nil, in which case the input handle is
// used for size queries too, which is what a Unix terminal expects.
func NewFile(in, out *os.File) *File {
	return &File{In: in, Out: out}
}

// Read is what the user types.
func (f *File) Read(p []byte) (int, error) {
	return f.In.Read(p)
}

// Raw puts the terminal into raw mode and hands back what restores it.
func (f *File) Raw() (func() error, error) {
	state, err := term.MakeRaw(int(f.In.Fd()))
	if err != nil {
		return nil, err
	}
	return func() error { return term.Restore(int(f.In.Fd()), state) }, nil
}

// outputFD is the handle a size query is put to. A terminal is sized from the
// side it writes to, which on Windows is the only handle that answers.
func (f *File) outputFD() int {
	if f.Out != nil {
		return int(f.Out.Fd())
	}
	return int(f.In.Fd())
}

// Attach joins the terminal to a console until the user types the escape key or
// the console ends. The terminal is put into raw mode first, so that every key
// reaches the machine instead of being read by the shell of the user.
func Attach(terminal Terminal, out io.Writer, remote io.ReadWriter) error {
	restore, err := terminal.Raw()
	if err != nil {
		return err
	}

	joinErr := join(endpoint{Reader: &escapeReader{reader: terminal, escape: escapeKey}, Writer: out}, remote)
	restoreErr := restore()

	if errors.Is(joinErr, errEscaped) {
		joinErr = nil
	}
	if joinErr != nil {
		return joinErr
	}
	return restoreErr
}

// Copy writes what a stream carries to a writer until the stream ends or the
// user stops waiting. A read of a stream does not end on its own, so a user who
// stops waiting has the connection taken away from it.
func Copy(ctx context.Context, out io.Writer, remote io.ReadCloser) error {
	defer stopOn(ctx, remote)()
	return stopped(ctx, copyStream(out, remote))
}

// join copies bytes between a local endpoint and a console, and returns as soon
// as either side ends. It leaves both sides open: only the caller knows how
// they are given up.
func join(local, remote io.ReadWriter) error {
	ended := make(chan error, 2)

	go func() { ended <- copyStream(remote, local) }()
	go func() { ended <- copyStream(local, remote) }()

	return <-ended
}

// endpoint pairs a reader and a writer into one side of a console.
type endpoint struct {
	io.Reader
	io.Writer
}

// copyStream copies one direction. The end of the side that is read is the end
// of the console, not a failure of it.
func copyStream(to io.Writer, from io.Reader) error {
	_, err := io.Copy(to, from)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

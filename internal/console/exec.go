package console

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"golang.org/x/term"
)

// The channels of an exec. One connection carries the input, the two outputs
// and the exit code of a command at once, so the first byte of every frame
// names which of them the rest of the frame belongs to.
const (
	ChannelStdin      byte = 0
	ChannelStdout     byte = 1
	ChannelStderr     byte = 2
	ChannelStatus     byte = 3
	ChannelResize     byte = 4
	ChannelStdinClose byte = 5
)

// inputBufferSize is how much of what the user types is sent in one frame.
const inputBufferSize = 4096

// ErrNoExitStatus says that the server never told how the command it ran had
// ended, which leaves nothing to end this process with.
var ErrNoExitStatus = errors.New("the server did not say how the command ended")

// TerminalSize is the size of a terminal, as the resize channel carries it.
type TerminalSize struct {
	Width  uint16 `json:"width"`
	Height uint16 `json:"height"`
}

// ExitStatus is how a command ended, as the status channel carries it. The
// message is empty for a command that ran and ended on its own.
type ExitStatus struct {
	ExitCode int    `json:"exitCode"`
	Message  string `json:"message"`
}

// MessageStream is the connection a command runs over. Its frames are kept
// apart, because the first byte of every frame names the channel it belongs to.
type MessageStream interface {
	ReadMessage() ([]byte, error)
	WriteMessage(payload []byte) error
	io.Closer
}

// ExecTerminal is the terminal a command with a TTY runs on. Raw hands back
// what puts the terminal back the way the user had it, Size is what the remote
// terminal is sized after, and Resized reports every change the user makes to
// it.
type ExecTerminal interface {
	Raw() (restore func() error, err error)
	Size() (TerminalSize, error)
	Resized() (changed <-chan struct{}, stop func())
}

// Size is how big the terminal of this process is right now.
func (f File) Size() (TerminalSize, error) {
	width, height, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return TerminalSize{}, err
	}
	return TerminalSize{Width: uint16(width), Height: uint16(height)}, nil
}

// ExecOptions is what running one command over a stream needs.
type ExecOptions struct {
	// Stdin is what the user types. A nil reader runs the command without any
	// input, and then nothing is ever sent on the input channel.
	Stdin io.Reader
	// Stdout and Stderr are where the command writes. A command with a TTY
	// writes everything to Stdout, because a terminal merges the two.
	Stdout io.Writer
	Stderr io.Writer

	// Terminal is the terminal the command is given, or nil when it is given
	// none.
	Terminal ExecTerminal
}

func (o ExecOptions) check() error {
	if o.Stdout == nil || o.Stderr == nil {
		return errors.New("a command needs somewhere to write to")
	}
	if o.Terminal != nil && o.Stdin == nil {
		return errors.New("a command with a terminal needs the input of the user")
	}
	return nil
}

// Exec runs one command over a stream and reports how it ended. The command
// runs on the far end, so this side only carries its input, its output and its
// exit code.
func Exec(ctx context.Context, remote MessageStream, options ExecOptions) (ExitStatus, error) {
	if err := options.check(); err != nil {
		return ExitStatus{}, err
	}

	// A read of a stream does not end on its own, so a user who stops waiting
	// has the connection taken away from it.
	defer stopOn(ctx, remote)()

	out := &sender{remote: remote}
	release := func() error { return nil }
	if options.Terminal != nil {
		giveUp, err := takeOverTerminal(out, options.Terminal)
		if err != nil {
			return ExitStatus{}, err
		}
		release = giveUp
	}

	if options.Stdin != nil {
		go func() { _ = forwardInput(out, options.Stdin) }()
	}

	status, ended, err := receive(remote, options.Stdout, options.Stderr)
	releaseErr := release()

	switch {
	case ctx.Err() != nil:
		// The user stopped waiting for the command, so nothing it might still
		// have said is missing.
		return ExitStatus{}, releaseErr
	case err != nil:
		return ExitStatus{}, err
	case !ended:
		return ExitStatus{}, ErrNoExitStatus
	default:
		return status, releaseErr
	}
}

// takeOverTerminal puts the terminal into raw mode, so that every key reaches
// the command instead of being read by the shell of the user, and keeps the far
// end told about the size of it. What it hands back gives the terminal back to
// the user.
func takeOverTerminal(out *sender, terminal ExecTerminal) (func() error, error) {
	restore, err := terminal.Raw()
	if err != nil {
		return nil, err
	}

	stopWatching, err := watchSize(out, terminal)
	if err != nil {
		_ = restore()
		return nil, err
	}

	return func() error {
		stopWatching()
		return restore()
	}, nil
}

// sender puts one frame at a time on the stream. What the user types and the
// size of the terminal are written by two goroutines, and a connection takes
// one writer at a time.
type sender struct {
	mutex  sync.Mutex
	remote MessageStream
}

func (s *sender) send(channel byte, payload []byte) error {
	frame := make([]byte, 0, len(payload)+1)
	frame = append(frame, channel)
	frame = append(frame, payload...)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.remote.WriteMessage(frame)
}

// receive hands out what the command writes until the stream ends, and tells
// whether the server said how the command ended.
func receive(remote MessageStream, stdout, stderr io.Writer) (ExitStatus, bool, error) {
	var status ExitStatus

	for {
		frame, err := remote.ReadMessage()
		if errors.Is(err, io.EOF) {
			return status, false, nil
		}
		if err != nil {
			return status, false, err
		}
		if len(frame) == 0 {
			continue
		}

		channel, payload := frame[0], frame[1:]
		switch channel {
		case ChannelStdout:
			if _, err := stdout.Write(payload); err != nil {
				return status, false, err
			}
		case ChannelStderr:
			if _, err := stderr.Write(payload); err != nil {
				return status, false, err
			}
		case ChannelStatus:
			if err := json.Unmarshal(payload, &status); err != nil {
				return status, false, fmt.Errorf("cannot read how the command ended: %w", err)
			}
			// Status is the final protocol frame. Returning immediately keeps a
			// proxy that closes without a WebSocket close frame from turning an
			// already completed command into a transport failure.
			return status, true, nil
		default:
			return status, false, fmt.Errorf("the server sent a frame of channel %d, which this client does not know", channel)
		}
	}
}

// forwardInput sends the input on the input channel and says once, at the end
// of it, that no more is coming. A command that reads to the end of its input,
// such as `cat` on a file piped into it, only ends once it is told so, and the
// other way to say it is to close the connection, which takes the command down
// before it can report how it ended.
func forwardInput(out *sender, in io.Reader) error {
	err := copyInput(out, in)

	// Whatever ended the input, none of it is coming any more, so the far end
	// is told either way.
	if closeErr := out.send(ChannelStdinClose, nil); err == nil {
		err = closeErr
	}
	return err
}

func copyInput(out *sender, in io.Reader) error {
	buffer := make([]byte, inputBufferSize)

	for {
		read, err := in.Read(buffer)
		if read > 0 {
			if sendErr := out.send(ChannelStdin, buffer[:read]); sendErr != nil {
				return sendErr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// watchSize tells the far end how big the terminal is, now and whenever the
// user resizes it, so that a command that draws a screen knows what it draws
// on. The returned function gives up watching.
func watchSize(out *sender, terminal ExecTerminal) (func(), error) {
	if err := sendSize(out, terminal); err != nil {
		return nil, err
	}

	changed, stop := terminal.Resized()
	done := make(chan struct{})

	go func() {
		for {
			select {
			case _, open := <-changed:
				if !open {
					return
				}
				if err := sendSize(out, terminal); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	return func() {
		stop()
		close(done)
	}, nil
}

func sendSize(out *sender, terminal ExecTerminal) error {
	size, err := terminal.Size()
	if err != nil {
		return err
	}

	payload, err := json.Marshal(size)
	if err != nil {
		return err
	}
	return out.send(ChannelResize, payload)
}

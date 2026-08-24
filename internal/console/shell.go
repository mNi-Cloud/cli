package console

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ShellStream preserves WebSocket frame types because VM shell uses binary
// frames for terminal bytes and text frames for JSON controls.
type ShellStream interface {
	ReadFrame() (text bool, payload []byte, err error)
	WriteMessage(payload []byte) error
	WriteTextMessage(payload []byte) error
	Close() error
}

type ShellTerminal interface {
	ExecTerminal
	io.Reader
}

type shellControl struct {
	Type     string `json:"type"`
	Cols     uint16 `json:"cols,omitempty"`
	Rows     uint16 `json:"rows,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
	Message  string `json:"message,omitempty"`
}

type lockedShellStream struct {
	ShellStream
	mu sync.Mutex
}

func (s *lockedShellStream) binary(payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.WriteMessage(payload)
}

func (s *lockedShellStream) control(value shellControl) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.WriteTextMessage(payload)
}

// Shell joins a local terminal to the guest shell protocol.
func Shell(terminal ShellTerminal, out io.Writer, remote ShellStream) (ExitStatus, error) {
	if terminal == nil || out == nil || remote == nil {
		return ExitStatus{}, errors.New("a shell needs a terminal, output, and connection")
	}
	restore, err := terminal.Raw()
	if err != nil {
		return ExitStatus{}, err
	}
	defer func() { _ = remote.Close() }()
	stream := &lockedShellStream{ShellStream: remote}
	if err := sendShellSize(stream, terminal); err != nil {
		_ = restore()
		return ExitStatus{}, err
	}
	stopResize := watchShellSize(stream, terminal)
	defer stopResize()

	inputDone := make(chan error, 1)
	go func() { inputDone <- copyShellInput(stream, terminal) }()

	status, outputErr := readShellOutput(out, remote)
	restoreErr := restore()
	select {
	case inputErr := <-inputDone:
		if inputErr != nil && !errors.Is(inputErr, errEscaped) && outputErr == nil {
			outputErr = inputErr
		}
	default:
	}
	if outputErr != nil {
		return ExitStatus{}, outputErr
	}
	return status, restoreErr
}

func copyShellInput(stream *lockedShellStream, terminal io.Reader) error {
	reader := &escapeReader{reader: terminal, escape: escapeKey}
	buffer := make([]byte, inputBufferSize)
	for {
		read, err := reader.Read(buffer)
		if read > 0 {
			if writeErr := stream.binary(buffer[:read]); writeErr != nil {
				return writeErr
			}
		}
		if errors.Is(err, errEscaped) || errors.Is(err, io.EOF) {
			if closeErr := stream.control(shellControl{Type: "close"}); closeErr != nil {
				return closeErr
			}
			return err
		}
		if err != nil {
			return err
		}
	}
}

func sendShellSize(stream *lockedShellStream, terminal ExecTerminal) error {
	size, err := terminal.Size()
	if err != nil {
		return err
	}
	return stream.control(shellControl{Type: "resize", Cols: size.Width, Rows: size.Height})
}

func watchShellSize(stream *lockedShellStream, terminal ExecTerminal) func() {
	changed, stop := terminal.Resized()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case _, open := <-changed:
				if !open {
					return
				}
				_ = sendShellSize(stream, terminal)
			case <-done:
				return
			}
		}
	}()
	return func() { stop(); close(done) }
}

func readShellOutput(out io.Writer, remote ShellStream) (ExitStatus, error) {
	for {
		text, payload, err := remote.ReadFrame()
		if errors.Is(err, io.EOF) {
			return ExitStatus{}, ErrNoExitStatus
		}
		if err != nil {
			return ExitStatus{}, err
		}
		if !text {
			if _, err := out.Write(payload); err != nil {
				return ExitStatus{}, err
			}
			continue
		}
		var control shellControl
		if err := json.Unmarshal(payload, &control); err != nil {
			return ExitStatus{}, fmt.Errorf("cannot read a shell control: %w", err)
		}
		switch control.Type {
		case "exit":
			return ExitStatus{ExitCode: control.ExitCode, Message: control.Message}, nil
		case "error":
			if control.Message == "" {
				control.Message = "the guest shell failed"
			}
			return ExitStatus{}, errors.New(control.Message)
		default:
			return ExitStatus{}, fmt.Errorf("the server sent an unknown shell control %q", control.Type)
		}
	}
}

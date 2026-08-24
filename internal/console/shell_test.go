package console

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

type shellFrame struct {
	text    bool
	payload []byte
}

type fakeShellStream struct {
	mu     sync.Mutex
	frames []shellFrame
	binary [][]byte
	text   [][]byte
	closed bool
}

func (s *fakeShellStream) ReadFrame() (bool, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.frames) == 0 {
		return false, nil, io.EOF
	}
	frame := s.frames[0]
	s.frames = s.frames[1:]
	return frame.text, frame.payload, nil
}

func (s *fakeShellStream) WriteMessage(payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.binary = append(s.binary, bytes.Clone(payload))
	return nil
}

func (s *fakeShellStream) WriteTextMessage(payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.text = append(s.text, bytes.Clone(payload))
	return nil
}

func (s *fakeShellStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *fakeShellStream) snapshot() (binary, text [][]byte, closed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte(nil), s.binary...), append([][]byte(nil), s.text...), s.closed
}

type fakeShellTerminal struct {
	*fakeExecTerminal
	io.Reader
}

func TestShellUsesBinaryTerminalFramesAndTextControls(t *testing.T) {
	exit, _ := json.Marshal(shellControl{Type: "exit", ExitCode: 3})
	stream := &fakeShellStream{frames: []shellFrame{
		{payload: []byte("guest$ ")},
		{text: true, payload: exit},
	}}
	terminal := &fakeShellTerminal{fakeExecTerminal: newFakeExecTerminal(120, 40), Reader: strings.NewReader("")}
	out := &bytes.Buffer{}

	status, err := Shell(terminal, out, stream)
	if err != nil {
		t.Fatalf("Shell() error = %v", err)
	}
	if status.ExitCode != 3 || out.String() != "guest$ " {
		t.Fatalf("Shell() = %+v, output %q", status, out)
	}
	_, textFrames, closed := stream.snapshot()
	if !terminal.wasRestored() || !closed {
		t.Error("Shell() did not restore the terminal and close the connection")
	}
	if len(textFrames) == 0 {
		t.Fatal("Shell() sent no initial resize")
	}
	var resize shellControl
	if err := json.Unmarshal(textFrames[0], &resize); err != nil {
		t.Fatal(err)
	}
	if resize.Type != "resize" || resize.Cols != 120 || resize.Rows != 40 {
		t.Errorf("initial control = %+v", resize)
	}
}

func TestShellInputSendsBytesAndCloseControl(t *testing.T) {
	stream := &lockedShellStream{ShellStream: &fakeShellStream{}}
	err := copyShellInput(stream, strings.NewReader("pwd"+string(rune(escapeKey))))
	if err != nil && !errors.Is(err, errEscaped) {
		t.Fatalf("copyShellInput() error = %v", err)
	}
	fake := stream.ShellStream.(*fakeShellStream)
	binaryFrames, textFrames, _ := fake.snapshot()
	if len(binaryFrames) != 1 || string(binaryFrames[0]) != "pwd" {
		t.Errorf("binary frames = %q", binaryFrames)
	}
	var closeControl shellControl
	if len(textFrames) != 1 || json.Unmarshal(textFrames[0], &closeControl) != nil || closeControl.Type != "close" {
		t.Errorf("text frames = %q", textFrames)
	}
}

package console

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeStream stands for the connection a command runs over. It hands out the
// frames it was given to say and keeps the ones it is sent.
type fakeStream struct {
	mutex sync.Mutex
	sent  [][]byte

	says     chan []byte
	ended    sync.Once
	closed   chan struct{}
	closeOne sync.Once
}

func newFakeStream(says ...[]byte) *fakeStream {
	stream := &fakeStream{says: make(chan []byte, len(says)+1), closed: make(chan struct{})}
	for _, frame := range says {
		stream.says <- frame
	}
	return stream
}

func (s *fakeStream) ReadMessage() ([]byte, error) {
	select {
	case frame, open := <-s.says:
		if !open {
			return nil, io.EOF
		}
		return frame, nil
	case <-s.closed:
		return nil, io.EOF
	}
}

func (s *fakeStream) WriteMessage(payload []byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.sent = append(s.sent, bytes.Clone(payload))
	return nil
}

func (s *fakeStream) Close() error {
	s.closeOne.Do(func() { close(s.closed) })
	return nil
}

// say hands out one more frame, for a server that answers what it was sent.
func (s *fakeStream) say(frame []byte) {
	s.says <- frame
}

// end lets the stream run out of frames, which is how a server that is done
// leaves.
func (s *fakeStream) end() {
	s.ended.Do(func() { close(s.says) })
}

func (s *fakeStream) frames() [][]byte {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return append([][]byte{}, s.sent...)
}

// fakeExecTerminal stands in for the terminal of the process, which a test has
// none of.
type fakeExecTerminal struct {
	size    TerminalSize
	rawErr  error
	resized chan struct{}

	mutex    sync.Mutex
	restored bool
	watching bool
}

func newFakeExecTerminal(width, height uint16) *fakeExecTerminal {
	return &fakeExecTerminal{
		size:    TerminalSize{Width: width, Height: height},
		resized: make(chan struct{}, 1),
	}
}

func (f *fakeExecTerminal) Raw() (func() error, error) {
	if f.rawErr != nil {
		return nil, f.rawErr
	}
	return func() error {
		f.mutex.Lock()
		defer f.mutex.Unlock()
		f.restored = true
		return nil
	}, nil
}

func (f *fakeExecTerminal) Size() (TerminalSize, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.size, nil
}

func (f *fakeExecTerminal) Resized() (<-chan struct{}, func()) {
	f.mutex.Lock()
	f.watching = true
	f.mutex.Unlock()

	return f.resized, func() {
		f.mutex.Lock()
		defer f.mutex.Unlock()
		f.watching = false
	}
}

// resize makes the terminal a new size and tells whoever watches it.
func (f *fakeExecTerminal) resize(width, height uint16) {
	f.mutex.Lock()
	f.size = TerminalSize{Width: width, Height: height}
	f.mutex.Unlock()

	f.resized <- struct{}{}
}

func (f *fakeExecTerminal) wasRestored() bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.restored
}

func (f *fakeExecTerminal) isWatching() bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.watching
}

type execResult struct {
	status ExitStatus
	err    error
}

func frame(channel byte, payload string) []byte {
	return append([]byte{channel}, payload...)
}

func statusFrame(t *testing.T, status ExitStatus) []byte {
	t.Helper()

	payload, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return append([]byte{ChannelStatus}, payload...)
}

// runExec starts a command and hands back where its result arrives, so that a
// test can look at the stream while the command is still running.
func runExec(ctx context.Context, stream MessageStream, options ExecOptions) <-chan execResult {
	done := make(chan execResult, 1)
	go func() {
		status, err := Exec(ctx, stream, options)
		done <- execResult{status: status, err: err}
	}()
	return done
}

func awaitExec(t *testing.T, done <-chan execResult) execResult {
	t.Helper()

	select {
	case result := <-done:
		return result
	case <-time.After(waitTimeout):
		t.Fatal("Exec() did not return after the stream ended")
		return execResult{}
	}
}

func outputOptions() (ExecOptions, *syncBuffer, *syncBuffer) {
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	return ExecOptions{Stdout: stdout, Stderr: stderr}, stdout, stderr
}

func TestExecSendsWhatTheUserTypesOnTheInputChannel(t *testing.T) {
	stream := newFakeStream()
	defer stream.end()

	options, _, _ := outputOptions()
	options.Stdin = strings.NewReader("ls\n")
	done := runExec(context.Background(), stream, options)

	waitFor(t, func() bool { return len(stream.frames()) > 0 }, "the input of the user")

	if got, want := stream.frames()[0], frame(ChannelStdin, "ls\n"); !bytes.Equal(got, want) {
		t.Errorf("the stream was sent %q, want %q", got, want)
	}

	stream.end()
	awaitExec(t, done)
}

func TestExecSaysOnceThatTheInputEnded(t *testing.T) {
	stream := newFakeStream()
	defer stream.end()

	options, _, _ := outputOptions()
	options.Stdin = strings.NewReader("ls\n")
	done := runExec(context.Background(), stream, options)

	waitFor(t, func() bool { return len(stream.frames()) > 1 }, "the end of the input")

	frames := stream.frames()
	if len(frames) != 2 {
		t.Fatalf("the stream was sent %q, want the input and the end of it", frames)
	}
	if got, want := frames[1], []byte{ChannelStdinClose}; !bytes.Equal(got, want) {
		t.Errorf("the stream was sent %q, want %q, which carries nothing beside the channel", got, want)
	}

	stream.end()
	awaitExec(t, done)
}

func TestExecWaitsForTheCommandAfterTheInputEnded(t *testing.T) {
	stream := newFakeStream()
	defer stream.end()

	options, _, _ := outputOptions()
	options.Stdin = strings.NewReader("what to read\n")
	done := runExec(context.Background(), stream, options)

	waitFor(t, func() bool { return len(stream.frames()) > 1 }, "the end of the input")

	select {
	case result := <-done:
		t.Fatalf("Exec() = %+v, want it to wait for the command after the input ended", result)
	case <-time.After(50 * time.Millisecond):
	}

	stream.say(statusFrame(t, ExitStatus{ExitCode: 7}))
	stream.end()

	result := awaitExec(t, done)
	if result.err != nil {
		t.Fatalf("Exec() error = %v", result.err)
	}
	if result.status.ExitCode != 7 {
		t.Errorf("ExitCode = %d, want the code the command ended with", result.status.ExitCode)
	}
}

func TestExecWithoutInputSendsNothing(t *testing.T) {
	stream := newFakeStream(statusFrame(t, ExitStatus{}))
	stream.end()

	options, _, _ := outputOptions()
	if _, err := Exec(context.Background(), stream, options); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	if got := stream.frames(); len(got) != 0 {
		t.Errorf("the stream was sent %q, want nothing without input", got)
	}
}

func TestExecWritesEachOutputWhereItBelongs(t *testing.T) {
	stream := newFakeStream(
		frame(ChannelStdout, "what it wrote"),
		frame(ChannelStderr, "what went wrong"),
		statusFrame(t, ExitStatus{}),
	)
	stream.end()

	options, stdout, stderr := outputOptions()
	if _, err := Exec(context.Background(), stream, options); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	if stdout.String() != "what it wrote" {
		t.Errorf("stdout = %q, want what the command wrote", stdout.String())
	}
	if stderr.String() != "what went wrong" {
		t.Errorf("stderr = %q, want what went wrong", stderr.String())
	}
}

func TestExecReportsHowTheCommandEnded(t *testing.T) {
	tests := []struct {
		name   string
		status ExitStatus
	}{
		{name: "success", status: ExitStatus{}},
		{name: "failure", status: ExitStatus{ExitCode: 3}},
		{name: "failure with a message", status: ExitStatus{ExitCode: 126, Message: "cannot run the command"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stream := newFakeStream(statusFrame(t, test.status))
			stream.end()

			options, _, _ := outputOptions()
			status, err := Exec(context.Background(), stream, options)
			if err != nil {
				t.Fatalf("Exec() error = %v", err)
			}
			if status != test.status {
				t.Errorf("Exec() = %+v, want %+v", status, test.status)
			}
		})
	}
}

func TestExecReportsAStreamThatEndedWithoutSayingHow(t *testing.T) {
	stream := newFakeStream(frame(ChannelStdout, "half of it"))
	stream.end()

	options, _, _ := outputOptions()
	if _, err := Exec(context.Background(), stream, options); !errors.Is(err, ErrNoExitStatus) {
		t.Fatalf("Exec() error = %v, want the missing exit code reported", err)
	}
}

func TestExecRefusesAChannelItDoesNotKnow(t *testing.T) {
	stream := newFakeStream(frame(9, "who knows"))
	stream.end()

	options, _, _ := outputOptions()
	_, err := Exec(context.Background(), stream, options)
	if err == nil {
		t.Fatal("Exec() error = nil, want an unknown channel refused")
	}
	if !strings.Contains(err.Error(), "9") {
		t.Errorf("Exec() error = %q, want it to name the channel", err)
	}
}

func TestExecIgnoresAnEmptyFrame(t *testing.T) {
	stream := newFakeStream([]byte{}, statusFrame(t, ExitStatus{ExitCode: 1}))
	stream.end()

	options, _, _ := outputOptions()
	status, err := Exec(context.Background(), stream, options)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if status.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want the frame that carried nothing passed over", status.ExitCode)
	}
}

func TestExecSendsTheSizeOfTheTerminal(t *testing.T) {
	stream := newFakeStream()
	defer stream.end()
	terminal := newFakeExecTerminal(80, 24)

	options, _, _ := outputOptions()
	options.Stdin = newBlockingReader()
	options.Terminal = terminal
	done := runExec(context.Background(), stream, options)

	waitFor(t, func() bool { return len(stream.frames()) > 0 }, "the size of the terminal")
	assertSize(t, stream.frames()[0], TerminalSize{Width: 80, Height: 24})

	waitFor(t, terminal.isWatching, "a watch on the size of the terminal")
	terminal.resize(120, 40)

	waitFor(t, func() bool { return len(stream.frames()) > 1 }, "the new size of the terminal")
	assertSize(t, stream.frames()[1], TerminalSize{Width: 120, Height: 40})

	stream.end()
	awaitExec(t, done)

	if terminal.isWatching() {
		t.Error("the size of the terminal is still watched after the command ended")
	}
}

func assertSize(t *testing.T, got []byte, want TerminalSize) {
	t.Helper()

	if len(got) == 0 || got[0] != ChannelResize {
		t.Fatalf("the stream was sent %q, want a frame of the resize channel", got)
	}

	var size TerminalSize
	if err := json.Unmarshal(got[1:], &size); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if size != want {
		t.Errorf("size = %+v, want %+v", size, want)
	}
}

func TestExecRestoresTheTerminalWhenTheCommandEnds(t *testing.T) {
	stream := newFakeStream(statusFrame(t, ExitStatus{}))
	stream.end()
	terminal := newFakeExecTerminal(80, 24)

	options, _, _ := outputOptions()
	options.Stdin = newBlockingReader()
	options.Terminal = terminal
	if _, err := Exec(context.Background(), stream, options); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	if !terminal.wasRestored() {
		t.Error("the terminal was left in raw mode")
	}
}

func TestExecRestoresTheTerminalWhenTheStreamFails(t *testing.T) {
	stream := newFakeStream(frame(9, "who knows"))
	stream.end()
	terminal := newFakeExecTerminal(80, 24)

	options, _, _ := outputOptions()
	options.Stdin = newBlockingReader()
	options.Terminal = terminal
	if _, err := Exec(context.Background(), stream, options); err == nil {
		t.Fatal("Exec() error = nil, want an unknown channel refused")
	}

	if !terminal.wasRestored() {
		t.Error("the terminal was left in raw mode")
	}
}

func TestExecReportsATerminalItCannotTakeOver(t *testing.T) {
	refused := errors.New("inappropriate ioctl for device")
	stream := newFakeStream(statusFrame(t, ExitStatus{}))
	stream.end()

	terminal := newFakeExecTerminal(80, 24)
	terminal.rawErr = refused

	options, _, _ := outputOptions()
	options.Stdin = newBlockingReader()
	options.Terminal = terminal
	if _, err := Exec(context.Background(), stream, options); !errors.Is(err, refused) {
		t.Fatalf("Exec() error = %v, want the terminal failure", err)
	}
	if got := stream.frames(); len(got) != 0 {
		t.Errorf("the stream was sent %q, want nothing before the terminal was taken over", got)
	}
}

func TestExecRefusesATerminalWithoutInput(t *testing.T) {
	stream := newFakeStream()
	defer stream.end()

	options, _, _ := outputOptions()
	options.Terminal = newFakeExecTerminal(80, 24)

	if _, err := Exec(context.Background(), stream, options); err == nil {
		t.Fatal("Exec() error = nil, want a terminal without input refused")
	}
}

func TestExecNeedsSomewhereToWrite(t *testing.T) {
	tests := []struct {
		name    string
		options ExecOptions
	}{
		{name: "no stdout", options: ExecOptions{Stderr: &syncBuffer{}}},
		{name: "no stderr", options: ExecOptions{Stdout: &syncBuffer{}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stream := newFakeStream()
			defer stream.end()

			if _, err := Exec(context.Background(), stream, test.options); err == nil {
				t.Fatal("Exec() error = nil, want somewhere to write asked for")
			}
		})
	}
}

func TestExecEndsWhenTheUserStopsWaiting(t *testing.T) {
	stream := newFakeStream()
	defer stream.end()

	ctx, stop := context.WithCancel(context.Background())
	options, _, _ := outputOptions()
	done := runExec(ctx, stream, options)

	stop()

	result := awaitExec(t, done)
	if result.err != nil {
		t.Errorf("Exec() error = %v, want a command the user stopped waiting for to be no failure", result.err)
	}
}

func TestCopyWritesWhatTheStreamCarries(t *testing.T) {
	remote := newFakeConsole("2024-01-01 ready\n")
	out := &syncBuffer{}

	done := make(chan error, 1)
	go func() { done <- Copy(context.Background(), out, remote) }()

	waitFor(t, func() bool { return out.String() == "2024-01-01 ready\n" }, "the log line")

	_ = remote.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Copy() error = %v, want a stream that ended to be no failure", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Copy() did not return after the stream ended")
	}
}

func TestCopyEndsWhenTheUserStopsWaiting(t *testing.T) {
	remote := newFakeConsole()
	defer func() { _ = remote.Close() }()

	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Copy(ctx, &syncBuffer{}, remote) }()

	stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Copy() error = %v, want a stream the user left to be no failure", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Copy() did not return after the user stopped waiting")
	}
}

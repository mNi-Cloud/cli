package console

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

const waitTimeout = 5 * time.Second

// syncBuffer collects output that is written while the test reads it.
type syncBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.Write(p)
}

func (b *syncBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.String()
}

// fakeTerminal stands in for the terminal of the process, which a test has
// none of.
type fakeTerminal struct {
	io.Reader
	rawErr   error
	restored bool
}

func (f *fakeTerminal) Raw() (func() error, error) {
	if f.rawErr != nil {
		return nil, f.rawErr
	}
	return func() error {
		f.restored = true
		return nil
	}, nil
}

// blockingReader stands for an input nobody types on.
type blockingReader struct {
	release chan struct{}
}

func newBlockingReader() *blockingReader {
	return &blockingReader{release: make(chan struct{})}
}

func (r *blockingReader) Read([]byte) (int, error) {
	<-r.release
	return 0, io.EOF
}

// fakeConsole keeps what it is sent and hands out what it was given to say. It
// only ends when it is closed.
type fakeConsole struct {
	mutex    sync.Mutex
	received bytes.Buffer

	says   chan []byte
	closed chan struct{}
	once   sync.Once
}

func newFakeConsole(says ...string) *fakeConsole {
	console := &fakeConsole{says: make(chan []byte, len(says)), closed: make(chan struct{})}
	for _, line := range says {
		console.says <- []byte(line)
	}
	return console
}

func (c *fakeConsole) Read(p []byte) (int, error) {
	select {
	case said := <-c.says:
		return copy(p, said), nil
	case <-c.closed:
		return 0, io.EOF
	}
}

func (c *fakeConsole) Write(p []byte) (int, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.received.Write(p)
}

func (c *fakeConsole) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

func (c *fakeConsole) sent() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.received.String()
}

func TestAttachSendsWhatIsTypedUntilTheEscapeKey(t *testing.T) {
	terminal := &fakeTerminal{Reader: strings.NewReader("root" + string(rune(escapeKey)) + "never")}
	console := newFakeConsole()
	defer func() { _ = console.Close() }()

	if err := Attach(terminal, &syncBuffer{}, console); err != nil {
		t.Fatalf("Attach() error = %v, want leaving the console to be no failure", err)
	}

	if got := console.sent(); got != "root" {
		t.Errorf("the console was sent %q, want what was typed before the escape key", got)
	}
	if !terminal.restored {
		t.Error("the terminal was left in raw mode")
	}
}

func TestAttachWritesWhatTheConsoleSays(t *testing.T) {
	typing := newBlockingReader()
	defer close(typing.release)

	terminal := &fakeTerminal{Reader: typing}
	console := newFakeConsole("login: ")
	defer func() { _ = console.Close() }()

	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() { done <- Attach(terminal, out, console) }()

	waitFor(t, func() bool { return out.String() == "login: " }, "the console output")

	_ = console.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Attach() error = %v, want a console that ended to be no failure", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Attach() did not return after the console ended")
	}

	if !terminal.restored {
		t.Error("the terminal was left in raw mode")
	}
}

func TestAttachReportsATerminalItCannotTakeOver(t *testing.T) {
	refused := errors.New("inappropriate ioctl for device")
	terminal := &fakeTerminal{Reader: strings.NewReader(""), rawErr: refused}
	console := newFakeConsole()
	defer func() { _ = console.Close() }()

	if err := Attach(terminal, &syncBuffer{}, console); !errors.Is(err, refused) {
		t.Fatalf("Attach() error = %v, want the terminal failure", err)
	}
	if console.sent() != "" {
		t.Error("the console was written to although the terminal was never taken over")
	}
}

// waitFor polls a condition, because the two directions of a console run on
// their own.
func waitFor(t *testing.T, met func() bool, what string) {
	t.Helper()

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if met() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s never came", what)
}

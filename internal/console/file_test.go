package console

import (
	"os"
	"testing"
)

// TestFileSizesFromTheOutputHandle guards the Windows fix: a terminal is sized
// from the handle it writes to, because the input handle answers no size query
// there. Without an output handle, the input handle is sized from, which is
// what a Unix terminal expects.
func TestFileSizesFromTheOutputHandle(t *testing.T) {
	in, inWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	defer inWriter.Close()

	out, outReader, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	defer outReader.Close()

	file := NewFile(in, out)
	if got, want := file.outputFD(), int(out.Fd()); got != want {
		t.Errorf("outputFD() = %d, want the output handle %d", got, want)
	}

	file = NewFile(in, nil)
	if got, want := file.outputFD(), int(in.Fd()); got != want {
		t.Errorf("outputFD() = %d, want the input handle %d when there is no output handle", got, want)
	}
}

// TestFileReadsFromTheInputHandle keeps the input and output handles apart: the
// bytes the user types come from the input handle, never from the output one.
func TestFileReadsFromTheInputHandle(t *testing.T) {
	in, inWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	defer inWriter.Close()

	out, outReader, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	defer outReader.Close()

	file := NewFile(in, out)
	go func() {
		_, _ = inWriter.Write([]byte("hi"))
	}()

	buffer := make([]byte, 2)
	read, err := file.Read(buffer)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if read != 2 || string(buffer) != "hi" {
		t.Errorf("Read() = %d %q, want 2 hi", read, buffer)
	}
}

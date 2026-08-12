package console

import (
	"bytes"
	"io"
)

// escapeReader reads what the user types and stops at the key that leaves the
// console. Everything typed before it is still handed on, so the machine gets
// the whole line the user was writing.
type escapeReader struct {
	reader  io.Reader
	escape  byte
	escaped bool
}

func (e *escapeReader) Read(p []byte) (int, error) {
	if e.escaped {
		return 0, errEscaped
	}

	read, err := e.reader.Read(p)
	if read == 0 {
		return read, err
	}

	at := bytes.IndexByte(p[:read], e.escape)
	if at < 0 {
		return read, err
	}

	e.escaped = true
	if at == 0 {
		return 0, errEscaped
	}
	return at, nil
}

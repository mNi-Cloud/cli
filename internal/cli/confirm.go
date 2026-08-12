package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// yesFlagName is the flag that answers a confirmation ahead of time.
const yesFlagName = "yes"

// Confirm asks the user to allow a command that destroys something. The answer
// defaults to no, so an empty line stops the command.
func (d *Deps) Confirm(question string) (bool, error) {
	if !d.Interactive {
		return false, errNotATerminal
	}

	fmt.Fprintf(d.Out, "%s [y/N]: ", question)

	line, err := bufio.NewReader(d.In).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("cannot read the answer: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// errNotATerminal marks a run that cannot ask anything, so that each command
// can name the flag that answers its own question.
var errNotATerminal = errors.New("the input is not a terminal")

// isTerminal reports whether a question put to an input stream would reach a
// person. A pipe, a file and /dev/null cannot answer, so such a run is never
// asked.
func isTerminal(in io.Reader) bool {
	_, ok := terminalFile(in)
	return ok
}

// terminalFile returns the terminal an input stream is, if it is one. The file
// mode alone does not tell them apart, because /dev/null is a character device
// like a terminal is, so the terminal is asked itself.
func terminalFile(in io.Reader) (*os.File, bool) {
	file, ok := in.(*os.File)
	if !ok {
		return nil, false
	}
	if !term.IsTerminal(int(file.Fd())) {
		return nil, false
	}
	return file, true
}

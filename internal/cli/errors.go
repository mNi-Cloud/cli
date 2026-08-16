package cli

import (
	"errors"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/auth"
	"github.com/urfave/cli/v3"
)

// UserFacing peels the transport wrapper off an error whose own message already
// tells the user what to do. A missing session surfaces from inside an HTTP
// round trip, so without this the advice is buried behind the request URL.
//
// It reports nothing for an error that carries no message of its own, such as
// the exit code of a command that already wrote whatever it had to say.
func UserFacing(err error) error {
	if err == nil || err.Error() == "" {
		return nil
	}

	var loginRequired *auth.LoginRequiredError
	if errors.As(err, &loginRequired) {
		return loginRequired
	}
	if api.IsInsufficientScope(err) {
		return errors.New("this session was not granted the scope the mNi Cloud API needs: run `mni login` again, because renewing a session cannot add one")
	}
	return err
}

// ExitCode is the status this process ends with after an error. A command that
// ran a program for the user ends with the code of that program, so that a
// script reads it the way it reads the program run on this machine.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitCoder cli.ExitCoder
	if errors.As(err, &exitCoder) {
		return exitCoder.ExitCode()
	}
	return 1
}

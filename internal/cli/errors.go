package cli

import (
	"errors"

	"github.com/mNi-Cloud/cli/internal/auth"
)

// UserFacing peels the transport wrapper off an error whose own message already
// tells the user what to do. A missing session surfaces from inside an HTTP
// round trip, so without this the advice is buried behind the request URL.
func UserFacing(err error) error {
	var loginRequired *auth.LoginRequiredError
	if errors.As(err, &loginRequired) {
		return loginRequired
	}
	return err
}

package cli

import (
	"errors"

	"github.com/mNi-Cloud/cli/internal/api"
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
	if api.IsInsufficientScope(err) {
		return errors.New("this session was not granted the scope the mNi Cloud API needs: run `mni login` again, because renewing a session cannot add one")
	}
	return err
}

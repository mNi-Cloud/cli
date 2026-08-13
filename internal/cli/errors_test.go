package cli

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/auth"
)

func TestUserFacingSendsATokenWithoutTheScopeBackToLogin(t *testing.T) {
	refused := &api.Error{
		StatusCode: http.StatusForbidden,
		Message:    "Forbidden",
		Challenge:  `Bearer error="insufficient_scope", scope="mni:api"`,
	}

	got := UserFacing(refused).Error()
	if !strings.Contains(got, "mni login") {
		t.Errorf("UserFacing() = %q, want it to send the user to `mni login`", got)
	}
	if strings.Contains(got, "expired") {
		t.Errorf("UserFacing() = %q, want it to not read as a session that ran out", got)
	}
}

func TestUserFacingLeavesAPlainRefusalAlone(t *testing.T) {
	refused := &api.Error{StatusCode: http.StatusForbidden, Message: "you may not delete this vpc"}

	if got := UserFacing(refused); got != error(refused) {
		t.Errorf("UserFacing() = %q, want the refusal of the server as it is", got)
	}
}

func TestUserFacingLiftsTheLoginAdviceOutOfARoundTrip(t *testing.T) {
	missing := &auth.LoginRequiredError{Context: "mnicloud"}
	wrapped := &url.Error{Op: "Get", URL: "https://api.mnicloud.jp/tenants", Err: missing}

	if got := UserFacing(wrapped); got != error(missing) {
		t.Errorf("UserFacing() = %q, want the advice on its own", got)
	}
}

package auth

import (
	"context"
	"fmt"
	"net/url"
)

// AuthorizationError is an RFC 6749 §4.1.2.1 error handed back on the redirect.
type AuthorizationError struct {
	Code        string
	Description string
}

func (e *AuthorizationError) Error() string {
	message := "the identity provider refused the login: " + e.Code
	if e.Description != "" {
		message += " (" + e.Description + ")"
	}
	return message
}

// callbackResult is the authorization response the browser handed back.
type callbackResult struct {
	Code   string
	State  string
	Issuer string
}

type callbackOutcome struct {
	result callbackResult
	err    error
}

// responseSource is one way for the authorization response to reach the CLI.
type responseSource interface {
	// Outcomes hands back the authorization response once it arrives.
	Outcomes() <-chan callbackOutcome
}

// firstOutcome waits until one of the sources hands back the authorization
// response, and takes that one. A source that never answers does not hold up
// the others.
func firstOutcome(ctx context.Context, sources ...responseSource) (callbackResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	first := make(chan callbackOutcome, len(sources))
	for _, source := range sources {
		go func() {
			select {
			case outcome := <-source.Outcomes():
				first <- outcome
			case <-ctx.Done():
			}
		}()
	}

	select {
	case outcome := <-first:
		return outcome.result, outcome.err
	case <-ctx.Done():
		return callbackResult{}, ctx.Err()
	}
}

// readCallback reads the authorization response of RFC 6749 §4.1.2 out of the
// query the identity provider redirected to.
func readCallback(query url.Values) callbackOutcome {
	if code := query.Get("error"); code != "" {
		return callbackOutcome{err: &AuthorizationError{
			Code:        code,
			Description: query.Get("error_description"),
		}}
	}

	code := query.Get("code")
	if code == "" {
		return callbackOutcome{err: fmt.Errorf("the identity provider redirected back without an authorization code")}
	}

	return callbackOutcome{result: callbackResult{
		Code:   code,
		State:  query.Get("state"),
		Issuer: query.Get("iss"),
	}}
}

package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

const pastedRedirect = "http://localhost:43123/callback?code=the-code&state=the-state&iss=https%3A%2F%2Fissuer.test"

func newTestRedirectPaste(input io.Reader, output io.Writer) *redirectPaste {
	redirect := &url.URL{Scheme: "http", Host: "localhost:43123", Path: "/callback"}
	return newRedirectPaste(redirect, input, output)
}

// readTyped runs a paste over what the user typed until the paste stops.
func readTyped(t *testing.T, typed string) (*redirectPaste, *bytes.Buffer) {
	t.Helper()

	output := &bytes.Buffer{}
	paste := newTestRedirectPaste(strings.NewReader(typed), output)
	paste.read(context.Background())
	return paste, output
}

// handedBack returns the outcome a paste handed back, if it handed one back.
func handedBack(paste *redirectPaste) (callbackOutcome, bool) {
	select {
	case outcome := <-paste.Outcomes():
		return outcome, true
	default:
		return callbackOutcome{}, false
	}
}

func TestRedirectPasteTakesThePastedRedirect(t *testing.T) {
	for _, typed := range []string{
		pastedRedirect + "\n",
		"  " + pastedRedirect + " \r\n",
		pastedRedirect,
	} {
		t.Run(typed, func(t *testing.T) {
			paste, output := readTyped(t, typed)

			outcome, ok := handedBack(paste)
			if !ok {
				t.Fatal("read() handed back nothing, want the pasted response")
			}
			if outcome.err != nil {
				t.Fatalf("outcome error = %v", outcome.err)
			}
			want := callbackResult{Code: "the-code", State: "the-state", Issuer: "https://issuer.test"}
			if outcome.result != want {
				t.Errorf("outcome = %+v, want %+v", outcome.result, want)
			}
			if output.Len() != 0 {
				t.Errorf("output = %q, want nothing asked", output)
			}
		})
	}
}

func TestRedirectPasteAsksAgainForALineThatIsNotTheRedirect(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		reason string
	}{
		{name: "empty line", line: "", reason: "it is empty"},
		{name: "blank line", line: "   ", reason: "it is empty"},
		{name: "plain text", line: "yes please", reason: "it is not a URL"},
		{name: "no scheme", line: "localhost:43123/callback?code=c", reason: "it is not a URL"},
		{
			name:   "authorization URL",
			line:   "https://issuer.test/auth?client_id=mni-cli&redirect_uri=http%3A%2F%2Flocalhost%3A43123%2Fcallback",
			reason: "it goes to https://issuer.test, not to http://localhost:43123",
		},
		{
			name:   "port of another login",
			line:   "http://localhost:9999/callback?code=c&state=s",
			reason: "it goes to http://localhost:9999, not to http://localhost:43123",
		},
		{
			name:   "another host",
			line:   "http://127.0.0.1:43123/callback?code=c&state=s",
			reason: "it goes to http://127.0.0.1:43123, not to http://localhost:43123",
		},
		{
			name:   "another scheme",
			line:   "https://localhost:43123/callback?code=c&state=s",
			reason: "it goes to https://localhost:43123, not to http://localhost:43123",
		},
		{
			name:   "another path",
			line:   "http://localhost:43123/favicon.ico?code=c&state=s",
			reason: `its path is "/favicon.ico", not "/callback"`,
		},
		{
			name:   "no path",
			line:   "http://localhost:43123?code=c&state=s",
			reason: `its path is "", not "/callback"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paste, output := readTyped(t, tt.line+"\n"+pastedRedirect+"\n")

			wantAsked := "Could not use the pasted line (" + tt.reason + "). " +
				"Paste the URL that starts with http://localhost:43123/callback and press Enter.\n"
			if output.String() != wantAsked {
				t.Errorf("output = %q, want %q", output, wantAsked)
			}

			outcome, ok := handedBack(paste)
			if !ok {
				t.Fatal("read() handed back nothing, want the redirect pasted after the bad line")
			}
			if outcome.err != nil || outcome.result.Code != "the-code" {
				t.Errorf("outcome = %+v, want the response of the redirect pasted after the bad line", outcome)
			}
		})
	}
}

func TestRedirectPasteHandsBackARefusal(t *testing.T) {
	paste, output := readTyped(t,
		"http://localhost:43123/callback?error=access_denied&error_description=the+user+said+no&state=s\n"+
			pastedRedirect+"\n")

	outcome, ok := handedBack(paste)
	if !ok {
		t.Fatal("read() handed back nothing, want the refusal")
	}
	var refusal *AuthorizationError
	if !errors.As(outcome.err, &refusal) {
		t.Fatalf("outcome error = %v, want an AuthorizationError", outcome.err)
	}
	if refusal.Code != "access_denied" {
		t.Errorf("Code = %q, want %q", refusal.Code, "access_denied")
	}
	if output.Len() != 0 {
		t.Errorf("output = %q, want the refusal to end the paste without asking again", output)
	}
}

func TestRedirectPasteHandsBackARedirectWithoutACode(t *testing.T) {
	paste, _ := readTyped(t, "http://localhost:43123/callback?state=s\n"+pastedRedirect+"\n")

	outcome, ok := handedBack(paste)
	if !ok {
		t.Fatal("read() handed back nothing, want the missing code reported")
	}
	if outcome.err == nil || !strings.Contains(outcome.err.Error(), "authorization code") {
		t.Errorf("outcome error = %v, want it to say the code is missing", outcome.err)
	}
}

func TestRedirectPasteStopsAtTheEndOfTheInput(t *testing.T) {
	for _, typed := range []string{"", "not a URL\n"} {
		t.Run(typed, func(t *testing.T) {
			paste, _ := readTyped(t, typed)

			if outcome, ok := handedBack(paste); ok {
				t.Errorf("read() handed back %+v, want nothing once the input ends", outcome)
			}
		})
	}
}

func TestRedirectPasteReportsAnInputThatBreaks(t *testing.T) {
	broken := errors.New("the terminal went away")
	paste := newTestRedirectPaste(iotest.ErrReader(broken), &bytes.Buffer{})

	paste.read(context.Background())

	outcome, ok := handedBack(paste)
	if !ok {
		t.Fatal("read() handed back nothing, want the read error")
	}
	if !errors.Is(outcome.err, broken) {
		t.Errorf("outcome error = %v, want it to wrap %v", outcome.err, broken)
	}
}

func TestRedirectPasteIsQuietOnceTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	output := &bytes.Buffer{}
	paste := newTestRedirectPaste(strings.NewReader("not a URL\n"+pastedRedirect+"\n"), output)
	paste.read(ctx)

	if outcome, ok := handedBack(paste); ok {
		t.Errorf("read() handed back %+v after the context ended, want nothing", outcome)
	}
	if output.Len() != 0 {
		t.Errorf("output = %q after the context ended, want nothing", output)
	}
}

func TestRedirectPasteStartReadsInTheBackground(t *testing.T) {
	input, typing := io.Pipe()
	t.Cleanup(func() { _ = typing.Close() })

	paste := newTestRedirectPaste(input, &bytes.Buffer{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	paste.Start(ctx)

	if _, err := io.WriteString(typing, pastedRedirect+"\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}

	result, err := firstOutcome(ctx, paste)
	if err != nil {
		t.Fatalf("firstOutcome() error = %v", err)
	}
	if result.Code != "the-code" {
		t.Errorf("Code = %q, want %q", result.Code, "the-code")
	}
}

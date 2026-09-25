package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// redirectPaste takes the authorization response from a redirect URL the user
// pastes. A browser on another machine than the CLI, as over SSH, cannot reach
// the loopback listener and ends on an error page, but its address bar still
// holds the whole redirect.
type redirectPaste struct {
	redirectURI *url.URL
	input       io.Reader
	output      io.Writer
	outcomes    chan callbackOutcome
}

// newRedirectPaste reads pasted lines from input and says on output why a line
// is not the redirect to redirectURI.
func newRedirectPaste(redirectURI *url.URL, input io.Reader, output io.Writer) *redirectPaste {
	return &redirectPaste{
		redirectURI: redirectURI,
		input:       input,
		output:      output,
		outcomes:    make(chan callbackOutcome, 1),
	}
}

// Start reads pasted lines in the background. A read on a terminal cannot be
// cut short, so the reader may stay blocked after ctx ends, but from then on it
// neither prints nor hands back anything.
func (p *redirectPaste) Start(ctx context.Context) {
	go p.read(ctx)
}

// Outcomes hands back the authorization response once the redirect is pasted.
func (p *redirectPaste) Outcomes() <-chan callbackOutcome {
	return p.outcomes
}

// read takes lines until one is the redirect. A line that is not asks for the
// next one. The end of the input hands back nothing, which leaves the response
// to the loopback listener.
func (p *redirectPaste) read(ctx context.Context) {
	lines := bufio.NewScanner(p.input)
	for lines.Scan() {
		if ctx.Err() != nil {
			return
		}

		query, err := p.redirectQuery(lines.Text())
		if err != nil {
			fmt.Fprintf(p.output, "Could not use the pasted line (%v). Paste the URL that starts with %s and press Enter.\n", err, p.redirectURI)
			continue
		}
		p.outcomes <- readCallback(query)
		return
	}

	if err := lines.Err(); err != nil && ctx.Err() == nil {
		p.outcomes <- callbackOutcome{err: fmt.Errorf("cannot read the pasted URL: %w", err)}
	}
}

// redirectQuery returns the query of a pasted line that is the redirect this
// login waits for. The URL has to go to the very address the loopback listener
// took, so that a URL of another login is not taken for this one.
func (p *redirectPaste) redirectQuery(line string) (url.Values, error) {
	text := strings.TrimSpace(line)
	if text == "" {
		return nil, errors.New("it is empty")
	}

	pasted, err := url.Parse(text)
	if err != nil || pasted.Scheme == "" || pasted.Host == "" {
		return nil, errors.New("it is not a URL")
	}
	if !sameOrigin(pasted, p.redirectURI) {
		return nil, fmt.Errorf("it goes to %s, not to %s", origin(pasted), origin(p.redirectURI))
	}
	if pasted.Path != p.redirectURI.Path {
		return nil, fmt.Errorf("its path is %q, not %q", pasted.Path, p.redirectURI.Path)
	}
	return pasted.Query(), nil
}

func sameOrigin(a, b *url.URL) bool {
	return a.Scheme == b.Scheme && a.Hostname() == b.Hostname() && a.Port() == b.Port()
}

func origin(u *url.URL) string {
	return u.Scheme + "://" + u.Host
}

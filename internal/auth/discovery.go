package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const (
	discoveryPath = "/.well-known/openid-configuration"

	grantAuthorizationCode = "authorization_code"
	grantRefreshToken      = "refresh_token"
)

// Metadata is the part of the OpenID provider configuration this CLI uses.
type Metadata struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	ScopesSupported                            []string `json:"scopes_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
}

// SupportsRefreshToken reports whether a stored session can be renewed without
// sending the user back to the browser.
func (m Metadata) SupportsRefreshToken() bool {
	return slices.Contains(m.GrantTypesSupported, grantRefreshToken)
}

// MetadataSource reads the configuration of an OpenID provider.
type MetadataSource interface {
	Discover(ctx context.Context, issuer string) (Metadata, error)
}

// Discoverer reads provider configuration over HTTP.
type Discoverer struct {
	HTTPClient *http.Client
}

// Discover reads and checks the configuration an issuer publishes.
func (d Discoverer) Discover(ctx context.Context, issuer string) (Metadata, error) {
	issuer = strings.TrimSuffix(issuer, "/")
	if issuer == "" {
		return Metadata{}, fmt.Errorf("no issuer to discover")
	}
	if _, err := url.Parse(issuer); err != nil {
		return Metadata{}, fmt.Errorf("issuer %q is not a URL: %w", issuer, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+discoveryPath, nil)
	if err != nil {
		return Metadata{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.httpClient().Do(req)
	if err != nil {
		return Metadata{}, fmt.Errorf("cannot reach the identity provider at %s: %w", issuer, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("the identity provider at %s answered %d for its configuration", issuer, resp.StatusCode)
	}

	var metadata Metadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return Metadata{}, fmt.Errorf("cannot read the configuration of %s: %w", issuer, err)
	}

	if err := metadata.validate(issuer); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

func (d Discoverer) httpClient() *http.Client {
	if d.HTTPClient != nil {
		return d.HTTPClient
	}
	return http.DefaultClient
}

// validate holds the provider to what this CLI needs. OpenID Connect Discovery
// §4.3 requires the published issuer to match the one that was asked.
func (m Metadata) validate(issuer string) error {
	if strings.TrimSuffix(m.Issuer, "/") != issuer {
		return fmt.Errorf("the identity provider at %s calls itself issuer %q", issuer, m.Issuer)
	}
	if m.AuthorizationEndpoint == "" {
		return fmt.Errorf("the configuration of %s has no authorization_endpoint", issuer)
	}
	if m.TokenEndpoint == "" {
		return fmt.Errorf("the configuration of %s has no token_endpoint", issuer)
	}
	if !slices.Contains(m.GrantTypesSupported, grantAuthorizationCode) {
		return fmt.Errorf("the identity provider at %s does not offer the %s grant", issuer, grantAuthorizationCode)
	}
	if !slices.Contains(m.CodeChallengeMethodsSupported, ChallengeMethodS256) {
		return fmt.Errorf("the identity provider at %s does not offer the %s code challenge", issuer, ChallengeMethodS256)
	}
	return nil
}

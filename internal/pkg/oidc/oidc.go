package oidc

import (
	"context"
	"errors"
	"strings"

	oidc "github.com/coreos/go-oidc"
	"golang.org/x/oauth2"
)

type (
	oidcFlow struct {
		oidcProvider oidc.Provider
		oauth2Config oauth2.Config
	}
)

func GetIdToken(ctx context.Context, endpoint string, realm string, clientId string) (*string, error) {
	oidcFlow := oidcFlow{}
	endpoint, _ = strings.CutSuffix(endpoint, "/")

	provider, err := oidc.NewProvider(ctx, endpoint+"/realms/"+realm)
	if err != nil {
		return nil, errors.New("Failed to fetch discovery document: " + err.Error())
	}
	oidcFlow.oidcProvider = *provider

	oidcFlow.oauth2Config = oauth2.Config{
		ClientID:    clientId,
		Endpoint:    provider.Endpoint(),
		RedirectURL: "http://localhost:51850/auth/callback",
		Scopes:      []string{oidc.ScopeOpenID, "profile", "roles"},
	}

	server := NewServer(oidcFlow)

	err = server.Serve()
	if err != nil {
		return nil, errors.New("Failed to start server: " + err.Error())
	}

	res := <-server.Res
	if res.Error != nil {
		return nil, errors.New("Failed to get id token: " + (*res.Error).Error())
	}
	return res.IdToken, nil
}

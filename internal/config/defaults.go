package config

// The endpoints of mNi Cloud. A login that names no server signs in here.
const (
	// DefaultContextName is the name a first login gives to the context it writes.
	DefaultContextName = "yakumo"
	// DefaultServer is the api-gateway of mNi Cloud.
	DefaultServer = "https://yakumo-api.mnicloud.jp"
	// DefaultIssuer is the identity provider of mNi Cloud.
	DefaultIssuer = "https://yakumo-auth.mnicloud.jp"
	// DefaultClientID is the public client the CLI is registered as.
	DefaultClientID = "mni-cli"
	// DefaultRedirectURI is the loopback URL the browser comes back to. Port 0
	// leaves the choice to the OS, which RFC 8252 §7.3 asks the authorization
	// server to allow for a native app.
	DefaultRedirectURI = "http://localhost:0/callback"
)

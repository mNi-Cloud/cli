package api

// Tenant is one tenant as the caller sees it.
type Tenant struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	Phase       string `json:"phase"`
	Role        string `json:"role"`
}

// NewTenant is the body POST /tenants takes. The owner is the caller, so the
// server fills it in.
type NewTenant struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

// Member is one member of a tenant, beside its owner.
type Member struct {
	User  string   `json:"user"`
	Roles []string `json:"roles"`
}

// NewMember is the body POST /tenants/{tenant}/members takes. The user is
// named by username, and the server resolves it to a user id.
type NewMember struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

// Identity is the caller behind the access token, as GET /me reports it.
type Identity struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Scopes   []string `json:"scopes"`
}

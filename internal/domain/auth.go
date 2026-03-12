package domain

import "context"

// AuthUser carries the authenticated caller's identity.
// Propagated via context by an auth middleware layer.
type AuthUser struct {
	ID    int64
	Roles []string
}

type ctxKeyAuthUser struct{}

// WithAuthUser returns a copy of ctx carrying u.
func WithAuthUser(ctx context.Context, u AuthUser) context.Context {
	return context.WithValue(ctx, ctxKeyAuthUser{}, u)
}

// AuthUserFromContext retrieves the AuthUser stored in ctx, if any.
func AuthUserFromContext(ctx context.Context) (AuthUser, bool) {
	u, ok := ctx.Value(ctxKeyAuthUser{}).(AuthUser)
	return u, ok
}

package rest

import "context"

// ctxKeyRequestID is the HTTP-layer context key for a request correlation ID.
// Defined in the rest adapter package — not in domain — because request IDs
// are an HTTP transport concern, not a business rule.
type ctxKeyRequestID struct{}

// WithRequestID returns a copy of ctx carrying the given request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID{}, id)
}

// RequestIDFromContext retrieves the request ID stored in ctx.
// Returns an empty string when no ID is present.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID{}).(string)
	return id
}

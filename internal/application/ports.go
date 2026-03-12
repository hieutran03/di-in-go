package application

import "context"

// ── Secondary ports — declared here, implemented by infrastructure ─────────────
//
// The application layer defines WHAT it needs; infrastructure decides HOW.
// This satisfies the Dependency Inversion Principle.

// Logger is the logging secondary port.
// With returns a new Logger with the given key-value pairs pre-attached,
// enabling per-request enrichment without storing state in the singleton.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
}

// Validator validates domain-level input coming from the outside world.
type Validator interface {
	ValidateCreateUser(name, email string) error
}

// EmailService is the notification secondary port.
type EmailService interface {
	SendWelcome(ctx context.Context, email, name string) error
}

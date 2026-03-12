// Package email provides a stub application.EmailService for local development.
// Replace with an SMTP/SES/SendGrid adapter in production.
package email

import (
	"context"

	"github.com/example/di_in_go/internal/application"
)

type smtpStub struct{ log application.Logger }

// NewStub returns a no-op email sender that logs instead of sending.
// Lifecycle: Singleton — stateless, safe to share across goroutines.
func NewStub(log application.Logger) application.EmailService {
	return &smtpStub{log: log}
}

func (s *smtpStub) SendWelcome(_ context.Context, emailAddr, name string) error {
	s.log.Info("[stub] sending welcome email", "to", emailAddr, "name", name)
	return nil
}

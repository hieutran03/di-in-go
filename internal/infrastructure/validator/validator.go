// Package validator provides a regex-backed application.Validator implementation.
package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/example/di_in_go/internal/application"
)

type regexValidator struct{ emailRe *regexp.Regexp }

// New returns an application.Validator.
// Lifecycle: Singleton — stateless, safe to share across goroutines.
func New() application.Validator {
	return &regexValidator{
		emailRe: regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`),
	}
}

func (v *regexValidator) ValidateCreateUser(name, email string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name is required")
	}
	if !v.emailRe.MatchString(email) {
		return fmt.Errorf("invalid email address")
	}
	return nil
}

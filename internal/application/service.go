package application

import (
	"context"
	"fmt"

	"github.com/example/di_in_go/internal/domain"
)

// UserService defines the available application use cases.
// Handlers depend on this interface, not the concrete struct — DIP satisfied.
type UserService interface {
	Create(ctx context.Context, req domain.CreateUserRequest) (domain.User, error)
	GetByID(ctx context.Context, id int64) (domain.User, error)
}

// userService is the concrete implementation.
// It only knows about domain types and application ports — never about HTTP or DB.
type userService struct {
	repo  domain.UserRepository
	email EmailService
	val   Validator
	log   Logger
}

// NewUserService constructs a UserService with all secondary ports injected.
// Every parameter is an interface, making this trivially testable.
func NewUserService(
	repo domain.UserRepository,
	email EmailService,
	val Validator,
	log Logger,
) UserService {
	return &userService{repo: repo, email: email, val: val, log: log}
}

func (s *userService) Create(ctx context.Context, req domain.CreateUserRequest) (domain.User, error) {
	if err := s.val.ValidateCreateUser(req.Name, req.Email); err != nil {
		return domain.User{}, fmt.Errorf("validation: %w", err)
	}
	u, err := s.repo.Create(ctx, domain.User{Name: req.Name, Email: req.Email})
	if err != nil {
		return domain.User{}, err
	}
	_ = s.email.SendWelcome(ctx, u.Email, u.Name) // best-effort side effect
	return u, nil
}

func (s *userService) GetByID(ctx context.Context, id int64) (domain.User, error) {
	return s.repo.GetByID(ctx, id)
}

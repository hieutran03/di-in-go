package domain

import (
	"context"
	"time"
)

// User is the core domain entity.
// It has no dependency on any framework or infrastructure concern.
type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUserRequest is the input value object for user creation.
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserRepository is the persistence port declared by the domain layer.
// The dependency rule: domain declares the interface; infrastructure implements it.
// No import from infrastructure ever appears here.
type UserRepository interface {
	Create(ctx context.Context, u User) (User, error)
	GetByID(ctx context.Context, id int64) (User, error)
}

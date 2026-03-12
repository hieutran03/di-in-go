// Package repository provides persistence adapters that implement domain.UserRepository.
package repository

import (
	"context"
	"fmt"

	"github.com/example/di_in_go/internal/application"
	"github.com/example/di_in_go/internal/domain"
	infradb "github.com/example/di_in_go/internal/infrastructure/db"
)

// memoryUserRepo is the standard (non-transactional) repository.
// Lifecycle: Singleton or Scoped depending on the DI approach used.
type memoryUserRepo struct {
	db  *infradb.MemoryDB
	log application.Logger
}

// NewMemory returns a domain.UserRepository backed by MemoryDB.
func NewMemory(d *infradb.MemoryDB, log application.Logger) domain.UserRepository {
	return &memoryUserRepo{db: d, log: log}
}

func (r *memoryUserRepo) Create(_ context.Context, u domain.User) (domain.User, error) {
	created, err := r.db.Insert(u)
	if err != nil {
		return domain.User{}, fmt.Errorf("repository.Create: %w", err)
	}
	r.log.Info("user created", "id", created.ID)
	return created, nil
}

func (r *memoryUserRepo) GetByID(_ context.Context, id int64) (domain.User, error) {
	u, ok := r.db.FindByID(id)
	if !ok {
		return domain.User{}, fmt.Errorf("user %d not found", id)
	}
	return u, nil
}

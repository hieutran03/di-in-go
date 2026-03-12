package application

import (
	"context"
	"fmt"

	"github.com/example/di_in_go/internal/domain"
)

// UnitOfWork is the application boundary for transactional grouping.
// Implementations produce domain.UserRepository instances that share
// the active domain.Tx stored in the provided context.
//
// This mirrors the Unit of Work pattern from DDD and maps to
// @Transactional (Spring) / TransactionScope (.NET) at the scope level.
type UnitOfWork interface {
	Users(ctx context.Context) domain.UserRepository
}

// WithTransaction begins a Tx via starter, injects it into ctx, calls fn,
// then commits on success or rolls back on any error.
//
// Go's equivalent of @Transactional — an explicit, composable scope boundary.
func WithTransaction(
	ctx context.Context,
	starter domain.TxStarter,
	fn func(ctx context.Context) error,
) error {
	tx, err := starter.BeginTx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	txCtx := domain.WithTx(ctx, tx)

	if fnErr := fn(txCtx); fnErr != nil {
		tx.Rollback()
		return fnErr
	}
	return tx.Commit()
}

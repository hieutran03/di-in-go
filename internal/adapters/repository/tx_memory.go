package repository

import (
	"context"
	"fmt"

	"github.com/example/di_in_go/internal/application"
	"github.com/example/di_in_go/internal/domain"
	infradb "github.com/example/di_in_go/internal/infrastructure/db"
)

// ── TxMemoryUoW ───────────────────────────────────────────────────────────────

// TxMemoryUoW implements application.UnitOfWork using MemoryDB.
// It is a Singleton factory; the repositories it produces are Scoped to a Tx.
type TxMemoryUoW struct {
	db  *infradb.MemoryDB
	log application.Logger
}

// NewTxMemoryUoW returns an application.UnitOfWork.
// Lifecycle: Singleton — the UoW itself is stateless; only produced repos are scoped.
func NewTxMemoryUoW(d *infradb.MemoryDB, log application.Logger) application.UnitOfWork {
	return &TxMemoryUoW{db: d, log: log}
}

// Users returns a domain.UserRepository scoped to the domain.Tx in ctx.
// Lifecycle: Scoped — the returned repo is valid only for the active transaction.
func (u *TxMemoryUoW) Users(ctx context.Context) domain.UserRepository {
	return &txUserRepo{db: u.db, log: u.log}
}

// ── txUserRepo ────────────────────────────────────────────────────────────────

// txUserRepo enforces that all writes occur inside an active domain.Tx.
// This mirrors @Transactional repositories in Spring — calling Create without
// a transaction in context returns an explicit error instead of silently succeeding.
type txUserRepo struct {
	db  *infradb.MemoryDB
	log application.Logger
}

func (r *txUserRepo) Create(ctx context.Context, u domain.User) (domain.User, error) {
	tx, ok := domain.TxFromContext(ctx)
	if !ok {
		return domain.User{}, fmt.Errorf("txUserRepo.Create: no active transaction in context — wrap call in application.WithTransaction")
	}

	// In production: pass tx.(*sql.Tx) to the actual INSERT query here.
	created, err := r.db.Insert(u)
	if err != nil {
		return domain.User{}, err
	}

	// Annotate the MemoryTx log for educational output.
	if memTx, ok := tx.(*infradb.MemoryTx); ok {
		memTx.Note(fmt.Sprintf("INSERT user id=%d", created.ID))
	}

	r.log.Info("user inserted in tx", "id", created.ID, "tx", txID(tx))
	return created, nil
}

func (r *txUserRepo) GetByID(_ context.Context, id int64) (domain.User, error) {
	u, ok := r.db.FindByID(id)
	if !ok {
		return domain.User{}, fmt.Errorf("user %d not found", id)
	}
	return u, nil
}

func txID(tx domain.Tx) string {
	if m, ok := tx.(*infradb.MemoryTx); ok {
		return m.ID()
	}
	return "unknown"
}

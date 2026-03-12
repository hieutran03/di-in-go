package domain

import "context"

// Tx represents an atomic unit of work at the domain boundary.
// The interface is intentionally minimal so non-SQL stores can implement it.
type Tx interface {
	Commit() error
	Rollback()
}

// TxStarter opens a new transaction.
// In production, *sql.DB implements this.  In tests, a fake can too.
type TxStarter interface {
	BeginTx() (Tx, error)
}

// ── context propagation ───────────────────────────────────────────────────────

type ctxKeyTx struct{}

// WithTx returns a copy of ctx carrying tx.
func WithTx(ctx context.Context, tx Tx) context.Context {
	return context.WithValue(ctx, ctxKeyTx{}, tx)
}

// TxFromContext retrieves the Tx stored in ctx, if any.
func TxFromContext(ctx context.Context) (Tx, bool) {
	tx, ok := ctx.Value(ctxKeyTx{}).(Tx)
	return tx, ok
}

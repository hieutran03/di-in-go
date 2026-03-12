// Package db provides an in-memory MemoryDB that implements domain.TxStarter.
// In production, replace MemoryDB with a wrapper around *sql.DB and MemoryTx
// with a wrapper around *sql.Tx.
package db

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/di_in_go/internal/domain"
)

// ── MemoryDB ──────────────────────────────────────────────────────────────────

// MemoryDB is a thread-safe in-memory store.
// It implements domain.TxStarter so it can be used with application.WithTransaction.
type MemoryDB struct {
	mu      sync.Mutex
	records map[int64]domain.User
	seq     atomic.Int64
	txSeq   atomic.Int64
}

func New() *MemoryDB { return &MemoryDB{records: make(map[int64]domain.User)} }

// Insert persists u and assigns a new sequential ID.
func (d *MemoryDB) Insert(u domain.User) (domain.User, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	u.ID = d.seq.Add(1)
	u.CreatedAt = time.Now().UTC()
	d.records[u.ID] = u
	return u, nil
}

// FindByID retrieves a user by ID.
func (d *MemoryDB) FindByID(id int64) (domain.User, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	u, ok := d.records[id]
	return u, ok
}

// BeginTx satisfies domain.TxStarter.
// In production: return db.sqlDB.BeginTx(ctx, nil) wrapped in a MemoryTx.
func (d *MemoryDB) BeginTx() (domain.Tx, error) {
	tx := &MemoryTx{id: fmt.Sprintf("tx-%d", d.txSeq.Add(1))}
	slog.Info("TX BEGIN", "tx", tx.id)
	return tx, nil
}

// ── MemoryTx ──────────────────────────────────────────────────────────────────

// MemoryTx simulates a database transaction.
// In production, wrap *sql.Tx here and delegate Commit/Rollback to it.
type MemoryTx struct {
	id  string
	ops []string
}

// ID returns the transaction identifier for logging.
func (t *MemoryTx) ID() string { return t.id }

// Note appends a human-readable operation description to the transaction log.
// Adapters may call this for educational output; production code would omit it.
func (t *MemoryTx) Note(op string) { t.ops = append(t.ops, op) }

func (t *MemoryTx) Commit() error {
	slog.Info("TX COMMIT", "tx", t.id, "ops", t.ops)
	return nil
}

func (t *MemoryTx) Rollback() {
	slog.Info("TX ROLLBACK", "tx", t.id)
}

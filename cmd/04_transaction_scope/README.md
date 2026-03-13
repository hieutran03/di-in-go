# Approach 4 — Transaction Scope / Unit of Work

`application.WithTransaction` starts a DB transaction, injects it into `context.Context`, runs a callback, then **commits on success or rolls back on any error**. Inside the callback, `UnitOfWork.Users(txCtx)` returns a repo that reads the active `Tx` from context — no explicit `*sql.Tx` parameter threading.

This is the Go equivalent of `@Transactional` (Spring) / `TransactionScope` (.NET).

---

## How It Works

```
txUserService.Create(ctx, req)
  │
  ├── validate(req)
  │
  ├── application.WithTransaction(ctx, db, func(txCtx) error {
  │       repo := uow.Users(txCtx)          ← repo is scoped to the active Tx
  │       user, err := repo.Create(txCtx, ...) ← writes inside the Tx
  │
  │       // Additional repos share the SAME Tx automatically:
  │       // profileRepo := uow.Profiles(txCtx)
  │
  │       return err
  │   })
  │   └── nil  → COMMIT
  │   └── err  → ROLLBACK
  │
  └── email.SendWelcome(ctx, ...)   ← post-commit side effect (outside Tx)
```

---

## Lifecycle

| Object | Lifetime | Notes |
|---|---|---|
| Logger, DB, Validator, EmailService | Singleton | Created once in `main` |
| UnitOfWork | Singleton | Factory that produces scoped repos |
| domain.Tx | **Scoped** | Alive for the duration of the `WithTransaction` callback |
| txUserRepo | **Scoped** | Produced by `uow.Users(txCtx)`; shares the active `Tx` |

---

## Unit of Work Pattern

```
UnitOfWork (Singleton)
  │
  ├── .Users(ctx)     → txUserRepo   (reads Tx from ctx)
  ├── .Profiles(ctx)  → txProfileRepo (same Tx from ctx)
  └── .Orders(ctx)    → txOrderRepo   (same Tx from ctx)
         ↑
         All share domain.TxFromContext(ctx) — one commit covers all writes
```

---

## When to Use

- Any **write path that spans multiple tables/repos** and must be atomic.
- When you want `@Transactional`-style guarantees without sprinkling `*sql.Tx` through every function signature.
- Ensures post-commit side effects (email, events) only fire after the DB write succeeds.

## Trade-offs

| Pro | Con |
|---|---|
| Automatic commit/rollback — no forgotten commits | More indirection than passing `*sql.Tx` directly |
| Multiple repos join the same Tx transparently | Context must carry the Tx correctly throughout |
| Post-commit effects are cleanly separated | Requires a `UnitOfWork` abstraction layer |

---

## Run

```bash
go run ./cmd/04_transaction_scope/
# Listens on :8083
```

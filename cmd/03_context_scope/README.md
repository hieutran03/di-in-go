# Approach 3 — Context-based Request Scope

All services and repositories are Singletons. Three middleware layers enrich `context.Context` before the handler runs. Handlers and services read per-request values directly from context — no extra parameters, no struct fields for request data.

---

## Middleware Chain

```
Incoming request
  │
  ▼
RequestIDMiddleware
  sets:  ctx = rest.WithRequestID(ctx, newUUID())
  reads: rest.RequestIDFromContext(ctx)
  │
  ▼
AuthMiddleware
  sets:  ctx = domain.WithAuthUser(ctx, parseToken(r))
  reads: domain.AuthUserFromContext(ctx)
  │
  ▼
TxMiddleware
  sets:  ctx = domain.WithTx(ctx, db.Begin())
  reads: domain.TxFromContext(ctx)
  commits or rolls back AFTER the handler returns
  │
  ▼
Handler (reads all three values from ctx, no parameter drilling)
```

---

## Lifecycle

| Object | Lifetime | Notes |
|---|---|---|
| Logger, DB, Validator, EmailService | Singleton | Shared across all requests |
| UserRepository, UserService, UserHandler | Singleton | Same — stateless, safe to share |
| RequestID | Scoped (context) | Unique string stored in `context.Context` for this request |
| AuthUser | Scoped (context) | Parsed from the request token, lives for the request |
| Tx | Scoped (context) | DB transaction; committed/rolled back by `TxMiddleware` after the handler |

---

## Request Flow

```
Incoming HTTP request
  → RequestIDMiddleware  → ctx now has: requestID
  → AuthMiddleware       → ctx now has: requestID, authUser
  → TxMiddleware         → ctx now has: requestID, authUser, tx
  → contextAwareHandler.Create(w, r)
      reqID := rest.RequestIDFromContext(r.Context())   // read scoped value
      log.With("request_id", reqID).Info(...)
      → inner UserHandler.Create
          → UserService.Create(ctx, req)                // svc is a singleton
  ← TxMiddleware.defer: commit or rollback tx
```

---

## When to Use

- When multiple orthogonal concerns (auth, tracing, transactions) must reach deep into the call stack without changing method signatures.
- Idiomatic Go pattern — works with `net/http` and any router (chi, gorilla, echo).
- When you want singletons but still need per-request identity/auth/transaction data available everywhere.

## Trade-offs

| Pro | Con |
|---|---|
| No parameter drilling for cross-cutting concerns | Context values are untyped — wrong key = silent nil |
| Standard Go pattern, no extra libraries | Order of middleware matters (auth before tx, etc.) |
| Singletons = low allocation overhead | Context must be threaded through every function |

---

## Run

```bash
go run ./cmd/03_context_scope/
# Listens on :8082
```

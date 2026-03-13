# Approach 7 — Middleware-created Request-Scoped Dependencies

`ScopeFactory` (Singleton) knows how to build a complete `RequestScope` object for each HTTP request. `ScopeMiddleware` is the scope boundary: it creates the scope, attaches it to `context.Context`, and defers `scope.Close()` so resources are released when the handler returns.

`ScopedUserHandler` holds **no service references** — it always reads the service out of the scope in context. This is the closest Go analog to `AddScoped` (ASP.NET Core) or `REQUEST` scope (NestJS/Nest DI).

---

## Architecture

```
┌─────────────────────────────────── Singleton lifetime ───────────────────────────────────┐
│  logger.New()   infradb.New()   validator.New()   email.NewStub(log)                    │
│                                                                                          │
│  ScopeFactory ─ holds: emailSvc, val, singleton log, repoConstructor func               │
│  ScopedUserHandler ─ holds: only singleton log (NO svc reference)                       │
└──────────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────── Per-request scope ────────────────────────────────────┐
│  RequestScope {                                                                          │
│    enrichedLog  = log.With("request_id", id)         ← scoped Logger                   │
│    repo         = NewMemory(db, enrichedLog)         ← scoped UserRepository            │
│    svc          = NewUserService(repo, email, val, enrichedLog) ← scoped UserService    │
│  }                                                                                       │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Request Flow

```
Incoming HTTP request
  │
  ▼
RequestIDMiddleware
  → injects RequestID string into ctx                 (Scoped value)
  │
  ▼
ScopeMiddleware
  → factory.NewScope(requestID, ctx)                  (creates RequestScope)
  →   builds: enrichedLog → repo → svc
  → ctx = rest.WithScope(ctx, scope)
  → defer scope.Close()                               (cleanup after handler)
  │
  ▼
ScopedUserHandler.Create(w, r)
  → scope := rest.ScopeFromContext(r.Context())
  → svc   := scope.UserService()                      (reads from scope, not a field)
  → svc.Create(r.Context(), req)
  │
  ▼
handler returns → ScopeMiddleware defer fires → scope.Close()
```

---

## Lifecycle

| Object | Lifetime | Notes |
|---|---|---|
| Logger, DB, Validator, EmailService | Singleton | Created once at startup |
| ScopeFactory | Singleton | Holds construction logic, creates scopes on demand |
| ScopedUserHandler | Singleton | Holds no service — reads from scope in context |
| RequestScope | **Scoped** | Created per request by `ScopeMiddleware`, closed after handler |
| Enriched Logger | **Scoped** | Tagged with `request_id` for this request only |
| UserRepository | **Scoped** | Built inside `RequestScope`, uses scoped logger |
| UserService | **Scoped** | Built inside `RequestScope`, uses scoped repo |

---

## When to Use

- When you want true **per-request scoping with automatic cleanup** analogous to `AddScoped`.
- When handlers should be fully **decoupled from construction logic** (testable with a mock scope).
- Ideal when scoped objects hold resources that must be released after each request (DB connections, tracing spans, per-request caches).

## Trade-offs

| Pro | Con |
|---|---|
| Cleanest separation between construction and use | More moving parts than simpler approaches |
| Handlers are trivially testable (inject a mock scope) | `ScopeFactory` and `RequestScope` types add boilerplate |
| Automatic resource cleanup via `defer scope.Close()` | Scope must be read from context — nil scope = runtime panic |
| Closest to framework-provided scoping (ASP.NET, NestJS) | Over-engineered for services without per-request resources |

---

## Run

```bash
go run ./cmd/07_middleware_scope/
# Listens on :8086
```

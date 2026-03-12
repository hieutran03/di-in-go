# Dependency Injection in Go — Clean Architecture

Seven DI approaches, all sharing the same **Clean Architecture** layer structure.
Only the wiring code in `cmd/` differs between approaches.

---

## Project Layout

```
di_in_go/
├── go.mod
│
├── internal/
│   ├── domain/                    ← Entities + primary ports (no internal imports)
│   │   ├── user.go                  User, CreateUserRequest, UserRepository interface
│   │   ├── tx.go                    Tx, TxStarter interfaces + context helpers
│   │   └── auth.go                  AuthUser + context helpers
│   │
│   ├── application/               ← Use cases + secondary ports
│   │   ├── ports.go                 Logger, Validator, EmailService interfaces
│   │   ├── service.go               UserService interface + implementation
│   │   └── uow.go                   UnitOfWork interface + WithTransaction helper
│   │
│   ├── adapters/                  ← Interface adapters (depends on domain + application)
│   │   ├── rest/
│   │   │   ├── handler.go           UserHandler (struct-based, uses application.UserService)
│   │   │   ├── middleware.go        RequestID, Auth, Tx middleware + Chain helper
│   │   │   ├── scope.go             RequestScope, ScopeFactory, ScopeMiddleware, ScopedUserHandler
│   │   │   ├── context.go           HTTP-layer request ID context key
│   │   │   └── response.go          WriteJSON / ReadJSON helpers
│   │   └── repository/
│   │       ├── memory.go            memoryUserRepo  implements domain.UserRepository
│   │       └── tx_memory.go         TxMemoryUoW + txUserRepo (transaction-aware)
│   │
│   └── infrastructure/            ← Frameworks & drivers (depends on domain + application)
│       ├── db/memory.go             MemoryDB + MemoryTx  (implements domain.TxStarter)
│       ├── logger/slog.go           slogLogger           (implements application.Logger)
│       ├── validator/validator.go   regexValidator       (implements application.Validator)
│       ├── email/stub.go            smtpStub             (implements application.EmailService)
│       └── container/container.go  Reflection-based DI container (approach 06 only)
│
└── cmd/                           ← Composition roots — ONLY wiring lives here
    ├── 01_manual/main.go            :8080
    ├── 02_function_scope/main.go    :8081
    ├── 03_context_scope/main.go     :8082
    ├── 04_transaction_scope/main.go :8083
    ├── 05_wire_di/
    │   ├── providers.go             ProvideXxx constructor functions
    │   ├── wire_gen.go              Simulated generated InitApp() wiring
    │   └── main.go                  :8084
    ├── 06_runtime_container/main.go :8085
    └── 07_middleware_scope/main.go  :8086
```

---

## Dependency Rule (Clean Architecture)

```
         ┌──────────────────────────────────────────────┐
         │            infrastructure/                    │  db, logger, email, validator
         │   ┌──────────────────────────────────────┐   │
         │   │           adapters/                   │   │  rest handlers, repositories
         │   │   ┌──────────────────────────────┐   │   │
         │   │   │        application/           │   │   │  use cases, ports (interfaces)
         │   │   │   ┌──────────────────────┐   │   │   │
         │   │   │   │      domain/          │   │   │   │  entities, repository interface
         │   │   │   └──────────────────────┘   │   │   │
         │   │   └──────────────────────────────┘   │   │
         │   └──────────────────────────────────────┘   │
         └──────────────────────────────────────────────┘
         Arrows point INWARD only.  Inner layers are unaware of outer layers.
```

| Layer            | Imports from                    | Never imports            |
|-----------------|--------------------------------|--------------------------|
| `domain`         | stdlib only                     | anything internal        |
| `application`    | `domain`                        | adapters, infrastructure |
| `adapters`       | `domain`, `application`, infra  | cmd                      |
| `infrastructure` | `domain`, `application`         | adapters, cmd            |
| `cmd`            | everything                      | other cmd packages       |

---

## Quick Start

```bash
go run ./cmd/01_manual/

curl -s -X POST localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com"}' | jq .

curl -s localhost:8080/users/1 | jq .
```

---

## Lifecycle Vocabulary

| Term          | Meaning                                   | Go mechanism                                   |
|-------------|------------------------------------------|------------------------------------------------|
| **Singleton** | One instance for the process lifetime   | Created in `cmd/*/main.go`, injected everywhere |
| **Scoped**    | One instance per HTTP request           | `context.Context` or `RequestScope`            |
| **Transient** | New instance on every use               | `NewXxx()` called per request / per call       |

---

## Approach 1 — Manual Constructor Injection  `cmd/01_manual/`

All dependencies injected via constructor functions. `main()` controls creation order.
The entire wiring fits in ~15 lines.

```go
// cmd/01_manual/main.go
log      := logger.New()                                      // Singleton
db       := infradb.New()                                     // Singleton
val      := validator.New()                                   // Singleton
emailSvc := email.NewStub(log)                                // Singleton
repo     := repository.NewMemory(db, log)                     // Singleton
svc      := application.NewUserService(repo, emailSvc, val, log) // Singleton
h        := rest.NewUserHandler(svc, log)                     // Singleton

// RequestIDMiddleware generates one ID per request → injects into ctx (Scoped)
srv := &http.Server{Handler: rest.Chain(mux, rest.RequestIDMiddleware)}
```

**Lifecycle table**

| Object           | Lifecycle  | Location         |
|-----------------|-----------|------------------|
| DB, Logger, …   | Singleton | `main()`         |
| RequestID       | Scoped    | Middleware → ctx |

---

## Approach 2 — Function Scope Injection  `cmd/02_function_scope/`

Handler factories close over singletons.  `UserRepository` and `UserService` are
**Transient** — constructed fresh inside each handler call, discarded after response.

```go
// cmd/02_function_scope/main.go
func makeCreateHandler(db *infradb.MemoryDB, log application.Logger, ...) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Transient: new per request, eligible for GC after handler returns
        repo := repository.NewMemory(db, log)
        svc  := application.NewUserService(repo, emailSvc, val, log)

        svc.Create(r.Context(), req)
    }
}
```

**Lifecycle table**

| Object           | Lifecycle  | Mechanism                 |
|-----------------|-----------|---------------------------|
| DB, Logger, …   | Singleton | Closed over by factory    |
| UserRepository  | Transient | `NewMemory()` per request |
| UserService     | Transient | `NewUserService()` per request |

---

## Approach 3 — Context-based Request Scope  `cmd/03_context_scope/`

Three middleware layers compose the request scope by writing typed values into
`context.Context`.  Services are Singletons; the **values** they consume are scoped.

```
rest.RequestIDMiddleware  →  rest.WithRequestID(ctx, id)
rest.AuthMiddleware        →  domain.WithAuthUser(ctx, user)
rest.TxMiddleware          →  domain.WithTx(ctx, tx)  [auto-commit after handler]
```

Reading scoped values anywhere in the call tree, no parameter drilling:
```go
reqID     := rest.RequestIDFromContext(ctx)
user, _   := domain.AuthUserFromContext(ctx)
tx, _     := domain.TxFromContext(ctx)

enriched  := log.With("request_id", reqID, "user_id", user.ID) // no singleton mutation
```

**Lifecycle table**

| Value          | Lifecycle  | Set by                  |
|---------------|-----------|-------------------------|
| RequestID     | Scoped    | `RequestIDMiddleware`   |
| AuthUser      | Scoped    | `AuthMiddleware`        |
| Tx            | Scoped    | `TxMiddleware`          |
| All services  | Singleton | `main()`                |

---

## Approach 4 — Transaction Scope / Unit of Work  `cmd/04_transaction_scope/`

`application.WithTransaction` is the formal transaction scope boundary — the Go
equivalent of `@Transactional` (Spring) / `TransactionScope` (.NET).

```go
// application/uow.go — the scope boundary
func WithTransaction(ctx context.Context, starter domain.TxStarter,
    fn func(context.Context) error) error {

    tx, _  := starter.BeginTx()
    txCtx  := domain.WithTx(ctx, tx)   // inject Tx into scope

    if err := fn(txCtx); err != nil {
        tx.Rollback()                   // scope ends on error: rollback
        return err
    }
    return tx.Commit()                  // scope ends on success: commit
}

// Usage inside txUserService.Create:
application.WithTransaction(ctx, db, func(txCtx context.Context) error {
    repo    := uow.Users(txCtx)                        // Scoped repo — shares the Tx
    created, err = repo.Create(txCtx, user)
    return err
})
// Email sent AFTER commit — outside the transaction scope.
```

`txUserRepo` (`adapters/repository/tx_memory.go`) returns an explicit error if
called without an active transaction — no silent data loss.

**Lifecycle table**

| Object          | Lifecycle  | Mechanism                            |
|----------------|-----------|--------------------------------------|
| DB, UoW, …     | Singleton | `main()`                             |
| domain.Tx      | Scoped    | Alive for `WithTransaction` callback |
| txUserRepo     | Scoped    | Produced by UoW inside the callback  |

---

## Approach 5 — Compile-time DI (Google Wire style)  `cmd/05_wire_di/`

You write provider functions; the `wire` CLI writes `InitApp()`.
No reflection — errors in the dependency graph are detected at code-generation time.

**You write (`providers.go`)**
```go
func ProvideLogger() application.Logger                           { return logger.New() }
func ProvideDB() *infradb.MemoryDB                                { return infradb.New() }
func ProvideEmailService(log application.Logger) application.EmailService {
    return email.NewStub(log)
}
func ProvideUserRepository(db *infradb.MemoryDB,
    log application.Logger) domain.UserRepository {
    return repository.NewMemory(db, log)
}
func ProvideUserService(repo domain.UserRepository, ...) application.UserService { ... }
func ProvideUserHandler(svc application.UserService, ...) *rest.UserHandler { ... }
```

**Wire generates (`wire_gen.go`)**
```go
// Code generated by Wire. DO NOT EDIT.
func InitApp() *App {
    log      := logger.New()
    db       := infradb.New()
    emailSvc := email.NewStub(log)
    repo     := repository.NewMemory(db, log)
    svc      := application.NewUserService(repo, emailSvc, val, log)
    h        := rest.NewUserHandler(svc, log)
    return NewApp(h)
}
```

Wire calls each provider exactly once per `InitApp()` — all objects are Singletons.
Scoped objects must still be modelled via `context.Context` (approach 3).

---

## Approach 6 — Runtime DI Container (Uber Dig / Fx style)  `cmd/06_runtime_container/`

`infrastructure/container/container.go` provides a ~100-line reflection-based
container that mirrors `go.uber.org/dig`'s `Provide` / `Invoke` / lifecycle API.

```go
c := container.New()

// Registration — order independent; graph resolved lazily on Invoke/Start.
c.Provide(logger.New)            // () → application.Logger
c.Provide(infradb.New)           // () → *infradb.MemoryDB
c.Provide(validator.New)         // () → application.Validator
c.Provide(email.NewStub)         // (application.Logger) → application.EmailService
c.Provide(provideUserService)    // (...) → application.UserService
c.Provide(rest.NewUserHandler)   // (...) → *rest.UserHandler
c.Provide(newServer)             // (*rest.UserHandler) → *Server

// Lifecycle hooks — mirrors go.uber.org/fx Lifecycle
c.OnStart(func(_ context.Context) error {
    return c.Invoke(func(s *Server, log application.Logger) error {
        go s.srv.ListenAndServe()
        return nil
    })
})
c.OnStop(func(ctx context.Context) error {
    return c.Invoke(func(s *Server) error { return s.srv.Shutdown(ctx) })
})

// Resolves the graph and runs OnStart hooks.
// Each constructor is called at most once → Singleton cache.
c.Start(context.Background())
```

The container caches resolved values by type — all registered objects are treated
as Singletons.  Scoped behaviour requires a child container per request.

---

## Approach 7 — Middleware-created Request-Scoped Dependencies  `cmd/07_middleware_scope/`

`ScopeFactory` (Singleton) produces a `RequestScope` on every request.
`ScopeMiddleware` is the scope boundary — the direct Go analog of `AddScoped`.

```
Singleton layer (main)
  DB · Logger · Validator · EmailService
  ScopeFactory ───── creates per request ──────►  RequestScope  (Scoped)
                                                    ├── enrichedLogger  ← log.With(reqID)
                                                    ├── UserRepository  ← NewMemory(db, enrichedLog)
                                                    └── UserService     ← NewUserService(...)
```

```go
// adapters/rest/scope.go

func (f *ScopeFactory) NewScope(reqID string) *RequestScope {
    scopedLog := f.log.With("request_id", reqID)                       // scoped, no singleton mutation
    repo      := f.newRepo(scopedLog)                                   // scoped repository
    svc       := application.NewUserService(repo, f.email, f.val, scopedLog) // scoped service
    return &RequestScope{UserService: svc, RequestID: reqID, ...}
}

func ScopeMiddleware(factory *ScopeFactory, log application.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            scope := factory.NewScope(RequestIDFromContext(r.Context()))  // Scoped
            ctx   := WithScope(r.Context(), scope)

            defer func() { /* scope.Dispose(): log duration, run cleanups */ }()

            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

`ScopedUserHandler` stores **no** service references — reads from the scope at call time:
```go
func (h *ScopedUserHandler) Create(w http.ResponseWriter, r *http.Request) {
    scope := MustScope(r.Context())
    u, err := scope.UserService.Create(r.Context(), req)
    ...
}
```

Testing — inject a fake scope, zero mocking frameworks needed:
```go
fakeScope := &rest.RequestScope{UserService: &stubUserService{}}
ctx := rest.WithScope(req.Context(), fakeScope)
handler.Create(w, req.WithContext(ctx))
```

**Lifecycle table**

| Object           | Lifecycle  | Mechanism                     |
|-----------------|-----------|-------------------------------|
| DB, Logger, …   | Singleton | `main()`                      |
| ScopeFactory    | Singleton | `main()`                      |
| enrichedLogger  | Scoped    | `ScopeFactory.NewScope()`     |
| UserRepository  | Scoped    | `ScopeFactory.NewScope()`     |
| UserService     | Scoped    | `ScopeFactory.NewScope()`     |

---

## Final Comparison Table

| Criterion                    | 01 Manual  | 02 Function | 03 Context  | 04 Tx Scope | 05 Wire    | 06 Container | 07 Middleware |
|-----------------------------|-----------|------------|------------|------------|-----------|-------------|--------------|
| **Complexity**               | Low       | Low        | Medium     | Medium     | Medium    | High        | Medium       |
| **Readability**              | ★★★★★    | ★★★★☆      | ★★★★☆     | ★★★★☆     | ★★★☆☆     | ★★★☆☆      | ★★★★★        |
| **Runtime overhead**         | Zero      | Zero       | Negligible | Negligible | Zero      | Startup     | Per-req alloc|
| **Testability**              | Excellent | Excellent  | Good       | Good       | Excellent | Good        | Excellent    |
| **Singleton support**        | ✓ Explicit| ✓ Closure  | ✓ Explicit | ✓ Explicit | ✓ Generated| ✓ Cache    | ✓ Explicit   |
| **Scoped support**           | Via ctx   | Via closure| ✓ Native   | ✓ Native   | Via ctx   | Sub-container| ✓ Native   |
| **Transient support**        | Manual new| ✓ Native   | Manual new | Manual new | Factory fn| Factory fn  | Manual new   |
| **Lifecycle hooks**          | Manual    | Manual     | Defer      | Defer/cb   | Manual    | ✓ Built-in  | ✓ defer/mw   |
| **TX scope**                 | Mediocre  | Mediocre   | Good       | ✓ Excellent| Mediocre  | Good        | Good         |
| **Compile-time safety**      | ✓ Full    | ✓ Full     | Partial*   | ✓ Full     | ✓ Full    | ✗ Runtime   | Partial*     |
| **Scalability (50+ types)**  | Tedious   | Very tedious| Good      | Good       | ✓ Excellent| ✓ Excellent| Good         |
| **External tools**           | None      | None       | None       | None       | `wire` CLI| None†       | None         |
| **Best for**                 | Small APIs| Stateless λ| Cross-cutting| DB-heavy | Large apps| Plugin/DI   | REST APIs    |

\* `TxFromContext`, `ScopeFromContext` type-assert at runtime — add integration tests.
† Replace `infrastructure/container` with `go.uber.org/dig` for production.

---

## Clean Architecture Benefit

Because all approaches share the same `internal/` packages, you can change DI approach
by modifying only `cmd/*/main.go`.  Domain, application, and adapter code never changes.

| What changes         | What stays the same                    |
|---------------------|----------------------------------------|
| `cmd/*/main.go`     | `internal/domain/` — entities, interfaces |
|                     | `internal/application/` — use cases    |
|                     | `internal/adapters/` — HTTP, repos     |
|                     | `internal/infrastructure/` — DB, email |

---

## Bonus: Mapping Go Patterns to DI Scopes

### Singleton — one instance, process lifetime
```go
log  := logger.New()                   // application.Logger — shared singleton
repo := repository.NewMemory(db, log)  // shared, injected into service constructor
```

### Scoped — one instance per request

**A) context.Context** (approach 3)
```go
ctx = rest.WithRequestID(ctx, "req-42")  // set once in middleware
id  = rest.RequestIDFromContext(ctx)     // read anywhere downstream, no drilling
```

**B) RequestScope in context** (approach 7)
```go
scope = factory.NewScope(reqID)          // new scoped object graph per request
ctx   = rest.WithScope(ctx, scope)
// Anywhere downstream: rest.MustScope(ctx).UserService.Create(...)
```

**C) WithTransaction callback** (approach 4)
```go
application.WithTransaction(ctx, db, func(txCtx context.Context) error {
    // This callback IS the scope.
    // Every repo via uow.Users(txCtx) shares the same *Tx.
    return repo.Create(txCtx, user)
})
```

### Transient — new instance every time
```go
// A) Function scope: value struct per request
repo := repository.NewMemory(db, log)      // new allocation per handler call

// B) Container: register a factory, not a value
c.Provide(func() *WorkBuffer { return &WorkBuffer{} })
// Container calls the factory on every Invoke — result is NOT cached
```

---

## Decision Guide

```
Dependency stateless and shared across all requests?
    └─ Singleton. Create in main(), inject via interface.

Need per-request isolation (enriched logger, auth context, tx)?
    ├─ Simple scalar values   → context.Context (approach 3)
    └─ Full object graphs     → ScopeMiddleware  (approach 7)

Multiple repositories must share one DB transaction?
    └─ WithTransaction + UnitOfWork (approach 4)

Codebase growing large (30+ injected types)?
    ├─ Compile-time safety preferred  → Wire  (approach 5)
    └─ Runtime flexibility / plugins  → Dig/Fx (approach 6)

Purely functional / stateless handlers?
    └─ Function scope injection (approach 2)

New project / small team unfamiliar with DI frameworks?
    └─ Manual constructor injection (approach 1) — readable by anyone.
```

# Approach 5 — Compile-time DI (Google Wire)

You write **provider functions** — pure constructors that declare their inputs and outputs. The Wire CLI reads those signatures, resolves the dependency graph at **compile time**, and emits `wire_gen.go` with a single `InitApp()` function. No reflection at runtime; the generated code is plain Go.

---

## Workflow

```
providers.go              wire CLI             wire_gen.go
──────────────            ─────────            ───────────
ProvideLogger()     ──►   resolves   ──►   func InitApp() *App {
ProvideDB()               graph              log     := logger.New()
ProvideValidator()        topologically      db      := infradb.New()
ProvideEmailService()     sorts              val     := validator.New()
ProvideUserRepository()   providers          emailSvc := email.NewStub(log)
ProvideUserService()                         repo    := repository.NewMemory(db, log)
ProvideUserHandler()                         svc     := application.NewUserService(...)
NewApp()                                     h       := rest.NewUserHandler(svc, log)
                                             return NewApp(h)
                                           }
```

---

## Provider Graph

```
ProvideLogger()                         → application.Logger
ProvideDB()                             → *infradb.MemoryDB
ProvideValidator()                      → application.Validator
ProvideEmailService(Logger)             → application.EmailService
ProvideUserRepository(MemoryDB, Logger) → domain.UserRepository
ProvideUserService(UserRepository, ...) → application.UserService
ProvideUserHandler(UserService, Logger) → *rest.UserHandler
NewApp(UserHandler)                     → *App
```

Wire matches each **return type** to the matching **parameter type** of the next provider. If a provider requires a type that no other provider returns, Wire reports a **compile-time error**.

---

## Lifecycle

| Object | Lifetime | Notes |
|---|---|---|
| All objects | Singleton | Each provider called exactly once by `InitApp()` |
| RequestID | Scoped | Middleware, same as other approaches |

---

## Setup

```bash
# Install the Wire CLI (one-time)
go install github.com/google/wire/cmd/wire@latest

# Regenerate wire_gen.go after changing providers
wire ./cmd/05_wire_di/

# Run without regenerating (wire_gen.go is committed)
go run ./cmd/05_wire_di/
```

---

## Files

| File | Purpose |
|---|---|
| `providers.go` | Provider functions — **you write these** |
| `wire_gen.go` | `InitApp()` — **Wire generates this; do not edit** |
| `main.go` | `App` struct, `NewApp`, and `main()` entry point |

---

## When to Use

- Medium-to-large services where manual wiring becomes error-prone.
- Teams that want all DI errors caught at compile time rather than runtime.
- Codebases that already commit generated code (protobuf, sqlc, etc.) and accept that pattern.

## Trade-offs

| Pro | Con |
|---|---|
| All wiring errors are compile-time | Requires a CLI code-generation step |
| Generated code is plain Go — no runtime overhead | `wire_gen.go` must be regenerated when providers change |
| Provider functions are easy to unit test | Adds a build tool dependency |

---

## Run

```bash
go run ./cmd/05_wire_di/
# Listens on :8084
```

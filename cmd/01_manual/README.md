# Approach 1 — Manual Constructor Injection

The simplest and most explicit wiring strategy. Every dependency is passed via constructor arguments; `main()` controls the wiring order. There is no framework, no reflection, and no generated code.

---

## Dependency Graph

```
logger.New()
infradb.New()
validator.New()
email.NewStub(log)
     │
     ├── repository.NewMemory(db, log)
     │        │
     │        └── application.NewUserService(repo, emailSvc, val, log)
     │                  │
     │                  └── rest.NewUserHandler(svc, log)
     │                              │
     │                              └── http.ServeMux → http.Server
```

Each constructor receives only the interfaces it declares — dependencies flow strictly top-down.

---

## Lifecycle

| Object | Lifetime | Notes |
|---|---|---|
| Logger, DB, Validator, EmailService | Singleton | Created once in `main`, shared for the process lifetime |
| UserRepository, UserService, UserHandler | Singleton | Same — created once and reused across all requests |
| RequestID | Scoped | Generated fresh per request by `RequestIDMiddleware` |

---

## Request Flow

```
Incoming HTTP request
  → RequestIDMiddleware  attaches a unique ID to context
  → UserHandler.Create / UserHandler.Get
      → UserService (singleton)
          → UserRepository (singleton)
              → MemoryDB (singleton)
```

---

## When to Use

- Small services, CLIs, or lambdas where the full dependency set is known at compile time.
- Teams that prefer zero-magic, fully traceable wiring.
- Early stages of a project before the graph grows large.

## Trade-offs

| Pro | Con |
|---|---|
| Zero dependencies, no tooling needed | Constructor argument lists grow with the graph |
| Errors are compile-time (wrong type = won't build) | Wiring order must be maintained manually |
| Dead-simple to read and debug | Refactoring a dependency requires touching `main` |

---

## Run

```bash
go run ./cmd/01_manual/
# Listens on :8080
```

# Approach 2 — Function Scope Injection

Handler factories close over singleton infrastructure objects. On every HTTP request the factory's returned `http.HandlerFunc` allocates a **fresh** `UserRepository` and `UserService` — then discards them when the request finishes. The factory function itself is the composition root.

---

## Dependency Graph

```
main() — Singletons closed over by each factory
  ├── DB          ─┐
  ├── Logger       ├──► makeCreateHandler(...) → http.HandlerFunc
  ├── Validator    │         on each request:
  └── EmailService ┘           repo := NewMemory(db, log)    ← Transient
                               svc  := NewUserService(repo, ...)  ← Transient
```

---

## Lifecycle

| Object | Lifetime | Notes |
|---|---|---|
| Logger, DB, Validator, EmailService | Singleton | Created once in `main`, closed over by handler factories |
| UserRepository, UserService | **Transient** | Allocated fresh on every HTTP request, GC'd after it returns |
| RequestID | Scoped | Injected per request by `RequestIDMiddleware` |

---

## Request Flow

```
Incoming HTTP request
  → RequestIDMiddleware  attaches a unique ID to context
  → makeCreateHandler closure runs
      repo := repository.NewMemory(db, log)           ← new allocation
      svc  := application.NewUserService(repo, ...)   ← new allocation
      svc.Create(r.Context(), req)
      ← repo and svc go out of scope, eligible for GC
```

---

## When to Use

- When services hold mutable per-request state that must not leak across requests.
- Stateful services (e.g. per-request caches, per-request tracing context) where shared singletons are unsafe.
- When you want strict isolation between concurrent requests without locks.

## Trade-offs

| Pro | Con |
|---|---|
| Complete request isolation — no shared mutable state | More heap allocations per request |
| No synchronisation needed on service/repo objects | Higher GC pressure under load |
| Easy to test each handler closure independently | Singletons still need to be thread-safe (DB, Logger, etc.) |

---

## Run

```bash
go run ./cmd/02_function_scope/
# Listens on :8081
```

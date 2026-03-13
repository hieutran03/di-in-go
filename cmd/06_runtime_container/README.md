# Approach 6 — Runtime DI Container (Uber Dig / Fx style)

A runtime container where provider constructors are registered in **any order**. The container resolves the dependency graph lazily when `Invoke` is called, caches all instances (Singleton), and manages startup/shutdown via lifecycle hooks. This mirrors the API of `go.uber.org/dig` and `go.uber.org/fx`.

---

## How It Works

```
c := container.New()

── Registration phase (no execution yet) ──────────────────────────────────
c.Provide(logger.New)           ()                         → application.Logger
c.Provide(infradb.New)          ()                         → *infradb.MemoryDB
c.Provide(validator.New)        ()                         → application.Validator
c.Provide(email.NewStub)        (application.Logger)       → application.EmailService
c.Provide(provideUserService)   (MemoryDB, EmailSvc, ...)  → application.UserService
c.Provide(rest.NewUserHandler)  (UserService, Logger)      → *rest.UserHandler
c.Provide(newServer)            (*rest.UserHandler)        → *Server

── Lifecycle hooks ─────────────────────────────────────────────────────────
c.OnStart(func(ctx) { c.Invoke(func(s *Server, log Logger) { go s.srv.ListenAndServe() }) })
c.OnStop (func(ctx) { c.Invoke(func(s *Server)             {    s.srv.Shutdown(ctx)    }) })

── Start ────────────────────────────────────────────────────────────────────
c.Start(ctx)
  └── container walks graph, calls each provider once, caches results
  └── fires OnStart hooks in registration order
```

---

## Lifecycle

| Object | Lifetime | Notes |
|---|---|---|
| All registered types | Singleton | Container caches the resolved instance by type after first `Invoke` |
| Scoped | (not shown) | Achievable via a child container created per request |
| Transient | (not shown) | Provider returns `func() T`; caller invokes the factory each time |

---

## Startup / Shutdown Sequence

```
c.Start(ctx)
  1. Resolves full dependency graph (providers called once, results cached)
  2. Fires OnStart hooks in order → server begins listening

SIGINT / SIGTERM received

c.Stop(ctx)
  1. Fires OnStop hooks in reverse order → server shuts down gracefully
```

---

## Comparison with Wire (Approach 5)

| | Wire (compile-time) | Runtime Container |
|---|---|---|
| Graph resolution | At code generation time | At first `Invoke` call |
| Wiring errors | Compile-time | Runtime panic on first use |
| Tooling required | Wire CLI | None |
| Lifecycle hooks | Not built-in | Built-in (`OnStart`/`OnStop`) |
| Registration order | Must follow type dependencies | Any order |

---

## When to Use

- Large services with many providers where compile-time wiring is too verbose.
- When you need built-in lifecycle management (ordered start/stop).
- When registration order flexibility matters (e.g. plugin-style provider registration).

## Trade-offs

| Pro | Con |
|---|---|
| Order-independent provider registration | Graph errors surface at runtime, not compile time |
| Built-in lifecycle hooks for clean startup/shutdown | Slightly more runtime overhead than generated code |
| Familiar pattern if coming from Spring / NestJS | Harder to trace wiring without a tool |

---

## Run

```bash
go run ./cmd/06_runtime_container/
# Listens on :8085
```

package rest

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/example/di_in_go/internal/application"
	"github.com/example/di_in_go/internal/domain"
)

// ── RequestScope ─────────────────────────────────────────────────────────────
//
// RequestScope bundles all per-request scoped services into a single value.
// Its lifetime equals exactly one HTTP request — analogous to an ASP.NET
// IServiceScope or a NestJS REQUEST-scoped provider.

type RequestScope struct {
	UserService application.UserService
	RequestID   string
	StartedAt   time.Time
}

type ctxKeyScope struct{}

// WithScope returns a copy of ctx carrying s.
func WithScope(ctx context.Context, s *RequestScope) context.Context {
	return context.WithValue(ctx, ctxKeyScope{}, s)
}

// ScopeFromContext retrieves the RequestScope stored in ctx, if any.
func ScopeFromContext(ctx context.Context) (*RequestScope, bool) {
	s, ok := ctx.Value(ctxKeyScope{}).(*RequestScope)
	return s, ok
}

// MustScope retrieves the RequestScope or panics with a clear diagnostic.
// Analogous to GetRequiredService in ASP.NET.
func MustScope(ctx context.Context) *RequestScope {
	s, ok := ScopeFromContext(ctx)
	if !ok {
		panic("rest: no RequestScope in context — ensure ScopeMiddleware is wired")
	}
	return s
}

// ── ScopedUserHandler ─────────────────────────────────────────────────────────
//
// ScopedUserHandler holds NO service references directly.
// It reads the scoped UserService from the RequestScope at call time,
// making it trivially testable by injecting a fake scope via context.

type ScopedUserHandler struct{ log application.Logger }

func NewScopedUserHandler(log application.Logger) *ScopedUserHandler {
	return &ScopedUserHandler{log: log}
}

func (h *ScopedUserHandler) Create(w http.ResponseWriter, r *http.Request) {
	scope := MustScope(r.Context())

	var req domain.CreateUserRequest
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	u, err := scope.UserService.Create(r.Context(), req)
	if err != nil {
		h.log.Error("create user", "err", err, "request_id", scope.RequestID)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (h *ScopedUserHandler) Get(w http.ResponseWriter, r *http.Request) {
	scope := MustScope(r.Context())

	var id int64
	fmt.Sscanf(r.PathValue("id"), "%d", &id)

	u, err := scope.UserService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// ── ScopeFactory ──────────────────────────────────────────────────────────────
//
// ScopeFactory is a Singleton.  Its NewScope method is called once per request
// (inside ScopeMiddleware) to produce fresh Scoped service instances.
//
// It captures only singleton dependencies so requests stay isolated.
// The RepoConstructor delegate keeps ScopeFactory decoupled from concrete
// repository and infrastructure packages.

// RepoConstructor is a factory function that creates a domain.UserRepository
// with the given per-request scoped logger.
type RepoConstructor func(log application.Logger) domain.UserRepository

// ScopeFactory creates RequestScopes.
type ScopeFactory struct {
	newRepo RepoConstructor
	email   application.EmailService
	val     application.Validator
	log     application.Logger
}

func NewScopeFactory(
	newRepo RepoConstructor,
	email application.EmailService,
	val application.Validator,
	log application.Logger,
) *ScopeFactory {
	return &ScopeFactory{newRepo: newRepo, email: email, val: val, log: log}
}

// NewScope builds a fresh RequestScope for one HTTP request.
// Each call creates new scoped objects enriched with reqID.
func (f *ScopeFactory) NewScope(reqID string) *RequestScope {
	scopedLog := f.log.With("request_id", reqID)                       // scoped logger
	repo := f.newRepo(scopedLog)                                       // scoped repository
	svc := application.NewUserService(repo, f.email, f.val, scopedLog) // scoped service
	return &RequestScope{
		UserService: svc,
		RequestID:   reqID,
		StartedAt:   time.Now(),
	}
}

// ── ScopeMiddleware ───────────────────────────────────────────────────────────
//
// ScopeMiddleware is the scope boundary — the Go analog of AddScoped in ASP.NET.
// It creates a RequestScope for every HTTP request, attaches it to context,
// and disposes it (via defer) once the handler returns.

func ScopeMiddleware(factory *ScopeFactory, log application.Logger) func(http.Handler) http.Handler {
	var counter atomic.Int64
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := RequestIDFromContext(r.Context())
			if reqID == "" {
				reqID = fmt.Sprintf("req-%d", counter.Add(1))
			}

			scope := factory.NewScope(reqID)
			ctx := WithScope(r.Context(), scope)

			// defer = scope.Dispose(): log duration, run cleanup hooks.
			defer func() {
				log.Info("scope disposed",
					"request_id", reqID,
					"duration_ms", time.Since(scope.StartedAt).Milliseconds(),
				)
			}()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

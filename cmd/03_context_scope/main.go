// Approach 3 — Context-based Request Scope
//
// Three middleware layers compose the request scope.  Each layer reads or
// writes a typed value in context.Context:
//
//	requestIDMiddleware → rest.WithRequestID  → rest.RequestIDFromContext
//	authMiddleware      → domain.WithAuthUser → domain.AuthUserFromContext
//	txMiddleware        → domain.WithTx       → domain.TxFromContext
//
// Services and repositories are Singletons; the values they read from context
// carry the per-request scope.  The logger is enriched per-call from context.
//
// Lifecycle:
//
//	Singleton  → all services and repositories
//	Scoped     → RequestID, AuthUser, Tx (all stored in context)
//	Transient  → none
//
// See README.md for per-request flow and when to use.
//
// Run: go run ./cmd/03_context_scope/
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/di_in_go/internal/adapters/repository"
	"github.com/example/di_in_go/internal/adapters/rest"
	"github.com/example/di_in_go/internal/application"
	infradb "github.com/example/di_in_go/internal/infrastructure/db"
	"github.com/example/di_in_go/internal/infrastructure/email"
	"github.com/example/di_in_go/internal/infrastructure/logger"
	"github.com/example/di_in_go/internal/infrastructure/validator"
)

// contextAwareHandler wraps a UserHandler to demonstrate pulling
// enriched context values before delegating to the service.
type contextAwareHandler struct {
	inner *rest.UserHandler
	log   application.Logger
}

func newContextAwareHandler(svc application.UserService, log application.Logger) *contextAwareHandler {
	return &contextAwareHandler{
		inner: rest.NewUserHandler(svc, log),
		log:   log,
	}
}

// Create enriches the logger from context before delegating to the shared handler.
// In a real system you would also read AuthUser from context for authorisation checks.
func (h *contextAwareHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Read scoped values from context — no parameter drilling needed.
	reqID := rest.RequestIDFromContext(r.Context())
	enriched := h.log.With("request_id", reqID)
	enriched.Info("handling create user request")

	// Delegate to the inner handler which calls the service.
	h.inner.Create(w, r)
}

func (h *contextAwareHandler) Get(w http.ResponseWriter, r *http.Request) {
	h.inner.Get(w, r)
}

func main() {
	// ── Singletons ──
	log := logger.New()
	db := infradb.New()
	val := validator.New()
	emailSvc := email.NewStub(log)
	repo := repository.NewMemory(db, log)
	svc := application.NewUserService(repo, emailSvc, val, log)
	h := newContextAwareHandler(svc, log)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", h.Create)
	mux.HandleFunc("GET /users/{id}", h.Get)

	// Middleware chain — each layer adds one scoped value to context:
	//   RequestIDMiddleware → injects RequestID (Scoped)
	//   AuthMiddleware      → injects AuthUser  (Scoped)
	//   TxMiddleware        → injects Tx        (Scoped, commits/rolls back after handler)
	srv := &http.Server{
		Addr: ":8082",
		Handler: rest.Chain(mux,
			rest.RequestIDMiddleware,
			rest.AuthMiddleware,
			rest.TxMiddleware(db, log),
		),
	}

	go func() {
		log.Info("03_context_scope listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	srv.Shutdown(context.Background())
}

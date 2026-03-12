// Approach 7 — Middleware-created Request-Scoped Dependencies
//
// ScopeFactory (Singleton) produces a RequestScope on every HTTP request.
// ScopeMiddleware is the scope boundary: it calls factory.NewScope, attaches
// the result to context, and defers cleanup when the handler returns.
// ScopedUserHandler reads application.UserService from the RequestScope — it
// stores NO service references directly, making it trivially testable.
//
// This is the closest Go analog to AddScoped (ASP.NET) / REQUEST scope (NestJS).
//
// Lifecycle:
//
//	Singleton  → DB, Logger, Validator, EmailService, ScopeFactory
//	Scoped     → RequestScope, enriched Logger, UserRepository, UserService
//	Transient  → none
//
// Run: go run ./cmd/07_middleware_scope/
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
	"github.com/example/di_in_go/internal/domain"
	infradb "github.com/example/di_in_go/internal/infrastructure/db"
	"github.com/example/di_in_go/internal/infrastructure/email"
	"github.com/example/di_in_go/internal/infrastructure/logger"
	"github.com/example/di_in_go/internal/infrastructure/validator"
)

func main() {
	// ── Singletons — created once at startup ──────────────────────────────────
	log := logger.New()
	db := infradb.New()
	val := validator.New()
	emailSvc := email.NewStub(log)

	// ScopeFactory is a Singleton that produces Scoped objects on demand.
	// The RepoConstructor keeps ScopeFactory decoupled from concrete infra packages.
	factory := rest.NewScopeFactory(
		func(scopedLog application.Logger) domain.UserRepository {
			return repository.NewMemory(db, scopedLog)
		},
		emailSvc,
		val,
		log,
	)

	// ScopedUserHandler holds only the base logger — no service references.
	h := rest.NewScopedUserHandler(log)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", h.Create)
	mux.HandleFunc("GET /users/{id}", h.Get)

	// Middleware chain:
	//   RequestIDMiddleware → injects RequestID into ctx    (Scoped string value)
	//   ScopeMiddleware     → creates RequestScope, injects into ctx  (Scoped object)
	srv := &http.Server{
		Addr: ":8086",
		Handler: rest.Chain(mux,
			rest.RequestIDMiddleware,
			rest.ScopeMiddleware(factory, log),
		),
	}

	go func() {
		log.Info("07_middleware_scope listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down")
	srv.Shutdown(context.Background())
}
